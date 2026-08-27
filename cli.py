#!/usr/bin/env python3
"""
taproot_cli.py — interactive SQL shell for a running taproot server.

Talks to the server's HTTP endpoint (POST /query, raw SQL body, JSON
response) added in server/server.go. Two modes:

  - Interactive (stdin is a TTY): a REPL with live syntax highlighting
    while you type, multi-line statements (end with ';'), history, and
    colorized table output.
  - Batch (stdin is piped/redirected): reads SQL from stdin, splits on
    ';', runs each statement, prints results. Handy for scripting:
        cat schema.sql | python3 taproot_cli.py

Usage:
    python3 taproot_cli.py                      # connect to localhost:8080
    python3 taproot_cli.py --host db.local --port 9090
    python3 taproot_cli.py -c "SELECT * FROM users"
    echo "SHOW TABLES;" | python3 taproot_cli.py

Dependencies: requests, prompt_toolkit, rich
    pip install requests prompt_toolkit rich
"""

import argparse
import os
import re
import sys
import time

import requests
from prompt_toolkit import PromptSession
from prompt_toolkit.auto_suggest import AutoSuggestFromHistory
from prompt_toolkit.history import FileHistory
from prompt_toolkit.lexers import Lexer
from prompt_toolkit.styles import Style
from rich.console import Console
from rich.markup import escape
from rich.panel import Panel
from rich.table import Table

# --------------------------------------------------------------------------
# SQL syntax highlighting (mirrors the keyword set in sql/tokenizer.go)
# --------------------------------------------------------------------------

KEYWORDS = {
    "SELECT", "FROM", "WHERE",
    "INSERT", "INTO", "VALUES",
    "UPDATE", "SET",
    "DELETE",
    "CREATE", "TABLE", "DROP",
    "DESC", "DESCRIBE",
    "SHOW", "TABLES",
    "PRIMARY", "KEY",
    "AND", "OR", "NOT",
    "NULL", "TRUE", "FALSE",
    "INT", "INTEGER", "TEXT", "VARCHAR", "STRING",
    "FLOAT", "REAL", "DOUBLE", "BOOL", "BOOLEAN",
}

TOKEN_RE = re.compile(
    r"""
      (?P<string>'[^']*'|"[^"]*")
    | (?P<number>\d+\.\d+|\d+)
    | (?P<word>[A-Za-z_][A-Za-z0-9_]*)
    | (?P<op>!=|<>|<=|>=|[=<>(),;*])
    """,
    re.VERBOSE,
)

PROMPT_STYLE = Style.from_dict({
    "sql.keyword": "bold #61afef",
    "sql.string": "#98c379",
    "sql.number": "#d19a66",
    "sql.identifier": "#e5c07b",
    "sql.operator": "#c678dd",
    "prompt": "bold #56b6c2",
})


class SQLLexer(Lexer):
    """Token-level syntax highlighting for the prompt_toolkit input buffer."""

    def lex_document(self, document):
        lines = document.lines

        def get_line(lineno):
            line = lines[lineno]
            fragments = []
            pos = 0
            for m in TOKEN_RE.finditer(line):
                if m.start() > pos:
                    fragments.append(("", line[pos:m.start()]))
                kind = m.lastgroup
                text = m.group()
                if kind == "string":
                    style = "class:sql.string"
                elif kind == "number":
                    style = "class:sql.number"
                elif kind == "word":
                    style = "class:sql.keyword" if text.upper() in KEYWORDS else "class:sql.identifier"
                elif kind == "op":
                    style = "class:sql.operator"
                else:
                    style = ""
                fragments.append((style, text))
                pos = m.end()
            if pos < len(line):
                fragments.append(("", line[pos:]))
            return fragments

        return get_line


# --------------------------------------------------------------------------
# Server client
# --------------------------------------------------------------------------

class TaprootError(Exception):
    pass


class TaprootClient:
    def __init__(self, host="localhost", port=8080, timeout=10):
        self.base_url = f"http://{host}:{port}"
        self.timeout = timeout
        self.session = requests.Session()

    def health(self):
        try:
            resp = self.session.get(f"{self.base_url}/health", timeout=self.timeout)
            return resp.status_code == 200
        except requests.exceptions.RequestException:
            return False

    def query(self, sql_text):
        try:
            resp = self.session.post(
                f"{self.base_url}/query",
                data=sql_text.encode("utf-8"),
                timeout=self.timeout,
                headers={"Connection": "close"},
            )
        except requests.exceptions.ConnectionError as e:
            raise TaprootError(f"could not connect to {self.base_url} — is the server running?") from e
        except requests.exceptions.Timeout as e:
            raise TaprootError(f"request to {self.base_url} timed out") from e
        except requests.exceptions.RequestException as e:
            raise TaprootError(str(e)) from e

        try:
            return resp.json()
        except ValueError as e:
            raise TaprootError(f"server returned a non-JSON response: {resp.text[:200]!r}") from e


# --------------------------------------------------------------------------
# Statement buffering (need to know when a multi-line statement is "done",
# without cutting a ';' inside a string literal)
# --------------------------------------------------------------------------

def statement_is_complete(buffer):
    text = buffer.rstrip()
    if not text.endswith(";"):
        return False
    in_string = None
    for ch in text:
        if in_string:
            if ch == in_string:
                in_string = None
        elif ch in ("'", '"'):
            in_string = ch
    return in_string is None


def split_statements(text):
    statements = []
    buf = ""
    in_string = None
    for ch in text:
        buf += ch
        if in_string:
            if ch == in_string:
                in_string = None
        elif ch in ("'", '"'):
            in_string = ch
        elif ch == ";" and in_string is None:
            stripped = buf.strip()
            if stripped and stripped != ";":
                statements.append(stripped)
            buf = ""
    tail = buf.strip()
    if tail:
        statements.append(tail)
    return statements


# --------------------------------------------------------------------------
# Output rendering
# --------------------------------------------------------------------------

def format_value(v):
    if v is None:
        return "[dim italic]NULL[/dim italic]"
    if isinstance(v, bool):
        return "[green]TRUE[/green]" if v else "[red]FALSE[/red]"
    if isinstance(v, (int, float)):
        return f"[cyan]{v}[/cyan]"
    return f"[yellow]{escape(str(v))}[/yellow]"


def render_result(console, result, elapsed=None):
    if result.get("Error"):
        console.print(f"[bold red]Error:[/bold red] {escape(result['Error'])}")
        return

    columns = result.get("Columns")
    rows = result.get("Rows") or []

    if columns:
        numeric_cols = []
        for i in range(len(columns)):
            vals = [row[i] for row in rows if row[i] is not None]
            is_numeric = bool(vals) and all(
                isinstance(v, (int, float)) and not isinstance(v, bool) for v in vals
            )
            numeric_cols.append(is_numeric)

        table = Table(show_header=True, header_style="bold cyan", border_style="grey50")
        for col, is_num in zip(columns, numeric_cols):
            table.add_column(escape(col), justify="right" if is_num else "left")

        for row in rows:
            table.add_row(*(format_value(v) for v in row))

        console.print(table)
        n = len(rows)
        suffix = f" in {elapsed:.3f}s" if elapsed is not None else ""
        console.print(f"[dim]{n} row{'s' if n != 1 else ''}{suffix}[/dim]")
    else:
        msg = result.get("Message") or ""
        suffix = f" [dim]({elapsed:.3f}s)[/dim]" if elapsed is not None else ""
        console.print(f"[bold green]\u2713[/bold green] {escape(msg)}{suffix}")


def execute_and_render(client, console, stmt):
    start = time.monotonic()
    try:
        result = client.query(stmt)
    except TaprootError as e:
        console.print(f"[bold red]Error:[/bold red] {escape(str(e))}")
        return
    elapsed = time.monotonic() - start
    render_result(console, result, elapsed)


# --------------------------------------------------------------------------
# Meta-commands
# --------------------------------------------------------------------------

META_HELP = """\
[bold]Meta-commands[/bold]
  .tables               list tables (shortcut for SHOW TABLES)
  .describe <table>     describe a table (shortcut for DESC <table>)
  .clear                clear the screen
  .help                 show this help
  .exit, .quit          leave the shell

SQL statements end with ';' and can span multiple lines.
Supported: CREATE TABLE, DROP TABLE, DESC/DESCRIBE, SHOW TABLES,
INSERT, SELECT, UPDATE, DELETE.
WHERE supports: =  !=  <>  <  <=  >  >=  AND  OR
"""


def handle_meta_command(line, client, console):
    """Returns True if the shell should exit."""
    parts = line.split(maxsplit=1)
    cmd = parts[0].lower()
    arg = parts[1].strip() if len(parts) > 1 else ""

    if cmd in (".exit", ".quit", ".q"):
        return True
    if cmd in (".help", ".h"):
        console.print(Panel(META_HELP, title="taproot", border_style="cyan"))
        return False
    if cmd == ".tables":
        execute_and_render(client, console, "SHOW TABLES")
        return False
    if cmd in (".describe", ".desc"):
        if not arg:
            console.print("[red]usage: .describe <table>[/red]")
        else:
            execute_and_render(client, console, f"DESC {arg}")
        return False
    if cmd in (".clear", ".cls"):
        console.clear()
        return False

    console.print(f"[red]unknown meta-command:[/red] {escape(cmd)} (try .help)")
    return False


# --------------------------------------------------------------------------
# Interactive REPL
# --------------------------------------------------------------------------

def print_banner(console, client):
    console.print(Panel.fit(
        "[bold cyan]taproot[/bold cyan] SQL shell\n"
        f"[dim]connected to {client.base_url}[/dim]\n"
        "[dim]type .help for meta-commands, end statements with ;[/dim]",
        border_style="cyan",
    ))


def run_interactive(client, console):
    history_path = os.path.expanduser("~/.taproot_history")
    session = PromptSession(
        history=FileHistory(history_path),
        auto_suggest=AutoSuggestFromHistory(),
        lexer=SQLLexer(),
        style=PROMPT_STYLE,
    )

    print_banner(console, client)

    buffer = ""
    while True:
        prompt_label = "taproot> " if not buffer else "     -> "
        try:
            line = session.prompt([("class:prompt", prompt_label)])
        except KeyboardInterrupt:
            buffer = ""
            console.print()
            continue
        except EOFError:
            console.print()
            break

        if not buffer and line.strip().startswith("."):
            if handle_meta_command(line.strip(), client, console):
                break
            continue

        buffer += line + "\n"

        if statement_is_complete(buffer):
            stmt = buffer.strip()
            buffer = ""
            if stmt and stmt != ";":
                execute_and_render(client, console, stmt)

    console.print("[dim]bye[/dim]")


def run_batch(client, console):
    text = sys.stdin.read()
    statements = split_statements(text)

    exit_code = 0
    for stmt in statements:
        try:
            result = client.query(stmt)
        except TaprootError as e:
            console.print(f"[bold red]Error:[/bold red] {escape(str(e))}")
            exit_code = 1
            continue
        render_result(console, result)
        if result.get("Error"):
            exit_code = 1

    return exit_code


# --------------------------------------------------------------------------
# Entry point
# --------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(description="Interactive SQL shell for taproot")
    parser.add_argument("--host", default=os.environ.get("TAPROOT_HOST", "localhost"))
    parser.add_argument("--port", type=int, default=int(os.environ.get("TAPROOT_PORT", "8080")))
    parser.add_argument("-c", "--command", help="run a single statement and exit")
    args = parser.parse_args()

    console = Console()
    client = TaprootClient(host=args.host, port=args.port)

    if not client.health():
        console.print(f"[bold red]Could not reach taproot server at {client.base_url}[/bold red]")
        console.print("[dim]Start it with: TAPROOT_DATA_DIR=./data go run .[/dim]")
        sys.exit(1)

    if args.command:
        execute_and_render(client, console, args.command)
        return

    if sys.stdin.isatty():
        run_interactive(client, console)
    else:
        sys.exit(run_batch(client, console))


if __name__ == "__main__":
    main()
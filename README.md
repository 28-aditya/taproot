<div align="center">

<img src="assets/logonbg.png" alt="Taproot logo" width="180">

# taproot

A database, built from scratch in Go — no framework, no Postgres or SQLite underneath it, no ORM.

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-in%20progress-yellow?style=flat-square)

</div>

---

## What this is

Raw TCP socket all the way down to bytes on disk. No frameworks handling the HTTP parsing, no existing database doing the storage, no ORM in between.

```
client (Python)
  │
  ▼
my own HTTP server           (raw TCP sockets, Go)
  │
  ▼
my own SQL parser + executor  (tokenizer → AST → execution)
  │
  ▼
my own B+tree storage engine  (insert / search / delete)
  │
  ▼
disk file                    (persisted, survives a restart)
```

---

## How I'm building it

Rough order, since each piece depends on the last one working:

**1. HTTP server first. ** Just a raw socket listener — accept a connection, read the bytes, parse out the method/path/headers/body, write a response back. No `net/http`, done by hand. Split across `server.go` (listener + accept loop), `request.go` (parsing raw bytes into a `Request`), and `response.go` (writing status/headers/body back).

**2. In-memory B+tree.  In progress.** Insert, search, delete — no disk yet. Node struct, tree wrapper, `findLeaf`, `SearchTree`, and easy-case `Insert` (no splitting) are done. Still to build: leaf splitting, cascading splits up through parent/root, and a range-scan function over the leaf chain (needed for `SELECT ... WHERE`). Delete is deliberately being left minimal — not a priority for this project.

**3. Make it persistent.** Fixed-size pages written to a file, node splits creating new pages, parent pointers updated. The real test here: insert data, kill the process, start it again, data's still there.

**4. SQL parser + executor.** Small subset to start — `CREATE TABLE`, `INSERT`, `SELECT ... WHERE`. Tokenize the query string, build a tree out of the tokens, walk that tree and call into the B+tree to actually do the work. `UPDATE`, `DELETE`, joins, and `ALTER` are stretch goals, not core scope.

**5. Wire it all together.** HTTP endpoint takes raw SQL, runs it through the parser, executes it against the storage engine, sends back the result as JSON. Also building a small CLI client in Python (colored output, startup screen with the logo) so I'm not typing curl commands with escaped SQL strings the whole time — the client is pure presentation, just sending/receiving over HTTP, so it doesn't touch the "built from scratch" scope of the server/storage/SQL layers.

**6. Write-up.** Once it works, documenting why I made the calls I did (page size, B-tree vs B+tree, why I stopped at equality-only `WHERE`, etc.) — that's most of the value for anyone (including future me) reading this later.

---

## Running it

```bash
go run main.go
```

in one terminal, then in another:

```bash
python client.py
```

```
taproot> CREATE TABLE users (id, name)
OK

taproot> INSERT INTO users VALUES (1, 'Aditya')
OK, 1 row affected

taproot> SELECT * FROM users
+----+--------+
| id | name   |
+----+--------+
| 1  | Aditya |
+----+--------+
```

---

## Status

- [x] HTTP server
- [ ] In-memory B+tree (search/easy-insert done, splitting + range scan left)
- [ ] Disk persistence
- [ ] SQL parser + executor
- [ ] Everything wired together + CLI client
- [ ] Write-up
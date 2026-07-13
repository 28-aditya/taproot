<div align="center">

<!-- TODO: drop your mascot/logo here, e.g. assets/mascot.png -->
<img src="assets/logonbg.png" alt="Taproot mascot" width="180"><br>

# 🌱 taproot

**A database, built from the ground up. No framework, no external DB, no shortcuts.**

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-in%20progress-yellow?style=flat-square)
![License](https://img.shields.io/badge/license-MIT-blue?style=flat-square)

</div>

---

## What is this?

taproot is a relational database, written from scratch in Go — down to the socket.

There's no framework parsing your HTTP requests, no Postgres or SQLite underneath, no ORM. Every layer between "a client sends a query" and "bytes land on disk" is code in this repo. The name comes from the B-tree at its core — every lookup starts at the root and grows down from there, same as a taproot anchors and feeds an entire tree.

```
client
  │
  ▼
your own HTTP server        (raw TCP sockets)
  │
  ▼
your own SQL parser + executor   (tokenizer → AST → execution)
  │
  ▼
your own B-tree storage engine   (insert / search / delete)
  │
  ▼
disk file                   (persisted, survives a restart)
```

The goal isn't to build something production-ready — it's to actually understand what a database *is*, by building one. As Feynman put it: what I cannot create, I do not understand.

---

## Usage

Once it's running, it looks like this:

```bash
$ go run server.go &
Server listening on :4221

$ go run client.go
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

taproot> exit
```

Two terminals: one running the server, one running the interactive client shell.

---

## Roadmap

- [ ] **HTTP server** — raw TCP listener, request parsing, routing, concurrent connections
- [ ] **In-memory B+tree** — insert / search / delete, correctness before persistence
- [ ] **Disk persistence** — fixed-size pages, node splits, survives a process restart
- [ ] **SQL parser + executor** — tokenizer → AST → execution against the B+tree
- [ ] **Wire it together** — HTTP endpoint runs SQL against the storage engine, plus an interactive CLI client
- [ ] **Write-up** — architecture notes on *why*, not just *what*

---

## Why Go

Enough distance from Python/JS to be a genuinely new skill and a decent low-level-systems signal, without the borrow-checker tax of Rust or the memory-management tax of C eating the whole timeline. Zero non-standard-library dependencies.

---

## Built with / learned from

- [CodeCrafters](https://codecrafters.io) — "Build your own HTTP server" and "Build your own SQLite" challenge tracks
- [*Build Your Own Database From Scratch in Go*](https://build-your-own.org/database/) — James Smith

Read, understood, then implemented independently — not copy-pasted.

---

## License

MIT

<div align="center">
<sub>Every query starts at the root.</sub>
</div>

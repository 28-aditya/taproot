<div align="center">

<img src="assets/logonbg.png" alt="Taproot logo" width="180">

# taproot

A database, built from scratch in Go — no framework, no Postgres or SQLite underneath it, no ORM.

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-in%20progress-yellow?style=flat-square)

</div>

---

## What this is

I wanted to actually understand what a database is doing under the hood instead of just importing one, so I'm building one myself — from the raw TCP socket all the way down to bytes on disk. No frameworks handling the HTTP parsing, no existing database doing the storage, no ORM in between.

The name's from the B-tree at the core of it — every lookup starts at the root node and works its way down, same idea as a taproot.

```
client
  │
  ▼
my own HTTP server           (raw TCP sockets)
  │
  ▼
my own SQL parser + executor  (tokenizer → AST → execution)
  │
  ▼
my own B-tree storage engine  (insert / search / delete)
  │
  ▼
disk file                    (persisted, survives a restart)
```

---

## How I'm building it

Rough order, since each piece depends on the last one working:

**1. HTTP server first.** Just a raw socket listener — accept a connection, read the bytes, parse out the method/path/headers, write a response back. No `net/http`, doing this by hand. Fastest way to get something real running, and it's the layer everything else eventually plugs into.

**2. In-memory B+tree.** Insert, search, delete — no disk yet. Get the actual tree logic right before adding the complexity of persistence on top. Testing this by throwing a few thousand random keys at it and checking everything comes back sorted and correct.

**3. Make it persistent.** Fixed-size pages written to a file, node splits creating new pages, parent pointers updated. The real test here: insert data, kill the process, start it again, data's still there.

**4. SQL parser + executor.** Small subset to start — `CREATE TABLE`, `INSERT`, `SELECT ... WHERE`. Tokenize the query string, build a tree out of the tokens, walk that tree and call into the B-tree to actually do the work.

**5. Wire it all together.** HTTP endpoint takes raw SQL, runs it through the parser, executes it against the storage engine, sends back the result as JSON. Also building a small CLI client so I'm not typing curl commands with escaped SQL strings the whole time.

**6. Write-up.** Once it works, documenting why I made the calls I did (page size, B-tree vs B+tree, etc.) — that's most of the value for anyone (including future me) reading this later.

---

## Running it

```bash
go run server.go
```

in one terminal, then in another:

```bash
go run client.go
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

- [ ] HTTP server
- [ ] In-memory B+tree
- [ ] Disk persistence
- [ ] SQL parser + executor
- [ ] Everything wired together + CLI client
- [ ] Write-up
package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"taproot/sql"
)

var (
	catalog   *sql.Catalog
	catalogMu sync.Mutex 
)

func Start() {
	dataDir := os.Getenv("TAPROOT_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	var err error
	catalog, err = sql.OpenCatalog(dataDir)
	if err != nil {
		log.Fatalf("Failed to open catalog at %q: %v", dataDir, err)
	}

	ln, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Port binding failed: %v", err)
	}

	defer ln.Close()

	fmt.Printf("Server is listening on port 8080 (data dir: %s)\n", dataDir)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	err := conn.SetDeadline(time.Now().Add(5 * time.Minute))
	if err != nil {
		log.Printf("Failed to set connection deadline: %v", err)
		return
	}

	reader := bufio.NewReader(conn)

	req, err := parseRequest(reader)
	if err != nil {
		log.Printf("Failed to parse request: %v", err)
		return
	}

	log.Printf("\n=====================\nParsed request\nMethod=%v\nPath=%v\n=====================", req.Method, req.Path)

	switch {
	case req.Method == "GET" && req.Path == "/health":
		writeResponse(conn, "200 OK", "OK")

	case req.Method == "POST" && req.Path == "/query":
		handleQuery(conn, req)

	default:
		writeResponse(conn, "200 OK", "OK")
	}
}

func handleQuery(conn net.Conn, req Request) {
	query := string(req.Body)

	catalogMu.Lock()
	result, err := sql.Run(query, catalog)
	catalogMu.Unlock()

	if err != nil {
		writeJSON(conn, "400 Bad Request", map[string]string{"Error": err.Error()})
		return
	}

	writeJSON(conn, "200 OK", result)
}
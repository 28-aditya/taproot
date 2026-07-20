package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

func Start() {
	ln, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Port binding failed: %v", err)
	}

	defer ln.Close()

	fmt.Println("Server is listening on port 8080")

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

	switch req.Path {
	default:
		writeResponse(conn, "200 OK", "OK")
	}
}
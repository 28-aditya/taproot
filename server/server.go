package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type Request struct {
    Method  string
    Path    string
    Version string
    Headers map[string]string
}

func Start() {
	ln, err := net.Listen("tcp", ":8080")

	if err != nil {
		log.Fatalf("Port binding failed: %v", err)
	}

	defer ln.Close()		// closes server if connection fails

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

	err:= conn.SetDeadline(time.Now().Add(5*time.Minute))

	if err != nil {
		log.Printf("Failed to set connection deadline: %v",err)
		return
	}

	reader := bufio.NewReader(conn)

	headers := make(map[string]string)

	requestLine, err := reader.ReadString('\n')
	
	if err != nil {
		log.Printf("Failed to get HTTP path header")
	}

	parts := strings.Fields(requestLine)
	method := parts[0]
	path := parts[1]
	version := parts[2]

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Client left or error occurred: %v", err)
			break
		}

		message = strings.TrimRight(message,"\r\n")

		if message == "" {
			break
		}

		key, value, found := strings.Cut(message, ": ")

		if found {
			headers[key] = value
		}
	}

	req := Request{
		Method: method,
		Path: path,
		Version: version,
		Headers: headers,
	}

	log.Printf("\n=====================\nParsed request\nMethod=%v\nPath=%v\n=====================", req.Method, req.Path)

	switch req.Path {
	default:
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nOK"))
	}
}
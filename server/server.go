package server

import (
	"fmt"
	"log"
	"net"
	"time"
	"bufio"
	"strings"
)

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

	parts := strings.Fields(requestLine)
	method := parts[0]
	path := parts[1]
	version := parts[2]

	if err != nil {
		log.Printf("Failed to get HTTP path header")
	}


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
}
package server

import (
	"bufio"
	"io"
	"log"
	"strconv"
	"strings"
	"fmt"
)

type Request struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    []byte
}

func parseRequest(reader *bufio.Reader) (Request, error) {
	headers := make(map[string]string)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Failed to get HTTP path header")
		return Request{}, err
	}

	parts := strings.Fields(requestLine)
	if len(parts) < 3 {
		return Request{}, fmt.Errorf("malformed request line: %q", requestLine)
	}
	method := parts[0]
	path := parts[1]
	version := parts[2]

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Client left or error occurred: %v", err)
			break
		}

		message = strings.TrimRight(message, "\r\n")

		if message == "" {
			break
		}

		key, value, found := strings.Cut(message, ": ")

		if found {
			headers[key] = value
		}
	}

	var body []byte

	bodyLenStr, ok := headers["Content-Length"]

	if ok {
		bodyLen, err := strconv.Atoi(bodyLenStr)
		if err != nil {
			log.Printf("Body length conversion error: %v", err)
			return Request{}, err
		}

		body = make([]byte, bodyLen)

		_, err = io.ReadFull(reader, body)
		if err != nil {
			log.Printf("I/O read error: %v", err)
			return Request{}, err
		}
	}

	req := Request{
		Method:  method,
		Path:    path,
		Version: version,
		Headers: headers,
		Body:    body,
	}

	return req, nil
}
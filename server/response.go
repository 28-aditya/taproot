package server

import (
	"encoding/json"
	"fmt"
	"net"
)

func writeResponse(conn net.Conn, status string, body string) {
	fmt.Fprintf(conn, "HTTP/1.1 %s\r\n", status)
	fmt.Fprintf(conn, "Content-Length: %d\r\n", len(body))
	fmt.Fprintf(conn, "Connection: close\r\n")
	fmt.Fprintf(conn, "\r\n")
	fmt.Fprint(conn, body)
}

func writeJSON(conn net.Conn, status string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		body := `{"Error":"failed to encode response"}`
		fmt.Fprintf(conn, "HTTP/1.1 500 Internal Server Error\r\n")
		fmt.Fprintf(conn, "Content-Type: application/json\r\n")
		fmt.Fprintf(conn, "Content-Length: %d\r\n", len(body))
		fmt.Fprintf(conn, "Connection: close\r\n")
		fmt.Fprintf(conn, "\r\n")
		fmt.Fprint(conn, body)
		return
	}

	fmt.Fprintf(conn, "HTTP/1.1 %s\r\n", status)
	fmt.Fprintf(conn, "Content-Type: application/json\r\n")
	fmt.Fprintf(conn, "Content-Length: %d\r\n", len(data))
	fmt.Fprintf(conn, "Connection: close\r\n")
	fmt.Fprintf(conn, "\r\n")
	conn.Write(data)
}
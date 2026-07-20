package server

import (
	"fmt"
	"net"
)

func writeResponse(conn net.Conn, status string, body string) {
	fmt.Fprintf(conn, "HTTP/1.1 %s\r\n", status)
	fmt.Fprintf(conn, "Content-Length: %d\r\n", len(body))
	fmt.Fprintf(conn, "\r\n")
	fmt.Fprint(conn, body)
}
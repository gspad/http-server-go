package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

const (
	PROTOCOL                = "HTTP/1.1 "
	HTTP_STATUS_OK          = "200 OK"
	HTTP_STATUS_BAD_REQUEST = "400 Bad Request"
	HTTP_STATUS_NOT_FOUND   = "404 Not Found"
	CONTENT_TYPE            = "Content-Type: text/plain"
	CONTENT_LENGTH          = "Content-Length: "
)

func main() {
	listener := RealListener{}
	ctx := context.Background()
	defer ctx.Done()
	err := StartServer(ctx, listener, "4221")
	if err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
}

type NetworkListener interface {
	Listen(network, address string) (net.Listener, error)
}

func StartServer(ctx context.Context, listener NetworkListener, port string) error {
	fmt.Println("Starting server on port:", port)

	l, err := listener.Listen("tcp", "0.0.0.0:"+port)
	if err != nil {
		return fmt.Errorf("failed to bind to port %s", port)
	}

	defer l.Close()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Server stopping")
			return nil
		default:
			conn, err := l.Accept()
			if err != nil {
				fmt.Println(fmt.Errorf("error accepting connection %v", err))
			}
			go handleConnection(conn)
		}
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	dataBuffer := make([]byte, 0, 4096)
	buf := make([]byte, 4096)

	n, err := conn.Read(buf)
	if err != nil {
		if err != io.EOF {
			fmt.Println("Error reading:", err)
			return
		}
	}

	dataBuffer = append(dataBuffer, buf[:n]...)
	handleHttpRequest(conn, dataBuffer)
}

func handleHttpRequest(conn net.Conn, data []byte) {
	dataString := string(data)
	lines := strings.Split(dataString, "\r\n")
	method := strings.Split(lines[0], " ")[0]

	if len(method) < 2 {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_BAD_REQUEST + "\r\n\r\n"))
		return
	}
	switch method {
	case "GET":
		handleGetRequest(conn, lines)
	default:
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
	}
}

func handleGetRequest(conn net.Conn, headerLines []string) {
	path := strings.Split(headerLines[0], " ")[1]

	if path == "/" {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_OK + "\r\n\r\n"))
	} else if strings.HasPrefix(path, "/echo/") {
		content := strings.TrimPrefix(path, "/echo/")
		response := fmt.Sprintf(
			PROTOCOL+HTTP_STATUS_OK+"\r\n"+CONTENT_TYPE+"\r\n"+CONTENT_LENGTH+"%d\r\n\r\n%s", len(content), content)
		conn.Write([]byte(response))
	} else if path == "/user-agent" {
		userAgentValue := ""

		for _, line := range headerLines {
			header := strings.Split(line, ":")
			if header[0] == "User-Agent" {
				userAgentValue = strings.TrimSpace(header[1])
				break
			}
		}

		response := fmt.Sprintf(PROTOCOL+HTTP_STATUS_OK+"\r\n"+CONTENT_TYPE+"\r\n"+CONTENT_LENGTH+"%d\r\n\r\n%s", len(userAgentValue), userAgentValue)
		conn.Write([]byte(response))
	} else {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
	}
}

type RealListener struct{}

func (RealListener) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

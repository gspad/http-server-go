package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
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
	buf := make([]byte, 1024)

	for {
		// Set a timeout for reading from the connection
		timeoutDuration := 2 * time.Minute
		err := conn.SetReadDeadline(time.Now().Add(timeoutDuration))
		if err != nil {
			fmt.Println("Error setting deadline:", err)
			break
		}

		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				fmt.Println("Error reading:", err)
			}
			break
		}

		dataBuffer := append(dataBuffer, buf[:n]...)

		if !strings.Contains(string(dataBuffer), "\r\n\r\n") {
			continue
		}

		handleHttpRequest(conn, dataBuffer)
		dataBuffer = dataBuffer[:0]
	}
}

func handleHttpRequest(conn net.Conn, data []byte) {
	dataString := string(data)
	path := strings.Split(dataString, " ")
	if len(path) < 2 {
		conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
		return
	}
	switch path[0] {
	case "GET":
		handleGetRequest(conn, path[1])
	default:
		conn.Write([]byte("HTTP/1.1 405 Method Not Allowed\r\n\r\n"))
	}
}

func handleGetRequest(conn net.Conn, path string) {
	if path == `/` {
		conn.Write([]byte(`HTTP/1.1 200 OK\r\n\r\n`))
		println("GETS HERE")
	} else if len(path) > 1 {
		content := path[1:]
		response := fmt.Sprintf(
			`HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s`, len(content), content)
		conn.Write([]byte(response))
	} else {
		conn.Write([]byte(`HTTP/1.1 404 Not Found\r\n\r\n`))
	}
}

type RealListener struct{}

func (RealListener) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
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
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)

	if err != nil {
		fmt.Println("Error reading:", err)
	}

	data := string(buf[:n])
	println("Received data:", data)

	path := strings.Split(data, " ")

	if path[0] == "GET" {
		if path[1] == "/" {
			conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		} else if path[1] == "/echo/abc" {
			content := strings.Split(path[1], "/")

			response := fmt.Sprintf(
				`HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s`, len(content[2]), content[2])

			fmt.Printf("Production response: %q\n", response)

			conn.Write([]byte(response))
		} else {
			conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		}
	}
}

type RealListener struct{}

func (RealListener) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
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
	conn.Read(buf)

	data := buf[:6]

	if string(data) == "GET / " {
		conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	} else {
		conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	}
}

type RealListener struct{}

func (RealListener) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

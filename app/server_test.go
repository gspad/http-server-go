package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/spf13/afero"
)

type FakeListener struct {
	connChan chan net.Conn
	closed   chan struct{}
}

func NewFakeListener() *FakeListener {
	return &FakeListener{
		connChan: make(chan net.Conn),
		closed:   make(chan struct{}),
	}
}

func (fl *FakeListener) Listen(network, address string) (net.Listener, error) {
	return fl, nil
}

func (fl *FakeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-fl.connChan:
		return conn, nil
	case <-fl.closed:
		return nil, fmt.Errorf("Listener closed")
	}
}

func (fl *FakeListener) Close() error {
	close(fl.closed)
	return nil
}

func (fl *FakeListener) Addr() net.Addr {
	return nil
}

func (fl *FakeListener) QueueConn(conn net.Conn) {
	fl.connChan <- conn
}

type FakeConn struct {
	net.Conn
	readData  []byte
	writeData chan []byte
}

func (fc *FakeConn) Read(b []byte) (n int, err error) {
	if len(fc.readData) == 0 {
		return 0, io.EOF
	}

	n = copy(b, fc.readData)
	fc.readData = fc.readData[n:]
	return n, nil
}

func (fc *FakeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (fc *FakeConn) Write(b []byte) (n int, err error) {
	fc.writeData <- b
	return len(b), nil
}

func (fc *FakeConn) Close() error {
	println("Closing connection")
	close(fc.writeData)
	return nil
}

func setupTest() (*Server, *FakeListener, context.Context, context.CancelFunc) {
	fakeListener := NewFakeListener()
	inMemoryFs := afero.NewMemMapFs()

	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	flags.String("directory", "", "Directory to serve files from")
	err := flags.Set("directory", "/home/projects/codecrafters-http-server-go/testfiles")
	if err != nil {
		panic(err)
	}

	flags.Parse([]string{})
	parsedFlags := make(map[string]string)
	flags.VisitAll(func(f *flag.Flag) {
		parsedFlags[f.Name] = f.Value.String()
	})

	config := ServerConfig{
		l:     fakeListener,
		fs:    inMemoryFs,
		flags: parsedFlags,
	}

	server := NewServer(config)
	ctx, cancel := context.WithCancel(context.Background())

	return server, fakeListener, ctx, cancel
}

func TestMyTCPServer(t *testing.T) {
	server, fl, ctx, cancel := setupTest()
	defer cancel()

	writeData := make(chan []byte)
	readData := make([]byte, 1024)
	copy(readData, "GET / HTTP/1.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)

	go server.Start(ctx, "4221")

	for {
		select {
		case buf := <-writeData:
			println("Received test data:", string(buf))

			if string(buf) == "HTTP/1.1 200 OK\r\n\r\n" {
				return
			} else {
				t.Fatalf("Unexpected response from server: %s", string(buf))
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

func TestTCPBodyResponse(t *testing.T) {
	server, fl, ctx, cancel := setupTest()
	defer cancel()

	writeData := make(chan []byte)
	readData := make([]byte, 1024)
	copy(readData, "GET /echo/yikes/dooby-Coo HTTP/1.1\r\nHost: localhost:4221\r\nUser-Agent: curl/7.64.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)

	go server.Start(ctx, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) == "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 15\r\n\r\nyikes/dooby-Coo" {
				return
			} else {
				t.Fatalf("Unexpected response from server: %q", string(buf))
				return
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

func TestParsingHeaders(t *testing.T) {
	server, fl, ctx, cancel := setupTest()
	defer cancel()

	writeData := make(chan []byte)
	readData := make([]byte, 1024)

	copy(readData, "GET /user-agent HTTP/1.1\r\nHost: localhost:4221\r\nUser-Agent: curl/7.64.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)

	go server.Start(ctx, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) == "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 11\r\n\r\ncurl/7.64.1" {
				return
			} else {
				t.Fatalf("Unexpected response from server: %q", string(buf))
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

func TestParseFile(t *testing.T) {
	server, fl, ctx, cancel := setupTest()
	defer cancel()

	file, err := server.config.fs.Create("/home/projects/codecrafters-http-server-go/testfiles/testfile.txt")
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	println("filepath: ", file.Name())

	defer file.Close()

	_, err = file.Write([]byte("Hello, test data!"))
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}

	writeData := make(chan []byte)
	readData := make([]byte, 1024)

	copy(readData, "GET /files/testfile.txt HTTP/1.1\r\nHost: localhost:4221\r\nUser-Agent: curl/7.64.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)
	go server.Start(ctx, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) == "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: 17\r\n\r\nHello, test data!" {
				return
			} else {
				t.Fatalf("Unexpected response from server: %q", string(buf))
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

func TestParseFileNotFound(t *testing.T) {
	server, fl, ctx, cancel := setupTest()
	defer cancel()

	writeData := make(chan []byte)
	readData := make([]byte, 1024)

	copy(readData, "GET /files/testfile.txt HTTP/1.1\r\nHost: localhost:4221\r\nUser-Agent: curl/7.64.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)
	go server.Start(ctx, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) == "HTTP/1.1 404 Not Found\r\n\r\n" {
				return
			} else {
				t.Fatalf("Unexpected response from server: %q", string(buf))
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

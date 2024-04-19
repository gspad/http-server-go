package main

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
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
		return nil, fmt.Errorf("listener closed")
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
	copy(b, fc.readData)
	return len(fc.readData), nil
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

func TestMyTCPServer(t *testing.T) {
	fl := NewFakeListener()
	writeData := make(chan []byte)
	readData := make([]byte, 1024)
	copy(readData, "GET / HTTP/1.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go StartServer(ctx, fl, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) != "HTTP/1.1 200 OK\r\n\r\n" {
				t.Fatalf("Unexpected response from server: %s", string(buf))
			}
			return
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

func TestTCPBodyResponse(t *testing.T) {
	fl := NewFakeListener()
	writeData := make(chan []byte)
	readData := make([]byte, 1024)
	copy(readData, "GET /echo/abc HTTP/1.1\r\nHost: localhost:4221\r\nUser-Agent: curl/7.64.1\r\n\r\n")

	fakeConn := &FakeConn{
		writeData: writeData,
		readData:  readData,
	}

	go fl.QueueConn(fakeConn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go StartServer(ctx, fl, "4221")

	for {
		select {
		case buf := <-writeData:
			if string(buf) != `HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 3\r\n\r\nabc` {
				t.Fatalf("Unexpected response from server: %s", string(buf))
			}
			return
		case <-time.After(1 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

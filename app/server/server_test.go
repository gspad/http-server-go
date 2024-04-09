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

func (fl *FakeListener) Listen(network, address string) (net.Listener, error) {
	return fl, nil
}

func NewFakeListener() *FakeListener {
	return &FakeListener{
		connChan: make(chan net.Conn),
		closed:   make(chan struct{}),
	}
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
	return 0, nil
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
	fakeConn := &FakeConn{writeData: writeData}

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
		case <-time.After(3 * time.Second):
			t.Fatal("Timed out waiting for data")
		}
	}
}

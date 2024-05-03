package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/spf13/afero"
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
	var directory string
	flag.StringVar(&directory, "directory", "", "directory for file lookup")
	flag.Parse()
	parsedFlags := make(map[string]string)
	flag.VisitAll(func(f *flag.Flag) {
		parsedFlags[f.Name] = f.Value.String()
	})

	listener := RealListener{}
	fs := afero.NewOsFs()

	config := ServerConfig{
		l:     listener,
		fs:    fs,
		flags: parsedFlags,
	}

	ctx := context.Background()

	server := NewServer(config)
	err := server.Start(ctx, "4221")
	if err != nil {
		fmt.Println("Failed to start server:", err)
		os.Exit(1)
	}
}

type ServerConfig struct {
	l     NetworkListener
	fs    afero.Fs
	flags map[string]string
}

type RealListener struct{}

func (RealListener) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

type NetworkListener interface {
	Listen(network, address string) (net.Listener, error)
}

type Server struct {
	config ServerConfig
}

func NewServer(config ServerConfig) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Start(ctx context.Context, port string) error {
	fmt.Println("Starting server on port:", port)

	l, err := s.config.l.Listen("tcp", "0.0.0.0:"+port)
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
			go s.handleConnection(conn)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
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
	s.handleHttpRequest(conn, dataBuffer)
}

func (s *Server) handleHttpRequest(conn net.Conn, data []byte) {
	dataString := string(data)
	lines := strings.Split(dataString, "\r\n")
	method := strings.Split(lines[0], " ")[0]

	if len(method) < 2 {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_BAD_REQUEST + "\r\n\r\n"))
		return
	}

	s.handleMethod(conn, method, lines)
}

func (s *Server) handleMethod(conn net.Conn, method string, lines []string) {
	switch method {
	case "GET":
		s.handleGetRequest(conn, lines)
	default:
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
	}
}

func (s *Server) handleGetRequest(conn net.Conn, headerLines []string) {
	path := strings.Split(headerLines[0], " ")[1]

	if path == "/" {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_OK + "\r\n\r\n"))
	} else if strings.HasPrefix(path, "/echo/") {
		s.writeEcho(conn, path)
	} else if path == "/user-agent" {
		s.writeUserAgent(conn, headerLines)
	} else if strings.HasPrefix(path, "/files/") {
		s.getFile(conn, headerLines)
	} else {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
	}
}

func (s *Server) writeEcho(conn net.Conn, path string) {
	content := strings.TrimPrefix(path, "/echo/")
	response := fmt.Sprintf(
		PROTOCOL+HTTP_STATUS_OK+"\r\n"+CONTENT_TYPE+"\r\n"+CONTENT_LENGTH+"%d\r\n\r\n%s", len(content), content)
	conn.Write([]byte(response))
}

func (s *Server) writeUserAgent(conn net.Conn, headerLines []string) {
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
}

func (s *Server) getFile(conn net.Conn, headerLines []string) {
	filePath := strings.Split(headerLines[0], " ")[1]
	filePath = strings.TrimPrefix(filePath, "/files/")
	dir := s.config.flags["directory"]

	if dir == "" {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
		return
	}

	absolutePath := removeDuplicateSlash(dir + "/" + filePath)
	file, err := s.config.fs.Open(absolutePath)

	if err != nil {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
		return
	}

	data, err := afero.ReadFile(s.config.fs, file.Name())
	defer file.Close()

	if err != nil {
		conn.Write([]byte(PROTOCOL + HTTP_STATUS_NOT_FOUND + "\r\n\r\n"))
		return
	}

	response := fmt.Sprintf(PROTOCOL+HTTP_STATUS_OK+"\r\n"+"Content-Type: application/octet-stream"+"\r\n"+CONTENT_LENGTH+"%d\r\n\r\n%s", 17, string(data))
	conn.Write([]byte(response))
}

func removeDuplicateSlash(path string) string {
	if strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}

	return path
}

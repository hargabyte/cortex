// Package daemon socket.go provides Unix socket communication for the CX daemon.
// The socket allows cx commands to communicate with a running daemon for
// fast query execution against the warm in-memory graph.
package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RequestType identifies the type of request sent to the daemon.
type RequestType string

const (
	// RequestTypeHealth is a simple health check request.
	RequestTypeHealth RequestType = "health"

	// RequestTypeStatus requests detailed daemon status.
	RequestTypeStatus RequestType = "status"

	// RequestTypeQuery is a cx query request (find, show, impact, etc.).
	RequestTypeQuery RequestType = "query"

	// RequestTypeStop requests the daemon to shut down.
	RequestTypeStop RequestType = "stop"
)

// Request represents a request sent to the daemon.
type Request struct {
	// Type identifies the request type.
	Type RequestType `json:"type"`

	// Command is the cx command to execute (for query requests).
	Command string `json:"command,omitempty"`

	// Args are the command arguments.
	Args []string `json:"args,omitempty"`

	// Options are additional command options.
	Options map[string]interface{} `json:"options,omitempty"`
}

// Response represents a response from the daemon.
type Response struct {
	// Success indicates if the request was successful.
	Success bool `json:"success"`

	// Error contains the error message if Success is false.
	Error string `json:"error,omitempty"`

	// Data contains the response data.
	Data map[string]interface{} `json:"data,omitempty"`
}

// RequestHandler is a function that handles a request and returns a response.
type RequestHandler func(req *Request) *Response

// Socket manages the Unix socket server for daemon communication.
type Socket struct {
	path     string
	listener net.Listener
	handler  RequestHandler

	// Connection management
	conns    map[net.Conn]struct{}
	connsMu  sync.Mutex
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// NewSocket creates a new Socket with the given path and request handler.
func NewSocket(path string, handler RequestHandler) (*Socket, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	return &Socket{
		path:     path,
		handler:  handler,
		conns:    make(map[net.Conn]struct{}),
		shutdown: make(chan struct{}),
	}, nil
}

// Start starts the socket server and begins accepting connections.
func (s *Socket) Start() error {
	// Ensure the socket directory exists
	socketDir := filepath.Dir(s.path)
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	// Remove any existing socket file
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove existing socket: %w", err)
	}

	// Create the Unix socket listener
	listener, err := net.Listen("unix", s.path)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}
	s.listener = listener

	// Set socket permissions (allow user only)
	if err := os.Chmod(s.path, 0600); err != nil {
		s.listener.Close()
		return fmt.Errorf("set socket permissions: %w", err)
	}

	// Start accepting connections in background
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// Stop stops the socket server and closes all connections.
func (s *Socket) Stop() error {
	// Signal shutdown
	close(s.shutdown)

	// Close listener to stop accepting new connections
	if s.listener != nil {
		s.listener.Close()
	}

	// Close all active connections
	s.connsMu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.connsMu.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	// Remove socket file
	os.Remove(s.path)

	return nil
}

// acceptLoop accepts incoming connections.
func (s *Socket) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdown:
				return
			default:
				// Log error but continue accepting
				continue
			}
		}

		s.trackConn(conn, true)
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

// handleConn handles a single client connection.
func (s *Socket) handleConn(conn net.Conn) {
	defer func() {
		s.trackConn(conn, false)
		conn.Close()
		s.wg.Done()
	}()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read request
	decoder := json.NewDecoder(conn)
	var req Request
	if err := decoder.Decode(&req); err != nil {
		if err != io.EOF {
			s.sendError(conn, fmt.Sprintf("decode request: %v", err))
		}
		return
	}

	// Handle request
	resp := s.handler(&req)
	if resp == nil {
		resp = &Response{
			Success: false,
			Error:   "handler returned nil response",
		}
	}

	// Send response
	s.sendResponse(conn, resp)
}

// trackConn adds or removes a connection from tracking.
func (s *Socket) trackConn(conn net.Conn, add bool) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()

	if add {
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
}

// sendResponse sends a response to the client.
func (s *Socket) sendResponse(conn net.Conn, resp *Response) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	encoder := json.NewEncoder(conn)
	return encoder.Encode(resp)
}

// sendError sends an error response to the client.
func (s *Socket) sendError(conn net.Conn, errMsg string) error {
	return s.sendResponse(conn, &Response{
		Success: false,
		Error:   errMsg,
	})
}

// Path returns the socket path.
func (s *Socket) Path() string {
	return s.path
}

// Client provides a simple client for communicating with the daemon.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new daemon client.
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		socketPath = DefaultSocketPath()
	}
	return &Client{
		socketPath: socketPath,
		timeout:    5 * time.Second,
	}
}

// SetTimeout sets the connection timeout.
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// Send sends a request to the daemon and returns the response.
func (c *Client) Send(req *Request) (*Response, error) {
	// Connect to socket
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	// Set deadlines
	deadline := time.Now().Add(c.timeout)
	conn.SetDeadline(deadline)

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &resp, nil
}

// Health sends a health check request to the daemon.
func (c *Client) Health() (*Response, error) {
	return c.Send(&Request{Type: RequestTypeHealth})
}

// Status requests the daemon's current status.
func (c *Client) Status() (*Response, error) {
	return c.Send(&Request{Type: RequestTypeStatus})
}

// Query sends a query request to the daemon.
func (c *Client) Query(command string, args []string, options map[string]interface{}) (*Response, error) {
	return c.Send(&Request{
		Type:    RequestTypeQuery,
		Command: command,
		Args:    args,
		Options: options,
	})
}

// Stop requests the daemon to shut down.
func (c *Client) Stop() (*Response, error) {
	return c.Send(&Request{Type: RequestTypeStop})
}

// IsConnectable checks if the daemon socket is reachable.
func (c *Client) IsConnectable() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// WaitForDaemon waits for the daemon to become available.
// It polls the socket with exponential backoff up to maxWait.
func (c *Client) WaitForDaemon(maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	backoff := 50 * time.Millisecond

	for time.Now().Before(deadline) {
		if c.IsConnectable() {
			// Verify with health check
			resp, err := c.Health()
			if err == nil && resp.Success {
				return nil
			}
		}

		time.Sleep(backoff)
		backoff = backoff * 2
		if backoff > 1*time.Second {
			backoff = 1 * time.Second
		}
	}

	return fmt.Errorf("daemon did not become available within %v", maxWait)
}

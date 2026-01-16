package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSocketCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	handler := func(req *Request) *Response {
		return &Response{Success: true}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}

	if sock.Path() != socketPath {
		t.Errorf("expected path %s, got %s", socketPath, sock.Path())
	}
}

func TestSocketNilHandler(t *testing.T) {
	_, err := NewSocket("/tmp/test.sock", nil)
	if err == nil {
		t.Error("expected error with nil handler")
	}
}

func TestSocketStartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	handler := func(req *Request) *Response {
		return &Response{Success: true}
	}

	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}

	// Start socket
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}

	// Verify socket file exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Error("socket file should exist after start")
	}

	// Stop socket
	if err := sock.Stop(); err != nil {
		t.Fatalf("stop socket: %v", err)
	}

	// Verify socket file is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed after stop")
	}
}

func TestClientServerCommunication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create handler that echoes request type
	handler := func(req *Request) *Response {
		return &Response{
			Success: true,
			Data: map[string]interface{}{
				"request_type": string(req.Type),
			},
		}
	}

	// Start server
	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Create client
	client := NewClient(socketPath)
	client.SetTimeout(5 * time.Second)

	// Test health request
	resp, err := client.Health()
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	if !resp.Success {
		t.Error("health request should succeed")
	}
	if reqType, ok := resp.Data["request_type"].(string); !ok || reqType != string(RequestTypeHealth) {
		t.Errorf("expected request_type %s, got %v", RequestTypeHealth, resp.Data["request_type"])
	}

	// Test status request
	resp, err = client.Status()
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	if !resp.Success {
		t.Error("status request should succeed")
	}
	if reqType, ok := resp.Data["request_type"].(string); !ok || reqType != string(RequestTypeStatus) {
		t.Errorf("expected request_type %s, got %v", RequestTypeStatus, resp.Data["request_type"])
	}
}

func TestClientIsConnectable(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")
	client := NewClient(socketPath)

	// Should not be connectable without server
	if client.IsConnectable() {
		t.Error("should not be connectable without server")
	}

	// Start server
	handler := func(req *Request) *Response {
		return &Response{Success: true}
	}
	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Should be connectable with server
	if !client.IsConnectable() {
		t.Error("should be connectable with server running")
	}
}

func TestClientWaitForDaemon(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")
	client := NewClient(socketPath)

	// Should timeout without server
	err = client.WaitForDaemon(500 * time.Millisecond)
	if err == nil {
		t.Error("should timeout without server")
	}

	// Start server after a delay
	handler := func(req *Request) *Response {
		return &Response{
			Success: true,
			Data:    map[string]interface{}{"healthy": true},
		}
	}
	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		sock.Start()
	}()
	defer sock.Stop()

	// Should succeed when server starts
	err = client.WaitForDaemon(2 * time.Second)
	if err != nil {
		t.Errorf("should succeed when server starts: %v", err)
	}
}

func TestRequestTypes(t *testing.T) {
	// Verify all request types are distinct
	types := []RequestType{
		RequestTypeHealth,
		RequestTypeStatus,
		RequestTypeQuery,
		RequestTypeStop,
	}

	seen := make(map[RequestType]bool)
	for _, rt := range types {
		if seen[rt] {
			t.Errorf("duplicate request type: %s", rt)
		}
		seen[rt] = true

		if string(rt) == "" {
			t.Error("request type should not be empty")
		}
	}
}

func TestClientQuery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create handler that verifies query request
	handler := func(req *Request) *Response {
		if req.Type != RequestTypeQuery {
			return &Response{
				Success: false,
				Error:   "expected query request",
			}
		}
		return &Response{
			Success: true,
			Data: map[string]interface{}{
				"command": req.Command,
				"args":    req.Args,
				"options": req.Options,
			},
		}
	}

	// Start server
	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Create client and send query
	client := NewClient(socketPath)
	resp, err := client.Query("find", []string{"UserService"}, map[string]interface{}{
		"limit": 10,
	})
	if err != nil {
		t.Fatalf("query request: %v", err)
	}

	if !resp.Success {
		t.Errorf("query should succeed: %s", resp.Error)
	}

	if cmd, ok := resp.Data["command"].(string); !ok || cmd != "find" {
		t.Errorf("expected command 'find', got %v", resp.Data["command"])
	}
}

func TestClientStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "cx-socket-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create handler that acknowledges stop request
	handler := func(req *Request) *Response {
		return &Response{
			Success: true,
			Data: map[string]interface{}{
				"request_type": string(req.Type),
			},
		}
	}

	// Start server
	sock, err := NewSocket(socketPath, handler)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	if err := sock.Start(); err != nil {
		t.Fatalf("start socket: %v", err)
	}
	defer sock.Stop()

	// Send stop request
	client := NewClient(socketPath)
	resp, err := client.Stop()
	if err != nil {
		t.Fatalf("stop request: %v", err)
	}

	if !resp.Success {
		t.Error("stop request should succeed")
	}

	if reqType, ok := resp.Data["request_type"].(string); !ok || reqType != string(RequestTypeStop) {
		t.Errorf("expected request_type %s, got %v", RequestTypeStop, resp.Data["request_type"])
	}
}

package net

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
)

// Test server helper
type testServer struct {
	listener net.Listener
	addr     string
	wg       sync.WaitGroup
	stopChan chan struct{}
}

func newTestServer() (*testServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &testServer{
		listener: listener,
		addr:     listener.Addr().String(),
		stopChan: make(chan struct{}),
	}

	return server, nil
}

func (s *testServer) start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.stopChan:
				return
			default:
			}

			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stopChan:
					return
				default:
					continue
				}
			}

			go s.handleConnection(conn)
		}
	}()
}

func (s *testServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	// Echo server that handles length-prefixed messages
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		// Read 4-byte length header
		headerBuf := make([]byte, 4)
		_, err := io.ReadFull(conn, headerBuf)
		if err != nil {
			return
		}

		length := binary.BigEndian.Uint32(headerBuf)
		if length > 1024*1024 { // 1MB limit
			return
		}

		// Read message data
		messageBuf := make([]byte, length)
		_, err = io.ReadFull(conn, messageBuf)
		if err != nil {
			return
		}

		// Echo back the message (with length header)
		conn.Write(headerBuf)
		conn.Write(messageBuf)
	}
}

func (s *testServer) stop() {
	close(s.stopChan)
	s.listener.Close()
	s.wg.Wait()
}

func TestNewSocket(t *testing.T) {
	netModule := &Net{}

	socket := netModule.NewSocket()
	if socket == nil {
		t.Fatal("NewSocket returned nil")
	}

	// Check initial state
	if !socket.state.IsClosed() {
		t.Error("New socket should be in CLOSED state")
	}
	if socket.readers == nil {
		t.Error("readers channel should be initialized")
	}
	if socket.writers == nil {
		t.Error("writers channel should be initialized")
	}
}

func TestSocketConnect(t *testing.T) {
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	// Parse host and port from server address
	host := "127.0.0.1"
	port := server.listener.Addr().(*net.TCPAddr).Port

	// Test successful connection
	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !socket.state.IsOpen() {
		t.Error("Socket should be in OPEN state")
	}

	socket.Close()

	// Test connection to non-existent server
	err = socket.Connect("127.0.0.1", 1, SocketConfig{
		Timeout: 100,
	})
	if err == nil {
		t.Error("Connect should have failed")
	}
}

func TestSocketEventHandlers(t *testing.T) {
	t.Skip("Event handlers require full VU context with EventManager - tested in integration tests")
	
	// This test requires a proper k6 VU with runtime and event loop
	// Event handlers are now managed by EventManager which requires:
	// 1. A valid VU with RegisterCallback
	// 2. A sobek.Runtime for converting values
	// 3. An event loop to queue callbacks
	// These are tested in the integration test suite with k6 run
}

func TestSocketSendReceive(t *testing.T) {
	t.Skip("Send/Receive requires full VU context with EventManager - tested in integration tests")
	// This test requires event handlers which need proper k6 VU context
}

func TestSocketWrite(t *testing.T) {
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	// Parse host and port from server address
	host := "127.0.0.1"
	port := server.listener.Addr().(*net.TCPAddr).Port

	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer socket.Close()

	// Test raw write (should not add length header)
	testData := []byte{0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'}
	err = socket.Write(testData)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
}

func TestSocketNotConnected(t *testing.T) {
	netModule := &Net{}
	socket := netModule.NewSocket()

	// Test send when not connected
	err := socket.Send([]byte("test"))
	if err == nil {
		t.Error("Send should fail when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got: %v", err)
	}

	// Test write when not connected
	err = socket.Write([]byte("test"))
	if err == nil {
		t.Error("Write should fail when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got: %v", err)
	}
}

func TestSocketClose(t *testing.T) {
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	// Parse host and port from server address
	host := "127.0.0.1"
	port := server.listener.Addr().(*net.TCPAddr).Port

	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !socket.state.IsOpen() {
		t.Error("Socket should be connected")
	}

	err = socket.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if !socket.state.IsClosed() {
		t.Error("Socket should be in CLOSED state after close")
	}

	// Test double close (should not error)
	err = socket.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

func TestDecodeMessage(t *testing.T) {
	netModule := &Net{}

	tests := []struct {
		name     string
		encoding string
		input    []byte
		expected interface{}
	}{
		{
			name:     "binary encoding",
			encoding: "binary",
			input:    []byte{0x01, 0x02, 0x03},
			expected: []byte{0x01, 0x02, 0x03},
		},
		{
			name:     "utf-8 encoding",
			encoding: "utf-8",
			input:    []byte("Hello, World!"),
			expected: "Hello, World!",
		},
		{
			name:     "utf-8 with null bytes",
			encoding: "utf-8",
			input:    []byte("Hello\x00\x00\x00"),
			expected: "Hello",
		},
		{
			name:     "default encoding (binary)",
			encoding: "",
			input:    []byte{0x01, 0x02, 0x03},
			expected: []byte{0x01, 0x02, 0x03},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket := netModule.NewSocket()
			socket.config.Encoding = tt.encoding
			if tt.encoding == "" {
				socket.config.Encoding = "binary" // Set default
			}

			result := socket.decodeMessage(tt.input)

			switch expected := tt.expected.(type) {
			case []byte:
				if resultBytes, ok := result.([]byte); ok {
					if len(resultBytes) != len(expected) {
						t.Errorf("Length mismatch: got %d, want %d", len(resultBytes), len(expected))
						return
					}
					for i, b := range expected {
						if resultBytes[i] != b {
							t.Errorf("Byte %d: got %02x, want %02x", i, resultBytes[i], b)
						}
					}
				} else {
					t.Errorf("Type mismatch: got %T, want []byte", result)
				}
			case string:
				if resultString, ok := result.(string); ok {
					if resultString != expected {
						t.Errorf("String mismatch: got %s, want %s", resultString, expected)
					}
				} else {
					t.Errorf("Type mismatch: got %T, want string", result)
				}
			}
		})
	}
}

func TestLengthFieldValidation(t *testing.T) {
	// Create test server for connecting sockets
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()
	server.start()

	netModule := &Net{}

	tests := []struct {
		name              string
		lengthFieldLength int
		shouldError       bool
	}{
		{"1 byte length", 1, false},
		{"2 byte length", 2, false},
		{"4 byte length", 4, false},
		{"8 byte length", 8, false},
		{"invalid 3 byte length", 3, true},
		{"invalid 5 byte length", 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket := netModule.NewSocket()

			// Parse host and port from server address
			host := "127.0.0.1"
			port := server.listener.Addr().(*net.TCPAddr).Port

			// Connect the socket so we can test Send validation
			err := socket.Connect(host, port, SocketConfig{
				Timeout:           1000,
				LengthFieldLength: tt.lengthFieldLength,
			})
			if err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			defer socket.Close()

			testData := []byte("test")
			err = socket.Send(testData)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error for invalid length field length")
				}
				if !strings.Contains(err.Error(), "unsupported length field length") {
					t.Errorf("Expected 'unsupported length field length' error, got: %v", err)
				}
			} else {
				// For valid lengths, Send should succeed 
				if err != nil {
					t.Errorf("Send failed with valid length field: %v", err)
				}
			}
		})
	}
}

func BenchmarkSocketSend(b *testing.B) {
	server, err := newTestServer()
	if err != nil {
		b.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	// Parse host and port from server address
	host := "127.0.0.1"
	port := server.listener.Addr().(*net.TCPAddr).Port

	err = socket.Connect(host, port, SocketConfig{
		Timeout:           5000,
		LengthFieldLength: 4,
		MaxLength:         1024,
	})
	if err != nil {
		b.Fatalf("Connect failed: %v", err)
	}
	defer socket.Close()

	testData := []byte("benchmark test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := socket.Send(testData)
		if err != nil {
			b.Fatalf("Send failed: %v", err)
		}
	}
}

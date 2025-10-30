package net

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
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

	tests := []struct {
		name     string
		config   SocketConfig
		expected SocketConfig
	}{
		{
			name:   "default config",
			config: SocketConfig{},
			expected: SocketConfig{
				LengthFieldLength: 0,
				MaxLength:         0,
				Encoding:          "binary",
				Delimiter:         "",
			},
		},
		{
			name: "custom config",
			config: SocketConfig{
				LengthFieldLength: 4,
				MaxLength:         2048,
				Encoding:          "utf-8",
				Delimiter:         "\r\n",
			},
			expected: SocketConfig{
				LengthFieldLength: 4,
				MaxLength:         2048,
				Encoding:          "utf-8",
				Delimiter:         "\r\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socket := netModule.NewSocket(tt.config)
			if socket == nil {
				t.Fatal("NewSocket returned nil")
			}

			if socket.config.LengthFieldLength != tt.expected.LengthFieldLength {
				t.Errorf("LengthFieldLength = %d, want %d", socket.config.LengthFieldLength, tt.expected.LengthFieldLength)
			}
			if socket.config.MaxLength != tt.expected.MaxLength {
				t.Errorf("MaxLength = %d, want %d", socket.config.MaxLength, tt.expected.MaxLength)
			}
			if socket.config.Encoding != tt.expected.Encoding {
				t.Errorf("Encoding = %s, want %s", socket.config.Encoding, tt.expected.Encoding)
			}
			if socket.config.Delimiter != tt.expected.Delimiter {
				t.Errorf("Delimiter = %s, want %s", socket.config.Delimiter, tt.expected.Delimiter)
			}
		})
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
	socket := netModule.NewSocket(SocketConfig{})

	// Test successful connection
	err = socket.Connect(server.addr, 5000)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !socket.connected {
		t.Error("Socket should be connected")
	}

	socket.Close()

	// Test connection to non-existent server
	err = socket.Connect("127.0.0.1:0", 100)
	if err == nil {
		t.Error("Connect should have failed")
	}
}

func TestSocketEventHandlers(t *testing.T) {
	netModule := &Net{}
	socket := netModule.NewSocket(SocketConfig{})

	var dataReceived []byte
	var messageReceived interface{}
	var errorReceived error
	var endCalled bool

	socket.On("data", func(data []byte) {
		dataReceived = data
	})

	socket.On("message", func(message interface{}) {
		messageReceived = message
	})

	socket.On("error", func(err error) {
		errorReceived = err
	})

	socket.On("end", func() {
		endCalled = true
	})

	// Test that handlers are set
	if socket.onData == nil {
		t.Error("onData handler not set")
	}
	if socket.onMessage == nil {
		t.Error("onMessage handler not set")
	}
	if socket.onError == nil {
		t.Error("onError handler not set")
	}
	if socket.onEnd == nil {
		t.Error("onEnd handler not set")
	}

	// Test calling handlers
	testData := []byte("test data")
	socket.onData(testData)
	if string(dataReceived) != "test data" {
		t.Errorf("Data handler received %s, want 'test data'", string(dataReceived))
	}

	testMessage := "test message"
	socket.onMessage(testMessage)
	if messageReceived != testMessage {
		t.Errorf("Message handler received %v, want %s", messageReceived, testMessage)
	}

	testError := fmt.Errorf("test error")
	socket.onError(testError)
	if errorReceived.Error() != "test error" {
		t.Errorf("Error handler received %v, want 'test error'", errorReceived)
	}

	socket.onEnd()
	if !endCalled {
		t.Error("End handler not called")
	}
}

func TestSocketSendReceive(t *testing.T) {
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket(SocketConfig{
		LengthFieldLength: 4,
		MaxLength:         1024,
		Encoding:          "binary",
	})

	var receivedData []byte
	var receivedMessage interface{}
	dataReceived := make(chan bool, 1)
	messageReceived := make(chan bool, 1)

	socket.On("data", func(data []byte) {
		receivedData = make([]byte, len(data))
		copy(receivedData, data)
		select {
		case dataReceived <- true:
		default:
		}
	})

	socket.On("message", func(message interface{}) {
		receivedMessage = message
		select {
		case messageReceived <- true:
		default:
		}
	})

	err = socket.Connect(server.addr, 5000)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer socket.Close()

	// Give the read loop time to start
	time.Sleep(100 * time.Millisecond)

	// Send test message
	testMessage := []byte("Hello, World!")
	err = socket.Send(testMessage)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait for response
	select {
	case <-dataReceived:
		// Check that we received the full message (header + payload)
		if len(receivedData) != 4+len(testMessage) {
			t.Errorf("Received data length %d, expected %d", len(receivedData), 4+len(testMessage))
		}

		// Check length header
		expectedLength := uint32(len(testMessage))
		actualLength := binary.BigEndian.Uint32(receivedData[:4])
		if actualLength != expectedLength {
			t.Errorf("Length header %d, expected %d", actualLength, expectedLength)
		}

		// Check payload
		payload := receivedData[4:]
		if string(payload) != string(testMessage) {
			t.Errorf("Payload %s, expected %s", string(payload), string(testMessage))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for data")
	}

	select {
	case <-messageReceived:
		// Check decoded message
		if messageBytes, ok := receivedMessage.([]byte); ok {
			if string(messageBytes) != string(testMessage) {
				t.Errorf("Message %s, expected %s", string(messageBytes), string(testMessage))
			}
		} else {
			t.Errorf("Message type %T, expected []byte", receivedMessage)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

func TestSocketWrite(t *testing.T) {
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()

	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket(SocketConfig{})

	err = socket.Connect(server.addr, 5000)
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
	socket := netModule.NewSocket(SocketConfig{})

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
	socket := netModule.NewSocket(SocketConfig{})

	err = socket.Connect(server.addr, 5000)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !socket.connected {
		t.Error("Socket should be connected")
	}

	err = socket.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if socket.connected {
		t.Error("Socket should not be connected after close")
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
			socket := netModule.NewSocket(SocketConfig{
				Encoding: tt.encoding,
			})

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
			socket := netModule.NewSocket(SocketConfig{
				LengthFieldLength: tt.lengthFieldLength,
			})

			// Connect the socket so we can test Send validation
			err := socket.Connect(server.addr, 1000)
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
	socket := netModule.NewSocket(SocketConfig{
		LengthFieldLength: 4,
		MaxLength:         1024,
	})

	err = socket.Connect(server.addr, 5000)
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

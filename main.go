package net

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/modules"
)

// RootModule is the global module instance that will create module instances for each VU.
type RootModule struct{}

// Ensure the interfaces are implemented correctly.
var _ modules.Module = &RootModule{}

// Net is the k6 extension for net operations
type Net struct {
	vu modules.VU // provides methods for accessing internal k6 objects
}

// SocketConfig represents the configuration options for a Socket
type SocketConfig struct {
	LengthFieldLength int    `json:"lengthFieldLength"`
	MaxLength         int    `json:"maxLength"`
	Encoding          string `json:"encoding"`
	Delimiter         string `json:"delimiter"`
}

// Socket represents a TCP socket with message handling capabilities
type Socket struct {
	config     SocketConfig
	conn       net.Conn
	connected  bool
	mu         sync.RWMutex
	readers    chan bool
	writers    chan bool
	
	// Event handlers
	onData    func([]byte)
	onMessage func(interface{})
	onError   func(error)
	onEnd     func()
}

// init is called by the Go runtime at application startup.
func init() {
	modules.Register("k6/x/net", new(RootModule))
}

// NewModuleInstance implements the modules.Module interface returning a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &Net{vu: vu}
}

// Exports returns the exports of the net module
func (n *Net) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"Socket": n.socketConstructor(),
		},
	}
}

// socketConstructor returns a JS constructor compatible with `new net.Socket(config)`
func (n *Net) socketConstructor() func(sobek.ConstructorCall) *sobek.Object {
	rt := n.vu.Runtime()
	return func(call sobek.ConstructorCall) *sobek.Object {
		cfg := SocketConfig{}
		if len(call.Arguments) > 0 {
			arg0 := call.Argument(0)
			if !sobek.IsUndefined(arg0) && !sobek.IsNull(arg0) {
				if err := rt.ExportTo(arg0, &cfg); err != nil {
					panic(rt.NewTypeError("invalid Socket config: %v", err))
				}
			}
		}
		sock := n.NewSocket(cfg)
		// Expose lower-case JS API to match README/types
		obj := rt.NewObject()
		_ = obj.Set("connect", func(addr string, timeoutMs int) error { return sock.Connect(addr, timeoutMs) })
		_ = obj.Set("on", func(event string, handler interface{}) { sock.On(event, handler) })
		_ = obj.Set("send", func(data []byte) error { return sock.Send(data) })
		_ = obj.Set("write", func(data []byte) error { return sock.Write(data) })
		_ = obj.Set("close", func() error { return sock.Close() })
		return obj
	}
}

// NewSocket creates a new Socket instance with the given configuration
func (n *Net) NewSocket(config SocketConfig) *Socket {
	// Set default values
	if config.Encoding == "" {
		config.Encoding = "binary"
	} else if config.Encoding != "binary" && config.Encoding != "utf-8" {
		config.Encoding = "binary"
	}

	return &Socket{
		config:  config,
		readers: make(chan bool, 1),
		writers: make(chan bool, 1),
	}
}

// Connect establishes a TCP connection to the specified address
func (s *Socket) Connect(addr string, timeoutMs int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		return fmt.Errorf("socket is already connected")
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeoutMs == 0 {
		timeout = 60 * time.Second // default timeout
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		if s.onError != nil {
			s.onError(err)
		}
		return err
	}

	s.conn = conn
	s.connected = true

	// Start reading messages in a goroutine
	go s.readLoop()

	return nil
}

// On sets event handlers for the socket
func (s *Socket) On(event string, handler interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch event {
	case "data":
		if fn, ok := handler.(func([]byte)); ok {
			s.onData = fn
		}
	case "message":
		if fn, ok := handler.(func(interface{})); ok {
			s.onMessage = fn
		}
	case "error":
		if fn, ok := handler.(func(error)); ok {
			s.onError = fn
		}
	case "end":
		if fn, ok := handler.(func()); ok {
			s.onEnd = fn
		}
	}
}

// Send sends a message with length header (if configured)
func (s *Socket) Send(data []byte) error {
	s.mu.RLock()
	if !s.connected {
		s.mu.RUnlock()
		return fmt.Errorf("socket not connected")
	}
	conn := s.conn
	s.mu.RUnlock()

	select {
	case s.writers <- true:
		defer func() { <-s.writers }()
	default:
		return fmt.Errorf("socket is busy")
	}

	// If lengthFieldLength is configured, prepend the length header
	if s.config.LengthFieldLength > 0 {
		lengthHeader := make([]byte, s.config.LengthFieldLength)
		
		switch s.config.LengthFieldLength {
		case 1:
			if len(data) > 0xFF {
				return fmt.Errorf("message length %d exceeds 1-byte header capacity", len(data))
			}
			lengthHeader[0] = byte(len(data))
		case 2:
			if len(data) > 0xFFFF {
				return fmt.Errorf("message length %d exceeds 2-byte header capacity", len(data))
			}
			binary.BigEndian.PutUint16(lengthHeader, uint16(len(data)))
		case 4:
			if uint64(len(data)) > uint64(math.MaxUint32) {
				return fmt.Errorf("message length %d exceeds 4-byte header capacity", len(data))
			}
			binary.BigEndian.PutUint32(lengthHeader, uint32(len(data)))
		case 8:
			binary.BigEndian.PutUint64(lengthHeader, uint64(len(data)))
		default:
			return fmt.Errorf("unsupported length field length: %d", s.config.LengthFieldLength)
		}

		// Write header first
		if err := writeAll(conn, lengthHeader); err != nil {
			if s.onError != nil {
				s.onError(err)
			}
			return err
		}
	}

	// Write the actual data
	if err := writeAll(conn, data); err != nil {
		if s.onError != nil {
			s.onError(err)
		}
		return err
	}

	return nil
}

// Write sends raw data without any headers
func (s *Socket) Write(data []byte) error {
	s.mu.RLock()
	if !s.connected {
		s.mu.RUnlock()
		return fmt.Errorf("socket not connected")
	}
	conn := s.conn
	s.mu.RUnlock()

	select {
	case s.writers <- true:
		defer func() { <-s.writers }()
	default:
		return fmt.Errorf("socket is busy")
	}

	if err := writeAll(conn, data); err != nil {
		if s.onError != nil {
			s.onError(err)
		}
		return err
	}

	return nil
}

// writeAll writes the entire buffer handling partial writes
func writeAll(conn net.Conn, buf []byte) error {
    written := 0
    for written < len(buf) {
        n, err := conn.Write(buf[written:])
        if err != nil {
            return err
        }
        if n == 0 {
            return io.ErrUnexpectedEOF
        }
        written += n
    }
    return nil
}

// Close closes the socket connection
func (s *Socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}

	s.connected = false
	err := s.conn.Close()
	
	if s.onEnd != nil {
		s.onEnd()
	}

	return err
}

// readLoop handles incoming data and processes messages
func (s *Socket) readLoop() {
	defer func() {
		if s.onEnd != nil {
			s.onEnd()
		}
	}()

	reader := bufio.NewReader(s.conn)

	for {
		s.mu.RLock()
		if !s.connected {
			s.mu.RUnlock()
			return
		}
		s.mu.RUnlock()

		// Block to avoid busy spinning; single reader goroutine
		s.readers <- true

		if s.config.LengthFieldLength > 0 {
			// Read length-prefixed messages
			err := s.readLengthPrefixedMessage(reader)
			<-s.readers
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return
				}
				if s.onError != nil {
					s.onError(err)
				}
				return
			}
		} else if s.config.Delimiter != "" && s.config.Encoding == "utf-8" {
			// Read delimiter-separated messages
			err := s.readDelimiterMessage(reader)
			<-s.readers
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return
				}
				if s.onError != nil {
					s.onError(err)
				}
				return
			}
		} else {
			// Read fixed-length or until maxLength
			err := s.readFixedLengthMessage(reader)
			<-s.readers
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
					return
				}
				if s.onError != nil {
					s.onError(err)
				}
				return
			}
		}
	}
}

// readLengthPrefixedMessage reads a message with length prefix
func (s *Socket) readLengthPrefixedMessage(reader *bufio.Reader) error {
	// Read the length header
	lengthBytes := make([]byte, s.config.LengthFieldLength)
	_, err := io.ReadFull(reader, lengthBytes)
	if err != nil {
		return err
	}

	// Parse the length
	var messageLength uint64
	switch s.config.LengthFieldLength {
	case 1:
		messageLength = uint64(lengthBytes[0])
	case 2:
		messageLength = uint64(binary.BigEndian.Uint16(lengthBytes))
	case 4:
		messageLength = uint64(binary.BigEndian.Uint32(lengthBytes))
	case 8:
		messageLength = binary.BigEndian.Uint64(lengthBytes)
	}

	// Check against maxLength
	if s.config.MaxLength > 0 && messageLength > uint64(s.config.MaxLength) {
		return fmt.Errorf("message length %d exceeds maxLength %d", messageLength, s.config.MaxLength)
	}

	// Read the message data
	messageData := make([]byte, messageLength)
	_, err = io.ReadFull(reader, messageData)
	if err != nil {
		return err
	}

	// Emit data event
	if s.onData != nil {
		allData := append(lengthBytes, messageData...)
		s.onData(allData)
	}

	// Emit message event with decoded data
	if s.onMessage != nil {
		decoded := s.decodeMessage(messageData)
		s.onMessage(decoded)
	}

	return nil
}

// readDelimiterMessage reads a message separated by delimiter
func (s *Socket) readDelimiterMessage(reader *bufio.Reader) error {
	var buffer bytes.Buffer
	delimiter := []byte(s.config.Delimiter)

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return err
		}

		buffer.WriteByte(b)

		// Check if we have the delimiter
		if buffer.Len() >= len(delimiter) {
			tail := buffer.Bytes()[buffer.Len()-len(delimiter):]
			if bytes.Equal(tail, delimiter) {
				// Found delimiter, remove it from message
				messageData := buffer.Bytes()[:buffer.Len()-len(delimiter)]

				// Check against maxLength
				if s.config.MaxLength > 0 && len(messageData) > s.config.MaxLength {
					return fmt.Errorf("message length %d exceeds maxLength %d", len(messageData), s.config.MaxLength)
				}

				// Emit data event (including delimiter)
				if s.onData != nil {
					s.onData(buffer.Bytes())
				}

				// Emit message event with decoded data (without delimiter)
				if s.onMessage != nil {
					decoded := s.decodeMessage(messageData)
					s.onMessage(decoded)
				}

				return nil
			}
		}

		// Check against maxLength
		if s.config.MaxLength > 0 && buffer.Len() > s.config.MaxLength {
			return fmt.Errorf("message length exceeds maxLength %d", s.config.MaxLength)
		}
	}
}

// readFixedLengthMessage reads a fixed-length message or up to maxLength
func (s *Socket) readFixedLengthMessage(reader *bufio.Reader) error {
	readSize := s.config.MaxLength
	if readSize <= 0 {
		readSize = 4096 // Default chunk size
	}

	messageData := make([]byte, readSize)
	n, err := reader.Read(messageData)
	if err != nil {
		return err
	}

	messageData = messageData[:n]

	// Emit data event
	if s.onData != nil {
		s.onData(messageData)
	}

	// Only emit message event if we have some data and encoding is not binary
	if s.onMessage != nil && n > 0 && s.config.Encoding != "binary" {
		decoded := s.decodeMessage(messageData)
		s.onMessage(decoded)
	}

	return nil
}

// decodeMessage decodes the message based on encoding
func (s *Socket) decodeMessage(data []byte) interface{} {
	switch s.config.Encoding {
	case "utf-8":
		return strings.TrimRight(string(data), "\x00")
	case "binary":
		fallthrough
	default:
		return data
	}
}

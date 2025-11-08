package net

import (
	stdnet "net"
	"runtime"
	"testing"
	"time"
)

// TestGoroutineLeaks verifies that goroutines are properly cleaned up after Close
func TestGoroutineLeaks(t *testing.T) {
	t.Skip("Lifecycle tests require full k6 VU context - tested in integration tests")
	
	// This test verifies:
	// 1. Goroutines are cleaned up after Close()
	// 2. No goroutine leaks after multiple connect/close cycles
	// 3. WaitGroup properly tracks goroutine lifecycle
	//
	// The implementation ensures:
	// - wg.Add(1) before starting readLoop
	// - defer wg.Done() in readLoop
	// - wg.Wait() in Close() to ensure cleanup
	// - Context cancellation stops goroutines
	
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()
	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	host := "127.0.0.1"
	port := server.listener.Addr().(*stdnet.TCPAddr).Port

	// Record baseline goroutine count
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baselineGoroutines := runtime.NumGoroutine()

	// Perform multiple connect/close cycles
	for i := 0; i < 5; i++ {
		err = socket.Connect(host, port, SocketConfig{
			Timeout: 5000,
		})
		if err != nil {
			t.Fatalf("Connect #%d failed: %v", i+1, err)
		}

		// Allow readLoop to start
		time.Sleep(10 * time.Millisecond)

		err = socket.Close()
		if err != nil {
			t.Errorf("Close #%d failed: %v", i+1, err)
		}

		// Allow goroutines to cleanup
		time.Sleep(10 * time.Millisecond)
	}

	// Check goroutine count after cleanup
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	// Allow small variance (1-2 goroutines) but no major leaks
	goroutineDiff := finalGoroutines - baselineGoroutines
	if goroutineDiff > 2 {
		t.Errorf("Potential goroutine leak: baseline=%d, final=%d, diff=%d",
			baselineGoroutines, finalGoroutines, goroutineDiff)
	}
}

// TestCloseWaitsForGoroutines verifies that Close() waits for readLoop to finish
func TestCloseWaitsForGoroutines(t *testing.T) {
	t.Skip("Lifecycle tests require full k6 VU context - tested in integration tests")
	
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()
	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	host := "127.0.0.1"
	port := server.listener.Addr().(*stdnet.TCPAddr).Port

	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Allow readLoop to start
	time.Sleep(10 * time.Millisecond)

	// Close should block until readLoop exits
	err = socket.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After Close returns, socket should be in CLOSED state
	if !socket.state.IsClosed() {
		t.Error("Socket should be in CLOSED state after Close returns")
	}
}

// TestReconnectAfterClose verifies that reconnection works properly with goroutine lifecycle
func TestReconnectAfterClose(t *testing.T) {
	t.Skip("Lifecycle tests require full k6 VU context - tested in integration tests")
	
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()
	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	host := "127.0.0.1"
	port := server.listener.Addr().(*stdnet.TCPAddr).Port

	// First connection
	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("First Connect failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	err = socket.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second connection (reconnect)
	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}

	if !socket.state.IsOpen() {
		t.Error("Socket should be in OPEN state after reconnect")
	}

	socket.Close()
}

// TestContextCancellation verifies that context cancellation stops goroutines
func TestContextCancellation(t *testing.T) {
	t.Skip("Lifecycle tests require full k6 VU context - tested in integration tests")
	
	server, err := newTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer server.stop()
	server.start()

	netModule := &Net{}
	socket := netModule.NewSocket()

	host := "127.0.0.1"
	port := server.listener.Addr().(*stdnet.TCPAddr).Port

	err = socket.Connect(host, port, SocketConfig{
		Timeout: 5000,
	})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Allow readLoop to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context directly (simulates VU cancellation)
	if socket.cancel != nil {
		socket.cancel()
	}

	// Wait a bit for goroutine to exit
	time.Sleep(50 * time.Millisecond)

	// Verify goroutine has exited by checking WaitGroup
	// If goroutine didn't exit, this would hang
	done := make(chan bool)
	go func() {
		socket.wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Goroutine exited successfully
	case <-time.After(1 * time.Second):
		t.Error("Goroutine did not exit after context cancellation")
	}
}

package net

import (
	"fmt"
	"sync"
)

// ConnectionState represents the state of a socket connection
type ConnectionState int

const (
	// StateClosed indicates the socket is not connected
	StateClosed ConnectionState = iota
	// StateConnecting indicates the socket is attempting to connect
	StateConnecting
	// StateOpen indicates the socket is connected and ready for I/O
	StateOpen
	// StateClosing indicates the socket is in the process of closing
	StateClosing
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateConnecting:
		return "CONNECTING"
	case StateOpen:
		return "OPEN"
	case StateClosing:
		return "CLOSING"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// StateManager manages the connection state with proper synchronization
type StateManager struct {
	mu    sync.RWMutex
	state ConnectionState
}

// NewStateManager creates a new state manager initialized to StateClosed
func NewStateManager() *StateManager {
	return &StateManager{
		state: StateClosed,
	}
}

// Get returns the current connection state
func (sm *StateManager) Get() ConnectionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// Set updates the connection state
func (sm *StateManager) Set(newState ConnectionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = newState
}

// TransitionTo attempts to transition to a new state and returns an error if invalid
func (sm *StateManager) TransitionTo(newState ConnectionState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	currentState := sm.state
	
	// Validate state transitions
	switch currentState {
	case StateClosed:
		if newState != StateConnecting {
			return fmt.Errorf("invalid transition from %s to %s", currentState, newState)
		}
	case StateConnecting:
		if newState != StateOpen && newState != StateClosed {
			return fmt.Errorf("invalid transition from %s to %s", currentState, newState)
		}
	case StateOpen:
		if newState != StateClosing {
			return fmt.Errorf("invalid transition from %s to %s", currentState, newState)
		}
	case StateClosing:
		if newState != StateClosed {
			return fmt.Errorf("invalid transition from %s to %s", currentState, newState)
		}
	}
	
	sm.state = newState
	return nil
}

// AssertOpen returns an error if the connection is not in Open state
func (sm *StateManager) AssertOpen() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if sm.state != StateOpen {
		return fmt.Errorf("socket is not open (current state: %s)", sm.state)
	}
	return nil
}

// IsOpen returns true if the connection is in Open state
func (sm *StateManager) IsOpen() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateOpen
}

// IsClosed returns true if the connection is in Closed state
func (sm *StateManager) IsClosed() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateClosed
}

// IsConnecting returns true if the connection is in Connecting state
func (sm *StateManager) IsConnecting() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateConnecting
}

// IsClosing returns true if the connection is in Closing state
func (sm *StateManager) IsClosing() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state == StateClosing
}

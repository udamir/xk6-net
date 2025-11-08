package net

import (
	"fmt"

	"github.com/grafana/sobek"
	"github.com/mstoykov/k6-taskqueue-lib/taskqueue"
	"go.k6.io/k6/js/modules"
)

// EventManager handles event callbacks and ensures they run on the event loop
type EventManager struct {
	tq        *taskqueue.TaskQueue
	vu        modules.VU
	onData    sobek.Callable
	onMessage sobek.Callable
	onError   sobek.Callable
	onEnd     sobek.Callable
}

// NewEventManager creates a new event manager
func NewEventManager(vu modules.VU) *EventManager {
	return &EventManager{
		tq: taskqueue.New(vu.RegisterCallback),
		vu: vu,
	}
}

// CopyHandlersFrom copies event handlers from another EventManager
func (em *EventManager) CopyHandlersFrom(other *EventManager) {
	if other == nil {
		return
	}
	em.onData = other.onData
	em.onMessage = other.onMessage
	em.onError = other.onError
	em.onEnd = other.onEnd
}

// SetHandler sets an event handler
func (em *EventManager) SetHandler(event string, handler sobek.Value) {
	if sobek.IsUndefined(handler) || sobek.IsNull(handler) {
		// Clear handler
		switch event {
		case "data":
			em.onData = nil
		case "message":
			em.onMessage = nil
		case "error":
			em.onError = nil
		case "end":
			em.onEnd = nil
		}
		return
	}

	callable, ok := sobek.AssertFunction(handler)
	if !ok {
		return
	}

	switch event {
	case "data":
		em.onData = callable
	case "message":
		em.onMessage = callable
	case "error":
		em.onError = callable
	case "end":
		em.onEnd = callable
	}
}

// QueueCallback queues a custom callback to run on the event loop
func (em *EventManager) QueueCallback(fn func() error) {
	em.tq.Queue(fn)
}

// CallOnData queues the onData handler to be called on the event loop
func (em *EventManager) CallOnData(data []byte) {
	if em.onData == nil {
		return
	}

	em.tq.Queue(func() error {
		// Check if context is still valid before calling into JS
		ctx := em.vu.Context()
		if ctx.Err() != nil {
			// Context canceled, don't call handler
			return nil
		}

		rt := em.vu.Runtime()
		_, err := em.onData(sobek.Undefined(), rt.ToValue(data))
		if err != nil && ctx.Err() == nil {
			// Only log if context is still valid (not a cancellation error)
			fmt.Printf("[xk6-net] Error calling onData handler: %v\n", err)
		}
		return err
	})
}

// CallOnMessage queues the onMessage handler to be called on the event loop
func (em *EventManager) CallOnMessage(message interface{}) {
	if em.onMessage == nil {
		return
	}

	em.tq.Queue(func() error {
		// Check if context is still valid before calling into JS
		ctx := em.vu.Context()
		if ctx.Err() != nil {
			// Context canceled, don't call handler
			return nil
		}

		rt := em.vu.Runtime()
		_, err := em.onMessage(sobek.Undefined(), rt.ToValue(message))
		if err != nil && ctx.Err() == nil {
			// Only log if context is still valid (not a cancellation error)
			fmt.Printf("[xk6-net] Error calling onMessage handler: %v\n", err)
		}
		return err
	})
}

// CallOnError queues the onError handler to be called on the event loop
func (em *EventManager) CallOnError(err error) {
	if em.onError == nil {
		return
	}

	em.tq.Queue(func() error {
		// Check if context is still valid before calling into JS
		ctx := em.vu.Context()
		if ctx.Err() != nil {
			// Context canceled, don't call handler
			return nil
		}

		rt := em.vu.Runtime()
		_, callErr := em.onError(sobek.Undefined(), rt.ToValue(err.Error()))
		if callErr != nil && ctx.Err() == nil {
			// Only log if context is still valid (not a cancellation error)
			fmt.Printf("[xk6-net] Error calling onError handler: %v\n", callErr)
		}
		return callErr
	})
}

// CallOnEnd queues the onEnd handler to be called on the event loop
func (em *EventManager) CallOnEnd() {
	if em.onEnd == nil {
		return
	}

	em.tq.Queue(func() error {
		// Check if context is still valid before calling into JS
		ctx := em.vu.Context()
		if ctx.Err() != nil {
			// Context canceled, don't call handler
			return nil
		}

		_, err := em.onEnd(sobek.Undefined())
		if err != nil && ctx.Err() == nil {
			// Only log if context is still valid (not a cancellation error)
			fmt.Printf("[xk6-net] Error calling onEnd handler: %v\n", err)
		}
		return err
	})
}

// Close closes the task queue
func (em *EventManager) Close() {
	em.tq.Close()
}

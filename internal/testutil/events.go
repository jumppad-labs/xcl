package testutil

import (
	"sync"

	"github.com/jumppad-labs/xcl/events"
)

// EventRecorder records every event handed to it. Events arrive from
// concurrent walk goroutines and from plugin goroutines, so recording is
// guarded by a mutex. The zero value is ready to use
type EventRecorder struct {
	mu     sync.Mutex
	events []events.Event
}

// Record appends e to the recorded events, its signature matches an event
// handler so it can be passed as one
func (r *EventRecorder) Record(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// Events returns a copy of the events recorded so far, in the order they
// were received
func (r *EventRecorder) Events() []events.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]events.Event{}, r.events...)
}

// EventsWithPhase returns the events in recorded with the given phase
func EventsWithPhase(recorded []events.Event, phase string) []events.Event {
	found := []events.Event{}
	for _, e := range recorded {
		if e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

// ResourceOperations returns, for each resource a provider was called for, the
// operations of its create, update and destroy steps that succeeded, in the
// order they were reported. A provider call reports a start event, so a
// resource that never reported one, such as a variable, an output or a
// registered type, is left out. A replaced resource shows destroy then create.
func ResourceOperations(recorded []events.Event) map[string][]string {
	provided := map[string]bool{}
	for _, e := range recorded {
		if e.ResourceID != "" && e.Phase == events.PhaseStart {
			provided[e.ResourceID] = true
		}
	}

	operations := map[string][]string{}
	for _, e := range recorded {
		if !provided[e.ResourceID] || e.Phase != events.PhaseSuccess {
			continue
		}

		switch e.Operation {
		case events.OperationCreate, events.OperationUpdate, events.OperationDestroy:
			operations[e.ResourceID] = append(operations[e.ResourceID], e.Operation)
		}
	}

	return operations
}

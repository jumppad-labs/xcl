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

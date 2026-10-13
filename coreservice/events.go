package coreservice

import (
	"encoding/json"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/events"
)

// handleEvent is the event handler installed when the host asked for events:
// each event is posted on the queue as an Event task. Config.run delivers
// events one at a time and returns only once every event is delivered, so
// the operation's finished task always follows its last event, and a host
// slow to poll holds the emitters back exactly as a slow Go handler does.
func (c *configuration) handleEvent(e events.Event) {
	c.queue.post(&corepb.PollResponse{Task: &corepb.PollResponse_Event{Event: eventToProto(e)}})
}

// eventToProto is events.Event field for field. Meta crosses as JSON.
func eventToProto(e events.Event) *corepb.Event {
	out := &corepb.Event{
		TimeUnixNano: e.Time.UnixNano(),
		Source:       e.Source,
		Operation:    e.Operation,
		Phase:        e.Phase,
		ResourceType: e.ResourceType,
		ResourceId:   e.ResourceID,
		File:         e.File,
		DurationNano: e.Duration.Nanoseconds(),
		Error:        toProto(e.Error),
		Data:         e.Data,
	}

	if e.Time.IsZero() {
		out.TimeUnixNano = 0
	}

	if len(e.Meta) > 0 {
		if meta, err := json.Marshal(e.Meta); err == nil {
			out.MetaJson = meta
		}
	}

	return out
}

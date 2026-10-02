package xcl

import (
	"github.com/jumppad-labs/xcl/events"
)

// Event is one thing xcl or a plugin reports, such as a provider's Create
// starting or succeeding for a resource, a plugin loading, or a log message.
// See the events package for its fields and the names they take.
type Event = events.Event

// EventHandler receives every event produced by Validate, Apply and Destroy.
// It is called one event at a time, in the order the events were emitted,
// never concurrently, and every event of a call has been delivered before
// the call returns.
type EventHandler = events.Handler

// EventDataLevel says what resource data lifecycle events carry, set with
// WithEventData.
type EventDataLevel = events.DataLevel

const (
	// EventDataNone carries no resource data on any event. It is the default.
	EventDataNone = events.DataNone

	// EventDataRaw carries the resource as it was before the provider was
	// called, on every lifecycle event.
	EventDataRaw = events.DataRaw

	// EventDataProcessed carries, on a success event, the resource as xcl
	// records it in state, including the values the provider filled in. It is
	// the form EncodeSavedEntity reads, so a receiver can turn it straight
	// back into configuration text.
	EventDataProcessed = events.DataProcessed
)

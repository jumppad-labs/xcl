// Package events defines the single event shape that xcl and its plugins use
// to report everything they do: resource lifecycle steps, plugin discovery and
// loading, warnings, errors and log messages. xcl writes no output of its
// own, every diagnostic is an Event delivered to the application's Handler.
package events

import "time"

// Reserved Meta keys. A log event carries its severity under KeyLevel and
// its text under KeyMessage. Details supplied by the caller of a log method
// never overwrite these keys.
const (
	KeyLevel   = "level"
	KeyMessage = "message"
)

// SourceCore is the Source of every event produced by xcl itself. Events
// produced by a plugin carry the plugin's name as their Source.
const SourceCore = "core"

// Levels are the severities of a log event, held in Meta[KeyLevel].
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Phases say where in an operation an event sits.
const (
	// PhaseStart is emitted when an operation begins
	PhaseStart = "start"

	// PhaseSuccess is emitted when an operation completes without error
	PhaseSuccess = "success"

	// PhaseError is emitted when an operation fails, the event carries the error
	PhaseError = "error"

	// PhaseLog marks a log message written during the event's Operation
	PhaseLog = "log"

	// PhaseBlocked announces that emitters waited because the buffer of
	// undelivered events was full, the event's Duration is how long
	PhaseBlocked = "blocked"
)

// Operations are the core operation names an event can carry.
const (
	OperationParse    = "parse"
	OperationValidate = "validate"
	OperationApply    = "apply"
	OperationDestroy  = "destroy"
	OperationCreate   = "create"
	OperationRead     = "read"
	OperationChanged  = "changed"
	OperationUpdate   = "update"
	OperationDiscover = "discover"
	OperationLoad     = "load"
	OperationEvents   = "events"
)

// Event is one thing xcl or a plugin reports. Every kind of event shares this
// flat shape, anything specific to one kind, such as a log message's level
// and text, is held in Meta.
type Event struct {
	// Time is when the event happened, it is stamped when the event is
	// emitted if it is zero
	Time time.Time

	// Source is SourceCore for events produced by xcl, or the name of the
	// plugin that produced the event
	Source string

	// Operation is what was being done, one of the Operation constants. A
	// log event written during a provider call carries the lifecycle step in
	// progress, i.e. "create", so a resource's events read start, log...,
	// success or error.
	Operation string

	// Phase is one of the Phase constants
	Phase string

	// ResourceType is the "<type>.<name>" of the resource, i.e. "postgres.main"
	ResourceType string

	// ResourceID is the full path of the resource, i.e. "resource.postgres.main".
	// It is empty for events that do not concern a resource.
	ResourceID string

	// File is the file the resource was declared in
	File string

	// Duration is how long the operation took, set for success and error,
	// and for a blocked event how long emitters waited
	Duration time.Duration

	// Error is the reason the operation failed, set for the error phase
	Error error

	// Data is the serialized resource, set only for lifecycle events of
	// provider backed resources
	Data []byte

	// Meta holds the event's details. A log event carries its severity under
	// KeyLevel and its text under KeyMessage, alongside the key/value details
	// given with the message, which keep their names.
	Meta map[string]any
}

// Handler is the application's receiver for events. It is called one event
// at a time, in the order the events were emitted, never concurrently.
type Handler func(Event)

// Emit is what every emitter in xcl holds to report an event. A nil Emit is
// silent, callers check for nil before emitting.
type Emit func(Event)

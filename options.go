package xcl

import (
	"fmt"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
)

// ConfigOption is a functional option for configuring Config
type ConfigOption func(*Config) error

// WithRegistry adds a registry the Config gets its Go types and plugins from.
// It may be given any number of times: registries are read, and their plugins
// load, in the order they were given, and within a registry its plugins load
// in the order they were registered. NewConfig reads every registry's types
// and returns an error when they clash. Nothing is loaded until the first
// operation that needs plugins. A nil registry is a programmer error and
// panics.
func WithRegistry(r registry.Registry) ConfigOption {
	if r == nil {
		panic("xcl: registry must not be nil")
	}

	return func(c *Config) error {
		c.registries = append(c.registries, r)
		return nil
	}
}

// WithStateStore sets the state store for persistence
// Uses the existing state.StateStore interface from state/state_store.go
// Without WithStateStore or WithStatePath nothing is persisted; this is the
// supported mode for configuration-only use.
func WithStateStore(ss state.StateStore) ConfigOption {
	return func(c *Config) error {
		c.stateStore = ss
		return nil
	}
}

// WithStatePath persists state in a state.FileStateStore that keeps its file
// in the directory dir, creating the directory when it does not exist.
// NewConfig returns an error when the store cannot be created. When both
// WithStatePath and WithStateStore are given, the one given last is used.
// Without either, nothing is persisted; this is the supported mode for
// configuration-only use.
func WithStatePath(dir string) ConfigOption {
	return func(c *Config) error {
		store, err := state.NewFileStateStore(dir)
		if err != nil {
			return fmt.Errorf("failed to create state store: %w", err)
		}

		c.stateStore = store
		return nil
	}
}

// WithStateMask encrypts every sensitive value written to state with m, and
// opens it again when state is loaded. State then never holds a sensitive
// value in plain text. m is usually mask.EncryptAES256GCM, with a key kept
// wherever the application keeps its secrets.
//
// m must implement mask.Reversible, since state is read back to the real
// value. A one-way masker, such as mask.HashHMACSHA256, mask.Omit or
// mask.Redact, fails NewConfig with ErrMaskNotReversible, and so does nil.
//
// Loading state that holds a value this masker cannot open, because it was
// written under a different key or by a different masker, fails with
// ErrUnrecoverable rather than losing the value. State written in plain text
// still loads, and is encrypted on the next save.
func WithStateMask(m mask.Masker) ConfigOption {
	return func(c *Config) error {
		if m == nil {
			return fmt.Errorf("%w: no masker was given", xclerrors.ErrMaskNotReversible)
		}

		if _, ok := m.(mask.Reversible); !ok {
			return &xclerrors.MaskNotReversibleError{Masker: m.Name()}
		}

		c.stateMask = m
		return nil
	}
}

// DefaultEventBufferSize is the number of undelivered events a Validate,
// Apply or Destroy holds when WithEventBufferSize is not used
const DefaultEventBufferSize = 1024

// WithEventHandler sets the receiver for every event Validate, Apply and
// Destroy produce: the operation starting and finishing, each block being
// parsed, each provider call starting, succeeding or failing, plugin log
// messages, warnings and errors. The handler is called one event at a time
// and in the order the events were emitted, never concurrently. Emitting an
// event never waits for the handler, only for space in the buffer, see
// WithEventBufferSize, and every event of a call has been delivered before
// the call returns. Without a handler xcl produces no output at all.
//
// A panic in the handler is not recovered: xcl starts no new provider call,
// lets the calls in progress finish and saves state, then the panic continues
// from the Validate, Apply or Destroy call with its original value and stack.
func WithEventHandler(handler EventHandler) ConfigOption {
	return func(c *Config) error {
		c.eventHandler = handler
		return nil
	}
}

// WithEventBufferSize sets the number of events that can wait to be delivered
// to the event handler. Once the buffer is full, emitting waits for the
// handler to catch up instead of dropping events, and the handler is then
// sent one blocked event saying how long emitting waited. A size below 1 is
// treated as 1. The default is DefaultEventBufferSize.
func WithEventBufferSize(size int) ConfigOption {
	return func(c *Config) error {
		c.eventBufferSize = size
		return nil
	}
}

// WithVariables sets variables to pass to HCL parsing
func WithVariables(vars map[string]any) ConfigOption {
	return func(c *Config) error {
		c.variables = vars
		return nil
	}
}

// WithEventData says what resource data lifecycle events carry.
//
// Events carry none by default, so a resource's configuration and state do not
// travel through an event handler unless an application asks for them.
// EventDataRaw carries the resource as it was before the provider was called.
// EventDataProcessed carries, on a success event, the resource as xcl records
// it in state, which is the form EncodeSavedEntity reads.
//
// Sensitive values in event data are masked by the event masker, see
// WithEventMask, and in state by the state masker, see WithStateMask. Processed
// event data is therefore what state holds only where both mask alike.
func WithEventData(level EventDataLevel) ConfigOption {
	return func(c *Config) error {
		c.eventData = level
		return nil
	}
}

// WithEventMask masks every sensitive value in the resource data events carry
// with m, for example mask.HashHMACSHA256, which lets a receiver correlate
// values without seeing them. The default is mask.Redact(), which shows each
// sensitive value as an envelope holding the marker:
//
//	{"xcl_masked":"redact","value":"(sensitive)"}
//
// Only Event.Data is affected: errors and log details always show the marker.
// A nil masker fails NewConfig. When WithEventMask and WithNoEventMask are
// both given, the one given last is used.
func WithEventMask(m mask.Masker) ConfigOption {
	return func(c *Config) error {
		if m == nil {
			return fmt.Errorf("no event masker was given, use WithNoEventMask to turn event masking off")
		}

		c.eventMask = m
		c.eventMaskOff = false
		return nil
	}
}

// WithNoEventMask turns event masking off, so the resource data events carry
// holds real sensitive values. Use it only when every event receiver is
// trusted with secrets. Errors and log details still show the marker. When
// WithEventMask and WithNoEventMask are both given, the one given last is
// used.
func WithNoEventMask() ConfigOption {
	return func(c *Config) error {
		c.eventMask = nil
		c.eventMaskOff = true
		return nil
	}
}

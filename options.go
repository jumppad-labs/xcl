package xcl

import (
	"fmt"

	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
)

// ConfigOption is a functional option for configuring Config
type ConfigOption func(*Config) error

// WithPluginRegistry sets the plugin registry to use
// If not provided, only the builtin resource types will be available
func WithPluginRegistry(pr *registry.PluginRegistry) ConfigOption {
	return func(c *Config) error {
		c.pluginRegistry = pr
		return nil
	}
}

// WithStateStore sets the state store for persistence
// Uses the existing state.StateStore interface from state/state_store.go
// If not provided, state will not be persisted
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
func WithEventData(level EventDataLevel) ConfigOption {
	return func(c *Config) error {
		c.eventData = level
		return nil
	}
}

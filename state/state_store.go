package state

// StateStore persists the entities a configuration declares, so that a run can
// tell what the previous run produced.
//
// It exchanges plain values. Storing them is all this contract does: how they
// are typed and searched is the configuration object's concern, not a store's,
// and an implementation needs no library type in its signatures.
type StateStore interface {
	// Load retrieves what the previous run saved: either the entities as they
	// were saved, or each one's raw record as a json.RawMessage, []byte or
	// map[string]any, which the configuration types with its own registry.
	// Returns nil if nothing was saved (first run).
	// Returns an error if a saved state exists but cannot be loaded.
	Load() ([]any, error)

	// Save persists the entities a run produced. Each element is the entity's
	// JSON record as a json.RawMessage, with every sensitive value written as
	// its real value, so marshalling the slice with encoding/json stores the
	// real values. Load may return the same raw messages.
	// The implementation should ensure atomic writes to prevent corruption.
	Save(entities []any) error

	// Exists returns true if a saved state exists.
	Exists() bool

	// Clear removes the saved state.
	// This is useful for resetting or during cleanup operations.
	Clear() error
}

package state

// StateStore persists the entities a configuration declares, so that a run can
// tell what the previous run produced.
//
// It exchanges plain entities. Storing them is all this contract does: how they
// are searched is the configuration object's concern, not a store's, and an
// implementation needs no library type in its signatures.
type StateStore interface {
	// Load retrieves the entities saved by the previous run.
	// Returns nil if nothing was saved (first run).
	// Returns an error if a saved state exists but cannot be loaded.
	Load() ([]any, error)

	// Save persists the entities a run produced.
	// The implementation should ensure atomic writes to prevent corruption.
	Save(entities []any) error

	// Exists returns true if a saved state exists.
	Exists() bool

	// Clear removes the saved state.
	// This is useful for resetting or during cleanup operations.
	Clear() error
}

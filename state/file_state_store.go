package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

type FileStateStore struct {
	path     string
	registry *registry.PluginRegistry
}

func NewFileStateStore(path string, registry *registry.PluginRegistry) (*FileStateStore, error) {
	// Check if the file exists
	// If not, create an empty state file
	// Else, load the state from the file
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return createStateAtPath(path, registry)
	}

	fss := &FileStateStore{
		path:     path,
		registry: registry,
	}
	return fss, nil
}

// Load the previously saved configuration state from the file
func (fs *FileStateStore) Load() ([]any, error) {
	if _, err := os.Stat(fs.path); err != nil {
		return nil, fmt.Errorf("state file does not exist at %s", fs.path)
	}

	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("unable to read state file at %s: %w", fs.path, err)
	}

	// Phase 1: Unmarshal to raw messages to preserve JSON structure
	var rawMessages []*json.RawMessage
	err = json.Unmarshal(data, &rawMessages)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize state file at %s: %w", fs.path, err)
	}

	// Phase 2: Create typed resources and unmarshal into them.
	//
	// A record that cannot be understood is reported, never skipped. Dropping
	// one silently yields a smaller configuration than the file holds, which
	// then gets written back over the file on the next save, losing whatever
	// was dropped. Every failure below therefore accumulates into unresolved
	// and surfaces as one error naming what could not be read
	resources := []any{}
	unresolved := []string{}

	record := func(name string) {
		if !slices.Contains(unresolved, name) {
			unresolved = append(unresolved, name)
		}
	}

	for i, rawMsg := range rawMessages {
		resource, err := savedentity.Decode(fs.registry, *rawMsg)
		if err != nil {
			// a type nobody registered names itself, so the error can say what
			// to register. Anything else names the record, by its own id where
			// it had one and otherwise by its position in the file
			var unregistered *xclerrors.UnregisteredTypeError
			if errors.As(err, &unregistered) {
				record(unregistered.Type)
				continue
			}

			var invalid *xclerrors.InvalidSavedDataError
			if errors.As(err, &invalid) && invalid.ID != "" {
				record(invalid.ID)
				continue
			}

			record(fmt.Sprintf("entry %d", i))

			continue
		}

		// Append the typed resource pointer
		resources = append(resources, resource)
	}

	if len(unresolved) > 0 {
		slices.Sort(unresolved)
		return nil, UnknownTypesError{Types: unresolved}
	}

	return resources, nil
}

// Exists checks if the state file exists
func (fs *FileStateStore) Exists() bool {
	if _, err := os.Stat(fs.path); err == nil {
		return true
	}

	return false
}

// Clear removes the state file
func (fs *FileStateStore) Clear() error {
	return os.Remove(fs.path)
}

// Save the current configuration state to the file
func (fs *FileStateStore) Save(entities []any) error {
	if entities == nil {
		entities = []any{}
	}

	d, err := json.MarshalIndent(entities, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to serialize state: %w", err)
	}

	// Remove the existing file if it exists
	if _, err := os.Stat(fs.path); err == nil {
		err = os.Remove(fs.path)
		if err != nil {
			return fmt.Errorf("unable to remove existing state file: %w", err)
		}
	}

	err = os.WriteFile(fs.path, d, 0644)
	if err != nil {
		return fmt.Errorf("unable to write state file: %w", err)
	}

	return nil
}

func createStateAtPath(path string, registry *registry.PluginRegistry) (*FileStateStore, error) {
	fs := &FileStateStore{
		path:     path,
		registry: registry,
	}
	err := fs.Save(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create state file at %s: %w", path, err)
	}

	return fs, nil
}

package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StateFileName is the name of the file a FileStateStore keeps state in,
// inside the directory it is given
const StateFileName = "state.json"

// FileStateStore keeps state in a JSON file. It reads and writes the records
// and nothing more: Load returns each saved record as raw JSON, and turning a
// record back into a typed entity is left to whoever consumes the state.
type FileStateStore struct {
	path string
}

// NewFileStateStore returns a store that keeps state in StateFileName inside
// dir. The directory is created when it does not exist, and an empty state
// file is created in it when there is none, so a first run needs no setup.
func NewFileStateStore(dir string) (*FileStateStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("unable to create state directory at %s: %w", dir, err)
	}

	path := filepath.Join(dir, StateFileName)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return createStateAtPath(path)
	}

	fss := &FileStateStore{
		path: path,
	}
	return fss, nil
}

// Path returns the path of the file the store keeps state in
func (fs *FileStateStore) Path() string {
	return fs.path
}

// Load reads the previously saved state from the file. Each entity comes back
// as the json.RawMessage it was saved as, in the order it was saved.
func (fs *FileStateStore) Load() ([]any, error) {
	if _, err := os.Stat(fs.path); err != nil {
		return nil, fmt.Errorf("state file does not exist at %s", fs.path)
	}

	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, fmt.Errorf("unable to read state file at %s: %w", fs.path, err)
	}

	var records []json.RawMessage
	err = json.Unmarshal(data, &records)
	if err != nil {
		return nil, fmt.Errorf("unable to deserialize state file at %s: %w", fs.path, err)
	}

	entities := make([]any, 0, len(records))
	for _, record := range records {
		entities = append(entities, record)
	}

	return entities, nil
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

func createStateAtPath(path string) (*FileStateStore, error) {
	fs := &FileStateStore{
		path: path,
	}
	err := fs.Save(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create state file at %s: %w", path, err)
	}

	return fs, nil
}

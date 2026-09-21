package state

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

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
		// entry N is the fallback label for a record too malformed to name
		// itself, so the error can still point at a position in the file
		entry := fmt.Sprintf("entry %d", i)

		// Peek at the metadata to get type and name
		var metadata map[string]any
		err := json.Unmarshal(*rawMsg, &metadata)
		if err != nil {
			record(entry)
			continue
		}

		// Extract meta information
		metaMap, ok := metadata["meta"].(map[string]any)
		if !ok {
			record(entry)
			continue
		}

		// prefer the record's own identity over its position once there is one
		if id, ok := metaMap["id"].(string); ok && id != "" {
			entry = id
		}

		resourceType, ok := metaMap["type"].(string)
		if !ok || resourceType == "" {
			record(entry)
			continue
		}

		// A resource is created from its variety, every other kind from the
		// kind itself. Both axes are written to state, so read the variety
		// back where the record carries one
		if subtype, ok := metaMap["subtype"].(string); ok && subtype != "" {
			resourceType = subtype
		}

		resourceName, ok := metaMap["name"].(string)
		if !ok || resourceName == "" {
			record(entry)
			continue
		}

		// Create a typed resource reference using the registry
		typedResource, err := fs.registry.CreateResource(resourceType, resourceName)
		if err != nil {
			// the type is not registered, or the file predates the split and
			// records a kind that no longer resolves
			record(resourceType)
			continue
		}

		// Re-marshal and unmarshal into the typed reference. This overwrites
		// what CreateResource set with what the file holds, so a record
		// missing an axis keeps whatever the registry derived for it
		resData, err := json.Marshal(metadata)
		if err != nil {
			record(entry)
			continue
		}

		err = json.Unmarshal(resData, typedResource)
		if err != nil {
			record(entry)
			continue
		}

		// Append the typed resource pointer
		resources = append(resources, typedResource)
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

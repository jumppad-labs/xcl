package parser

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// State holds the entities a parse produced, and is the parser's working
// store while it runs.
//
// It is internal deliberately. Storing entities and answering questions about
// them are different jobs: the public state package persists entities as a
// plain slice, and addresses are resolved by the configuration object. Nothing
// outside this package needs a container.
type State struct {
	resources []any      // All resources in the configuration
	mu        sync.Mutex // Protects concurrent access
}

// NewState creates a new empty State
func NewState() *State {
	return &State{
		resources: []any{},
	}
}

// GetResources returns all resources
// This satisfies parser.ResourceProvider interface
func (s *State) GetResources() []any {
	return s.resources
}

// AppendResource adds a resource to the state
// If the given resource does not have ResourceBase embedded, an error is returned
func (s *State) AppendResource(r any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.addResource(r)
}

// RemoveResource removes a resource from the state
func (s *State) RemoveResource(rf any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pos := -1
	rfMeta, err := types.GetMeta(rf)
	if err != nil {
		return state.ResourceNotFoundError{}
	}

	for i, r := range s.resources {
		rMeta, err := types.GetMeta(r)
		if err != nil {
			continue
		}
		if rfMeta.Name == rMeta.Name &&
			rfMeta.Type == rMeta.Type &&
			rfMeta.Subtype == rMeta.Subtype &&
			rfMeta.Module == rMeta.Module {
			pos = i
			break
		}
	}

	if pos > -1 {
		s.resources = append(s.resources[:pos], s.resources[pos+1:]...)
		return nil
	}

	return state.ResourceNotFoundError{}
}

// ResourceCount returns the number of resources
func (s *State) ResourceCount() int {
	return len(s.resources)
}

// Bytes returns the state serialized as bytes
func (s *State) Bytes() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return json.MarshalIndent(s.resources, "", "  ")
}

// addResource is the internal unlocked version
func (s *State) addResource(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return fmt.Errorf("resource does not have ResourceBase embedded: %w", err)
	}

	// An entity is already held when one of the same kind, variety, name and
	// module is. This compares metadata rather than resolving an address,
	// because storing entities does not require knowing how they are addressed
	for _, e := range s.resources {
		existing, err := types.GetMeta(e)
		if err != nil {
			continue
		}

		if existing.Module == meta.Module &&
			existing.Type == meta.Type &&
			existing.Subtype == meta.Subtype &&
			existing.Name == meta.Name {
			return state.ResourceExistsError{Name: meta.Name}
		}
	}

	s.resources = append(s.resources, r)

	return nil
}

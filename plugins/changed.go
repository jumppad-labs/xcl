package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/internal/plaintype"
	"github.com/jumppad-labs/xcl/internal/wire"
)

// resourceBaseKeys are the JSON keys of types.ResourceBase. They hold xcl's own
// metadata about a resource, not provider state, so they never count as a change.
var resourceBaseKeys = []string{"meta", "depends_on", "disabled"}

// DefaultChanged provides default change detection for a ResourceProvider.
// Embed it in a provider to get a Changed method; defining Changed on the
// provider overrides it.
//
//	type MyProvider struct {
//		plugins.DefaultChanged[*MyResource]
//	}
type DefaultChanged[T any] struct{}

// Changed answers entity.Update when old and new differ and entity.NoChange
// when they do not. It compares the JSON form of both resources, ignoring xcl's
// resource metadata (meta, depends_on and disabled).
//
// It ignores both the changes and the dependencies lists, and never answers entity.Replace: a provider that
// cannot change some settings in place, or that must react to a replaced
// dependency, defines its own Changed and defers to this one for the rest.
func (DefaultChanged[T]) Changed(ctx context.Context, old T, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	oldValue, err := comparableJSON(old)
	if err != nil {
		return entity.NoChange, fmt.Errorf("unable to compare old resource: %w", err)
	}

	newValue, err := comparableJSON(new)
	if err != nil {
		return entity.NoChange, fmt.Errorf("unable to compare new resource: %w", err)
	}

	if reflect.DeepEqual(oldValue, newValue) {
		return entity.NoChange, nil
	}

	return entity.Update, nil
}

// comparableJSON returns the JSON form of a resource with xcl's metadata removed.
// A nil resource returns nil. A plain resource is compared in the form the
// engine stores it in, so its Meta and mirror fields are removed as
// ResourceBase's are, whatever its tags call them.
func comparableJSON(resource any) (any, error) {
	if b, ok := plaintype.LookupPlain(reflect.TypeOf(resource)); ok && !isNil(resource) {
		stored := b.New()
		if err := b.FromPlain(stored, resource); err != nil {
			return nil, err
		}

		resource = stored
	}

	data, err := wire.Marshal(resource)
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	if fields, ok := value.(map[string]any); ok {
		for _, key := range resourceBaseKeys {
			delete(fields, key)
		}
	}

	return value, nil
}

// isNil reports whether resource is nil or a nil pointer
func isNil(resource any) bool {
	if resource == nil {
		return true
	}

	value := reflect.ValueOf(resource)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

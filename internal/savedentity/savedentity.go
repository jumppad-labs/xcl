// Package savedentity reads one saved entity record back into its typed Go
// value.
//
// It is the single reader of the stored record format. The state store uses it
// for every record it loads, and the public conversion of saved data to
// configuration text uses it for one record at a time, so there is exactly one
// place that knows how a saved record names its type.
package savedentity

import (
	"encoding/json"
	"fmt"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// Decode returns the typed entity for one saved record. registry resolves the
// record's type, including the types a plugin provides, which are resolvable
// only once the registry has loaded.
//
// It fails with *xclerrors.UnregisteredTypeError when the registry cannot
// create the type the record names, and with *xclerrors.InvalidSavedDataError
// when the record cannot be read at all. The error carries the record's id
// where the record was readable enough to name itself.
func Decode(registry *registry.PluginRegistry, data []byte) (any, error) {
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, &xclerrors.InvalidSavedDataError{Err: err}
	}

	meta, ok := record["meta"].(map[string]any)
	if !ok {
		return nil, &xclerrors.InvalidSavedDataError{Err: fmt.Errorf("record has no meta")}
	}

	// prefer the record's own identity over nothing once there is one, so a
	// record that fails later can still be named
	id, _ := meta["id"].(string)

	resourceType, ok := meta["type"].(string)
	if !ok || resourceType == "" {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: fmt.Errorf("record has no type")}
	}

	// A resource is created from its variety, every other kind from the kind
	// itself. Both axes are written to state, so read the variety back where
	// the record carries one
	if subtype, ok := meta["subtype"].(string); ok && subtype != "" {
		resourceType = subtype
	}

	resourceName, ok := meta["name"].(string)
	if !ok || resourceName == "" {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: fmt.Errorf("record has no name")}
	}

	// Create a typed resource reference using the registry
	resource, err := registry.CreateResource(resourceType, resourceName)
	if err != nil {
		// the type is not registered, or the record predates the split and
		// names a kind that no longer resolves
		return nil, &xclerrors.UnregisteredTypeError{Type: resourceType}
	}

	// Re-marshal and unmarshal into the typed reference. This overwrites what
	// CreateResource set with what the record holds, so a record missing an
	// axis keeps whatever the registry derived for it
	resData, err := json.Marshal(record)
	if err != nil {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: err}
	}

	if err := json.Unmarshal(resData, resource); err != nil {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: err}
	}

	return resource, nil
}

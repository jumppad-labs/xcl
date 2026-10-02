// Package savedentity reads one saved entity record back into its typed Go
// value.
//
// It is the single reader of the stored record format. A state store only reads
// and writes records, so the parser types every record it loads from one with
// DecodeAll, and the public conversion of saved data to configuration text uses
// Decode for one record at a time, so there is exactly one place that knows how
// a saved record names its type.
package savedentity

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
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

// DecodeAll returns the typed entities for what a state store loaded.
//
// A store hands back whatever it keeps: a json.RawMessage, []byte or
// map[string]any is a saved record and is decoded with Decode, anything else
// is taken to be an entity already and is returned unchanged, so a store that
// keeps entities in memory needs no decoding.
//
// A record that cannot be understood is reported, never skipped. Dropping one
// silently yields a smaller configuration than the store holds, which then
// gets saved back over it on the next save, losing whatever was dropped. Every
// failure therefore accumulates into one state.UnknownTypesError naming what
// could not be read.
func DecodeAll(registry *registry.PluginRegistry, loaded []any) ([]any, error) {
	entities := make([]any, 0, len(loaded))
	unresolved := []string{}

	record := func(name string) {
		if !slices.Contains(unresolved, name) {
			unresolved = append(unresolved, name)
		}
	}

	for i, item := range loaded {
		data, isRecord, err := recordData(item)
		if !isRecord {
			entities = append(entities, item)
			continue
		}

		if err == nil && registry == nil {
			return nil, fmt.Errorf("saved state holds records but there is no plugin registry to type them")
		}

		if err == nil {
			var entity any
			entity, err = Decode(registry, data)
			if err == nil {
				entities = append(entities, entity)
				continue
			}
		}

		// a type nobody registered names itself, so the error can say what to
		// register. Anything else names the record, by its own id where it had
		// one and otherwise by its position in what was loaded
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
	}

	if len(unresolved) > 0 {
		slices.Sort(unresolved)
		return nil, state.UnknownTypesError{Types: unresolved}
	}

	return entities, nil
}

// recordData returns the JSON of item when item is a saved record rather than
// an entity, and whether it was one
func recordData(item any) ([]byte, bool, error) {
	switch record := item.(type) {
	case json.RawMessage:
		return record, true, nil
	case []byte:
		return record, true, nil
	case map[string]any:
		data, err := json.Marshal(record)
		return data, true, err
	default:
		return nil, false, nil
	}
}

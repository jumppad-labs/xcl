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
	"github.com/jumppad-labs/xcl/internal/catalog"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// ReadOptions say how a reader treats masked values in a saved record.
//
// A record holds each sensitive value either plainly or as a masker's
// envelope (see the mask package). Plain values pass through unchanged in
// every mode.
type ReadOptions struct {
	// Mask is the state masker, nil when none is configured. Loading state
	// opens every envelope with it, and fails with
	// *xclerrors.UnrecoverableError for any envelope it cannot open, so a
	// secret is never silently replaced by a value that would then be saved
	// back over it.
	Mask mask.Masker

	// ForDisplay turns every envelope into types.SensitiveMarker instead of
	// opening it, so masked data shown to someone never presents ciphertext
	// or a hash as a value.
	ForDisplay bool
}

// Decode returns the typed entity for one saved record. registry resolves the
// record's type, including the types a plugin provides, which are resolvable
// only once the registry has loaded.
//
// It fails with *xclerrors.UnregisteredTypeError when the registry cannot
// create the type the record names, and with *xclerrors.InvalidSavedDataError
// when the record cannot be read at all. The error carries the record's id
// where the record was readable enough to name itself.
//
// Masked values are treated as read says, before the record is typed. A
// masked value that cannot be opened fails with
// *xclerrors.UnrecoverableError naming the record and the masker.
func Decode(registry *catalog.Catalog, data []byte, read ReadOptions) (any, error) {
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

	entityType, ok := meta["type"].(string)
	if !ok || entityType == "" {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: fmt.Errorf("record has no type")}
	}

	// an entity is created from its type and its subtype, where it has one,
	// both of which are written to state
	subtype, _ := meta["subtype"].(string)

	name, ok := meta["name"].(string)
	if !ok || name == "" {
		return nil, &xclerrors.InvalidSavedDataError{ID: id, Err: fmt.Errorf("record has no name")}
	}

	opened, err := openMasked(record, read, id)
	if err != nil {
		return nil, err
	}
	record = opened.(map[string]any)

	// Create a typed entity using the registry
	resource, err := registry.CreateEntity(entityType, subtype, name)
	if err != nil {
		// the type is not registered, or not loaded
		return nil, &xclerrors.UnregisteredTypeError{Type: types.TypeKey(entityType, subtype)}
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
//
// A masked value that cannot be opened with read's masker is not one more
// unreadable record: it is returned at once as *xclerrors.UnrecoverableError,
// so the load fails with xclerrors.ErrUnrecoverable naming the entity and the
// masker.
func DecodeAll(registry *catalog.Catalog, loaded []any, read ReadOptions) ([]any, error) {
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
			entity, err = Decode(registry, data, read)
			if err == nil {
				entities = append(entities, entity)
				continue
			}
		}

		var unrecoverable *xclerrors.UnrecoverableError
		if errors.As(err, &unrecoverable) {
			return nil, err
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

// openMasked walks a decoded record and replaces every envelope in it: with
// the marker when reading for display, and otherwise with the value the state
// masker opens it to. id names the record in any error.
func openMasked(node any, read ReadOptions, id string) (any, error) {
	switch value := node.(type) {
	case map[string]any:
		if masked, ok := mask.IsMaskedObject(value); ok {
			return openEnvelope(masked, read, id)
		}

		for key, child := range value {
			opened, err := openMasked(child, read, id)
			if err != nil {
				return nil, err
			}

			value[key] = opened
		}

		return value, nil

	case []any:
		for i, child := range value {
			opened, err := openMasked(child, read, id)
			if err != nil {
				return nil, err
			}

			value[i] = opened
		}

		return value, nil
	}

	return node, nil
}

func openEnvelope(masked mask.Masked, read ReadOptions, id string) (any, error) {
	if read.ForDisplay {
		return types.SensitiveMarker, nil
	}

	if read.Mask == nil {
		return nil, &xclerrors.UnrecoverableError{
			ID:       id,
			MaskedBy: masked.By,
			Reason:   "no state masker is configured",
		}
	}

	data, err := mask.Open(masked, read.Mask)
	if err != nil {
		var unrecoverable *xclerrors.UnrecoverableError
		if errors.As(err, &unrecoverable) {
			unrecoverable.ID = id
			return nil, unrecoverable
		}

		return nil, &xclerrors.UnrecoverableError{ID: id, MaskedBy: masked.By, Err: err}
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, &xclerrors.UnrecoverableError{ID: id, MaskedBy: masked.By, Reason: "it opened to invalid JSON", Err: err}
	}

	return value, nil
}

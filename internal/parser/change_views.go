package parser

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
)

// A resource's changes are worked out once, by resourceChanges with sensitive
// values revealed, and that one result is shown two ways: planChanges for the
// plan and pluginChanges for the provider. Both views come from the same
// changes, so the plan and the provider are always told the same settings.

// planChanges returns the plan's view of revealed: a copy in which every
// sensitive change carries neither value unless reveal is set. It is exactly
// what resourceChanges returns without reveal.
func planChanges(revealed []diff.Change, reveal bool) []diff.Change {
	if revealed == nil {
		return nil
	}

	changes := make([]diff.Change, 0, len(revealed))
	for _, change := range revealed {
		if change.Sensitive && !reveal {
			change.Before = nil
			change.After = nil
		}

		changes = append(changes, change)
	}

	return changes
}

// pluginChanges returns the provider's view of revealed: each change as an
// entity.PropertyChange holding real values as plain JSON values, the same
// values a plugin running as a separate program decodes from the wire.
func pluginChanges(revealed []diff.Change) ([]entity.PropertyChange, error) {
	if len(revealed) == 0 {
		return nil, nil
	}

	changes := make([]entity.PropertyChange, 0, len(revealed))
	for _, change := range revealed {
		before, err := jsonValue(change.Before)
		if err != nil {
			return nil, fmt.Errorf("unable to convert the previous value of %s: %w", change.Path, err)
		}

		after, err := jsonValue(change.After)
		if err != nil {
			return nil, fmt.Errorf("unable to convert the new value of %s: %w", change.Path, err)
		}

		changes = append(changes, entity.PropertyChange{
			Path:      change.Path,
			Before:    before,
			After:     after,
			Unknown:   change.Unknown,
			Sensitive: change.Sensitive,
		})
	}

	return changes, nil
}

// resolveUnknown returns a copy of revealed in which every unknown change
// holds its real new value, read from configured, the resource decoded with
// every value known. The change stays listed even when its value turns out
// equal to the saved one, so the settings match those the plan showed.
func resolveUnknown(revealed []diff.Change, configured any) []diff.Change {
	if revealed == nil {
		return nil
	}

	changes := make([]diff.Change, 0, len(revealed))
	for _, change := range revealed {
		if change.Unknown {
			value := valueAt(reflect.ValueOf(configured), change.Path)

			change.Unknown = false
			change.After = plainValue(value, true)
			change.Sensitive = change.Sensitive || containsSensitive(value)
		}

		changes = append(changes, change)
	}

	return changes
}

// valueAt returns the value at path inside value, following configuration
// names, list positions and map keys. It returns the zero Value when the path
// leads nowhere.
func valueAt(value reflect.Value, path diff.Path) reflect.Value {
	for _, step := range path {
		value = indirect(value)
		if !value.IsValid() || value.Type() == ctyValueType || isSensitiveType(value.Type()) {
			return reflect.Value{}
		}

		switch step.Kind {
		case diff.StepAttribute:
			value = fieldNamed(value, step.Attribute)

		case diff.StepIndex:
			if (value.Kind() != reflect.Slice && value.Kind() != reflect.Array) || step.Index >= value.Len() {
				return reflect.Value{}
			}

			value = value.Index(step.Index)

		case diff.StepKey:
			value = mapElementNamed(value, step.Key)
		}
	}

	return value
}

// fieldNamed returns the field of a struct value that configuration names
// name, or the zero Value
func fieldNamed(value reflect.Value, name string) reflect.Value {
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}

	for _, f := range structFields(value.Type()) {
		if f.name == name {
			return value.FieldByIndex(f.index)
		}
	}

	return reflect.Value{}
}

// mapElementNamed returns the element of a map value whose key has the string
// form key, or the zero Value
func mapElementNamed(value reflect.Value, key string) reflect.Value {
	if value.Kind() != reflect.Map || value.IsNil() {
		return reflect.Value{}
	}

	iter := value.MapRange()
	for iter.Next() {
		if mapKeyString(iter.Key()) == key {
			return iter.Value()
		}
	}

	return reflect.Value{}
}

// jsonValue returns value as a plain JSON value: string, float64, bool, nil,
// []any or map[string]any
func jsonValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		return nil, err
	}

	return plain, nil
}

package parser

import (
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/types"
)

// unknownValues is consulted when an entity's evaluation context is built.
// It can replace the value a linked entity is reached through, so values only
// known once an apply has run read as unknown. Apply builds contexts without
// one.
type unknownValues interface {
	contextValue(entity any, value cty.Value) cty.Value
}

// contextValue returns the value a linked entity is reached through in a diff:
// the computed fields of an entity an apply would create, replace or update
// are unknown, since its provider has not computed them yet, and so is every
// configured value of the entity recorded as unknown. The entity itself is
// never changed.
func (r *diffRecorder) contextValue(entity any, value cty.Value) cty.Value {
	meta, err := types.GetMeta(entity)
	if err != nil {
		return value
	}

	if r.isPending(meta.ID) {
		for _, field := range computedFields(reflect.TypeOf(entity)) {
			value = unknownAtNames(value, strings.Split(field.path, "."))
		}
	}

	for _, path := range r.unknownPaths(meta.ID) {
		value = unknownAtPath(value, path)
	}

	return value
}

// unknownAtNames returns value with the attribute at the dotted names made
// unknown. A list, set, tuple or map along the way, such as a list of blocks,
// has the rest of the names applied to each of its elements.
func unknownAtNames(value cty.Value, names []string) cty.Value {
	if len(names) == 0 {
		return cty.UnknownVal(value.Type()).WithMarks(value.Marks())
	}

	return transformKnown(value, func(unmarked cty.Value) cty.Value {
		switch {
		case unmarked.Type().IsObjectType():
			if !unmarked.Type().HasAttribute(names[0]) {
				return unmarked
			}

			attributes := unmarked.AsValueMap()
			attributes[names[0]] = unknownAtNames(attributes[names[0]], names[1:])
			return cty.ObjectVal(attributes)

		case isElementCollection(unmarked.Type()):
			return mapElements(unmarked, func(element cty.Value) cty.Value {
				return unknownAtNames(element, names)
			})
		}

		return unmarked
	})
}

// unknownAtPath returns value with the value at path made unknown. A path
// that does not lead anywhere in value leaves it as it is.
func unknownAtPath(value cty.Value, path diff.Path) cty.Value {
	if len(path) == 0 {
		return cty.UnknownVal(value.Type()).WithMarks(value.Marks())
	}

	step := path[0]

	return transformKnown(value, func(unmarked cty.Value) cty.Value {
		valueType := unmarked.Type()

		switch {
		case step.Kind == diff.StepAttribute && valueType.IsObjectType():
			if !valueType.HasAttribute(step.Attribute) {
				return unmarked
			}

			attributes := unmarked.AsValueMap()
			attributes[step.Attribute] = unknownAtPath(attributes[step.Attribute], path[1:])
			return cty.ObjectVal(attributes)

		case step.Kind == diff.StepKey && (valueType.IsMapType() || valueType.IsObjectType()):
			elements := unmarked.AsValueMap()
			element, ok := elements[step.Key]
			if !ok {
				return unmarked
			}

			elements[step.Key] = unknownAtPath(element, path[1:])
			if valueType.IsMapType() {
				return cty.MapVal(elements)
			}

			return cty.ObjectVal(elements)

		case step.Kind == diff.StepIndex && (valueType.IsListType() || valueType.IsTupleType()):
			elements := unmarked.AsValueSlice()
			if step.Index < 0 || step.Index >= len(elements) {
				return unmarked
			}

			elements[step.Index] = unknownAtPath(elements[step.Index], path[1:])
			if valueType.IsListType() {
				return cty.ListVal(elements)
			}

			return cty.TupleVal(elements)
		}

		return unmarked
	})
}

// transformKnown applies transform to the unmarked form of a known, non-null
// value and puts its marks back. An unknown or null value is returned as it
// is.
func transformKnown(value cty.Value, transform func(cty.Value) cty.Value) cty.Value {
	if !value.IsKnown() || value.IsNull() {
		return value
	}

	unmarked, marks := value.Unmark()

	return transform(unmarked).WithMarks(marks)
}

// isElementCollection returns true for the types whose elements are reached
// without a name: lists, sets, tuples and maps
func isElementCollection(t cty.Type) bool {
	return t.IsListType() || t.IsSetType() || t.IsTupleType() || t.IsMapType()
}

// mapElements returns the collection with transform applied to each of its
// elements. Every element keeps its type, so the collection keeps its own.
func mapElements(collection cty.Value, transform func(cty.Value) cty.Value) cty.Value {
	collectionType := collection.Type()

	if collection.LengthInt() == 0 {
		return collection
	}

	if collectionType.IsMapType() {
		elements := collection.AsValueMap()
		for key, element := range elements {
			elements[key] = transform(element)
		}

		return cty.MapVal(elements)
	}

	elements := collection.AsValueSlice()
	for i, element := range elements {
		elements[i] = transform(element)
	}

	switch {
	case collectionType.IsListType():
		return cty.ListVal(elements)
	case collectionType.IsSetType():
		return cty.SetVal(elements)
	default:
		return cty.TupleVal(elements)
	}
}

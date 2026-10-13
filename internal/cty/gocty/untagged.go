package gocty

// This file is an addition to the upstream go-cty library, recorded in
// internal/cty/UPSTREAM.md. It is licensed under the MIT License, as the
// rest of this package is.

import (
	"reflect"
	"sync"

	"github.com/jumppad-labs/xcl/internal/xcl/tags"
)

// untaggedField is an object attribute decoded into a struct field that has
// no tag naming it, registered with RegisterUntaggedField.
type untaggedField struct {
	attribute string
	field     string
	fieldType reflect.Type
}

var (
	untaggedMutex  sync.RWMutex
	untaggedFields []untaggedField
)

// RegisterUntaggedField makes FromCtyValue decode the object attribute
// attribute into a struct's field called field, of type fieldType, when that
// field has no tag and no tagged field takes the attribute. It is only ever
// read into: ImpliedConstraintType lists it as an optional attribute, so a
// value converted against the struct's type keeps it, and it gives nothing to
// ImpliedType or ToCtyValue.
func RegisterUntaggedField(attribute, field string, fieldType reflect.Type) {
	untaggedMutex.Lock()
	defer untaggedMutex.Unlock()

	untaggedFields = append(untaggedFields, untaggedField{attribute: attribute, field: field, fieldType: fieldType})
}

// untaggedFieldsOf returns, by attribute, the index of each field of the
// struct type st registered to receive an attribute
func untaggedFieldsOf(st reflect.Type) map[string]int {
	untaggedMutex.RLock()
	attributes := make([]string, 0, len(untaggedFields))
	for _, registered := range untaggedFields {
		attributes = append(attributes, registered.attribute)
	}
	untaggedMutex.RUnlock()

	found := map[string]int{}
	for _, attribute := range attributes {
		if index, ok := untaggedFieldFor(st, attribute); ok {
			found[attribute] = index
		}
	}

	return found
}

// untaggedFieldFor returns the index of the field of the struct type st that
// attribute is registered to be decoded into, and false when there is none.
func untaggedFieldFor(st reflect.Type, attribute string) (int, bool) {
	untaggedMutex.RLock()
	defer untaggedMutex.RUnlock()

	for _, registered := range untaggedFields {
		if registered.attribute != attribute {
			continue
		}

		field, ok := st.FieldByName(registered.field)
		if !ok || len(field.Index) != 1 || !field.IsExported() || field.Type != registered.fieldType {
			continue
		}

		if _, tagged := field.Tag.Lookup(tags.Name); tagged {
			continue
		}

		return field.Index[0], true
	}

	return 0, false
}

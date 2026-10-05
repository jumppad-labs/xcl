package gocty

// This file is an addition to the upstream go-cty library, recorded in
// internal/cty/UPSTREAM.md. It is licensed under the MIT License, as the
// rest of this package is.

import (
	"reflect"
	"sync"

	"github.com/jumppad-labs/xcl/internal/cty"
)

// Wrapper describes a Go type that wraps exactly one inner value and carries
// a mark through cty. A registered wrapper converts as its inner type:
//
//   - ImpliedType gives the inner type's cty type.
//   - ToCtyValue converts the inner value and marks the result with Mark.
//   - FromCtyValue into a wrapper removes the marks and wraps the decoded
//     inner value, whether or not the cty value was marked.
type Wrapper struct {
	// Inner recognises a wrapper type and returns its inner type.
	Inner func(t reflect.Type) (inner reflect.Type, ok bool)

	// Unwrap returns the inner value held by wrapper.
	Unwrap func(wrapper reflect.Value) reflect.Value

	// Wrap stores inner into target, which is an addressable wrapper value.
	Wrap func(target reflect.Value, inner reflect.Value)

	// Mark is applied to the cty value on the way into cty.
	Mark any

	// ExtraMarks, when set, returns further marks to apply to the cty value
	// of a particular wrapper value, alongside Mark.
	ExtraMarks func(wrapper reflect.Value) []any
}

var (
	wrappersMutex sync.RWMutex
	wrappers      []Wrapper
)

// RegisterWrapper registers a wrapper type with the conversion functions in
// this package. It is intended to be called from an init function.
func RegisterWrapper(wrapper Wrapper) {
	wrappersMutex.Lock()
	defer wrappersMutex.Unlock()

	wrappers = append(wrappers, wrapper)
}

// wrapperFor returns the registered wrapper for t and t's inner type.
func wrapperFor(t reflect.Type) (Wrapper, reflect.Type, bool) {
	if t == nil {
		return Wrapper{}, nil, false
	}

	wrappersMutex.RLock()
	defer wrappersMutex.RUnlock()

	for _, wrapper := range wrappers {
		if inner, ok := wrapper.Inner(t); ok {
			return wrapper, inner, true
		}
	}

	return Wrapper{}, nil, false
}

// toCtyWrapper converts the inner value of a wrapper and marks the result.
func toCtyWrapper(wrapper Wrapper, inner reflect.Type, val reflect.Value, ty cty.Type, path cty.Path) (cty.Value, error) {
	innerValue := wrapper.Unwrap(val)
	if !innerValue.IsValid() {
		innerValue = reflect.Zero(inner)
	}

	converted, err := toCtyValue(innerValue, ty, path)
	if err != nil {
		return cty.NilVal, err
	}

	marks := []any{wrapper.Mark}
	if wrapper.ExtraMarks != nil {
		marks = append(marks, wrapper.ExtraMarks(val)...)
	}

	return converted.WithMarks(cty.NewValueMarks(marks...)), nil
}

// fromCtyWrapper decodes an unmarked copy of val into a new inner value and
// stores it in target.
func fromCtyWrapper(wrapper Wrapper, inner reflect.Type, val cty.Value, target reflect.Value, path cty.Path) error {
	unmarked, _ := val.UnmarkDeep()

	innerTarget := reflect.New(inner)
	if err := fromCtyValue(unmarked, innerTarget, path); err != nil {
		return err
	}

	wrapper.Wrap(target, innerTarget.Elem())

	return nil
}

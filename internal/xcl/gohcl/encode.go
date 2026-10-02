// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/gocty"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
)

// EncodeOptions controls what EncodeBody writes.
type EncodeOptions struct {
	// IncludeComputed writes fields whose tag carries the computed option.
	// They are owned by the provider rather than written in configuration,
	// so they are left out unless this is set. Each one written is marked
	// with ComputedComment when there is one.
	IncludeComputed bool

	// ComputedComment is written after each computed field, so a reader can
	// tell a value the provider filled in from one that was configured. It
	// is used only when IncludeComputed is set, and only for a field written
	// directly into a body, not for one inside an object value.
	ComputedComment string
}

// EncodeBody replaces the contents of the given hclwrite Body with attributes
// and blocks derived from the given value, which must be a struct value or a
// pointer to a struct value with the struct tags defined in this package.
//
// It writes what DecodeBody reads. Fields of a struct embedded under a
// "remain" tag are written alongside the embedding struct's own fields, in
// field order, because that is how the decoder fills them. A field held in an
// interface is written by the type of the value it holds. Fields holding
// nothing, a nil pointer, slice, map or interface, are left out, while a zero
// number, false or empty string that is present is written.
//
// Fields that decode attributes into hcl.Expression or hcl.Attribute values,
// and fields that decode blocks into hcl.Body or hcl.Attributes values, are
// ignored, as this function does not have enough information to write them.
// Fields tagged as "label" are ignored too. Use EncodeAsBlock to produce a
// whole hclwrite.Block including block labels.
//
// Unlike EncodeIntoBody, a value that cannot be represented as configuration
// is reported as an error naming the field, rather than by panicking.
//
// The layout of the resulting HCL source is derived from the ordering of the
// struct fields, with blank lines around nested blocks of different types.
func EncodeBody(val any, dst *hclwrite.Body, options EncodeOptions) (err error) {
	// hclwrite and gocty are derived from upstream HCL, which reports a value
	// it cannot represent by panicking rather than returning an error, for
	// example an unknown or capsule value in hclwrite.TokensForValue. This
	// recover is the boundary that keeps those panics inside the fork so the
	// public API can return an error. It covers this function only, and the
	// cases reached directly from here are returned as errors below.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot encode %T as configuration: %v", val, r)
		}
	}()

	rv, ty, err := structValue(val)
	if err != nil {
		return err
	}

	dst.Clear()

	enc := &bodyEncoder{dst: dst, options: options}

	return enc.encodeStruct(rv, ty, "")
}

// EncodeIntoBody replaces the contents of the given hclwrite Body with
// attributes and blocks derived from the given value, which must be a
// struct value or a pointer to a struct value with the struct tags defined
// in this package.
//
// This function can work only with fully-decoded data. It will ignore any
// fields that decode attributes into either hcl.Attribute or hcl.Expression
// values, and any fields that decode blocks into hcl.Attributes values. This
// function does not have enough information to complete the decoding of these
// types.
//
// Any fields tagged as "label" are ignored by this function. Use EncodeAsBlock
// to produce a whole hclwrite.Block including block labels.
//
// As long as a suitable value is given to encode and the destination body
// is non-nil, this function will always complete. It will panic in case of
// any errors in the calling program, such as passing an inappropriate type
// or a nil body. Use EncodeBody to receive those as errors instead.
//
// The layout of the resulting HCL source is derived from the ordering of
// the struct fields, with blank lines around nested blocks of different types.
// Fields representing attributes should usually precede those representing
// blocks so that the attributes can group togather in the result. For more
// control, use the hclwrite API directly.
func EncodeIntoBody(val interface{}, dst *hclwrite.Body) {
	if err := EncodeBody(val, dst, EncodeOptions{IncludeComputed: true}); err != nil {
		panic(err.Error())
	}
}

// EncodeAsBlock creates a new hclwrite.Block populated with the data from
// the given value, which must be a struct or pointer to struct with the
// struct tags defined in this package.
//
// If the given struct type has fields tagged with "label" tags then they
// will be used in order to annotate the created block with labels.
//
// This function has the same constraints as EncodeIntoBody and will panic
// if they are violated.
func EncodeAsBlock(val interface{}, blockType string) *hclwrite.Block {
	rv, ty, err := structValue(val)
	if err != nil {
		panic(err.Error())
	}

	block, err := encodeAsBlock(rv, ty, blockType, EncodeOptions{IncludeComputed: true}, "")
	if err != nil {
		panic(err.Error())
	}

	return block
}

// structValue resolves val to the struct value to encode, following a pointer
// to the struct it points at
func structValue(val any) (reflect.Value, reflect.Type, error) {
	rv := reflect.ValueOf(val)
	if !rv.IsValid() {
		return reflect.Value{}, nil, fmt.Errorf("value is nil, not struct")
	}

	ty := rv.Type()
	if ty.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return reflect.Value{}, nil, fmt.Errorf("value is a nil %s, not struct", ty)
		}

		rv = rv.Elem()
		ty = rv.Type()
	}

	if ty.Kind() != reflect.Struct {
		return reflect.Value{}, nil, fmt.Errorf("value is %s, not struct", ty.Kind())
	}

	return rv, ty, nil
}

// encodeAsBlock builds a block of the given type from a struct value, taking
// its labels from the value's label fields and its body from the rest
func encodeAsBlock(rv reflect.Value, ty reflect.Type, blockType string, options EncodeOptions, path string) (*hclwrite.Block, error) {
	tags := getFieldTags(ty)

	labels := make([]string, len(tags.Labels))
	for i, lf := range tags.Labels {
		lv := rv.Field(lf.FieldIndex)
		// We just stringify whatever we find. It should always be a string
		// but if not then we'll still do something reasonable.
		labels[i] = fmt.Sprintf("%s", lv.Interface())
	}

	block := hclwrite.NewBlock(blockType, labels)

	enc := &bodyEncoder{dst: block.Body(), options: options}
	if err := enc.encodeStruct(rv, ty, path); err != nil {
		return nil, err
	}

	return block, nil
}

// bodyEncoder writes struct fields into one destination body. It holds the
// layout state that decides where blank lines fall, so a struct embedded
// under a "remain" tag and the struct embedding it lay out as one body.
type bodyEncoder struct {
	dst     *hclwrite.Body
	options EncodeOptions

	// prevWasBlock records whether the last thing written was a block, so an
	// attribute that follows one is separated from it by a blank line
	prevWasBlock bool
}

// bodyField is one field to write, in the order it is declared
type bodyField struct {
	index int
	name  string
	kind  fieldKind
}

type fieldKind int

const (
	fieldAttribute fieldKind = iota
	fieldBlock
	fieldRemain
)

// encodeStruct writes the fields of rv into the encoder's body, in field
// order. path names the struct within the value being encoded, so an error
// can say which field it came from, and is empty at the top level.
func (e *bodyEncoder) encodeStruct(rv reflect.Value, ty reflect.Type, path string) error {
	tags := getFieldTags(ty)

	for _, f := range bodyFields(ty) {
		// a field the provider owns is not configuration, so it is left out
		// unless the caller asked to see it
		computed := tags.Computed[f.name]
		if computed && !e.options.IncludeComputed {
			continue
		}

		field := ty.Field(f.index)
		fieldVal := rv.Field(f.index)

		var err error
		switch f.kind {
		case fieldRemain:
			err = e.encodeRemain(field, fieldVal, path)
		case fieldAttribute:
			err = e.encodeAttribute(field, fieldVal, f.name, joinPath(path, f.name), computed)
		case fieldBlock:
			err = e.encodeBlock(field, fieldVal, f.name, joinPath(path, f.name))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// bodyFields lists the fields of ty that make up a body, in the order they are
// declared, so a struct embedded under a remain tag is walked in its place
func bodyFields(ty reflect.Type) []bodyField {
	tags := getFieldTags(ty)

	fields := make([]bodyField, 0, len(tags.Attributes)+len(tags.Blocks)+1)
	for n, i := range tags.Attributes {
		fields = append(fields, bodyField{index: i, name: n, kind: fieldAttribute})
	}
	for n, i := range tags.Blocks {
		fields = append(fields, bodyField{index: i, name: n, kind: fieldBlock})
	}
	if tags.Remain != nil {
		fields = append(fields, bodyField{index: *tags.Remain, kind: fieldRemain})
	}

	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].index < fields[j].index
	})

	return fields
}

// encodeRemain writes the fields of a struct embedded under a "remain" tag
// into the same body, which is where the decoder reads them from. A remain
// field that holds an undecoded body is skipped, as there is nothing to write.
func (e *bodyEncoder) encodeRemain(field reflect.StructField, fieldVal reflect.Value, path string) error {
	// only an embedded struct carries fields of its own, hcl.Body and
	// hcl.Attributes remains hold leftovers this encoder cannot write
	if !field.Anonymous {
		return nil
	}

	ty := field.Type
	if ty.Kind() == reflect.Ptr {
		if fieldVal.IsNil() {
			return nil
		}

		fieldVal = fieldVal.Elem()
		ty = ty.Elem()
	}

	if ty.Kind() != reflect.Struct {
		return nil
	}

	return e.encodeStruct(fieldVal, ty, path)
}

// encodeAttribute writes one attribute, taking the value held in an interface
// field by its dynamic type. A field holding nothing is left out.
func (e *bodyEncoder) encodeAttribute(field reflect.StructField, fieldVal reflect.Value, name, path string, computed bool) error {
	// these hold an undecoded expression rather than a value, so there is
	// nothing to write
	if field.Type == exprType || field.Type == attrType {
		return nil
	}

	fieldVal, ok := resolveValue(fieldVal)
	if !ok {
		return nil
	}

	if e.prevWasBlock {
		e.dst.AppendNewline()
		e.prevWasBlock = false
	}

	val, err := e.attributeValue(fieldVal, path)
	if err != nil {
		return err
	}

	attr := e.dst.SetAttributeValue(name, val)

	// mark a value the provider filled in, so a reader can tell it from one
	// that was configured
	if computed && e.options.ComputedComment != "" {
		attr.SetLineComment(e.options.ComputedComment)
	}

	return nil
}

// attributeValue converts a field's value to the value written for it.
//
// A struct carrying xcl tags becomes an object built from the fields this
// encoder would write, rather than from every tagged field the way gocty
// builds one. That is what makes the rules about computed fields, empty
// values and the caller's own bookkeeping reach inside an attribute whose
// value is a whole object, and not just the fields of the body itself.
// Anything else is converted by gocty as before.
func (e *bodyEncoder) attributeValue(val reflect.Value, path string) (cty.Value, error) {
	switch val.Kind() {
	case reflect.Struct:
		if tagged(val.Type()) {
			return e.objectValue(val, path)
		}

	case reflect.Slice, reflect.Array:
		if tagged(elementType(val.Type())) {
			return e.tupleValue(val, path)
		}

	case reflect.Map:
		if tagged(elementType(val.Type())) {
			return e.mapValue(val, path)
		}
	}

	valTy, err := gocty.ImpliedType(val.Interface())
	if err != nil {
		return cty.NilVal, fmt.Errorf("%s: cannot encode %T as a configuration value: %w", path, val.Interface(), err)
	}

	out, err := gocty.ToCtyValue(val.Interface(), valTy)
	if err != nil {
		return cty.NilVal, fmt.Errorf("%s: cannot encode %T as %#v: %w", path, val.Interface(), valTy, err)
	}

	return out, nil
}

// objectValue builds the object written for a struct held as an attribute,
// from the fields this encoder would write
func (e *bodyEncoder) objectValue(rv reflect.Value, path string) (cty.Value, error) {
	attrs := map[string]cty.Value{}

	tags := getFieldTags(rv.Type())

	for _, f := range bodyFields(rv.Type()) {
		// An object attribute holds a copy of a value, not something written
		// in configuration. Its embedded base carries the bookkeeping every
		// entity has, depends_on, disabled and meta, which nobody wrote here,
		// so the base is left out and only the type's own fields are written.
		if f.kind == fieldRemain {
			continue
		}

		if tags.Computed[f.name] && !e.options.IncludeComputed {
			continue
		}

		fieldVal, ok := resolveValue(rv.Field(f.index))
		if !ok {
			continue
		}

		val, err := e.attributeValue(fieldVal, joinPath(path, f.name))
		if err != nil {
			return cty.NilVal, err
		}

		attrs[f.name] = val
	}

	if len(attrs) == 0 {
		return cty.EmptyObjectVal, nil
	}

	return cty.ObjectVal(attrs), nil
}

// tupleValue builds the value written for a sequence of tagged structs
func (e *bodyEncoder) tupleValue(rv reflect.Value, path string) (cty.Value, error) {
	if rv.Len() == 0 {
		return cty.EmptyTupleVal, nil
	}

	values := make([]cty.Value, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem, ok := resolveValue(rv.Index(i))
		if !ok {
			continue
		}

		val, err := e.attributeValue(elem, fmt.Sprintf("%s[%d]", path, i))
		if err != nil {
			return cty.NilVal, err
		}

		values = append(values, val)
	}

	if len(values) == 0 {
		return cty.EmptyTupleVal, nil
	}

	return cty.TupleVal(values), nil
}

// mapValue builds the value written for a map of tagged structs
func (e *bodyEncoder) mapValue(rv reflect.Value, path string) (cty.Value, error) {
	attrs := map[string]cty.Value{}

	for _, key := range rv.MapKeys() {
		elem, ok := resolveValue(rv.MapIndex(key))
		if !ok {
			continue
		}

		name := fmt.Sprintf("%v", key.Interface())

		val, err := e.attributeValue(elem, fmt.Sprintf("%s[%s]", path, name))
		if err != nil {
			return cty.NilVal, err
		}

		attrs[name] = val
	}

	if len(attrs) == 0 {
		return cty.EmptyObjectVal, nil
	}

	return cty.ObjectVal(attrs), nil
}

// tagged reports whether values of ty carry xcl field tags, which is what
// makes this encoder able to build the value itself
func tagged(ty reflect.Type) bool {
	if ty == nil {
		return false
	}

	for ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	if ty.Kind() != reflect.Struct {
		return false
	}

	tags := getFieldTags(ty)

	return len(tags.Attributes) > 0 || len(tags.Blocks) > 0 || tags.Remain != nil
}

// elementType is the type held by a slice, array or map
func elementType(ty reflect.Type) reflect.Type {
	switch ty.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return ty.Elem()
	}

	return nil
}

// encodeBlock writes a block field, one block per entry when the field holds
// a sequence of them. A field holding nothing is left out.
func (e *bodyEncoder) encodeBlock(field reflect.StructField, fieldVal reflect.Value, name, path string) error {
	fieldTy := field.Type
	if fieldTy.Kind() == reflect.Ptr {
		fieldTy = fieldTy.Elem()
	}

	elemTy := fieldTy
	isSeq := elemTy.Kind() == reflect.Slice || elemTy.Kind() == reflect.Array
	if isSeq {
		elemTy = elemTy.Elem()
	}

	// these hold an undecoded body rather than a decoded block, so there is
	// nothing to write
	if bodyType.AssignableTo(elemTy) || attrsType.AssignableTo(elemTy) {
		return nil
	}

	// each block field opens a group of its own, so the first block written
	// for it is separated from whatever came before
	e.prevWasBlock = false

	fieldVal, ok := resolveValue(fieldVal)
	if !ok {
		return nil
	}

	if !isSeq {
		return e.appendBlock(fieldVal, name, path)
	}

	for i := 0; i < fieldVal.Len(); i++ {
		elemVal, ok := resolveValue(fieldVal.Index(i))
		if !ok {
			continue
		}

		if err := e.appendBlock(elemVal, name, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}

	return nil
}

// appendBlock encodes one struct value as a block and adds it to the body
func (e *bodyEncoder) appendBlock(rv reflect.Value, name, path string) error {
	ty := rv.Type()
	if ty.Kind() != reflect.Struct {
		return fmt.Errorf("%s: cannot encode %s as a block, it is not a struct", path, ty.Kind())
	}

	block, err := encodeAsBlock(rv, ty, name, e.options, path)
	if err != nil {
		return err
	}

	if !e.prevWasBlock {
		e.dst.AppendNewline()
		e.prevWasBlock = true
	}

	e.dst.AppendBlock(block)

	return nil
}

// resolveValue follows a field to the value it holds, through a pointer and
// through an interface, so a value held in an untyped field is written by the
// type it actually has. It reports false when the field holds nothing, which
// is a nil pointer, interface, slice or map, and is left out rather than
// written as null.
func resolveValue(val reflect.Value) (reflect.Value, bool) {
	for {
		if !val.IsValid() {
			return val, false
		}

		switch val.Kind() {
		case reflect.Ptr, reflect.Interface:
			if val.IsNil() {
				return val, false
			}

			val = val.Elem()
		case reflect.Slice, reflect.Map:
			return val, !val.IsNil()
		default:
			return val, true
		}
	}
}

// joinPath names a field within the struct at path, for error messages
func joinPath(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}

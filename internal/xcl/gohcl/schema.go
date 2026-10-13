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
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/tags"
)

// ImpliedBodySchema produces a hcl.BodySchema derived from the type of the
// given value, which must be a struct value or a pointer to one. If an
// inappropriate value is passed, this function will panic.
//
// The second return argument indicates whether the given struct includes
// a "remain" field, and thus the returned schema is non-exhaustive.
//
// This uses the tags on the fields of the struct to discover how each
// field's value should be expressed within configuration. If an invalid
// mapping is attempted, this function will panic.
func ImpliedBodySchema(val interface{}) (schema *hcl.BodySchema, partial bool) {
	ty := reflect.TypeOf(val)

	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	schema, partial, err := impliedBodySchema(ty)
	if err != nil {
		panic(err.Error())
	}

	return schema, partial
}

// impliedBodySchema is ImpliedBodySchema for a type, returning the problem
// with a type that cannot be decoded instead of panicking. The decoder and
// the checker use it, so a bad Go type reaches the caller as a diagnostic.
func impliedBodySchema(ty reflect.Type) (schema *hcl.BodySchema, partial bool, err error) {
	if ty.Kind() == reflect.Ptr {
		ty = ty.Elem()
	}

	if ty.Kind() != reflect.Struct {
		return nil, false, fmt.Errorf("%s cannot be decoded from a body: it is not a struct", ty)
	}

	var attrSchemas []hcl.AttributeSchema
	var blockSchemas []hcl.BlockHeaderSchema

	tags, err := fieldTagsOf(ty)
	if err != nil {
		return nil, false, err
	}

	attrNames := make([]string, 0, len(tags.Attributes))
	for n := range tags.Attributes {
		attrNames = append(attrNames, n)
	}
	sort.Strings(attrNames)
	for _, n := range attrNames {
		idx := tags.Attributes[n]
		optional := tags.Optional[n]
		field := ty.Field(idx)

		var required bool

		switch {
		case field.Type.AssignableTo(exprType):
			// If we're decoding to hcl.Expression then absense can be
			// indicated via a null value, so we don't specify that
			// the field is required during decoding.
			required = false
		case field.Type.Kind() != reflect.Ptr && !optional:
			required = true
		default:
			required = false
		}

		attrSchemas = append(attrSchemas, hcl.AttributeSchema{
			Name:     n,
			Required: required,
		})
	}

	blockNames := make([]string, 0, len(tags.Blocks))
	for n := range tags.Blocks {
		blockNames = append(blockNames, n)
	}
	sort.Strings(blockNames)
	for _, n := range blockNames {
		idx := tags.Blocks[n]
		field := ty.Field(idx)
		fty := field.Type

		// A slice collects every block of the type; a map collects them by
		// name. Either is a collection whose blocks may be named.
		collection := false
		switch fty.Kind() {
		case reflect.Slice:
			collection = true
			fty = fty.Elem()
		case reflect.Map:
			if fty.Key().Kind() != reflect.String {
				return nil, false, blockFieldError(ty, field, "a map of blocks must have string keys")
			}
			collection = true
			fty = fty.Elem()
		}
		if fty.Kind() == reflect.Ptr {
			fty = fty.Elem()
		}
		if fty.Kind() != reflect.Struct {
			return nil, false, blockFieldError(ty, field, "struct required")
		}

		ftags, err := fieldTagsOf(fty)
		if err != nil {
			return nil, false, err
		}

		labelNames, labelsOptional := blockLabels(collection, ftags)

		blockSchemas = append(blockSchemas, hcl.BlockHeaderSchema{
			Type:           n,
			LabelNames:     labelNames,
			LabelsOptional: labelsOptional,
		})
	}

	partial = tags.Remain != nil
	schema = &hcl.BodySchema{
		Attributes: attrSchemas,
		Blocks:     blockSchemas,
	}
	return schema, partial, nil
}

// blockFieldError describes a block field whose type cannot hold blocks.
func blockFieldError(owner reflect.Type, field reflect.StructField, reason string) error {
	return fmt.Errorf(
		"field %s of %s cannot hold blocks, the block tag needs a struct, a pointer to one, or a slice or map of them, not %s: %s",
		field.Name, owner, field.Type, reason,
	)
}

// ValidateSchemaType reports the first problem that would stop configuration
// being decoded into values of t, a struct type or a pointer to one, without
// decoding anything: a malformed xcl tag, a tagged field that is unexported,
// a second remain or body field, a label field that is not a string, a block
// field that is not a struct, a pointer to one, or a slice or map (with
// string keys) of them, a remain or body field of a type that cannot hold a
// body, or an attribute whose Go type has no configuration form. Block
// element types are checked the same way, recursively. A caller registering
// a Go type runs it once, so a bad type is reported where the type is
// declared rather than as a panic while configuration is read.
func ValidateSchemaType(t reflect.Type) error {
	if t == nil {
		return fmt.Errorf("a type is required")
	}

	return validateSchemaType(t, map[reflect.Type]bool{})
}

func validateSchemaType(t reflect.Type, seen map[reflect.Type]bool) error {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return fmt.Errorf("%s cannot be decoded from a body: it is not a struct", t)
	}

	// a type that holds blocks of its own type is checked once
	if seen[t] {
		return nil
	}
	seen[t] = true

	if _, err := fieldTagsOf(t); err != nil {
		return err
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		tag, ok := field.Tag.Lookup(tags.Name)
		if !ok || tag == "" {
			continue
		}

		// the tag parsed above, so it parses here
		ft, _ := tags.Parse(tag)

		if field.PkgPath != "" {
			return fmt.Errorf("field %s of %s is unexported, so xcl cannot set it from configuration", field.Name, t)
		}

		switch ft.Kind {
		case tags.KindLabel:
			if field.Type.Kind() != reflect.String {
				return fmt.Errorf("field %s of %s cannot hold a label, a label field must be a string, not %s", field.Name, t, field.Type)
			}

		case tags.KindBlock:
			elem := field.Type
			switch elem.Kind() {
			case reflect.Slice:
				elem = elem.Elem()
			case reflect.Map:
				if elem.Key().Kind() != reflect.String {
					return blockFieldError(t, field, "a map of blocks must have string keys")
				}
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() != reflect.Struct {
				return blockFieldError(t, field, "struct required")
			}

			if err := validateSchemaType(elem, seen); err != nil {
				return err
			}

		case tags.KindRemain, tags.KindBody:
			if err := validateBodyFieldType(t, field, seen); err != nil {
				return err
			}

		case tags.KindAttr, tags.KindOptional:
			if err := validateAttributeFieldType(t, field); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateBodyFieldType checks a remain or body field, which holds the body
// itself, its attributes, a struct or map decoded from it, or a pointer to
// one of those.
func validateBodyFieldType(owner reflect.Type, field reflect.StructField, seen map[reflect.Type]bool) error {
	if bodyType.AssignableTo(field.Type) || attrsType.AssignableTo(field.Type) {
		return nil
	}

	target := field.Type
	if target.Kind() == reflect.Ptr {
		target = target.Elem()
	}

	switch target.Kind() {
	case reflect.Struct:
		return validateSchemaType(target, seen)
	case reflect.Map:
		return nil
	default:
		return fmt.Errorf(
			"field %s of %s cannot hold the rest of a body, a remain or body field must be an hcl.Body, hcl.Attributes, a struct or a map, not %s",
			field.Name, owner, field.Type,
		)
	}
}

// validateAttributeFieldType checks that an attribute field's Go type has a
// configuration form: an expression or attribute kept as it is, a cty value,
// or a type gocty can decode into.
func validateAttributeFieldType(owner reflect.Type, field reflect.StructField) error {
	if exprType.AssignableTo(field.Type) || attrType.AssignableTo(field.Type) || field.Type.AssignableTo(reflect.TypeOf(cty.Value{})) {
		return nil
	}

	// the same target DecodeExpression is given, a pointer to the field
	if _, err := gocty.ImpliedConstraintType(reflect.New(field.Type).Interface()); err != nil {
		return fmt.Errorf("field %s of %s has no configuration form: %s", field.Name, owner, err)
	}

	return nil
}

// blockLabels returns the label names a block type takes, and whether a block
// may leave them off. A block collected into a slice or a map, whose element
// has at most one label field, takes one optional name label: the element's
// label field, or "name" when it has none. A map needs the name to key its
// entries, and reports a block without one when decoding; a slice keeps the
// name when the element has a field for it and drops it otherwise. A single
// block, and one whose element has two or more label fields, must carry
// every label.
func blockLabels(collection bool, ftags *fieldTags) ([]string, bool) {
	if collection && len(ftags.Labels) <= 1 {
		name := "name"
		if len(ftags.Labels) == 1 {
			name = ftags.Labels[0].Name
		}

		return []string{name}, true
	}

	if len(ftags.Labels) == 0 {
		return nil, false
	}

	labelNames := make([]string, len(ftags.Labels))
	for i, l := range ftags.Labels {
		labelNames[i] = l.Name
	}

	return labelNames, false
}

type fieldTags struct {
	Attributes map[string]int
	Blocks     map[string]int
	Labels     []labelField
	Remain     *int
	Body       *int
	Optional   map[string]bool

	// Computed holds the names of the attribute and block fields whose tag
	// carries the computed option, meaning the field is owned by the provider
	// rather than written in configuration. The decoder does not act on it,
	// the encoder leaves such fields out unless asked for them.
	Computed map[string]bool
}

type labelField struct {
	FieldIndex int
	Name       string
}

// getFieldTags reads the xcl tags of ty's fields, and panics on a malformed
// declaration. It is used where the type has been validated; the decoder and
// the checker use fieldTagsOf.
func getFieldTags(ty reflect.Type) *fieldTags {
	ret, err := fieldTagsOf(ty)
	if err != nil {
		panic(err.Error())
	}

	return ret
}

// fieldTagsOf reads the xcl tags of ty's fields, returning the problem with a
// malformed tag or a second remain or body field.
func fieldTagsOf(ty reflect.Type) (*fieldTags, error) {
	ret := &fieldTags{
		Attributes: map[string]int{},
		Blocks:     map[string]int{},
		Optional:   map[string]bool{},
		Computed:   map[string]bool{},
	}

	ct := ty.NumField()
	for i := 0; i < ct; i++ {
		field := ty.Field(i)
		tag, ok := field.Tag.Lookup(tags.Name)
		if !ok || tag == "" {
			continue
		}

		ft, err := tags.Parse(tag)
		if err != nil {
			return nil, fmt.Errorf("field %s of %s: %s", field.Name, ty, err)
		}

		name := ft.Name
		if ft.Computed {
			ret.Computed[name] = true
		}

		switch ft.Kind {
		case tags.KindAttr:
			ret.Attributes[name] = i
		case tags.KindBlock:
			ret.Blocks[name] = i
		case tags.KindLabel:
			ret.Labels = append(ret.Labels, labelField{
				FieldIndex: i,
				Name:       name,
			})
		case tags.KindRemain:
			if ret.Remain != nil {
				return nil, fmt.Errorf("field %s of %s: only one 'remain' tag is permitted", field.Name, ty)
			}
			idx := i // copy, because this loop will continue assigning to i
			ret.Remain = &idx
		case tags.KindBody:
			if ret.Body != nil {
				return nil, fmt.Errorf("field %s of %s: only one 'body' tag is permitted", field.Name, ty)
			}
			idx := i // copy, because this loop will continue assigning to i
			ret.Body = &idx
		case tags.KindOptional:
			ret.Attributes[name] = i
			ret.Optional[name] = true
		}
	}

	return ret, nil
}

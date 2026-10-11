// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"fmt"
	"reflect"

	"github.com/jumppad-labs/xcl/internal/cty"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/cty/gocty"
	"github.com/jumppad-labs/xcl/internal/cty/convert"
)

// DecodeBody extracts the configuration within the given body into the given
// value. This value must be a non-nil pointer to either a struct or
// a map, where in the former case the configuration will be decoded using
// struct tags and in the latter case only attributes are allowed and their
// values are decoded into the map.
//
// The given EvalContext is used to resolve any variables or functions in
// expressions encountered while decoding. This may be nil to require only
// constant values, for simple applications that do not support variables or
// functions.
//
// The returned diagnostics should be inspected with its HasErrors method to
// determine if the populated value is valid and complete. If error diagnostics
// are returned then the given value may have been partially-populated but
// may still be accessed by a careful caller for static analysis and editor
// integration use-cases.
func DecodeBody(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("target value must be a pointer, not %s", rv.Type().String()))
	}

	return decodeBodyToValue(body, ctx, rv.Elem())
}

func decodeBodyToValue(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	et := val.Type()
	switch et.Kind() {
	case reflect.Struct:
		return decodeBodyToStruct(body, ctx, val)
	case reflect.Map:
		return decodeBodyToMap(body, ctx, val)
	default:
		return hcl.Diagnostics{unsupportedTargetDiagnostic(body, fmt.Errorf("%s cannot be decoded from a body: it is not a struct or a map", et))}
	}
}

// unsupportedTargetDiagnostic reports a Go type configuration cannot be
// decoded into, at the body it was to be decoded from. The type is the
// program's, not the author's, so the problem is reported rather than
// panicked, and a caller registering types can catch it earlier with
// ValidateSchemaType.
func unsupportedTargetDiagnostic(body hcl.Body, err error) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported decode target",
		Detail:   err.Error(),
		Subject:  body.MissingItemRange().Ptr(),
	}
}

func decodeBodyToStruct(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	schema, partial, err := impliedBodySchema(val.Type())
	if err != nil {
		return hcl.Diagnostics{unsupportedTargetDiagnostic(body, err)}
	}

	var content *hcl.BodyContent
	var leftovers hcl.Body
	var diags hcl.Diagnostics
	if partial {
		content, leftovers, diags = body.PartialContent(schema)
	} else {
		content, diags = body.Content(schema)
	}
	if content == nil {
		return diags
	}

	tags := getFieldTags(val.Type())

	if tags.Body != nil {
		fieldIdx := *tags.Body
		field := val.Type().Field(fieldIdx)
		fieldV := val.Field(fieldIdx)
		switch {
		case bodyType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(body))

		default:
			diags = append(diags, decodeBodyToValue(body, ctx, fieldV)...)
		}
	}

	if tags.Remain != nil {
		fieldIdx := *tags.Remain
		field := val.Type().Field(fieldIdx)
		fieldV := val.Field(fieldIdx)
		switch {
		case bodyType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(leftovers))
		case attrsType.AssignableTo(field.Type):
			attrs, attrsDiags := leftovers.JustAttributes()
			if len(attrsDiags) > 0 {
				diags = append(diags, attrsDiags...)
			}
			fieldV.Set(reflect.ValueOf(attrs))
		default:
			diags = append(diags, decodeBodyToValue(leftovers, ctx, fieldV)...)
		}
	}

	for name, fieldIdx := range tags.Attributes {
		attr := content.Attributes[name]
		field := val.Type().Field(fieldIdx)
		fieldV := val.Field(fieldIdx)

		if attr == nil {
			if !exprType.AssignableTo(field.Type) {
				continue
			}

			// As a special case, if the target is of type hcl.Expression then
			// we'll assign an actual expression that evalues to a cty null,
			// so the caller can deal with it within the cty realm rather
			// than within the Go realm.
			synthExpr := hcl.StaticExpr(cty.NullVal(cty.DynamicPseudoType), body.MissingItemRange())
			fieldV.Set(reflect.ValueOf(synthExpr))
			continue
		}

		// an unexported field cannot be set, and taking its address to
		// decode into would panic
		if field.PkgPath != "" {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Unsupported decode target",
				Detail:   fmt.Sprintf("Attribute %q: field %s of %s is unexported, so xcl cannot set it from configuration.", name, field.Name, val.Type()),
				Subject:  &attr.Range,
			})
			continue
		}

		switch {
		case attrType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(attr))
		case exprType.AssignableTo(field.Type):
			fieldV.Set(reflect.ValueOf(attr.Expr))
		default:
			diags = append(diags, nameAttributeInDiagnostics(name, DecodeExpression(
				attr.Expr, ctx, fieldV.Addr().Interface(),
			))...)
		}
	}

	blocksByType := content.Blocks.ByType()

	for typeName, fieldIdx := range tags.Blocks {
		blocks := blocksByType[typeName]
		field := val.Type().Field(fieldIdx)

		ty := field.Type
		isSlice := false
		isMap := false
		isPtr := false
		switch ty.Kind() {
		case reflect.Slice:
			isSlice = true
			ty = ty.Elem()
		case reflect.Map:
			isMap = true
			ty = ty.Elem()
		}
		if ty.Kind() == reflect.Ptr {
			isPtr = true
			ty = ty.Elem()
		}

		if len(blocks) > 1 && !isSlice && !isMap {
			diags = append(diags, &hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Duplicate %s block", typeName),
				Detail: fmt.Sprintf(
					"Only one %s block is allowed. Another was defined at %s.",
					typeName, blocks[0].DefRange.String(),
				),
				Subject: &blocks[1].DefRange,
			})
			continue
		}

		if len(blocks) == 0 {
			if isSlice || isMap || isPtr {
				if val.Field(fieldIdx).IsNil() {
					val.Field(fieldIdx).Set(reflect.Zero(field.Type))
				}
			} else {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf("Missing %s block", typeName),
					Detail:   fmt.Sprintf("A %s block is required.", typeName),
					Subject:  body.MissingItemRange().Ptr(),
				})
			}
			continue
		}

		switch {

		case isMap:
			diags = append(diags, decodeBlocksToMap(blocks, ctx, val.Field(fieldIdx), ty, isPtr, typeName)...)

		case isSlice:
			elemType := ty
			if isPtr {
				elemType = reflect.PtrTo(ty)
			}
			sli := val.Field(fieldIdx)
			if sli.IsNil() {
				sli = reflect.MakeSlice(reflect.SliceOf(elemType), len(blocks), len(blocks))
			}

			// a name is optional in a list, but two blocks cannot share one
			named := map[string]*hcl.Block{}

			for i, block := range blocks {
				if len(block.Labels) > 0 {
					if first, seen := named[block.Labels[0]]; seen {
						diags = append(diags, duplicateBlockNameDiagnostic(typeName, first, block))
					} else {
						named[block.Labels[0]] = block
					}
				}

				if isPtr {
					if i >= sli.Len() {
						sli = reflect.Append(sli, reflect.New(ty))
					}
					v := sli.Index(i)
					if v.IsNil() {
						v = reflect.New(ty)
					}
					diags = append(diags, decodeBlockToValue(block, ctx, v.Elem())...)
					sli.Index(i).Set(v)
				} else {
					if i >= sli.Len() {
						sli = reflect.Append(sli, reflect.Indirect(reflect.New(ty)))
					}
					diags = append(diags, decodeBlockToValue(block, ctx, sli.Index(i))...)
				}
			}

			if sli.Len() > len(blocks) {
				sli.SetLen(len(blocks))
			}

			val.Field(fieldIdx).Set(sli)

		default:
			block := blocks[0]
			if isPtr {
				v := val.Field(fieldIdx)
				if v.IsNil() {
					v = reflect.New(ty)
				}
				diags = append(diags, decodeBlockToValue(block, ctx, v.Elem())...)
				val.Field(fieldIdx).Set(v)
			} else {
				diags = append(diags, decodeBlockToValue(block, ctx, val.Field(fieldIdx))...)
			}

		}

	}

	return diags
}

func decodeBodyToMap(body hcl.Body, ctx *hcl.EvalContext, v reflect.Value) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if attrs == nil {
		return diags
	}

	mv := reflect.MakeMap(v.Type())

	for k, attr := range attrs {
		switch {
		case attrType.AssignableTo(v.Type().Elem()):
			mv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(attr))
		case exprType.AssignableTo(v.Type().Elem()):
			mv.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(attr.Expr))
		default:
			ev := reflect.New(v.Type().Elem())
			diags = append(diags, DecodeExpression(attr.Expr, ctx, ev.Interface())...)
			mv.SetMapIndex(reflect.ValueOf(k), ev.Elem())
		}
	}

	v.Set(mv)

	return diags
}

// decodeBlocksToMap collects blocks into the map field, keyed by each block's
// name. A block without a name, and a second block with a name already used,
// are reported at the block. The element's label field, when it has one, is
// set to the key.
func decodeBlocksToMap(blocks hcl.Blocks, ctx *hcl.EvalContext, field reflect.Value, ty reflect.Type, isPtr bool, typeName string) hcl.Diagnostics {
	var diags hcl.Diagnostics

	collected := field
	if collected.IsNil() {
		collected = reflect.MakeMap(field.Type())
	}

	named := map[string]*hcl.Block{}

	for _, block := range blocks {
		if len(block.Labels) == 0 {
			diags = append(diags, missingBlockNameDiagnostic(typeName, block))
			continue
		}

		name := block.Labels[0]
		if first, seen := named[name]; seen {
			diags = append(diags, duplicateBlockNameDiagnostic(typeName, first, block))
			continue
		}
		named[name] = block

		elem := reflect.New(ty)
		diags = append(diags, decodeBlockToValue(block, ctx, elem.Elem())...)

		if isPtr {
			collected.SetMapIndex(reflect.ValueOf(name), elem)
		} else {
			collected.SetMapIndex(reflect.ValueOf(name), elem.Elem())
		}
	}

	field.Set(collected)

	return diags
}

// missingBlockNameDiagnostic reports a block collected by name that carries
// none, at the block.
func missingBlockNameDiagnostic(typeName string, block *hcl.Block) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "A name is required",
		Detail: fmt.Sprintf(
			"%s blocks are collected by name, so each one needs a name, e.g. %s \"http\" { ... }.",
			typeName, typeName,
		),
		Subject: &block.DefRange,
	}
}

// checkBlockNames reports the name problems decoding the blocks of one type
// would find: a block without a name when named is set, as for a map of
// blocks, and a second block carrying a name already used, for maps and
// lists alike. It is shared by the decoder and CheckBody, so validation
// reports them without decoding.
func checkBlockNames(typeName string, blocks hcl.Blocks, named bool) hcl.Diagnostics {
	var diags hcl.Diagnostics
	seen := map[string]*hcl.Block{}

	for _, block := range blocks {
		if len(block.Labels) == 0 {
			if named {
				diags = append(diags, missingBlockNameDiagnostic(typeName, block))
			}
			continue
		}

		if first, dup := seen[block.Labels[0]]; dup {
			diags = append(diags, duplicateBlockNameDiagnostic(typeName, first, block))
			continue
		}
		seen[block.Labels[0]] = block
	}

	return diags
}

// duplicateBlockNameDiagnostic reports second, which carries the name first
// already took, at its name.
func duplicateBlockNameDiagnostic(typeName string, first, second *hcl.Block) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  fmt.Sprintf("Duplicate %s name", typeName),
		Detail: fmt.Sprintf(
			"A %s named %q was already defined at %s. Each %s needs its own name.",
			typeName, second.Labels[0], first.DefRange.String(), typeName,
		),
		Subject: second.LabelRanges[0].Ptr(),
	}
}

func decodeBlockToValue(block *hcl.Block, ctx *hcl.EvalContext, v reflect.Value) hcl.Diagnostics {
	diags := decodeBodyToValue(block.Body, ctx, v)

	if len(block.Labels) > 0 {
		blockTags, err := fieldTagsOf(v.Type())
		if err != nil {
			return append(diags, unsupportedTargetDiagnostic(block.Body, err))
		}

		for li, lv := range block.Labels {
			// a name accepted for an element with no label field is dropped
			if li >= len(blockTags.Labels) {
				break
			}

			lfieldIdx := blockTags.Labels[li].FieldIndex
			labelField := v.Type().Field(lfieldIdx)
			if labelField.Type.Kind() != reflect.String || labelField.PkgPath != "" {
				diags = append(diags, &hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Unsupported decode target",
					Detail:   fmt.Sprintf("Label %q: field %s of %s cannot hold a label, a label field must be an exported string, not %s.", lv, labelField.Name, v.Type(), labelField.Type),
					Subject:  block.LabelRanges[li].Ptr(),
				})
				continue
			}

			v.Field(lfieldIdx).Set(reflect.ValueOf(lv))
		}
	}

	return diags
}

// DecodeExpression extracts the value of the given expression into the given
// value. This value must be something that gocty is able to decode into,
// since the final decoding is delegated to that package.
//
// The given EvalContext is used to resolve any variables or functions in
// expressions encountered while decoding. This may be nil to require only
// constant values, for simple applications that do not support variables or
// functions.
//
// The returned diagnostics should be inspected with its HasErrors method to
// determine if the populated value is valid and complete. If error diagnostics
// are returned then the given value may have been partially-populated but
// may still be accessed by a careful caller for static analysis and editor
// integration use-cases.
func DecodeExpression(expr hcl.Expression, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	srcVal, diags := expr.Value(ctx)

	// The value is converted against a constraint rather than the plain
	// implied type, so an object or map value may leave out the attributes
	// whose fields are tagged optional, as a block may leave out their
	// attributes. The constraint fills them with null, which decoding leaves
	// at the field's zero value.
	convTy, err := gocty.ImpliedConstraintType(val)
	if err != nil {
		return append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsupported decode target",
			Detail:   fmt.Sprintf("The value cannot be decoded into %T, which has no configuration form: %s.", val, err),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
	}

	srcVal, err = convert.Convert(srcVal, convTy)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
		return diags
	}

	err = gocty.FromCtyValue(srcVal, val)
	if err != nil {
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Unsuitable value type",
			Detail:   fmt.Sprintf("Unsuitable value: %s", err.Error()),
			Subject:  expr.StartRange().Ptr(),
			Context:  expr.Range().Ptr(),
		})
	}

	return diags
}

// nameAttributeInDiagnostics prefixes the detail of each error diagnostic
// with the name of the attribute being decoded, so an unsuitable value is
// reported against the field it was assigned to.
func nameAttributeInDiagnostics(name string, diags hcl.Diagnostics) hcl.Diagnostics {
	for _, diag := range diags {
		if diag.Severity != hcl.DiagError {
			continue
		}

		diag.Detail = fmt.Sprintf("Attribute %q: %s", name, diag.Detail)
	}

	return diags
}

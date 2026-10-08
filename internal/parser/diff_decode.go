package parser

import (
	"reflect"
	"sort"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

// decodeForDiff decodes body into entity as gohcl.DecodeBody does, for a diff,
// where ctx can hold values only known once an apply has run. A Go field can
// not hold an unknown value, so every attribute is evaluated first: the path
// of each unknown inside its value is returned, and the unknown is replaced by
// a placeholder of its type, an empty string, zero, false or an empty
// collection, before decoding. Each unknown is reported on its own, so the
// rest of a value that holds one stays concrete.
//
// The decode works on a copy of the body, the parsed body is never changed.
func decodeForDiff(body *hclsyntax.Body, ctx *hcl.EvalContext, entity any) ([]diff.Path, hcl.Diagnostics) {
	unknown := []diff.Path{}

	known := knownBody(body, ctx, diff.Path{}, dereference(reflect.TypeOf(entity)), &unknown)

	return unknown, gohcl.DecodeBody(known, ctx, entity)
}

// knownBody returns a copy of body in which every attribute that evaluates to
// a value holding unknowns is replaced by that value with placeholders for
// them, recording the path of each unknown under path. entityType is the Go
// type the body decodes into, it decides how paths are written; it may be
// nil.
func knownBody(body *hclsyntax.Body, ctx *hcl.EvalContext, path diff.Path, entityType reflect.Type, unknown *[]diff.Path) *hclsyntax.Body {
	copied := *body

	copied.Attributes = make(hclsyntax.Attributes, len(body.Attributes))

	// attributes are visited in name order, so unknown paths are recorded in
	// the same order every time
	names := make([]string, 0, len(body.Attributes))
	for name := range body.Attributes {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		attribute := body.Attributes[name]
		copied.Attributes[name] = attribute

		value, diags := attribute.Expr.Value(ctx)
		if diags.HasErrors() || value.IsWhollyKnown() {
			// the decode reports the same errors, with its own context
			continue
		}

		fieldType := namedFieldType(entityType, name)
		*unknown = append(*unknown, unknownPaths(value, path.Attribute(name), fieldType)...)

		replaced := *attribute
		replaced.Expr = &hclsyntax.LiteralValueExpr{Val: withPlaceholders(value), SrcRange: attribute.Expr.Range()}
		copied.Attributes[name] = &replaced
	}

	copied.Blocks = make(hclsyntax.Blocks, 0, len(body.Blocks))
	occurrences := map[string]int{}

	for _, block := range body.Blocks {
		blockPath := path.Attribute(block.Type)
		fieldType := namedFieldType(entityType, block.Type)

		// a list of blocks is indexed by the block's position among the
		// blocks of its type
		elementType := fieldType
		if fieldType != nil && (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array) {
			blockPath = blockPath.Index(occurrences[block.Type])
			elementType = fieldType.Elem()
		}
		occurrences[block.Type]++

		copiedBlock := *block
		copiedBlock.Body = knownBody(block.Body, ctx, blockPath, dereference(elementType), unknown)
		copied.Blocks = append(copied.Blocks, &copiedBlock)
	}

	return &copied
}

// namedFieldType returns the type of the field of struct type t that
// configuration names name, or nil when there is none
func namedFieldType(t reflect.Type, name string) reflect.Type {
	t = dereference(t)
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}

	for _, f := range structFields(t) {
		if f.name == name {
			return f.field.Type
		}
	}

	return nil
}

// unknownPaths returns the path of every unknown inside value, which sits at
// path. Each unknown is reported on its own: a value holding one is split
// into its parts until it stands alone. goType is the Go type the value
// decodes into, a map field is stepped into by key and a struct by
// attribute; it may be nil, then the value's own type decides.
func unknownPaths(value cty.Value, path diff.Path, goType reflect.Type) []diff.Path {
	value, _ = value.Unmark()

	if !value.IsKnown() {
		return []diff.Path{path}
	}

	if value.IsNull() || value.IsWhollyKnown() {
		return nil
	}

	goType = dereference(goType)
	valueType := value.Type()
	found := []diff.Path{}

	switch {
	case valueType.IsObjectType() || valueType.IsMapType():
		elements := value.AsValueMap()

		keys := make([]string, 0, len(elements))
		for key := range elements {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		byKey := valueType.IsMapType()
		if goType != nil {
			byKey = goType.Kind() == reflect.Map
		}

		for _, key := range keys {
			if byKey {
				found = append(found, unknownPaths(elements[key], path.Key(key), elementGoType(goType))...)
				continue
			}

			found = append(found, unknownPaths(elements[key], path.Attribute(key), namedFieldType(goType, key))...)
		}

	case valueType.IsListType() || valueType.IsTupleType():
		for i, element := range value.AsValueSlice() {
			found = append(found, unknownPaths(element, path.Index(i), elementGoType(goType))...)
		}

	default:
		// a set's elements have no position, the whole set is unknown
		found = append(found, path)
	}

	return found
}

// elementGoType returns the element type of a Go list or map type, or nil
func elementGoType(t reflect.Type) reflect.Type {
	if t == nil {
		return nil
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return t.Elem()
	}

	return nil
}

// withPlaceholders returns value with every unknown inside it replaced by a
// placeholder of its type, keeping the marks of every value
func withPlaceholders(value cty.Value) cty.Value {
	unmarked, marks := value.Unmark()

	if !unmarked.IsKnown() {
		return placeholder(unmarked.Type()).WithMarks(marks)
	}

	if unmarked.IsNull() || unmarked.IsWhollyKnown() {
		return value
	}

	valueType := unmarked.Type()

	switch {
	case valueType.IsObjectType():
		attributes := unmarked.AsValueMap()
		for name, attribute := range attributes {
			attributes[name] = withPlaceholders(attribute)
		}

		return cty.ObjectVal(attributes).WithMarks(marks)

	case isElementCollection(valueType):
		return mapElements(unmarked, withPlaceholders).WithMarks(marks)
	}

	return value
}

// placeholder returns the known value decoded in place of an unknown of type
// t: an empty string, zero, false, an empty collection, an object or tuple of
// placeholders, or null when the type itself is not known
func placeholder(t cty.Type) cty.Value {
	switch {
	case t == cty.String:
		return cty.StringVal("")
	case t == cty.Number:
		return cty.Zero
	case t == cty.Bool:
		return cty.False
	case t.IsListType():
		return cty.ListValEmpty(t.ElementType())
	case t.IsSetType():
		return cty.SetValEmpty(t.ElementType())
	case t.IsMapType():
		return cty.MapValEmpty(t.ElementType())
	case t.IsObjectType():
		attributes := map[string]cty.Value{}
		for name, attributeType := range t.AttributeTypes() {
			attributes[name] = placeholder(attributeType)
		}

		return cty.ObjectVal(attributes)
	case t.IsTupleType():
		elements := []cty.Value{}
		for _, elementType := range t.TupleElementTypes() {
			elements = append(elements, placeholder(elementType))
		}

		return cty.TupleVal(elements)
	}

	return cty.NullVal(t)
}

// withSavedValues sets every value of entity at the given unknown paths to the
// value saved holds at the same path. A resource whose configured values are
// only known once an apply has run is still read and asked whether it
// changed: its provider sees what it had at those paths rather than
// placeholders. A path saved does not reach, such as a list element that did
// not exist before, keeps its placeholder. saved is a resource of the same Go
// type as entity.
func withSavedValues(entity any, saved any, paths []diff.Path) {
	for _, path := range paths {
		setSavedValue(reflect.ValueOf(entity), reflect.ValueOf(saved), path)
	}
}

// setSavedValue sets the value at path inside target to the value at the same
// path inside saved. target must be settable once dereferenced.
func setSavedValue(target, saved reflect.Value, path diff.Path) {
	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			return
		}
		target = target.Elem()
	}

	for saved.Kind() == reflect.Pointer {
		if saved.IsNil() {
			return
		}
		saved = saved.Elem()
	}

	if !saved.IsValid() || !target.CanSet() {
		return
	}

	if len(path) == 0 {
		if saved.Type().AssignableTo(target.Type()) {
			target.Set(saved)
		}
		return
	}

	step := path[0]
	switch step.Kind {
	case diff.StepAttribute:
		if target.Kind() != reflect.Struct || saved.Type() != target.Type() {
			return
		}

		for _, f := range structFields(target.Type()) {
			if f.name == step.Attribute {
				setSavedValue(target.FieldByIndex(f.index), saved.FieldByIndex(f.index), path[1:])
				return
			}
		}

	case diff.StepIndex:
		if target.Kind() != reflect.Slice && target.Kind() != reflect.Array {
			return
		}

		if step.Index >= target.Len() || step.Index >= saved.Len() {
			return
		}

		setSavedValue(target.Index(step.Index), saved.Index(step.Index), path[1:])

	case diff.StepKey:
		if target.Kind() != reflect.Map || target.IsNil() || target.Type().Key().Kind() != reflect.String {
			return
		}

		key := reflect.ValueOf(step.Key).Convert(target.Type().Key())

		savedElement := saved.MapIndex(key)
		if !savedElement.IsValid() {
			return
		}

		// map elements can not be set in place, the element is copied, set
		// and stored back
		element := reflect.New(target.Type().Elem()).Elem()
		if current := target.MapIndex(key); current.IsValid() {
			element.Set(current)
		}

		setSavedValue(element, savedElement, path[1:])
		target.SetMapIndex(key, element)
	}
}

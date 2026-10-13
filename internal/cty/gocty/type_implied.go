package gocty

import (
	"reflect"

	"github.com/jumppad-labs/xcl/internal/cty"
)

// ImpliedType takes an arbitrary Go value (as an interface{}) and attempts
// to find a suitable cty.Type instance that could be used for a conversion
// with ToCtyValue.
//
// This allows -- for simple situations at least -- types to be defined just
// once in Go and the cty types derived from the Go types, but in the process
// it makes some assumptions that may be undesirable so applications are
// encouraged to build their cty types directly if exacting control is
// required.
//
// Not all Go types can be represented as cty types, so an error may be
// returned which is usually considered to be a bug in the calling program.
// In particular, ImpliedType will never use capsule types in its returned
// type, because it cannot know the capsule types supported by the calling
// program.
func ImpliedType(gv interface{}) (cty.Type, error) {
	rt := reflect.TypeOf(gv)
	var path cty.Path
	return impliedType(rt, path, false)
}

// ImpliedConstraintType is ImpliedType for use as a conversion constraint: a
// struct becomes an object type whose attributes tagged optional are optional,
// so a value converted against it may leave them out and gets null in their
// place. A type with optional attributes is only valid as a constraint, never
// as the type of a value, so ToCtyValue takes ImpliedType instead.
func ImpliedConstraintType(gv interface{}) (cty.Type, error) {
	rt := reflect.TypeOf(gv)
	var path cty.Path
	return impliedType(rt, path, true)
}

// impliedType returns the cty type for rt. When constraint is set, struct
// attributes tagged optional are marked optional in the object type.
func impliedType(rt reflect.Type, path cty.Path, constraint bool) (cty.Type, error) {
	// A registered wrapper type converts as its inner type.
	if _, inner, ok := wrapperFor(rt); ok {
		return impliedType(inner, path, constraint)
	}

	switch rt.Kind() {

	case reflect.Ptr:
		return impliedType(rt.Elem(), path, constraint)

	// Primitive types
	case reflect.Bool:
		return cty.Bool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cty.Number, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cty.Number, nil
	case reflect.Float32, reflect.Float64:
		return cty.Number, nil
	case reflect.String:
		return cty.String, nil

	// Collection types
	case reflect.Slice:
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.Number)})
		ety, err := impliedType(rt.Elem(), path, constraint)
		if err != nil {
			return cty.NilType, err
		}
		return cty.List(ety), nil
	case reflect.Map:
		if !stringType.AssignableTo(rt.Key()) {
			return cty.NilType, path.NewErrorf("no cty.Type for %s (must have string keys)", rt)
		}
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.String)})
		ety, err := impliedType(rt.Elem(), path, constraint)
		if err != nil {
			return cty.NilType, err
		}
		return cty.Map(ety), nil

	// Structural types
	case reflect.Struct:
		return impliedStructType(rt, path, constraint)

	default:
		return cty.NilType, path.NewErrorf("no cty.Type for %s", rt)
	}
}

func impliedStructType(rt reflect.Type, path cty.Path, constraint bool) (cty.Type, error) {
	if valueType.AssignableTo(rt) {
		// Special case: cty.Value represents cty.DynamicPseudoType, for
		// type conformance checking.
		return cty.DynamicPseudoType, nil
	}

	fieldIdxs := structTagIndices(rt)
	if len(fieldIdxs) == 0 {
		return cty.NilType, path.NewErrorf("no cty.Type for %s (no xcl field tags)", rt)
	}

	atys := make(map[string]cty.Type, len(fieldIdxs))

	// optional collects, for a constraint, the attributes whose field is
	// tagged optional
	var optional []string

	{
		// Temporary extension of path for attributes
		path := append(path, nil)

		for k, fi := range fieldIdxs {
			path[len(path)-1] = cty.GetAttrStep{Name: k}

			field := rt.Field(fi)
			aty, err := impliedType(field.Type, path, constraint)
			if err != nil {
				return cty.NilType, err
			}

			atys[k] = aty

			if constraint && optionalAttribute(field) {
				optional = append(optional, k)
			}
		}

		// an untagged field registered to receive an attribute is a target
		// of conversion only, so it is part of a constraint, optionally, and
		// never of the type of a value
		if constraint {
			for k, fi := range untaggedFieldsOf(rt) {
				if _, tagged := atys[k]; tagged {
					continue
				}

				path[len(path)-1] = cty.GetAttrStep{Name: k}

				aty, err := impliedType(rt.Field(fi).Type, path, constraint)
				if err != nil {
					return cty.NilType, err
				}

				atys[k] = aty
				optional = append(optional, k)
			}
		}
	}

	// now check any anonymous fields
	ct := rt.NumField()
	for i := 0; i < ct; i++ {
		field := rt.Field(i)
		if field.Anonymous {
			anon, err := impliedStructType(field.Type, path, constraint)
			if err != nil {
				return cty.NilType, err
			}

			for k, v := range anon.AttributeTypes() {
				atys[k] = v
			}

			for k := range anon.OptionalAttributes() {
				optional = append(optional, k)
			}
		}
	}

	if len(optional) > 0 {
		return cty.ObjectWithOptionalAttrs(atys, optional), nil
	}

	return cty.Object(atys), nil
}

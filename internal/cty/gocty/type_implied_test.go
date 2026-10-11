package gocty

import (
	"fmt"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
)

func TestImpliedType(t *testing.T) {
	tests := []struct {
		Input interface{}
		Want  cty.Type
	}{
		// Primitive types
		{
			int(0),
			cty.Number,
		},
		{
			int8(0),
			cty.Number,
		},
		{
			int16(0),
			cty.Number,
		},
		{
			int32(0),
			cty.Number,
		},
		{
			int64(0),
			cty.Number,
		},
		{
			uint(0),
			cty.Number,
		},
		{
			uint8(0),
			cty.Number,
		},
		{
			uint16(0),
			cty.Number,
		},
		{
			uint32(0),
			cty.Number,
		},
		{
			uint64(0),
			cty.Number,
		},
		{
			float32(0),
			cty.Number,
		},
		{
			float64(0),
			cty.Number,
		},
		{
			false,
			cty.Bool,
		},
		{
			"",
			cty.String,
		},

		// Collection types
		{
			[]int(nil),
			cty.List(cty.Number),
		},
		{
			[][]int(nil),
			cty.List(cty.List(cty.Number)),
		},
		{
			map[string]int(nil),
			cty.Map(cty.Number),
		},
		{
			map[string]map[string]int(nil),
			cty.Map(cty.Map(cty.Number)),
		},
		{
			map[string][]int(nil),
			cty.Map(cty.List(cty.Number)),
		},

		// Structs
		{
			testStruct{},
			cty.Object(map[string]cty.Type{
				"name":   cty.String,
				"number": cty.Number,
			}),
		},

		// Pointers (unwrapped and ignored)
		{
			ptrToInt(0),
			cty.Number,
		},
		{
			ptrToBool(false),
			cty.Bool,
		},
		{
			ptrToString(""),
			cty.String,
		},
		{
			&testStruct{},
			cty.Object(map[string]cty.Type{
				"name":   cty.String,
				"number": cty.Number,
			}),
		},

		// Dynamic
		{
			cty.NilVal,
			cty.DynamicPseudoType,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%#v", test.Input), func(t *testing.T) {
			got, err := ImpliedType(test.Input)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if !got.Equals(test.Want) {
				t.Fatalf(
					"wrong result\ninput: %#v\ngot:   %#v\nwant:  %#v",
					test.Input, got, test.Want,
				)
			}
		})
	}
}

func TestImpliedTypeKeepsOptionalAttributesRequired(t *testing.T) {
	ty, err := ImpliedType(optionalEndpoint{})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !ty.IsObjectType() {
		t.Fatalf("wrong type %#v; want an object type", ty)
	}

	if len(ty.OptionalAttributes()) != 0 {
		t.Errorf("ImpliedType marked attributes optional: %#v; want none", ty.OptionalAttributes())
	}

	want := cty.Object(map[string]cty.Type{
		"port": cty.Number,
		"tls":  cty.Bool,
	})
	if !ty.Equals(want) {
		t.Errorf("wrong type %#v; want %#v", ty, want)
	}
}

func TestImpliedConstraintTypeMarksOptionalAttributes(t *testing.T) {
	ty, err := ImpliedConstraintType(optionalEndpoint{})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !ty.IsObjectType() {
		t.Fatalf("wrong type %#v; want an object type", ty)
	}

	atys := ty.AttributeTypes()
	if _, ok := atys["port"]; !ok {
		t.Errorf("attribute types %#v are missing port", atys)
	}
	if _, ok := atys["tls"]; !ok {
		t.Errorf("attribute types %#v are missing tls", atys)
	}

	optional := ty.OptionalAttributes()
	if _, ok := optional["tls"]; !ok {
		t.Errorf("optional attributes %#v are missing tls", optional)
	}
	if _, ok := optional["port"]; ok {
		t.Errorf("optional attributes %#v wrongly include port", optional)
	}
}

func TestImpliedConstraintTypeMarksOptionalAttributesInsideMap(t *testing.T) {
	ty, err := ImpliedConstraintType(map[string]optionalEndpoint{})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if !ty.IsMapType() {
		t.Fatalf("wrong type %#v; want a map type", ty)
	}

	ety := ty.ElementType()
	if !ety.IsObjectType() {
		t.Fatalf("wrong element type %#v; want an object type", ety)
	}

	optional := ety.OptionalAttributes()
	if _, ok := optional["tls"]; !ok {
		t.Errorf("element optional attributes %#v are missing tls", optional)
	}
	if _, ok := optional["port"]; ok {
		t.Errorf("element optional attributes %#v wrongly include port", optional)
	}
}

package registry

import (
	"fmt"
	"reflect"

	"github.com/jumppad-labs/xcl/internal/schema"
)

// TypeFromSchema declares a config-only block type from a schema v1
// document, the language-neutral description of a block type that a host
// written in another language, or a program with no Go type of its own,
// writes. The document names the type and optional subtype and lists the
// fields: attributes, nested blocks, lists of blocks and maps of named
// blocks, each optional, sensitive, computed or defaulted as the document
// says. The returned Type's Prototype is a Go struct built from it, which a
// registry declares exactly as it declares a hand-written type:
//
//	t, err := registry.TypeFromSchema(doc)
//	local.RegisterType(t.Prototype, t.Type, t.Subtype)
//
// A sensitive field may hold a string, number, bool, list of strings or map
// of strings; any other shape, and an unknown type, kind or format, is
// returned as an error naming the field.
func TypeFromSchema(doc []byte) (Type, error) {
	built, parsed, err := schema.BuildTypeV1(doc)
	if err != nil {
		return Type{}, fmt.Errorf("schema: %w", err)
	}

	return Type{
		Type:      parsed.Type,
		Subtype:   parsed.Subtype,
		Prototype: reflect.New(built).Interface(),
	}, nil
}

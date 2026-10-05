package schema

import (
	"reflect"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/types"
)

// sensitiveTypePrefix starts the schema type name of every types.Sensitive
// instantiation.
const sensitiveTypePrefix = "types.Sensitive["

// KnownTypes returns the Go types the host rebuilds plugin types with, keyed
// by the type name a plugin's schema records for them. The plugin registry
// and the plugin test helpers both use it, so a plugin type is rebuilt the
// same way in tests as in real use.
//
// Go cannot instantiate a generic type at run time, so every types.Sensitive
// instantiation a plugin type may use is listed here. A plugin type using any
// other instantiation fails to load.
func KnownTypes() map[string]reflect.Type {
	known := map[string]reflect.Type{
		"types.Meta":         reflect.TypeOf(types.Meta{}),
		"types.ResourceBase": reflect.TypeOf(types.ResourceBase{}),
		"cty.Value":          reflect.TypeOf(cty.Value{}),
	}

	for _, sensitiveType := range []reflect.Type{
		reflect.TypeOf(types.Sensitive[string]{}),
		reflect.TypeOf(types.Sensitive[int]{}),
		reflect.TypeOf(types.Sensitive[int64]{}),
		reflect.TypeOf(types.Sensitive[float64]{}),
		reflect.TypeOf(types.Sensitive[bool]{}),
		reflect.TypeOf(types.Sensitive[[]string]{}),
		reflect.TypeOf(types.Sensitive[map[string]string]{}),
	} {
		known[sensitiveType.String()] = sensitiveType
	}

	return known
}

var sensitiveValueType = reflect.TypeOf((*types.SensitiveValue)(nil)).Elem()

// isSensitiveType reports whether t is a types.Sensitive instantiation.
func isSensitiveType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Implements(sensitiveValueType)
}

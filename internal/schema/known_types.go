package schema

import (
	"reflect"

	"github.com/jumppad-labs/xcl/types"
)

// sensitiveTypePrefix starts the schema type name of every types.Sensitive
// instantiation.
const sensitiveTypePrefix = "types.Sensitive["

// sensitiveTypes are the types.Sensitive instantiations a schema's sensitive
// fields are built as, keyed by type name. Go cannot instantiate a generic
// type at run time, so a sensitive value of any other type is refused.
var sensitiveTypes = func() map[string]reflect.Type {
	known := map[string]reflect.Type{}

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
}()

var sensitiveValueType = reflect.TypeOf((*types.SensitiveValue)(nil)).Elem()

// isSensitiveType reports whether t is a types.Sensitive instantiation.
func isSensitiveType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Implements(sensitiveValueType)
}

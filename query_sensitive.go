package xcl

import (
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/types"
)

var (
	querySensitiveValueType = reflect.TypeOf((*types.SensitiveValue)(nil)).Elem()
	queryCtyValueType       = reflect.TypeOf(cty.Value{})
)

// sensitiveFieldBlocked reports the first field where a conversion from one
// Go type to another would copy a sensitive value into a field that is not
// declared sensitive. Fields are matched by their JSON names, as the copy
// matches them, and the walk follows nested structs, slices and maps. A target
// typed any or cty.Value holds whatever it is given, so it never blocks.
//
// field is the dotted path of the blocking field.
func sensitiveFieldBlocked(from, to reflect.Type) (field string, blocked bool) {
	return walkSensitiveFields(from, to, "", map[[2]reflect.Type]bool{})
}

func walkSensitiveFields(from, to reflect.Type, path string, visited map[[2]reflect.Type]bool) (string, bool) {
	from = dereferenceType(from)
	to = dereferenceType(to)

	if from == nil || to == nil {
		return "", false
	}

	if isSensitiveGoType(from) {
		if to.Kind() == reflect.Interface || to == queryCtyValueType || isSensitiveGoType(to) {
			return "", false
		}

		return path, true
	}

	if to.Kind() == reflect.Interface || to == queryCtyValueType {
		return "", false
	}

	pair := [2]reflect.Type{from, to}
	if visited[pair] {
		return "", false
	}
	visited[pair] = true

	switch from.Kind() {
	case reflect.Struct:
		if to.Kind() != reflect.Struct || isSensitiveGoType(to) {
			return "", false
		}

		targetFields := jsonFieldTypes(to)
		for name, fromType := range jsonFieldTypesInOrder(from) {
			toType, ok := targetFields[name]
			if !ok {
				continue
			}

			if field, blocked := walkSensitiveFields(fromType, toType, joinFieldPath(path, name), visited); blocked {
				return field, true
			}
		}

	case reflect.Slice, reflect.Array:
		if to.Kind() == reflect.Slice || to.Kind() == reflect.Array {
			return walkSensitiveFields(from.Elem(), to.Elem(), path, visited)
		}

	case reflect.Map:
		if to.Kind() == reflect.Map {
			return walkSensitiveFields(from.Elem(), to.Elem(), path, visited)
		}
	}

	return "", false
}

// jsonField is a struct field the JSON copy writes, by name.
type jsonField struct {
	name      string
	fieldType reflect.Type
}

// jsonFieldTypesInOrder returns the fields of a struct the JSON copy writes,
// in declaration order. Untagged embedded structs are flattened into the
// parent; an outer field hides an embedded one of the same name.
func jsonFieldTypesInOrder(structType reflect.Type) func(yield func(string, reflect.Type) bool) {
	fields := collectJSONFields(structType, map[reflect.Type]bool{})

	return func(yield func(string, reflect.Type) bool) {
		seen := map[string]bool{}
		for _, field := range fields {
			if seen[field.name] {
				continue
			}
			seen[field.name] = true

			if !yield(field.name, field.fieldType) {
				return
			}
		}
	}
}

// jsonFieldTypes returns the fields of a struct the JSON copy writes, by name.
func jsonFieldTypes(structType reflect.Type) map[string]reflect.Type {
	byName := map[string]reflect.Type{}
	for name, fieldType := range jsonFieldTypesInOrder(structType) {
		byName[name] = fieldType
	}

	return byName
}

func collectJSONFields(structType reflect.Type, visited map[reflect.Type]bool) []jsonField {
	if visited[structType] {
		return nil
	}
	visited[structType] = true

	own := []jsonField{}
	embedded := []jsonField{}

	for i := 0; i < structType.NumField(); i++ {
		structField := structType.Field(i)

		tag := structField.Tag.Get("json")
		if tag == "-" {
			continue
		}

		name, _, _ := strings.Cut(tag, ",")

		if structField.Anonymous && name == "" {
			embeddedType := dereferenceType(structField.Type)
			if embeddedType.Kind() == reflect.Struct && !isSensitiveGoType(embeddedType) {
				embedded = append(embedded, collectJSONFields(embeddedType, visited)...)
				continue
			}
		}

		if !structField.IsExported() {
			continue
		}

		if name == "" {
			name = structField.Name
		}

		own = append(own, jsonField{name: name, fieldType: structField.Type})
	}

	return append(own, embedded...)
}

func isSensitiveGoType(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && t.Implements(querySensitiveValueType)
}

func dereferenceType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

func joinFieldPath(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}

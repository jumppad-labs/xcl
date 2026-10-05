package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// CreateInstanceFromSchema takes a JSON schema generally created using GenerateFromInstance
// and dynamically creates a struct.
// This is a generic function that can create any struct type.
// typeMapping allows mapping type names to actual reflect.Types for proper type creation
func CreateInstanceFromSchema(data []byte, typeMapping map[string]reflect.Type) (any, error) {
	e := &Attribute{}
	err := json.Unmarshal(data, e)
	if err != nil {
		return nil, err
	}

	result, err := parseAttribute(e, typeMapping)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type PropertyType struct {
	OuterPointer bool
	Struct       bool
	Slice        bool
	Map          bool
	MapKey       string
	InnerPointer bool
	Type         string
}

// parseAttribute always returns a pointer to the struct
func parseAttribute(attribute *Attribute, typeMapping map[string]reflect.Type) (any, error) {
	// Start with embedded ResourceBase
	fields := []reflect.StructField{}

	for _, a := range attribute.Properties {
		if a.Anonymous {
			// Extract the field name from the type (e.g., "types.ResourceBase" -> "ResourceBase")
			fieldName := extractTypeName(a.Type)

			// Skip anonymous fields with unexported names since they can't have PkgPath set
			// and would cause reflection errors
			if len(fieldName) > 0 && fieldName[0] >= 'a' && fieldName[0] <= 'z' {
				continue
			}

			t, err := parseType(a.Type)
			if err != nil {
				return nil, err
			}

			embeddedType, err := parseInnerType(t, a, typeMapping)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", fieldName, err)
			}

			field := reflect.StructField{
				Name:      fieldName, // Anonymous fields need the type name
				Type:      embeddedType,
				Tag:       reflect.StructTag(a.Tags),
				Anonymous: true,
			}

			fields = append(fields, field)
			continue
		}

		// Skip regular fields with unexported names since they can't be set via reflection
		if len(a.Name) > 0 && a.Name[0] >= 'a' && a.Name[0] <= 'z' {
			continue
		}

		t, err := parseType(a.Type)
		if err != nil {
			return nil, err
		}

		if t.Slice {
			innerType, err := parseInnerType(t, a, typeMapping)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", a.Name, err)
			}
			sliceType := reflect.SliceOf(innerType)

			if t.OuterPointer {
				sliceType = reflect.PointerTo(sliceType)
			}

			nf := reflect.StructField{
				Name: a.Name,
				Type: sliceType,
				Tag:  reflect.StructTag(a.Tags),
			}

			fields = append(fields, nf)

		} else if t.Map {
			innerType, err := parseInnerType(t, a, typeMapping)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", a.Name, err)
			}

			keyType := reflect.TypeOf(t.MapKey)
			mapType := reflect.MapOf(keyType, innerType)

			if t.OuterPointer {
				mapType = reflect.PointerTo(mapType)
			}

			field := reflect.StructField{
				Name: a.Name,
				Type: mapType,
				Tag:  reflect.StructTag(a.Tags),
			}

			fields = append(fields, field)
		} else {
			innerType, err := parseInnerType(t, a, typeMapping)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", a.Name, err)
			}

			field := reflect.StructField{
				Name: a.Name,
				Type: innerType,
				Tag:  reflect.StructTag(a.Tags),
			}

			fields = append(fields, field)
		}
	}

	structType := reflect.StructOf(fields)
	instance := reflect.New(structType)

	// Return the struct directly - it implements Resource through embedded ResourceBase
	return instance.Interface(), nil
}

func parseType(t string) (*PropertyType, error) {
	if t == "" {
		return nil, fmt.Errorf("unable to parse type: %s", t)
	}

	tp := PropertyType{}
	rest := t

	if strings.HasPrefix(rest, "*") {
		tp.OuterPointer = true
		rest = rest[1:]
	}

	switch {
	case strings.HasPrefix(rest, "[]"):
		tp.Slice = true
		rest = rest[2:]

	case strings.HasPrefix(rest, "map["):
		// the key ends at the bracket that closes "map[", which a scan of the
		// bracket depth finds even when the key or value is generic, such as
		// map[string]types.Sensitive[string]
		closing := matchingBracket(rest, len("map"))
		if closing < 0 {
			return nil, fmt.Errorf("unable to parse type: %s", t)
		}

		tp.Map = true
		tp.MapKey = rest[len("map["):closing]
		rest = rest[closing+1:]
	}

	if strings.HasPrefix(rest, "*") {
		tp.InnerPointer = true
		rest = rest[1:]
	}

	if rest == "" {
		return nil, fmt.Errorf("unable to parse type: %s", t)
	}

	tp.Type = rest

	return &tp, nil
}

// matchingBracket returns the index of the bracket that closes the one at
// open, counting nested brackets, or -1 when it is not closed.
func matchingBracket(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func parseInnerType(t *PropertyType, a *Attribute, typeMapping map[string]reflect.Type) (reflect.Type, error) {
	var innerType reflect.Type

	switch t.Type {
	case "string":
		innerType = reflect.TypeOf("")
	case "bool":
		innerType = reflect.TypeOf(true)
	case "byte":
		innerType = reflect.TypeOf(byte(0))
	case "rune":
		innerType = reflect.TypeOf(rune(0))
	case "int":
		innerType = reflect.TypeOf(0)
	case "int64":
		innerType = reflect.TypeOf(int64(0))
	case "int32":
		innerType = reflect.TypeOf(int32(0))
	case "uint":
		innerType = reflect.TypeOf(uint(0))
	case "uint64":
		innerType = reflect.TypeOf(uint64(0))
	case "uint32":
		innerType = reflect.TypeOf(uint32(0))
	case "uintptr":
		innerType = reflect.TypeOf(uintptr(0))
	case "float":
		innerType = reflect.TypeOf(0.0)
	case "float64":
		innerType = reflect.TypeOf(float64(0.0))
	case "float32":
		innerType = reflect.TypeOf(float32(0.0))
	case "complex64":
		innerType = reflect.TypeOf(complex64(0))
	case "complex128":
		innerType = reflect.TypeOf(complex128(0))
	default:
		// Check type mapping first
		if typeMapping != nil {
			if mappedType, exists := typeMapping[t.Type]; exists {
				innerType = mappedType
				break
			}
		}

		// a sensitive instantiation can not be built at run time, so one the
		// host does not know fails rather than silently losing its value
		if strings.HasPrefix(t.Type, sensitiveTypePrefix) {
			return nil, fmt.Errorf("unsupported sensitive type %s, supported sensitive types are types.Sensitive[string], [int], [int64], [float64], [bool], [[]string] and [map[string]string]", t.Type)
		}

		// Handle interface{} and other unrecognized types
		if strings.Contains(t.Type, "interface") {
			innerType = reflect.TypeOf((*interface{})(nil)).Elem()
		} else {
			// For any other unrecognized type, use interface{} as fallback
			innerType = reflect.TypeOf((*interface{})(nil)).Elem()
		}
	}

	// if the property is a pointer, we need to wrap the inner type in a pointer
	if innerType != nil && (t.OuterPointer || t.InnerPointer) {
		innerType = reflect.PointerTo(innerType)
	}

	// if there are properties, we need to create a struct
	if a.Properties != nil {
		// Check if we have a type mapping that should take precedence
		hasTypeMapping := false
		if typeMapping != nil {
			if _, exists := typeMapping[t.Type]; exists {
				hasTypeMapping = true
			}
		}

		// Only create anonymous struct if we don't have a type mapping
		// Type mappings should take precedence over property-based struct creation
		if !hasTypeMapping {
			se, err := parseAttribute(a, typeMapping)
			if err != nil {
				return nil, err
			}

			innerType = reflect.TypeOf(se)

			// parse attribute always returns a pointer to the struct
			// if the inner type is not a pointer, we need to dereference it
			if !t.InnerPointer && !t.OuterPointer {
				innerType = reflect.TypeOf(se).Elem()
			}
		}
	}

	return innerType, nil
}

// extractTypeName extracts the type name from a full type string
// e.g., "types.ResourceBase" -> "ResourceBase", "MyStruct" -> "MyStruct"
func extractTypeName(typeStr string) string {
	// Handle qualified types like "package.Type"
	parts := strings.Split(typeStr, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1] // Return the last part (type name)
	}
	return typeStr // Return as-is if no package qualifier
}

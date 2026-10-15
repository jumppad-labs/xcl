package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/types"
)

// SchemaVersion1 is the schema_version of the language-neutral block type
// schema, the form a host written in another language declares a type in.
const SchemaVersion1 = 1

// SchemaV1 is a language-neutral description of a block type: the type and
// subtype configuration declares it with, and the fields a block of it holds.
// Any language can write one; the host builds the Go type the parser decodes
// into from it with BuildTypeV1.
//
// ResourceBase (depends_on, disabled, meta) is never listed. Every block type
// has it and BuildTypeV1 adds it.
type SchemaV1 struct {
	SchemaVersion int    `json:"schema_version"`
	Type          string `json:"type"`
	Subtype       string `json:"subtype,omitempty"`

	// Singleton marks a type whose block is written without labels, i.e.
	// runner {}, of which a module holds one. A singleton has no subtype
	Singleton bool `json:"singleton,omitempty"`

	Fields []FieldV1 `json:"fields"`
}

// FieldV1 is one field of a SchemaV1 block type.
type FieldV1 struct {
	// Name is the name configuration uses for the field, in snake_case
	Name string `json:"name"`

	// Type is the value's type: "string", "number", "bool", "any",
	// {"list": T}, {"map": T}, {"object": [fields]}, or {"entity": [fields]}
	// for a value holding a block, as a reference to one gives it, which
	// carries meta, disabled and depends_on beside its fields
	Type json.RawMessage `json:"type"`

	// Kind is "attribute", the default, or "block"
	Kind string `json:"kind,omitempty"`

	// Repeated marks a block that may appear more than once, which becomes a
	// list of blocks
	Repeated bool `json:"repeated,omitempty"`

	// Optional marks a field that may be left out
	Optional bool `json:"optional,omitempty"`

	// Sensitive marks a value that is a secret
	Sensitive bool `json:"sensitive,omitempty"`

	// Computed marks a field owned by the provider, which configuration may
	// not set; it implies Optional
	Computed bool `json:"computed,omitempty"`

	// Default is the JSON value an optional attribute takes when left out
	Default json.RawMessage `json:"default,omitempty"`

	// Format is a hint on a number: "int", the default, or "float"
	Format string `json:"format,omitempty"`

	// Label names the field of a block's element that holds the block's
	// name: optional on a list of blocks, and filled from the key in a map
	// of blocks
	Label string `json:"label,omitempty"`

	// Key marks an attribute that identifies an element in a list of blocks:
	// the saved and configured elements are paired by it when computed values
	// are carried over, rather than by position
	Key bool `json:"key,omitempty"`
}

// Field kinds.
const (
	FieldKindAttribute = "attribute"
	FieldKindBlock     = "block"
)

// Number formats.
const (
	NumberFormatInt   = "int"
	NumberFormatFloat = "float"
)

// IsV1 reports whether data is a schema v1 document, which is what carries
// "schema_version": 1; the legacy schema has no version.
func IsV1(data []byte) bool {
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}

	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}

	return header.SchemaVersion == SchemaVersion1
}

// ParseV1 decodes a schema v1 document without building a type.
func ParseV1(data []byte) (*SchemaV1, error) {
	s := &SchemaV1{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("schema is not valid JSON: %w", err)
	}

	if s.SchemaVersion != SchemaVersion1 {
		return nil, fmt.Errorf("unsupported schema_version %d, this host reads schema version %d", s.SchemaVersion, SchemaVersion1)
	}

	if s.Type == "" {
		return nil, fmt.Errorf("schema names no type")
	}

	if s.Singleton && s.Subtype != "" {
		return nil, fmt.Errorf("schema for type %s: a singleton takes no subtype", types.TypeKey(s.Type, s.Subtype))
	}

	return s, nil
}

// BuildTypeV1 builds the Go struct type a block of the schema's type is
// decoded into. The type embeds types.ResourceBase first and carries the
// xcl, json and default tags the parser, decoder, change detection and
// computed carry-over read, so they run on it with no knowledge that it came
// from a schema.
//
// A sensitive field is limited to the instantiations in KnownTypes; any other
// shape, and an unknown type, kind or format, is an error naming the field.
func BuildTypeV1(data []byte) (reflect.Type, *SchemaV1, error) {
	s, err := ParseV1(data)
	if err != nil {
		return nil, nil, err
	}

	fields, err := buildFieldsV1(s.Fields, "", true)
	if err != nil {
		return nil, nil, fmt.Errorf("schema for type %s: %w", types.TypeKey(s.Type, s.Subtype), err)
	}

	if s.Singleton {
		fields = append(fields, singletonField)
	}

	return reflect.StructOf(fields), s, nil
}

// SingletonField is the name of the field that marks a type built from a
// schema as a singleton. It is a zero length array: it takes no space, has
// no xcl tag so configuration never sees it, and is left out of JSON.
const SingletonField = "XclSingleton"

// singletonMarker is the element type of the singleton field, so only a type
// this package built carries it
type singletonMarker struct{}

var singletonField = reflect.StructField{
	Name: SingletonField,
	Type: reflect.ArrayOf(0, reflect.TypeFor[singletonMarker]()),
	Tag:  `json:"-"`,
}

// IsSingleton reports whether t, a struct type built from a schema, was
// built from one marked as a singleton
func IsSingleton(t reflect.Type) bool {
	field, ok := t.FieldByName(SingletonField)
	return ok && field.Type == singletonField.Type
}

// resourceBaseField is the anonymous types.ResourceBase every built block
// type starts with, tagged as a hand-written type tags it.
var resourceBaseField = reflect.StructField{
	Name:      "ResourceBase",
	Type:      reflect.TypeOf(types.ResourceBase{}),
	Anonymous: true,
	Tag:       `xcl:",remain"`,
}

// reservedFieldNames are the names ResourceBase already gives every block
// type, which a schema may not redeclare.
var reservedFieldNames = map[string]bool{
	"depends_on": true,
	"disabled":   true,
	"meta":       true,
}

// buildFieldsV1 builds the struct fields for one object. path names the
// enclosing field in errors, empty at the top level. top adds ResourceBase,
// and refuses the names it reserves.
func buildFieldsV1(fields []FieldV1, path string, top bool) ([]reflect.StructField, error) {
	built := []reflect.StructField{}
	if top {
		built = append(built, resourceBaseField)
	}

	seen := map[string]bool{}

	for _, f := range fields {
		where := f.Name
		if path != "" {
			where = path + "." + f.Name
		}

		if f.Name == "" {
			return nil, fmt.Errorf("a field has no name")
		}

		if top && reservedFieldNames[f.Name] {
			return nil, fmt.Errorf("field %s: %q is held by every block type and cannot be declared", where, f.Name)
		}

		goName := goFieldName(f.Name)
		if seen[goName] {
			return nil, fmt.Errorf("field %s: its Go name %s is already taken by another field", where, goName)
		}
		seen[goName] = true

		sf, err := buildFieldV1(f, where, goName)
		if err != nil {
			return nil, err
		}

		built = append(built, sf)
	}

	return built, nil
}

// buildFieldV1 builds one struct field.
func buildFieldV1(f FieldV1, where, goName string) (reflect.StructField, error) {
	switch f.Kind {
	case "", FieldKindAttribute:
		return buildAttributeV1(f, where, goName)
	case FieldKindBlock:
		return buildBlockV1(f, where, goName)
	default:
		return reflect.StructField{}, fmt.Errorf("field %s: unknown kind %q, expected %q or %q", where, f.Kind, FieldKindAttribute, FieldKindBlock)
	}
}

// buildAttributeV1 builds an attribute field: name = value.
func buildAttributeV1(f FieldV1, where, goName string) (reflect.StructField, error) {
	if f.Repeated {
		return reflect.StructField{}, fmt.Errorf("field %s: an attribute cannot be repeated, use a list type or a block", where)
	}

	if f.Label != "" {
		return reflect.StructField{}, fmt.Errorf("field %s: only a block has a label", where)
	}

	goType, err := valueTypeV1(f.Type, f.Format, where)
	if err != nil {
		return reflect.StructField{}, err
	}

	if f.Sensitive {
		goType, err = sensitiveTypeV1(goType, where)
		if err != nil {
			return reflect.StructField{}, err
		}
	}

	optional := f.Optional || f.Computed

	options := []string{f.Name}
	if optional {
		options = append(options, "optional")
	}
	if f.Computed {
		options = append(options, "computed")
	}
	if f.Key {
		options = append(options, "key")
	}

	tag := fmt.Sprintf(`xcl:%q json:%q`, strings.Join(options, ","), jsonTagV1(f.Name, optional))

	if len(f.Default) > 0 {
		if !optional {
			return reflect.StructField{}, fmt.Errorf("field %s: only an optional attribute has a default", where)
		}

		if f.Computed {
			return reflect.StructField{}, fmt.Errorf("field %s: a computed attribute has no default, the provider sets it", where)
		}

		text, err := defaultTagV1(f.Default, goType, where)
		if err != nil {
			return reflect.StructField{}, err
		}

		tag += fmt.Sprintf(` default:%q`, text)
	}

	return reflect.StructField{Name: goName, Type: goType, Tag: reflect.StructTag(tag)}, nil
}

// buildBlockV1 builds a block field: a nested block, a list of blocks or a
// map of named blocks.
func buildBlockV1(f FieldV1, where, goName string) (reflect.StructField, error) {
	if f.Sensitive {
		return reflect.StructField{}, fmt.Errorf("field %s: a block cannot be sensitive, mark its fields", where)
	}

	if f.Computed {
		return reflect.StructField{}, fmt.Errorf("field %s: a block cannot be computed, mark its fields", where)
	}

	if len(f.Default) > 0 {
		return reflect.StructField{}, fmt.Errorf("field %s: a block has no default", where)
	}

	if f.Key {
		return reflect.StructField{}, fmt.Errorf("field %s: a block cannot be a key, mark the attribute that identifies it", where)
	}

	objectFields, named, err := blockElementV1(f.Type, where)
	if err != nil {
		return reflect.StructField{}, err
	}

	if named && f.Repeated {
		return reflect.StructField{}, fmt.Errorf("field %s: a map of blocks is already a collection, it cannot be repeated too", where)
	}

	elementFields, err := buildFieldsV1(objectFields, where, false)
	if err != nil {
		return reflect.StructField{}, err
	}

	if f.Label != "" {
		if reservedFieldNames[f.Label] {
			return reflect.StructField{}, fmt.Errorf("field %s: label %q is a reserved name", where, f.Label)
		}

		for _, existing := range elementFields {
			if existing.Name == goFieldName(f.Label) {
				return reflect.StructField{}, fmt.Errorf("field %s: label %q is also declared as a field of the block", where, f.Label)
			}
		}

		label := reflect.StructField{
			Name: goFieldName(f.Label),
			Type: reflect.TypeOf(""),
			Tag:  reflect.StructTag(fmt.Sprintf(`xcl:%q json:%q`, f.Label+",label", f.Label+",omitempty")),
		}

		elementFields = append([]reflect.StructField{label}, elementFields...)
	}

	element := reflect.StructOf(elementFields)

	var goType reflect.Type
	switch {
	case named:
		goType = reflect.MapOf(reflect.TypeOf(""), element)
	case f.Repeated:
		goType = reflect.SliceOf(element)
	case f.Optional:
		// a pointer lets the block be left out
		goType = reflect.PointerTo(element)
	default:
		goType = element
	}

	tag := fmt.Sprintf(`xcl:%q json:%q`, f.Name+",block", jsonTagV1(f.Name, true))

	return reflect.StructField{Name: goName, Type: goType, Tag: reflect.StructTag(tag)}, nil
}

// blockElementV1 reads a block's type, which is {"object": [fields]} for a
// nested block or a list of blocks, and {"map": {"object": [fields]}} for a
// map of named blocks. It returns the element's fields and whether the
// block is a map.
func blockElementV1(raw json.RawMessage, where string) ([]FieldV1, bool, error) {
	var typed struct {
		Object []FieldV1       `json:"object"`
		Map    json.RawMessage `json:"map"`
	}

	if len(raw) == 0 || raw[0] != '{' {
		return nil, false, fmt.Errorf("field %s: a block's type must be an object or a map of objects, got %s", where, string(raw))
	}

	if err := json.Unmarshal(raw, &typed); err != nil {
		return nil, false, fmt.Errorf("field %s: a block's type must be an object or a map of objects: %w", where, err)
	}

	if typed.Object != nil {
		return typed.Object, false, nil
	}

	if len(typed.Map) > 0 {
		var inner struct {
			Object []FieldV1 `json:"object"`
		}

		if typed.Map[0] == '{' {
			if err := json.Unmarshal(typed.Map, &inner); err == nil && inner.Object != nil {
				return inner.Object, true, nil
			}
		}

		return nil, false, fmt.Errorf("field %s: a map of blocks must map to an object, got %s", where, string(typed.Map))
	}

	return nil, false, fmt.Errorf("field %s: a block's type must be an object or a map of objects, got %s", where, string(raw))
}

var anyType = reflect.TypeOf((*any)(nil)).Elem()

// valueTypeV1 turns a value type into a Go type.
func valueTypeV1(raw json.RawMessage, format, where string) (reflect.Type, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("field %s: no type", where)
	}

	if raw[0] == '"' {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil {
			return nil, fmt.Errorf("field %s: %w", where, err)
		}

		switch name {
		case "string":
			return reflect.TypeOf(""), nil
		case "bool":
			return reflect.TypeOf(true), nil
		case "any":
			return anyType, nil
		case "number":
			switch format {
			case "", NumberFormatInt:
				return reflect.TypeOf(0), nil
			case NumberFormatFloat:
				return reflect.TypeOf(0.0), nil
			default:
				return nil, fmt.Errorf("field %s: unknown number format %q, expected %q or %q", where, format, NumberFormatInt, NumberFormatFloat)
			}
		default:
			return nil, fmt.Errorf("field %s: unknown type %q, expected string, number, bool, any, {\"list\": T}, {\"map\": T}, {\"object\": [fields]} or {\"entity\": [fields]}", where, name)
		}
	}

	if raw[0] != '{' {
		return nil, fmt.Errorf("field %s: a type must be a name or an object, got %s", where, string(raw))
	}

	var typed struct {
		List   json.RawMessage `json:"list"`
		Map    json.RawMessage `json:"map"`
		Object []FieldV1       `json:"object"`
		Entity []FieldV1       `json:"entity"`
	}

	if err := json.Unmarshal(raw, &typed); err != nil {
		return nil, fmt.Errorf("field %s: %w", where, err)
	}

	switch {
	case typed.Entity != nil:
		// an entity, as a reference to a block gives it, carries the meta,
		// disabled and depends_on every block has beside its own fields
		fields, err := buildFieldsV1(typed.Entity, where, true)
		if err != nil {
			return nil, err
		}

		return reflect.StructOf(fields), nil
	case len(typed.List) > 0:
		element, err := valueTypeV1(typed.List, format, where)
		if err != nil {
			return nil, err
		}

		return reflect.SliceOf(element), nil
	case len(typed.Map) > 0:
		element, err := valueTypeV1(typed.Map, format, where)
		if err != nil {
			return nil, err
		}

		return reflect.MapOf(reflect.TypeOf(""), element), nil
	case typed.Object != nil:
		fields, err := buildFieldsV1(typed.Object, where, false)
		if err != nil {
			return nil, err
		}

		return reflect.StructOf(fields), nil
	default:
		return nil, fmt.Errorf("field %s: unknown type %s, expected string, number, bool, any, {\"list\": T}, {\"map\": T}, {\"object\": [fields]} or {\"entity\": [fields]}", where, string(raw))
	}
}

// sensitiveTypeV1 wraps a value type in the matching types.Sensitive
// instantiation. Go cannot instantiate a generic at run time, so only the
// shapes in KnownTypes are available.
func sensitiveTypeV1(inner reflect.Type, where string) (reflect.Type, error) {
	key := sensitiveTypePrefix + inner.String() + "]"

	wrapped, ok := KnownTypes()[key]
	if !ok {
		return nil, fmt.Errorf("field %s: a sensitive value of type %s cannot be held, sensitive values may be string, number, bool, {\"list\": \"string\"} or {\"map\": \"string\"}", where, inner)
	}

	return wrapped, nil
}

// defaultTagV1 renders a default value as the text the default tag holds: a
// string's own text, and the JSON text of anything else.
func defaultTagV1(raw json.RawMessage, goType reflect.Type, where string) (string, error) {
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", fmt.Errorf("field %s: %w", where, err)
		}

		return text, nil
	}

	// the default must be a value the field can hold
	target := reflect.New(goType)
	if err := json.Unmarshal(raw, target.Interface()); err != nil {
		return "", fmt.Errorf("field %s: default %s is not a %s: %w", where, string(raw), goType, err)
	}

	return string(raw), nil
}

// jsonTagV1 is the json tag of a field: the schema name, left out of JSON
// when it is optional and empty.
func jsonTagV1(name string, optional bool) string {
	if optional {
		return name + ",omitempty"
	}

	return name
}

// goFieldName is the exported Go name of a schema field: the snake_case name
// in CamelCase, which reflect.StructOf needs and nothing else reads.
func goFieldName(name string) string {
	var b strings.Builder

	for _, part := range strings.Split(name, "_") {
		if part == "" {
			continue
		}

		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}

	if b.Len() == 0 {
		return "Field"
	}

	return b.String()
}

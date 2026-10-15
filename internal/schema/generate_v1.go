package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/plaintype"
	"github.com/jumppad-labs/xcl/internal/xcl/tags"
	"github.com/jumppad-labs/xcl/types"
)

// GenerateV1 writes the schema v1 document of a block type from t, the Go
// struct a block of it is decoded into, which embeds types.ResourceBase. It
// is the inverse of BuildTypeV1: the type the host builds from the document
// carries the xcl, json and default tags t carries, so configuration decodes
// into it as it does into t, and the JSON of one decodes into the other.
//
// An embedded struct's fields are described as the type's own. A block
// element that holds itself, directly or through other blocks, is described
// once among the document's definitions, named after its block, and referred
// to wherever it is used. A shape the document cannot describe is an error
// naming the field: a field without an xcl tag, a json name other than its
// xcl name, a map keyed by anything but a string, a value that holds itself,
// and a sensitive value of a type sensitiveTypes does not hold.
func GenerateV1(t reflect.Type, typeName, subtype string, singleton bool) ([]byte, error) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", t)
	}

	g := &generatorV1{
		inside:      map[reflect.Type]string{t: ""},
		recurring:   map[reflect.Type]string{},
		definitions: map[string][]FieldV1{},
	}

	fields, err := g.fields(t, "")
	if err != nil {
		return nil, fmt.Errorf("type %s: %w", types.TypeKey(typeName, subtype), err)
	}

	doc := SchemaV1{
		SchemaVersion: SchemaVersion1,
		Type:          typeName,
		Subtype:       subtype,
		Singleton:     singleton,
		Fields:        fields,
	}

	if len(g.definitions) > 0 {
		doc.Definitions = g.definitions
	}

	return json.Marshal(doc)
}

// generatorV1 describes one block type
type generatorV1 struct {
	// inside holds the struct types being described on the current path,
	// each with the name of the block it was entered through, empty for the
	// block type itself and for a value
	inside map[reflect.Type]string

	// recurring holds the block element types found inside themselves, each
	// with the name of its definition
	recurring map[reflect.Type]string

	// definitions are the described recurring element types, by name
	definitions map[string][]FieldV1
}

// fields describes the fields of one struct. path names the enclosing field
// in errors, empty at the top level. types.ResourceBase is never among them:
// BuildTypeV1 adds it to a block type and to an entity.
func (g *generatorV1) fields(t reflect.Type, path string) ([]FieldV1, error) {
	fields := []FieldV1{}

	for i := range t.NumField() {
		sf := t.Field(i)

		where := sf.Name
		if path != "" {
			where = path + "." + sf.Name
		}

		// an entity's metadata, embedded or as a Meta field, is described by
		// the entity form, see valueType, and added back by BuildTypeV1
		if (sf.Anonymous && sf.Type == resourceBaseField.Type) || isMetaField(sf) {
			continue
		}

		// the marker naming a plain type is the plugin's own Go type, which
		// the host has no way to build; it is never configuration
		if sf.Name == plaintype.MarkerField {
			continue
		}

		// an embedded struct's fields are the type's own, in configuration
		// as in JSON, so they are described in its place
		if sf.Anonymous {
			embedded := sf.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}

			if embedded.Kind() != reflect.Struct {
				return nil, fmt.Errorf("field %s: an embedded %s cannot be described", where, embedded)
			}

			lifted, err := g.fields(embedded, path)
			if err != nil {
				return nil, err
			}

			fields = append(fields, lifted...)
			continue
		}

		tag, ok := sf.Tag.Lookup(tags.Name)
		if !ok {
			return nil, fmt.Errorf("field %s: has no xcl tag, so the host would drop its value; tag it, as a computed field if the provider sets it", where)
		}

		parsed, err := tags.Parse(tag)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", where, err)
		}

		if parsed.Name == "" {
			return nil, fmt.Errorf("field %s: its xcl tag names no field", where)
		}

		// encoding/json matches names regardless of case, so a field without
		// a json tag still reads what the host writes under its xcl name
		if name := jsonName(sf); !strings.EqualFold(name, parsed.Name) {
			return nil, fmt.Errorf("field %s: its json name %q is not its xcl name %q, which the host writes it under", where, name, parsed.Name)
		}

		var field FieldV1
		switch parsed.Kind {
		case tags.KindAttr, tags.KindOptional:
			field, err = g.attribute(sf, parsed, where)
		case tags.KindBlock:
			field, err = g.block(sf, parsed, where)
		case tags.KindLabel:
			// a block element's label is named by the block that holds it,
			// see block
			continue
		default:
			return nil, fmt.Errorf("field %s: a %s field cannot be described", where, parsed.Kind)
		}

		if err != nil {
			return nil, err
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// attribute describes an attribute: name = value.
func (g *generatorV1) attribute(sf reflect.StructField, parsed tags.Field, where string) (FieldV1, error) {
	// a computed field is set by the provider, so configuration leaves it out
	if parsed.Computed && parsed.Kind != tags.KindOptional {
		return FieldV1{}, fmt.Errorf("field %s: a computed field must also be optional, i.e. `xcl:\"%s,optional,computed\"`", where, parsed.Name)
	}

	// a pointer may be left nil, so configuration may leave it out
	goType := sf.Type
	pointer := goType.Kind() == reflect.Pointer
	for goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}

	field := FieldV1{
		Name:     parsed.Name,
		Optional: (parsed.Kind == tags.KindOptional || pointer) && !parsed.Computed,
		Nullable: pointer,
		Computed: parsed.Computed,
		Key:      parsed.Key,
	}

	if isSensitiveType(goType) {
		inner, ok := sensitiveInnerV1[goType.String()]
		if !ok {
			return FieldV1{}, fmt.Errorf("field %s: unsupported sensitive type %s, sensitive values may be string, int, int64, float64, bool, []string or map[string]string", where, goType)
		}

		field.Sensitive = true
		goType = inner
	}

	valueType, format, err := g.valueType(goType, where)
	if err != nil {
		return FieldV1{}, err
	}

	field.Type = valueType
	field.Format = format

	if text, ok := sf.Tag.Lookup("default"); ok {
		field.Default = defaultJSONV1(text, goType)
	}

	return field, nil
}

// block describes a block field: a nested block, a list of blocks or a map of
// named blocks.
func (g *generatorV1) block(sf reflect.StructField, parsed tags.Field, where string) (FieldV1, error) {
	field := FieldV1{Name: parsed.Name, Kind: FieldKindBlock}

	element := sf.Type
	named := false

	switch element.Kind() {
	case reflect.Pointer:
		field.Optional = true
		element = element.Elem()
	case reflect.Slice:
		field.Repeated = true
		element = element.Elem()
	case reflect.Map:
		if element.Key().Kind() != reflect.String {
			return FieldV1{}, fmt.Errorf("field %s: a map of blocks must be keyed by a string, not %s", where, element.Key())
		}

		named = true
		element = element.Elem()
	}

	for element.Kind() == reflect.Pointer {
		element = element.Elem()
	}

	if element.Kind() != reflect.Struct {
		return FieldV1{}, fmt.Errorf("field %s: a block must hold a struct, not %s", where, element)
	}

	label, err := blockLabelV1(element, where)
	if err != nil {
		return FieldV1{}, err
	}
	field.Label = label

	elementType, err := g.element(element, parsed.Name, where)
	if err != nil {
		return FieldV1{}, err
	}

	if named {
		elementType, err = json.Marshal(map[string]json.RawMessage{"map": elementType})
		if err != nil {
			return FieldV1{}, err
		}
	}

	field.Type = elementType

	return field, nil
}

// element describes a block's element type, entered through the block called
// name: {"object": [fields]}, or {"ref": definition} for an element type that
// holds itself, found when it is met again inside itself.
func (g *generatorV1) element(element reflect.Type, name, where string) (json.RawMessage, error) {
	if entered, inside := g.inside[element]; inside {
		if entered == "" {
			return nil, fmt.Errorf("field %s: the block holds a %s, which holds the block, and only a block's own blocks can hold themselves", where, element)
		}

		return ref(g.recur(element, entered))
	}

	g.inside[element] = name
	fields, err := g.fields(element, where)
	delete(g.inside, element)

	if err != nil {
		return nil, err
	}

	// met inside itself, so described once and referred to here as well
	if definition, recurring := g.recurring[element]; recurring {
		g.definitions[definition] = fields
		return ref(definition)
	}

	return json.Marshal(map[string]any{"object": fields})
}

// recur names the definition of element, a block element type met inside
// itself, after the block it was entered through, and a number after that
// when another type already has the name
func (g *generatorV1) recur(element reflect.Type, block string) string {
	if definition, ok := g.recurring[element]; ok {
		return definition
	}

	definition := block
	for n := 2; g.named(definition); n++ {
		definition = fmt.Sprintf("%s_%d", block, n)
	}

	g.recurring[element] = definition

	return definition
}

// named reports whether a recurring type has the definition name already
func (g *generatorV1) named(definition string) bool {
	for _, taken := range g.recurring {
		if taken == definition {
			return true
		}
	}

	return false
}

// ref is a block element type referring to a definition
func ref(definition string) (json.RawMessage, error) {
	return json.Marshal(map[string]string{"ref": definition})
}

// blockLabelV1 returns the name of the label field of a block's element, or
// an empty string when it has none. A schema holds one label per element.
func blockLabelV1(element reflect.Type, where string) (string, error) {
	label := ""

	for i := range element.NumField() {
		parsed, err := tags.Parse(element.Field(i).Tag.Get(tags.Name))
		if err != nil || parsed.Kind != tags.KindLabel {
			continue
		}

		if label != "" {
			return "", fmt.Errorf("field %s: a block's element has one label, it has %q and %q", where, label, parsed.Name)
		}

		if element.Field(i).Type.Kind() != reflect.String {
			return "", fmt.Errorf("field %s: label %q must be a string", where, parsed.Name)
		}

		label = parsed.Name
	}

	return label, nil
}

// valueType describes a value's type, and the number format when it is a
// number.
func (g *generatorV1) valueType(t reflect.Type, where string) (json.RawMessage, string, error) {
	switch t.Kind() {
	case reflect.String:
		return json.RawMessage(`"string"`), "", nil
	case reflect.Bool:
		return json.RawMessage(`"bool"`), "", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return json.RawMessage(`"number"`), NumberFormatInt, nil
	case reflect.Float32, reflect.Float64:
		return json.RawMessage(`"number"`), NumberFormatFloat, nil
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return json.RawMessage(`"any"`), "", nil
		}
	case reflect.Pointer:
		return g.valueType(t.Elem(), where)
	}

	// any is a value of its own, never the element of a list or map, so a
	// collection of values that become any is any itself
	if holdsAny(t) {
		return json.RawMessage(`"any"`), "", nil
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		element, format, err := g.valueType(t.Elem(), where)
		if err != nil {
			return nil, "", err
		}

		list, err := json.Marshal(map[string]json.RawMessage{"list": element})
		return list, format, err
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, "", fmt.Errorf("field %s: a map must be keyed by a string, not %s", where, t.Key())
		}

		element, format, err := g.valueType(t.Elem(), where)
		if err != nil {
			return nil, "", err
		}

		m, err := json.Marshal(map[string]json.RawMessage{"map": element})
		return m, format, err
	case reflect.Struct:
		if !isSensitiveType(t) {
			// configuration writes a value out whole, so one cannot hold
			// itself as a block can
			if _, inside := g.inside[t]; inside {
				return nil, "", fmt.Errorf("field %s: a value of type %s holds itself, which only a block can", where, t)
			}

			g.inside[t] = ""
			fields, err := g.fields(t, where)
			delete(g.inside, t)

			if err != nil {
				return nil, "", err
			}

			// an entity, as a reference to a block gives it, keeps its meta
			form := "object"
			if isEntity(t) {
				form = "entity"
			}

			object, err := json.Marshal(map[string]any{form: fields})
			return object, "", err
		}
	}

	return nil, "", fmt.Errorf("field %s: a value of type %s cannot be described", where, t)
}

// holdsAny reports whether a value of t is described as any: a cty.Value,
// which holds whatever configuration gives it, or a list or map of them
func holdsAny(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Struct:
		return t == reflect.TypeFor[cty.Value]()
	case reflect.Slice, reflect.Array, reflect.Map:
		return holdsAny(t.Elem())
	}

	return false
}

// isEntity reports whether t, a struct, is an entity type: one holding its
// metadata, in a Meta field or through types.ResourceBase, itself or in a
// struct it embeds
func isEntity(t reflect.Type) bool {
	for i := range t.NumField() {
		sf := t.Field(i)
		if isMetaField(sf) {
			return true
		}

		if !sf.Anonymous {
			continue
		}

		embedded := sf.Type
		for embedded.Kind() == reflect.Pointer {
			embedded = embedded.Elem()
		}

		if embedded == resourceBaseField.Type || (embedded.Kind() == reflect.Struct && isEntity(embedded)) {
			return true
		}
	}

	return false
}

// isMetaField reports whether sf is a plain type's Meta field, which xcl
// fills with the entity's metadata
func isMetaField(sf reflect.StructField) bool {
	_, tagged := sf.Tag.Lookup(tags.Name)
	return sf.Name == "Meta" && sf.Type == metaField.Type && !tagged
}

// sensitiveInnerV1 maps every types.Sensitive instantiation sensitiveTypes holds
// to the type of the value it holds.
var sensitiveInnerV1 = map[string]reflect.Type{
	sensitiveTypePrefix + "string]":            reflect.TypeOf(""),
	sensitiveTypePrefix + "int]":               reflect.TypeOf(0),
	sensitiveTypePrefix + "int64]":             reflect.TypeOf(int64(0)),
	sensitiveTypePrefix + "float64]":           reflect.TypeOf(0.0),
	sensitiveTypePrefix + "bool]":              reflect.TypeOf(true),
	sensitiveTypePrefix + "[]string]":          reflect.TypeOf([]string{}),
	sensitiveTypePrefix + "map[string]string]": reflect.TypeOf(map[string]string{}),
}

// defaultJSONV1 is a default tag's text as the JSON value a schema holds: a
// string's own text as a JSON string, and the JSON text of anything else.
func defaultJSONV1(text string, goType reflect.Type) json.RawMessage {
	if goType.Kind() == reflect.String {
		quoted, _ := json.Marshal(text)
		return quoted
	}

	return json.RawMessage(text)
}

// jsonName is the name a field is written under in JSON: its json tag's name,
// or its Go name when it has none.
func jsonName(sf reflect.StructField) string {
	name, _, _ := strings.Cut(sf.Tag.Get("json"), ",")
	if name == "" {
		return sf.Name
	}

	return name
}

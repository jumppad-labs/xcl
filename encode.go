package xcl

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/types"
)

// computedComment marks a value a provider filled in, so a reader of the text
// can tell it from a value that was configured
const computedComment = "set by the provider"

// encodeOptions holds what the EncodeOption functions configure. It is
// unexported so the set of options stays closed to the functions below.
type encodeOptions struct {
	includeComputed bool
	revealSensitive bool
	showReferences  bool
}

// EncodeOption configures how an entity is written as configuration text.
// Construct one with IncludeComputed, RevealSensitive or ShowReferences.
type EncodeOption func(*encodeOptions)

// IncludeComputed also writes the fields a provider fills in, such as an
// address or an id the provider assigned.
//
// Text written this way is for reading. xcl refuses a configuration that sets
// a computed field, because the provider would overwrite it, so output made
// with this option does not validate. Leave it off for text that has to read
// back.
func IncludeComputed() EncodeOption {
	return func(o *encodeOptions) {
		o.includeComputed = true
	}
}

// RevealSensitive writes the real value of each sensitive field. Without it,
// each sensitive value is written as the string "(sensitive)",
// types.SensitiveMarker.
//
// A value read back from the marker, such as from event data, has no real
// value to show, so it is written as the marker even with this option.
func RevealSensitive() EncodeOption {
	return func(o *encodeOptions) {
		o.revealSensitive = true
	}
}

// ShowReferences writes each field whose value referred to another entity as
// the user wrote it, such as x = resource.b.one.y or a template mixing a
// reference with text, instead of the value it resolved to. Only the fields the
// text already holds are changed, so a field that is left out stays out.
//
// Text written this way is still for reading. A sensitive field shows its
// reference only when it was written as a single bare reference, such as
// password = variable.db_password, since an address holds no secret. Written
// any other way it is still shown as types.SensitiveMarker, unless
// RevealSensitive is also given.
//
// The references come from the entity's own bookkeeping, so the text is the
// same from a live entity and from its saved data. Data saved by an earlier
// version of xcl holds no references and is written with resolved values.
func ShowReferences() EncodeOption {
	return func(o *encodeOptions) {
		o.showReferences = true
	}
}

// EncodeEntity returns entity as configuration text in xcl's own syntax,
// formatted and ready to print or write to a .xcl file.
//
// entity is one entity, such as one returned by Find, FindByType, All or
// Entities after Apply. The text holds exactly one block:
// <type> "<subtype>" "<name>" for an entity with a subtype, i.e.
// resource "container" "nics", and <type> "<name>" for one without. Convert
// several entities by calling this once for each.
//
// Fields a provider filled in are left out unless IncludeComputed is given, so
// the text reads back to the same configured values. Values are written as the
// literals they resolved to, not as the expressions the original configuration
// used, unless ShowReferences is given.
//
// It fails with ErrNotEncodable when entity is not an entity, when it is a
// builtin such as a variable, output or module, or when it holds a value that
// has no configuration form. A failure returns no text.
func EncodeEntity(entity any, options ...EncodeOption) ([]byte, error) {
	opts := encodeOptions{}
	for _, option := range options {
		option(&opts)
	}

	return encodeEntity(entity, opts)
}

// EncodeSavedEntity returns the configuration text for one entity's saved
// data, exactly as state stores it and as events carry it at
// EventDataProcessed. The text is identical to what EncodeEntity returns for
// the same entity. Event data carries each sensitive value as
// types.SensitiveMarker rather than its real value, so text from event data
// shows the marker for it.
//
// registry resolves the record's type. A plugin's types resolve only once the
// registry has loaded, so this loads it if it has not been loaded already.
// Loading happens once per registry and is cached, so passing a registry no
// configuration has used yet works, and passing one that has costs nothing.
//
// It fails with ErrUnregisteredType when the registry does not know the type
// the record names, with ErrInvalidSavedData when the data is not one saved
// entity record, and with ErrNotEncodable on the same terms as EncodeEntity. A
// failure returns no text.
func EncodeSavedEntity(registry *registry.PluginRegistry, data []byte, options ...EncodeOption) ([]byte, error) {
	if registry == nil {
		return nil, &xclerrors.NotEncodableError{
			What:   "saved data",
			Reason: "no registry was given to resolve its type",
		}
	}

	opts := encodeOptions{}
	for _, option := range options {
		option(&opts)
	}

	// a plugin's types are resolvable only after the registry has loaded, and
	// loading is once only and cached, so this is safe whether or not a
	// configuration has already used this registry
	if err := registry.Load(nil); err != nil {
		return nil, err
	}

	entity, err := savedentity.Decode(registry, data)
	if err != nil {
		return nil, err
	}

	return encodeEntity(entity, opts)
}

// encodeEntity is the one path both entry points share, which is what makes
// an entity and its saved data give identical text
func encodeEntity(entity any, opts encodeOptions) ([]byte, error) {
	meta, err := types.GetMeta(entity)
	if err != nil {
		return nil, &xclerrors.NotEncodableError{
			What:   fmt.Sprintf("a value of type %T", entity),
			Reason: "it is not an entity",
			Err:    err,
		}
	}

	// a builtin declares xcl's own machinery rather than a thing a provider
	// creates, and its values do not survive being saved, so it is refused
	// rather than written wrongly
	switch meta.Type {
	case resources.TypeVariable, resources.TypeOutput, resources.TypeModule, resources.TypeRoot:
		return nil, &xclerrors.NotEncodableError{
			What:   entityName(meta),
			Reason: fmt.Sprintf("a %s is not written as configuration", meta.Type),
		}
	}

	// an entity is led by its type and labelled with its subtype, where it
	// has one, and its name
	block := hclwrite.NewBlock(meta.Type, []string{meta.Name})
	if meta.Subtype != "" {
		block = hclwrite.NewBlock(meta.Type, []string{meta.Subtype, meta.Name})
	}

	err = gohcl.EncodeBody(entity, block.Body(), gohcl.EncodeOptions{
		IncludeComputed: opts.includeComputed,
		ComputedComment: computedComment,
		ReplaceMarked:   sensitiveReplacement(opts.revealSensitive),
	})
	if err != nil {
		return nil, &xclerrors.NotEncodableError{
			What: entityName(meta),
			Err:  err,
		}
	}

	trimBookkeeping(entity, block.Body())

	if opts.showReferences {
		showReferences(meta, block.Body(), opts)
	}

	file := hclwrite.NewEmptyFile()
	file.Body().AppendBlock(block)

	// Bytes runs the formatter, so the text is ready to write as it is
	return file.Bytes(), nil
}

// sensitiveReplacement returns how a sensitive value is written in
// configuration text: as the marker, or as its real value when reveal is set.
// A value read back from the marker is written as the marker either way.
func sensitiveReplacement(reveal bool) func(cty.Value) cty.Value {
	return func(value cty.Value) cty.Value {
		if !value.HasMark(types.SensitiveMark) {
			return value
		}

		if reveal && !value.HasMark(types.RedactedMark) {
			return value
		}

		return cty.StringVal(types.SensitiveMarker)
	}
}

// trimBookkeeping removes what ResourceBase records about an entity but nobody
// wrote in the configuration, so the text shows only what a person put there.
//
// meta is xcl's own account of where the entity came from. depends_on holds
// only what the author wrote, exactly as written, since the dependencies xcl
// works out from references are kept in meta's links and never added to it,
// so it is shown when the author wrote one and left out otherwise. disabled
// is written only when something is disabled, which is how a person would
// leave it out.
func trimBookkeeping(entity any, body *hclwrite.Body) {
	body.RemoveAttribute("meta")

	if dependencies, err := types.GetDependencies(entity); err != nil || len(dependencies) == 0 {
		body.RemoveAttribute("depends_on")
	}

	if disabled, err := types.GetDisabled(entity); err == nil && !disabled {
		body.RemoveAttribute("disabled")
	}
}

// referencePathSegment matches one segment of a recorded reference's path, a
// nested block written as <type>[<index>]
var referencePathSegment = regexp.MustCompile(`^(.+)\[(\d+)\]$`)

// showReferences replaces the value of each attribute the entity recorded a
// reference for with the text the user wrote. It runs after the resolved text
// is written and trimmed, and changes only attributes that text holds, so it
// never adds a line the resolved text would not have. The paths are visited
// in sorted order so the text is the same on every call.
func showReferences(meta *types.Meta, body *hclwrite.Body, opts encodeOptions) {
	for _, path := range slices.Sorted(maps.Keys(meta.References)) {
		written := meta.References[path]

		attributeBody, name := findReferenceBody(body, path)
		if attributeBody == nil {
			continue
		}

		attribute := attributeBody.GetAttribute(name)
		if attribute == nil {
			continue
		}

		// a sensitive value is written as the marker, and keeps it unless the
		// user wrote a single bare reference, an address that holds no
		// secret, or real values were asked for
		if !opts.revealSensitive && holdsSensitiveMarker(attribute) && !isBareReference(written) {
			continue
		}

		tokens, ok := writtenTokens(written)
		if !ok {
			// text the parser accepted always re-lexes, this is a guard: the
			// attribute keeps its resolved value rather than failing a
			// conversion that is only for reading
			continue
		}

		attributeBody.SetAttributeRaw(name, tokens)
	}
}

// findReferenceBody returns the body holding the attribute a recorded path
// names, and the attribute's name. A path is the attribute's name, led by the
// nested blocks it sits in, i.e. "network[1].name". It returns nil when the
// text holds no such block.
func findReferenceBody(body *hclwrite.Body, path string) (*hclwrite.Body, string) {
	segments := strings.Split(path, ".")

	current := body
	for _, segment := range segments[:len(segments)-1] {
		match := referencePathSegment.FindStringSubmatch(segment)
		if match == nil {
			return nil, ""
		}

		index, err := strconv.Atoi(match[2])
		if err != nil {
			return nil, ""
		}

		var found *hclwrite.Block
		position := 0
		for _, block := range current.Blocks() {
			if block.Type() != match[1] {
				continue
			}

			if position == index {
				found = block
				break
			}

			position++
		}

		if found == nil {
			return nil, ""
		}

		current = found.Body()
	}

	return current, segments[len(segments)-1]
}

// holdsSensitiveMarker reports whether the written value of attribute holds
// the sensitive marker anywhere, including inside an object or a list
func holdsSensitiveMarker(attribute *hclwrite.Attribute) bool {
	written := attribute.Expr().BuildTokens(nil).Bytes()

	return strings.Contains(string(written), strconv.Quote(types.SensitiveMarker))
}

// isBareReference reports whether text is a single reference and nothing
// else, i.e. variable.db_password
func isBareReference(text string) bool {
	expression, diags := hclsyntax.ParseExpression([]byte(text), "", hcl.InitialPos)
	if diags.HasErrors() {
		return false
	}

	_, ok := expression.(*hclsyntax.ScopeTraversalExpr)

	return ok
}

// writtenTokens lexes text the user wrote as an attribute's value into the
// tokens that write it, so the formatter can lay it out
func writtenTokens(text string) (hclwrite.Tokens, bool) {
	file, diags := hclwrite.ParseConfig([]byte("value = "+text+"\n"), "", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, false
	}

	attribute := file.Body().GetAttribute("value")
	if attribute == nil {
		return nil, false
	}

	return attribute.Expr().BuildTokens(nil), true
}

// entityName names an entity in an error, by its address where it has one
func entityName(meta *types.Meta) string {
	if meta.ID != "" {
		return meta.ID
	}

	return fmt.Sprintf("%s.%s", meta.AddressType(), meta.Name)
}

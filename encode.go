package xcl

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/gohcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/internal/xcl/hclwrite"
	"github.com/jumppad-labs/xcl/types"
)

// computedComment marks a value a provider filled in, so a reader of the text
// can tell it from a value that was configured
const computedComment = "set by the provider"

// encodeOptions holds what the EncodeOption functions configure. It is
// unexported so the set of options stays closed to the functions below.
type encodeOptions struct {
	includeComputed bool
	includeEmpty    bool
	revealSensitive bool
	showReferences  bool
	renderer        highlight.Renderer
}

// EncodeOption configures how an entity is written as configuration text.
// Construct one with IncludeComputed, IncludeEmpty, RevealSensitive,
// ShowReferences or Highlight.
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

// IncludeEmpty also writes the attributes that hold nothing. Without it an
// optional attribute at its zero value is left out, and so is one that is not
// set, a nil pointer, slice or map, since leaving either out reads back to the
// same value. With it, an optional attribute is written at its zero value, and
// one that is not set is written as the zero value of the type it would hold:
// "" for a *string, [] for a slice, {} for a map. Unset blocks are still left
// out.
//
// Text written this way is for reading every field. It does not read back
// exactly: an unset field is written the same way as one set to its zero
// value, so a nil *string reads back as a pointer to "", and a nil slice or
// map as an empty one.
func IncludeEmpty() EncodeOption {
	return func(o *encodeOptions) {
		o.includeEmpty = true
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

// Highlight passes the finished text through renderer, labelling each token
// with the TextMate scope the xcl-vscode grammar gives it, such as
// storage.type.xcl for a block type. The text is unchanged apart from what the
// renderer adds, and highlighting combines with every other option, including
// ShowReferences, whose expressions are highlighted like any other text.
//
// Use highlight.NewANSIRenderer to colour text for a terminal, or write a
// highlight.Renderer for any other format. xcl never checks whether output is
// a terminal, so whether to ask for colour is the caller's decision. A nil
// renderer means no highlighting.
func Highlight(renderer highlight.Renderer) EncodeOption {
	return func(o *encodeOptions) {
		o.renderer = renderer
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
// the text reads back to the same configured values. Optional attributes at
// their zero value, and attributes that are not set, are left out unless
// IncludeEmpty is given. Values are written as the
// literals they resolved to, not as the expressions the original configuration
// used, unless ShowReferences is given. The text is plain, with no colour or
// markup, unless Highlight is given.
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
// the same entity. A sensitive value masked in state or in event data, see
// WithStateMask and WithEventMask, is shown as types.SensitiveMarker, never as
// ciphertext or a hash; a receiver that needs the real value opens it with
// mask.Unmask.
//
// The record's type is resolved through the Config's own types and plugins.
// A plugin's types resolve only once the plugins have loaded, so this loads
// them if no operation has yet. Loading happens once per Config and is
// cached, so calling this before any operation works, and calling it after
// costs nothing.
//
// It fails with ErrUnregisteredType when the Config does not know the type
// the record names, with ErrInvalidSavedData when the data is not one saved
// entity record, with ErrPluginLoad when a plugin fails to load, and with
// ErrNotEncodable on the same terms as EncodeEntity. A failure returns no
// text.
func (c *Config) EncodeSavedEntity(data []byte, options ...EncodeOption) ([]byte, error) {
	opts := encodeOptions{}
	for _, option := range options {
		option(&opts)
	}

	// a plugin's types are resolvable only after the plugins have loaded,
	// and loading is once only and cached, so this is safe whether or not an
	// operation has already run. External plugin processes are stopped again
	// once the record is typed, unless an operation is still using them.
	done, err := c.catalog.Use(nil)
	if err != nil {
		return nil, err
	}
	defer done()

	// masked values are shown as the marker, never as ciphertext or a hash
	entity, err := savedentity.Decode(c.catalog, data, savedentity.ReadOptions{ForDisplay: true})
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
		IncludeEmpty:    opts.includeEmpty,
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
	text := file.Bytes()

	// highlighting runs last, on the final text, so it sees exactly what is
	// returned and interacts with no other option
	if opts.renderer != nil {
		text = highlight.Text(text, opts.renderer)
	}

	return text, nil
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

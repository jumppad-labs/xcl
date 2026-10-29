package highlight

import (
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Kind is what a piece of output is, such as a block type, an attribute name
// or a line comment. Kinds are numbered in blocks of 100: the kind that ends
// in 00 is a category, its parent, and the kinds after it belong to it, so a
// theme that styles Comment styles every kind of comment. The numbers are a
// contract shared with every language xcl is used from, and never change.
type Kind int

// The kinds, grouped by their category.
const (
	// Text is unlabelled text: whitespace, braces, commas and anything else
	// no other kind describes. Every kind falls back to its style.
	Text Kind = 0

	// Block is the category of block types
	Block Kind = 100
	// BlockType is the type that opens a block with labels, such as resource
	// in resource "network" "main" {
	BlockType Kind = 101
	// NestedBlock is the type of a block with no labels, such as timeouts in
	// timeouts {
	NestedBlock Kind = 102

	// Label is the category of block labels
	Label Kind = 200
	// TypeLabel is the first of a block's two labels, its subtype
	TypeLabel Kind = 201
	// NameLabel is a block's name: its only label, or the second of two
	NameLabel Kind = 202

	// Attribute is the name of an attribute being set
	Attribute Kind = 300

	// String is a double quoted string, its quotes included
	String Kind = 400
	// StringEscape is an escape sequence in a quoted string, such as \n
	StringEscape Kind = 401
	// Heredoc is the body of a heredoc
	Heredoc Kind = 402
	// HeredocMarker is the marker that opens and closes a heredoc, such as EOF
	HeredocMarker Kind = 403
	// HeredocOperator is the << or <<- opening a heredoc
	HeredocOperator Kind = 404

	// Number is a number
	Number Kind = 500

	// Constant is true, false or null
	Constant Kind = 600

	// Function is a call to a function
	Function Kind = 700
	// BuiltinFunction is a call to a function xcl provides, such as len
	BuiltinFunction Kind = 701

	// Reference is the first part of a reference, such as resource in
	// resource.network.main.id
	Reference Kind = 800
	// Member is each part of a reference after the first
	Member Kind = 801

	// Operator is an operator, = included
	Operator Kind = 900

	// Punctuation is the category of punctuation
	Punctuation Kind = 1000
	// InterpolationBegin is the ${ opening an interpolation, and the #{{
	// opening a template marker in a heredoc
	InterpolationBegin Kind = 1001
	// InterpolationEnd is the } closing an interpolation, and the }} closing
	// a template marker in a heredoc
	InterpolationEnd Kind = 1002
	// TemplateDirective is a %{ ... } template directive, and the inside of
	// a #{{ ... }} template marker in a heredoc
	TemplateDirective Kind = 1003
	// Accessor is each . in a reference
	Accessor Kind = 1004

	// Comment is the category of comments
	Comment Kind = 1100
	// LineComment is a comment starting with //
	LineComment Kind = 1101
	// HashComment is a comment starting with #
	HashComment Kind = 1102
	// BlockComment is a comment between /* and */
	BlockComment Kind = 1103

	// Diff is the category of what a change summary marks as changing
	Diff Kind = 1200
	// Inserted is a created resource or an added value
	Inserted Kind = 1201
	// Deleted is a deleted resource or a removed value
	Deleted Kind = 1202
	// Changed is an updated or replaced resource, or a changed value
	Changed Kind = 1203

	// Problem is the category of what a report marks as wrong
	Problem Kind = 1300
	// Error is the severity of an error, and the source at fault
	Error Kind = 1301
	// Warning is the severity of a warning, and the source it is about
	Warning Kind = 1302

	// Report is the category of the parts of an error report
	Report Kind = 1400
	// Summary is a report's summary, after its severity
	Summary Kind = 1401
	// Location is the line of a report saying where the problem is
	Location Kind = 1402
	// Gutter is the line number written before each line of source
	Gutter Kind = 1403
	// Detail is a report's detail
	Detail Kind = 1404
)

// kindInfo is what is known of a kind: its name and its TextMate scope
type kindInfo struct {
	name  string
	scope string
}

// kinds holds every known kind. The scopes of the kinds the tokenizer emits
// are the names the xcl-vscode grammar (syntaxes/xcl.tmLanguage.json) gives
// the same text, so a VS Code colour theme matches them as the editor does.
var kinds = map[Kind]kindInfo{
	Text:               {"Text", ""},
	Block:              {"Block", "meta.block.xcl"},
	BlockType:          {"BlockType", "storage.type.xcl"},
	NestedBlock:        {"NestedBlock", "entity.name.function.block.xcl"},
	Label:              {"Label", "entity.name.xcl"},
	TypeLabel:          {"TypeLabel", "entity.name.type.xcl"},
	NameLabel:          {"NameLabel", "entity.name.tag.xcl"},
	Attribute:          {"Attribute", "variable.other.property.xcl"},
	String:             {"String", "string.quoted.double.xcl"},
	StringEscape:       {"StringEscape", "constant.character.escape.xcl"},
	Heredoc:            {"Heredoc", "string.unquoted.heredoc.xcl"},
	HeredocMarker:      {"HeredocMarker", "keyword.control.heredoc.xcl"},
	HeredocOperator:    {"HeredocOperator", "keyword.operator.heredoc.xcl"},
	Number:             {"Number", "constant.numeric.xcl"},
	Constant:           {"Constant", "constant.language.xcl"},
	Function:           {"Function", "entity.name.function.xcl"},
	BuiltinFunction:    {"BuiltinFunction", "support.function.builtin.xcl"},
	Reference:          {"Reference", "support.class.reference.xcl"},
	Member:             {"Member", "variable.other.member.xcl"},
	Operator:           {"Operator", "keyword.operator.xcl"},
	Punctuation:        {"Punctuation", "punctuation.xcl"},
	InterpolationBegin: {"InterpolationBegin", "punctuation.section.interpolation.begin.xcl"},
	InterpolationEnd:   {"InterpolationEnd", "punctuation.section.interpolation.end.xcl"},
	TemplateDirective:  {"TemplateDirective", "meta.interpolation.template.xcl"},
	Accessor:           {"Accessor", "punctuation.accessor.xcl"},
	Comment:            {"Comment", "comment.xcl"},
	LineComment:        {"LineComment", "comment.line.double-slash.xcl"},
	HashComment:        {"HashComment", "comment.line.number-sign.xcl"},
	BlockComment:       {"BlockComment", "comment.block.xcl"},
	Diff:               {"Diff", "markup.diff"},
	Inserted:           {"Inserted", "markup.inserted.diff"},
	Deleted:            {"Deleted", "markup.deleted.diff"},
	Changed:            {"Changed", "markup.changed.diff"},
	Problem:            {"Problem", "invalid.diagnostic"},
	Error:              {"Error", "invalid.illegal.diagnostic"},
	Warning:            {"Warning", "invalid.deprecated.diagnostic"},
	Report:             {"Report", "meta.report.xcl"},
	Summary:            {"Summary", "markup.heading.report.xcl"},
	Location:           {"Location", "meta.report.location.xcl"},
	Gutter:             {"Gutter", "comment.line.gutter.report.xcl"},
	Detail:             {"Detail", "meta.report.detail.xcl"},
}

// orderedKinds is every known kind in numeric order
var orderedKinds = func() []Kind {
	ordered := make([]Kind, 0, len(kinds))
	for kind := range kinds {
		ordered = append(ordered, kind)
	}

	slices.Sort(ordered)

	return ordered
}()

// Kinds returns every known kind in numeric order, so each category comes
// before the kinds in it
func Kinds() []Kind {
	return append([]Kind(nil), orderedKinds...)
}

// Parent returns the kind's category: the kind ending in 00 of its block of
// 100. A category is its own parent, and so is Text.
func (k Kind) Parent() Kind {
	return k / 100 * 100
}

// String returns the kind's name, such as BlockType, or Kind(n) for a kind
// this version does not know
func (k Kind) String() string {
	if info, ok := kinds[k]; ok {
		return info.name
	}

	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

// Class returns the HTML class name of the kind: xcl- and its name in kebab
// case, such as xcl-block-type. A kind this version does not know has its
// parent's class, or none when its parent is unknown too. Text has the class
// xcl-text, though the HTML formatter writes Text without a span.
func (k Kind) Class() string {
	info, ok := kinds[k]
	if !ok {
		if parent := k.Parent(); parent != k {
			if _, known := kinds[parent]; known {
				return parent.Class()
			}
		}

		return ""
	}

	return "xcl-" + kebab(info.name)
}

// Scope returns the TextMate scope of the kind, such as
// variable.other.property.xcl for Attribute: the name the xcl-vscode grammar
// gives the same text. It is used to style kinds from a VS Code colour theme.
// A kind this version does not know has its parent's scope.
func (k Kind) Scope() string {
	if info, ok := kinds[k]; ok {
		return info.scope
	}

	if parent := k.Parent(); parent != k {
		if info, ok := kinds[parent]; ok {
			return info.scope
		}
	}

	return ""
}

// kebab writes a name such as BlockType as block-type
func kebab(name string) string {
	var out strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i > 0 {
				out.WriteByte('-')
			}
			r = unicode.ToLower(r)
		}

		out.WriteRune(r)
	}

	return out.String()
}

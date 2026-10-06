package highlight

import "strings"

// The scopes Text labels pieces with. They are the names the xcl-vscode
// grammar (syntaxes/xcl.tmLanguage.json) gives the same text, .xcl suffix
// included, so they are exactly the names the editor shows and a VS Code
// colour theme matches against.
const (
	// ScopeBlockType labels the type that opens a block, such as resource
	// in resource "network" "main" {
	ScopeBlockType = "storage.type.xcl"

	// ScopeTypeLabel labels the first of a block's two labels, its subtype
	ScopeTypeLabel = "entity.name.type.xcl"

	// ScopeNameLabel labels a block's name, its only label or the second of two
	ScopeNameLabel = "entity.name.tag.xcl"

	// ScopeNestedBlock labels the type of a block with no labels, such as
	// container in container {
	ScopeNestedBlock = "entity.name.function.block.xcl"

	// ScopeAttribute labels the name of an attribute being set
	ScopeAttribute = "variable.other.property.xcl"

	// ScopeString labels a double quoted string, its quotes included
	ScopeString = "string.quoted.double.xcl"

	// ScopeEscape labels an escape sequence in a quoted string, such as \n
	ScopeEscape = "constant.character.escape.xcl"

	// ScopeInterpolationBegin labels the ${ opening an interpolation, and the
	// #{{ opening a template marker in a heredoc
	ScopeInterpolationBegin = "punctuation.section.interpolation.begin.xcl"

	// ScopeInterpolationEnd labels the } closing an interpolation, and the }}
	// closing a template marker in a heredoc
	ScopeInterpolationEnd = "punctuation.section.interpolation.end.xcl"

	// ScopeHeredoc labels the body of a heredoc
	ScopeHeredoc = "string.unquoted.heredoc.xcl"

	// ScopeHeredocOperator labels the << or <<- opening a heredoc
	ScopeHeredocOperator = "keyword.operator.heredoc.xcl"

	// ScopeHeredocMarker labels the marker that opens and closes a heredoc,
	// such as EOF
	ScopeHeredocMarker = "keyword.control.heredoc.xcl"

	// ScopeTemplateInterpolation labels the inside of a #{{ ... }} template
	// marker in a heredoc that is not an accessor or a member
	ScopeTemplateInterpolation = "meta.interpolation.template.xcl"

	// ScopeNumber labels a number
	ScopeNumber = "constant.numeric.xcl"

	// ScopeLanguageConstant labels true, false and null
	ScopeLanguageConstant = "constant.language.xcl"

	// ScopeBuiltinFunction labels a call to a function xcl provides, such as len
	ScopeBuiltinFunction = "support.function.builtin.xcl"

	// ScopeFunction labels a call to any other function
	ScopeFunction = "entity.name.function.xcl"

	// ScopeReference labels the first part of a reference, such as resource
	// in resource.network.main.id
	ScopeReference = "support.class.reference.xcl"

	// ScopeAccessor labels each . in a reference
	ScopeAccessor = "punctuation.accessor.xcl"

	// ScopeMember labels each part of a reference after the first
	ScopeMember = "variable.other.member.xcl"

	// ScopeOperator labels an operator, = included
	ScopeOperator = "keyword.operator.xcl"

	// ScopeLineComment labels a comment starting with //
	ScopeLineComment = "comment.line.double-slash.xcl"

	// ScopeHashComment labels a comment starting with #
	ScopeHashComment = "comment.line.number-sign.xcl"

	// ScopeBlockComment labels a comment between /* and */
	ScopeBlockComment = "comment.block.xcl"
)

// Renderer turns one labelled piece of configuration text into output. Text
// calls Render once for every piece of its input, in order, and joins what it
// returns. scope is the TextMate scope the xcl-vscode grammar gives the
// piece, one of the Scope constants, or "" for text the grammar leaves
// unlabelled, such as whitespace, braces and commas. Unlabelled text is passed
// too, so a renderer for a format that needs escaping sees every byte.
type Renderer interface {
	Render(scope, text string) string
}

// RendererFunc lets a plain function act as a Renderer
type RendererFunc func(scope, text string) string

// Render calls f
func (f RendererFunc) Render(scope, text string) string {
	return f(scope, text)
}

// Text returns text with every piece passed through renderer, in order. A
// renderer that returns each piece unchanged gives back text exactly, so
// highlighting adds only what the renderer adds. A nil renderer returns text
// as it is.
//
// Text accepts any input, including text that is not valid configuration:
// what cannot be labelled is passed through with the empty scope.
func Text(text []byte, renderer Renderer) []byte {
	if renderer == nil {
		return text
	}

	var out strings.Builder
	out.Grow(len(text))

	for _, p := range tokenize(text) {
		out.WriteString(renderer.Render(p.scope, p.text))
	}

	return []byte(out.String())
}

package highlight

import "strings"

// The scopes Pieces labels configuration with. They are the names the xcl-vscode
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

// The scopes diff output is labelled with. They are not grammar scopes: they
// are the TextMate diff scopes VS Code's own diff grammar uses, which colour
// themes already style through their markup.inserted, markup.deleted and
// markup.changed rules, so a theme colours an xcl diff the way it colours any
// other diff. A theme with no such rules leaves diff output in the default
// colour.
const (
	// ScopeInserted labels a created resource or an added value
	ScopeInserted = "markup.inserted.diff"

	// ScopeDeleted labels a deleted resource or a removed value
	ScopeDeleted = "markup.deleted.diff"

	// ScopeChanged labels an updated or replaced resource, or a changed value
	ScopeChanged = "markup.changed.diff"
)

// Renderer turns labelled pieces into output: plain text, terminal colour, or
// any other format. It is given every piece at once, in order, so it can
// reason across them, such as by line. The pieces hold every byte of the text,
// unlabelled text included, so a renderer for a format that needs escaping
// sees all of it.
type Renderer interface {
	Render(pieces []Piece) []byte
}

// RendererFunc lets a function over every piece act as a Renderer
type RendererFunc func(pieces []Piece) []byte

// Render calls f
func (f RendererFunc) Render(pieces []Piece) []byte {
	return f(pieces)
}

// PieceFunc lets a function over one piece act as a Renderer, for a renderer
// that styles each piece on its own: Render passes every piece to f in order
// and joins what it returns.
type PieceFunc func(piece Piece) string

// Render returns what f returns for each piece, joined
func (f PieceFunc) Render(pieces []Piece) []byte {
	var out strings.Builder
	for _, p := range pieces {
		out.WriteString(f(p))
	}

	return []byte(out.String())
}

// Text renders pieces as plain text: their text joined, with nothing added.
var Text Renderer = PieceFunc(func(piece Piece) string {
	return piece.Text
})

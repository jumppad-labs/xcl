package highlight

import (
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
)

// piece is one labelled run of the input. scope is "" for text the grammar
// leaves unlabelled.
type piece struct {
	scope string
	text  string
}

// builtinFunctions are the functions the xcl-vscode grammar labels
// support.function.builtin.xcl, copied verbatim from its functions rule
var builtinFunctions = map[string]bool{
	"abs": true, "ceil": true, "chomp": true, "chunklist": true, "coalescelist": true,
	"compact": true, "concat": true, "contains": true, "csvdecode": true, "dir": true,
	"distinct": true, "element": true, "env": true, "file": true, "flatten": true,
	"floor": true, "format": true, "formatdate": true, "formatlist": true, "home": true,
	"indent": true, "join": true, "jsondecode": true, "jsonencode": true, "keys": true,
	"len": true, "log": true, "lower": true, "max": true, "merge": true,
	"min": true, "parseint": true, "pow": true, "range": true, "regex": true,
	"regexall": true, "reverse": true, "setintersection": true, "setproduct": true, "setsubtract": true,
	"setunion": true, "signum": true, "slice": true, "sort": true, "split": true,
	"strrev": true, "substr": true, "template_file": true, "timeadd": true, "title": true,
	"trim": true, "trimprefix": true, "trimspace": true, "trimsuffix": true, "upper": true,
	"values": true, "zipmap": true,
}

// blockKeywords are the block types the grammar recognises even when the
// block's { is not on the same line
var blockKeywords = map[string]bool{
	"resource": true, "module": true, "variable": true, "output": true, "local": true,
}

// operatorTokens are the tokens the grammar's operators rule labels
// keyword.operator.xcl: == != >= <= && || + - * / % < > ! ? : and =
var operatorTokens = map[hclsyntax.TokenType]bool{
	hclsyntax.TokenEqual:         true,
	hclsyntax.TokenEqualOp:       true,
	hclsyntax.TokenNotEqual:      true,
	hclsyntax.TokenLessThan:      true,
	hclsyntax.TokenLessThanEq:    true,
	hclsyntax.TokenGreaterThan:   true,
	hclsyntax.TokenGreaterThanEq: true,
	hclsyntax.TokenAnd:           true,
	hclsyntax.TokenOr:            true,
	hclsyntax.TokenBang:          true,
	hclsyntax.TokenPlus:          true,
	hclsyntax.TokenMinus:         true,
	hclsyntax.TokenStar:          true,
	hclsyntax.TokenSlash:         true,
	hclsyntax.TokenPercent:       true,
	hclsyntax.TokenQuestion:      true,
	hclsyntax.TokenColon:         true,
	hclsyntax.TokenFatArrow:      true,
	hclsyntax.TokenDoubleColon:   true,
}

// frameKind says what a token is nested inside
type frameKind int

const (
	frameQuote     frameKind = iota // a double quoted string
	frameHeredoc                    // a heredoc body
	frameInterp                     // a ${ ... } interpolation
	frameDirective                  // a %{ ... } template directive
)

// frame is one level of nesting. inMarker records, for a heredoc, that a
// #{{ template marker was opened and has not yet been closed, as a marker
// may run across the lines of the heredoc.
type frame struct {
	kind     frameKind
	inMarker bool
}

// span gives every token and gap between start and end one scope, decided
// ahead of reaching them: a block label, which the grammar captures whole,
// or a template directive, which the grammar leaves as string text
type span struct {
	start, end int
	scope      string
}

// tokenizer walks the scanner's tokens in byte order with a cursor. Before
// each token it emits the gap between the cursor and the token, and it emits
// the text after the last token at the end, so the pieces always cover every
// byte of the input exactly once. That is what makes highlighting unable to
// change the text.
type tokenizer struct {
	src    []byte
	tokens hclsyntax.Tokens
	pieces []piece
	cursor int
	stack  []frame
	spans  []span
}

// tokenize splits src into labelled pieces that, joined, give back src
func tokenize(src []byte) []piece {
	// the diagnostics are ignored, an invalid token is simply left unlabelled
	tokens, _ := hclsyntax.LexConfig(src, "", hcl.InitialPos)

	t := &tokenizer{src: src, tokens: tokens}
	t.run()

	return t.pieces
}

func (t *tokenizer) run() {
	for i, token := range t.tokens {
		if token.Type == hclsyntax.TokenEOF {
			continue
		}

		start := t.clamp(token.Range.Start.Byte)
		end := t.clamp(token.Range.End.Byte)
		if start < t.cursor {
			start = t.cursor
		}
		if end <= start {
			continue
		}

		if start > t.cursor {
			t.emit(t.spanScope(t.cursor), string(t.src[t.cursor:start]))
		}

		text := string(t.src[start:end])
		if scope, ok := t.spanAt(start); ok {
			t.emit(scope, text)
		} else {
			t.label(i, text)
		}

		t.track(i)
		t.cursor = end
	}

	if t.cursor < len(t.src) {
		t.emit(t.spanScope(t.cursor), string(t.src[t.cursor:]))
	}
}

// clamp keeps a byte offset inside the source
func (t *tokenizer) clamp(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > len(t.src) {
		return len(t.src)
	}

	return offset
}

// emit appends a piece, joining it to the previous piece when both carry the
// same scope
func (t *tokenizer) emit(scope, text string) {
	if text == "" {
		return
	}

	if n := len(t.pieces); n > 0 && t.pieces[n-1].scope == scope {
		t.pieces[n-1].text += text
		return
	}

	t.pieces = append(t.pieces, piece{scope: scope, text: text})
}

// spanAt returns the scope of the span covering offset, dropping the spans
// the walk has already passed
func (t *tokenizer) spanAt(offset int) (string, bool) {
	for len(t.spans) > 0 && t.spans[0].end <= offset {
		t.spans = t.spans[1:]
	}

	if len(t.spans) > 0 && t.spans[0].start <= offset {
		return t.spans[0].scope, true
	}

	return "", false
}

// spanScope is the scope of gap text at offset: a span's scope inside one,
// and unlabelled otherwise
func (t *tokenizer) spanScope(offset int) string {
	scope, _ := t.spanAt(offset)
	return scope
}

// addSpan records a span, unless one already covers its start
func (t *tokenizer) addSpan(start, end int, scope string) {
	if _, ok := t.spanAt(start); ok {
		return
	}

	t.spans = append(t.spans, span{start: start, end: end, scope: scope})
}

// top returns the innermost frame, and false at the top level
func (t *tokenizer) top() (frame, bool) {
	if len(t.stack) == 0 {
		return frame{}, false
	}

	return t.stack[len(t.stack)-1], true
}

// track updates the nesting after token i
func (t *tokenizer) track(i int) {
	top, nested := t.top()

	switch t.tokens[i].Type {
	case hclsyntax.TokenOQuote:
		t.stack = append(t.stack, frame{kind: frameQuote})
	case hclsyntax.TokenOHeredoc:
		t.stack = append(t.stack, frame{kind: frameHeredoc})
	case hclsyntax.TokenTemplateInterp:
		t.stack = append(t.stack, frame{kind: frameInterp})
	case hclsyntax.TokenTemplateControl:
		t.stack = append(t.stack, frame{kind: frameDirective})
	case hclsyntax.TokenCQuote:
		if nested && top.kind == frameQuote {
			t.stack = t.stack[:len(t.stack)-1]
		}
	case hclsyntax.TokenCHeredoc:
		if nested && top.kind == frameHeredoc {
			t.stack = t.stack[:len(t.stack)-1]
		}
	case hclsyntax.TokenTemplateSeqEnd:
		if nested && (top.kind == frameInterp || top.kind == frameDirective) {
			t.stack = t.stack[:len(t.stack)-1]
		}
	}
}

// label emits the pieces of token i, whose text is text
func (t *tokenizer) label(i int, text string) {
	token := t.tokens[i]
	top, nested := t.top()

	if token.Type == hclsyntax.TokenComment {
		t.labelComment(text)
		return
	}

	if nested && top.kind == frameQuote {
		t.labelInQuote(i, text)
		return
	}

	if nested && top.kind == frameHeredoc {
		t.labelInHeredoc(i, text)
		return
	}

	// what is left is an expression: at the top level, inside an
	// interpolation or inside a directive outside a string
	switch token.Type {
	case hclsyntax.TokenTemplateSeqEnd:
		if nested && top.kind == frameInterp {
			t.emit(ScopeInterpolationEnd, text)
			return
		}
		t.emit("", text)
	case hclsyntax.TokenOQuote:
		t.emit(ScopeString, text)
	case hclsyntax.TokenOHeredoc:
		t.labelHeredocOpener(text)
	case hclsyntax.TokenNumberLit:
		t.emit(ScopeNumber, text)
	case hclsyntax.TokenIdent:
		t.emit(t.identScope(i, !nested), text)
	case hclsyntax.TokenDot:
		t.emit(t.dotScope(i), text)
	default:
		if operatorTokens[token.Type] {
			t.emit(ScopeOperator, text)
			return
		}
		t.emit("", text)
	}
}

// labelInQuote labels a token inside a double quoted string. Grammar rules
// string, string_escape and interpolation:
//
//	"string": { "begin": "\"", "end": "\"", "patterns": [string_escape, interpolation] }
//	"interpolation": { "begin": "\\$\\{", "end": "\\}" }
func (t *tokenizer) labelInQuote(i int, text string) {
	switch t.tokens[i].Type {
	case hclsyntax.TokenQuotedLit:
		t.labelQuotedLiteral(text)
	case hclsyntax.TokenTemplateInterp:
		t.emit(ScopeInterpolationBegin, text)
	case hclsyntax.TokenTemplateControl:
		t.labelDirective(i, ScopeString, text)
	default:
		// the closing quote, and anything the scanner found inside an
		// unterminated string
		t.emit(ScopeString, text)
	}
}

// labelInHeredoc labels a token inside a heredoc. Grammar rule heredoc:
//
//	"begin": "(<<-?)\\s*([A-Za-z_][A-Za-z0-9_]*)\\s*$", "end": "^\\s*\\2\\s*$",
//	"patterns": [template_interpolation, interpolation]
func (t *tokenizer) labelInHeredoc(i int, text string) {
	switch t.tokens[i].Type {
	case hclsyntax.TokenStringLit:
		t.labelHeredocBody(text)
	case hclsyntax.TokenTemplateInterp:
		t.emit(ScopeInterpolationBegin, text)
	case hclsyntax.TokenTemplateControl:
		t.labelDirective(i, ScopeHeredoc, text)
	case hclsyntax.TokenCHeredoc:
		t.labelHeredocCloser(text)
	default:
		t.emit(ScopeHeredoc, text)
	}
}

// labelDirective labels a %{ ... } template directive. The grammar has no
// rule for directives, so the whole directive, up to its closing }, keeps
// the scope of the string around it.
func (t *tokenizer) labelDirective(i int, scope, text string) {
	start := t.clamp(t.tokens[i].Range.Start.Byte)
	t.addSpan(start, t.directiveEnd(i), scope)
	t.emit(scope, text)
}

// directiveEnd returns the end of the } closing the directive token i
// opens, or the end of the text when it is never closed
func (t *tokenizer) directiveEnd(i int) int {
	depth := 1
	for j := i + 1; j < len(t.tokens); j++ {
		switch t.tokens[j].Type {
		case hclsyntax.TokenTemplateInterp, hclsyntax.TokenTemplateControl:
			depth++
		case hclsyntax.TokenTemplateSeqEnd:
			depth--
			if depth == 0 {
				return t.clamp(t.tokens[j].Range.End.Byte)
			}
		}
	}

	return len(t.src)
}

// identScope returns the scope of identifier i. topLevel is false inside an
// interpolation, where the grammar applies only its expression rules. The
// checks run in the order the grammar tries its rules, with the rules whose
// match starts earlier first.
func (t *tokenizer) identScope(i int, topLevel bool) string {
	// reference: "(\\.)([a-zA-Z_][a-zA-Z0-9_-]*)" starts at the dot, so it
	// claims an identifier after a dot before any rule starting at the
	// identifier itself
	if t.followsDot(i) {
		return ScopeMember
	}

	name := string(t.tokens[i].Bytes)

	if topLevel {
		if t.labelBlockHeader(i) {
			return ScopeBlockType
		}
		if t.labelBlockKeyword(i, name) {
			return ScopeBlockType
		}
		// nested block: "(?<![.\\w-])([a-zA-Z_][a-zA-Z0-9_-]*)\\b(?=\\s*\\{)"
		if t.nextIs(i, hclsyntax.TokenOBrace) {
			return ScopeNestedBlock
		}
		// attribute: "\\b([a-zA-Z_][a-zA-Z0-9_-]*)\\b(?=\\s*=(?!=))"
		if t.nextIs(i, hclsyntax.TokenEqual) {
			return ScopeAttribute
		}
	}

	// language_constant: "\\b(true|false|null)\\b"
	if name == "true" || name == "false" || name == "null" {
		return ScopeLanguageConstant
	}

	// functions: a built-in name, or any identifier, followed by (
	if t.nextIs(i, hclsyntax.TokenOParen) {
		if builtinFunctions[name] {
			return ScopeBuiltinFunction
		}
		return ScopeFunction
	}

	// reference: "(?<![.\\w-])([a-zA-Z_][a-zA-Z0-9_-]*)(?=\\.[a-zA-Z_])"
	if t.leadsMember(i) {
		return ScopeReference
	}

	return ""
}

// dotScope returns the scope of the dot token i: an accessor when an
// identifier follows it directly, and unlabelled otherwise
func (t *tokenizer) dotScope(i int) string {
	if t.adjacent(i, i+1) && t.tokens[i+1].Type == hclsyntax.TokenIdent {
		return ScopeAccessor
	}

	return ""
}

// labelBlockHeader applies the grammar's two labelled block rules to
// identifier i, recording spans for its labels when one matches:
//
//	"^\\s*([a-zA-Z_][a-zA-Z0-9_-]*)\\s+(\"[^\"]*\")\\s+(\"[^\"]*\")(?=\\s*\\{)"
//	"^\\s*([a-zA-Z_][a-zA-Z0-9_-]*)\\s+(\"[^\"]*\")(?=\\s*\\{)"
//
// With two labels the first is the type and the second the name, with one
// label it is the name.
func (t *tokenizer) labelBlockHeader(i int) bool {
	if !t.startsLine(i) {
		return false
	}

	firstEnd, ok := t.blockLabel(i)
	if !ok {
		return false
	}

	if secondEnd, ok := t.blockLabel(firstEnd); ok && t.nextIs(secondEnd, hclsyntax.TokenOBrace) {
		t.addLabelSpan(i+1, firstEnd, ScopeTypeLabel)
		t.addLabelSpan(firstEnd+1, secondEnd, ScopeNameLabel)
		return true
	}

	if t.nextIs(firstEnd, hclsyntax.TokenOBrace) {
		t.addLabelSpan(i+1, firstEnd, ScopeNameLabel)
		return true
	}

	return false
}

// labelBlockKeyword applies the grammar's rule for the built-in block
// keywords, which matches wherever the keyword is not followed by a dot, and
// labels up to two labels after it on the same line:
//
//	"\\b(resource|module|variable|output|local)\\b(?!\\s*\\.)(?:\\s+(\"[^\"]*\"))?(?:\\s+(\"[^\"]*\"))?"
func (t *tokenizer) labelBlockKeyword(i int, name string) bool {
	if !blockKeywords[name] || t.nextIs(i, hclsyntax.TokenDot) {
		return false
	}

	firstEnd, ok := t.blockLabel(i)
	if !ok {
		return true
	}
	t.addLabelSpan(i+1, firstEnd, ScopeTypeLabel)

	if secondEnd, ok := t.blockLabel(firstEnd); ok {
		t.addLabelSpan(firstEnd+1, secondEnd, ScopeNameLabel)
	}

	return true
}

// blockLabel reports whether a label, written "[^"]*" after whitespace,
// follows token i, and returns the index of the label's closing quote
func (t *tokenizer) blockLabel(i int) (int, bool) {
	open := i + 1
	if open >= len(t.tokens) || t.tokens[open].Type != hclsyntax.TokenOQuote {
		return 0, false
	}

	// \s+ before the label
	if t.adjacent(i, open) {
		return 0, false
	}

	// "[^"]*" ends at the first quote after the opening one
	start := t.clamp(t.tokens[open].Range.Start.Byte)
	end := -1
	for offset := start + 1; offset < len(t.src); offset++ {
		if t.src[offset] == '"' {
			end = offset + 1
			break
		}
		if t.src[offset] == '\n' {
			return 0, false
		}
	}
	if end < 0 {
		return 0, false
	}

	for j := open + 1; j < len(t.tokens); j++ {
		tokenEnd := t.tokens[j].Range.End.Byte
		if tokenEnd > end {
			return 0, false
		}
		if tokenEnd == end && t.tokens[j].Type == hclsyntax.TokenCQuote {
			return j, true
		}
	}

	return 0, false
}

// addLabelSpan gives tokens open to close, a label and its quotes, one scope
func (t *tokenizer) addLabelSpan(open, close int, scope string) {
	start := t.clamp(t.tokens[open].Range.Start.Byte)
	end := t.clamp(t.tokens[close].Range.End.Byte)
	t.spans = append(t.spans, span{start: start, end: end, scope: scope})
}

// startsLine reports whether only whitespace comes before token i on its line
func (t *tokenizer) startsLine(i int) bool {
	for offset := t.clamp(t.tokens[i].Range.Start.Byte) - 1; offset >= 0; offset-- {
		switch t.src[offset] {
		case '\n':
			return true
		case ' ', '\t', '\r', '\f', '\v':
			continue
		default:
			return false
		}
	}

	return true
}

// nextIs reports whether the token after i has type typ. Only whitespace can
// lie between two tokens, and newlines are tokens, so this is the grammar's
// (?=\s*x) lookahead.
func (t *tokenizer) nextIs(i int, typ hclsyntax.TokenType) bool {
	return i+1 < len(t.tokens) && t.tokens[i+1].Type == typ
}

// adjacent reports whether token j starts exactly where token i ends
func (t *tokenizer) adjacent(i, j int) bool {
	if i < 0 || j >= len(t.tokens) {
		return false
	}

	return t.tokens[i].Range.End.Byte == t.tokens[j].Range.Start.Byte
}

// followsDot reports whether identifier i comes straight after a dot
func (t *tokenizer) followsDot(i int) bool {
	return i > 0 && t.tokens[i-1].Type == hclsyntax.TokenDot && t.adjacent(i-1, i)
}

// leadsMember reports whether identifier i is followed straight away by a
// dot and another identifier
func (t *tokenizer) leadsMember(i int) bool {
	return t.nextIs(i, hclsyntax.TokenDot) && t.adjacent(i, i+1) &&
		t.nextIs(i+1, hclsyntax.TokenIdent) && t.adjacent(i+1, i+2)
}

// labelComment labels a comment by its prefix. The scanner includes a line
// comment's newline in its token, but the grammar ends the comment at $, so
// the newline is left unlabelled.
//
//	"comment.line.double-slash.xcl": { "begin": "//", "end": "$" }
//	"comment.line.number-sign.xcl": { "begin": "#", "end": "$" }
//	"comment.block.xcl": { "begin": "/\\*", "end": "\\*/" }
func (t *tokenizer) labelComment(text string) {
	switch {
	case len(text) >= 2 && text[:2] == "/*":
		t.emit(ScopeBlockComment, text)
		return
	case len(text) >= 2 && text[:2] == "//":
		body, newline := splitTrailingNewline(text)
		t.emit(ScopeLineComment, body)
		t.emit("", newline)
	case len(text) >= 1 && text[0] == '#':
		body, newline := splitTrailingNewline(text)
		t.emit(ScopeHashComment, body)
		t.emit("", newline)
	default:
		t.emit("", text)
	}
}

// splitTrailingNewline separates a trailing \n or \r\n from text
func splitTrailingNewline(text string) (string, string) {
	end := len(text)
	if end > 0 && text[end-1] == '\n' {
		end--
		if end > 0 && text[end-1] == '\r' {
			end--
		}
	}

	return text[:end], text[end:]
}

// labelQuotedLiteral labels the literal text of a quoted string, cutting out
// its escape sequences. Grammar rule string_escape:
//
//	"\\\\(n|r|t|\"|\\\\|u[0-9a-fA-F]{4})"
func (t *tokenizer) labelQuotedLiteral(text string) {
	start := 0
	for offset := 0; offset < len(text); {
		length := escapeLength(text[offset:])
		if length == 0 {
			offset++
			continue
		}

		t.emit(ScopeString, text[start:offset])
		t.emit(ScopeEscape, text[offset:offset+length])
		offset += length
		start = offset
	}

	t.emit(ScopeString, text[start:])
}

// escapeLength returns the length of the escape sequence text starts with,
// or 0 when it does not start with one the grammar recognises
func escapeLength(text string) int {
	if len(text) < 2 || text[0] != '\\' {
		return 0
	}

	switch text[1] {
	case 'n', 'r', 't', '"', '\\':
		return 2
	case 'u':
		if len(text) < 6 {
			return 0
		}
		for _, c := range []byte(text[2:6]) {
			if !isHexDigit(c) {
				return 0
			}
		}
		return 6
	}

	return 0
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v'
}

// labelHeredocOpener splits a heredoc's opening line, such as <<-EOF and its
// newline, into the operator, the marker and the rest. Grammar rule heredoc:
//
//	"begin": "(<<-?)\\s*([A-Za-z_][A-Za-z0-9_]*)\\s*$"
func (t *tokenizer) labelHeredocOpener(text string) {
	operatorEnd := 0
	switch {
	case len(text) >= 3 && text[:3] == "<<-":
		operatorEnd = 3
	case len(text) >= 2 && text[:2] == "<<":
		operatorEnd = 2
	}
	t.emit(ScopeHeredocOperator, text[:operatorEnd])

	offset := operatorEnd
	for offset < len(text) && isSpace(text[offset]) && text[offset] != '\n' {
		offset++
	}
	t.emit("", text[operatorEnd:offset])

	markerStart := offset
	if offset < len(text) && isIdentStart(text[offset]) {
		offset++
		for offset < len(text) && isIdentPart(text[offset]) {
			offset++
		}
	}
	t.emit(ScopeHeredocMarker, text[markerStart:offset])
	t.emit("", text[offset:])
}

// labelHeredocCloser splits a heredoc's closing line into its marker and the
// whitespace around it, which is left unlabelled
func (t *tokenizer) labelHeredocCloser(text string) {
	start := 0
	for start < len(text) && isSpace(text[start]) {
		start++
	}

	end := start
	for end < len(text) && !isSpace(text[end]) {
		end++
	}

	t.emit("", text[:start])
	t.emit(ScopeHeredocMarker, text[start:end])
	t.emit("", text[end:])
}

// labelHeredocBody labels a line of a heredoc's body, cutting out the
// #{{ ... }} template markers in it. A marker may run on to later lines, so
// whether one is open is kept on the heredoc's frame. Grammar rule
// template_interpolation:
//
//	"begin": "#\\{\\{", "end": "\\}\\}",
//	"patterns": [{ "match": "(\\.)([a-zA-Z_][a-zA-Z0-9_]*)" }]
func (t *tokenizer) labelHeredocBody(text string) {
	heredoc := &t.stack[len(t.stack)-1]

	offset := 0
	for offset < len(text) {
		if !heredoc.inMarker {
			open := indexOf(text, offset, "#{{")
			if open < 0 {
				t.emit(ScopeHeredoc, text[offset:])
				return
			}

			t.emit(ScopeHeredoc, text[offset:open])
			t.emit(ScopeInterpolationBegin, text[open:open+3])
			heredoc.inMarker = true
			offset = open + 3
			continue
		}

		switch {
		case hasPrefixAt(text, offset, "}}"):
			t.emit(ScopeInterpolationEnd, text[offset:offset+2])
			heredoc.inMarker = false
			offset += 2
		case text[offset] == '.' && offset+1 < len(text) && isIdentStart(text[offset+1]):
			t.emit(ScopeAccessor, text[offset:offset+1])
			end := offset + 2
			for end < len(text) && isIdentPart(text[end]) {
				end++
			}
			t.emit(ScopeMember, text[offset+1:end])
			offset = end
		default:
			t.emit(ScopeTemplateInterpolation, text[offset:offset+1])
			offset++
		}
	}
}

// indexOf returns the index of the first substr in text at or after from, or
// -1 when there is none
func indexOf(text string, from int, substr string) int {
	for offset := from; offset+len(substr) <= len(text); offset++ {
		if text[offset:offset+len(substr)] == substr {
			return offset
		}
	}

	return -1
}

// hasPrefixAt reports whether text holds prefix at offset
func hasPrefixAt(text string, offset int, prefix string) bool {
	return offset+len(prefix) <= len(text) && text[offset:offset+len(prefix)] == prefix
}

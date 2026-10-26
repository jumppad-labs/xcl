package highlight_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// documentedClasses are the class names the HTML formatter may write: the
// container's, and one for each kind. They are a public contract shared with
// every language xcl is used from, so they are written out here by hand.
var documentedClasses = map[string]bool{
	"xcl":                     true,
	"xcl-text":                true,
	"xcl-block":               true,
	"xcl-block-type":          true,
	"xcl-nested-block":        true,
	"xcl-label":               true,
	"xcl-type-label":          true,
	"xcl-name-label":          true,
	"xcl-attribute":           true,
	"xcl-string":              true,
	"xcl-string-escape":       true,
	"xcl-heredoc":             true,
	"xcl-heredoc-marker":      true,
	"xcl-heredoc-operator":    true,
	"xcl-number":              true,
	"xcl-constant":            true,
	"xcl-function":            true,
	"xcl-builtin-function":    true,
	"xcl-reference":           true,
	"xcl-member":              true,
	"xcl-operator":            true,
	"xcl-punctuation":         true,
	"xcl-interpolation-begin": true,
	"xcl-interpolation-end":   true,
	"xcl-template-directive":  true,
	"xcl-accessor":            true,
	"xcl-comment":             true,
	"xcl-line-comment":        true,
	"xcl-hash-comment":        true,
	"xcl-block-comment":       true,
	"xcl-diff":                true,
	"xcl-inserted":            true,
	"xcl-deleted":             true,
	"xcl-changed":             true,
	"xcl-problem":             true,
	"xcl-error":               true,
	"xcl-warning":             true,
	"xcl-report":              true,
	"xcl-summary":             true,
	"xcl-location":            true,
	"xcl-gutter":              true,
	"xcl-detail":              true,
}

// classAttribute captures the value of a class attribute
var classAttribute = regexp.MustCompile(`class="([^"]*)"`)

// htmlOne formats a single token with formatter and theme
func htmlOne(t *testing.T, formatter highlight.Formatter, theme highlight.Theme, kind highlight.Kind, text string) string {
	t.Helper()

	return format(t, formatter, theme, []highlight.Token{{Kind: kind, Text: text}})
}

// htmlOfSample formats the sample with formatter and the default theme
func htmlOfSample(t *testing.T, formatter highlight.Formatter) string {
	t.Helper()

	return format(t, formatter, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))
}

func TestHTMLWrapsTheOutputInAPreWithTheContainerClass(t *testing.T) {
	output := htmlOfSample(t, highlight.HTML)

	require.True(t, strings.HasPrefix(output, `<pre class="xcl">`), output)
	require.True(t, strings.HasSuffix(output, `</pre>`), output)
}

func TestHTMLOfNoTokensIsAnEmptyPre(t *testing.T) {
	output := format(t, highlight.HTML, highlight.DefaultTheme, nil)

	require.Equal(t, `<pre class="xcl"></pre>`, output)
}

func TestHTMLGivesAChildKindItsParentClassThenItsOwn(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.LineComment, "// a")

	require.Equal(t, `<pre class="xcl"><span class="xcl-comment xcl-line-comment">// a</span></pre>`, output)
}

func TestHTMLGivesACategoryKindOnlyItsOwnClass(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.Attribute, "a")

	require.Equal(t, `<pre class="xcl"><span class="xcl-attribute">a</span></pre>`, output)
}

func TestHTMLGivesAnUnknownKindItsParentClass(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.Kind(1105), "// a")

	require.Equal(t, `<pre class="xcl"><span class="xcl-comment">// a</span></pre>`, output)
}

func TestHTMLWritesTextWithoutASpan(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.Text, " {\n}\n")

	require.Equal(t, "<pre class=\"xcl\"> {\n}\n</pre>", output)
}

func TestHTMLWritesAKindWithNoClassWithoutASpan(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.Kind(9901), "x")

	require.Equal(t, `<pre class="xcl">x</pre>`, output)
}

func TestHTMLEscapesText(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.Text, `<&"'>`)

	require.Equal(t, `<pre class="xcl">&lt;&amp;&quot;&#39;&gt;</pre>`, output)
}

func TestHTMLEscapesTheTextOfASpan(t *testing.T) {
	output := htmlOne(t, highlight.HTML, highlight.DefaultTheme, highlight.String, `"<a & b>"`)

	require.Equal(t, `<pre class="xcl"><span class="xcl-string">&quot;&lt;a &amp; b&gt;&quot;</span></pre>`, output)
}

func TestHTMLWritesTheSampleTextEscaped(t *testing.T) {
	output := htmlOfSample(t, highlight.HTML)

	require.Contains(t, output, `<span class="xcl-label xcl-name-label">&quot;onprem&quot;</span>`)
	require.NotContains(t, output, `"onprem"`)
}

func TestHTMLHasNoInlineStyles(t *testing.T) {
	theme := highlight.DefaultTheme.With(highlight.Theme{highlight.Text: "bg:#000000 #ffffff"})

	output := format(t, highlight.HTML, theme, highlight.Tokenize(readSample(t)))

	require.NotContains(t, output, "style=")
}

func TestHTMLIgnoresTheTheme(t *testing.T) {
	tokens := highlight.Tokenize(readSample(t))

	withDefault := format(t, highlight.HTML, highlight.DefaultTheme, tokens)
	withOther := format(t, highlight.HTML, highlight.Theme{highlight.Attribute: "bold #ff8800"}, tokens)

	require.Equal(t, withDefault, withOther)
}

func TestHTMLUsesOnlyTheDocumentedClassNames(t *testing.T) {
	output := htmlOfSample(t, highlight.HTML)

	matches := classAttribute.FindAllStringSubmatch(output, -1)
	require.NotEmpty(t, matches)

	for _, match := range matches {
		for _, class := range strings.Fields(match[1]) {
			require.True(t, documentedClasses[class], "class %q is not documented", class)
		}
	}
}

func TestEveryKindClassIsDocumented(t *testing.T) {
	for _, kind := range highlight.Kinds() {
		require.True(t, documentedClasses[kind.Class()], "class %q of %s is not documented", kind.Class(), kind)
	}
}

func TestHTMLReturnsTheWriterError(t *testing.T) {
	err := highlight.HTML.Format(failingWriter{}, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	require.ErrorIs(t, err, errWriteFailed)
}

func TestHTMLInlineStylesSpans(t *testing.T) {
	output := htmlOfSample(t, highlight.HTMLInline)

	require.Contains(t, output, `style="`)
}

func TestHTMLInlineHasNoClasses(t *testing.T) {
	output := htmlOfSample(t, highlight.HTMLInline)

	require.NotContains(t, output, "class=")
}

func TestHTMLInlineWritesAStyledKindInASpan(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "#ff8800"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Attribute, "a")

	require.Equal(t, `<pre><span style="color: #ff8800">a</span></pre>`, output)
}

func TestHTMLInlineWritesANamedColourAsHex(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "bright-red"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Attribute, "a")

	require.Equal(t, `<pre><span style="color: #f14c4c">a</span></pre>`, output)
}

func TestHTMLInlineWritesAnUnstyledKindWithoutASpan(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "blue"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Operator, "=")

	require.Equal(t, `<pre>=</pre>`, output)
}

func TestHTMLInlineWritesThePreWithoutAStyleWhenTextIsUnstyled(t *testing.T) {
	output := htmlOfSample(t, highlight.HTMLInline)

	require.True(t, strings.HasPrefix(output, "<pre>"), output)
	require.True(t, strings.HasSuffix(output, "</pre>"), output)
}

func TestHTMLInlineStylesThePreWhenTextIsStyled(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "#eeeeee bg:#111111"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Text, "x")

	require.Equal(t, `<pre style="color: #eeeeee; background-color: #111111">x</pre>`, output)
}

func TestHTMLInlineWritesStyledTextWithoutASpan(t *testing.T) {
	theme := highlight.Theme{highlight.Text: "bold"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Text, "x")

	require.Equal(t, `<pre style="font-weight: bold">x</pre>`, output)
}

func TestHTMLInlineWritesDeclarationsInOrder(t *testing.T) {
	theme := highlight.Theme{highlight.Attribute: "dim underline italic bold bg:#102030 red"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.Attribute, "a")

	require.Equal(t,
		`<pre><span style="color: #cd3131; background-color: #102030; font-weight: bold; font-style: italic; text-decoration: underline; opacity: 0.5">a</span></pre>`,
		output)
}

func TestHTMLInlineStylesAKindByItsCategory(t *testing.T) {
	theme := highlight.Theme{highlight.Comment: "italic"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.HashComment, "# a")

	require.Equal(t, `<pre><span style="font-style: italic"># a</span></pre>`, output)
}

func TestHTMLInlineEscapesText(t *testing.T) {
	theme := highlight.Theme{highlight.String: "yellow"}

	output := htmlOne(t, highlight.HTMLInline, theme, highlight.String, `<&"'>`)

	require.Equal(t, `<pre><span style="color: #e5e510">&lt;&amp;&quot;&#39;&gt;</span></pre>`, output)
}

func TestHTMLInlineReturnsTheWriterError(t *testing.T) {
	err := highlight.HTMLInline.Format(failingWriter{}, highlight.DefaultTheme, highlight.Tokenize(readSample(t)))

	require.ErrorIs(t, err, errWriteFailed)
}

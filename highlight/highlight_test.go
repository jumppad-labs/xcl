package highlight_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// errWriteFailed is the error every write to a failingWriter returns
var errWriteFailed = errors.New("disk on fire")

// failingWriter is an io.Writer whose every write fails with errWriteFailed
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

// readSample reads testdata/sample.xcl
func readSample(t *testing.T) []byte {
	t.Helper()

	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	return input
}

// format formats tokens through formatter with theme, failing the test when
// formatting fails
func format(t *testing.T, formatter highlight.Formatter, theme highlight.Theme, tokens []highlight.Token) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, formatter.Format(&out, theme, tokens))

	return out.String()
}

func TestPlainWritesTheSampleInputUnchanged(t *testing.T) {
	input := readSample(t)

	require.Equal(t, string(input), renderText(t, highlight.Plain, input))
}

func TestPlainWritesMalformedInputUnchanged(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	require.Equal(t, string(input), renderText(t, highlight.Plain, input))
}

func TestPlainWritesNoTokensAsNothing(t *testing.T) {
	output := format(t, highlight.Plain, highlight.DefaultTheme, nil)

	require.Empty(t, output)
}

func TestPlainIgnoresTheTheme(t *testing.T) {
	tokens := []highlight.Token{
		{Kind: highlight.BlockType, Text: "resource"},
		{Kind: highlight.Text, Text: " "},
		{Kind: highlight.NameLabel, Text: `"a"`},
	}
	theme := highlight.Theme{highlight.BlockType: "bold red", highlight.NameLabel: "#ff8800"}

	output := format(t, highlight.Plain, theme, tokens)

	require.Equal(t, `resource "a"`, output)
}

func TestPlainReturnsTheWriterError(t *testing.T) {
	tokens := []highlight.Token{{Kind: highlight.BlockType, Text: "resource"}}

	err := highlight.Plain.Format(failingWriter{}, highlight.DefaultTheme, tokens)

	require.ErrorIs(t, err, errWriteFailed)
}

func TestFormatterFuncFormatsEachTokenInOrder(t *testing.T) {
	output := renderText(t, markers, []byte(`variable "cpu" {}`))

	require.Equal(t, `[storage.type.xcl]variable[/storage.type.xcl] [entity.name.tag.xcl]"cpu"[/entity.name.tag.xcl] {}`, output)
}

func TestFormatterFuncSeesEveryTokenAtOnce(t *testing.T) {
	counted := highlight.FormatterFunc(func(w io.Writer, _ highlight.Theme, tokens []highlight.Token) error {
		_, err := io.WriteString(w, strconv.Itoa(len(tokens)))
		return err
	})

	output := renderText(t, counted, []byte(`variable "cpu" {}`))

	require.Equal(t, "4", output)
}

func TestFormatterFuncIsGivenTheTheme(t *testing.T) {
	var given highlight.Theme
	capture := highlight.FormatterFunc(func(_ io.Writer, theme highlight.Theme, _ []highlight.Token) error {
		given = theme
		return nil
	})
	theme := highlight.Theme{highlight.Attribute: "#ff8800"}

	require.NoError(t, capture.Format(io.Discard, theme, nil))

	require.Equal(t, highlight.Theme{highlight.Attribute: "#ff8800"}, given)
}

// joined is the text of tokens, in order
func joined(tokens []highlight.Token) string {
	text := ""
	for _, token := range tokens {
		text += token.Text
	}

	return text
}

// requireWellFormed fails unless no token is empty and no two neighbours
// share a kind
func requireWellFormed(t *testing.T, tokens []highlight.Token) {
	t.Helper()

	for i, token := range tokens {
		require.NotEmpty(t, token.Text, "token %d is empty", i)

		if i > 0 {
			require.NotEqual(t, tokens[i-1].Kind, token.Kind, "tokens %d and %d share kind %s", i-1, i, token.Kind)
		}
	}
}

func TestTokenizeJoinsToTheSampleInput(t *testing.T) {
	input := readSample(t)

	require.Equal(t, string(input), joined(highlight.Tokenize(input)))
}

func TestTokenizeJoinsToTheReferencesInput(t *testing.T) {
	input, err := os.ReadFile("testdata/references.xcl")
	require.NoError(t, err)

	require.Equal(t, string(input), joined(highlight.Tokenize(input)))
}

func TestTokenizeJoinsToMalformedInput(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	require.Equal(t, string(input), joined(highlight.Tokenize(input)))
}

func TestTokenizeOfEmptyInputIsNoTokens(t *testing.T) {
	require.Empty(t, highlight.Tokenize(nil))
}

func TestTokenizeOfTheSampleIsWellFormed(t *testing.T) {
	requireWellFormed(t, highlight.Tokenize(readSample(t)))
}

func TestTokenizeOfTheReferencesIsWellFormed(t *testing.T) {
	input, err := os.ReadFile("testdata/references.xcl")
	require.NoError(t, err)

	requireWellFormed(t, highlight.Tokenize(input))
}

func TestTokenizeOfMalformedInputIsWellFormed(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	requireWellFormed(t, highlight.Tokenize(input))
}

func FuzzTokenizeKeepsText(f *testing.F) {
	f.Add([]byte(`variable "cpu" { default = 2 }`))
	f.Add([]byte(`x = "a ${var.b} c"`))
	f.Add([]byte("x = <<EOF\nno end\n"))
	f.Add([]byte("\xff\xfe"))
	f.Add([]byte("/* never closed"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, input []byte) {
		tokens := highlight.Tokenize(input)

		require.Equal(t, string(input), joined(tokens))
		requireWellFormed(t, tokens)
	})
}

func FuzzPlainKeepsText(f *testing.F) {
	sample, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(f, err)

	references, err := os.ReadFile("testdata/references.xcl")
	require.NoError(f, err)

	f.Add(sample)
	f.Add(references)
	f.Add([]byte(`x = "abc`))
	f.Add([]byte(`x = "${`))
	f.Add([]byte("${"))
	f.Add([]byte("x = <<EOF\nno end\n"))
	f.Add([]byte("x = <<-EOF\n  #{{ .Vars.a\n  EOF\n"))
	f.Add([]byte("\xff\xfe"))
	f.Add([]byte(`x = "a\xffb"`))
	f.Add([]byte(`x = "%{ if x }yes%{ endif }"`))
	f.Add([]byte("/* never closed"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, input []byte) {
		require.Equal(t, string(input), renderText(t, highlight.Plain, input))
	})
}

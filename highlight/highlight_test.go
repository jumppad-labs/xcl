package highlight_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

func TestTextRendersTheSampleInputUnchanged(t *testing.T) {
	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	require.Equal(t, string(input), renderText(highlight.Text, input))
}

func TestTextRendersMalformedInputUnchanged(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	require.Equal(t, string(input), renderText(highlight.Text, input))
}

func TestTextRendersNoPiecesAsNothing(t *testing.T) {
	require.Empty(t, highlight.Text.Render(nil))
}

func TestPieceFuncRendersEachPieceInOrder(t *testing.T) {
	output := renderText(markers, []byte(`variable "cpu" {}`))

	require.Equal(t, `[storage.type.xcl]variable[/storage.type.xcl] [entity.name.tag.xcl]"cpu"[/entity.name.tag.xcl] {}`, output)
}

func TestRendererFuncSeesEveryPieceAtOnce(t *testing.T) {
	counted := highlight.RendererFunc(func(pieces []highlight.Piece) []byte {
		return []byte(strconv.Itoa(len(pieces)))
	})

	output := renderText(counted, []byte(`variable "cpu" {}`))

	require.Equal(t, "4", output)
}

// joined is the text of pieces, in order
func joined(pieces []highlight.Piece) string {
	text := ""
	for _, p := range pieces {
		text += p.Text
	}

	return text
}

// requireWellFormed fails unless no piece is empty and no two neighbours
// share a scope
func requireWellFormed(t *testing.T, pieces []highlight.Piece) {
	t.Helper()

	for i, p := range pieces {
		require.NotEmpty(t, p.Text, "piece %d is empty", i)

		if i > 0 {
			require.NotEqual(t, pieces[i-1].Scope, p.Scope, "pieces %d and %d share scope %q", i-1, i, p.Scope)
		}
	}
}

func TestPiecesJoinToTheSampleInput(t *testing.T) {
	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	require.Equal(t, string(input), joined(highlight.Pieces(input)))
}

func TestPiecesJoinToTheReferencesInput(t *testing.T) {
	input, err := os.ReadFile("testdata/references.xcl")
	require.NoError(t, err)

	require.Equal(t, string(input), joined(highlight.Pieces(input)))
}

func TestPiecesJoinToMalformedInput(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	require.Equal(t, string(input), joined(highlight.Pieces(input)))
}

func TestPiecesOfEmptyInputAreNone(t *testing.T) {
	require.Empty(t, highlight.Pieces(nil))
}

func TestPiecesOfTheSampleAreWellFormed(t *testing.T) {
	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	requireWellFormed(t, highlight.Pieces(input))
}

func FuzzPiecesKeepText(f *testing.F) {
	f.Add([]byte(`variable "cpu" { default = 2 }`))
	f.Add([]byte(`x = "a ${var.b} c"`))
	f.Add([]byte("x = <<EOF\nno end\n"))
	f.Add([]byte("\xff\xfe"))
	f.Add([]byte("/* never closed"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, input []byte) {
		pieces := highlight.Pieces(input)

		require.Equal(t, string(input), joined(pieces))
		requireWellFormed(t, pieces)
	})
}

func FuzzTextKeepsText(f *testing.F) {
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
		require.Equal(t, string(input), renderText(highlight.Text, input))
	})
}

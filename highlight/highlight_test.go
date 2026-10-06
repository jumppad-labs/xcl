package highlight_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// identity returns every piece unchanged
var identity = highlight.RendererFunc(func(scope, text string) string {
	return text
})

func TestTextWithIdentityRendererReturnsSampleInput(t *testing.T) {
	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	output := highlight.Text(input, identity)

	require.Equal(t, string(input), string(output))
}

func TestTextWithIdentityRendererReturnsReferencesInput(t *testing.T) {
	input, err := os.ReadFile("testdata/references.xcl")
	require.NoError(t, err)

	output := highlight.Text(input, identity)

	require.Equal(t, string(input), string(output))
}

func TestTextWithIdentityRendererReturnsMalformedInput(t *testing.T) {
	input := []byte("resource \"a\" {\n  x = \"abc\n  y = ${\n  z = <<EOF\nnever closed\n")

	output := highlight.Text(input, identity)

	require.Equal(t, string(input), string(output))
}

func TestTextWithNilRendererReturnsInput(t *testing.T) {
	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	output := highlight.Text(input, nil)

	require.Equal(t, string(input), string(output))
}

func TestTextWithMarkerRendererChangesOutput(t *testing.T) {
	input := []byte(`variable "cpu" {}`)

	output := highlight.Text(input, markers)

	require.Equal(t, `[storage.type.xcl]variable[/storage.type.xcl] [entity.name.tag.xcl]"cpu"[/entity.name.tag.xcl] {}`, string(output))
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
		output := highlight.Text(input, identity)

		require.Equal(t, string(input), string(output))
	})
}

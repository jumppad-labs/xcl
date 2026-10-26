package highlight_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// probeTokens are formatted to tell formatters apart: Terminal colours the
// block type with the default theme, Plain writes the text alone
var probeTokens = []highlight.Token{
	{Kind: highlight.BlockType, Text: "resource"},
	{Kind: highlight.Text, Text: " "},
	{Kind: highlight.NameLabel, Text: `"a"`},
}

// probe formats probeTokens with formatter and the default theme
func probe(t *testing.T, formatter highlight.Formatter) string {
	t.Helper()

	require.NotNil(t, formatter)

	return format(t, formatter, highlight.DefaultTheme, probeTokens)
}

// requirePlain fails unless formatter writes as Plain does
func requirePlain(t *testing.T, formatter highlight.Formatter) {
	t.Helper()

	output := probe(t, formatter)

	require.Equal(t, `resource "a"`, output)
	require.NotContains(t, output, "\x1b[")
}

// requireTerminal fails unless formatter writes as Terminal does
func requireTerminal(t *testing.T, formatter highlight.Formatter) {
	t.Helper()

	output := probe(t, formatter)

	require.Equal(t, "\x1b[1;35mresource\x1b[0m \x1b[32m\"a\"\x1b[0m", output)
}

// openCharDevice opens /dev/null for writing, skipping the test where it
// cannot be opened or is not a character device
func openCharDevice(t *testing.T) *os.File {
	t.Helper()

	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("cannot open %s: %s", os.DevNull, err)
	}
	t.Cleanup(func() { file.Close() })

	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		t.Skipf("%s is not a character device", os.DevNull)
	}

	return file
}

func TestForABufferIsPlain(t *testing.T) {
	requirePlain(t, highlight.For(&bytes.Buffer{}))
}

func TestForARegularFileIsPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	file, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	require.NoError(t, err)
	t.Cleanup(func() { file.Close() })

	requirePlain(t, highlight.For(file))
}

func TestForANilFileIsPlain(t *testing.T) {
	var file *os.File

	requirePlain(t, highlight.For(file))
}

func TestForAClosedFileIsPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("cannot open %s: %s", os.DevNull, err)
	}
	require.NoError(t, file.Close())

	requirePlain(t, highlight.For(file))
}

func TestForACharacterDeviceIsTerminal(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	file := openCharDevice(t)

	requireTerminal(t, highlight.For(file))
}

func TestForACharacterDeviceWithNoColorSetIsPlain(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	file := openCharDevice(t)

	requirePlain(t, highlight.For(file))
}

func TestPlainIsPlain(t *testing.T) {
	requirePlain(t, highlight.Plain)
}

func TestTerminalIsTerminal(t *testing.T) {
	requireTerminal(t, highlight.Terminal)
}

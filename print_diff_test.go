package xcl

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// The plain output of each fixed change summary is compared with the output
// captured before printing was rebuilt in print_fixtures_test.go.

// errDiskFull is the error failingWriter fails with
var errDiskFull = errors.New("disk full")

// failingWriter fails every write
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errDiskFull
}

// printedDiff returns d printed with options
func printedDiff(t *testing.T, d *diff.Diff, options ...PrintOption) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintDiff(&out, d, options...))

	return out.String()
}

func TestDiffTokensJoinToThePlainPrintedDiff(t *testing.T) {
	d := printFixtureDiff(t, "example").diff

	require.Equal(t, printedDiff(t, d), joinedTokenText(d.Tokens()))
}

func TestPrintDiffOfANilDiffPrintsASummaryWithNoChanges(t *testing.T) {
	require.Equal(t, "Diff: no changes, 0 unchanged.\n", printedDiff(t, nil))
}

func TestPrintDiffForATerminalWritesColourCodes(t *testing.T) {
	d := printFixtureDiff(t, "example").diff

	require.Contains(t, printedDiff(t, d, WithFormatter(highlight.Terminal)), "\x1b[")
}

func TestPrintDiffForATerminalStripsToThePlainDiff(t *testing.T) {
	d := printFixtureDiff(t, "example").diff

	coloured := printedDiff(t, d, WithFormatter(highlight.Terminal))

	require.Equal(t, printedDiff(t, d), testutil.StripANSI(coloured))
}

func TestPrintDiffReturnsTheErrorWritingToTheDestination(t *testing.T) {
	d := printFixtureDiff(t, "example").diff

	err := PrintDiff(failingWriter{}, d)

	require.ErrorIs(t, err, errDiskFull)
}

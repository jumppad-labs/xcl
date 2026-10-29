package report_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/report"
)

const longDetail = "An argument named \"modls\" is not expected here. Did you mean \"models\"? " +
	"Arguments are checked against the block's schema when the configuration is read."

func TestWrapBreaksLinesAtTheWidth(t *testing.T) {
	wrapped := report.Wrap(longDetail, 40)

	lines := strings.Split(wrapped, "\n")
	require.Greater(t, len(lines), 1)
	for _, line := range lines {
		require.LessOrEqual(t, len(line), 40)
	}
}

func TestWrapKeepsEveryWord(t *testing.T) {
	wrapped := report.Wrap(longDetail, 40)

	require.Equal(t, strings.Fields(longDetail), strings.Fields(wrapped))
}

func TestWrapAtZeroLeavesTheDetailWhole(t *testing.T) {
	require.Equal(t, longDetail, report.Wrap(longDetail, 0))
}

func TestWrapAtANegativeWidthLeavesTheDetailWhole(t *testing.T) {
	require.Equal(t, longDetail, report.Wrap(longDetail, -1))
}

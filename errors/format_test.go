package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// formatDiagnostic is the unsupported argument in source, in agent.claude
func formatDiagnostic() *Diagnostic {
	d := NewDiagnostic(unsupported(), sources())
	d.Entity = "agent.claude"

	return d
}

// expectedReport is the plain report of formatDiagnostic
const expectedReport = "Error: Unsupported argument\n" +
	"\n" +
	"  on main.xcl line 2, in agent.claude:\n" +
	"     1 | agent \"claude\" {\n" +
	"     2 |   modls = [\"opus\"]\n" +
	"     3 | }\n" +
	"\n" +
	"An argument named \"modls\" is not expected here.\n" +
	"\n"

func TestDiagnosticFormattedWithVIsOneLine(t *testing.T) {
	d := formatDiagnostic()

	require.Equal(t, d.Error(), fmt.Sprintf("%v", d))
}

func TestDiagnosticFormattedWithSIsOneLine(t *testing.T) {
	d := formatDiagnostic()

	require.Equal(t, d.Error(), fmt.Sprintf("%s", d))
}

func TestDiagnosticFormattedWithQIsTheQuotedLine(t *testing.T) {
	d := formatDiagnostic()

	require.Equal(t, fmt.Sprintf("%q", d.Error()), fmt.Sprintf("%q", d))
}

func TestDiagnosticFormattedWithPlusVIsTheReportWithItsContext(t *testing.T) {
	require.Equal(t, expectedReport, fmt.Sprintf("%+v", formatDiagnostic()))
}

func TestDiagnosticFormattedWithAnUnknownVerbSaysSo(t *testing.T) {
	d := formatDiagnostic()

	require.Equal(t, "%!d(*errors.Diagnostic="+d.Error()+")", fmt.Sprintf("%d", d))
}

func TestDiagnosticWrappedWithWIsItsLine(t *testing.T) {
	d := formatDiagnostic()

	require.Equal(t, "loading: "+d.Error(), fmt.Errorf("loading: %w", d).Error())
}

func TestConfigErrorFormattedWithVIsItsLines(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(formatDiagnostic())
	ce.AppendError(errors.New("boom"))

	require.Equal(t, ce.Error(), fmt.Sprintf("%v", ce))
	require.Equal(t, ce.Error(), fmt.Sprintf("%s", ce))
}

func TestConfigErrorFormattedWithPlusVIsEveryReportAndMessage(t *testing.T) {
	warning := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Deprecated block"}, nil)

	ce := NewConfigError()
	ce.AppendError(Diagnostics{formatDiagnostic(), warning})
	ce.AppendError(errors.New("boom"))

	expected := expectedReport +
		"Warning: Deprecated block\n" +
		"\n" +
		"boom\n"
	require.Equal(t, expected, fmt.Sprintf("%+v", ce))
}

func TestConfigErrorWithoutADiagnosticFormattedWithPlusVIsItsMessage(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(errors.New("boom"))

	require.Equal(t, "boom\n", fmt.Sprintf("%+v", ce))
}

func TestFormattedWithPlusVTheDetailIsWrappedAtEightyColumns(t *testing.T) {
	d := formatDiagnostic()
	d.Detail = strings.Repeat("An argument with this name is not expected in this block. ", 4)

	report := fmt.Sprintf("%+v", d)

	detail := report[strings.Index(report, "An argument with"):]
	lines := strings.Split(strings.TrimSpace(detail), "\n")
	require.Greater(t, len(lines), 1)
	for _, line := range lines {
		require.LessOrEqual(t, len(line), 80)
	}
}

func TestConfigErrorWithOneProblemFormattedWithVIsASingleLine(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(formatDiagnostic())

	require.NotContains(t, fmt.Sprintf("%v", ce), "\n")
}

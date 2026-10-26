package xcl

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// reportSource is a file of configuration with a fault on line 2
const reportSource = "agent \"claude\" {\n  modls = [\"opus\"]\n}\n"

// modls is the range of `modls` in reportSource
var modls = &hcl.Range{
	Filename: "main.xcl",
	Start:    hcl.Pos{Line: 2, Column: 3, Byte: 19},
	End:      hcl.Pos{Line: 2, Column: 8, Byte: 24},
}

// reportDiagnostic is the diagnostic for `modls` in reportSource, made as
// the parser makes them
func reportDiagnostic(severity hcl.DiagnosticSeverity, subject *hcl.Range) *xclerrors.Diagnostic {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: severity,
		Summary:  "Unsupported argument",
		Detail:   `An argument named "modls" is not expected here.`,
		Subject:  subject,
	}, map[string][]byte{"main.xcl": []byte(reportSource)})
	d.Entity = "agent.claude"

	return d
}

// reportMarkers wraps each labelled token as [Kind]text[/Kind], naming the
// kind, and writes Text tokens as they are
var reportMarkers = highlight.FormatterFunc(func(w io.Writer, _ highlight.Theme, tokens []highlight.Token) error {
	for _, token := range tokens {
		if token.Kind == highlight.Text {
			if _, err := io.WriteString(w, token.Text); err != nil {
				return err
			}

			continue
		}

		name := token.Kind.String()
		if _, err := io.WriteString(w, "["+name+"]"+token.Text+"[/"+name+"]"); err != nil {
			return err
		}
	}

	return nil
})

// plainReport prints diagnostics as plain text, the detail wrapped at width
func plainReport(t *testing.T, diagnostics xclerrors.Diagnostics, width int) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintError(&out, diagnostics, WithWidth(width)))

	return out.String()
}

// markedReport prints diagnostics through reportMarkers, so a test can read
// the kind each token was labelled with
func markedReport(t *testing.T, diagnostics xclerrors.Diagnostics) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintError(&out, diagnostics, WithWidth(0), WithFormatter(reportMarkers)))

	return out.String()
}

func TestErrorReportLaysOutAnError(t *testing.T) {
	report := plainReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)

	expected := "Error: Unsupported argument\n" +
		"\n" +
		"  on main.xcl line 2, in agent.claude:\n" +
		"     1 | agent \"claude\" {\n" +
		"     2 |   modls = [\"opus\"]\n" +
		"     3 | }\n" +
		"\n" +
		"An argument named \"modls\" is not expected here.\n" +
		"\n"
	require.Equal(t, expected, report)
}

func TestErrorReportLabelsTheSeverityAsAnError(t *testing.T) {
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)})

	require.True(t, strings.HasPrefix(report, "[Error]Error[/Error][Summary]: "))
}

func TestErrorReportLabelsTheFaultAsAnError(t *testing.T) {
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)})

	require.Contains(t, report, "[Error]modls[/Error]")
}

func TestErrorReportLabelsTheRestOfTheSourceAsConfiguration(t *testing.T) {
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)})

	require.Contains(t, report, `[String]"opus"[/String]`)
}

func TestErrorReportLabelsAWarning(t *testing.T) {
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagWarning, modls)})

	require.True(t, strings.HasPrefix(report, "[Warning]Warning[/Warning][Summary]: "))
	require.Contains(t, report, "[Warning]modls[/Warning]")
}

func TestErrorReportNamesTheField(t *testing.T) {
	d := reportDiagnostic(hcl.DiagError, modls)
	d.Field = "models"

	require.Contains(t, plainReport(t, xclerrors.Diagnostics{d}, 0), "  on main.xcl line 2, in agent.claude, field models:\n")
}

func TestErrorReportWithoutASubjectHasNoLocation(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "No configuration", Detail: "Nothing was found to read."}, nil)

	require.Equal(t, "Error: No configuration\n\nNothing was found to read.\n\n", plainReport(t, xclerrors.Diagnostics{d}, 0))
}

func TestErrorReportWithoutSourceSaysSo(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unsupported argument", Subject: modls}, nil)

	require.Contains(t, plainReport(t, xclerrors.Diagnostics{d}, 0), "  on main.xcl line 2:\n  (source code not available)\n")
}

func TestErrorReportNeverReadsTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(reportSource), 0o600))

	subject := *modls
	subject.Filename = path
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unsupported argument", Subject: &subject}, nil)

	report := plainReport(t, xclerrors.Diagnostics{d}, 0)

	require.Contains(t, report, "(source code not available)")
	require.NotContains(t, report, "modls")
}

func TestErrorReportMarksAFaultAcrossLines(t *testing.T) {
	subject := &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 1, Column: 16},
		End:      hcl.Pos{Line: 3, Column: 2},
	}
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, subject)})

	require.Contains(t, report, "[Error]{[/Error]")
	require.Contains(t, report, `[Error]  modls = ["opus"][/Error]`)
	require.Contains(t, report, "[Gutter]     3 | [/Gutter][Error]}[/Error]")
}

func TestErrorReportMarksAnEmptyFaultAsOneCharacter(t *testing.T) {
	subject := &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 2, Column: 3},
		End:      hcl.Pos{Line: 2, Column: 3},
	}
	report := markedReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, subject)})

	require.Contains(t, report, "[Error]m[/Error][Attribute]odls[/Attribute]")
}

func TestErrorReportCountsColumnsInCharacters(t *testing.T) {
	// é is two bytes and one column, so modls starts at column 5 and byte 6
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported argument",
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 1, Column: 5},
			End:      hcl.Pos{Line: 1, Column: 10},
		},
	}, map[string][]byte{"main.xcl": []byte(`"é" modls`)})

	report := markedReport(t, xclerrors.Diagnostics{d})

	require.Contains(t, report, "[Error]modls[/Error]")
}

func TestErrorReportWrapsTheDetail(t *testing.T) {
	report := plainReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 20)

	require.Contains(t, report, "\nAn argument named\n")
}

func TestErrorReportWithoutAWidthLeavesTheDetailWhole(t *testing.T) {
	report := plainReport(t, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)

	require.Contains(t, report, "\nAn argument named \"modls\" is not expected here.\n")
}

func TestErrorReportLaysOutEachDiagnostic(t *testing.T) {
	first := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "First"}, nil)
	second := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Second"}, nil)

	require.Equal(t, "Error: First\n\nWarning: Second\n\n", plainReport(t, xclerrors.Diagnostics{first, second}, 0))
}

func TestPrintErrorForATerminalColoursTheSeverity(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, PrintError(&out, xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, WithFormatter(highlight.Terminal)))

	require.True(t, strings.HasPrefix(out.String(), "\x1b[1;31mError\x1b[0m"))
}

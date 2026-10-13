package xcl

import (
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

// reportMarkers wraps each labelled piece as [scope]text[/scope]
var reportMarkers = highlight.PieceFunc(func(p highlight.Piece) string {
	if p.Scope == "" {
		return p.Text
	}

	return "[" + p.Scope + "]" + p.Text + "[/" + p.Scope + "]"
})

func plainReport(diagnostics xclerrors.Diagnostics, width int) string {
	return string(highlight.Text.Render(DiagnosticPieces(diagnostics, width)))
}

func TestDiagnosticPiecesLayOutAnError(t *testing.T) {
	report := plainReport(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)

	expected := "Error: Unsupported argument\n" +
		"\n" +
		"  on main.xcl line 2, in agent.claude:\n" +
		"     2 |   modls = [\"opus\"]\n" +
		"\n" +
		"An argument named \"modls\" is not expected here.\n" +
		"\n"
	require.Equal(t, expected, report)
}

func TestDiagnosticPiecesLabelTheSeverityAsAnError(t *testing.T) {
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)))

	require.True(t, strings.HasPrefix(report, "[invalid.illegal.diagnostic]Error[/invalid.illegal.diagnostic]: "))
}

func TestDiagnosticPiecesLabelTheFaultAsAnError(t *testing.T) {
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)))

	require.Contains(t, report, "[invalid.illegal.diagnostic]modls[/invalid.illegal.diagnostic]")
}

func TestDiagnosticPiecesLabelTheRestOfTheSourceAsConfiguration(t *testing.T) {
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)))

	require.Contains(t, report, `[string.quoted.double.xcl]"opus"[/string.quoted.double.xcl]`)
}

func TestDiagnosticPiecesLabelAWarning(t *testing.T) {
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagWarning, modls)}, 0)))

	require.True(t, strings.HasPrefix(report, "[invalid.deprecated.diagnostic]Warning[/invalid.deprecated.diagnostic]: "))
	require.Contains(t, report, "[invalid.deprecated.diagnostic]modls[/invalid.deprecated.diagnostic]")
}

func TestDiagnosticPiecesNameTheField(t *testing.T) {
	d := reportDiagnostic(hcl.DiagError, modls)
	d.Field = "models"

	require.Contains(t, plainReport(xclerrors.Diagnostics{d}, 0), "  on main.xcl line 2, in agent.claude, field models:\n")
}

func TestDiagnosticPiecesWithoutASubjectHaveNoLocation(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "No configuration", Detail: "Nothing was found to read."}, nil)

	require.Equal(t, "Error: No configuration\n\nNothing was found to read.\n\n", plainReport(xclerrors.Diagnostics{d}, 0))
}

func TestDiagnosticPiecesWithoutSourceSaySo(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unsupported argument", Subject: modls}, nil)

	require.Contains(t, plainReport(xclerrors.Diagnostics{d}, 0), "  on main.xcl line 2:\n  (source code not available)\n")
}

func TestDiagnosticPiecesNeverReadTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(reportSource), 0o600))

	subject := *modls
	subject.Filename = path
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unsupported argument", Subject: &subject}, nil)

	report := plainReport(xclerrors.Diagnostics{d}, 0)

	require.Contains(t, report, "(source code not available)")
	require.NotContains(t, report, "modls")
}

func TestDiagnosticPiecesMarkAFaultAcrossLines(t *testing.T) {
	subject := &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 1, Column: 16},
		End:      hcl.Pos{Line: 3, Column: 2},
	}
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, subject)}, 0)))

	require.Contains(t, report, "[invalid.illegal.diagnostic]{[/invalid.illegal.diagnostic]")
	require.Contains(t, report, `[invalid.illegal.diagnostic]  modls = ["opus"][/invalid.illegal.diagnostic]`)
	require.Contains(t, report, "     3 | [invalid.illegal.diagnostic]}[/invalid.illegal.diagnostic]")
}

func TestDiagnosticPiecesMarkAnEmptyFaultAsOneCharacter(t *testing.T) {
	subject := &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 2, Column: 3},
		End:      hcl.Pos{Line: 2, Column: 3},
	}
	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, subject)}, 0)))

	require.Contains(t, report, "[invalid.illegal.diagnostic]m[/invalid.illegal.diagnostic][variable.other.property.xcl]odls[/variable.other.property.xcl]")
}

func TestDiagnosticPiecesCountColumnsInCharacters(t *testing.T) {
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

	report := string(reportMarkers.Render(DiagnosticPieces(xclerrors.Diagnostics{d}, 0)))

	require.Contains(t, report, "[invalid.illegal.diagnostic]modls[/invalid.illegal.diagnostic]")
}

func TestDiagnosticPiecesWrapTheDetail(t *testing.T) {
	report := plainReport(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 20)

	require.Contains(t, report, "\nAn argument named\n")
}

func TestDiagnosticPiecesWithoutAWidthLeaveTheDetailWhole(t *testing.T) {
	report := plainReport(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)

	require.Contains(t, report, "\nAn argument named \"modls\" is not expected here.\n")
}

func TestDiagnosticPiecesLayOutEachDiagnostic(t *testing.T) {
	first := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "First"}, nil)
	second := xclerrors.NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Second"}, nil)

	require.Equal(t, "Error: First\n\nWarning: Second\n\n", plainReport(xclerrors.Diagnostics{first, second}, 0))
}

func TestDiagnosticPiecesRenderedAsANSIColourTheSeverity(t *testing.T) {
	report := string(highlight.ANSI.Render(DiagnosticPieces(xclerrors.Diagnostics{reportDiagnostic(hcl.DiagError, modls)}, 0)))

	require.True(t, strings.HasPrefix(report, "\x1b[1;31mError\x1b[0m: "))
}

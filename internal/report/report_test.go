package report_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/report"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// modls is the range of `modls` on line 2 of a file
var modls = &hcl.Range{
	Filename: "main.xcl",
	Start:    hcl.Pos{Line: 2, Column: 3, Byte: 19},
	End:      hcl.Pos{Line: 2, Column: 8, Byte: 24},
}

// modlsProblem is an unsupported argument on line 2 of a three-line file,
// with the source of all three lines
func modlsProblem() report.Problem {
	return report.Problem{
		Summary:   "Unsupported argument",
		Detail:    `An argument named "modls" is not expected here.`,
		Entity:    "agent.claude",
		Subject:   modls,
		File:      "main.xcl",
		Start:     1,
		Lines:     []string{`agent "claude" {`, `  modls = ["opus"]`, `}`},
		HasSource: true,
	}
}

func joined(segments []report.Segment) string {
	var text strings.Builder
	for _, segment := range segments {
		text.WriteString(segment.Text)
	}

	return text.String()
}

func segmentsOf(segments []report.Segment, part report.Part) []report.Segment {
	var found []report.Segment
	for _, segment := range segments {
		if segment.Part == part {
			found = append(found, segment)
		}
	}

	return found
}

func TestLayoutWritesAnErrorReport(t *testing.T) {
	expected := "Error: Unsupported argument\n" +
		"\n" +
		"  on main.xcl line 2, in agent.claude:\n" +
		"     1 | agent \"claude\" {\n" +
		"     2 |   modls = [\"opus\"]\n" +
		"     3 | }\n" +
		"\n" +
		"An argument named \"modls\" is not expected here.\n" +
		"\n"

	require.Equal(t, expected, joined(report.Layout(modlsProblem())))
}

func TestLayoutLabelsTheSeverityOfAnError(t *testing.T) {
	segments := report.Layout(modlsProblem())

	require.Equal(t, report.Segment{Part: report.Severity, Text: "Error"}, segments[0])
}

func TestLayoutLabelsTheSummary(t *testing.T) {
	segments := report.Layout(modlsProblem())

	require.Equal(t, report.Segment{Part: report.Summary, Text: ": Unsupported argument"}, segments[1])
}

func TestLayoutLabelsTheLocation(t *testing.T) {
	locations := segmentsOf(report.Layout(modlsProblem()), report.Location)

	require.Len(t, locations, 1)
	require.Equal(t, "  on main.xcl line 2, in agent.claude:", locations[0].Text)
}

func TestLayoutLabelsAGutterForEachLine(t *testing.T) {
	gutters := segmentsOf(report.Layout(modlsProblem()), report.Gutter)

	require.Len(t, gutters, 3)
	require.Equal(t, "     1 | ", gutters[0].Text)
	require.Equal(t, "     2 | ", gutters[1].Text)
	require.Equal(t, "     3 | ", gutters[2].Text)
}

func TestLayoutLabelsTheDetailUnwrapped(t *testing.T) {
	problem := modlsProblem()
	problem.Detail = strings.Repeat("word ", 40)

	details := segmentsOf(report.Layout(problem), report.Detail)

	require.Len(t, details, 1)
	require.Equal(t, strings.Repeat("word ", 40), details[0].Text)
}

func TestLayoutMarksTheFaultInItsLine(t *testing.T) {
	sources := segmentsOf(report.Layout(modlsProblem()), report.Source)

	require.True(t, sources[1].HasFault)
	require.Equal(t, "modls", sources[1].Text[sources[1].Fault[0]:sources[1].Fault[1]])
}

func TestLayoutLeavesLinesWithoutTheFaultUnmarked(t *testing.T) {
	sources := segmentsOf(report.Layout(modlsProblem()), report.Source)

	require.False(t, sources[0].HasFault)
	require.False(t, sources[2].HasFault)
}

func TestLayoutLabelsAWarning(t *testing.T) {
	problem := modlsProblem()
	problem.Warning = true

	segments := report.Layout(problem)

	require.Equal(t, report.Segment{Part: report.Severity, Text: "Warning", Warning: true}, segments[0])
	require.True(t, segmentsOf(segments, report.Source)[1].Warning)
}

func TestLayoutNamesTheEntityAndField(t *testing.T) {
	problem := modlsProblem()
	problem.Field = "models"

	require.Contains(t, joined(report.Layout(problem)), "  on main.xcl line 2, in agent.claude, field models:\n")
}

func TestLayoutWithoutAnEntityNamesOnlyTheFileAndLine(t *testing.T) {
	problem := modlsProblem()
	problem.Entity = ""

	require.Contains(t, joined(report.Layout(problem)), "  on main.xcl line 2:\n")
}

func TestLayoutWithoutASubjectHasNoLocation(t *testing.T) {
	problem := report.Problem{Summary: "No configuration", Detail: "Nothing was found to read."}

	require.Equal(t, "Error: No configuration\n\nNothing was found to read.\n\n", joined(report.Layout(problem)))
}

func TestLayoutWithoutSourceSaysSo(t *testing.T) {
	problem := report.Problem{Summary: "Unsupported argument", Subject: modls}

	require.Equal(t, "Error: Unsupported argument\n\n  on main.xcl line 2:\n  (source code not available)\n\n", joined(report.Layout(problem)))
}

func TestLayoutWithoutDetailEndsAfterTheSource(t *testing.T) {
	problem := modlsProblem()
	problem.Detail = ""

	require.True(t, strings.HasSuffix(joined(report.Layout(problem)), "     3 | }\n\n"))
}

func TestLayoutMarksAFaultAcrossLines(t *testing.T) {
	problem := modlsProblem()
	problem.Subject = &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 1, Column: 16},
		End:      hcl.Pos{Line: 3, Column: 2},
	}

	sources := segmentsOf(report.Layout(problem), report.Source)

	require.Equal(t, "{", sources[0].Text[sources[0].Fault[0]:sources[0].Fault[1]])
	require.Equal(t, `  modls = ["opus"]`, sources[1].Text[sources[1].Fault[0]:sources[1].Fault[1]])
	require.Equal(t, "}", sources[2].Text[sources[2].Fault[0]:sources[2].Fault[1]])
}

func TestLayoutMarksAnEmptyFaultAsOneCharacter(t *testing.T) {
	problem := modlsProblem()
	problem.Subject = &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 2, Column: 3},
		End:      hcl.Pos{Line: 2, Column: 3},
	}

	sources := segmentsOf(report.Layout(problem), report.Source)

	require.Equal(t, [2]int{2, 3}, sources[1].Fault)
}

func TestLayoutCountsColumnsInCharacters(t *testing.T) {
	// é is two bytes and one column, so modls starts at column 5 and byte 6
	problem := report.Problem{
		Summary: "Unsupported argument",
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 1, Column: 5},
			End:      hcl.Pos{Line: 1, Column: 10},
		},
		File:      "main.xcl",
		Start:     1,
		Lines:     []string{`"é" modls`},
		HasSource: true,
	}

	source := segmentsOf(report.Layout(problem), report.Source)[0]

	require.Equal(t, "modls", source.Text[source.Fault[0]:source.Fault[1]])
}

func TestLayoutNeverWritesAnEmptySegment(t *testing.T) {
	problem := modlsProblem()
	problem.Lines = []string{"", `  modls = ["opus"]`, ""}

	for _, segment := range report.Layout(problem) {
		require.NotEmpty(t, segment.Text)
	}
}

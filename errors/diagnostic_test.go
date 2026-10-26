package errors

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// source is a file of configuration, its fault on line 2
const source = "agent \"claude\" {\n  modls = [\"opus\"]\n}\n"

// subject is the range of `modls` in source
var subject = &hcl.Range{
	Filename: "main.xcl",
	Start:    hcl.Pos{Line: 2, Column: 3, Byte: 19},
	End:      hcl.Pos{Line: 2, Column: 8, Byte: 24},
}

// unsupported is the diagnostic HCL gives for `modls` in source
func unsupported() *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported argument",
		Detail:   `An argument named "modls" is not expected here.`,
		Subject:  subject,
	}
}

func sources() map[string][]byte {
	return map[string][]byte{"main.xcl": []byte(source)}
}

func TestDiagnosticErrorIsOneLine(t *testing.T) {
	d := NewDiagnostic(unsupported(), sources())
	d.Entity = "agent.claude"

	require.Equal(t, `main.xcl:2,3-8: Unsupported argument; An argument named "modls" is not expected here. (agent.claude)`, d.Error())
}

func TestDiagnosticErrorNamesTheField(t *testing.T) {
	d := NewDiagnostic(unsupported(), sources())
	d.Entity = "agent.claude"
	d.Field = "models"

	require.Contains(t, d.Error(), "(agent.claude, models)")
}

func TestDiagnosticErrorWithoutASubjectHasNoPosition(t *testing.T) {
	d := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Nothing to read"}, nil)

	require.Equal(t, "Nothing to read", d.Error())
}

func TestDiagnosticErrorJoinsALongDetailIntoOneLine(t *testing.T) {
	d := NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Bad value",
		Detail:   "The value is wrong\nin two ways.",
	}, nil)

	require.Equal(t, "Bad value; The value is wrong in two ways.", d.Error())
}

func TestDiagnosticErrorHoldsNoSource(t *testing.T) {
	d := NewDiagnostic(unsupported(), sources())

	require.NotContains(t, d.Error(), `"opus"`)
}

func TestNewDiagnosticTakesTheLinesOfItsSubject(t *testing.T) {
	d := NewDiagnostic(unsupported(), sources())

	require.Equal(t, &SourceLines{File: "main.xcl", Start: 1, Lines: []string{`agent "claude" {`, `  modls = ["opus"]`, `}`}}, d.Source)
}

func TestNewDiagnosticTakesTheLinesOfItsContextToo(t *testing.T) {
	diagnostic := unsupported()
	diagnostic.Context = &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
		End:      hcl.Pos{Line: 3, Column: 2, Byte: 37},
	}

	d := NewDiagnostic(diagnostic, sources())

	require.Equal(t, 1, d.Source.Start)
	require.Len(t, d.Source.Lines, 3)
}

func TestNewDiagnosticEndingAtTheStartOfALineLeavesThatLineOut(t *testing.T) {
	diagnostic := unsupported()
	diagnostic.Subject = &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 2, Column: 3},
		End:      hcl.Pos{Line: 3, Column: 1},
	}

	d := NewDiagnostic(diagnostic, map[string][]byte{"main.xcl": []byte(longSource)})

	require.Equal(t, 1, d.Source.Start)
	require.Equal(t, []string{`agent "claude" {`, `  modls = ["opus"]`, `  effort = "high"`, `  tools = []`}, d.Source.Lines)
}

func TestNewDiagnosticTakesLinesWithoutTheirCarriageReturns(t *testing.T) {
	d := NewDiagnostic(unsupported(), map[string][]byte{"main.xcl": []byte("agent \"claude\" {\r\n  modls = [\"opus\"]\r\n}\r\n")})

	require.Equal(t, []string{`agent "claude" {`, `  modls = ["opus"]`, `}`}, d.Source.Lines)
}

func TestNewDiagnosticWithoutTheFileHasNoSource(t *testing.T) {
	d := NewDiagnostic(unsupported(), map[string][]byte{"other.xcl": []byte(source)})

	require.Nil(t, d.Source)
}

func TestNewDiagnosticWithASubjectPastTheEndHasNoSource(t *testing.T) {
	diagnostic := unsupported()
	diagnostic.Subject = &hcl.Range{Filename: "main.xcl", Start: hcl.Pos{Line: 40}, End: hcl.Pos{Line: 40}}

	d := NewDiagnostic(diagnostic, sources())

	require.Nil(t, d.Source)
}

func TestNewDiagnosticLeavesOutTheExpressionAndItsValues(t *testing.T) {
	diagnostic := unsupported()
	diagnostic.EvalContext = &hcl.EvalContext{}

	d := NewDiagnostic(diagnostic, sources())

	require.Nil(t, d.Expression)
	require.Nil(t, d.EvalContext)
}

func TestNewDiagnosticDoesNotChangeTheDiagnosticItWasMadeFrom(t *testing.T) {
	diagnostic := unsupported()
	diagnostic.EvalContext = &hcl.EvalContext{}

	NewDiagnostic(diagnostic, sources())

	require.NotNil(t, diagnostic.EvalContext)
}

func TestDiagnosticsHasErrorsWithAnError(t *testing.T) {
	diagnostics := Diagnostics{NewDiagnostic(unsupported(), nil)}

	require.True(t, diagnostics.HasErrors())
}

func TestDiagnosticsHasErrorsIsFalseWithOnlyWarnings(t *testing.T) {
	diagnostics := Diagnostics{NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Deprecated"}, nil)}

	require.False(t, diagnostics.HasErrors())
}

func TestDiagnosticsSplitIntoErrorsAndWarnings(t *testing.T) {
	warning := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagWarning, Summary: "Deprecated"}, nil)
	failure := NewDiagnostic(unsupported(), nil)
	diagnostics := Diagnostics{warning, failure}

	require.Equal(t, Diagnostics{failure}, diagnostics.Errs())
	require.Equal(t, Diagnostics{warning}, diagnostics.Warnings())
}

func TestDiagnosticsErrorIsOneLineEach(t *testing.T) {
	diagnostics := Diagnostics{
		NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "First"}, nil),
		NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Second"}, nil),
	}

	require.Equal(t, "First\nSecond", diagnostics.Error())
}

func TestADiagnosticInAConfigErrorIsFoundWithAs(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(NewDiagnostic(unsupported(), nil))

	var found *Diagnostic
	require.True(t, errors.As(ce, &found))
	require.Equal(t, "Unsupported argument", found.Summary)
}

func TestADiagnosticMatchesItsCause(t *testing.T) {
	cause := errors.New("connection refused")
	d := NewDiagnostic(unsupported(), nil)
	d.Cause = cause

	ce := NewConfigError()
	ce.AppendError(d)

	require.ErrorIs(t, ce, cause)
}

func TestADiagnosticWithoutACauseUnwrapsToNil(t *testing.T) {
	require.Nil(t, NewDiagnostic(unsupported(), nil).Unwrap())
}

func TestConfigErrorTakesDiagnosticsOneByOne(t *testing.T) {
	first := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "First"}, nil)
	second := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Second"}, nil)

	ce := NewConfigError()
	ce.AppendError(Diagnostics{first, second})

	require.Equal(t, []error{first, second}, ce.Errors)
}

func TestConfigErrorDiagnosticsLeavesOutOtherErrors(t *testing.T) {
	diagnostic := NewDiagnostic(unsupported(), nil)

	ce := NewConfigError()
	ce.AppendError(errors.New("not a diagnostic"))
	ce.AppendError(diagnostic)

	require.Equal(t, Diagnostics{diagnostic}, ce.Diagnostics())
}

// longSource is nine lines of configuration
const longSource = "agent \"claude\" {\n" +
	"  modls = [\"opus\"]\n" +
	"  effort = \"high\"\n" +
	"  tools = []\n" +
	"  timeout = 30\n" +
	"  retries = 2\n" +
	"  name = \"claude\"\n" +
	"  enabled = true\n" +
	"}\n"

// lineRange is the range of columns 3 to 8 on line
func lineRange(line int) *hcl.Range {
	return &hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: line, Column: 3},
		End:      hcl.Pos{Line: line, Column: 8},
	}
}

func diagnosticAt(subject *hcl.Range, src string) *Diagnostic {
	diagnostic := unsupported()
	diagnostic.Subject = subject

	return NewDiagnostic(diagnostic, map[string][]byte{"main.xcl": []byte(src)})
}

func TestNewDiagnosticInTheMiddleOfAFileTakesTwoLinesEitherSide(t *testing.T) {
	d := diagnosticAt(lineRange(5), longSource)

	require.Equal(t, 3, d.Source.Start)
	require.Equal(t, []string{`  effort = "high"`, `  tools = []`, `  timeout = 30`, `  retries = 2`, `  name = "claude"`}, d.Source.Lines)
}

func TestNewDiagnosticOnTheFirstLineOfAThreeLineFileTakesLinesOneToThree(t *testing.T) {
	d := diagnosticAt(lineRange(1), source)

	require.Equal(t, 1, d.Source.Start)
	require.Equal(t, []string{`agent "claude" {`, `  modls = ["opus"]`, `}`}, d.Source.Lines)
}

func TestNewDiagnosticOnTheLastLineTakesThatLineAndTheTwoBefore(t *testing.T) {
	d := diagnosticAt(lineRange(9), longSource)

	require.Equal(t, 7, d.Source.Start)
	require.Equal(t, []string{`  name = "claude"`, `  enabled = true`, `}`}, d.Source.Lines)
}

func TestNewDiagnosticInAFileEndingWithALineBreakTakesNoEmptyLineAfterIt(t *testing.T) {
	d := diagnosticAt(lineRange(3), source)

	require.Equal(t, []string{`agent "claude" {`, `  modls = ["opus"]`, `}`}, d.Source.Lines)
}

func TestNewDiagnosticInAFileEndingWithoutALineBreakTakesItsLastLine(t *testing.T) {
	d := diagnosticAt(lineRange(2), "agent \"claude\" {\n  modls = [\"opus\"]\n}")

	require.Equal(t, 1, d.Source.Start)
	require.Equal(t, []string{`agent "claude" {`, `  modls = ["opus"]`, `}`}, d.Source.Lines)
}

func TestNewDiagnosticAtTheEndOfAFileKeepsTheLineItCovers(t *testing.T) {
	// the end of a file ending with a line break is on the empty line after it
	d := diagnosticAt(&hcl.Range{Filename: "main.xcl", Start: hcl.Pos{Line: 4, Column: 1}, End: hcl.Pos{Line: 4, Column: 1}}, source)

	require.Equal(t, 2, d.Source.Start)
	require.Equal(t, []string{`  modls = ["opus"]`, `}`, ``}, d.Source.Lines)
}

func TestNewDiagnosticAcrossLinesTakesTwoLinesEitherSideOfTheSpan(t *testing.T) {
	d := diagnosticAt(&hcl.Range{
		Filename: "main.xcl",
		Start:    hcl.Pos{Line: 4, Column: 3},
		End:      hcl.Pos{Line: 6, Column: 5},
	}, longSource)

	require.Equal(t, 2, d.Source.Start)
	require.Len(t, d.Source.Lines, 7)
	require.Equal(t, `  modls = ["opus"]`, d.Source.Lines[0])
	require.Equal(t, `  enabled = true`, d.Source.Lines[6])
}

func TestNewDiagnosticNeverReadsTheFileForContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(path, []byte(longSource), 0o600))

	subject := lineRange(5)
	subject.Filename = path
	d := NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Unsupported argument", Subject: subject}, map[string][]byte{path: []byte("one\ntwo\nthree\nfour\nfive\n")})

	require.Equal(t, []string{"three", "four", "five"}, d.Source.Lines)
}

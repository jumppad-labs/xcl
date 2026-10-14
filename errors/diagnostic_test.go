package errors

import (
	"errors"
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

	require.Equal(t, &SourceLines{File: "main.xcl", Start: 2, Lines: []string{`  modls = ["opus"]`}}, d.Source)
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

	d := NewDiagnostic(diagnostic, sources())

	require.Equal(t, []string{`  modls = ["opus"]`}, d.Source.Lines)
}

func TestNewDiagnosticTakesLinesWithoutTheirCarriageReturns(t *testing.T) {
	d := NewDiagnostic(unsupported(), map[string][]byte{"main.xcl": []byte("agent \"claude\" {\r\n  modls = [\"opus\"]\r\n}\r\n")})

	require.Equal(t, []string{`  modls = ["opus"]`}, d.Source.Lines)
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
	ce.AppendError(&ParserError{Message: "old style"})
	ce.AppendError(diagnostic)

	require.Equal(t, Diagnostics{diagnostic}, ce.Diagnostics())
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package hclsyntax

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

// directiveSummary is the summary every rejected %{ directive reports
const directiveSummary = "Template directives are not supported"

// errorDiagnostics returns only the diagnostics with error severity
func errorDiagnostics(diags hcl.Diagnostics) hcl.Diagnostics {
	errs := hcl.Diagnostics{}
	for _, diag := range diags {
		if diag.Severity == hcl.DiagError {
			errs = append(errs, diag)
		}
	}

	return errs
}

func TestTemplateIfDirectiveIsAnError(t *testing.T) {
	src := `%{ if true }yes%{ endif }`

	_, diags := ParseTemplate([]byte(src), "test.xcl", hcl.InitialPos)
	require.True(t, diags.HasErrors())

	errs := errorDiagnostics(diags)
	require.Len(t, errs, 2, "each directive reports its own error")

	first := errs[0]
	require.Equal(t, directiveSummary, first.Summary)
	require.Contains(t, first.Detail, "format")
	require.Contains(t, first.Detail, "join")
	require.Contains(t, first.Detail, "template_file")

	require.NotNil(t, first.Subject)
	require.Equal(t, 1, first.Subject.Start.Line)
	require.Equal(t, 1, first.Subject.Start.Column)
	require.Equal(t, 1, first.Subject.End.Line)
	require.Equal(t, 13, first.Subject.End.Column, "the subject spans %%{ if true } which is 12 bytes")

	second := errs[1]
	require.Equal(t, directiveSummary, second.Summary)
	require.Equal(t, 16, second.Subject.Start.Column, "the second subject starts at the %%{ of endif")
}

func TestTemplateForDirectiveIsAnError(t *testing.T) {
	src := `%{ for v in ["a"] }${v}%{ endfor }`

	_, diags := ParseTemplate([]byte(src), "test.xcl", hcl.InitialPos)
	require.True(t, diags.HasErrors())

	errs := errorDiagnostics(diags)
	require.Len(t, errs, 2, "each directive reports its own error")

	require.Equal(t, directiveSummary, errs[0].Summary)
	require.NotNil(t, errs[0].Subject)
	require.Equal(t, 1, errs[0].Subject.Start.Line)
	require.Equal(t, 1, errs[0].Subject.Start.Column)

	require.Equal(t, directiveSummary, errs[1].Summary)
}

func TestHeredocDirectiveIsAnError(t *testing.T) {
	src := "<<EOT\nFoo\n%{ if true }bar%{ endif }\nEOT\n"

	_, diags := ParseExpression([]byte(src), "test.xcl", hcl.InitialPos)
	require.True(t, diags.HasErrors())

	errs := errorDiagnostics(diags)
	require.Len(t, errs, 2, "each directive reports its own error")

	first := errs[0]
	require.Equal(t, directiveSummary, first.Summary)
	require.NotNil(t, first.Subject)
	require.Equal(t, 3, first.Subject.Start.Line, "the directive is on the third line of the heredoc")
	require.Equal(t, 1, first.Subject.Start.Column)
}

func TestQuotedStringDirectiveIsAnErrorAtItsPosition(t *testing.T) {
	src := `"hello %{ if true }x%{ endif }"`

	_, diags := ParseExpression([]byte(src), "test.xcl", hcl.InitialPos)
	require.True(t, diags.HasErrors())

	errs := errorDiagnostics(diags)
	require.Len(t, errs, 2, "each directive reports its own error")

	first := errs[0]
	require.Equal(t, directiveSummary, first.Summary)
	require.NotNil(t, first.Subject)
	require.Equal(t, 1, first.Subject.Start.Line)
	require.Equal(t, 8, first.Subject.Start.Column, "the %%{ follows the opening quote and 'hello '")
}

func TestInterpolationStillParses(t *testing.T) {
	src := `hello ${name}`

	expr, diags := ParseTemplate([]byte(src), "test.xcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), diags.Error())

	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"name": cty.StringVal("world"),
		},
	}

	got, valDiags := expr.Value(ctx)
	require.False(t, valDiags.HasErrors(), valDiags.Error())
	require.True(t, cty.StringVal("hello world").RawEquals(got), "got %#v", got)
}

func TestEscapedDirectiveIsLiteral(t *testing.T) {
	src := `hello %%{ if true }world%%{ endif }`

	expr, diags := ParseTemplate([]byte(src), "test.xcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), diags.Error())

	got, valDiags := expr.Value(nil)
	require.False(t, valDiags.HasErrors(), valDiags.Error())
	require.True(t, cty.StringVal(`hello %{ if true }world%{ endif }`).RawEquals(got), "got %#v", got)
}

func TestDirectiveErrorDoesNotSuppressLaterErrors(t *testing.T) {
	// two directives followed by an interpolation that is never closed; the
	// directive errors must not put the parser into recovery and swallow the
	// later problem
	src := `%{ if true }a%{ endif } ${"a"`

	_, diags := ParseTemplate([]byte(src), "test.xcl", hcl.InitialPos)
	require.True(t, diags.HasErrors())

	errs := errorDiagnostics(diags)
	require.Len(t, errs, 3, "both directives and the unclosed interpolation are reported: %s", diags.Error())

	require.Equal(t, directiveSummary, errs[0].Summary)
	require.Equal(t, 1, errs[0].Subject.Start.Column)

	require.Equal(t, directiveSummary, errs[1].Summary)
	require.Equal(t, 14, errs[1].Subject.Start.Column)

	require.Equal(t, "Unclosed template interpolation sequence", errs[2].Summary)
}

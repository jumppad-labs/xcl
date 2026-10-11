// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

// A Go type configuration cannot be decoded into is the program's mistake,
// not the author's, but the decoder reports it as a diagnostic at a position
// in the file rather than panicking, so an application that did not validate
// its types up front still fails with an error.

type decodeStringBlockTarget struct {
	Listener string `xcl:"listener,block"`
}

type decodeInvalidTagTarget struct {
	Name string `xcl:"name,bogus"`
}

// a map keyed by anything but a string has no configuration form
type decodeMapIntAttributeTarget struct {
	Mode map[int]string `xcl:"mode"`
}

type decodeIntLabelElement struct {
	Name int `xcl:"name,label"`
}

type decodeIntLabelTarget struct {
	Listeners []decodeIntLabelElement `xcl:"listener,block"`
}

type decodeUnexportedAttributeTarget struct {
	port int `xcl:"port"` //nolint:unused // the tag is what is under test
}

// unsupportedTargetDiagnostics returns the error diagnostics with the
// Unsupported decode target summary
func unsupportedTargetDiagnostics(diags hcl.Diagnostics) hcl.Diagnostics {
	found := hcl.Diagnostics{}
	for _, diag := range diags {
		if diag.Severity == hcl.DiagError && diag.Summary == "Unsupported decode target" {
			found = append(found, diag)
		}
	}

	return found
}

func TestDecodeBodyReportsStringBlockFieldInsteadOfPanicking(t *testing.T) {
	body := parseCheckBody(t, `
listener "a" { }
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeBody(body, nil, &decodeStringBlockTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "cannot hold blocks")
	require.Contains(t, unsupported[0].Detail, "Listener")
	require.NotNil(t, unsupported[0].Subject)
}

func TestDecodeBodyReportsInvalidTagInsteadOfPanicking(t *testing.T) {
	body := parseCheckBody(t, `
name = "a"
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeBody(body, nil, &decodeInvalidTagTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "invalid xcl field tag")
	require.Contains(t, unsupported[0].Detail, "Name")
	require.NotNil(t, unsupported[0].Subject)
}

func TestDecodeBodyReportsAttributeWithoutConfigurationFormAtTheAttribute(t *testing.T) {
	body := parseCheckBody(t, `
other = 1
mode  = "x"
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeBody(body, nil, &decodeMapIntAttributeTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "no configuration form")
	require.NotNil(t, unsupported[0].Subject)
	require.Equal(t, 3, unsupported[0].Subject.Start.Line)
}

func TestDecodeBodyReportsNonStringLabelFieldAtTheLabel(t *testing.T) {
	body := parseCheckBody(t, `
listener "a" { }
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeBody(body, nil, &decodeIntLabelTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.NotEmpty(t, unsupported)
	require.Contains(t, unsupported[0].Detail, "cannot hold a label")
	require.Contains(t, unsupported[0].Detail, "Name")
	require.NotNil(t, unsupported[0].Subject)
	require.Equal(t, 2, unsupported[0].Subject.Start.Line)
}

func TestDecodeBodyReportsUnexportedAttributeFieldAtTheAttribute(t *testing.T) {
	body := parseCheckBody(t, `
port = 1
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeBody(body, nil, &decodeUnexportedAttributeTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "unexported")
	require.Contains(t, unsupported[0].Detail, "port")
	require.NotNil(t, unsupported[0].Subject)
	require.Equal(t, 2, unsupported[0].Subject.Start.Line)
}

func TestDecodeExpressionReportsTargetWithoutConfigurationForm(t *testing.T) {
	body := parseCheckBody(t, `
mode = "x"
`)

	attrs, attrDiags := body.JustAttributes()
	require.False(t, attrDiags.HasErrors(), attrDiags.Error())

	var fnTarget func()
	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = DecodeExpression(attrs["mode"].Expr, nil, &fnTarget)
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "no configuration form")
	require.NotNil(t, unsupported[0].Subject)
	require.Equal(t, 2, unsupported[0].Subject.Start.Line)
}

func TestCheckBodyReportsNonStructTarget(t *testing.T) {
	body := parseCheckBody(t, `
mode = "x"
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = CheckBody(body, "not a struct")
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "not a struct")
	require.NotNil(t, unsupported[0].Subject)
}

func TestCheckBodyReportsStringBlockFieldInsteadOfPanicking(t *testing.T) {
	body := parseCheckBody(t, `
listener "a" { }
`)

	var diags hcl.Diagnostics
	require.NotPanics(t, func() {
		diags = CheckBody(body, &decodeStringBlockTarget{})
	})

	require.True(t, diags.HasErrors())

	unsupported := unsupportedTargetDiagnostics(diags)
	require.Len(t, unsupported, 1)
	require.Contains(t, unsupported[0].Detail, "cannot hold blocks")
	require.NotNil(t, unsupported[0].Subject)
}

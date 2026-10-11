// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/stretchr/testify/require"
)

// optionalEndpoint is an object value whose tls attribute may be left out
type optionalEndpoint struct {
	Port int  `xcl:"port"`
	TLS  bool `xcl:"tls,optional"`
}

// optionalBase is embedded anonymously so its optional note attribute is
// flattened into the embedding struct's object type
type optionalBase struct {
	Note string `xcl:"note,optional"`
}

type optionalEmbedding struct {
	optionalBase

	Port int `xcl:"port"`
}

type optionalPointerHolder struct {
	Port  int               `xcl:"port"`
	Inner *optionalEndpoint `xcl:"inner,optional"`
}

func parseDecodeExpression(t *testing.T, src string) hcl.Expression {
	t.Helper()

	expr, diags := hclsyntax.ParseExpression([]byte(src), "test.xcl", hcl.InitialPos)
	require.False(t, diags.HasErrors(), diags.Error())

	return expr
}

func TestDecodeExpressionAllowsOmittedOptionalFieldInObject(t *testing.T) {
	expr := parseDecodeExpression(t, `{ port = 80 }`)

	var target optionalEndpoint
	diags := DecodeExpression(expr, nil, &target)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, 80, target.Port)
	require.False(t, target.TLS)
}

func TestDecodeExpressionAllowsOmittedOptionalFieldInMapOfObjects(t *testing.T) {
	expr := parseDecodeExpression(t, `{ http = { port = 80 }, https = { port = 443, tls = true } }`)

	var target map[string]optionalEndpoint
	diags := DecodeExpression(expr, nil, &target)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, target, 2)

	require.Equal(t, 80, target["http"].Port)
	require.False(t, target["http"].TLS)

	require.Equal(t, 443, target["https"].Port)
	require.True(t, target["https"].TLS)
}

func TestDecodeExpressionAllowsOmittedOptionalFieldInListOfObjects(t *testing.T) {
	expr := parseDecodeExpression(t, `[{ port = 80 }]`)

	var target []optionalEndpoint
	diags := DecodeExpression(expr, nil, &target)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, target, 1)
	require.Equal(t, 80, target[0].Port)
	require.False(t, target[0].TLS)
}

func TestDecodeExpressionRejectsOmittedRequiredFieldInObject(t *testing.T) {
	expr := parseDecodeExpression(t, `{ tls = true }`)

	var target optionalEndpoint
	diags := DecodeExpression(expr, nil, &target)
	require.True(t, diags.HasErrors())

	require.Equal(t, "Unsuitable value type", diags[0].Summary)
	require.Contains(t, diags[0].Detail, "port")
}

func TestDecodeExpressionAllowsOmittedOptionalFieldOfEmbeddedStruct(t *testing.T) {
	expr := parseDecodeExpression(t, `{ port = 80 }`)

	var target optionalEmbedding
	diags := DecodeExpression(expr, nil, &target)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, 80, target.Port)
	require.Equal(t, "", target.Note)
}

func TestDecodeExpressionLeavesOmittedOptionalPointerFieldNil(t *testing.T) {
	expr := parseDecodeExpression(t, `{ port = 80 }`)

	var target optionalPointerHolder
	diags := DecodeExpression(expr, nil, &target)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, 80, target.Port)
	require.Nil(t, target.Inner)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package hclsyntax

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

func parseOptionalLabelsBody(t *testing.T, src string) hcl.Body {
	t.Helper()

	f, diags := ParseConfig([]byte(src), "test.xcl", hcl.Pos{Line: 1, Column: 1})
	require.False(t, diags.HasErrors(), diags.Error())

	return f.Body
}

func optionalLabelsSchema(optional bool, labelNames ...string) *hcl.BodySchema {
	return &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "listener", LabelNames: labelNames, LabelsOptional: optional},
		},
	}
}

func TestBodyContentAcceptsBlockWithoutLabelsWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `listener { }`)

	content, diags := body.Content(optionalLabelsSchema(true, "name"))
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, content.Blocks, 1)
	require.Equal(t, "listener", content.Blocks[0].Type)
	require.Empty(t, content.Blocks[0].Labels)
}

func TestBodyContentAcceptsBlockWithLabelsWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `listener "http" { }`)

	content, diags := body.Content(optionalLabelsSchema(true, "name"))
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, content.Blocks, 1)
	require.Equal(t, []string{"http"}, content.Blocks[0].Labels)
}

func TestBodyContentRejectsBlockWithSomeLabelsWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `listener "a" { }`)

	_, diags := body.Content(optionalLabelsSchema(true, "kind", "name"))
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "Missing name for listener", diags[0].Summary)
}

func TestBodyContentRejectsBlockWithoutLabelsWhenRequired(t *testing.T) {
	body := parseOptionalLabelsBody(t, `listener { }`)

	_, diags := body.Content(optionalLabelsSchema(false, "name"))
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "Missing name for listener", diags[0].Summary)
}

func TestBodyContentRejectsExtraLabelWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `listener "http" "extra" { }`)

	_, diags := body.Content(optionalLabelsSchema(true, "name"))
	require.True(t, diags.HasErrors())

	require.Len(t, diags, 1)
	require.Equal(t, "Extraneous label for listener", diags[0].Summary)
}

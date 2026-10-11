// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package json

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

func parseOptionalLabelsBody(t *testing.T, src string) hcl.Body {
	t.Helper()

	f, diags := Parse([]byte(src), "test.json")
	require.False(t, diags.HasErrors(), diags.Error())

	return f.Body
}

func optionalLabelsSchema(optional bool) *hcl.BodySchema {
	return &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "listener", LabelNames: []string{"name"}, LabelsOptional: optional},
		},
	}
}

func TestBodyContentArrayYieldsUnlabelledBlocksWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `{"listener": [{"port": 80}, {"port": 8080}]}`)

	content, diags := body.Content(optionalLabelsSchema(true))
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, content.Blocks, 2)
	require.Equal(t, "listener", content.Blocks[0].Type)
	require.Empty(t, content.Blocks[0].Labels)
	require.Empty(t, content.Blocks[1].Labels)
}

func TestBodyContentObjectYieldsLabelledBlocksWhenOptional(t *testing.T) {
	body := parseOptionalLabelsBody(t, `{"listener": {"http": {"port": 80}, "https": {"port": 443}}}`)

	content, diags := body.Content(optionalLabelsSchema(true))
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, content.Blocks, 2)
	require.Equal(t, []string{"http"}, content.Blocks[0].Labels)
	require.Equal(t, []string{"https"}, content.Blocks[1].Labels)
}

func TestBodyContentArrayIsStillKeyedByLabelWhenRequired(t *testing.T) {
	body := parseOptionalLabelsBody(t, `{"listener": [{"http": {"port": 80}}]}`)

	content, diags := body.Content(optionalLabelsSchema(false))
	require.False(t, diags.HasErrors(), diags.Error())

	require.Len(t, content.Blocks, 1)
	require.Equal(t, []string{"http"}, content.Blocks[0].Labels)
}

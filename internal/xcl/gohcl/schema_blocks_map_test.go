// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0
// Modifications Copyright (c) Jumppad Labs

package gohcl

import (
	"testing"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

func TestImpliedBodySchemaMapBlockTakesOptionalNameLabel(t *testing.T) {
	schema, partial := ImpliedBodySchema(&mapApp{})
	require.False(t, partial)

	require.Len(t, schema.Blocks, 1)
	require.Equal(t, hcl.BlockHeaderSchema{
		Type:           "listener",
		LabelNames:     []string{"name"},
		LabelsOptional: true,
	}, schema.Blocks[0])
}

func TestImpliedBodySchemaListOfUnlabelledBlocksTakesOptionalNameLabel(t *testing.T) {
	schema, partial := ImpliedBodySchema(&unnamedListApp{})
	require.False(t, partial)

	require.Len(t, schema.Blocks, 1)
	require.Equal(t, hcl.BlockHeaderSchema{
		Type:           "listener",
		LabelNames:     []string{"name"},
		LabelsOptional: true,
	}, schema.Blocks[0])
}

func TestImpliedBodySchemaSingleBlockKeepsRequiredLabels(t *testing.T) {
	schema, partial := ImpliedBodySchema(&singleApp{})
	require.False(t, partial)

	require.Len(t, schema.Blocks, 1)
	require.Equal(t, hcl.BlockHeaderSchema{
		Type:           "listener",
		LabelNames:     []string{"name"},
		LabelsOptional: false,
	}, schema.Blocks[0])
}

func TestImpliedBodySchemaTwoLabelBlocksKeepRequiredLabels(t *testing.T) {
	schema, partial := ImpliedBodySchema(&twoLabelsApp{})
	require.False(t, partial)

	require.Len(t, schema.Blocks, 1)
	require.Equal(t, hcl.BlockHeaderSchema{
		Type:           "listener",
		LabelNames:     []string{"kind", "name"},
		LabelsOptional: false,
	}, schema.Blocks[0])
}

func TestImpliedBodySchemaPanicsOnMapBlockWithNonStringKeys(t *testing.T) {
	require.Panics(t, func() {
		ImpliedBodySchema(&intKeyApp{})
	})
}

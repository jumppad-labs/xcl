package parser

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/xcl"
	"github.com/stretchr/testify/require"
)

// contextVariables returns the variables setVariables has placed in the
// context, or an empty map when it placed none
func contextVariables(ctx *hcl.EvalContext) map[string]cty.Value {
	variables, ok := ctx.Variables["variable"]
	if !ok {
		return map[string]cty.Value{}
	}

	return variables.AsValueMap()
}

func TestSetVariablesReadsNothingFromEnvironmentWhenPrefixIsEmpty(t *testing.T) {
	t.Setenv("XCL_VAR_region", "us")
	t.Setenv("region", "us")

	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	setVariables(ctx, nil, "")

	variables := contextVariables(ctx)
	require.NotContains(t, variables, "region")
	require.NotContains(t, variables, "XCL_VAR_region")
}

func TestSetVariablesKeepsValueContainingEquals(t *testing.T) {
	t.Setenv("XCL_VAR_conn", "a=b=c")

	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	setVariables(ctx, nil, "XCL_VAR_")

	variables := contextVariables(ctx)
	require.Contains(t, variables, "conn")
	require.Equal(t, cty.StringVal("a=b=c"), variables["conn"])
}

func TestSetVariablesStripsOnlyThePrefixFromTheKey(t *testing.T) {
	t.Setenv("XCL_VAR_XCL_VAR_x", "1")

	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{}}

	setVariables(ctx, nil, "XCL_VAR_")

	variables := contextVariables(ctx)
	require.Contains(t, variables, "XCL_VAR_x")
	require.NotContains(t, variables, "x")
}

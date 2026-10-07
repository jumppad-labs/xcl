package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// decodeContainerSource is a container body whose env map holds one value
// only known once an apply has run, and whose first network block is named
// after one
const decodeContainerSource = `
default = "plain"

env = {
  A = "x"
  B = pending
}

network {
  name = pending
}

network {
  name = "second"
}
`

// pendingContext returns an evaluation context in which pending is a string
// only known once an apply has run
func pendingContext() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"pending": cty.UnknownVal(cty.String),
		},
	}
}

func TestDecodeForDiffRecordsUnknownInsideMapAndNestedBlock(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)
	container := &structs.Container{}

	paths, diags := decodeForDiff(body, pendingContext(), container)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, []diff.Path{
		diff.Path{}.Attribute("env").Key("B"),
		diff.Path{}.Attribute("network").Index(0).Attribute("name"),
	}, paths)
}

func TestDecodeForDiffWrittenPathsUseKeyForMapAndIndexForBlock(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)
	container := &structs.Container{}

	paths, diags := decodeForDiff(body, pendingContext(), container)
	require.False(t, diags.HasErrors(), diags.Error())

	written := []string{}
	for _, path := range paths {
		written = append(written, path.String())
	}

	require.Equal(t, []string{`env["B"]`, "network[0].name"}, written)
}

func TestDecodeForDiffDecodesKnownValuesConcretely(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)
	container := &structs.Container{}

	_, diags := decodeForDiff(body, pendingContext(), container)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, "plain", container.Default)
	require.Equal(t, "x", container.Env["A"])
	require.Len(t, container.Networks, 2)
	require.Equal(t, "second", container.Networks[1].Name)
}

func TestDecodeForDiffDecodesPlaceholderInPlaceOfUnknown(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)
	container := &structs.Container{}

	_, diags := decodeForDiff(body, pendingContext(), container)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Equal(t, "", container.Env["B"])
	require.Equal(t, "", container.Networks[0].Name)
}

func TestDecodeForDiffOfWhollyKnownBodyRecordsNoPaths(t *testing.T) {
	body := parseResourceBody(t, `
default = "plain"

network {
  name = known
}
`)
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"known": cty.StringVal("fixed"),
		},
	}
	container := &structs.Container{}

	paths, diags := decodeForDiff(body, ctx, container)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Empty(t, paths)
	require.Equal(t, "fixed", container.Networks[0].Name)
}

func TestDecodeForDiffLeavesParsedAttributeExpressionsInPlace(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)

	envExpr := body.Attributes["env"].Expr
	defaultExpr := body.Attributes["default"].Expr
	firstNetworkBody := body.Blocks[0].Body
	nameExpr := firstNetworkBody.Attributes["name"].Expr

	_, diags := decodeForDiff(body, pendingContext(), &structs.Container{})
	require.False(t, diags.HasErrors(), diags.Error())

	require.Same(t, envExpr, body.Attributes["env"].Expr)
	require.Same(t, defaultExpr, body.Attributes["default"].Expr)
	require.Same(t, firstNetworkBody, body.Blocks[0].Body)
	require.Same(t, nameExpr, body.Blocks[0].Body.Attributes["name"].Expr)
}

func TestDecodeForDiffLeavesParsedBodyEvaluatingToUnknown(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)
	ctx := pendingContext()

	_, diags := decodeForDiff(body, ctx, &structs.Container{})
	require.False(t, diags.HasErrors(), diags.Error())

	// re-evaluating the parsed body still reaches the unknown, no
	// placeholder was written into it
	name, valueDiags := body.Blocks[0].Body.Attributes["name"].Expr.Value(ctx)
	require.False(t, valueDiags.HasErrors(), valueDiags.Error())
	require.False(t, name.IsKnown())

	env, valueDiags := body.Attributes["env"].Expr.Value(ctx)
	require.False(t, valueDiags.HasErrors(), valueDiags.Error())
	require.False(t, env.IsWhollyKnown())
}

func TestDecodeForDiffDecodesTheSameBodyAgainWithKnownValues(t *testing.T) {
	body := parseResourceBody(t, decodeContainerSource)

	_, diags := decodeForDiff(body, pendingContext(), &structs.Container{})
	require.False(t, diags.HasErrors(), diags.Error())

	// an apply decoding the same parsed body once the value is known sees
	// the value, not the diff's placeholder
	known := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"pending": cty.StringVal("id-two"),
		},
	}
	container := &structs.Container{}

	paths, diags := decodeForDiff(body, known, container)
	require.False(t, diags.HasErrors(), diags.Error())

	require.Empty(t, paths)
	require.Equal(t, "id-two", container.Env["B"])
	require.Equal(t, "id-two", container.Networks[0].Name)
}

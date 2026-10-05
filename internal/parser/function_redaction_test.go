package parser

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/function"
	"github.com/jumppad-labs/xcl/internal/functions"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func TestRedactingFunctionRedactsSensitiveArgumentInFileError(t *testing.T) {
	dir := t.TempDir()
	wrapped := redactingFunctions(functions.GetDefaultFunctions(dir))

	_, err := wrapped["file"].Call([]cty.Value{
		cty.StringVal("s3cr3t-path-xyz").Mark(types.SensitiveMark),
	})
	require.Error(t, err)

	require.Contains(t, err.Error(), "(sensitive)")
	require.NotContains(t, err.Error(), "s3cr3t-path-xyz")
}

func TestRedactingFunctionLeavesPlainArgumentErrorUnchanged(t *testing.T) {
	dir := t.TempDir()
	unwrapped := functions.GetDefaultFunctions(dir)
	wrapped := redactingFunctions(unwrapped)

	_, expected := unwrapped["file"].Call([]cty.Value{cty.StringVal("plain-missing-file")})
	require.Error(t, expected)

	_, err := wrapped["file"].Call([]cty.Value{cty.StringVal("plain-missing-file")})
	require.Error(t, err)

	require.Equal(t, expected.Error(), err.Error())
}

func TestRedactingFunctionMarksResultOfSensitiveArgument(t *testing.T) {
	wrapped := redactingFunctions(functions.GetDefaultFunctions(t.TempDir()))

	result, err := wrapped["upper"].Call([]cty.Value{
		cty.StringVal("hello").Mark(types.SensitiveMark),
	})
	require.NoError(t, err)

	require.True(t, result.HasMark(types.SensitiveMark))

	unmarked, _ := result.Unmark()
	require.Equal(t, "HELLO", unmarked.AsString())
}

func TestRedactingFunctionLeavesResultOfPlainArgumentUnmarked(t *testing.T) {
	wrapped := redactingFunctions(functions.GetDefaultFunctions(t.TempDir()))

	result, err := wrapped["upper"].Call([]cty.Value{cty.StringVal("hello")})
	require.NoError(t, err)

	require.False(t, result.HasMark(types.SensitiveMark))
	require.Equal(t, "HELLO", result.AsString())
}

func TestRedactingFunctionKeepsArgErrorIndex(t *testing.T) {
	inner := function.New(&function.Spec{
		Params: []function.Parameter{
			{Name: "first", Type: cty.String},
			{Name: "second", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.NilVal, function.NewArgError(1, fmt.Errorf("invalid %s", args[1].AsString()))
		},
	})

	wrapped := redactingFunction(inner)

	_, err := wrapped.Call([]cty.Value{
		cty.StringVal("plain"),
		cty.StringVal("s3cr3t-arg").Mark(types.SensitiveMark),
	})
	require.Error(t, err)

	var argError function.ArgError
	require.True(t, errors.As(err, &argError))
	require.Equal(t, 1, argError.Index)
	require.Contains(t, err.Error(), "(sensitive)")
	require.NotContains(t, err.Error(), "s3cr3t-arg")
}

func TestRedactingFunctionRedactsCustomFunctionErrorAndPassesUnmarkedValue(t *testing.T) {
	var received cty.Value

	custom := function.New(&function.Spec{
		Params: []function.Parameter{{Name: "value", Type: cty.String}},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			received = args[0]
			return cty.NilVal, fmt.Errorf("bad value %s", args[0].AsString())
		},
	})

	wrapped := redactingFunctions(map[string]function.Function{"custom": custom})

	_, err := wrapped["custom"].Call([]cty.Value{
		cty.StringVal("s3cr3t-custom").Mark(types.SensitiveMark),
	})
	require.Error(t, err)

	require.Equal(t, "bad value (sensitive)", err.Error())
	require.False(t, received.IsMarked())
	require.Equal(t, "s3cr3t-custom", received.AsString())
}

package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// greetFunction builds a provider function from a Go func that joins a
// title and a count, the function most of these tests call
func greetFunction(t *testing.T) ProviderFunction {
	t.Helper()

	fn, err := NewFunction(func(title string, n int) (string, error) {
		return title + "-" + strconv.Itoa(n), nil
	})
	require.NoError(t, err)

	return fn
}

func TestNewFunctionDescribesSignature(t *testing.T) {
	fn, err := NewFunction(func(title string, n int, flag bool) (string, error) {
		return title, nil
	})
	require.NoError(t, err)

	sig := fn.Signature()
	require.Len(t, sig.Params, 3)

	require.Equal(t, "arg0", sig.Params[0].Name)
	require.Equal(t, `"string"`, string(sig.Params[0].Type))

	require.Equal(t, "arg1", sig.Params[1].Name)
	require.Equal(t, `"number"`, string(sig.Params[1].Type))

	require.Equal(t, "arg2", sig.Params[2].Name)
	require.Equal(t, `"bool"`, string(sig.Params[2].Type))

	require.Equal(t, `"string"`, string(sig.Return))
	require.Nil(t, sig.Variadic)
}

func TestNewFunctionDescribesListAndMapTypes(t *testing.T) {
	fn, err := NewFunction(func(tags []string) (map[string]int, error) {
		return nil, nil
	})
	require.NoError(t, err)

	sig := fn.Signature()
	require.Len(t, sig.Params, 1)
	require.Equal(t, `["list","string"]`, string(sig.Params[0].Type))
	require.Equal(t, `["map","number"]`, string(sig.Return))
}

func TestNewFunctionCallsGoFunc(t *testing.T) {
	fn := greetFunction(t)

	result, err := fn.Call(context.Background(), []byte(`["My App", 2]`))
	require.NoError(t, err)

	var decoded string
	require.NoError(t, json.Unmarshal(result, &decoded))
	require.Equal(t, "My App-2", decoded)
}

func TestNewFunctionCallsGoFuncWithListAndMap(t *testing.T) {
	fn, err := NewFunction(func(tags []string) (map[string]int, error) {
		counts := map[string]int{}
		for _, tag := range tags {
			counts[tag]++
		}

		return counts, nil
	})
	require.NoError(t, err)

	result, err := fn.Call(context.Background(), []byte(`[["a","b"]]`))
	require.NoError(t, err)
	require.JSONEq(t, `{"a":1,"b":1}`, string(result))
}

func TestNewFunctionReturnsGoFuncError(t *testing.T) {
	fn, err := NewFunction(func(title string) (string, error) {
		return "", errors.New("boom")
	})
	require.NoError(t, err)

	_, err = fn.Call(context.Background(), []byte(`["x"]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestNewFunctionRejectsWrongArgumentCount(t *testing.T) {
	fn := greetFunction(t)

	_, err := fn.Call(context.Background(), []byte(`["only one"]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "arguments given")
}

func TestNewFunctionRejectsNonArrayArguments(t *testing.T) {
	fn := greetFunction(t)

	_, err := fn.Call(context.Background(), []byte(`{"a":1}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a JSON array")
}

func TestNewFunctionRejectsUnsupportedParameter(t *testing.T) {
	_, err := NewFunction(func(s struct{}) (int, error) {
		return 0, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parameter 0")
}

func TestNewFunctionRejectsVariadic(t *testing.T) {
	_, err := NewFunction(func(parts ...string) (string, error) {
		return "", nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "variadic")
}

func TestNewFunctionRejectsWrongReturns(t *testing.T) {
	_, err := NewFunction(func(title string) string {
		return title
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must return a result and an error")
}

// twoFunctions builds a ProviderFunctions offering "a" and "b"
func twoFunctions(t *testing.T) ProviderFunctions {
	t.Helper()

	a, err := NewFunction(func(s string) (string, error) { return "a:" + s, nil })
	require.NoError(t, err)

	b, err := NewFunction(func(s string) (string, error) { return "b:" + s, nil })
	require.NoError(t, err)

	return NewFunctions(map[string]ProviderFunction{"a": a, "b": b})
}

func TestNewFunctionsGetFindAndList(t *testing.T) {
	fns := twoFunctions(t)

	got, err := fns.Get("a")
	require.NoError(t, err)
	require.NotNil(t, got)

	result, err := got.Call(context.Background(), []byte(`["x"]`))
	require.NoError(t, err)
	require.JSONEq(t, `"a:x"`, string(result))

	found, err := fns.Find("^a")
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Contains(t, found, "a")

	all, err := fns.List()
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Contains(t, all, "a")
	require.Contains(t, all, "b")
}

func TestNewFunctionsGetRejectsUnknownName(t *testing.T) {
	fns := twoFunctions(t)

	_, err := fns.Get("zzz")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not offered by this provider")
}

func TestNewFunctionsFindRejectsInvalidPattern(t *testing.T) {
	fns := twoFunctions(t)

	_, err := fns.Find("(")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid function pattern")
}

func TestFunctionSignatureRoundTripsThroughJSON(t *testing.T) {
	original := FunctionSignature{
		Params: []FunctionParam{
			{Name: "arg0", Type: json.RawMessage(`"string"`)},
			{Name: "arg1", Type: json.RawMessage(`["list","number"]`)},
		},
		Return: json.RawMessage(`["map","bool"]`),
	}

	encoded, err := json.Marshal(original)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "variadic")

	var decoded FunctionSignature
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, original, decoded)
}

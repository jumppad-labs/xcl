package functions

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/cty/convert"
	"github.com/stretchr/testify/require"
)

func TestCreateFunctionCreatesFunctionWithCorrectInParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	require.Equal(t, cty.String, ctyFunc.Params()[0].Type)
	require.Equal(t, cty.Number, ctyFunc.Params()[1].Type)
}

func TestCreateFunctionWithInvalidInParameterReturnsError(t *testing.T) {
	myfunc := func(a string, complex func() error) (int, error) {
		return 0, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)
}

func TestCreateFunctionCreatesFunctionWithCorrectOutParameters(t *testing.T) {
	myfunc := func(a string, b int) (int, error) {
		return 0, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	rt, err := ctyFunc.ReturnType([]cty.Type{cty.String, cty.Number})
	require.NoError(t, err)
	require.Equal(t, cty.Number, rt)
}

func TestCreateFunctionWithInvalidOutParameterReturnsError(t *testing.T) {
	myfunc := func(a string, b int) func() error {
		return func() error {
			return fmt.Errorf("oops")
		}
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.Error(t, err)

	myfunc2 := func(a string, b int) int {
		return 1
	}

	_, err = CreateCtyFunctionFromGoFunc(myfunc2)
	require.Error(t, err)
}

func TestCreateFunctionCallsFunction(t *testing.T) {
	myfunc := func(a, b int) (int, error) {
		return a + b, nil
	}

	ctyFunc, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)

	val, err := ctyFunc.Call([]cty.Value{cty.NumberIntVal(2), cty.NumberIntVal(3)})
	require.NoError(t, err)

	bf := val.AsBigFloat()
	i, _ := bf.Int64()
	require.Equal(t, int64(5), i)
}

func TestCreateFunctionHandlesIntegerInputParams(t *testing.T) {
	myfunc := func(a, b int) (int, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt16InputParams(t *testing.T) {
	myfunc := func(a, b int16) (int16, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt32InputParams(t *testing.T) {
	myfunc := func(a, b int32) (int32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesInt64InputParams(t *testing.T) {
	myfunc := func(a, b int64) (int64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUintInputParams(t *testing.T) {
	myfunc := func(a, b uint) (uint, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint16InputParams(t *testing.T) {
	myfunc := func(a, b uint16) (uint16, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint32InputParams(t *testing.T) {
	myfunc := func(a, b uint32) (uint32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesUint64InputParams(t *testing.T) {
	myfunc := func(a, b uint64) (uint64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesFloat32InputParams(t *testing.T) {
	myfunc := func(a, b float32) (float32, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesFloat64InputParams(t *testing.T) {
	myfunc := func(a, b float64) (float64, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

func TestCreateFunctionHandlesStringInputParams(t *testing.T) {
	myfunc := func(a, b string) (string, error) {
		return a + b, nil
	}

	_, err := CreateCtyFunctionFromGoFunc(myfunc)
	require.NoError(t, err)
}

// GoFuncSignature. The signature is the one description of a Go func that
// both the in-process and the plugin side of functions are built from, so it
// has to name each parameter, carry its configuration type, and refuse a
// func that configuration cannot call with a message naming the problem.

func TestGoFuncSignatureDescribesParametersAndResult(t *testing.T) {
	sig, err := GoFuncSignature(func(a string, b int, c bool) (string, error) { return a, nil })
	require.NoError(t, err)

	require.Len(t, sig.Params, 3)
	require.Equal(t, "arg0", sig.Params[0].Name)
	require.Equal(t, cty.String, sig.Params[0].Type)
	require.Equal(t, "arg1", sig.Params[1].Name)
	require.Equal(t, cty.Number, sig.Params[1].Type)
	require.Equal(t, "arg2", sig.Params[2].Name)
	require.Equal(t, cty.Bool, sig.Params[2].Type)
	require.Equal(t, cty.String, sig.Return)
	require.Equal(t, reflect.TypeOf(""), sig.ReturnGoType)
}

func TestGoFuncSignatureSupportsSlicesAndMaps(t *testing.T) {
	sig, err := GoFuncSignature(func(tags []string, ports map[string]int) ([]string, error) { return tags, nil })
	require.NoError(t, err)

	require.Len(t, sig.Params, 2)
	require.Equal(t, cty.List(cty.String), sig.Params[0].Type)
	require.Equal(t, cty.Map(cty.Number), sig.Params[1].Type)
	require.Equal(t, cty.List(cty.String), sig.Return)
}

func TestGoFuncSignatureRejectsNonFunc(t *testing.T) {
	_, err := GoFuncSignature("not a func")

	require.EqualError(t, err, "a Go func is required, not string")
}

func TestGoFuncSignatureRejectsNil(t *testing.T) {
	_, err := GoFuncSignature(nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "a Go func is required")
}

func TestGoFuncSignatureRejectsVariadic(t *testing.T) {
	_, err := GoFuncSignature(func(parts ...string) (string, error) { return "", nil })

	require.Error(t, err)
	require.Contains(t, err.Error(), "variadic functions are not supported")
}

func TestGoFuncSignatureRejectsWrongReturnCount(t *testing.T) {
	_, err := GoFuncSignature(func(s string) string { return s })

	require.Error(t, err)
	require.Contains(t, err.Error(), "functions must return a result and an error")
}

func TestGoFuncSignatureRejectsNonErrorSecondReturn(t *testing.T) {
	_, err := GoFuncSignature(func() (int, int) { return 0, 0 })

	require.Error(t, err)
	require.Contains(t, err.Error(), "functions must return a result and an error")
}

func TestGoFuncSignatureRejectsUnsupportedParameterType(t *testing.T) {
	_, err := GoFuncSignature(func(x struct{}) (int, error) { return 0, nil })

	require.Error(t, err)
	require.Contains(t, err.Error(), "parameter 0")
	require.Contains(t, err.Error(), "struct {}")
	require.Contains(t, err.Error(), "is not supported")
}

func TestGoFuncSignatureRejectsMapWithNonStringKeys(t *testing.T) {
	_, err := GoFuncSignature(func(x map[int]string) (int, error) { return 0, nil })

	require.Error(t, err)
	require.Contains(t, err.Error(), "parameter 0")
	require.Contains(t, err.Error(), "a map must have string keys")
}

func TestGoFuncSignatureRejectsUnsupportedResultType(t *testing.T) {
	_, err := GoFuncSignature(func() (chan int, error) { return nil, nil })

	require.Error(t, err)
	require.Contains(t, err.Error(), "result:")
	require.Contains(t, err.Error(), "chan int")
}

// CreateCtyFunctionFromGoFunc. Each argument is converted to the parameter's
// Go type and the result back to a configuration value, and an error the
// func returns is the error of the call.

func TestCreateCtyFunctionFromGoFuncCallsTheFuncWithConvertedArguments(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(s string, n int) (string, error) {
		return fmt.Sprintf("%s-%d", s, n), nil
	})
	require.NoError(t, err)

	result, err := fn.Call([]cty.Value{cty.StringVal("a"), cty.NumberIntVal(2)})
	require.NoError(t, err)

	require.Equal(t, cty.StringVal("a-2"), result)
}

func TestCreateCtyFunctionFromGoFuncConvertsSlicesAndMaps(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(xs []string) (map[string]int, error) {
		lengths := map[string]int{}
		for _, x := range xs {
			lengths[x] = len(x)
		}

		return lengths, nil
	})
	require.NoError(t, err)

	result, err := fn.Call([]cty.Value{cty.ListVal([]cty.Value{cty.StringVal("ab"), cty.StringVal("cde")})})
	require.NoError(t, err)

	require.Equal(t, cty.MapVal(map[string]cty.Value{
		"ab":  cty.NumberIntVal(2),
		"cde": cty.NumberIntVal(3),
	}), result)
}

func TestCreateCtyFunctionFromGoFuncAcceptsTupleConvertedToListAsConfigurationDoes(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(xs []string) (int, error) {
		return len(xs), nil
	})
	require.NoError(t, err)

	// a list literal in configuration, ["a", "b"], is a tuple; the function
	// call expression converts each argument to the parameter's type before
	// the call, which Call itself does not do
	tuple := cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})
	list, err := convert.Convert(tuple, fn.Params()[0].Type)
	require.NoError(t, err)

	result, err := fn.Call([]cty.Value{list})
	require.NoError(t, err)

	require.Equal(t, cty.NumberIntVal(2), result)
}

func TestCreateCtyFunctionFromGoFuncCallRejectsAnUnconvertedTuple(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(xs []string) (int, error) {
		return len(xs), nil
	})
	require.NoError(t, err)

	_, err = fn.Call([]cty.Value{cty.TupleVal([]cty.Value{cty.StringVal("a"), cty.StringVal("b")})})

	require.Error(t, err)
	require.Contains(t, err.Error(), "list of string required, but received tuple")
}

func TestCreateCtyFunctionFromGoFuncReturnsTheFuncError(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(s string) (string, error) {
		return "", errors.New("boom")
	})
	require.NoError(t, err)

	_, err = fn.Call([]cty.Value{cty.StringVal("a")})

	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestCreateCtyFunctionFromGoFuncReturnsBool(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(s string) (bool, error) {
		return s == "yes", nil
	})
	require.NoError(t, err)

	result, err := fn.Call([]cty.Value{cty.StringVal("yes")})
	require.NoError(t, err)

	require.Equal(t, cty.True, result)
}

func TestCreateCtyFunctionFromGoFuncReturnsFloat(t *testing.T) {
	fn, err := CreateCtyFunctionFromGoFunc(func(n float64) (float64, error) {
		return n / 2, nil
	})
	require.NoError(t, err)

	result, err := fn.Call([]cty.Value{cty.NumberFloatVal(3)})
	require.NoError(t, err)

	require.Equal(t, cty.NumberFloatVal(1.5), result)
}

func TestCreateCtyFunctionFromGoFuncRejectsAnUnsupportedSignature(t *testing.T) {
	_, err := CreateCtyFunctionFromGoFunc(func(s string) string { return s })

	require.Error(t, err)
	require.Contains(t, err.Error(), "functions must return a result and an error")
}

// BuiltinNames. The catalog refuses an application function named like a
// builtin, so the names have to be complete, sorted and safe to hand out.

func TestBuiltinNamesIsSortedAndContainsUpper(t *testing.T) {
	names := BuiltinNames()

	require.Len(t, names, 57)
	require.True(t, sort.StringsAreSorted(names))
	require.Contains(t, names, "upper")
	require.Contains(t, names, "format")
	require.Contains(t, names, "join")
	require.Contains(t, names, "template_file")
}

func TestBuiltinNamesMatchesTheDefaultFunctions(t *testing.T) {
	names := BuiltinNames()

	defaults := GetDefaultFunctions("")
	require.Len(t, names, len(defaults))

	for _, name := range names {
		require.Contains(t, defaults, name)
	}
}

func TestBuiltinNamesReturnsACopy(t *testing.T) {
	names := BuiltinNames()
	names[0] = "changed"

	require.NotEqual(t, "changed", BuiltinNames()[0])
}

// GoFunc.CallJSON calls the func with JSON arguments, one per parameter in
// the configuration form of its type, and returns the result as JSON. It is
// how a function is called across a plugin boundary.

// repeatFunc returns a GoFunc that repeats its list argument n times
func repeatFunc(t *testing.T) *GoFunc {
	t.Helper()

	fn, err := NewGoFunc(func(items []string, n int) ([]string, error) {
		repeated := []string{}
		for i := 0; i < n; i++ {
			repeated = append(repeated, items...)
		}

		return repeated, nil
	})
	require.NoError(t, err)

	return fn
}

func TestGoFuncCallJSONDecodesArgumentsAndEncodesResult(t *testing.T) {
	fn := repeatFunc(t)

	result, err := fn.CallJSON([]byte(`[["a","b"], 2]`))
	require.NoError(t, err)

	require.JSONEq(t, `["a","b","a","b"]`, string(result))
}

func TestGoFuncCallJSONRejectsWrongArgumentCount(t *testing.T) {
	fn := repeatFunc(t)

	_, err := fn.CallJSON([]byte(`[["a","b"]]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "1 arguments given, 2 expected")
}

func TestGoFuncCallJSONRejectsNonArray(t *testing.T) {
	fn := repeatFunc(t)

	_, err := fn.CallJSON([]byte(`{"items":["a"],"n":2}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "arguments must be a JSON array")
}

func TestGoFuncCallJSONRejectsAnArgumentOfTheWrongType(t *testing.T) {
	fn := repeatFunc(t)

	_, err := fn.CallJSON([]byte(`[["a","b"], "two"]`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "argument 1")
}

func TestGoFuncCallJSONReturnsTheFuncError(t *testing.T) {
	fn, err := NewGoFunc(func(s string) (string, error) {
		return "", errors.New("boom")
	})
	require.NoError(t, err)

	_, err = fn.CallJSON([]byte(`["a"]`))
	require.EqualError(t, err, "boom")
}

// CtyFunctionFromSignature builds a callable function from a signature and
// an implementation, so a plugin function can be called the way an
// application function is.

func TestCtyFunctionFromSignatureCallsTheImplementation(t *testing.T) {
	sig := Signature{
		Params: []Parameter{
			{Name: "first", Type: cty.String},
			{Name: "second", Type: cty.Number},
		},
		Return: cty.String,
	}

	var received []cty.Value
	fn := CtyFunctionFromSignature(sig, func(args []cty.Value) (cty.Value, error) {
		received = args
		return args[0], nil
	})

	require.Len(t, fn.Params(), 2)
	require.Equal(t, "first", fn.Params()[0].Name)
	require.Equal(t, cty.String, fn.Params()[0].Type)
	require.Equal(t, "second", fn.Params()[1].Name)
	require.Equal(t, cty.Number, fn.Params()[1].Type)

	result, err := fn.Call([]cty.Value{cty.StringVal("echo"), cty.NumberIntVal(7)})
	require.NoError(t, err)

	require.Equal(t, cty.StringVal("echo"), result)
	require.Equal(t, []cty.Value{cty.StringVal("echo"), cty.NumberIntVal(7)}, received)
}

func TestCtyFunctionFromSignatureReturnsTheImplementationsError(t *testing.T) {
	sig := Signature{
		Params: []Parameter{{Name: "first", Type: cty.String}},
		Return: cty.String,
	}

	fn := CtyFunctionFromSignature(sig, func(args []cty.Value) (cty.Value, error) {
		return cty.NilVal, errors.New("boom")
	})

	_, err := fn.Call([]cty.Value{cty.StringVal("a")})
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

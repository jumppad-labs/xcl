package utils

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/stretchr/testify/require"
)

func TestProcessesTypes(t *testing.T) {
	vars := map[string]cty.Value{}
	vars["string"] = cty.StringVal("abc")
	vars["number"] = cty.NumberIntVal(23)
	vars["bool"] = cty.BoolVal(true)
	vars["array"] = cty.ListVal(
		[]cty.Value{
			cty.StringVal("abc"),
			cty.StringVal("123"),
		})

	vars["map"] = cty.MapVal(map[string]cty.Value{
		"foo": cty.StringVal("abc"),
	})

	output := ParseVars(vars)

	require.Equal(t, "abc", output["string"])

	num := int64(output["number"].(float64))
	require.Equal(t, int64(23), num)

	require.True(t, output["bool"].(bool))

	require.Equal(t, "abc", output["array"].([]any)[0])
	require.Equal(t, "123", output["array"].([]any)[1])

	require.Equal(t, "abc", output["map"].(map[string]any)["foo"])
}

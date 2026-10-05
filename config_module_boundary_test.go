package xcl

import (
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// TestApplyResolvesAReExportedGrandchildOutput asserts that a value two modules
// deep reaches the root when each module in between re-exports it as one of
// its own outputs: the root's output.deep reads module.a.output.from_b, which
// module a sets from module.b.output.value.
func TestApplyResolvesAReExportedGrandchildOutput(t *testing.T) {
	path, err := filepath.Abs("./internal/test_fixtures/config/module_reexport/main.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Apply(path)
	require.NoError(t, err)

	require.Equal(t, "from-b", c.Outputs()["output.deep"])

	deep, err := Find[types.Output](c, "output.deep")
	require.NoError(t, err)
	require.Equal(t, "from-b", deep.Value)
}

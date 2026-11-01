package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/testutil"
)

// startRecordArgs builds the recordargs program, starts it as a plugin
// binary with args and returns the arguments it recorded. recordargs is not
// a plugin, so the start itself fails.
func startRecordArgs(t *testing.T, args ...string) string {
	t.Helper()

	binary := testutil.BuildProgram(t, "../internal/test_fixtures/recordargs")

	recorded := filepath.Join(t.TempDir(), "args")
	t.Setenv("XCL_RECORD_ARGS", recorded)

	host, err := executablePlugin(binary, args...).Start(nil)
	require.Error(t, err)
	require.Nil(t, host)

	contents, err := os.ReadFile(recorded)
	require.NoError(t, err)

	return string(contents)
}

func TestExecutableStartPassesItsArgumentsToTheProcess(t *testing.T) {
	recorded := startRecordArgs(t, "--flag", "value")

	require.Equal(t, "--flag\nvalue", recorded)
}

func TestExecutableStartWithoutArgumentsPassesNone(t *testing.T) {
	recorded := startRecordArgs(t)

	require.Equal(t, "", recorded)
}

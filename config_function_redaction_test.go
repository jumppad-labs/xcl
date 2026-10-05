package xcl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFunctionErrorOnSensitiveArgumentDoesNotLeakValue(t *testing.T) {
	_, err := applySensitiveFixture(t, "function_error")
	require.Error(t, err)

	require.Contains(t, err.Error(), "(sensitive)")
	require.NotContains(t, err.Error(), "s3cr3t-leak-path")
}

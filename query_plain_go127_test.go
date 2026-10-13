//go:build go1.27

package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/plain"
	"github.com/stretchr/testify/require"
)

func TestFindMethodReturnsAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	agent, err := c.Find[plain.Agent]("agent.claude")
	require.NoError(t, err)
	require.Equal(t, "claude", agent.Name)
}

func TestFindMethodAndFunctionAgreeOnAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	method, methodErr := c.Find[plain.Database]("database.postgres.main")
	function, functionErr := Find[plain.Database](c, "database.postgres.main")

	require.Equal(t, function, method)
	require.Equal(t, functionErr, methodErr)
}

func TestAllMethodReturnsEveryEntityOfAPlainType(t *testing.T) {
	c := setupPlainConfig(t)

	jobs, err := c.All[plain.Job]()
	require.NoError(t, err)
	require.Len(t, jobs, 3)
}

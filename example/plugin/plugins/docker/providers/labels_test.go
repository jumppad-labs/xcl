package providers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/types"
)

func TestLabelsNameTheExampleAndTheBlock(t *testing.T) {
	got := labels(types.Meta{ID: "docker.network.app"})

	require.Equal(t, map[string]string{
		"created_by": "xcl-example-plugin",
		"xcl_id":     "docker.network.app",
	}, got)
}

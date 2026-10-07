package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockHeaderWritesResourceAddressAsTypedBlock(t *testing.T) {
	require.Equal(t, `resource "container" "api"`, blockHeader("resource.container.api"))
}

func TestBlockHeaderWritesTwoSegmentAddressWithOneLabel(t *testing.T) {
	require.Equal(t, `server "web"`, blockHeader("server.web"))
}

func TestBlockHeaderWritesThreeSegmentAddressWithTwoLabels(t *testing.T) {
	require.Equal(t, `server "big" "web"`, blockHeader("server.big.web"))
}

func TestBlockHeaderSetsAsideModulePrefixOfResource(t *testing.T) {
	require.Equal(t, `resource "container" "api"`, blockHeader("module.app.resource.container.api"))
}

func TestBlockHeaderSetsAsideNestedModulePrefixes(t *testing.T) {
	require.Equal(t, `resource "x" "y"`, blockHeader("module.a.module.b.resource.x.y"))
}

func TestBlockHeaderSetsAsideModulePrefixOfNonResourceBlock(t *testing.T) {
	require.Equal(t, `server "web"`, blockHeader("module.app.server.web"))
}

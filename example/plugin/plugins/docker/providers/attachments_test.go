package providers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"
)

func TestIsNetworkAddressAcceptsANetworkBlock(t *testing.T) {
	require.True(t, isNetworkAddress("docker.network.app"))
}

func TestIsNetworkAddressAcceptsANetworkBlockInAModule(t *testing.T) {
	require.True(t, isNetworkAddress("module.x.docker.network.app"))
}

func TestIsNetworkAddressRejectsATemplate(t *testing.T) {
	require.False(t, isNetworkAddress("template.init"))
}

func TestIsNetworkAddressRejectsAContainer(t *testing.T) {
	require.False(t, isNetworkAddress("docker.container.web"))
}

func TestPreviousAttachmentsPutsBackARenamedNetwork(t *testing.T) {
	current := []entities.NetworkAttachment{
		{Name: "backend", Aliases: []string{"web"}},
	}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("network").Index(0).Attribute("name"), Before: "app", After: "backend"},
	}

	previous, err := previousAttachments(current, changes)
	require.NoError(t, err)
	require.Equal(t, []entities.NetworkAttachment{{Name: "app", Aliases: []string{"web"}}}, previous)
}

func TestPreviousAttachmentsDropsAnAddedNetwork(t *testing.T) {
	current := []entities.NetworkAttachment{
		{Name: "app", Aliases: []string{"web"}},
		{Name: "backend"},
	}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("network").Index(1), Before: nil, After: map[string]any{"name": "backend"}},
	}

	previous, err := previousAttachments(current, changes)
	require.NoError(t, err)
	require.Equal(t, []entities.NetworkAttachment{{Name: "app", Aliases: []string{"web"}}}, previous)
}

func TestPreviousAttachmentsRestoresARemovedNetwork(t *testing.T) {
	current := []entities.NetworkAttachment{}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("network").Index(0), Before: map[string]any{"name": "app", "aliases": []any{"web"}}, After: nil},
	}

	previous, err := previousAttachments(current, changes)
	require.NoError(t, err)
	require.Equal(t, []entities.NetworkAttachment{{Name: "app", Aliases: []string{"web"}}}, previous)
}

func TestPreviousAttachmentsDropsAnAddedAlias(t *testing.T) {
	current := []entities.NetworkAttachment{
		{Name: "app", Aliases: []string{"web", "www"}},
	}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("network").Index(0).Attribute("aliases").Index(1), Before: nil, After: "www"},
	}

	previous, err := previousAttachments(current, changes)
	require.NoError(t, err)
	require.Equal(t, []entities.NetworkAttachment{{Name: "app", Aliases: []string{"web"}}}, previous)
}

func TestPreviousAttachmentsIgnoresChangesOutsideTheNetworks(t *testing.T) {
	current := []entities.NetworkAttachment{
		{Name: "app"},
	}
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.27", After: "nginx:1.28"},
	}

	previous, err := previousAttachments(current, changes)
	require.NoError(t, err)
	require.Equal(t, []entities.NetworkAttachment{{Name: "app"}}, previous)
}

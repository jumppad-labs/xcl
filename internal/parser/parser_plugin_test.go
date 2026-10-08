package parser

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/catalog"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// TestPluginRegistration tests that we can register and use plugins
func TestPluginRegistration(t *testing.T) {
	// Create a new parser with TestLogger and a Catalog
	o := testOptions(t)
	o.Catalog = catalog.New()

	parser := NewParser(o)

	// Create a simple test plugin
	plugin := &SimpleTestPlugin{}

	// Register the plugin on a local registry added to the Catalog
	local := registry.NewLocal()
	local.RegisterPlugin(plugin)
	o.Catalog.AddRegistry(local)

	// Registering only records the plugin, it is started when plugins load
	err := o.Catalog.Load(nil)
	require.NoError(t, err, "Should load plugin without error")

	// Verify the plugin was added to the registry
	require.Len(t, parser.catalog.GetPluginHosts(), 1, "Should have one plugin host")

	// Try to create a resource using the plugin registry
	resource, err := parser.catalog.CreateEntity("resource", "person", "test_person")
	require.NoError(t, err, "Should create resource from plugin")
	require.NotNil(t, resource, "Resource should not be nil")
	meta, err := types.GetMeta(resource)
	require.NoError(t, err)
	require.Equal(t, "test_person", meta.Name)
	require.Equal(t, types.TypeResource, meta.Type)
	require.Equal(t, "person", meta.Subtype)
}

// TestPluginResourceCreationWithFallback tests plugin creation with fallback to registered types
func TestPluginResourceCreationWithFallback(t *testing.T) {
	o := testOptions(t)
	parser := NewParser(o)

	// Try to create a resource that doesn't exist in plugins (should fall back to registered types)
	// This should fail since we don't have any registered types for "nonexistent"
	_, err := parser.catalog.CreateEntity("resource", "nonexistent", "test")
	require.Error(t, err, "Should fail to create nonexistent resource type")
	require.Contains(t, err.Error(), "not found in any registered plugin")
}

// SimpleTestPlugin is a simple test plugin for testing
type SimpleTestPlugin struct {
	plugins.PluginBase
}

// Init initializes the test plugin
func (p *SimpleTestPlugin) Init(logger logger.Logger, state plugins.State) error {
	// Create test person resource and provider
	personResource := &person.Person{}
	personProvider := &person.ExampleProvider{}

	// Register the Person resource type with the plugin
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		logger,
		state,
		"resource",     // Top-level type
		"person",       // Sub-type
		personResource, // Resource instance
		personProvider, // Provider instance
	)
}

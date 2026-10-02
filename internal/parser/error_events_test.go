package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// eventsWith returns the recorded events with the given operation and phase,
// in the order they were emitted
func eventsWith(recorded []events.Event, operation, phase string) []events.Event {
	found := []events.Event{}
	for _, event := range recorded {
		if event.Operation == operation && event.Phase == phase {
			found = append(found, event)
		}
	}

	return found
}

func TestApplyWithMissingProviderEmitsErrorEventForTheResource(t *testing.T) {
	h := setupLifecycle(t)

	// the resolver finds no provider for any resource
	resolver := mocks.NewMockProviderResolver(t)
	resolver.EXPECT().GetProviderForResource(mock.Anything).Return(nil)

	collector := &eventCollector{}

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.ProviderResolver = resolver
	options.Emit = collector.collect

	p := NewParser(options)

	_, err := p.Apply(context.Background(), lifecycleOriginalConfig)
	require.ErrorContains(t, err, "no provider found for resource type network")

	// the network has no previous entry, so the step that would have run
	// first is create
	failed := eventsWith(collector.all(), events.OperationCreate, events.PhaseError)
	require.Len(t, failed, 1)

	event := failed[0]
	require.Equal(t, events.SourceCore, event.Source)
	require.Equal(t, lifecycleNetworkID, event.ResourceID)
	require.Equal(t, "network.one", event.ResourceType)
	require.Equal(t, lifecycleOriginalConfig, event.File)
	require.ErrorContains(t, event.Error, "no provider found for resource type network")

	// the error is reported once, not again as an apply error
	require.Empty(t, eventsWith(collector.all(), events.OperationApply, events.PhaseError))
	require.Equal(t, []string{"create error"}, eventsFor(collector.all(), lifecycleNetworkID))
}

func TestValidateReferenceProblemEmitsValidateErrorEventNamingTheResource(t *testing.T) {
	dir, err := filepath.Abs("../test_fixtures/config/undefined_resource_reference")
	require.NoError(t, err)

	collector := &eventCollector{}
	options := testOptions(t)
	options.Emit = collector.collect
	p, _ := setupParser(t, options)

	err = p.Validate(context.Background(), dir)
	require.Error(t, err)

	failed := eventsWith(collector.all(), events.OperationValidate, events.PhaseError)
	require.Len(t, failed, 1)

	event := failed[0]
	require.Equal(t, events.SourceCore, event.Source)
	require.Equal(t, filepath.Join(dir, "container.xcl"), event.File)
	require.Equal(t, "resource.container.consul", event.ResourceID)
	require.Equal(t, "container.consul", event.ResourceType)

	pe, ok := event.Error.(*errors.ParserError)
	require.True(t, ok, "validate error is a %T", event.Error)
	require.Equal(t, "resource 'resource.container.consul' refers to 'resource.network.nosuch.meta.name', which is not defined anywhere in the configuration", pe.Message)
}

func TestApplyDecodeFailureEmitsApplyErrorEventNamingTheResource(t *testing.T) {
	h := setupLifecycle(t)

	// a list can not be decoded into the network's string subnet, which only
	// decoding the body in the walk finds
	file := filepath.Join(t.TempDir(), "network.xcl")
	err := os.WriteFile(file, []byte(`resource "network" "one" {
  subnet = ["10.0.0.0/16", "10.1.0.0/16"]
}
`), 0o644)
	require.NoError(t, err)

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	_, err = p.Apply(context.Background(), file)
	require.ErrorContains(t, err, "unable to decode body")

	failed := eventsWith(collector.all(), events.OperationApply, events.PhaseError)
	require.Len(t, failed, 1)

	event := failed[0]
	require.Equal(t, events.SourceCore, event.Source)
	require.Equal(t, lifecycleNetworkID, event.ResourceID)
	require.Equal(t, "network.one", event.ResourceType)
	require.Equal(t, file, event.File)
	require.ErrorContains(t, event.Error, "unable to decode body")

	// the body never decoded, so no provider was called
	require.Empty(t, h.plugin.GetCalls())
}

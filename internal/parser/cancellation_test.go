package parser

import (
	"context"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/plugin/structs"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// cancellingResolver finds providers through resolver, and hands out an
// adapter whose Create cancels the operation's context before it does the
// create, then records the error of the context the call was given
type cancellingResolver struct {
	resolver ProviderResolver
	cancel   context.CancelFunc

	mu sync.Mutex
	// createErrs holds ctx.Err() of the context given to each Create, taken
	// after the operation was cancelled
	createErrs []error
}

func (r *cancellingResolver) GetProviderForResource(resource any) plugins.ProviderAdapter {
	adapter := r.resolver.GetProviderForResource(resource)
	if adapter == nil {
		return nil
	}

	return &cancellingAdapter{ProviderAdapter: adapter, resolver: r}
}

func (r *cancellingResolver) recorded() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]error{}, r.createErrs...)
}

// cancellingAdapter is the adapter handed out by cancellingResolver, every
// call but Create goes straight to the wrapped adapter
type cancellingAdapter struct {
	plugins.ProviderAdapter
	resolver *cancellingResolver
}

func (a *cancellingAdapter) Create(ctx context.Context, entityData []byte) ([]byte, error) {
	a.resolver.cancel()

	a.resolver.mu.Lock()
	a.resolver.createErrs = append(a.resolver.createErrs, ctx.Err())
	a.resolver.mu.Unlock()

	return a.ProviderAdapter.Create(ctx, entityData)
}

func TestApplyWithCancelledContextStartsNoProviderCall(t *testing.T) {
	h := setupLifecycle(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := h.newParser(t, nil)

	st, err := p.Apply(ctx, lifecycleDependentConfig)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, st)

	require.Empty(t, h.plugin.GetCalls())
	require.Equal(t, 0, st.ResourceCount())
}

func TestApplyWithCancelledContextKeepsPreviousEntryOfExistingResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleOriginalConfig)
	h.plugin.ResetCalls()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := h.newParser(t, nil)

	// the edited config changes the network, but it is never reached
	st, err := p.Apply(ctx, lifecycleEditedConfig)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, st)

	require.Empty(t, h.plugin.GetCalls())

	network := findResource[structs.Network](t, st.GetResources(), lifecycleNetworkID)
	require.Equal(t, "10.0.0.0/16", network.Subnet)
	require.Equal(t, types.StatusCreated, network.Meta.Status)
}

func TestCancellingApplyMidWalkLeavesUnreachedNewResourceOutOfState(t *testing.T) {
	h := setupLifecycle(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// b references a, so b is only reached once a's create has succeeded,
	// by which time the operation is cancelled
	collector := &eventCollector{}
	emit := func(e events.Event) {
		collector.collect(e)

		if e.ResourceID == referenceSourceID && e.Operation == events.OperationCreate && e.Phase == events.PhaseSuccess {
			cancel()
		}
	}

	p := h.newParser(t, emit)

	st, err := p.Apply(ctx, lifecycleReferenceConfig)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, st)

	require.Equal(t, []string{"create " + referenceSourceID}, h.plugin.GetCalls())
	require.Empty(t, eventsFor(collector.all(), referenceReferenceID))

	a := findResource[structs.Network](t, st.GetResources(), referenceSourceID)
	require.Equal(t, types.StatusCreated, a.Meta.Status)

	_, err = findByID(st.GetResources(), referenceReferenceID)
	require.Error(t, err, "b was never reached, it has no entry in the state")
}

func TestCancellingApplyDoesNotCancelContextOfRunningProviderCall(t *testing.T) {
	h := setupLifecycle(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resolver := &cancellingResolver{resolver: h.registry, cancel: cancel}

	options := testOptions(t)
	options.PluginRegistry = h.registry
	options.StateStore = h.store
	options.ProviderResolver = resolver

	p := NewParser(options)

	st, err := p.Apply(ctx, lifecycleOriginalConfig)
	require.ErrorIs(t, err, context.Canceled)

	// the operation was cancelled while Create ran, yet the context Create
	// was given was not
	require.Equal(t, []error{nil}, resolver.recorded())

	// the running create finished, so the network is in the state
	require.Equal(t, []string{"create " + lifecycleNetworkID}, h.plugin.GetCalls())
	network := findResource[structs.Network](t, st.GetResources(), lifecycleNetworkID)
	require.Equal(t, types.StatusCreated, network.Meta.Status)
}

func TestDestroyWithCancelledContextDestroysNothingAndKeepsEveryResource(t *testing.T) {
	h := setupLifecycle(t)
	h.applyAndSave(t, lifecycleDependentConfig)
	h.plugin.ResetCalls()

	saved := h.loadSaved(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	collector := &eventCollector{}
	p := h.newParser(t, collector.collect)

	remaining, err := p.Destroy(ctx, saved)
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, remaining)

	require.Empty(t, h.plugin.GetCalls())
	require.Empty(t, collector.all())
	require.Equal(t, stateIDs(t, saved), stateIDs(t, remaining.GetResources()))
	require.Equal(t, stateIDs(t, saved), stateIDs(t, h.loadSaved(t)))
}

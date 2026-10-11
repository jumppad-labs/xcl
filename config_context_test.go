package xcl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// contextTestTimeout bounds every wait in these tests, so a cancellation that
// does not propagate fails the test instead of hanging it
const contextTestTimeout = 5 * time.Second

// cancelledContext returns a context that is already cancelled
func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	return ctx
}

// applyDependentChainCancelledDuringFirstCreate applies dependentChainConfig
// under a context that is cancelled while network.first's create is held in
// progress by the TestPlugin's create hook. The hook is released once the
// context is cancelled, so network.first's create finishes and the walker
// then reaches container.second only after the cancellation. It returns the
// fixture and the error ApplyContext returned.
func applyDependentChainCancelledDuringFirstCreate(t *testing.T) (*deliveryFixture, error) {
	t.Helper()

	f := setupDeliveryConfig(t, dependentChainConfig)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	release := make(chan struct{})

	var startedOnce sync.Once
	f.plugin.SetCreateHook(func(id string) {
		if id != "resource.network.first" {
			return
		}

		startedOnce.Do(func() { close(started) })
		<-release
	})

	// ApplyContext blocks until the hook is released, so it runs on its own
	// goroutine while the test cancels the context
	applied := make(chan error, 1)
	go func() {
		applied <- f.config.ApplyContext(ctx, f.configFile)
	}()

	select {
	case <-started:
	case <-time.After(contextTestTimeout):
		close(release)
		t.Fatal("the create of resource.network.first never started")
	}

	cancel()
	close(release)

	select {
	case err := <-applied:
		return f, err
	case <-time.After(contextTestTimeout):
		t.Fatal("ApplyContext did not return after its context was cancelled")
	}

	return nil, nil
}

func TestApplyContextCancelledStopsNewProviderCalls(t *testing.T) {
	f, err := applyDependentChainCancelledDuringFirstCreate(t)
	require.Error(t, err)

	// the create in progress when the context was cancelled finished, the
	// create of container.second, which depends on it, was never started
	require.Equal(t, []string{"create resource.network.first"}, f.plugin.GetCalls())
	require.Equal(t, []string{"resource.network.first"}, f.plugin.GetCreatedResources())
}

func TestApplyContextCancelledSavesProgress(t *testing.T) {
	f, err := applyDependentChainCancelledDuringFirstCreate(t)
	require.Error(t, err)

	// the state holds exactly the resource whose create finished, the
	// resource that was never reached has no entry
	require.Equal(t, []string{"resource.network.first"}, savedIDs(t, f.store))
}

func TestApplyContextReturnsContextCanceled(t *testing.T) {
	_, err := applyDependentChainCancelledDuringFirstCreate(t)

	require.ErrorIs(t, err, context.Canceled)
}

func TestApplyContextWithCancelledContextCallsNoProvider(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	err := f.config.ApplyContext(cancelledContext(), f.configFile)
	require.ErrorIs(t, err, context.Canceled)

	require.Empty(t, f.plugin.GetCalls())
	require.Empty(t, savedIDs(t, f.store), "a cancelled apply that reached no resource saves nothing")
}

func TestApplyWithoutContextMatchesApplyContextWithBackground(t *testing.T) {
	withoutContext := setupDeliveryConfig(t, independentNetworksConfig)

	err := withoutContext.config.Apply(withoutContext.configFile)
	require.NoError(t, err)

	withContext := setupDeliveryConfig(t, independentNetworksConfig)

	err = withContext.config.ApplyContext(context.Background(), withContext.configFile)
	require.NoError(t, err)

	require.Equal(t, sortedCreatedResources(withoutContext.plugin), sortedCreatedResources(withContext.plugin))
	require.Equal(t, savedIDs(t, withoutContext.store), savedIDs(t, withContext.store))
	require.Equal(t, []string{"resource.network.one", "resource.network.three", "resource.network.two"}, savedIDs(t, withContext.store))
}

func TestDestroyContextCancelledKeepsRemainingResources(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)
	require.Equal(t, []string{"resource.container.second", "resource.network.first"}, savedIDs(t, f.store))

	f.plugin.ResetCalls()

	// the context is cancelled before the destroy starts, so no resource is
	// reached: none is destroyed and every one stays in the saved state
	err = f.config.DestroyContext(cancelledContext())
	require.ErrorIs(t, err, context.Canceled)

	require.Empty(t, f.plugin.GetDestroyedResources())
	require.Empty(t, f.plugin.GetCalls())
	require.Equal(t, []string{"resource.container.second", "resource.network.first"}, savedIDs(t, f.store))
}

func TestDiffContextCancelledReturnsContextCanceled(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	// a diff against a saved state reads and compares through the provider,
	// so a diff that reaches a resource is visible as a provider call
	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	f.plugin.ResetCalls()

	result, err := f.config.DiffContext(cancelledContext(), []string{f.configFile})
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)

	require.Empty(t, f.plugin.GetCalls())
}

func TestLoadContextCancelledReturnsContextCanceled(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	// a second Config on the same store holds nothing until it loads
	loading, _ := newLoadingConfig(t, f.store)
	require.Empty(t, loading.Entities())

	err = loading.LoadContext(cancelledContext())
	require.ErrorIs(t, err, context.Canceled)

	// the cancelled load read nothing into the Config
	require.Empty(t, loading.Entities())
}

func TestValidateContextCancelledReturnsContextCanceled(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	err := f.config.ValidateContext(cancelledContext(), f.configFile)
	require.ErrorIs(t, err, context.Canceled)

	// validate reads no provider and the cancelled one read no configuration
	require.Empty(t, f.plugin.GetCalls())
}

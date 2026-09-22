package xcl

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// independentNetworksConfig declares three networks, none depends on another
// so the walker creates them in parallel
const independentNetworksConfig = `
resource "network" "one" {
  subnet = "10.0.1.0/24"
}

resource "network" "two" {
  subnet = "10.0.2.0/24"
}

resource "network" "three" {
  subnet = "10.0.3.0/24"
}
`

// independentNetworksEventCount is the number of events the first apply of
// independentNetworksConfig delivers: the apply start, the TestPlugin's load
// start and load success, a parse for each network, a create start and a
// create success for each network, and the apply success
const independentNetworksEventCount = 13

// dependentChainConfig declares network.first and container.second, which
// depends on network.first through a reference
const dependentChainConfig = `
resource "network" "first" {
  subnet = "10.0.0.0/16"
}

resource "container" "second" {
  network {
    name = resource.network.first.meta.name
  }
}
`

// deliveryFixture is a Config wired to the TestPlugin and a real file state
// store, with the configuration it applies written to a temporary file
type deliveryFixture struct {
	config     *Config
	plugin     *parser.TestPlugin
	store      *state.FileStateStore
	configFile string
}

// setupDeliveryConfig registers the TestPlugin, creates a file state store,
// writes contents to a configuration file and builds a Config with the given
// extra options, such as an event handler
func setupDeliveryConfig(t *testing.T, contents string, opts ...ConfigOption) *deliveryFixture {
	t.Helper()

	home := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())

	t.Cleanup(func() {
		os.Setenv("HOME", home)
	})

	pr := registry.NewPluginRegistry()

	testPlugin := &parser.TestPlugin{}
	err := pr.RegisterPlugin(testPlugin)
	require.NoError(t, err)

	store, err := state.NewFileStateStore(filepath.Join(t.TempDir(), "state.json"), pr)
	require.NoError(t, err)

	configFile := filepath.Join(t.TempDir(), "main.xcl")
	err = os.WriteFile(configFile, []byte(contents), 0644)
	require.NoError(t, err)

	options := append([]ConfigOption{
		WithPluginRegistry(pr),
		WithStateStore(store),
	}, opts...)

	return &deliveryFixture{
		config:     NewConfig(options...),
		plugin:     testPlugin,
		store:      store,
		configFile: configFile,
	}
}

// snapshot returns a copy of every event the recorder has received so far
func (r *eventRecorder) snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]Event{}, r.events...)
}

// withoutBlocked returns the events that are not blocked announcements
func withoutBlocked(recorded []Event) []Event {
	kept := []Event{}
	for _, e := range recorded {
		if e.Operation == events.OperationEvents && e.Phase == events.PhaseBlocked {
			continue
		}

		kept = append(kept, e)
	}

	return kept
}

// onlyBlocked returns the blocked announcements
func onlyBlocked(recorded []Event) []Event {
	kept := []Event{}
	for _, e := range recorded {
		if e.Operation == events.OperationEvents && e.Phase == events.PhaseBlocked {
			kept = append(kept, e)
		}
	}

	return kept
}

// savedIDs returns the sorted IDs of every entity in the store
func savedIDs(t *testing.T, store *state.FileStateStore) []string {
	t.Helper()

	entities, err := store.Load()
	require.NoError(t, err)

	ids := []string{}
	for _, entity := range entities {
		meta, err := types.GetMeta(entity)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)

	return ids
}

// sortedCreatedResources returns the sorted IDs of every resource the plugin
// was asked to create
func sortedCreatedResources(p *parser.TestPlugin) []string {
	created := p.GetCreatedResources()
	sort.Strings(created)

	return created
}

// slowReceiver returns a handler that records every event and then sleeps
// for delay
func slowReceiver(recorder *eventRecorder, delay time.Duration) EventHandler {
	return func(e Event) {
		recorder.handle(e)
		time.Sleep(delay)
	}
}

func TestApplyDeliversEveryEventBeforeReturning(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	atReturn := recorder.snapshot()

	time.Sleep(100 * time.Millisecond)

	afterWait := recorder.snapshot()

	require.Len(t, atReturn, independentNetworksEventCount)
	require.Len(t, afterWait, len(atReturn))

	last := atReturn[len(atReturn)-1]
	require.Equal(t, events.OperationApply, last.Operation)
	require.Equal(t, events.PhaseSuccess, last.Phase)
}

func TestApplyDeliversErrorEventBeforeReturningError(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	injected := errors.New("network-one-refused")
	f.plugin.SetCreateError("resource.network.one", injected)

	err := f.config.Apply(f.configFile)
	require.ErrorContains(t, err, injected.Error())

	atReturn := recorder.snapshot()
	require.NotEmpty(t, atReturn)

	last := atReturn[len(atReturn)-1]
	require.Equal(t, events.OperationApply, last.Operation)
	require.Equal(t, events.PhaseError, last.Phase)
	require.Equal(t, err, last.Error)
}

func TestValidateDeliversEveryEventBeforeReturning(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	err := f.config.Validate(f.configFile)
	require.NoError(t, err)

	atReturn := recorder.snapshot()

	time.Sleep(100 * time.Millisecond)

	afterWait := recorder.snapshot()

	// the validate start, the TestPlugin's load start and success, a parse
	// for each network and the validate success
	require.Len(t, atReturn, 7)
	require.Len(t, afterWait, len(atReturn))

	last := atReturn[len(atReturn)-1]
	require.Equal(t, events.OperationValidate, last.Operation)
	require.Equal(t, events.PhaseSuccess, last.Phase)
}

func TestDestroyDeliversEveryEventBeforeReturning(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	recorder.mu.Lock()
	recorder.events = nil
	recorder.mu.Unlock()

	err = f.config.Destroy()
	require.NoError(t, err)

	atReturn := recorder.snapshot()

	time.Sleep(100 * time.Millisecond)

	afterWait := recorder.snapshot()

	// the destroy start, a destroy start and success for each network and
	// the destroy success
	require.Len(t, atReturn, 8)
	require.Len(t, afterWait, len(atReturn))

	last := atReturn[len(atReturn)-1]
	require.Equal(t, events.OperationDestroy, last.Operation)
	require.Equal(t, events.PhaseSuccess, last.Phase)
	require.Empty(t, last.ResourceID)
}

func TestEveryEventTimeFallsWithinTheCall(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	before := time.Now()
	err := f.config.Apply(f.configFile)
	after := time.Now()
	require.NoError(t, err)

	recorded := recorder.snapshot()
	require.Len(t, recorded, independentNetworksEventCount)

	for _, e := range recorded {
		require.False(t, e.Time.Before(before), "event %s %s %s is before the call", e.ResourceID, e.Operation, e.Phase)
		require.False(t, e.Time.After(after), "event %s %s %s is after the call", e.ResourceID, e.Operation, e.Phase)
	}
}

func TestEveryCoreEventNamesCoreAsSource(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	// the TestPlugin writes no log messages, so every event is produced by
	// xcl itself
	recorded := recorder.snapshot()
	require.Len(t, recorded, independentNetworksEventCount)

	for _, e := range recorded {
		require.Equal(t, events.SourceCore, e.Source, "event %s %s %s", e.ResourceID, e.Operation, e.Phase)
	}
}

func TestApplyDoesNotWaitForSlowReceiver(t *testing.T) {
	delay := 100 * time.Millisecond

	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig,
		WithEventHandler(slowReceiver(recorder, delay)),
		WithEventBufferSize(64),
	)

	started := time.Now()
	err := f.config.Apply(f.configFile)
	returned := time.Now()
	require.NoError(t, err)

	// every event was handled before Apply returned, which takes at least
	// one delay per event
	require.Len(t, recorder.snapshot(), independentNetworksEventCount)
	require.GreaterOrEqual(t, returned.Sub(started), independentNetworksEventCount*delay)

	// the provider's calls all finished long before the receiver caught up
	calls := f.plugin.GetCallTimes()
	require.Len(t, calls, 3)

	lastFinished := calls[0].Finished
	for _, c := range calls {
		if c.Finished.After(lastFinished) {
			lastFinished = c.Finished
		}
	}

	require.Less(t, lastFinished.Sub(started), independentNetworksEventCount*delay/2)
}

func TestApplyWithBufferOfOneDeliversEveryEvent(t *testing.T) {
	fastRecorder := &eventRecorder{}
	fast := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(fastRecorder.handle))

	err := fast.config.Apply(fast.configFile)
	require.NoError(t, err)

	slowRecorder := &eventRecorder{}
	slow := setupDeliveryConfig(t, independentNetworksConfig,
		WithEventHandler(slowReceiver(slowRecorder, 20*time.Millisecond)),
		WithEventBufferSize(1),
	)

	err = slow.config.Apply(slow.configFile)
	require.NoError(t, err)

	require.Len(t, withoutBlocked(fastRecorder.snapshot()), independentNetworksEventCount)
	require.Len(t, withoutBlocked(slowRecorder.snapshot()), len(withoutBlocked(fastRecorder.snapshot())))
}

func TestApplyWithBufferOfOneAnnouncesBlocking(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig,
		WithEventHandler(slowReceiver(recorder, 20*time.Millisecond)),
		WithEventBufferSize(1),
	)

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	recorded := recorder.snapshot()

	// a run can hold several blocked stretches, each announced once
	blocked := onlyBlocked(recorded)
	require.NotEmpty(t, blocked)
	for _, e := range blocked {
		require.Greater(t, e.Duration, time.Duration(0))
		require.Equal(t, events.SourceCore, e.Source)
		require.Empty(t, e.ResourceID)
	}

	require.Len(t, withoutBlocked(recorded), independentNetworksEventCount)

	last := recorded[len(recorded)-1]
	require.Equal(t, events.OperationApply, last.Operation)
	require.Equal(t, events.PhaseSuccess, last.Phase)
}

func TestReceiverIsNeverCalledConcurrently(t *testing.T) {
	var inFlight atomic.Int32
	var mostInFlight atomic.Int32
	var handled atomic.Int32

	receiver := func(e Event) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)

		for {
			most := mostInFlight.Load()
			if current <= most || mostInFlight.CompareAndSwap(most, current) {
				break
			}
		}

		handled.Add(1)

		// hold the call long enough for a concurrent call to overlap it
		time.Sleep(2 * time.Millisecond)
	}

	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(receiver))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	require.Equal(t, int32(independentNetworksEventCount), handled.Load())
	require.Equal(t, int32(1), mostInFlight.Load())
}

func TestResourceEventsArriveInStartSuccessOrder(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	for _, id := range []string{"resource.network.one", "resource.network.two", "resource.network.three"} {
		startIndex := eventIndex(recorder, id, events.OperationCreate, events.PhaseStart)
		successIndex := eventIndex(recorder, id, events.OperationCreate, events.PhaseSuccess)

		require.NotEqual(t, -1, startIndex, "no create start for %s", id)
		require.NotEqual(t, -1, successIndex, "no create success for %s", id)
		require.Less(t, startIndex, successIndex, "create success for %s arrived before its start", id)
	}
}

func TestFailedCreateStopsDependantsWithReceiver(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, dependentChainConfig, WithEventHandler(recorder.handle))

	injected := errors.New("network-first-refused")
	f.plugin.SetCreateError("resource.network.first", injected)

	err := f.config.Apply(f.configFile)
	require.ErrorContains(t, err, injected.Error())

	require.Equal(t, []string{"resource.network.first"}, f.plugin.GetCreatedResources())
	require.Empty(t, recorder.find("resource.container.second", events.OperationCreate, events.PhaseStart))
}

func TestFailedCreateStopsDependantsWithoutReceiver(t *testing.T) {
	f := setupDeliveryConfig(t, dependentChainConfig)

	injected := errors.New("network-first-refused")
	f.plugin.SetCreateError("resource.network.first", injected)

	err := f.config.Apply(f.configFile)
	require.ErrorContains(t, err, injected.Error())

	require.Equal(t, []string{"resource.network.first"}, f.plugin.GetCreatedResources())
}

func TestFailedCreateEmitsErrorEventWithReturnedError(t *testing.T) {
	recorder := &eventRecorder{}
	f := setupDeliveryConfig(t, dependentChainConfig, WithEventHandler(recorder.handle))

	injected := errors.New("network-first-refused")
	f.plugin.SetCreateError("resource.network.first", injected)

	err := f.config.Apply(f.configFile)
	require.ErrorIs(t, err, injected)

	// the event carries the provider's own error, the returned error wraps
	// the same failure
	failed := recorder.find("resource.network.first", events.OperationCreate, events.PhaseError)
	require.Len(t, failed, 1)
	require.ErrorIs(t, failed[0].Error, injected)
	require.ErrorContains(t, err, failed[0].Error.Error())
}

func TestSlowReceiverDoesNotChangeApplyResult(t *testing.T) {
	injected := errors.New("network-two-refused")

	withoutReceiver := setupDeliveryConfig(t, independentNetworksConfig)
	withoutReceiver.plugin.SetCreateError("resource.network.two", injected)

	expectedErr := withoutReceiver.config.Apply(withoutReceiver.configFile)
	require.ErrorContains(t, expectedErr, injected.Error())

	recorder := &eventRecorder{}
	withReceiver := setupDeliveryConfig(t, independentNetworksConfig,
		WithEventHandler(slowReceiver(recorder, 20*time.Millisecond)),
	)
	withReceiver.plugin.SetCreateError("resource.network.two", injected)

	// apply the same file, the returned error names the file it was declared in
	err := withReceiver.config.Apply(withoutReceiver.configFile)
	require.Equal(t, expectedErr.Error(), err.Error())

	require.Equal(t, sortedCreatedResources(withoutReceiver.plugin), sortedCreatedResources(withReceiver.plugin))
	require.Equal(t, savedIDs(t, withoutReceiver.store), savedIDs(t, withReceiver.store))
}

func TestIgnoringReceiverDoesNotChangeApplyResult(t *testing.T) {
	injected := errors.New("network-two-refused")

	withoutReceiver := setupDeliveryConfig(t, independentNetworksConfig)
	withoutReceiver.plugin.SetCreateError("resource.network.two", injected)

	expectedErr := withoutReceiver.config.Apply(withoutReceiver.configFile)
	require.ErrorContains(t, expectedErr, injected.Error())

	ignore := func(e Event) {}
	withReceiver := setupDeliveryConfig(t, independentNetworksConfig, WithEventHandler(ignore))
	withReceiver.plugin.SetCreateError("resource.network.two", injected)

	// apply the same file, the returned error names the file it was declared in
	err := withReceiver.config.Apply(withoutReceiver.configFile)
	require.Equal(t, expectedErr.Error(), err.Error())

	require.Equal(t, sortedCreatedResources(withoutReceiver.plugin), sortedCreatedResources(withReceiver.plugin))
	require.Equal(t, savedIDs(t, withoutReceiver.store), savedIDs(t, withReceiver.store))
}

func TestWithEventBufferSizeSetsLimit(t *testing.T) {
	c := NewConfig(WithEventBufferSize(7))

	require.Equal(t, 7, c.eventBufferSize)
}

func TestDefaultEventBufferSizeIsUsedWhenUnset(t *testing.T) {
	c := NewConfig()

	// run treats an unset size as DefaultEventBufferSize
	require.Equal(t, 0, c.eventBufferSize)
	require.Equal(t, 1024, DefaultEventBufferSize)
}

// captureStandardStreams points os.Stdout and os.Stderr at pipes until the
// returned function is called, which puts them back and returns everything
// written to each
func captureStandardStreams(t *testing.T) func() (string, string) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	restored := false
	restore := func() {
		if restored {
			return
		}

		restored = true
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}

	t.Cleanup(restore)

	var wg sync.WaitGroup
	var stdout, stderr bytes.Buffer

	wg.Go(func() { io.Copy(&stdout, stdoutReader) })
	wg.Go(func() { io.Copy(&stderr, stderrReader) })

	return func() (string, string) {
		restore()

		stdoutWriter.Close()
		stderrWriter.Close()
		wg.Wait()

		stdoutReader.Close()
		stderrReader.Close()

		return stdout.String(), stderr.String()
	}
}

func TestApplyWithoutReceiverWritesNothingToStdoutOrStderr(t *testing.T) {
	f := setupDeliveryConfig(t, independentNetworksConfig)

	finish := captureStandardStreams(t)

	err := f.config.Apply(f.configFile)

	stdout, stderr := finish()

	require.NoError(t, err)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

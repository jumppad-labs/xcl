package e2e_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/testutil"
	"github.com/jumppad-labs/xcl/plugins/registry"
)

// TestParseEventCarriesFile asserts each resource's parse is reported as a
// success by core with the file it was parsed from
func TestParseEventCarriesFile(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	files := map[string]string{}
	for _, e := range kubeEventsWithOperation(recorded, events.OperationParse) {
		require.Equal(t, events.SourceCore, e.Source)
		require.Equal(t, events.PhaseSuccess, e.Phase)
		files[e.ResourceID] = filepath.Base(e.File)
	}

	require.Equal(t, map[string]string{
		"variable.image_tag": "deployment.xcl",
		"variable.replicas":  "deployment.xcl",
		"config_map.api":     "deployment.xcl",
		"deployment.api":     "deployment.xcl",
		"secret.db":          "secret.xcl",
		"service.api":        "ingress.xcl",
		"ingress.api":        "ingress.xcl",
		"output.api_url":     "ingress.xcl",
	}, files)
}

// TestCreateOfRegisteredTypeReportsSuccessWithoutStart asserts every
// resource's create is reported as a success only, registered types have no
// provider so there is nothing to start
func TestCreateOfRegisteredTypeReportsSuccessWithoutStart(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	ids := []string{}
	for _, e := range kubeEventsWithOperation(recorded, events.OperationCreate) {
		require.Equal(t, events.PhaseSuccess, e.Phase)
		ids = append(ids, e.ResourceID)
	}
	sort.Strings(ids)

	require.Equal(t, kubeDeclaredIDs, ids)
}

// TestRegisteredTypesReportNoLogEvents asserts a successful apply and destroy
// of registered types reports no log messages, without plugins there is
// nothing to write them
func TestRegisteredTypesReportNoLogEvents(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	for _, e := range recorded {
		require.NotEqual(t, events.PhaseLog, e.Phase, "unexpected log event: %+v", e)
	}
}

// TestSuccessfulLifecycleReportsNoErrors asserts a successful apply and
// destroy reports no error event and no event carrying an error
func TestSuccessfulLifecycleReportsNoErrors(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	for _, e := range recorded {
		require.NotEqual(t, events.PhaseError, e.Phase, "unexpected error event: %+v", e)
		require.NoError(t, e.Error, "unexpected event with an error: %+v", e)
	}
}

// TestRegisteredTypesReportEveryEventFromCore asserts every event comes from
// xcl itself when no plugin is registered
func TestRegisteredTypesReportEveryEventFromCore(t *testing.T) {
	recorded := kubeLifecycleEvents(t)
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.Equal(t, events.SourceCore, e.Source, "event from another source: %+v", e)
	}
}

// TestEveryEventCarriesOperationAndPhase asserts every event says what was
// being done and where in it the event sits
func TestEveryEventCarriesOperationAndPhase(t *testing.T) {
	recorded := kubeLifecycleEvents(t)
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.NotEmpty(t, e.Operation, "event without an operation: %+v", e)
		require.NotEmpty(t, e.Phase, "event without a phase: %+v", e)
	}
}

// TestParseErrorEventNamesBlockAndFile asserts a block that fails to parse is
// reported as a parse error, naming the block and its file
func TestParseErrorEventNamesBlockAndFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.xcl")
	require.NoError(t, os.WriteFile(file, []byte(`resource "nosuchtype" "broken" {}`), 0644))

	recorder := &testutil.EventRecorder{}
	c := newKubeConfig(t, registry.NewPluginRegistry(), recorder.Record, t.TempDir(), testStateKey)

	require.Error(t, c.Apply(dir))

	parses := kubeEventsWithOperation(recorder.Events(), events.OperationParse)
	failed := testutil.EventsWithPhase(parses, events.PhaseError)

	require.Len(t, failed, 1)
	require.Equal(t, events.SourceCore, failed[0].Source)
	require.Equal(t, "resource.nosuchtype.broken", failed[0].ResourceID)
	require.Equal(t, file, failed[0].File)
	require.Error(t, failed[0].Error)
}

// TestDestroyOfRegisteredTypeReportsSuccessWithoutStart asserts every applied
// resource's destroy is reported as a success only, registered types have no
// provider so there is nothing to start
func TestDestroyOfRegisteredTypeReportsSuccessWithoutStart(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	ids := []string{}
	for _, e := range kubeEventsWithOperation(recorded, events.OperationDestroy) {
		// the destroy operation's own start and success concern no resource
		if e.ResourceID == "" {
			continue
		}

		require.Equal(t, events.PhaseSuccess, e.Phase)
		ids = append(ids, e.ResourceID)
	}
	sort.Strings(ids)

	require.Equal(t, kubeDeclaredIDs, ids)
}

// TestDestroyOperationReportsStartFirstAndSuccessLast asserts the destroy as
// a whole is reported, starting before any resource and succeeding after
// every one
func TestDestroyOperationReportsStartFirstAndSuccessLast(t *testing.T) {
	recorded := kubeLifecycleEvents(t)

	destroy := kubeEventsWithOperation(recorded, events.OperationDestroy)
	require.NotEmpty(t, destroy)

	first := destroy[0]
	require.Equal(t, events.PhaseStart, first.Phase)
	require.Empty(t, first.ResourceID)

	last := destroy[len(destroy)-1]
	require.Equal(t, events.PhaseSuccess, last.Phase)
	require.Empty(t, last.ResourceID)
}

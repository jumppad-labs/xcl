package parser

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/stretchr/testify/require"
)

// parseEventRecorder records the parse events of a parser, walk events are
// not recorded. Parsing modules recurses, and the walk fires events from
// concurrent goroutines, so recording is guarded by a mutex.
type parseEventRecorder struct {
	mu       sync.Mutex
	recorded []events.Event
}

func (r *parseEventRecorder) record(e events.Event) {
	if e.Operation != events.OperationParse {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.recorded = append(r.recorded, e)
}

// withPhase returns the recorded parse events with the given phase
func (r *parseEventRecorder) withPhase(phase string) []events.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := []events.Event{}
	for _, e := range r.recorded {
		if e.Phase == phase {
			found = append(found, e)
		}
	}

	return found
}

func TestApplyFiresParseEventForEveryResourceWithItsFile(t *testing.T) {
	mainFile, err := filepath.Abs(registeredBasicConfig)
	require.NoError(t, err)
	moduleFile := filepath.Join(filepath.Dir(mainFile), "module", "db.xcl")

	recorder := &parseEventRecorder{}
	h := setupRegisteredTypes(t)
	p := h.newParser(t, recorder.record)

	_, err = p.Apply(context.Background(), mainFile)
	require.NoError(t, err)

	files := map[string]string{}
	resourceTypes := map[string]string{}
	for _, e := range recorder.withPhase("success") {
		require.NoError(t, e.Error)
		require.Equal(t, events.SourceCore, e.Source)
		files[e.ResourceID] = e.File
		resourceTypes[e.ResourceID] = e.ResourceType
	}

	require.Equal(t, map[string]string{
		"variable.environment":                   mainFile,
		"resource.database.main":                 mainFile,
		"module.shared":                          mainFile,
		"resource.app.web":                       mainFile,
		"resource.consumer.reader":               mainFile,
		"module.shared.resource.database.shared": moduleFile,
		"module.shared.output.location":          moduleFile,
	}, files)

	require.Equal(t, "database.main", resourceTypes["resource.database.main"])
	require.Equal(t, "variable.environment", resourceTypes["variable.environment"])
	require.Equal(t, "module.shared", resourceTypes["module.shared"])
	require.Empty(t, recorder.withPhase("error"))
}

func TestParseEventHasNoStartPhase(t *testing.T) {
	recorder := &parseEventRecorder{}
	h := setupRegisteredTypes(t)
	p := h.newParser(t, recorder.record)

	_, err := p.Apply(context.Background(), registeredBasicConfig)
	require.NoError(t, err)

	require.Empty(t, recorder.withPhase("start"))
}

func TestValidateFiresParseEvents(t *testing.T) {
	recorder := &parseEventRecorder{}
	h := setupRegisteredTypes(t)
	p := h.newParser(t, recorder.record)

	err := p.Validate(context.Background(), registeredBasicConfig)
	require.NoError(t, err)

	require.Len(t, recorder.withPhase("success"), 7)
}

func TestParseEventReportsResourceThatFailsToParse(t *testing.T) {
	file, err := filepath.Abs("../test_fixtures/config/unregistered_type/unknown.xcl")
	require.NoError(t, err)

	recorder := &parseEventRecorder{}
	options := testOptions(t)
	options.Emit = recorder.record
	p, _ := setupParser(t, options)

	_, err = p.Apply(context.Background(), file)
	require.Error(t, err)

	failed := recorder.withPhase("error")
	require.Len(t, failed, 1)
	require.Equal(t, "resource.nosuchtype.example", failed[0].ResourceID)
	require.Equal(t, "nosuchtype.example", failed[0].ResourceType)
	require.Equal(t, file, failed[0].File)
	require.ErrorContains(t, failed[0].Error, "nosuchtype")
}

func TestParseEventReportsResourcesThatParseAlongsideOneThatFails(t *testing.T) {
	file, err := filepath.Abs("../test_fixtures/config/unregistered_type/unknown.xcl")
	require.NoError(t, err)

	recorder := &parseEventRecorder{}
	options := testOptions(t)
	options.Emit = recorder.record
	p, _ := setupParser(t, options)

	_, err = p.Apply(context.Background(), file)
	require.Error(t, err)

	parsed := recorder.withPhase("success")
	require.Len(t, parsed, 1)
	require.Equal(t, "resource.network.onprem", parsed[0].ResourceID)
	require.Equal(t, file, parsed[0].File)
}

func TestParseEventReportsFileWithInvalidSyntax(t *testing.T) {
	file, err := filepath.Abs("../test_fixtures/config/process_error/bad_format.xcl")
	require.NoError(t, err)

	recorder := &parseEventRecorder{}
	options := testOptions(t)
	options.Emit = recorder.record
	p, _ := setupParser(t, options)

	_, err = p.Apply(context.Background(), file)
	require.Error(t, err)

	failed := recorder.withPhase("error")
	require.NotEmpty(t, failed)
	for _, e := range failed {
		require.Empty(t, e.ResourceID, "a syntax error is not in a resource")
		require.Equal(t, file, e.File)
		require.Error(t, e.Error)
	}

	require.Empty(t, recorder.withPhase("success"))
}

func TestParseEventForModuleComesBeforeItsResources(t *testing.T) {
	recorder := &parseEventRecorder{}
	h := setupRegisteredTypes(t)
	p := h.newParser(t, recorder.record)

	_, err := p.Apply(context.Background(), registeredBasicConfig)
	require.NoError(t, err)

	order := map[string]int{}
	for i, e := range recorder.withPhase("success") {
		order[e.ResourceID] = i
	}

	require.Less(t, order["module.shared"], order["module.shared.resource.database.shared"])
	require.Less(t, order["module.shared"], order["module.shared.output.location"])
}

func TestParseEventReportsModuleWhoseSourceIsMissing(t *testing.T) {
	file, err := filepath.Abs("../test_fixtures/config/missing_module_source/missing.xcl")
	require.NoError(t, err)

	recorder := &parseEventRecorder{}
	options := testOptions(t)
	options.Emit = recorder.record
	p, _ := setupParser(t, options)

	_, err = p.Apply(context.Background(), file)
	require.Error(t, err)

	failed := recorder.withPhase("error")
	require.Len(t, failed, 1)
	require.Equal(t, "module.gone", failed[0].ResourceID)
	require.Equal(t, "module.gone", failed[0].ResourceType)
	require.Equal(t, file, failed[0].File)
	require.ErrorContains(t, failed[0].Error, "does_not_exist")

	require.Empty(t, recorder.withPhase("success"))
}

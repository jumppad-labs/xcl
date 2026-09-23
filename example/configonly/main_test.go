package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/example/configonly/resources"
	"github.com/jumppad-labs/xcl/example/prettylog"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// configDir is the configuration this example parses
const configDir = "./config"

// declaredResourceIDs is every resource the configuration declares, sorted
var declaredResourceIDs = []string{
	"output.api_url",
	"resource.config_map.api",
	"resource.deployment.api",
	"resource.ingress.api",
	"resource.service.api",
	"variable.image_tag",
	"variable.replicas",
}

func resourceIDs(t *testing.T, found []any) []string {
	t.Helper()

	ids := []string{}
	for _, r := range found {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		ids = append(ids, meta.ID)
	}

	sort.Strings(ids)
	return ids
}

func findResource(t *testing.T, found []any, id string) any {
	t.Helper()

	for _, r := range found {
		meta, err := types.GetMeta(r)
		require.NoError(t, err)

		if meta.ID == id {
			return r
		}
	}

	require.Failf(t, "resource not found", "no resource with id %s", id)
	return nil
}

// deployment runs the example and returns the parsed deployment, the
// resource most of these tests are about
func deployment(t *testing.T) *resources.Deployment {
	t.Helper()

	found, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	d, ok := findResource(t, found, "resource.deployment.api").(*resources.Deployment)
	require.True(t, ok)

	return d
}

func TestConfigOnlyExampleFindsDeclaredResources(t *testing.T) {
	out := &bytes.Buffer{}

	found, err := run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Equal(t, declaredResourceIDs, resourceIDs(t, found))
}

// TestConfigOnlyExampleReturnsRegisteredGoTypes asserts each block is decoded
// into the Go type that was registered for it, a registered type is held as
// itself rather than a type generated from a schema
func TestConfigOnlyExampleReturnsRegisteredGoTypes(t *testing.T) {
	found, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	_, ok := findResource(t, found, "resource.config_map.api").(*resources.ConfigMap)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.deployment.api").(*resources.Deployment)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.service.api").(*resources.Service)
	require.True(t, ok)

	_, ok = findResource(t, found, "resource.ingress.api").(*resources.Ingress)
	require.True(t, ok)
}

// TestConfigOnlyExampleDecodesRepeatedBlocks asserts a block that appears
// more than once is decoded into a slice, in the order it was written
func TestConfigOnlyExampleDecodesRepeatedBlocks(t *testing.T) {
	d := deployment(t)

	require.Len(t, d.Containers, 2)
	require.Equal(t, "api", d.Containers[0].Name)
	require.Equal(t, "proxy", d.Containers[1].Name)

	require.Len(t, d.Containers[0].Ports, 2)
	require.Equal(t, "http", d.Containers[0].Ports[0].Name)
	require.Equal(t, 8080, d.Containers[0].Ports[0].ContainerPort)
	require.Equal(t, "metrics", d.Containers[0].Ports[1].Name)
	require.Equal(t, 9090, d.Containers[0].Ports[1].ContainerPort)

	require.Len(t, d.Containers[1].Ports, 1)
	require.Equal(t, 8081, d.Containers[1].Ports[0].ContainerPort)
}

// TestConfigOnlyExampleDecodesNestedBlocks asserts blocks nested inside a
// nested block are decoded, resources holds limits and requests
func TestConfigOnlyExampleDecodesNestedBlocks(t *testing.T) {
	d := deployment(t)

	api := d.Containers[0]
	require.NotNil(t, api.Resources)
	require.Equal(t, "500m", api.Resources.Limits.CPU)
	require.Equal(t, "512Mi", api.Resources.Limits.Memory)
	require.Equal(t, "100m", api.Resources.Requests.CPU)
	require.Equal(t, "128Mi", api.Resources.Requests.Memory)

	require.Len(t, api.VolumeMounts, 1)
	require.Equal(t, "config", api.VolumeMounts[0].Name)
	require.Equal(t, "/etc/api", api.VolumeMounts[0].Path)

	require.Len(t, d.Volumes, 1)
	require.Equal(t, "config", d.Volumes[0].Name)
}

// TestConfigOnlyExampleLeavesOmittedBlockNil asserts a block the
// configuration leaves out is nil, the proxy container sets no resources
func TestConfigOnlyExampleLeavesOmittedBlockNil(t *testing.T) {
	d := deployment(t)

	require.Nil(t, d.Containers[1].Resources)
	require.Empty(t, d.Containers[1].Env)
	require.Empty(t, d.Containers[1].VolumeMounts)
}

// TestConfigOnlyExampleReadsValuesFromConfigMap asserts values read out of a
// map attribute of another resource are resolved
func TestConfigOnlyExampleReadsValuesFromConfigMap(t *testing.T) {
	d := deployment(t)

	env := d.Containers[0].Env
	require.Len(t, env, 2)
	require.Equal(t, "DB_HOST", env[0].Name)
	require.Equal(t, "postgres.default.svc", env[0].Value)
	require.Equal(t, "LOG_LEVEL", env[1].Name)
	require.Equal(t, "info", env[1].Value)

	require.Equal(t, "resource.config_map.api", d.Volumes[0].ConfigMap)
}

// TestConfigOnlyExampleReadsVariables asserts a variable is read both as a
// value of its own and inside an interpolated string
func TestConfigOnlyExampleReadsVariables(t *testing.T) {
	d := deployment(t)

	require.Equal(t, 3, d.Replicas)
	require.Equal(t, "ghcr.io/example/api:1.2.0", d.Containers[0].Image)
}

// TestConfigOnlyExampleLinksServiceToDeployment asserts the service names the
// deployment by id and reads its target port out of it, the port coming from
// a repeated block referenced by position
func TestConfigOnlyExampleLinksServiceToDeployment(t *testing.T) {
	found, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	service, ok := findResource(t, found, "resource.service.api").(*resources.Service)
	require.True(t, ok)

	require.Equal(t, "resource.deployment.api", service.Deployment)
	require.Equal(t, 80, service.Port)
	require.Equal(t, 8080, service.TargetPort)
}

// TestConfigOnlyExampleLinksIngressToService asserts the ingress rule names
// the service it routes to by id, and reads its port
func TestConfigOnlyExampleLinksIngressToService(t *testing.T) {
	found, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	ingress, ok := findResource(t, found, "resource.ingress.api").(*resources.Ingress)
	require.True(t, ok)

	require.Equal(t, "api.example.com", ingress.Host)
	require.Len(t, ingress.Rules, 1)
	require.Equal(t, "/", ingress.Rules[0].Path)
	require.Equal(t, "resource.service.api", ingress.Rules[0].Service)
	require.Equal(t, 80, ingress.Rules[0].Port)
}

func TestConfigOnlyExamplePrintsEveryResource(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	for _, id := range declaredResourceIDs {
		require.Contains(t, out.String(), "  "+id+"\n")
	}
}

// TestConfigOnlyExamplePrintsNestedBlocks asserts the printed deployment
// walks the blocks nested inside it
func TestConfigOnlyExamplePrintsNestedBlocks(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Deployments\n")
	require.Contains(t, out.String(), "  resource.deployment.api replicas=3\n")
	require.Contains(t, out.String(), "    container api image=ghcr.io/example/api:1.2.0\n")
	require.Contains(t, out.String(), "      port http container_port=8080\n")
	require.Contains(t, out.String(), "      env DB_HOST=postgres.default.svc\n")
	require.Contains(t, out.String(), "      limits cpu=500m memory=512Mi\n")
	require.Contains(t, out.String(), "      requests cpu=100m memory=128Mi\n")
	require.Contains(t, out.String(), "      volume_mount config path=/etc/api\n")
	require.Contains(t, out.String(), "    volume config config_map=resource.config_map.api\n")
	require.Contains(t, out.String(), "    container proxy image=ghcr.io/example/proxy:0.4.1\n")
}

// TestConfigOnlyExamplePrintsLinkedResources asserts the printed service and
// ingress hold the values they read from the blocks they reference
func TestConfigOnlyExamplePrintsLinkedResources(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Service\n")
	require.Contains(t, out.String(), "  resource.service.api deployment=resource.deployment.api port=80 target_port=8080\n")
	require.Contains(t, out.String(), "## Ingress\n")
	require.Contains(t, out.String(), "  resource.ingress.api host=api.example.com\n")
	require.Contains(t, out.String(), "    rule path=/ service=resource.service.api port=80\n")
}

func TestConfigOnlyExampleFailsForMissingConfig(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), "./does-not-exist", filepath.Join(t.TempDir(), "state.json"))
	require.Error(t, err)
}

// TestConfigOnlyExampleImportsNoPluginCode asserts the configuration only
// example uses no plugin or provider code, the plugin registry is the only
// package it may use from plugins
func TestConfigOnlyExampleImportsNoPluginCode(t *testing.T) {
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imp := range file.Imports {
				path, err := strconv.Unquote(imp.Path.Value)
				require.NoError(t, err)

				if path == "github.com/jumppad-labs/xcl/plugins/registry" {
					continue
				}

				require.False(t,
					strings.HasPrefix(path, "github.com/jumppad-labs/xcl/plugins"),
					"%s imports plugin code %s", name, path,
				)
			}
		}
	}
}

// eventRecorder records every event the run reports, it is the handler the
// tests pass in place of the example's pretty printer. The handler is never
// called concurrently, the mutex guards the reads the tests make
type eventRecorder struct {
	mu     sync.Mutex
	events []xcl.Event
}

func (r *eventRecorder) handle(e xcl.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, e)
}

// snapshot returns a copy of every event recorded so far
func (r *eventRecorder) snapshot() []xcl.Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]xcl.Event{}, r.events...)
}

// withOperation returns every event reported for operation, in the order
// they were reported
func (r *eventRecorder) withOperation(operation string) []xcl.Event {
	found := []xcl.Event{}
	for _, e := range r.snapshot() {
		if e.Operation == operation {
			found = append(found, e)
		}
	}

	return found
}

// runRecordingEvents runs the example with a recorder as its event handler
// and returns the recorder once the run has succeeded
func runRecordingEvents(t *testing.T) *eventRecorder {
	t.Helper()

	recorder := &eventRecorder{}

	_, err := run(&bytes.Buffer{}, recorder.handle, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return recorder
}

// TestConfigOnlyExampleReportsParseEventWithFile asserts each resource's
// parse is reported as a success by core with the file it was parsed from
func TestConfigOnlyExampleReportsParseEventWithFile(t *testing.T) {
	recorder := runRecordingEvents(t)

	files := map[string]string{}
	for _, e := range recorder.withOperation(events.OperationParse) {
		require.Equal(t, events.SourceCore, e.Source)
		require.Equal(t, events.PhaseSuccess, e.Phase)
		files[e.ResourceID] = filepath.Base(e.File)
	}

	require.Equal(t, map[string]string{
		"variable.image_tag":      "main.xcl",
		"variable.replicas":       "main.xcl",
		"resource.config_map.api": "main.xcl",
		"resource.deployment.api": "main.xcl",
		"resource.service.api":    "main.xcl",
		"resource.ingress.api":    "main.xcl",
		"output.api_url":          "main.xcl",
	}, files)
}

// TestConfigOnlyExampleReportsCreateSuccessWithoutStart asserts every
// resource's create is reported as a success only, registered types have no
// provider so there is nothing to start
func TestConfigOnlyExampleReportsCreateSuccessWithoutStart(t *testing.T) {
	recorder := runRecordingEvents(t)

	ids := []string{}
	for _, e := range recorder.withOperation(events.OperationCreate) {
		require.Equal(t, events.PhaseSuccess, e.Phase)
		ids = append(ids, e.ResourceID)
	}
	sort.Strings(ids)

	require.Equal(t, declaredResourceIDs, ids)
}

// TestConfigOnlyExampleReportsNoLogEvents asserts a successful run reports no
// log messages, without plugins there is nothing to write them
func TestConfigOnlyExampleReportsNoLogEvents(t *testing.T) {
	recorder := runRecordingEvents(t)

	for _, e := range recorder.snapshot() {
		require.NotEqual(t, events.PhaseLog, e.Phase, "unexpected log event: %+v", e)
	}
}

// TestConfigOnlyExampleReportsNoErrors asserts a successful run reports no
// error event, only an error would stand out
func TestConfigOnlyExampleReportsNoErrors(t *testing.T) {
	recorder := runRecordingEvents(t)

	for _, e := range recorder.snapshot() {
		require.NotEqual(t, events.PhaseError, e.Phase, "unexpected error event: %+v", e)
		require.NoError(t, e.Error, "unexpected event with an error: %+v", e)
	}
}

// TestConfigOnlyExampleReportsEveryEventFromCore asserts every event the run
// reports comes from xcl itself, the example has no plugins
func TestConfigOnlyExampleReportsEveryEventFromCore(t *testing.T) {
	recorder := runRecordingEvents(t)

	recorded := recorder.snapshot()
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.Equal(t, events.SourceCore, e.Source, "event from another source: %+v", e)
	}
}

// TestConfigOnlyExampleReportsOperationAndPhaseOnEveryEvent asserts every
// event the run reports says what was being done and where in it the event
// sits
func TestConfigOnlyExampleReportsOperationAndPhaseOnEveryEvent(t *testing.T) {
	recorder := runRecordingEvents(t)

	recorded := recorder.snapshot()
	require.NotEmpty(t, recorded)

	for _, e := range recorded {
		require.NotEmpty(t, e.Operation, "event without an operation: %+v", e)
		require.NotEmpty(t, e.Phase, "event without a phase: %+v", e)
	}
}

// TestConfigOnlyExampleReportsParseErrorWithFile asserts a block that fails
// to parse is reported as a parse error, naming the block and its file
func TestConfigOnlyExampleReportsParseErrorWithFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.xcl")
	err := os.WriteFile(file, []byte(`resource "nosuchtype" "broken" {}`), 0644)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	_, err = run(&bytes.Buffer{}, recorder.handle, registry.NewPluginRegistry(), dir, filepath.Join(t.TempDir(), "state.json"))
	require.Error(t, err)

	failed := []xcl.Event{}
	for _, e := range recorder.withOperation(events.OperationParse) {
		if e.Phase == events.PhaseError {
			failed = append(failed, e)
		}
	}

	require.Len(t, failed, 1)
	require.Equal(t, events.SourceCore, failed[0].Source)
	require.Equal(t, "resource.nosuchtype.broken", failed[0].ResourceID)
	require.Equal(t, file, failed[0].File)
	require.Error(t, failed[0].Error)
}

// TestConfigOnlyExampleDestroysEverythingItApplied asserts the state saved after a run is
// empty, everything that was applied has been destroyed
func TestConfigOnlyExampleDestroysEverythingItApplied(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")

	_, err := run(&bytes.Buffer{}, nil, registry.NewPluginRegistry(), configDir, statePath)
	require.NoError(t, err)

	reg := registry.NewPluginRegistry()
	require.NoError(t, reg.RegisterType("config_map", &resources.ConfigMap{}))
	require.NoError(t, reg.RegisterType("deployment", &resources.Deployment{}))
	require.NoError(t, reg.RegisterType("service", &resources.Service{}))
	require.NoError(t, reg.RegisterType("ingress", &resources.Ingress{}))

	store, err := state.NewFileStateStore(statePath, reg)
	require.NoError(t, err)

	saved, err := store.Load()
	require.NoError(t, err)
	require.Empty(t, saved)
}

func TestConfigOnlyExamplePrintsNoResourcesRemaining(t *testing.T) {
	out := &bytes.Buffer{}

	_, err := run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	require.Contains(t, out.String(), "## Destroyed\n  0 resources remaining\n")
}

// TestConfigOnlyExampleReportsDestroySuccessWithoutStart asserts every
// applied resource's destroy is reported as a success only, registered types
// have no provider so there is nothing to start
func TestConfigOnlyExampleReportsDestroySuccessWithoutStart(t *testing.T) {
	recorder := runRecordingEvents(t)

	ids := []string{}
	for _, e := range recorder.withOperation(events.OperationDestroy) {
		// the destroy operation's own start and success concern no resource
		if e.ResourceID == "" {
			continue
		}

		require.Equal(t, events.PhaseSuccess, e.Phase)
		ids = append(ids, e.ResourceID)
	}
	sort.Strings(ids)

	require.Equal(t, declaredResourceIDs, ids)
}

// TestConfigOnlyExampleReportsDestroyOperationStartAndSuccess asserts the
// destroy as a whole is reported, starting before any resource and
// succeeding after every one
func TestConfigOnlyExampleReportsDestroyOperationStartAndSuccess(t *testing.T) {
	recorder := runRecordingEvents(t)

	destroy := recorder.withOperation(events.OperationDestroy)
	require.NotEmpty(t, destroy)

	first := destroy[0]
	require.Equal(t, events.PhaseStart, first.Phase)
	require.Empty(t, first.ResourceID)

	last := destroy[len(destroy)-1]
	require.Equal(t, events.PhaseSuccess, last.Phase)
	require.Empty(t, last.ResourceID)
}

// methodFormLookups are the lookups that the configuration also offers as
// generic methods from Go 1.27, the form an example must not be written in
var methodFormLookups = []string{"Find", "FindByType", "FindOne", "All"}

// TestConfigOnlyExampleUsesPortableLookupForm asserts the example looks
// entities up through the package level functions rather than the generic
// methods of the same name. Generic methods arrived in Go 1.27, so the method
// form would stop the code a reader copies from compiling on the project's
// minimum supported version. Entities, EntityCount and Outputs are ordinary
// methods and are not affected
func TestConfigOnlyExampleUsesPortableLookupForm(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	require.NoError(t, err)

	lookups := 0

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// a lookup names its type at the call, xcl.Find[T](c, address), so the
		// function called is the selector wrapped in an index expression
		fun := call.Fun
		switch indexed := fun.(type) {
		case *ast.IndexExpr:
			fun = indexed.X
		case *ast.IndexListExpr:
			fun = indexed.X
		}

		selector, ok := fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		if !slices.Contains(methodFormLookups, selector.Sel.Name) {
			return true
		}

		qualifier, ok := selector.X.(*ast.Ident)
		require.True(t, ok,
			"main.go calls %s as a method, which needs Go 1.27, call xcl.%s instead", selector.Sel.Name, selector.Sel.Name)

		require.Equal(t, "xcl", qualifier.Name,
			"main.go calls the %s method form, which needs Go 1.27, call xcl.%s instead", selector.Sel.Name, selector.Sel.Name)

		lookups++

		return true
	})

	require.NotZero(t, lookups, "the guard found no lookups in main.go, so it proves nothing")
}

// capturedOutput is what was written to the process's standard output and
// standard error while a function ran
type capturedOutput struct {
	stdout string
	stderr string
}

// captureStandardStreams runs fn with os.Stdout and os.Stderr redirected to
// pipes, and returns what was written to each. The streams are restored when
// fn returns, and again in cleanup should fn fail the test
func captureStandardStreams(t *testing.T, fn func()) capturedOutput {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	restore := func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}
	t.Cleanup(restore)

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var drained sync.WaitGroup
	drained.Add(2)

	go func() {
		defer drained.Done()
		_, _ = io.Copy(stdout, stdoutReader)
	}()

	go func() {
		defer drained.Done()
		_, _ = io.Copy(stderr, stderrReader)
	}()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	fn()

	restore()

	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())
	drained.Wait()

	require.NoError(t, stdoutReader.Close())
	require.NoError(t, stderrReader.Close())

	return capturedOutput{stdout: stdout.String(), stderr: stderr.String()}
}

// TestRunWithoutReceiverWritesNothingToStdoutOrStderr asserts xcl writes
// nothing of its own when no event handler is given, the report the example
// prints goes to out alone
func TestRunWithoutReceiverWritesNothingToStdoutOrStderr(t *testing.T) {
	out := &bytes.Buffer{}

	var runErr error
	captured := captureStandardStreams(t, func() {
		_, runErr = run(out, nil, registry.NewPluginRegistry(), configDir, filepath.Join(t.TempDir(), "state.json"))
	})

	require.NoError(t, runErr)
	require.Empty(t, captured.stdout)
	require.Empty(t, captured.stderr)
	require.Contains(t, out.String(), "## Resources\n")
}

// renderEvents runs the example with the pretty printer the program itself
// uses, writing to a buffer rather than the terminal, and returns everything
// it wrote. The registry is shared with the printer exactly as main shares
// it, since it is what types the entity an event carries
func renderEvents(t *testing.T) string {
	t.Helper()

	r := registry.NewPluginRegistry()
	rendered := &bytes.Buffer{}

	_, err := run(&bytes.Buffer{}, prettylog.Handler(rendered, slog.LevelInfo, r), r, configDir, filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)

	return rendered.String()
}

// TestConfigOnlyExampleShowsCreatedEntities asserts every registered type the
// example creates has its configuration written beneath the line announcing
// it, nested blocks and all
func TestConfigOnlyExampleShowsCreatedEntities(t *testing.T) {
	rendered := renderEvents(t)

	// a block is written indented beneath the line that announced it, either
	// as a resource or under its own keyword
	require.Regexp(t, `(?m)^\s+(resource "|[a-z_]+ ")`, rendered)

	require.Contains(t, rendered, `resource "config_map" "api" {`)
	require.Contains(t, rendered, `resource "deployment" "api" {`)
	require.Contains(t, rendered, `resource "service" "api" {`)
	require.Contains(t, rendered, `resource "ingress" "api" {`)

	// the nested blocks of the deployment are written too, and the formatter
	// aligns the equals signs, so the gap before one is matched rather than
	// written out
	require.Contains(t, rendered, "container {")
	require.Regexp(t, `container_port\s+= 8080`, rendered)
	require.Regexp(t, `target_port\s+= 8080`, rendered)
}

// stateAtApply captures the state file as it stood when the apply succeeded.
// The run destroys everything it applied before it returns, which leaves the
// file holding an empty array, so the records have to be read while they are
// still there
type stateAtApply struct {
	path    string
	records []json.RawMessage
	err     error
}

// handle reads the state file once the apply has succeeded, which is after
// the state was saved and before the destroy empties it again
func (s *stateAtApply) handle(e xcl.Event) {
	if e.Operation != events.OperationApply || e.Phase != events.PhaseSuccess {
		return
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		s.err = err
		return
	}

	s.err = json.Unmarshal(data, &s.records)
}

// savedID returns the address a saved record carries, which is how a record
// is matched to the entity it was written from
func savedID(t *testing.T, record json.RawMessage) string {
	t.Helper()

	var envelope struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}

	require.NoError(t, json.Unmarshal(record, &envelope))
	require.NotEmpty(t, envelope.Meta.ID)

	return envelope.Meta.ID
}

// TestConfigOnlyExampleEntityAndStateAgree asserts the configuration text of a
// saved record is identical to the text of the entity it was written from,
// for every record the apply saved. A variable or output is never written as
// configuration, so those records are skipped
func TestConfigOnlyExampleEntityAndStateAgree(t *testing.T) {
	r := registry.NewPluginRegistry()
	statePath := filepath.Join(t.TempDir(), "state.json")
	saved := &stateAtApply{path: statePath}

	applied, err := run(&bytes.Buffer{}, saved.handle, r, configDir, statePath)
	require.NoError(t, err)

	require.NoError(t, saved.err)
	require.NotEmpty(t, saved.records)

	compared := 0

	for _, record := range saved.records {
		fromState, err := xcl.EncodeSavedEntity(r, record)
		if errors.Is(err, xcl.ErrNotEncodable) {
			continue
		}
		require.NoError(t, err)

		id := savedID(t, record)

		fromEntity, err := xcl.EncodeEntity(findResource(t, applied, id))
		require.NoError(t, err)

		require.Equal(t, string(fromEntity), string(fromState), "the saved record and the entity disagree for %s", id)

		compared++
	}

	require.NotZero(t, compared, "no record was encodable, so the comparison proves nothing")
}

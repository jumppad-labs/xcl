package xcl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/plugins/example/pkg/person"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/stretchr/testify/require"
)

// externalPersonPluginName is the name the external person plugin built by
// buildExamplePlugin is known by, the Source of its log events
const externalPersonPluginName = "xcl-plugin-example"

// inProcessPersonPluginName is the Go type name of PersonPlugin, the Source
// of its log events when it runs in process
const inProcessPersonPluginName = "PersonPlugin"

// PersonPlugin is the person plugin of plugins/example, run in process. It
// registers the same resource and provider as the external plugin binary.
type PersonPlugin struct {
	plugins.PluginBase
}

func (p *PersonPlugin) Init(l logger.Logger, s plugins.State) error {
	return plugins.RegisterResourceProvider(
		&p.PluginBase,
		l,
		s,
		"resource",
		"person",
		&person.Person{},
		&person.ExampleProvider{},
	)
}

// onePersonConfig declares one person, resource.person.ada
const onePersonConfig = `
resource "person" "ada" {
  first_name = "Ada"
  last_name  = "Lovelace"
  age        = 36
}
`

// threePeopleConfig declares three people
const threePeopleConfig = `
resource "person" "ada" {
  first_name = "Ada"
  last_name  = "Lovelace"
}

resource "person" "grace" {
  first_name = "Grace"
  last_name  = "Hopper"
}

resource "person" "alan" {
  first_name = "Alan"
  last_name  = "Turing"
}
`

// firstTeamConfig and secondTeamConfig declare people with distinct resource
// IDs, so each Config's plugin logs can be told apart
const firstTeamConfig = `
resource "person" "first_one" {
  first_name = "First"
  last_name  = "One"
}

resource "person" "first_two" {
  first_name = "First"
  last_name  = "Two"
}
`

const secondTeamConfig = `
resource "person" "second_one" {
  first_name = "Second"
  last_name  = "One"
}

resource "person" "second_two" {
  first_name = "Second"
  last_name  = "Two"
}
`

// personFixture is a Config wired to one kind of person plugin and a file
// state store, with the configuration it applies written to a file
type personFixture struct {
	config     *Config
	configFile string
	statePath  string
}

// stopPluginHosts stops every plugin process the registry started when the
// test ends
func stopPluginHosts(t *testing.T, pr *registry.PluginRegistry) {
	t.Helper()

	t.Cleanup(func() {
		for _, host := range pr.GetPluginHosts() {
			host.Stop()
		}
	})
}

// setupPersonConfig writes contents to main.xcl in dir, creates a file state
// store at dir/state.json and builds a Config using pr and opts
func setupPersonConfig(t *testing.T, pr *registry.PluginRegistry, dir, contents string, opts ...ConfigOption) *personFixture {
	t.Helper()

	configFile := filepath.Join(dir, "main.xcl")
	err := os.WriteFile(configFile, []byte(contents), 0644)
	require.NoError(t, err)

	return newPersonConfig(t, pr, configFile, filepath.Join(dir, "state.json"), opts...)
}

// newPersonConfig creates a file state store at statePath and builds a
// Config using pr and opts that applies configFile
func newPersonConfig(t *testing.T, pr *registry.PluginRegistry, configFile, statePath string, opts ...ConfigOption) *personFixture {
	t.Helper()

	store, err := state.NewFileStateStore(statePath, pr)
	require.NoError(t, err)

	options := append([]ConfigOption{
		WithPluginRegistry(pr),
		WithStateStore(store),
	}, opts...)

	return &personFixture{
		config:     NewConfig(options...),
		configFile: configFile,
		statePath:  statePath,
	}
}

// inProcessPersonRegistry returns a registry holding the in-process
// PersonPlugin
func inProcessPersonRegistry(t *testing.T) *registry.PluginRegistry {
	t.Helper()

	pr := registry.NewPluginRegistry()

	err := pr.RegisterPlugin(&PersonPlugin{})
	require.NoError(t, err)

	return pr
}

// externalPersonRegistry returns a registry holding the external person
// plugin binary, whose processes are stopped when the test ends
func externalPersonRegistry(t *testing.T, binary string) *registry.PluginRegistry {
	t.Helper()

	pr := registry.NewPluginRegistry()
	stopPluginHosts(t, pr)

	err := pr.RegisterPluginWithPath(binary)
	require.NoError(t, err)

	return pr
}

// pluginLogsWithMessage returns the log events the recorder received with
// the given message, from any Source
func pluginLogsWithMessage(recorder *eventRecorder, msg string) []Event {
	found := []Event{}
	for _, e := range logEvents(recorder) {
		if e.Meta[events.KeyMessage] == msg {
			found = append(found, e)
		}
	}

	return found
}

// pluginLogsFrom returns the log events the recorder received from source
func pluginLogsFrom(recorder *eventRecorder, source string) []Event {
	found := []Event{}
	for _, e := range logEvents(recorder) {
		if e.Source == source {
			found = append(found, e)
		}
	}

	return found
}

// resourceIDsOf returns the distinct resource IDs of found, sorted
func resourceIDsOf(found []Event) []string {
	seen := map[string]bool{}
	for _, e := range found {
		seen[e.ResourceID] = true
	}

	ids := []string{}
	for id := range seen {
		ids = append(ids, id)
	}

	sort.Strings(ids)
	return ids
}

// detailNames returns the names of a log event's details, leaving out its
// level and message, sorted
func detailNames(e Event) []string {
	names := []string{}
	for name := range e.Meta {
		if name == events.KeyLevel || name == events.KeyMessage {
			continue
		}

		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// detailText returns each of a log event's details as text, the form an
// external plugin's details cross the process boundary in
func detailText(e Event) map[string]string {
	text := map[string]string{}
	for _, name := range detailNames(e) {
		text[name] = fmt.Sprint(e.Meta[name])
	}

	return text
}

// captureAllOutput points os.Stdout, os.Stderr and the standard library's
// default logger, which go-plugin writes some messages to, at pipes until the
// returned function is called, which puts them back and returns everything
// written to the standard output and the standard error
func captureAllOutput(t *testing.T) func() (string, string) {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalLogOutput := log.Writer()

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	log.SetOutput(stderrWriter)

	restored := false
	restore := func() {
		if restored {
			return
		}

		restored = true
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		log.SetOutput(originalLogOutput)
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

// filesUnder returns the path, relative to dir, of every file and directory
// under dir, sorted
func filesUnder(t *testing.T, dir string) []string {
	t.Helper()

	found := []string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == dir {
			return nil
		}

		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		found = append(found, relative)
		return nil
	})
	require.NoError(t, err)

	sort.Strings(found)
	return found
}

func TestExternalPluginLogCarriesResourceAndStep(t *testing.T) {
	// built before HOME is isolated, so the build uses the real module cache
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	recorder := &eventRecorder{}
	f := setupPersonConfig(t, externalPersonRegistry(t, binary), t.TempDir(), onePersonConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	created := pluginLogsWithMessage(recorder, "Creating person")
	require.Len(t, created, 1)
	require.Equal(t, "resource.person.ada", created[0].ResourceID)
	require.Equal(t, "person.ada", created[0].ResourceType)
	require.Equal(t, f.configFile, created[0].File)
	require.Equal(t, events.OperationCreate, created[0].Operation)
	require.Equal(t, events.PhaseLog, created[0].Phase)

	// the second apply reads the person the first created
	err = f.config.Apply(f.configFile)
	require.NoError(t, err)

	read := pluginLogsWithMessage(recorder, "Reading person")
	require.Len(t, read, 1)
	require.Equal(t, "resource.person.ada", read[0].ResourceID)
	require.Equal(t, "person.ada", read[0].ResourceType)
	require.Equal(t, f.configFile, read[0].File)
	require.Equal(t, events.OperationRead, read[0].Operation)
}

func TestInProcessAndExternalPluginLogsMatchExceptSource(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	// both Configs apply the same file, so their logs name the same File,
	// each keeps its own state
	configFile := writeConfigFile(t, onePersonConfig)

	inProcessRecorder := &eventRecorder{}
	inProcess := newPersonConfig(t, inProcessPersonRegistry(t), configFile, filepath.Join(t.TempDir(), "state.json"), WithEventHandler(inProcessRecorder.handle))

	err := inProcess.config.Apply(inProcess.configFile)
	require.NoError(t, err)

	externalRecorder := &eventRecorder{}
	external := newPersonConfig(t, externalPersonRegistry(t, binary), configFile, filepath.Join(t.TempDir(), "state.json"), WithEventHandler(externalRecorder.handle))

	err = external.config.Apply(external.configFile)
	require.NoError(t, err)

	inProcessCreated := pluginLogsWithMessage(inProcessRecorder, "Creating person")
	require.Len(t, inProcessCreated, 1)

	externalCreated := pluginLogsWithMessage(externalRecorder, "Creating person")
	require.Len(t, externalCreated, 1)

	inProcessLog := inProcessCreated[0]
	externalLog := externalCreated[0]

	require.Equal(t, inProcessLog.Meta[events.KeyLevel], externalLog.Meta[events.KeyLevel])
	require.Equal(t, inProcessLog.Meta[events.KeyMessage], externalLog.Meta[events.KeyMessage])
	require.Equal(t, detailNames(inProcessLog), detailNames(externalLog))
	require.Equal(t, detailText(inProcessLog), detailText(externalLog))
	require.Equal(t, inProcessLog.ResourceID, externalLog.ResourceID)
	require.Equal(t, inProcessLog.ResourceType, externalLog.ResourceType)
	require.Equal(t, inProcessLog.Operation, externalLog.Operation)
	require.Equal(t, inProcessLog.File, externalLog.File)

	require.Equal(t, inProcessPersonPluginName, inProcessLog.Source)
	require.Equal(t, externalPersonPluginName, externalLog.Source)
}

func TestExternalPluginLogNamesPluginAsSource(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	recorder := &eventRecorder{}
	f := setupPersonConfig(t, externalPersonRegistry(t, binary), t.TempDir(), onePersonConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	created := pluginLogsWithMessage(recorder, "Creating person")
	require.Len(t, created, 1)
	require.Equal(t, filepath.Base(binary), created[0].Source)
	require.Equal(t, externalPersonPluginName, created[0].Source)
}

func TestEachConfigReceivesOnlyItsOwnPluginLogs(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	pr := externalPersonRegistry(t, binary)

	firstRecorder := &eventRecorder{}
	first := setupPersonConfig(t, pr, t.TempDir(), firstTeamConfig, WithEventHandler(firstRecorder.handle))

	secondRecorder := &eventRecorder{}
	second := setupPersonConfig(t, pr, t.TempDir(), secondTeamConfig, WithEventHandler(secondRecorder.handle))

	err := first.config.Apply(first.configFile)
	require.NoError(t, err)

	err = second.config.Apply(second.configFile)
	require.NoError(t, err)

	firstCreated := pluginLogsWithMessage(firstRecorder, "Creating person")
	require.Equal(t, []string{"resource.person.first_one", "resource.person.first_two"}, resourceIDsOf(firstCreated))

	secondCreated := pluginLogsWithMessage(secondRecorder, "Creating person")
	require.Equal(t, []string{"resource.person.second_one", "resource.person.second_two"}, resourceIDsOf(secondCreated))

	// every plugin log either receiver got names only that receiver's people,
	// or no resource at all for the plugin's own messages
	for _, e := range pluginLogsFrom(firstRecorder, externalPersonPluginName) {
		require.NotContains(t, e.ResourceID, "second", "the first receiver got a log for the second Config: %+v", e)
	}

	for _, e := range pluginLogsFrom(secondRecorder, externalPersonPluginName) {
		require.NotContains(t, e.ResourceID, "first", "the second receiver got a log for the first Config: %+v", e)
	}
}

func TestEachConfigReceivesOnlyItsOwnPluginLogsWhenApplyingConcurrently(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	pr := externalPersonRegistry(t, binary)

	firstRecorder := &eventRecorder{}
	first := setupPersonConfig(t, pr, t.TempDir(), firstTeamConfig, WithEventHandler(firstRecorder.handle))

	secondRecorder := &eventRecorder{}
	second := setupPersonConfig(t, pr, t.TempDir(), secondTeamConfig, WithEventHandler(secondRecorder.handle))

	var wg sync.WaitGroup
	var firstErr, secondErr error

	wg.Go(func() { firstErr = first.config.Apply(first.configFile) })
	wg.Go(func() { secondErr = second.config.Apply(second.configFile) })
	wg.Wait()

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)

	firstCreated := pluginLogsWithMessage(firstRecorder, "Creating person")
	require.Equal(t, []string{"resource.person.first_one", "resource.person.first_two"}, resourceIDsOf(firstCreated))

	secondCreated := pluginLogsWithMessage(secondRecorder, "Creating person")
	require.Equal(t, []string{"resource.person.second_one", "resource.person.second_two"}, resourceIDsOf(secondCreated))
}

func TestNoReceiverWritesNothingWithBothPluginKinds(t *testing.T) {
	binary := buildExamplePlugin(t)

	home := t.TempDir()
	t.Setenv("HOME", home)

	workDir := t.TempDir()
	t.Chdir(workDir)

	inProcessDir := filepath.Join(workDir, "in-process")
	externalDir := filepath.Join(workDir, "external")
	require.NoError(t, os.Mkdir(inProcessDir, 0755))
	require.NoError(t, os.Mkdir(externalDir, 0755))

	inProcess := setupPersonConfig(t, inProcessPersonRegistry(t), inProcessDir, threePeopleConfig)
	external := setupPersonConfig(t, externalPersonRegistry(t, binary), externalDir, threePeopleConfig)

	filesBefore := filesUnder(t, workDir)
	require.Equal(t, []string{
		"external",
		"external/main.xcl",
		"external/state.json",
		"in-process",
		"in-process/main.xcl",
		"in-process/state.json",
	}, filesBefore)

	finish := captureAllOutput(t)

	inProcessValidateErr := inProcess.config.Validate(inProcess.configFile)
	inProcessApplyErr := inProcess.config.Apply(inProcess.configFile)
	inProcessDestroyErr := inProcess.config.Destroy()

	externalValidateErr := external.config.Validate(external.configFile)
	externalApplyErr := external.config.Apply(external.configFile)
	externalDestroyErr := external.config.Destroy()

	stdout, stderr := finish()

	require.NoError(t, inProcessValidateErr)
	require.NoError(t, inProcessApplyErr)
	require.NoError(t, inProcessDestroyErr)
	require.NoError(t, externalValidateErr)
	require.NoError(t, externalApplyErr)
	require.NoError(t, externalDestroyErr)

	require.Empty(t, stdout)
	require.Empty(t, stderr)

	require.Equal(t, filesBefore, filesUnder(t, workDir))
	require.Empty(t, filesUnder(t, home))
}

func TestEveryExternalPluginLogReachesReceiver(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	recorder := &eventRecorder{}
	f := setupPersonConfig(t, externalPersonRegistry(t, binary), t.TempDir(), threePeopleConfig, WithEventHandler(recorder.handle))

	err := f.config.Apply(f.configFile)
	require.NoError(t, err)

	created := pluginLogsWithMessage(recorder, "Creating person")
	require.Len(t, created, 3)
	require.Equal(t, []string{"resource.person.ada", "resource.person.alan", "resource.person.grace"}, resourceIDsOf(created))

	err = f.config.Destroy()
	require.NoError(t, err)

	destroyed := pluginLogsWithMessage(recorder, "Destroying person")
	require.Len(t, destroyed, 3)
	require.Equal(t, []string{"resource.person.ada", "resource.person.alan", "resource.person.grace"}, resourceIDsOf(destroyed))

	for _, e := range destroyed {
		require.Equal(t, events.OperationDestroy, e.Operation)
		require.Equal(t, false, e.Meta["force"])
	}

	// the provider logs nothing else during an apply and a destroy, the
	// plugin's other logs are go-plugin's, which concern no resource
	providerLogs := []Event{}
	for _, e := range pluginLogsFrom(recorder, externalPersonPluginName) {
		if e.ResourceID != "" {
			providerLogs = append(providerLogs, e)
		}
	}
	require.Len(t, providerLogs, 6)
}

func TestReceiverSeesNothingOutsideItWithReceiverSet(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	inProcessRecorder := &eventRecorder{}
	inProcess := setupPersonConfig(t, inProcessPersonRegistry(t), t.TempDir(), threePeopleConfig, WithEventHandler(inProcessRecorder.handle))

	externalRecorder := &eventRecorder{}
	external := setupPersonConfig(t, externalPersonRegistry(t, binary), t.TempDir(), threePeopleConfig, WithEventHandler(externalRecorder.handle))

	finish := captureAllOutput(t)

	inProcessValidateErr := inProcess.config.Validate(inProcess.configFile)
	inProcessApplyErr := inProcess.config.Apply(inProcess.configFile)
	inProcessDestroyErr := inProcess.config.Destroy()

	externalValidateErr := external.config.Validate(external.configFile)
	externalApplyErr := external.config.Apply(external.configFile)
	externalDestroyErr := external.config.Destroy()

	stdout, stderr := finish()

	require.NoError(t, inProcessValidateErr)
	require.NoError(t, inProcessApplyErr)
	require.NoError(t, inProcessDestroyErr)
	require.NoError(t, externalValidateErr)
	require.NoError(t, externalApplyErr)
	require.NoError(t, externalDestroyErr)

	require.Empty(t, stdout)
	require.Empty(t, stderr)

	// the receivers got the plugins' logs instead
	require.Len(t, pluginLogsWithMessage(inProcessRecorder, "Creating person"), 3)
	require.Len(t, pluginLogsWithMessage(externalRecorder, "Creating person"), 3)
}

// pluginShutdownSettleTime is how long the stray output tests wait after an
// external plugin is stopped, go-plugin reports the end of its connection
// from its own goroutines once the process has gone
const pluginShutdownSettleTime = 250 * time.Millisecond

// TestStoppingUnusedExternalPluginWritesNothing asserts stopping an external
// plugin that was loaded but never called writes nothing to the standard
// output or error
func TestStoppingUnusedExternalPluginWritesNothing(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	pr := registry.NewPluginRegistry()

	err := pr.RegisterPluginWithPath(binary)
	require.NoError(t, err)

	c := NewConfig(WithPluginRegistry(pr))

	finish := captureAllOutput(t)

	// the configuration uses no person, so no provider of the plugin is called
	validateErr := c.Validate(writeConfigFile(t, variableOnlyConfig))

	for _, host := range pr.GetPluginHosts() {
		host.Stop()
	}

	time.Sleep(pluginShutdownSettleTime)

	stdout, stderr := finish()

	require.NoError(t, validateErr)
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

// TestExternalPluginRejectedForTypeClashWritesNothing asserts an external
// plugin that is stopped while loading, because it provides a type name that
// is already registered, writes nothing to the standard output or error
func TestExternalPluginRejectedForTypeClashWritesNothing(t *testing.T) {
	binary := buildExamplePlugin(t)
	t.Setenv("HOME", t.TempDir())

	pr := registry.NewPluginRegistry()

	err := pr.RegisterType("person", &registered.Database{})
	require.NoError(t, err)

	err = pr.RegisterPluginWithPath(binary)
	require.NoError(t, err)

	c := NewConfig(WithPluginRegistry(pr))

	finish := captureAllOutput(t)

	validateErr := c.Validate(writeConfigFile(t, variableOnlyConfig))

	time.Sleep(pluginShutdownSettleTime)

	stdout, stderr := finish()

	var clash *registry.TypeNameClashError
	require.True(t, errors.As(validateErr, &clash))
	require.Empty(t, stdout)
	require.Empty(t, stderr)
}

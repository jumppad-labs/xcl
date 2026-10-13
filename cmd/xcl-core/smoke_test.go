package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// The smoke tests build the program exactly as the Makefile does, with cgo
// disabled and the version stamped, start it, and talk to it over its real
// standard input and output. Behaviour is tested against serve and the
// coreservice package; these prove the program wires its stdio, signals and
// exit statuses as a binding expects.

// smokeVersion is the version the smoke tests stamp into the program
const smokeVersion = "v0.0.0-smoke"

// widgetConfig declares one widget, the block type the fixture plugin provides
const widgetConfig = "widget \"app\" {\n  size = 1\n  part {\n    name = \"a\"\n  }\n}\n"

// builtEngine is the program built once per test binary
var builtEngine struct {
	once   sync.Once
	dir    string
	path   string
	output string
	err    error
}

func TestMain(m *testing.M) {
	code := m.Run()

	if builtEngine.dir != "" {
		os.RemoveAll(builtEngine.dir)
	}

	os.Exit(code)
}

// buildEngine builds the program from this package with cgo disabled and
// smokeVersion stamped, once per test binary, and returns its path
func buildEngine(t *testing.T) string {
	t.Helper()

	builtEngine.once.Do(func() {
		dir, err := os.MkdirTemp("", "xcl-core-smoke-*")
		if err != nil {
			builtEngine.err = err
			return
		}

		builtEngine.dir = dir

		name := "xcl-core"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}

		path := filepath.Join(dir, name)

		build := exec.Command("go", "build", "-ldflags", "-X github.com/jumppad-labs/xcl/coreservice.Version="+smokeVersion, "-o", path, ".")
		build.Env = append(os.Environ(), "CGO_ENABLED=0")

		output, err := build.CombinedOutput()
		builtEngine.path, builtEngine.output, builtEngine.err = path, string(output), err
	})

	require.NoError(t, builtEngine.err, "unable to build xcl-core: %s", builtEngine.output)

	return builtEngine.path
}

// lockedBuffer is a bytes.Buffer safe to write from the goroutine that
// copies the program's standard error while a test reads it
type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buffer.String()
}

// engine is one running xcl-core --serve, driven over its standard input and
// output the way a binding drives it
type engine struct {
	t       *testing.T
	command *exec.Cmd
	stdin   *os.File
	stderr  *lockedBuffer

	// responses receives every frame read from standard output, and
	// readErr the error that ended the reading, io.EOF for a clean end
	// between frames
	responses chan *corepb.Response
	readErr   chan error

	exited  chan struct{}
	waitErr error

	nextID uint64
}

// startEngine builds the program and starts it with --serve, its home
// directory under a temporary directory. The test ends by killing the
// program if it is still running.
func startEngine(t *testing.T) *engine {
	t.Helper()

	binary := buildEngine(t)
	home := t.TempDir()

	stdinReader, stdinWriter, err := os.Pipe()
	require.NoError(t, err)

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	e := &engine{
		t:         t,
		stdin:     stdinWriter,
		stderr:    &lockedBuffer{},
		responses: make(chan *corepb.Response, 64),
		readErr:   make(chan error, 1),
		exited:    make(chan struct{}),
	}

	e.command = exec.Command(binary, "--serve")
	e.command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home)
	e.command.Stdin = stdinReader
	e.command.Stdout = stdoutWriter
	e.command.Stderr = e.stderr
	newProcessGroup(e.command)

	require.NoError(t, e.command.Start())

	// the program holds its own ends of the pipes now
	stdinReader.Close()
	stdoutWriter.Close()

	go func() {
		defer close(e.exited)
		e.waitErr = e.command.Wait()
	}()

	go func() {
		defer stdoutReader.Close()

		for {
			response := &corepb.Response{}
			if err := readFrame(stdoutReader, response); err != nil {
				e.readErr <- err
				return
			}

			e.responses <- response
		}
	}()

	t.Cleanup(func() {
		e.stdin.Close()

		select {
		case <-e.exited:
		case <-time.After(responseTimeout):
			e.command.Process.Kill()
			<-e.exited
			t.Errorf("xcl-core did not exit after the test; stderr:\n%s", e.stderr.String())
		}
	})

	return e
}

// send writes a Request frame carrying payload, or no payload when it is
// nil, and returns its id
func (e *engine) send(handle uint64, method string, payload proto.Message) uint64 {
	e.t.Helper()

	var body []byte
	if payload != nil {
		var err error
		body, err = proto.Marshal(payload)
		require.NoError(e.t, err)
	}

	e.nextID++
	id := e.nextID

	require.NoError(e.t, writeFrame(e.stdin, &corepb.Request{Id: id, Handle: handle, Method: method, Payload: body}))

	return id
}

// next returns the next response frame the program writes, failing the test
// when standard output ends, holds anything but a frame, or stays silent
func (e *engine) next() *corepb.Response {
	e.t.Helper()

	select {
	case response := <-e.responses:
		return response
	case err := <-e.readErr:
		e.t.Fatalf("standard output ended before a response arrived: %v; stderr:\n%s", err, e.stderr.String())
	case <-time.After(responseTimeout):
		e.t.Fatalf("no response arrived in time; stderr:\n%s", e.stderr.String())
	}

	return nil
}

// ok sends one request, requires its response to carry the same id and
// status 0, and decodes its payload into out
func (e *engine) ok(handle uint64, method string, payload, out proto.Message) {
	e.t.Helper()

	id := e.send(handle, method, payload)

	response := e.next()
	require.Equal(e.t, id, response.GetId())
	require.Equal(e.t, uint32(0), response.GetStatus(), "%s returned status %d; stderr:\n%s", method, int32(response.GetStatus()), e.stderr.String())

	if out != nil {
		require.NoError(e.t, proto.Unmarshal(response.GetPayload(), out))
	}
}

// getVersion is GetVersion, requiring the stamped version in the answer
func (e *engine) getVersion() {
	e.t.Helper()

	version := &corepb.GetVersionResponse{}
	e.ok(0, "GetVersion", &corepb.GetVersionRequest{}, version)

	require.Equal(e.t, smokeVersion, version.GetVersion())
	require.Equal(e.t, uint32(1), version.GetAbiVersion())
}

// createConfig is CreateConfig, returning the new handle
func (e *engine) createConfig(request *corepb.CreateConfigRequest) uint64 {
	e.t.Helper()

	created := &corepb.CreateConfigResponse{}
	e.ok(0, "CreateConfig", request, created)
	require.NotZero(e.t, created.GetHandle())

	return created.GetHandle()
}

// runToEnd polls the configuration behind handle until the operation opID
// finishes and returns its finished task
func (e *engine) runToEnd(handle, opID uint64) *corepb.OperationFinished {
	e.t.Helper()

	deadline := time.Now().Add(responseTimeout)

	for time.Now().Before(deadline) {
		polled := &corepb.PollResponse{}
		e.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 200}, polled)

		finished := polled.GetFinished()
		if finished == nil {
			continue
		}

		require.Equal(e.t, opID, finished.GetOperationId())

		return finished
	}

	e.t.Fatalf("operation %d did not finish in time; stderr:\n%s", opID, e.stderr.String())

	return nil
}

// waitExit waits for the program to exit and returns its exit code
func (e *engine) waitExit() int {
	e.t.Helper()

	select {
	case <-e.exited:
	case <-time.After(responseTimeout):
		e.t.Fatalf("xcl-core did not exit in time; stderr:\n%s", e.stderr.String())
	}

	if e.waitErr == nil {
		return 0
	}

	var exitErr *exec.ExitError
	require.ErrorAs(e.t, e.waitErr, &exitErr, "stderr:\n%s", e.stderr.String())

	return exitErr.ExitCode()
}

// requireCleanEnd waits for standard output to end and requires that it
// ended between frames, with nothing but frames before it
func (e *engine) requireCleanEnd() {
	e.t.Helper()

	select {
	case response := <-e.responses:
		e.t.Fatalf("unexpected response %d after the input ended", response.GetId())
	case err := <-e.readErr:
		require.ErrorIs(e.t, err, io.EOF, "standard output held something other than frames; stderr:\n%s", e.stderr.String())
	case <-time.After(responseTimeout):
		e.t.Fatalf("standard output did not end in time; stderr:\n%s", e.stderr.String())
	}
}

func TestVersionFlagPrintsStampedVersion(t *testing.T) {
	binary := buildEngine(t)

	run := testutil.RunProgram(t, binary, "--version")
	require.NoError(t, run.Err, run.Stderr)
	require.Equal(t, smokeVersion+"\n", run.Stdout)
}

func TestNoArgumentsPrintsUsageAndExitsTwo(t *testing.T) {
	binary := buildEngine(t)

	run := testutil.RunProgram(t, binary)

	var exitErr *exec.ExitError
	require.True(t, errors.As(run.Err, &exitErr), "expected an exit error, got %v", run.Err)
	require.Equal(t, 2, exitErr.ExitCode())
	require.Contains(t, run.Stderr, "usage: xcl-core")
	require.Empty(t, run.Stdout)
}

func TestServeOverPipesValidatesAConfiguration(t *testing.T) {
	stateDir := t.TempDir()
	file := writeAppConfig(t)

	e := startEngine(t)
	e.getVersion()

	handle := e.createConfig(appRequest(stateDir))

	started := &corepb.StartOperationResponse{}
	e.ok(handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}}, started)
	require.NotZero(t, started.GetOperationId())

	finished := e.runToEnd(handle, started.GetOperationId())
	require.Nil(t, finished.GetError())

	e.ok(handle, "Release", nil, nil)
}

func TestClosingStdinExitsZero(t *testing.T) {
	e := startEngine(t)
	e.getVersion()

	require.NoError(t, e.stdin.Close())

	e.requireCleanEnd()
	require.Equal(t, 0, e.waitExit())
}

func TestInterruptDoesNotStopTheEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT is a POSIX signal; Windows is covered by TestCtrlBreakDoesNotStopTheEngineOnWindows")
	}

	e := startEngine(t)

	// a first answer proves the program is serving, so its handlers are in
	// place before the signal arrives
	e.getVersion()

	require.NoError(t, e.command.Process.Signal(os.Interrupt))

	e.getVersion()

	select {
	case <-e.exited:
		t.Fatalf("xcl-core exited after an interrupt; stderr:\n%s", e.stderr.String())
	default:
	}
}

func TestCtrlBreakDoesNotStopTheEngineOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("console control events exist only on Windows")
	}

	e := startEngine(t)

	// a first answer proves the program is serving, so its handlers are in
	// place before the event arrives
	e.getVersion()

	sendCtrlBreak(t, e.command.Process.Pid)

	e.getVersion()

	select {
	case <-e.exited:
		t.Fatalf("xcl-core exited after Ctrl-Break; stderr:\n%s", e.stderr.String())
	default:
	}
}

func TestTerminateStopsTheEngine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM cannot be sent to a process on Windows")
	}

	e := startEngine(t)

	// a first answer proves the program is serving, so its handlers are in
	// place before the signal arrives
	e.getVersion()

	require.NoError(t, e.command.Process.Signal(syscall.SIGTERM))

	require.Equal(t, 0, e.waitExit())
}

func TestExternalPluginDoesNotReadFrames(t *testing.T) {
	plugin := testutil.BuildFixturePlugin(t, "../..")
	stateDir := t.TempDir()

	file := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(file, []byte(widgetConfig), 0o644))

	e := startEngine(t)

	handle := e.createConfig(&corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{Kind: &corepb.Registry_Local{Local: &corepb.LocalRegistry{
			ExternalPlugins: []string{plugin},
		}}}},
		StateStore: &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: stateDir}},
	})

	started := &corepb.StartOperationResponse{}
	e.ok(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{file}}, started)

	finished := e.runToEnd(handle, started.GetOperationId())
	require.Nil(t, finished.GetError(), "the apply failed: %v", finished.GetError())

	// the plugin ran while frames were flowing, and the program still reads
	// and answers them
	e.getVersion()

	found := &corepb.EntityResponse{}
	e.ok(handle, "Find", &corepb.FindRequest{Address: "widget.app"}, found)
	require.Contains(t, string(found.GetEntity()), `"part_names":"a"`)

	// everything the program wrote to standard output was frames, ending
	// cleanly once its input ended
	require.NoError(t, e.stdin.Close())

	e.requireCleanEnd()
	require.Equal(t, 0, e.waitExit())
}

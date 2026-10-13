package coreservice

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// driveTimeout bounds how long drive waits for an operation to finish
const driveTimeout = 10 * time.Second

// appSchema is a schema v1 document declaring the config-only block type app,
// with a required image and an optional sensitive password
const appSchema = `{"schema_version":1,"type":"app","fields":[{"name":"image","type":"string"},{"name":"password","type":"string","optional":true,"sensitive":true}]}`

// appConfig declares one app, a variable the request overrides and an output
// that publishes the variable's value
const appConfig = "variable \"region\" {\n  default = \"eu\"\n}\n\napp \"web\" {\n  image    = \"nginx\"\n  password = \"s3cret\"\n}\n\noutput \"where\" {\n  value = variable.region\n}\n"

// widgetConfig declares one widget, the block type the fixture plugin provides
const widgetConfig = "widget \"app\" {\n  size = 1\n  part {\n    name = \"a\"\n  }\n}\n"

// testClient drives a Service the way a binding does: each call marshals a
// request, passes the bytes through Call and unmarshals what comes back.
type testClient struct {
	t *testing.T
	s *Service

	// functions answers every function task drive polls, when set. A nil
	// result leaves the task outstanding, which is how a test holds a call
	// open.
	functions func(*corepb.FunctionTask) *corepb.FunctionResult
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()

	return &testClient{t: t, s: New()}
}

// call marshals req and dispatches method on handle, returning the raw
// response bytes and the status
func (c *testClient) call(handle uint64, method string, req proto.Message) ([]byte, Status) {
	c.t.Helper()

	body, err := proto.Marshal(req)
	require.NoError(c.t, err)

	return c.s.Call(handle, method, body)
}

// ok dispatches method and requires StatusOK, unmarshalling the response into
// resp
func (c *testClient) ok(handle uint64, method string, req, resp proto.Message) {
	c.t.Helper()

	body, status := c.call(handle, method, req)
	require.Equal(c.t, StatusOK, status, "%s returned status %d: %s", method, status, errorMessage(body))
	require.NoError(c.t, proto.Unmarshal(body, resp))
}

// fail dispatches method and requires StatusError, returning the Error it
// carries
func (c *testClient) fail(handle uint64, method string, req proto.Message) *corepb.Error {
	c.t.Helper()

	body, status := c.call(handle, method, req)
	require.Equal(c.t, StatusError, status, "%s returned status %d", method, status)

	out := &corepb.Error{}
	require.NoError(c.t, proto.Unmarshal(body, out))

	return out
}

// createConfig is CreateConfig, returning the new handle
func (c *testClient) createConfig(req *corepb.CreateConfigRequest) uint64 {
	c.t.Helper()

	resp := &corepb.CreateConfigResponse{}
	c.ok(0, "CreateConfig", req, resp)
	require.NotZero(c.t, resp.GetHandle())

	return resp.GetHandle()
}

// apply applies the configuration file to the configuration behind handle
// through the in-package hook, since the operation methods are not yet
// available
func (c *testClient) apply(handle uint64, file string) {
	c.t.Helper()

	conf := c.s.lookup(handle)
	require.NotNil(c.t, conf)
	require.NoError(c.t, conf.cfg.Apply(file))
}

// find is Find, returning the entity's JSON
func (c *testClient) find(handle uint64, address string) []byte {
	c.t.Helper()

	resp := &corepb.EntityResponse{}
	c.ok(handle, "Find", &corepb.FindRequest{Address: address}, resp)

	return resp.GetEntity()
}

// start dispatches one of the Start methods and returns the id of the
// operation it started
func (c *testClient) start(handle uint64, method string, req proto.Message) uint64 {
	c.t.Helper()

	resp := &corepb.StartOperationResponse{}
	c.ok(handle, method, req, resp)
	require.NotZero(c.t, resp.GetOperationId())

	return resp.GetOperationId()
}

// drive polls the configuration behind handle the way a host does until the
// operation opID finishes, and returns its finished task. Every event is
// passed to onEvent when it is set, every state task is answered by store,
// which must be set when one arrives. The test fails when the operation does
// not finish within driveTimeout.
func (c *testClient) drive(handle, opID uint64, store *fakeStore, onEvent func(*corepb.Event)) *corepb.OperationFinished {
	c.t.Helper()

	deadline := time.Now().Add(driveTimeout)

	for time.Now().Before(deadline) {
		resp := &corepb.PollResponse{}
		c.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 200}, resp)

		switch task := resp.GetTask().(type) {
		case nil:
			continue

		case *corepb.PollResponse_Event:
			if onEvent != nil {
				onEvent(task.Event)
			}

		case *corepb.PollResponse_State:
			require.NotNil(c.t, store, "a %s state task arrived but the test has no host store", task.State.GetKind())
			require.Equal(c.t, opID, task.State.GetOperationId())

			c.ok(handle, "Complete", &corepb.CompleteRequest{
				TaskId: task.State.GetTaskId(),
				Result: &corepb.CompleteRequest_State{State: store.answer(task.State)},
			}, &corepb.CompleteResponse{})

		case *corepb.PollResponse_Function:
			require.NotNil(c.t, c.functions, "function task %s arrived but the test has no function handler", task.Function.GetName())
			require.Equal(c.t, opID, task.Function.GetOperationId())

			result := c.functions(task.Function)
			if result == nil {
				continue
			}

			c.ok(handle, "Complete", &corepb.CompleteRequest{
				TaskId: task.Function.GetTaskId(),
				Result: &corepb.CompleteRequest_Function{Function: result},
			}, &corepb.CompleteResponse{})

		case *corepb.PollResponse_Finished:
			require.Equal(c.t, opID, task.Finished.GetOperationId())

			return task.Finished

		default:
			c.t.Fatalf("unexpected task %T", task)
		}
	}

	c.t.Fatalf("operation %d did not finish within %s", opID, driveTimeout)

	return nil
}

// fakeStore is the state store a host implements: it keeps the records the
// engine last saved and records the kind of every task it answered
type fakeStore struct {
	saved [][]byte
	kinds []corepb.StateTask_Kind

	// failWith, when set, is the message every task is answered with as an
	// error instead of a result
	failWith string
}

// answer is the host's result for one state task
func (f *fakeStore) answer(task *corepb.StateTask) *corepb.StateResult {
	f.kinds = append(f.kinds, task.GetKind())

	if f.failWith != "" {
		return &corepb.StateResult{Error: &corepb.Error{Code: CodeInternal, Message: f.failWith}}
	}

	switch task.GetKind() {
	case corepb.StateTask_EXISTS:
		return &corepb.StateResult{Exists: len(f.saved) > 0}
	case corepb.StateTask_LOAD:
		return &corepb.StateResult{Entities: f.saved}
	case corepb.StateTask_SAVE:
		f.saved = task.GetEntities()
	case corepb.StateTask_CLEAR:
		f.saved = nil
	}

	return &corepb.StateResult{}
}

// count returns how many tasks of kind the store answered
func (f *fakeStore) count(kind corepb.StateTask_Kind) int {
	n := 0
	for _, k := range f.kinds {
		if k == kind {
			n++
		}
	}

	return n
}

// errorMessage is the message of the Error in body, for a failure message
func errorMessage(body []byte) string {
	out := &corepb.Error{}
	if err := proto.Unmarshal(body, out); err != nil {
		return ""
	}

	return out.GetCode() + ": " + out.GetMessage()
}

// writeConfig writes contents to main.xcl in a fresh temporary directory and
// returns the file's path
func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(file, []byte(contents), 0o644))

	return file
}

// appRequest is a CreateConfigRequest declaring the app type from its schema,
// keeping state under a temporary directory and setting the region variable
func appRequest(t *testing.T) *corepb.CreateConfigRequest {
	t.Helper()

	return &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{Kind: &corepb.Registry_Local{Local: &corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Schema: []byte(appSchema)}},
		}}}},
		StateStore:    &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: t.TempDir()}},
		VariablesJson: []byte(`{"region":"us"}`),
	}
}

// appliedApp returns a client holding a configuration to which appConfig has
// been applied, and its handle
func appliedApp(t *testing.T) (*testClient, uint64) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))
	c.apply(handle, writeConfig(t, appConfig))

	return c, handle
}

// serviceMethods are the names of every method the CoreService descriptor
// declares
func serviceMethods(t *testing.T) []string {
	t.Helper()

	service := corepb.File_xcl_core_v1_core_proto.Services().ByName("CoreService")
	require.NotNil(t, service)

	methods := service.Methods()
	names := make([]string, 0, methods.Len())
	for i := 0; i < methods.Len(); i++ {
		names = append(names, string(methods.Get(i).Name()))
	}

	return names
}

func TestEveryServiceMethodHasAHandler(t *testing.T) {
	names := serviceMethods(t)
	require.NotEmpty(t, names)

	for _, name := range names {
		require.Contains(t, handlers, name, "method %s has no handler", name)
	}
}

func TestEveryHandlerIsAServiceMethod(t *testing.T) {
	names := serviceMethods(t)

	for name := range handlers {
		require.Contains(t, names, name, "handler %s is not a CoreService method", name)
	}
}

func TestUnknownMethodReturnsStatus(t *testing.T) {
	s := New()

	body, status := s.Call(0, "Nope", nil)
	require.Equal(t, StatusUnknownMethod, status)
	require.Nil(t, body)
}

func TestBadRequestBytesReturnsStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	// a lone field tag with no value does not parse as any message
	body, status := c.s.Call(handle, "Find", []byte{0x0a})
	require.Equal(t, StatusBadRequest, status)
	require.Nil(t, body)
}

func TestUnknownHandleReturnsStatus(t *testing.T) {
	c := newTestClient(t)

	body, status := c.call(42, "Find", &corepb.FindRequest{Address: "app.web"})
	require.Equal(t, StatusUnknownHandle, status)
	require.Nil(t, body)
}

func TestHandleZeroForConfigMethodReturnsStatus(t *testing.T) {
	c := newTestClient(t)

	body, status := c.call(0, "Find", &corepb.FindRequest{Address: "app.web"})
	require.Equal(t, StatusHandleMismatch, status)
	require.Nil(t, body)
}

func TestHandleGivenToGetVersionReturnsMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	body, status := c.call(handle, "GetVersion", &corepb.GetVersionRequest{})
	require.Equal(t, StatusHandleMismatch, status)
	require.Nil(t, body)
}

func TestReleasedHandleReturnsUnknownHandle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	require.Equal(t, StatusOK, c.s.Release(handle))

	_, status := c.call(handle, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, StatusUnknownHandle, status)
}

func TestReleaseUnknownHandleReturnsStatus(t *testing.T) {
	s := New()

	require.Equal(t, StatusUnknownHandle, s.Release(7))
}

func TestHandlesAreNeverReused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)

	first := c.createConfig(appRequest(t))
	require.Equal(t, StatusOK, c.s.Release(first))

	second := c.createConfig(appRequest(t))
	require.NotEqual(t, first, second)
}

func TestGetVersionReportsVersion(t *testing.T) {
	previous := Version
	Version = "v9.9.9"
	t.Cleanup(func() { Version = previous })

	c := newTestClient(t)

	resp := &corepb.GetVersionResponse{}
	c.ok(0, "GetVersion", &corepb.GetVersionRequest{}, resp)

	require.Equal(t, "v9.9.9", resp.GetVersion())
	require.Equal(t, uint32(1), resp.GetAbiVersion())
}

func TestCallIsSafeForConcurrentUse(t *testing.T) {
	c, handle := appliedApp(t)

	const workers = 8
	const rounds = 20

	// the requests are marshalled on the test goroutine, and statuses
	// collects every status the workers see so the assertions run here once
	// the workers are done
	findRequest, err := proto.Marshal(&corepb.FindRequest{Address: "app.web"})
	require.NoError(t, err)

	statuses := make(chan Status, workers*rounds*4)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < rounds; j++ {
				_, status := c.s.Call(0, "GetVersion", nil)
				statuses <- status

				_, status = c.s.Call(handle, "Find", findRequest)
				statuses <- status

				_, status = c.s.Call(handle, "Entities", nil)
				statuses <- status

				_, status = c.s.Call(handle, "Outputs", nil)
				statuses <- status
			}
		}()
	}

	wg.Wait()
	close(statuses)

	for status := range statuses {
		require.Equal(t, StatusOK, status)
	}
}

func TestCloseReleasesEveryConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	first := c.createConfig(appRequest(t))
	second := c.createConfig(appRequest(t))

	c.s.Close()

	_, status := c.call(first, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, StatusUnknownHandle, status)

	_, status = c.call(second, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, StatusUnknownHandle, status)
}

func TestCloseSavesStateOfRunningApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	conf := c.s.lookup(f.handle)
	require.NotNil(t, conf)

	op := conf.current()
	require.NotNil(t, op)

	// the held create returns once Close has cancelled the operation, as a
	// provider call already running finishes on its own
	go func() {
		<-op.ctx.Done()
		close(release)
	}()

	c.s.Close()

	// Close returned only once the operation had ended
	select {
	case <-op.done:
	default:
		t.Fatal("Close returned while the operation was still running")
	}

	// the create that was running when Close was called completed and the
	// apply saved it, and container.second, which depends on it, never began
	require.Equal(t, []string{"resource.network.first"}, savedIDs(t, f.stateDir))
}

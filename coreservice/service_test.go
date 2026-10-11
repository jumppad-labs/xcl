package coreservice

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

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

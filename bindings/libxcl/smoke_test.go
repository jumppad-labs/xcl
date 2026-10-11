package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/jumppad-labs/xcl/coreservice"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// The smoke test calls through the C ABI: the ABI version, the stamped
// engine version, creating a configuration, running one validate through the
// poll loop, and releasing the configuration. Behaviour is tested in the
// coreservice package; this proves the four exports move bytes correctly.

const smokeSchema = `{"schema_version":1,"type":"app","fields":[{"name":"image","type":"string"}]}`

func call(t *testing.T, handle uint64, method string, req proto.Message) (int32, []byte) {
	t.Helper()

	var request []byte
	if req != nil {
		var err error
		request, err = proto.Marshal(req)
		require.NoError(t, err)
	}

	return callThroughABI(handle, method, request)
}

func TestABIVersionIsOne(t *testing.T) {
	require.Equal(t, uint32(1), abiVersionThroughABI())
	require.Equal(t, coreservice.ABIVersion, abiVersionThroughABI())
}

func TestGetVersionReportsStampedVersion(t *testing.T) {
	status, body := call(t, 0, "GetVersion", &corepb.GetVersionRequest{})
	require.Equal(t, int32(coreservice.StatusOK), status)

	resp := &corepb.GetVersionResponse{}
	require.NoError(t, proto.Unmarshal(body, resp))

	// the Makefile stamps coreservice.Version through -ldflags; a plain go
	// test reports the development version
	require.Equal(t, coreservice.Version, resp.GetVersion())
	require.NotEmpty(t, resp.GetVersion())
	require.Equal(t, uint32(1), resp.GetAbiVersion())
}

func TestUnknownMethodReturnsNegativeStatusAndNoBody(t *testing.T) {
	status, body := call(t, 0, "Nope", nil)
	require.Equal(t, int32(coreservice.StatusUnknownMethod), status)
	require.Empty(t, body)
}

func TestCreateValidateAndReleaseThroughABI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	status, body := call(t, 0, "CreateConfig", &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{Kind: &corepb.Registry_Local{Local: &corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Schema: []byte(smokeSchema)}},
		}}}},
	})
	require.Equal(t, int32(coreservice.StatusOK), status)

	created := &corepb.CreateConfigResponse{}
	require.NoError(t, proto.Unmarshal(body, created))
	require.NotZero(t, created.GetHandle())

	file := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(file, []byte("app \"web\" {\n  image = \"nginx\"\n}\n"), 0644))

	status, body = call(t, created.GetHandle(), "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}})
	require.Equal(t, int32(coreservice.StatusOK), status)

	started := &corepb.StartOperationResponse{}
	require.NoError(t, proto.Unmarshal(body, started))

	var finished *corepb.OperationFinished
	deadline := time.Now().Add(10 * time.Second)
	for finished == nil {
		require.True(t, time.Now().Before(deadline), "the validate did not finish in time")

		status, body = call(t, created.GetHandle(), "Poll", &corepb.PollRequest{WaitMs: 200})
		require.Equal(t, int32(coreservice.StatusOK), status)

		polled := &corepb.PollResponse{}
		require.NoError(t, proto.Unmarshal(body, polled))
		finished = polled.GetFinished()
	}

	require.Equal(t, started.GetOperationId(), finished.GetOperationId())
	require.Nil(t, finished.GetError())

	require.Equal(t, int32(coreservice.StatusOK), releaseThroughABI(created.GetHandle()))

	status, body = call(t, created.GetHandle(), "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, int32(coreservice.StatusUnknownHandle), status)
	require.Empty(t, body)
}

func TestApplicationErrorCarriesErrorMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	status, body := call(t, 0, "CreateConfig", &corepb.CreateConfigRequest{})
	require.Equal(t, int32(coreservice.StatusOK), status)

	created := &corepb.CreateConfigResponse{}
	require.NoError(t, proto.Unmarshal(body, created))
	defer releaseThroughABI(created.GetHandle())

	status, body = call(t, created.GetHandle(), "Find", &corepb.FindRequest{Address: "app.nope"})
	require.Equal(t, int32(coreservice.StatusError), status)

	failure := &corepb.Error{}
	require.NoError(t, proto.Unmarshal(body, failure))
	require.Equal(t, coreservice.CodeNotFound, failure.GetCode())
}

func TestReleaseUnknownHandleReturnsStatus(t *testing.T) {
	require.Equal(t, int32(coreservice.StatusUnknownHandle), releaseThroughABI(12345))
}

func TestFreeNilIsIgnored(t *testing.T) {
	xcl_free(nil)
}

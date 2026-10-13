package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/jumppad-labs/xcl/coreservice"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// responseTimeout bounds how long a test waits for one response, or for
// serve or the engine program to return
const responseTimeout = 10 * time.Second

// appSchema is a schema v1 document declaring the config-only block type app
const appSchema = `{"schema_version":1,"type":"app","fields":[{"name":"image","type":"string"}]}`

// appConfig declares one app
const appConfig = "app \"web\" {\n  image = \"nginx\"\n}\n"

// writeFrame writes message as one frame: a 4-byte big-endian length
// followed by the serialised message
func writeFrame(w io.Writer, message proto.Message) error {
	body, err := proto.Marshal(message)
	if err != nil {
		return err
	}

	frame := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(frame, uint32(len(body)))
	copy(frame[4:], body)

	_, err = w.Write(frame)

	return err
}

// readFrame reads one frame from r into message. It returns io.EOF only when
// r ends cleanly before a frame starts; anything else that is not a whole
// frame holding a message is an error.
func readFrame(r io.Reader, message proto.Message) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size > maxFrameSize {
		return fmt.Errorf("frame of %d bytes is larger than the %d byte limit", size, maxFrameSize)
	}

	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return fmt.Errorf("truncated frame: %w", err)
	}

	return proto.Unmarshal(body, message)
}

// appRequest is a CreateConfigRequest declaring the app type from appSchema
// and keeping its state under stateDir
func appRequest(stateDir string) *corepb.CreateConfigRequest {
	return &corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{Kind: &corepb.Registry_Local{Local: &corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Schema: []byte(appSchema)}},
		}}}},
		StateStore: &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: stateDir}},
	}
}

// writeAppConfig writes appConfig to main.xcl in a fresh temporary directory
// and returns the file's path
func writeAppConfig(t *testing.T) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), "main.xcl")
	require.NoError(t, os.WriteFile(file, []byte(appConfig), 0o644))

	return file
}

// pipeHost is a host talking to serve over a pair of in-memory pipes, with a
// real Service behind it
type pipeHost struct {
	t       *testing.T
	service *coreservice.Service

	input     *io.PipeWriter
	responses chan *corepb.Response

	served     chan error
	finishOnce sync.Once
	serveErr   error

	nextID uint64
}

// startServe runs serve in a goroutine over a pair of pipes and reads every
// response frame it writes into a channel. Closing the input when the test
// ends returns serve.
func startServe(t *testing.T) *pipeHost {
	t.Helper()

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()

	host := &pipeHost{
		t:         t,
		service:   coreservice.New(),
		input:     inputWriter,
		responses: make(chan *corepb.Response, 64),
		served:    make(chan error, 1),
	}

	go func() {
		host.served <- serve(context.Background(), inputReader, outputWriter, host.service)
	}()

	go func() {
		defer close(host.responses)

		for {
			response := &corepb.Response{}
			if err := readFrame(outputReader, response); err != nil {
				return
			}

			host.responses <- response
		}
	}()

	t.Cleanup(func() {
		host.finish()
		outputReader.Close()
	})

	return host
}

// send writes a Request frame carrying payload, or no payload when it is
// nil, and returns its id
func (h *pipeHost) send(handle uint64, method string, payload proto.Message) uint64 {
	h.t.Helper()

	var body []byte
	if payload != nil {
		var err error
		body, err = proto.Marshal(payload)
		require.NoError(h.t, err)
	}

	h.nextID++
	id := h.nextID

	require.NoError(h.t, writeFrame(h.input, &corepb.Request{Id: id, Handle: handle, Method: method, Payload: body}))

	return id
}

// next returns the next response serve writes, failing the test when none
// arrives in time
func (h *pipeHost) next() *corepb.Response {
	h.t.Helper()

	select {
	case response, ok := <-h.responses:
		require.True(h.t, ok, "serve's output ended before a response arrived")
		return response
	case <-time.After(responseTimeout):
		h.t.Fatal("no response arrived in time")
	}

	return nil
}

// call sends one request, waits for its response and returns it
func (h *pipeHost) call(handle uint64, method string, payload proto.Message) *corepb.Response {
	h.t.Helper()

	id := h.send(handle, method, payload)

	response := h.next()
	require.Equal(h.t, id, response.GetId())

	return response
}

// ok sends one request, requires status 0 and decodes its payload into out
func (h *pipeHost) ok(handle uint64, method string, payload, out proto.Message) {
	h.t.Helper()

	response := h.call(handle, method, payload)
	require.Equal(h.t, uint32(0), response.GetStatus(), "%s returned status %d", method, int32(response.GetStatus()))
	require.NoError(h.t, proto.Unmarshal(response.GetPayload(), out))
}

// createConfig is CreateConfig, returning the new handle
func (h *pipeHost) createConfig(request *corepb.CreateConfigRequest) uint64 {
	h.t.Helper()

	created := &corepb.CreateConfigResponse{}
	h.ok(0, "CreateConfig", request, created)
	require.NotZero(h.t, created.GetHandle())

	return created.GetHandle()
}

// finish closes the input and returns what serve returned, failing the test
// when serve does not return in time
func (h *pipeHost) finish() error {
	h.t.Helper()

	h.finishOnce.Do(func() {
		h.input.Close()

		select {
		case h.serveErr = <-h.served:
		case <-time.After(responseTimeout):
			h.t.Error("serve did not return after its input ended")
		}
	})

	return h.serveErr
}

// writeRaw writes bytes to serve's input as they are, for frames that are
// not well formed
func (h *pipeHost) writeRaw(data []byte) {
	h.t.Helper()

	_, err := h.input.Write(data)
	require.NoError(h.t, err)
}

// waitServed waits for serve to return without closing its input
func (h *pipeHost) waitServed() error {
	h.t.Helper()

	select {
	case err := <-h.served:
		h.finishOnce.Do(func() { h.serveErr = err })
		return err
	case <-time.After(responseTimeout):
		h.t.Fatal("serve did not return in time")
	}

	return nil
}

func TestServeAnswersWithTheRequestID(t *testing.T) {
	host := startServe(t)

	host.nextID = 41
	id := host.send(0, "GetVersion", &corepb.GetVersionRequest{})
	require.Equal(t, uint64(42), id)

	response := host.next()
	require.Equal(t, uint64(42), response.GetId())
	require.Equal(t, uint32(0), response.GetStatus())

	version := &corepb.GetVersionResponse{}
	require.NoError(t, proto.Unmarshal(response.GetPayload(), version))
	require.Equal(t, uint32(1), version.GetAbiVersion())
	require.Equal(t, coreservice.Version, version.GetVersion())
}

func TestServeAnswersCancelWhilePollWaits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stateDir := t.TempDir()
	file := writeAppConfig(t)

	host := startServe(t)
	handle := host.createConfig(appRequest(stateDir))

	// run one validate to its end, so the configuration has an operation
	// Cancel can name and nothing left for Poll to return
	started := &corepb.StartOperationResponse{}
	host.ok(handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}}, started)

	deadline := time.Now().Add(responseTimeout)
	for {
		require.True(t, time.Now().Before(deadline), "the validate did not finish in time")

		polled := &corepb.PollResponse{}
		host.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 200}, polled)

		if polled.GetFinished() != nil {
			require.Nil(t, polled.GetFinished().GetError())
			break
		}
	}

	// the Poll waits for a task that never comes, so only Cancel can be
	// answered until the configuration is released
	pollID := host.send(handle, "Poll", &corepb.PollRequest{WaitMs: 60_000})
	cancelID := host.send(handle, "Cancel", &corepb.CancelRequest{OperationId: started.GetOperationId()})

	first := host.next()
	require.Equal(t, cancelID, first.GetId())
	require.Equal(t, uint32(0), first.GetStatus())

	select {
	case response := <-host.responses:
		t.Fatalf("response %d arrived while the Poll should still be waiting", response.GetId())
	case <-time.After(200 * time.Millisecond):
	}

	// ending the input releases the configuration, which ends the Poll
	require.NoError(t, host.finish())

	last := host.next()
	require.Equal(t, pollID, last.GetId())
}

func TestServeReleaseFreesHandle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stateDir := t.TempDir()

	host := startServe(t)
	handle := host.createConfig(appRequest(stateDir))

	response := host.call(handle, "Release", nil)
	require.Equal(t, uint32(0), response.GetStatus())
	require.Empty(t, response.GetPayload())

	_, status := host.service.Call(handle, "Entities", nil)
	require.Equal(t, coreservice.StatusUnknownHandle, status)
}

func TestServeCallOnReleasedHandleReturnsUnknownHandle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stateDir := t.TempDir()

	host := startServe(t)
	handle := host.createConfig(appRequest(stateDir))

	released := host.call(handle, "Release", nil)
	require.Equal(t, uint32(0), released.GetStatus())

	response := host.call(handle, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, int32(-3), int32(response.GetStatus()))
	require.Equal(t, int32(coreservice.StatusUnknownHandle), int32(response.GetStatus()))
	require.Empty(t, response.GetPayload())
}

func TestServeUnknownMethodReturnsNegativeStatus(t *testing.T) {
	host := startServe(t)

	response := host.call(0, "Nope", &corepb.GetVersionRequest{})
	require.Equal(t, int32(-1), int32(response.GetStatus()))
	require.Equal(t, int32(coreservice.StatusUnknownMethod), int32(response.GetStatus()))
	require.Empty(t, response.GetPayload())
}

func TestServeEndOfInputReleasesConfigurations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stateDir := t.TempDir()

	host := startServe(t)
	handle := host.createConfig(appRequest(stateDir))

	require.NoError(t, host.finish())

	_, status := host.service.Call(handle, "Entities", nil)
	require.Equal(t, coreservice.StatusUnknownHandle, status)
}

func TestServeTruncatedFrameReturnsError(t *testing.T) {
	host := startServe(t)

	// the header claims 100 bytes and only 10 follow before the input ends
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 100)

	host.writeRaw(header)
	host.writeRaw(make([]byte, 10))

	require.Error(t, host.finish())
}

func TestServeOversizedFrameReturnsError(t *testing.T) {
	host := startServe(t)

	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, maxFrameSize+1)

	host.writeRaw(header)

	// serve refuses the length without waiting for the bytes it claims
	require.Error(t, host.waitServed())
}

func TestServeUndecodableFrameReturnsError(t *testing.T) {
	host := startServe(t)

	// a lone length-delimited field tag with no length does not parse as a
	// Request
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, 1)

	host.writeRaw(header)
	host.writeRaw([]byte{0x0a})

	require.Error(t, host.waitServed())
}

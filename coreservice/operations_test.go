package coreservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/jumppad-labs/xcl/state"
)

// networkConfig declares one network of the in-process TestPlugin
const networkConfig = "resource \"network\" \"one\" {\n  subnet = \"10.0.1.0/24\"\n}\n"

// networkChangedConfig is networkConfig with the subnet changed
const networkChangedConfig = "resource \"network\" \"one\" {\n  subnet = \"10.0.2.0/24\"\n}\n"

// chainConfig declares a container that depends on a network, so the
// container's create can only start once the network's create has returned
const chainConfig = "resource \"network\" \"first\" {\n  subnet = \"10.0.0.0/16\"\n}\n\nresource \"container\" \"second\" {\n  network {\n    name = resource.network.first.meta.name\n  }\n}\n"

// bogusConfig declares a block of a type no registry knows
const bogusConfig = "bogus \"x\" {\n  image = \"nginx\"\n}\n"

// aesKey is a 32 byte key for an AES-256-GCM state mask
const aesKey = "0123456789abcdef0123456789abcdef"

// hostRequest is appRequest with the host implementing the state store and
// events delivered through Poll with processed data
func hostRequest(t *testing.T) *corepb.CreateConfigRequest {
	t.Helper()

	req := appRequest(t)
	req.StateStore = &corepb.StateStore{Kind: &corepb.StateStore_Host{Host: true}}
	req.Events = true
	req.EventData = corepb.EventDataLevel_EVENT_DATA_PROCESSED

	return req
}

// pluginFixture is a configuration built in-package around the TestPlugin,
// which the service cannot receive through a message, keeping its state in
// a file under stateDir
type pluginFixture struct {
	handle   uint64
	plugin   *parser.TestPlugin
	stateDir string
}

// newPluginConfig registers the TestPlugin on a local registry, builds a
// Config around it and adds it to the client's service
func newPluginConfig(t *testing.T, c *testClient) *pluginFixture {
	t.Helper()

	local := registry.NewLocal()
	plugin := &parser.TestPlugin{}
	local.RegisterPlugin(plugin)

	stateDir := t.TempDir()

	cfg, err := xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath(stateDir))
	require.NoError(t, err)

	conf := &configuration{cfg: cfg, queue: newTaskQueue()}

	return &pluginFixture{handle: c.s.add(conf), plugin: plugin, stateDir: stateDir}
}

// holdCreate makes the plugin hold the create of the resource id until
// release is closed, closing started once that create has begun
func holdCreate(plugin *parser.TestPlugin, id string) (started, release chan struct{}) {
	started = make(chan struct{})
	release = make(chan struct{})

	var once sync.Once
	plugin.SetCreateHook(func(got string) {
		if got != id {
			return
		}

		once.Do(func() { close(started) })
		<-release
	})

	return started, release
}

// waitStarted waits for the held create to begin, releasing it and failing
// the test when it never does
func waitStarted(t *testing.T, started, release chan struct{}) {
	t.Helper()

	select {
	case <-started:
	case <-time.After(driveTimeout):
		close(release)
		t.Fatal("the held create never started")
	}
}

// cancelDuringFirstCreate starts an apply of chainConfig, cancels it through
// the service while the create of network.first is held, releases the create
// and drives the operation to its end
func cancelDuringFirstCreate(t *testing.T) (*testClient, *pluginFixture, *corepb.OperationFinished) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	c.ok(f.handle, "Cancel", &corepb.CancelRequest{OperationId: opID}, &corepb.CancelResponse{})
	close(release)

	return c, f, c.drive(f.handle, opID, nil, nil)
}

// savedIDs returns the sorted ids of every record in the state file under
// dir
func savedIDs(t *testing.T, dir string) []string {
	t.Helper()

	store, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	records, err := store.Load()
	require.NoError(t, err)

	ids := []string{}
	for _, record := range records {
		raw, ok := record.(json.RawMessage)
		require.True(t, ok, "the file store loads raw records, got %T", record)

		ids = append(ids, recordID(t, raw))
	}

	sort.Strings(ids)

	return ids
}

// recordID is the meta.id of one JSON record
func recordID(t *testing.T, record []byte) string {
	t.Helper()

	var saved struct {
		Meta struct {
			ID string `json:"id"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(record, &saved))

	return saved.Meta.ID
}

// recordFor returns the record in records whose meta.id is id
func recordFor(t *testing.T, records [][]byte, id string) []byte {
	t.Helper()

	for _, record := range records {
		if recordID(t, record) == id {
			return record
		}
	}

	t.Fatalf("no record has id %s", id)

	return nil
}

// eventName is "<operation>/<phase>", the shape the observed order is
// written in
func eventName(e *corepb.Event) string {
	return e.GetOperation() + "/" + e.GetPhase()
}

// createSuccessOf drives an apply of appConfig with events and returns the
// create/success event of app.web
func createSuccessOf(t *testing.T) *corepb.Event {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	var created *corepb.Event
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	finished := c.drive(handle, opID, &fakeStore{}, func(e *corepb.Event) {
		if e.GetOperation() == "create" && e.GetPhase() == "success" && e.GetResourceId() == "app.web" {
			created = e
		}
	})

	require.Nil(t, finished.GetError())
	require.NotNil(t, created, "no create/success event for app.web was delivered")

	return created
}

// nextStateTask polls until a state task arrives, failing the test when the
// operation finishes or the deadline passes first
func nextStateTask(t *testing.T, c *testClient, handle uint64) *corepb.StateTask {
	t.Helper()

	deadline := time.Now().Add(driveTimeout)

	for time.Now().Before(deadline) {
		resp := &corepb.PollResponse{}
		c.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 200}, resp)

		switch task := resp.GetTask().(type) {
		case *corepb.PollResponse_State:
			return task.State
		case *corepb.PollResponse_Finished:
			t.Fatal("the operation finished before any state task arrived")
		}
	}

	t.Fatal("no state task arrived in time")

	return nil
}

// wireDiff is the shape a host decodes diff_json into, with each change's
// path in its written form
type wireDiff struct {
	Summary struct {
		Create    int `json:"create"`
		Update    int `json:"update"`
		Replace   int `json:"replace"`
		Delete    int `json:"delete"`
		Unchanged int `json:"unchanged"`
	} `json:"summary"`
	Resources []struct {
		Address string `json:"address"`
		Action  string `json:"action"`
		Changes []struct {
			Path   string `json:"path"`
			Before any    `json:"before"`
			After  any    `json:"after"`
		} `json:"changes"`
	} `json:"resources"`
}

// applyThenDiffNetwork applies networkConfig through the service and then
// starts a diff against networkChangedConfig, returning its finished task
func applyThenDiffNetwork(t *testing.T) *corepb.OperationFinished {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, networkConfig)}})
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())

	opID = c.start(f.handle, "StartDiff", &corepb.StartDiffRequest{Paths: []string{writeConfig(t, networkChangedConfig)}})

	return c.drive(f.handle, opID, nil, nil)
}

func TestApplyEndsWithFinishedTask(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	finished := c.drive(handle, opID, &fakeStore{}, nil)

	require.Nil(t, finished.GetError())
	require.Equal(t, opID, finished.GetOperationId())

	// nothing follows the finished task
	resp := &corepb.PollResponse{}
	c.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 50}, resp)
	require.Nil(t, resp.GetTask())
}

func TestApplyMatchesGoApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// through the service
	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, nil, nil).GetError())

	resp := &corepb.EntitiesResponse{}
	c.ok(handle, "Entities", &corepb.EntitiesRequest{}, resp)

	// directly through xcl.Config with the same type, state kind and variables
	declared, err := registry.TypeFromSchema([]byte(appSchema))
	require.NoError(t, err)

	local := registry.NewLocal()
	local.RegisterType(declared.Prototype, declared.Type)

	cfg, err := xcl.NewConfig(
		xcl.WithRegistry(local),
		xcl.WithStatePath(t.TempDir()),
		xcl.WithVariables(map[string]any{"region": "us"}),
	)
	require.NoError(t, err)
	require.NoError(t, cfg.Apply(writeConfig(t, appConfig)))

	direct := [][]byte{}
	for _, e := range cfg.Entities() {
		data, err := json.Marshal(e)
		require.NoError(t, err)

		direct = append(direct, data)
	}

	require.Equal(t, decodeEntities(t, direct), decodeEntities(t, resp.GetEntities()))
}

// decodeEntities decodes JSON records as maps, dropping meta.file which names
// the temporary file each configuration was read from, and orders them by id
func decodeEntities(t *testing.T, records [][]byte) []map[string]any {
	t.Helper()

	decoded := []map[string]any{}
	for _, record := range records {
		entity := map[string]any{}
		require.NoError(t, json.Unmarshal(record, &entity))

		if meta, ok := entity["meta"].(map[string]any); ok {
			delete(meta, "file")
		}

		decoded = append(decoded, entity)
	}

	sort.Slice(decoded, func(i, j int) bool {
		return fmt.Sprint(decoded[i]["meta"].(map[string]any)["id"]) < fmt.Sprint(decoded[j]["meta"].(map[string]any)["id"])
	})

	return decoded
}

func TestStartWithoutPathsIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	out := c.fail(handle, "StartApply", &corepb.StartApplyRequest{})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "paths", out.GetDetails()["argument"])

	out = c.fail(handle, "StartValidate", &corepb.StartValidateRequest{})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "paths", out.GetDetails()["argument"])

	out = c.fail(handle, "StartDiff", &corepb.StartDiffRequest{})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "paths", out.GetDetails()["argument"])
}

func TestOperationIDsIncrease(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))
	file := writeConfig(t, appConfig)

	first := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{file}})
	require.Equal(t, uint64(1), first)
	require.Nil(t, c.drive(handle, first, nil, nil).GetError())

	second := c.start(handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}})
	require.Equal(t, uint64(2), second)
	require.Nil(t, c.drive(handle, second, nil, nil).GetError())

	// ids are per configuration
	other := c.createConfig(appRequest(t))
	require.Equal(t, uint64(1), c.start(other, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}}))
}

func TestFinishedTaskQueuedAfterOperationEnds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)
	conf := c.s.lookup(f.handle)

	started, release := holdCreate(f.plugin, "resource.network.first")

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	// the operation cannot end while the configuration's lock is held, so a
	// host that polls its finished task meanwhile would find it still in
	// progress and have its next call refused
	conf.mu.Lock()
	close(release)

	finishedEarly := false
	for {
		resp := &corepb.PollResponse{}
		c.ok(f.handle, "Poll", &corepb.PollRequest{WaitMs: 500}, resp)

		if resp.GetTask() == nil {
			break
		}

		if resp.GetFinished() != nil {
			finishedEarly = true
			break
		}
	}

	conf.mu.Unlock()

	require.False(t, finishedEarly, "the finished task arrived while the operation was in progress")
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())
}

func TestEventsArriveBeforeFinishedInOrder(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	seen := []string{}
	finishedAfter := -1

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	finished := c.drive(handle, opID, &fakeStore{}, func(e *corepb.Event) {
		seen = append(seen, eventName(e))
	})
	finishedAfter = len(seen)

	require.Nil(t, finished.GetError())
	require.NotEmpty(t, seen)
	require.Equal(t, "apply/start", seen[0])
	require.Equal(t, "apply/success", seen[len(seen)-1])
	require.Contains(t, seen, "create/success")

	// every event had been delivered when the finished task arrived
	require.Equal(t, len(seen), finishedAfter)
}

func TestEventDataMaskedByDefault(t *testing.T) {
	created := createSuccessOf(t)

	require.NotEmpty(t, created.GetData())
	require.NotContains(t, string(created.GetData()), "s3cret")

	var entity struct {
		Image    string         `json:"image"`
		Password map[string]any `json:"password"`
	}
	require.NoError(t, json.Unmarshal(created.GetData(), &entity))

	require.Equal(t, "nginx", entity.Image)
	require.Equal(t, map[string]any{"xcl_masked": "redact", "value": "(sensitive)"}, entity.Password)
}

func TestEventFieldsCarried(t *testing.T) {
	created := createSuccessOf(t)

	require.Equal(t, "create", created.GetOperation())
	require.Equal(t, "success", created.GetPhase())
	require.Equal(t, "app.web", created.GetResourceId())

	// ResourceType is "<type>.<name>", see events.Event
	require.Equal(t, "app.web", created.GetResourceType())
	require.Equal(t, "core", created.GetSource())
	require.NotZero(t, created.GetTimeUnixNano())
	require.Nil(t, created.GetError())
	require.True(t, len(created.GetMetaJson()) > 0 || created.GetFile() != "", "neither meta_json nor file was carried")

	if len(created.GetMetaJson()) > 0 {
		meta := map[string]any{}
		require.NoError(t, json.Unmarshal(created.GetMetaJson(), &meta))
	}
}

func TestEventsOffDeliversOnlyFinished(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	finished := c.drive(handle, opID, nil, func(e *corepb.Event) {
		t.Fatalf("event %s delivered with events off", eventName(e))
	})

	require.Nil(t, finished.GetError())
}

func TestHostStateStoreReceivesSaveOnApply(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, store, nil).GetError())

	require.Equal(t, 1, store.count(corepb.StateTask_SAVE))

	// the variable, the app and the output are saved
	require.Len(t, store.saved, 3)

	record := recordFor(t, store.saved, "app.web")
	require.Contains(t, string(record), `"image":"nginx"`)
	require.Contains(t, string(record), `"s3cret"`)
}

func TestHostStateStoreServesLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	store := &fakeStore{}

	first := c.createConfig(hostRequest(t))
	opID := c.start(first, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(first, opID, store, nil).GetError())
	require.NotEmpty(t, store.saved)

	store.kinds = nil

	second := c.createConfig(hostRequest(t))
	opID = c.start(second, "StartLoad", &corepb.StartLoadRequest{})
	require.Nil(t, c.drive(second, opID, store, nil).GetError())

	require.Equal(t, 1, store.count(corepb.StateTask_LOAD))

	entity := c.find(second, "app.web")
	require.Contains(t, string(entity), `"image":"nginx"`)
}

func TestHostStateStoreWithMaskReceivesEncryptedValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	req := hostRequest(t)
	req.StateMask = &corepb.Mask{Kind: &corepb.Mask_Aes256GcmKey{Aes256GcmKey: []byte(aesKey)}}

	c := newTestClient(t)
	handle := c.createConfig(req)

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, store, nil).GetError())

	record := recordFor(t, store.saved, "app.web")
	require.Contains(t, string(record), `"image":"nginx"`)
	require.NotContains(t, string(record), "s3cret")
}

func TestHostStateStoreErrorFailsOperation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	store := &fakeStore{failWith: "the host's disk is full"}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	finished := c.drive(handle, opID, store, nil)

	require.NotNil(t, finished.GetError())
	require.Contains(t, finished.GetError().GetMessage(), "the host's disk is full")
}

func TestHostStateStoreClearOnDestroy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, store, nil).GetError())
	require.NotEmpty(t, store.saved)

	store.kinds = nil

	opID = c.start(handle, "StartDestroy", &corepb.StartDestroyRequest{})
	finished := c.drive(handle, opID, store, nil)

	require.Nil(t, finished.GetError())

	// the destroy loads what was saved and saves the remaining records after
	// each of the three is removed, the last save carrying none; it never
	// clears the store
	require.Equal(t, 1, store.count(corepb.StateTask_LOAD))
	require.Equal(t, 3, store.count(corepb.StateTask_SAVE))
	require.Equal(t, 0, store.count(corepb.StateTask_CLEAR))
	require.Empty(t, store.saved)
}

func TestDiffFinishedCarriesDiffJSON(t *testing.T) {
	finished := applyThenDiffNetwork(t)

	require.Nil(t, finished.GetError())
	require.NotEmpty(t, finished.GetDiffJson())

	found := wireDiff{}
	require.NoError(t, json.Unmarshal(finished.GetDiffJson(), &found))

	require.Equal(t, 1, found.Summary.Update)
	require.Len(t, found.Resources, 1)

	network := found.Resources[0]
	require.Equal(t, "resource.network.one", network.Address)
	require.Equal(t, "update", network.Action)
	require.Len(t, network.Changes, 1)
	require.Equal(t, "subnet", network.Changes[0].Path)
	require.Equal(t, "10.0.1.0/24", network.Changes[0].Before)
	require.Equal(t, "10.0.2.0/24", network.Changes[0].After)
}

func TestDiffMatchesGoDiff(t *testing.T) {
	finished := applyThenDiffNetwork(t)
	require.Nil(t, finished.GetError())

	// the same apply and diff directly through xcl.Config
	local := registry.NewLocal()
	local.RegisterPlugin(&parser.TestPlugin{})

	cfg, err := xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath(t.TempDir()))
	require.NoError(t, err)
	require.NoError(t, cfg.Apply(writeConfig(t, networkConfig)))

	direct, err := cfg.Diff([]string{writeConfig(t, networkChangedConfig)})
	require.NoError(t, err)

	want, err := json.Marshal(direct)
	require.NoError(t, err)

	require.JSONEq(t, string(want), string(finished.GetDiffJson()))
}

func TestCancelRunningApplyReturnsCancelledAndSaves(t *testing.T) {
	_, f, finished := cancelDuringFirstCreate(t)

	require.NotNil(t, finished.GetError())
	require.Equal(t, CodeCancelled, finished.GetError().GetCode())

	// the create that was in progress finished and was saved
	require.Equal(t, []string{"resource.network.first"}, savedIDs(t, f.stateDir))
}

func TestCancelStartsNoProviderCallAfterwards(t *testing.T) {
	_, f, finished := cancelDuringFirstCreate(t)
	require.NotNil(t, finished.GetError())

	// container.second depends on network.first, so its create could only
	// have started after the cancel, and it never did
	require.Equal(t, []string{"create resource.network.first"}, f.plugin.GetCalls())
}

func TestCancelUnknownOperationIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	out := c.fail(handle, "Cancel", &corepb.CancelRequest{OperationId: 0})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "0", out.GetDetails()["operation_id"])

	out = c.fail(handle, "Cancel", &corepb.CancelRequest{OperationId: 7})
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "7", out.GetDetails()["operation_id"])
}

func TestCancelFinishedOperationIsNoOp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))
	file := writeConfig(t, appConfig)

	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{file}})
	require.Nil(t, c.drive(handle, opID, nil, nil).GetError())

	c.ok(handle, "Cancel", &corepb.CancelRequest{OperationId: opID}, &corepb.CancelResponse{})

	// the configuration is untouched: a lookup and a further operation work
	require.Contains(t, string(c.find(handle, "app.web")), `"image":"nginx"`)

	next := c.start(handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}})
	require.Nil(t, c.drive(handle, next, nil, nil).GetError())
}

func TestSecondOperationRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	file := writeConfig(t, chainConfig)
	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{file}})
	waitStarted(t, started, release)

	out := c.fail(f.handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{file}})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	close(release)
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())
}

func TestLookupDuringOperationRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	out := c.fail(f.handle, "Find", &corepb.FindRequest{Address: "resource.network.first"})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	out = c.fail(f.handle, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	out = c.fail(f.handle, "Outputs", &corepb.OutputsRequest{})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	close(release)
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())
}

func TestEncodeEntityDuringOperationRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	record := []byte(`{"meta":{"id":"resource.network.first","name":"first","type":"resource","subtype":"network"},"subnet":"10.0.0.0/16"}`)

	out := c.fail(f.handle, "EncodeEntity", &corepb.EncodeEntityRequest{Entity: record})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	close(release)
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())
}

func TestEncodeSavedEntityDuringOperationRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	record := []byte(`{"meta":{"id":"resource.network.first","name":"first","type":"resource","subtype":"network"},"subnet":"10.0.0.0/16"}`)

	out := c.fail(f.handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: record})
	require.Equal(t, CodeOperationInProgress, out.GetCode())

	close(release)
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())
}

func TestLateCompleteIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	req := hostRequest(t)
	req.Events = false

	c := newTestClient(t)
	handle := c.createConfig(req)

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})

	task := nextStateTask(t, c, handle)
	answer := &corepb.CompleteRequest{
		TaskId: task.GetTaskId(),
		Result: &corepb.CompleteRequest_State{State: store.answer(task)},
	}

	c.ok(handle, "Complete", answer, &corepb.CompleteResponse{})

	out := c.fail(handle, "Complete", answer)
	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, fmt.Sprint(task.GetTaskId()), out.GetDetails()["task_id"])

	require.Nil(t, c.drive(handle, opID, store, nil).GetError())
}

func TestCompleteUnknownTaskIsInvalidArgument(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	out := c.fail(handle, "Complete", &corepb.CompleteRequest{
		TaskId: 99,
		Result: &corepb.CompleteRequest_State{State: &corepb.StateResult{}},
	})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "99", out.GetDetails()["task_id"])
}

func TestPollWithoutTasksReturnsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	started := time.Now()

	resp := &corepb.PollResponse{}
	c.ok(handle, "Poll", &corepb.PollRequest{WaitMs: 10}, resp)

	require.Nil(t, resp.GetTask())
	require.Less(t, time.Since(started), time.Second)
}

func TestReleaseCancelsAndWaits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	started, release := holdCreate(f.plugin, "resource.network.first")

	c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, chainConfig)}})
	waitStarted(t, started, release)

	// the held create returns once Release has cancelled the operation, as
	// a provider call already running finishes on its own
	conf := c.s.lookup(f.handle)
	require.NotNil(t, conf)

	op := conf.current()
	require.NotNil(t, op)

	go func() {
		<-op.ctx.Done()
		close(release)
	}()

	require.Equal(t, StatusOK, c.s.Release(f.handle))

	// Release returned only once the operation had ended
	select {
	case <-op.done:
	default:
		t.Fatal("Release returned while the operation was still running")
	}

	require.Equal(t, []string{"create resource.network.first"}, f.plugin.GetCalls())

	_, status := c.call(f.handle, "Entities", &corepb.EntitiesRequest{})
	require.Equal(t, StatusUnknownHandle, status)
}

func TestFailedProviderReportedInFinished(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	f.plugin.SetCreateError("resource.network.one", errors.New("boom"))

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, networkConfig)}})
	finished := c.drive(f.handle, opID, nil, nil)

	failure := finished.GetError()
	require.NotNil(t, failure)
	require.Equal(t, CodeConfig, failure.GetCode())
	require.Contains(t, failure.GetMessage(), "create failed for resource.network.one: boom")
	require.Len(t, failure.GetCauses(), 1)

	// the parser reports the failed create as a diagnostic at the block that
	// failed, so the cause is a provider error that names the step and the
	// address, with the provider's message as its detail
	cause := failure.GetCauses()[0]
	require.Equal(t, CodeProvider, cause.GetCode())
	require.Equal(t, "create", cause.GetDetails()["step"])
	require.Equal(t, "resource.network.one", cause.GetDetails()["address"])
	require.Equal(t, "resource.network.one", cause.GetDetails()["entity"])
	require.Equal(t, "1", cause.GetDetails()["line"])
	require.Equal(t, "1", cause.GetDetails()["column"])
	require.NotEmpty(t, cause.GetDetails()["file"])
	require.Equal(t, "Provider error", cause.GetDetails()["summary"])
	require.Contains(t, cause.GetDetails()["detail"], "create failed for resource.network.one: boom")
}

func TestValidateInvalidConfigFinishedHasConfigError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	opID := c.start(handle, "StartValidate", &corepb.StartValidateRequest{Paths: []string{writeConfig(t, bogusConfig)}})
	finished := c.drive(handle, opID, nil, nil)

	require.NotNil(t, finished.GetError())
	require.Equal(t, CodeConfig, finished.GetError().GetCode())
	require.Contains(t, finished.GetError().GetMessage(), "bogus")
}

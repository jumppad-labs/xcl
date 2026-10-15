package coreservice

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// entities is Entities, returning each entity's JSON
func (c *testClient) entities(handle uint64) [][]byte {
	c.t.Helper()

	resp := &corepb.EntitiesResponse{}
	c.ok(handle, "Entities", &corepb.EntitiesRequest{}, resp)

	return resp.GetEntities()
}

// appRecord is the saved record of app.web, taken from Entities
func (c *testClient) appRecord(handle uint64) []byte {
	c.t.Helper()

	for _, entity := range c.entities(handle) {
		if strings.Contains(string(entity), `"app.web"`) {
			return entity
		}
	}

	require.FailNow(c.t, "app.web is not among the entities")

	return nil
}

// isJSONObject reports whether data is one JSON object
func isJSONObject(data []byte) bool {
	var object map[string]any

	return json.Unmarshal(data, &object) == nil
}

func TestEntitiesReturnsEveryEntityAsJSON(t *testing.T) {
	c, handle := appliedApp(t)

	entities := c.entities(handle)

	// the variable, the app and the output
	require.Len(t, entities, 3)

	joined := ""
	for _, entity := range entities {
		require.True(t, isJSONObject(entity), "entity is not a JSON object: %s", entity)
		joined += string(entity)
	}

	require.Contains(t, joined, `"variable.region"`)
	require.Contains(t, joined, `"app.web"`)
	require.Contains(t, joined, `"output.where"`)
}

func TestFindReturnsEntityJSON(t *testing.T) {
	c, handle := appliedApp(t)

	entity := c.find(handle, "app.web")

	require.True(t, isJSONObject(entity))
	require.Contains(t, string(entity), `"image":"nginx"`)
	require.Contains(t, string(entity), `"app.web"`)
}

func TestFindUnknownAddressIsNotFound(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "Find", &corepb.FindRequest{Address: "app.nope"})

	require.Equal(t, CodeNotFound, out.GetCode())
	require.Equal(t, "app.nope", out.GetDetails()["address"])
}

func TestFindEmptyAddressIsInvalidArgument(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "Find", &corepb.FindRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "address", out.GetDetails()["argument"])
}

func TestFindByTypeReturnsEntities(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EntitiesResponse{}
	c.ok(handle, "FindByType", &corepb.FindByTypeRequest{Path: []string{"app"}}, resp)

	require.Len(t, resp.GetEntities(), 1)
	require.Contains(t, string(resp.GetEntities()[0]), `"app.web"`)
}

func TestFindByTypeUnknownTypeIsUnknownType(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "FindByType", &corepb.FindByTypeRequest{Path: []string{"nope"}})

	require.Equal(t, CodeUnknownType, out.GetCode())
	require.Equal(t, "nope", out.GetDetails()["name"])
}

func TestFindOneReturnsEntity(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EntityResponse{}
	c.ok(handle, "FindOne", &corepb.FindByTypeRequest{Path: []string{"app"}}, resp)

	require.Contains(t, string(resp.GetEntity()), `"app.web"`)
	require.Contains(t, string(resp.GetEntity()), `"image":"nginx"`)
}

func TestLookupsReturnSensitiveMarker(t *testing.T) {
	c, handle := appliedApp(t)

	entity := c.find(handle, "app.web")
	require.Contains(t, string(entity), `"password":"(sensitive)"`)
	require.NotContains(t, string(entity), "s3cret")

	for _, entity := range c.entities(handle) {
		require.NotContains(t, string(entity), "s3cret")
	}
}

func TestOutputsReturnJSONObject(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.OutputsResponse{}
	c.ok(handle, "Outputs", &corepb.OutputsRequest{}, resp)

	require.JSONEq(t, `{"output.where":"us"}`, string(resp.GetOutputsJson()))
}

func TestOutputsEmptyIsEmptyObject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	resp := &corepb.OutputsResponse{}
	c.ok(handle, "Outputs", &corepb.OutputsRequest{}, resp)

	require.JSONEq(t, `{}`, string(resp.GetOutputsJson()))
}

func TestEncodeSavedEntityRendersText(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: c.appRecord(handle)}, resp)

	require.Contains(t, string(resp.GetText()), `app "web"`)
	require.Contains(t, string(resp.GetText()), `image`)
	require.NotContains(t, string(resp.GetText()), "\x1b[")
}

func TestEncodeSavedEntityHighlightAddsColour(t *testing.T) {
	c, handle := appliedApp(t)

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: c.appRecord(handle), Highlight: true}, resp)

	require.Contains(t, string(resp.GetText()), "\x1b[")
	require.Contains(t, string(resp.GetText()), "web")
}

func TestEncodeSavedEntityEmptyIsInvalidArgument(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "entity", out.GetDetails()["argument"])
}

// boxSchema is a schema v1 document declaring the config-only block type box,
// with a required image and an optional tag that is not sensitive, so an
// unset tag is plainly empty
const boxSchema = `{"schema_version":1,"type":"box","fields":[{"name":"image","type":"string"},{"name":"tag","type":"string","optional":true}]}`

// boxConfig declares one box that leaves its tag unset
const boxConfig = "box \"plain\" {\n  image = \"nginx\"\n}\n"

// referenceConfig is appConfig with a second app whose image is set from
// app.web, so its record holds a recorded reference
const referenceConfig = appConfig + "\napp \"copy\" {\n  image = app.web.image\n}\n"

// recordOf is the record of address, taken from Entities
func (c *testClient) recordOf(handle uint64, address string) []byte {
	c.t.Helper()

	for _, entity := range c.entities(handle) {
		if strings.Contains(string(entity), `"id":"`+address+`"`) {
			return entity
		}
	}

	require.FailNow(c.t, address+" is not among the entities")

	return nil
}

// encodeSaved is EncodeSavedEntity with options, returning the text
func (c *testClient) encodeSaved(handle uint64, record []byte, options *corepb.EncodeOptions) string {
	c.t.Helper()

	resp := &corepb.EncodeSavedEntityResponse{}
	c.ok(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{Entity: record, Options: options}, resp)

	return string(resp.GetText())
}

// appliedBox returns a client holding a configuration of the box type to
// which boxConfig has been applied, and its handle
func appliedBox(t *testing.T) (*testClient, uint64) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(&corepb.CreateConfigRequest{
		Registries: []*corepb.Registry{{Kind: &corepb.Registry_Local{Local: &corepb.LocalRegistry{
			Types: []*corepb.DeclaredType{{Schema: []byte(boxSchema)}},
		}}}},
		StateStore: &corepb.StateStore{Kind: &corepb.StateStore_Path{Path: t.TempDir()}},
	})
	c.apply(handle, writeConfig(t, boxConfig))

	return c, handle
}

// savedAppRecord applies appConfig through a host state store and returns
// the record of app.web the engine saved, which holds the real password
func savedAppRecord(t *testing.T) (*testClient, uint64, []byte) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(hostRequest(t))

	store := &fakeStore{}
	opID := c.start(handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, appConfig)}})
	require.Nil(t, c.drive(handle, opID, store, nil).GetError())

	return c, handle, recordFor(t, store.saved, "app.web")
}

func TestEncodeSavedEntityHidesSensitiveByDefault(t *testing.T) {
	c, handle, record := savedAppRecord(t)
	require.Contains(t, string(record), "s3cret")

	text := c.encodeSaved(handle, record, nil)

	require.Equal(t, "app \"web\" {\n  image    = \"nginx\"\n  password = \"(sensitive)\"\n}\n", text)
	require.NotContains(t, text, "s3cret")
}

func TestEncodeSavedEntityRevealsSensitiveWhenAsked(t *testing.T) {
	c, handle, record := savedAppRecord(t)
	require.Contains(t, string(record), "s3cret")

	text := c.encodeSaved(handle, record, &corepb.EncodeOptions{RevealSensitive: true})

	require.Equal(t, "app \"web\" {\n  image    = \"nginx\"\n  password = \"s3cret\"\n}\n", text)
}

func TestEncodeSavedEntityKeepsMarkerWhenRevealed(t *testing.T) {
	c, handle := appliedApp(t)

	// a lookup record holds the marker in place of the password
	record := c.appRecord(handle)
	require.Contains(t, string(record), `"password":"(sensitive)"`)

	text := c.encodeSaved(handle, record, &corepb.EncodeOptions{RevealSensitive: true})

	require.Equal(t, "app \"web\" {\n  image    = \"nginx\"\n  password = \"(sensitive)\"\n}\n", text)
	require.NotContains(t, text, "s3cret")
}

func TestEncodeSavedEntityLeavesEmptyOutByDefault(t *testing.T) {
	c, handle := appliedBox(t)

	text := c.encodeSaved(handle, c.recordOf(handle, "box.plain"), nil)

	require.Equal(t, "box \"plain\" {\n  image = \"nginx\"\n}\n", text)
}

func TestEncodeSavedEntityIncludesEmptyWhenAsked(t *testing.T) {
	c, handle := appliedBox(t)

	text := c.encodeSaved(handle, c.recordOf(handle, "box.plain"), &corepb.EncodeOptions{IncludeEmpty: true})

	require.Equal(t, "box \"plain\" {\n  image = \"nginx\"\n  tag   = \"\"\n}\n", text)
}

func TestEncodeSavedEntityResolvesReferencesByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))
	c.apply(handle, writeConfig(t, referenceConfig))

	record := c.recordOf(handle, "app.copy")
	require.Contains(t, string(record), `"references":{"image":"app.web.image"}`)

	text := c.encodeSaved(handle, record, nil)

	require.Equal(t, "app \"copy\" {\n  image    = \"nginx\"\n  password = \"(sensitive)\"\n}\n", text)
}

func TestEncodeSavedEntityShowsReferencesWhenAsked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))
	c.apply(handle, writeConfig(t, referenceConfig))

	record := c.recordOf(handle, "app.copy")
	require.Contains(t, string(record), `"references":{"image":"app.web.image"}`)

	text := c.encodeSaved(handle, record, &corepb.EncodeOptions{ShowReferences: true})

	require.Equal(t, "app \"copy\" {\n  image    = app.web.image\n  password = \"(sensitive)\"\n}\n", text)
}

func TestEncodeSavedEntityLeavesComputedOutByDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, networkConfig)}})
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())

	record := c.recordOf(f.handle, "resource.network.one")
	require.Contains(t, string(record), `"provider_id":"id-one"`)

	text := c.encodeSaved(f.handle, record, nil)

	require.Equal(t, "resource \"network\" \"one\" {\n  subnet = \"10.0.1.0/24\"\n}\n", text)
}

func TestEncodeSavedEntityIncludesComputedWhenAsked(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	f := newPluginConfig(t, c)

	opID := c.start(f.handle, "StartApply", &corepb.StartApplyRequest{Paths: []string{writeConfig(t, networkConfig)}})
	require.Nil(t, c.drive(f.handle, opID, nil, nil).GetError())

	record := c.recordOf(f.handle, "resource.network.one")
	require.Contains(t, string(record), `"provider_id":"id-one"`)

	text := c.encodeSaved(f.handle, record, &corepb.EncodeOptions{IncludeComputed: true})

	require.Equal(t, "resource \"network\" \"one\" {\n  subnet      = \"10.0.1.0/24\"\n  provider_id = \"id-one\" # set by the provider\n}\n", text)
}

func TestEncodeSavedEntityOptionsMatchGo(t *testing.T) {
	c, handle, record := savedAppRecord(t)

	conf := c.s.lookup(handle)
	require.NotNil(t, conf)

	want, err := conf.cfg.EncodeSavedEntity(record, xcl.IncludeComputed(), xcl.IncludeEmpty(), xcl.RevealSensitive(), xcl.ShowReferences())
	require.NoError(t, err)

	text := c.encodeSaved(handle, record, &corepb.EncodeOptions{
		IncludeComputed: true,
		IncludeEmpty:    true,
		RevealSensitive: true,
		ShowReferences:  true,
	})

	require.Equal(t, string(want), text)
}

func TestEncodeSavedEntityWithoutOptionsIsUnchanged(t *testing.T) {
	c, handle, record := savedAppRecord(t)

	conf := c.s.lookup(handle)
	require.NotNil(t, conf)

	want, err := conf.cfg.EncodeSavedEntity(record)
	require.NoError(t, err)

	withoutOptions := c.encodeSaved(handle, record, nil)
	emptyOptions := c.encodeSaved(handle, record, &corepb.EncodeOptions{})

	require.Equal(t, string(want), withoutOptions)
	require.Equal(t, string(want), emptyOptions)
	require.Equal(t, "app \"web\" {\n  image    = \"nginx\"\n  password = \"(sensitive)\"\n}\n", withoutOptions)
}

func TestEncodeSavedEntityVariableIsNotEncodable(t *testing.T) {
	c, handle := appliedApp(t)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{
		Entity:  c.recordOf(handle, "variable.region"),
		Options: &corepb.EncodeOptions{RevealSensitive: true},
	})

	require.Equal(t, CodeNotEncodable, out.GetCode())
}

func TestEncodeSavedEntityUnknownTypeIsUnregistered(t *testing.T) {
	c, handle := appliedApp(t)

	// a record no registry knows, written by hand since no apply can make one
	record := []byte(`{"meta":{"id":"nope.web","name":"web","type":"nope"},"image":"nginx"}`)

	out := c.fail(handle, "EncodeSavedEntity", &corepb.EncodeSavedEntityRequest{
		Entity:  record,
		Options: &corepb.EncodeOptions{ShowReferences: true},
	})

	require.Equal(t, CodeUnregisteredType, out.GetCode())
}

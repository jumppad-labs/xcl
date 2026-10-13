package coreservice

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

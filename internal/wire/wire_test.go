package wire_test

import (
	"encoding/json"
	"testing"

	"github.com/jumppad-labs/xcl/internal/cty"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// requireSameAsEncodingJSON asserts wire.Marshal writes exactly the bytes
// encoding/json writes for a value holding nothing sensitive, which is the
// oracle for every test that calls it.
func requireSameAsEncodingJSON(t *testing.T, value any) {
	t.Helper()

	expected, err := json.Marshal(value)
	require.NoError(t, err)

	actual, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, string(expected), string(actual))
}

type plainEmbeddedLeft struct {
	Name  string `json:"name"`
	Left  string `json:"left"`
	Clash string
}

type plainEmbeddedRight struct {
	Name  string
	Right string `json:"right"`
	Clash string
}

type plainTaggedDominance struct {
	plainEmbeddedLeft
	plainEmbeddedRight
	Outer string `json:"outer"`
}

// plainDepthDominance has an untagged Clash that hides the embedded one
type plainDepthDominance struct {
	plainEmbeddedLeft
	Clash string
}

type plainOptions struct {
	Omitted    string `json:"omitted,omitempty"`
	Skipped    string `json:"-"`
	Dash       string `json:"-,"`
	Quoted     int    `json:"quoted,string"`
	QuotedBool bool   `json:"quoted_bool,string"`
	Kept       string `json:"kept"`
}

type plainCustomMarshal struct {
	Value string
}

func (p plainCustomMarshal) MarshalJSON() ([]byte, error) {
	return []byte(`{"custom":"` + p.Value + `"}`), nil
}

type plainWithCustom struct {
	Custom plainCustomMarshal  `json:"custom"`
	Ptr    *plainCustomMarshal `json:"ptr"`
}

type plainCollections struct {
	IntKeys   map[int]string `json:"int_keys"`
	NilSlice  []string       `json:"nil_slice"`
	NilMap    map[string]int `json:"nil_map"`
	Bytes     []byte         `json:"bytes"`
	EmptyList []string       `json:"empty_list"`
}

func TestMarshalResourceBaseMatchesEncodingJSON(t *testing.T) {
	base := types.ResourceBase{
		DependsOn: []string{"resource.a.b"},
		Meta: types.Meta{
			ID:   "resource.database.main",
			Name: "main",
			Type: "resource",
			Properties: map[string]any{
				"zeta":  1,
				"alpha": "two",
				"list":  []any{1, "x"},
			},
			Links: []string{"resource.app.web"},
		},
	}

	requireSameAsEncodingJSON(t, base)
	requireSameAsEncodingJSON(t, &base)
}

func TestMarshalRegisteredDatabaseMatchesEncodingJSON(t *testing.T) {
	database := &registered.Database{
		Location: "eu",
		Port:     5432,
		Timeouts: &registered.Timeouts{Connect: 5, Read: 10},
	}
	database.Meta.ID = "resource.database.main"

	requireSameAsEncodingJSON(t, database)
}

func TestMarshalRegisteredAppMatchesEncodingJSON(t *testing.T) {
	app := &registered.App{
		Environment:      "prod",
		DatabaseLocation: "eu",
		DatabasePort:     5432,
		SharedLocation:   "shared",
	}

	requireSameAsEncodingJSON(t, app)
}

func TestMarshalOutputMatchesEncodingJSON(t *testing.T) {
	output := &types.Output{
		CtyValue:    cty.StringVal("hello"),
		Value:       map[string]any{"b": 2, "a": []any{"x"}},
		Description: "an output",
	}
	output.Meta.ID = "output.first"

	requireSameAsEncodingJSON(t, output)
}

func TestMarshalOutputWithNilValuesMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, &types.Output{})
}

func TestMarshalEmbeddedStructsWithConflictingNamesMatchEncodingJSON(t *testing.T) {
	value := plainTaggedDominance{
		plainEmbeddedLeft:  plainEmbeddedLeft{Name: "l", Left: "left", Clash: "lc"},
		plainEmbeddedRight: plainEmbeddedRight{Name: "r", Right: "right", Clash: "rc"},
		Outer:              "outer",
	}

	requireSameAsEncodingJSON(t, value)
}

func TestMarshalShallowerFieldDominatesEmbeddedFieldLikeEncodingJSON(t *testing.T) {
	value := plainDepthDominance{
		plainEmbeddedLeft: plainEmbeddedLeft{Name: "inner", Left: "left", Clash: "inner"},
		Clash:             "outer",
	}

	requireSameAsEncodingJSON(t, value)
}

func TestMarshalOmitEmptyMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainOptions{})
}

func TestMarshalDashTagMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainOptions{Skipped: "skip", Dash: "dash", Kept: "k"})
}

func TestMarshalStringOptionMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainOptions{Quoted: 42, QuotedBool: true, Omitted: "o"})
}

func TestMarshalCustomMarshalJSONMatchesEncodingJSON(t *testing.T) {
	value := plainWithCustom{
		Custom: plainCustomMarshal{Value: "one"},
		Ptr:    &plainCustomMarshal{Value: "two"},
	}

	requireSameAsEncodingJSON(t, value)
}

func TestMarshalNilPointerWithCustomMarshalJSONMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainWithCustom{})
}

func TestMarshalMapWithIntKeysMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainCollections{
		IntKeys: map[int]string{10: "ten", 2: "two", -1: "minus"},
	})
}

func TestMarshalNilSliceAndMapMatchEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainCollections{})
}

func TestMarshalByteSliceMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, plainCollections{
		Bytes:     []byte("hello bytes"),
		EmptyList: []string{},
	})
}

func TestMarshalNilMatchesEncodingJSON(t *testing.T) {
	requireSameAsEncodingJSON(t, nil)
}

func TestMarshalIndentMatchesEncodingJSON(t *testing.T) {
	database := &registered.Database{
		Location: "eu",
		Port:     5432,
		Timeouts: &registered.Timeouts{Connect: 5},
	}

	expected, err := json.MarshalIndent(database, "", "  ")
	require.NoError(t, err)

	actual, err := wire.MarshalIndent(database, "", "  ")
	require.NoError(t, err)

	require.Equal(t, string(expected), string(actual))
}

type withSensitiveField struct {
	Name     string                  `json:"name"`
	Password types.Sensitive[string] `json:"password"`
}

type nestedSensitive struct {
	Inner withSensitiveField `json:"inner"`
}

type embeddedSensitive struct {
	withSensitiveField
	Other string `json:"other"`
}

type withSensitiveSlice struct {
	Items []types.Sensitive[string] `json:"items"`
}

type withSensitiveMap struct {
	Items map[string]types.Sensitive[string] `json:"items"`
}

type withAnyField struct {
	Value any `json:"value"`
}

type withSensitiveInt struct {
	Count types.Sensitive[int] `json:"count"`
}

func TestMarshalTopLevelSensitiveWritesRealValue(t *testing.T) {
	data, err := wire.Marshal(types.NewSensitive("hunter2"))
	require.NoError(t, err)

	require.Equal(t, `"hunter2"`, string(data))
}

func TestMarshalSensitiveStructFieldWritesRealValue(t *testing.T) {
	value := withSensitiveField{Name: "admin", Password: types.NewSensitive("hunter2")}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"name":"admin","password":"hunter2"}`, string(data))
}

func TestMarshalSensitiveInNestedStructWritesRealValue(t *testing.T) {
	value := nestedSensitive{Inner: withSensitiveField{Name: "admin", Password: types.NewSensitive("hunter2")}}

	data, err := wire.Marshal(&value)
	require.NoError(t, err)

	require.Equal(t, `{"inner":{"name":"admin","password":"hunter2"}}`, string(data))
}

func TestMarshalSensitiveInEmbeddedStructWritesRealValue(t *testing.T) {
	value := embeddedSensitive{
		withSensitiveField: withSensitiveField{Name: "admin", Password: types.NewSensitive("hunter2")},
		Other:              "x",
	}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"name":"admin","password":"hunter2","other":"x"}`, string(data))
}

func TestMarshalSensitiveSliceElementWritesRealValue(t *testing.T) {
	value := withSensitiveSlice{Items: []types.Sensitive[string]{
		types.NewSensitive("one"),
		types.NewSensitive("two"),
	}}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"items":["one","two"]}`, string(data))
}

func TestMarshalSensitiveMapValueWritesRealValue(t *testing.T) {
	value := withSensitiveMap{Items: map[string]types.Sensitive[string]{
		"b": types.NewSensitive("two"),
		"a": types.NewSensitive("one"),
	}}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"items":{"a":"one","b":"two"}}`, string(data))
}

func TestMarshalSensitiveHeldInAnyFieldWritesRealValue(t *testing.T) {
	value := withAnyField{Value: types.NewSensitive("hunter2")}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"value":"hunter2"}`, string(data))
}

func TestMarshalSensitiveIntWritesRealValue(t *testing.T) {
	value := withSensitiveInt{Count: types.NewSensitive(42)}

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"count":42}`, string(data))
}

func TestMarshalRedactedSensitiveWritesTheMarker(t *testing.T) {
	var value withSensitiveField
	err := json.Unmarshal([]byte(`{"name":"admin","password":"(sensitive)"}`), &value)
	require.NoError(t, err)

	data, err := wire.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"name":"admin","password":"(sensitive)"}`, string(data))
}

func TestEncodingJSONStillWritesTheMarkerForSensitiveField(t *testing.T) {
	value := withSensitiveField{Name: "admin", Password: types.NewSensitive("hunter2")}

	data, err := json.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"name":"admin","password":"(sensitive)"}`, string(data))
}

func TestEncodingJSONStillWritesTheMarkerForSensitiveHeldInAnyField(t *testing.T) {
	value := withAnyField{Value: types.NewSensitive("hunter2")}

	data, err := json.Marshal(value)
	require.NoError(t, err)

	require.Equal(t, `{"value":"(sensitive)"}`, string(data))
}

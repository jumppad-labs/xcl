package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// roundTripPropertyChanges encodes changes to their protocol form and decodes
// them again, as they would cross to a plugin run as a separate program
func roundTripPropertyChanges(t *testing.T, changes []entity.PropertyChange) []entity.PropertyChange {
	t.Helper()

	encoded, err := toProtoPropertyChanges(changes)
	require.NoError(t, err)

	decoded, err := fromProtoPropertyChanges(encoded)
	require.NoError(t, err)

	return decoded
}

func TestPropertyChangesRoundTripAPlainChange(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.0", After: "nginx:2.0"},
	}

	decoded := roundTripPropertyChanges(t, changes)

	expected := []entity.PropertyChange{
		{
			Path:   entity.Path{{Kind: entity.StepAttribute, Attribute: "image"}},
			Before: "nginx:1.0",
			After:  "nginx:2.0",
		},
	}
	require.Equal(t, expected, decoded)
}

func TestPropertyChangesRoundTripANestedPathWithIndexAndKey(t *testing.T) {
	changes := []entity.PropertyChange{
		{
			Path:   entity.Path{}.Attribute("container").Index(2).Attribute("env").Key("LOG_LEVEL"),
			Before: "info",
			After:  "debug",
		},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	expectedPath := entity.Path{
		{Kind: entity.StepAttribute, Attribute: "container"},
		{Kind: entity.StepIndex, Index: 2},
		{Kind: entity.StepAttribute, Attribute: "env"},
		{Kind: entity.StepKey, Key: "LOG_LEVEL"},
	}
	require.Equal(t, expectedPath, decoded[0].Path)
	require.Equal(t, `container[2].env["LOG_LEVEL"]`, decoded[0].Path.String())
}

func TestPropertyChangesRoundTripASensitiveChangeWithItsRealValues(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("password"), Before: "old-secret", After: "new-secret", Sensitive: true},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.True(t, decoded[0].Sensitive)
	require.Equal(t, "old-secret", decoded[0].Before)
	require.Equal(t, "new-secret", decoded[0].After)
}

func TestPropertyChangesEncodeASensitiveChangeWithItsRealValues(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("password"), Before: "old-secret", After: "new-secret", Sensitive: true},
	}

	encoded, err := toProtoPropertyChanges(changes)
	require.NoError(t, err)

	require.Len(t, encoded, 1)
	require.True(t, encoded[0].Sensitive)
	require.Equal(t, []byte(`"old-secret"`), encoded[0].Before)
	require.Equal(t, []byte(`"new-secret"`), encoded[0].After)
}

func TestPropertyChangesRoundTripAnUnknownChangeWithNoAfter(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("ip_address"), Before: "10.0.0.1", Unknown: true},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.True(t, decoded[0].Unknown)
	require.Nil(t, decoded[0].After)
	require.Equal(t, "10.0.0.1", decoded[0].Before)
}

func TestPropertyChangesRoundTripANilBeforeForAnAddedSetting(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("label"), Before: nil, After: "web"},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Nil(t, decoded[0].Before)
	require.Equal(t, "web", decoded[0].After)
}

func TestPropertyChangesRoundTripANilAfterForARemovedSetting(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("label"), Before: "web", After: nil},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Equal(t, "web", decoded[0].Before)
	require.Nil(t, decoded[0].After)
	require.False(t, decoded[0].Unknown)
}

func TestPropertyChangesRoundTripNumbersAsFloat64(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("count"), Before: 1, After: 2},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Equal(t, float64(1), decoded[0].Before)
	require.Equal(t, float64(2), decoded[0].After)
}

func TestPropertyChangesRoundTripBooleans(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("privileged"), Before: false, After: true},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Equal(t, false, decoded[0].Before)
	require.Equal(t, true, decoded[0].After)
}

func TestPropertyChangesRoundTripMapsAsMapOfAny(t *testing.T) {
	changes := []entity.PropertyChange{
		{
			Path:   entity.Path{}.Attribute("env"),
			Before: map[string]string{"LOG_LEVEL": "info"},
			After:  map[string]any{"LOG_LEVEL": "debug", "WORKERS": 4},
		},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Equal(t, map[string]any{"LOG_LEVEL": "info"}, decoded[0].Before)
	require.Equal(t, map[string]any{"LOG_LEVEL": "debug", "WORKERS": float64(4)}, decoded[0].After)
}

func TestPropertyChangesRoundTripListsAsSliceOfAny(t *testing.T) {
	changes := []entity.PropertyChange{
		{
			Path:   entity.Path{}.Attribute("ports"),
			Before: []int{80},
			After:  []string{"80", "443"},
		},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Len(t, decoded, 1)
	require.Equal(t, []any{float64(80)}, decoded[0].Before)
	require.Equal(t, []any{"80", "443"}, decoded[0].After)
}

func TestPropertyChangesRoundTripKeepsOrderOfSeveralChanges(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{}.Attribute("image"), Before: "nginx:1.0", After: "nginx:2.0"},
		{Path: entity.Path{}.Attribute("count"), Before: float64(1), After: float64(3)},
		{Path: entity.Path{}.Attribute("token"), Before: "a", After: "b", Sensitive: true},
	}

	decoded := roundTripPropertyChanges(t, changes)

	require.Equal(t, changes, decoded)
}

func TestPropertyChangesEncodeAnEmptyListAsNil(t *testing.T) {
	encoded, err := toProtoPropertyChanges([]entity.PropertyChange{})
	require.NoError(t, err)

	require.Nil(t, encoded)
}

func TestPropertyChangesEncodeANilListAsNil(t *testing.T) {
	encoded, err := toProtoPropertyChanges(nil)
	require.NoError(t, err)

	require.Nil(t, encoded)
}

func TestPropertyChangesDecodeAnEmptyListAsNil(t *testing.T) {
	decoded, err := fromProtoPropertyChanges([]*proto.PropertyChange{})
	require.NoError(t, err)

	require.Nil(t, decoded)
}

func TestPropertyChangesDecodeANilListAsNil(t *testing.T) {
	decoded, err := fromProtoPropertyChanges(nil)
	require.NoError(t, err)

	require.Nil(t, decoded)
}

func TestPropertyChangesDecodeRejectsAnUnknownStepKind(t *testing.T) {
	changes := []*proto.PropertyChange{
		{
			Path:  []*proto.PathStep{{Kind: proto.StepKind(99), Attribute: "image"}},
			After: []byte(`"nginx:2.0"`),
		},
	}

	_, err := fromProtoPropertyChanges(changes)

	require.ErrorContains(t, err, "unknown step kind 99")
}

func TestPropertyChangesEncodeRejectsAnUnknownStepKind(t *testing.T) {
	changes := []entity.PropertyChange{
		{Path: entity.Path{{Kind: entity.StepKind(99), Attribute: "image"}}, After: "nginx:2.0"},
	}

	_, err := toProtoPropertyChanges(changes)

	require.ErrorContains(t, err, "unknown step kind 99")
}

func TestPropertyChangesDecodeRejectsInvalidValueJSON(t *testing.T) {
	changes := []*proto.PropertyChange{
		{
			Path:  []*proto.PathStep{{Kind: proto.StepKind_STEP_KIND_ATTRIBUTE, Attribute: "image"}},
			After: []byte(`{not json`),
		},
	}

	_, err := fromProtoPropertyChanges(changes)

	require.ErrorContains(t, err, "change image: after")
}

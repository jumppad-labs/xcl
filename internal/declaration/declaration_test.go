package declaration

import (
	"testing"

	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// Thing is a plain Go resource type used to test declarations
type Thing struct {
	types.ResourceBase `xcl:",remain"`

	Size int `xcl:"size" json:"size"`
}

// NotAResource is a struct that does not embed types.ResourceBase
type NotAResource struct {
	Size int `xcl:"size" json:"size"`
}

func TestValidateAcceptsATypeWithoutASubtype(t *testing.T) {
	err := Validate(&Thing{}, "thing")
	require.NoError(t, err)
}

func TestValidateAcceptsATypeWithASubtype(t *testing.T) {
	err := Validate(&Thing{}, "resource", "thing")
	require.NoError(t, err)
}

func TestValidateFailsForNoName(t *testing.T) {
	err := Validate(&Thing{})
	require.EqualError(t, err, "an entity type must be named")
}

func TestValidateFailsForAnEmptyType(t *testing.T) {
	err := Validate(&Thing{}, "")
	require.EqualError(t, err, "an entity type must be named")
}

func TestValidateFailsForMoreThanOneSubtype(t *testing.T) {
	err := Validate(&Thing{}, "server", "big", "small")
	require.EqualError(t, err, `type "server" takes at most one subtype, got 2`)
}

func TestValidateFailsForAnEmptySubtype(t *testing.T) {
	err := Validate(&Thing{}, "server", "")
	require.EqualError(t, err, `type "server" was given an empty subtype, leave it out to register the type without one`)
}

func TestValidateFailsForNonPointer(t *testing.T) {
	err := Validate(Thing{}, "resource", "thing")
	require.EqualError(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)
}

func TestValidateFailsForNil(t *testing.T) {
	err := Validate(nil, "resource", "thing")
	require.EqualError(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)
}

func TestValidateFailsForNilPointer(t *testing.T) {
	var thing *Thing

	err := Validate(thing, "resource", "thing")
	require.EqualError(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)
}

func TestValidateFailsForPointerToNonStruct(t *testing.T) {
	size := 1

	err := Validate(&size, "resource", "thing")
	require.EqualError(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase`)
}

func TestValidateFailsForTypeWithoutResourceBase(t *testing.T) {
	err := Validate(&NotAResource{}, "resource", "thing")
	require.ErrorContains(t, err, `type "resource.thing" must be a pointer to a struct that embeds types.ResourceBase: `)
}

package gocty

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/cty"
)

// origin is what the tests decode the "origin" attribute into
type origin struct {
	Where string `xcl:"where"`
}

// located takes "origin" in an untagged field
type located struct {
	Name   string `xcl:"name"`
	Origin origin
}

// taggedOrigin names the field with a tag, so the registration does not apply
type taggedOrigin struct {
	Name   string  `xcl:"name"`
	Origin *origin `xcl:"place,optional"`
}

// otherOrigin has a field of that name of another type
type otherOrigin struct {
	Name   string `xcl:"name"`
	Origin string
}

func init() {
	RegisterUntaggedField("origin", "Origin", reflect.TypeFor[origin]())
}

func locatedValue() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"name":   cty.StringVal("web"),
		"origin": cty.ObjectVal(map[string]cty.Value{"where": cty.StringVal("main.xcl")}),
	})
}

func TestFromCtyValueFillsARegisteredUntaggedField(t *testing.T) {
	var target located
	require.NoError(t, FromCtyValue(locatedValue(), &target))

	require.Equal(t, "main.xcl", target.Origin.Where)
}

func TestFromCtyValueLeavesATaggedFieldOfThatNameAlone(t *testing.T) {
	var target taggedOrigin
	require.NoError(t, FromCtyValue(locatedValue(), &target))

	require.Nil(t, target.Origin)
}

func TestFromCtyValueLeavesAFieldOfAnotherTypeAlone(t *testing.T) {
	var target otherOrigin
	require.NoError(t, FromCtyValue(locatedValue(), &target))

	require.Empty(t, target.Origin)
}

func TestImpliedConstraintTypeHoldsARegisteredUntaggedFieldAsOptional(t *testing.T) {
	ty, err := ImpliedConstraintType(located{})
	require.NoError(t, err)

	require.True(t, ty.HasAttribute("origin"))
	require.True(t, ty.AttributeOptional("origin"))
}

func TestImpliedTypeLeavesARegisteredUntaggedFieldOut(t *testing.T) {
	ty, err := ImpliedType(located{})
	require.NoError(t, err)

	require.False(t, ty.HasAttribute("origin"))
}

package catalog

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/stretchr/testify/require"
)

// TypePath is what lets a lookup name a Go type and nothing else: the address
// segments are read back from how the type was registered rather than spelled
// out by the caller: its type, followed by its subtype where it has one.

func TestTypePathReturnsTheResourcePathForAResourceRegistration(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	path, ok := c.TypePath(reflect.TypeFor[Thing]())
	require.True(t, ok)
	require.Equal(t, []string{"resource", "thing"}, path)
}

func TestTypePathReturnsTheTypePathForARegistrationWithoutASubtype(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "thing"))

	path, ok := c.TypePath(reflect.TypeFor[Thing]())
	require.True(t, ok)
	require.Equal(t, []string{"thing"}, path)
}

func TestTypePathReturnsTheTypeAndSubtypeOfAnyType(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "server", "big"))

	path, ok := c.TypePath(reflect.TypeFor[Thing]())
	require.True(t, ok)
	require.Equal(t, []string{"server", "big"}, path)
}

func TestTypePathReturnsTheBuiltinPathForABuiltinType(t *testing.T) {
	c := New()

	path, ok := c.TypePath(reflect.TypeFor[resources.Variable]())
	require.True(t, ok)
	require.Equal(t, []string{"variable"}, path)
}

func TestTypePathResolvesAPointerAndANonPointerAlike(t *testing.T) {
	c := New()

	require.NoError(t, c.RegisterType(&Thing{}, "resource", "thing"))

	value, valueOK := c.TypePath(reflect.TypeFor[Thing]())
	require.True(t, valueOK)

	pointer, pointerOK := c.TypePath(reflect.TypeFor[*Thing]())
	require.True(t, pointerOK)

	require.Equal(t, value, pointer)
}

// A type the registry cannot reach has no path at all. A plugin provides a
// schema and no Go type, so a type that only a plugin declares is reported the
// same way as one that was never registered: the caller falls back to naming
// the kind.

func TestTypePathRejectsATypeThatWasNeverRegistered(t *testing.T) {
	c := New()

	path, ok := c.TypePath(reflect.TypeFor[Gadget]())
	require.False(t, ok)
	require.Nil(t, path)
}

func TestTypePathRejectsAPluginProvidedType(t *testing.T) {
	c := New()

	c.AddRegistry(localWith(&thingPlugin{}))

	err := c.Load(nil)
	require.NoError(t, err)

	// the registry knows the name, because the loaded plugin declares it
	require.True(t, c.KnownType("resource", "thing"))

	// but the type exists host side only as a schema, so there is nothing to
	// reflect against and no path to derive
	path, ok := c.TypePath(reflect.TypeFor[Thing]())
	require.False(t, ok)
	require.Nil(t, path)
}

func TestTypePathRejectsANilType(t *testing.T) {
	c := New()

	path, ok := c.TypePath(nil)
	require.False(t, ok)
	require.Nil(t, path)
}

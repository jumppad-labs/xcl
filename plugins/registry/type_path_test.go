package registry

import (
	"reflect"
	"testing"

	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/stretchr/testify/require"
)

// TypePath is what lets a lookup name a Go type and nothing else: the address
// segments are read back from how the type was registered rather than spelled
// out by the caller. A kind led registration is reached through the resource
// keyword, a bare one and a builtin lead their own declaration.

func TestTypePathReturnsTheResourcePathForAKindLedRegistration(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	path, ok := r.TypePath(reflect.TypeFor[Thing]())
	require.True(t, ok)
	require.Equal(t, []string{"resource", "thing"}, path)
}

func TestTypePathReturnsTheBarePathForABareRegistration(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterBareType("thing", &Thing{})
	require.NoError(t, err)

	path, ok := r.TypePath(reflect.TypeFor[Thing]())
	require.True(t, ok)
	require.Equal(t, []string{"thing"}, path)
}

func TestTypePathReturnsTheBuiltinPathForABuiltinType(t *testing.T) {
	r := NewPluginRegistry()

	path, ok := r.TypePath(reflect.TypeFor[resources.Variable]())
	require.True(t, ok)
	require.Equal(t, []string{"variable"}, path)
}

func TestTypePathResolvesAPointerAndANonPointerAlike(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterType("thing", &Thing{})
	require.NoError(t, err)

	value, valueOK := r.TypePath(reflect.TypeFor[Thing]())
	require.True(t, valueOK)

	pointer, pointerOK := r.TypePath(reflect.TypeFor[*Thing]())
	require.True(t, pointerOK)

	require.Equal(t, value, pointer)
}

// A type the registry cannot reach has no path at all. A plugin provides a
// schema and no Go type, so a type that only a plugin declares is reported the
// same way as one that was never registered: the caller falls back to naming
// the kind.

func TestTypePathRejectsATypeThatWasNeverRegistered(t *testing.T) {
	r := NewPluginRegistry()

	path, ok := r.TypePath(reflect.TypeFor[Gadget]())
	require.False(t, ok)
	require.Nil(t, path)
}

func TestTypePathRejectsAPluginProvidedType(t *testing.T) {
	r := NewPluginRegistry()

	err := r.RegisterPlugin(&thingPlugin{})
	require.NoError(t, err)

	err = r.Load(nil)
	require.NoError(t, err)

	// the registry knows the name, because the loaded plugin declares it
	require.True(t, r.KnownType("thing"))

	// but the type exists host side only as a schema, so there is nothing to
	// reflect against and no path to derive
	path, ok := r.TypePath(reflect.TypeFor[Thing]())
	require.False(t, ok)
	require.Nil(t, path)
}

func TestTypePathRejectsANilType(t *testing.T) {
	r := NewPluginRegistry()

	path, ok := r.TypePath(nil)
	require.False(t, ok)
	require.Nil(t, path)
}

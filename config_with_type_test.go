package xcl

import (
	"fmt"
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/registry"
	"github.com/stretchr/testify/require"
)

// Declaring a type or adding a registry wrongly is a programmer error, the
// same on every run, so it panics with a message naming the type.

// panicMessage runs f and returns the message it panicked with, failing the
// test when it does not panic
func panicMessage(t *testing.T, f func()) (message string) {
	t.Helper()

	defer func() {
		recovered := recover()
		require.NotNil(t, recovered, "expected a panic")
		message = fmt.Sprint(recovered)
	}()

	f()

	return ""
}

func TestWithTypePanicsOnEmptyName(t *testing.T) {
	message := panicMessage(t, func() {
		WithType(&registered.Database{})
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, "named")
}

func TestWithTypePanicsOnMoreThanOneSubtype(t *testing.T) {
	message := panicMessage(t, func() {
		WithType(&registered.Database{}, "resource", "database", "extra")
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"resource"`)
	require.Contains(t, message, "at most one subtype")
}

func TestWithTypePanicsOnEmptySubtype(t *testing.T) {
	message := panicMessage(t, func() {
		WithType(&registered.Database{}, "resource", "")
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"resource"`)
	require.Contains(t, message, "empty subtype")
}

func TestWithTypePanicsOnNonEntityType(t *testing.T) {
	// Timeouts is a nested block, it does not embed types.ResourceBase
	message := panicMessage(t, func() {
		WithType(&registered.Timeouts{}, "timeouts")
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"timeouts"`)
	require.Contains(t, message, "types.ResourceBase")
}

func TestNewConfigPanicsOnDuplicateType(t *testing.T) {
	message := panicMessage(t, func() {
		NewConfig(
			WithType(&registered.Database{}, "resource", "database"),
			WithType(&registered.App{}, "resource", "database"),
		)
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"resource.database"`)
}

func TestNewConfigPanicsOnTypeInBothForms(t *testing.T) {
	message := panicMessage(t, func() {
		NewConfig(
			WithType(&registered.Cache{}, "server"),
			WithType(&registered.Server{}, "server", "big"),
		)
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"server"`)
}

func TestNewConfigPanicsOnBuiltinTypeName(t *testing.T) {
	message := panicMessage(t, func() {
		NewConfig(WithType(&registered.Database{}, "variable"))
	})

	require.Contains(t, message, "xcl: ")
	require.Contains(t, message, `"variable"`)
	require.Contains(t, message, "builtin")
}

func TestWithRegistryPanicsOnNil(t *testing.T) {
	require.PanicsWithValue(t, "xcl: registry must not be nil", func() {
		WithRegistry(nil)
	})
}

func TestNewConfigAcceptsTypesAndRegistriesInAnyOrder(t *testing.T) {
	first := registry.NewLocal()
	second := registry.NewLocal()

	c, err := NewConfig(
		WithRegistry(first),
		WithType(&registered.Database{}, "resource", registered.TypeDatabase),
		WithRegistry(second),
		WithType(&registered.Cache{}, registered.TypeCache),
	)
	require.NoError(t, err)

	require.Equal(t, []registry.Registry{first, second}, c.registries)
	require.True(t, c.catalog.IsRegisteredType("resource", registered.TypeDatabase))
	require.True(t, c.catalog.IsRegisteredType(registered.TypeCache, ""))
}

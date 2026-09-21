//go:build go1.27

package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/stretchr/testify/require"
)

// The lookup surface is spelled twice, as a method on the configuration and as
// a package level function, and neither spelling carries any logic of its own.
// These tests hold the two forms against each other: given the same
// configuration and the same inputs, they hand back the very same entity, and
// they fail with the very same error.
//
// Generic methods arrived in Go 1.27, so the method form only exists from that
// version. This file is excluded below it, which is why the equivalence can
// only be asserted here: on the floor toolchain there is one spelling and
// nothing to compare it with.

func TestEveryLookupIsAvailableAsAFunctionAndAsAMethod(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.NotNil(t, database)

	databaseMethod, err := c.Find[registered.Database]("resource.database.main")
	require.NoError(t, err)
	require.NotNil(t, databaseMethod)

	databases, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)
	require.Len(t, databases, 2)

	databasesMethod, err := c.FindByType[registered.Database]("resource", "database")
	require.NoError(t, err)
	require.Len(t, databasesMethod, 2)

	app, err := FindOne[registered.App](c, "resource", "app")
	require.NoError(t, err)
	require.NotNil(t, app)

	appMethod, err := c.FindOne[registered.App]("resource", "app")
	require.NoError(t, err)
	require.NotNil(t, appMethod)

	all, err := All[registered.Database](c)
	require.NoError(t, err)
	require.Len(t, all, 2)

	allMethod, err := c.All[registered.Database]()
	require.NoError(t, err)
	require.Len(t, allMethod, 2)
}

// Where the lookup succeeds, both spellings hand back the entity the
// configuration holds, so what the caller has is the same value and not two
// copies of it that could be changed independently.

func TestFindReturnsTheSameEntityFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)

	method, err := c.Find[registered.Database]("resource.database.main")
	require.NoError(t, err)

	require.Same(t, function, method)
}

func TestFindByTypeReturnsTheSameEntitiesFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, err := FindByType[registered.Database](c, "resource", "database")
	require.NoError(t, err)

	method, err := c.FindByType[registered.Database]("resource", "database")
	require.NoError(t, err)

	require.Len(t, method, len(function))
	require.Len(t, function, 2)

	for i := range function {
		require.Same(t, function[i], method[i])
	}
}

func TestFindOneReturnsTheSameEntityFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, err := FindOne[registered.App](c, "resource", "app")
	require.NoError(t, err)

	method, err := c.FindOne[registered.App]("resource", "app")
	require.NoError(t, err)

	require.Same(t, function, method)
}

func TestAllReturnsTheSameEntitiesFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, err := All[registered.Database](c)
	require.NoError(t, err)

	method, err := c.All[registered.Database]()
	require.NoError(t, err)

	require.Len(t, method, len(function))
	require.Len(t, function, 2)

	for i := range function {
		require.Same(t, function[i], method[i])
	}
}

// Where the lookup fails, both spellings fail in the same way: the same
// condition to match on and the same words to read, so a caller who moves
// between the two has nothing to relearn about handling errors.

func TestFindFailsIdenticallyFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, functionErr := Find[registered.Database](c, "resource.database.missing")
	require.Error(t, functionErr)
	require.Nil(t, function)

	method, methodErr := c.Find[registered.Database]("resource.database.missing")
	require.Error(t, methodErr)
	require.Nil(t, method)

	require.ErrorIs(t, functionErr, ErrNotFound)
	require.ErrorIs(t, methodErr, ErrNotFound)
	require.Equal(t, functionErr.Error(), methodErr.Error())
}

func TestFindByTypeFailsIdenticallyFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, functionErr := FindByType[registered.Database](c, "nonsense")
	require.Error(t, functionErr)
	require.Nil(t, function)

	method, methodErr := c.FindByType[registered.Database]("nonsense")
	require.Error(t, methodErr)
	require.Nil(t, method)

	require.ErrorIs(t, functionErr, ErrUnknownType)
	require.ErrorIs(t, methodErr, ErrUnknownType)
	require.Equal(t, functionErr.Error(), methodErr.Error())
}

func TestFindOneFailsIdenticallyFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	function, functionErr := FindOne[registered.Database](c, "resource", "database")
	require.Error(t, functionErr)
	require.Nil(t, function)

	method, methodErr := c.FindOne[registered.Database]("resource", "database")
	require.Error(t, methodErr)
	require.Nil(t, method)

	require.ErrorIs(t, functionErr, ErrNotUnique)
	require.ErrorIs(t, methodErr, ErrNotUnique)
	require.Equal(t, functionErr.Error(), methodErr.Error())
}

func TestAllFailsIdenticallyFromBothSpellings(t *testing.T) {
	c := setupFindConfig(t)

	// Timeouts is the timeouts block of a Database, it has no address of its
	// own and so cannot be enumerated by either spelling
	function, functionErr := All[registered.Timeouts](c)
	require.Error(t, functionErr)
	require.Nil(t, function)

	method, methodErr := c.All[registered.Timeouts]()
	require.Error(t, methodErr)
	require.Nil(t, method)

	require.ErrorIs(t, functionErr, ErrNotAnEntity)
	require.ErrorIs(t, methodErr, ErrNotAnEntity)
	require.Equal(t, functionErr.Error(), methodErr.Error())
}

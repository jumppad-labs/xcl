package xcl

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/stretchr/testify/require"
)

// The lookup surface replaced a helper that was constructed for one Go type
// and answered only for that type. One configuration now answers for as many
// types as the caller asks about.
//
// A lookup names its type at the call, not at a construction, so a caller asks
// a configuration about whatever type they like, one after another, with
// nothing to set up in between.

func TestConsecutiveLookupsOfDifferentTypesAllSucceed(t *testing.T) {
	c := setupFindConfig(t)

	database, err := Find[registered.Database](c, "resource.database.main")
	require.NoError(t, err)
	require.Equal(t, "resource.database.main", database.Meta.ID)

	app, err := Find[registered.App](c, "resource.app.web")
	require.NoError(t, err)
	require.Equal(t, "resource.app.web", app.Meta.ID)

	consumers, err := FindByType[registered.Consumer](c, "resource", "consumer")
	require.NoError(t, err)
	require.Len(t, consumers, 1)
	require.Equal(t, "resource.consumer.reader", consumers[0].Meta.ID)
}

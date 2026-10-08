package xcl

import (
	"fmt"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func TestEventEntityReturnsRegisteredGoType(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")

	entity, err := succeeded.Entity()
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)
	require.Equal(t, "admin", secret.Username)
}

func TestEventEntityShowsSensitiveValuesMasked(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")

	entity, err := succeeded.Entity()
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)
	require.Equal(t, types.SensitiveMarker, fmt.Sprint(secret.Password))
}

func TestEventEntityHoldsNoRealSensitiveValue(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")

	entity, err := succeeded.Entity()
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)
	require.Equal(t, "", secret.Password.Reveal())
}

func TestEventEntityWithoutDataReturnsNil(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataProcessed)

	started := eventDataSingleEvent(t, recorder, "", events.OperationApply, events.PhaseStart)
	require.Empty(t, started.Data)

	entity, err := started.Entity()
	require.NoError(t, err)
	require.Nil(t, entity)
}

func TestEventEntityFromRawDataReturnsRegisteredGoType(t *testing.T) {
	recorder, _ := applySensitiveFixtureWithEventData(t, EventDataRaw)

	succeeded := eventDataSingleEvent(t, recorder, eventSensitiveSecretID, "create", "success")
	require.NotEmpty(t, succeeded.Data)

	entity, err := succeeded.Entity()
	require.NoError(t, err)

	secret, ok := entity.(*registered.Secret)
	require.True(t, ok, "expected *registered.Secret, got %T", entity)
	require.Equal(t, "admin", secret.Username)
	require.Equal(t, types.SensitiveMarker, fmt.Sprint(secret.Password))
}

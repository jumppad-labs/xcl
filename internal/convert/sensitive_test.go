package convert

import (
	"testing"

	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

func TestGoToCtyValueMarksSensitiveFieldWithInnerType(t *testing.T) {
	secret := &registered.Secret{
		Username: "admin",
		Password: types.NewSensitive("hunter2"),
	}

	val, err := GoToCtyValue(secret)
	require.NoError(t, err)

	password := val.GetAttr("password")
	require.True(t, password.HasMark(types.SensitiveMark))

	unmarked, _ := password.Unmark()
	require.Equal(t, "hunter2", unmarked.AsString())
}

func TestGoToCtyValueLeavesPlainSiblingOfSensitiveFieldUnmarked(t *testing.T) {
	secret := &registered.Secret{
		Username: "admin",
		Password: types.NewSensitive("hunter2"),
	}

	val, err := GoToCtyValue(secret)
	require.NoError(t, err)

	username := val.GetAttr("username")
	require.False(t, username.HasMark(types.SensitiveMark))
	require.Equal(t, "admin", username.AsString())
}

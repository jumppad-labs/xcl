package xcl

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// plainCredential declares as plain values the fields a plugin credential
// declares sensitive, so converting into it must be refused.
type plainCredential struct {
	types.ResourceBase `xcl:",remain"`

	Username string `json:"username"`
	Password string `json:"password"`
	Pin      int    `json:"pin"`
}

// sensitiveCredential mirrors the plugin credential, so converting into it is
// allowed and the sensitive values survive.
type sensitiveCredential struct {
	types.ResourceBase `xcl:",remain"`

	Username string                  `json:"username"`
	Password types.Sensitive[string] `json:"password"`
	Pin      types.Sensitive[int]    `json:"pin"`
}

// setupSensitiveCredential applies a configuration declaring one plugin
// credential and returns the configuration and the raw plugin entity.
func setupSensitiveCredential(t *testing.T) (*Config, any) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "credential.xcl")
	err := os.WriteFile(path, []byte(`
resource "credential" "db" {
  username = "admin"
  password = "s3cret"
  pin      = 1234
}
`), 0o600)
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Apply(path)
	require.NoError(t, err)

	for _, entity := range c.Entities() {
		meta, err := types.GetMeta(entity)
		if err != nil {
			continue
		}

		if meta.Type == "resource" && meta.Subtype == "credential" {
			return c, entity
		}
	}

	require.FailNow(t, "the applied configuration has no credential entity")

	return nil, nil
}

func requireSensitiveFieldRefused(t *testing.T, err error) {
	t.Helper()

	require.ErrorIs(t, err, xclerrors.ErrTypeMismatch)

	var mismatch *xclerrors.TypeMismatchError
	require.ErrorAs(t, err, &mismatch)
	require.Equal(t, "password", mismatch.Field)
	require.Contains(t, err.Error(), `field "password" is sensitive`)
}

func TestFindRefusesSensitiveFieldIntoPlainType(t *testing.T) {
	c, _ := setupSensitiveCredential(t)

	found, err := Find[plainCredential](c, "resource.credential.db")
	require.Nil(t, found)
	requireSensitiveFieldRefused(t, err)
}

func TestFindByTypeRefusesSensitiveFieldIntoPlainType(t *testing.T) {
	c, _ := setupSensitiveCredential(t)

	found, err := FindByType[plainCredential](c, "resource", "credential")
	require.Nil(t, found)
	requireSensitiveFieldRefused(t, err)
}

func TestAsRefusesSensitiveFieldIntoPlainType(t *testing.T) {
	_, entity := setupSensitiveCredential(t)

	converted, err := As[plainCredential](entity)
	require.Nil(t, converted)
	requireSensitiveFieldRefused(t, err)
}

func TestFindAcceptsSensitiveFieldIntoSensitiveType(t *testing.T) {
	c, _ := setupSensitiveCredential(t)

	found, err := Find[sensitiveCredential](c, "resource.credential.db")
	require.NoError(t, err)

	require.Equal(t, "admin", found.Username)
	require.Equal(t, "s3cret", found.Password.Reveal())
	require.Equal(t, 1234, found.Pin.Reveal())
}

func TestFindByTypeAcceptsSensitiveFieldIntoSensitiveType(t *testing.T) {
	c, _ := setupSensitiveCredential(t)

	found, err := FindByType[sensitiveCredential](c, "resource", "credential")
	require.NoError(t, err)

	require.Len(t, found, 1)
	require.Equal(t, "admin", found[0].Username)
	require.Equal(t, "s3cret", found[0].Password.Reveal())
	require.Equal(t, 1234, found[0].Pin.Reveal())
}

func TestAsAcceptsSensitiveFieldIntoSensitiveType(t *testing.T) {
	_, entity := setupSensitiveCredential(t)

	converted, err := As[sensitiveCredential](entity)
	require.NoError(t, err)

	require.Equal(t, "admin", converted.Username)
	require.Equal(t, "s3cret", converted.Password.Reveal())
	require.Equal(t, 1234, converted.Pin.Reveal())
}

// All and Decode only reach types the registry holds a Go type for, and a
// plugin provided entity has none, so neither can be pointed at a plugin
// credential. They share asType with the lookups above, which is covered.

// The tests below drive sensitiveFieldBlocked directly.

type blockedLogin struct {
	User     string                  `json:"user"`
	Password types.Sensitive[string] `json:"password"`
}

type plainLogin struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func TestSensitiveFieldBlockedReportsNestedStructPath(t *testing.T) {
	type from struct {
		Login blockedLogin `json:"login"`
	}
	type to struct {
		Login plainLogin `json:"login"`
	}

	field, blocked := sensitiveFieldBlocked(reflect.TypeFor[from](), reflect.TypeFor[to]())
	require.True(t, blocked)
	require.Equal(t, "login.password", field)
}

func TestSensitiveFieldBlockedFlattensEmbeddedStructs(t *testing.T) {
	type embeddedSecret struct {
		Token types.Sensitive[string] `json:"token"`
	}
	type from struct {
		embeddedSecret
		Name string `json:"name"`
	}
	type to struct {
		Name  string `json:"name"`
		Token string `json:"token"`
	}

	field, blocked := sensitiveFieldBlocked(reflect.TypeFor[from](), reflect.TypeFor[to]())
	require.True(t, blocked)
	require.Equal(t, "token", field)
}

func TestSensitiveFieldBlockedAllowsTargetTypedAny(t *testing.T) {
	type from struct {
		Password types.Sensitive[string] `json:"password"`
	}
	type to struct {
		Password any `json:"password"`
	}

	field, blocked := sensitiveFieldBlocked(reflect.TypeFor[from](), reflect.TypeFor[to]())
	require.False(t, blocked)
	require.Empty(t, field)
}

func TestSensitiveFieldBlockedBlocksSliceOfStructsWithSensitiveElement(t *testing.T) {
	type from struct {
		Logins []blockedLogin `json:"logins"`
	}
	type to struct {
		Logins []plainLogin `json:"logins"`
	}

	field, blocked := sensitiveFieldBlocked(reflect.TypeFor[from](), reflect.TypeFor[to]())
	require.True(t, blocked)
	require.Equal(t, "logins.password", field)
}

func TestSensitiveFieldBlockedAllowsPlainToPlain(t *testing.T) {
	field, blocked := sensitiveFieldBlocked(reflect.TypeFor[plainLogin](), reflect.TypeFor[plainLogin]())
	require.False(t, blocked)
	require.Empty(t, field)
}

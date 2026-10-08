package xcl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/savedentity"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// recordingStore is a StateStore written in the test, it keeps every slice
// Save was given in order.
type recordingStore struct {
	mu    sync.Mutex
	saves [][]any
	last  []any
	saved bool
}

func (r *recordingStore) Load() ([]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.last, nil
}

func (r *recordingStore) Save(entities []any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.saves = append(r.saves, entities)
	r.last = entities
	r.saved = true

	return nil
}

func (r *recordingStore) Exists() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.saved
}

func (r *recordingStore) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.last = nil
	r.saved = false

	return nil
}

// withSecretTypes declares the Secret and SecretConsumer fixture types
func withSecretTypes() ConfigOption {
	secret := WithType(&registered.Secret{}, "resource", registered.TypeSecret)
	consumer := WithType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer)

	return func(c *Config) error {
		if err := secret(c); err != nil {
			return err
		}

		return consumer(c)
	}
}

func sensitiveBasicPath(t *testing.T) string {
	t.Helper()

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive/basic/main.xcl")
	require.NoError(t, err)

	return path
}

func TestApplySavesRealSensitiveValueToStateFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store, err := state.NewFileStateStore(t.TempDir())
	require.NoError(t, err)

	c, err := NewConfig(withSecretTypes(), WithStateStore(store))
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	contents, err := os.ReadFile(store.Path())
	require.NoError(t, err)

	require.Contains(t, string(contents), "from-literal")
	require.Contains(t, string(contents), "from-variable")
	require.NotContains(t, string(contents), "(sensitive)")
}

// A later run reads the state back through its own store and the Config's
// catalog, the same decode Destroy and Apply use, and gets the real value, not the marker.
func TestLoadedStateHoldsRealSensitiveValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	firstStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	first, err := NewConfig(withSecretTypes(), WithStateStore(firstStore))
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	secondStore, err := state.NewFileStateStore(dir)
	require.NoError(t, err)

	records, err := secondStore.Load()
	require.NoError(t, err)

	loaded, err := savedentity.DecodeAll(first.catalog, records, savedentity.ReadOptions{})
	require.NoError(t, err)

	var literal *registered.Secret
	for _, entity := range loaded {
		secret, ok := entity.(*registered.Secret)
		if ok && secret.Meta.ID == "resource.secret.literal" {
			literal = secret
		}
	}

	require.NotNil(t, literal)
	require.Equal(t, "from-literal", literal.Password.Reveal())
}

func TestCustomStateStoreReceivesRawMessagesWithRealSensitiveValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &recordingStore{}

	c, err := NewConfig(withSecretTypes(), WithStateStore(store))
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	require.NotEmpty(t, store.last)

	joined := ""
	for _, element := range store.last {
		raw, ok := element.(json.RawMessage)
		require.True(t, ok, "element is %T, want json.RawMessage", element)

		joined += string(raw)
	}

	require.Contains(t, joined, `"password":"from-literal"`)
	require.NotContains(t, joined, "(sensitive)")
}

func TestStateSavedDuringDestroyHoldsRealSensitiveValues(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &recordingStore{}

	applied, err := NewConfig(withSecretTypes(), WithStateStore(store))
	require.NoError(t, err)
	require.NoError(t, applied.Apply(sensitiveBasicPath(t)))

	savesAfterApply := len(store.saves)

	destroyer, err := NewConfig(withSecretTypes(), WithStateStore(store))
	require.NoError(t, err)
	require.NoError(t, destroyer.Destroy())

	destroySaves := store.saves[savesAfterApply:]
	require.NotEmpty(t, destroySaves)

	// every save is raw messages, and the first one still holds resources that
	// were not destroyed yet, so it must carry a real secret
	for _, save := range destroySaves {
		for _, element := range save {
			_, ok := element.(json.RawMessage)
			require.True(t, ok, "element is %T, want json.RawMessage", element)
		}
	}

	joined := ""
	for _, element := range destroySaves[0] {
		joined += string(element.(json.RawMessage))
	}

	require.Contains(t, joined, `"password":"from-`)
	require.NotContains(t, joined, "(sensitive)")
}

func TestEncodeForStateWritesRealSensitiveValue(t *testing.T) {
	secret := &registered.Secret{Username: "admin", Password: types.NewSensitive("hunter2")}

	encoded, _, err := parser.EncodeForState([]any{secret}, nil)
	require.NoError(t, err)
	require.Len(t, encoded, 1)

	raw, ok := encoded[0].(json.RawMessage)
	require.True(t, ok)

	require.Contains(t, string(raw), `"password":"hunter2"`)
}

package xcl

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/mask"
)

// stateMaskKey and otherStateMaskKey are two different 32 byte AES-256 keys
var (
	stateMaskKey      = []byte("0123456789abcdef0123456789abcdef")
	otherStateMaskKey = []byte("fedcba9876543210fedcba9876543210")
)

// newAESStateMasker returns the AES-256-GCM masker for key
func newAESStateMasker(t *testing.T, key []byte) mask.Masker {
	t.Helper()

	masker, err := mask.EncryptAES256GCM(key)
	require.NoError(t, err)

	return masker
}

// readStateDir returns the contents of every file under dir, joined. The
// file state store writes indented JSON, so a key and its value are separated
// by ": ".
func readStateDir(t *testing.T, dir string) string {
	t.Helper()

	var contents bytes.Buffer

	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		contents.Write(data)

		return nil
	})
	require.NoError(t, err)

	return contents.String()
}

// reverseStateMasker is a test-local reversible masker: it reverses the bytes
// of a value, prefixed with a marker so its output is recognisable in state.
type reverseStateMasker struct{}

const reverseMarker = "reversed:"

func (reverseStateMasker) Name() string { return "reverse" }

func (reverseStateMasker) Mask(value json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(reverseMarker + reverseText(string(value)))
}

func (reverseStateMasker) Unmask(masked json.RawMessage) (json.RawMessage, error) {
	var text string
	if err := json.Unmarshal(masked, &text); err != nil {
		return nil, err
	}

	return json.RawMessage(reverseText(text[len(reverseMarker):])), nil
}

func reverseText(text string) string {
	runes := []rune(text)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}

	return string(runes)
}

// oneWayStateMasker is a test-local masker that cannot open what it masks
type oneWayStateMasker struct{}

func (oneWayStateMasker) Name() string { return "one-way" }

func (oneWayStateMasker) Mask(json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`"gone"`), nil
}

var (
	_ mask.Reversible = reverseStateMasker{}
	_ mask.Masker     = oneWayStateMasker{}
)

func TestStateWithEncryptionMaskerHoldsNoSecret(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	c, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	contents := readStateDir(t, dir)

	require.NotEmpty(t, contents)
	require.NotContains(t, contents, "from-literal")
	require.NotContains(t, contents, "from-variable")
	require.Contains(t, contents, `"xcl_masked": "aes-256-gcm"`)
}

func TestStateWithEncryptionMaskerReloadsRealValue(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	first, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	second, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, second.Apply(sensitiveBasicPath(t)))

	secret, err := Find[registered.Secret](second, "resource.secret.literal")
	require.NoError(t, err)
	require.Equal(t, "from-literal", secret.Password.Reveal())

	fromVariable, err := Find[registered.Secret](second, "resource.secret.from_variable")
	require.NoError(t, err)
	require.Equal(t, "from-variable", fromVariable.Password.Reveal())
}

func TestStateWithDifferentKeyFailsToLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	first, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	second, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, otherStateMaskKey)),
	)
	require.NoError(t, err)

	err = second.Apply(sensitiveBasicPath(t))

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)
}

func TestEncryptedStateWithoutMaskerFailsToLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	first, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	second, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
	)
	require.NoError(t, err)

	err = second.Apply(sensitiveBasicPath(t))

	require.ErrorIs(t, err, xclerrors.ErrUnrecoverable)
}

func TestEncryptedStateDestroyWithSameKeySucceeds(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	applied, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, applied.Apply(sensitiveBasicPath(t)))

	destroyer, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)

	require.NoError(t, destroyer.Destroy())

	contents := readStateDir(t, dir)
	require.NotContains(t, contents, "from-literal")
	require.NotContains(t, contents, "from-variable")
}

func TestPlainStateLoadsWithMaskerAndIsEncryptedOnSave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	plain, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
	)
	require.NoError(t, err)
	require.NoError(t, plain.Apply(sensitiveBasicPath(t)))

	require.Contains(t, readStateDir(t, dir), "from-literal")

	masked, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, masked.Apply(sensitiveBasicPath(t)))

	contents := readStateDir(t, dir)
	require.NotContains(t, contents, "from-literal")
	require.NotContains(t, contents, "from-variable")
	require.Contains(t, contents, `"xcl_masked": "aes-256-gcm"`)

	secret, err := Find[registered.Secret](masked, "resource.secret.literal")
	require.NoError(t, err)
	require.Equal(t, "from-literal", secret.Password.Reveal())
}

func TestDestroySavesEncryptedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	store := &recordingStore{}

	applied, err := NewConfig(
		withSecretTypes(),
		WithStateStore(store),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, applied.Apply(sensitiveBasicPath(t)))

	savesAfterApply := len(store.saves)

	destroyer, err := NewConfig(
		withSecretTypes(),
		WithStateStore(store),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
	)
	require.NoError(t, err)
	require.NoError(t, destroyer.Destroy())

	destroySaves := store.saves[savesAfterApply:]
	require.NotEmpty(t, destroySaves)

	for _, save := range destroySaves {
		joined := ""
		for _, element := range save {
			raw, ok := element.(json.RawMessage)
			require.True(t, ok, "element is %T, want json.RawMessage", element)

			joined += string(raw)
		}

		require.NotContains(t, joined, "from-literal")
		require.NotContains(t, joined, "from-variable")
	}

	// the first save still holds resources not yet destroyed, so their
	// secrets must be there, encrypted
	first := ""
	for _, element := range destroySaves[0] {
		first += string(element.(json.RawMessage))
	}

	require.Contains(t, first, `"xcl_masked":"aes-256-gcm"`)
}

func TestCustomReversibleStateMaskerOutputInState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	first, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(reverseStateMasker{}),
	)
	require.NoError(t, err)
	require.NoError(t, first.Apply(sensitiveBasicPath(t)))

	contents := readStateDir(t, dir)
	require.NotContains(t, contents, "from-literal")
	require.Contains(t, contents, `"xcl_masked": "reverse"`)
	// the JSON string "from-literal" reversed, inside a JSON string
	require.Contains(t, contents, `"value": "reversed:\"laretil-morf\""`)

	second, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(reverseStateMasker{}),
	)
	require.NoError(t, err)
	require.NoError(t, second.Apply(sensitiveBasicPath(t)))

	secret, err := Find[registered.Secret](second, "resource.secret.literal")
	require.NoError(t, err)
	require.Equal(t, "from-literal", secret.Password.Reveal())
}

func TestNewConfigRejectsHashStateMasker(t *testing.T) {
	hash, err := mask.HashHMACSHA256(stateMaskKey)
	require.NoError(t, err)

	cfg, err := NewConfig(WithStateMask(hash))

	require.ErrorIs(t, err, xclerrors.ErrMaskNotReversible)
	require.EqualError(t, err, `the state masker must be reversible: "hmac-sha256" cannot recover the values it masks`)
	require.Nil(t, cfg)
}

func TestNewConfigRejectsOmitStateMasker(t *testing.T) {
	cfg, err := NewConfig(WithStateMask(mask.Omit()))

	require.ErrorIs(t, err, xclerrors.ErrMaskNotReversible)
	require.EqualError(t, err, `the state masker must be reversible: "omit" cannot recover the values it masks`)
	require.Nil(t, cfg)
}

func TestNewConfigRejectsRedactStateMasker(t *testing.T) {
	cfg, err := NewConfig(WithStateMask(mask.Redact()))

	require.ErrorIs(t, err, xclerrors.ErrMaskNotReversible)
	require.EqualError(t, err, `the state masker must be reversible: "redact" cannot recover the values it masks`)
	require.Nil(t, cfg)
}

func TestNewConfigRejectsCustomOneWayStateMasker(t *testing.T) {
	cfg, err := NewConfig(WithStateMask(oneWayStateMasker{}))

	require.ErrorIs(t, err, xclerrors.ErrMaskNotReversible)
	require.EqualError(t, err, `the state masker must be reversible: "one-way" cannot recover the values it masks`)
	require.Nil(t, cfg)
}

func TestNewConfigAcceptsEncryptionStateMasker(t *testing.T) {
	masker := newAESStateMasker(t, stateMaskKey)

	cfg, err := NewConfig(WithStateMask(masker))

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, masker, cfg.stateMask)
}

// plaintextStateWarnings returns the recorded warn-level log events carrying
// the plain text state warning for operation
func plaintextStateWarnings(recorder *eventRecorder, operation string) []Event {
	found := []Event{}
	for _, e := range recorder.Events() {
		if e.Operation != operation || e.Phase != events.PhaseLog {
			continue
		}

		if e.Meta[events.KeyLevel] != events.LevelWarn {
			continue
		}

		if e.Meta[events.KeyMessage] != parser.PlaintextStateWarning {
			continue
		}

		found = append(found, e)
	}

	return found
}

// allPlaintextStateWarnings returns every recorded event, of any operation,
// whose message is the plain text state warning
func allPlaintextStateWarnings(recorder *eventRecorder) []Event {
	found := []Event{}
	for _, e := range recorder.Events() {
		if e.Meta[events.KeyMessage] == parser.PlaintextStateWarning {
			found = append(found, e)
		}
	}

	return found
}

// withNonSensitiveTypes declares the types used by the registered/basic
// fixture, none of which has a sensitive field
func withNonSensitiveTypes() ConfigOption {
	declared := []ConfigOption{
		WithType(&registered.Database{}, "resource", registered.TypeDatabase),
		WithType(&registered.App{}, "resource", registered.TypeApp),
		WithType(&registered.Consumer{}, "resource", registered.TypeConsumer),
		WithType(&registered.Cache{}, "resource", registered.TypeCache),
	}

	return func(c *Config) error {
		for _, option := range declared {
			if err := option(c); err != nil {
				return err
			}
		}

		return nil
	}
}

func TestPlainStateWithSensitiveValueWarnsOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	recorder := &eventRecorder{}

	c, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	warnings := plaintextStateWarnings(recorder, events.OperationApply)
	require.Len(t, warnings, 1)
	require.Equal(t, events.SourceCore, warnings[0].Source)
	require.Equal(t, events.OperationApply, warnings[0].Operation)
	require.Equal(t, events.PhaseLog, warnings[0].Phase)
	require.Equal(t, events.LevelWarn, warnings[0].Meta[events.KeyLevel])
	require.Equal(t, parser.PlaintextStateWarning, warnings[0].Meta[events.KeyMessage])

	require.Len(t, allPlaintextStateWarnings(recorder), 1)

	contents := readStateDir(t, dir)
	require.Contains(t, contents, "from-literal")
	require.Contains(t, contents, "from-variable")
}

func TestPlainStateWithoutSensitiveValuesDoesNotWarn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	recorder := &eventRecorder{}

	c, err := NewConfig(
		withNonSensitiveTypes(),
		WithStatePath(dir),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	path, err := filepath.Abs("./internal/test_fixtures/config/registered/basic/main.xcl")
	require.NoError(t, err)

	require.NoError(t, c.Apply(path))

	require.NotEmpty(t, readStateDir(t, dir))
	require.Empty(t, allPlaintextStateWarnings(recorder))
}

func TestNoStateStoreDoesNotWarn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	recorder := &eventRecorder{}

	c, err := NewConfig(
		withSecretTypes(),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	require.NotEmpty(t, recorder.Events())
	require.Empty(t, allPlaintextStateWarnings(recorder))
}

func TestStateMaskerDoesNotWarn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	recorder := &eventRecorder{}

	c, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithStateMask(newAESStateMasker(t, stateMaskKey)),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)

	require.NoError(t, c.Apply(sensitiveBasicPath(t)))

	require.NotContains(t, readStateDir(t, dir), "from-literal")
	require.Empty(t, allPlaintextStateWarnings(recorder))
}

func TestDestroyWithPlainSensitiveStateWarnsOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()

	applied, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
	)
	require.NoError(t, err)
	require.NoError(t, applied.Apply(sensitiveBasicPath(t)))

	require.Contains(t, readStateDir(t, dir), "from-literal")

	recorder := &eventRecorder{}

	destroyer, err := NewConfig(
		withSecretTypes(),
		WithStatePath(dir),
		WithEventHandler(recorder.Record),
	)
	require.NoError(t, err)
	require.NoError(t, destroyer.Destroy())

	warnings := plaintextStateWarnings(recorder, events.OperationDestroy)
	require.Len(t, warnings, 1)
	require.Equal(t, events.SourceCore, warnings[0].Source)
	require.Equal(t, events.OperationDestroy, warnings[0].Operation)
	require.Equal(t, events.PhaseLog, warnings[0].Phase)
	require.Equal(t, events.LevelWarn, warnings[0].Meta[events.KeyLevel])
	require.Equal(t, parser.PlaintextStateWarning, warnings[0].Meta[events.KeyMessage])

	require.Len(t, allPlaintextStateWarnings(recorder), 1)
}

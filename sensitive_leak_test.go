package xcl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/internal/test_fixtures/registered"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/mask"
	"github.com/jumppad-labs/xcl/plugins/registry"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
	"github.com/stretchr/testify/require"
)

// knownSecret is the one value every fixture under config/sensitive_leak
// holds in a sensitive field. No output path may contain it.
const knownSecret = "s3cr3t-leak-check-7f1d"

// leakFixture is an applied sensitive_leak configuration
type leakFixture struct {
	config   *Config
	registry *registry.PluginRegistry
	plugin   *parser.TestPlugin
	store    *state.FileStateStore
	stateDir string
	recorder *eventRecorder
	err      error
}

// leakPath returns the absolute path of a fixture under config/sensitive_leak
func leakPath(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs("./internal/test_fixtures/config/sensitive_leak/" + name)
	require.NoError(t, err)

	return path
}

// leakStateMaskKey is the fixed 32 byte AES-256-GCM key the leak fixtures
// encrypt sensitive values in state with
var leakStateMaskKey = []byte("leak-suite-aes-256-gcm-key-32byt")

// applyLeakFixture applies the named fixture with the registered secret types
// and the test plugin, recording every event at the given level. Sensitive
// values in state are encrypted with AES-256-GCM under leakStateMaskKey. The
// apply error is returned in the fixture so a test can inspect a failure.
func applyLeakFixture(t *testing.T, name string, level EventDataLevel, messages ...parser.LogMessage) *leakFixture {
	t.Helper()

	return applyLeakFixtureWithOptions(t, name, level, messages)
}

// applyLeakFixtureWithOptions is applyLeakFixture with extra configuration
// options, which are added after the defaults so they can override them
func applyLeakFixtureWithOptions(t *testing.T, name string, level EventDataLevel, messages []parser.LogMessage, options ...ConfigOption) *leakFixture {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	reg := registry.NewPluginRegistry()
	require.NoError(t, reg.RegisterType(&registered.Secret{}, "resource", registered.TypeSecret))
	require.NoError(t, reg.RegisterType(&registered.SecretConsumer{}, "resource", registered.TypeSecretConsumer))

	plugin := &parser.TestPlugin{}
	plugin.SetLogOnCreate(messages...)
	require.NoError(t, reg.RegisterPlugin(plugin))

	stateDir := t.TempDir()
	store, err := state.NewFileStateStore(stateDir)
	require.NoError(t, err)

	stateMasker, err := mask.EncryptAES256GCM(leakStateMaskKey)
	require.NoError(t, err)

	recorder := &eventRecorder{}

	all := []ConfigOption{
		WithPluginRegistry(reg),
		WithStateStore(store),
		WithStateMask(stateMasker),
		WithEventHandler(recorder.handle),
		WithEventData(level),
	}
	all = append(all, options...)

	c, err := NewConfig(all...)
	require.NoError(t, err)

	return &leakFixture{
		config:   c,
		registry: reg,
		plugin:   plugin,
		store:    store,
		stateDir: stateDir,
		recorder: recorder,
		err:      c.Apply(leakPath(t, name)),
	}
}

// applyLeakMain applies the main leak fixture, which must succeed
func applyLeakMain(t *testing.T, level EventDataLevel, messages ...parser.LogMessage) *leakFixture {
	t.Helper()

	f := applyLeakFixture(t, "main.xcl", level, messages...)
	require.NoError(t, f.err)

	return f
}

// leakResources returns the applied entities that are not outputs
func leakResources(f *leakFixture) []any {
	resources := []any{}

	for _, entity := range f.config.Entities() {
		if _, ok := entity.(*types.Output); !ok {
			resources = append(resources, entity)
		}
	}

	return resources
}

// leakOutputs returns the applied output entities
func leakOutputs(f *leakFixture) []*types.Output {
	found := []*types.Output{}

	for _, entity := range f.config.Entities() {
		if output, ok := entity.(*types.Output); ok {
			found = append(found, output)
		}
	}

	return found
}

// eventText is every event's identifying fields, data, details and error text
func eventText(recorded []Event) string {
	var text strings.Builder

	for _, e := range recorded {
		fmt.Fprintf(&text, "%s %s %s %s %s\n", e.Source, e.Operation, e.Phase, e.ResourceID, e.File)
		text.Write(e.Data)
		fmt.Fprintf(&text, "\n%v\n", e.Meta)

		if e.Error != nil {
			text.WriteString(e.Error.Error())
		}
	}

	return text.String()
}

// valueFormatHandler is a slog.Handler that formats every attribute value
// with %v, the way a handler that is not text or JSON might
type valueFormatHandler struct {
	out *bytes.Buffer
}

func (h *valueFormatHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *valueFormatHandler) Handle(_ context.Context, record slog.Record) error {
	fmt.Fprintf(h.out, "%s", record.Message)

	record.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.out, " %s=%v", a.Key, a.Value.Any())
		fmt.Fprintf(h.out, " %s=%+v", a.Key, a.Value.Any())

		return true
	})

	h.out.WriteString("\n")

	return nil
}

func (h *valueFormatHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *valueFormatHandler) WithGroup(string) slog.Handler      { return h }

// leakLogMessage is a plugin log with the sensitive value and a struct
// holding it as details
func leakLogMessage() parser.LogMessage {
	return parser.LogMessage{
		Level:   "info",
		Message: "created",
		Args: []any{
			"password", types.NewSensitive(knownSecret),
			"holder", struct{ Password types.Sensitive[string] }{types.NewSensitive(knownSecret)},
		},
	}
}

func TestLeakEventsAtRawDataLevel(t *testing.T) {
	f := applyLeakMain(t, EventDataRaw)

	text := eventText(f.recorder.snapshot())

	require.NotEmpty(t, f.recorder.snapshot())
	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakEventsAtProcessedDataLevel(t *testing.T) {
	f := applyLeakMain(t, EventDataProcessed)

	text := eventText(f.recorder.snapshot())

	require.NotEmpty(t, f.recorder.snapshot())
	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakSlogBridgeTextHandler(t *testing.T) {
	out := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug}))

	f := applyLeakMain(t, EventDataProcessed, leakLogMessage())
	for _, e := range f.recorder.snapshot() {
		events.SlogHandler(logger)(e)
	}

	require.NotEmpty(t, out.String())
	require.NotContains(t, out.String(), knownSecret)
	require.Contains(t, out.String(), types.SensitiveMarker)
}

func TestLeakSlogBridgeJSONHandler(t *testing.T) {
	out := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug}))

	f := applyLeakMain(t, EventDataProcessed, leakLogMessage())
	for _, e := range f.recorder.snapshot() {
		events.SlogHandler(logger)(e)
	}

	require.NotEmpty(t, out.String())
	require.NotContains(t, out.String(), knownSecret)
	require.Contains(t, out.String(), types.SensitiveMarker)
}

func TestLeakSlogBridgeCustomHandlerFormattingWithPercentV(t *testing.T) {
	out := &bytes.Buffer{}
	logger := slog.New(&valueFormatHandler{out: out})

	f := applyLeakMain(t, EventDataProcessed, leakLogMessage())
	for _, e := range f.recorder.snapshot() {
		events.SlogHandler(logger)(e)
	}

	require.NotEmpty(t, out.String())
	require.NotContains(t, out.String(), knownSecret)
	require.Contains(t, out.String(), types.SensitiveMarker)
}

// TestLeakInProcessPluginLogDetails has the test plugin log sensitive values,
// bare and held in a struct, as details and reads the resulting log event.
// The plugin's log hooks take fixed arguments set before the apply, so a
// plugin cannot be made to log the applied entity itself, see
// TestLeakEntityFormattedAsTextForExternalPluginLogs for that.
func TestLeakInProcessPluginLogDetails(t *testing.T) {
	f := applyLeakMain(t, EventDataNone, leakLogMessage())

	logged := logEvents(f.recorder)
	require.NotEmpty(t, logged)

	text := eventText(logged)

	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

// TestLeakEntityFormattedAsTextForExternalPluginLogs formats every applied
// entity with %v, which is how an external plugin's log details cross
// (plugins/grpc_clients.go), no external plugin logs an entity itself.
func TestLeakEntityFormattedAsTextForExternalPluginLogs(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	var text strings.Builder
	for _, entity := range leakResources(f) {
		text.WriteString(fmt.Sprintf("%v\n", entity))
	}

	require.NotContains(t, text.String(), knownSecret)
	require.Contains(t, text.String(), types.SensitiveMarker)
}

func TestLeakEntitiesFormattedWithPercentV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	var text strings.Builder
	for _, entity := range leakResources(f) {
		text.WriteString(fmt.Sprintf("%v\n", entity))
	}

	require.NotContains(t, text.String(), knownSecret)
	require.Contains(t, text.String(), types.SensitiveMarker)
}

func TestLeakEntitiesFormattedWithPercentPlusV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	var text strings.Builder
	for _, entity := range leakResources(f) {
		text.WriteString(fmt.Sprintf("%+v\n", entity))
	}

	require.NotContains(t, text.String(), knownSecret)
	require.Contains(t, text.String(), types.SensitiveMarker)
}

func TestLeakEntitiesFormattedWithPercentHashV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	var text strings.Builder
	for _, entity := range leakResources(f) {
		text.WriteString(fmt.Sprintf("%#v\n", entity))
	}

	require.NotContains(t, text.String(), knownSecret)
	require.Contains(t, text.String(), types.SensitiveMarker)
}

func TestLeakFunctionErrorOnTheSecret(t *testing.T) {
	f := applyLeakFixture(t, "function_error/main.xcl", EventDataNone)

	require.Error(t, f.err)
	require.NotContains(t, f.err.Error(), knownSecret)
	require.Contains(t, f.err.Error(), types.SensitiveMarker)
}

// TestLeakValidationErrorOfSensitiveToPlainAssignment asserts the error names
// the field and does not hold the secret. No value is printed in this error,
// so the marker is not asserted.
func TestLeakValidationErrorOfSensitiveToPlainAssignment(t *testing.T) {
	f := applyLeakFixture(t, "plain_assignment/main.xcl", EventDataNone)

	require.Error(t, f.err)
	require.Contains(t, f.err.Error(), `field "note"`)
	require.NotContains(t, f.err.Error(), knownSecret)
}

func TestLeakEncodeEntityOfRegisteredSecret(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	out, err := EncodeEntity(encodeEntityByID(t, f.config, "resource.secret.a"))
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

func TestLeakEncodeEntityOfPluginCredential(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	out, err := EncodeEntity(encodeEntityByID(t, f.config, "resource.credential.b"))
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

func TestLeakEncodeEntityOfInterpolatedConsumer(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	out, err := EncodeEntity(encodeEntityByID(t, f.config, "resource.secret_consumer.interpolated"))
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

func TestLeakEncodeSavedEntityOfRegisteredSecret(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	record := encodeSavedRecordByID(t, f.store.Path(), "resource.secret.a")

	out, err := EncodeSavedEntity(f.registry, record)
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

func TestLeakEncodeSavedEntityOfPluginCredential(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	record := encodeSavedRecordByID(t, f.store.Path(), "resource.credential.b")

	out, err := EncodeSavedEntity(f.registry, record)
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

func TestLeakEncodeSavedEntityOfInterpolatedConsumer(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	record := encodeSavedRecordByID(t, f.store.Path(), "resource.secret_consumer.interpolated")

	out, err := EncodeSavedEntity(f.registry, record)
	require.NoError(t, err)

	require.NotContains(t, string(out), knownSecret)
	require.Contains(t, string(out), types.SensitiveMarker)
}

// printLeakEntities prints every applied entity in the format
func printLeakEntities(t *testing.T, f *leakFixture, format logger.PrintFormat) string {
	t.Helper()

	out := &bytes.Buffer{}
	printer := logger.NewResourcePrinter(logger.WithWriter(out), logger.WithColor(false))

	for _, entity := range f.config.Entities() {
		require.NoError(t, printer.PrintResource(entity, format))
	}

	return out.String()
}

func TestLeakPrinterTable(t *testing.T) {
	text := printLeakEntities(t, applyLeakMain(t, EventDataNone), logger.FormatTable)

	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakPrinterTree(t *testing.T) {
	text := printLeakEntities(t, applyLeakMain(t, EventDataNone), logger.FormatTree)

	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakPrinterCard(t *testing.T) {
	text := printLeakEntities(t, applyLeakMain(t, EventDataNone), logger.FormatCard)

	// the card shows no field values at all, so the marker is not asserted
	require.NotEmpty(t, text)
	require.NotContains(t, text, knownSecret)
}

func TestLeakPrinterJSON(t *testing.T) {
	text := printLeakEntities(t, applyLeakMain(t, EventDataNone), logger.FormatJSON)

	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakOutputsMapFormatted(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	outputs := f.config.Outputs()
	require.Contains(t, outputs, "output.secret")

	text := fmt.Sprintf("%v %+v %#v", outputs, outputs, outputs)

	require.NotContains(t, text, knownSecret)
	require.Contains(t, text, types.SensitiveMarker)
}

func TestLeakOutputsMapMarshalledToJSON(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	data, err := json.Marshal(f.config.Outputs())
	require.NoError(t, err)

	require.NotContains(t, string(data), knownSecret)
	require.Contains(t, string(data), types.SensitiveMarker)
}

func TestLeakOutputEntitiesMarshalledToJSON(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	outputs := leakOutputs(f)
	require.Len(t, outputs, 2)

	for _, output := range outputs {
		data, err := json.Marshal(output)
		require.NoError(t, err)

		require.NotContains(t, string(data), knownSecret)
		require.Contains(t, string(data), types.SensitiveMarker)
	}
}

func TestLeakOutputEntitiesFormattedWithPercentV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	for _, output := range leakOutputs(f) {
		text := fmt.Sprintf("%v", output)

		require.NotContains(t, text, knownSecret)
		require.Contains(t, text, types.SensitiveMarker)
	}
}

func TestLeakOutputEntitiesFormattedWithPercentPlusV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	for _, output := range leakOutputs(f) {
		text := fmt.Sprintf("%+v", output)

		require.NotContains(t, text, knownSecret)
		require.Contains(t, text, types.SensitiveMarker)
	}
}

func TestLeakOutputEntitiesFormattedWithPercentHashV(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	for _, output := range leakOutputs(f) {
		text := fmt.Sprintf("%#v", output)

		require.NotContains(t, text, knownSecret)
		require.Contains(t, text, types.SensitiveMarker)
	}
}

// TestLeakSuiteStateFileHoldsNoSecret reads every byte the state store wrote
// after applying the main leak fixture: the secret is encrypted, so only the
// masked envelope is on disk.
func TestLeakSuiteStateFileHoldsNoSecret(t *testing.T) {
	f := applyLeakMain(t, EventDataNone)

	contents := readStateDir(t, f.stateDir)

	require.NotEmpty(t, contents)
	require.NotContains(t, contents, knownSecret)
	require.Contains(t, contents, mask.EnvelopeKey)
	require.Contains(t, contents, mask.AES256GCMName)
}

package logger

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/internal/plaintype"
	"github.com/jumppad-labs/xcl/types"
)

// plainPrinterAgent is a block type with no types.ResourceBase.
type plainPrinterAgent struct {
	Models []string `xcl:"models,optional" json:"models"`
}

// newPlainPrinterEntity returns an entity of plainPrinterAgent as xcl stores
// it, in the type built from it.
func newPlainPrinterEntity(t *testing.T) any {
	t.Helper()

	built, err := plaintype.Build(reflect.TypeOf(plainPrinterAgent{}), 1)
	require.NoError(t, err)

	entity := built.New()
	meta, err := types.GetMeta(entity)
	require.NoError(t, err)
	meta.ID = "agent.claude"
	meta.Type = "agent"
	meta.Name = "claude"

	reflect.ValueOf(entity).Elem().FieldByName("Models").Set(reflect.ValueOf([]string{"opus"}))

	return entity
}

func TestPrinterTableShowsAPlainEntitysFields(t *testing.T) {
	buffer := &bytes.Buffer{}
	printer := NewResourcePrinter(WithWriter(buffer), WithColor(false), WithShowEmpty(true))

	require.NoError(t, printer.PrintResource(newPlainPrinterEntity(t), FormatTable))

	require.Contains(t, buffer.String(), "models")
	require.Contains(t, buffer.String(), "opus")
}

func TestPrinterTableLeavesOutThePlainTypeMarker(t *testing.T) {
	buffer := &bytes.Buffer{}
	printer := NewResourcePrinter(WithWriter(buffer), WithColor(false), WithShowEmpty(true))

	require.NoError(t, printer.PrintResource(newPlainPrinterEntity(t), FormatTable))

	require.NotContains(t, buffer.String(), plaintype.MarkerField)
}

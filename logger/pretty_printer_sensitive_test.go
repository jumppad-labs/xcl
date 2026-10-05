package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/types"
)

const printerSecret = "s3cr3t-printer-value"

type sensitivePrinterResource struct {
	types.ResourceBase

	ImagePullToken types.Sensitive[string]            `json:"image_pull_token"`
	Password       types.Sensitive[string]            `json:"password"`
	Tokens         []types.Sensitive[string]          `json:"tokens"`
	TokenMap       map[string]types.Sensitive[string] `json:"token_map"`
}

func newSensitivePrinterResource() *sensitivePrinterResource {
	resource := &sensitivePrinterResource{
		ImagePullToken: types.NewSensitive(printerSecret),
		Password:       types.NewSensitive(printerSecret),
		Tokens:         []types.Sensitive[string]{types.NewSensitive(printerSecret)},
		TokenMap:       map[string]types.Sensitive[string]{"key": types.NewSensitive(printerSecret)},
	}
	resource.Meta.ID = "resource.container.web"
	resource.Meta.Type = "resource"
	resource.Meta.Subtype = "container"
	resource.Meta.Name = "web"

	return resource
}

func printSensitiveResource(t *testing.T, resource any, format PrintFormat, opts ...PrinterOption) string {
	t.Helper()

	buffer := &bytes.Buffer{}
	all := append([]PrinterOption{WithWriter(buffer), WithColor(false)}, opts...)

	err := NewResourcePrinter(all...).PrintResource(resource, format)
	require.NoError(t, err)

	return buffer.String()
}

func TestPrinterTableRedactsSensitiveByDefault(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatTable)

	require.Contains(t, output, "(sensitive)")
	require.NotContains(t, output, printerSecret)
}

func TestPrinterTableRevealsSensitiveWhenAsked(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatTable, WithRevealSensitive(true))

	require.Contains(t, output, printerSecret)
}

func TestPrinterTreeRedactsSensitiveByDefault(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatTree)

	require.Contains(t, output, "(sensitive)")
	require.NotContains(t, output, printerSecret)
}

func TestPrinterTreeRevealsSensitiveWhenAsked(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatTree, WithRevealSensitive(true))

	require.Contains(t, output, printerSecret)
}

func TestPrinterCardRedactsSensitiveByDefault(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatCard)

	require.Contains(t, output, "(sensitive)")
	require.NotContains(t, output, printerSecret)
}

func TestPrinterCardRevealsSensitiveWhenAsked(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatCard, WithRevealSensitive(true))

	require.Contains(t, output, printerSecret)
}

func TestPrinterJSONRedactsSensitiveByDefault(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatJSON)

	require.Contains(t, output, `"(sensitive)"`)
	require.NotContains(t, output, printerSecret)
}

func TestPrinterJSONRevealsSensitiveWhenAsked(t *testing.T) {
	output := printSensitiveResource(t, newSensitivePrinterResource(), FormatJSON, WithRevealSensitive(true))

	require.Contains(t, output, `"s3cr3t-printer-value"`)
}

func TestPrinterRedactedValueKeepsMarkerWhenRevealing(t *testing.T) {
	resource := newSensitivePrinterResource()

	var redacted types.Sensitive[string]
	require.NoError(t, json.Unmarshal([]byte(`"(sensitive)"`), &redacted))
	resource.Password = redacted

	output := printSensitiveResource(t, resource, FormatTable, WithRevealSensitive(true))

	require.Contains(t, output, "(sensitive)")
}

func TestPrinterTableRedactsSensitiveSliceElementByDefault(t *testing.T) {
	resource := newSensitivePrinterResource()
	resource.ImagePullToken = types.Sensitive[string]{}
	resource.Password = types.Sensitive[string]{}
	resource.TokenMap = nil

	output := printSensitiveResource(t, resource, FormatTable)

	require.Contains(t, output, "(sensitive)")
	require.NotContains(t, output, printerSecret)
}

func TestPrinterTableRedactsSensitiveMapElementByDefault(t *testing.T) {
	resource := newSensitivePrinterResource()
	resource.ImagePullToken = types.Sensitive[string]{}
	resource.Password = types.Sensitive[string]{}
	resource.Tokens = nil

	output := printSensitiveResource(t, resource, FormatTable)

	require.Contains(t, output, "(sensitive)")
	require.NotContains(t, output, printerSecret)
}

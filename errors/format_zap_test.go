package errors_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// zapConfigError is a configuration error holding the unsupported argument on
// line 2 of a three-line file
func zapConfigError() *xclerrors.ConfigError {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported argument",
		Detail:   `An argument named "modls" is not expected here.`,
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 2, Column: 3, Byte: 19},
			End:      hcl.Pos{Line: 2, Column: 8, Byte: 24},
		},
	}, map[string][]byte{"main.xcl": []byte("agent \"claude\" {\n  modls = [\"opus\"]\n}\n")})

	ce := xclerrors.NewConfigError()
	ce.AppendError(d)

	return ce
}

// logWithZap logs err with zap.Error to a JSON encoder and returns the entry
func logWithZap(t *testing.T, err error) map[string]any {
	var out bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&out),
		zap.DebugLevel,
	))

	logger.Error("loading configuration", zap.Error(err))
	require.NoError(t, logger.Sync())

	entry := map[string]any{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &entry))

	return entry
}

func TestZapLogsTheReportOfAConfigErrorInErrorVerbose(t *testing.T) {
	ce := zapConfigError()

	entry := logWithZap(t, ce)

	require.Equal(t, fmt.Sprintf("%+v", ce), entry["errorVerbose"])
	require.Contains(t, entry["errorVerbose"], `     2 |   modls = ["opus"]`)
}

func TestZapLogsTheOneLineMessageOfAConfigErrorInError(t *testing.T) {
	ce := zapConfigError()

	entry := logWithZap(t, ce)

	require.Equal(t, ce.Error(), entry["error"])
}

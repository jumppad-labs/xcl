package errors

import (
	"testing"

	"github.com/stretchr/testify/require"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

func TestAppendErrorAddsError(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Boom"}, nil))

	require.Len(t, ce.Errors, 1)
}

func TestErrorReturnsOneLineForEachError(t *testing.T) {
	ce := NewConfigError()
	ce.AppendError(NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Boom"}, nil))
	ce.AppendError(NewDiagnostic(&hcl.Diagnostic{Severity: hcl.DiagError, Summary: "Bang"}, nil))

	require.Equal(t, "Boom\nBang", ce.Error())
}

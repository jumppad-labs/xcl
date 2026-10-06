package xcl

import (
	"errors"
	"fmt"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/stretchr/testify/require"
)

// The theme error is re-exported beside the encoding errors so an application
// can match a bad colour theme without importing the errors package.

func TestReExportedInvalidThemeSentinelIsTheSameValueAsTheErrorsPackage(t *testing.T) {
	require.Same(t, xclerrors.ErrInvalidTheme, ErrInvalidTheme)
}

func TestReExportedInvalidThemeErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("creating renderer: %w", &InvalidThemeError{
		Source: "themes/dark.json",
		Reason: "tokenColors is not a list",
	})

	require.ErrorIs(t, wrapped, ErrInvalidTheme)

	var detail *InvalidThemeError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "themes/dark.json", detail.Source)
	require.Equal(t, "tokenColors is not a list", detail.Reason)
}

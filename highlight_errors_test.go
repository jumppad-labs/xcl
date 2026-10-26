package xcl

import (
	"errors"
	"fmt"
	"testing"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
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

// The style error is re-exported beside the theme error so an application can
// match a misspelt style word without importing the errors package.

func TestReExportedInvalidStyleSentinelIsTheSameValueAsTheErrorsPackage(t *testing.T) {
	require.Same(t, xclerrors.ErrInvalidStyle, ErrInvalidStyle)
}

func TestReExportedInvalidStyleErrorMatchesTheSentinel(t *testing.T) {
	err := error(&InvalidStyleError{Kind: "Attribute", Style: "bold orang", Word: "orang"})

	require.ErrorIs(t, err, ErrInvalidStyle)
}

func TestReExportedInvalidStyleErrorIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("checking theme: %w", &InvalidStyleError{Kind: "Attribute", Style: "bold orang", Word: "orang"})

	var detail *InvalidStyleError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "Attribute", detail.Kind)
	require.Equal(t, "bold orang", detail.Style)
	require.Equal(t, "orang", detail.Word)
}

func TestAThemeWithAMisspeltWordFailsValidationWithTheReExportedStyleError(t *testing.T) {
	err := highlight.Theme{highlight.Attribute: "bold orang"}.Validate()

	require.ErrorIs(t, err, ErrInvalidStyle)
}

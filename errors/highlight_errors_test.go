package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// themeErrCause stands in for the read or JSON error behind a theme that
// could not be used, so a test can find it again through the detail.
var themeErrCause = errors.New("unexpected end of JSON input")

// InvalidThemeError. A bad theme is reported rather than replaced, so the
// error has to match the condition, carry the underlying cause, and name both
// the theme and what was wrong with it.

func TestInvalidThemeErrorMatchesSentinel(t *testing.T) {
	err := &InvalidThemeError{
		Source: "themes/dark.json",
		Reason: "tokenColors is not a list",
	}

	require.ErrorIs(t, err, ErrInvalidTheme)
}

func TestInvalidThemeErrorMatchesUnderlyingError(t *testing.T) {
	wrapped := fmt.Errorf("creating renderer: %w", &InvalidThemeError{
		Source: "themes/dark.json",
		Reason: "theme is not valid JSON",
		Err:    themeErrCause,
	})

	require.ErrorIs(t, wrapped, ErrInvalidTheme)
	require.ErrorIs(t, wrapped, themeErrCause)
}

func TestInvalidThemeErrorMessageNamesSourceAndReason(t *testing.T) {
	err := &InvalidThemeError{
		Source: "themes/dark.json",
		Reason: `tokenColors[3].settings.foreground "#zz0000" is not a colour`,
		Err:    errors.New("invalid hex digit"),
	}

	require.Equal(
		t,
		`theme themes/dark.json is invalid: tokenColors[3].settings.foreground "#zz0000" is not a colour: invalid hex digit`,
		err.Error(),
	)
}

func TestInvalidThemeErrorIsRecoveredWithAs(t *testing.T) {
	wrapped := fmt.Errorf("creating renderer: %w", &InvalidThemeError{
		Source: "themes/dark.json",
		Reason: "theme is not valid JSON",
		Err:    themeErrCause,
	})

	var detail *InvalidThemeError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "themes/dark.json", detail.Source)
	require.Equal(t, "theme is not valid JSON", detail.Reason)
	require.Same(t, themeErrCause, detail.Err)
}

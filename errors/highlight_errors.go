package errors

import (
	"errors"
	"fmt"
)

// ErrInvalidTheme is returned by highlight.NewANSIRenderer when the colour
// theme it is given cannot be read or is not a valid VS Code colour theme. A
// bad theme is reported rather than replaced with the default colours. It is
// matched with errors.Is, and wrapped by an InvalidThemeError detail
// recovered with errors.As.
//
// It is declared here, rather than in the highlight package that raises it,
// so the public package can re-export it beside its other errors.
var ErrInvalidTheme = errors.New("invalid theme")

// InvalidThemeError reports a theme that cannot be used. Source is the file
// path the theme was read from, or "theme" for one given as a reader. Reason
// says what was wrong and where, such as
// `tokenColors[3].settings.foreground "#zz0000" is not a colour`, and Err is
// the underlying read or JSON error, when there is one.
type InvalidThemeError struct {
	Source string
	Reason string
	Err    error
}

func (e *InvalidThemeError) Error() string {
	msg := fmt.Sprintf("theme %s is invalid", e.Source)
	if e.Reason != "" {
		msg = fmt.Sprintf("%s: %s", msg, e.Reason)
	}

	if e.Err != nil {
		msg = fmt.Sprintf("%s: %s", msg, e.Err)
	}

	return msg
}

// Unwrap returns ErrInvalidTheme and the underlying error, so both answer
// errors.Is
func (e *InvalidThemeError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrInvalidTheme}
	}

	return []error{ErrInvalidTheme, e.Err}
}

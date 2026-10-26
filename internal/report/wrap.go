package report

import "github.com/mitchellh/go-wordwrap"

// Wrap wraps detail at width columns, breaking lines between words. A width
// of 0 or less leaves detail unchanged.
func Wrap(detail string, width int) string {
	if width <= 0 {
		return detail
	}

	return wordwrap.WrapString(detail, uint(width))
}

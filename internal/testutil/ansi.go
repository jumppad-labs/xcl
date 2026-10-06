package testutil

import "regexp"

// ansiCodes matches an SGR escape sequence, the colour and style codes a
// terminal renderer writes
var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI returns text with every SGR escape sequence removed, which is the
// plain text a terminal renderer coloured
func StripANSI(text string) string {
	return ansiCodes.ReplaceAllString(text, "")
}

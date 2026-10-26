package highlight

import (
	"io"
	"os"
)

// For returns the usual format for writing to w: Terminal when w is a
// terminal, an *os.File that is a character device, and the NO_COLOR
// environment variable is unset or empty (see https://no-color.org), and
// Plain otherwise, such as for a file, a pipe or a buffer.
//
// xcl never decides on its own whether to colour output: For is how a caller
// asks it to make the usual choice. A caller that already decides colour,
// such as through its logging library, passes Terminal or Plain itself.
func For(w io.Writer) Formatter {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return Plain
	}

	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return Plain
	}

	if os.Getenv("NO_COLOR") != "" {
		return Plain
	}

	return Terminal
}

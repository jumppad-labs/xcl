package main

import (
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/highlight"
)

// inspect writes the resource at address in c to out as configuration text in
// xcl's own syntax. It shows the values the provider set, each marked, and
// every reference as it was written. The text is syntax highlighted when out
// is a terminal.
func inspect(out io.Writer, c *xcl.Config, address string) error {
	entity, err := c.FindResource(address)
	if errors.Is(err, xcl.ErrNotFound) {
		return fmt.Errorf("no resource %s in the saved state, xcl-docker status lists them", address)
	}
	if err != nil {
		return err
	}

	// xcl never checks for a terminal, asking it for colour is the program's
	// decision. It is decided for out itself, so output to a file or a pipe
	// is plain text.
	return xcl.PrintEntity(out, entity, xcl.IncludeComputed(), xcl.ShowReferences(), xcl.WithFormatter(formatterFor(out)))
}

// formatterFor returns the format to write to out in: terminal colour when
// lipgloss finds out shows colour, the same decision the example's logging
// makes, and plain text otherwise
func formatterFor(out io.Writer) highlight.Formatter {
	if lipgloss.NewRenderer(out).ColorProfile() != termenv.Ascii {
		return highlight.Terminal
	}

	return highlight.Plain
}

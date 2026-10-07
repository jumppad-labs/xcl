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

	options := []xcl.EncodeOption{xcl.IncludeComputed(), xcl.ShowReferences()}

	// xcl never checks for a terminal, asking it for colour is the program's
	// decision. It is decided for out itself, so output to a file or a pipe
	// is plain text.
	if lipgloss.NewRenderer(out).ColorProfile() != termenv.Ascii {
		renderer, err := highlight.NewANSIRenderer()
		if err != nil {
			return err
		}

		options = append(options, xcl.Highlight(renderer))
	}

	text, err := xcl.EncodeEntity(entity, options...)
	if err != nil {
		return err
	}

	_, err = out.Write(text)

	return err
}

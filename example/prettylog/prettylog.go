// Package prettylog is the event receiver the examples share. It renders
// everything xcl reports, lifecycle events, plugin log messages and errors,
// as styled terminal lines.
//
// It is an example, not part of xcl's API. It shows how little an application
// needs: xcl ships events.SlogHandler, which turns the event stream into
// log/slog records, and any slog.Handler can then format them. Here that
// handler is charmbracelet/log, chosen for its readable, coloured output.
// Only the examples import it, so it never becomes a dependency of an
// application that uses xcl.
package prettylog

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log"
	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
)

// LevelEnv is the environment variable LevelFromEnv reads the level from
const LevelEnv = "XCL_LOG_LEVEL"

// Handler returns an event handler that writes every event at level or
// above to w as a styled line, showing its severity, source, and the resource
// and step where there is one:
//
//	10:04AM INFO create start source=core operation=create phase=start resource=resource.postgres.main ...
//	10:04AM INFO created database source=ExamplePlugin operation=create phase=log resource=resource.postgres.main ...
//
// Every example sets it up in one line:
//
//	xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv()))
func Handler(w io.Writer, level slog.Level) xcl.EventHandler {
	logger := charmlog.NewWithOptions(w, charmlog.Options{
		Level:           charmlog.Level(level),
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	logger.SetStyles(styles())

	return events.SlogHandler(slog.New(logger))
}

// styles colours the keys that say where an event came from and what it is
// about, so they stand out from the details
func styles() *charmlog.Styles {
	s := charmlog.DefaultStyles()

	s.Keys["source"] = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	s.Values["source"] = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	s.Keys["resource"] = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	s.Values["resource"] = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	s.Keys["operation"] = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	s.Values["operation"] = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)

	return s
}

// LevelFromEnv returns the level named by the XCL_LOG_LEVEL environment
// variable, one of debug, info, warn or error, and info when it is unset or
// names none of them
func LevelFromEnv() slog.Level {
	switch strings.ToLower(os.Getenv(LevelEnv)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

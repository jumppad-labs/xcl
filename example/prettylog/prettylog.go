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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log"
	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/plugins/registry"
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
// When an entity is created it also writes that entity's configuration
// beneath the success line, so a reader sees what was actually made,
// including the values the provider filled in:
//
//	10:04AM INFO create success source=core operation=create resource=resource.postgres.main
//	  resource "postgres" "main" {
//	    port              = 5432
//	    connection_string = "postgres://..." # set by the provider
//	  }
//
// That needs two things from the application: the registry, which is what
// types a saved entity, and event data at xcl.EventDataProcessed, which is
// what puts the entity on the event in the first place. Pass a nil registry
// to leave the configuration out.
//
// Every example sets it up in one line:
//
//	xcl.WithEventHandler(prettylog.Handler(os.Stderr, prettylog.LevelFromEnv(), registry))
func Handler(w io.Writer, level slog.Level, reg *registry.PluginRegistry) xcl.EventHandler {
	logger := charmlog.NewWithOptions(w, charmlog.Options{
		Level:           charmlog.Level(level),
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	logger.SetStyles(styles())

	delegate := events.SlogHandler(slog.New(logger))

	// the configuration is written at the default level and below, so a run
	// asking only for warnings and errors stays terse
	if reg == nil || level > slog.LevelInfo {
		return delegate
	}

	return func(e xcl.Event) {
		delegate(e)
		writeConfiguration(w, logger, reg, e)
	}
}

// writeConfiguration writes the configuration of an entity that has just been
// created, beneath the line reporting it. An event carries the entity only
// when the application asked for xcl.EventDataProcessed, so this does nothing
// by default.
func writeConfiguration(w io.Writer, logger *charmlog.Logger, reg *registry.PluginRegistry, e xcl.Event) {
	if e.Operation != events.OperationCreate || e.Phase != events.PhaseSuccess || len(e.Data) == 0 {
		return
	}

	// the example output is for a person to read, so it shows the values the
	// provider filled in as well as the ones that were configured
	text, err := xcl.EncodeSavedEntity(reg, e.Data, xcl.IncludeComputed())
	if err != nil {
		// a variable, output or module is never written as configuration, so
		// there is simply nothing to show and nothing to report
		if errors.Is(err, xcl.ErrNotEncodable) {
			return
		}

		// anything else is a real failure, such as a type nothing registered.
		// One line and carry on, an example that cannot render one resource is
		// still worth watching
		logger.Warn("unable to show configuration", "resource", e.ResourceID, "error", err)
		return
	}

	fmt.Fprintf(w, "%s\n", indent(string(text)))
}

// indent shifts every non-empty line of text right, so the configuration sits
// visibly under the line that announced it
func indent(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}

	return strings.Join(lines, "\n")
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

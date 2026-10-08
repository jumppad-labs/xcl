// Package prettylog is the event receiver the examples share. It renders
// everything xcl reports, lifecycle events, plugin log messages and errors,
// as styled terminal lines, with the configuration of each entity it creates
// coloured by xcl's own highlight package.
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
	"github.com/muesli/termenv"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/highlight"
)

// LevelEnv is the environment variable LevelFromEnv reads the level from
const LevelEnv = "XCL_LOG_LEVEL"

// Handler returns an event handler that writes every event at level or
// above to w as a styled line, showing its severity, source, and the resource
// and step where there is one:
//
//	10:04AM INFO create start source=core operation=create phase=start resource=template.welcome ...
//	10:04AM INFO rendered template source=TemplatePlugin operation=create phase=log resource=template.welcome ...
//
// When an entity is created it also writes that entity's configuration
// beneath the success line, so a reader sees what was actually made,
// including the values the provider filled in. When w shows colour, the
// configuration is coloured by xcl itself, through xcl.Highlight and the
// highlight package's terminal renderer, the way the xcl-vscode extension
// colours it in the editor. The example has no highlighting code of its own:
//
//	10:04AM INFO create success source=core operation=create resource=template.welcome
//	  template "welcome" {
//	    source      = "Hello {{name}}"
//	    destination = "/tmp/welcome.txt"
//	  }
//
// That needs one thing from the application: event data at
// xcl.EventDataProcessed, which is what puts the entity on the event in the
// first place. The event itself returns the entity, typed by the
// configuration that delivered it, so the handler needs nothing else.
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

	delegate := events.SlogHandler(slog.New(logger))

	// the configuration is written at the default level and below, so a run
	// asking only for warnings and errors stays terse
	if level > slog.LevelInfo {
		return delegate
	}

	// the example output is for a person to read, so it shows the values the
	// provider filled in as well as the ones that were configured
	options := []xcl.EncodeOption{xcl.IncludeComputed()}

	// colour is decided for w itself, as the logger does, so output to a
	// file or a buffer is plain text. xcl never checks for a terminal, asking
	// it for colour is the application's decision.
	if lipgloss.NewRenderer(w).ColorProfile() != termenv.Ascii {
		// the default theme is built in, so creating the renderer cannot
		// fail. Were it ever to, the configuration is simply written plain.
		renderer, err := highlight.NewANSIRenderer()
		if err == nil {
			options = append(options, xcl.Highlight(renderer))
		}
	}

	return func(e xcl.Event) {
		delegate(e)
		writeConfiguration(w, logger, options, e)
	}
}

// writeConfiguration writes the configuration of an entity that has just been
// created, beneath the line reporting it. An event carries the entity only
// when the application asked for xcl.EventDataProcessed, so this does nothing
// by default.
func writeConfiguration(w io.Writer, logger *charmlog.Logger, options []xcl.EncodeOption, e xcl.Event) {
	if e.Operation != events.OperationCreate || e.Phase != events.PhaseSuccess {
		return
	}

	entity, err := e.Entity()
	if err != nil {
		logger.Warn("unable to show configuration", "resource", e.ResourceID, "error", err)
		return
	}

	// an event without data has no entity, so there is nothing to show
	if entity == nil {
		return
	}

	text, err := xcl.EncodeEntity(entity, options...)
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

	// indenting after highlighting is safe, the renderer leaves every newline
	// outside its colour codes
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

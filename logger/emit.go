package logger

import (
	"fmt"
	"maps"

	"github.com/jumppad-labs/xcl/events"
)

// badKey is the Meta key given to a trailing argument that has no value,
// matching log/slog
const badKey = "!BADKEY"

// eventLogger turns every log call into one log event, it writes nothing
// itself
type eventLogger struct {
	emit events.Emit
	base events.Event
	tags map[string]any
}

// Ensure eventLogger implements the Logger interface
var _ Logger = (*eventLogger)(nil)

// New returns a Logger that emits every message as an event with the phase
// events.PhaseLog. The event starts as a copy of base, which carries its
// source, operation and resource identity. Its Meta holds the message's
// key/value arguments under their own names, with the level under
// events.KeyLevel and the text under events.KeyMessage; an argument named
// like a reserved key never overwrites them. A nil emit gives a Logger that
// emits nothing.
func New(emit events.Emit, base events.Event) Logger {
	return &eventLogger{emit: emit, base: base}
}

// Nop returns a Logger that emits nothing
func Nop() Logger {
	return New(nil, events.Event{})
}

// Debug emits a debug log event
func (l *eventLogger) Debug(msg string, args ...any) {
	l.log(events.LevelDebug, msg, args)
}

// Info emits an info log event
func (l *eventLogger) Info(msg string, args ...any) {
	l.log(events.LevelInfo, msg, args)
}

// Warn emits a warn log event
func (l *eventLogger) Warn(msg string, args ...any) {
	l.log(events.LevelWarn, msg, args)
}

// Error emits an error log event
func (l *eventLogger) Error(msg string, args ...any) {
	l.log(events.LevelError, msg, args)
}

// log emits one log event with level and msg, the logger's tags and args as
// its details
func (l *eventLogger) log(level, msg string, args []any) {
	if l.emit == nil {
		return
	}

	meta := make(map[string]any, len(l.tags)+len(args)/2+2)
	maps.Copy(meta, l.tags)

	for i := 0; i < len(args); i += 2 {
		if i+1 == len(args) {
			meta[badKey] = args[i]
			break
		}

		meta[argKey(args[i])] = args[i+1]
	}

	// the reserved keys are written last so a caller's details never
	// overwrite them
	meta[events.KeyLevel] = level
	meta[events.KeyMessage] = msg

	e := l.base
	e.Phase = events.PhaseLog
	e.Meta = meta

	l.emit(e)
}

// withTag returns a copy of l with the detail key=value added to every event,
// the key "resource" sets the event's ResourceID instead
func (l *eventLogger) withTag(key string, value any) *eventLogger {
	tagged := &eventLogger{emit: l.emit, base: l.base, tags: maps.Clone(l.tags)}

	// an empty key only copies the logger
	if key == "" {
		return tagged
	}

	if key == "resource" {
		tagged.base.ResourceID = fmt.Sprint(value)
		return tagged
	}

	if tagged.tags == nil {
		tagged.tags = map[string]any{}
	}

	tagged.tags[key] = value

	return tagged
}

// argKey is the Meta key for a log argument in a key position
func argKey(key any) string {
	if s, ok := key.(string); ok {
		return s
	}

	return fmt.Sprint(key)
}

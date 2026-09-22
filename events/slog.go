package events

import (
	"context"
	"log/slog"
	"sort"
)

// SlogHandler returns a Handler that writes every event to logger. Log events
// are written at their own level with their message, lifecycle, discovery and
// load events are written at info, or error for the error phase, and blocked
// announcements at warn. Filtering by severity is the level of logger's
// handler, so an "info" logger writes no debug log events:
//
//	xcl.WithEventHandler(events.SlogHandler(slog.Default()))
//
// The event's fields are written as the attributes source, operation, phase,
// resource, type, file, duration and error, when set, followed by its Meta
// details in key order. Data is not written.
func SlogHandler(logger *slog.Logger) Handler {
	return func(e Event) {
		ctx := context.Background()
		handler := logger.Handler()

		level := slogLevel(e)
		if !handler.Enabled(ctx, level) {
			return
		}

		record := slog.NewRecord(e.Time, level, slogMessage(e), 0)
		record.AddAttrs(slogAttrs(e)...)

		// a receiver cannot return an error, and a failed write must not
		// affect processing
		_ = handler.Handle(ctx, record)
	}
}

// slogLevel is the level e is written at
func slogLevel(e Event) slog.Level {
	switch e.Phase {
	case PhaseLog:
		level, _ := e.Meta[KeyLevel].(string)
		switch level {
		case LevelDebug:
			return slog.LevelDebug
		case LevelWarn:
			return slog.LevelWarn
		case LevelError:
			return slog.LevelError
		default:
			return slog.LevelInfo
		}
	case PhaseError:
		return slog.LevelError
	case PhaseBlocked:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}

// slogMessage is the message e is written with, the log message for a log
// event, otherwise "<operation> <phase>"
func slogMessage(e Event) string {
	if e.Phase == PhaseLog {
		message, _ := e.Meta[KeyMessage].(string)
		return message
	}

	return e.Operation + " " + e.Phase
}

// slogAttrs are the attributes e is written with
func slogAttrs(e Event) []slog.Attr {
	attrs := []slog.Attr{}

	addString := func(key, value string) {
		if value != "" {
			attrs = append(attrs, slog.String(key, value))
		}
	}

	addString("source", e.Source)
	addString("operation", e.Operation)
	addString("phase", e.Phase)
	addString("resource", e.ResourceID)
	addString("type", e.ResourceType)
	addString("file", e.File)

	if e.Duration != 0 {
		attrs = append(attrs, slog.Duration("duration", e.Duration))
	}

	if e.Error != nil {
		attrs = append(attrs, slog.String("error", e.Error.Error()))
	}

	keys := make([]string, 0, len(e.Meta))
	for key := range e.Meta {
		if e.Phase == PhaseLog && (key == KeyLevel || key == KeyMessage) {
			continue
		}

		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		attrs = append(attrs, slog.Any(key, e.Meta[key]))
	}

	return attrs
}

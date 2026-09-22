package plugins

import (
	"context"

	"github.com/jumppad-labs/xcl/logger"
)

// loggerKey is the context key the per-call logger is stored under
type loggerKey struct{}

// Logger returns the logger for the provider call ctx belongs to. xcl binds
// it to the resource being worked on, the lifecycle step and the plugin, so a
// provider logs with that context without passing any of it:
//
//	plugins.Logger(ctx).Info("created", "remote_id", id)
//
// Every message becomes an event on the application's event stream. When ctx
// carries no logger, i.e. outside a provider call, the returned logger emits
// nothing.
func Logger(ctx context.Context) logger.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey{}).(logger.Logger); ok && l != nil {
			return l
		}
	}

	return logger.Nop()
}

// WithLogger returns a copy of ctx carrying l as the logger for a provider
// call, it is used by xcl before calling a provider
func WithLogger(ctx context.Context, l logger.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

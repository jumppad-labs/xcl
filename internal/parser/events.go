package parser

import (
	"context"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/types"
)

// emit sends e to the options' emitter, it does nothing when there is none
func emit(options *ParserOptions, e events.Event) {
	if options == nil || options.Emit == nil {
		return
	}

	options.Emit(e)
}

// emitting returns true when options has an emitter, so callers can skip
// building an event nobody receives
func emitting(options *ParserOptions) bool {
	return options != nil && options.Emit != nil
}

// lifecycleEvent builds an event for a step of a resource's lifecycle, with
// the resource's type, ID and file taken from its meta
func lifecycleEvent(meta *types.Meta, operation, phase string, duration time.Duration, err error, data []byte) events.Event {
	return events.Event{
		Source:       events.SourceCore,
		Operation:    operation,
		Phase:        phase,
		ResourceType: resourceType(meta),
		ResourceID:   meta.ID,
		File:         meta.File,
		Duration:     duration,
		Error:        err,
		Data:         data,
	}
}

// emitLifecycle emits a lifecycle event for the resource described by meta
func emitLifecycle(options *ParserOptions, meta *types.Meta, operation, phase string, duration time.Duration, err error, data []byte) {
	if !emitting(options) {
		return
	}

	emit(options, lifecycleEvent(meta, operation, phase, duration, err, data))
}

// emitParse emits a parse event for a block read from file, a success when
// err is nil, otherwise an error. resourceType and resourceID are empty when
// the problem can not be tied to a resource, i.e. a file that is not valid
// syntax.
func emitParse(options *ParserOptions, resourceType, resourceID, file string, err error) {
	if !emitting(options) {
		return
	}

	phase := events.PhaseSuccess
	if err != nil {
		phase = events.PhaseError
	}

	emit(options, events.Event{
		Source:       events.SourceCore,
		Operation:    events.OperationParse,
		Phase:        phase,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		File:         file,
		Error:        err,
	})
}

// emitOperationError emits an error event for a failure of the operation as a
// whole, one that belongs to no single resource
func emitOperationError(options *ParserOptions, operation string, err error) {
	if !emitting(options) {
		return
	}

	emit(options, events.Event{
		Source:    events.SourceCore,
		Operation: operation,
		Phase:     events.PhaseError,
		Error:     err,
	})
}

// providerContext returns the context a provider call for operation on the
// resource described by meta is made with. It is never cancelled, so a call
// already running always finishes, and it carries a logger bound to the
// resource and the step, whose messages are emitted as log events. The
// plugin's host names the plugin as the logger's source.
func providerContext(ctx context.Context, options *ParserOptions, meta *types.Meta, operation string) context.Context {
	callCtx := context.WithoutCancel(ctx)
	if !emitting(options) {
		return callCtx
	}

	return plugins.WithLogger(callCtx, logger.New(options.Emit, lifecycleEvent(meta, operation, events.PhaseLog, 0, nil, nil)))
}

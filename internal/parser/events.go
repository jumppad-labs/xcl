package parser

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/mask"
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

// eventData decides what a lifecycle event carries, from the level the
// configuration asked for and the phase the event sits at. It is the one
// place that decision is made, so an emission site passes what it holds and
// never chooses.
//
// pre is the resource as it was before the provider was called, where the
// caller already had to serialize it, and r is the resource itself. At
// DataNone nothing is serialized at all, so the default costs no work.
//
// Event data is shown to whoever receives events, so every sensitive value in
// it is written through the configured event masker, options.EventMask. A nil
// event masker carries real values, which only happens when event masking was
// turned off. pre holds real values, since it is what a provider receives, so
// it is never passed on as it is when it holds a sensitive value and a masker
// is set.
func eventData(options *ParserOptions, phase string, pre []byte, r any) []byte {
	if options == nil {
		return nil
	}

	marshal := func() []byte {
		if r == nil {
			return nil
		}

		result, err := wire.Encode(r, wire.Options{Mask: options.EventMask})
		if err != nil {
			// the event carries nothing rather than what reached a
			// provider, which holds real sensitive values
			return nil
		}

		return result.Data
	}

	switch options.EventData {
	case events.DataRaw:
		if pre != nil {
			return maskedSnapshot(pre, r, options.EventMask)
		}

		// a type handled without a provider was never serialized, so the raw
		// resource is the resource itself
		return marshal()

	case events.DataProcessed:
		// only a success has a result to report, every other phase runs
		// before the provider returned
		if phase == events.PhaseSuccess {
			return marshal()
		}

		if pre != nil {
			return maskedSnapshot(pre, r, options.EventMask)
		}

		return marshal()
	}

	return nil
}

// maskedSnapshot returns the pre-call snapshot pre as event data. pre was
// written with real sensitive values, so when eventMask is set it is read back
// into a value of r's type and encoded again with the masker. A snapshot that
// holds no sensitive value, or one shown with masking turned off, is returned
// as it is, so event data for it is unchanged. Without r's type the sensitive
// values in pre cannot be found, so the event then carries nothing.
func maskedSnapshot(pre []byte, r any, eventMask mask.Masker) []byte {
	if eventMask == nil {
		return pre
	}

	if r == nil {
		return nil
	}

	resourceType := reflect.TypeOf(r)
	if resourceType.Kind() != reflect.Pointer {
		return nil
	}

	snapshot := reflect.New(resourceType.Elem())
	if err := json.Unmarshal(pre, snapshot.Interface()); err != nil {
		return nil
	}

	result, err := wire.Encode(snapshot.Interface(), wire.Options{Mask: eventMask})
	if err != nil {
		return nil
	}

	if !result.Sensitive {
		return pre
	}

	return result.Data
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

// emitLifecycle emits a lifecycle event for the resource described by meta.
//
// pre is the resource as it was before the provider was called, where the
// caller already holds it, and r is the resource. What the event carries is
// decided by eventData from the configured level, not by the caller.
func emitLifecycle(options *ParserOptions, meta *types.Meta, operation, phase string, duration time.Duration, err error, pre []byte, r any) {
	if !emitting(options) {
		return
	}

	emit(options, lifecycleEvent(meta, operation, phase, duration, err, eventData(options, phase, pre, r)))
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

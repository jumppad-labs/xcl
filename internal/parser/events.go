package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/wire"
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

// eventData decides what a lifecycle event carries, from the level the
// configuration asked for and the phase the event sits at. It is the one
// place that decision is made, so an emission site passes what it holds and
// never chooses.
//
// pre is the resource as it was before the provider was called, where the
// caller already had to serialize it, and r is the resource itself. At
// DataNone nothing is serialized at all, so the default costs no work.
//
// Event data is shown to whoever receives events, so it is always encoded
// with encoding/json and every sensitive value shows types.SensitiveMarker.
// pre holds real values, since it is what a provider receives, so it is
// never passed on as it is when it holds a sensitive value.
func eventData(options *ParserOptions, phase string, pre []byte, r any) []byte {
	if options == nil {
		return nil
	}

	marshal := func() []byte {
		if r == nil {
			return nil
		}

		data, err := json.Marshal(r)
		if err != nil {
			// the event carries nothing rather than what reached a
			// provider, which holds real sensitive values
			return nil
		}

		return data
	}

	switch options.EventData {
	case events.DataRaw:
		if pre != nil {
			return redactedSnapshot(pre, r)
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
			return redactedSnapshot(pre, r)
		}

		return marshal()
	}

	return nil
}

// redactedSnapshot returns the pre-call snapshot pre as event data. pre was
// written with real sensitive values, so it is read back into a value of r's
// type and encoded again with encoding/json, which shows each sensitive value
// as the marker. A snapshot that holds no sensitive value is returned as it
// is, so event data for such an entity is unchanged.
func redactedSnapshot(pre []byte, r any) []byte {
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

	redacted, err := json.Marshal(snapshot.Interface())
	if err != nil {
		return nil
	}

	revealed, err := wire.Marshal(snapshot.Interface())
	if err != nil {
		return nil
	}

	if bytes.Equal(redacted, revealed) {
		return pre
	}

	return redacted
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

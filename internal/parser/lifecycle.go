package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/resources"
	"github.com/jumppad-labs/xcl/internal/wire"
	"github.com/jumppad-labs/xcl/internal/xcl/hclsyntax"
	"github.com/jumppad-labs/xcl/logger"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/state"
	"github.com/jumppad-labs/xcl/types"
)

// resourceLifecycle decides and runs the provider calls for each decoded
// resource. One is built per walk and shared by every walk callback goroutine,
// it holds no per-resource state of its own.
type resourceLifecycle struct {
	// ctx is the operation's context, once it is cancelled no new provider
	// call starts
	ctx context.Context

	// previous is the state saved by the last apply, it is never nil
	previous *State
	resolver ProviderResolver
	options  *ParserOptions

	// types reports the registered types, which like builtins have no provider
	types TypeRegistry

	// bodies are the HCL bodies of the parsed resources keyed by ID
	bodies map[string]*hclsyntax.Body

	// progress records the outcome of every resource the lifecycle handles
	progress *applyProgress

	// mode is what the walk does with each resource, apply unless set
	mode walkMode

	// recorder collects what an apply would do in a diff walk, nil in apply
	recorder *diffRecorder

	// diffOptions are the settings of a diff walk
	diffOptions diff.Options
}

// operation is the core operation the lifecycle's walk is part of
func (l *resourceLifecycle) operation() string {
	if l.mode == walkDiff {
		return events.OperationDiff
	}

	return events.OperationApply
}

// apply runs the provider lifecycle for a decoded resource. The path is chosen
// from the resource's entry in the previous state:
//
//   - absent: the resource is created
//   - created or updated: the resource is read, then updated if it changed
//   - failed, destroy_failed or anything else: the resource is rebuilt, it is
//     destroyed using its saved copy, then created
//
// The outcome is recorded in the apply progress. A resource whose provider call
// failed is recorded as failed, an error before any provider call records
// nothing, so the resource counts as not reached.
func (l *resourceLifecycle) apply(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	err = l.run(r)
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		if meta.Status == types.StatusFailed || meta.Status == types.StatusDestroyFailed {
			l.progress.record(meta.ID, outcome{failed: true, saved: r})
		}

		// a failure outside a provider call, such as a resource that does
		// not serialize, has no event yet
		if !reported(err) {
			emitWalkError(l.options, l.operation(), meta, err)
		}

		return err
	}

	l.progress.record(meta.ID, outcome{saved: r})
	return nil
}

// run chooses and runs the provider calls for a resource
func (l *resourceLifecycle) run(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	// builtin and registered resource types have no provider, they always succeed
	if handledWithoutProvider(l.types, meta) {
		emitLifecycle(l.options, meta, events.OperationCreate, events.PhaseSuccess, 0, nil, nil, r)
		return nil
	}

	adapter := l.resolver.GetProviderForResource(r)
	if adapter == nil {
		err := fmt.Errorf("no provider found for resource type %s", meta.AddressType())
		emitLifecycle(l.options, meta, l.firstStep(meta), events.PhaseError, 0, err, nil, r)
		return reportedError{err}
	}

	old, err := findByID(l.previous.GetResources(), meta.ID)
	if err != nil {
		var notFound state.ResourceNotFoundError
		if errors.As(err, &notFound) {
			return l.create(r, adapter)
		}

		return fmt.Errorf("unable to find resource %s in previous state: %w", meta.ID, err)
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return err
	}

	switch oldMeta.Status {
	case types.StatusCreated, types.StatusUpdated:
		return l.read(r, old, adapter)
	default:
		return l.rebuild(r, old, adapter)
	}
}

// diff records what an apply would do with a decoded resource, choosing as
// run does from the resource's entry in the previous state, without creating,
// updating or destroying anything:
//
//   - absent: the resource would be created
//   - created or updated: the resource is refreshed through its provider, it
//     would be created again when the provider no longer finds it, updated
//     when it changed, and is otherwise unchanged
//   - failed, destroy_failed or anything else: the resource would be
//     replaced, no provider is called
//
// Resources without a provider take no part in a diff: they are neither
// recorded nor reported in an event.
func (l *resourceLifecycle) diff(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	err = l.diffResource(r, meta)
	if errors.Is(err, errNotReached) {
		return err
	}

	// a failure outside a provider call has no event yet
	if err != nil && !reported(err) {
		emitWalkError(l.options, l.operation(), meta, err)
	}

	return err
}

// diffResource chooses and records the action an apply would take for a
// resource
func (l *resourceLifecycle) diffResource(r any, meta *types.Meta) error {
	if handledWithoutProvider(l.types, meta) {
		return nil
	}

	adapter := l.resolver.GetProviderForResource(r)
	if adapter == nil {
		err := fmt.Errorf("no provider found for resource type %s", meta.AddressType())
		emitLifecycle(l.options, meta, l.firstStep(meta), events.PhaseError, 0, err, nil, r)
		return reportedError{err}
	}

	old, err := findByID(l.previous.GetResources(), meta.ID)
	if err != nil {
		var notFound state.ResourceNotFoundError
		if errors.As(err, &notFound) {
			l.recordPending(meta, diff.ActionCreate, nil, r)
			return nil
		}

		return fmt.Errorf("unable to find resource %s in previous state: %w", meta.ID, err)
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return err
	}

	switch oldMeta.Status {
	case types.StatusCreated, types.StatusUpdated:
	default:
		l.recordPending(meta, diff.ActionReplace, old, r)
		return nil
	}

	// a resource that depends on a value only known once an apply has run
	// is not read: its provider would be asked about a half-resolved
	// resource. It is reported as updated, its unknown values say why
	if len(l.recorder.unknownPaths(meta.ID)) > 0 {
		l.recordPending(meta, diff.ActionUpdate, old, r)
		return nil
	}

	outcome, copies, err := l.refresh(r, old, adapter)
	if err != nil {
		return err
	}

	switch outcome {
	case refreshNotFound:
		// refresh restored the configured values
		l.recordPending(meta, diff.ActionCreate, nil, r)
	case refreshChanged:
		// the configuration as written is compared, not what the provider
		// read back, so drift alone lists no changes
		configured, err := decodeCopy(r, copies.configured)
		if err != nil {
			return err
		}

		l.recordPending(meta, diff.ActionUpdate, old, configured)
	default:
		l.recorder.recordUnchanged()
	}

	return nil
}

// recordPending records a resource an apply would create, replace or update.
// Its computed values are unknown until the apply runs: nothing says which
// computed values an update leaves alone, so every one of them is assumed to
// change, and the resources that use them are reported as updated too. The
// diff may list a resource an apply then leaves alone, never the other way
// round.
func (l *resourceLifecycle) recordPending(meta *types.Meta, action diff.Action, saved, configured any) {
	l.recorder.markPending(meta.ID)
	l.recorder.record(diff.Resource{
		Address: meta.ID,
		Action:  action,
		Changes: l.changes(meta, action, saved, configured),
	})
}

// changes returns the changes to a resource's configured values an apply
// taking action would make
func (l *resourceLifecycle) changes(meta *types.Meta, action diff.Action, saved, configured any) []diff.Change {
	return resourceChanges(action, saved, configured, l.bodies[meta.ID], l.recorder.unknownPaths(meta.ID), l.diffOptions.RevealSensitive)
}

// decodeCopy returns a new resource of the same type as r holding the
// serialized copy data
func decodeCopy(r any, data []byte) (any, error) {
	copied := reflect.New(reflect.TypeOf(r).Elem())
	if err := json.Unmarshal(data, copied.Interface()); err != nil {
		return nil, fmt.Errorf("unable to decode configured copy: %w", err)
	}

	return copied.Interface(), nil
}

// create calls the provider's Create for a resource
func (l *resourceLifecycle) create(r any, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	data, err := wire.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	duration, err := l.callProvider(events.OperationCreate, r, data, func(ctx context.Context) ([]byte, error) {
		return adapter.Create(ctx, data)
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	l.warnChangedConfiguredValues(events.OperationCreate, r, data)

	meta.Status = types.StatusCreated

	// the success is emitted after the status is set, so that processed event
	// data is the resource exactly as state is about to record it
	emitLifecycle(l.options, meta, events.OperationCreate, events.PhaseSuccess, duration, nil, data, r)

	return nil
}

// refreshOutcome is what a refresh found out about a resource in the previous
// state
type refreshOutcome int

const (
	// refreshNotFound is a resource the provider no longer finds, its
	// configured values have been restored
	refreshNotFound refreshOutcome = iota

	// refreshChanged is a resource the provider reports as changed
	refreshChanged

	// refreshUnchanged is a resource the provider reports as unchanged
	refreshUnchanged
)

// refreshed holds the serialized copies a refresh worked from
type refreshed struct {
	// old is the copy saved by the last apply
	old []byte

	// configured is the resource as configured, before the saved computed
	// values were carried onto it and before the provider read it
	configured []byte

	// read is the resource as the provider read it, empty when it was not
	// found
	read []byte
}

// refresh reads a resource that exists in the previous state through its
// provider and asks whether it changed. The computed values saved last time
// are carried onto the configured resource, the provider reads the real
// resource, and change detection compares the saved copy with what was read.
// It never creates or updates anything: apply and diff each act on the
// outcome.
//
// A resource the provider no longer finds has its configured values restored,
// since its computed values went with the real resource. A provider error
// marks the resource failed.
func (l *resourceLifecycle) refresh(r any, old any, adapter plugins.ProviderAdapter) (refreshOutcome, refreshed, error) {
	copies := refreshed{}

	meta, err := types.GetMeta(r)
	if err != nil {
		return refreshUnchanged, copies, err
	}

	// the previous resource is never modified, it is only serialized
	copies.old, err = wire.Marshal(old)
	if err != nil {
		return refreshUnchanged, copies, fmt.Errorf("unable to serialize previous resource %s: %w", meta.ID, err)
	}

	copies.configured, err = wire.Marshal(r)
	if err != nil {
		return refreshUnchanged, copies, fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	// computed values are carried over before the read, so they survive even
	// when the provider's Read adds nothing, and change detection sees the same
	// computed values on both copies
	if err := carryComputedValues(r, copies.old); err != nil {
		return refreshUnchanged, copies, err
	}

	newData, err := wire.Marshal(r)
	if err != nil {
		return refreshUnchanged, copies, fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	duration, err := l.callProvider(events.OperationRead, r, newData, func(ctx context.Context) ([]byte, error) {
		return adapter.Read(ctx, copies.old, newData)
	})
	if errors.Is(err, plugins.ErrNotFound) {
		// the real resource is gone, its computed values went with it
		if err := replaceValues(r, copies.configured); err != nil {
			return refreshNotFound, copies, err
		}

		return refreshNotFound, copies, nil
	}

	if errors.Is(err, errNotReached) {
		return refreshUnchanged, copies, err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return refreshUnchanged, copies, err
	}

	emitLifecycle(l.options, meta, events.OperationRead, events.PhaseSuccess, duration, nil, newData, r)

	copies.read, err = wire.Marshal(r)
	if err != nil {
		return refreshUnchanged, copies, fmt.Errorf("unable to serialize read resource %s: %w", meta.ID, err)
	}

	l.warnChangedConfiguredValues(events.OperationRead, r, newData)

	changed := false
	duration, err = l.callProvider(events.OperationChanged, r, copies.read, func(ctx context.Context) ([]byte, error) {
		var changedErr error
		changed, changedErr = adapter.Changed(ctx, copies.old, copies.read)
		return nil, changedErr
	})
	if errors.Is(err, errNotReached) {
		return refreshUnchanged, copies, err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return refreshUnchanged, copies, err
	}

	emitLifecycle(l.options, meta, events.OperationChanged, events.PhaseSuccess, duration, nil, copies.read, r)

	if changed {
		return refreshChanged, copies, nil
	}

	return refreshUnchanged, copies, nil
}

// read handles a resource that exists in the previous state. It is refreshed
// through its provider and updated only when it changed. A resource the
// provider no longer finds is created again from its configuration.
func (l *resourceLifecycle) read(r any, old any, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return err
	}

	outcome, copies, err := l.refresh(r, old, adapter)
	if err != nil {
		return err
	}

	switch outcome {
	case refreshNotFound:
		return l.create(r, adapter)
	case refreshUnchanged:
		// an unchanged resource keeps what was read and its previous status
		meta.Status = oldMeta.Status
		return nil
	}

	duration, err := l.callProvider(events.OperationUpdate, r, copies.read, func(ctx context.Context) ([]byte, error) {
		return adapter.Update(ctx, copies.read)
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	l.warnChangedConfiguredValues(events.OperationUpdate, r, copies.read)

	meta.Status = types.StatusUpdated

	emitLifecycle(l.options, meta, events.OperationUpdate, events.PhaseSuccess, duration, nil, copies.read, r)

	return nil
}

// rebuild handles a resource saved as failed or destroy_failed. The resource is
// destroyed using its saved copy, then created again, whether or not its
// configuration changed. When the destroy fails the resource keeps the saved
// copy, which holds the identity needed to try the destroy again, and is marked
// destroy_failed.
func (l *resourceLifecycle) rebuild(r any, old any, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	// the previous resource is never modified, it is only serialized
	oldData, err := wire.Marshal(old)
	if err != nil {
		return fmt.Errorf("unable to serialize previous resource %s: %w", meta.ID, err)
	}

	duration, err := l.callProvider(events.OperationDestroy, r, oldData, func(ctx context.Context) ([]byte, error) {
		return nil, adapter.Destroy(ctx, oldData, false)
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		// keep the saved copy, it holds the identity needed to destroy it later
		if keepErr := replaceValues(r, oldData); keepErr != nil {
			return errors.Join(err, keepErr)
		}

		meta.Status = types.StatusDestroyFailed
		return err
	}

	// the destroyed copy is what this step acted on, the create that follows
	// reports the resource it builds in its place
	emitLifecycle(l.options, meta, events.OperationDestroy, events.PhaseSuccess, duration, nil, oldData, old)

	return l.create(r, adapter)
}

// replaceValues replaces the values of a resource with the given serialized copy.
// The resource keeps its own metadata, which describes where it is configured.
func replaceValues(r any, data []byte) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	configuredMeta := *meta

	value := reflect.ValueOf(r).Elem()
	value.Set(reflect.Zero(value.Type()))

	if err := json.Unmarshal(data, r); err != nil {
		return fmt.Errorf("unable to restore values of %s: %w", configuredMeta.ID, err)
	}

	meta, err = types.GetMeta(r)
	if err != nil {
		return err
	}

	*meta = configuredMeta
	return nil
}

// carryComputedValues copies the computed values of the saved copy onto the
// resource, at any depth
func carryComputedValues(r any, savedData []byte) error {
	resource := reflect.ValueOf(r).Elem()

	saved := reflect.New(resource.Type())
	if err := json.Unmarshal(savedData, saved.Interface()); err != nil {
		return fmt.Errorf("unable to read saved computed values: %w", err)
	}

	copyComputed(resource, saved.Elem())
	return nil
}

// warnChangedConfiguredValues emits a warn log event for every configured value
// the provider changed during operation, bound to the resource and the step.
// before is the resource as sent to the provider, the resource now holds what
// it returned.
func (l *resourceLifecycle) warnChangedConfiguredValues(operation string, r any, before []byte) {
	if !emitting(l.options) {
		return
	}

	meta, err := types.GetMeta(r)
	if err != nil {
		return
	}

	after, err := wire.Marshal(r)
	if err != nil {
		return
	}

	log := logger.New(l.options.Emit, lifecycleEvent(meta, operation, "", 0, nil, nil))
	warnChangedConfiguredValues(log, meta.ID, l.bodies[meta.ID], reflect.TypeOf(r), before, after)
}

// callProvider wraps a single provider call. It fires the start event, times the
// call, fires the error event and, on success, decodes a non-empty result into
// the resource. data is the serialized resource sent with the events.
//
// It does not fire the success event. The caller does that once it has set the
// resource's final status, so that processed event data carries the same status
// the resource is recorded in state with. The call's duration is returned for
// the caller to report.
//
// No call starts once the operation's context is cancelled, errNotReached is
// returned instead and no event is fired. The call is given a copy of the
// context that is never cancelled, so a call already running always finishes,
// carrying a logger bound to the resource and the step, see plugins.Logger.
func (l *resourceLifecycle) callProvider(operation string, r any, data []byte, call func(ctx context.Context) ([]byte, error)) (time.Duration, error) {
	meta, err := types.GetMeta(r)
	if err != nil {
		return 0, err
	}

	if l.ctx.Err() != nil {
		return 0, errNotReached
	}

	id := meta.ID

	emitLifecycle(l.options, meta, operation, events.PhaseStart, 0, nil, data, r)
	start := time.Now()
	result, err := call(providerContext(l.ctx, l.options, meta, operation))
	duration := time.Since(start)

	if err != nil {
		emitLifecycle(l.options, meta, operation, events.PhaseError, duration, err, data, r)
		return duration, reportedError{fmt.Errorf("%s failed for %s: %w", operation, id, err)}
	}

	if len(result) > 0 {
		if err := json.Unmarshal(result, r); err != nil {
			decodeErr := fmt.Errorf("unable to decode %s result: %w", operation, err)
			emitLifecycle(l.options, meta, operation, events.PhaseError, duration, decodeErr, data, r)
			return duration, reportedError{fmt.Errorf("%s failed for %s: %w", operation, id, decodeErr)}
		}
	}

	return duration, nil
}

// firstStep is the provider step that would run first for a resource: create
// when it has no entry in the previous state, read when it was created or
// updated, and destroy for a rebuild
func (l *resourceLifecycle) firstStep(meta *types.Meta) string {
	old, err := findByID(l.previous.GetResources(), meta.ID)
	if err != nil {
		return events.OperationCreate
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return events.OperationCreate
	}

	switch oldMeta.Status {
	case types.StatusCreated, types.StatusUpdated:
		return events.OperationRead
	default:
		return events.OperationDestroy
	}
}

// handledWithoutProvider returns true for resource types that have no
// provider: the builtin types xcl handles itself, and the plain Go types
// registered without a plugin. typeRegistry may be nil, in which case only
// builtin types are handled without a provider.
// It takes the whole meta rather than a single name because a registered Go
// type is registered under its type and subtype together.
func handledWithoutProvider(typeRegistry TypeRegistry, meta *types.Meta) bool {
	if meta.Type == resources.TypeVariable ||
		meta.Type == resources.TypeOutput ||
		meta.Type == resources.TypeModule ||
		meta.Type == resources.TypeRoot {
		return true
	}

	// a registered Go type is registered under its type and subtype
	return typeRegistry != nil && typeRegistry.IsRegisteredType(meta.Type, meta.Subtype)
}

// resourceType returns the "<type>.<name>" form used in parser events
func resourceType(meta *types.Meta) string {
	return fmt.Sprintf("%s.%s", meta.AddressType(), meta.Name)
}

package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
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

	// operationName is the core operation the walk is part of, a diff or an
	// apply; apply unless set. A decide walk runs for both.
	operationName string

	// recorder is the decision record a decide walk fills, nil in apply
	recorder *diffRecorder

	// current is the parsed configuration, the dependencies of a resource
	// are resolved against it
	current ResourceProvider

	// addresses resolves the addresses in a resource's links
	addresses *resources.AddressParser

	// diffOptions are the settings of a diff walk
	diffOptions diff.Options
}

// operation is the core operation the lifecycle's walk is part of
func (l *resourceLifecycle) operation() string {
	if l.operationName != "" {
		return l.operationName
	}

	return events.OperationApply
}

// apply runs the provider lifecycle for a decoded resource, following the
// decision the decide pass recorded for it:
//
//   - create or replace: the resource is created, a replaced resource was
//     destroyed before the walk started
//   - update: the resource is updated in place
//   - unchanged: no provider is called, the resource keeps what was read and
//     its previous status
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

	// the decide pass recorded what happens to every provider-backed
	// resource, the act pass only follows it
	dec, ok := l.recorder.lookup(meta.ID)
	if !ok {
		return fmt.Errorf("no decision was made for resource %s", meta.ID)
	}

	switch dec.action {
	case diff.ActionCreate, diff.ActionReplace:
		// a replaced resource was destroyed before the walk started
		return l.create(r, adapter)
	case diff.ActionUpdate:
		return l.update(r, dec, adapter)
	default:
		return l.keep(r, dec)
	}
}

// update calls the provider's Update for a resource decided update. It is
// sent the configuration as decoded now, with real values, and the computed
// values the decide pass read.
func (l *resourceLifecycle) update(r any, dec decision, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	if err := carryComputedValues(r, dec.read); err != nil {
		return err
	}

	data, err := wire.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	duration, err := l.callProvider(events.OperationUpdate, r, data, func(ctx context.Context) ([]byte, error) {
		return adapter.Update(ctx, data)
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	l.warnChangedConfiguredValues(events.OperationUpdate, r, data)

	meta.Status = types.StatusUpdated

	emitLifecycle(l.options, meta, events.OperationUpdate, events.PhaseSuccess, duration, nil, data, r)

	return nil
}

// keep saves a resource decided unchanged as its provider read it while
// deciding, with its previous status. No provider is called.
func (l *resourceLifecycle) keep(r any, dec decision) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	old, err := findByID(l.previous.GetResources(), meta.ID)
	if err != nil {
		return fmt.Errorf("unable to find resource %s in previous state: %w", meta.ID, err)
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return err
	}

	if err := replaceValues(r, dec.read); err != nil {
		return err
	}

	// replaceValues keeps the configured meta, the saved status carries over
	meta, err = types.GetMeta(r)
	if err != nil {
		return err
	}

	meta.Status = oldMeta.Status
	return nil
}

// decide records what an apply does with a decoded resource, choosing from
// the resource's entry in the previous state, without creating, updating or
// destroying anything:
//
//   - absent: the resource is created
//   - created or updated: the resource is read through its provider and its
//     provider asks whether it changed, told which of its dependencies the
//     same apply updates or replaces. It is created again when the provider
//     no longer finds it, and otherwise updated, replaced or left unchanged
//     as the provider answers
//   - failed, destroy_failed or anything else: the resource is replaced, no
//     provider is called
//
// A resource whose configured values are only known once the apply has run
// is read with its saved values in their place, and is at least updated: its
// inputs change, whatever its provider says.
//
// Resources without a provider take no part: they are neither recorded nor
// reported in an event. A provider error fails the decision, the resource is
// not marked failed since nothing was attempted.
func (l *resourceLifecycle) decide(r any) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	err = l.decideResource(r, meta)
	if errors.Is(err, errNotReached) {
		return err
	}

	// a failure outside a provider call has no event yet
	if err != nil && !reported(err) {
		emitWalkError(l.options, l.operation(), meta, err)
	}

	return err
}

// decideResource chooses and records the action an apply takes for a
// resource
func (l *resourceLifecycle) decideResource(r any, meta *types.Meta) error {
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
			l.recorder.decide(meta.ID, decision{action: diff.ActionCreate})
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
		l.recordReplace(meta, diff.ReplaceFailed, nil, old, r)
		l.recorder.decide(meta.ID, decision{action: diff.ActionReplace, reason: diff.ReplaceFailed})
		return nil
	}

	// every parent is decided before its dependents, so the record knows
	// what happens to each dependency
	dependencies, err := dependencyChanges(r, l.current, l.addresses, l.types, l.recorder)
	if err != nil {
		return fmt.Errorf("unable to resolve the dependencies of %s: %w", meta.ID, err)
	}

	// a value only known once the apply has run is read as what was saved,
	// the provider is told why through its dependencies
	unknown := l.recorder.unknownPaths(meta.ID)
	if len(unknown) > 0 {
		withSavedValues(r, old, unknown)
	}

	outcome, copies, err := l.refresh(r, old, adapter, dependencies)
	if err != nil {
		return err
	}

	// the inputs of a resource with unknown values change, so it is at
	// least updated; a provider may still answer replace
	if len(unknown) > 0 && outcome == refreshUnchanged {
		outcome = refreshChanged
	}

	switch outcome {
	case refreshNotFound:
		// refresh restored the configured values
		l.recordPending(meta, diff.ActionCreate, nil, r)
		l.recorder.decide(meta.ID, decision{action: diff.ActionCreate, dependencies: dependencies})

	case refreshReplace:
		configured, err := decodeCopy(r, copies.configured)
		if err != nil {
			return err
		}

		reason, replacedDeps := replaceReason(dependencies)

		l.recordReplace(meta, reason, replacedDeps, old, configured)
		l.recorder.decide(meta.ID, decision{
			action:       diff.ActionReplace,
			reason:       reason,
			replacedDeps: replacedDeps,
			dependencies: dependencies,
			read:         copies.read,
		})

	case refreshChanged:
		// the configuration as written is compared, not what the provider
		// read back, so drift alone lists no changes
		configured, err := decodeCopy(r, copies.configured)
		if err != nil {
			return err
		}

		l.recordPending(meta, diff.ActionUpdate, old, configured)
		l.recorder.decide(meta.ID, decision{action: diff.ActionUpdate, dependencies: dependencies, read: copies.read})

	default:
		l.recorder.recordUnchanged()
		l.recorder.decide(meta.ID, decision{dependencies: dependencies, read: copies.read})
	}

	return nil
}

// replaceReason returns why a provider answered replace: because of the
// replaced dependencies it was told about, sorted, or, when there were none,
// because it cannot update the resource in place
func replaceReason(dependencies []entity.DependencyChange) (diff.ReplaceReason, []string) {
	replaced := []string{}
	for _, dependency := range dependencies {
		if dependency.Change == entity.Replace {
			replaced = append(replaced, dependency.Address)
		}
	}

	if len(replaced) == 0 {
		return diff.ReplaceProvider, nil
	}

	sort.Strings(replaced)
	return diff.ReplaceDependency, replaced
}

// recordPending records a resource an apply would create, replace or update.
// Its computed values are unknown until the apply runs: nothing says which
// computed values an update leaves alone, so every one of them is assumed to
// change, and the resources that use them are reported as updated too. The
// diff may list a resource an apply then leaves alone, never the other way
// round.
func (l *resourceLifecycle) recordPending(meta *types.Meta, action diff.Action, saved, configured any) {
	l.recordPendingResource(meta, diff.Resource{Action: action}, saved, configured)
}

// recordReplace records a resource an apply would replace, with the reason
// and the replaced dependencies behind it, see recordPending
func (l *resourceLifecycle) recordReplace(meta *types.Meta, reason diff.ReplaceReason, replacedDeps []string, saved, configured any) {
	l.recordPendingResource(meta, diff.Resource{Action: diff.ActionReplace, Reason: reason, ReplacedDeps: replacedDeps}, saved, configured)
}

// recordPendingResource records resource, completed with the entity's
// address and changes, see recordPending
func (l *resourceLifecycle) recordPendingResource(meta *types.Meta, resource diff.Resource, saved, configured any) {
	resource.Address = meta.ID
	resource.Changes = l.changes(meta, resource.Action, saved, configured)

	l.recorder.markPending(meta.ID)
	l.recorder.record(resource)
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

	// refreshChanged is a resource the provider reports as changed, it can be
	// updated in place
	refreshChanged

	// refreshReplace is a resource the provider reports as changed in a way it
	// cannot update in place, it must be destroyed and created again
	refreshReplace

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
// does not mark the resource failed: refresh only runs while deciding, when
// nothing has been attempted.
func (l *resourceLifecycle) refresh(r any, old any, adapter plugins.ProviderAdapter, dependencies []entity.DependencyChange) (refreshOutcome, refreshed, error) {
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
		return refreshUnchanged, copies, err
	}

	emitLifecycle(l.options, meta, events.OperationRead, events.PhaseSuccess, duration, nil, newData, r)

	copies.read, err = wire.Marshal(r)
	if err != nil {
		return refreshUnchanged, copies, fmt.Errorf("unable to serialize read resource %s: %w", meta.ID, err)
	}

	l.warnChangedConfiguredValues(events.OperationRead, r, newData)

	change := entity.NoChange
	duration, err = l.callProvider(events.OperationChanged, r, copies.read, func(ctx context.Context) ([]byte, error) {
		var changedErr error
		change, changedErr = adapter.Changed(ctx, copies.old, copies.read, dependencies)
		return nil, changedErr
	})
	if errors.Is(err, errNotReached) {
		return refreshUnchanged, copies, err
	}

	if err != nil {
		return refreshUnchanged, copies, err
	}

	emitLifecycle(l.options, meta, events.OperationChanged, events.PhaseSuccess, duration, nil, copies.read, r)

	switch change {
	case entity.Update:
		return refreshChanged, copies, nil
	case entity.Replace:
		return refreshReplace, copies, nil
	default:
		return refreshUnchanged, copies, nil
	}
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
// updated, and destroy for a replacement of a failed resource
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

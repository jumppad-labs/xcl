package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/resources"
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
			emitWalkError(l.options, meta, err)
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
		emitLifecycle(l.options, meta, events.OperationCreate, events.PhaseSuccess, 0, nil, nil)
		return nil
	}

	adapter := l.resolver.GetProviderForResource(r)
	if adapter == nil {
		err := fmt.Errorf("no provider found for resource type %s", meta.AddressType())
		emitLifecycle(l.options, meta, l.firstStep(meta), events.PhaseError, 0, err, nil)
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

// create calls the provider's Create for a resource
func (l *resourceLifecycle) create(r any, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	err = l.callProvider(events.OperationCreate, r, data, func(ctx context.Context) ([]byte, error) {
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
	return nil
}

// read handles a resource that exists in the previous state. The computed values
// saved last time are carried onto the configured resource, the provider reads
// the real resource, change detection compares the saved copy with what was read
// and the resource is updated only when it changed. A resource the provider no
// longer finds is created again from its configuration.
func (l *resourceLifecycle) read(r any, old any, adapter plugins.ProviderAdapter) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	oldMeta, err := types.GetMeta(old)
	if err != nil {
		return err
	}

	// the previous resource is never modified, it is only serialized
	oldData, err := json.Marshal(old)
	if err != nil {
		return fmt.Errorf("unable to serialize previous resource %s: %w", meta.ID, err)
	}

	configuredData, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	// computed values are carried over before the read, so they survive even
	// when the provider's Read adds nothing, and change detection sees the same
	// computed values on both copies
	if err := carryComputedValues(r, oldData); err != nil {
		return err
	}

	newData, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize resource %s: %w", meta.ID, err)
	}

	err = l.callProvider(events.OperationRead, r, newData, func(ctx context.Context) ([]byte, error) {
		return adapter.Read(ctx, oldData, newData)
	})
	if errors.Is(err, plugins.ErrNotFound) {
		// the real resource is gone, its computed values went with it
		if err := replaceValues(r, configuredData); err != nil {
			return err
		}

		return l.create(r, adapter)
	}

	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	readData, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("unable to serialize read resource %s: %w", meta.ID, err)
	}

	l.warnChangedConfiguredValues(events.OperationRead, r, newData)

	changed := false
	err = l.callProvider(events.OperationChanged, r, readData, func(ctx context.Context) ([]byte, error) {
		var changedErr error
		changed, changedErr = adapter.Changed(ctx, oldData, readData)
		return nil, changedErr
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	// an unchanged resource keeps what was read and its previous status
	if !changed {
		meta.Status = oldMeta.Status
		return nil
	}

	err = l.callProvider(events.OperationUpdate, r, readData, func(ctx context.Context) ([]byte, error) {
		return adapter.Update(ctx, readData)
	})
	if errors.Is(err, errNotReached) {
		return err
	}

	if err != nil {
		meta.Status = types.StatusFailed
		return err
	}

	l.warnChangedConfiguredValues(events.OperationUpdate, r, readData)

	meta.Status = types.StatusUpdated
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
	oldData, err := json.Marshal(old)
	if err != nil {
		return fmt.Errorf("unable to serialize previous resource %s: %w", meta.ID, err)
	}

	err = l.callProvider(events.OperationDestroy, r, oldData, func(ctx context.Context) ([]byte, error) {
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

	after, err := json.Marshal(r)
	if err != nil {
		return
	}

	log := logger.New(l.options.Emit, lifecycleEvent(meta, operation, "", 0, nil, nil))
	warnChangedConfiguredValues(log, meta.ID, l.bodies[meta.ID], reflect.TypeOf(r), before, after)
}

// callProvider wraps a single provider call. It fires the start event, times the
// call, fires the success or error event and, on success, decodes a non-empty
// result into the resource. data is the serialized resource sent with the events.
//
// No call starts once the operation's context is cancelled, errNotReached is
// returned instead and no event is fired. The call is given a copy of the
// context that is never cancelled, so a call already running always finishes,
// carrying a logger bound to the resource and the step, see plugins.Logger.
func (l *resourceLifecycle) callProvider(operation string, r any, data []byte, call func(ctx context.Context) ([]byte, error)) error {
	meta, err := types.GetMeta(r)
	if err != nil {
		return err
	}

	if l.ctx.Err() != nil {
		return errNotReached
	}

	id := meta.ID

	emitLifecycle(l.options, meta, operation, events.PhaseStart, 0, nil, data)
	start := time.Now()
	result, err := call(providerContext(l.ctx, l.options, meta, operation))
	duration := time.Since(start)

	if err != nil {
		emitLifecycle(l.options, meta, operation, events.PhaseError, duration, err, data)
		return reportedError{fmt.Errorf("%s failed for %s: %w", operation, id, err)}
	}

	if len(result) > 0 {
		if err := json.Unmarshal(result, r); err != nil {
			decodeErr := fmt.Errorf("unable to decode %s result: %w", operation, err)
			emitLifecycle(l.options, meta, operation, events.PhaseError, duration, decodeErr, data)
			return reportedError{fmt.Errorf("%s failed for %s: %w", operation, id, decodeErr)}
		}
	}

	emitLifecycle(l.options, meta, operation, events.PhaseSuccess, duration, nil, data)
	return nil
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
// It takes the whole meta rather than a single name because the two questions
// it asks sit on different axes: the builtins are stanza kinds, while a
// registered Go type is registered under its variety.
func handledWithoutProvider(typeRegistry TypeRegistry, meta *types.Meta) bool {
	if meta.Type == resources.TypeVariable ||
		meta.Type == resources.TypeOutput ||
		meta.Type == resources.TypeModule ||
		meta.Type == resources.TypeRoot {
		return true
	}

	// a kind led type is registered under its variety and a bare one under its
	// own keyword, which is exactly what AddressType returns
	return typeRegistry != nil && typeRegistry.IsRegisteredType(meta.AddressType())
}

// resourceType returns the "<type>.<name>" form used in parser events
func resourceType(meta *types.Meta) string {
	return fmt.Sprintf("%s.%s", meta.AddressType(), meta.Name)
}

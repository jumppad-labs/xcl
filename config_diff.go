package xcl

import (
	"context"
	"fmt"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/internal/parser"
	"github.com/jumppad-labs/xcl/logger"
)

// Diff reports what Apply would do with the configuration discovered from
// paths, without doing any of it: which resources an apply would create,
// update, replace or delete, and how many it would leave unchanged.
//
// Diff takes the same paths as Apply and resolves the configuration the same
// way, so it fails in the same cases: no paths, a configuration that does not
// parse or validate, one that declares no blocks (ErrEmptyConfiguration), or a
// saved state that cannot be loaded. It compares against the state Apply
// would use: the state loaded from the StateStore, or an empty state when
// there is none.
//
// Every resource in the saved state that is still configured is read and
// compared through its provider, exactly as Apply does, so a resource changed
// outside xcl is reported as updated and one that no longer exists as
// created. A provider error while reading or comparing fails the diff with an
// error naming the resource.
//
// Diff never creates, updates or destroys a resource, never saves state, and
// leaves what Entities returns untouched. It runs as the diff operation, with
// the same events and logging as Apply. Sensitive values are left out of the
// result unless diff.RevealSensitive is given.
func (c *Config) Diff(paths []string, options ...diff.Option) (*diff.Diff, error) {
	var result *diff.Diff

	err := c.run(events.OperationDiff, func(ctx context.Context, emit events.Emit) error {
		if len(paths) == 0 {
			return fmt.Errorf("at least one path is required")
		}

		p := parser.NewParser(&parser.ParserOptions{
			EventData:      c.eventData,
			StateStore:     c.stateStore,
			StateMask:      c.stateMask,
			EventMask:      c.eventMask,
			PluginRegistry: c.pluginRegistry,
			Variables:      convertVariablesToStringMap(c.variables),
			Emit:           emit,
		})

		found, err := p.Diff(ctx, diff.NewOptions(options...), paths...)
		if err != nil {
			return err
		}

		logger.New(emit, events.Event{
			Source:    events.SourceCore,
			Operation: events.OperationDiff,
		}).Debug("diff complete",
			"create", found.Summary.Create,
			"update", found.Summary.Update,
			"replace", found.Summary.Replace,
			"delete", found.Summary.Delete,
			"unchanged", found.Summary.Unchanged,
		)

		result = found
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

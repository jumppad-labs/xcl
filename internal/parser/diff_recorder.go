package parser

import (
	"sort"
	"sync"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/types"
)

// walkMode is what a walk of the configuration does with each entity
type walkMode int

const (
	// walkApply runs each entity's provider lifecycle
	walkApply walkMode = iota

	// walkDecide records what an apply does with each entity, without
	// creating, updating or destroying anything. Plans and applies share it.
	walkDecide
)

// decision is one entity's outcome from the decide pass
type decision struct {
	// action is what the apply does with the entity: create, update, replace
	// or delete. It is empty for an entity left unchanged.
	action diff.Action

	// reason says why the entity is replaced, empty for any other action
	reason diff.ReplaceReason

	// replacedDeps are the addresses of the replaced dependencies behind a
	// replacement with reason diff.ReplaceDependency, sorted
	replacedDeps []string

	// dependencies are what the entity's provider was told about the
	// dependencies the same apply will update or replace
	dependencies []entity.DependencyChange

	// changes are the plan's view of the entity's changed settings, with
	// sensitive values hidden unless the diff options reveal them
	changes []diff.Change

	// read is the entity as its provider read it while deciding, empty when
	// it was not read
	read []byte
}

// diffRecorder is the decision record of one operation: what the decide pass
// found out for each entity, the diff it reports, and the pending and unknown
// bookkeeping dependents consult. The walk's callbacks run concurrently, so
// every method is guarded by mu. The DAG visits a parent before its
// dependents, so what is recorded for an entity is in place before any entity
// that depends on it consults it.
type diffRecorder struct {
	mu sync.Mutex

	// decisions holds the decision for each entity, by entity ID
	decisions map[string]decision

	// pending holds the IDs of the entities an apply would create, replace
	// or update, whose computed values are unknown until it does
	pending map[string]bool

	// unknown holds, by entity ID, the paths of configured values only known
	// once an apply has run
	unknown map[string][]diff.Path

	// resources are the entities an apply would change
	resources []diff.Resource

	// unchanged counts the provider-backed entities an apply would leave as
	// they are
	unchanged int
}

func newDiffRecorder() *diffRecorder {
	return &diffRecorder{
		decisions: map[string]decision{},
		pending:   map[string]bool{},
		unknown:   map[string][]diff.Path{},
	}
}

// decide records the decision for the entity, replacing any recorded before
func (r *diffRecorder) decide(id string, dec decision) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.decisions[id] = dec
}

// lookup returns the decision recorded for the entity, false when none was
func (r *diffRecorder) lookup(id string) (decision, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	dec, ok := r.decisions[id]
	return dec, ok
}

// replaced returns the IDs of the entities decided replace, sorted
func (r *diffRecorder) replaced() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := []string{}
	for id, dec := range r.decisions {
		if dec.action == diff.ActionReplace {
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)
	return ids
}

// toDestroy returns the entities saved in previous that the apply destroys
// before it creates or updates anything: those decided replace and those
// decided delete. They are returned in the order previous holds them.
func (r *diffRecorder) toDestroy(previous *State) []any {
	r.mu.Lock()
	defer r.mu.Unlock()

	targets := []any{}
	for _, saved := range previous.GetResources() {
		meta, err := types.GetMeta(saved)
		if err != nil {
			continue
		}

		dec, ok := r.decisions[meta.ID]
		if !ok {
			continue
		}

		if dec.action == diff.ActionReplace || dec.action == diff.ActionDelete {
			targets = append(targets, saved)
		}
	}

	return targets
}

// markPending records that an apply would create, replace or update the
// entity
func (r *diffRecorder) markPending(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pending[id] = true
}

// isPending returns true when an apply would create, replace or update the
// entity
func (r *diffRecorder) isPending(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.pending[id]
}

// recordUnknown records paths of the entity's configured values that are only
// known once an apply has run
func (r *diffRecorder) recordUnknown(id string, paths []diff.Path) {
	if len(paths) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.unknown[id] = append(r.unknown[id], paths...)
}

// unknownPaths returns the recorded unknown paths of the entity
func (r *diffRecorder) unknownPaths(id string) []diff.Path {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]diff.Path{}, r.unknown[id]...)
}

// record records an entity an apply would change
func (r *diffRecorder) record(resource diff.Resource) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resources = append(r.resources, resource)
}

// recordUnchanged counts an entity an apply would leave as it is
func (r *diffRecorder) recordUnchanged() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.unchanged++
}

// result returns the diff: the recorded entities sorted by address, and the
// summary counting them by action
func (r *diffRecorder) result() *diff.Diff {
	r.mu.Lock()
	defer r.mu.Unlock()

	resources := append([]diff.Resource{}, r.resources...)
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Address < resources[j].Address
	})

	summary := diff.Summary{Unchanged: r.unchanged}
	for _, resource := range resources {
		switch resource.Action {
		case diff.ActionCreate:
			summary.Create++
		case diff.ActionUpdate:
			summary.Update++
		case diff.ActionReplace:
			summary.Replace++
		case diff.ActionDelete:
			summary.Delete++
		}
	}

	return &diff.Diff{Summary: summary, Resources: resources}
}

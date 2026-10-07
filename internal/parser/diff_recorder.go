package parser

import (
	"sort"
	"sync"

	"github.com/jumppad-labs/xcl/diff"
)

// walkMode is what a walk of the configuration does with each entity
type walkMode int

const (
	// walkApply runs each entity's provider lifecycle
	walkApply walkMode = iota

	// walkDiff records what an apply would do with each entity, without
	// creating, updating or destroying anything
	walkDiff
)

// diffRecorder collects the outcome of a diff walk. The walk's callbacks run
// concurrently, so every method is guarded by mu. The DAG visits a parent
// before its dependents, so what is recorded for an entity is in place before
// any entity that depends on it consults it.
type diffRecorder struct {
	mu sync.Mutex

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
		pending: map[string]bool{},
		unknown: map[string][]diff.Path{},
	}
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

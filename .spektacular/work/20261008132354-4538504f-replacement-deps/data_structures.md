**Public, package `github.com/jumppad-labs/xcl/plugins`.** These are the design's shapes, unchanged.

```go
// Change is what applying a new configuration needs for one resource
type Change int

const (
	NoChange Change = iota // leave the resource as it is
	Update                 // call Update in place
	Replace                // Destroy the resource, then Create it again
)

func (c Change) String() string // "no change", "update", "replace"

// DependencyChange is a resource this one depends on that the same apply
// will update or replace
type DependencyChange struct {
	Address string // e.g. "docker.network.app"
	Change  Change // Update or Replace, never NoChange
}

type ResourceProvider[T any] interface {
	// ... Create, Destroy, Read, Update unchanged
	Changed(ctx context.Context, old, new T, dependencies []DependencyChange) (Change, error)
}

func (DefaultChanged[T]) Changed(ctx context.Context, old, new T, dependencies []DependencyChange) (Change, error)
```

Every byte-level layer changes the same way, gaining `dependencies` and returning `Change`:

```go
// ProviderAdapter
Changed(ctx context.Context, oldEntityData, newEntityData []byte, dependencies []DependencyChange) (Change, error)

// Plugin / PluginHost
Changed(ctx context.Context, entityType, entitySubType string, oldEntityData, newEntityData []byte, dependencies []DependencyChange) (Change, error)
```

**gRPC protocol (`plugins/plugin.proto`).** This is a serialization boundary, and the change breaks compatibility (allowed by the spec).

```proto
enum Change {
  CHANGE_NO_CHANGE = 0;
  CHANGE_UPDATE    = 1;
  CHANGE_REPLACE   = 2;
}

message DependencyChange {
  string address = 1;
  Change change  = 2;
}

message ChangedRequest {
  string entity_type      = 1;
  string entity_sub_type  = 2;
  bytes  old_entity_data  = 3;
  bytes  new_entity_data  = 4;
  repeated DependencyChange dependencies = 5;
}

message ChangedResponse {
  reserved 1;               // was: bool changed
  string error  = 2;
  Change change = 3;
}
```

**Public, package `github.com/jumppad-labs/xcl/diff`.** `Resource` gains the reason for a replacement. Both new fields are empty for every other action, and both are carried in the JSON form.

```go
// ReplaceReason says why a resource will be replaced
type ReplaceReason string

const (
	ReplaceFailed     ReplaceReason = "failed"     // its last apply failed
	ReplaceProvider   ReplaceReason = "provider"   // its provider cannot update it in place
	ReplaceDependency ReplaceReason = "dependency" // a resource it depends on is replaced
)

type Resource struct {
	Address      string        `json:"address"`
	Action       Action        `json:"action"`
	Reason       ReplaceReason `json:"reason,omitempty"`
	ReplacedDeps []string      `json:"replaced_dependencies,omitempty"` // sorted; set with ReplaceDependency
	Changes      []Change      `json:"changes,omitempty"`
}
```

`ActionReplace`'s doc comment widens to "destroyed and created again". `Render`'s comment line for a replacement reads, by reason:
- `ReplaceFailed`: `will be replaced, its last apply failed`
- `ReplaceProvider`: `will be replaced, it cannot be updated in place`
- `ReplaceDependency`: `will be replaced because a, b are replaced`

**Internal, package `internal/parser`.** These are contracts between the parts of the walk; the names are indicative.

```go
// decision is one entity's outcome from the decide pass
type decision struct {
	action       diff.Action               // create, update, replace, delete; "" = unchanged
	reason       diff.ReplaceReason
	replacedDeps []string
	dependencies []plugins.DependencyChange // what Changed was told
	read         []byte                     // decide-pass read copy (wire JSON)
}

// decisions is the decision record: the diffRecorder grown to hold a
// decision per entity ID alongside its pending/unknown bookkeeping
type decisions struct { /* mutex, map[id]decision, pending, unknown, unchanged count */ }
func (d *decisions) decide(id string, dec decision)
func (d *decisions) lookup(id string) (decision, bool)
func (d *decisions) toDestroy(previous *State) []any // replaced + removed saved entities
func (d *decisions) result() *diff.Diff              // plan; sorted, with summary

// dependencyChanges resolves an entity's links to provider-backed resources
// (looking through outputs, variables, modules, config-only types) and
// returns those the record says will update or replace
func dependencyChanges(entity any, state *State, record *decisions) []plugins.DependencyChange

// refreshOutcome gains the provider's answer
type refreshOutcome int // refreshNotFound, refreshChanged(update), refreshReplace, refreshUnchanged
func (l *resourceLifecycle) refresh(r, old any, adapter plugins.ProviderAdapter, deps []plugins.DependencyChange) (refreshOutcome, refreshed, error)
```

The walk keeps its two modes, but the meaning changes. `walkDecide` (formerly `walkDiff`) records decisions and never acts. `walkApply` is now the act walk, and it requires a completed decision record.

**Test plugin (`internal/parser` `TestPlugin`).** `ChangedResults` becomes `map[string]plugins.Change`, and `SetChangedResult(id string, change plugins.Change)`. A new `ChangedDependencies map[string][]plugins.DependencyChange` is read with `GetChangedDependencies(id)` and holds the list from the last `Changed` call for each ID.

**Unchanged.** The saved-state format (statuses, `Meta.Links`), event payloads and operations (a replace is still `destroy` followed by `create` for one ID), and the `Create`, `Read`, `Update` and `Destroy` signatures.

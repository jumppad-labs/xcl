---
created_date: "2026-09-21"
document_status: draft
---

# Config diff: what an apply would change

`Config.Diff` reports what `Config.Apply` would do with the same configuration,
without doing any of it: how many resources would change, and for each one
which configured parameters change and how. The result is a plain data
structure, and its JSON encoding is the output format.

## Entry point

```go
func (c *Config) Diff(paths ...string) (*diff.Diff, error)
```

`Diff` takes the same parameters as `Apply` and resolves the configuration the
same way, so it fails in the same cases: no paths, a configuration that does
not parse or validate, one that declares no blocks (`ErrEmptyConfiguration`),
a dependency graph that cannot be built, or a saved state that cannot be
loaded (e.g. `state.UnknownTypesError`).

It compares against the previous state `Apply` would use: the state loaded from
the `StateStore`, or an empty state when there is no store.

`Diff` has no side effects. It never calls a provider, never saves state, fires
no lifecycle events, and leaves `Config.Entities()` untouched.

## A prediction from saved state

`Diff` walks the same dependency graph `Apply` does, decoding each body in
dependency order, and compares each decoded resource with its saved copy. That
makes it a prediction, not a promise: an apply decides whether to update an
existing resource by calling the provider's `Changed(saved, read)` against the
live infrastructure, while `Diff` compares configuration with the saved copy.
Drift in the real infrastructure is invisible to it.

Computed values are resolved the way the lifecycle resolves them. For a
resource in the saved state, the saved computed values are carried onto the
configured copy before comparing, so a reference to that resource's computed
fields resolves to the saved value. Only a resource that will be created or
replaced has computed values that are genuinely unknown.

## Actions

Each resource gets exactly one action:

| Action | When |
|---|---|
| `create` | declared in the configuration, not in the saved state |
| `update` | in both, and at least one configured field differs from the saved copy |
| `replace` | saved as `failed` or `destroy_failed`, which an apply always rebuilds, whether or not its configuration changed |
| `delete` | in the saved state and no longer in the configuration, decided by the same rule `Apply`'s removal phase uses |

A resource in both with no difference in any configured field is unchanged. It
is counted in the summary but not listed.

Only entities an apply hands to a provider take part. Variables, outputs,
modules, disabled blocks and registered config-only types are neither listed
nor counted.

## Changes

A resource's changes are one flat list, one entry per changed configured field,
in field declaration order. Computed fields are never compared.

- **Leaf fields** are compared by value. A differing leaf is one entry with
  `before` and `after`.
- **Nested blocks** are descended into, so the path grows:
  `network.aliases[1]`.
- **Lists** are compared by index, as in `configured_check.go`. Removing
  `ports[0]` shows up as changes to every port after it.
- **Maps** are compared by key: `env["LOG_LEVEL"]`.
- **An added list element or map key** is a single entry for the whole element,
  with no `before`; a removed one has no `after`. The element's fields are not
  itemised.
- **`create`** lists each configured top-level field as its own entry with only
  an `after`; nested blocks are whole values.
- **`replace`** lists its differences from the saved copy the same way `update`
  does, and may have none.
- **`delete`** has no changes.

A value that depends on a computed field of a resource being created or
replaced is reported with `unknown: true` and no `after`. When an entry's value
would *contain* an unknown — an added block with one unknown field, say — it is
split into its children until the unknown stands alone, so every other value
stays concrete.

One constraint for the plan to solve: the apply walk decodes a body straight
into a Go struct, which cannot hold an unknown value. For a field that depends
on one, `Diff` has to evaluate the expression itself and record the path as
unknown rather than decoding it.

## Types

The types live in a new public package, `github.com/jumppad-labs/xcl/diff`.

```go
type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionReplace Action = "replace"
	ActionDelete  Action = "delete"
)

type Diff struct {
	Summary   Summary    `json:"summary"`
	Resources []Resource `json:"resources"` // sorted by address; unchanged resources omitted
}

type Summary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Replace   int `json:"replace"`
	Delete    int `json:"delete"`
	Unchanged int `json:"unchanged"`
}

type Resource struct {
	Address string   `json:"address"`
	Action  Action   `json:"action"`
	Changes []Change `json:"changes,omitempty"`
}

type Change struct {
	Path    Path `json:"path"`
	Before  any  `json:"before,omitempty"` // absent: the field or element was added
	After   any  `json:"after,omitempty"`  // absent: removed, or unknown
	Unknown bool `json:"unknown,omitempty"`
}
```

The number of changed resources is `Create + Update + Replace + Delete`,
offered as a method on `Diff` rather than a stored field so it cannot disagree
with its parts.

`Path` is a list of segments, not a string, so a consumer that needs a tree can
rebuild one without parsing strings back apart:

```go
type Path []Step

type Step struct {
	Kind      StepKind // attribute, index or key
	Attribute string
	Index     int
	Key       string
}

func (p Path) String() string // image, ports[0].host, env["LOG_LEVEL"]
```

`Path` marshals to JSON as its string form.

## JSON format

`json.Marshal` of a `Diff` produces:

```json
{
  "summary": { "create": 1, "update": 1, "replace": 0, "delete": 1, "unchanged": 4 },
  "resources": [
    { "address": "resource.container.api", "action": "update",
      "changes": [ { "path": "ports[0].host", "before": 8080, "after": 9090 },
                   { "path": "ports[2]", "after": { "host": 443, "local": 8443 } } ] },
    { "address": "resource.container.web", "action": "create",
      "changes": [ { "path": "image", "after": "nginx" },
                   { "path": "db_host", "unknown": true } ] },
    { "address": "resource.postgres.old", "action": "delete" }
  ]
}
```

## Not in this design

- Pluggable output — a terminal diff view, a pretty HCL view, or any renderer
  abstraction. These build on the `Diff` types later; the segmented `Path` is
  there for them.
- Drift detection by calling providers' `Read`, a possible later option to
  `Diff`.
- Masking sensitive values.
- Matching list elements by identity rather than by index.
- Any CLI.

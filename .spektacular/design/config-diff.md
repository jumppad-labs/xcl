---
created_date: "2026-09-21"
document_status: draft
spec: 20261007105731-2388b579-diff
specs:
    - 20261007105731-2388b579-diff
    - 20261007111826-cf3b66d8-diff-rendering-and-docs
---

# Config diff: what an apply would change

`Config.Diff` reports what `Config.Apply` would do with the same configuration,
without doing any of it: how many resources would change, and for each one
which configured parameters change and how. The result is a plain data
structure; its JSON encoding is the machine-readable format, and a renderer
turns it into a readable, git-diff-style view.

## Entry point

```go
func (c *Config) Diff(paths []string, options ...diff.Option) (*diff.Diff, error)
```

`Diff` takes the same paths as `Apply` and resolves the configuration the same
way, so it fails in the same cases: no paths, a configuration that does not
parse or validate, one that declares no blocks (`ErrEmptyConfiguration`), a
dependency graph that cannot be built, or a saved state that cannot be loaded
(e.g. `state.UnknownTypesError`).

It compares against the previous state `Apply` would use: the state loaded from
the `StateStore`, or an empty state when there is no store.

The only option today is `diff.RevealSensitive()`; see Sensitive values.

## Refreshing through providers

`Diff` walks the same dependency graph `Apply` does, in the same order, and for
each resource that is in the saved state and still configured it makes the
same two provider calls the apply update path makes:

1. `Read(saved, configured)`, with the saved computed values carried onto the
   configured copy first, exactly as the lifecycle does.
2. `Changed(saved, read)`.

That makes drift in the real infrastructure visible: a resource altered outside
xcl is reported as an update even when its configuration did not change.

`Diff` never calls `Create`, `Update` or `Destroy`, never saves state, and
leaves `Config.Entities()` untouched. Providers' `Read` and `Changed` are
expected to be free of side effects, as they already are for apply.

Resources that get no provider call:

- one being created, deleted or replaced;
- one whose configuration depends on a value that is unknown until apply (see
  Unknown values). Calling `Read` with a half-resolved resource would ask the
  provider about something that does not exist yet.

When `Read` returns `plugins.ErrNotFound`, the real resource is gone and an
apply would create it again, so the resource is reported as `create`.

When `Read` or `Changed` returns any other error, `Diff` stops and returns that
error wrapped with the resource's address. There is no partial result.

## Events

`Diff` runs as its own operation, `diff`, through the same `run` wrapper as
`Validate`, `Apply` and `Destroy`: the same start and finish events, the same
logger, and the same panic handling. Each `Read` and `Changed` call emits the
lifecycle events it emits during an apply. No create, update or destroy events
fire, because nothing is created, updated or destroyed.

## Actions

Each resource gets exactly one action:

| Action | When |
|---|---|
| `create` | declared in the configuration and not in the saved state, or in the saved state but its provider's `Read` reports it not found |
| `update` | in both, and the provider's `Changed` reports a change, or it depends on an unknown value |
| `replace` | saved as `failed` or `destroy_failed`, which an apply always rebuilds, whether or not its configuration changed |
| `delete` | in the saved state and no longer in the configuration, decided by the same rule `Apply`'s removal phase uses |

A resource whose provider reports no change is unchanged. It is counted in the
summary but not listed.

Only entities an apply hands to a provider take part. Variables, outputs,
modules, disabled blocks and registered config-only types are neither listed
nor counted.

## Changes

A resource's changes are one flat list, one entry per changed configured field,
in field declaration order. For `update` and `replace` they compare the saved
copy with the configured copy. Computed fields are never compared, so values a
provider fills in never show up as noise.

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
- **`update`** may have no changes, when the provider reported a change that
  is not in any configured field: drift in the real resource.
- **`replace`** lists its differences from the saved copy the same way `update`
  does, and may have none.
- **`delete`** has no changes.

## Unknown values

A value that depends on a computed field of a resource being created or
replaced is reported with `unknown: true` and no `after`. When an entry's value
would *contain* an unknown — an added block with one unknown field, say — it is
split into its children until the unknown stands alone, so every other value
stays concrete.

A resource in the saved state that depends on an unknown value is reported as
`update`: it gets no provider call, so whether an apply would really change it
cannot be known, and the unknown entries say why.

One constraint for the plan to solve: the apply walk decodes a body straight
into a Go struct, which cannot hold an unknown value. For a field that depends
on one, `Diff` has to evaluate the expression itself and record the path as
unknown rather than decoding it.

## Sensitive values

A change to a field marked sensitive, or to any value inside one, is reported
with `sensitive: true` and with `before` and `after` absent, so the values
never reach the result, its JSON, or the rendering. The entry is still
present: that a sensitive value changed is itself useful. With
`diff.RevealSensitive()` both values are included and `sensitive` is still set.

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
	Path      Path `json:"path"`
	Before    any  `json:"before,omitempty"`    // absent: added, or sensitive
	After     any  `json:"after,omitempty"`     // absent: removed, unknown, or sensitive
	Unknown   bool `json:"unknown,omitempty"`
	Sensitive bool `json:"sensitive,omitempty"`
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
  "summary": { "create": 1, "update": 2, "replace": 1, "delete": 1, "unchanged": 4 },
  "resources": [
    { "address": "resource.container.api", "action": "update",
      "changes": [ { "path": "ports[0].host", "before": 8080, "after": 9090 },
                   { "path": "ports[2]", "after": { "host": 443, "local": 8443 } },
                   { "path": "env[\"DB_PASSWORD\"]", "sensitive": true } ] },
    { "address": "resource.container.cache", "action": "update" },
    { "address": "resource.container.web", "action": "create",
      "changes": [ { "path": "image", "after": "nginx" },
                   { "path": "db_host", "unknown": true } ] },
    { "address": "resource.network.app", "action": "replace" },
    { "address": "resource.postgres.old", "action": "delete" }
  ]
}
```

## Rendering

```go
func Render(d *Diff, options ...RenderOption) []byte
func Highlight(renderer highlight.Renderer) RenderOption
```

`Render` turns a `Diff` into text in the style of a git diff. Without
`Highlight` the output is plain text with no escape codes. With it, lines are
coloured through the same `highlight.Renderer` and themes the encoder uses:
added lines and markers use the theme's inserted colour, removed ones its
deleted colour, changed ones its changed colour, and values are highlighted as
HCL. Themes that do not define those colours fall back to plain text for them.

Resources appear in the result's order. Each has a comment line saying what
will happen, a block header marked with the action, one line per change with
`=` signs aligned within the block, and a closing brace:

| Marker | Meaning |
|---|---|
| `+` | created resource, or added field or element |
| `-` | deleted resource, or removed field or element |
| `~` | updated resource, or changed field (`before -> after`) |
| `-/+` | replaced resource |

Unknown values render as `(known after apply)`, sensitive ones as
`(sensitive value)`. An update with no changes says the resource changed
outside xcl. A delete and a replace with no changes render as a header with an
empty body. The output ends with one summary line; when nothing changes it is
the only line.

The JSON example above renders as:

```
  # resource.container.api will be updated
  ~ resource "container" "api" {
      ~ ports[0].host        = 8080 -> 9090
      + ports[2]             = { host = 443, local = 8443 }
      ~ env["DB_PASSWORD"]   = (sensitive value)
    }

  # resource.container.cache changed outside xcl and will be updated
  ~ resource "container" "cache" {}

  # resource.container.web will be created
  + resource "container" "web" {
      + image   = "nginx"
      + db_host = (known after apply)
    }

  # resource.network.app will be replaced, its last apply failed
-/+ resource "network" "app" {}

  # resource.postgres.old will be deleted
  - resource "postgres" "old" {}

Diff: 1 to create, 2 to update, 1 to replace, 1 to delete, 4 unchanged.
```

With nothing to change:

```
Diff: no changes, 9 unchanged.
```

## Not in this design

- Matching list elements by identity rather than by index.
- Showing the saved values of a deleted resource.
- Saving a diff and applying exactly that diff later.
- Any CLI.

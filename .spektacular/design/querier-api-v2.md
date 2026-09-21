---
created_date: "2026-09-20"
document_status: draft
---

# Query API v2: consuming a parsed configuration

> Source: [jumppad-labs/hclconfig#61](https://github.com/jumppad-labs/hclconfig/issues/61)

Parsing a configuration hands back a flat, untyped collection, and reaching
into it is more work than unmarshalling YAML into a struct. This design removes
`Querier[T]`, puts the lookup surface on `Config` as generic methods with
portable function equivalents, splits the type and subtype axes that `Meta`
currently conflates, and makes `output` blocks reachable through the same
lookup as everything else.

## Vocabulary: entity

**`entity` is the umbrella term** for everything the configuration declares —
resources, variables, outputs and modules alike. `resource` stays the name of
one stanza, not the name of the taxonomy.

It is already the vocabulary of the plugin boundary: 167 whole-word uses across
`plugins/`, including the protobuf wire format, where
`Create(entityType, entitySubType string, entityData []byte)` is precisely this
type-and-subtype taxonomy. Adopting it unifies two vocabularies the project
already runs rather than adding a third.

This is not a new decision. Erik argued for the term in earlier discussion and
it was adopted then; this document follows that decision rather than reopening
it.

The alternative, considered and rejected, was keeping `resource` as the
umbrella and adding a `Kind` to mark system entities. It renames nothing, but
leaves `resource` meaning both one stanza and all of them — and the address
grammar already treats `resource` as a sibling of `output`, `variable` and
`local` (`internal/resources/fqrn.go:58`), not their parent.

## No syntactic sugar between stanza forms

`resource "container" "nics"` and `container "nics"` are **different types**,
not two spellings of one. The first is type `resource`, subtype `container`,
addressed `resource.container.nics`; the second is type `container` with no
subtype, addressed `container.nics`. Neither normalises to the other, and a Go
type is registered under one form or the other, never both.

This keeps the positional rule honest: segment one is always the type, so
`FindByType("container")` matches bare-form container entities where they
exist, and reports an unknown type where they do not. Nothing has to know
which form a stanza was written in.

Entities with a single label — `variable`, `output`, `module`, and any
bare-form type — carry an **empty** subtype.

## Two spellings, one implementation

Generic methods arrived in **Go 1.27**, verified against the toolchain here:

```
./main.go:6:23: generic method requires go1.27 or later (-lang was set to go1.25)
```

1.27 is too new to force on consumers, so the surface ships twice from one
implementation, split by build tag:

```go
// query.go — no build tag, compiles from Go 1.18
func find[T any](c *Config, id string) (*T, error) { ... }
func Find[T any](c *Config, id string) (*T, error) { return find[T](c, id) }

// query_methods_go127.go — //go:build go1.27
func (c *Config) Find[T any](id string) (*T, error) { return find[T](c, id) }
```

A `//go:build go1.27` line raises the language version for that file alone, so
the methods compile where they can and drop out where they cannot. Confirmed
both ways against the same module with `go.mod` at `go 1.25.0`: the 1.27
toolchain builds and runs both forms, the 1.25 toolchain builds and runs the
function form with the method file excluded.

The methods are the destination and the documented form; the functions are the
portable equivalent, deprecated once the floor reaches 1.27. `go.mod` stays at
`go 1.25.0`. Examples use the function form so they build on the floor version,
and CI runs `GOTOOLCHAIN=go1.25.0 go build ./...` as its own job — nobody
compiles that path locally, so it breaks silently otherwise.

## Everything lives in package `xcl`

`Querier[T]` and `NewQuerier` are **removed**. They existed to bind `T` at
construction, which a generic method now does per call. Every one of the 15
call sites in this repo — 8 in the examples, 7 in the tests — constructs a
querier and makes exactly one call on it, and consecutive lookups are usually
of different types:

```go
xcl.NewQuerier[resources.Service](c).FindResource("resource.service.api")   // was
c.Find[resources.Service]("resource.service.api")                          // is
```

Because the methods are on `Config` they are in package `xcl`, so the functions
belong there too. Nothing needs a new package: `pluginRegistry` is reachable
directly, no `TypePath` accessor is needed on `Config`, and `internal/resources`
is already importable from the root.

## Meta carries both axes

`blockResource` (`internal/parser/parser.go:565-575`) writes one field from two
different axes:

```go
fqrn := resources.FQRN{Module: module, Type: b.Type}    // "output", "variable", "module"

case b.Type == types.TypeResource && len(b.Labels) == 2:
    fqrn.Type = b.Labels[0]        // "postgres" — the subtype overwrites Type
    fqrn.Resource = b.Labels[1]
```

So `Meta.Type` holds the subtype for a `resource` stanza and the stanza keyword
for the others, and the word `resource` never appears in it at all. That is why
`FindByType("resource")` returns silently empty today, and why, once type held
`resource` for every resource stanza, a typed lookup on it would hand `As[T]`
entities of several Go types. `schema.UnmarshalUntyped` does not fail on
mismatched structs — unknown fields are dropped, absent fields zero, and both
embed `ResourceBase` — so that path returns half-empty values with no error.

`Meta` therefore gains a real split, populated the same way for every stanza:

```go
Type    string   // "resource", "variable", "output", "module"
Subtype string   // "container", "postgres"; empty for single-label stanzas
```

`resources.FQRN` gains the same field, so an address is
`[module...].<type>.<subtype>.<name>[.attribute]`.

## The lookup surface

```go
c.Find[T](id string) (*T, error)
c.FindByType[T](path ...string) ([]*T, error)
c.FindOne[T](path ...string) (*T, error)
c.All[T]() ([]*T, error)
c.Entities() []any
c.Outputs() map[string]any
As[T any](entity any) (*T, error)          // function only, takes no Config
```

### Find

```go
c.Find[resources.Service]("resource.service.api")   // (*Service, error)
c.Find[string]("output.api_url")                    // (*string,  error)
```

Resolves an id through `Config.FindResource` (`config.go:59`) and its FQRN
parsing (`state/state.go:81`), so it handles module-relative and
non-normalised paths, and a raw `Meta.Links` entry such as
`"resource.deployment.api.meta.id"` — FQRN parsing matches the address parts
and ignores a trailing attribute. Following a reference is a lookup by the id
the reference holds, so there is no separate `Resolve`.

`output.api_url` is an address like any other, so it is the same call. A
consumer should not have to know that an output is stored differently to ask
for it by name. Where the id names an `output`, `Find` returns its **value**
converted to `T` rather than the entity — the `Value any` field of
`internal/resources.Output` (`internal/resources/output.go:11`), populated
during the apply walk (`internal/parser/callbacks.go:173`). A full id is
required: `api_url` is not an address, and inferring the stanza is how
surprises happen. Module outputs work through their full id,
`module.analytics.output.location`.

### FindByType

Segments are address parts, matched positionally from the root: segment one is
the type, segment two the subtype. A subtype alone matches nothing, because
nothing is typed `container`.

```go
// declared as resource "container" "nics"
c.FindByType[resources.Container]("resource", "container")   // ([]*Container, nil)
c.FindByType[resources.PostgreSQL]("resource", "postgres")   // plugin type
c.FindByType[resources.Redis]("resource", "redis")           // ([]*Redis, nil) — none declared

// declared as container "nics" — a different type, not the same entity
c.FindByType[resources.Container]("container")               // ([]*Container, nil)

c.FindByType[T]("nosuchtype")                                // ErrUnknownType
c.FindByType[T]("resource")                                  // ErrNotTypeable
c.FindByType[T]("output")                                    // ErrNotTypeable → Outputs()
```

A set is typed only when the segments pin exactly one Go type. The type axis
never does, so `"resource"` and `"output"` fail for the same reason — outputs
are not a special case. At least one segment is required; `All` is the
no-segment form.

### FindOne

```go
c.FindOne[resources.Deployment]("resource", "deployment")   // (*Deployment, error)
```

The same query as `FindByType`, for the common case of a configuration that
declares exactly one of something. Exactly one match returns it, none returns
`ErrNotFound`, and two or more returns `ErrNotUnique` naming how many were
found.

It exists because the single value is what callers reach for first — a
configuration usually has one deployment, one ingress — and `FindByType`
forces an index and a length check that is easy to skip:

```go
deps, _ := c.FindByType[resources.Deployment]("resource", "deployment")
name := deps[0].Containers[0].Name    // panics when nothing was declared

dep, err := c.FindOne[resources.Deployment]("resource", "deployment")
name := dep.Containers[0].Name        // err says which of the two things went wrong
```

### All

```go
c.All[resources.Deployment]()   // subtype derived from T, no string
```

Derived by reflecting `T` against the registry's prototypes, which are stored
as concrete pointers (`registeredTypes types.RegisteredTypes`,
`plugins/registry/plugin_registry.go:20`, e.g. `&resources.Deployment{}`):

```go
// TypePath returns the address segments a registered type is reached by:
// {"resource", "container"} for resource "container" "nics", or {"container"}
// for the bare form. A type is registered under one form, never both.
func (r *PluginRegistry) TypePath(t reflect.Type) ([]string, bool)
```

This works for `RegisterType` types only. A plugin registers its Go type inside
the plugin and the host only ever sees a schema, so there is no Go type to
match: `All[T]` on a plugin-provided type returns `ErrNotRegistered`, naming
`FindByType("resource", "postgres")` as the form to use.

### Entities and Outputs

Heterogeneous enumeration cannot be typed, so it is untyped and non-generic:

```go
c.Entities()   // []any — every entity: resources, variables, outputs, modules
c.Outputs()    // map[string]any — every output value, by id
```

`Entities` is what `GetResources()` already returns (`config.go:51`), renamed
to say what it holds; `GetResources` is kept as a deprecated alias. `Outputs`
is what `FindByType("output")` points at, and `ResourceCount()` becomes
`EntityCount()` on the same basis.

`output` is the only stanza whose value is projected — `internal/resources`
holds `module`, `output`, `root` and `variable`, and `Variable` carries only
`Default cty.Value` with no resolved value.

### As

```go
func As[T any](entity any) (*T, error)
```

The exported form of `asType[T]` (`querier.go:82`), which removes the
`types.GetMeta` dance every loop over `Entities()` starts with
(`example/configonly/main.go:99-107`). An entity that already is a `*T` is
returned as is; anything else is copied through `schema.UnmarshalUntyped`
(`internal/schema/unmarshal.go:7`).

**`As` verifies before it converts.** Where `T` is a registered type, its
subtype is checked against `Meta.Subtype` first, and a mismatch returns
`ErrTypeMismatch` rather than reaching the JSON fallback — otherwise the
silent-garbage path above stays open to anyone calling `As` over `Entities()`,
which is exactly what exporting it invites. `As` does not project an output's
value; that belongs to the lookup path, so `Find` is `FindResource`, then the
output branch, then `As`.

## Nested blocks are not entities

A nested block is a field of the entity that holds it, not an entity of its
own. `Deployment` embeds `types.ResourceBase` and has an address; the
`Container`, `Port`, `Volume` and `VolumeMount` blocks inside it do not embed
it, carry no `Meta`, and cannot be addressed
(`example/configonly/resources/resources.go:47-51`). They are reached by field
access:

```go
dep, err := c.Find[resources.Deployment]("resource.deployment.api")
for _, ctr := range dep.Containers {
    for _, p := range ctr.Ports { ... }
}
```

The API must say so rather than let the attempt fail quietly, because trying to
query one is the natural first move:

```go
c.FindByType[resources.Container]("resource", "container")   // would be ([], nil)
c.All[resources.Container]()                                 // would be ErrNotRegistered
```

The first is the silent empty this design exists to remove — nothing is typed
`resource.container`, so the honest answer is "containers are not addressable",
not "you have none". The second points at `FindByType`, which cannot help
either. Both return `ErrNotAnEntity` instead, naming the parent-field access.

The check is already written: `findResourceBase`
(`types/resource_helpers.go:9`) reports `ResourceBase field not found in
resource type Container` for exactly this case.

A nested `container` block and a top-level `container "nics"` entity remain
unrelated things that share a word, kept apart by `Meta.Type` and
`Meta.Subtype`.

## Errors

Sentinels checked with `errors.Is`, each wrapped by a struct carrying the
detail for `errors.As`. This follows the house style set by
`plugins.ErrNotFound` (`plugins/errors.go:10`), whose doc comment already
states the convention.

```go
// an ordinary outcome — handle it
ErrNotFound        // no entity at that address                      (Find)

// the question has no answer — a bug in the caller
ErrUnknownType     // segment one is not a type: ("container")       (FindByType)
ErrNotTypeable     // the axis spans several Go types: ("resource")  (FindByType)
ErrNotRegistered   // T has no registered name — a plugin type       (All)
ErrTypeMismatch    // entity is not a T                              (As, Find)
ErrNotAnEntity     // T is a nested block, it has no address          (All, As, FindByType)

// an ordinary outcome for a query that expects one match
ErrNotUnique       // two or more matched, with the count            (FindOne)
```

An empty result with a nil error means the question was answered and the answer
is none. An error means it could not be answered. That distinction is what
stops a typo reading as "you have none of those", and it is the reason `Find`
returns an error rather than comma-ok.

`state.ResourceNotFoundError` (`state/errors.go:9`) gains an `Is` method so it
matches `ErrNotFound`, giving one not-found concept for configuration lookups.
It stays distinct from `plugins.ErrNotFound`, which means the **real**
infrastructure is gone and triggers a recreate — same words, different
condition, and the doc comments must say so.

## Files to change

| File | Change |
|---|---|
| `query.go` (new, replaces `querier.go`) | unexported implementations; exported `Find`, `FindByType`, `FindOne`, `All`, `As`, `Entities`, `Outputs` |
| `query_methods_go127.go` (new) | `//go:build go1.27`; the same surface as methods on `Config` |
| `query_errors.go` (new) | the seven sentinels and their detail structs |
| `querier.go`, `querier_test.go` | **deleted** |
| `config.go` | `Entities()`/`EntityCount()`, deprecating `GetResources()`/`ResourceCount()` |
| `types/resource.go` | `Meta.Subtype`; `Meta.Type` now always the stanza |
| `internal/resources/fqrn.go` | `FQRN.Subtype`; parse and format the extra segment |
| `internal/parser/parser.go` | `blockResource` sets both axes for every stanza |
| `state/errors.go` | `ResourceNotFoundError.Is` |
| `plugins/registry/plugin_registry.go` | `TypePath(reflect.Type) ([]string, bool)` |
| `query_test.go`, `query_errors_test.go` (new) | the surface, both spellings, every error case |
| `example/*/main.go`, `example/*/main_test.go` | the function form throughout |
| `README.md` | rewrite "### Querying resources" (`README.md:267`); add reading outputs |
| `docs/state.md` | note that lookups remain linear scans |
| CI | a `GOTOOLCHAIN=go1.25.0 go build ./...` job |

`asType` is used nowhere outside `querier.go`, so that file lifts out whole.

## Breaking changes (v2, unreleased)

- `Querier[T]`, `NewQuerier`, `Querier.FindResource` and
  `Querier.FindResourcesByType` are all removed. The examples and
  `querier_test.go` are the only in-repo callers.
- `Meta.Type` changes meaning for `resource` stanzas: it becomes `"resource"`,
  and the old value moves to `Meta.Subtype`. Anything matching on `Meta.Type`
  must be updated, and saved state written by v1 needs migrating or
  regenerating.
- Addresses gain a subtype segment, so `ParseFQRN` accepts a form v1 did not.
- Lookups no longer panic on an entity without metadata (`querier.go:32`,
  `querier.go:59`) and match by parsed FQRN rather than exact string
  comparison (`querier.go:35`).

## Not in scope

Exposing the DAG or a dependency-ordered public walk, an index (every lookup
stays a linear scan), `Meta.Properties` (allocated but never written), implementing the bare
`container "nics"` stanza form — the parser accepts only `variable`,
`resource`, `module` and `output` today (`internal/parser/parser.go:543`), and
this design only has to leave room for it — and
the stale `Process`/`ToJSON` sections in `README.md` that document methods
which do not exist.

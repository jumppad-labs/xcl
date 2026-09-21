---
created_date: "2026-09-21"
document_status: final
closed_date: "2026-09-21"
---

# Plan: 20260921093100-query-api-v2

<!-- Metadata -->
<!-- Created: 2026-09-21T12:32:52Z -->
<!-- Commit: 13e0311 -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Developers embedding XCL ask a parsed configuration directly for what they want — one item by its
address, everything of a given kind, the one item they expect there to be exactly one of, or
everything of a Go type — in a single call, with the values a configuration publishes for its
consumers reachable the same way rather than through separate machinery. Every question that cannot
be answered now returns a specific, recognisable error instead of an empty result or a value with
half its fields unset, which is what today's surface does. Developers building on this library get a
shorter and more obvious way to reach a value and get their mistakes reported where they are made,
and consumers on the oldest supported Go toolchain lose no capability, because the whole surface
ships in a portable spelling alongside the newer one.

## Conventions

- **Entity is the umbrella term (glossary)** — binding on naming, and the reason `GetResources()`/`ResourceCount()` become `Entities()`/`EntityCount()` and the enumeration is described as covering every kind of declaration. The entry states the target explicitly and notes the code has not caught up, so this plan is the catching up.
- **Testing & Mocking: testify `require`, NEVER table-driven, never mix positive and negative cases in one function, favour verbosity over abstraction** — drives the whole testing approach. The error taxonomy alone has seven failure conditions each needing its own named negative test, and the design demands both spellings be proven equal; without this convention that would collapse into one table, which is forbidden here.
- **Code style: explicit error handling, small focused interfaces, descriptive names, `any` over `interface{}`, stdlib idiom** — bears directly on the surface's shape: `Entities() []any` and `Outputs() map[string]any` use `any`; the registry's new "do you know this name?" query is one method rather than a widened interface; and "always handle errors explicitly" is the whole point of replacing silent empties and half-populated values with errors.
- **Follow the established error convention — sentinels matched by identity, each wrapped by a type carrying the detail** — this is also a spec constraint, not merely a convention. It fixes how the seven sentinels are declared and how `state.ResourceNotFoundError` gains its `Is` method, and it rules out introducing a second convention alongside `plugins.ErrNotFound`.
- **Generate test state with a real apply, not a hand-written state file** — applies to the state-format work. The kind/variety split changes what is written to state, and the "silently skipped resource" behaviour must be tested against state a real apply produced, not a fixture hand-written to the new shape. The entry's stated exception (hand-write only when the test is *about* the state format) covers the one case asserting the on-disk shape itself.
- **Never modify dependencies; HCL lives in-tree at `internal/xcl` under MPL-2.0** — a live risk here, because accepting the bare stanza form touches block parsing. If any change reaches `internal/xcl`, the HashiCorp header stays, `// Modifications Copyright (c) Jumppad Labs` is added, and `internal/xcl/UPSTREAM.md` records it. Bulk rename operations stay scoped to first-party files.
- **Dependencies: prefer the standard library** — the whole feature is `reflect`, `errors`, `strings` and `encoding/json`; no third-party dependency is introduced, and none is needed.

Deliberately dropped, with reasons, so the omissions are visible rather than accidental:

- **Database & external services (prepared statements, connection pooling)** — no database and no external service is involved; this is an in-memory lookup surface over already-parsed configuration.
- **Patterns & Architecture: thin handlers, `context.Context` for request-scoped values, graceful shutdown** — no handlers, no requests and no service lifecycle here. The one clause that does apply, dependency injection for testability, is already satisfied: the surface reaches the registry through `Config`'s existing field rather than a global.
- **Development standards: structured logging** — the lookup surface deliberately logs nothing. It is a synchronous library call that returns a value or an error, and logging a caller's failed lookup would be noise in the embedding application's log.
- **Project structure (`/cmd`, `/internal`, `/pkg`, `/api`, `/configs`)** — this repo does not follow that layout: the public API is at the module root and `pkg/` does not exist. The design places the new surface in package `xcl` at the root alongside `Config`, which is where the existing public API lives. Following the convention literally would move the API and break every consumer.

## Architecture & Design Decisions

The work lands in two registered repos. Substantially all of it is in **xclconfig**
(`/home/nicj/code/github.com/jumppad-labs/xcl`): the lookup surface, the data-model split, the
error taxonomy, the parser change, the examples, `README.md`, `docs/state.md` and CI. **xcl-website**
(`/home/nicj/code/github.com/jumppad-labs/xcl-website`) carries only the documentation-site
migration — four `.mdx` pages, six lines, hand-edited. The design document `querier-api-v2` fixes
the shape of all of it: the surface (`Find`, `FindByType`, `FindOne`, `All`, `Entities`, `Outputs`,
`As`), the `entity` vocabulary, the seven-sentinel error taxonomy, and the two-spellings delivery
mechanism. What follows decides how that shape is built, not what it is.

**The data model is split before the surface is built, not alongside it.** The kind/variety split
is the load-bearing change and everything else depends on it: `FindByType[T]("resource","container")`
cannot be implemented correctly while `Meta.Type` holds the variety for `resource` stanzas and the
kind for every other, so a surface-first ordering would build the new API against data that cannot
answer it. The split is also by far the riskiest edit in the plan — `Meta.Type` is a single key
driving HCL expression namespacing, state-file identity, provider dispatch, the resource `ID` and
every event string, across roughly thirty production read sites. Doing it first, on its own, with
the superseded querier minimally adapted to stay green, keeps that blast radius in one reviewable
change and keeps a failure bisectable. A single big-bang change was rejected for exactly that
reason: with no intermediate green state, a regression in the expression language would be
indistinguishable from a regression in the new surface. Both rejected orderings, with citations,
are in `research.md#alternatives-considered-and-rejected`.

**Two consequences of the split need deciding explicitly, because the design fixes the destination
but not the route.** First, HCL expression namespacing is keyed on `Meta.Type` today
(`internal/parser/context.go:126,136`), which is what makes `resource.postgres.main` resolve in a
configuration. Once `Meta.Type` holds `"resource"` for every resource stanza, that key must move to
`Meta.Subtype` for resource-kind entities while staying `Meta.Type` for `variable`, `output` and
`module`. This is invisible from the public API and catastrophic if missed — every cross-resource
reference stops resolving — so it is called out as its own acceptance-tested step rather than folded
into "update the read sites". Second, state written before the split carries the old `meta.type`,
and `state/file_state_store.go:69-73` *silently skips* a resource it cannot resolve rather than
erroring. The spec's non-goals permit regenerating state, so no migration is written; but the silent
skip is turned into a reported error, because "your infrastructure quietly vanished from state" is
the same class of dishonesty this whole spec exists to remove.

**Registration gains a second entry point rather than a changed one.** `TypePath` must be able to
say whether a registered Go type is reached as `{"resource","container"}` or `{"container"}`, and
today `RegisterType(name, resource)` records only a name. Rather than change its meaning — every
existing caller passes a variety intending the kind-led form — `RegisterType` keeps exactly its
current semantics and a sibling entry point registers the bare form, so a type is registered under
one form or the other and never both, as the design requires. Relatedly, the parser's keyword
whitelist (`internal/parser/parser.go:521-531`) needs a "do you know this name?" question that no
single method answers today: `IsRegisteredType` covers `RegisterType` types only, not builtins and
not plugin-provided types. The registry already scans all three sources in `checkTypeName`
(`plugins/registry/plugin_registry.go:89-104`) to *reject* a clash; that scan is inverted into one
query the parser can ask, which both enables the bare stanza form and avoids a fourth place where
the three type sources are enumerated (*small, focused interfaces*; *composition over inheritance*).

**The error taxonomy establishes a convention rather than copying one.** `plugins.ErrNotFound` is
the only error in the repo that implements the sentinel-plus-detail pattern end to end; the typed
errors in `state/`, `types/` and `plugins/registry/` are plain structs with an `Error()` method,
nothing in first-party code implements `Is`, `As` or `Unwrap`, and receivers are inconsistent. The
seven new sentinels therefore follow `plugins/errors.go` deliberately and uniformly — package-level
`var Err… = errors.New(…)`, a doc comment naming who returns it and that it is matched with
`errors.Is`, and a detail struct wrapping it for `errors.As` — and `types.ErrTypeNotRegistered` is
the cautionary counter-example to avoid, being formatted with `%s` and matched nowhere. The one
not-found concept the spec demands is achieved by giving `state.ResourceNotFoundError` an `Is`
method rather than by replacing it, which leaves its seven existing construction sites untouched
while making every one of them answer to `errors.Is(err, ErrNotFound)`; it stays distinct from
`plugins.ErrNotFound`, whose doc comment already says it means the *real* infrastructure is gone
(*always handle errors explicitly*).

**The site migration is hand-edited, and the compile-proof mechanism is deliberately absent.** The
user dropped compile-proofing of documentation-site samples during discovery, which reduces the site
work to four Go code blocks and two prose mentions across four `.mdx` pages, plus the landing page's
`<FeatureCard>` blurb that names `NewQuerier[T]` by name. Site samples are still *written* in the
portable function form so a reader copying one is served, but that is a convention the plan asserts
rather than a property CI verifies. The guard that does run is a case-insensitive search for the
superseded name, which matters because two of the six site mentions are lowercase and invisible to a
capital-Q grep. One clause of the spec's site acceptance criterion is consequently descoped; the plan
states this in the open rather than quietly claiming the criterion is met. On the library side
nothing is descoped: the floor build is proven by a dedicated `GOTOOLCHAIN=go1.25.0 go build ./...`
CI job, which also forces correcting the existing workflow's Go 1.22 pin — below the module's own
declared floor of 1.25.0, and stale today.

## Component Breakdown

- **Lookup implementation (new, package `xcl`)** — owns the actual behaviour of every lookup: resolving an address, matching a positional kind query, deriving addressing from a Go type, enumerating everything, and projecting a published value. It is written once, unexported, and knows nothing about which spelling called it. It reaches the type registry and the current state through the configuration object it is handed, so it needs no new wiring and no accessor added to that object.

- **Portable lookup functions (new, package `xcl`)** — the spelling that compiles on the project's minimum supported Go version. A thin, exported pass-through to the lookup implementation, carrying no logic of its own. This is the form every bundled example, test and documentation sample uses, so that what a reader copies works on the floor toolchain.

- **Lookup methods (new, package `xcl`, excluded below the required toolchain)** — the same surface expressed as generic methods on the configuration, and the form the documentation presents as the destination. It is a second pass-through to the same implementation, selected by build constraint so it compiles where the toolchain allows and disappears where it does not. Because both spellings delegate to one implementation, they cannot drift in behaviour; that they do not is asserted rather than assumed.

- **Query error taxonomy (new, in the project's existing errors package)** — owns the vocabulary of failure: the sentinels callers match by identity, and the detail types that carry the specifics behind each. It lives in the package that already holds the project's error types, so that both the configuration package and the state layer can depend on it without either owning it, and the public package re-exports the sentinels so callers keep matching them by their familiar names. It is what makes an unanswerable question distinguishable from an answer of "none", and it is consumed by every part of the lookup surface. It follows the convention the plugin boundary already established rather than introducing a second one.

- **Type conversion (new exported behaviour, package `xcl`)** — converts an item obtained from untyped enumeration into a caller's Go type, in one call. It owns the verification step that the previous conversion path lacked: where the system knows what an item's type should be, it checks before converting and refuses a mismatch, instead of letting a structural copy return a partially populated value. It is the single conversion path the rest of the lookup surface funnels through.

- **Superseded query helper (removed)** — the construct-then-call helper and its methods are deleted outright rather than deprecated, along with its tests. Its one internal conversion helper is not lost but promoted, in verified form, into the type-conversion component above.

- **Configuration object (changed)** — remains the single entry point consumers hold. It gains the lookup surface and the enumeration and count operations named for the fact that they cover every kind of declaration, with the previous names retained as deprecated aliases so existing calls keep compiling. Its existing responsibilities — parse, validate, apply, destroy, state ownership — are unchanged.

- **Entity metadata (changed)** — the record every declared item carries. It gains a genuine separation between the kind of stanza an item is and its specific variety, populated identically for every declaration rather than one field meaning different things depending on the stanza. This is the change the rest of the plan rests on, and it is also the widest-reaching: this record is read by state persistence, provider dispatch, the lifecycle, event reporting and the expression language.

- **Address representation (changed)** — parses and formats the addresses used throughout. It gains the variety as its own segment, so that an address and a positional kind query share one grammar and an address produced for an item is accepted by a lookup for that same item. It must keep tolerating the address forms already in circulation: module-relative, non-normalised, and carrying a trailing attribute suffix.

- **Expression namespacing (changed)** — the part of parsing that decides which name a declared item answers to inside a configuration's expressions. It currently keys off the single conflated field and must key off the variety for resource-kind items while continuing to key off the kind for the rest. It is called out separately from the other readers of that record because it is the only one whose failure is invisible from the public surface and total in effect.

- **Block parsing (changed)** — decides which leading keywords may open a top-level declaration. It moves from a fixed list to consulting the type registry, so that a declaration written with its variety as the leading keyword is accepted alongside the existing kind-led form, and the two remain distinct types with distinct addresses rather than one being rewritten into the other.

- **Type registry (changed)** — remains the single place types come from. It gains two responsibilities: answering which address segments a registered Go type is reached by, so a lookup can derive addressing without a string; and answering whether a name is a type it knows at all, across every source it draws from, which is the question block parsing now asks. Registration keeps its current meaning, with a sibling entry point for the bare declaration form so a type is registered under one form only. It also keeps its existing inability to reflect a plugin-provided type, which is why a Go-type lookup for one reports that it is not registered and names the query form that does work.

- **Nested-block refusal (changed)** — the existing check that reports a type carrying no addressable identity. Today its result reaches callers as a panic or a silent empty; it becomes the source of the not-an-entity error, and its message must name access through the containing declaration. The check itself already exists and is reused rather than rewritten.

- **State lookup and not-found reporting (changed)** — owns finding a declared item in current state and saying so when there is none. Its existing not-found condition gains identity with the lookup surface's, giving one recognisable not-found concept across configuration lookups while staying distinct from the unrelated condition meaning real infrastructure is absent. It stops answering a metadata failure with a panic.

- **State persistence (changed)** — reads and writes the on-disk record. It must carry both axes, and it stops silently discarding a stored item whose recorded kind it cannot resolve: that becomes a reported failure. No migration of previously written state is provided; regenerating it is acceptable.

- **Published-value projection (changed)** — the apply-time step that resolves a published value into a plain Go value. It is unchanged in when it runs, but it becomes reachable: the lookup surface projects through it so a published value is retrieved by address like anything else, yielding the value rather than the declaration that produced it, and a companion operation returns every published value keyed by address.

- **Bundled examples (changed)** — the three runnable programs. Every lookup in them moves to the portable form. One of them is additionally repaired: it currently claims in both its documentation and its tests to tear down what it applied, but does not, and its tests fail on that today.

- **Library documentation (changed)** — replaces its description of the superseded helper with the new surface, and gains the worked example of retrieving a published value it has never had. The note that lookups remain linear scans is carried forward.

- **Documentation site (changed, separate repo)** — four pages carry the superseded helper, including the landing page's headline example and the feature description beside it; all move to the new surface, written in the portable form. Two of the mentions are lowercase prose and invisible to a case-sensitive search, which the guard covering this work accounts for.

- **Continuous integration (changed)** — gains a job that builds against the pinned minimum supported version on every change, because no contributor exercises that path locally. This also forces correcting the existing configuration's toolchain pin, which currently sits below the module's own declared floor.

- **Project knowledge (changed)** — the architecture entry describing how consumers reach into a parsed configuration still documents the superseded helper, including in its diagram. It is revised to the new surface, through the knowledge workflow's propose-then-confirm rather than written directly.

## Data Structures & Interfaces

**The lookup surface (new, public, package `xcl`)**. Seven operations, each shipping in two
spellings from one unexported implementation. The method form is the documented destination; the
function form is what compiles on the declared minimum Go version and what every example, test and
documentation sample uses. `As` is a function in both worlds — it takes no configuration, so there
is nothing for a method to hang off.

```go
// methods — behind a Go-version build constraint
func (c *Config) Find[T any](id string) (*T, error)
func (c *Config) FindByType[T any](path ...string) ([]*T, error)
func (c *Config) FindOne[T any](path ...string) (*T, error)
func (c *Config) All[T any]() ([]*T, error)

// portable equivalents — no build constraint
func Find[T any](c *Config, id string) (*T, error)
func FindByType[T any](c *Config, path ...string) ([]*T, error)
func FindOne[T any](c *Config, path ...string) (*T, error)
func All[T any](c *Config) ([]*T, error)

// untyped, non-generic — heterogeneous results cannot be typed
func (c *Config) Entities() []any
func (c *Config) Outputs() map[string]any

// conversion — function only
func As[T any](entity any) (*T, error)
```

`FindByType` and `FindOne` take address segments matched positionally from the root: segment one
the kind, segment two the variety. At least one segment is required — the no-segment form is
`All`. A query returning no matches returns an empty result and a nil error; a query that cannot
be answered returns an error. Those two outcomes are never represented the same way, which is why
`Find` returns an error rather than a comma-ok.

**Query errors (new, public, declared in the project's existing errors package and re-exported)**. Sentinels matched by identity with `errors.Is`, each
wrapped by a detail type recoverable with `errors.As`, following the convention the plugin boundary
already set. `ErrNotFound` and `ErrNotUnique` are ordinary outcomes a caller handles; the middle
five mean the question itself had no answer, which is a caller bug.

```go
var (
    ErrNotFound      = errors.New(...) // no entity at that address
    ErrUnknownType   = errors.New(...) // segment one is not a kind
    ErrNotTypeable   = errors.New(...) // the axis spans several Go types
    ErrNotRegistered = errors.New(...) // T has no registered name — a plugin type
    ErrTypeMismatch  = errors.New(...) // the entity is not a T
    ErrNotAnEntity   = errors.New(...) // T is a nested block; it has no address
    ErrNotUnique     = errors.New(...) // two or more matched a single-expected query
)
```

Each detail type carries what a caller needs to act on and wraps its sentinel: the address or
segments queried, the Go type involved, and — for the not-unique case — **the count found**, which
the spec requires be reported. The not-typeable detail additionally names the operation that does
work, so a query for published values points at the call that returns them and a Go-type lookup for
a plugin-provided type points at the kind lookup. Detail types use pointer receivers, matching the
one typed error already matched with `errors.As` in production code.

**Entity metadata (changed, public)**. The single most consequential contract in this plan. One
field currently carries two different axes depending on the stanza; it becomes two fields populated
identically for every declaration without exception.

```go
type Meta struct {
    Type    string // the stanza: "resource", "variable", "output", "module"
    Subtype string // the variety: "container", "postgres"; EMPTY for single-label stanzas
    // ... ID, Name, Module, File, Line, Column, Properties, Links, Parents, Status unchanged
}
```

This is a **serialization boundary**, not just an in-memory change: these fields' JSON tags are the
on-disk state format, and the stored kind is read back to reconstruct each item's Go type on load.
State written before this change is not migrated — regenerating it is acceptable per the spec — but
a stored item whose recorded kind cannot be resolved must now be reported rather than silently
dropped from the loaded state. It is also the key the expression language resolves references
through, so the two fields must be populated before any reference is evaluated.

**Address representation (changed, internal)**. Gains the variety as its own segment, so that
addresses and positional kind queries share one grammar:
`[module...].<type>.<subtype>.<name>[.attribute]`. Parsing and formatting must round-trip: an
address containing a variety segment parses into its parts and formats back to the identical
string, and an address produced for a declaration is accepted by a lookup for that declaration.
Three existing input forms must keep resolving — module-relative, non-normalised, and carrying a
trailing attribute suffix such as one taken verbatim from a stored reference between items.

**Type registry (changed, public)**. Two new questions, and one new way in.

```go
// the address segments a registered Go type is reached by:
// {"resource", "container"} kind-led, or {"container"} bare. Never both.
func (r *PluginRegistry) TypePath(t reflect.Type) ([]string, bool)

// is this name a type this registry knows, from any of its sources?
func (r *PluginRegistry) KnownType(name string) bool
```

`TypePath` is what lets a Go-type lookup derive its addressing without the caller restating a name
as a string. It answers only for types registered from a real Go type; a plugin-provided type exists
host-side as a schema with no Go type to reflect against, so it is reported as not registered rather
than guessed at. `KnownType` spans every source the registry draws from — builtin, registered and
plugin-provided — and is the question block parsing asks before accepting a leading keyword. It is
deliberately a new method rather than a widening of the existing registered-type check, whose
current narrow meaning the lifecycle depends on.

Registration keeps its present meaning and gains a sibling for the bare declaration form, so the
form is explicit at the call site and a type is registered under one form only:

```go
func (r *PluginRegistry) RegisterType(name string, resource any) error     // unchanged: kind-led
func (r *PluginRegistry) RegisterBareType(name string, resource any) error // new: bare form
```

**Configuration object (changed, public)**. Gains the lookup surface above, plus enumeration and
counting renamed for what they actually hold, with the previous names kept as deprecated aliases
returning identical results.

```go
func (c *Config) Entities() []any  // was GetResources, retained as a deprecated alias
func (c *Config) EntityCount() int // was ResourceCount, retained as a deprecated alias
```

**Not-found identity (changed, public)**. All seven sentinels are declared in the package that already holds this project's error types, and re-exported from the public configuration package so a caller writes the name they expect while the standard library's `errors` stays unaliased in their own code. That placement is what makes one not-found concept possible at all: the configuration package imports the state layer, so the state layer cannot import it back, and a neutral home both can depend on removes the cycle without either side owning the vocabulary. The state layer's existing not-found error gains an `Is`
method so it answers to the lookup surface's `ErrNotFound`, giving one recognisable not-found
concept across configuration lookups without touching its seven construction sites or its existing
`errors.As` callers. It keeps its value receiver for that reason. It stays distinct from the
plugin-boundary not-found condition, which means the real infrastructure is gone and triggers a
recreate — same words, different meaning, and both doc comments must say so.

**Removed from the public surface.** The construct-then-call query helper, its constructor and its
two methods are deleted. Building code that constructs it no longer compiles.

## Implementation Detail

**One implementation, two spellings, selected by build constraint.** This is the one genuinely new
pattern the plan introduces, and it is new to first-party code here: the repo splits files by
operating system today, but has never split one by language version. Every lookup is written once as
an unexported function taking the configuration as an ordinary parameter. Two exported layers sit on
top — package-level functions with no constraint, and generic methods in a file excluded below the
required toolchain — and neither carries logic. A developer adding an eighth operation therefore
writes it three times by signature but once by behaviour, and the reviewer's question is only
whether the two pass-throughs match. Because the constraint raises the language version for its file
alone, the method file simply drops out on the floor toolchain rather than failing to build. The
equality of the two forms is asserted by tests that run where both exist, rather than argued for.

**A lookup becomes a pipeline with one honest answer at each stage.** The superseded helper did one
linear scan and one structural copy, and both stages could fail quietly — a scan that matched
nothing returned an empty result whether or not the question made sense, and the copy returned a
half-populated value alongside its error. The replacement separates resolving from converting, and
gives each stage the vocabulary to refuse. Resolution decides whether the question is answerable at
all: whether a segment names a kind, whether the axis pins exactly one Go type, whether the Go type
is one the registry can reach. Only a resolved, answerable query reaches conversion, and conversion
verifies before it converts rather than after. The practical shape for a reader is that the error
returned names which stage refused, and an empty result now means exactly one thing.

**Conversion gains a gate in front of the existing structural copy.** The copy itself is unchanged —
a value that already is the requested type is returned as-is, and anything else goes through the
existing structural round-trip. What is new is that where the system knows what an item's type
should be, that is checked first and a mismatch is refused outright. This matters because the
conversion helper is being promoted from unexported to public: exporting it invites callers to run
it across an untyped enumeration, which is precisely the loop where a silent half-populated value
would be produced today. The gate is what makes exporting it safe rather than a new way to get
garbage.

**Splitting the two axes is a data-model migration disguised as a field addition.** The metadata
record's single conflated field is read in roughly thirty places, and they do not all want the same
half. Three groups need distinguishing carefully: readers that want the stanza keyword, readers that
want the variety, and readers that want whichever the old field happened to hold and will silently
keep compiling either way. The third group is the hazard, because the compiler cannot help — both
fields are strings. The working method is therefore to change the field's meaning and let the
compiler find nothing, then walk every reader deliberately rather than relying on the build. Two of
those readers are load-bearing beyond the Go API and are treated as their own units of work: the
expression language resolves configuration references through this field, and the persisted state
format stores it and reads it back to reconstruct each item's type on load.

**Silent degradation is replaced by reported failure at the state boundary.** The loader currently
skips a stored item whose recorded kind it cannot resolve, so a stale state file quietly yields a
smaller configuration rather than an error. That is the same failure mode the whole feature exists
to remove, one layer down, and the axis split makes it newly reachable. No migration is written —
regenerating state is explicitly acceptable — but the skip becomes a reported failure, so a state
file from before the change announces itself instead of half-loading.

**Block parsing moves from a closed list to a question.** Accepting a declaration written with its
variety as the leading keyword means the parser stops matching against four hard-coded keywords and
starts asking the registry whether it knows a name. The registry already performs exactly that
three-source scan in order to reject duplicate registrations; the plan inverts it into a query
rather than writing a fourth enumeration of where types come from. The visible consequence in the
parser is that the arm which used to produce "only these four stanzas are valid" now produces an
unknown-type error naming the keyword, which is the same information with a better reason attached.

**Addressing and querying converge on one grammar.** Adding the variety as its own address segment
is what lets a positional kind query and a written address mean the same thing, so that segment one
is always the kind and a variety alone matches nothing rather than matching in the wrong position.
The constraint a reader should hold onto is round-tripping: an address formatted for a declaration
has to parse back into the same parts and be accepted by a lookup for that declaration. The existing
parser is more permissive than its formatter in ways that predate this work, and the plan's job is
to add the segment without widening that gap.

**Errors follow one convention, established properly for the first time.** The pattern — a
package-level sentinel matched by identity, wrapped by a type carrying the specifics — already
exists at the plugin boundary and is documented there, but nothing else in first-party code
implements it fully; the typed errors elsewhere are plain structs that happen to have an `Error`
method. The seven new sentinels follow the documented pattern uniformly and are declared together in the
package that already holds this project's error types, so one vocabulary lives in one place and both
the configuration package and the state layer can reach it. The existing state-layer
not-found error is brought into it by gaining identity with the new one rather than being replaced,
so its existing callers and construction sites are untouched. The trap the plan explicitly avoids is
the one the repo already contains: a typed error formatted into a message with `%s` instead of
wrapped, which is unrecoverable by the caller and matched nowhere.

**Consumers converge on the portable spelling.** Every bundled example and test, and every
documentation sample in both repos, is written in the function form, so what a reader copies compiles
on the declared minimum version. The method form is what the prose presents as the destination. This
is a deliberate asymmetry between what is documented as idiomatic and what is shown in runnable code,
and the documentation has to say why, or the next contributor will "fix" the inconsistency.

**What is not being introduced.** No index or lookup cache — every lookup stays a linear scan, which
is adequate at one configuration's scale and is stated in the documentation so the next reader does
not assume otherwise. No dependency-ordered public walk, no new package, and no third-party
dependency: the whole feature is reflection, errors and the standard library, and the surface lives
in the existing public package beside the configuration object it hangs off.

## Dependencies

**Design documents this plan was built on**

- **`querier-api-v2.md`, from the `design` design source** — the settled shape this plan implements: the seven-operation lookup surface and its exact signatures, the `entity` vocabulary, the kind/variety split of the metadata record, the seven-sentinel error taxonomy, the two-spellings delivery mechanism and its build constraint, and the inventory of files to change. Binding. Architecture is built on it rather than re-derived. One divergence, resolved in the spec's favour and flagged here because it is the only place the two disagree: the design lists implementing the bare declaration form under "Not in scope", while the spec requires it and calls it a facilitating change. The spec's own constraint is that the design governs *how* and the spec governs *what must be true*, so the bare form is in scope for this plan.

**Runtime and language dependencies**

- **Go 1.27 generic methods** — the language feature the method spelling needs. Not a dependency to install: it is why the surface ships twice. Verified available on this machine (the default toolchain is 1.27.0) and verified that the floor toolchain resolves from local cache, so both halves can be exercised during development.
- **Go 1.25.0, the declared minimum** — must remain the module's floor; raising it is forbidden by a spec constraint. The floor is what the portable spelling exists for and what the new CI job proves. No `go.mod` change.
- **Standard library only (`reflect`, `errors`, `encoding/json`, `strings`)** — the whole feature. `reflect` backs deriving addressing from a Go type; `errors` backs the sentinel-and-detail convention; the existing structural conversion continues to use `encoding/json`. **No new third-party dependency is introduced**, consistent with the convention preferring the standard library.
- **testify `require`** — the existing and only assertion library, used for every new test. Already present; no change.

**Internal packages this plan depends on, and whether they change**

- **The public configuration package** — hosts the new surface beside the existing configuration object. **Changes**: gains the lookup surface in both spellings, the error taxonomy, the renamed enumeration and count operations with deprecated aliases, and loses the superseded query helper and its tests entirely.
- **Entity metadata and its reflection helpers** — provide access to each declared item's record, including through nested embedding. **Changes**: the record gains the second axis. The existing helper that reports a type carrying no addressable identity is reused unchanged as the source of the not-an-entity error, rather than a new check being written.
- **Address parsing and formatting** — provides the address grammar every lookup resolves through. **Changes**: gains the variety segment in both parsing and formatting, and must round-trip.
- **The parser (block parsing, expression context, lifecycle, callbacks)** — populates the metadata record and resolves references. **Changes**: block parsing consults the registry instead of a fixed keyword list; the expression namespace key moves to the variety for resource-kind items; every reader of the conflated field is walked. The published-value projection it performs at apply time is unchanged in behaviour but becomes reachable through the lookup surface.
- **The type registry** — the single source of types, already holding builtins, registered Go types and plugin-provided schemas. **Changes**: gains the type-to-address-segments question, a single "is this a known name" question spanning all three sources, and a sibling registration entry point for the bare declaration form. Its existing narrow registered-type check is deliberately left alone because the lifecycle depends on its current meaning.
- **State: lookup, error reporting and persistence** — provides finding a declared item and the on-disk format. **Changes**: the not-found error gains identity with the lookup surface's; a metadata failure stops being a panic; the persisted record carries both axes; and a stored item whose recorded kind cannot be resolved is reported rather than silently dropped.
- **The project's existing `errors` package** — already holds `ConfigError` and `ParserError`. **Changes**: gains the seven query sentinels and their detail types. It is the neutral home that lets the state layer and the configuration package share one not-found concept, and it is safe to depend on from both: it imports only the in-repo HCL fork and one third-party wrapping helper, so neither direction creates a cycle.
- **The plugin boundary (`plugins`, its wire format and hosts)** — already carries the type-and-subtype taxonomy this plan adopts, and supplies the error convention being followed. **No change**: its vocabulary is the reason the naming here is what it is, and its not-found condition must stay distinct from the configuration-lookup one.
- **The structural conversion helper** — the existing round-trip that fills a caller's type. **No change** to the conversion itself; it simply gains a verification gate in front of it.
- **The in-repo HCL fork** — parsing lives here. **Change only if unavoidable.** Accepting the bare declaration form may not need to reach it at all, but if it does, the fork's licence rules bind: keep the upstream header, add the modification notice, and record the change in the fork's upstream notes. Bulk rename operations stay scoped to first-party files.
- **The three bundled example programs** — the runnable proof of the surface. **Change**: every lookup moves to the portable spelling. One additionally needs repairing: it does not do the teardown its own documentation and tests claim, and those tests fail today.

**Planning dependencies**

- **Spec `20260921093100-query-api-v2`** — the source of requirements and acceptance criteria. Final and closed. Governs what must be true where it and the design differ.
- **Plan `20260919120639-config-only-types-and-examples`** — the immediate predecessor, already landed. It introduced registration of plain Go types, the type-name clash gate whose three-source scan this plan inverts into a query, and the three example programs with the shared runnable shape this plan migrates. It also last changed the superseded helper's list-by-type method, and its recorded reasoning for that change is precisely what the new type-to-address-segments question now answers. **Nothing needs to land first** — it is already in the tree.
- **Project knowledge, `xclconfig` repo tier** — the `entity` glossary entry is binding on naming and independently mandates the renamed enumeration and count operations. The conventions entries bind the testing style, the error convention, and the fork's licence rules. The architecture entry describing how consumers reach into a parsed configuration still documents the superseded helper and is **revised by this plan** through the knowledge workflow's propose-then-confirm.

**Explicitly not depended on**

- **A documentation-site sample-compilation mechanism** — considered and dropped by decision during planning. The site keeps its existing manual policy, so this plan introduces no Go module, no Go toolchain and no snippet tooling into the site repo. One clause of a spec acceptance criterion is descoped as a result, stated plainly in the testing approach rather than quietly passed over.
- **A state migration** — regenerating state written before this change is acceptable per the spec's non-goals, so none is written.
- **A fix for the plugin example's pre-built-binary test failure** — pre-existing, unrelated, and left tracked separately by decision. It means a full-suite run is not currently a clean signal, which the testing approach accounts for rather than assumes away.

## Testing Approach

**Test types.** The bulk is unit testing of the lookup surface and the error taxonomy against a
configuration applied from real fixtures, in the same internal-package style the superseded helper's
tests already use. Around that sit: integration tests at the parser level for the two-axis split, the
bare declaration form and reference resolution, because those need a real parse and a real apply to
mean anything; state round-trip tests covering both the new persisted shape and the refusal to load a
record whose kind cannot be resolved; equivalence tests asserting the two spellings agree; and
end-to-end tests of the three bundled example programs, which already run as ordinary package tests
and therefore fail the normal test run when an example breaks. Everything follows the house
conventions without exception: testify `require`, one behaviour per named test function, positive and
negative cases in **separate** functions, no table-driven tests anywhere, and verbosity preferred over
shared abstraction. State needed by a re-apply or removal test is produced by a real first apply
rather than hand-written, except where the test is *about* the on-disk shape itself, which is the
documented exception to that convention.

**Where coverage concentrates, and why.** Three areas carry most of it.

The **error taxonomy** is densest, because it is the whole point of the feature: seven failure
conditions, each needing its own named negative test asserting that *that specific* reason is
recognisable and its detail recoverable, plus the paired positive test that a well-formed query
matching nothing returns an empty result and a nil error. The convention forbidding mixed
positive/negative functions is what keeps these honest — the distinction being tested is precisely
between two outcomes that used to look identical, so they cannot share a test.

The **two-axis split** is next, because it is the widest-reaching change and the compiler cannot help:
both axes are strings, so a reader left on the wrong one keeps building. Coverage therefore targets the
readers whose failure is invisible from the public surface — reference resolution through the
expression language, provider dispatch, the provider-less short-circuit, the persisted record, and the
identity used to find an item in state — rather than only the new lookups.

**Conversion** is third: every registered type converted to a non-corresponding type, asserting an
error and no value, because the failure being prevented is a returned struct whose fields are quietly
unset.

**Load-bearing assertions, in plain language.**

- A lookup by address returns a populated value of the caller's type from a single call expression, for a module-relative address, a non-normalised one, and one carrying a trailing attribute suffix taken verbatim from a stored reference — all three resolving to the same item.
- A kind lookup returns every match and only matches; a kind lookup for a variety with nothing declared returns empty with no error; a variety name alone matches nothing and fails rather than matching in the wrong position.
- The single-expected lookup discriminates all three outcomes, and the more-than-one outcome reports how many were found.
- A registered Go type is queryable with no string at all, returning what the equivalent kind lookup returns; a plugin-provided type instead reports that it is not registered and names the query form that does work.
- Enumeration covers every kind of declaration — resource-kind items, variables, published values and modules alike — and its count matches the declarations present.
- A published value is retrieved by address like anything else, yields the **value** rather than the declaration that produced it, resolves inside a module only by its full address, and a bare name fails rather than being inferred.
- Every declaration reports both axes, with an empty variety for single-label stanzas, and no declaration reports its variety in place of its kind.
- An address containing a variety segment parses and formats back to the identical string, and an address produced for a declaration is accepted by a lookup for it.
- The two stanza forms are different types with different addresses: each is returned by its own kind lookup and neither by the other's.
- A nested block query is refused with an error naming access through the containing declaration, not answered with an empty result.
- A not-found outcome from stored state and one from the lookup surface are recognisable as the same condition, while the unrelated condition meaning real infrastructure is absent stays distinguishable from both.
- Both spellings return equal values and equal errors for identical inputs, asserted on a toolchain where both exist.
- **Regression guard**: existing parser, plugin, lifecycle, state and example behaviour is unchanged apart from the call sites and metadata readers this plan deliberately touches. Reference resolution across a configuration is the specific thing to watch, since the expression language keys off a field whose meaning changes.

**Success metrics — how each is verified.**

- *"Every one of the existing lookup call sites — 8 in examples, 7 in tests — reduces to a single call expression, with none requiring more code after migration than before."* — **Behavioural test** for the call-site half: the repository is asserted to contain no construction of the superseded helper, and every example and test compiles and passes using the new surface, which cannot be true unless all 15 were migrated. The "no more code than before" comparison is **manual — captured in the implementation test plan**; it is a property of the diff, not of the running code.
- *"Zero query forms answer an unanswerable question with an empty result: each of the seven identified failure conditions is covered by a test asserting its specific error, including the kind-axis query that returns silently empty today."* — **Behavioural test.** This is the error-taxonomy coverage above, one named test per condition. The kind-axis query that silently returns empty today gets an explicit test asserting it now returns the not-typeable error, so the specific regression this metric names is pinned.
- *"Zero paths return a partially populated value in place of an error, evidenced by mismatch tests covering conversion of each registered type to a non-corresponding type."* — **Behavioural test.** Each registered type is converted to a non-corresponding type and asserted to produce an error and no value; repeated across the set, so a single leaked path fails the suite.
- *"Retrieving a published value requires no knowledge of how it is stored, evidenced by at least one bundled example and one documentation sample that retrieves one using an address alone."* — **Split.** The bundled example is a **behavioural test**: an example retrieves a published value by address and its test asserts the returned value, and that example runs in the normal test run. The documentation sample is **manual — captured in the implementation test plan**, because sample compilation was dropped from scope; its presence and correctness are reviewed, not executed.
- *"A search for the superseded query helper across the library and the documentation site returns zero matches."* — **Behavioural test** for the library, where absence is additionally guaranteed by the code not compiling if any caller remained. For the documentation site this search is the **only** guard, since samples are no longer compile-checked; it must be case-insensitive, because some site mentions are lowercase prose that a case-sensitive search misses entirely.
- *"The minimum-supported-version build job passes on every change from merge onward, with no period in which it is absent or skipped."* — **Split.** That the job exists, is wired to run on every change, and passes against the pinned floor is a **behavioural test** — it is CI configuration plus a green run, and it also forces correcting the existing workflow's toolchain pin, which currently sits below the module's own declared floor. The "from merge onward, with no period absent or skipped" clause is **manual — captured in the implementation test plan**: it is a claim about the repository's history after this lands, not something a test can assert at implementation time.

**Deliberate gaps.**

- **Documentation-site samples are not compile-checked.** Dropped by decision during planning; the site keeps its existing manual "update the pages to match" policy. This descopes one clause of the spec's site acceptance criterion — the clause requiring every sample to compile under the pinned minimum version. The other clauses of that criterion, that a search finds no superseded mentions and that the landing page's headline example and feature description both show the new surface, remain fully in scope and are verified. Stated here in the open so plan and spec do not silently disagree.
- **One example program's test failure is pre-existing and stays.** The plugin example's end-to-end test requires a binary that a fresh checkout does not build, so a full-suite run is not currently a clean signal. Left tracked separately by decision. The consequence for this plan is that the new floor CI job is scoped to building rather than to running the whole suite, and no phase may claim "the suite is green" without naming this known failure.
- **A second example program's failure is *not* a gap — it is fixed here.** That example's own tests already assert teardown behaviour its code does not perform; the plan repairs the code, and those existing tests become the verification.
- **No performance or scale testing.** Every lookup remains a linear scan by design, indexing is an explicit non-goal, and there is no latency or throughput requirement to assert.
- **No new mocks.** The surface is exercised against real applied configurations, which is both more honest and cheaper here than mocking the registry or state; the existing state-store mock continues to serve the tests that already use it.

## Milestones & Phases

### Milestone 1: Every declaration says both what kind it is and what variety it is

**What changes**: A configuration's declarations stop reporting one thing in place of another.
Every declared item — a resource, a variable, a published value, a module alike — now reports the
kind of stanza it is and its specific variety independently, with the variety empty where a stanza
carries a single label. Addresses gain the variety as a segment of their own, so an address and a
positional query finally agree on one grammar and an address written for an item is accepted by a
lookup for that item. Configuration authors also gain a second way to declare a top-level item,
using its variety as the leading keyword with a single label, which the system treats as a
genuinely different type with a different address rather than a rewriting of the existing form.
Alongside this, a saved configuration that can no longer be understood says so: a stored item whose
recorded kind cannot be resolved is reported instead of being quietly dropped from what gets loaded.

Much of this milestone is groundwork whose benefit only becomes visible in the next one, and it is
worth its own milestone for exactly that reason. The field it changes is read in roughly thirty
places and reaches well beyond the lookup API — it is how references between items resolve inside a
configuration, how saved state identifies what it holds, and how work is routed to the right
provider. Landing it alone, and proving the existing behaviour still holds, is what makes the
following milestone reviewable as an API change rather than as an API change tangled with a data
migration.

**Validation point**: An existing configuration still parses, applies, resolves every reference
between its items, and round-trips through saved state unchanged. Every declaration exposes both
axes, with a resource-kind declaration additionally exposing its variety and a variable, published
value and module each exposing an empty one. An address containing a variety segment parses and
formats back to the identical string. A configuration declaring one item with a leading kind keyword
and another of the same variety with the variety as the leading keyword yields two declarations with
different addresses. A saved configuration whose recorded kind cannot be resolved produces an error
rather than a short list. The existing test suite passes, excepting the one pre-existing failure this
plan does not own.


#### - [ ] Phase 1.1: Record kind and variety separately on every declaration

**Repo:** xclconfig

Every declared item gains a genuine separation between the kind of stanza it is and its specific
variety, populated the same way for every stanza rather than one field meaning different things
depending on which stanza produced it. Items declared with a single label report an empty variety.
This is the change the rest of the plan rests on, and it is also the one the compiler cannot help
with — both axes are text, so every place that reads the old field keeps building whether or not it
now reads the right one. Those readers are therefore walked deliberately, one at a time, with the
reference-resolution path treated separately in the next phase because its failure is invisible from
the outside.

*Technical detail:* [context.md#phase-11](./context.md#phase-11-record-kind-and-variety-separately-on-every-declaration)

**Acceptance criteria**:
- [ ] A resource-kind declaration reports `resource` as its kind and its declared variety separately, and neither is reported in place of the other.
- [ ] A variable, a published value and a module each report their own kind and an empty variety.
- [ ] Work is still routed to the correct provider for every resource that has one, and resources handled without a provider are still handled without one.
- [ ] Applying, re-applying and destroying an existing configuration behave exactly as they did before this phase.
- [ ] Every event reported while applying a configuration still names the item it concerns.

#### - [ ] Phase 1.2: Keep references between items resolving

**Repo:** xclconfig

References written inside a configuration — one item reading another's field, a module output, a
variable — are resolved through the same field this plan has just changed the meaning of. This phase
moves that resolution onto the variety for resource-kind items while leaving it on the kind for
everything else. It is separated from the rest of the readers because it is the only one whose
failure is both total and silent from the outside: nothing about the library's surface would look
wrong, and every configuration that refers to anything would stop working.

*Technical detail:* [context.md#phase-12](./context.md#phase-12-keep-references-between-items-resolving)

**Acceptance criteria**:
- [ ] An item that reads a field of another item receives that field's value.
- [ ] An item that reads a variable, and an item that reads a module's published value, each receive the value.
- [ ] A reference that names something undefined is still reported as undefined, naming the item that made it.
- [ ] A configuration whose items depend on one another is still processed in dependency order.

#### - [ ] Phase 1.3: Carry the variety as its own address segment

**Repo:** xclconfig

Addresses gain the variety as a segment of their own, so that a written address and a positional
query share one grammar and segment one is always the kind. Parsing and formatting must agree: an
address produced for a declaration has to parse back into the same parts and be accepted by a lookup
for that same declaration. The three address forms already in circulation keep working — a
module-relative address, one that is not normalised, and one carrying a trailing attribute suffix
such as those stored in references between items.

*Technical detail:* [context.md#phase-13](./context.md#phase-13-carry-the-variety-as-its-own-address-segment)

**Acceptance criteria**:
- [ ] An address containing a variety segment parses into its parts and formats back to the identical string.
- [ ] The address produced for a declaration is accepted as the address of that declaration.
- [ ] A module-relative address, a non-normalised address, and an address carrying a trailing attribute suffix all resolve to the item they name.
- [ ] Addresses for variables, published values and modules are unchanged in form.

#### - [ ] Phase 1.4: Accept a declaration led by its variety

**Repo:** xclconfig

Configuration authors can declare a top-level item using its variety as the leading keyword with a
single label, alongside the existing form led by a kind keyword. The two are different types with
different addresses; neither is rewritten into the other, and a Go type is registered under one form
or the other, never both. The parser stops checking a fixed list of four keywords and instead asks
the type registry whether it knows the name, which also means an unrecognised keyword now fails by
saying it is not a known type rather than by listing the four stanzas that used to be allowed.

*Technical detail:* [context.md#phase-14](./context.md#phase-14-accept-a-declaration-led-by-its-variety)

**Acceptance criteria**:
- [ ] A configuration declaring a top-level item with its variety as the leading keyword and a single label parses without error.
- [ ] That declaration reports the leading keyword as its kind and an empty variety.
- [ ] A configuration declaring one item in each form yields two declarations with different addresses.
- [ ] A leading keyword that names nothing the system knows is rejected with an error naming that keyword.
- [ ] Every existing configuration continues to parse exactly as before.

#### - [ ] Phase 1.5: Report saved state that can no longer be understood

**Repo:** xclconfig

Loading a saved configuration whose recorded kind cannot be resolved currently drops that item
silently, so a stale saved file yields a smaller configuration rather than an error. Splitting the
two axes makes that newly reachable, and a configuration quietly losing items is the same dishonesty
this whole piece of work exists to remove. Loading such a file now fails and says what it could not
resolve. No migration of previously saved state is provided — regenerating it is acceptable.

*Technical detail:* [context.md#phase-15](./context.md#phase-15-report-saved-state-that-can-no-longer-be-understood)

**Acceptance criteria**:
- [ ] Loading a saved configuration containing an item whose recorded kind cannot be resolved fails with an error naming what could not be resolved.
- [ ] Loading a saved configuration written by the current version succeeds and returns every item it holds.
- [ ] A saved configuration round-trips: what is written is what comes back, with both axes intact.

### Milestone 2: Ask the configuration directly, and get an honest answer

**What changes**: Reaching into a parsed configuration becomes a single call instead of two. A
developer asks the configuration itself for one item by its address, for everything of a given kind,
for the one item they expect there to be exactly one of, or for everything of a Go type without
restating its name as a string — each as their own Go type, with no throwaway helper constructed
first and no per-type setup between consecutive lookups of different types. Everything a
configuration declares can be enumerated without naming a type, and an item from that enumeration
converts to a Go type in one call. Values a configuration publishes for its consumers join the same
lookup: one is retrieved by its address like anything else, yields the value itself rather than the
declaration that produced it, and all of them can be obtained together keyed by address.

The more important change is what happens when the answer is no. A question that is well-formed but
matches nothing returns an empty result and no error; a question that cannot be answered returns an
error, and these two are never represented the same way again. Each reason for failure is separately
recognisable and carries its detail: the address matched nothing, the named kind is not a kind, the
query spans more than one Go type, the Go type was never registered, the item is not of the
requested type, the requested type is not addressable at all, or more than one thing matched a query
expecting one — which reports how many it found. Asking for a block that only exists nested inside
another declaration now says so, naming the declaration it is reached through, instead of replying
that you have none. Converting an item to a type that does not correspond to it is refused rather
than quietly returning a value with half its fields unset. Every one of these is available both as a
method and in a portable form that behaves identically on the project's oldest supported toolchain,
and the superseded helper is gone rather than deprecated.

**Validation point**: Each lookup returns a populated value of the requested type from a single call
expression, tolerating module-relative, non-normalised and attribute-suffixed addresses alike. Each
of the seven failure reasons is distinguishable from every other and its detail recoverable, with the
kind-axis query that returns silently empty today now returning an error. Every registered type
converted to a non-corresponding type errors and yields no value. A published value is retrieved by
address, a module's only by its full address, and a bare name fails. Both spellings return equal
values and equal errors for identical inputs. Code constructing the superseded helper no longer
compiles.


#### - [ ] Phase 2.1: One vocabulary for every way a lookup can fail

**Repo:** xclconfig

Every way a lookup can fail gets a name a caller can test for, and a carrier for the detail behind
it. Seven conditions are covered: the address matched nothing, the named kind is not a kind, the
query spans more than one Go type, the Go type was never registered, the item is not of the requested
type, the requested type is not addressable at all, and more than one item matched a query expecting
one — which reports how many. The existing not-found condition raised when an item is absent from
saved state is brought into the same identity, so a failure to find a declared item reads as one
condition across the whole lookup surface, while staying distinct from the unrelated condition that
means real infrastructure has gone missing.

*Technical detail:* [context.md#phase-21](./context.md#phase-21-one-vocabulary-for-every-way-a-lookup-can-fail)

**Acceptance criteria**:
- [ ] Each of the seven failure reasons is distinguishable programmatically from every other.
- [ ] The detail behind each failure is recoverable by the caller, including the number found when more than one matched a query expecting one.
- [ ] A not-found result originating from saved state and one originating from the lookup surface are recognisable as the same condition.
- [ ] The condition meaning real infrastructure is absent remains distinguishable from both, and both are documented as meaning different things.

#### - [ ] Phase 2.2: Look up one item by address, published values included

**Repo:** xclconfig

A developer retrieves one declared item by its address, as their own Go type, in a single call with
nothing constructed first. Values a configuration publishes for its consumers are reached the same
way: a published value's address returns the resolved value itself, converted to the type asked for,
rather than the declaration that produced it — so a caller no longer needs to know such values are
held differently in order to ask for one. A full address is required; a bare name is not an address
and is not inferred. This phase also introduces conversion of an item to a caller's Go type as a
public operation, with the verification that the old conversion path lacked: where the system knows
what an item's type should be, a mismatch is refused instead of returning a value with half its
fields unset.

*Technical detail:* [context.md#phase-22](./context.md#phase-22-look-up-one-item-by-address-published-values-included)

**Acceptance criteria**:
- [ ] An address lookup returns a populated value of the requested Go type and no error, in a single call expression with no intermediate value constructed first.
- [ ] A module-relative address, a non-normalised address, and an address carrying a trailing attribute suffix all resolve to the same item.
- [ ] An address lookup for a published value returns that value converted to the requested type — not the declaration that produced it, and not a zero value.
- [ ] A published value declared inside a module resolves by its complete address, and a lookup of the bare name alone fails.
- [ ] Converting an item to a Go type that does not correspond to it returns an error and no value, never a partially populated one.
- [ ] An address matching nothing returns a recognisable not-found error rather than an empty value.

#### - [ ] Phase 2.3: Look up every item of a kind, and the one item expected

**Repo:** xclconfig

A developer retrieves all declared items matching a kind by supplying the same leading address
segments those items' addresses use, and separately asks for the one item they expect a configuration
to declare exactly one of, receiving it directly instead of a collection to index into. This is where
the silent empty is finally closed: a query naming only the kind, which spans several Go types, now
says so rather than returning nothing; a variety name with no kind before it fails rather than
matching in the wrong position; and a query for a block that exists only nested inside another
declaration is refused with an error naming the declaration it is reached through, instead of
replying that you declared none.

*Technical detail:* [context.md#phase-23](./context.md#phase-23-look-up-every-item-of-a-kind-and-the-one-item-expected)

**Acceptance criteria**:
- [ ] A kind lookup naming the kind and a variety returns exactly the items of that variety and no others.
- [ ] A kind lookup naming a variety with nothing declared returns an empty result and no error.
- [ ] A kind lookup given only a variety name, with no kind before it, fails rather than returning items of that variety.
- [ ] A kind lookup naming a kind that spans several Go types returns an error rather than the empty result it returns today, and a lookup naming published values returns the same error while naming the call that does return them.
- [ ] The single-expected lookup returns the item when exactly one matches, a not-found error when none match, and an error reporting the count when two or more match.
- [ ] A kind lookup naming a block that exists only nested inside another declaration is refused with an error naming access through the containing declaration.

#### - [ ] Phase 2.4: Look up by Go type, and enumerate everything declared

**Repo:** xclconfig

Where a Go type has been registered, a developer retrieves every declared item of that type without
restating its name as a string, with the addressing worked out from the registration. A plugin
provides only a schema and no Go type, so a type living inside a plugin cannot be matched this way
and says so, naming the query form that does work. Separately, everything a configuration declares
can be enumerated without naming a type at all — covering every kind, not only resources — and every
published value can be obtained together, keyed by address. The operations that enumerate and count
are renamed for the fact that they cover every kind of declaration, with their previous names kept
as deprecated aliases.

*Technical detail:* [context.md#phase-24](./context.md#phase-24-look-up-by-go-type-and-enumerate-everything-declared)

**Acceptance criteria**:
- [ ] A lookup naming only a registered Go type, passing no address segments, returns every declared item of that type, matching what the equivalent kind lookup returns.
- [ ] A Go-type lookup for a type that exists only inside a plugin reports that it is not registered and names the kind-lookup form as the alternative.
- [ ] A Go-type lookup for a nested block's type is refused with the same not-addressable error the kind lookup gives.
- [ ] Enumerating everything declared yields entries for resource-kind items, variables, published values and modules alike, and the count matches the number of declarations.
- [ ] An item taken from that enumeration converts to its Go type in a single call, giving a value equal to the one an address lookup returns.
- [ ] A single call returns every published value keyed by address, one entry per published declaration.
- [ ] The enumeration and count operations are available under their new names, and calls written against their previous names still compile and return identical results.

#### - [ ] Phase 2.5: Ship both spellings and remove the superseded helper

**Repo:** xclconfig

The whole surface becomes available twice from one implementation: as methods on the configuration,
which is the documented destination, and in a portable form that compiles and behaves identically on
the project's declared minimum toolchain, so nobody loses capability by not having the newest Go. The
two cannot drift because neither carries logic, and that they do not is asserted rather than assumed.
With the replacement complete, the superseded construct-then-call helper and its tests are deleted
outright rather than deprecated alongside it.

*Technical detail:* [context.md#phase-25](./context.md#phase-25-ship-both-spellings-and-remove-the-superseded-helper)

**Acceptance criteria**:
- [ ] Every lookup is available both as a method and in a portable form.
- [ ] For every lookup, the two forms return equal values and equal errors for identical inputs, asserted where both are available.
- [ ] The library builds and its tests pass with the toolchain pinned to the project's declared minimum version, with the portable form available and exercised.
- [ ] Consecutive lookups of different Go types against the same configuration both succeed with no setup call between them.
- [ ] Code constructing the superseded helper no longer compiles, and no reference to it remains anywhere in the repository.

### Milestone 3: Everything bundled uses the new surface, and the oldest supported toolchain is proven

**What changes**: Every example and test shipped with the library uses the new lookup surface,
written in the form that compiles on the project's declared minimum toolchain, so what a developer
copies out of the repository works for them rather than requiring a newer Go than they have. One of
the bundled examples additionally starts doing what it has always claimed to do: it currently
describes tearing down what it applied, and is tested as though it does, but does not — that is
repaired here, since the example is being rewritten for the new surface anyway. At least one example
demonstrates retrieving a published value by address alone, which nothing in the repository shows
today. Finally, the build against the oldest supported toolchain is checked automatically on every
change, because no contributor exercises that path locally and a break in it is otherwise completely
silent — which also means correcting the project's existing automation, whose pinned toolchain
currently sits below the version the project itself declares as its floor.

**Validation point**: No construction of the superseded helper remains anywhere in the repository,
and every example and test compiles and passes using the new surface under the pinned minimum
toolchain. Every example runs to completion producing its expected output, including the teardown
the repaired example's existing tests already require. An example retrieves a published value using
an address alone and asserts the value it gets back. The automated build against the pinned minimum
version exists, runs on every change, and passes.


#### - [ ] Phase 3.1: Repair the configuration-only example's teardown

**Repo:** xclconfig

One bundled example describes tearing down everything it applied, and is tested as though it does,
but never actually does it — three of its own tests fail on this today, before any of this plan's
work. The example is being rewritten for the new surface in the next phase, so it is repaired first,
on the current surface, to give that rewrite a green baseline to land on. This is a pre-existing
defect unrelated to the lookup API; it is fixed here only because the plan already has the file open
and because an example that does not run to completion cannot demonstrate anything else.

*Technical detail:* [context.md#phase-31](./context.md#phase-31-repair-the-configuration-only-examples-teardown)

**Acceptance criteria**:
- [ ] The configuration-only example tears down everything it applied and reports that it has done so.
- [ ] The example's existing tests covering teardown pass without being modified to match the code.
- [ ] The example's documented description of what it does matches what it does.

#### - [ ] Phase 3.2: Move every bundled example and test to the new surface

**Repo:** xclconfig

Every lookup in the repository's examples and tests moves to the new surface, written in the portable
form so that what a reader copies out of the repository compiles for them on the oldest supported
toolchain rather than requiring the newest. At least one example gains something the repository has
never shown: retrieving a published value using an address alone, with no knowledge of how such
values are stored. Each migrated call becomes a single expression where it was previously a
construction followed by a call.

*Technical detail:* [context.md#phase-32](./context.md#phase-32-move-every-bundled-example-and-test-to-the-new-surface)

**Acceptance criteria**:
- [ ] No example or test constructs the superseded helper, and every lookup in them is a single call expression.
- [ ] Every example and test compiles and passes under the pinned minimum toolchain.
- [ ] Every example runs to completion producing its expected output.
- [ ] At least one example retrieves a published value using an address alone and asserts the value returned.

#### - [ ] Phase 3.3: Prove the build on the oldest supported toolchain

**Repo:** xclconfig

The build against the project's declared minimum toolchain is checked automatically on every change.
Nobody compiles that path locally — the newest toolchain satisfies the declared minimum, so an
ordinary build proves nothing about it — which means a break in it is otherwise completely silent
until a consumer on an older Go reports it. Setting this up also forces correcting the project's
existing automation, whose pinned toolchain currently sits below the version the project itself
declares as its floor.

*Technical detail:* [context.md#phase-33](./context.md#phase-33-prove-the-build-on-the-oldest-supported-toolchain)

**Acceptance criteria**:
- [ ] An automated check builds the project against the pinned minimum supported version.
- [ ] That check runs on every change rather than on demand.
- [ ] The check passes, with the portable form present and the method form correctly excluded.
- [ ] The existing automation no longer pins a toolchain below the project's declared minimum.

### Milestone 4: The documentation describes the library that now exists

**What changes**: A developer reading the project's documentation is shown the surface the library
actually has. The library's own documentation replaces its description of the superseded helper with
the new surface and gains a worked example of retrieving a published value, which it has never
covered despite the library supporting the concept. Every page of the documentation site that shows
or names the superseded helper moves to the new surface, including the landing page's headline
example — the first code a visitor reads — and the feature description beside it that names the old
helper explicitly. Site examples are written in the portable form for the same reason the bundled
ones are. The project's own recorded architecture knowledge, which still presents the superseded
helper as the way consumers reach into a configuration, is brought in line so the team's reference
material does not contradict the shipped library.

**Validation point**: A case-insensitive search for the superseded helper across the library and the
documentation site returns no matches — case-insensitive because some site mentions are lowercase
prose. The library's documentation shows the new surface in place of the old section and contains a
worked example of retrieving a published value. The landing page's headline example and its
accompanying feature description both show the new surface. The knowledge entry describes the new
surface, recorded through the knowledge workflow with confirmation rather than written directly.

#### - [ ] Phase 4.1: Rewrite the library documentation for the new surface

**Repo:** xclconfig

The library's own documentation replaces its description of the superseded helper with the new
surface, and gains something it has never had: a worked example of retrieving a value a configuration
publishes. The documentation presents the method form as the destination while its runnable examples
use the portable form, and it says why, so the next contributor does not tidy away an inconsistency
that is deliberate. The note that lookups remain plain linear scans is carried forward, since nothing
here introduces an index.

*Technical detail:* [context.md#phase-41](./context.md#phase-41-rewrite-the-library-documentation-for-the-new-surface)

**Acceptance criteria**:
- [ ] The library's documentation contains no description of the superseded helper and shows the new surface in its place.
- [ ] It contains a worked example of retrieving a published value, which it previously had none of.
- [ ] It explains why documented idiom and shown examples use different spellings.
- [ ] It still records that lookups are linear scans.

#### - [ ] Phase 4.2: Move every documentation site page to the new surface

**Repo:** xcl-website

Every page on the documentation site that demonstrates or names the superseded helper moves to the
new surface, including the landing page's headline example — the first code a visitor reads — and the
feature description beside it, which names the old helper explicitly. Site examples are written in
the portable form, for the same reason the bundled ones are. Note that some mentions are lowercase
prose rather than the helper's exact name, so the check that no mention survives has to ignore case.
Samples on the site are not compiled as part of this work, by decision taken during planning.

*Technical detail:* [context.md#phase-42](./context.md#phase-42-move-every-documentation-site-page-to-the-new-surface)

**Acceptance criteria**:
- [ ] A case-insensitive search of the documentation site's sources for the superseded helper returns no matches, in prose and in code alike.
- [ ] The landing page's headline example shows the new surface.
- [ ] The feature description accompanying it describes the new surface rather than naming the superseded helper.
- [ ] Every migrated code sample is written in the portable form.
- [ ] The site builds and every affected page renders.

#### - [ ] Phase 4.3: Bring the recorded architecture knowledge in line

**Repo:** xclconfig

The project's recorded knowledge about how consumers reach into a parsed configuration still presents
the superseded helper as the way to do it, including in its diagram of the public surface, and still
carries open questions about that helper's shape which this work settles. It is revised so the team's
own reference material does not contradict the library that now exists. The revision goes through the
knowledge workflow's propose-then-confirm rather than being written directly.

*Technical detail:* [context.md#phase-43](./context.md#phase-43-bring-the-recorded-architecture-knowledge-in-line)

**Acceptance criteria**:
- [ ] The recorded architecture knowledge describes the new surface and no longer presents the superseded helper.
- [ ] Its diagram of the public surface names what the library now exposes.
- [ ] Questions it raised about the superseded helper's shape are resolved or removed, since the work has settled them.
- [ ] The revision was proposed and confirmed before being written, not written directly.

## Open Questions

Most of what looked uncertain during discovery was resolved during planning rather than parked here:
the not-found sentinel's import direction (an import cycle, resolved by following the re-export
precedent already in the codebase), whether enumeration genuinely covers every kind of declaration
(verified — nothing filters by kind on the way into state), and whether the floor toolchain is
reachable for the new build job (verified locally, it resolves from cache). Three uncertainties
remain that genuinely cannot be settled until the code is being changed.

**1. Whether any reader of the conflated metadata field has been left on the wrong axis.**

*Depends on*: exercising a real configuration end to end, because the compiler cannot detect this —
both axes are strings, so every one of the roughly thirty read sites keeps building whether or not it
was changed correctly. The inventory gathered during planning is believed complete, but "believed
complete" is exactly the claim that a silent failure defeats.

*What the implementer should do*: treat a reference that stops resolving, a resource that stops
finding its provider, or an item that stops being found in saved state as a symptom of this rather
than of the new lookup surface — the axis split is the more likely cause, and Phases 1.1 and 1.2 are
where to look. If a reader is found that the plan does not list, add it and carry on; the plan's
inventory being incomplete is expected to be a small correction, not a design problem. **STOP and ask
only if** a reader turns out to need *both* axes in a way that cannot be expressed by the two fields
— that would mean the two-axis model itself is insufficient, which is a design question, not an
implementation one.

**2. Whether accepting a declaration led by its variety requires any change inside the vendored HCL
fork.**

*Depends on*: attempting it. Block-header parsing is generic — the fixed keyword list lives in this
project's own parser, not in the fork — so the expectation is firmly that no fork change is needed.
That expectation has not been proven by execution.

*What the implementer should do*: if the fork does need changing, the licence rules bind and are not
optional: keep the upstream MPL header, add the modification notice, and record the change in the
fork's upstream notes. Do not edit any dependency outside the fork under any circumstances. **STOP and
ask if** the change required would be substantial rather than incidental, since a significant
divergence from upstream is a cost the user should weigh rather than absorb inside a phase.

**3. Whether the registry's builtin-detection probe still behaves once the kind field is uniform.**

*Depends on*: running it. One provider-lookup path identifies builtins by attempting to create a
resource under a placeholder name and inferring from the outcome. That inference was written when the
kind field held the variety for resource stanzas and the keyword for everything else; making the
field uniform changes what it sees.

*What the implementer should do*: this surfaces in Phase 1.1 as work being routed to a provider for
something that should not have one, or the reverse. Replacing the probe with the registry's own
type-source knowledge is a reasonable local fix and needs no approval. **STOP and ask if** fixing it
requires changing the provider-resolution contract itself, rather than just how the question is
asked — that reaches plugin behaviour, which this spec does not otherwise touch.

## Out of Scope

**Excluded by decision taken during planning**

- **Proving that documentation-site code samples compile.** Dropped by the user during discovery. Site pages still move to the new surface and are still written in the portable form, but nothing verifies that they build. This descopes one clause of the spec's site acceptance criterion — the clause requiring every sample to compile under the pinned minimum version; the clauses requiring no superseded mentions and requiring the landing page to show the new surface remain in scope and are verified. The site keeps its existing manual policy, recorded in its own README: the example pages quote the library's example code, and are updated by hand when it changes. No Go module, Go toolchain or snippet tooling is added to the site repo. **Not currently tracked elsewhere** — if it is wanted later it needs its own spec, and the three mechanisms costed during planning are written up in this plan's research document.

- **Fixing the plugin example's failing end-to-end test.** Left alone by the user's decision. That test requires a binary at a path a fresh checkout does not build, with no build step in the test and no skip, so the full suite fails on a clean clone today, independently of this work. The consequence carried into this plan is that the new minimum-version job builds rather than runs the suite, and no phase may claim the suite is green without naming this. **Tracked separately** — the sibling plugin example already solves the same problem by building its binary from within the test, which is the obvious model whenever it is picked up.

**Excluded by the spec's non-goals**

- **Exposing the dependency graph, or a public dependency-ordered walk over a configuration.** Not part of this surface.
- **Indexing or lookup-performance work.** Every lookup remains a linear scan, and the new surface adds a second scan over a materialised list on top of the existing one. This is adequate at one configuration's scale, is stated in the documentation so the next reader does not assume otherwise, and is deliberately not optimised.
- **Populating the per-item properties map that is allocated but never written.** Untouched.
- **Migrating configuration state written by the previous version.** Regenerating it is acceptable. What this plan *does* add is that such a file now fails loudly instead of loading a silently smaller configuration — that is honesty about the absence of a migration, not a migration.
- **A migration guide, deprecation window or compatibility shim for consumers outside this repository.** The version carrying these changes is unreleased and has none. Within the repository, the two renamed enumeration and count operations do keep their previous names as deprecated aliases, because that costs nothing and keeps internal callers working.
- **New documentation-site pages covering published values or error semantics.** Existing pages move to the new surface; broadening the site's coverage is separate work.
- **Correcting the library documentation's stale sections.** Roughly two hundred lines of the README document methods that no longer exist anywhere in the code. This is unrelated to the query surface and is tracked separately. It is called out explicitly in the phase that rewrites the neighbouring section, because anyone working there will be looking straight at it.
- **Any change to how a configuration is authored, beyond accepting the additional stanza form.** The expression language, the existing stanza keywords and their meanings are unchanged. This work is otherwise about how a parsed configuration is consumed.

**Excluded by the chosen design, and worth naming so the omissions are visible**

- **Unifying the two near-duplicate provider-lookup paths.** Both read the field this plan splits and both are corrected, but they are left as two. Merging them would widen the diff on the single riskiest change in the plan for no benefit this spec asks for.
- **Normalising error receivers across the codebase.** Receivers are inconsistent today — some value, some pointer. New errors are written consistently and the existing not-found error keeps its value receiver so its seven construction sites and existing callers are untouched. A repo-wide normalisation is not attempted.
- **Repairing the pre-existing oddity in how bare `local` addresses round-trip.** A parsed `local` address currently formats back out in a different shape, and there is no Go type or constant behind that keyword at all. It predates this work and widening the addressing change to cover it invites regressions in the one place this plan most needs to be careful.
- **Raising the project's declared minimum Go version.** Forbidden by a spec constraint, and the reason the surface ships in two spellings at all. The existing automation's toolchain pin *is* corrected, because it currently sits below the declared minimum, but the declared minimum itself does not move.

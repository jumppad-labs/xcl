# State & Persistence

## What the state layer is, and is not

The `state` package **stores and retrieves entities**. That is all it does.

It does not search a configuration, and it does not parse addresses. Asking a
configuration what it declares is the job of `Config` — see
[the querying section of the README](../README.md#querying-a-configuration) —
and resolving an address is done there, against the types the `Config`'s catalog knows.
The `state` package imports nothing address-related, which a test in
`state/dependencies_test.go` guards.

There is no public container type. A `StateStore` exchanges plain entities, so
an implementation needs no type from this library in its signatures and has
nothing to construct. The parser keeps its own working container internally
(`internal/parser/entities.go`), which is a mutex-protected flat list with no
index or status table — a resource's operational status lives on its own
`types.Meta.Status` field (see [Resource statuses](#resource-statuses)).

Each saved resource keeps its `meta.links` and `meta.module`
([`types/resource.go`](../types/resource.go#L47)): every dependency it has,
written in `depends_on` or worked out from a reference, as an address relative
to the module it sits in. `Destroy` resolves them against the saved state with
the same builder that orders creation, and walks that graph backwards, so it
needs no configuration. Saved state no longer carries `meta.parents`; state
written by earlier versions that still holds it loads, the key ignored, and is
ordered from its links.

Every lookup is a **linear scan** comparing `types.GetMeta(r)` fields against a
parsed FQRN (fully-qualified resource name, `internal/resources/fqrn.go`) —
there is no index, which is fine at the scale this is used (one config's worth
of resources) but worth knowing if you're tempted to call these in a hot loop.

That applies to the public lookup surface too. `xcl.Find`, `xcl.FindByType`,
`xcl.FindOne` and `xcl.All` scan `Config.Entities()`, so an address lookup is
O(n) over the entities the configuration holds. `xcl.All[T]` additionally
resolves `T` to its address segments through the `Config`'s catalog before it
scans. None of this is indexed or cached, deliberately.

Key operations:

- **`AppendResource(r)`** ([`state.go:35`](../state/state.go#L35)) — computes
  the resource's FQRN from `Module`/`Name`/`Type` in its `Meta`, sets
  `Meta.ID` to that FQRN string, and errors with `ResourceExistsError` if a
  resource with the same FQRN is already present.
- **`FindResource(path)`** ([`state.go:81`](../state/state.go#L81)) — parses
  `path` as an FQRN and scans for a match.
- **`FindRelativeResource(path, parentModule)`** ([`state.go:89`](../state/state.go#L89)) —
  same, but prefixes `path`'s module with `parentModule` first; used when
  resolving a reference from inside a module to something in its own scope.
- **`FindModuleResources(module, includeSubModules)`** ([`state.go:132`](../state/state.go#L132)) —
  used by the parser to cascade `disabled` status onto everything inside a
  disabled module (`includeSubModules=true` matches by module-path prefix).
- **`Bytes()`** ([`state.go:178`](../state/state.go#L178)) — the
  serialization boundary: `json.MarshalIndent(s.resources, "", "  ")`. This
  is what any `StateStore.Save` implementation is expected to persist.

## Resource statuses

xcl records what happened to each resource in `Meta.Status`
([`types/status.go`](../types/status.go)). These are the only values it
sets:

| Status | Meaning | Next apply |
|---|---|---|
| `created` | the provider created the resource | read, then updated if changed |
| `updated` | the provider updated the resource | read, then updated if changed |
| `failed` | a provider call for the resource failed | rebuilt: destroyed, then created |
| `destroyed` | the provider destroyed the resource | — (never saved) |
| `destroy_failed` | destroying the resource failed | removed again if its block is gone, otherwise rebuilt: the destroy is tried again, then created |

`destroyed` is never saved: a destroyed resource is removed from the state
instead. A `destroy_failed` resource is retried by the next `Destroy`, or by
the next `Apply` as above.

## State saved after a failed apply

A failed apply still produces a state to save. `Parser.Apply` returns it
together with the error, and `Config.Apply` saves it before returning the
error, so the next apply picks up where this one stopped:

- resources the walk reached are saved with their new values and status;
- the failing resource is saved as `failed`, or `destroy_failed` if a
  rebuild's destroy failed;
- resources that existed before but were not reached keep their previous
  entry;
- new resources that were not reached are left out.

When a removed resource fails to be destroyed, the apply stops before
anything is created or changed, and the state saved is the previous state
minus what was destroyed, with the failures kept as `destroy_failed` (see
[State during a destroy](#state-during-a-destroy)). The next apply retries
the removal first.

Nothing is saved when the configuration doesn't parse or validate, declares
no blocks (`xcl.ErrEmptyConfiguration`), or the dependency graph can't be
built: no provider was called, so the previous state still stands. See
[Parser & Resource Lifecycle](parser-lifecycle.md#state-saved-after-a-failed-apply)
for how the state is built.

## State during a destroy

`Config.Destroy` and the removal phase of `Config.Apply` save the state
through the `StateStore` after every resource they destroy, not once at the
end ([`internal/parser/destroy.go`](../internal/parser/destroy.go#L22)):

- a destroyed resource is removed from the state;
- a resource whose destroy failed is kept, as the saved copy, with status
  `destroy_failed`;
- the resources it depends on are never reached, so they stay as they were.
  Unrelated resources are still destroyed.

The saved state is therefore correct at every step. If a destroy is
interrupted, or returns an error, running `Destroy` again picks up with what
is left. `Destroy` with no saved state, or an empty one, returns nil and
writes nothing.

## Reading a saved record back as configuration

A saved record can be turned back into configuration text with
`c.EncodeSavedEntity(data)`, a method on the `Config`. The `Config`'s catalog,
its declared types and its registries' plugins, is what types the record,
including the types a plugin provides, so the plugins are loaded if they have
not been loaded already.

```go
text, err := c.EncodeSavedEntity(record)
```

An event handler can also read the record an event carries as its entity with
`event.Entity()`, which returns it as the registered Go type with every
sensitive value masked, or `nil, nil` when the event carries no data.

The same record reaches an event receiver when a configuration asks for
`xcl.EventDataProcessed`, so the same call works on either. The one difference
is sensitive values, which state and events mask separately: state holds them
plainly or encrypted by the state masker, while event data writes them through
the event masker, by default as `{"xcl_masked":"redact","value":"(sensitive)"}`.
Processed event data and the state record are therefore the same only where
both mask alike. `EncodeSavedEntity` shows every masked value as
`(sensitive)`, from state or from events, never as ciphertext or a hash, and
even with `xcl.RevealSensitive()`, since it does not open masked values.

Each saved record also carries `meta.references`, the text the user wrote for
each field that referred to another entity, so
`c.EncodeSavedEntity(record, xcl.ShowReferences())` shows those
references exactly as `EncodeEntity` does for the live entity. A record saved
by an earlier version has none, and shows resolved values.

The stored format has one reader. Both the file state store's `Load` and
`EncodeSavedEntity` go through it, so there is a single place that knows how a
record names its type. A record naming a type the `Config` does not know fails
with `xcl.ErrUnregisteredType`, and one that cannot be read at all with
`xcl.ErrInvalidSavedData`.

The text is for reading rather than for feeding back to xcl; see
[Converting to configuration text](../README.md#converting-to-configuration-text).

## Sensitive values in state

State holds the real value of every `types.Sensitive` field, so a later run
can read it back. With no state masker it is held in plain text, and every
`Apply` or `Destroy` that writes one emits a single warn-level log event:
`sensitive values are stored unencrypted in state; use xcl.WithStateMask to
encrypt them`.

`xcl.WithStateMask(m)` masks each sensitive value as it is saved, with a
masker that must implement `mask.Reversible`, usually
`mask.EncryptAES256GCM(key)`. A one-way masker fails `NewConfig` with
`xcl.ErrMaskNotReversible`. Only the sensitive values are masked, each written
in place of its value as an envelope naming the masker:

```json
"password": {"xcl_masked": "aes-256-gcm", "value": "o8Rk1x...base64..."}
```

Every load, at the start of an apply and of a destroy, opens each envelope
with the configured masker before the record is typed. An envelope that does
not open fails the load with `xcl.ErrUnrecoverable`, naming the entity and the
masker: state encrypted under another key, state masked by another masker, or
masked state loaded with no masker configured. A masked value is never read
back as the marker, because the next save would write that over the real
value. Plain values load with or without a masker, so adding one to existing
state works, and the next save encrypts it.

A custom `StateStore` needs no change: with a state masker configured, the raw
records it receives already hold envelopes in place of sensitive values.

## `StateStore` — the persistence contract

```go
// state/state_store.go
type StateStore interface {
    Load() ([]any, error)      // entities or raw records; nil if nothing was saved
    Save(entities []any) error
    Exists() bool
    Clear() error
}
```

The contract exchanges plain values deliberately. Storing them is all it
does: how they are typed and searched is the configuration's concern, not a
store's. `Load` may hand back the entities it was given, or the raw records it
saved them as; the parser types raw records with the `Config`'s catalog, so a store
never needs one. Writing one needs no library type — `state/custom_store_test.go` has a
twenty-line in-memory implementation that round-trips a real apply, written
against the public API alone.

**State keeps real sensitive values.** `Save` receives each entity already
encoded, as a `json.RawMessage` holding its record with every
`types.Sensitive` field written as its real value. A sensitive value marshals
to `(sensitive)` through `encoding/json`, so encoding the entities first is
what lets a store that simply marshals what it is given keep the real values,
and reload them unchanged. A store that inspected the typed entities must
decode the raw records instead. Without a state masker the state file
therefore holds secrets in plain text; configure one with `xcl.WithStateMask`
(see [Sensitive values in state](#sensitive-values-in-state)), or protect the
file accordingly.

`Parser.Apply` (and `Parser.Validate`) call `Exists()`/`Load()` at the
start of every run to get the "previous state", the state saved by the last
apply. `Config.Destroy` calls them too, to get the state to destroy.
`Parser.Apply` uses each resource's entry in it to decide between
create, read-then-update and rebuild (see
[Parser & Resource Lifecycle](parser-lifecycle.md)). `Config.Apply` calls
`Save()` after adopting the entities the parse produced, including after a failed apply
(see [State saved after a failed apply](#state-saved-after-a-failed-apply)).
Destroying, in `Config.Destroy` or an apply's removal phase, calls `Save()`
after every resource (see [State during a destroy](#state-during-a-destroy)).

`state/mocks/mock_state_store.go` is a generated mock of this interface
(same mockery setup as the plugin mocks — see [Plugin
Architecture](plugins.md)). Tests must stub `Exists()` even when it's
expected to return `false` — `Parser.Apply` and `Parser.Validate` call it
unconditionally (`parseAndValidate`,
[`internal/parser/parser.go:375`](../internal/parser/parser.go#L375)), so a
bare mock with no expectation set panics on the first call.

## `FileStateStore` — the on-disk implementation

[`state/file_state_store.go`](../state/file_state_store.go) is the only
`StateStore` implementation in this repo. Four things worth knowing:

**The store only reads and writes records.** It knows nothing about types and
needs no catalog. `Load()` ([`file_state_store.go`](../state/file_state_store.go))
unmarshals the top-level array and returns each record as the
`json.RawMessage` it was saved as. A record whose type nobody registered
loads like any other; a file that is not a JSON array fails the load.

**Typing happens where state is consumed.** The parser, when it reads the
previous state for `Apply`/`Validate` and at the start of `Destroy`, passes
what the store loaded through
[`savedentity.DecodeAll`](../internal/savedentity/savedentity.go) with the
catalog ([`internal/catalog/catalog.go`](../internal/catalog/catalog.go)). A `json.RawMessage`, `[]byte` or `map[string]any` is a saved
record and is decoded: read `meta.type`, `meta.subtype` (empty for an entity
without one) and `meta.name`, call `catalog.CreateEntity(type, subtype,
name)` to get a correctly-typed *empty* instance, then unmarshal the record
into it. Anything else is taken to be an
entity already and passes through unchanged, which is why a store that keeps
entities in memory needs no decoding.

**A type that is not registered fails the load.** When a saved record's type
can't be created by the catalog (e.g. a plugin that's no longer loaded, or
a type no registry given to the `Config` declares with `RegisterType`), decoding returns
[`state.UnknownTypesError`](../state/errors.go) naming every such type,
sorted and unique, instead of dropping the records — a state returned
without them would be saved without them, erasing resources that still
exist. Records that are malformed, or missing `meta`, `meta.type` or
`meta.name`, are reported in the same error by their id, or by their
position as `entry N`. This affects `Apply` and `Destroy` alike, so register
every type and plugin before loading state.

**`Save` is not atomic.** ([`file_state_store.go`](../state/file_state_store.go))
It removes the existing file, then writes the new one — not a
write-to-temp-then-rename. A crash between the remove and the write would
lose the state file. Worth keeping in mind if this is ever hardened for
production use.

`NewFileStateStore(dir)` takes a directory, not a file. It keeps state in
`state.StateFileName` (`state.json`) inside it, creating the directory and an
empty state file automatically if they don't exist yet (`createStateAtPath`,
[`file_state_store.go`](../state/file_state_store.go)) — callers don't need to
special-case "first run." An existing state file is kept as it is.
`store.Path()` returns the file's full path.

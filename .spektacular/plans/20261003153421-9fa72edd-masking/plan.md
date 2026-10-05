---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Plan: 20261003153421-9fa72edd-masking

<!-- Metadata -->
<!-- Created: 2026-10-05T10:30:08Z -->
<!-- Commit: d554c1d -->
<!-- Branch: main -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Sensitive values are already protected wherever xcl displays them, but state stores them in plain text, and events can only ever show the fixed marker. This plan adds pluggable masking through a new `mask` package. It provides a `Masker` interface developers can implement, four built-ins (AES-256-GCM encryption, keyed HMAC-SHA256 hashing, omit and redact), and a self-describing envelope that names the masker behind every masked value. `xcl.WithStateMask` encrypts sensitive values in state with a reversible masker and reads them back. A one-way masker is rejected, and a warning is emitted when state holds secrets in plain text. Event data is redacted by default, can use any masker through `xcl.WithEventMask`, and carries real values only after `xcl.WithNoEventMask()`, while errors and log details always show the marker. Application developers can keep secrets out of state files and event pipelines without giving up state, and the examples, library docs and documentation site show how.

## Conventions

- **Testing & Mocking: testify `require`, no table-driven tests, never mix positive and negative cases, favour verbosity** — each masker property, option, load outcome and leak check gets its own named accepted test and its own rejected test.
- **Generate test state with a real apply, not a hand-written state file** — encrypted-state, reload, wrong-key and plaintext-warning tests run a real `Apply` and load what it wrote. The one exception is a test about the envelope format itself.
- **Shared errors live in the `errors` package; sentinel-and-detail convention** — `ErrUnrecoverable` with `UnrecoverableError` and `ErrMaskNotReversible` with `MaskNotReversibleError` go there with pointer receivers, re-exported from `xcl`.
- **Dependencies: prefer the standard library** — the maskers use `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`, `crypto/rand` and `encoding/base64`; no new module.
- **Code style: standard Go conventions, `any` over `interface{}`, small focused interfaces** — `Masker` is two methods, and reversibility is a separate `Reversible` interface rather than a flag.
- **Project structure: `/internal` is private** — `internal/wire` stays internal; the public masking surface is the `mask` package and the three options.
- **Development standards: structured logs** — the plaintext-state warning is a structured warn-level log event through the existing event logger.

## Architecture & Design Decisions

All Go work lands in the `xclconfig` repo. The documentation site's events guide and its new state masking page land in `xcl-website`.

**A masker is a small public interface, and every masked value names the masker that produced it.** A new public package, `mask`, holds the `Masker` interface, the `Reversible` interface for maskers that can open what they produced, and the four built-ins the spec names: `mask.EncryptAES256GCM`, `mask.HashHMACSHA256`, `mask.Omit` and `mask.Redact`. A masker sees the JSON encoding of a sensitive value's real value and returns what to write in its place. xcl then writes the result in a fixed envelope, `{"xcl_masked":"<masker name>","value":<output>}`, in exactly the position the real value would occupy. `Omit` writes the envelope with no `value`, and `Redact` writes the marker `(sensitive)` as its value. Because every masked value is self-describing, a reader can tell which masker produced it, and so whether it can be recovered, without knowing the Go type it belongs to. `mask.Unmask` opens one envelope with a reversible masker of the same name. Given a one-way masker, a different masker, or the wrong key, it fails with `ErrUnrecoverable` rather than returning a wrong value. The interface lives in `mask`, not the root package, so the parser can use it without an import cycle (`architecture/shared-public-types-live-in-types.md`). Its errors live in the `errors` package and are re-exported from `xcl` (`conventions/shared-errors-package.md`). The built-ins use only the standard library's `crypto` packages (`conventions/dependencies.md`).

**The masker is chosen per serialisation call, at the call sites the sensitive-values plan established.** That plan routes every internal hop through `internal/wire`, which writes each `types.SensitiveValue` revealed, and it chooses the encoder at each call site, never through global state. This plan gives `wire` one optional argument: a masker that, when present, replaces each sensitive value with its envelope. It also reports whether any sensitive value was written. Provider calls, change detection and query conversion pass no masker, so they keep real values. The shared state-encoding helper used by both save sites (`config.go:315`, `internal/parser/destroy.go:130`) passes the configured state masker. Event data (`internal/parser/events.go:37-83`) passes the configured event masker, which is `mask.Redact()` unless the developer chooses another or turns masking off with `WithNoEventMask()`. So one entity can be encrypted in state, redacted in its events and real at its provider in the same apply, as the spec's Technical Approach expects. Processed event data and state therefore stop being byte-identical whenever masking differs.

**State is opened where it is read, and anything that cannot be opened fails the load.** Every state load goes through `savedentity.DecodeAll` (`internal/parser/parser.go:360,451`), and every saved-data read through `savedentity.Decode`, which already holds each record as a generic JSON tree before typing it (`learnings/state-save-and-load-points.md`). Before typing, a new pass replaces each envelope in that tree. When loading state, an envelope from the configured state masker is opened to the real value, which the sensitive type reads back as it already does. An envelope from any other masker, or one that does not open, fails the load with `ErrUnrecoverable` naming the entity and the masker. A redacted value read back would otherwise be saved over the real one. When reading for display, as `EncodeSavedEntity` does with state or event data, every envelope becomes the marker, so a hash or ciphertext is never presented as a value. `WithStateMask` refuses a masker that is not `Reversible` when `NewConfig` runs. Plain values in state still load, so adding a state masker to an existing project works: the next save encrypts. With no state masker, the save helper's "a sensitive value was written" report makes Apply and Destroy emit one warn-level log event per operation saying sensitive values are stored unencrypted. A configuration with no sensitive values emits none.

**Errors and log details are outside event masking.** Errors are built with `fmt`, and log details hold the self-protecting `Sensitive[T]` value, so both keep showing the marker whatever the event masker. Turning event masking off changes only `Event.Data`. The examples that hold secrets encrypt their state from a key in the environment, and the library docs, the site's events guide and a new state masking page describe all of this.

Rejected directions: a masker chosen inside `MarshalJSON` through global or goroutine state, masking the encoded bytes after the fact, a type-driven custom decoder for loading, a bare marker for redact, dropping the key for omit, and silently redacting unopenable state. The evidence for each is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Masking package (new, public `mask`)**: owns the `Masker` and `Reversible` interfaces, the envelope that masked data is written in, the four built-in maskers, and `Unmask`, which opens one envelope or reports it unrecoverable. It knows nothing about entities, state or events. The wire encoder, saved-entity reading and the configuration options all depend on it, and application code implements its interface to supply a custom masker.
- **Masking errors (changed, `errors`)**: gains `ErrUnrecoverable`, for a masked value that cannot be opened, and `ErrMaskNotReversible`, for a one-way masker given for state, each with a detail type. The root package re-exports both.
- **Revealing wire encoder (changed, `internal/wire`)**: still the one place that serialises an entity with its sensitive values handled deliberately. It gains an optional masker. Without one it writes real values, as before. With one it writes each sensitive value as the masker's envelope, at any depth, including inside `any`, maps and slices. It also reports whether it wrote any sensitive value at all. Its callers decide which masker, if any, to pass.
- **State encoding helper (changed, parser)**: the helper both save sites use passes the configured state masker to the wire encoder. It returns whether any sensitive value went to state unmasked, so the caller can warn.
- **Saved-entity reader (changed, `internal/savedentity`)**: before typing a record, it replaces each envelope in the record. There are two modes. Loading state opens envelopes with the state masker and fails with `ErrUnrecoverable` for anything it cannot open. Reading for display turns every envelope into the marker. Plain values pass through unchanged in both modes.
- **Parser options and load paths (changed, parser)**: the parser's options carry the state masker and the event masker. The apply and destroy load paths read state in the strict mode with the state masker. Destroy's per-resource save and Apply's final save use the encoding helper with it.
- **Event data (changed, parser events)**: event data at both levels is encoded through the wire encoder with the configured event masker. That is the redact masker by default, or none when masking is turned off. Log events and errors are untouched.
- **Configuration options (changed, root package)**: `WithStateMask` sets the state masker and rejects a masker that is not reversible. `WithEventMask` sets the event masker. `WithNoEventMask` turns event masking off. `Config` hands the maskers to the parser. Apply and Destroy emit the plaintext-state warning once per operation when the save reported unmasked sensitive values.
- **Configuration text from saved data (changed, root `encode`)**: `EncodeSavedEntity` reads in display mode, so masked state or event data turns into configuration text showing the marker.
- **Bundled examples (changed)**: the application-config and plugin examples, which hold passwords, encrypt their state with a key taken from the environment. Without a key they run with the plaintext warning showing. Their tests assert that no secret appears in state or in event data.
- **Library documentation (changed)**: the README's sensitive-values section, the state guide, the changelog and the README/CHANGELOG content tests.
- **Documentation site (changed, `xcl-website`)**: the events guide gains a section on masking event data, and a new state masking guide page is linked from the navigation.

## Data Structures & Interfaces

**`mask.Masker` (new, public interface).** It is what a developer implements to supply custom masking. `Name` identifies the masker and is written into everything it masks. `Mask` receives the JSON encoding of a sensitive value's real value and returns the JSON to write as the envelope's `value`. A nil result writes no value.

```go
package mask

type Masker interface {
    Name() string
    Mask(value json.RawMessage) (json.RawMessage, error)
}
```

**`mask.Reversible` (new, public interface).** A masker that can open what it produced. Only a `Reversible` masker is accepted for state. `Unmask` returns the original JSON, or an error when the masked value does not open, for example under a different key.

```go
type Reversible interface {
    Masker
    Unmask(masked json.RawMessage) (json.RawMessage, error)
}
```

**`mask.Masked` (new, public type): the envelope.** Every masked value in state or event data is written in this shape, in place of the real value. The key `xcl_masked` is reserved for it.

```go
type Masked struct {
    By    string          `json:"xcl_masked"`      // the producing masker's Name
    Value json.RawMessage `json:"value,omitempty"` // absent for Omit
}
```

**Built-in maskers (new, public constructors).**

```go
func EncryptAES256GCM(key []byte) (Masker, error) // name "aes-256-gcm"; Reversible; 32-byte key; value is base64(nonce‖ciphertext)
func HashHMACSHA256(key []byte) (Masker, error)   // name "hmac-sha256"; one-way; non-empty key; value is hex HMAC
func Omit() Masker                                // name "omit"; one-way; no value
func Redact() Masker                              // name "redact"; one-way; value "(sensitive)"
```

**`mask.Unmask` (new, public function).** It opens one envelope with a masker. It fails with `ErrUnrecoverable` when the masker is not `Reversible`, when the envelope names a different masker, or when the masker cannot open the value. It never returns a wrong value. `mask.IsMasked` reports whether some JSON is an envelope.

```go
func Unmask(data json.RawMessage, m Masker) (json.RawMessage, error)
func IsMasked(data json.RawMessage) (Masked, bool)
```

**Errors (new, `errors`, re-exported from `xcl`).**

```go
var ErrUnrecoverable = errors.New(...)     // a masked value cannot be recovered
type UnrecoverableError struct {
    ID       string // the entity, where known
    MaskedBy string // the masker named in the envelope
    Reason   string // one-way masker, different masker, or failed to open
    Err      error
}

var ErrMaskNotReversible = errors.New(...) // the state masker must be reversible
type MaskNotReversibleError struct{ Masker string }
```

Both detail types use pointer receivers and wrap their sentinel. The message for `MaskNotReversibleError` reads `the state masker must be reversible: "<name>" cannot recover the values it masks`.

**Configuration options (new, public).**

```go
func WithStateMask(m mask.Masker) ConfigOption // nil or not Reversible fails NewConfig
func WithEventMask(m mask.Masker) ConfigOption // nil fails NewConfig
func WithNoEventMask() ConfigOption            // events carry real sensitive values
```

The default event masker is `mask.Redact()`. Where `WithEventMask` and `WithNoEventMask` are both given, the last one wins, as with `WithStatePath` and `WithStateStore`.

**`internal/wire` (changed, internal).** It gains an options form. Without a masker its output is unchanged.

```go
type Options struct {
    Mask mask.Masker // nil writes real values
}

type Result struct {
    Data      []byte
    Sensitive bool // at least one sensitive value was written
}

func Encode(v any, options Options) (Result, error)
```

`Marshal` and `MarshalIndent` keep their signatures and behave as `Encode` with no masker.

**`internal/savedentity` (changed, internal).** `Decode` and `DecodeAll` take a `ReadOptions` saying how to treat envelopes.

```go
type ReadOptions struct {
    Mask       mask.Masker // the state masker; nil when none is configured
    ForDisplay bool        // every envelope becomes the marker instead of being opened
}
```

**`ParserOptions` (changed, internal).** It gains `StateMask mask.Masker` and `EventMask mask.Masker`, where a nil `EventMask` means real values. `Config` always fills `EventMask`, with `mask.Redact()` unless masking was turned off.

**Stored and event record format (changed).** A sensitive value is written in one of three ways:
- plain, as before, in state without a state masker, and in event data with masking turned off;
- as an envelope everywhere else.

Plain values and envelopes can both appear in state that is being migrated to a masker. Both load.

**Plaintext-state warning event (new).** It is a log event with `Source` `core`, the operation's `Operation` (`apply` or `destroy`) and `Phase` `log`. Its `Meta` holds level `warn` and the message `sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them`. It is emitted at most once per operation, and only when a state store is configured and a sensitive value was written unmasked.

## Implementation Detail

**New pattern: a masker chosen per call, alongside the existing encoder choice.** The sensitive-values plan made every serialisation call choose between the revealing wire encoder and plain `encoding/json`. This plan adds one more argument to that same choice: which masker, if any, the wire encoder applies. A reader of the code will see three kinds of call:
- calls that keep the truth pass no masker: provider calls, change detection and conversions;
- the state save passes the state masker;
- event data passes the event masker.

No global or per-goroutine state is involved, so concurrent provider calls and event emission cannot interfere, as the spec's Technical Approach requires. The wire encoder's package comment gains a line saying who passes which masker, and the existing static test on its importers is unchanged.

**New pattern: a self-describing envelope, opened in the record tree.** Masked data is never interpreted by Go type. It is recognised by shape wherever it sits in a saved record, and opened or replaced before the record is typed. That keeps all knowledge of masked data in two places: the masking package, which defines and opens the envelope, and the saved-entity reader, which walks a record. The sensitive type's own JSON reading is unchanged, because it only ever sees a real value or the marker. The reader's two modes, strict for state and display for configuration text, are an explicit option rather than inferred from context.

**New public package with a small surface.** `mask` is a leaf package: it depends only on the standard library, the `types` marker and the `errors` package. Built-ins are constructed through functions, so their key handling stays private. Reversibility is expressed by a second interface, which is how the state option tells a reversible masker from a one-way one at setup without a flag a custom masker could misreport.

**Existing patterns followed.**
- The options follow the shape of the existing ones. `WithStateMask` sits beside `WithStateStore` and `WithStatePath` and, like `WithStatePath`, can fail `NewConfig`. `WithEventMask` and `WithNoEventMask` sit beside `WithEventData`, and the last one given wins.
- The plaintext warning is a log event emitted through the existing event logger, as the configured-value warning already is.
- The new errors follow the sentinel-and-detail convention and are re-exported from the root package.
- Saved data stays readable by the one reader, `savedentity`, which already holds "never drop a record silently" as a rule. It now also never redacts state silently.

**Code-shape changes.**
- The state-encoding helper gains a masker argument and a "wrote unmasked sensitive values" result.
- Event data encoding moves from plain `encoding/json` to the wire encoder with the event masker, so the default event output is produced by `mask.Redact()` rather than by the sensitive type's own `MarshalJSON`.
- `Config` gains two masker fields and threads them through `ParserOptions` exactly as it threads `EventData` today.

**Public surface UX.** A developer who does nothing gets redacted events and a one-time warning per apply when secrets go to state in plain text. To encrypt state they add `xcl.WithStateMask(m)`, where `m` comes from `mask.EncryptAES256GCM(key)`, and keep the key wherever they keep secrets. Passing a hash, omit or redact masker there fails `NewConfig` with a message saying the state masker must be reversible. To change what events show, they pass `xcl.WithEventMask(...)` with any masker, for example a keyed hash so values can be correlated without being revealed. `xcl.WithNoEventMask()` sends real values to a trusted receiver. A custom masker is a type with `Name` and `Mask`, plus `Unmask` if it should be usable for state. A receiver that reads event data can open a reversible envelope with `mask.Unmask`, and gets `xcl.ErrUnrecoverable` for anything else.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references.
- **Upstream plan `20261003134528-327e0657-references-and-secrets` (must land first).** It provides `types.Sensitive[T]`, `types.SensitiveValue` with `RevealAny`, `types.SensitiveMarker` and `types.IsRedacted`. It also provides `internal/wire` with its importer static test, the shared state-encoding helper used by both save sites, `StateStore.Save` receiving raw JSON, and event data always re-encoded from the typed entity. This plan changes the wire encoder, the encoding helper and event data encoding. It also replaces that plan's fixed event redaction with the configured event masker.
- **Upstream plan `20261003153421-6ec0eab3-module-boundary-and-output-entities` (must land first, through the plan above).** `types.Output.Value` may hold sensitive leaves inside `any`, which the wire encoder must mask and the saved-entity reader must open.
- **Upstream plans `20261003153421-bf87d907-references-as-written` and `20261003153421-c283547c-user-depends-on` (ordered before this one).** They add nothing secret to saved records. They share the README, the CHANGELOG, `readme_test.go` and the site, so this plan's entries go on top of theirs.
- **`mask` package (new, public).** It depends only on the standard library, `types` (for the marker) and `errors`.
- **`errors` package (changed).** It gains `ErrUnrecoverable`, `UnrecoverableError`, `ErrMaskNotReversible` and `MaskNotReversibleError`. Its thin import list is kept.
- **`internal/wire` (changed).** It gains the masker option and the sensitive-written report. Existing calls are unchanged.
- **`internal/savedentity` (changed).** It gains the envelope pass and its two reading modes.
- **`internal/parser` (changed).** It gains the masker options, the load paths in strict mode, the masked save helper and event data through the wire encoder.
- **Root package `xcl` (changed).** It gains the three options, the error re-exports, the plaintext warning and `EncodeSavedEntity` in display mode.
- **Standard library only.** The plan uses `crypto/aes`, `crypto/cipher`, `crypto/hmac`, `crypto/sha256`, `crypto/rand`, `encoding/base64`, `encoding/hex` and `encoding/json`, and adds no new module dependency. Tests use testify `require`, which is already a dependency.
- **`internal/cty` and `internal/xcl` (unchanged).** No change is expected to either copied library, so neither `UPSTREAM.md` changes. If one turns out to be needed, it keeps the licence headers and is recorded there.
- **`xcl-website` repo.** It gains a section in the events guide, a new state masking page and a navigation entry. Its snippets are copied text with no code dependency, so it lands after the library docs. Its gate is its own clean install, build and type-check.
- **Knowledge entries relied on.** `learnings/state-save-and-load-points.md`, `gotchas/custom-marshaljson-changes-internal-hops.md`, `architecture/shared-public-types-live-in-types.md`, `conventions/shared-errors-package.md`, `conventions/test-state-from-real-apply.md` and `conventions/dependencies.md`. None are changed by this plan.
- **GitHub issue jumppad-labs/xcl#1.** It can be closed once this plan lands, because events, plugin log details, state and printed output are all covered by this plan and its predecessor.

## Testing Approach

All tests follow the project's conventions:
- they use testify `require`;
- there are no table-driven tests, and every accepted case and every rejected case has its own named test function;
- test state comes from a real `Apply`, and a reload test builds a second `Config` on the state the first apply wrote.

The one exception is a unit test of the envelope format itself, which compares against a fixed document. One known secret value is used throughout the new fixtures, so each leak assertion is a plain "output does not contain this string".

**Unit tests: the built-in maskers and `Unmask` (heaviest unit coverage).** These carry the spec's "Built-in maskers behave as named" criterion directly.
- The encryption masker:
  - its output differs from the input;
  - it opens to the original with the same key;
  - it fails to open under a different key;
  - two maskings of one value differ, because the nonce is fresh;
  - a key that is not 32 bytes is rejected.
- The keyed hash gives the same output for the same input and key, and a different output for a different key. An empty key is rejected.
- `Omit` leaves no value. `Redact` gives the marker.
- `Unmask` opens a reversible envelope. Given a one-way masker's envelope, a different masker's envelope, or a wrong key, it fails with `ErrUnrecoverable` and returns no value. Each case is its own test.

**Unit tests: the wire encoder and the saved-entity reader.**
- With a masker, the wire encoder writes an envelope in place of every sensitive value: top-level, nested, embedded, in slices and maps, and held in `any`.
- With no masker its output is byte-identical to before, and it reports correctly whether any sensitive value was written.
- The reader in state mode opens envelopes from the state masker, and passes plain values through.
- It fails with `ErrUnrecoverable`, naming the entity and the masker, for an envelope from another masker, a one-way masker, or a wrong key.
- In display mode it turns every envelope into the marker.

**End-to-end tests: state (the spec's state criteria).**
- With the encryption masker configured for state, a real apply leaves a state file that does not contain the secret anywhere. A second configuration with the same key reads the real value back. The same holds through destroy's per-resource saves.
- With a different key, loading fails with `ErrUnrecoverable`. With no masker, encrypted state also fails to load.
- State written in plain text loads once a masker is added, and the next save encrypts it.
- A custom reversible masker configured for state puts its own output in state for each sensitive value.
- With no state masker, an apply of a configuration holding a sensitive value writes it in plain text and emits exactly one warning event saying sensitive values are stored unencrypted. A configuration with no sensitive values emits no such warning, and neither does a run with no state store.
- `NewConfig` with each one-way built-in, or a custom one-way masker, for state fails with `ErrMaskNotReversible`, and the message says the state masker must be reversible.

**End-to-end tests: events (the spec's event criteria).**
- With event data at both levels and no event masker chosen, every event carrying resource data shows each sensitive value as the redact envelope holding the marker, and never the secret.
- A custom event masker's output appears in event data for each sensitive value.
- With `WithNoEventMask()`, event data carries the real values.
- `EncodeSavedEntity` on masked event data and on encrypted state shows the marker, and never ciphertext or a hash.

**Leak and redaction tests.**
- With event masking turned off, an error whose message includes a sensitive value contains the marker and not the secret, and so do log details. Each is its own test.
- The predecessor's end-to-end leak suite gains state with the encryption masker, so one run covers events, log details, errors, configuration text, printer output and state under default settings plus encrypted state.

**Regression.** The existing parser, state, plugin, event, encode and query tests must pass. The predecessor's event-data tests change from asserting the bare marker to asserting the redact envelope.

**Examples and documentation.**
- The application-config and plugin example tests set a state key and assert that neither the state file nor any event data contains a known secret. A run without the key prints the plaintext warning.
- The README and CHANGELOG content tests are extended for the masking section and the changelog entry, each in its own test.
- The site has no content tests. Its build and type-check are the automated gate.

**Success metrics.**
- *Zero occurrences of a known test secret in state (with an encryption masker) or in event data, across the full test suite and every bundled example, under default settings*:
  - **Behavioural test.** The end-to-end state and event tests, the extended leak suite and the per-example tests assert it for every save site, both event data levels and every bundled example.
  - **Manual — captured in the implementation test plan.** Run the full suite and every example with the state key set, then search every state file and all captured output for the known secret.
- *Issue #1 can be closed: every area it lists — events, plugin log details, state, and printed output — is covered*:
  - **Behavioural test.** The leak suite covers events, plugin log details and printed output. The state tests cover state.
  - **Manual — captured in the implementation test plan.** Walk issue #1's list against the merged behaviour and close the issue.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: read the events guide's masking section and the new state masking page on the `xcl-website` site in a browser. Check that they cover configuring state and event maskers, the four built-ins, the plaintext-state warning and turning event masking off. Check also that the page is reachable from the navigation.
- **Manual — captured in the implementation test plan**: read the README's masking subsections and the state guide. Check that they describe key handling, refuse to suggest a one-way state masker, and match the site.

**Deliberate gaps.**
- There is no test for state written by earlier versions beyond plain-value loading (spec Non-Goals).
- There is no whole-file state encryption, so no test (spec Non-Goals).
- Key rotation is not tested, because it is not offered: changing the key makes existing encrypted state unreadable, as the wrong-key test shows.

## Milestones & Tasks

### Milestone 1: Developers can mask a sensitive value with a built-in or their own masker

**What changes**: A new public `mask` package lets developers turn a sensitive value into masked data, and open it again where that is possible. It offers four built-ins:
- AES-256-GCM encryption, which can be reversed;
- a keyed HMAC-SHA256 hash, which is one way;
- omit, which leaves no value;
- redact, which writes the marker.

A developer can also write their own by implementing `Masker`. Masked data names the masker that produced it. `mask.Unmask` returns the original only when it can do so faithfully, and otherwise reports the value as unrecoverable. Inside xcl, the internal encoder can now mask every sensitive value it writes, and the saved-data reader can recognise masked values. Nothing is configured to use them yet, so state and events behave as before.

**Validation point**: Unit tests prove each built-in behaves as named and that `Unmask` refuses one-way, mismatched and wrong-key data. Encoder and reader tests prove envelopes are written and recognised at every depth. The full existing suite passes unchanged.

#### - [x] Task: Add the masking package and its built-in maskers
**Id:** 63dd046a-f9af-47cc-9ee6-de20de649e9f
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

A new public `mask` package defines the `Masker` interface developers implement, the `Reversible` interface for maskers that can open what they produce, and the envelope every masked value is written in, which names its masker. It provides the four built-ins the spec names: AES-256-GCM encryption, keyed HMAC-SHA256 hashing, omit and redact. `mask.Unmask` opens an envelope only when it can do so faithfully, and otherwise reports the value as unrecoverable. The shared errors package gains the unrecoverable and not-reversible errors, re-exported from `xcl`.

*Technical detail:* [context.md#task-add-the-masking-package-and-its-built-in-maskers](./context.md#task-add-the-masking-package-and-its-built-in-maskers)

**Acceptance criteria**:
- [x] The encryption masker's output differs from its input, opens to the original with the same key, and fails to open with a different key.
- [x] The keyed hash gives the same output for the same input and key, and a different output for a different key.
- [x] The omit masker leaves no value, and the redact masker gives the fixed marker.
- [x] Every masked value names the masker that produced it.
- [x] Opening data produced by a one-way masker, by a different masker, or under the wrong key reports it as unrecoverable and returns no value.
- [x] A developer-written type with a name and a mask method satisfies the masker interface.

#### - [x] Task: Mask sensitive values in the internal encoder
**Id:** bd389804-35d5-4b1a-b162-c80d2af8c8e3
**Repo:** xclconfig
**Depends on:**
- 63dd046a-f9af-47cc-9ee6-de20de649e9f — Add the masking package and its built-in maskers
**Execution:** agent

The internal encoder that writes entities with their real sensitive values gains an optional masker. When it is given one, each sensitive value, wherever it sits in the entity, is written as that masker's envelope. The encoder also reports whether it wrote any sensitive value at all, which the plaintext warning needs. Without a masker its output is exactly as before, so every existing hop is unaffected.

*Technical detail:* [context.md#task-mask-sensitive-values-in-the-internal-encoder](./context.md#task-mask-sensitive-values-in-the-internal-encoder)

**Acceptance criteria**:
- [x] With a masker, every sensitive value is written as that masker's envelope, at the top level, nested, embedded, and inside lists, maps and dynamic values.
- [x] Without a masker, the output is byte-identical to before.
- [x] The encoder reports correctly whether an entity held any sensitive value.

#### - [x] Task: Recognise masked values when reading saved data
**Id:** d6ea57af-4fe8-4fcc-8167-68ff75ca644c
**Repo:** xclconfig
**Depends on:**
- 63dd046a-f9af-47cc-9ee6-de20de649e9f — Add the masking package and its built-in maskers
**Execution:** agent

The one reader of saved records learns to find masked values before it types a record, and it has two modes. When loading state, it opens each value with the state masker. It fails, naming the entity and the masker, for anything it cannot open, so a secret is never silently replaced by a redacted value. When reading for display, it turns every masked value into the marker. Plain values pass through unchanged in both modes.

*Technical detail:* [context.md#task-recognise-masked-values-when-reading-saved-data](./context.md#task-recognise-masked-values-when-reading-saved-data)

**Acceptance criteria**:
- [x] A record holding values masked by the state masker reads back with the real values.
- [x] A record holding a value masked by another masker, by a one-way masker, or under a different key fails to load with an unrecoverable-value error naming the entity and the masker.
- [x] Read for display, any masked value reads back as the marker, and never as ciphertext or a hash.
- [x] A record holding plain sensitive values reads as before, with or without a state masker.

### Milestone 2: State encrypts sensitive values when given a key, and warns when it holds them in plain text

**What changes**: A developer can pass `xcl.WithStateMask` with the encryption masker, or with their own reversible masker. After that, state never holds a sensitive value in plain text, and a later run with the same key reads the real values back. Loading state that cannot be opened, under the wrong key or with no masker, fails clearly instead of losing the secret. Giving a one-way masker for state fails at setup with an error saying the state masker must be reversible. Without a state masker, state still works and holds values in plain text. Each apply or destroy that writes a sensitive value then emits one warning saying so.

**Validation point**: End-to-end tests apply a configuration holding a known secret with the encryption masker and find no trace of it in the state file. They reload the state with the same key and fail it under another. They show the one-way rejection, the warning with plain state, and the absence of the warning when there is nothing sensitive.

#### - [x] Task: Encrypt sensitive values in state
**Id:** 7a0b0774-b861-400c-9056-bb43cb73d3f5
**Repo:** xclconfig
**Depends on:**
- bd389804-35d5-4b1a-b162-c80d2af8c8e3 — Mask sensitive values in the internal encoder
- d6ea57af-4fe8-4fcc-8167-68ff75ca644c — Recognise masked values when reading saved data
**Execution:** agent

`xcl.WithStateMask` sets the masker for state, and setting up xcl with a masker that cannot recover values fails with an error saying the state masker must be reversible. Both state save points, after apply and during destroy, write sensitive values through the state masker. Both load points open them with it. With the encryption masker configured, the state file never holds a secret, and a later configuration with the same key reads the real values back. Without a state masker, state works exactly as before.

*Technical detail:* [context.md#task-encrypt-sensitive-values-in-state](./context.md#task-encrypt-sensitive-values-in-state)

**Acceptance criteria**:
- [x] After applying a configuration holding a known secret with the encryption masker, the state file does not contain the secret anywhere, including after destroy's saves.
- [x] A new configuration with the same key loads that state and reads back the real value. With a different key, or with no masker, loading fails with an unrecoverable-value error.
- [x] A developer's own reversible masker configured for state puts its output in state for each sensitive value and reads it back.
- [x] Setting up xcl with a one-way masker for state fails with an error saying the state masker must be reversible.
- [x] State written in plain text loads once a masker is configured, and the next save encrypts it.

#### - [x] Task: Warn when state holds sensitive values in plain text
**Id:** bc117ce9-4307-4c5c-846d-0346cbc35e55
**Repo:** xclconfig
**Depends on:**
- 7a0b0774-b861-400c-9056-bb43cb73d3f5 — Encrypt sensitive values in state
**Execution:** agent

When no state masker is configured and an apply or destroy writes a sensitive value to state, xcl emits one warning event for that operation saying sensitive values are stored unencrypted and how to encrypt them. A configuration with nothing sensitive, a run with no state store, and a run with a state masker emit no such warning.

*Technical detail:* [context.md#task-warn-when-state-holds-sensitive-values-in-plain-text](./context.md#task-warn-when-state-holds-sensitive-values-in-plain-text)

**Acceptance criteria**:
- [x] Applying a configuration holding a sensitive value with no state masker writes it in plain text and emits exactly one warning saying sensitive values are stored unencrypted.
- [x] A configuration with no sensitive values emits no such warning.
- [x] With a state masker configured, or with no state store, no such warning is emitted.

### Milestone 3: Events redact sensitive values by default, and developers choose how, or turn it off

**What changes**: Resource data on events shows each sensitive value through the configured event masker:
- by default, the redact masker, showing the marker;
- through `xcl.WithEventMask`, any built-in or custom masker, such as a keyed hash for correlation;
- with `xcl.WithNoEventMask()`, the real values.

Errors and log details keep showing only the marker whatever is chosen. Turning masked event data or masked state back into configuration text shows the marker.

**Validation point**: End-to-end tests at both event data levels show the redact envelope by default, a custom masker's output when one is chosen, and real values when masking is off. They show errors and log details still redacted with masking off, and configuration text showing the marker for masked data. The extended leak suite passes.

#### - [x] Task: Mask event data with the configured event masker
**Id:** d6e49599-822b-495b-9369-d76f8a497267
**Repo:** xclconfig
**Depends on:**
- bd389804-35d5-4b1a-b162-c80d2af8c8e3 — Mask sensitive values in the internal encoder
- d6ea57af-4fe8-4fcc-8167-68ff75ca644c — Recognise masked values when reading saved data
**Execution:** agent

Resource data on events is written through the configured event masker. That is the redact masker by default, any masker given with `xcl.WithEventMask`, or none after `xcl.WithNoEventMask()`, in which case events carry real values. Turning masked event data or masked state into configuration text shows the marker. Log events and errors are not affected by this choice.

*Technical detail:* [context.md#task-mask-event-data-with-the-configured-event-masker](./context.md#task-mask-event-data-with-the-configured-event-masker)

**Acceptance criteria**:
- [x] With events carrying resource data and no event masker chosen, every such event shows each sensitive value as the marker, named as redacted, and never the real value, at both data levels.
- [x] A developer's own event masker's output appears in event data for each sensitive value.
- [x] With event masking turned off, event data carries the real sensitive values.
- [x] Masked event data and encrypted state each turn into configuration text that shows the marker.

#### - [x] Task: Prove errors and every output stay redacted whatever the event masking
**Id:** 2f0a47bf-6508-4afa-93b3-9ee472f95ad3
**Repo:** xclconfig
**Depends on:**
- 7a0b0774-b861-400c-9056-bb43cb73d3f5 — Encrypt sensitive values in state
- d6e49599-822b-495b-9369-d76f8a497267 — Mask event data with the configured event masker
**Execution:** agent

With event masking turned off, xcl errors that mention a sensitive value, and plugin log details, still show only the marker. The existing end-to-end leak suite is extended with state encrypted by the encryption masker. One run then proves that a known secret appears nowhere under default settings: not in events, log details, errors, configuration text, printed output or state.

*Technical detail:* [context.md#task-prove-errors-and-every-output-stay-redacted-whatever-the-event-masking](./context.md#task-prove-errors-and-every-output-stay-redacted-whatever-the-event-masking)

**Acceptance criteria**:
- [x] With event masking turned off, an error whose message includes a sensitive value contains the marker and not the real value.
- [x] With event masking turned off, plugin log details still show the marker.
- [x] Under default settings with state encrypted, no captured event, log, error, configuration text, printed output or state file contains the known secret.

### Milestone 4: The examples encrypt their state and the documentation explains masking

**What changes**: The application-config and plugin examples encrypt their state when a key is supplied in the environment, and show the plaintext warning when it is not. The README, the state guide and the changelog describe:
- the maskers and how to configure them for state and for events;
- the plaintext warning;
- turning event masking off;
- writing a custom masker.

The documentation site's events guide gains a masking section, and a new state masking page is reachable from the navigation.

**Validation point**: The example tests find no known secret in state or event data with a key set. The README and changelog content tests pass, and fail if the new text is removed. The site builds and type-checks.

#### - [x] Task: Encrypt the examples' state
**Id:** c6bb82ad-918f-4bdb-85bf-d7e6948a2676
**Repo:** xclconfig
**Depends on:**
- bc117ce9-4307-4c5c-846d-0346cbc35e55 — Warn when state holds sensitive values in plain text
- d6e49599-822b-495b-9369-d76f8a497267 — Mask event data with the configured event masker
**Execution:** agent

The application-config and plugin examples, which hold passwords, encrypt their state with the encryption masker when a key is supplied in the environment. Without one, they run with the plaintext warning showing. Their tests run with a key and assert that the state file and all event data hold no known secret. They also check that a run without a key prints the warning.

*Technical detail:* [context.md#task-encrypt-the-examples-state](./context.md#task-encrypt-the-examples-state)

**Acceptance criteria**:
- [x] With a key set, running each of the two examples leaves no password or secret in its state file or in any event data it prints.
- [x] Without a key, each of the two examples still runs and prints the plaintext-state warning.
- [x] The configuration-only example, which holds no secret, prints no warning.

#### - [x] Task: Document masking in the library
**Id:** 957cadca-2015-4bb2-8297-769cdd4bfbd9
**Repo:** xclconfig
**Depends on:**
- 2f0a47bf-6508-4afa-93b3-9ee472f95ad3 — Prove errors and every output stay redacted whatever the event masking
- c6bb82ad-918f-4bdb-85bf-d7e6948a2676 — Encrypt the examples' state
**Execution:** agent

The README's sensitive-values section gains subsections covering:
- encrypting state with `WithStateMask`, including key handling and the reversible-only rule;
- the plaintext warning;
- masking events with `WithEventMask`, and turning masking off with `WithNoEventMask`;
- the four built-ins;
- writing a custom masker;
- opening masked event data.

The state guide stops saying processed event data is identical to state, and explains masked state. The changelog gains an entry for this spec with its breaking changes, and content tests guard all of it.

*Technical detail:* [context.md#task-document-masking-in-the-library](./context.md#task-document-masking-in-the-library)

**Acceptance criteria**:
- [x] The README describes configuring state and event maskers, the four built-in maskers, the plaintext-state warning, turning event masking off and writing a custom masker.
- [x] The state guide describes masked state and no longer says event data is always byte for byte what state holds.
- [x] The changelog has an entry for this spec that lists its breaking changes.
- [x] Content tests fail if any of these sections or the changelog entry is removed.

#### - [x] Task: Document masking on the site
**Id:** c6c7a600-ee67-4ea5-845b-f3110a25f0c4
**Repo:** xcl-website
**Depends on:**
- 957cadca-2015-4bb2-8297-769cdd4bfbd9 — Document masking in the library
**Execution:** agent

The events guide gains a section on what event data shows for sensitive values. It covers the default redaction, choosing another masker and turning masking off. A new state masking guide page, linked from the navigation, covers encrypting state, the reversible-only rule, the plaintext warning and the four built-in maskers. The wording follows the library documentation.

*Technical detail:* [context.md#task-document-masking-on-the-site](./context.md#task-document-masking-on-the-site)

**Acceptance criteria**:
- [x] The events guide describes event masking by default, choosing a masker and turning masking off.
- [x] The site has a state masking page, reachable from the navigation, covering configuring the state masker, the built-in maskers and the plaintext-state warning.
- [x] The site builds and type-checks.

## Open Questions

- **Does any lifecycle event site pass pre-call bytes without the typed entity?** This depends on how the sensitive-values plan leaves `eventData` and its callers. Without the typed entity, the event masker cannot find sensitive values in the bytes, which now hold real values. The event task adds a test that every lifecycle emission passes the entity. If a site cannot, it must decode the bytes into the entity's registered type before encoding. If no type can be found, it must carry no data rather than real values. No question to the user is needed unless that would drop data an existing test or documented behaviour requires. In that case, STOP and ask.
- **Does the predecessor's wire encoder treat every sensitive value in one place?** This depends on the encoder as built. If sensitive values inside `any`, maps or slices take a separate branch, each branch must apply the masker and set the "sensitive written" report. The encoder tests for each position will show it. This needs no question to the user.

## Out of Scope

- **Backwards compatibility with existing state files** (spec Non-Goals). Plain-text state from earlier versions happens to load, but nothing beyond that is promised or tested.
- **Whole-file encryption of state** (spec Non-Goals). Only sensitive values are masked. Whole-file encryption may come later.
- **Key rotation and multiple decryption keys.** Changing the state key makes existing encrypted state unreadable until it is decrypted with the old key. No re-key tool is provided.
- **Key management.** xcl takes key bytes. Where the key is kept, and how it is fetched, is the application's concern.
- **Masking log-event details and errors.** These always show the marker, through the self-protecting sensitive type. Event masking applies only to resource data on events.
- **Opening masked data inside `EncodeSavedEntity`.** Configuration text from masked data always shows the marker. A receiver that needs the real value opens it with `mask.Unmask`.
- **Masking values in fields not declared sensitive, and declaring a `variable` sensitive.** These are left out, as in the sensitive-values plan `20261003134528-327e0657-references-and-secrets`.
- **Changes to the copied cty and HCL libraries.** None are planned.

## Changelog


### 2026-10-05 — Task: Add the masking package and its built-in maskers

**What was done**: Added the public `mask` package with the `Masker` and `Reversible` interfaces, the `Masked` envelope (`{"xcl_masked":...,"value":...}`), `Envelope`, `IsMasked`, `IsMaskedObject`, `Unmask`, `Open`, and the four built-ins `EncryptAES256GCM`, `HashHMACSHA256`, `Omit` and `Redact` (standard library crypto only). The `errors` package gained `ErrUnrecoverable`/`UnrecoverableError` and `ErrMaskNotReversible`/`MaskNotReversibleError`, re-exported from `xcl`.

**Deviations**: Added small helpers beyond the plan's surface: `mask.Envelope` (masks one value and wraps it, used by the wire encoder), `mask.IsMaskedObject` and `mask.Open` (for the saved-entity reader, which holds decoded maps), and exported name constants (`AES256GCMName`, `HMACSHA256Name`, `OmitName`, `RedactName`, `EnvelopeKey`). `UnrecoverableError.Unwrap` returns both the sentinel and the underlying cause.

**Files changed**:
- `xclconfig: mask/mask.go`
- `xclconfig: mask/aes.go`
- `xclconfig: mask/hmac.go`
- `xclconfig: mask/omit.go`
- `xclconfig: mask/redact.go`
- `xclconfig: mask/aes_test.go`
- `xclconfig: mask/hmac_test.go`
- `xclconfig: mask/omit_test.go`
- `xclconfig: mask/redact_test.go`
- `xclconfig: mask/unmask_test.go`
- `xclconfig: mask/custom_test.go`
- `xclconfig: errors/mask_errors.go`
- `xclconfig: errors/mask_errors_test.go`
- `xclconfig: config.go`

**Discoveries**: `Open` checks the masker name before reversibility, so a one-way envelope opened with a different masker reports "different masker" rather than "one way".

### 2026-10-05 — Task: Mask sensitive values in the internal encoder

**What was done**: `internal/wire` gained `Options{Mask}`, `Result{Data, Sensitive}` and `Encode`. The recursive walk now runs on a per-call `encoder` value carrying the options and a "sensitive written" flag; with a masker, each sensitive value is written as `mask.Envelope(<real value JSON>, masker)`, and a redacted value masks the marker. `Marshal` is `Encode` with zero options, so every existing hop is byte-identical.

**Deviations**: None.

**Files changed**:
- `xclconfig: internal/wire/wire.go`
- `xclconfig: internal/wire/wire_test.go`

**Discoveries**: The real value handed to a masker is encoded by a fresh, maskless inner encoder, so a masker always receives the real JSON regardless of nesting.

### 2026-10-05 — Task: Recognise masked values when reading saved data

**What was done**: `savedentity.Decode` and `DecodeAll` take a `ReadOptions{Mask, ForDisplay}`. Before typing a record, `openMasked` walks the decoded tree and replaces every envelope: with the marker for display, otherwise with the value the state masker opens it to. Anything that cannot be opened fails with `*xclerrors.UnrecoverableError` naming the record and masker; `DecodeAll` returns it directly instead of folding it into `state.UnknownTypesError`. Every caller passes `savedentity.ReadOptions{}` for now.

**Deviations**: None. Callers updated mechanically, including test files across the repo (state, plugins/example, parser, root tests).

**Files changed**:
- `xclconfig: internal/savedentity/savedentity.go`
- `xclconfig: internal/savedentity/savedentity_test.go`
- `xclconfig: internal/savedentity/decode_all_test.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: encode.go`
- `xclconfig: internal/parser/dag_test.go`
- `xclconfig: internal/parser/lifecycle_test.go`
- `xclconfig: config_destroy_test.go`
- `xclconfig: config_outputs_sensitive_test.go`
- `xclconfig: config_sensitive_roundtrip_test.go`
- `xclconfig: config_sensitive_state_test.go`
- `xclconfig: config_test.go`
- `xclconfig: config_validate_test.go`
- `xclconfig: encode_errors_test.go`
- `xclconfig: plugins/example/apply_test.go`
- `xclconfig: plugins/example/sensitive_test.go`
- `xclconfig: state/custom_store_test.go`
- `xclconfig: state/file_state_store_apply_test.go`

**Discoveries**: `registered.Secret` (`resource`/`secret`, `Password types.Sensitive[string]`) is the existing fixture type for sensitive-record tests.

### 2026-10-05 — Task: Encrypt sensitive values in state

**What was done**: Added `xcl.WithStateMask` (rejects nil and non-`Reversible` maskers with `ErrMaskNotReversible`), a `stateMask` field on `Config` and `StateMask` on `ParserOptions`, threaded through all three parser constructions. `parser.EncodeForState(entities, stateMask)` now encodes through `wire.Encode` with the state masker and also returns whether a sensitive value was written unmasked; both save sites pass the masker, and both load paths read with `ReadOptions{Mask: StateMask}`.

**Deviations**: The nil-masker rejection wraps `ErrMaskNotReversible` (with "no masker was given") rather than a separate error, so one sentinel covers every invalid state masker. Existing test callers of `EncodeForState` pass `nil` and ignore the new bool.

**Files changed**:
- `xclconfig: options.go`
- `xclconfig: config.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/state_encode.go`
- `xclconfig: internal/parser/destroy.go`
- `xclconfig: config_state_mask_test.go`
- `xclconfig: config_options_test.go`
- `xclconfig: config_sensitive_state_test.go`
- `xclconfig: internal/parser/lifecycle_test.go`
- `xclconfig: internal/parser/registered_types_test.go`
- `xclconfig: plugins/example/apply_test.go`
- `xclconfig: plugins/example/sensitive_test.go`

**Discoveries**: The file state store writes indented JSON, so string assertions on a state file must match `"xcl_masked": "aes-256-gcm"` with a space; a recording store sees the compact form.

### 2026-10-05 — Task: Warn when state holds sensitive values in plain text

**What was done**: Apply and Destroy emit one warn-level log event per operation (Source `core`, the operation's name, Phase `log`) with the message `parser.PlaintextStateWarning` when a save to a configured store wrote a sensitive value with no state masker. Apply uses the flag `EncodeForState` returns; Destroy's `destroyer` records it across its per-resource saves and `Parser.Destroy` warns once after the walk.

**Deviations**: `example/plugin/main_test.go` assumed a run emits no warnings. Until the examples task gives the example a state key, `TestPluginExampleReportsNoWarnings` skips the plaintext warning and `TestPluginExampleReportsProviderCallLogsAtInfo` counts only non-core log events.

**Files changed**:
- `xclconfig: internal/parser/state_encode.go`
- `xclconfig: internal/parser/destroy.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: config.go`
- `xclconfig: config_state_mask_test.go`
- `xclconfig: example/plugin/main_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Mask event data with the configured event masker

**What was done**: Added `xcl.WithEventMask` (nil fails `NewConfig`) and `xcl.WithNoEventMask()`, last one wins; `NewConfig` defaults the event masker to `mask.Redact()` and passes it as `ParserOptions.EventMask`. `eventData` now encodes through `wire.Encode` with the event masker, and the pre-call snapshot is re-read into the resource's type and masked (`maskedSnapshot`, formerly `redactedSnapshot`). `EncodeSavedEntity` reads in display mode, so masked event data and encrypted state show the marker. Docs on `Event.Data`, `DataProcessed`, `EventDataProcessed` and `WithEventData` updated.

**Deviations**: `parser.DefaultOptions()` also sets `EventMask: mask.Redact()`, so a standalone parser built from its defaults redacts event data like `Config` does; a zero `ParserOptions` still means real values. With masking off, a pre-call snapshot is passed on as is. The planned test that every lifecycle emission passes the entity was not added: inspection shows every lifecycle emission with data passes `r`; the one site passing nil (`callbacks.go`, apply error) carries no data.

**Files changed**:
- `xclconfig: options.go`
- `xclconfig: config.go`
- `xclconfig: internal/parser/parser.go`
- `xclconfig: internal/parser/events.go`
- `xclconfig: encode.go`
- `xclconfig: events.go`
- `xclconfig: events/events.go`
- `xclconfig: config_event_mask_test.go`
- `xclconfig: config_event_sensitive_test.go`
- `xclconfig: config_sensitive_roundtrip_test.go`
- `xclconfig: internal/parser/events_sensitive_test.go`
- `xclconfig: plugins/example/sensitive_test.go`

**Discoveries**: Event consumers now see a sensitive value as `{"xcl_masked":"redact","value":"(sensitive)"}` rather than the bare marker string.

### 2026-10-05 — Task: Prove errors and every output stay redacted whatever the event masking

**What was done**: The end-to-end leak suite now applies its fixtures with state encrypted by AES-256-GCM (`applyLeakFixtureWithOptions`), and `TestLeakSuiteStateFileHoldsNoSecret` checks the state file holds no secret and holds envelopes. New tests show that with `WithNoEventMask()` function errors, validation errors and plugin log details still never contain the secret.

**Deviations**: No production code changed (a proof task). The validation-error test asserts the secret is absent but not the marker, because that error prints no value at all, as the existing leak test for the same fixture does.

**Files changed**:
- `xclconfig: sensitive_leak_test.go`
- `xclconfig: config_event_mask_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Encrypt the examples' state

**What was done**: The application-config and plugin examples read `XCL_STATE_KEY` (base64, 32 bytes) and, when it is set, add `xcl.WithStateMask(mask.EncryptAES256GCM(key))`; `run` takes the key as a new last parameter. Without a key they run as before and xcl's plaintext-state warning shows. Their tests run with a fixed test key and assert no password reaches the state file or any event data; separate tests check the warning without a key, and that the configuration-only example never warns. The plugin example's warning tolerances added in the warning task were removed again.

**Deviations**: The plugin example destroys before `run` returns, so its state test reads the state files when the apply success event arrives. Also fixed a flaky assertion in `TestPluginEventDataShowsTheMarkerForTheSensitiveInteger`, which searched the whole event data for "1234" and could match the random temp path; it now checks only the `pin` field.

**Files changed**:
- `xclconfig: example/appconfig/main.go`
- `xclconfig: example/appconfig/main_test.go`
- `xclconfig: example/plugin/main.go`
- `xclconfig: example/plugin/main_test.go`
- `xclconfig: example/configonly/main_test.go`
- `xclconfig: config_sensitive_roundtrip_test.go`

**Discoveries**: Event data includes `meta.file`, a temp path in tests, so a leak assertion for a short or numeric secret must target the field, not the whole payload.

### 2026-10-05 — Task: Document masking in the library

**What was done**: The README's Sensitive values section gained subsections on encrypting state, the plaintext state warning, masking events, the built-in maskers (a table) and writing your own masker, and "Resource data on events" now says sensitive values in `Data` are masked. `docs/state.md` replaced the "event data shows the marker, state holds real values" passage, gained a "Sensitive values in state" section, and points the plain-text note at `WithStateMask`. `CHANGELOG.md` has a top entry for this spec with its breaking changes. Ten content tests in `readme_test.go` guard all of it.

**Deviations**: The CHANGELOG Breaking list names only changes against the last release (epic decision); the earlier, unreleased references-and-secrets entry that describes bare-marker event data was left as written, and this entry states the envelope format.

**Files changed**:
- `xclconfig: README.md`
- `xclconfig: docs/state.md`
- `xclconfig: CHANGELOG.md`
- `xclconfig: readme_test.go`

**Discoveries**: None.

### 2026-10-05 — Task: Document masking on the site

**What was done**: The events guide gained "Sensitive values in event data" (default redact envelope, `xcl.WithEventMask` with a keyed-hash example, `xcl.WithNoEventMask()`, errors and log details always redacted, `mask.Unmask`). A new State masking page covers the plaintext warning, encrypting state with `mask.EncryptAES256GCM` and key handling, the reversible-only rule, reloading and the wrong-key error, the built-in masker table and a custom reversible masker; it is linked from the Guides navigation, the site README's Pages table and the Sensitive values page.

**Deviations**: The site has no content tests, so the test step wrote none; `npm ci`, `npm run build` and `npx astro check` (0 errors, 0 warnings) are the gate.

**Files changed**:
- `xcl-website: src/pages/events.mdx`
- `xcl-website: src/pages/state-masking.mdx`
- `xcl-website: src/pages/sensitive-values.mdx`
- `xcl-website: src/components/Nav.astro`
- `xcl-website: README.md`

**Discoveries**: None.

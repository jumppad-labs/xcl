---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Research: 20261003153421-9fa72edd-masking

## Alternatives considered and rejected

- **Choosing the masker inside `Sensitive[T].MarshalJSON` through global or goroutine state.** Rejected: Go's JSON marshalling carries no per-call context, and the spec's Technical Approach directs choosing the masker per serialisation call, not through global state. The same `Sensitive[T]` is marshalled concurrently for state, provider calls and events (`internal/parser/lifecycle.go` walks in parallel), so any global would race. The predecessor plan already routes every internal hop through `internal/wire` at the call site (references-and-secrets plan, "Implementation Detail: two encodings of one entity, chosen by the caller").
- **A masking stage over the encoded JSON bytes, after `wire.Marshal`.** Rejected: once encoded, a sensitive value is indistinguishable from a plain string. Only the typed walk in `internal/wire` knows which leaves are `types.SensitiveValue`.
- **A type-driven custom decoder (`wire.Unmarshal`) for loading masked state.** Rejected: it would duplicate `encoding/json`'s decoding rules in reflection code. `savedentity.Decode` already unmarshals each record into a generic `map[string]any` first (`internal/savedentity/savedentity.go:31-34`), so masked values can be replaced in that tree before the typed unmarshal, and `Sensitive[T].UnmarshalJSON` then receives the real `T` as the predecessor already supports. This also covers sensitive leaves held in `any` (`types.Output.Value`), which a type-driven decoder could not see.
- **Writing the redact masker's output as the bare marker string `"(sensitive)"`, and an envelope only for other maskers.** Rejected: the spec's acceptance criterion "Masked data names its masker" needs every masked value to name its masker. One uniform envelope shape is also simpler for readers.
- **Omitting the JSON key entirely for `mask.Omit`.** Rejected: the value would then be indistinguishable from a field that was never set, and the data would not name its masker. `Omit` writes an envelope that names it and carries no value.
- **Whole-file encryption of state.** Rejected by the spec's Non-Goals.
- **Silently reading an unopenable masked state value as redacted.** Rejected: the next save would write the redacted value back over the real one, losing the secret. State loading fails loudly instead (the same reasoning `savedentity.DecodeAll` records for unreadable records, `internal/savedentity/savedentity.go:91-96`).
- **Applying event masking to log-event `Meta` details.** Rejected: log details hold the `Sensitive[T]` itself and already format as the marker, and the predecessor sends them across the plugin boundary as the marker. The spec scopes event masking to "resource data on events".
- **Built-in masker constructors that defer key errors to `NewConfig`.** Rejected in favour of idiomatic Go: constructors that take a key return `(Masker, error)`.

## Chosen approach — evidence

- `config.go:315` and `internal/parser/destroy.go:130` are the only state save sites; `internal/parser/parser.go:360` (destroy) and `:451` (apply) are the only state load sites, both through `savedentity.DecodeAll` (`learnings/state-save-and-load-points.md`).
- The predecessor plan encodes each entity with `internal/wire` into `json.RawMessage` at both save sites, in one helper in `internal/parser` that both call. Adding a masker argument to that helper, and to `wire`, keeps the masker per call.
- `internal/parser/events.go:37-83` (`eventData`) is the single place event data is decided. The predecessor makes it always re-encode from the typed entity. Swapping `encoding/json` for `wire` with the event masker is a local change there.
- `savedentity.Decode` (`internal/savedentity/savedentity.go:31-77`) holds each record as `map[string]any` before the typed unmarshal: the natural place to replace envelopes.
- `encode.go:82-109` (`EncodeSavedEntity`) is the public reader of saved and event data, and calls `savedentity.Decode`.
- `options.go` holds every `ConfigOption`. `WithStatePath` (`options.go:37-46`) shows an option returning a setup error, which `NewConfig` (`config.go:155-170`) surfaces, so a one-way state masker can be rejected "when xcl is set up".
- `logger.New(emit, base).Warn` (`logger/emit.go:31-53`) produces a warn-level log event, the existing way core emits warnings (also `ParserOptions.Emit` doc, `internal/parser/parser.go:110-114`).
- The standard library provides `crypto/aes`, `crypto/cipher` (GCM), `crypto/hmac`, `crypto/sha256`, `crypto/rand` and `encoding/base64`, so no new dependency is needed (`conventions/dependencies.md`).
- `cty.Value` has no `MarshalJSON` (none found under `internal/cty`), so `resources.Variable.Default` (`internal/resources/variable.go:13`) serialises as `{}` and module inputs are `json:"-"` (`internal/resources/module.go:25`). Variables and modules therefore put no secret into state or event data, so masking `Sensitive[T]` leaves is enough for the "no plain secret in state" criterion.

## Files examined

- `xclconfig:config.go:131-170` — `Config` fields and `NewConfig` applying options in order; the first failing option's error is returned.
- `xclconfig:config.go:283-322` — Apply builds `ParserOptions`, saves `c.entities` through the store at `:315`; `emit` is in scope in the `run` closure.
- `xclconfig:config.go:335-371` — Destroy loads state from the store at `:345` and hands it to `p.Destroy`.
- `xclconfig:config.go:405-460` — `run`: events only flow with a handler; `emit` is nil otherwise.
- `xclconfig:options.go:1-103` — every `ConfigOption`; `WithEventData` at `:95`.
- `xclconfig:events.go:1-40` — root re-exports of event types and data levels.
- `xclconfig:events/events.go:110-150` — `Event.Data` doc and `DataLevel`.
- `xclconfig:internal/parser/events.go:28-83` — `eventData`, the one place event data is produced.
- `xclconfig:internal/parser/parser.go:84-117` — `ParserOptions` (`StateStore`, `EventData`, `Emit`).
- `xclconfig:internal/parser/parser.go:337-380,436-465` — the destroy and apply load paths through `savedentity.DecodeAll`.
- `xclconfig:internal/parser/destroy.go:122-142` — destroy's per-resource save.
- `xclconfig:internal/savedentity/savedentity.go:1-163` — `Decode`, `DecodeAll`, `recordData`; records held as `map[string]any`.
- `xclconfig:encode.go:59-110` — `EncodeEntity` and `EncodeSavedEntity`.
- `xclconfig:logger/emit.go:1-80` — event logger; `Warn` emits a `LevelWarn` log event.
- `xclconfig:internal/resources/variable.go:11-15`, `internal/resources/module.go:14-26` — variable and module serialisation.
- `xclconfig:example/{appconfig,configonly,plugin}/main.go` — all three examples use `WithStatePath` and `WithEventData(xcl.EventDataProcessed)`.
- `xclconfig:example/plugin/config/main.xcl:9-34` — password fed from `variable.db_password`.
- `xclconfig:readme_test.go:1-90` — README/CHANGELOG content-test style: one `require.Contains` theme per test.
- `xclconfig:static_output_test.go:113-172` — the AST-based static test pattern the predecessor extends for `internal/wire` importers.
- `xclconfig:docs/state.md:120-143` — says processed event data is byte for byte what state holds, which this spec makes untrue.
- `xclconfig:CHANGELOG.md:1-30` — entry format: `## <spec name>`, prose, `**Breaking:**` list.
- `xcl-website:src/components/Nav.astro:8-25` — Guides dropdown.
- `xcl-website:src/pages/events.mdx` — sections: slog, the event, operations and phases, sources, delivery, errors, pretty receiver.
- `xcl-website:package.json` — `astro build`; gate is `npm ci`, build and `astro check`.
- GitHub issue jumppad-labs/xcl#1 — lists events, plugin log details, state and printed output as the areas to cover.

## External references

- Go `crypto/cipher` GCM (`cipher.NewGCM`, `Seal`/`Open`, 12-byte nonce) — the AES-256-GCM masker; `Open` fails on a wrong key, giving the "fails to recover with a different key" criterion.
- Go `crypto/hmac` with `crypto/sha256` — the keyed one-way masker; deterministic per key.
- NIST SP 800-38D — GCM nonces must never repeat under one key, so each `Mask` draws a fresh random nonce from `crypto/rand`.
- jumppad-labs/xcl#1 — the success metric's checklist of areas.

## Prior plans / specs consulted

- `20261003134528-327e0657-references-and-secrets` (plan) — provides `types.Sensitive[T]`, `types.SensitiveValue` (`RevealAny`), `types.SensitiveMarker`, `types.IsRedacted`, `internal/wire` with an import-restriction static test, the shared state-encoding helper used at both save sites, `StateStore.Save` receiving `json.RawMessage`, and `eventData` always re-encoding from the typed entity. Its Out of Scope hands pluggable masking, state encryption, the plaintext-state warning and `WithNoEventMask` to this spec.
- `20261003153421-6ec0eab3-module-boundary-and-output-entities` (plan) — `types.Output` as an entity; its `Value` may hold sensitive leaves, which masking must reach inside `any`.
- `20261003153421-bf87d907-references-as-written` (plan) — adds `meta.references` (reference text, not values) to saved records and event data; no secret is added. Sensitive values keep the marker in configuration text.
- `20261003153421-c283547c-user-depends-on` (plan) — no overlap beyond the shared README/CHANGELOG/site files.
- Knowledge: `learnings/state-save-and-load-points.md`, `gotchas/custom-marshaljson-changes-internal-hops.md`, `architecture/shared-public-types-live-in-types.md`, `conventions/shared-errors-package.md`, `conventions/test-state-from-real-apply.md`, `conventions/testing-and-mocking.md`, `conventions/dependencies.md`.

## Open assumptions

- The predecessor's shared state-encoding helper and `eventData` re-encoding exist as planned when this lands. If either save site still hands the store typed entities, STOP: masking must hook in at the encoding helper, not in the stores.
- `internal/wire` walks every `types.SensitiveValue` through one code path (including inside `any`, maps and slices), so one masker hook covers all of them. If the predecessor's walk has more than one sensitive branch, each must take the masker.
- `cty.Value` keeps serialising as `{}`, so variables put no secret into state. If a variable's value starts reaching state, the "no plain secret in state" criterion needs revisiting: STOP and ask.
- No existing saved record contains an object whose only keys are `xcl_masked` and `value`. The key is reserved and documented.

## Drafting assumptions

### Masked values are written as a self-describing envelope (discovery)
- **Decision**: every masked value is written in place of the real value as `{"xcl_masked":"<masker name>","value":<masker output>}`, with `value` absent for `mask.Omit`. The redact masker writes `{"xcl_masked":"redact","value":"(sensitive)"}`.
- **Rationale**: the spec requires masked data to name its masker; one uniform shape lets readers detect and open masked values without type knowledge, including inside `any`.
- **Rejected**: bare marker for redact (does not name the masker); dropping the key for omit (indistinguishable from unset).

### Masked state is opened in the generic record tree, before typing (discovery)
- **Decision**: `savedentity` replaces each envelope in the decoded `map[string]any` record before the typed unmarshal: opened with the state masker when loading state, turned into the marker when reading for display (`EncodeSavedEntity`).
- **Rationale**: `savedentity.Decode` already holds the record as a generic tree; this avoids a custom typed decoder and reaches sensitive leaves inside `any`.
- **Rejected**: a reflection-based `wire.Unmarshal`.

### Unopenable masked state fails the load (discovery)
- **Decision**: loading state that holds a value masked by a masker other than the configured state masker, or one that fails to open (wrong key), fails with `ErrUnrecoverable` naming the entity and the masker.
- **Rationale**: reading it as redacted would save the redacted value back over the real one on the next save.
- **Rejected**: silently redacting.

### Event masking applies only to resource data (discovery)
- **Decision**: `WithEventMask`/`WithNoEventMask` change only `Event.Data`. Log details and errors keep showing the marker.
- **Rationale**: the spec scopes event masking to resource data and requires errors to stay redacted; log details hold the self-protecting value.
- **Rejected**: masking `Meta` too.

### Chosen direction: per-call masker through internal/wire, envelopes opened in savedentity (architecture)
- **Decision**: add an optional masker to `internal/wire`; the state-encoding helper passes the state masker, `eventData` passes the event masker, every other hop passes none. `savedentity` opens envelopes in the generic record tree, strictly for state and as the marker for display. Key design decisions: one envelope shape for every masker; `Reversible` as a separate interface; one-way state masker rejected in `NewConfig`; unopenable state fails the load; event masking only changes `Event.Data`.
- **Rationale**: matches the spec's "choose the masker per serialisation call" direction and the predecessor's call-site design; touches only the two save sites, the two load sites, `eventData` and `EncodeSavedEntity`.
- **Rejected**: (B) cloning entities and swapping sensitive values for masked ones before plain encoding, which needs deep copies of arbitrary entities with unexported fields (High effort); (C) json/v2 per-call marshal options, already rejected by the predecessor plan.

### `Masker` lives in the `mask` package (architecture)
- **Decision**: the interface, `Reversible`, the envelope type and the built-ins all live in a new public `mask` package.
- **Rationale**: the spec names `mask.EncryptAES256GCM` and friends; keeping the interface beside them gives one import; it is not the root package, so the shared-types knowledge entry is honoured.
- **Rejected**: `types.Masker` (splits one feature over two packages for no cycle benefit).

### Conventions selected (architecture)
- **Decision**: kept testing, real-apply state, shared errors, dependencies, code style, project structure and structured logs. Dropped database/external-services and patterns-and-architecture (no services, handlers or DB).
- **Rationale**: only these bear on the masking code and its tests.
- **Rejected**: listing every convention.

### Examples encrypt state from an environment key (components)
- **Decision**: the application-config and plugin examples call `WithStateMask(mask.EncryptAES256GCM(key))` when `XCL_STATE_KEY` (base64, 32 bytes) is set, and otherwise run without a state masker, so the plaintext warning shows. Their tests set the key. The configuration-only example holds no secret and is left alone.
- **Rationale**: the success metric needs zero secrets in state with an encryption masker across every bundled example; a hard-coded key in example code would teach bad practice; falling back keeps the examples runnable with no setup and demonstrates the warning.
- **Rejected**: a constant demo key; requiring the key (examples would fail out of the box).

### Issue #1 closure is a manual check, not a task (testing_approach)
- **Decision**: closing jumppad-labs/xcl#1 is listed as a manual check in the test plan.
- **Rationale**: reviews and sign-offs are never plan tasks.
- **Rejected**: a `human` task to close the issue.

### No key rotation (testing_approach)
- **Decision**: no key-rotation support; a different key fails the load.
- **Rationale**: the spec does not ask for it; failing loudly is safe.
- **Rejected**: multiple decryption keys.

### Event masker default applied in Config, not the parser (tasks)
- **Decision**: `Config` fills `ParserOptions.EventMask` with `mask.Redact()` unless `WithNoEventMask()`; a nil `EventMask` in the parser means real values.
- **Rationale**: keeps the parser free of defaults, like `EventData`; the public default lives where the options live.
- **Rejected**: defaulting inside the parser (a zero `ParserOptions` would then be unable to express "off").

### Plaintext warning once per operation, as a warn log event (tasks)
- **Decision**: one warn-level log event per Apply or Destroy that writes an unmasked sensitive value to a configured store.
- **Rationale**: destroy saves after every resource; one warning per save would flood the receiver.
- **Rejected**: a warning per save; a new event phase.

## Rehydration cues

- `spektacular spec file read 20261003153421-9fa72edd-masking`
- `spektacular plan file read 20261003134528-327e0657-references-and-secrets plan` and `... context` (sections "Keep real sensitive values on every internal hop" and "Show only the marker in event data").
- Re-read `config.go:283-371`, `options.go`, `internal/parser/events.go:28-83`, `internal/savedentity/savedentity.go`, `encode.go:59-110`.
- `spektacular knowledge read --data '{"tier":"repo","name":"xclconfig","path":"learnings/state-save-and-load-points.md"}'`
- `gh issue view 1` in `/home/nicj/code/github.com/jumppad-labs/xcl`.

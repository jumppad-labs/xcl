---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Context: 20261003153421-9fa72edd-masking

## Current State Analysis

- **Values today**: after the sensitive-values plan lands, `types.Sensitive[T]` shows the marker everywhere it is displayed. `internal/wire` writes real values for state and provider calls. State holds real values in plain text, and event data re-encodes the typed entity with plain `encoding/json`, so it always shows the bare marker `"(sensitive)"`.
- **State save sites**: `config.go:315` (Apply) and `internal/parser/destroy.go:130` (Destroy, after every resource). Both go through the predecessor's shared state-encoding helper.
- **State load sites**: `internal/parser/parser.go:360` (destroy) and `:451` (apply), both through `savedentity.DecodeAll` (`internal/savedentity/savedentity.go:98-150`). `EncodeSavedEntity` (`encode.go:82-109`) reads one record through `savedentity.Decode`, which holds the record as `map[string]any` before typing (`:31-77`).
- **Event data**: decided in one place, `eventData` (`internal/parser/events.go:37-83`).
- **Options**: every `ConfigOption` is in `options.go`. `Config` threads `EventData` into `ParserOptions` at `config.go:236,290,359`. Warnings are warn-level log events built with `logger.New(emit, base).Warn` (`logger/emit.go:31-53`).
- **Examples**: appconfig, configonly and plugin all use `WithStatePath` and `WithEventData(xcl.EventDataProcessed)` (`example/*/main.go`).
- **No masking exists**: there is no `mask` package, no masker option, and no warning for plain-text state.
- **Variables and modules**: neither puts a value into saved records. `cty.Value` serialises as `{}`, and module inputs are `json:"-"`.

## Per-Task Technical Notes

Requirement-to-repo resolution: every requirement except the site half of "Masking is documented" lands in `xclconfig` (root `/home/nicj/code/github.com/jumppad-labs/xcl`). The events guide section and the state masking page land in `xcl-website` (root `/home/nicj/code/github.com/jumppad-labs/xcl-website`). Line numbers are as of commit `d554c1d`. The four predecessor plans change several of these files first, so where a predecessor adds or moves code (`internal/wire`, the state-encoding helper, `types/sensitive.go`, `types/output.go`), use what exists when this plan starts.

### Task: Add the masking package and its built-in maskers

- `mask/mask.go` (new):
  - `Masker`, `Reversible`, `Masked` (`By` as `xcl_masked`, `Value` as `value,omitempty`);
  - `IsMasked(json.RawMessage) (Masked, bool)`, true only for a JSON object whose keys are `xcl_masked` (a non-empty string) and optionally `value`;
  - `Unmask(json.RawMessage, Masker) (json.RawMessage, error)`. It returns `*xclerrors.UnrecoverableError` wrapping `ErrUnrecoverable` when the data is not an envelope, the masker is not `Reversible`, `Masked.By != m.Name()`, or `Unmask` fails.
  - The package comment states the reserved key and that `Mask` receives the JSON of the real value.
- `mask/aes.go` (new): `EncryptAES256GCM(key []byte) (Masker, error)`.
  - It rejects `len(key) != 32` with an error naming the length.
  - `Name()` returns `"aes-256-gcm"`.
  - `Mask` uses `aes.NewCipher` and `cipher.NewGCM`, a fresh 12-byte nonce from `crypto/rand`, and writes JSON string `base64.StdEncoding(nonce‖Seal)`.
  - `Unmask` decodes, splits the nonce and calls `Open`. A failure returns an error.
  - The key is copied at construction.
- `mask/hmac.go` (new): `HashHMACSHA256(key []byte) (Masker, error)`.
  - It rejects an empty key.
  - `Name()` returns `"hmac-sha256"`.
  - `Mask` writes JSON string `hex(hmac.New(sha256.New, key).Sum(value))`.
  - It does not implement `Reversible`.
- `mask/omit.go`, `mask/redact.go` (new):
  - `Omit()` has name `"omit"`, and `Mask` returns nil;
  - `Redact()` has name `"redact"`, and `Mask` returns `json.Marshal(types.SensitiveMarker)`.
- `errors/errors.go` (or the file holding `ErrTypeMismatch`), changed:
  - `ErrUnrecoverable` with `UnrecoverableError{ID, MaskedBy, Reason string; Err error}`, with pointer receivers, `Error()` and `Unwrap()` returning the sentinel;
  - `ErrMaskNotReversible` with `MaskNotReversibleError{Masker string}`, whose message reads `the state masker must be reversible: "<name>" cannot recover the values it masks`.
  - Keep the import list thin.
- `config.go:16-120`: re-export `ErrUnrecoverable` and `ErrMaskNotReversible`, and alias `UnrecoverableError` and `MaskNotReversibleError`, following `config.go:111-124`.
- Tests:
  - `mask/aes_test.go`: differs from input; round-trips; a wrong key fails; two maskings differ; a 16-byte key is rejected.
  - `mask/hmac_test.go`: same input and key give the same output; a different key gives a different output; an empty key is rejected.
  - `mask/omit_test.go` and `mask/redact_test.go`.
  - `mask/unmask_test.go`, with one function each for:
    - reversible success;
    - a one-way envelope;
    - a mismatched masker name;
    - a wrong key;
    - not an envelope;
    - `IsMasked` on a plain object.
  - `mask/custom_test.go`: a test-local custom masker satisfies `Masker`.
  - Each test is its own function using `require`.
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: 2 parallel agents, one on `mask` and its tests and one on the errors and re-exports, then a sequential build.

### Task: Mask sensitive values in the internal encoder

- `internal/wire/wire.go` (from the predecessor), changed:
  - add `Options{Mask mask.Masker}`, `Result{Data []byte; Sensitive bool}` and `Encode(v any, o Options) (Result, error)`;
  - `Marshal` and `MarshalIndent` become `Encode` with zero `Options`;
  - in the single `types.SensitiveValue` branch, set `Sensitive = true`. With `o.Mask != nil`:
    - `plain, _ := json.Marshal(sv.RevealAny())`;
    - `out, err := o.Mask.Mask(plain)`;
    - write `json.Marshal(mask.Masked{By: o.Mask.Name(), Value: out})`.
  - A redacted value (`types.IsRedacted`) still sets `Sensitive`, and with a masker it masks the marker.
  - Thread the options through the recursive walk, so the per-type "may contain sensitive" cache stays valid and delegation to `encoding/json` is used only for types that cannot contain a sensitive value.
  - Update the package comment: provider calls, change detection and conversions pass no masker, the state helper passes the state masker, and event data passes the event masker.
- `internal/wire/wire_test.go`, changed:
  - with a test masker, top-level, nested, embedded, slice, map and `any`-held sensitive values each become an envelope, each in its own test;
  - without a masker, the existing byte-identical tests stay unchanged;
  - `Sensitive` is false for an entity with no sensitive value and true for one with a sensitive value.
- `static_output_test.go`: no change to the allowed importers list, because no new package imports `internal/wire`.
- **Complexity**: Medium
- **Token estimate**: ~25k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Recognise masked values when reading saved data

- `internal/savedentity/savedentity.go:31-77` (`Decode`):
  - add `ReadOptions{Mask mask.Masker; ForDisplay bool}` as a parameter (`Decode(registry, data, read ReadOptions)`);
  - after `json.Unmarshal(data, &record)` (`:32`), call a new `openMasked(record, read, id)` that walks the `map[string]any` and `[]any` tree;
  - for each `map[string]any` whose keys match the envelope, re-marshal it and use `mask.IsMasked`;
  - `ForDisplay` replaces it with `types.SensitiveMarker`;
  - otherwise it calls `mask.Unmask(raw, read.Mask)`, unmarshals the result into `any` and substitutes it;
  - an error becomes `*xclerrors.UnrecoverableError{ID: id, MaskedBy: by, Reason: ...}`. With `read.Mask == nil` the reason is "no state masker is configured".
- `internal/savedentity/savedentity.go:98-150` (`DecodeAll`): add the `ReadOptions` parameter and pass it through. Report an `UnrecoverableError` directly, not folded into `state.UnknownTypesError`, so `errors.Is(err, ErrUnrecoverable)` holds. Keep "never skip a record".
- Callers to update for the new signature:
  - `internal/parser/parser.go:360,451` (done in the state task; here pass `ReadOptions{}` to keep behaviour);
  - `encode.go:104` (the display mode is set in the event task; here `ReadOptions{}`).
- Tests:
  - in `internal/savedentity/savedentity_test.go`, one function each for: an envelope from the state masker opens; a different masker name fails; a one-way envelope fails; a wrong key fails; no masker with an envelope fails; display mode gives the marker for an AES envelope and for an HMAC envelope; plain records are unchanged;
  - in `decode_all_test.go`: an unrecoverable record fails `DecodeAll` with `ErrUnrecoverable`.
  - These are format tests, so hand-built records are allowed (`conventions/test-state-from-real-apply.md` exception).
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Encrypt sensitive values in state

- `options.go:20-46`: add `WithStateMask(m mask.Masker) ConfigOption`. A nil `m` fails. If `m` is not `mask.Reversible`, it returns `&xclerrors.MaskNotReversibleError{Masker: m.Name()}`. Otherwise it sets `c.stateMask`.
- `config.go:131-140`: add `stateMask mask.Masker` to `Config`. Pass `StateMask: c.stateMask` in the three `ParserOptions` literals (`config.go:236`, `:290`, `:359`).
- `internal/parser/parser.go:84-117`: add `StateMask mask.Masker` to `ParserOptions`.
- `internal/parser/parser.go:360`: `savedentity.DecodeAll(p.pluginRegistry, saved, savedentity.ReadOptions{Mask: p.options.StateMask})` (destroy load).
- `internal/parser/parser.go:451`: the same for the apply load.
- The predecessor's shared state-encoding helper in `internal/parser` (used by `config.go:315` and `internal/parser/destroy.go:130`):
  - add a masker argument;
  - use `wire.Encode(entity, wire.Options{Mask: masker})`;
  - return `(records []any, unmaskedSensitive bool, err error)`, where `unmaskedSensitive` is true when any `Result.Sensitive` and the masker is nil.
  - Destroy's `destroyer` gains the masker from `ParserOptions.StateMask`.
- `config.go:315`: call the helper with `c.stateMask`.
- `internal/parser/destroy.go:122-142`: call the helper with the masker.
- Tests:
  - `config_state_mask_test.go` (new, root), with a fixture `internal/test_fixtures/config/state_mask/` holding a registered type with a `types.Sensitive[string]` field set to a known secret. One function each for:
    - `TestStateWithEncryptionMaskerHoldsNoSecret` (real apply, read the state file bytes, `NotContains`);
    - `TestStateWithEncryptionMaskerReloadsRealValue` (second `Config`, same key, `Find`, `Reveal`);
    - `TestStateWithDifferentKeyFailsToLoad`;
    - `TestEncryptedStateWithoutMaskerFailsToLoad`;
    - `TestPlainStateLoadsWithMaskerAndIsEncryptedOnSave`;
    - `TestDestroySavesEncryptedState` (state file after a partial destroy; use a failing destroy so state remains);
    - `TestCustomReversibleStateMaskerOutputInState`;
    - `TestNewConfigRejectsHashStateMasker`, `...OmitStateMasker`, `...RedactStateMasker` and `...CustomOneWayStateMasker`, each asserting `ErrorIs(ErrMaskNotReversible)` and the message text;
    - `TestNewConfigAcceptsEncryptionStateMasker`.
  - `config_options_test.go`: `TestNewConfigWithNilStateMaskFails`.
- **Complexity**: High
- **Token estimate**: ~50k tokens
- **Agent strategy**: Parallel analysis of the save and load paths, then sequential integration: options, then parser load and save, then tests.

### Task: Warn when state holds sensitive values in plain text

- `config.go:283-322` (Apply): after the save helper returns `unmaskedSensitive == true` and `emit != nil`, call `logger.New(emit, events.Event{Source: events.SourceCore, Operation: events.OperationApply}).Warn("sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them")`. Emit only when `c.stateStore != nil`.
- `internal/parser/destroy.go:122-142`: the `destroyer` records `unmaskedSensitive` across its saves. `Parser.Destroy` emits the same warning once, with `Operation: events.OperationDestroy`, through `options.Emit`, after the walk (`internal/parser/parser.go:337-380`).
- Put the message text in one exported-in-package constant shared by both sites (for example `parser.PlaintextStateWarning`), so tests match one string.
- Tests in `config_state_mask_test.go`:
  - `TestPlainStateWithSensitiveValueWarnsOnce` (count the warn events, check the message, and check the state file contains the secret in plain text);
  - `TestPlainStateWithoutSensitiveValuesDoesNotWarn`;
  - `TestNoStateStoreDoesNotWarn`;
  - `TestStateMaskerDoesNotWarn`;
  - `TestDestroyWithPlainSensitiveStateWarnsOnce`.
- **Complexity**: Low
- **Token estimate**: ~20k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Mask event data with the configured event masker

- `options.go:85-103`: add these options:
  - `WithEventMask(m mask.Masker)`: nil fails; it sets `c.eventMask = m`, `c.eventMaskOff = false`;
  - `WithNoEventMask()`: it sets `c.eventMask = nil`, `c.eventMaskOff = true`.
  - Their doc comments say the default is `mask.Redact()`, and that only `Event.Data` is affected.
- `config.go:131-140`: add the fields. In `NewConfig` (`:155-170`), after options, set `c.eventMask = mask.Redact()` when it is nil and `!c.eventMaskOff`. Pass `EventMask: c.eventMask` in all three `ParserOptions`.
- `internal/parser/parser.go:84-117`: add `EventMask mask.Masker`, where nil means real values. Document that `Config` always sets it.
- `internal/parser/events.go:37-83` (`eventData`, as the predecessor leaves it): replace its `encoding/json` re-encode with `wire.Encode(entity, wire.Options{Mask: options.EventMask})`. The pre-call snapshot path unmarshals `pre` into a new instance of `r`'s type and then encodes it the same way. For a nil `r`, it decodes `pre` into `map[string]any`. That does not identify sensitive values, so where `r` is nil keep the predecessor's behaviour, and assert in a test that every lifecycle emission site passes `r`.
- `internal/parser/test_plugin.go` and the parser tests that build `ParserOptions` directly: a zero `EventMask` now means real values in event data, so tests that relied on the predecessor's marker set `EventMask: mask.Redact()`.
- `encode.go:82-109` (`EncodeSavedEntity`): call `savedentity.Decode(registry, data, savedentity.ReadOptions{ForDisplay: true})`. Update the doc comment: masked values show the marker.
- `events/events.go:110-115` (`Data` doc), `events.go:24-38` (`EventDataProcessed` doc) and `options.go:87-96` (`WithEventData` doc): say that sensitive values are masked by the event masker, so processed data equals the state record only where both use the same masking.
- Tests in `config_event_mask_test.go` (new, root), one function each:
  - `TestRawEventDataRedactsSensitiveByDefault`;
  - `TestProcessedEventDataRedactsSensitiveByDefault` (each asserts the `{"xcl_masked":"redact","value":"(sensitive)"}` envelope and `NotContains` the secret);
  - `TestCustomEventMaskerOutputInEventData`;
  - `TestHashEventMaskerIsDeterministicAcrossEvents`;
  - `TestNoEventMaskCarriesRealValues`;
  - `TestLastEventMaskOptionWins`;
  - `TestNewConfigWithNilEventMaskFails`;
  - `TestEncodeSavedEntityShowsMarkerForMaskedEventData`;
  - `TestEncodeSavedEntityShowsMarkerForEncryptedState`.
  - Update the predecessor's event-data sensitive tests from the bare marker to the redact envelope.
  - `config_event_data_test.go:189` (`TestProcessedCreateSuccessMatchesStateRecord`) keeps passing for its entity, which holds no sensitive field.
- **Complexity**: Medium
- **Token estimate**: ~40k tokens
- **Agent strategy**: 2 parallel agents, one on the options and `Config` plumbing and one on `eventData` and `EncodeSavedEntity`, then the tests sequentially.

### Task: Prove errors and every output stay redacted whatever the event masking

- `sensitive_leak_test.go` (from the predecessor), changed:
  - configure state with `mask.EncryptAES256GCM` using a test key;
  - add `TestLeakSuiteStateFileHoldsNoSecret`;
  - update the event-capture assertions to `NotContains(knownSecret)` and `Contains(types.SensitiveMarker)`, which still holds inside the redact envelope.
- `config_event_mask_test.go`, one function each:
  - `TestErrorRedactedWithEventMaskingOff` (`WithNoEventMask()`, a fixture where a function fails on the sensitive argument, as in the predecessor's function-error test; assert the returned error and the error event's `Error` text contain the marker and not the secret);
  - `TestValidationErrorRedactedWithEventMaskingOff`;
  - `TestPluginLogDetailsRedactedWithEventMaskingOff` (in-process test plugin logging the entity as a detail).
- **Complexity**: Low
- **Token estimate**: ~20k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Encrypt the examples' state

- `example/appconfig/main.go:60-90`:
  - `run` gains a `stateKey []byte` parameter. When it is non-nil, it builds `mask.EncryptAES256GCM(stateKey)` and adds `xcl.WithStateMask(m)`.
  - `main` reads `XCL_STATE_KEY` (base64) and decodes it. An invalid value is an error. An unset value passes nil.
  - Comment that the key must come from a secret store and never be committed.
- `example/plugin/main.go:90-112`: the same change.
- `example/prettylog/prettylog.go:84-160`: no change expected. Warn-level log events already print. Check that the warning shows in the pretty output.
- `example/appconfig/main_test.go`:
  - existing `run(...)` calls pass a test key;
  - add `TestAppConfigExampleStateHoldsNoSecret` (read every file in the state dir; `NotContains` the `DB_PASSWORD` value);
  - add `TestAppConfigExampleEventDataHoldsNoSecret`;
  - add `TestAppConfigExampleWithoutKeyWarnsAboutPlainState`;
  - `TestRunWithoutReceiverWritesNothingToStdoutOrStderr` (`:394`) keeps passing.
- `example/plugin/main_test.go`: the same three tests, with secrets `password` and `analytics` (`example/plugin/config/main.xcl:9-11`, `config/modules/db/db.xcl:11`). Update `TestMain` (`:42`) if it builds the run.
- `example/configonly/main_test.go`: add `TestConfigOnlyExampleDoesNotWarnAboutPlainState`.
- `example/README` or the example doc comments, if they list flags or environment: mention `XCL_STATE_KEY`.
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: 2 parallel agents, one on appconfig and configonly, one on plugin.

### Task: Document masking in the library

- `README.md`, inside the predecessor's `## Sensitive values` section (after "Converting to configuration text", `:653`), add:
  - `### Encrypting sensitive values in state`: `xcl.WithStateMask(`, `mask.EncryptAES256GCM(`, the 32-byte key from a secret store, the reversible-only rule and `xcl.ErrMaskNotReversible`, the wrong-key failure `xcl.ErrUnrecoverable`, plain state loading then encrypting, and no key rotation;
  - `### The plaintext state warning`: the message text;
  - `### Masking sensitive values in events`: the default redact envelope with an example, `xcl.WithEventMask(`, `mask.HashHMACSHA256(` for correlation, `xcl.WithNoEventMask()`, and that errors and log details always show the marker;
  - `### Built-in maskers`: a table of the four, with name, reversible and output;
  - `### Writing your own masker`: implementing `mask.Masker` and `mask.Reversible`, `mask.Unmask(` for receivers, and the reserved `xcl_masked` key.
- `README.md:582-652` ("Resource data on events"): say sensitive values in `Data` are masked, and link to the new subsection.
- `docs/state.md:120-143`: replace "it is byte for byte what the state file holds" with an explanation that sensitive values are masked differently in state and events, and that `EncodeSavedEntity` shows the marker for masked values. Add a `## Sensitive values in state` section before `## \`StateStore\``, covering masked state, the envelope shape, the load failures, and that a custom `StateStore` sees envelopes.
- `CHANGELOG.md:1`: add a top entry `## 20261003153421-9fa72edd-masking`. Its prose covers the maskers, the options, the warning and the envelope. Under `**Breaking:**`, listing only changes against the last release (epic decision: the epic ships together, so the sensitive-values spec's bare-marker event data is never released and is not listed):
  - state encrypted with one key cannot be loaded without it;
  - `savedentity` load failures can now be `xcl.ErrUnrecoverable`.
- `readme_test.go`, one theme per test:
  - `TestReadmeDocumentsStateMasking` (`"xcl.WithStateMask("`, `"mask.EncryptAES256GCM("`);
  - `TestReadmeDocumentsThePlaintextStateWarning`;
  - `TestReadmeDocumentsEventMasking` (`"xcl.WithEventMask("`);
  - `TestReadmeDocumentsTurningEventMaskingOff` (`"xcl.WithNoEventMask()"`);
  - `TestReadmeDocumentsTheBuiltInMaskers` (all four names);
  - `TestReadmeDocumentsCustomMaskers` (`"mask.Masker"`);
  - `TestStateGuideDocumentsMaskedState` (reads `docs/state.md`);
  - `TestChangelogRecordsMasking` (the heading);
  - `TestChangelogListsTheEncryptedStateKeyBreakingChange` (the key-required and `xcl.ErrUnrecoverable` items).
- **Complexity**: Medium
- **Token estimate**: ~30k tokens
- **Agent strategy**: Single agent, sequential execution.

### Task: Document masking on the site

- `xcl-website:src/pages/events.mdx`: add `## Sensitive values in event data` after "The event" (`:51-92`). It covers the default redact envelope, `xcl.WithEventMask` with an example using `mask.HashHMACSHA256`, `xcl.WithNoEventMask()`, that errors and log details always show the marker, and opening data with `mask.Unmask`.
- `xcl-website:src/pages/state-masking.mdx` (new): layout and components as in `events.mdx`. It covers:
  - why state holds sensitive values;
  - the plaintext warning;
  - `xcl.WithStateMask` with `mask.EncryptAES256GCM`, and key handling;
  - the reversible-only rule;
  - reloading and the wrong-key error;
  - the built-in masker table;
  - writing your own reversible masker.

  Link to the predecessor's Sensitive values page where it exists.
- `xcl-website:src/components/Nav.astro:19-24`: add `{ label: "State masking", href: "/state-masking/" }` to Guides.
- `xcl-website:README.md`: add the page to the Pages table.
- Gate: `npm ci`, then `npm run build` and `npx astro check` in `xcl-website`.
- **Complexity**: Low
- **Token estimate**: ~20k tokens
- **Agent strategy**: Single agent, sequential execution.

## Testing Strategy

Every test uses testify `require`. There are no table-driven tests, and positive and negative cases go in separate functions. State comes from a real `Apply`, except for format-level tests of the envelope and the saved-entity reader. The overall strategy is in plan.md's Testing Approach. Per task:

- **Add the masking package and its built-in maskers**: unit tests per masker property, per `Unmask` outcome, and for a custom masker satisfying the interface.
- **Mask sensitive values in the internal encoder**: an envelope at each position (top-level, nested, embedded, slice, map, `any`), byte-identical output without a masker, and the sensitive-written report.
- **Recognise masked values when reading saved data**: opening in state mode, each unrecoverable case, display mode giving the marker, and plain records unchanged.
- **Encrypt sensitive values in state**: no secret in the state file after a real apply and after destroy's saves, reload with the same key, failure with a different key or no masker, plain state migrating, a custom reversible masker, and each one-way rejection.
- **Warn when state holds sensitive values in plain text**: exactly one warning with a sensitive value, none without one, none with no store, none with a masker, and one per destroy.
- **Mask event data with the configured event masker**: the redact envelope by default at both levels, a custom masker, a deterministic hash, masking off, last option wins, and `EncodeSavedEntity` showing the marker for masked event data and encrypted state.
- **Prove errors and every output stay redacted whatever the event masking**: function and validation errors and plugin log details with masking off, and the leak suite with encrypted state.
- **Encrypt the examples' state**: no secret in state or event data with a key, the warning without a key, and no warning for the configuration-only example.
- **Document masking in the library**: README, state guide and CHANGELOG content tests.
- **Document masking on the site**: build and `astro check`, plus the manual browser review in the implementation test plan.

## Project References

- Spec: `20261003153421-9fa72edd-masking` (epic `20261003134528-327e0657-references-and-secrets`).
- Upstream plans:
  - `20261003134528-327e0657-references-and-secrets` (sensitive values, `internal/wire`, state-encoding helper, event data re-encoding);
  - `20261003153421-6ec0eab3-module-boundary-and-output-entities`;
  - `20261003153421-bf87d907-references-as-written`;
  - `20261003153421-c283547c-user-depends-on`.
- Design documents: none.
- Knowledge (`xclconfig` store):
  - `learnings/state-save-and-load-points.md`
  - `gotchas/custom-marshaljson-changes-internal-hops.md`
  - `architecture/shared-public-types-live-in-types.md`
  - `conventions/shared-errors-package.md`
  - `conventions/test-state-from-real-apply.md`
  - `conventions/testing-and-mocking.md`
  - `conventions/dependencies.md`
- Issue: jumppad-labs/xcl#1.
- Repo roots: `xclconfig` is at `/home/nicj/code/github.com/jumppad-labs/xcl`, and `xcl-website` is at `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Only "Encrypt sensitive values in state" is High, because it touches the options, both load paths and both save paths. Run the full suite after it, and after the event task.

## Migration Notes

- **Existing state**: plain-text state loads with or without a state masker. Once a masker is added, the next save encrypts it. Encrypted state needs the same masker and key to load. Removing the masker makes it unloadable until the masker is restored.
- **Event consumers**: a sensitive value in `Event.Data` changes from the bare string `"(sensitive)"` to the envelope `{"xcl_masked":"redact","value":"(sensitive)"}`. `EncodeSavedEntity` handles both. Consumers that compared processed event data with state bytes must expect differences for entities holding sensitive values.
- **Custom `StateStore` implementations**: no code change. With a state masker configured, the raw JSON they receive holds envelopes in place of sensitive values.
- **External plugins**: unaffected. Provider calls carry real values, unmasked.

## Performance Considerations

- AES-GCM and HMAC-SHA256 on short values cost microseconds each, and they run only for sensitive values at state saves and event emission.
- Destroy saves after every resource, so with encryption every sensitive value is re-encrypted per save. This is linear in the number of sensitive values, and negligible next to provider calls.
- Opening envelopes walks each loaded record's generic tree once. The record is already decoded into that tree today.
- At `EventDataNone` (the default) nothing is encoded for events, so event masking costs nothing.

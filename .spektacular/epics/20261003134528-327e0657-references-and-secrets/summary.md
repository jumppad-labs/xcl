---
created_date: "2026-10-05"
---

# Planning summary: 20261003134528-327e0657-references-and-secrets

## Decisions
- **Sensitive values in configuration text under `ShowReferences()`** — plans: references-and-secrets, references-as-written. Outcome: a sensitive attribute written as a single bare reference shows that reference (an address does not reveal the secret); any other expression keeps `"(sensitive)"`; `RevealSensitive()` shows everything. references-and-secrets' plan updated to record the exception.
- **Default event masking shape** — plans: references-and-secrets, masking. Outcome: every masker, the default Redact included, writes the envelope `{"xcl_masked":"<name>","value":...}`; Redact's value is `"(sensitive)"`. masking updates references-and-secrets' event-data tests to assert the envelope.
- **CHANGELOG `**Breaking:**` scope** — plans: masking (and all plans in the epic). Outcome: the epic ships together, so each list names only changes against the last release; masking drops its "envelope instead of bare marker" and "no longer byte-identical" items and tests the key-required/`ErrUnrecoverable` items instead.
- **Destroy order source** — plan: user-depends-on (raised at review). Outcome: destroy builds the same graph from `Meta.Links` with the create graph's builder and walks it in reverse; `Meta.Parents` is removed (breaking: `meta.parents` leaves saved state).

## Order added for shared files
None added.

## 20261003134528-327e0657-references-and-secrets
### Approach
`types.Sensitive[T]` hides its value on every standard Go display path (Stringer, GoStringer, Formatter, slog LogValuer, MarshalText, MarshalJSON all write `(sensitive)`); only `Reveal()` returns it. Sensitivity travels through configuration as a cty mark via a generic wrapper registered with the in-repo `gocty`, covering references, interpolation, functions, module inputs and outputs. A new static validation stage rejects sensitive parts reaching a plain field (naming the field), replacing today's panic. A new `internal/wire` encoder writes real values for state, provider calls, change detection, the host state callback and query conversion. Events, configuration text and the printer show the marker unless explicitly asked to reveal.

### Milestones and tasks
- M1 Declare sensitive fields; the real value is kept only where it must be: add the sensitive value type; carry sensitive values through evaluation; keep real values on every internal hop; show only the marker in event data; support sensitive fields on plugin types; prove round-trip through apply and state.
- M2 Sensitivity follows the value and can't reach a plain field: reject sensitive-to-plain in validation; keep sensitivity in output values; refuse converting a sensitive field into a plain Go field.
- M3 Errors, configuration text and printers never show a secret unless asked: redact sensitive arguments in function errors; redact in configuration text; redact in the resource printer; prove no secret leaks from any output path.
- M4 Examples and docs: declare the examples' secrets sensitive; document in the library; document on xcl-website.

### Tasks for a person
None.

### Out of scope
Protection against an attacker with process access; detecting secrets in undeclared fields; preventing leaks after `Reveal()`; redacting the user's source text in diagnostics; pluggable masking, state encryption, the plaintext warning and `WithNoEventMask` (masking spec); sensitive variables; plugin types beyond a fixed set of `Sensitive[T]` instantiations; unrelated stale site content.

### Drafting assumptions
- Direction: cty marks, a self-redacting type, and a revealing internal encoder.
- `MarshalJSON` writes the marker; `UnmarshalJSON` accepts the real `T`.
- State save sites hand stores `json.RawMessage` (breaking).
- Marker `(sensitive)` exported as `types.SensitiveMarker`.
- gocty/gohcl changes are generic hooks recorded in `UPSTREAM.md`.
- Plugin instantiations: `string`, `int`, `int64`, `float64`, `bool`, `[]string`, `map[string]string`; others fail plugin loading naming the field.
- Reading back the bare marker gives a redacted value (`Reveal()` returns zero).
- Options: `xcl.RevealSensitive()`, `logger.WithRevealSensitive(bool)`.
- `errors.TypeMismatchError` gains `Field`; `ErrTypeMismatch` stays the sentinel.
- `types.Output` gains `SensitivePaths` (`sensitive_paths`).
- Examples use `Reveal()` only where genuinely needed.

### Project-wide rules
- CHANGELOG top entry with `**Breaking:**`: `StateStore.Save` receives `json.RawMessage`; event data shows the marker and is no longer byte-identical to state for sensitive entities. Guarded by `readme_test.go`.
- README sections (new `## Sensitive values`, plugin-guide warning) guarded by content tests.
- New public types in `types`: `Sensitive[T]`, `SensitiveValue`, `SensitiveMarker`, `SensitiveMark`, `IsRedacted`; `types` imports `internal/cty/gocty`.
- Every internal serialisation hop uses `internal/wire`; plain `encoding/json` is for display only; a static test restricts importers.
- State save sites (`config.go` after Apply, `internal/parser/destroy.go`) encode before saving; masking hooks in there.
- Event data is re-encoded from the typed entity, never reusing provider-call bytes; masking replaces the fixed redaction.
- Configuration text writes sensitive values as the marker, except a single bare reference under `ShowReferences()` (epic decision).
- `internal/cty`/`internal/xcl` changes keep licence headers and go in `UPSTREAM.md`.
- Shared errors stay in `errors`.
- Tests: testify `require`, no table-driven tests, separate positive/negative functions, real-apply state.
- xcl-website gate: build and `astro check` after `npm ci`, plus browser review.

### Manual checks
- Run the full test suite verbosely and search all output for the known test secret (success metric 1).
- Review the example code for any workaround other than declaring sensitive and calling `Reveal()` (success metric 2).
- Read the new Sensitive values page and refreshed example pages on xcl-website: declaring fields, `Reveal()`, both sensitive-to-plain errors, the marker in JSON and templates; quoted outputs match the examples.
- Read the plugin developer guide's sensitive-fields section: an unwrapped value is no longer protected and must not be logged or emitted.

## 20261003153421-bf87d907-references-as-written
### Approach
Parsing records the exact text written after `=` for every attribute holding a reference (same rule as `Meta.Links`), keyed by attribute path (`location`, `network[1].name`), in a new `types.Meta.References map[string]string` (`references`, omitted when empty). `Meta` already flows through state, `replaceValues`, plugins and events, so no new code is needed on those paths. A new `xcl.ShowReferences()` option, beside `IncludeComputed()` and `RevealSensitive()`, replaces each recorded attribute's tokens in the `hclwrite` body inside the root `encodeEntity`, so `EncodeEntity` and `EncodeSavedEntity` stay byte-identical. The MPL `gohcl` fork is untouched.

### Milestones and tasks
- M1 Configuration text can show references as the user wrote them: record references as written while parsing; show references in configuration text on request.
- M2 Documentation: library docs (README, CHANGELOG, `readme_test.go`, a line in `docs/state.md`); new site guide `src/pages/configuration-text.mdx` linked under Guides (xcl-website).

### Tasks for a person
None.

### Out of scope
Making the text reprocessable; references in state saved by earlier versions (shown resolved until re-applied); `depends_on` in the text (user-depends-on); references for variables, outputs and modules; restoring comments or layout; references in the printer or event display.

### Drafting assumptions
- Nested blocks are numbered by position among blocks of the same type.
- Only attributes the resolved text already writes are replaced.
- Under `ShowReferences()` without `RevealSensitive()`, a sensitive attribute shows its reference only when the expression is a single bare reference; otherwise `"(sensitive)"`. With `RevealSensitive()` every reference is shown.
- Recorded text is the bytes between `attr.EqualsRange.End` and `attr.SrcRange.End`.
- Unparseable recorded text falls back to the resolved value without error.
- Replacements are applied in sorted path order.
- No breaking changes; the CHANGELOG entry says so.
- Open for implementation: if the quoted marker can be missing from written tokens, fall back to checking the Go field for `types.SensitiveValue`; stop and ask only if neither works.

### Project-wide rules
- CHANGELOG top entry headed with the spec name, no `**Breaking:**` list, content test in `readme_test.go`.
- Bare-reference exception to the configuration-text marker rule (epic decision).
- New field on `types.Meta`.
- Configuration-text options are closed functional options in the root package: `IncludeComputed()`, `RevealSensitive()`, `ShowReferences()`.
- `internal/xcl` untouched.
- Saved records gain an optional `meta.references` key written through the wire-encoded save sites.
- References recorded as written, never re-scoped; `Meta.Links` and the boundary check unchanged.
- This plan creates the site's `/configuration-text/` guide; user-depends-on extends it.
- xcl-website gate: build and `astro check` after `npm ci`.
- Tests: testify `require`, no table-driven tests, separate positive/negative functions, real-apply state.

### Manual checks
- Open the new configuration-text guide on xcl-website: reachable from navigation, describes requesting references with an as-written example beside the default resolved form.

## 20261003153421-c283547c-user-depends-on
### Approach
One dependency source, `Meta.Links`, and one graph builder for create and destroy. The body of `buildCreateDAG` moves into a shared `buildDependencyGraph(rp, addresses, nodes, rootName, requireParentModule)`: dependencies come from `getResourceDependencies` over `Meta.Links` (module-relative via `AppendParentModule`, module-wide entries expanded, plus the module a node sits in), resolved against every entity `rp` holds, with edges only between nodes in the set. Create calls it over all entities; destroy calls it over the targets, resolved against the full working state (parents outside the set ignored), and walks it with `dag.Walker{Reverse: true}`. The destroyer gains an `addresses` field set at both construction sites in `parser.go`. Saved state already holds `links`, `module` and the variable, output, module and disabled entities, so destroy needs no configuration. `Meta.Parents` is removed. Then the `DependsOn` mirror is removed: `types.AppendUniqueDependency` becomes `types.AppendUniqueLink` and the copy loop in `buildCreateDAG` is deleted; `DependsOn` holds the strings as written. Written `depends_on` entries stay in `Meta.Links`, so validation, evaluation and the module boundary check are unchanged. `trimBookkeeping` drops `depends_on` only when empty. Sequencing: create switch, destroy switch, Parents removal, then the mirror removal — the reader always moves before a record is removed.

### Milestones and tasks
- M1 Create and destroy order rests on every dependency, from one source: order creation from links; order destruction from links with the create graph's builder (destroy-from-saved-state tests incl. module-wide `depends_on`, module resources before the module, new `module_internal_reference` fixture); stop recording parents in saved state (field, golden fixture, `parents_test.go`/`destroy_test.go`/`removal_test.go`/`registered_types_test.go` rewrites, docs/state.md, overview.md, parser-lifecycle.md, plugin-developer-guide.md).
- M2 A written `depends_on` list is kept and shown exactly as written: keep the written list as written (re-runs destroy-from-saved-state tests with the mirror gone); show it in configuration text.
- M3 Documentation: library docs (README, CHANGELOG, `readme_test.go`); extend the site's configuration-text guide (xcl-website); rewrite `learnings/depends-on-mirrors-links.md` via `spek-knowledge` (destroy reads links, no `Meta.Parents`; the user confirms the entry text when it runs).

### Tasks for a person
None (the knowledge rewrite asks the user to confirm its text).

### Out of scope
Backwards compatibility with old state (no migration from `parents`); which references count as dependencies, module expansion or parent-module edges; interpolation in `depends_on`; showing worked-out dependencies anywhere new; `depends_on` in `Meta.References`; any `internal/xcl` change.

### Drafting assumptions
- Destroy reuses the create graph's builder and walks it in reverse; `Meta.Parents` is removed (user decision). Recorded Parents, resolving against targets only, and a separate destroy builder are rejected alternatives.
- Subset destroy resolves against the full working state and keeps only edges between targets.
- A missing parent module is an error for create but ignored for destroy, so a destroy never refuses to start over incomplete state.
- Old state: the `parents` key is ignored on load and destroy orders from `links`; records with unusable links are destroyed without that ordering. State saved before `parents` existed is now destroyed children first.
- `AppendUniqueDependency` is replaced by `AppendUniqueLink` rather than redefined.
- `DependsOn` holds strings as written, not canonical `fqdn.String()`.
- Open for implementation: if the walk's decode writes `depends_on` in a different form, stop and ask (fix would touch `internal/xcl`); if the sensitive-values plan's wire-encoded state does not keep `meta.links` and `meta.module` as plain strings, stop and ask.

### Project-wide rules
- `Meta.Links` is the single source of every dependency, for create order, destroy order, validation and evaluation; `DependsOn` is what the user wrote; there is no `Meta.Parents`.
- Create and destroy share one graph builder; destroy walks it in reverse.
- Written `depends_on` stays in `Meta.Links` and subject to the module boundary check.
- Public helper rename: `types.AppendUniqueLink` replaces `types.AppendUniqueDependency`.
- Configuration text writes `depends_on` when written, trims it otherwise; not part of `Meta.References`/`ShowReferences()`.
- Module-scoped keys via `FQRN.AppendParentModule`.
- CHANGELOG `**Breaking:**` (against the last release): `DependsOn` narrowed; `AppendUniqueDependency` replaced; `depends_on` now shown in configuration text; `types.Meta.Parents` removed and saved state no longer carries `meta.parents` (destroy orders from `meta.links`; older state still loads with the key ignored). Content tests in `readme_test.go`.
- Extends `/configuration-text/`; xcl-website gate is build plus `astro check` after `npm ci`.
- Tests: testify `require`, no table-driven tests, separate positive/negative functions, real-apply state.

### Manual checks
- Read the site's configuration-text guide: the new section says a written `depends_on` list is kept and shown as written, worked-out dependencies are not added, and the example is correct.

## 20261003153421-6ec0eab3-module-boundary-and-output-entities
### Approach
Two independent halves. A new validation stage-2 check judges every reference in `Meta.Links` as written (interpolations and `depends_on`): it passes only if it has no module path, or names one direct child module and targets an `output`; violations are `ParserError`s naming the reference, reported once (not also as undefined), enforced by both `Validate` and `Apply`. `resolveReference` composes nested keys with `FQRN.AppendParentModule`, fixing re-export of a child's output. `resources.Output` moves to `types.Output` (same fields, state unchanged); the output special cases in `query.go` are removed so `Find`, `FindByType`, `All` and `Decode` return output entities; `Outputs()` stays.

### Milestones and tasks
- M1 Applications read outputs as entities: move the output type into `types`; return outputs as entities from every lookup; read outputs as entities in the plugin example.
- M2 Configurations reach into a module only through its outputs: resolve re-exported outputs of nested modules; reject references that cross a module boundary (incl. three-level nesting test).
- M3 Documentation: library docs (README, docs/modules.md, CHANGELOG, content tests); xcl-website docs; rewrite `architecture/ux-flow.md` via `spek-knowledge` (approved option A wording: `Find[types.Output]` + `.Value`, `Outputs()` note).
- M1 and M2 can run in parallel.

### Tasks for a person
None.

### Out of scope
Jumppad's own `output` entity; restricting Go lookups of module internals by address; outward module references (root-scope fallback unchanged); tightening the evaluation context; a matchable error sentinel; how `depends_on` relates to `Links`; rewording ux-flow.md's validation-stage part.

### Drafting assumptions
- The boundary applies to configuration only; the Go API can still reach module internals by address.
- Judged on the reference as written; `module.a` itself stays valid.
- `Output` moves with no alias; `resources.TypeOutput` stays as the keyword constant.
- No new error sentinel: violations are `ParserError`s collected into `ConfigError`.
- New `output_entities` fixture rather than editing `registered/basic`.
- The re-export Apply test asserts through `Outputs()`, keeping M2 independent of M1.
- The plugin example prints `.Value` with `%q`, so output lines stay identical.

### Project-wide rules
- CHANGELOG top entry headed with the spec name, `**Breaking:**` list (`Find[string]` on an output now fails with `ErrTypeMismatch`; a reference into a module not to an output fails validation), content test in `readme_test.go`.
- README sections guarded by `readme_test.go` content tests.
- `types.Output` is the public output type; `types` may import `internal/cty`.
- User-written `depends_on` entries sit in `Meta.Links` and are boundary-checked.
- Module-scoped keys are always built with `FQRN.AppendParentModule`.
- xcl-website gate: build and type-check after `npm ci`.
- Tests: testify `require`, no table-driven tests, separate positive/negative functions, state from a real apply.

### Manual checks
- Read the xcl-website Modules card and plugin example page in a browser: they describe the output-only boundary with a re-export example and show an output read as an entity.

## 20261003153421-9fa72edd-masking
### Approach
A new public `mask` package holds the `Masker` interface (`Name`, `Mask(json.RawMessage)`), a separate `Reversible` interface (`Unmask`), the envelope `{"xcl_masked":"<name>","value":...}`, `mask.Unmask`/`IsMasked`, and built-ins `EncryptAES256GCM(key)`, `HashHMACSHA256(key)` (each `(Masker, error)`), `Omit()` and `Redact()`. `internal/wire` gains an optional per-call masker and reports whether it wrote a sensitive value: provider calls, change detection and conversions pass none; the shared state-encoding helper passes the state masker; `eventData` passes the event masker. `savedentity` opens envelopes in the generic record tree before typing (state masker on load, failing with `ErrUnrecoverable`; marker for display via `EncodeSavedEntity`). `WithStateMask` rejects non-reversible maskers (`ErrMaskNotReversible`); `WithEventMask` defaults to `mask.Redact()`; `WithNoEventMask()` turns it off. One warn-level event per Apply/Destroy when a store is configured and unmasked sensitive values were written.

### Milestones and tasks
- M1 Developers can mask a value with a built-in or their own masker: add the `mask` package and built-ins (with errors and root re-exports); mask sensitive values in the internal encoder; recognise masked values when reading saved data.
- M2 State encrypts with a key, warns in plain text: encrypt sensitive values in state; warn when state holds sensitive values in plain text.
- M3 Events redact by default; developers choose how or turn it off: mask event data with the configured masker; prove errors and every output stay redacted whatever the event masking.
- M4 Examples and docs: encrypt the examples' state; document masking in the library; document masking on xcl-website.

### Tasks for a person
None.

### Out of scope
Backwards compatibility with old state; whole-file state encryption; key rotation, multiple keys and key management; masking log details and errors (always the marker); opening masked data inside `EncodeSavedEntity`; sensitive variables or undeclared fields; any `internal/cty`/`internal/xcl` change.

### Drafting assumptions
- Every masked value, Redact included, is an envelope; Omit writes no `value`; Redact writes `"(sensitive)"` as its value (epic decision).
- Masked state is opened in `savedentity`'s record tree; no custom typed decoder.
- Unopenable state fails with `ErrUnrecoverable`, never silently redacted.
- Event masking only affects `Event.Data`.
- `Masker` lives in `mask`, not `types`.
- appconfig and plugin examples encrypt state when `XCL_STATE_KEY` (base64, 32 bytes) is set, otherwise show the warning.
- Closing issue #1 is a manual check.
- `Config` applies the default `Redact`; a nil `ParserOptions.EventMask` means real values.
- The plaintext warning is once per operation.

### Project-wide rules
- CHANGELOG top entry `## 20261003153421-9fa72edd-masking`; `**Breaking:**` lists only changes against the last release (epic decision): encrypted state needs its key to load; loads can fail with `ErrUnrecoverable`. Guarded by `readme_test.go`.
- README masking subsections inside `## Sensitive values` plus "Resource data on events", each with its own content test; `docs/state.md` content test.
- `ErrUnrecoverable`/`UnrecoverableError` and `ErrMaskNotReversible`/`MaskNotReversibleError` in `errors`, pointer receivers, re-exported from `xcl`.
- `mask` is a public package outside the root package.
- `internal/wire` stays the only revealing encoder; gains `Options{Mask}` and `Encode` returning `Result{Data, Sensitive}`.
- Save sites hand the store `json.RawMessage` (envelopes with a state masker); `savedentity.Decode`/`DecodeAll` gain `ReadOptions`.
- Event data goes through `wire` with the event masker, replacing the fixed redaction; the sensitive-values event tests change to assert the envelope.
- `xcl_masked` is a reserved key in saved records and event data.
- xcl-website gate: `npm ci`, build, `astro check`, browser review.
- Tests: testify `require`, no table-driven tests, separate positive/negative functions, real-apply state (envelope format tests use hand-built records).

### Manual checks
- Run the full suite and every example with the state key set; search every state file and all captured output for the known secret.
- Walk issue #1's list (events, plugin log details, state, printed output) against merged behaviour and close the issue.
- Browser review of the site's events guide masking section and the new State masking page (reachable from navigation).
- Read the README masking subsections and `docs/state.md`: key handling, no one-way state masker suggested, agreement with the site.

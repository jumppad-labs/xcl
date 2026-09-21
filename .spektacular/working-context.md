# Working Context

**Plan `20260921093100-query-api-v2` is COMPLETE.** All 19 phases implemented, tested, verified and
ticked; test plan and changelog records written; spec reconciled. The implement workflow reached
`reconcile_spec` and is ready to finish.

## Final state

- `go test ./...` fully green on **both** toolchains — 1.27 and `GOTOOLCHAIN=go1.25.0`. Zero failures.
  The three `example/configonly` teardown failures that pre-dated this work are fixed (Phase 3.1).
- `go build`, `go vet` clean on both. The site (`xcl-website`) builds, all four pages render.
- 61 of 62 spec checkboxes satisfied. The one left open is the acceptance criterion
  "No documentation-site page shows the superseded helper", because one of its three clauses —
  every site sample compiles under the pinned minimum Go version — was **descoped by user decision
  during planning**. Its other two clauses are met and verified.

## Scope added during implementation, by user decision

**Milestone 5 did not exist in the original plan.** The user decided mid-run that `Config` is the
only public query surface and `state` is store-and-retrieve only, needing no dependency on the
address type. Three phases were added: take lookups out of storage, narrow `StateStore` to `[]any`
and unexport the container, document the result. This is the only breaking public change.

Two knowledge entries were written (propose-then-confirm):
`architecture/config-is-the-public-query-surface.md` (new) and `gotchas/meta-type-holds-two-axes.md`
(updated — the split landed, and `FQRN` carried the identical conflation).

## Working preferences learned this session

- **The user cares about the public interface; internal form is mine to decide** against normal Go
  practice. Do not bring internal shapes for arbitration — bring public surface changes.
- **Do not stop between phases.** Chain work; report at natural milestones, not every step.
- Architectural detail found during implementation goes in the **plan**, not a new spec, when it is
  an internal detail discovered while planning/building.

## Things deliberately left, with reasons

- **A malformed address is not matchable.** `Find[T](c, "location")` fails with a plain `errors.New`
  from `ParseFQRN`; every other failure on the surface answers `errors.Is`. None of the seven
  sentinels fits: `ErrNotFound` would conflate a caller bug with an ordinary outcome, which the
  design separates deliberately, and `ErrUnknownType` fits a bad segment but not a truncated address.
  Resolving it means an eighth sentinel or widening one of the seven — both change a surface the
  design fixes. **Left for the user.**
- **`UnknownTypesError.Types`** is now a narrow name for what it holds (type names, record ids and
  `entry N` positions). Renaming is a public API change; Phase 5.2 reshaped that package already, so
  it was the moment, but it was not in scope. Worth doing next time that package is opened.
- **`glossary/entity.md`** cites `Config.GetResources()` as evidence the code has not caught up. The
  primary name is `Entities()` now; `state.resources` genuinely still says resource, so the entry's
  substance stands. Softening it needs propose-then-confirm.
- **README's stale tail** (~232 lines documenting `Process`, `ToJSON`, `ParseCallback`, all absent
  from the code) is a named spec non-goal and was not touched.
- **`plugins/example` needs a pre-built binary** a fresh checkout does not produce, so a clean clone
  still fails `go test ./...` there. Tracked separately; it is why the floor CI job builds and vets
  rather than running the suite.
- **`Makefile`'s `install-mockery` pins v2**, two majors behind the `.mockery.yml` the project uses.
  v3.5.5 also cannot parse under Go 1.27 — `go install .../v3@latest` (v3.8.0) works.

## The lesson this plan kept teaching

Three production bugs and three wrong plan instructions all had the same shape: **both axes are
strings, so the compiler cannot help.** The build stayed green while behaviour changed underneath,
every time. The one edit that *was* compiler-enforced — changing `handledWithoutProvider` to take
`*types.Meta` — caught its second call site instantly. Where a refactor cannot be compiler-checked,
change a signature so that part of it is.

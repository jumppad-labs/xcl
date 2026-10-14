# Working context: python-plain-dataclasses

Target repo: xcl-python (`/home/nicj/code/github.com/jumppad-labs/xcl-python`). The Python binding is
a separate repo from xcl; it drives the xcl-core engine as a subprocess.

## Problem

The Go library (PR #13, plain-structs) dropped the need to embed `types.ResourceBase`: a plain Go
struct registers as a block type, with an optional `Meta types.Meta` field and optional
`disabled` / `depends_on` fields mirroring the values every block has. The user asked whether Python
can do the same ("we removed the need for having a ResourceBase inside the go code, is this not
possible in python?").

## Findings (verified by running against a locally built engine)

- Python does not structurally need `xcl.Resource`. `@xcl.entity` only checks the class is a
  dataclass (`xcl/_entity.py:90`); the schema skips `meta`/`depends_on`/`disabled`
  (`BASE_FIELDS`, `xcl/_schema.py:26,98`); hydration fills `meta` only if declared
  (`xcl/_schema.py:446`). A plain dataclass (no Resource) applied, found, listed and destroyed
  fine, with `disabled`/`depends_on` set in configuration.
- `.address` exists only on `Resource`; a plain class reads `obj.meta.id`.
- `@xcl.entity` IS currently required: `register_type(cls)` (`xcl/registry.py:35,120`) calls
  `registration_of(cls)`, which rejects undecorated classes. The decorator (1) stamps
  `__xcl_type__`/`__xcl_subtype__`/`__xcl_schema__` on the class and (2) records it in a
  process-wide `_registry` used by untyped lookups: `entities()`, `find(None, ...)`,
  `event.entity()` (`xcl/_schema.py:433`, `xcl/config.py:177`, `xcl/events.py:64`).
- Bug: `registration_of(None)` (e.g. `cfg.all(None)`) raises
  `AttributeError: 'NoneType' object has no attribute '__name__'` instead of `SchemaError`
  (`xcl/_entity.py:110`).
- Field shapes come from annotations, no markup needed: `str` required attribute;
  default or `| None` optional; dataclass -> nested block; `list[DC]` repeated block;
  `dict[str, DC]` map of named blocks. `xcl.field` only for `name=` (config name differs),
  `block=False` (dataclass as object attribute), `label=True`. Sensitive/Computed are type wrappers.
- `xcl.field(name="image")` on attribute `images` works (verified). User: "it is nice that it is
  optional" — keep the markup-free default.

## Agreed direction (my proposal, user said "this is good" / "yes let's do that")

1. Entity classes are plain dataclasses; `xcl.Resource` becomes optional (convenience, or
   deprecated as Go did with ResourceBase).
2. Names given at registration: `local.register_type(Database, "database", "postgres")`, mirroring
   Go's `registry.Register[Database](local, "database", "postgres")`. `@xcl.entity` stays as
   optional sugar; `register_type(App)` with no names falls back to the decorator's record.
3. Untyped lookups use the Config's own registries rather than the process-wide `_registry`
   (also fixes: two configs can't map the same type to different classes today). Events must
   carry the config's class map since `event.entity()` has no Config in scope.
4. `registration_of` raises a proper `SchemaError` for unregistered / None classes.
5. README and tests show plain dataclasses; `xcl.field` presented as the exception.
6. Add test coverage for plain (undecorated, non-Resource) dataclasses.

## Interview answers (2026-10-14)

- `xcl.Resource` and `@xcl.entity`: **remove both** (breaking), not sugar, not deprecate.
- xcl-website docs: **in scope**.
- xcl parity fixtures: **extend** — Python plain dataclasses vs Go plain structs, schemas match.

Note: `spektacular repo list` errors because the xcl-vscode location is missing on disk
(`../../xcl-vscode/.spektacular` resolves outside jumppad-labs). Registry config read directly:
xcl-python root `/home/nicj/code/github.com/jumppad-labs/xcl-python`, xcl-website `../../xcl-website`.

## Constraints step (2026-10-14)

- Constraint 3 reworded to "no engine or protocol change"; the work is pure Python (engine already
  accepts DeclaredType{type, subtype, schema}).
- Release coupling discussed: package version == xcl tag, exact engine version check
  (`xcl/_client.py:76`); xcl tag dispatches xcl-python release built from xcl-python `main`.
  Gaps (no Python-only release, releases from untagged main, ordering for engine-dependent
  changes) — **deferred to a separate follow-up spec**, user: "we can do this later".
- Ship order: xcl parity fixtures merged -> xcl-python main merged -> tag xcl -> website.
- User wants the simplest release process; lockstep ("when we release xcl ... tag both") is fine
  for now. Concern raised: many languages in future. Recommendation given for the follow-up:
  stay lockstep, turn xcl release.yml's single xcl-python dispatch into a list of binding repos
  when a second language arrives, decouple (protocol-compat check instead of exact version) only
  when a binding needs independent releases. Not part of this spec.

## Follow-up spec to capture after this one: CI and release automation (decisions, 2026-10-14)

- PR checks on xcl run each dependent language's build/test suite against the PR's head SHA;
  merge only when everything is green (required checks). Direction: each binding repo publishes a
  reusable `workflow_call` test workflow taking the xcl ref; xcl's PR workflow calls them.
  Coordinated breaking changes: same-branch-name convention (test against binding branch of the
  same name if it exists) — proposed, not yet confirmed by user.
- Auto-tagging: every PR carries one label: `major` / `minor` / `patch`, or an override label for
  maintenance/docs that does not release. On merge the next semver tag is pushed automatically.
  `major` means 1.0.0 (user confirmed).
- A new tag triggers downstream language releases (existing dispatch, generalised to a list).
- Gotchas to carry: GITHUB_TOKEN-pushed tags don't trigger workflows (App token or call
  release.yml directly); release notes from changelog entries since last tag.
- Fold in deferred release-coupling notes (lockstep kept; user: a change in xcl updating every
  language is fine).

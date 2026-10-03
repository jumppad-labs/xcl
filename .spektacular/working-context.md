# Working context: 20261003081552-e1e07cbe-config-decode

## Problem

The user wants to "clean up the config a little": parse HCL like this

```hcl
server "vault_current" { location = "http://localhost:8200"  type = "current" }
server "vault_arc"     { location = "http://localhost:9100"  type = "arc" }

mount "secrets_classic_v1" {
  path = "secrets/kv1"
  type = "kv1"
  classic { server = server.vault_current  mount_path = "v1/secrets" }
  arc     { server = server.vault_arc      store_id   = env("ARC_SECRET_STORE_ID") }
}
```

directly into an application struct

```go
type Config struct {
    Servers     []*Server
    MountPoints []*Mount
}
```

instead of calling `All[Server]` / `All[Mount]` and assembling it by hand.
User's words: "I was kind of thinking could I do something like
c.ParseInto(&Config{}) or something, I don't really like the ParseInto".

## Decisions (user-confirmed via question dialog)

- Name: `c.Decode(&cfg)` (recommended option chosen); package-level
  `xcl.Decode(c, &cfg)` proposed for symmetry with `Find` (method form only on
  Go 1.27+ for generic methods — but Decode is not generic, so a plain method
  works on every version).
- Exported fields whose type is not a registered type: left untouched.
- Single `*T` field: FindOne semantics — exactly one → set, none → nil, more
  than one → error wrapping ErrNotUnique.
- `[]*T` field (T registered): receives what `All[T]` returns.
- User chose to capture as a spec before implementing.
- Earlier in session: NewConfig now returns (*Config, error); WithStatePath added.

## Facts from code exploration

- No existing aggregate decode API. Closest: `All[T]` (query.go:308) uses
  `pluginRegistry.TypePath(reflect.Type)` then `findByType`.
- Query results are the config's own instances (As returns stored *T as-is),
  in declaration order in practice (files sorted by name, blocks in order) —
  not documented as a guarantee.
- Whole-block references (`server = server.vault_current`) decode into `Server`
  or `*Server` fields as a copy; no test covers the pointer form or a
  registered non-`resource` type referenced from a nested block.
- Nested optional single blocks decode into pointer fields, nil when absent.
- User `type` attribute does not clash with internal `meta.type`.
- `env()` works in registered-type blocks; unset var → "".
- Results are not filtered for disabled entities.
- User's original HCL referenced undeclared `server.vault_arc`; added above.

## Interview answers (user-confirmed)

- Decode before Apply: no error, empty slices / nil pointers (as All does).
- Disabled blocks: included, same as All.
- Docs: README + xcl-website configuration-only example page.
- Started this spec with `spec new --force`, dropping the in-progress
  `20260923095659-references-and-secrets` workflow state (its interview.md
  remains in .spektacular/work/ for later).
- User feedback: Decode is generic — spec text must not be written around
  servers/mounts. Criteria use registered types A, B and unregistered U; the
  server/mount HCL is only the docs' worked example, not a requirement.
- User (mid technical-approach step): "just keep going I will review the final doc"
  — draft remaining sections without per-section confirmation; user reviews
  the assembled spec at verification.
- Spec written to store as 20261003081552-e1e07cbe-config-decode (user: "looks good").
- Split check: not offered — one weak signal only (>7 requirements), requirements
  tightly coupled around one call; moderate threshold needs two weak signals.
- Pending outside the workflow: knowledge update for architecture/ux-flow.md
  staged at .spektacular/tmp/ux-flow.md, awaiting user yes/no.

# Plan workflow (started 2026-10-03)

- Plan workflow started for 20261003081552-e1e07cbe-config-decode (user picked it).
- Spec read. Success metrics to carry into Testing Approach: one call fills a
  struct regardless of #types; new type = new field only; every shipped example
  program that assembles config from several block types uses Decode.
- Discovery done (research.md in work dir). Key learnings: Decode can't call
  generic all[T] at runtime -> refactor findByType core + As into non-generic
  reflect.Type helpers shared by both. Spike showed whole-block refs into T and
  *T nested fields already work (coverage only). Only example/configonly
  qualifies for the "examples use Decode" metric (plugin example = plugin types).
  Website configuration-only.mdx Go snippets are stale vs repo.
- Architecture chosen: shared non-generic cores (reflect.Type) behind
  findByType/all/As + Decode two-phase; new ErrInvalidDecodeTarget; NotUniqueError
  unwrapped; new decode.go; website only configuration-only.mdx.
- Components drafted (decoder, shared type-path/kind-scan/conversion cores,
  new error, fixtures, configonly example, docs).
- Data structures drafted: Decode(target any) error both forms; ErrInvalidDecodeTarget; cores typePath/entitiesOf/asType; fixtures Endpoint/Mount.
- Impl detail drafted. Examples/docs use method form c.Decode (non-generic, Go 1.25 ok).
- Dependencies drafted (no design docs; stdlib only).
- Testing approach drafted: each success metric = behavioural test (incl. AST guard on configonly).
- Milestones: M1 shared core + ref coverage; M2 Decode; M3 docs/example/site.
- Tasks drafted (7 ids, xcl-website task last). configonly example: Service/Ingress become *T fields, no Find left; guard NotZero dropped.
- Open questions: two impl-time STOP-and-ask items (refactor regressions, non-pointer registered entities).
- Out of scope drafted.
- Assembled; staged plan/context/research templates in .spektacular/tmp/.
- Verification passed (removed shell commands from plan).
- plan.md written to store.
- context.md written to store.
- research.md written; work dir removed. Now in walkthrough.
- Walkthrough: user said existing resources can be reused for Decode tests. Applied:
  reuse Database/App/Consumer/Cache + registered/{basic,bare,disabled}; only new =
  2 ordering fixtures (cache first/second) + test-local cacheClient type & cache_client
  fixture for whole-block refs. All 3 docs rewritten. Walkthrough resumes at beat 2.
- Walkthrough signed off by user (nested-struct non-goal confirmed). Advancing to finished.

# Implement workflow (started 2026-10-03)

- read_plan passed: structure complete; drift = line offsets only (printDeployments
  :140 not :141, portable guard NotZero :592 not :594) — targets exist, adapt on the fly.
- Spec coverage complete, nothing descoped. No ## Changelog in plan → first-task run.
- Repo roots: xclconfig = /home/nicj/code/github.com/jumppad-labs/xcl,
  xcl-website = /home/nicj/code/github.com/jumppad-labs/xcl-website.
- Task 1 (shared core) analysed in main context: only query.go changes; find[T] keeps calling As[T].
- Task 1 implemented: asType/convertibleTo/entitiesOf/oneOf/addressableType/typePath in query.go.
  gofmt -l lists many pre-existing internal/* files — not ours, ignore.
- Task 1 test step: no new tests by design (plan); proof = existing suite unedited.
- Task 1 verified green (go test ./..., go1.25 build+vet). Examples are in the root module (no own go.mod).
- Task 1 ticked in plan.
- Task 1 changelog entry written. USER: "run the rest without asking" — loop tasks autonomously,
  stop only on failures/mismatches.
- Task 2: fixture registered/cache_client/main.xcl added (implement); tests next via sub-agent.
- Task 2 done (tests pass, no code fix needed).
- Task 3 implemented: ErrInvalidDecodeTarget + InvalidDecodeTargetError{Type,Nil} in errors/query_errors.go; re-exported in config.go.
- Task 3 done.
- Task 4 (Decode) analysed: reuse setupFindConfig/setupBareTypeConfig/setupCacheClientConfig; decode.go written in main ctx, tests by sub-agent.
- Task 4 implemented: decode.go (Decode fn+method, decode two-phase, isCollectionField/isSingleField).
- Task 4: order tests flaked — parser parsedResources is a map, ranged at parseAndValidate.
  USER chose fix in parser ("ordering is probably useful ... when we do diff"): parsed.order +
  parsed.store in internal/parser/parser.go. Now green 40x. Deviation logged in changelog.
- Task 5 implemented: appConfig + c.Decode in example/configonly/main.go; output byte-identical.
- Task 5 done.
- Task 6 implemented: README Decode section (+configonly snippet, vocab, two-spellings), CHANGELOG entry incl. ordering change.
- Task 6 done.
- Task 7 implemented: website configuration-only.mdx program section, Decode snippet, notice bullet, output order.
- Task 7 done; all tasks complete. Website needs npm ci before make check.
- test-plan written (none required).
- Feature changelog: project + xclconfig + xcl-website records written.
- Spec reconciled: all 26 checkboxes satisfied.

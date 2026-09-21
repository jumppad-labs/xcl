---
created_date: "2026-09-21"
document_status: final
closed_date: "2026-09-21"
---

# Test plan: 20260921093100-query-api-v2

Three of the spec's success metrics contain a clause that no test can assert, because each is a
property of the diff, of prose, or of the repository's future history rather than of the running
code. Everything else in the spec is covered by the automated suite.

Each procedure below is grounded in what was actually built. All commands are run from the
`xclconfig` repo root, `/home/nicj/code/github.com/jumppad-labs/xcl`, unless stated otherwise.

---

## 1. Migrated call sites need no more code than before

**Metric.** *"Every one of the existing lookup call sites — 8 in examples, 7 in tests — reduces to a
single call expression, with none requiring more code after migration than before."*

**Automated half.** That all 15 were migrated is already proven: `TestNoCodeReferencesTheSupersededQuerier`
(root, `query_migration_test.go`) parses every first-party `.go` file in the build and fails on any
`NewQuerier` or `Querier` identifier, and the examples and their tests compile and pass. Neither could
be true with a call site left behind.

**Manual half.** "No more code than before" is a property of the diff.

**How.** For each of the eight example call sites, compare the migrated expression against the
original:

```bash
for f in example/appconfig/main.go example/configonly/main.go example/plugin/main.go; do
  echo "=== $f ==="
  diff <(git show HEAD:"$f" | grep -n 'NewQuerier') <(grep -n 'xcl\.Find\|xcl\.FindByType' "$f")
done
```

**Expected result.** Every migrated site is **one** call expression. A lookup that was
`xcl.NewQuerier[T](c).FindResource(addr)` is now `xcl.Find[T](c, addr)`; one that was
`.FindResourcesByType("v")` is now `xcl.FindByType[T](c, "resource", "v")`. No site gains a line, an
intermediate variable, or a setup call. The seven test call sites lived in `querier_test.go`, which
was deleted; their behaviour was re-covered in `query_test.go`, `query_by_type_test.go` and
`query_all_test.go` — see the coverage audit in the Phase 2.5 changelog entry, which records which of
the seven were already covered, which were genuine gaps, and which was an intended behaviour change.

**Who / when.** Reviewer, at review of this change. It is a one-off check, not a recurring one.

---

## 2. A documentation sample retrieves a published value by address

**Metric.** *"Retrieving a published value requires no knowledge of how it is stored, evidenced by at
least one bundled example and one documentation sample that retrieves one using an address alone."*

**Automated half.** The bundled example is covered. `example/plugin` retrieves two published values by
address and prints them, and `TestPluginExampleRetrievesPublishedValues` /
`TestPluginExamplePrintsPublishedTotal` assert the values and the total. That example runs in the
ordinary test run.

**Manual half.** The documentation sample. Compile-proofing of samples was dropped by decision during
planning, so no tool checks it.

**How.** Read the README's `#### Reading values a configuration publishes` subsection and confirm it
shows retrieval by address alone:

```bash
sed -n '/#### Reading values a configuration publishes/,/^#### /p' README.md
```

**Expected result.** The sample uses `xcl.Find[string](c, "output.web_database")` and
`xcl.Find[string](c, "module.analytics.output.location")` — an address and nothing else, with no
reference to `Output`, `CtyValue`, or how outputs are stored. It also shows `c.Outputs()`.

*Note:* the README's Go snippets were additionally verified by execution during implementation — each
was mirrored into a temporary test and run against a real applied configuration. That is not a
standing guarantee, since nothing re-runs it; treat this check as a read-through.

**Who / when.** Reviewer, at review, and again whenever the README's querying section is edited.

---

## 3. The minimum-version build job runs on every change from merge onward

**Metric.** *"The minimum-supported-version build job passes on every change from merge onward, with
no period in which it is absent or skipped."*

**Automated half.** That the job exists, is wired to every change, and pins the declared minimum is
guarded by four tests in `ci_workflow_test.go`: a job pins `GOTOOLCHAIN` to the minimum `go.mod`
declares; no job pins a Go version below it; the workflow triggers on push and pull request; and
`go.mod` declares no `toolchain` directive that would override the job's pin. Each was mutation-tested
by reintroducing the defect.

**Manual half.** "From merge onward, with no period absent or skipped" is a claim about the
repository's history *after* this lands, which cannot be observed from the working tree.

**How.** After merge, confirm the job actually ran and passed:

```bash
gh run list --workflow=go.yml --branch=main --limit 5
gh run view <run-id> --log | grep -A3 'Confirm the toolchain really is the minimum'
```

Then confirm it has not been skipped since:

```bash
gh run list --workflow=go.yml --branch=main --limit 50 \
  --json databaseId,conclusion,headSha | jq '.[] | select(.conclusion != "success")'
```

**Expected result.** The `Build on minimum supported Go` job appears in every run, concludes
`success`, and its version-gate step prints `go version go1.25.0 linux/amd64`. No run on `main` is
missing the job. Locally every command the job runs already passes under `GOTOOLCHAIN=go1.25.0`.

**Who / when.** Whoever merges, immediately after merge; then at each release as a spot check.

---

## Descoped, not manual

One clause of the spec's site acceptance criterion is **descoped**, not deferred to manual
verification: *"every code sample on the site compiles under the pinned minimum Go version."*
Compile-proofing of documentation-site samples was dropped by user decision during planning, and the
descoping is recorded under `**Descoped requirements**` in `plan.md`. Site samples are written in the
portable function form by convention; nothing verifies it mechanically, and no procedure here claims
otherwise.

The other clauses of that criterion are met and were verified: a case-insensitive search of the site
sources returns no mention of the superseded helper, and the built HTML for all four pages contains
`xcl.Find[PostgreSQL]` and `Find[T]` with zero occurrences of the old name.

## Known-red path, unrelated to this change

`plugins/example/e2e_test.go` needs a binary at `./build/example` that a fresh checkout does not
produce, so `go test ./...` fails on a clean clone for reasons unrelated to this work. It was left
tracked separately by decision, and is why the floor CI job builds and vets rather than running the
suite. Locally the suite is fully green on both toolchains because the binary exists.

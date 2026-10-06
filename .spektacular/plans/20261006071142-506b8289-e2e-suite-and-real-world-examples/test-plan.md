---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Test plan: 20261006071142-506b8289-e2e-suite-and-real-world-examples

These are the success metrics and reviews the plan's Testing Approach marks as manual. Everything else runs in xcl's `go test ./...`: the e2e suite (`e2e/`), and through it each example module's tests.

Run every command from the repository root unless a step says otherwise. Set `GOTOOLCHAIN=local` when the local Go is 1.25.0 or newer.

## 1. A stylistic rewrite of an example leaves the run green (success metric)

- **What**: changing how an example is written, without changing what it does, fails no test.
- **How**:
  1. In `example/configonly/main.go`, change how the program is written without changing its output. For example, replace the single `c.Decode(&cfg)` with separate `xcl.FindByType[resources.Deployment](c, "deployment")`-style lookups that fill the same `appConfig`, or rename `printDeployments` and inline `printRouting`.
  2. Make a similar change in `example/plugin/main.go`: swap `xcl.Find[resources.App](c, "resource.app.web")` for `xcl.FindByType` plus a filter, or move a helper.
  3. Run `go test ./...` from the root, which runs the examples through `e2e.TestConfigOnlyExampleTestsPass` and `e2e.TestPluginExampleTestsPass`.
- **Expected**: every package reports `ok`. No test fails because of how the source is written. (Before this spec, `TestConfigOnlyExampleMakesNoPerTypeLookups` and the `UsesPortableLookupForm` tests would have failed.)
- **Who / when**: the reviewer of this spec's change, before merge. Revert the edits afterwards.

## 2. A regression in xcl is caught by the e2e suite or the library's own tests, never only by an example (success metric)

- **What**: breaking a library behaviour named in `e2e/COVERAGE.md` makes a covering e2e or library test fail, not just an example test.
- **How**: break each behaviour below, run `go test ./e2e/... -count=1`, check the named test fails, then revert.
  1. **Decoding.** In `decode.go`, make `Decode` skip slice fields, so they are left nil. Expect `TestDecodeFillsRepeatedBlocksInOrder` and `TestDecodeFillsNestedBlocks` to fail.
  2. **Masking.** In the `mask` package, make the AES-GCM masker return the plaintext unchanged. Expect `TestStateHoldsNoSecretWithKey` and `TestPluginStateHoldsNoSecretWithKey` to fail.
  3. **Plaintext warning.** Remove the plaintext-state warning event that the config emits when no state mask is set (search for `sensitive values are stored unencrypted in state`). Expect `TestPlainStateWarningWithoutKey` and `TestPluginPlainStateWarningWithoutKey` to fail.
  4. **Plugin log scope.** In the plugin logging path, drop the resource ID from provider log events. Expect `TestInProcessProviderCreateLogsReportedAsCreateEvents` to fail.
  5. **Fixture value.** In `e2e/testdata/kube/deployment.xcl`, change `container_port = 8080` to `8082`. Expect `TestDecodeFillsRepeatedBlocksInOrder` and `TestDecodeResolvesReferenceIntoRepeatedBlockByPosition` to fail. This one was spot-checked during implementation.
- **Expected**: each break fails at least one named e2e test, or a library test in the root package.
- **Who / when**: the reviewer, before merge.

## 3. prettylog builds and tests alone against a published xcl (acceptance check)

- **What**: prettylog is self-contained.
- **How**:
  1. `cp -r example/prettylog /tmp/prettylog-standalone && cd /tmp/prettylog-standalone`
  2. `go mod edit -dropreplace github.com/jumppad-labs/xcl`
  3. `go get github.com/jumppad-labs/xcl@<published version or commit containing this change>`. prettylog needs the public API it uses today, including `xcl.EncodeSavedEntity` and `xcl.IncludeComputed`, so pick a version that has them. Until one is tagged, use the pseudo-version of the merge commit.
  4. `go mod tidy && go build ./... && go vet ./... && go test ./...`
- **Expected**: build, vet and every test pass. No file outside the directory is read: the tests use `testdata/encode/main.xcl` and the local fixtures in `fixtures_test.go`.
- **Who / when**: the reviewer or release manager, once a version containing this change is published.

## 4. Coverage-map review (acceptance check)

- **What**: every library-behaviour test removed from the examples is listed once, beside a covering test that exists.
- **How**:
  1. List the removed tests: `git show 8271816:example/configonly/main_test.go | grep '^func Test'` and the same for `example/plugin/main_test.go`. Also check the working-tree versions as of `ea490ea`, which add tests that 8271816 lacks.
  2. Open `e2e/COVERAGE.md`. Every name should appear in exactly one place: a row of the `configonly` or `plugin` table, the "Stayed with the example or prettylog" list (split tests appear in both a row and this list), or the source-inspection table.
  3. For each `e2e.TestX` in the third column, `grep -n 'func TestX(' e2e/*_test.go` must find it.
  4. Read a sample of the e2e tests against the example tests they replace, and confirm the assertions match.
- **Expected**: there are no missing or duplicated rows, every covering test exists, and the sampled assertions are equivalent.
- **Who / when**: the reviewer, before merge.

## 5. Root module no longer carries example-only dependencies (acceptance check)

- **What**: the dependencies only the examples use have left xcl's module.
- **How**: `go mod tidy -diff` (no output), then `grep -n 'charmbracelet\|muesli/termenv\|kr/pretty' go.mod`.
- **Expected**: the tidy diff is empty and the grep finds nothing. Those dependencies appear only in `example/*/go.mod`.
- **Who / when**: the reviewer, before merge.

## 6. Fresh checkout: the full run passes and e2e imports only public packages plus `internal/testutil` (acceptance check)

- **What**: the e2e suite passes from a fresh checkout, and it uses xcl only through public packages.
- **How**:
  1. `git clone <repo> /tmp/xcl-fresh && cd /tmp/xcl-fresh && git checkout <branch>`
  2. `go test ./... -race`. There is no pre-built plugin binary: `e2e/main_test.go` builds the external plugin fixture itself, and the example runner tests build what they need.
  3. `go list -f '{{join .XTestImports "\n"}}' ./e2e | grep jumppad-labs/xcl/internal`
  4. `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./e2e/fixtures/... | grep '/internal'`
- **Expected**: every package is `ok`. Step 3 prints only `github.com/jumppad-labs/xcl/internal/testutil`, and step 4 prints nothing.
- **Who / when**: the reviewer, before merge.

## 7. CI workflow review (manual review)

- **What**: the minimum-Go job builds and vets every example module on Go 1.25.0.
- **How**: read `.github/workflows/go.yml`, job `build-minimum-go`, step "Build and vet the examples". Then look at the job's log on the PR's CI run.
- **Expected**: the step loops over `example/configonly`, `example/plugin` and `example/prettylog` under `GOTOOLCHAIN=go1.25.0`, and the run is green. During implementation all three examples were built and vetted locally with the cached go1.25.0 toolchain.
- **Who / when**: the reviewer, on the PR.

## 8. README review (manual review)

- **What**: the README describes the examples as standalone modules and says how their tests run.
- **How**: read `README.md` under "Running them" (examples section).
- **Expected**: it says each example is its own module with a `replace`, how to build it outside the repo, how to run its tests, and that `go test ./...` runs them through `e2e/`, linking `e2e/COVERAGE.md`.
- **Who / when**: the reviewer, on the PR.

---
created_date: "2026-10-08"
document_status: final
closed_date: "2026-10-08"
---

# Test plan: config and plugin registries

These are the success metrics and manual reviews the plan marks "Manual — captured in the implementation test plan". Everything else is covered by the automated suites: root, `internal/catalog`, `registry`, `errors`, `events`, `e2e`, `plugins/example` and each example module.

Paths are relative to the xclconfig repo root unless they are prefixed `xcl-website:`.

## Success metric 1: the configuration-only example is minimal

- **What to measure**: from creating the configuration to having its decoded values, the setup in `example/configonly/main.go` has no error checks other than the ones on creating the config and applying or decoding it. It also has no state, key, mask or catalog code.
- **How**:
  1. Open `example/configonly/main.go` and read `loadConfig` and `run`.
  2. Run `grep -nE "NewPluginRegistry|registry\.|WithStatePath|WithStateStore|WithStateMask|mask\.|crypto/rand|MkdirTemp|RegisterType" example/configonly/main.go`.
  3. Run `cd example/configonly && DB_PASSWORD=x go run . ./config`.
- **Expected result**:
  - The grep prints nothing.
  - `loadConfig` is `xcl.NewConfig` with five `xcl.WithType(...)` options (`config_map`, `secret`, `deployment`, `service`, `ingress`) plus the caller's options. Its only `if err != nil` checks follow `NewConfig`, `Apply` and `Decode`.
  - The run prints `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)`.
- **Who / when**: the reviewer of this change, before merging `spek/20261008071608-eb05cae0-config-and-plugin-registries`.

## Success metric 2: every registration failure case still has a test

- **What to measure**: each registration failure that had a test before the change still has one afterwards, as a panic test or a load-time error test.
- **How**: compare the old tests with the new ones.
  - Old tests: `git show 465566f:plugins/registry/plugin_registry_test.go`. List every test that asserted an error from `RegisterType`, `RegisterPlugin` or `RegisterPluginWithPath`, or a clash or form error. They are around lines 146-613, 970, 984, 227 and 1266.
  - New tests: `go test -list 'Panic|LoadFails|Clash|Form' ./internal/catalog .`.
  - Map each old case to a new test:
    - empty name, more than one subtype, empty subtype, non-pointer, nil, nil pointer, pointer to a non-struct, missing ResourceBase, duplicate, builtin names, both forms → `TestRegisterTypePanics*` in `internal/catalog/catalog_test.go` and `TestWithTypePanics*`/`TestNewConfigPanics*` in `config_with_type_test.go`
    - a clash with a loaded plugin → `TestLoadFailsForPluginTypeClashingWithDeclaredType` and `TestLoadFailsWithClashForTypeRegisteredAfterRegistryAdded`
    - a form clash with a plugin → `TestLoadFailsForPluginTypeInTheOtherFormOfADeclaredType`
    - a missing binary → `TestLoadFailsNamingRegistryForMissingBinary` and `TestMissingPluginBinaryFailsFirstApplyNamingRegistry`
    - a nil plugin → `TestLocalRegisterNilPluginPanics` and `TestInProcessPanicsOnNil`
- **Expected result**: every old case maps to at least one new, passing test. The only intentional removals are the message-only tests for `TypeFormError`/`TypeNameClashError` (now in `errors/type_errors_test.go`) and `TestRegisterPluginWithPathAcceptsMissingBinary` (now `registry.TestLocalRegisterMissingPathDoesNotFail`).
- **Who / when**: the reviewer of this change, before merging.

## Manual review 1: documentation shows only the new API

- **What to look at**:
  - xcl-website: `src/pages/index.mdx`, `events.mdx`, `plugin-logging.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx`, `examples/plugins.mdx`, the new `registries.mdx` and `src/components/Nav.astro`.
  - xclconfig: `README.md`, `docs/overview.md`, `docs/plugins.md`, `docs/state.md`, `docs/parser-lifecycle.md`, `docs/README.md` and `e2e/COVERAGE.md`.
- **How**:
  1. In xcl-website, run `npm ci && npm run dev` and open `/`, `/events/`, `/plugin-logging/`, `/configuration-text/`, `/examples/configuration-only/`, `/examples/plugins/` and `/registries/`.
  2. In each repo, run `grep -rn "WithPluginRegistry\|NewPluginRegistry\|RegisterPluginWithPath\|DiscoverPlugins\|CastResourceTo\|plugins/registry" --include='*.md' --include='*.mdx' --include='*.astro' . | grep -v CHANGELOG.md | grep -v node_modules | grep -v .spektacular`.
- **What to look for**:
  - The grep prints nothing.
  - No three-argument `prettylog.Handler` remains.
  - The Registries guide has sections on declaring types, configuration only without state, local registries, load order, how problems are reported, clashes and writing your own registry.
  - "Registries" appears under Guides in the nav.
  - The guide says any failing plugin, including a discovered one, fails the load.
  - Check that the illustrative `Remote` registry in "Writing your own registry" is clearly marked as an example and does not suggest xcl ships a remote registry, which is out of scope.
  - The README Custom Functions section (`p.RegisterType`, around line 1824) was out of date before this change and may be raised separately.
- **Pass**: no old API is shown anywhere, both new sections are reachable from the nav, and `npm run build` plus `npx astro check` report 0 errors.
- **Who / when**: the docs owner, before the website is deployed.

## Manual review 2: copied snippets match the example sources

- **What to look at**: the Go code blocks titled `example/...` or `registry/...` in `xcl-website:src/pages/**/*.mdx`.
- **How**: for each block, find its text in the matching file in xclconfig (`example/configonly/main.go`, `example/plugin/main.go`, `example/prettylog/prettylog.go`, `registry/registry.go`). The throwaway checker written during implementation did this: 37 blocks, 0 mismatches.
- **Pass**: every block appears verbatim in its source. The only exception is the `prettylog` `Handler` block in `events.mdx`, which is deliberately shortened with `// ...`; every line it keeps appears in the source.
- **Who / when**: the reviewer of this change, before merging both repos.

## Manual review 3: no committed build outputs

- **How**:
  1. Run `git ls-files configonly`.
  2. For each of `example/configonly`, `example/plugin` and `example/prettylog`, run `(cd <dir> && go build .)`, then `git status --porcelain --untracked-files=all`.
  3. Delete the built binaries afterwards.
- **Pass**: step 1 prints nothing. Building adds no new entry to `git status`; `example/configonly/configonly` and `example/plugin/plugin` are ignored.
- **Who / when**: the reviewer of this change, before merging.

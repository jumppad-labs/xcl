---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Test plan: 20261011095738-fad05e41-language-surface

Every success metric that could be covered by a behavioural test is covered in the test suites (xcl root package, `internal/parser`, `internal/xcl/gohcl`, `internal/xcl/hclsyntax`, `internal/cty/gocty`, `internal/catalog`, `plugins`, the plugin template's `e2e`, and the extension's tokenize test). What follows are the manual metrics and reviews the plan lists, with procedures against the real implementation, xcl v0.3.0 (commit `6cdf936`, tag `v0.3.0`).

## Manual success metrics

### 1. Every claim on xcl.dev and in the README about the environment prefix, functions and templating matches what the latest release does

**What to measure**: each claim on the site and in the README is true of xcl v0.3.0 when tried.

**How**:
1. Open the corrected pages in the website worktree, or on xcl.dev once deployed: `src/pages/index.mdx` (the "Variables and outputs" and "Functions" feature cards), `src/pages/examples/configuration-only.mdx` and `src/pages/examples/plugins.mdx` (the configuration comments), `src/pages/plugin-template.mdx` (the `Functions` sentence), and the xcl `README.md`.
2. Try each claim against the release:
   - Environment prefix: in the xcl repository at v0.3.0 run `XCL_VAR_cpu_resources=1000 go test ./internal/parser/ -run TestLoadsVariablesInEnvVarOverridingVariableDefaults -count=1`, and `HCL_VAR_region=us go test ./ -run TestApplyIgnoresHCLVarPrefixedEnvironmentVariable -count=1`; both pass, showing `XCL_VAR_` applies and `HCL_VAR_` does not.
   - Function count: `go test ./internal/functions/ -run TestBuiltinNamesIsSortedAndContainsUpper -count=1 -v` passes with 57 names, matching "57 built in".
   - Application and plugin functions: `go test ./ -run 'TestApplyCallsApplicationFunction$|TestApplyCallsExternalPluginFunction' -count=1 -v` passes, matching "register a Go function with the application, or offer one from a plugin".
   - Templating: `go test ./ -run TestValidateRejectsTemplateDirectiveAtItsPosition -count=1` passes, so no page may describe `%{ }` directives as supported (none does).
3. Search the site and README for stale text: `grep -rn "HCL_VAR_\|round sixty\|%{" src/pages` in the website and `grep -n "HCL_VAR_\|sixty" README.md` in xcl both print nothing.

**Expected result**: every claim is borne out by the commands above and both searches print nothing.

**Who / when**: the person reviewing the website change, before the site is deployed, and again against xcl.dev after deployment.

### 2. A plugin author can add a working function to a plugin built from the template by following the template alone

**What to measure**: starting from a fresh copy of `xcl-plugin-template` at the commit that moves it to xcl v0.3.0, a reader of its README adds a new function to the note provider and sees it work, without consulting the xcl source.

**How**:
1. Clone the template at that commit into a scratch directory and run `make build && make test`; both pass.
2. Follow the README's "Add a function" section only: add a func such as `noteTitle(name string) (string, error)` with the unexported helpers in `providers/note.go`, add it to the `plugins.NewFunctions` map in `NewNoteProvider` under a new name (for example `note_title`), add a unit test through `Functions().Get("note_title")` as `providers/note_test.go` does, and use `note_title(...)` for an attribute in `examples/basic/main.xcl` (the `content` attribute keeps the e2e assertions intact if the result equals `Hello from xcl`; otherwise adjust the assertion as the README's step 5 says).
3. Run `make test`, then `make inprocess` and `make external`.

**Expected result**: all three pass, the applied note holds the function's result in process and as a separate program, and the reader needed nothing beyond the README.

**Who / when**: a developer who has not worked on the template, before the template's next release.

## Manual reviews

### A. Website and README text review

**What to look at**: in the website worktree, `src/pages/index.mdx` lines 148–176 (the Variables and outputs card and the Functions card), `src/pages/examples/configuration-only.mdx` lines 160–161, `src/pages/examples/plugins.mdx` lines 74–75, `src/pages/plugin-template.mdx` lines 66–70; in xcl, `README.md` lines 15–17 and `docs/plugin-developer-guide.md` "Offering a function", `docs/plugins.md` "Application functions".

**What to look for**: the environment prefix shown is `XCL_VAR_` everywhere; the function count is 57 and the card says applications and plugins can add functions; `Functions` is described as offering functions to configuration; nothing reads as describing templating as supported; the prose reads well in place.

**Passes when**: the reviewer finds no stale claim and no wording to change, and `npm run build` in the website worktree reports 13 pages built.

### B. Extension highlighting in VS Code

**What to look at**: install the extension from the `xcl-vscode` worktree (`npm run package`, then install the `.vsix`, or open the worktree with the extension development host) and open `test/fixtures/sample.xcl`.

**What to look for**: on the `directive`, `loop` and `block` lines, the `%{ if true }`, `%{ endif }`, `%{ for x in list }` and `%{ endfor }` text is coloured as plain string text, the same colour as the surrounding string; on the `loop` line and the `msg` line, `${x}` and `${variable.cpu_resources}` are coloured as interpolation (the `${` and `}` punctuation and the expression inside differ from the string colour); inside the `source` heredoc, `#{{ .Vars.data_dir }}` keeps its template-interpolation colouring.

**Passes when**: only `${ }` (and the heredoc `#{{ }}`) is coloured as template syntax; `%{ }` is not. The automated check is `npm test` in the worktree, which prints `ok — 206 tokens checked`.

### C. CHANGELOG Breaking list review

**What to look at**: xcl `CHANGELOG.md`, the entry `## 20261011095738-fad05e41-language-surface` at the top, its **Breaking:** list, against the changed public surface: `registry/registry.go` (`Registry.Functions`, `registry.Function`), `plugins/functions.go` (`ProviderFunction`, `FunctionSignature`, `NewFunction`, `NewFunctions`), `plugins/plugin.go` and `plugins/plugin_host.go` (`GetFunctions`, `CallFunction`), `internal/parser/parser.go` (`ParserOptions` without `CustomFunctions`, `DefaultVariableEnvPrefix`), `internal/xcl/hclsyntax/parser_template.go` (directive rejection), `internal/xcl/gohcl/schema.go` (`blockLabels`, `ValidateSchemaType`), and `plugins/plugin.proto` (the two added RPCs and four messages).

**What to look for**: each of those changes has a bullet that says what changed and what a user does about it; the entry says the release is v0.3.0; no bullet describes something that did not ship; the "list block accepts and drops a name when the element has no label field" bullet is present, since it changes an error into acceptance.

**Passes when**: the reviewer can match every bullet to a change in the surface above and finds no change in that surface without a bullet.

## Notes from implementation

- The plan expected a `Config.Diff` test to show a change to one named block; `Config.Diff` leaves provider-less registered entities out, so that criterion is covered by `resourceChanges` over two real applies in `internal/parser` (`TestResourceChangesOfOneChangedNamedBlockNamesOnlyThatBlock`) and a root second-apply test. No manual check is needed.
- No binary built against the pre-functions protocol exists in the tree; the old-plugin case is proven with an `Unimplemented` loopback client in `plugins/grpc_server_test.go` and end to end with a plugin offering no functions. A reviewer with an older `xcl-plugin-docker` v0.1.1 release binary can confirm it loads against v0.3.0 by registering it with `RegisterExternalPlugin` and applying `example/plugin/config/full`; it should apply, and `GetFunctions` is read as none.

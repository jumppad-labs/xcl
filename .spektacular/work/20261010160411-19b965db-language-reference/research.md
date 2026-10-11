## Alternatives considered and rejected

- **One long reference page** (`/language/`) holding every topic. Rejected: the site's existing guides run 150-600 lines each (`xcl-website:src/pages/*.mdx`); a single page covering files, Go mapping, meta, references, variables, outputs, modules, functions (56 entries) and expressions would pass 1500 lines, and the second success metric (answer a question with a link to a section) works as well with a handful of pages, each with anchored headings.
- **Put the reference pages under the existing "Guides" nav dropdown.** Rejected: Guides already has 10 entries (`xcl-website:src/components/Nav.astro:17-29`) and the reference is a set of pages, not one guide; it would bury the reference and mix it with Go-API guides. The sibling spec's "Using XCL with Go" guide is the natural fit for Guides.
- **A separate "Go mapping" page.** Rejected by the spec's Technical Approach: the config-to-Go mapping sits next to each language feature.
- **Copying the removed README sections.** Rejected by the spec and by the code: they claim `HCL_VAR_` env overrides, `local` blocks, git module sources, `p.RegisterFunction` and `.hcl` module files, none of which work in v0.2.1 (see Open assumptions / gaps below). `xcl:git show bdba483^:README.md` lines 1317-2060.
- **Automated tests for the examples.** Rejected by the spec's Non-Goals and by knowledge `conventions/testing-and-mocking.md` (no tests that read repository/website files).

## Chosen approach — evidence

- Site is Astro 5 MDX; each page is `src/pages/<slug>.mdx` with `layout: ../layouts/Shell.astro`, a `Hero` and a `Prose` body (`xcl-website:src/pages/sensitive-values.mdx:1-20`, `src/components/sections/Prose.astro`). Nested folders give nested URLs (`src/pages/examples/*.mdx` → `/examples/<slug>/`).
- Main nav is the `items` array in `xcl-website:src/components/Nav.astro:8-30`; a dropdown entry is `{label, children:[{label, href}]}` and its top-level link opens the first child. Active matching is by pathname prefix (`Nav.astro:33-38`), so a `/language/` prefix highlights a "Language" entry for every page under it.
- Home page links: hero buttons and "What is XCL?" section (`xcl-website:src/pages/index.mdx:16-30`), feature cards (`index.mdx:~125-215`) with `[Title →](/path/)` links.
- xcl README has a "Documentation" list of xcl.dev links (`xcl:README.md:166-177`).
- xcl-website README has a Pages table to keep current (`xcl-website:README.md` "## Pages").
- No redirects exist (knowledge `xcl-website:gotchas/no-redirects-on-page-removal.md`); new pages only, nothing removed.
- Changelog: one `## <spec name>` entry per spec in `xcl:CHANGELOG.md` with a `**Breaking:**` list (e.g. `CHANGELOG.md:3-11`, prior docs plan `20261009092551-82db0140-plugin-template` task "Changelog (changed, xcl)").
- Build checks: `npm run build` and `npx astro check` (`xcl-website:Makefile`).

### Language facts (v0.2.1; HEAD differs only by the race fix in `internal/parser/lifecycle.go` `decodeResult`)

- Files: only `.xcl` (case-insensitive), directory or file paths, not recursive (`xcl:internal/parser/util.go:40-73`). `*.vars` files (`name = value`) in each root path's directory are auto-loaded, not inside modules (`util.go:79-123`, `parse_test.go:450`).
- Labels: a keyword registered with a subtype takes exactly `subtype`, `name`; without, exactly `name` (`internal/parser/parser.go:959-991`); mixing is a `TypeFormError` (`internal/catalog/catalog.go:354-360`). Built-in keywords `variable`, `output`, `module`; legacy `resource "<subtype>" "<name>"` still accepted (`catalog.go:226-229`). Reserved names `resource`, `module`, `output`, `variable`; names `[0-9a-zA-Z_-]`, not all digits (`util.go:126-140`).
- Addresses: `type.subtype.name`, `type.name`, `variable.x`, `output.x`, `module.m`, nested `module.a.b`, inside module `module.m.<address>` (`internal/resources/fqrn.go:266-294`).
- Registration: `registry.Local.RegisterType(&T{}, type[, subtype])` (`registry/local.go:90-103`, `example/configonly/main.go:96-100`); plugins `plugins.RegisterResourceProvider(..., typeName, subTypeName, &T{}, provider)` (`plugins/plugin.go:71-90`).
- Struct tags: `xcl:"<name>[,attr|optional|block|label|remain|body][,computed][,key]"` (`internal/xcl/tags/tags.go`, `internal/xcl/gohcl/doc.go`). Required unless `optional`, pointer or `hcl.Expression` (`gohcl/schema.go:52-64`). Blocks: struct required, pointer optional, slice repeated (`gohcl/decode.go:136-233`, `example/configonly/resources/resources.go:54,74`). `block` on a map panics: map of blocks unsupported (`gohcl/schema.go:82-91`). Struct-typed attribute = sub-object, settable from a whole entity (`internal/test_fixtures/plugin/structs/container.go:19`, `test_fixtures/config/interpolation/interpolation.xcl:10`). Map attributes `map[string]string` (`resources.go:28`) and `map[string]Struct` (`container.go:41`, `test_fixtures/config/valid_property_paths/main.xcl:23-29`). `computed` must be `optional`; setting it in config is a validation error (`internal/parser/validate.go:193-203, 249-397`); examples `example/plugin/plugins/docker/entities/container.go:31,34`. `key` pairs block-list elements (`internal/parser/computed.go:18-21`). `json` tags needed for state, plugin wire and provider results (`internal/savedentity/savedentity.go:100-105`, `internal/wire/wire.go:27-30`). `default:"..."` tag applied before decode (`internal/parser/callbacks.go:129`).
- Meta: `ResourceBase` gives `depends_on`, `disabled`, `meta` (`types/resource.go:86-91`); configuration can read `meta.id`, `meta.name`, `meta.type`, `meta.subtype`, `meta.module`, `meta.file`, `meta.line`, `meta.column`; `Properties`, `Links`, `References`, `Status` have no `xcl` tag (knowledge `gotchas/xcl-tags-gate-what-reaches-cty.md`). Examples `example/configonly/config/ingress.xcl:9`, `deployment.xcl:62`.
- References: attribute, nested block index `container[0].port[0].container_port`, whole entity into struct/map attribute; collected into `Meta.Links` and graph edges (`parser.go:1250-1260`, `internal/parser/dag.go:62-129`); dependencies processed first, independent entities concurrently (`internal/dag/walk.go:293,300`), destroy reversed; cycles rejected (`parser.go:1355-1398`, `internal/dag/dag.go:111-139`). Computed values readable by dependents (`lifecycle_test.go:962`, `example/plugin/config/full/main.xcl:69`). `depends_on` literal address strings (`parser.go:1268-1297`); `disabled` an expression.
- Variables: `default` required, `description` optional (`internal/resources/variable.go:13-14`); from Go `xcl.WithVariables(map[string]any)` (`options.go:125-131`) stringified then parsed as int, bool, else string (`config.go:518-524`, `util.go:186`); precedence default < .vars < env < WithVariables (`internal/parser/context.go:200-215`).
- Outputs: `value`, `description` optional (`types/output.go:22-33`); Go reads with `Config.Outputs()` or `Find[types.Output]` (`config.go:303-327`, `query.go:29`); root `output.x` not referenceable (knowledge `gotchas/root-outputs-are-not-referenceable.md`).
- Modules: `source` (required, literal, local path relative to declaring file), `version` (ignored), `variables` object, `depends_on`, `disabled` (`internal/resources/module.go:16-25`, `parser.go:1162-1240`); remote sources fail (`internal/parser/validate_test.go:344-370`); outputs `module.m.output.o`; boundary rejects anything else (`internal/parser/references.go:82-108`, `validate.go:101-158`); nested re-export (`config_module_boundary_test.go:15`).
- Functions: 56 in `GetDefaultFunctions` (`internal/functions/functions.go:429-487`): `dir element env file home len template_file trim` plus go-cty stdlib `abs ceil chomp chunklist coalescelist compact concat contains csvdecode distinct flatten floor format formatdate formatlist indent join jsondecode jsonencode keys log lower max merge min parseint pow range regex regexall reverse setintersection setproduct setsubtract setunion signum slice sort split strrev substr timeadd title trimprefix trimspace trimsuffix upper values zipmap` (the implementer recounts against the table).
- Expressions: full HCL-syntax fork (`internal/xcl/hclsyntax`); dependency tracking walks templates, function calls, traversals, object/tuple constructors, conditionals, binary/unary ops, splats (`internal/parser/exp.go:28-148`) but not `for` expressions, parentheses, dynamic index or `%{for}` (`TemplateJoinExpr`).

## Files examined

- `xcl-website:src/components/Nav.astro:8-38` — nav items and active matching.
- `xcl-website:src/pages/index.mdx` — hero, "What is XCL?", feature cards (Variables card claims `HCL_VAR_<name>`; Functions card "around sixty").
- `xcl-website:src/pages/sensitive-values.mdx:1-20` — page skeleton to copy.
- `xcl-website:src/components/sections/Prose.astro` — prose wrapper.
- `xcl-website:README.md` — Pages table.
- `xcl-website:astro.config.mjs`, `Makefile`, `package.json` — build and check commands.
- `xcl:README.md:166-177` — Documentation links list.
- `xcl:CHANGELOG.md:3-11` — changelog entry shape.
- `xcl:docs/modules.md` — local-only module sources confirmed.
- `xcl:config.go:344-352,378-386` — `ParserOptions` built without `VariableEnvPrefix` or `CustomFunctions`.
- `xcl:internal/parser/util.go:165-183` — `setVariables` with empty prefix maps every environment variable to `variable.<NAME>`.
- `xcl:options.go:22-178` — no option to register a function.
- `xcl:internal/resources/default.go:6-12` — built-in types: variable, output, module, root; no local.
- `xcl:example/configonly/resources/resources.go` — nested, repeated blocks, map attribute, sensitive map.
- Remaining citations listed under Chosen approach.

## External references

- HCL native syntax spec (github.com/hashicorp/hcl hclsyntax/spec.md) — the fork's expression grammar; used only to know which expression forms exist, never cited on the pages.
- go-cty function/stdlib docs (pkg.go.dev/github.com/zclconf/go-cty/cty/function/stdlib) — signatures for the stdlib functions in the function table.
- Handlebars (handlebarsjs.com) via raymond — `template_file` template syntax and the `quote`/`trim` helpers.

## Prior plans / specs consulted

- Plan `20261009092551-82db0140-plugin-template` — how a docs change lands across `xcl` and `xcl-website`: one site page plus nav entry, README links, one CHANGELOG entry, manual checks for build and review in the test plan.
- Spec `20261011084948-c995ed25-using-xcl-with-go` (sibling, planned in parallel) — owns a Go getting-started guide (config, registering types, parse, apply, destroy, state, Find); it also edits `Nav.astro`, `index.mdx`, `xcl:README.md` and `xcl-website:README.md`.
- Old README `xcl:git show bdba483^:README.md` lines 1317-2060 — raw material for struct tags, references, variables, locals, modules, functions; partly stale.

## Open assumptions

- Gaps between the spec's acceptance criteria and v0.2.1 (raised with the user before architecture): no `local` blocks; environment override has no prefix (every environment variable is offered as `variable.<NAME>`, so `HCL_VAR_x` does not work); no public API to register a function; no map of nested blocks; configuration cannot reference an entity inside a module (only its outputs; Go `Find` can by full address).
- Behaviours inferred from code but untested: root `output.x` fails at evaluation; references inside `for`, parentheses, `%{for}` and dynamic indexes are not tracked; absolute module sources are joined to the declaring file's directory. The implementer confirms each by running it against the release before writing it.
- The release the pages are checked against is the latest xcl tag at publication (v0.2.1 today; the home page badge already says v0.2.2). If a new tag changes any fact above, the pages follow the tag.

## Rehydration cues

- `spektacular spec file read 20261010160411-19b965db-language-reference`
- `spektacular repo list` (xcl and xcl-website roots)
- `spektacular knowledge always-applied --tier repo --filter xcl --filter xcl-website`
- `spektacular knowledge read` for `gotchas/root-outputs-are-not-referenceable.md`, `gotchas/xcl-tags-gate-what-reaches-cty.md`, `xcl-website gotchas/no-redirects-on-page-removal.md`
- Re-read `xcl-website:src/components/Nav.astro`, `xcl-website:src/pages/index.mdx`, `xcl:config.go` (ParserOptions), `xcl:internal/parser/util.go`, `xcl:internal/functions/functions.go:429-487`, `xcl:internal/xcl/gohcl/schema.go`.
- `git -C xcl show bdba483^:README.md`

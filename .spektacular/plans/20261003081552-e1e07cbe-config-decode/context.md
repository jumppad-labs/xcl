---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
---

# Context: 20261003081552-e1e07cbe-config-decode

## Current State Analysis

- **There is no aggregate decode API.** Applications call one typed lookup per block type: `All[T]` (`query.go:308-327`), `FindByType[T]` (`query.go:165`), `FindOne[T]` (`query.go:175`) and `Find[T]` (`query.go:29`). Each has a Go 1.27 method form in `query_methods_go127.go`, behind `//go:build go1.27`.
- **Every rule lives in generic code.** These are:
  - `addressable[T]` (`query.go:285`)
  - the scan in `findByType[T]` (`query.go:179-217`)
  - the identity-or-copy conversion in `As[T]` (`query.go:68-95`)
  - the type → path derivation in `all[T]` (`query.go:312-327`), via `PluginRegistry.TypePath(reflect.Type)` (`plugins/registry/plugin_registry.go:737`)

  Generic code cannot be instantiated from a runtime `reflect.Type`.
- **Instances, not copies.** Registered entities are stored as `*T` and returned as is (`query.go:73-75`). `require.Same` pins this for `All` vs `FindByType` (`query_all_test.go:23-38`).
- **Ordering.** `Apply` adopts the parser's state in append order (`config.go:304`, `internal/parser/entities.go:33-44`). This is declaration order, not alphabetical, and was confirmed by a spike.
- **Before Apply.** `NewConfig` always installs a registry and an empty `entities` (`config.go:149-168`), so lookups on an unapplied config return empty results.
- **Whole-block references.** A spike confirmed that a nested block referencing a registered bare-keyword type (`cache = cache.x`, confirmed with throwaway types) decodes into both `*T` and `T` fields with values filled in. No committed test covers this.
- **Errors.** Lookup sentinels and detail types are in `errors/query_errors.go` and re-exported in `config.go:24-104`.
- **Examples.**
  - `example/configonly/main.go` registers four resource types and uses `FindByType` (`:141`) and `Find` (`:194`, `:201`).
  - Its guard `TestConfigOnlyExampleUsesPortableLookupForm` (`example/configonly/main_test.go:549-595`) requires at least one `xcl.`-qualified lookup.
- **Documentation.**
  - README "Querying a configuration" is at `README.md:309-416`, and "Configuration only" at `README.md:68-119`.
  - `readme_test.go` guards the README and CHANGELOG content.
  - The website page `xcl-website:src/pages/examples/configuration-only.mdx` has stale Go snippets at `:224-256`.

## Per-Task Technical Notes

### Requirement → repo and files

| Spec requirement | Repo | Files |
|---|---|---|
| Fill a structure in one call | xclconfig | `decode.go` (new), `decode_test.go` (new) |
| Collection field receives every block of its type | xclconfig | `decode.go`, `query.go` (shared scan) |
| Same blocks a typed lookup returns | xclconfig | `query.go` (shared cores), `decode.go` |
| The configuration's own blocks, not copies | xclconfig | `query.go` (`asType` identity branch) |
| Single-block field filled when exactly one exists | xclconfig | `decode.go` |
| More than one → not-unique error | xclconfig | `decode.go`, reuses `errors/query_errors.go:150-159` |
| Fields matched by type, not name | xclconfig | `decode.go` |
| Other fields left alone | xclconfig | `decode.go` |
| Unusable target is an error | xclconfig | `errors/query_errors.go`, `config.go`, `decode.go` |
| Nothing processed → empty, not error | xclconfig | `decode.go` (relies on `config.go:150-164`) |
| Referenced blocks arrive filled in | xclconfig | `query_references_test.go` (new, test-local types), one new fixture file |
| Documented with an example | xclconfig, xcl-website | `README.md`, `CHANGELOG.md`, `readme_test.go`, `example/configonly/*`; `xcl-website:src/pages/examples/configuration-only.mdx` |

### Task: Move the typed lookups onto a shared reflective core

**File changes** (all in `xclconfig:query.go`):
- `query.go:68-95` `As[T]`: move the body into a new `asType(entity any, want reflect.Type) (any, error)`. The pieces map as follows:
  - nil entity → `&xclerrors.TypeMismatchError{Want: want, Got: "nothing"}`
  - identity → `if reflect.TypeOf(entity) == reflect.PointerTo(want) { return entity, nil }`
  - refusal → `convertibleTo(entity, want)`
  - copy → `typed := reflect.New(want).Interface(); schema.UnmarshalUntyped(entity, typed)`, with the same wrapped `TypeMismatchError`

  `As[T]` becomes: `v, err := asType(entity, reflect.TypeFor[T]()); if err != nil { return nil, err }; return v.(*T), nil`. **Keep `As`'s doc comment unchanged.**
- `query.go:103-127` `convertible[T]` → `convertibleTo(entity any, want reflect.Type) error` (same body with `want` passed in). Remove the generic version; `As` is its only caller.
- `query.go:179-217` `findByType[T]` → split it.
  - New `func (c *Config) entitiesOf(want reflect.Type, path ...string) ([]any, error)` holds `addressableType(want)`, the empty-path `NotTypeableError{Use: "All"}`, `c.typeable(path)`, and the scan loop, calling `asType(e, want)`. It returns `[]any{}` (non-nil) when nothing matches.
  - `findByType[T]` keeps its signature. It calls `entitiesOf(reflect.TypeFor[T](), path...)` and builds `[]*T` with `make([]*T, 0, len(found))`, casting each element. It must return `[]*T{}` (non-nil) on empty, exactly as today (`query.go:191` initialises `found := []*T{}`).
- `query.go:219-235` `findOne[T]`: add `func oneOf(found []any, path []string) (any, bool, error)` so `Decode` builds the identical `NotUniqueError`. It returns (entity, found, err) where err is `&xclerrors.NotUniqueError{Segments: path, Count: n}` for n>1. `findOne[T]` turns `found == false` into `NotFoundError{Address: strings.Join(path, ".")}`; `Decode` turns it into nil.
- `query.go:285-296` `addressable[T]` → `addressableType(want reflect.Type) error`, using `types.GetMeta(reflect.New(want).Interface())`. Keep `NotAnEntityError{Type: want, ReachedThrough: "declaration"}`. Replace the generic version (callers: `findByType`, `all`).
- `query.go:312-327` `all[T]`: extract `func (c *Config) typePath(want reflect.Type) ([]string, error)`. It holds `addressableType`, the nil-registry and `TypePath`-false `NotRegisteredError{Type: want, Use: "FindByType with the type and subtype"}`. `all[T]` becomes `path, err := c.typePath(reflect.TypeFor[T]()); ...; return findByType[T](c, path...)`. Keep the "deliberately goes through the kind lookup" comment, moved to sit beside the delegation.
- Order of checks must be preserved exactly. `all` runs addressable first, then registry; `findByType` runs addressable, then empty path, then typeable. Otherwise error precedence changes (see `query_all_test.go:104-120`, `TestAllRejectsANestedBlockWithTheSameConditionAsTheKindLookup`).
- No change to `query_methods_go127.go`. The methods still call `find`/`findByType`/`findOne`/`all`.

**Verification:** run `go test ./...` with no test edits; run `GOTOOLCHAIN=go1.25.0 go build ./... && go vet ./...` (mirrors `.github/workflows/go.yml` `build-minimum-go`).

**Complexity:** Medium
**Token estimate:** ~25k tokens
**Agent strategy:** Single agent, sequential. The edits are interdependent within one file. Run the full suite after each extraction (`asType`, then `addressableType`, then `entitiesOf`, then `typePath`).

### Task: Cover whole-block references to registered blocks

**Reuse first (user decision at walkthrough).** No new types go into the shared fixture package `internal/test_fixtures/registered`. No existing registered type has a nested block that references another registered non-`resource` block. The nearest candidate, the plugin `structs.Container.NetworkObj`, is a plugin `resource` type and value form only. So the one missing shape is declared **test-locally** and references the existing `registered.Cache` (registered bare, so declared `cache "x" {}`).

**File changes:**
- New `query_references_test.go` (package `xcl`). File-scope test-local types, with tag shapes modelled on `registered.Database`/`Timeouts` (`internal/test_fixtures/registered/types.go:28-45`):
  ```go
  // cacheClient is registered bare as "cache_client"; its nested blocks
  // reference a registered cache as a whole
  type cacheClient struct {
      types.ResourceBase `xcl:",remain"`
      Primary *cacheLink `xcl:"primary,block" json:"primary,omitempty"`
      Mirror  *cacheCopy `xcl:"mirror,block" json:"mirror,omitempty"`
  }
  type cacheLink struct { Cache *registered.Cache `xcl:"cache" json:"cache"` } // pointer form
  type cacheCopy struct { Cache registered.Cache  `xcl:"cache" json:"cache"` } // value form
  ```
  - Helper `setupCacheClientConfig(t *testing.T) *Config`, modelled on `setupBareTypeConfig` (`query_by_type_test.go:25-55`). It registers `&registered.Cache{}` as `registered.TypeCache` and `&cacheClient{}` as `"cache_client"`, then applies the fixture below against a file state store in `t.TempDir()`.
  - `TestWholeBlockReferenceFillsAPointerFieldInANestedBlock`: `Find[cacheClient](c, "cache_client.app")`. `Primary.Cache` is non-nil, with `Location == "us-east"` and `Meta.ID == "cache.east"`.
  - `TestWholeBlockReferenceFillsAValueFieldInANestedBlock`: `Mirror.Cache.Location == "eu-west"`.
  - Test style: prose comment block above the tests, `require` only, one behaviour each (see `query_all_test.go:14-38`).
- New fixture `internal/test_fixtures/config/registered/cache_client/main.xcl`: `cache "east" { location = "us-east" }`, `cache "west" { location = "eu-west" }`, `cache_client "app" { primary { cache = cache.east }  mirror { cache = cache.west } }`.
- If either test fails, fix the decode path in the parser's reference resolution under `internal/parser`. **STOP and report** before a fix if the cause is in `internal/xcl` (the MPL-licensed HCL copy), which has extra obligations (`conventions/never-modify-dependencies.md`).

**Complexity:** Low
**Token estimate:** ~8k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Add the invalid decode target error

**File changes:**
- `errors/query_errors.go:23-56`: add to the `var (...)` block:
  ```go
  // ErrInvalidDecodeTarget is returned by Decode when the target is not a
  // non-nil pointer to a struct. Match it with errors.Is.
  ErrInvalidDecodeTarget = errors.New("decode target must be a non-nil pointer to a struct")
  ```
  Update the block's leading comment (`:21-22`), which counts "the other five". Make it say the invalid target is also a caller bug, keeping the count accurate.
- Same file, after `NotUniqueError` (`:150-159`): add `type InvalidDecodeTargetError struct { Type reflect.Type }` with pointer receivers.
  - The struct is `{ Type reflect.Type; Nil bool }`. `Nil` is true when a typed nil pointer was passed. `Error()` reads `"decode target must be a non-nil pointer to a struct, got nil"` when `Type == nil`, `"..., got nil *main.Config"` when `Nil`, and `fmt.Sprintf("..., got %s", e.Type)` otherwise.
  - `Unwrap() error { return ErrInvalidDecodeTarget }`.

  `reflect`, `fmt` and `errors` are already imported (`:3-8`), so the import list is unchanged.
- `errors/query_errors_test.go`: add `TestInvalidDecodeTargetErrorUnwrapsToItsSentinel` and `TestInvalidDecodeTargetErrorNamesTheTypePassed`, following the existing tests in that file.
- `config.go:33-63`: add `ErrInvalidDecodeTarget = xclerrors.ErrInvalidDecodeTarget` with a one-line comment, and update the "The other five" sentence at `:31-32`.
- `config.go:91-104`: add the alias `InvalidDecodeTargetError = xclerrors.InvalidDecodeTargetError`.

**Complexity:** Low
**Token estimate:** ~8k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Implement Decode with its tests

**File changes:**
- New `decode.go` (package `xcl`, no licence header, matching `query.go`):
  ```go
  // Decode fills target ... (full contract: shapes, order, instances, nil/empty,
  // not-unique, invalid target, untouched fields, all-or-nothing, not generic so
  // available on every supported Go version)
  func Decode(c *Config, target any) error { return decode(c, target) }

  // Decode fills target from the entities the configuration declares.
  // See the package level Decode for the full contract.
  func (c *Config) Decode(target any) error { return decode(c, target) }
  ```
  The method sits in `decode.go`, **not** in `query_methods_go127.go`; it carries no build tag.
- `decode(c *Config, target any) error`:
  1. `v := reflect.ValueOf(target)`. If `!v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct`, return `&xclerrors.InvalidDecodeTargetError{Type: reflect.TypeOf(target), Nil: v.IsValid() && v.Kind() == reflect.Pointer && v.IsNil()}`.
  2. `s := v.Elem()`. Build `pending := []struct{ field reflect.Value; value reflect.Value }{}`. For `i := range s.NumField()`: `sf := s.Type().Field(i)`; skip `!sf.IsExported()`; `f := s.Field(i)`; skip `!f.CanSet()`.
     - **Collection shape:** `ft.Kind() == reflect.Slice && ft.Elem().Kind() == reflect.Pointer && ft.Elem().Elem().Kind() == reflect.Struct`.
       - `want := ft.Elem().Elem()`. If `path, err := c.typePath(want)` fails, `continue`. That covers not registered, not addressable and plugin types, which are left untouched.
       - `found, err := c.entitiesOf(want, path...)`; `return err` on error.
       - `slice := reflect.MakeSlice(ft, 0, len(found))`, appending `reflect.ValueOf(e)` for each.
       - Add the pair to `pending`.
     - **Single shape:** `ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct`.
       - `want := ft.Elem()`, then `typePath` (`continue` on error), then `entitiesOf`.
       - `one, ok, err := oneOf(found, path)`; `return err` on err (the `*NotUniqueError`, unwrapped).
       - The value is `reflect.ValueOf(one)` if ok, else `reflect.Zero(ft)`.
  3. Only after the loop: `for _, p := range pending { p.field.Set(p.value) }`. Return nil.
  - `c == nil` is not handled specially. The doc comment states that `c` must be non-nil, exactly as for `Find` (whose `find` dereferences `c` too).
- New `decode_test.go` (package `xcl`). **It reuses the existing registered types and fixtures** wherever they fit (user decision at walkthrough):
  - `setupFindConfig` (`query_test.go:28-66`, `registered/basic`) gives two `Database` (`resource.database.main`, plus `module.shared.resource.database.shared` in the module), one `App`, one `Consumer`, and `Cache` registered but undeclared.
  - `setupBareTypeConfig` (`query_by_type_test.go:25-55`, `registered/bare`) gives `cache.main` and `resource.database.main`.
  - A new helper, `setupDisabledConfig`, follows `setupFindConfig` but applies the existing `registered/disabled/main.xcl` (one disabled `resource.database.off`).
  - `setupCacheClientConfig` comes from the reference-coverage task.
  - The only new fixtures are two ordering files, `internal/test_fixtures/config/registered/ordered/forward/main.xcl` (`cache "first"`, then `cache "second"`) and `.../ordered/reversed/main.xcl` (`cache "second"`, then `cache "first"`). Each takes a `location`. They are applied by a helper `setupOrderedConfig(t, fixture)` that registers only `registered.Cache` bare. These are needed because the only same-type pair in the existing fixtures is split across the root and a module, so it cannot be written in the opposite order.

  Test-local struct types are declared at file scope:
  ```go
  type databasesAndApps struct { Databases []*registered.Database; Apps []*registered.App }
  type renamedDatabases struct { Anything []*registered.Database }
  type singleApp struct { App *registered.App }
  type singleDatabase struct { Database *registered.Database }
  type singleCache struct { Cache *registered.Cache }
  type orderedCaches struct { Caches []*registered.Cache }
  type withUnrelated struct { Databases []*registered.Database; Name string; Settings appSettings }  // appSettings: unregistered struct U
  type withUnrelatedPointer struct { Settings *appSettings } // *U shape, unregistered
  type threeTypes struct { Databases []*registered.Database; App *registered.App; Consumers []*registered.Consumer }
  type databasesOnly struct { Databases []*registered.Database }
  type databasesAndCaches struct { Databases []*registered.Database; Caches []*registered.Cache }
  type clients struct { Clients []*cacheClient }
  ```
  One test per item below, with positives and negatives in separate functions:
  - `TestDecodeFillsCollectionFieldsFromTheConfiguration` (`setupFindConfig`, `databasesAndApps`): 2 databases with `Location`/`Port` as configured, and 1 app.
  - `TestDecodeKeepsDeclarationOrder` (`setupOrderedConfig("forward")`): IDs are `cache.first`, `cache.second`.
  - `TestDecodeFollowsTheOppositeDeclarationOrder` (`"reversed"`): IDs are `cache.second`, `cache.first`.
  - `TestDecodeMatchesAllElementForElement` (`setupFindConfig`): for both fields, `require.Len`, then `require.Same` per index against `All[...]`.
  - `TestDecodeMatchesAllIncludingADisabledBlock` (`setupDisabledConfig`): `Databases` has length 1, is `Same` as `All[registered.Database]`, and has `Disabled == true`.
  - `TestDecodeReturnsTheSameInstanceAsFind` (`setupFindConfig`): `require.Same` against `Find[registered.Database](c, "resource.database.main")`. Then mutate `Location` through the decoded pointer and read it back through `Find`.
  - `TestDecodeSetsASingleFieldWhenExactlyOneIsDeclared` (`setupFindConfig`, `singleApp`): `resource.app.web`.
  - `TestDecodeLeavesASingleFieldUnsetWhenNoneIsDeclared` (`setupFindConfig`, `singleCache`, where Cache is registered but not declared): NoError, nil.
  - `TestDecodeFailsForASingleFieldDeclaredMoreThanOnce` (`setupFindConfig`, `singleDatabase` pre-set to a sentinel `&registered.Database{}`):
    - `require.ErrorIs(err, ErrNotUnique)`
    - `errors.As` gives `*NotUniqueError` with `Count == 2`
    - `require.Equal(t, err, findOneErr)`, where `_, findOneErr := FindOne[registered.Database](c, "resource", "database")`
    - the field is still the sentinel pointer (`require.Same`)
  - `TestDecodeLeavesOtherFieldsUnchangedWhenASingleFieldFails`: a struct with `Apps []*registered.App` and `Database *registered.Database`. `Apps` is pre-set to nil and is still nil after the error, which proves all-or-nothing.
  - `TestDecodeIgnoresFieldNames`: `renamedDatabases.Anything` equals `databasesAndApps.Databases` (`require.Same` per index).
  - `TestDecodeLeavesUnrelatedFieldsUntouched`: `Name` and `Settings` are pre-set and still equal afterwards. `withUnrelatedPointer.Settings` is pre-set and still `Same` afterwards.
  - The rejection tests are `TestDecodeRejectsAStructPassedByValue`, `TestDecodeRejectsANilPointer` (`(*databasesAndApps)(nil)`), `TestDecodeRejectsAnUntypedNil` and `TestDecodeRejectsAPointerToANonStruct` (`&someInt`). Each asserts:
    - `require.ErrorIs(err, ErrInvalidDecodeTarget)`
    - `c.Entities()` length and identity are unchanged (`require.Same` on the first entity, before and after)
    - for the by-value and non-struct cases, the value is unchanged
  - `TestDecodeBeforeApplyLeavesCollectionsEmptyAndSinglesUnset`: `NewConfig(WithPluginRegistry(reg))` registering Database and App as in `setupFindConfig`, with no Apply. Collections are `require.NotNil` and `require.Empty`; singles are `require.Nil`.
  - `TestDecodeMethodAndFunctionGiveEqualResults` (`setupFindConfig`) and `TestDecodeMethodAndFunctionGiveEqualErrors` (`setupFindConfig`, `singleDatabase`). These are separate functions because one is positive and one is negative.
  - `TestDecodeFillsWholeBlockReferencesThroughAPointerField` and `TestDecodeFillsWholeBlockReferencesThroughAValueField` (`setupCacheClientConfig`, `clients`, via `Clients[0]`).
  - `TestDecodeFillsEveryRegisteredTypeInOneCall` (`setupFindConfig`, `threeTypes`): every field equals the corresponding `All`/`FindOne`. This is success metric 1.
  - `TestDecodeFillsAFieldForANewlyDeclaredTypeWithNoOtherCode` (`setupBareTypeConfig`): `databasesOnly` and `databasesAndCaches` are each filled by the same single `Decode` call with no other code. The added `Caches` field holds `cache.main`. This is success metric 2.

**Complexity:** Medium
**Token estimate:** ~35k tokens
**Agent strategy:** 2 parallel agents after the dependencies land. Agent A writes `decode.go`. Agent B writes `decode_test.go` against the contract above. Then a sequential integration pass runs `go test ./...` and the Go 1.25 build and vet.

### Task: Assemble the config-only example with Decode

**File changes:**
- `example/configonly/main.go`:
  - Add, near `run` (`:61`), a type with a doc comment explaining that `Decode` fills it in one call and that a new block type needs only a new field:
    ```go
    // appConfig is the application's view of the configuration ...
    type appConfig struct {
        ConfigMaps  []*resources.ConfigMap
        Deployments []*resources.Deployment
        Service     *resources.Service
        Ingress     *resources.Ingress
    }
    ```
  - In `run`, after `c.Apply(dir)` (`:101-103`) and the `## Resources` loop (`:105-113`), add `var cfg appConfig; if err := c.Decode(&cfg); err != nil { return nil, err }`.
  - `printDeployments(out, cfg.Deployments)`: change the signature at `:141` to take `[]*resources.Deployment`, and remove the `FindByType` call and the commented Go 1.27 block (`:142-151`).
  - `printRouting(out, cfg.Service, cfg.Ingress)`: change `:193` to take the two pointers and remove both `Find` calls (`:194-205`).
  - Keep every `fmt.Fprint*` line byte-identical, so the output tests (`main_test.go:223-268`) pass unchanged.
  - Update the package comment (`:1-19`) to mention that the program gathers the configuration into one struct with `Decode`.
- `example/configonly/main_test.go`:
  - `:538-595` `TestConfigOnlyExampleUsesPortableLookupForm`: remove the `require.NotZero(t, lookups, ...)` line (`:594`). The example legitimately makes no generic lookups now; replace the line with a comment saying the guard still catches any lookup added later.
  - Add `TestConfigOnlyExampleAssemblesItsConfigurationWithDecode`. It parses `main.go` with `go/parser`, counts `*ast.CallExpr` whose selector name is `Decode`, and requires exactly one.
  - Add a separate negative test, `TestConfigOnlyExampleMakesNoPerTypeLookups`. It requires zero calls whose selector, after unwrapping `IndexExpr`/`IndexListExpr`, is `FindByType`, `All`, `FindOne` or `Find`. This is success metric 3.
  - The existing behaviour tests (`:90-221`) use `run`'s returned `[]any`, which is unchanged.
- `static_examples_test.go:17-22` globs examples. Confirm none of its checks require a lookup call.

**Complexity:** Low
**Token estimate:** ~15k tokens
**Agent strategy:** Single agent, sequential execution. Run `cd example/configonly && go test ./...`, then `go run . ./config` and diff the output against the pre-change output captured first.

### Task: Document Decode in the README and changelog

**File changes:**
- `README.md:309-347` ("Querying a configuration"):
  - After the lookup code block (`:313-325`), add a `#### Filling a struct of your own` subsection with a worked example: the server/mount configuration from the spec's technical approach. It shows HCL, an application struct, `c.Decode(&cfg)`, and the result.
  - The subsection states the contract:
    - `[]*T` gets every entity of T, as `All` returns it: declaration order, the configuration's own instances, disabled blocks included.
    - `*T` gets the single entity, is set to nil when T is not declared, and returns an error matching `xcl.ErrNotUnique` when T is declared more than once.
    - Other fields are untouched.
    - A target that is not a non-nil pointer to a struct returns `xcl.ErrInvalidDecodeTarget`.
    - On error nothing is assigned.
    - Before `Apply` the call succeeds with empty slices and nil pointers.
    - No tags are read and nested structs are not entered.
- `README.md:369-397` ("When a lookup cannot be answered"): add `ErrInvalidDecodeTarget` and `*InvalidDecodeTargetError` to the vocabulary list.
- `README.md:399-416` ("Two spellings, one implementation"): add a paragraph saying that `Decode` is not generic, so `c.Decode(&cfg)` and `xcl.Decode(c, &cfg)` are both available on Go 1.25, and that the examples use the method form for it.
- `README.md:68-119` ("Configuration only"): add one sentence and a short snippet pointing at `Decode` as how the example gathers its blocks.
- `CHANGELOG.md:1-2`: insert `## 20261003081552-e1e07cbe-config-decode` above the current top entry. It is a paragraph in the existing style naming `c.Decode(&cfg)` / `xcl.Decode(c, &cfg)`, the shapes, the outcomes, `xcl.ErrInvalidDecodeTarget` (`*xcl.InvalidDecodeTargetError`), and the example change. There are no breaking changes.
- `readme_test.go`: add, in the style of `:22-27`:
  - `TestReadmeDocumentsDecode`: contains `"#### Filling a struct of your own"`, `".Decode(&"` and `"ErrInvalidDecodeTarget"`.
  - `TestChangelogRecordsDecode`: contains `"xcl.Decode("` and `"ErrInvalidDecodeTarget"`.

**Complexity:** Low
**Token estimate:** ~12k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Show Decode on the docs site configuration-only page

**File changes** (repo root `/home/nicj/code/github.com/jumppad-labs/xcl-website`):
- `xcl-website:src/pages/examples/configuration-only.mdx:216-288` ("The program"):
  - `:223-227`: update the `run` snippet to the current signature `run(out io.Writer, handler xcl.EventHandler, r *registry.PluginRegistry, dir string, stateDir string) ([]any, error)`.
  - `:229-256`: update the registration and config snippet to `r.RegisterType(&resources.ConfigMap{}, "resource", "config_map")` and the rest, `xcl.WithStatePath(stateDir)` and `xcl.WithEventData(xcl.EventDataProcessed)`. Copy these from the rewritten `xclconfig:example/configonly/main.go`.
  - `:266-288`: replace the `FindByType` snippet with the `appConfig` struct and the `c.Decode(&cfg)` call (`title="example/configonly/main.go"`), then a short `printDeployments(out, cfg.Deployments)` excerpt.
  - Rewrite the prose at `:266` to introduce `Decode`: one call fills the struct, each field gets every block of its type, a single pointer gets the one block, and a new block type is one new field. Keep the mention that `Find` answers single-address questions.
- `:348-364` ("What to notice"): add one bold-lead bullet on gathering the configuration with `Decode`.
- Follow the page conventions: fenced blocks with a `title="<repo path>"` meta, inside `<Prose>`; no new components; no change to `Nav.astro`.
- Verify with `make check` (`npx astro check`) and `make build` in the website repo.

**Complexity:** Low
**Token estimate:** ~10k tokens
**Agent strategy:** Single agent, sequential execution. Its working directory is the `xcl-website` root, and the rewritten example `main.go` from `xclconfig` is passed to it as source.

## Testing Strategy

The strategy follows the plan's Testing Approach, broken down by task:

- **Move the typed lookups onto a shared reflective core.** This task adds no new tests. The proof is the existing suite passing **unedited**: `query_test.go`, `query_by_type_test.go`, `query_all_test.go`, `query_errors_test.go`, `query_migration_test.go`, `query_equivalence_go127_test.go`, `entity_subtype_test.go` and the example tests. The Go 1.25 build and vet must also pass.
- **Cover whole-block references to registered blocks.** Two positive tests go in `query_references_test.go`, covering the pointer form and the value form. They use test-local types referencing the existing `registered.Cache`, and one new `cache_client` fixture, through `Find`.
- **Add the invalid decode target error.** Two tests go in `errors/query_errors_test.go`, covering the unwrap to the sentinel and the message naming the type.
- **Implement Decode with its tests.** `decode_test.go` gets one function per acceptance criterion, plus success metrics 1 and 2, as listed in the task notes.
  - Positive and negative cases are always separate functions.
  - The three unusable-target cases are separate negative tests, with the nil case split further into untyped nil and typed nil.
  - Not-unique gets two negative tests: the error identity and count, and all-or-nothing assignment.
- **Assemble the config-only example with Decode.**
  - The existing output and behaviour tests must pass unchanged.
  - New AST guards check for exactly one `Decode` call and for zero lookups. This is success metric 3.
  - The portable-lookup guard loses only its `NotZero` assertion.
- **Document Decode in the README and changelog.** Add `TestReadmeDocumentsDecode` and `TestChangelogRecordsDecode` to `readme_test.go`.
- **Show Decode on the docs site configuration-only page.** The site has no content tests. Verify with `make check` and `make build` in `xcl-website`, and review the rendered page by eye.

Conventions apply throughout:
- testify `require` only
- no table-driven tests
- no mixing of positive and negative cases in one function
- state is always produced by a real `Apply` of an `.xcl` fixture

Success metrics map to tests as follows:
- Metric 1 → `TestDecodeFillsEveryRegisteredTypeInOneCall`
- Metric 2 → `TestDecodeFillsAFieldForANewlyDeclaredTypeWithNoOtherCode`
- Metric 3 → `TestConfigOnlyExampleAssemblesItsConfigurationWithDecode` and `TestConfigOnlyExampleMakesNoPerTypeLookups`

## Project References

- **Spec.** `20261003081552-e1e07cbe-config-decode`, read via `spektacular spec file read`.
- **Design documents.** None. The spec carries no design references.
- **Knowledge (repo tier, `xclconfig`).**
  - `conventions/shared-errors-package.md`
  - `conventions/testing-and-mocking.md`
  - `conventions/test-state-from-real-apply.md`
  - `conventions/code-style.md`
  - `conventions/dependencies.md`
  - `glossary/entity.md`
  - `architecture/config-is-the-public-query-surface.md`
  - `conventions/never-modify-dependencies.md`, relevant only if a reference fix reaches `internal/xcl`
- **Prior plan (historical).** `20260921093100-query-api-v2`, which established the lookup surface, `TypePath` and the two-spellings rule.
- **Repo roots.**
  - `xclconfig`: `/home/nicj/code/github.com/jumppad-labs/xcl`
  - `xcl-website`: `/home/nicj/code/github.com/jumppad-labs/xcl-website`

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Per task:

| Task | Tier | Budget | Strategy |
|------|------|--------|----------|
| Move the typed lookups onto a shared reflective core | Medium | ~25k | Single agent, sequential: the edits are interdependent within one file |
| Cover whole-block references to registered blocks | Low | ~12k | Single agent |
| Add the invalid decode target error | Low | ~8k | Single agent |
| Implement Decode with its tests | Medium | ~35k | Two parallel agents (`decode.go`, `decode_test.go`), then sequential integration |
| Assemble the config-only example with Decode | Low | ~15k | Single agent |
| Document Decode in the README and changelog | Low | ~12k | Single agent |
| Show Decode on the docs site configuration-only page | Low | ~10k | Single agent, working directory set to the `xcl-website` root |

The first three tasks have no dependencies on each other and can run in parallel. The README and example tasks can also run in parallel once Decode lands.

## Migration Notes

None. This change is purely additive. Every existing exported signature and behaviour is unchanged, and the example's printed output is unchanged. The only consumer-visible new names are `Config.Decode`, `xcl.Decode`, `xcl.ErrInvalidDecodeTarget` and `xcl.InvalidDecodeTargetError`.

## Performance Considerations

`Decode` costs one linear scan of `c.Entities()` per eligible field, the same as calling `All` once per type, plus reflection over the target's direct fields. That is adequate at one configuration's scale; the README already states that lookups are linear scans with no index. The core refactor adds one `reflect.Type` comparison per entity, replacing a type assertion, which is negligible.

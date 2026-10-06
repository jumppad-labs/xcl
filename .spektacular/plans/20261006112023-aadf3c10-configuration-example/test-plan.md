---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Test plan: 20261006112023-aadf3c10-configuration-example

The spec defines no success metrics beyond its acceptance criteria. Everything below is a manual review listed in the plan's Testing Approach. Who / when: the reviewer of this spec's branch, before it merges (or before the epic's release).

## 1. The configuration example stands alone

- **Where**: `example/configonly` in the xcl repo.
- **How**:
  1. `cp -r example/configonly /tmp/configonly-copy && cd /tmp/configonly-copy`
  2. Remove both `replace` lines from `go.mod`, then `go get github.com/jumppad-labs/xcl@<published version>` and `go get github.com/jumppad-labs/xcl/example/prettylog@<published version>` (the prettylog module must be published, or vendored alongside, for this to work).
  3. `go mod tidy && go build ./... && DB_PASSWORD=x go test ./...`
- **Pass**: it builds and every test (5 route tests, 2 smoke tests) passes. Note: this needs a published xcl version that contains the APIs used (`Decode`, `WithStateMask`, `WithEventData`); until one is published, record the check as blocked rather than failed.

## 2. No test-only seams

- **Where**: `example/configonly/main.go`, `example/configonly/routes.go`.
- **What to look for**: every function (`newStateKey`, `main`, `run`, `loadConfig`, `ingressRoutes`, `findContainerPort`, `Route.String`), parameter and return value is used by the program itself. `loadConfig`'s `options` are passed by `run`; `run(dir)` exists so the temporary state directory is removed before `os.Exit`.
- **Pass**: nothing exists only for the tests.

## 3. The configuration is unchanged

- **How**: from the xcl repo root, `git diff 97ee3be -- example/configonly/config example/configonly/resources` (97ee3be is the commit this spec started from).
- **Pass**: empty diff; the config map, secret, deployment, service and ingress are as before.

## 4. Routes are reported

- **How**: `cd example/configonly && make run`.
- **Pass**: standard output holds exactly one line per ingress path, `api.example.com/ -> service.api:80 -> deployment.api container api port http (8080)`, naming host, path, service, deployment, container and port; events go to standard error; exit status 0.

## 5. Website snippets match the source

- **Where**: in xcl-website, `src/pages/examples/configuration-only.mdx` and `src/pages/sensitive-values.mdx`; README.md "Configuration only" section in the xcl repo.
- **What to look for**: every code block titled with an `example/configonly/...` file matches that file's current contents (the two snippets from inside `run` are dedented by one tab); the "Run it" output lines match a real `make run` (timestamps and durations aside); the README's trimmed `deployment "api"` snippet and `appConfig`/`Decode` lines match `config/deployment.xcl` and `main.go`.
- **Pass**: no snippet differs from source other than indentation.

## 6. No application config example remains

- **How**:
  - xcl repo: `grep -rn -i -e appconfig -e "application config" -e application-config --exclude-dir=.spektacular --exclude-dir=.git . | grep -v CHANGELOG.md` — only `appConfig` (the configuration example's struct) and the e2e suite's `kubeAppConfig` may appear; `ls example` shows only `configonly`, `plugin`, `prettylog`.
  - xcl-website: `grep -rn -i -e application-config -e "application config" -e appconfig src` — only `appConfig` in configuration-only.mdx may appear; `make build` succeeds and `grep -rl application-config dist` prints nothing.
- **Pass**: as stated; historical CHANGELOG.md entries are allowed to mention it.

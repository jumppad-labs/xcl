---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Test plan: 20261003134528-327e0657-references-and-secrets

Manual procedures for the success metrics and reviews the plan's Testing Approach marks as manual. Everything else is covered by the automated suite (`go test ./...` in `xclconfig`).

## 1. No known test secret appears in the full suite's output

- **What to measure**: occurrences of the known test secrets in all test output under default settings. Threshold: 0, outside lines that are the test's own assertions or fixture paths.
- **How**: from the `xclconfig` root:
  ```
  go test -count=1 -v ./... > /tmp/xcl-suite.log 2>&1
  grep -n -e 's3cr3t-leak-check-7f1d' -e 'app-s3cret-9f8e7d6c' -e 'pg-s3cret-example' -e 'pg-an4lytics-example' /tmp/xcl-suite.log
  ```
  Also run each example by hand and search its output:
  ```
  cd example/appconfig && DB_PASSWORD=app-s3cret-9f8e7d6c XCL_LOG_LEVEL=debug go run . 2>&1 | grep -c app-s3cret-9f8e7d6c
  cd example/plugin && make build && XCL_LOG_LEVEL=debug go run . 2>&1 | grep -c -e pg-s3cret-example -e pg-an4lytics-example
  ```
- **Expected result**: the suite passes, the first `grep` finds nothing (a hit inside a `-v` test name or a failure message quoting the expectation must be inspected and justified), and each example command prints `0`.
- **Who / when**: the implementer or reviewer, before the epic is merged and before a release.

## 2. Example code keeps secrets out of output only by declaring them sensitive

- **What to look at**: `example/appconfig/resources/resources.go`, `example/appconfig/main.go`, `example/plugin/resources/resources.go`, `example/plugin/internal/plugin.go`, and any other file under `example/`.
- **What to look for**: every password or secret field is `types.Sensitive[...]`; `Reveal()` is called only where the value is used (`databaseURL` in appconfig, `connect` in the plugin provider); nothing filters, masks, truncates or omits a secret by other means (no string replacement, no hand-picked print fields chosen to avoid it, no `json:"-"` on a secret).
- **Pass**: no workaround found; each `Reveal()` result stays inside the function that needs it and is never printed, logged or returned.
- **Who / when**: a reviewer, during code review of this change.

## 3. Documentation site pages read correctly

- **Where**: in `xcl-website`, run `npm ci && npm run dev`, then open `http://localhost:4321/sensitive-values/`, `/examples/application-config/`, `/examples/plugins/`, `/plugin-logging/` and `/`.
- **What to look for**:
  - The Sensitive values page is reachable from the Guides menu and the home page feature card, and covers declaring a sensitive field, `Reveal()`, the validation error (`field "..." of ... is not declared sensitive ...`), the Go type error (`ErrTypeMismatch` naming the field), and that the application's own JSON and templates show `(sensitive)`.
  - Every quoted code block matches the files it is titled with in `xclconfig` (`resources.go`, `main.go`, `plugin.go`, `main.xcl` including `default = "pg-s3cret-example"`).
  - Every quoted output matches what the examples now print: compare against `go run .` in `example/appconfig` (the JSON's `"password": "(sensitive)"`) and in `example/plugin` (no password anywhere, connection strings without a password).
- **Pass**: all pages render, links work, and no quoted code or output differs from the repository.
- **Who / when**: a reviewer, before the site change is deployed.

## 4. Plugin developer guide warns about unwrapping

- **Where**: `xclconfig/docs/plugin-developer-guide.md`, section `## Sensitive fields`, and the note at the end of `## Logging from a provider`.
- **What to look for**: the guide says that once `Reveal()` is called the value is an ordinary value, no longer protected, and must not be logged or otherwise emitted; it lists the supported plugin instantiations and the load error for others.
- **Pass**: a plugin author reading only that section would know not to log, return in errors, or copy a revealed value into a plain field.
- **Who / when**: a reviewer, during code review.

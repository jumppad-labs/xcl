---
created_date: "2026-09-22"
document_status: final
closed_date: "2026-09-22"
---

# Test plan: 20260922061954-event-based-logging

Every success metric is covered by automated behavioural tests except two, which the plan's Testing Approach classed as manual: how the pretty example receiver looks, and the content and rendering of the docs site pages. The automated coverage for the others is:

- Direct log calls in library code: 0. Covered by `static_output_test.go`.
- No output with no receiver: 0 bytes. Covered by `TestApplyWithoutReceiverWritesNothingToStdoutOrStderr`, `TestNoReceiverWritesNothingWithBothPluginKinds` and each example's `TestRunWithoutReceiverWritesNothingToStdoutOrStderr`.
- One-line receiver in every example, no registry logger. Covered by `static_examples_test.go`.
- Events lost: 0. Covered by the `internal/eventstream` tests and `TestApplyWithBufferOfOneDeliversEveryEvent`.
- Example plugins log resource context without passing it. Covered by the example tests and `TestExamplePluginsDoNotLogResourceOrEventDetails`.

One known exception, accepted by the user: go-plugin v1.6.3 writes `[ERR] plugin: plugin acceptAndServe error: broker closed` to standard error when an external plugin is stopped within milliseconds of starting. Procedure 1 notes it if it appears.

## 1. The styled example receiver is readable

**What to measure**: when each example program runs, every event appears through the styled receiver as one line. Each line shows the severity, the `source`, and, where the event is about a resource, the `resource` and the step (`operation`). Nothing else is written to standard error.

**How**: from the repo root `/home/nicj/code/github.com/jumppad-labs/xcl`, in a colour-capable terminal:

1. `cd example/plugin && make run`. This builds `build/external`, then runs the example.
2. `cd example/plugin && XCL_LOG_LEVEL=debug go run .`
3. `cd example/configonly && make run`
4. `cd example/appconfig && make run`
5. `cd example/plugin && rm -rf build && go run .`. This is the failure path.

**Expected result**:
- Runs 1, 3 and 4 exit 0.
- Every standard-error line starts with a time (e.g. `1:04PM`), then a coloured level (`INFO`/`DEBU`/`WARN`/`ERRO`), a message, and `source=...` (`core`, `ExamplePlugin` or `external`), with `source`, `resource` and `operation` visibly coloured.
- Run 1 shows `load start`/`load success` for `ExamplePlugin` and `external`, `parse success` lines with `file=`, and for each postgres, redis, app and ingress resource `create start`, then a provider line (e.g. `created database source=ExamplePlugin operation=create phase=log resource=resource.postgres.main ...`), then `create success`. The destroy sequence follows. Standard output shows the `## Resources`, query and `## Destroyed` report.
- Run 2 additionally shows `DEBU` lines: `registering block types`, `provider ready` with `provider=postgres`/`provider=redis`, and go-plugin lines with `component=go-plugin`.
- Run 5 exits non-zero. The last line reads `error: plugin ./build/external failed to load: ... , build it with `make build` in example/plugin`, and a `load error` line appears first.
- No line appears in any other format. The only allowed exception is the go-plugin `broker closed` line above.

**Who / when**: a maintainer, before merging the `event-based-logging` branch and before each release that changes `example/prettylog` or `events.SlogHandler`.

## 2. The docs site pages read correctly and render

**What to measure**: `/events/`, `/plugin-logging/` and the three example pages render without layout problems. Their content matches the library, and every quoted code block matches the example sources. The navigation reaches the new pages.

**How**: from `/home/nicj/code/github.com/jumppad-labs/xcl-website`:

1. `make check`. Expect 0 errors and 0 warnings.
2. `npm run build`. Expect 6 pages built.
3. `npm run dev`, then open the printed URL (typically `http://localhost:4321/`).
4. Open the "Guides" dropdown in the navigation and visit "Events and logging" (`/events/`) and "Plugin logging and loading" (`/plugin-logging/`). Then visit each page under "Examples". Check at desktop width and at a phone width of about 375px.
5. For each `title="example/..."` code block, compare it with the same file under `/home/nicj/code/github.com/jumppad-labs/xcl/example/`.
6. `grep -rn "NewPluginRegistry(log\|NewStdOutLogger\|eventlog" src/ README.md`

**Expected result**:
- Steps 1 and 2 pass.
- Every page renders with the site's layout and no overflowing code blocks, at both widths.
- `/events/` describes the Event fields, operations and phases, delivery guarantees, receiver panics, the one-line `events.SlogHandler` setup, `prettylog` and the go-plugin exception.
- `/plugin-logging/` describes `plugins.Logger(ctx)`, resource and step binding, the external boundary (values as text), the Init logger, lazy loading, `ErrPluginLoad` and clash timing.
- Quoted code matches the sources. The known pre-existing condensed quotes are the `printDeployments` excerpt on configuration-only and the TLS/route excerpt on application-config.
- Step 6 prints nothing.

**Who / when**: the docs owner, before publishing the site update that goes with the xcl release containing this change.

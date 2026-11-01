---
tags: [testing, fixtures, plugins]
---

# Build fixture programs before isolating HOME in a test

Tests that start external plugins often point `HOME` at a temporary directory first, with `isolateHome(t)` in the root package or `t.Setenv("HOME", t.TempDir())` elsewhere, so the GitHub registry's cache and other per-user state stay out of the real home directory.

Any `go build` the test runs after that, such as `testutil.BuildProgram` or `testutil.BuildFixturePlugin`, sees the temporary `HOME` too. With no explicit `GOMODCACHE` or `GOPATH`, Go downloads every module into `$HOME/go/pkg/mod` inside the temporary directory, and it writes module files read-only. The test itself passes, but then fails at cleanup:

```
TempDir RemoveAll cleanup: unlinkat /tmp/Test.../001/go/pkg/mod/...: permission denied
```

It also leaves the read-only directory behind under `/tmp`.

**Rule:** build every fixture program first, then isolate `HOME`:

```go
plugin := testutil.BuildFixturePlugin(t, ".")
launcher := testutil.BuildProgram(t, "./internal/test_fixtures/plugins/launcher")
isolateHome(t)
```

`BuildFixturePlugin` builds each fixture once per test binary, so it can still slip through if another test in the same binary built that fixture earlier. A fixture built with `BuildProgram` is rebuilt every time, so it fails reliably when the order is wrong.

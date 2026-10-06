# Shared test helpers live in `internal/testutil`

**Tier:** always-applied

- A test helper needed by more than one package goes in `internal/testutil`,
  as an ordinary `.go` file. Never copy a helper into each package's
  `_test.go` files, and never put shared helpers in a `_test.go` file at the
  repo root: Go only compiles a `_test.go` file into its own package's tests,
  so nothing else can import it.
- Only `_test.go` files may import `internal/testutil`. That keeps it out of
  every library and binary.
- `internal/testutil` must not import the root `xcl` package. The root
  package's in-package tests import `testutil`, and that would be an import
  cycle. The same applies to any package whose in-package tests use it: a
  helper that needs such a package belongs with that package's tests, or the
  tests move to an external `_test` package.
- Helpers take `testing.TB`, call `t.Helper()`, and use `t.TempDir()` and
  `t.Cleanup()` rather than managing cleanup by hand.
- A helper used by one package only stays in that package's `_test.go` files.
  Package-specific queries over shared types (for example a `find` over
  `testutil.EventRecorder` events) also stay local.

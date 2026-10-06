# Testing & Mocking

**Tier:** always-applied

- Include unit tests for all business logic.
- Use testify `require` for unit tests.
- Use Mockery for mocking interfaces.
- NEVER use table-driven tests.
- Add integration tests for HTTP handlers.
- NEVER mix positive and negative tests in the same test function.
- Ensure tests are easy to read, favor verbosity over too much abstraction.
- ALWAYS locate tests with the source they test. A `_test.go` file tests the
  code in its own directory; never put tests in the repo root or a parent
  package that reach into another directory (globbing, parsing or building
  sources elsewhere). Each example under `example/` carries its own tests,
  including a `smoke_test.go` that builds and runs the real binary.
- NEVER write tests that inspect repository files instead of exercising code.
  No test reads `README.md`, `CHANGELOG.md`, the `docs/` guides or website
  pages and asserts on their text; no test reads CI workflow files or
  `go.mod`; and no test walks, parses or greps the repository's own source to
  enforce a rule (forbidden imports, banned calls, removed identifiers). No
  plan adds such a test to "guard" a changelog entry, README section, CI
  setting or code rule. Documentation and CI configuration are checked by
  human review; code rules are enforced by review and by tests of behaviour.


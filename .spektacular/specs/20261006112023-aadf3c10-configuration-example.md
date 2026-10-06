---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
epic: 20261006071139-7b266535-examples-and-output
---

# Feature: 20261006112023-aadf3c10-configuration-example

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

The configuration example is rewritten as a real application: it loads its configuration into its own types and acts on it by reporting where each ingress path sends traffic. It comes with the tests a developer would write for their own configuration-driven code, so application developers get an example they can copy and run as their own project. The documentation website's pages for it are updated to match.

<!--
  REQUIREMENTS
  Specific, testable behaviours the feature must deliver.
  Format: bold title on the checkbox line, detail indented below.
  Rules:
    - Use active voice: "Users can...", "The system must..."
    - Each requirement should be independently verifiable
    - Focus on WHAT, not HOW — avoid prescribing implementation
    - Keep each item atomic — one behaviour per line
-->
## Requirements

- [ ] **The configuration example stands alone**
  The configuration example builds, runs and tests on its own as if it were a separate project.
- [ ] **The configuration example reads as real code**
  The configuration example contains no code that exists only so tests can reach inside it.
- [ ] **The configuration example has a smoke test**
  The configuration example has a test that builds and runs its program the way a user would, and checks it succeeds.
- [ ] **The configuration example uses its configuration**
  The configuration example decodes its configuration into the application's own types and uses it. It reports, for each ingress path, the host, the path, and the service, deployment, container and port the traffic reaches.
- [ ] **The configuration example shows testing config-driven code**
  The configuration example's tests check the code that consumes the configuration against a test configuration, the way a developer would test their own application.
- [ ] **The configuration example's config is unchanged**
  The configuration example keeps its current configuration: a config map, a secret, a deployment, a service and an ingress, split across several files.
- [ ] **The application config example is removed**
  The separate application config example, which repeated the configuration example, is removed from the repository.
- [ ] **The website matches the configuration example**
  The documentation website's configuration example page, and any page quoting the configuration example, match the rewritten example. The application config example's page is removed or merged into the configuration example's.

<!--
  CONSTRAINTS
  Hard boundaries the solution must operate within. These are non-negotiable.
  Format: one bullet point per constraint.
  Examples:
    - Must integrate with the existing authentication system
    - Cannot introduce breaking changes to the public API
    - Must support the current minimum supported runtime versions
  Leave blank if there are no constraints.
-->
## Constraints

- **Each example is its own Go module.** It has its own module definition, so example-only dependencies never become dependencies of xcl itself.
- **Examples use only xcl's public packages.** They can't import xcl's internal packages, so a shared helper an example needs has to be one xcl publishes for application and plugin authors.
- **The project's testing conventions apply to every new and moved test.** testify `require`; Mockery for mocks; no table-driven tests; never positive and negative cases in the same test function; tests live with the code they test.

<!--
  ACCEPTANCE CRITERIA
  The specific, binary conditions that define "done".
  Format: bold title on the checkbox line, verifiable detail indented below.
  Each criterion must be:
    - Independently verifiable (pass/fail, not subjective)
    - Traceable back to a requirement above
    - Testable by someone who didn't write the code
-->
## Acceptance Criteria

- [ ] **The configuration example stands alone**
  Copying the configuration example's directory out of the repository, and pointing it at a published xcl version, leaves it buildable and its tests passing.
- [ ] **No test-only seams**
  The configuration example's program exposes no function, parameter or return value that only its tests use. A function the program itself also calls does not count.
- [ ] **Smoke test passes**
  The configuration example's smoke test builds the program, runs it with its default arguments, and passes.
- [ ] **Routes are reported**
  Running the configuration example prints, for each ingress path, the host, the path, and the service, deployment, container and port the traffic reaches.
- [ ] **Consuming code is tested against a test config**
  The configuration example's tests load a test configuration separate from the example's own, and check the reported routes.
- [ ] **The config is unchanged**
  The configuration example's configuration declares the same config map, secret, deployment, service and ingress as before this work.
- [ ] **The application config example is gone**
  The repository contains no application config example.
- [ ] **The website matches**
  Every code snippet on the website taken from the configuration example matches its current source. No page refers to the application config example.

<!--
  TECHNICAL APPROACH
  High-level technical direction to guide the planning agent. Include:
    - Key architectural decisions already made
    - Preferred patterns or technologies if known
    - Integration points with existing systems
    - Known risks or areas of uncertainty
  Format: one bullet point per direction/steer.
  Leave blank if you want the planner to propose the approach.
-->
## Technical Approach

- **Decode then derive routes.** Decode into the application's own config struct, with one small function that works out ingress routes. `main` prints the result, and the tests test that function against a test config.
- **Point the module at the local xcl** with a `replace`, so it builds against the working tree in the repo.
- **Smoke test** runs the built binary with standard library process handling.
- **Website updates** cover the configuration-only and application-config pages, and the pages quoting configuration example snippets (sensitive values, state masking). This overlaps with docs issue #4.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- No success metrics beyond the acceptance criteria have been defined for this spec.

<!--
  NON-GOALS
  Explicitly state what this spec does NOT cover. This is as important as
  the requirements — it prevents scope creep and sets clear expectations.
  Format: one bullet point per exclusion.
  Examples:
    - "Mobile support is out of scope (tracked in #456)"
    - "Internationalisation will be addressed in a follow-up spec"
  Leave blank if there are no explicit exclusions to call out.
-->
## Non-Goals

- **Generating Kubernetes manifests** from the configuration example. xcl's example types don't cover all of Kubernetes.
- **Docs drift not caused by the examples.** The rest of issue #4 (for example the README's state-masking snippet) stays with that issue.

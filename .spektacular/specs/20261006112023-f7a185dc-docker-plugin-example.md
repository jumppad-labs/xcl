---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
epic: 20261006071139-7b266535-examples-and-output
---

# Feature: 20261006112023-f7a185dc-docker-plugin-example

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

The plugin example is rewritten as a real project modelled on Jumppad: an external plugin that creates real Docker networks and containers, and an in-process plugin that renders templates to files. Each plugin comes with the tests a plugin author would write, so plugin developers get an example they can run for real and copy as their own project. The documentation website's pages for it are updated to match.

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

- [x] **The plugin example stands alone**
  The plugin example builds, runs and tests on its own as if it were a separate project.
- [x] **The plugin example reads as real code**
  The plugin example contains no code that exists only so tests can reach inside it.
- [x] **The plugin example has a smoke test**
  The plugin example has a test that builds and runs its program the way a user would, and checks it succeeds.
- [x] **A Docker plugin creates networks and containers**
  The plugin example includes a plugin that creates and destroys Docker networks and containers, with a container able to join a network declared in the same configuration.
- [x] **A template plugin renders files**
  The plugin example includes a plugin that renders a template with variables to a file, and removes the file on destroy.
- [x] **Plugin providers are unit tested without Docker**
  Each plugin's providers have unit tests that run without a Docker engine, the way a plugin author would test their own plugin.
- [x] **Real-Docker tests skip cleanly**
  Tests that need a real Docker engine are skipped, not failed, when no Docker engine is available.
- [x] **The application wiring is tested**
  The plugin example tests that the application loads both plugins and applies a configuration that uses them.
- [x] **The website matches the plugin example**
  The documentation website's plugin example page, and any page quoting the plugin example, match the rewritten example.

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

- **The Docker plugin uses real Docker.** When run, the Docker plugin talks to a real Docker engine, not a simulation. Its unit tests may use a stand-in.
- **Each example is its own Go module.** It has its own module definition, so example-only dependencies such as the Docker SDK never become dependencies of xcl itself.
- **Examples use only xcl's public packages.** They can't import xcl's internal packages, so a shared helper an example needs has to be one xcl publishes for application and plugin authors.
- **Plugin block forms are fixed.** Docker resources are written as `docker "network"` and `docker "container"` blocks, and templates as `template` blocks with no subtype, all without the `resource` keyword.
- **The Docker plugin is external and the template plugin is in-process.** The Docker plugin is built as its own binary; the template plugin is compiled into the application.
- **Library changes stay backwards compatible.** xcl's library may change where the example needs a capability it lacks, such as plugins registering a type with no subtype, and any such change must be backwards compatible.
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

- [ ] **The plugin example stands alone**
  Copying the plugin example's directory out of the repository, and pointing it at a published xcl version, leaves it buildable and its tests passing, or skipped where they need Docker and none is available.
- [x] **No test-only seams**
  The plugin example's program exposes no function, parameter or return value that only its tests use. A function the program itself also calls does not count.
- [x] **Smoke test passes**
  With Docker running, the plugin example's smoke test builds the program, runs it with its default arguments, and passes.
- [x] **Real Docker resources appear and disappear**
  With Docker running, applying the plugin example creates a Docker network and a container attached to it, both visible in Docker, and destroying it removes both.
- [x] **The template is rendered and removed**
  With Docker running, applying the plugin example writes the template's destination file with the variables substituted, and destroying it removes the file.
- [x] **Typed block forms parse**
  With Docker running, the plugin example's configuration, written in the fixed block forms, applies without error.
- [x] **Provider tests need no Docker**
  With no Docker engine available, every provider unit test in the plugin example runs and passes.
- [x] **Docker tests skip**
  With no Docker engine available, the tests that need one report as skipped, and the test run passes.
- [x] **Wiring test passes**
  With Docker running, the plugin example's wiring test applies a configuration that uses both plugins, and finds the resources of both. Without Docker it is skipped.
- [x] **Existing plugins are unaffected**
  Any library change made for the example leaves existing configurations and plugins that register types with a subtype loading and applying unchanged, and the library's existing tests pass.
- [x] **The website matches**
  Every code snippet on the website taken from the plugin example matches its current source.

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

- **Model the Docker plugin on Jumppad's structure.** A thin interface mirrors the Docker SDK methods the plugin uses, the real SDK client satisfies it, providers hold it as a field, and unit tests use a Mockery-generated mock of it. Pin the Docker SDK to the version Jumppad uses.
- **Model the template plugin on Jumppad's template resource.** Test it against a temporary directory, and reuse the Handlebars library xcl already depends on.
- **Point the module at the local xcl** with a `replace`, so it builds against the working tree in the repo.
- **Skip on Docker availability.** Real-Docker tests check whether a Docker engine is reachable and skip if not, rather than depending on build tags or `-short`.
- **Smoke test** runs the built binary with standard library process handling.
- **Risk to verify early:** plugins register types by type and subtype, but it hasn't been confirmed that a plugin can register a type with no subtype, as `template` needs.
- **Website updates** cover the plugins page and the pages quoting plugin example snippets (events, plugin logging). This overlaps with docs issue #4.

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

- **Jumppad-level completeness for the Docker plugin.** No volumes, health checks, image builds, port ranges, sidecars or refresh/change detection. Just enough network and container to be useful and testable.
- **Docs drift not caused by the examples.** The rest of issue #4 (for example the README's state-masking snippet) stays with that issue.

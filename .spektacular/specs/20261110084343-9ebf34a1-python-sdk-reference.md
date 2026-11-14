---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
epic: 20261031080131-8c2a58ec-typescript-sdk
---

# Feature: 20261110084343-9ebf34a1-python-sdk-reference

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Python developers using xcl have examples and a long README, but no reference for the package's API and no walkthrough on the documentation site. This change publishes a complete reference for the Python package on xcl.dev, generated from the package itself so it stays accurate, and adds a guide that takes a Python developer from an empty program to a working one. It serves anyone adopting xcl from Python, who can then look up any name or follow one path from start to finish without reading the package's source.

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

- [x] **A complete Python reference**
  Readers can look up every public name the Python package exports, with its signature, description, parameters, return value and the errors it documents, and every public member of each public class: its constructor, methods and properties, including inherited ones, but not other dunder methods.
- [ ] **Published on the documentation site**
  The Python reference is published on the documentation site, inside the site's own layout, navigation and footer.
- [x] **Reference stays in step with the package**
  When the package's documentation changes, the reference reflects it for that version with no separate edit.
- [x] **Synchronous and asyncio clients both covered**
  Readers can find the synchronous and the asyncio client in the reference and see which is which.
- [x] **The reference names its version**
  The reference shows which version of the package it describes.
- [ ] **The reference follows releases**
  A new release of the Python package reaches the published reference without anyone editing the site by hand.
- [x] **Undocumented names are caught**
  A public name or member with no doc comment fails the package's checks before it is released.
- [x] **Doc comments complete**
  Every public name and member has a doc comment.
- [x] **A guide for Python developers**
  A guide walks a Python developer from an empty program to a working one: installing from PyPI, creating a configuration, registering entity types and functions, using Go plugins, applying and destroying configuration, keeping state, finding entities, handling errors, and turning on printing and logging.
- [x] **Synchronous first, then asyncio**
  The guide covers the synchronous API first, then the asyncio API and when to use it, such as in web services.
- [x] **Guide links rather than repeats**
  The guide links to the site's topic pages for detail, and each API name it introduces links to that name in the reference.
- [x] **Current behaviour only**
  Every behaviour the guide describes matches the latest released package when the guide is published; anything planned but not built is marked as not yet supported.
- [x] **Reachable from the site**
  Readers can reach the reference and the guide from the site's main navigation and from the home page, and the package's README and the xcl README link to both.
- [x] **Positioning and terminology**
  The guide describes XCL as its own language, calls blocks entities and not resources (except inside code identifiers), and avoids calling it a superset of HCL, using the word "interpolation" or comparing it to Terraform.

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

- The reference must be generated from the package's signatures and doc comments, not written by hand.
- The pages must be part of the existing xcl.dev site, and the reference must be served at `/reference/python`.
- The package's public export list must be the definition of a public name.
- The tooling that generates the reference must not become a runtime dependency of the `xcl-lang` package, which the `xcl-python` repository publishes.
- The guide's program code must be Python only, with no language tabs; XCL configuration examples are permitted.
- The package's behaviour must not change; in `xcl-python` only doc comments, checks, the README and release workflows may change.
- The site must still build and deploy with its existing deployment on a push to `main`.

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

- [x] **Every export documented**
  For every name in the package's public export list, the built site has an entry at `/reference/python` showing its signature, its description and, where documented, its parameters, return value and errors, and for each public class an entry for every public member.
- [x] **Inside the site**
  A reference page on the built site shows the site's navigation and footer, and has no broken internal links.
- [ ] **A doc comment change appears**
  Given a package version with changed doc comment text, building the site against that version changes that entry's text, with no edit to the site's content.
- [x] **Both clients**
  The reference has entries for both the synchronous and the asyncio clients, each labelled as such.
- [x] **Version shown**
  The reference shows a version equal to the package version it was generated from.
- [ ] **A release reaches the site**
  After a new xcl-python release, the published reference shows that release's version, with no commit made by hand to the site.
- [x] **Missing doc comment fails**
  Removing the doc comment from one public method makes the package's checks fail, naming that method; with it restored they pass.
- [ ] **Doc comment check passes**
  The package's doc comment check passes on the released package with no exclusions for public names or members.
- [x] **Guide is complete**
  Following the guide from top to bottom with only the code it shows, including its install step from PyPI, produces a Python program that creates a configuration, registers an entity type and a function, uses a Go plugin, applies configuration, keeps state between runs, finds an entity by address and by type, handles a configuration error, turns on logging, prints an entity and destroys it, and that also runs the asyncio version of apply.
- [x] **Asyncio after synchronous**
  In the guide, the synchronous API appears before the asyncio API, and the asyncio section says when to use it.
- [x] **Guide links out**
  The guide links to the registries, plugins, state and events pages, and each API name it introduces links to an entry that exists in the reference.
- [x] **Matches the current release**
  Each code and configuration example in the guide, tried against the latest released package at publication, behaves as the guide says, and each feature it marks as not yet supported is unsupported in that release.
- [ ] **Reachable**
  On the built site, the reference and the guide appear in the main navigation and are linked from the home page, and the xcl-python README and the xcl README each link to both, and every one of those links opens the right page.
- [x] **Terminology**
  The guide does not contain the words "superset", "interpolation" or "Terraform", it describes XCL as its own language, and it calls blocks entities, using "resource" only inside code identifiers.

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

- Prefer extracting the API to structured data with a static tool such as Griffe, which reads source without importing the package, and rendering it through Astro pages, so the reference inherits the site's layout. A self-contained HTML generator such as pdoc is the simpler alternative if the styling is acceptable.
- Pin the package version the site documents in the website repository, and have xcl-python's release workflow dispatch an update to it, the same release-triggered update pattern used between xcl and xcl-python.
- Read the public export list from `xcl/__init__.py`, since many names are defined in private modules.
- Keep the doc comment check beside the package's existing checks in its CI.
- Add the guide under the site's existing Guides menu, and add a Reference entry to the navigation, which the TypeScript reference can later share.
- Link names in the guide to stable per-name anchors in the reference.
- Risk: a release dispatch to the website publishes straight to xcl.dev, because the site deploys on every push to `main`.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- A Python developer can answer "what does this method take and return?" from the site without opening the package's source.
- A new Python developer reaches a working apply against a Go plugin by following the guide alone.
- The published reference shows the latest package version within one hour of each release.

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

- A reference or guide for TypeScript: a later spec in this epic.
- A Go guide; Go's reference stays on pkg.go.dev.
- Publishing reference pages for older package versions.
- Rewriting the package's README.

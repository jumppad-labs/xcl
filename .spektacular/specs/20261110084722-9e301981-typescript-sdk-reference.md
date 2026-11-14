---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
epic: 20261031080131-8c2a58ec-typescript-sdk
---

# Feature: 20261110084722-9e301981-typescript-sdk-reference

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Once xcl's TypeScript package exists, its developers will need the same help Python and Go developers get on the documentation site. This change publishes a complete reference for the TypeScript package on xcl.dev that stays accurate as the package changes, adds a guide that takes a TypeScript developer from an empty program to a working one, and shows TypeScript beside Go and Python in every multi-language example on the site. It serves anyone adopting xcl from TypeScript, who can look up any name, follow one path from start to finish, and read the site's examples in their own language.

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

- [ ] **A complete TypeScript reference**
  Readers can look up every public name the TypeScript package exports, with its signature, description, parameters, return value and the errors it documents, and every public member of each public class: its constructor, methods and properties, including inherited ones.
- [ ] **Published on the documentation site**
  The TypeScript reference is published on the documentation site, inside the site's own layout, navigation and footer.
- [ ] **Reference stays in step with the package**
  When the package's documentation changes, the reference reflects it for that version with no separate edit.
- [ ] **The reference names its version**
  The reference shows which version of the package it describes.
- [ ] **The reference follows releases**
  A new release of the TypeScript package reaches the published reference without anyone editing the site by hand.
- [ ] **Undocumented names are caught**
  A public name or member with no doc comment fails the package's checks before it is released.
- [ ] **A guide for TypeScript developers**
  A guide walks a TypeScript developer from an empty program to a working one: installing from npm, creating a configuration, defining and registering entity types and functions, using Go plugins, applying and destroying configuration, keeping state, finding entities, handling errors, cancelling an operation, and turning on printing and logging.
- [ ] **Guide links rather than repeats**
  The guide links to the site's topic pages for detail, and each API name it introduces links to that name in the reference.
- [ ] **Current behaviour only**
  Every behaviour the guide describes matches the latest released package when the guide is published; anything planned but not built is marked as not yet supported.
- [ ] **Reachable from the site**
  Readers can reach the reference and the guide from the site's main navigation and from the home page, and the package's README and the xcl README link to both.
- [ ] **Positioning and terminology**
  The guide describes XCL as its own language, calls blocks entities and not resources (except inside code identifiers), and avoids calling it a superset of HCL, using the word "interpolation" or comparing it to Terraform.
- [ ] **TypeScript in every multi-language example**
  Every example on the site that offers a choice of Go or Python also offers TypeScript. Where the behaviour has no TypeScript equivalent, the tab says so.
- [ ] **One language choice across the site**
  Choosing TypeScript shows TypeScript in every example on the page and is remembered on later pages, as choosing Go or Python is.
- [ ] **Go stays the default**
  A reader who has made no choice sees Go, and a reader who already chose Python still sees Python, and neither sees another language's code before the expected one appears.
- [ ] **TypeScript examples are correct**
  Every TypeScript example on the site compiles against the latest released package when it is published, and one that stops compiling is reported by the site's checks.
- [ ] **The site names TypeScript**
  The home page shows how to install the TypeScript package, and places that say xcl is available from Go and Python also name TypeScript.

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

- The reference must be generated from the package's signatures and doc comments with TypeDoc, not written by hand.
- The pages must be part of the existing xcl.dev site, and the reference must be served at `/reference/typescript`.
- A name must count as public if, and only if, it is exported from the package's entry point; protected members and members marked `@internal` are not public.
- TypeDoc must not become a runtime dependency of the `xcl-lang` package, which the `xcl-typescript` repository publishes.
- The guide's program code must be TypeScript only, with no language tabs; XCL configuration examples are permitted.
- The package's behaviour must not change; in `xcl-typescript` only doc comments, checks, their development dependencies and configuration, the README and release workflows may change.
- The site must still build and deploy with its existing deployment on a push to `main`.
- The TypeScript core SDK (20261031080136-264e0eab-typescript-core-sdk) and TypeScript printing (20261110083940-53be9068-typescript-printing) must be delivered first.

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

- [ ] **Every export documented**
  For every name the package's entry point exports, the built site has an entry at `/reference/typescript` showing its signature, its description and, where documented, its parameters, return value and errors, and for each public class an entry for its constructor and every public method and property, including inherited ones.
- [ ] **Inside the site**
  Every reference page on the built site shows the site's layout, navigation and footer.
- [ ] **A doc comment change appears**
  Given a package version with changed doc comment text, building the site against that version changes that entry's text, with no edit to the site's content.
- [ ] **Version shown**
  The reference shows a version equal to the package version it was generated from.
- [ ] **A release reaches the site**
  After a new xcl-typescript release, the published reference shows that release's version, with no commit made by hand to the site. A release that breaks an example still updates the reference; the broken example is reported separately.
- [ ] **Missing doc comment fails**
  Removing the doc comment from one public method makes the package's checks fail, naming that method; with it restored they pass.
- [ ] **Doc comment check passes**
  The package's doc comment check passes on the released package with no exclusions for public names or members.
- [ ] **Guide is complete**
  Following the guide from top to bottom with only the code it shows, including its install step from npm, produces a TypeScript program that creates a configuration, defines and registers an entity type and a function, uses a Go plugin, applies configuration, keeps state between runs, finds an entity by address and by type, handles a configuration error, cancels an operation, turns on logging, prints an entity and destroys the applied configuration.
- [ ] **Guide links out**
  The guide links to the registries, plugins, state and events pages, and each API name it introduces links to an entry that exists in the reference.
- [ ] **Matches the current release**
  Each code and configuration example in the guide, tried against the latest released package at publication, behaves as the guide says, and each feature it marks as not yet supported is unsupported in that release.
- [ ] **Reachable**
  On the built site, the reference and the guide appear in the main navigation and are linked from the home page, the xcl-typescript README and the xcl README each link to both, and every one of those links opens the right page.
- [ ] **Terminology**
  The guide does not contain the words "superset", "interpolation" or "Terraform", it describes XCL as its own language, and it calls blocks entities, using "resource" only inside code identifiers.
- [ ] **A TypeScript tab everywhere**
  On the built site, every code box offering Go and Python has a TypeScript tab, on every page, holding TypeScript code or a statement that the behaviour has no TypeScript equivalent.
- [ ] **The choice carries**
  Choosing TypeScript in one box switches every box on the page to TypeScript, and after moving to another page and reloading, boxes still show TypeScript.
- [ ] **Existing readers unaffected**
  With no stored choice the page first shows Go, and with Python as the stored choice it first shows Python, and the reader never sees another language's code first.
- [ ] **Examples compile**
  Every TypeScript tab's code, wrapped into a program where it is a fragment, type-checks in a strict TypeScript project against the latest released package, and a tab that stops compiling fails the site's checks.
- [ ] **The site names TypeScript**
  The home page's install section includes the npm install command for `xcl-lang`, and every page that lists the languages xcl is available from also names TypeScript.

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

- Use TypeDoc's JSON output rendered through Astro pages, so the reference inherits the site's layout and, if the Python reference is already there, shares its Reference navigation entry and page templates. `typedoc-plugin-markdown` feeding MDX is the alternative.
- Pin the package version the site documents in the website repository, and have xcl-typescript's release workflow dispatch an update to it, the same release-triggered update pattern used between xcl and xcl-python.
- Extend `CodeTabs.astro` with a `typescript` slot and its panel rules, and teach the pre-paint script in `Head.astro` to accept `typescript` as a value of the stored language preference (key `xcl-lang`).
- Decide in planning how to type-check tabs: extract them at build time, or keep examples as checked source files the pages include.
- Keep the guide under the site's existing Guides menu, beside the Python guide.
- Risk: the site has 107 multi-language code boxes on 19 pages, so writing and checking their TypeScript is most of the work, and a few may depend on behaviour with no TypeScript equivalent.
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

- A TypeScript developer can answer "what does this method take and return?" from the site without opening the package's source.
- A new TypeScript developer reaches a working apply against a Go plugin by following the guide alone.
- The published reference shows the latest package version within one hour of each release.
- No open issue reports a TypeScript example on the site that does not work.

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

- A reference or guide for Python: an earlier spec in this epic.
- A Go guide; Go's reference stays on pkg.go.dev.
- Publishing reference pages for older package versions.
- Language tabs in examples that show only XCL configuration.
- Languages other than Go, Python and TypeScript in the tabs.

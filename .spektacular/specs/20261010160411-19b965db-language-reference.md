---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-11"
epic: 20261010160411-19b965db-language-reference
---

# Feature: 20261010160411-19b965db-language-reference

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

xcl.dev explains individual features but has no page that describes the XCL language itself, or how configuration maps onto the types an application defines. This spec adds a language reference that describes each part of the language and shows the matching application code in Go and Python, and gives every code example on the site language tabs so readers see their own language. It is for developers building XCL into their applications and for the people who write configuration for them.

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

- [x] **Reference is reachable from the site**
  Readers can find the language reference from the site's main navigation and from the home page, and the xcl README points to it.
- [x] **Files and blocks**
  The reference explains which files XCL reads, how a block's labels give an entity's type, optional subtype and name, and the address each block gets, using `database "postgres" "main"` and `app "web"` as the examples.
- [x] **Config to type mapping**
  For each shape of configuration, the reference shows the XCL next to the Go and Python types it fills in, covering required and optional attributes, nested blocks, lists of blocks, named blocks collected by name, map attributes (supported, but not the encouraged style), and fields the application computes rather than the author writes.
- [x] **Meta**
  The reference explains the built-in fields every entity has, such as its name and address, and how configuration refers to them.
- [x] **References and order**
  The reference explains how one entity refers to another's values, and how those references decide the order entities are processed in.
- [x] **Variables and outputs**
  The reference explains how to declare variables, give them defaults, build a variable's default from other values, override them from the application or with `XCL_VAR_<name>` environment variables, and how outputs work.
- [x] **Modules**
  The reference explains how to use a module, pass it values and read its outputs; that configuration reaches a module only through its outputs, while application code can reach any entity inside it by its full address; and which module sources work today.
- [x] **Functions**
  The reference lists every built-in function with an example, explains how an application registers its own functions and how plugins provide them, that function calls are evaluated while configuration is read, and how name clashes are reported.
- [x] **Expressions**
  The reference explains the expressions configuration can use: references in strings, lists, maps, conditionals, `for` expressions and splat. It states that string template directives are not part of XCL and that templating is done with functions.
- [x] **Current behaviour only**
  Every behaviour the reference describes matches the latest tagged xcl release when the pages are published; anything planned but not built is marked as not yet supported rather than described as working.
- [x] **Language tabs**
  Every code example on xcl.dev that shows application code offers it in Go and Python, chosen by tab. Picking a language switches every example on the page and is remembered across pages.
- [x] **Python wherever there is Go**
  Every existing page that shows Go application code also shows the Python equivalent, except the plugin authoring pages.
- [x] **Plugin authoring is Go**
  The plugin template and plugin example pages state that plugins are written in Go for now, and that they can be used from both Go and Python applications.
- [x] **Positioning and terminology**
  The reference describes XCL as its own language, calls blocks entities and not resources (except inside Go identifiers), and avoids describing it as a superset of HCL, using the word "interpolation", or comparing it to Terraform.

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

- The pages must live on the existing xcl.dev site and fit its existing structure and navigation; the site is now the home of the documentation and the README stays an overview.
- Plugin authoring examples stay Go-only until Python plugins exist.
- Python code uses the API defined by the python-support spec (`20261011130709-aa3eb933-python-support`).
- Go code must use the library's current exported names, such as `types.ResourceBase`, even where they still say "resource".

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

- [x] **Reachable**
  On the built site, the language reference appears in the main navigation and is linked from the home page, and the xcl README contains a link to it; every one of those links opens the reference.
- [x] **Files and blocks covered**
  The reference states the file extension XCL reads, shows `database "postgres" "main"` and `app "web"`, says which label is the type, the subtype and the name in each, states that the subtype is optional, and gives the address of each (`database.postgres.main`, `app.web`).
- [x] **Type mapping covered**
  For each of: a required attribute, an optional attribute, a nested block, a list of nested blocks, named blocks collected into a map, a map attribute, and a computed field, the reference shows an XCL snippet next to the type that receives it, tabbed in Go (with its struct tags) and Python.
- [x] **Meta covered**
  The reference lists each built-in field of an entity and shows a configuration snippet that refers to one of them on another entity.
- [x] **References covered**
  The reference shows one entity using a value from another and states that the referenced entity is processed first.
- [x] **Variables and outputs covered**
  The reference shows a variable with a default, a variable whose default is built from another value, each way to override it (from the application, and with an `XCL_VAR_<name>` environment variable), and an output being declared and used. It does not describe locals.
- [x] **Modules covered**
  The reference shows a module being used with an input and an output read from it in configuration, and application code finding an entity inside the module by its full address. It states that configuration can reach a module only through its outputs, and names the module sources that work today and those that do not.
- [x] **Functions covered**
  Every built-in function in the current release appears in the reference with an example. The reference shows an application registering a function in Go and in Python, and a plugin-provided function being called.
- [x] **Expressions covered**
  For each of a string with an embedded reference, a list, a map, a conditional, a `for` expression and a splat, the reference has an example. It states that `%{ }` template directives are not supported and points to functions for templating.
- [x] **Matches the current release**
  Each XCL and Go example in the reference, when tried by hand against the latest tagged xcl release at the time the pages are published, behaves as the reference says; each feature it marks as not yet supported is indeed unsupported in that release.
- [x] **Tabs work and are remembered**
  On the built site, picking Python in one code box switches every code box on the page to Python, and on going to another page its code boxes open on Python too.
- [x] **No Go-only examples**
  No page on xcl.dev outside the plugin authoring pages has a Go code box without a Python tab.
- [x] **Plugin pages state Go only**
  The plugin template and plugin example pages state that plugins are written in Go for now and can be used from Go and Python applications.
- [x] **Terminology**
  The reference does not contain the words "superset", "interpolation" or "Terraform", and does not use "resource" for a block, except inside Go identifiers.

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

- Show the config-to-Go-type mapping alongside each language feature in the reference, rather than collecting it in a separate section.
- The sections removed from the xcl README (struct tags, references, variables, locals, modules, functions) are a useful starting point, but parts are stale, so treat them as raw material to check against the code rather than text to copy.
- How the reference is divided into pages, and where it sits in the site's navigation, is left for the plan to propose.
- Use one reusable tabbed code component across the site, with the reader's language choice stored in the browser.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- Every language topic removed from the README (listed in Technical Approach) has a home on xcl.dev, so no question about the language needs the README's git history to answer.
- Questions and issues about how to write configuration or map it to a Go or Python type can be answered with a link to a section of the reference.
- A Python developer never meets a Go-only code example outside plugin authoring.

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

- No automated tests for the examples on the new pages; they are checked by hand.
- No changes to the xcl-vscode highlighter; adding formatting to it and publishing it is separate, later work.
- The reference documents the behaviour delivered by the language-surface and python-support specs. Gaps or bugs found while writing it are reported, not fixed in this work.
- Existing pages change only to add Python next to Go and to add links to and from the new pages.

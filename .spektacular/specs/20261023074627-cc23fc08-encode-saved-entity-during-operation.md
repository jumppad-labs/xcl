---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
epic: 20261023074219-4357d314-printing-and-pretty-output
---

# Feature: 20261023074627-cc23fc08-encode-saved-entity-during-operation

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->

## Overview

A program written in another language than Go, such as Python, cannot show what a resource has just become while a change is being applied: it receives each resource as it is created, but xcl refuses to turn it back into readable configuration until the whole change has finished. This change lets such a program render each resource the moment it arrives, as Go programs already can, so tools built on xcl can show people what is being created as it happens rather than all at the end.

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

- [ ] **Saved records render while an operation runs**
  Users can render a resource's saved record, in the form events carry it, as configuration text while an apply, validate, diff, destroy or load is still running, including from inside an event handler.
- [ ] **Same text during and after**
  The system must produce exactly the same text for the same record and the same rendering options (such as including computed values) whether it is rendered during an operation or after it.
- [ ] **The operation is unaffected**
  The system must let an operation run to completion, with the same result, events and saved state, when its event handler renders saved records as the events arrive.
- [ ] **Rendering a looked-up or edited entity is still refused during an operation**
  The system must keep refusing to render a looked-up or edited entity while an operation runs, with the same operation-in-progress error as today, because that rendering can read state the operation is still changing.
- [ ] **Errors stay specific**
  The system must report an invalid or unrecognised record rendered during an operation with the same error it reports outside one, not as an operation-in-progress error.
- [ ] **Python ships the capability**
  Users of the Python package can render saved records from their event handler during an operation, from both the synchronous and the asynchronous Python clients, without building the engine themselves.
- [ ] **Documentation says when rendering is allowed**
  The documentation for rendering configuration text, in the Python package and on the xcl website, must state that saved records can be rendered at any time, including from an event handler while an operation runs, and that rendering a looked-up entity cannot; the project's design for other-language support must no longer say that every render is refused during an operation.

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

- Rendering a saved record must be the only call newly allowed during an operation.
- The Python package must not wait or block inside an event handler to get around the refusal: the handler runs on the loop that drives the operation, and waiting there deadlocks it.
- The Python package can only rely on the change once it ships an engine built from an xcl release that contains it; the engine is bundled at a pinned xcl version.

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

- [ ] **Rendering from a synchronous event handler**
  A Python program, using the synchronous client with events set to carry each resource's saved record, applies a configuration that creates one resource, and its event handler renders the record of the create success event with computed values included. The render returns text without error, the apply completes successfully, and the text equals what the same render returns for the same record after the apply.
- [ ] **Rendering from an asynchronous event handler**
  The same program written against the asynchronous client renders the record from its handler with the same result, and the apply completes.
- [ ] **The apply is unchanged by rendering**
  An apply whose event handler renders every create success record produces the same result, the same sequence of event operations and phases, and the same saved state as the same apply with a handler that renders nothing.
- [ ] **Every operation allows it**
  A render of a saved record requested while each of apply, validate, diff, destroy and load is running returns text rather than an operation-in-progress error.
- [ ] **Looked-up entity still refused**
  From the same event handler, rendering the event's entity as a looked-up entity, rather than as its saved record, fails with the operation-in-progress error, and the apply still completes.
- [ ] **Edited entity still refused**
  From the same event handler, rendering an entity the handler has edited fails with the operation-in-progress error.
- [ ] **Invalid record during an operation**
  Rendering a record that is not valid JSON from the event handler fails with the invalid saved data error, not with the operation-in-progress error.
- [ ] **Unrecognised record during an operation**
  Rendering a well-formed record of a resource type no registry provides, from the event handler, fails with the same error the same render gives after the operation, not with the operation-in-progress error.
- [ ] **Installed package has it**
  In a fresh environment with the released Python package installed from its distribution, the synchronous rendering criterion above passes with no engine built locally, and the package reports an engine version that is a released xcl version containing the change.
- [ ] **Docs updated**
  The xcl website's configuration text page and the Python package's documentation for rendering saved records each state that rendering a saved record works during an operation and from an event handler, and that rendering a looked-up entity does not; the other-language design no longer says every render is refused during an operation.

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

- Prefer changing the core service's guard rather than the Go library: Go's saved-record encoder already documents calls during an operation as safe, and only the service-level check refuses them.
- Use the guard's own reason ("the entity slice is replaced when an operation ends") as the test for each call: a call that never reads the live entities does not need it.
- Known risk to confirm during planning: the saved-record encoder loads and briefly uses the plugin catalog to type the record; its documentation says external plugin processes are left running while an operation uses them, which should be verified under a concurrent apply.
- The first consumer is a Python logger that prints each created resource's configuration beneath its success event; it should need no further engine change.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->

## Success Metrics

- A Python program can print each created resource's configuration beneath its success event during an apply, with no buffering until the apply ends.

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

- Changing the Go library's encoding API; Go programs already render in process.
- Events carrying rendered configuration text themselves.
- A TypeScript counterpart; there is no TypeScript package yet.

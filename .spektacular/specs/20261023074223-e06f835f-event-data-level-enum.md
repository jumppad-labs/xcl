---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
epic: 20261023074219-4357d314-printing-and-pretty-output
---

# Feature: 20261023074223-e06f835f-event-data-level-enum

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->

## Overview

A Python program that asks xcl to attach resource data to events chooses how much by typing a bare word, which nothing checks until the program runs and which editors cannot complete. This change gives that choice a named set of values in the main package, matching the one Go programs already use, so a developer picks "processed" from a list, a mistake is caught by their tools before it runs, and the Python and Go programs read the same.

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

- [ ] **A named set of event data levels in the main package**
  Users can refer to each event data level (none, raw and processed) as a named value importable from the package's top level, without importing a submodule.
- [ ] **Both configuration flavours take the named levels**
  Users can pass a named level when creating either the synchronous or the asynchronous configuration, and the configuration attaches exactly the event data that level means today.
- [ ] **Leaving the option out means none**
  The system must attach no resource data to events when the user leaves the event data option out, as it does today.
- [ ] **Bare words are refused**
  The system must refuse an event data level given as a plain string, including one spelled exactly like a valid level, with an error that names the named set of levels so the user knows what to pass instead.
- [ ] **Anything else is refused**
  The system must refuse any value that is not one of the named levels, including an explicit None, with the same error, before any configuration is created.
- [ ] **Type checkers understand the option**
  A static type checker run over a user's program must flag a plain string passed as the event data level, and accept a named level.
- [ ] **Documentation shows the named levels**
  Every documented example and description of the event data option, in the package's own documentation and on the xcl website, must use the named levels.
- [ ] **The parity record pairs the levels**
  The Python-to-Go parity record must pair Go's event data level type with its Python counterpart.
- [ ] **Release notes explain the break**
  The release notes for the version that ships this change must say that plain string levels are no longer accepted and show the one-line change from the string form to the named form.

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

- The event data levels must be an enum exported from the package's top level.
- The option must accept only the enum. A plain string is refused, with no fallback and no deprecation period; this is an accepted breaking change for a pre-1.0 package.
- The solution must work on the package's existing minimum Python version, 3.11.

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

- [ ] **Levels import from the top level**
  The event data level enum can be imported from the package's top level, and it has exactly three members, for none, raw and processed.
- [ ] **Processed data reaches events, synchronously and asynchronously**
  Applying a configuration that creates one resource, with the processed level chosen, delivers a create success event whose entity is the created resource, for both the synchronous and the asynchronous configuration.
- [ ] **Raw data reaches events**
  The same apply with the raw level chosen delivers events carrying the resource's raw data, matching what the string "raw" delivered before this change.
- [ ] **Left out and none carry nothing**
  The same apply with the option left out, and again with the none level, delivers events that carry no resource data.
- [ ] **A correctly spelled string is refused**
  Creating either configuration with the event data level given as the string "processed" fails with an error whose message names the event data level enum, and no configuration is created.
- [ ] **Other values are refused**
  Creating either configuration with the event data level given as an integer, as an explicit None, or as the string "verbose" fails with the same error, and no configuration is created.
- [ ] **A type checker flags a string**
  Running mypy or pyright over a snippet that passes "processed" as the event data level reports an error on that line, and reports none for the same snippet passing the processed member.
- [ ] **Docs updated**
  A search of the package README and the xcl website for the event data option finds no example passing a bare string level.
- [ ] **Parity record updated**
  The parity record's event data row names Go's event data level type and the Python enum.
- [ ] **Release notes**
  The release notes for the shipping version contain a line stating that string levels are refused, with the string-to-enum migration example.

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

- Name the enum after Go's equivalent, `xcl.EventDataLevel`, with members `NONE`, `RAW` and `PROCESSED`, so Go and Python programs read the same and the parity record pairs them one to one.
- Prefer a string-valued enum whose values are today's wire words, so reprs and logs stay readable; because such members compare equal to their strings, refusing plain strings has to check the value's type rather than its equality.
- Prefer a `TypeError` for a refused value, the Python convention for an argument of the wrong type.
- The protocol already carries the level as its own enum, so nothing on the wire needs to change.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->

## Success Metrics

- No post-delivery success metric has been defined; correctness is covered by the acceptance criteria.

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

- Adding new event data levels.
- Turning other string-valued options into enums; each would be its own change.
- A TypeScript counterpart; there is no TypeScript package yet.
- Any change to the engine or its protocol.

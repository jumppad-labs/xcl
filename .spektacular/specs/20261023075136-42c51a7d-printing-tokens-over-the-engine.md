---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
designs:
    - source: design
      path: printing-and-highlighting.md
epic: 20261023074219-4357d314-printing-and-pretty-output
---

# Feature: 20261023075136-42c51a7d-printing-tokens-over-the-engine

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->

## Overview

Programs written in languages other than Go cannot show xcl's errors with their source, cannot colour resources or change summaries their own way, and cannot use an editor colour theme, because the engine they call into, which runs xcl's Go core for them, sends them either a single line or finished terminal colour codes. This change has the engine send every error's full report, and every rendered resource and change summary, as labelled text (text split into pieces, each tagged with what it is, such as an attribute name or a comment), and convert editor themes for them, so each language can print the same readable output as Go, styled as its users choose, without asking the engine again.

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

- [ ] **Every error carries its report**
  The system must attach the labelled report (the report split into parts, each labelled with its kind, such as an attribute name or a line comment) to every error it returns to another language that holds a configuration problem, including errors delivered to an event handler during an operation, so the language can print it without asking the engine again.
- [ ] **Reports match Go's**
  The labelled report another language receives must render, as plain text, exactly the report Go prints for the same problem, before any wrapping of the detail.
- [ ] **Errors without a configuration problem**
  The system must return errors that hold no configuration problem with no report, and their message unchanged.
- [ ] **Rendered resources come as labelled text**
  Users of another language can ask for a rendered resource, from a saved record or from a looked-up or edited entity, as labelled text as well as plain text.
- [ ] **Change summaries come as labelled text**
  The system must deliver a change summary to another language with its labelled text alongside the structured summary it delivers today.
- [ ] **Editor themes convert without a configuration**
  Users of another language can convert a VS Code colour theme into styles keyed by kind of text, without creating a configuration.
- [ ] **Kinds agree across languages**
  The kinds of labelled text the engine sends must have the same identities and categories as Go's, and an unknown kind from a newer engine must still fall into its category.
- [ ] **Existing callers keep working**
  Messages from a caller built against the previous protocol must still be accepted, and a request for the previous terminal-colour output must be treated as a request for plain text rather than failing.
- [ ] **Protocol documentation**
  The protocol's own documentation, and the project's design for other-language support, must describe the report on errors, labelled text, and theme conversion.

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

- The work must be built to the design document `design/printing-and-highlighting.md` (source `design`), which fixes what the engine sends, the kind numbering and the message shapes.
- Protocol changes must be additive: fields and methods are added, the field older callers use to request terminal-colour output is reserved and never reused, and the protocol's breaking-change check must pass.
- Lexing configuration text and laying out reports and change summaries must stay in the core; no language reimplements them.
- The report must be sent unwrapped; wrapping the detail is left to each language when it prints.

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

- [ ] **Report on an apply error**
  Applying a configuration with an unsupported block through the engine returns an error carrying labelled report text whose plain rendering equals Go's plain report for the same configuration.
- [ ] **Report during an operation**
  The error delivered to an event handler for the same failure, while the apply runs, carries the same labelled report.
- [ ] **No report for other errors**
  Looking up an address that does not exist through the engine returns a not-found error with no report and the same message as today.
- [ ] **Labelled resource text**
  Rendering a created resource through the engine, from its saved record, from its looked-up entity and from an edited entity, with labelled text requested, returns labelled text whose plain rendering equals the plain text returned alongside it.
- [ ] **Labelled change summary**
  A diff through the engine returns labelled text whose plain rendering equals Go's plain change summary for the same diff, alongside the structured diff.
- [ ] **Theme conversion**
  Converting a VS Code theme that colours properties orange, with no configuration created, returns a style for the attribute kind containing that orange.
- [ ] **Kind identities**
  For every kind in the published kind list, the number and category another language receives equal those Go reports.
- [ ] **Unknown kinds fall into their category**
  A kind number a language does not recognise, within a known category's range, is placed in that category and printed in that category's style.
- [ ] **Previous callers**
  A rendering request from a caller built against the previous protocol, asking for terminal colour, succeeds and returns plain text.
- [ ] **Compatibility check passes**
  The protocol's breaking-change check against the main branch reports no breaking change.
- [ ] **Docs**
  The protocol documentation and the other-language design describe the report on errors, labelled text and theme conversion.

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

- Lay the report out once, when an error is converted for the engine, from the problem's summary, location, source context and detail.
- Reuse the core's existing TextMate selector rules for theme conversion rather than writing new matching logic.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->

## Success Metrics

- A Python program prints a configuration problem with its source using only what the engine already returned, with zero extra engine calls.

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

- Formatting labelled text into terminal colour, HTML or SVG in any language; each language's own spec covers that.
- Structured problem fields beyond what errors carry today.
- A TypeScript client; there is no TypeScript package yet.

---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
designs:
    - source: design
      path: printing-and-highlighting.md
epic: 20261023074219-4357d314-printing-and-pretty-output
---

# Feature: 20261023075302-bad71237-printing-and-highlighting-python

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->

## Overview

Python programs using xcl see configuration mistakes as a single line, cannot style xcl's output, and cannot render it for a web page or an image; even an unhandled error gives no hint of where in the configuration the problem is. This change gives Python the same readable output as Go: an error shows the lines in question wherever it ends up, in a traceback, a log or printed on purpose; resources and change summaries print in one call; and output can be plain, coloured, a web page the site restyles, or an image, with the common logging setups needing little or no extra code.

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

- [ ] **One-call error printing with context**
  Users can print any xcl error to a stream of their choice, or get it as a string, in one call, and a configuration problem is shown as the same report Go prints: severity and summary, location, numbered source lines with the fault marked, and detail.
- [ ] **Unhandled errors show their report**
  The system must make an xcl error that holds a configuration problem show its plain report in an unhandled error's traceback and in an exception logged with its traceback.
- [ ] **Ordinary messages stay one line**
  The system must keep an xcl error's string form to one line.
- [ ] **Detail wraps to a width**
  The system must wrap a report's detail text at 80 columns by default, as Go does, and users can choose another width or no wrapping.
- [ ] **One-call printing of resources and change summaries**
  Users can print a resource as configuration text, and a change summary, in one call each, with the same formatting choices as errors and the existing choices about which values to include.
- [ ] **Choice of output format**
  Users can choose plain text, terminal colour, HTML marked up with classes, HTML with inline styles, or SVG, for any printed output.
- [ ] **Websites restyle HTML output**
  Users can generate a stylesheet from a theme, and HTML output carries the same class names as Go's.
- [ ] **Themes extend the default**
  Users can change the style of any kind of text by giving only the kinds they want to change, written as the same style strings Go uses, with the same fallback from a kind to its category (such as line comments to comments).
- [ ] **Themes can be checked**
  Users can validate a theme and are told about any style word that is not recognised; printing with such a theme ignores the unknown word rather than failing.
- [ ] **Editor themes can be imported**
  Users can load a VS Code colour theme and use it as a theme.
- [ ] **Custom output formats**
  Users can write their own output format that receives the whole labelled text at once and writes to any stream, and use it anywhere a built-in format is accepted.
- [ ] **Labelled text for extenders**
  Users can obtain the labelled text (the output split into parts, each labelled with its kind, such as an attribute name or a line comment) of an error's report, a resource's configuration text and a change summary.
- [ ] **Colour chosen from the stream on request**
  Users can ask for a format chosen from their stream, which picks terminal colour for a terminal unless the NO_COLOR convention is set, and plain text otherwise.
- [ ] **Events reach the standard logging system**
  Users can send xcl's events to Python's standard logging in one line, at the same levels and with the same messages Go's logging bridge uses, with the event's details available to formatters.
- [ ] **Coloured reports in log output**
  Users can give a standard logging handler a formatter that renders a logged xcl error's report in the format of their choice, in one line.
- [ ] **Replacement for the previous option is documented**
  The release notes must give the replacement for the removed option that requested terminal-coloured text when rendering a resource.
- [ ] **Documentation describes Python printing**
  The package's documentation, its parity record and the xcl website must document error printing, how reports appear on unhandled and logged errors, output formats, themes, stylesheet generation, custom formats and logging integration, with Python examples, and the website must no longer say the report is unavailable from Python.

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

- The work must be built to the design document `design/printing-and-highlighting.md` (source `design`), which fixes the Python surface, the split between the core and the language, the style syntax and the logging integration.
- Python's built-in formats must produce output identical to Go's, checked against the shared reference outputs.
- The package must not lex configuration or lay out reports itself; it formats the labelled text returned by the bundled xcl engine (the Go core the package runs and calls).
- The bundled engine must be an xcl release containing the Go printing and engine labelled-text work before this ships.
- The package must not add a runtime dependency beyond protobuf.
- The package must keep Python 3.11 as its minimum supported version.
- The package must never emit colour unless the user chooses a colour format, or explicitly asks for a format chosen from the stream; default output is never colour-detected.
- The previous option requesting terminal-coloured text must be removed, not deprecated; the break is accepted for a pre-1.0 package.

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

- [ ] **A bad configuration prints as a report**
  A Python program that applies a configuration with an unsupported block on line 7 and prints the raised error as plain text writes the same report Go writes for that configuration, byte for byte.
- [ ] **As a string**
  Getting the same error's report as a string returns the same text the print writes.
- [ ] **Unhandled error**
  Running a script whose apply raises that error, without catching it, prints a traceback followed by the plain report.
- [ ] **Logged with traceback**
  Logging the caught error with its traceback through standard logging writes the plain report after the traceback.
- [ ] **One-line string form**
  Converting the error to a string gives a single line.
- [ ] **Resource and change summary**
  Printing a created resource with computed values included, and printing a change summary, write the same text as Go for the same resource and diff.
- [ ] **Formats match the reference outputs**
  Every built-in format applied to the shared reference inputs produces the shared reference outputs byte for byte.
- [ ] **Stylesheet**
  Generating a stylesheet from the default theme produces the same CSS as Go.
- [ ] **Theme extension and fallback**
  A theme made from the default plus a comment style colours line, hash and block comments with it; adding a line-comment style overrides it for line comments only.
- [ ] **Invalid style words**
  A theme containing an unknown style word reports that word when validated, and printing with it still succeeds, ignoring the word.
- [ ] **Detail wrapping**
  A problem whose detail is longer than 80 characters prints with no detail line over 80 columns by default, with no detail line over 40 columns when a width of 40 is chosen, and on a single line when wrapping is off.
- [ ] **VS Code theme**
  Loading a VS Code theme that colours properties orange and printing a resource with it colours attribute names orange.
- [ ] **Custom format**
  A user-written format passed to error printing, resource printing and change-summary printing receives the whole labelled text at once in each case and writes its output to the given stream.
- [ ] **Labelled text**
  Obtaining the labelled text of an error's report, of a resource and of a change summary each returns parts that, joined together, equal the matching plain output, each labelled with a kind.
- [ ] **Colour chosen from the stream**
  Asking for a format chosen from a terminal stream gives terminal colour; from a file, or with NO_COLOR set, gives plain text.
- [ ] **Events to logging**
  Configuring the logging bridge as the event handler and applying a configuration writes log records whose levels and messages match Go's logging bridge for the same apply, with the event's resource and operation available on each record.
- [ ] **Coloured log reports**
  A standard logging handler given the coloured formatter, logging a caught xcl error with its traceback, writes the report with terminal colour codes.
- [ ] **Old option gone, notes present**
  Passing the previous highlighting option when rendering a resource fails as an unexpected argument, and the release notes name its replacement.
- [ ] **Docs**
  The README, the parity record and the website's problems and configuration text pages show Python examples of error printing, how reports appear on unhandled and logged errors, output formats, themes, stylesheet generation, custom formats and logging, and the website no longer says the report is unavailable from Python.

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

- Follow the standard library's own pairs, `traceback.print_exception` and `traceback.format_exception`, for the shape of error printing.
- Use exception notes so tracebacks, logging, rich and loguru show reports without adapters.
- Rely on structlog's and loguru's standard logging integration.
- Mirror the kind numbering from the protocol by hand and test it against the protocol, as the package's other enums are.

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

- The pretty logger for Python, which is a separate spec in this epic.
- Dedicated adapters for structlog or loguru.
- A TypeScript implementation.

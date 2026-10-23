---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
designs:
    - source: design
      path: printing-and-highlighting.md
epic: 20261023074219-4357d314-printing-and-pretty-output
---

# Feature: 20261023074926-eaa51527-printing-and-highlighting-core

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->

## Overview

When xcl finds a mistake in a configuration, it used to show the lines in question; today it prints a single line, and getting the full picture takes several steps few people know about. Colouring xcl's output is just as awkward: it follows an editor's conventions rather than the language's, cannot be restyled to match a website, and cannot produce whole documents such as images. This change makes xcl print any error, resource or change summary readably in one step, with the surrounding lines restored, as plain text, terminal colour, a web page the site can restyle, or an image. It also publishes reference outputs that every language's printing is checked against.

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
  Users can print any error returned by xcl to a destination of their choice in one call, and a configuration problem is shown as a report: its severity and summary, where it is (file, line, block and field), the numbered source lines around it with the fault marked, and its detail.
- [ ] **Surrounding lines are restored**
  The report must show up to two lines before and after the lines at fault, clipped at the start and end of the file.
- [ ] **Every problem in a wrapped error is printed**
  The system must print every configuration problem inside an error, including one the application has wrapped with its own context, and print an error that holds no configuration problem as its ordinary message.
- [ ] **Ordinary messages stay one line**
  The system must keep an error's ordinary message to one line, and print the full report when the caller asks for the error's detailed formatting.
- [ ] **Detail wraps to a width**
  The system must wrap a report's detail text at 80 columns by default, and users can choose another width or no wrapping.
- [ ] **One-call printing of resources and change summaries**
  Users can print a resource as configuration text, and a change summary, in one call each, with the same formatting choices as errors, and with the existing choices about which values to include.
- [ ] **Choice of output format**
  Users can choose plain text, terminal colour, HTML marked up with classes, HTML with inline styles, or SVG, for any printed output.
- [ ] **Websites restyle HTML output**
  The system must mark every coloured part of HTML output with stable, documented class names for its kind and its kind's category (each piece of output is labelled with a kind, such as an attribute name or a line comment, and kinds are grouped into categories, such as comments), so a website's own stylesheet controls the colours.
- [ ] **Stylesheets from themes**
  Users can generate a stylesheet from any theme, as a starting point for a website's own styles.
- [ ] **Themes extend the default**
  Users can change the style of any kind of text by giving only the kinds they want to change, written as short style strings, and styling a category styles every kind in it unless a kind is styled itself.
- [ ] **Themes can be checked**
  Users can validate a theme and are told about any style word that is not recognised; printing with such a theme ignores the unknown word rather than failing.
- [ ] **Editor themes can be imported**
  Users can load a VS Code colour theme and use it as a theme.
- [ ] **Custom output formats**
  Users can write their own output format that receives the whole labelled text at once and writes to any destination, and use it anywhere a built-in format is accepted.
- [ ] **Labelled text for extenders**
  Users can obtain the labelled text (the output split into parts, each labelled with its kind) of an error report, a resource's configuration text, a change summary, or any configuration text they hold.
- [ ] **Colour chosen from the destination on request**
  Users can ask for a format chosen from their destination, which picks terminal colour for a terminal unless the NO_COLOR convention is set, and plain text otherwise.
- [ ] **Reports in structured logs**
  A configuration error logged through a structured logger that records an error's detailed formatting must carry the full report.
- [ ] **Reference outputs for every language**
  The system must publish reference outputs for every built-in format, and for the class names HTML output uses, over a fixed set of inputs, and the Go formats must match them; other languages are checked against the same outputs in their own work.
- [ ] **Replacements for the previous API are documented**
  The release notes must give a one-line replacement for each removed highlighting, report and change-summary rendering function.
- [ ] **Examples use the new printing**
  Every example program that prints errors, resources or change summaries must use the new printing, and the examples that print errors must show reports.
- [ ] **Documentation describes the new printing**
  The xcl website must document printing errors, resources and change summaries, output formats, themes, class names and stylesheet generation, custom formats, and logging integration, with Go examples.

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

- The work must be built to the design document `design/printing-and-highlighting.md` (source `design`), which fixes the split between the core and each language, the token kinds and their numbering, the style syntax, the formatter shape, the print functions and the removals.
- The core library must not decide on its own whether to emit colour.
- Lexing configuration text must stay in the core and must not be reimplemented by any language.
- Restoring surrounding lines must use the source already read when the configuration was parsed, never re-read files from disk.
- The root module must stay free of cgo and of any new third-party dependency for highlighting; chroma is not used.
- The previous highlighting, report and change-summary rendering API must be removed, not deprecated; the break is accepted for a pre-1.0 library.
- The existing xcl-vscode highlighting parity tests must continue to pass.

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
  Applying a configuration with an unsupported block on line 7 and printing the returned error as plain text writes a report whose first line is "Error: Unsupported block type", whose location line names the file, line 7, the block and the field, which shows numbered lines 5 to 9 of the file, and which ends with the detail.
- [ ] **Context clipped at the file's edges**
  A fault on line 1 of a three-line file prints lines 1 to 3, and a fault on the last line of a file prints that line and the two before it.
- [ ] **Wrapped error with several problems**
  A configuration with two problems, whose error the application wraps with its own message, prints both reports, in order.
- [ ] **Non-configuration error**
  Printing an error that holds no configuration problem writes exactly its message followed by a newline.
- [ ] **One line versus detailed formatting**
  Formatting the configuration error in its ordinary form produces a single line, and asking for its detailed formatting produces the same text as printing it as plain text.
- [ ] **Detail wrapping**
  A problem whose detail is longer than 80 characters prints with no detail line over 80 columns by default, with no detail line over 40 columns when the width is 40, and on a single line when wrapping is off.
- [ ] **Resource and change summary**
  Printing a fixed set of resources with computed values included, and a fixed set of change summaries, as plain text, matches output captured from the current release's encoder and change-summary renderer for the same inputs.
- [ ] **Terminal colour**
  Printing the bad configuration's error with terminal colour writes colour codes, ends every coloured line with a reset before its newline, and with the colour codes removed is identical to the plain output.
- [ ] **HTML with classes**
  Printing the error as HTML writes a single preformatted block carrying the documented container class, in which the fault is marked with the error class and its category class, every other coloured part carries its kind's documented class, no inline styles appear, and text from the configuration is escaped.
- [ ] **HTML with inline styles**
  Printing the same error as inline-styled HTML writes the same structure with style attributes and no classes.
- [ ] **SVG**
  Printing a resource as SVG writes a well-formed SVG document that an SVG parser accepts, with one text line per source line and a view box wide enough for the longest line.
- [ ] **Stylesheet from a theme**
  Generating a stylesheet from the default theme produces CSS that a CSS parser accepts, in which every rule is scoped to the documented container class, and generating it from a theme that changes only attributes changes only the attribute rule.
- [ ] **Theme extension and category fallback**
  A theme that styles only comments colours line, hash and block comments with that style, and a theme that also styles line comments colours line comments with their own style; every kind not mentioned keeps the default style.
- [ ] **Invalid style words**
  A theme containing an unknown style word reports that word when validated, and printing with it still succeeds, ignoring the word.
- [ ] **VS Code theme**
  Loading a VS Code theme that colours properties orange and printing a resource with it colours attribute names orange.
- [ ] **Custom format**
  A user-written format that wraps each labelled part in brackets, passed to the error printing, receives the whole report at once and writes its output to the given destination.
- [ ] **Labelled text**
  Obtaining the labelled text of a resource returns parts that, joined together, equal its plain output, each labelled with a kind; the same holds for an error report and a change summary.
- [ ] **Colour chosen from the destination**
  Asking for a format chosen from a terminal destination gives terminal colour; from a file, or from a terminal with NO_COLOR set, gives plain text.
- [ ] **Report in a structured log**
  Logging the bad configuration's error through zap writes a log entry whose detailed error field contains the full plain report.
- [ ] **Reference outputs published**
  The repository holds reference outputs for every built-in format applied to a fixed set of inputs (an error report, a resource, a change summary) and a list of every HTML class name, and the Go formats' output matches them byte for byte in the test suite.
- [ ] **Release notes**
  The release notes list a one-line replacement for each removed highlighting, report and change-summary rendering function.
- [ ] **Examples print reports**
  Running each example program against a configuration with a mistake prints a report with the faulty source lines.
- [ ] **Website documents it**
  The xcl website has pages or sections covering printing errors, resources and change summaries, output formats, themes, the class names, stylesheet generation, custom formats and logging integration, each with a Go example, and no page still documents a removed function.

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

- Follow the precedents named in the design: `go/scanner.PrintError` and CUE's `errors.Print`/`Details` for writer-based error printing, HCL's diagnostic writer for colour as an explicit choice, and chroma's tokens/style/formatter separation.
- Keep TextMate scopes only as a per-kind mapping, for importing VS Code themes and for the xcl-vscode parity tests.
- Let encode options also act as print options so a resource prints in one call with both.
- Prefer leaving `events.SlogHandler` as it is; zap picks up the detailed report through its verbose error field once the error types implement detailed formatting.
- Wrap detail when printing rather than when laying the report out, so a laid-out report stays unwrapped for other languages.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->

## Success Metrics

- Showing a configuration problem with its source takes one call in Go, instead of finding the problems in the error, laying them out and rendering them as separate steps.
- No example program in the repository prints a configuration error as a single line.

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

- The Python implementation of tokens, themes, formatters and printing, which is a separate spec in this epic.
- A TypeScript implementation; the design defines it, but there is no TypeScript package yet.
- Making labelled text and imported themes available to other languages, which is a separate spec in this epic.
- Structured logging fields for configuration problems in Go's slog JSON output.
- Changing what an error report says, beyond restoring the surrounding lines and labelling its parts.

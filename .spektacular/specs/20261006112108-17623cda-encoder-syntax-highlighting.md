---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
epic: 20261006071139-7b266535-examples-and-output
sources:
    - uri: https://github.com/jumppad-labs/xcl/issues/6
      retrieved_date: "2026-10-06"
---

# Feature: 20261006112108-17623cda-encoder-syntax-highlighting

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Applications built on xcl can turn configuration back into text, but only as plain text. Anyone who wants it coloured, such as in a terminal or a log, has to write their own highlighter, as the logging example does today. This adds highlighting as an option when encoding, using the same token names and theme format as code editors like VS Code. Configuration then looks the same in an application's output as it does in the editor, and developers can plug in other output formats where they need them.

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

- [ ] **Highlighting is opt-in**
  Encoded configuration text is uncoloured unless the caller asks for highlighting.
- [ ] **Tokens use editor-standard names**
  Every token of highlighted text is labelled with the same standard token names the xcl editor extension uses.
- [ ] **Output formats are pluggable**
  Developers can supply their own renderer to turn labelled tokens into any output format.
- [ ] **Terminal colour ships built in**
  xcl ships a terminal-colour renderer.
- [ ] **Default theme follows the terminal**
  With no theme given, the terminal renderer uses only the terminal's standard 16 palette colours, so the result follows the user's terminal palette.
- [ ] **Editor themes work in the terminal**
  The terminal renderer accepts an editor colour theme and colours tokens as the editor would with that theme, including the bold, italic and underline styles the theme sets.
- [ ] **Unmatched tokens keep the default colour**
  A token that no rule in the theme matches is written in the terminal's default colour.
- [ ] **Bad themes are reported**
  A theme that cannot be read or is invalid is reported as an error when the renderer is created, rather than silently replaced with defaults.
- [ ] **All encoder output can be highlighted**
  Highlighting works on any text the encoder produces, including output that keeps references to other configuration as expressions rather than resolving them to values.
- [ ] **Highlighting never changes the text**
  Removing the colour from highlighted output gives exactly the uncoloured text.
- [ ] **The logging example uses it**
  The logging example uses the library's highlighting and no longer has its own highlighter.
- [ ] **Highlighting is documented**
  The documentation website explains how to turn on highlighting, use a theme and write a renderer.

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

- **No charmbracelet dependency in the library.** The library gains no dependency on the charmbracelet terminal-UI libraries the examples use, and the existing dependency-boundary test still passes.
- **Existing encoding is unchanged.** Existing encode calls and their output do not change; highlighting only adds to the public API.
- **Token names are TextMate scopes, the same set the xcl-vscode grammar assigns.**
- **Themes are read in the VS Code colour theme format.**
- **No terminal detection.** The library never checks whether output is a terminal; colouring is the caller's decision.
- **The project's testing conventions apply:** testify `require`, Mockery for mocks, no table-driven tests, never positive and negative cases in the same test function, and tests live with the code they test.

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

- [ ] **Plain by default**
  Encoding an entity without asking for highlighting gives text with no colour codes, identical to today's output.
- [ ] **Standard token names**
  For a configuration containing a token of each scope the xcl editor extension's grammar assigns, each token is labelled with the name that grammar gives the same text.
- [ ] **Custom renderer**
  A renderer written by a test, which wraps each token in markers naming its label, produces output in which every token is wrapped with the expected label.
- [ ] **Default colours**
  Highlighting with the built-in terminal renderer and no theme produces output that uses only the standard 16-colour foreground codes, and no 256-colour or 24-bit codes.
- [ ] **Theme colours**
  Highlighting with a reference editor theme colours every kind of token with the colour and font style that theme gives it, and a more specific rule in the theme overrides a general one.
- [ ] **Unmatched tokens**
  With a theme that has no rule for some kind of token, those tokens are written with no colour codes.
- [ ] **Invalid theme**
  Creating the terminal renderer from an unreadable or invalid theme returns an error.
- [ ] **References highlighted**
  Highlighting output that keeps references as expressions colours the first segment of each reference, as the editor grammar does.
- [ ] **Text unchanged**
  For every configuration in the repository's examples and test fixtures, stripping colour codes from the highlighted output gives the uncoloured output byte for byte.
- [ ] **Logging example migrated**
  The logging example's coloured output is produced by the library, and the example contains no highlighting code of its own.
- [ ] **Docs section**
  The documentation website's encoding page has a section showing how to enable highlighting, pass a theme, and write a renderer.

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

- Tokenise with the HCL syntax scanner already in xcl, rather than regular expressions, so any valid text is handled.
- Shape: an encode option takes a renderer; a renderer interface turns each (scope, text) pair into output; an ANSI renderer is built from a theme.
- Theme matching follows TextMate rules: scope-prefix selectors, with the most specific selector winning.
- The ANSI renderer writes 24-bit colour for theme colours, and basic 16-colour codes for the default theme.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- Every place xcl's examples show configuration in a terminal uses the library's highlighting, with no highlighting code of their own.

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

- **A built-in HTML renderer.** HTML is left to the presentation layer, such as the website's highlighter using the editor grammar.
- **Sharing a tokeniser with the planned `xcl fmt` tool (#5).** Deferred: tokeniser reuse will be considered when #5 is specified.


---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Plan: 20261010160411-19b965db-language-reference

<!-- Metadata -->
<!-- Created: 2026-10-11T15:02:21Z -->
<!-- Commit: 021b4ec4aaabdce2b0b6af19356647fdd7317d37 -->
<!-- Branch: f-libxcl -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

xcl.dev describes XCL feature by feature, but it has no reference for the language itself or for how configuration maps onto the types an application defines, and every code example is Go only. This plan adds an eight-page language reference that shows each shape of configuration beside its Go and Python types, and a reusable Go/Python tabbed code box that remembers the reader's choice across pages. It also puts Python next to Go on every existing page that shows application code, and links the reference from the navigation, the home page and the xcl README. It serves developers building XCL into Go or Python applications and the people who write configuration for them, and it documents the language as released in xcl v0.3.0 and v0.4.0.

## Conventions

- **Never write tests that inspect repository files; documentation is checked by human review** — every page, the README link and the CHANGELOG entry are verified by hand. No test reads the site's pages, and the examples are tried by hand against the release (spec non-goal).
- **Glossary: entity** — the reference calls every declared block an entity. "resource" appears only inside Go identifiers such as `types.ResourceBase`, as the spec's terminology rule requires.
- **Dependencies (prefer what exists, document any addition)** — the tab component is written in Astro with an inline script instead of adding a tabs package. No dependency is added to xcl-website.
- **Project structure (public packages at module top level)** — Go examples import the public packages (`github.com/jumppad-labs/xcl`, `registry`, `types`, `plugins`) and never `internal/`.
- **Website gotcha: removing a page breaks its URL (no redirects)** — no existing page is moved or renamed, and the new `/language/` slugs are chosen once and kept.

Not applied: the Go testing, mocking, shared-helper, errors-package, real-apply state, graph-ordering, provider Changed/Update, database and never-modify-dependency conventions. This work adds no Go code to the xcl library.

## Architecture & Design Decisions

**Options weighed.**

1. **A site-owned `CodeTabs` Astro component plus a `/language/` section of eight pages** (chosen). One component in `xcl-website/src/components/CodeTabs.astro` takes one fenced code block per language through named slots (`go`, `python`), so Expressive Code still renders each block (`astro.config.mjs:8-10`). A pre-paint script in `Head.astro` reads the stored choice and sets `data-lang` on `<html>`; CSS shows the matching panel in every box, so there is no flash and every box on the page follows one attribute. A click updates that attribute and the stored value. The reference is a section of pages under `/language/`, linked from a new "Language" dropdown in the nav. *Pros:* no new dependency; one source of truth for the language choice; the existing page structure, layout and nav are unchanged in shape; per-topic pages give stable section links. *Cons:* a small amount of hand-written client script and ARIA wiring to get right. *Effort:* Medium.
2. **Move the site onto Starlight to use its synced `<Tabs>`.** *Pros:* tabs, sidebar and search for free. *Cons:* restructures every page and the layout, which the spec's "fit the existing structure" constraint rules out, and the site has no redirects (`gotchas/no-redirects-on-page-removal`). *Effort:* High.
3. **A third-party Astro tabs package, with the reference as one long page.** *Pros:* less component code. *Cons:* an extra dependency that does not switch every box together or remember the choice across pages; one page holding eight topic areas would be longer than the longest page today (`examples/plugins.mdx`, 1853 lines) and harder to link to by section. *Effort:* Medium.

**Shape of the chosen solution.**

*Tabbed code (xcl-website).*
- `CodeTabs.astro` renders a tab list (Go, Python) and two panels. Each panel holds an ordinary fenced block, keeping titles such as `main.go` / `main.py`.
- The choice lives in `localStorage` under one key (`xcl-lang`, values `go` / `python`, default `go`). Reading and writing are wrapped so a browser that blocks storage still switches the page, just without remembering.
- `Head.astro` gets a tiny inline script that runs before paint. `Shell.astro` gets the click and keyboard handler, beside its existing inline scripts (`Shell.astro:28-70`). The WAI-ARIA tabs pattern is followed: `role="tablist"`, `aria-selected`, arrow keys.
- Plugin code stays as plain fenced Go blocks. Where an application capability has no Python form (a custom `Registry`, a custom masker, `EncodeEntity`, a custom highlight renderer), the Python panel holds a short "not yet available from Python" note naming what Python does offer.

*Python next to Go (xcl-website).*
- Every existing Go box that shows application code is wrapped in `CodeTabs` with a Python equivalent written against the python-support API (`@xcl.entity` dataclasses, `LocalRegistry`, `GitHubRegistry`, `xcl.Config`, `xcl.aio.Config`, `mask.*`).
- The plugin authoring pages are `plugin-template`, `examples/plugins`, `replacement` and `plugin-logging`. Their plugin code stays Go-only. Their boxes that show an application loading or using a plugin get Python tabs. Each page states that plugins are written in Go for now and can be used from Go and Python applications.

*The language reference (xcl-website), eight pages under `/language/`.*
- `/language/` is the overview: what XCL is, the files it reads, and the page list.
- `/language/entities/` covers files and blocks, labels (type, optional subtype, name) and addresses, using `database "postgres" "main"` and `app "web"`. It also covers the built-in fields (`meta`, `depends_on`, `disabled`).
- `/language/attributes-and-blocks/` covers each configuration shape: XCL beside its Go and Python type in `CodeTabs`. This puts the mapping alongside each feature, as the spec's Technical Approach asks.
- `/language/references/` covers references and processing order.
- `/language/expressions/` covers the expression forms, and states that `%{ }` directives are not part of XCL.
- `/language/variables-and-outputs/`, `/language/modules/` and `/language/functions/` cover the rest. The functions page has all 57 built-ins (`internal/functions/functions.go:430-486`), application functions in Go and Python, plugin functions, and clash errors.
- Wherever configuration meets application code (types, variables from the application, finding an entity inside a module, registering a function), the page uses `CodeTabs`.

*Reachability.*
- `Nav.astro` gains a "Language" dropdown between "Features" and "Examples", listing the reference pages with the overview first.
- `index.mdx` gains a "Learn the language" link in the "What is XCL?" section and a reference link in a new "Documentation" section placed after "Two examples". The sibling guides spec adds its two guide links to that same section, after the reference.
- The home page hero's version badge is updated to the release the reference is verified against (the release containing language-surface and python-support, expected v0.4.0), and its tagline says XCL is available for Go and Python.
- The xcl README's Documentation list gains "Language reference" as its first item.
- The xcl CHANGELOG gains one entry for this spec, in prose, with **Breaking:** None.

**Release gating and verification.** The pages describe the language as released in xcl v0.3.0 (language-surface) and v0.4.0 (python-support). Writing can start on the plans' stated behaviour. Every XCL, Go and Python example is then tried by hand against the tagged release that contains both, using throwaway programs outside the repositories. A feature that does not behave as described is marked "not yet supported", or is reported as a gap and not fixed here (spec non-goal). Module sources are named only after they have been tried: today the parser resolves local directories only (`docs/modules.md:120-126`), so remote sources are expected to be listed as not supported.

**Why this direction.** It is the only option that meets the spec's "one reusable tabbed component, choice stored in the browser" direction while keeping the site's structure, layout and dependency set. It also keeps every page's existing URL, which matters because the site has no redirects. Splitting the reference into topic pages gives each acceptance area its own linkable home. That serves the success metric that a configuration question can be answered with a link to a section. The evidence for each rejected option is in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

**xcl-website**

- **CodeTabs component** (new)
  - The one reusable tabbed code box, used across the whole site.
  - Owns the tab list (Go, Python) and one panel per language. Each panel holds an ordinary fenced code block, which the existing code renderer highlights and frames as usual.
  - Holds no state of its own: which panel shows is decided by the page-wide language attribute.
  - The only new component; justified because the site has no tab primitive and the spec asks for one.
- **Language choice script** (new, part of the layout)
  - Owns the reader's language: it reads the stored choice before the page paints, sets the page-wide attribute, and updates attribute and storage when a tab is chosen.
  - Every CodeTabs box on the page follows the attribute, so one choice switches them all, and the stored value carries to the next page.
  - It falls back to Go when nothing is stored or storage is blocked.
  - It lives beside the layout's existing copy-button and anchor scripts.
- **Site layout and head** (changed)
  - The head hosts the pre-paint part of the language script, and the shell hosts the click and keyboard handling.
  - Neither changes page structure.
- **Navigation** (changed)
  - Gains a "Language" dropdown between "Features" and "Examples" listing the reference pages, overview first.
  - The existing dropdowns are unchanged, apart from the sibling guides spec adding its two guides to "Guides".
- **Site links config** (changed)
  - Gains the xcl-python repository and PyPI project URLs, so pages link to them from one place.
- **Language reference pages** (new, eight pages under `/language/`)
  - Overview; entities (files, labels, addresses, built-in fields); attributes and blocks (each configuration shape beside its Go and Python type); references and order; expressions; variables and outputs; modules; functions.
  - They own the description of the language. They use CodeTabs wherever configuration meets application code, and plain XCL blocks elsewhere.
  - They link to the topic pages and guides for API walkthroughs, and do not repeat them.
- **Existing topic pages** (changed): registries, GitHub registry, events, sensitive values, state masking, configuration text, diffs, examples/configuration-only.
  - Each Go application-code box becomes a CodeTabs box with the Python equivalent.
  - A Go-only capability gets a Python panel that says it is not yet available from Python and names what Python offers instead.
  - No prose is reworked beyond what the Python version needs, and links to the reference are added where a topic touches the language.
- **Plugin authoring pages** (changed): plugin template, examples/plugins, replacement, plugin logging.
  - Plugin code stays Go-only.
  - Boxes showing an application loading or using a plugin become CodeTabs boxes.
  - Each page gains a short statement that plugins are written in Go for now and can be used from Go and Python applications.
- **Home page** (changed)
  - Its two Go boxes become CodeTabs boxes.
  - It gains a link to the reference in "What is XCL?" and a "Documentation" section after "Two examples". The reference is listed first there, and the sibling spec's guides join it after the reference.
  - Its hero badge shows the release the reference is verified against (expected v0.4.0), and its tagline says XCL is available for Go and Python.

**xcl**

- **README** (changed): the Documentation list gains "Language reference" as its first item.
- **CHANGELOG** (changed): one entry for this spec, in prose, with **Breaking:** None.

**Relationships**
- The reference pages, topic pages, plugin pages and home page all depend on CodeTabs and the language choice script.
- The reference describes behaviour delivered by the language-surface and python-support work. It is verified against the release that contains both before it is published.

## Data Structures & Interfaces

No library types or wire formats change. The contracts are between the site's components and the reader's browser.

**CodeTabs usage contract (MDX)**

````mdx
import CodeTabs from "../components/CodeTabs.astro";

<CodeTabs>
  <div slot="go">
    ```go title="main.go"
    local.RegisterType(&PostgreSQL{}, "database", "postgres")
    ```
  </div>
  <div slot="python">
    ```python title="main.py"
    local.register_type(Postgres)
    ```
  </div>
</CodeTabs>
````

- The two slots are named after the languages, `go` and `python`. Both are required, and Go comes first.
- The component takes no props. Each panel's own fence title (`main.go`, `main.py`) is the file label.

**Language choice (browser)**

- Storage key `xcl-lang` in `localStorage`. Its value is `"go"` or `"python"`; any other value or no value means `"go"`.
- The page-wide attribute is `data-lang` on `<html>`, with the same values. CSS shows a panel only when its language equals the attribute.
- Choosing a tab sets the attribute and stores the value. Reads and writes are guarded, so blocked storage leaves the page working, just not remembered.

**Rendered tab markup (accessibility contract)**

- A `role="tablist"` holds two `role="tab"` buttons carrying `data-lang-choice="go|python"` and `aria-selected`.
- Each panel is `role="tabpanel"` with `data-lang-panel="go|python"`.
- Arrow keys move between the tabs. The selected state follows the page-wide attribute.

**Reference page set and slugs (fixed once; the site has no redirects)**

| Slug | Page |
|---|---|
| `/language/` | Language reference (overview, files) |
| `/language/entities/` | Entities: blocks, labels, addresses, built-in fields |
| `/language/attributes-and-blocks/` | Attributes and blocks, with Go and Python types |
| `/language/references/` | References and processing order |
| `/language/expressions/` | Expressions |
| `/language/variables-and-outputs/` | Variables and outputs |
| `/language/modules/` | Modules |
| `/language/functions/` | Functions |

**Navigation item**

The `Language` entry in the nav's items array has the children above, in that order.

**Python "not yet available" panel**

A Python panel for a Go-only capability holds a short prose note instead of a code fence. The note names the capability, says it is not yet available from Python, and links the xcl-python parity list.

## Implementation Detail

**A new site-wide pattern: one language attribute, many boxes.**
- The site gains its first piece of shared client state. Before, each page's script only wired up local widgets such as copy buttons.
- The pattern is CSS-driven. The page has one attribute, and every tabbed box shows the panel that matches it.
- A box therefore never needs to find the other boxes, and a box added to a page later joins the switching automatically.
- The script that sets the attribute runs before first paint, so a Python reader never sees Go flash into place.

**Pages stay ordinary MDX.**
- An author writes a tabbed box as two fenced blocks inside one component, the same fences they already write.
- Highlighting, titles, wrapping and line numbers all come from the existing code renderer, unchanged.
- Untabbed boxes (XCL, shell, plugin code) stay plain fences. A reader of the MDX can see at a glance which examples are application code: those are the tabbed ones.

**The reference is a section, not a page.**
- The site's pages are flat today, apart from `examples/`. The reference introduces a second directory, `language/`, following that same `examples/` precedent.
- Each page opens with a short statement of what it covers, then one heading per topic. Headings are worded so their anchors make useful links, for example `#named-blocks-collected-by-name`.
- Every page ends with links to the next page and to the related topic pages.
- Each page puts the configuration first. Where application code is involved, a tabbed box follows, showing the Go type with its struct tags beside the Python dataclass.

**Python examples mirror the Go examples one for one.**
- Each Python panel does what its Go panel does, with the same names, entity types and values, so a reader switching tabs sees the same program.
- Python follows the python-support API: dataclasses with `@xcl.entity`, `snake_case` methods, `xcl.Config` as a context manager. It uses the synchronous API, except on pages about events or long operations, where the asyncio form is mentioned and the guide is linked for detail.
- A Go capability with no Python form gets a "not yet available from Python" panel. The plan never invents an API.

**Writing from the plans, verifying against the release.**
- Pages are drafted from the language-surface and python-support plans' stated behaviour, so writing can run alongside that work.
- Before anything is published, every example is tried with throwaway Go and Python programs against the tagged release that contains both (v0.4.0, installed from the Go module proxy and PyPI). Those programs are not committed.
- Any behaviour that differs is changed in the page to what the release does, marked "not yet supported", or reported as a gap. Library bugs are not fixed here.

**Existing patterns followed.** The work keeps the existing patterns:
- the nav dropdown data array;
- the shared site links config;
- the `Shell` layout's inline scripts;
- the README's Documentation list;
- the CHANGELOG's per-spec entry.

## Dependencies

- **Design documents this plan was built on: none.** The spec references no design document.
- **Spec `20261011095738-fad05e41-language-surface` and its final plan — must land and be released (xcl v0.3.0) before the reference is verified and published.** It provides the language the reference describes:
  - `XCL_VAR_` overrides;
  - named blocks collected into maps, and optional labels in lists;
  - references inside `for` expressions;
  - optional fields in object values;
  - template directives rejected;
  - application functions (`RegisterFunction`) and plugin functions;
  - function name clash errors.

  Its own website task fixes the `HCL_VAR_` and "around sixty" text on existing pages. This plan builds on those corrected pages and does not repeat that work.
- **Spec `20261011130709-aa3eb933-python-support` and its final plan — must land and be released (xcl v0.4.0, `xcl` on PyPI) before Python examples are verified and published.** It provides the Python API every Python panel uses:
  - `@xcl.entity` dataclasses, `xcl.Resource`, `xcl.field`, `Sensitive` and `Computed`;
  - `LocalRegistry` and `GitHubRegistry`;
  - `xcl.Config` and `xcl.aio.Config`;
  - the `mask` constructors and the exception classes.

  It also provides the parity list (`PARITY.md` in xcl-python) that "not yet available from Python" notes link to.
- **Sibling spec `20261011084948-c995ed25-using-xcl-with-go` (planned in parallel, not a blocker).** It adds the Go and Python guides. Both plans edit the nav, the home page and the xcl README's Documentation list, so the placement rules must be reconciled: this plan adds the "Language" dropdown, the home page "Documentation" section and the README's first item, and the guides join them. The reference links to the guides for API walkthroughs once they exist. Until then it links to the existing topic pages.
- **xcl-website stack (Astro 5, `@astrojs/mdx` 4, `astro-expressive-code` 0.43, Tailwind CSS 4).** Used as is. No new package, and no version change.
- **The xcl git history (`bdba483^:README.md`).** The removed Struct Tags, References, Variables, Local, Modules and Functions sections are raw material only. Parts are stale (`HCL_VAR_`, `local`, GitHub module sources, "Custom Functions" via parser options).
- **Go toolchain and Python 3.10+ on the verifier's machine.** These are for the throwaway programs that try each example against the release. Nothing is committed.
- **Prior plans consulted as history:** the two dependency plans above. No other prior plan bears on the website structure this changes.

## Testing Approach

**Kinds of check**

- **Build checks (automated, on every task that touches the site).** The site's type check (components and MDX) and its production build must pass. The build fails on malformed MDX, an unknown component or a bad slot, which catches most breakage of the tab component's use. No new test tooling is added to xcl-website: it has none today, and by convention website content is checked by human review, never by tests that read pages.
- **Example verification against the release (by hand, using throwaway programs).** Every XCL, Go and Python example on the new reference pages, and every new Python panel on the existing pages, is tried against the tagged xcl release that contains both dependency specs. The Go side uses the Go module, and the Python side uses the `xcl` package from PyPI at the same version. Each "not yet supported" or "not yet available from Python" claim is also checked to be really unsupported. The programs are not committed. The task records which examples were tried and any gap found. Gaps are reported, not fixed (spec non-goal).
- **Link checks (by hand on the built site).** Every new nav item, home page link and README link opens the right page. Every in-reference link and anchor resolves.
- **No automated tests for the examples** (spec non-goal), and no tests reading README, CHANGELOG or page text (convention).

**Most effort** goes to example verification on the reference pages. "Matches the current release" is the acceptance criterion most likely to fail silently: the pages are drafted from plans, before the code exists.

**What the checks guarantee**
- The site builds, and every tabbed box has a Go and a Python panel.
- Every example in the reference behaves as the page says in the published release.
- The terminology rules hold on the reference pages.

**Deliberate gaps**
- No browser test automates tab switching or persistence. These are manual checks below.
- No visual regression testing of the restyled code boxes.
- The xcl-vscode highlighter is unchanged (spec non-goal), so XCL blocks keep their current highlighting.

**Success metrics**
- *Every language topic removed from the README (struct tags, references, variables, locals, modules, functions) has a home on xcl.dev.* **Manual — captured in the implementation test plan.** Map each removed topic to its reference page. Locals map to the variables page, which explains that a variable's default can be built from other values.
- *Questions about writing configuration or mapping it to a Go or Python type can be answered with a link to a section of the reference.* **Manual — captured in the implementation test plan.** Check that every acceptance area has its own heading anchor on a reference page.
- *A Python developer never meets a Go-only code example outside plugin authoring.* **Manual — captured in the implementation test plan.** On the built site, check every page outside the four plugin authoring pages and the two per-language guides (`/using-xcl-with-go/`, `/using-xcl-with-python/`, single-language by their spec) for a Go box without a Python tab.

**Manual reviews**
- **Manual — captured in the implementation test plan:** on the built site, picking Python in one box switches every box on the page, and other pages then open on Python. Reload, keyboard switching, and a browser with storage blocked are checked too.
- **Manual — captured in the implementation test plan:** the language reference is in the main nav and linked from the home page, and the xcl README links to it. Every one of those links opens the reference.
- **Manual — captured in the implementation test plan:** each acceptance area of the spec is covered on the reference pages:
  - files and blocks, with addresses;
  - the type mapping for seven shapes, in Go with tags and in Python;
  - meta;
  - references and order;
  - variables and outputs, with no locals;
  - modules, including which sources work;
  - every built-in function with an example, plus app functions in Go and Python and a plugin function;
  - the six expression forms and the `%{ }` statement.
- **Manual — captured in the implementation test plan:** a terminology review of the reference pages. Check there is no "superset", "interpolation" or "Terraform", and no "resource" for a block outside Go identifiers.
- **Manual — captured in the implementation test plan:** the four plugin authoring pages state that plugins are written in Go for now and can be used from Go and Python applications.
- **Manual — captured in the implementation test plan:** review the xcl CHANGELOG entry for this spec (prose, then **Breaking:** None).

## Milestones & Tasks

### Milestone 1: Every code example on xcl.dev speaks Go and Python

**What changes**
- Code boxes that show application code get Go and Python tabs.
- Choosing Python in one box switches every box on the page, and the choice is remembered on the next page.
- Every existing page that shows Go application code also shows the Python version, written against the released Python package. Where Python has no equivalent yet, its tab says so.
- The plugin authoring pages keep their plugin code in Go, and say plainly that plugins are written in Go for now and work from Go and Python applications.

**Validation point**: The site's type check and production build pass. On the built site, every page outside the four plugin authoring pages and the two per-language guides (`/using-xcl-with-go/`, `/using-xcl-with-python/`) shows a Python tab on each Go box. Picking Python switches the page and carries to the next page.

#### - [x] Task: Tabbed Go and Python code boxes
**Id:** 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

This adds the one reusable code box with Go and Python tabs that every page will use. The reader's choice is held as a single page-wide setting that is applied before the page paints, so every box on the page shows the same language. The choice is stored in the browser and carried to the next page. Tabs work with a mouse and with the keyboard. If the browser blocks storage, switching still works on the page.

*Technical detail:* [context.md#task-tabbed-go-and-python-code-boxes](./context.md#task-tabbed-go-and-python-code-boxes)

**Acceptance criteria**:
- [x] A page can show one example as a Go tab and a Python tab, each with its usual highlighting and file title.
- [x] Choosing Python in one box shows Python in every box on the page.
- [x] After choosing Python, another page opens with its boxes on Python, with no visible flash of Go.
- [x] With no stored choice, or with storage blocked, boxes show Go and switching still works on the page.
- [x] The tabs can be reached and switched with the keyboard, and screen readers announce them as tabs.
- [x] Untabbed code boxes look and behave as before.

#### - [x] Task: Python tabs on the registry, GitHub registry and events pages
**Id:** 39c9a656-fece-4c02-9178-8f34281d1e06
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

These three pages show how an application declares types, loads plugins, installs them from GitHub and receives events, all in Go only. Each application-code box becomes a tabbed box with the Python equivalent, written against the Python package's registries, config options and event handler. Writing your own registry has no Python form yet, so its Python tab says so and names the registries Python offers.

*Technical detail:* [context.md#task-python-tabs-on-the-registry-github-registry-and-events-pages](./context.md#task-python-tabs-on-the-registry-github-registry-and-events-pages)

**Acceptance criteria**:
- [x] Every Go box on the registries, GitHub registry and events pages has a Python tab.
- [x] Each Python tab does what its Go tab does, using the same names and values.
- [x] Capabilities Python lacks, such as a custom registry, have a Python tab that says so and points to what Python offers.
- [x] The pages' prose changes only where the Python version needs it.

#### - [x] Task: Python tabs on the sensitive values, state masking, configuration text and diff pages
**Id:** 9d760ac9-8252-4b65-9857-8fa070b1389f
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

These pages show sensitive fields, state encryption, turning saved data back into configuration text, and diffs, all in Go only. Each application-code box gains a Python tab using the package's sensitive type, mask constructors, saved-entity encoding and diff. The Go features with no Python form get a Python tab that says so and names the nearest Python equivalent:
- writing your own masker;
- converting a live entity to text;
- custom highlight renderers.

*Technical detail:* [context.md#task-python-tabs-on-the-sensitive-values-state-masking-configuration-text-and-diff-pages](./context.md#task-python-tabs-on-the-sensitive-values-state-masking-configuration-text-and-diff-pages)

**Acceptance criteria**:
- [x] Every Go box on the sensitive values, state masking, configuration text and diff pages has a Python tab.
- [x] Python tabs show `Sensitive[...]` with `reveal()`, state encryption with a mask, saved-entity text and diff reading as the Python package does them.
- [x] Each Go-only capability's Python tab says it is not yet available from Python and names what Python offers.

#### - [x] Task: Python tabs on the home page and the configuration-only example
**Id:** 8dfc7c3e-5f65-44e8-8120-77fdcea3a716
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

The home page introduces XCL with a Go type and a Go program. The configuration-only example walks through a Go program that reads a deployment into Go types. Each box gains a Python tab: the type as an `@xcl.entity` dataclass, and the program using the synchronous Python API. The home page's program uses a plugin loaded as a separate program in the Python tab, because Python cannot host a Go plugin in process.

*Technical detail:* [context.md#task-python-tabs-on-the-home-page-and-the-configuration-only-example](./context.md#task-python-tabs-on-the-home-page-and-the-configuration-only-example)

**Acceptance criteria**:
- [x] Both Go boxes on the home page have Python tabs showing the same entity type and program.
- [x] Every Go box on the configuration-only example page has a Python tab, including the tests, shown as pytest functions.
- [x] The Python dataclasses use the same block types, field names, optional fields, sensitive and computed fields as their Go structs.

#### - [x] Task: Plugin authoring pages say plugins are Go and tab the application code
**Id:** d60e0167-82d9-4e00-81a8-158e5bc54469
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

The plugin template, plugin example, replacement and plugin logging pages are about writing plugins, and plugins can only be written in Go today. Each page gains a short note saying that plugins are written in Go for now and can be used from both Go and Python applications. Plugin code stays Go-only. Boxes that show an application loading or using a plugin get Python tabs, because a Python application loads the same plugin.

*Technical detail:* [context.md#task-plugin-authoring-pages-say-plugins-are-go-and-tab-the-application-code](./context.md#task-plugin-authoring-pages-say-plugins-are-go-and-tab-the-application-code)

**Acceptance criteria**:
- [x] The plugin template, plugin example, replacement and plugin logging pages each state that plugins are written in Go for now and can be used from Go and Python applications.
- [x] Plugin code on these pages is unchanged and has no Python tab.
- [x] Boxes that show an application loading a plugin, from a path, a directory or GitHub, have a Python tab.
- [x] An application box whose Go form has no Python equivalent, such as registering a plugin in process, has a Python tab that says so.

### Milestone 2: XCL has a language reference

**What changes**
- xcl.dev gains a language reference: eight pages that describe XCL itself, with the matching Go and Python types beside each shape of configuration. They cover:
  - files, entities and addresses;
  - attributes and blocks;
  - references and processing order;
  - expressions;
  - variables and outputs;
  - modules;
  - every built-in function, plus application and plugin functions.
- The pages describe the language as released, and mark anything not yet supported.

**Validation point**: The site's type check and production build pass. Each of the eight `/language/` pages renders, and each spec acceptance area has its own heading on one of them. The pages contain no "superset", "interpolation" or "Terraform".

#### - [x] Task: Reference overview, entities, and attributes and blocks pages
**Id:** 315c65be-3c7d-4010-8fc0-f08fd81bab16
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

This starts the reference with three pages.
- **Overview:** what XCL is and which files it reads.
- **Entities:**
  - how `database "postgres" "main"` and `app "web"` give an entity's type, optional subtype, name and address;
  - the built-in fields every entity has, and how configuration refers to them.
- **Attributes and blocks:** each shape of configuration beside the Go struct and the Python dataclass that receive it. The shapes are:
  - a required attribute and an optional attribute;
  - a nested block and a list of blocks;
  - named blocks collected by name;
  - a map attribute;
  - a computed field.

*Technical detail:* [context.md#task-reference-overview-entities-and-attributes-and-blocks-pages](./context.md#task-reference-overview-entities-and-attributes-and-blocks-pages)

**Acceptance criteria**:
- [x] The overview states that XCL reads `.xcl` files (and `.vars` files for variable values) and lists the reference pages.
- [x] The entities page shows `database "postgres" "main"` and `app "web"`, names the type, subtype and name labels in each, says the subtype is optional, and gives the addresses `database.postgres.main` and `app.web`.
- [x] The entities page lists each built-in field of an entity and shows configuration referring to one on another entity.
- [x] For each of the seven shapes, the attributes and blocks page shows an XCL snippet beside a tabbed Go type (with struct tags) and Python type. The map attribute is described as supported but not the encouraged style.
- [x] The pages call blocks entities, use "resource" only inside Go identifiers, and never say "superset", "interpolation" or "Terraform".

#### - [x] Task: References and expressions pages
**Id:** ef9c18fa-f532-4b8e-8114-9d89a07ff6b8
**Repo:** xcl-website
**Depends on:**
- 315c65be-3c7d-4010-8fc0-f08fd81bab16 — Reference overview, entities, and attributes and blocks pages
**Execution:** agent

The references page shows one entity using another's values, including a whole entity and a named block by name. It explains that references, and `depends_on`, decide the order entities are processed in, with a referenced entity always processed first. The expressions page has an example of each form configuration can use:
- a string with an embedded reference;
- lists and maps;
- conditionals;
- `for` expressions;
- splat.

It states that `%{ }` template directives are not part of XCL, and that templating is done with functions.

*Technical detail:* [context.md#task-references-and-expressions-pages](./context.md#task-references-and-expressions-pages)

**Acceptance criteria**:
- [x] The references page shows one entity using a value from another, and states that the referenced entity is processed first.
- [x] The references page explains `depends_on` and that a cycle is reported as an error.
- [x] The expressions page has an example for each of: a string with an embedded reference, a list, a map, a conditional, a `for` expression and a splat.
- [x] The expressions page states that `%{ }` directives are not supported and points to the functions page for templating.
- [x] Neither page says "interpolation".

#### - [x] Task: Variables and outputs, and modules pages
**Id:** 62c9bfc3-e7d0-461f-b1f6-3d4315b0f899
**Repo:** xcl-website
**Depends on:**
- 315c65be-3c7d-4010-8fc0-f08fd81bab16 — Reference overview, entities, and attributes and blocks pages
**Execution:** agent

The variables and outputs page covers:
- declaring variables and giving them defaults;
- building a default from other values;
- overriding a variable from the application (tabbed Go and Python), from `.vars` files, and with `XCL_VAR_<name>` environment variables, in their order of precedence;
- declaring and reading outputs.

The modules page covers:
- using a module, passing it values and reading its outputs;
- the rule that configuration reaches a module only through its outputs, while application code can find any entity inside it by its full address (tabbed Go and Python);
- which module sources work today and which do not.

*Technical detail:* [context.md#task-variables-and-outputs-and-modules-pages](./context.md#task-variables-and-outputs-and-modules-pages)

**Acceptance criteria**:
- [x] The variables page shows a variable with a default, and a variable whose default is built from another value.
- [x] It shows each way to override a variable: from the application in Go and Python, and with an `XCL_VAR_<name>` environment variable.
- [x] It shows an output being declared and used, and does not describe locals.
- [x] The modules page shows a module used with an input and an output read from it in configuration.
- [x] It shows application code finding an entity inside the module by its full address, in Go and Python.
- [x] It states that configuration reaches a module only through its outputs, and names the module sources that work today and those that do not.

#### - [x] Task: Functions page
**Id:** 48e85d3d-4c21-45fe-8828-d7ac70a51265
**Repo:** xcl-website
**Depends on:**
- 315c65be-3c7d-4010-8fc0-f08fd81bab16 — Reference overview, entities, and attributes and blocks pages
**Execution:** agent

The functions page lists every built-in function, grouped by kind, each with a one-line description and an example. It explains:
- that function calls are evaluated while configuration is read, so plugins only see the results;
- how an application registers its own function, in tabbed Go and Python;
- how a plugin offers functions and how configuration calls one;
- how a name used twice is reported, and when.

*Technical detail:* [context.md#task-functions-page](./context.md#task-functions-page)

**Acceptance criteria**:
- [x] Every built-in function in the release appears with an example.
- [x] The page shows an application registering a function in Go and in Python, and configuration calling it.
- [x] The page shows a plugin-provided function being called from configuration, and links to the plugin template for writing one.
- [x] The page states that function calls are evaluated while configuration is read.
- [x] It explains how name clashes are reported: when the config is created for application functions, and when plugins load for plugin functions.

### Milestone 3: The reference is reachable and matches the release

**What changes**
- Readers reach the reference from a new "Language" menu in the site navigation and from the home page, and the xcl README points to it.
- Every example in the reference, and every new Python example across the site, has been tried against the tagged xcl release that contains the language and Python work. The pages now say exactly what that release does.
- The xcl changelog records the reference.

**Validation point**: The nav, home page and README links open the reference. The verification record lists every example as tried, with each difference resolved in the page or reported as a gap. The site's production build passes. The manual checks in the Testing Approach are carried into the implementation test plan.

#### - [x] Task: Link the reference from the navigation and the home page
**Id:** 852e2985-5a4e-4448-8315-58f2c7a261be
**Repo:** xcl-website
**Depends on:**
- 315c65be-3c7d-4010-8fc0-f08fd81bab16 — Reference overview, entities, and attributes and blocks pages
- ef9c18fa-f532-4b8e-8114-9d89a07ff6b8 — References and expressions pages
- 62c9bfc3-e7d0-461f-b1f6-3d4315b0f899 — Variables and outputs, and modules pages
- 48e85d3d-4c21-45fe-8828-d7ac70a51265 — Functions page
**Execution:** agent

The site navigation gains a "Language" menu between "Features" and "Examples", listing the reference pages with the overview first. The home page gets two links to the reference:
- a "Learn the language" link in its introduction;
- a new "Documentation" section after the examples, listing the reference first; the Go and Python guides will follow it there.

The home page's feature cards for variables, modules and functions link to their reference pages. The hero's version badge is updated to the release the reference is verified against (the release containing language-surface and python-support, expected v0.4.0), and the hero tagline now says XCL is available for Go and Python.

*Technical detail:* [context.md#task-link-the-reference-from-the-navigation-and-the-home-page](./context.md#task-link-the-reference-from-the-navigation-and-the-home-page)

**Acceptance criteria**:
- [x] The main navigation has a "Language" menu listing all eight reference pages, and each item opens its page.
- [x] The home page links to the reference from its introduction and from a "Documentation" section.
- [x] The variables, modules and functions feature cards link to their reference pages.
- [x] The home page hero's version badge shows the release the reference is verified against (expected v0.4.0), and its tagline says XCL is available for Go and Python.
- [x] No existing page is moved or renamed.

#### - [x] Task: Point the xcl README and changelog at the reference
**Id:** a3314b5e-b571-417c-9b31-1317a4a710b1
**Repo:** xcl
**Depends on:**
- 852e2985-5a4e-4448-8315-58f2c7a261be — Link the reference from the navigation and the home page
**Execution:** agent

The xcl README stays an overview and sends readers to xcl.dev for documentation. Its Documentation list gains the language reference as its first item. The changelog gets one entry for this spec describing the reference and the Go and Python tabs across the site, with no breaking changes.

*Technical detail:* [context.md#task-point-the-xcl-readme-and-changelog-at-the-reference](./context.md#task-point-the-xcl-readme-and-changelog-at-the-reference)

**Acceptance criteria**:
- [x] The README's Documentation list starts with a link to the language reference on xcl.dev.
- [x] The changelog has a `## 20261010160411-19b965db-language-reference` entry in prose, followed by **Breaking:** None.

#### - [x] Task: Try every example against the release and correct the pages
**Id:** 0b1d93be-ee1f-4e83-b454-056decf10f38
**Repo:** xcl-website
**Depends on:**
- 39c9a656-fece-4c02-9178-8f34281d1e06 — Python tabs on the registry, GitHub registry and events pages
- 9d760ac9-8252-4b65-9857-8fa070b1389f — Python tabs on the sensitive values, state masking, configuration text and diff pages
- 8dfc7c3e-5f65-44e8-8120-77fdcea3a716 — Python tabs on the home page and the configuration-only example
- d60e0167-82d9-4e00-81a8-158e5bc54469 — Plugin authoring pages say plugins are Go and tab the application code
- ef9c18fa-f532-4b8e-8114-9d89a07ff6b8 — References and expressions pages
- 62c9bfc3-e7d0-461f-b1f6-3d4315b0f899 — Variables and outputs, and modules pages
- 48e85d3d-4c21-45fe-8828-d7ac70a51265 — Functions page
**Execution:** agent

The pages were written from the plans for the language and Python work. This task tries every example against the tagged xcl release that contains both, using throwaway Go and Python programs that are not committed. It covers every XCL, Go and Python example in the reference and every new Python tab across the site. Wherever the release behaves differently, the page is corrected or the feature is marked not yet supported. Library bugs are reported as gaps, not fixed. Every "not yet supported" claim is also checked to be really unsupported.

*Technical detail:* [context.md#task-try-every-example-against-the-release-and-correct-the-pages](./context.md#task-try-every-example-against-the-release-and-correct-the-pages)

**Acceptance criteria**:
- [x] Every XCL, Go and Python example in the reference, and every new Python tab on the site, behaves as its page says in the release that contains the language and Python work.
- [x] Each feature the pages mark as not yet supported, or not yet available from Python, is indeed unavailable in that release.
- [x] The module sources named as working have each been tried and work, and those named as not working fail.
- [x] The built-in function list matches the release exactly.
- [x] Any gap or bug found is listed in the task's record for reporting, and is not fixed in the library.

## Open Questions

- **Do fenced code blocks inside named slots render through Expressive Code?** This depends on how Astro's MDX integration passes slotted children once Expressive Code has transformed them, which only shows when the component is first used. If they do not render, the component takes the code as props and renders it with Expressive Code's own `<Code>` component. The usage contract changes, but the behaviour does not. Do not stop for this.
- **Does the released Python package match the python-support plan's names?** Examples: the exact `xcl.field` label form, exception class names, and the `Event.entity()` and diff attribute names. This depends on the v0.4.0 release. The verification task makes the pages follow the release. Stop and ask only if a capability the reference's acceptance criteria require is missing from Python entirely, such as registering a function or finding by address. In that case the "Type mapping covered" or "Functions covered" criterion could not be met.
- **Does the release behave as the dependency plans say?** Examples: `XCL_VAR_` precedence, named-block maps, `for` references, directive rejection, and clash timing. This only shows when the examples are tried. A difference is written up as the release's actual behaviour or marked not yet supported, and reported as a gap (spec non-goal: not fixed here).

No other questions remain open. Placement, slugs, page split and the plugin-authoring page set are settled in the assumption log.

## Out of Scope

- No automated tests for the examples on the new pages, and no browser tests of the tabs. These are checked by hand (spec non-goal; website convention).
- No changes to the xcl-vscode highlighter. Formatting support and publishing the extension are separate, later work (spec non-goal).
- No fixes to the library. Gaps or bugs found while writing or verifying the reference are reported, not fixed (spec non-goal). They belong to follow-ups on the language-surface or python-support work.
- Existing pages change only to add Python next to Go, to add the plugin-authoring statement, to add links to and from the new pages, and to update the home page hero's badge and tagline for the Go and Python release. There is no rewrite of their prose (spec non-goal).
- No "Using XCL with Go" or "Using XCL with Python" guides. They are owned by the sibling spec `20261011084948-c995ed25-using-xcl-with-go`. This plan only provides the home page "Documentation" section they join.
- No Python plugin authoring docs. Plugins are Go only until Python plugins exist (spec constraint; the later Python-providers work in this epic).
- No Python equivalents for Go-only capabilities: a custom registry, a custom masker, `EncodeEntity`, custom highlight renderers, or in-process plugin registration. Their tabs say "not yet available from Python".
- No Python example programs added to the xcl repository. Python tabs on example pages are illustrative.
- No search, sidebar or site restructure (Starlight or similar). No page is moved or renamed, because the site has no redirects.
- No API renames: Go examples keep `types.ResourceBase` and other current names (spec constraint).
- No description of locals. The reference covers building a variable's default from other values instead.

## Changelog

### 2026-10-11 — Task: Tabbed Go and Python code boxes

**What was done**: Added `CodeTabs.astro`, a tabbed code box with `go` and `python` named slots that keeps each fenced block rendering through Expressive Code with its title. A pre-paint inline script in `Head.astro` reads the `xcl-lang` localStorage key and sets `data-lang` on `<html>`, the component's CSS shows the panel matching that attribute in every box, and a handler in `Shell.astro` switches the attribute, stores the choice, syncs `aria-selected`/`tabindex`, handles ArrowLeft/ArrowRight/Home/End, and keeps the clicked box in place on screen.

**Deviations**: None in behaviour. Astro requires `<slot name>` to be a static string, so the two panels are written out rather than mapped. No test files were written: the plan routes tab behaviour to manual checks, and xcl-website has no test tooling. The browser criteria were nonetheless exercised with a throwaway headless-Chrome self-test page (deleted) covering click switching, second-visit persistence, CSP-sandboxed storage and keyboard navigation.

**Files changed**:
- `xcl-website: src/components/CodeTabs.astro`
- `xcl-website: src/components/Head.astro`
- `xcl-website: src/layouts/Shell.astro`

**Discoveries**: Fenced code blocks inside `<div slot="go">` do render through Expressive Code (the plan's open question), provided MDX gets a blank line after the opening `<div>` and before `</div>`. Inline `<script>` bodies with braces cannot live in an `.mdx` page because MDX parses `{}` as expressions; put page scripts in components or external files. Selected-tab styling is driven by `html[data-lang]` so the right tab is lit before any handler runs.

### 2026-10-11 — Task: Python tabs on the registry, GitHub registry and events pages

**What was done**: Every Go application-code box on the registries (9), GitHub registry (6) and events (6) pages is now a `CodeTabs` box with a Python panel written against the released `xcl-lang` package: `xcl.LocalRegistry` / `xcl.GitHubRegistry`, `@xcl.entity` dataclasses, `xcl.Config(registries=…, on_event=…, event_data=…, event_mask=…, event_buffer_size=…)`, the matching `xcl.*Error` exceptions, and a `logging`-based event handler plus a standard-library `prettylog.py`. The two "Writing your own registry" boxes carry a "not yet available from Python" note linking the parity list. `site.ts` gained `pythonRepoURL`, `pythonParityURL` and `pypiURL`.

**Deviations**: Go fences are unchanged; prose changed only by one sentence per box naming the Python form. The GitHub-registry "pinning a version" Python panel does not state what Python sees for an invalid version: Go's `RegisterPlugin` panics and the core service has no `recover`, so the verification task must establish it. No sentence was added about a raising Python event handler because the package source shows the exception propagating out of the operation rather than being reported as its error.

**Files changed**:
- `xcl-website: src/pages/registries.mdx`
- `xcl-website: src/pages/github-registry.mdx`
- `xcl-website: src/pages/events.mdx`
- `xcl-website: src/config/site.ts`

**Discoveries**: In MDX a Markdown link cannot take a JSX expression (`[x]({url})` renders a literal `%7Burl%7D`); use `<a href={url}>`. Plugin install failures reach Python as `xcl.PluginNotFoundError` / `xcl.PluginVerificationError` with `details` `repository`, `version` and `platform` (`coreservice/errors.go` maps `PluginInstallError` before `PluginLoadError`).

### 2026-10-11 — Task: Python tabs on the sensitive values, state masking, configuration text and diff pages

**What was done**: Every Go application-code box on the sensitive values (3), state masking (2), configuration text (9) and diff (4) pages is now a `CodeTabs` box with a Python panel: `xcl.Sensitive[T]` dataclass fields, `xcl.ConfigError.problems`, `state_mask=xcl.mask.aes256gcm(key)`, `cfg.encode_saved_entity(event.data, highlight=…)`, `event.entity()`, and `cfg.diff(…, reveal_sensitive=…)` with `Diff.summary`, `resources`, `changes` and `after_value`. Seven Go-only capabilities (a custom masker, `EncodeEntity`, `IncludeComputed`, `ShowReferences`, editor themes, a custom renderer, diff rendering/colour) carry a "not yet available from Python" note naming what Python offers and linking the parity list.

**Deviations**: None in scope. The sensitive-values page's Python panel states truthfully that a `Sensitive` read back through a lookup prints `(sensitive)` and `reveal()` returns `None` in Python, because real values never leave the engine through a lookup; it does not mirror Go's `Reveal()` returning the value.

**Files changed**:
- `xcl-website: src/pages/sensitive-values.mdx`
- `xcl-website: src/pages/state-masking.mdx`
- `xcl-website: src/pages/configuration-text.mdx`
- `xcl-website: src/pages/diff.mdx`

**Discoveries**: Python has no text rendering of a diff and no live-entity encoding; `Diff.raw` holds the JSON document. A looked-up `xcl.Sensitive` is always redacted (`is_redacted` is `True`), which the reference's sensitive-fields section must also say.

### 2026-10-11 — Task: Python tabs on the home page and the configuration-only example

**What was done**: The home page's two Go boxes (the `PostgreSQL` type and the program that applies and finds it) and the configuration-only example's seven Go boxes (the types, `main.go`, `routes.go` and the two tests) are now `CodeTabs` boxes with Python panels: `@xcl.entity` dataclasses with the same block types, optional, sensitive and computed fields; a `main.py` that fills an `AppConfig` with one `cfg.all(...)` per type; `routes.py` in plain Python over `meta.id`; and `test_routes.py` as pytest functions. The home page's Python program loads the plugin as an external binary with a comment saying Python loads Go plugins as separate programs. One sentence on the example page says the Python tabs are shown for comparison and are not in the repository.

**Deviations**: None in scope. Go's single `Decode` call has no Python form, so the Python `main.py` says so in one sentence and uses `all` per type. The hero badge and tagline were left as they are for the later home-page task, as planned.

**Files changed**:
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/examples/configuration-only.mdx`

**Discoveries**: A Python dataclass that names a later class in an annotation needs `from __future__ import annotations`; the package resolves the hints with `typing.get_type_hints` (`xcl/_schema.py`), so forward references in the Go field order work.

### 2026-10-11 — Task: Plugin authoring pages say plugins are Go and tab the application code

**What was done**: The plugin template, plugin example, replacement and plugin logging pages each open with a statement that plugins are written in Go for now and work from both Go and Python applications. Plugin, provider and test code stays Go-only with no tabs. The application boxes gained Python panels: loading a plugin from a path, a directory or GitHub on the template and logging pages (3 and 2 boxes), and the twelve `xcl-docker` program boxes on the plugin example (`main.py`, `plan.py`, `status.py`, `inspect.py`) using `register_external_plugin`, `xcl.Config(state_path=…, on_event=…, event_data="processed")`, `apply`, `diff`, `load`, `find_by_type(None, …)`, `find(None, …)`, `destroy` and `xcl.PluginLoadError`. In-process registration, rendered diff text and live-entity encoding carry "not yet available from Python" notes with the parity link.

**Deviations**: None. The replacement page gets the statement only, as all nine of its boxes are provider code.

**Files changed**:
- `xcl-website: src/pages/plugin-template.mdx`
- `xcl-website: src/pages/plugin-logging.mdx`
- `xcl-website: src/pages/replacement.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`

**Discoveries**: Entities of a plugin's types come back to Python as `xcl.Entity` (a read-only mapping with `.meta` and `.address`) when `None` is passed as the class, which is how the status and inspect commands read Docker entities.

### 2026-10-11 — Task: Reference overview, entities, and attributes and blocks pages

**What was done**: Started the language reference under `/language/` with three pages. The overview says what XCL is, which files it reads (`.xcl` files per directory, `.vars` files beside them, modules for other directories) and lists the seven topic pages. The entities page covers blocks and labels with `database "postgres" "main"` and `app "web"`, addresses, the built-in fields (`depends_on`, `disabled`, `meta.*`) with configuration reading `meta.id` and `meta.name` on other entities, and the built-in entity types, with CodeTabs boxes for type registration and the entity base. The attributes and blocks page walks one `app "web"` example through required and optional attributes, nested blocks, lists of blocks, named blocks collected by name (with the two error texts), map attributes (supported, named blocks encouraged), object values, computed fields, sensitive fields and the entity base, each as XCL beside a tabbed Go struct with tags and a Python dataclass.

**Deviations**: `xcl.Find[*App]` in the entities page was corrected to `xcl.Find[App]` (`Find[T]` takes the struct type). The Python shared-fields example declares the subclass `kw_only=True`, which is needed by dataclasses when a required field follows a defaulted one; that the package accepts it is left to the verification task.

**Files changed**:
- `xcl-website: src/pages/language/index.mdx`
- `xcl-website: src/pages/language/entities.mdx`
- `xcl-website: src/pages/language/attributes-and-blocks.mdx`

**Discoveries**: `Find[T any](c, address) (*T, error)` returns a pointer, so the type parameter is the struct type; an existing Go fence on `configuration-text.mdx` uses `xcl.Find[*Postgres]` and is reported as a gap, not changed.

### 2026-10-11 — Task: References and expressions pages

**What was done**: The references page shows one entity using another's attribute, a whole entity and a list of entities as values (with a tabbed Go struct and Python dataclass receiving them), a named block referenced by name, processing order (the referenced entity is always processed first, independent entities run concurrently, destroy runs in reverse, a cycle is an error), `depends_on`, and the module boundary in one paragraph. The expressions page gives an example for a string with an embedded `${ }` reference, lists, maps, conditionals, both `for` forms, splat, indexing and attribute access, operators, and templating: `%{ }` directives are rejected with the library's message, and the same results are shown with a conditional inside `${ }`, `format` and `join`, pointing to `template_file` on the functions page.

**Deviations**: None. The writer ran every XCL example on both pages through the Python package against a locally built engine, and the Go receiving types through a scratch Go module; nothing was committed. One fact was established by that probe and stated on the page: in Python a registered entity class cannot be the type of a field that receives a whole entity, so a plain dataclass with `xcl.field(block=False)` is used.

**Files changed**:
- `xcl-website: src/pages/language/references.mdx`
- `xcl-website: src/pages/language/expressions.mdx`

**Discoveries**: Registering a Python class that uses an `xcl.Resource` subclass as a `block=False` field fails with "no cty.Type for interface {}" (from `Meta.extra`); a plain dataclass works. The exact wording of the cycle error is not asserted by any test, so the page does not quote one.

### 2026-10-11 — Task: Variables and outputs, and modules pages

**What was done**: The variables and outputs page covers declaring variables, a default built from another value (`variable "url" { default = "http://${app.web.host}" }`, the home of derived values, without naming locals), setting variables from the application (tabbed `xcl.WithVariables` / `variables=`), `.vars` files, `XCL_VAR_<name>` environment variables, their precedence, outputs with string, list and map values, and reading outputs from the application (tabbed `c.Outputs()` and `xcl.Find[types.Output]` / `cfg.outputs()`). The modules page covers using a module with `source` and `variables`, reading `module.db.output.port`, the module boundary with the library's error text verbatim and the re-export pattern, finding an entity inside a module by its full address (tabbed `xcl.Find[PostgreSQL]` / `cfg.find(PostgreSQL, …)`), and module sources: a local directory only, with git, GitHub and HTTP sources stated as not yet supported and `version` as accepted but unused.

**Deviations**: None. The Go `Find` examples use the struct type (`xcl.Find[PostgreSQL]`), as `Find[T]` returns `*T`.

**Files changed**:
- `xcl-website: src/pages/language/variables-and-outputs.mdx`
- `xcl-website: src/pages/language/modules.mdx`

**Discoveries**: There is no Go option to name a `.vars` file; only discovery beside each root path. The derived-default example and the hypothetical `app` / `database "postgres"` types the pages use must be declared by the verification task when it tries them.

### 2026-10-11 — Task: Functions page

**What was done**: The functions page explains that calls are evaluated while configuration is read, lists all 57 built-ins grouped as strings, numbers, collections, encoding, dates, and files and environment, each with a one-line description and an example in a runnable `output` block with its result as a comment, shows an application registering `slugify` in tabbed Go (`local.RegisterFunction`) and Python (`local.register_function`) with the supported signatures, shows a plugin-provided function called from configuration with a link to the plugin template, and explains name clashes: application clashes reported when the configuration is created, plugin clashes when plugins load on the first operation, with a tabbed `errors.As` / `except xcl.FunctionNameClashError` box.

**Deviations**: None. The built-in list was diffed against the map in `internal/functions/functions.go` and matches exactly (57).

**Files changed**:
- `xcl-website: src/pages/language/functions.mdx`

**Discoveries**: xcl vendors go-cty under `internal/cty`, and three built-ins differ from cty's stdlib: `trim(str)` takes one argument and trims whitespace, `element` also accepts a map key, and `len` works on strings as well as collections. Results shown as comments for set functions, `csvdecode` and `regex` groups follow the source and are for the verification task to confirm.

### 2026-10-11 — Task: Link the reference from the navigation and the home page

**What was done**: The site navigation gained a "Language" dropdown between "Features" and "Examples" listing the eight reference pages, overview first. The home page's "What is XCL?" section ends with a sentence linking the reference and a "Learn the language →" button; a new "Documentation" section after "Two examples" lists the language reference first, leaving room for the two guides; the "Variables and outputs", "Modules" and "Functions" feature cards link to their reference pages; the hero badge now reads "v0.4.0 · Go and Python" and the tagline ends "Available for Go and Python."

**Deviations**: None. No page was moved or renamed.

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/index.mdx`

**Discoveries**: None.

### 2026-10-11 — Task: Point the xcl README and changelog at the reference

**What was done**: The xcl README's Documentation list now starts with "Language reference" linking https://xcl.dev/language/ and naming what it covers, with Go and Python types. The CHANGELOG gained a `## 20261010160411-19b965db-language-reference` entry above the python-support entry, in prose: the eight-page reference, the Go and Python tabs across the site with the choice remembered across pages, the "not yet available from Python" notes, and the plugin-authoring statement, followed by **Breaking:** None.

**Deviations**: None.

**Files changed**:
- `xcl: README.md`
- `xcl: CHANGELOG.md`

**Discoveries**: None.

### 2026-10-11 — Task: Try every example against the release and correct the pages

**What was done**: Every XCL and Go example on the eight reference pages was tried with throwaway Go programs against `github.com/jumppad-labs/xcl v0.4.0` from the Go module proxy (no `replace`), one scratch module per page group, running `Validate`, `Apply` and the lookups each page shows. Every Python panel on the site (the reference pages and the twelve topic and example pages) was tried against the same release's engine built locally from the tagged commit (`libxcl` from the xcl worktree at 93e1904, loaded through `XCL_LIBRARY_PATH`) with the xcl-python checkout, because `xcl-lang` 0.4.0 is not on PyPI yet. The Docker example plugin v0.2.0 release served the plugin-dependent snippets. Nothing was committed from the scratch directories.

Go and XCL results: every example behaves as its page says, with these corrections made to the pages: the overview now says a path with another extension is read as empty rather than rejected; the duplicate-name error on the attributes page is quoted with its real separator and range (`Duplicate listener name: A listener named "http" was already defined at <file>:4,3-18`); the references page says a reference cycle is not caught by `Validate` and fails at `Apply` with a dependency graph error; the functions page sorts the three set functions' results into lists and says a set published as it is through an `output` is not yet supported; the variables page no longer claims a root output can be read as `output.<name>` from another block, pointing to a derived variable default instead. The built-in list matches the release exactly (57). Module sources: a local relative directory works; `github.com/...` and `git::` sources fail; `version` is accepted and unused, as the page says. The module boundary message matches verbatim.

Python results: every Python panel on the reference pages, the registries, GitHub registry and events pages, and the sensitive values, state masking, configuration text and diff pages behaves as its page says, and every "not yet available from Python" note names something the package really lacks. One correction: the GitHub registry page's "pinning a version" Python panel now says that a bad repository or version aborts the process when `xcl.Config` is created, because the engine's panic is not recovered, rather than raising a Python exception. The home page, configuration-only example, plugin example, plugin template and plugin logging pages were tried last: the home program, the configuration-only program and its two pytest tests, and the `xcl-docker` program's apply, plan, status, inspect and destroy commands all ran, the Docker-backed ones against a real Docker engine, and the plugin registries loaded the Docker plugin from a path, a directory and the GitHub registry with the site's release key. One correction: the plugin example's `inspect.py` is now `inspect_entity.py`, with a sentence saying why, because a module named `inspect` beside `main.py` shadows the standard library's `inspect`, which `dataclasses` imports, so `import xcl` fails with a circular import.

**Deviations**: Python examples were checked against the tagged source rather than the PyPI wheel, by decision of the orchestrator, because the xcl-python release run failed; the pages keep `pip install xcl-lang`. The session running the last Python pass (home page, examples, plugin pages) was cut off before it reported, so a later session re-ran that pass from its scratch programs against the current page text. Docker was available, so the Docker-backed examples ran for real rather than stopping at `validate`.

**Files changed**:
- `xcl-website: src/pages/language/index.mdx`
- `xcl-website: src/pages/language/attributes-and-blocks.mdx`
- `xcl-website: src/pages/language/references.mdx`
- `xcl-website: src/pages/language/functions.mdx`
- `xcl-website: src/pages/language/variables-and-outputs.mdx`
- `xcl-website: src/pages/github-registry.mdx`
- `xcl-website: src/pages/examples/plugins.mdx`
- `xcl-website: src/pages/examples/configuration-only.mdx` (touched by the pass that was cut off; its panels were re-verified as they stand)

**Discoveries**: Library gaps found and not fixed here, for upstream: (1) `Validate` accepts a reference cycle; only `Apply` fails, with the raw error `unable to validate dependency graph: multiple roots: []dag.Vertex{...}` that names no entity. (2) A set-typed value published through an `output` reads back as its cty `GoString` (`internal/parser/util.go convertCtyToGo` has no set case); typed `[]string` fields decode sets correctly. (3) A root-level `output.<name>` reference from another block passes `Validate` but fails at `Apply` with `There is no variable named "output"`; only `module.<name>.output.<name>` works from the parent. (4) An entity type whose struct has no fields of its own (only `types.ResourceBase`) is never added to the evaluation context, so references to its `meta` fail at `Apply`. (5) `EncodeEntity` prints an unnamed list-block element as `volume "" { … }`. (6) `xcl-lang` 0.4.0 is not published: the xcl-python release run failed, and PyPI publication waits on an out-of-process engine for musl wheels. (7) The pre-existing Go fence on `configuration-text.mdx` uses `xcl.Find[*Postgres]`, but `Find[T]` takes the struct type.
Python package gaps for upstream: (8) a bad GitHub plugin repository or version aborts the Python process (SIGABRT) when `xcl.Config` is created, since `coreservice.buildGitHubRegistry` calls the panicking `RegisterPlugin` with no `recover`; (9) environment variables set from Python at runtime are invisible to the in-process engine, which sees the environment the process started with; (10) an omitted object attribute declared `default=None` hydrates as the zero object, and an omitted `xcl.Sensitive[str] | None` field as a redacted `Sensitive`, not `None`; (11) a registered entity class used as a field type is rejected only when the `Config` is created (`InvalidTypeError`), and a non-dataclass base between `xcl.Resource` and the registered class silently drops its fields; (12) `TypeNameClashError.details` names the synthesised Go struct rather than the Python class, and `PluginNotFoundError` / `PluginVerificationError` are not subclasses of `PluginLoadError` in Python. (13) A program module named `inspect.py` beside one that imports `xcl` breaks the import by shadowing the standard library; this is a Python fact rather than a package gap, and the page names the file differently. The Python panels were verified against the engine built from tag v0.4.0 (commit 93e1904) and the xcl-python checkout at 81673e5, not a PyPI wheel.

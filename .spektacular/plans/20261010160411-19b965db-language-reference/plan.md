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

#### - [ ] Task: Tabbed Go and Python code boxes
**Id:** 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

This adds the one reusable code box with Go and Python tabs that every page will use. The reader's choice is held as a single page-wide setting that is applied before the page paints, so every box on the page shows the same language. The choice is stored in the browser and carried to the next page. Tabs work with a mouse and with the keyboard. If the browser blocks storage, switching still works on the page.

*Technical detail:* [context.md#task-tabbed-go-and-python-code-boxes](./context.md#task-tabbed-go-and-python-code-boxes)

**Acceptance criteria**:
- [ ] A page can show one example as a Go tab and a Python tab, each with its usual highlighting and file title.
- [ ] Choosing Python in one box shows Python in every box on the page.
- [ ] After choosing Python, another page opens with its boxes on Python, with no visible flash of Go.
- [ ] With no stored choice, or with storage blocked, boxes show Go and switching still works on the page.
- [ ] The tabs can be reached and switched with the keyboard, and screen readers announce them as tabs.
- [ ] Untabbed code boxes look and behave as before.

#### - [ ] Task: Python tabs on the registry, GitHub registry and events pages
**Id:** 39c9a656-fece-4c02-9178-8f34281d1e06
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

These three pages show how an application declares types, loads plugins, installs them from GitHub and receives events, all in Go only. Each application-code box becomes a tabbed box with the Python equivalent, written against the Python package's registries, config options and event handler. Writing your own registry has no Python form yet, so its Python tab says so and names the registries Python offers.

*Technical detail:* [context.md#task-python-tabs-on-the-registry-github-registry-and-events-pages](./context.md#task-python-tabs-on-the-registry-github-registry-and-events-pages)

**Acceptance criteria**:
- [ ] Every Go box on the registries, GitHub registry and events pages has a Python tab.
- [ ] Each Python tab does what its Go tab does, using the same names and values.
- [ ] Capabilities Python lacks, such as a custom registry, have a Python tab that says so and points to what Python offers.
- [ ] The pages' prose changes only where the Python version needs it.

#### - [ ] Task: Python tabs on the sensitive values, state masking, configuration text and diff pages
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
- [ ] Every Go box on the sensitive values, state masking, configuration text and diff pages has a Python tab.
- [ ] Python tabs show `Sensitive[...]` with `reveal()`, state encryption with a mask, saved-entity text and diff reading as the Python package does them.
- [ ] Each Go-only capability's Python tab says it is not yet available from Python and names what Python offers.

#### - [ ] Task: Python tabs on the home page and the configuration-only example
**Id:** 8dfc7c3e-5f65-44e8-8120-77fdcea3a716
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

The home page introduces XCL with a Go type and a Go program. The configuration-only example walks through a Go program that reads a deployment into Go types. Each box gains a Python tab: the type as an `@xcl.entity` dataclass, and the program using the synchronous Python API. The home page's program uses a plugin loaded as a separate program in the Python tab, because Python cannot host a Go plugin in process.

*Technical detail:* [context.md#task-python-tabs-on-the-home-page-and-the-configuration-only-example](./context.md#task-python-tabs-on-the-home-page-and-the-configuration-only-example)

**Acceptance criteria**:
- [ ] Both Go boxes on the home page have Python tabs showing the same entity type and program.
- [ ] Every Go box on the configuration-only example page has a Python tab, including the tests, shown as pytest functions.
- [ ] The Python dataclasses use the same block types, field names, optional fields, sensitive and computed fields as their Go structs.

#### - [ ] Task: Plugin authoring pages say plugins are Go and tab the application code
**Id:** d60e0167-82d9-4e00-81a8-158e5bc54469
**Repo:** xcl-website
**Depends on:**
- 0d3d8554-a31b-4984-9ddb-d0f0b9b09b8a — Tabbed Go and Python code boxes
**Execution:** agent

The plugin template, plugin example, replacement and plugin logging pages are about writing plugins, and plugins can only be written in Go today. Each page gains a short note saying that plugins are written in Go for now and can be used from both Go and Python applications. Plugin code stays Go-only. Boxes that show an application loading or using a plugin get Python tabs, because a Python application loads the same plugin.

*Technical detail:* [context.md#task-plugin-authoring-pages-say-plugins-are-go-and-tab-the-application-code](./context.md#task-plugin-authoring-pages-say-plugins-are-go-and-tab-the-application-code)

**Acceptance criteria**:
- [ ] The plugin template, plugin example, replacement and plugin logging pages each state that plugins are written in Go for now and can be used from Go and Python applications.
- [ ] Plugin code on these pages is unchanged and has no Python tab.
- [ ] Boxes that show an application loading a plugin, from a path, a directory or GitHub, have a Python tab.
- [ ] An application box whose Go form has no Python equivalent, such as registering a plugin in process, has a Python tab that says so.

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

#### - [ ] Task: Reference overview, entities, and attributes and blocks pages
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
- [ ] The overview states that XCL reads `.xcl` files (and `.vars` files for variable values) and lists the reference pages.
- [ ] The entities page shows `database "postgres" "main"` and `app "web"`, names the type, subtype and name labels in each, says the subtype is optional, and gives the addresses `database.postgres.main` and `app.web`.
- [ ] The entities page lists each built-in field of an entity and shows configuration referring to one on another entity.
- [ ] For each of the seven shapes, the attributes and blocks page shows an XCL snippet beside a tabbed Go type (with struct tags) and Python type. The map attribute is described as supported but not the encouraged style.
- [ ] The pages call blocks entities, use "resource" only inside Go identifiers, and never say "superset", "interpolation" or "Terraform".

#### - [ ] Task: References and expressions pages
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
- [ ] The references page shows one entity using a value from another, and states that the referenced entity is processed first.
- [ ] The references page explains `depends_on` and that a cycle is reported as an error.
- [ ] The expressions page has an example for each of: a string with an embedded reference, a list, a map, a conditional, a `for` expression and a splat.
- [ ] The expressions page states that `%{ }` directives are not supported and points to the functions page for templating.
- [ ] Neither page says "interpolation".

#### - [ ] Task: Variables and outputs, and modules pages
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
- [ ] The variables page shows a variable with a default, and a variable whose default is built from another value.
- [ ] It shows each way to override a variable: from the application in Go and Python, and with an `XCL_VAR_<name>` environment variable.
- [ ] It shows an output being declared and used, and does not describe locals.
- [ ] The modules page shows a module used with an input and an output read from it in configuration.
- [ ] It shows application code finding an entity inside the module by its full address, in Go and Python.
- [ ] It states that configuration reaches a module only through its outputs, and names the module sources that work today and those that do not.

#### - [ ] Task: Functions page
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
- [ ] Every built-in function in the release appears with an example.
- [ ] The page shows an application registering a function in Go and in Python, and configuration calling it.
- [ ] The page shows a plugin-provided function being called from configuration, and links to the plugin template for writing one.
- [ ] The page states that function calls are evaluated while configuration is read.
- [ ] It explains how name clashes are reported: when the config is created for application functions, and when plugins load for plugin functions.

### Milestone 3: The reference is reachable and matches the release

**What changes**
- Readers reach the reference from a new "Language" menu in the site navigation and from the home page, and the xcl README points to it.
- Every example in the reference, and every new Python example across the site, has been tried against the tagged xcl release that contains the language and Python work. The pages now say exactly what that release does.
- The xcl changelog records the reference.

**Validation point**: The nav, home page and README links open the reference. The verification record lists every example as tried, with each difference resolved in the page or reported as a gap. The site's production build passes. The manual checks in the Testing Approach are carried into the implementation test plan.

#### - [ ] Task: Link the reference from the navigation and the home page
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
- [ ] The main navigation has a "Language" menu listing all eight reference pages, and each item opens its page.
- [ ] The home page links to the reference from its introduction and from a "Documentation" section.
- [ ] The variables, modules and functions feature cards link to their reference pages.
- [ ] The home page hero's version badge shows the release the reference is verified against (expected v0.4.0), and its tagline says XCL is available for Go and Python.
- [ ] No existing page is moved or renamed.

#### - [ ] Task: Point the xcl README and changelog at the reference
**Id:** a3314b5e-b571-417c-9b31-1317a4a710b1
**Repo:** xcl
**Depends on:**
- 852e2985-5a4e-4448-8315-58f2c7a261be — Link the reference from the navigation and the home page
**Execution:** agent

The xcl README stays an overview and sends readers to xcl.dev for documentation. Its Documentation list gains the language reference as its first item. The changelog gets one entry for this spec describing the reference and the Go and Python tabs across the site, with no breaking changes.

*Technical detail:* [context.md#task-point-the-xcl-readme-and-changelog-at-the-reference](./context.md#task-point-the-xcl-readme-and-changelog-at-the-reference)

**Acceptance criteria**:
- [ ] The README's Documentation list starts with a link to the language reference on xcl.dev.
- [ ] The changelog has a `## 20261010160411-19b965db-language-reference` entry in prose, followed by **Breaking:** None.

#### - [ ] Task: Try every example against the release and correct the pages
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
- [ ] Every XCL, Go and Python example in the reference, and every new Python tab on the site, behaves as its page says in the release that contains the language and Python work.
- [ ] Each feature the pages mark as not yet supported, or not yet available from Python, is indeed unavailable in that release.
- [ ] The module sources named as working have each been tried and work, and those named as not working fail.
- [ ] The built-in function list matches the release exactly.
- [ ] Any gap or bug found is listed in the task's record for reporting, and is not fixed in the library.

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

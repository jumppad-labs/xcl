---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Test plan: 20261010160411-19b965db-language-reference

Manual checks for the success metrics and reviews the plan's Testing Approach routed to a person. The automated gates (`npx astro check` and `npm run build` in xcl-website) ran green at the end of every site task and are not repeated here. Every example on the reference pages and every Python panel across the site was tried against xcl v0.4.0 during the last task; the results are in the plan's changelog entry for "Try every example against the release and correct the pages".

## Setup

1. In the xcl-website checkout run `npm ci`, then `npm run build` and `npm run preview`. The site is served at `http://localhost:4321/`.
2. Use a browser with developer tools; one check needs a browser or a private window that blocks `localStorage`.
3. For the README and changelog reviews open `README.md` and `CHANGELOG.md` in the xcl checkout.

## Success metrics

### 1. Every language topic removed from the README has a home on xcl.dev

**What to check**: each topic the xcl README once covered (struct tags, references, variables, locals, modules, functions) is described on a reference page.

**How**: open each page below and confirm it covers the topic.

| Removed README topic | Reference page | Section |
|---|---|---|
| Struct tags | `/language/attributes-and-blocks/` | Required attributes, Optional attributes, Nested blocks, Lists of blocks, Named blocks collected by name, Map attributes, Object values, Computed fields, Sensitive fields, The entity base (each with a tabbed Go struct carrying `xcl:"..."` tags and a Python dataclass) |
| References | `/language/references/` | Referring to another entity, Referring to a named block, Processing order, depends_on, Across modules |
| Variables | `/language/variables-and-outputs/` | Declaring variables, Setting variables from the application, .vars files, Environment variables, Precedence |
| Locals | `/language/variables-and-outputs/` | Defaults built from other values (a variable whose default is built from another value; the page does not describe a `local` block, by design) |
| Modules | `/language/modules/` | Using a module, Reading a module's outputs, The module boundary, Finding entities inside a module from the application, Module sources |
| Functions | `/language/functions/` | When functions run, Built-in functions (Strings, Numbers, Collections, Encoding, Dates, Files and environment), Functions from your application, Functions from plugins, Name clashes |

**Pass**: every row's section exists on the named page and describes the topic.

**Who / when**: the documentation reviewer, before the site is published.

### 2. Configuration questions can be answered with a link to a section

**What to check**: each acceptance area of the spec has its own heading anchor on one reference page, so a question can be answered with a URL.

**How**: open each URL and confirm the browser scrolls to a heading of that name.

- Files and blocks, with addresses: `/language/#files`, `/language/entities/#blocks-and-labels`, `/language/entities/#addresses`
- The type mapping for the seven shapes, Go with tags and Python: `/language/attributes-and-blocks/#required-attributes`, `#optional-attributes`, `#nested-blocks`, `#lists-of-blocks`, `#named-blocks-collected-by-name`, `#map-attributes`, `#computed-fields` (plus `#object-values`, `#sensitive-fields`, `#the-entity-base`)
- Meta and the built-in fields: `/language/entities/#built-in-fields`
- References and order: `/language/references/#referring-to-another-entity`, `#processing-order`, `#depends_on`
- Variables and outputs, no locals: `/language/variables-and-outputs/#declaring-variables`, `#defaults-built-from-other-values`, `#precedence`, `#outputs`
- Modules, including which sources work: `/language/modules/#using-a-module`, `#the-module-boundary`, `#module-sources`
- Every built-in function, app functions in Go and Python, a plugin function: `/language/functions/#built-in-functions`, `#functions-from-your-application`, `#functions-from-plugins`, `#name-clashes`
- The six expression forms and the `%{ }` statement: `/language/expressions/#strings-with-references`, `#lists`, `#maps`, `#conditionals`, `#for-expressions`, `#splat`, `#templating`

**Pass**: every anchor resolves to a heading on its page. The heading slugs are generated from the headings listed in each page's table of headings (`grep '^## ' src/pages/language/*.mdx`); if a slug differs, the heading still exists and the link is corrected in this plan.

**Who / when**: the documentation reviewer, before the site is published.

### 3. A Python developer never meets a Go-only code example outside plugin authoring

**What to check**: on every page outside the four plugin authoring pages (`/plugin-template/`, `/examples/plugins/`, `/replacement/`, `/plugin-logging/`) and the two per-language guides (`/using-xcl-with-go/`, `/using-xcl-with-python/`, single-language by their own spec), every Go code box has a Python tab.

**How**: with the site built, run from the xcl-website checkout:

```
for f in dist/**/index.html dist/index.html; do
  case "$f" in *plugin-template*|*examples/plugins*|*replacement*|*plugin-logging*|*using-xcl-with-*) continue;; esac
  go=$(grep -o 'data-language="go"' "$f" | wc -l)
  tabs=$(grep -o 'data-lang-panel="go"' "$f" | wc -l)
  echo "$f go-boxes=$go tabbed=$tabs"
done
```

Then open each page listed with `go-boxes` greater than `tabbed` and look at the untabbed Go box by hand.

**Pass**: on every page outside the exemptions, each Go box is inside a tabbed box (the two counts match, or every untabbed Go box is plugin code shown on a page not in the exemption list, which would be a defect to fix).

**Who / when**: the documentation reviewer, before the site is published, and whenever a page gains a Go box.

## Manual reviews

### A. Tab switching and persistence

**Where**: `/` (home page), then `/registries/`, `/language/attributes-and-blocks/`.

**What to do**:
1. Open `/` in a fresh browser profile. Every tabbed box shows Go.
2. Click "Python" on the first box. Every tabbed box on the page now shows Python, and each box's Python tab is highlighted with `aria-selected="true"`.
3. Open `/registries/`. Every box opens on Python with no visible flash of Go. Reload the page: still Python.
4. Focus a "Go" tab with the keyboard and press ArrowRight, ArrowLeft, Home and End. The selection moves between the two tabs and the panels follow.
5. In developer tools, `localStorage.getItem("xcl-lang")` is `"python"`. Remove it and reload: boxes show Go.
6. In a private window, or with site data blocked for `localhost`, open `/language/attributes-and-blocks/`. Boxes show Go; clicking "Python" still switches every box on the page; the next page opens on Go (the choice is not remembered, by design).
7. With a screen reader or the accessibility tree in developer tools, confirm each box exposes a `tablist` with two `tab` elements and two `tabpanel` elements.

**Pass**: every step behaves as written.

**Who / when**: the documentation reviewer, before publishing, and after any change to `src/components/CodeTabs.astro`, `src/components/Head.astro` or `src/layouts/Shell.astro`.

### B. The reference is reachable

**Where**: the site navigation, `/`, and the xcl `README.md`.

**What to do**:
1. The nav has a "Language" menu between "Features" and "Examples" with eight items in this order: Language reference, Entities, Attributes and blocks, References, Expressions, Variables and outputs, Modules, Functions. Each opens its page under `/language/`.
2. On `/`, the "What is XCL?" section ends with a sentence linking the language reference and a "Learn the language →" button; both open `/language/`.
3. On `/`, a "Documentation" section follows "Two examples" and lists the language reference first; the "Variables and outputs", "Modules" and "Functions" feature cards link to `/language/variables-and-outputs/`, `/language/modules/` and `/language/functions/`.
4. The hero badge reads "v0.4.0 · Go and Python" and the tagline ends "Available for Go and Python."
5. In the xcl README, the Documentation list's first item is "Language reference" linking `https://xcl.dev/language/`.
6. On each reference page, every link in its text and its "Next" section opens the page or anchor it names.

**Pass**: every link opens the right page; nothing 404s.

**Who / when**: the documentation reviewer, before publishing.

### C. Acceptance areas covered on the reference pages

**Where**: the eight pages under `/language/`.

**What to do**: read each page against the spec's acceptance areas and tick each:
- files and blocks, with addresses (`/language/`, `/language/entities/`: `database "postgres" "main"` and `app "web"`, addresses `database.postgres.main` and `app.web`, subtype optional);
- the type mapping for the seven shapes, in Go with tags and in Python (`/language/attributes-and-blocks/`, one tabbed box per shape; map attributes described as supported but not the encouraged style);
- meta and the other built-in fields, with configuration reading one on another entity (`/language/entities/#built-in-fields`);
- references and processing order, `depends_on`, cycles reported as an error at `Apply` (`/language/references/`);
- variables and outputs, with no locals (`/language/variables-and-outputs/`: defaults, a default built from another value, overrides from the application in Go and Python, `.vars` files, `XCL_VAR_<name>`, precedence, outputs);
- modules, including which sources work (`/language/modules/`: a local directory works; git, GitHub and HTTP sources are stated as not yet supported; `version` accepted and unused);
- every built-in function with an example (57 on `/language/functions/`, matching `internal/functions/functions.go`), an application function in Go and Python, a plugin function, clash reporting;
- the six expression forms and the `%{ }` statement (`/language/expressions/`: string with a reference, list, map, conditional, `for`, splat; `%{ }` directives rejected, templating through functions).

**Pass**: every area has its section and example.

**Who / when**: the documentation reviewer, before publishing.

### D. Terminology review of the reference pages

**Where**: `src/pages/language/*.mdx`.

**What to do**: run from the xcl-website checkout:

```
grep -niE 'superset|interpolat|terraform' src/pages/language/*.mdx
grep -n 'resource' src/pages/language/*.mdx
```

**Pass**: the first command prints nothing. Every match of the second is inside a Go identifier or code (`types.ResourceBase`, `ResourceBase`, `xcl.Resource`, `resource_id`, `resources`) and never names a block in prose; blocks are called entities.

**Who / when**: the documentation reviewer, before publishing, and on every later edit of a reference page.

### E. Plugin authoring pages state that plugins are Go

**Where**: `/plugin-template/`, `/examples/plugins/`, `/replacement/`, `/plugin-logging/`.

**What to do**: near the top of each page, find the sentence beginning "Plugins are written in Go for now" and confirm it says the plugin works from both Go and Python applications. Confirm plugin and provider code on these pages is a plain Go box with no Python tab, and that boxes showing an application loading or using a plugin are tabbed.

**Pass**: all four pages carry the statement; plugin code is Go-only; application boxes are tabbed.

**Who / when**: the documentation reviewer, before publishing.

### F. CHANGELOG entry for this spec

**Where**: `CHANGELOG.md` in the xcl checkout.

**What to do**: find the `## 20261010160411-19b965db-language-reference` heading. Its body is prose describing the eight-page reference, the Go and Python tabs with the remembered choice, the "not yet available from Python" notes and the plugin-authoring statement, and it ends with `**Breaking:** None`.

**Pass**: the entry exists in that form, above the python-support entry.

**Who / when**: the maintainer cutting the next xcl release.

## Gaps reported, not fixed

The verification task found library and package behaviour that the pages now describe as it is; each is listed under Discoveries in the plan changelog's "Try every example against the release and correct the pages" entry, for follow-up outside this spec. The one that affects a reader directly: `xcl-lang` 0.4.0 is not yet on PyPI, so the `pip install xcl-lang` lines across the site do not work until the first published Python release.

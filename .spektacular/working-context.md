# Working context: 20261011130709-aa3eb933-python-support

## Problem and motivation
- Epic 20261010160411-19b965db-language-reference started as a docs reorg. User: "the core problem is that once we support multiple languages we are going to need to completely change the docs again. So I think we should just bite the bullet and add the other language support into this epic. The outcome of the language support and how that affects the api will drive what shape our docs need to be."
- Positioning: "I aim to create a language that builds on the readability of HCL for modern processing. I also intend to bring the library to Python, Javascript and other languages." Docs present the language first, Go library second.
- User: "let's start to add the spec for supporting python that is based on the design doc libxcl".

## Design
- design/libxcl.md (source `design`), status draft, user settling it (revised: out-of-process `xcl-core` deferred; protobuf carried through one C function, not RPC; codegen section added).
- Python = libxcl phases 1-4: schema v1; plugin protocol v2; Python plugin SDK out of process; CoreService + buf + coreservice + bindings/libxcl + Python binding + wheels (+ context-taking op methods in root). Node = phase 5. xcl-core + Wasm later.
- Design open questions relevant to Python: plugin distribution via GitHub registry; sync vs asyncio first; host-language functions not designed; ABI version independence; number `format` hint; schema v1 lacks `key` tag/`body` kind; toolchain versions.

## Interactions
- language-surface spec (20261011095738-fad05e41-language-surface, finished, not in the epic yet): plugin-hosted functions via a protocol RPC + app-registered functions; protocol v2 reworks the same protocol → order or fold. Named child blocks into Go maps → schema v1 must describe them.
- Docs specs come last and get revised; guide plan likely stale.
- xcl-python repo doesn't exist and isn't registered.

## Epic state (planning on hold)
- Guide plan final + summary section written. Reference plan paused at discovery. Earlier notes backed up in session scratchpad wc-before-python.md.

## Interview answers (python-support)
- See .spektacular/work/20261011130709-aa3eb933-python-support/interview.md. Key: Go plugins only, no Python providers; Python functions yes; sync + asyncio, docs lead sync; new xcl-python repo; PyPI; 3.10+; depends on language-surface; site docs later in the epic.
- User: 'this is all good, keep going until you are done' (after constraints) — draft remaining sections, review, commit, add both specs to epic without stopping per section.

## Done (2026-10-11)
- python-support spec finished and stored (design ref design/libxcl.md). Review: 14 of 15 fixes applied under the user's "keep going until you are done"; dropped "add no-reimplementation constraint" (covered by design ref). Split not offered (tightly coupled on the engine binding).
- Epic updated: overview rewritten for multi-language; specs language-surface → python-support → reference, guide (reference and guide depend on both). Reversible with `spektacular epic write`.
- Still to do later: revise reference spec ACs (locals dropped, module refs via outputs/Go Find, language-neutral); guide plan likely stale; register xcl-python repo once created; Python plugins/protocol v2 and Node are future specs.

## Epic planning run 2 (spek-plan-epic, restarted 2026-10-11)
- Reference + guide specs revised (user OK): Go/Python tabs on all app code (remembered), Python on all existing pages except plugin authoring (Go-only for now), per-language guides (Go, Python), locals dropped, XCL_VAR_, modules via outputs, functions 3 sources, expressions list w/o %{}.
- Cleaned: guide plan deleted, reference lane + work dir removed. Epic summary still holds stale guide section (overwrite at Step 6).
- Order: language-surface (ready) → python-support → reference, guide.
- Started child: language-surface.
- User away (swim): "just keep going, if you have any questions save them up to the end. Unless this is so big that you absolutely can not continue." → answer child QUESTIONs with the recommended default, log them under "Deferred questions" below for end review.

## Deferred questions (answered with default, review with user)
1. language-surface @discovery — when are plugin function clashes reported? AC says "creating the config fails", but plugins load lazily on first operation (where plugin type clashes are reported). Answered default A: app-vs-builtin/app clashes fail NewConfig; clashes involving a plugin fail the first operation when plugins load (FunctionNameClashError naming function + both sources), matching how type clashes work. Spec AC wording may need a tweak ("creating the config or its first operation"). Alternatives: B eager plugin load in NewConfig; C optional preload option.
- language-surface: DONE (plan final). Summary notes in session scratchpad done-language-surface.md. Issue to fix at review: plan assumes release tag v0.2.0 but latest tag is v0.2.1 (breaking → v0.3.0). Built-in count 57 vs 56 to double-check.
- Started child: python-support.
2. python-support @tasks — xcl-python not registered, so plan store refuses its tasks. Answered default A: child registers xcl-python via `spektacular repo add` at /home/nicj/code/github.com/jumppad-labs/xcl-python (creates dir + .spektacular footprint, changes project repo config; reversible). Human task becomes "create GitHub repo and connect the local dir". User earlier agreed the spec creates the repo and I'd register it.
- python-support: DONE (plan final, 23+ tasks, humans: create GH repo, signing key+dispatch token, PyPI Trusted Publishing, cut v0.4.0). xcl-python registered via repo add. Rules: CHANGELOG `## <spec>` + **Breaking:** None; schema v1 gains named-block map + label; CoreService error codes append-only; root CGO_ENABLED=0; new libxcl-release.yml on v* tags; PyPI version = tag w/o v; additive root API (*Context ops, FindEntitiesByType/FindOneEntity, registry.TypeFromSchema). Release sequence v0.3.0 (language-surface) → v0.4.0 (python). Implementation-time questions: MakeFunc accepted by language-surface conversion? PyPI name `xcl` available?
- Started children: language-reference, using-xcl-with-go (parallel).
- language-reference: DONE (12 agent tasks, no human). Rules: CodeTabs.astro (go/python slots, html data-lang, localStorage xcl-lang, default go); 8 pages under /language/; Nav: new "Language" dropdown between Features and Examples, guides expected at top of Guides dropdown; Home: new "Documentation" Section after "Two examples" (reference first, guides join) + "Learn the language" link; README: reference first in Documentation, guides follow; site.ts gains pythonRepoURL/pythonParityURL/pypiURL; CHANGELOG entry Breaking: None. Plugin authoring pages = plugin-template, examples/plugins, replacement, plugin-logging. Leaves home hero "v0.2.2" badge + "Python to come" tagline alone (follow-up). 57 built-ins confirmed.
- using-xcl-with-go: DONE (2 guides /using-xcl-with-go/, /using-xcl-with-python/; pdoc API docs for xcl-python on GH Pages; human: enable GH Pages). All 4 plans final. Loop ended.

## Step 6 pending — disagreements to settle with user (do NOT write summary until settled)
D1 Home page: reference adds new "Documentation" Section after "Two examples" (reference first, guides join) vs guides put both guides in closing CtaBanner ("Start building with XCL", Go primary, Python secondary). Proposal: do both — Documentation section lists reference + both guides; CtaBanner keeps guide buttons.
D2 README Documentation order: reference says reference first then guides; guides says guides are first two bullets. Proposal: reference, Go guide, Python guide.
D3 Tabs rule: reference's "no Go box without Python tab outside plugin authoring" must exempt the two per-language guides. Proposal: exempt guides (spec constraint) — edit reference plan's check + manual check.
D4 Release tags: language-surface plan assumes v0.2.0; python-support expects v0.3.0 then v0.4.0. Latest tag v0.2.1. Proposal: language-surface → v0.3.0.
Other notes for review: home hero badge says "v0.2.2" (latest tag v0.2.1) + "Python to come" tagline — nobody owns updating it; site.ts gains both reference's pythonRepoURL/pythonParityURL/pypiURL and guides' pythonReferenceURL (compatible).
- User accepted D1–D4 + both defaults ("I think the questions are right"); hero badge/tagline folded into reference plan's home task (D5). Agent applying edits to LS/REF/GUIDE plan docs. Next: write epic summary (4 sections + decisions), epic order, review.
- Epic summary written (4 sections + decisions, 7 entries). epic order added: using-xcl-with-go depends on language-reference (shared xcl-website/xcl files; some listed paths like main.go/app.web are parse artifacts). Next: Step 7 review with user, knowledge offers, final report.
- Knowledge: 9 entries written (8 xcl, 1 xcl-website), incl. decisions/xcl-is-its-own-language.md extended with language-surface decisions. Epic planning complete; next is "implement this epic".

## Epic implement run (spek-implement-epic, started 2026-10-11)
- Epic 20261010160411-19b965db-language-reference. Order: language-surface → python-support → language-reference → using-xcl-with-go.
- Start question: xcl-plugin-template/xcl-plugin-docker dirty → user will commit them first. xcl-python has no commits → user will add an initial commit before python-support starts.
- Waiting on user to say go.

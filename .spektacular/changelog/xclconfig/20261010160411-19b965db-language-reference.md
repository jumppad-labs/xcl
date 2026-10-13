---
created_date: "2026-10-11"
document_status: draft
project: xclconfig
spec: 20261010160411-19b965db-language-reference
plan: 20261010160411-19b965db-language-reference
---

# The README points to the language reference, and the changelog records it

xcl's README now sends readers to the new language reference on xcl.dev as the first item of its Documentation list, where every part of XCL is described with the matching Go and Python types beside each shape of configuration. The CHANGELOG records the reference and the Go and Python tabs across the site. Nothing in the library changes.

> Derived from project xclconfig (spektacular), spec/plan 20261010160411-19b965db-language-reference. See the project-level record for the full feature.

## What changed in this repo

- `README.md`: the Documentation list starts with "Language reference", linking `https://xcl.dev/language/` and naming what it covers: files, entities, attributes and blocks, references, expressions, variables, modules and functions, with Go and Python types.
- `CHANGELOG.md`: a `## 20261010160411-19b965db-language-reference` entry above the python-support entry, in prose, describing the eight-page reference, the Go and Python tabs with the choice remembered across pages, the "not yet available from Python" notes and the plugin-authoring statement, followed by **Breaking:** None.

No Go code changed. Verifying the reference against v0.4.0 found library behaviour that is reported for follow-up rather than fixed here: `Validate` accepts a reference cycle and only `Apply` fails, with a raw dependency-graph error naming no entity; a set-typed value published through an `output` reads back as its cty `GoString`; a root-level `output.<name>` reference from another block passes `Validate` and fails at `Apply`; an entity type with no fields beyond `types.ResourceBase` is never added to the evaluation context, so references to its `meta` fail at `Apply`; `EncodeEntity` prints an unnamed list-block element as `volume "" { }`; and a bad GitHub plugin repository or version panics in `registry.(*GitHub).RegisterPlugin` with no `recover` on the core-service path, which aborts a Python process.

## Why

The README is an overview and xcl.dev is the home of the documentation. With the language itself now documented there, the README's first pointer is the reference, so a reader who starts at the repository finds the language before the feature pages.

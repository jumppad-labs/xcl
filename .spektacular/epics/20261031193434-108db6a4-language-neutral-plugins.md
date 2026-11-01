---
created_date: "2026-10-31"
document_status: draft
specs:
    - name: 20261031180836-7529c085-plugin-release-contract-v2
      depends_on: []
---


## Overview

Plugins for xcl are written in Go today. This epic opens them to any language: a plugin written in Python runs from Go, Python or TypeScript, and the reverse. It starts with a language-neutral release layout (an xcl index with checksums, per-platform builds, digest-pinned container images and one signature over the index; design `plugin-release-assets-v2.md`), and with local plugins that run from a command such as `python3 plugin.py`, so a plugin in another language can be tried by hand on protocol 1. Later specs add a transport every language can speak, a Python plugin SDK (design `python-plugins.md`), an example Python plugin repository and the docs that go with it.

The planned specs, in order:

1. Plugin release contract v2, ending with local plugins run from a command
2. Plugin protocol 2 and its stdin/stdout transport (design not yet written; revises `libxcl.md`)
3. Python plugin SDK (depends on 2)
4. An example Python plugin repository, built from the SDK (depends on 3)
5. Docs for writing plugins in other languages, in the repository and on xcl.dev (depends on 3 and 4)

## Specs

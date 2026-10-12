---
created_date: "2026-10-11"
document_status: draft
spec: 20261010160411-19b965db-language-reference
specs:
    - name: 20261011095738-fad05e41-language-surface
      depends_on: []
    - name: 20261011130709-aa3eb933-python-support
      depends_on:
        - 20261011095738-fad05e41-language-surface
    - name: 20261011211634-a8895873-python-out-of-process-engine
      depends_on:
        - 20261011130709-aa3eb933-python-support
    - name: 20261011232013-da18c586-python-release-hardening
      depends_on:
        - 20261011130709-aa3eb933-python-support
    - name: 20261010160411-19b965db-language-reference
      depends_on:
        - 20261011095738-fad05e41-language-surface
        - 20261011130709-aa3eb933-python-support
        - 20261011211634-a8895873-python-out-of-process-engine
        - 20261011232013-da18c586-python-release-hardening
    - name: 20261011084948-c995ed25-using-xcl-with-go
      depends_on:
        - 20261011095738-fad05e41-language-surface
        - 20261011130709-aa3eb933-python-support
        - 20261011211634-a8895873-python-out-of-process-engine
        - 20261011232013-da18c586-python-release-hardening
        - 20261010160411-19b965db-language-reference
---



# Epic: 20261010160411-19b965db-language-reference

## Overview

The xcl README was cut back to an overview, leaving xcl.dev without a description of the XCL language or a walkthrough of using it. XCL is also meant to be a language with more than one implementation, so documentation written only for Go would have to be redone once other languages arrive. This epic settles the language surface, brings XCL to Python on the same engine Go uses, and then writes the language reference and the getting-started guides in the shape that the multi-language API calls for. It is for developers building XCL into their applications, in Go or Python, and for the people who write configuration for them.

## Specs

| # | Spec | Scope |
|---|---|---|
| 1 | 20261011095738-fad05e41-language-surface | Settle the XCL language surface and fix the library to match: environment prefix, application and plugin functions, named blocks into maps, no core templating |
| 2 | 20261011130709-aa3eb933-python-support | XCL for Python on the shared engine (libxcl): sync and asyncio APIs, Python functions, Go plugins, PyPI wheels |
| 3 | 20261011211634-a8895873-python-out-of-process-engine | The engine as a bundled subprocess in every Python package, on all eight platforms with no C toolchain in the release; first published Python release v0.5.0 |
| 4 | 20261011232013-da18c586-python-release-hardening | Test suite on every supported Python and checkout layout, a signed test plugin replacing Docker in end-to-end tests |
| 5 | 20261010160411-19b965db-language-reference | XCL language reference with the mapping to implementation types; to be revised once the multi-language API is settled |
| 6 | 20261011084948-c995ed25-using-xcl-with-go | Guide to the core Go API; to be revised alongside the reference |

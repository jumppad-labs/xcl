---
created_date: "2026-11-10"
document_status: draft
project: xclconfig
spec: 20261031080136-264e0eab-typescript-core-sdk
plan: 20261031080136-264e0eab-typescript-core-sdk
---

An xcl release now also produces a TypeScript package. Each release tells `xcl-typescript` about the new tag and size, and every pull request to xcl is tested against the TypeScript package as well as the Python one, so a change that breaks either is caught before it merges.

> Derived from project xcl (https://github.com/jumppad-labs/xcl), spec/plan 20261031080136-264e0eab-typescript-core-sdk. See the project-level record for the full feature.

## What changed in this repo

- The release workflow dispatches `xcl-typescript` with the tag and size, in a job of its own so one binding's failure cannot affect the other's dispatch. It needs a new secret, `XCL_TYPESCRIPT_DISPATCH_TOKEN`.
- The bindings gate runs the TypeScript suite against each pull request's commit and needs it as well as Python's.
- The TypeScript SDK design document was corrected.

## Why

The TypeScript package bundles the engine at the same version as the xcl tag, so each release has to reach it, and a change here must not break it unnoticed.

## Before merging

Merge this after `xcl-typescript`'s test workflow is on its `main` and the dispatch token exists, or xcl pull requests and the next release fail.

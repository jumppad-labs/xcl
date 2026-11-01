---
tags: [parsing, gohcl, validation]
---

# A nested block's name is all its labels in a list, but only the first in a map

A nested block type can have more than one label field. For example, a
`Platform` struct with `OS string `xcl:"os,label"`` and
`Arch string `xcl:"arch,label"`` decodes `platform "linux" "amd64" { ... }`
and fills both fields. How duplicates are detected depends on the field that
collects the blocks:

- **A list of blocks** (`[]Platform`) identifies each block by all of its
  labels together. `platform "linux" "amd64"` and `platform "linux" "arm64"`
  are two names, and only an exact repeat of every label is a "Duplicate
  platform name".
- **A map of blocks** (`map[string]Platform`) is keyed by the first label
  alone, because that is the map key. Two blocks sharing a first label clash
  there, whatever their other labels are. Use a list when blocks are unique by
  a compound key.

Both rules live in `checkBlockNames` and `listBlockKey` in
`internal/xcl/gohcl/decode.go`, which the decoder and CheckBody share.

Before 2026-10-31, lists also compared only the first label, so a plugin
release index with two `linux` platforms was refused. That was fixed as a bug
while designing the plugin package format.

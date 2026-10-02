# internal/xcl — derived from HashiCorp HCL

This directory contains a modified copy of the HashiCorp Configuration Language
(HCL) library.

| | |
|---|---|
| Upstream | https://github.com/hashicorp/hcl |
| Module | `github.com/hashicorp/hcl/v2` |
| Version | `v2.21.0` |
| Copyright | Copyright (c) 2014 HashiCorp, Inc. |
| License | Mozilla Public License, version 2.0 — see [LICENSE](LICENSE) |

## Licensing

Every file in this directory is licensed under MPL-2.0, including any
modifications made to it. The rest of this repository is licensed under
Apache-2.0, except for the other third-party directories listed in the
repository's NOTICE file; the MPL-2.0 boundary for HCL is this directory.

When working in this directory:

- Keep the existing `Copyright (c) HashiCorp, Inc.` and
  `SPDX-License-Identifier: MPL-2.0` header on every file.
- When modifying a file, add `// Modifications Copyright (c) Jumppad Labs`
  below the existing header if it is not already present.
- Any new file containing code copied or adapted from this directory must also
  carry the MPL-2.0 header and live in this directory.
- Do not move code from this directory into Apache-2.0 licensed files.
- Do not use the HCL or HashiCorp names in a way that implies endorsement.

## What was imported

Only the packages this module depends on:

- `.` (root `hcl` package)
- `ext/customdecode`
- `gohcl`
- `hclparse`
- `hclsyntax` (including the Ragel sources for the generated scanners)
- `hclwrite`
- `json`

Import paths were rewritten from `github.com/hashicorp/hcl/v2/...` to
`github.com/jumppad-labs/xcl/internal/xcl/...`.

## Modifications

- `hclparse/parser.go`: `ParseHCLFile` renamed to `ParseXCLFile`.
- `gohcl`: struct tags are read from the `xcl` key instead of `hcl`, and
  parsed by the `tags` package. A tag is a name followed by comma separated
  options, at most one of which is a kind (`attr`, `block`, `label`, `remain`,
  `body`, `optional`). The XCL options `computed` and `key` are also accepted.
  `gohcl` uses `internal/cty/gocty` instead of `github.com/zclconf/go-cty/cty/gocty`.
- `tags`: new package, adapted from the tag handling in `gohcl/schema.go`, so
  that `gohcl` and `internal/cty/gocty` share one parser for `xcl` struct tags.
- All packages: go-cty imports rewritten from `github.com/zclconf/go-cty/cty/...`
  to `github.com/jumppad-labs/xcl/internal/cty/...`.
- Test files in `gohcl`: struct tags changed from `hcl` to `xcl`.
- Test files `ops_test.go`, `hclsyntax/parser_test.go`,
  `hclsyntax/structure_at_pos_test.go`, `hclsyntax/walk_test.go`,
  `hclsyntax/expression_static_test.go`: non-constant format strings passed to
  `t.Errorf`/`t.Logf` changed to use `"%s"`, required by `go vet` under Go 1.24+.
- `hclsyntax/token_type_string.go`, `json/tokentype_string.go`: added the
  missing MPL-2.0 header to these generated files. Re-running `go generate`
  (stringer) will drop it; re-add it afterwards.
- `gohcl/check.go`: new `CheckBody`, which checks a body against a Go value's
  implied schema the way `DecodeBody` would (remain fields, nested and
  repeated blocks) without decoding or evaluating any expression.
- `gohcl/encode.go`: new `EncodeBody`, which writes what `DecodeBody` reads.
  Upstream's encoder drops data the decoder fills in, so it is repaired for
  XCL's types, and the repairs are shared with `EncodeIntoBody` and
  `EncodeAsBlock`, which keep their signatures and their panic on failure:
  - fields of a struct embedded under a `remain` tag are written alongside the
    embedding struct's own fields, in field order, the way the decoder fills
    them. Upstream ignored `remain` entirely, which dropped every field of an
    embedded base such as `types.ResourceBase`. A `remain` field holding an
    `hcl.Body` or `hcl.Attributes` is still skipped.
  - a value held in an interface field is written by the type of the value it
    holds. Upstream skipped any field an `hcl.Expression` was assignable to,
    which is every field typed `any`, so fields of plugin types rebuilt from a
    schema were dropped. Only fields whose type is exactly `hcl.Expression` or
    `*hcl.Attribute` are skipped now.
  - a field holding nothing, a nil pointer, interface, slice or map, is left
    out rather than written as `null`. A zero number, false or empty string
    that is present is still written.
  - a field whose tag carries XCL's `computed` option is left out unless
    `EncodeOptions.IncludeComputed` asks for it, because it is owned by the
    provider rather than written in configuration.
  - a value that cannot be represented is returned as an error naming the
    field. `EncodeBody` recovers the panics raised by `hclwrite` and
    `internal/cty/gocty` for unknown, capsule and unsupported values, so the
    panic does not cross the fork boundary.
- `gohcl/schema.go`: `fieldTags` carries a `Computed` set, so the encoder can
  act on the `computed` option that `tags.Parse` already reads.
- `gohcl/encode_body_test.go`: new, covering the `EncodeBody` behaviour above.
- `gohcl/encode.go`: an attribute whose value is a struct carrying `xcl` tags
  is built by the encoder rather than handed whole to `gocty`. Upstream's
  `gocty.ImpliedType` builds an object from every tagged field, so the rules
  about computed fields and empty values reached only the fields of the body
  itself, never inside such an object. The encoder now recurses, and leaves the
  struct's `remain` base out of the object, because an object attribute holds a
  copy of a value rather than something written in configuration.
  `EncodeOptions.ComputedComment` marks each computed field it writes.
- `hclwrite/ast_body.go`: **bug fix.** `SetAttributeRaw`, `SetAttributeValue`
  and `SetAttributeTraversal` each declared a new `attr` inside the else branch,
  shadowing the one they return. Adding an attribute therefore returned nil
  rather than the attribute created, contradicting the documented "return value
  is the attribute that was either modified in-place or created". The inner
  declarations are now assignments. This is an upstream defect, present in
  hashicorp/hcl, not something XCL introduced.
- `hclwrite/ast_attribute.go`: new `SetLineComment` and `SetLeadComment`, so a
  caller can write a comment against an attribute. The `leadComments` and
  `lineComments` nodes already existed but nothing could set them.
- `hclwrite/node.go`: **bug fix.** `node.ReplaceWith` rewired the replaced
  node's neighbours but left the owning list's own `first`/`last` pointers on
  the node it had just detached. Replacing the first node of a list therefore
  left `first` on a detached node whose `after` was nil, so walking the list
  yielded that node alone and silently dropped everything after it. It surfaced
  through `SetLeadComment`, whose `leadComments` node is the first of an
  attribute's children: setting a lead comment deleted the whole attribute.
  `SetLineComment` escaped only because `lineComments` is neither first nor
  last. Another upstream defect, present in hashicorp/hcl.

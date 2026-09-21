---
tags: [entity, resource, vocabulary]
---

# Entity

An **entity** is anything a configuration declares — a `resource`, `variable`,
`output` or `module` block alike. `resource` names one stanza, not the
taxonomy: in an address it is a sibling of `output`, `variable` and `local`
(`internal/resources/fqrn.go:58`), never their parent.

The term is already the vocabulary of the plugin boundary, where
`Create(entityType, entitySubType string, entityData []byte)` carries exactly
this type-and-subtype taxonomy. The rest of the code has yet to follow —
`Config.GetResources()` and `state.resources` still say resource — so this
entry states the target. The reasoning is recorded in the design document
`design/querier-api-v2.md`.

---
tags: [types, plugins, query, reflection]
---

# Only plugin entities convert into a different named Go type

`Find`, `FindByType`, `As`, `All` and `Decode` can convert an entity into a different named Go type only when the entity is a plugin entity, an anonymous struct the host rebuilt from a schema (see [[plugin-types-rebuilt-with-structof]]).

A registered type converts only to itself. So a test that needs "convert into some other Go type" cannot use a registered fixture type with `All` or `Decode`; drive it through a plugin entity, or through `Find`, `FindByType` or `As`, which share the conversion path.

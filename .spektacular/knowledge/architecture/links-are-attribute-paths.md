---
tags: [links, references, meta, addressing, dag]
---

# Meta.Links holds attribute paths, not entity addresses

Each entry in `types.Meta.Links` is the full path of the referenced attribute as written, such as `resource.network.c.subnet`, not the address of the entity, `resource.network.c`.

Code that needs the entity (graph building, boundary checks, dependency listings) has to reduce the link to its entity address first; comparing a link directly against an entity address never matches.

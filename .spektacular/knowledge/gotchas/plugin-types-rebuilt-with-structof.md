---
tags: [plugins, schema, reflection]
---

# Plugin types are rebuilt on the host with reflect.StructOf

The host rebuilds plugin types with `reflect.StructOf`, using a type map hard-coded to `types.Meta`, `types.ResourceBase` and `cty.Value`.

The trap: a plugin struct field whose type is any other named struct, such as a generic wrapper, comes out on the host as an empty `struct{}` with no error. Its data silently disappears.

Instead, add the type to that map. For a generic type, add each instantiation explicitly, because Go cannot build generic types at run time.

# Parity fixtures

Each directory is one fixture the Go and Python parity programs both run,
so the two can be compared record for record:

- `types.json` lists the schema v1 documents of the config-only types both
  sides register. Go registers them with `registry.TypeFromSchema`; Python
  derives the same documents from its dataclasses and checks them against
  this file.
- `main.xcl` is the configuration applied.
- `changed/main.xcl`, when present, is the configuration diffed after the
  apply, with one setting changed.
- `variables.json`, when present, holds the variables passed in. A value of
  `<tmp>` is replaced by the run's temporary directory.

The `functions` fixture calls `slugify`, which each side registers as its
own function: lower case, spaces to dashes.
- `hand_built.json`, when present, is a JSON array of entity addresses that
  a binding also builds by hand, with the values `main.xcl` gives them, for
  the encode stage.

The record's `encode` section holds, for every entity that is not a
variable, output or module, keyed by address, its configuration text under
each of the sixteen combinations of the encode choices: `looked_up`, the
entity as a lookup returns it, and `saved`, its record in state. A
combination is named by the choices it turns on, joined with `+` in the
order `computed`, `empty`, `reveal`, `references`, and `none` when it turns
on none. An address listed in `hand_built.json` also has `hand_built`, the
text a binding's hand-built copy must give, which is the entity's own. The
`encode` fixture covers a type with a subtype and one without, a sensitive
field, a reference, a sensitive bare reference and template, an unset
optional string, a nullable attribute left out, a configured empty list and
an unset map.

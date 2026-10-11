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

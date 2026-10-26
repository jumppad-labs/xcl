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

The record also holds what a person is shown, so a binding's printing and
logging are compared with Go's:

- `printed_validate` and `printed_apply`: the report `xcl.PrintError` prints
  for a failed validate and apply, plain and wrapped at 80 columns, or `""`
  when the call succeeded.
- `printed_entities`: for every entity that is not a variable, output or
  module, keyed by address, the text `xcl.PrintEntity` prints for it with
  `xcl.IncludeComputed()`, plain.
- `printed_diff_before`: the change summary `xcl.PrintDiff` prints for the
  diff before the apply, plain, or the outcome when the diff failed.
- `printed_diff_changed`, when the fixture has `changed/main.xcl`: the change
  summary `xcl.PrintDiff` prints for that diff, plain.
- `log`: every event of the run up to the load, written through
  `events.SlogHandler` at every level, as `{"level", "message", "resource",
  "operation"}` (`level` one of `debug`, `info`, `warn`, `error`; `resource`
  and `operation` `""` when the record has none), sorted by operation,
  resource, level and message.
- `pretty_log`: for a fixture whose apply succeeds, what Go's example pretty
  logger (`example/prettylog`, `prettylog.Handler` at info) writes for a fresh
  apply, in a state directory of its own, with event data at
  `xcl.EventDataProcessed`, into a buffer: a sorted list of groups, each an
  event line joined by `\n` with the indented configuration written beneath
  it. Colour codes are removed, each line's leading time is replaced by
  `<time>` and every `duration=<value>` by `duration=<duration>`. Sorting the
  groups keeps resources created concurrently from reordering them. It is
  `null` for a fixture whose apply fails, because Go's example writes no
  report beneath an error.

Paths in all of them are replaced as elsewhere in the record.

The `unsupported_block` fixture's configuration has a nested `timeout` block,
which the `app` type does not have, opening on line 7, so its validate and
apply fail with an unsupported-block report showing lines 5 to 9.

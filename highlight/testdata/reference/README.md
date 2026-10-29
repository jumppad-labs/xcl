# Reference outputs

These files are the contract every language's printing is held to. They are
produced by xcl's Go formatters and checked byte for byte by
`highlight/reference_test.go`; other languages check their own formatters
against the same files, with no engine involved.

## Inputs

Three labelled token streams, each a JSON array of tokens in order:

```json
[
  { "kind": 1301, "text": "Error" },
  { "kind": 1401, "text": ": Unsupported block type" }
]
```

`kind` is the number of the token's kind (`Text` 0, `BlockType` 101,
`LineComment` 1101, ... as numbered in `highlight/kind.go`), and `text` is its
text. Joined in order, the texts are the whole output.

- `report.tokens.json`: an error report holding an error and a warning, with
  the detail unwrapped.
- `entity.tokens.json`: a resource's configuration text, `inputs/entity.xcl`
  as the tokenizer labels it.
- `diff.tokens.json`: a change summary with every action.

## Outputs

For each input `<input>`, formatted with the default theme:

| File | Format |
| --- | --- |
| `<input>.plain.txt` | Plain |
| `<input>.terminal.txt` | Terminal |
| `<input>.html` | HTML, with classes |
| `<input>.inline.html` | HTML with inline styles |
| `<input>.svg` | SVG |

Besides those:

- `default.css`: the stylesheet generated from the default theme.
- `classes.txt`: the container class `xcl`, then the class of every kind after
  `Text`, in kind order, one per line.

## Regenerating

A change to a format, the default theme or a class name changes these files,
and that change is reviewed like any other change to a public contract:

```shell
go test ./highlight/ -run TestReference -update
```

Run it twice and the files do not change the second time.

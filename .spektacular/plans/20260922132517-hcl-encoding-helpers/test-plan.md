---
created_date: "2026-09-23"
document_status: final
closed_date: "2026-09-23"
---

# Test plan: 20260922132517-hcl-encoding-helpers

Two of the plan's success metrics are not covered by an automated behavioural
test. The rest are, and live in the test suite rather than here:

- **Agreement (0 mismatches between entity and saved data)** — covered by
  `TestPluginExampleEntityAndStateAgree`, `TestConfigOnlyExampleEntityAndStateAgree`
  and `TestAppConfigExampleEntityAndStateAgree`, each of which compares every
  saved record against its entity byte for byte and carries a guard so it
  cannot pass on an empty set.
- **Visible in the examples** — the presence and ordering of each entity's text
  is covered by `TestPluginExampleShowsPostgresConfigurationAfterCreate`,
  `TestPluginExampleShowsEveryCreatedEntity`,
  `TestConfigOnlyExampleShowsCreatedEntities` and
  `TestAppConfigExampleShowsCreatedEntities`. Only the *look* of the real
  terminal output is manual, below.
- **Round trip (100% of example resources read back)** — **descoped.** The user
  decided the text is for display rather than reprocessing, so it is not
  validated as a configuration. See `**Descoped requirements**` in `plan.md`.

---

## 1. Visible in the examples — a look at real terminal output

**What to measure.** That a person running an example sees each created
entity's configuration, correctly placed and readable, in a real terminal with
colour. The automated tests assert on a buffer, which cannot show whether the
result is actually legible.

**How.** From the xcl repository root:

```
cd example/plugin
make build          # builds the external plugin into ./build/external
go run . ./config ./build/external
```

Then, from the repository root:

```
go run ./example/configonly
go run ./example/appconfig
```

**Expected result.** For each of the three programs:

- every resource that is created is followed, on the lines directly beneath its
  `create success` line, by its configuration, indented two spaces so it reads
  as belonging to that line;
- the block header is the form a person writes, e.g.
  `resource "postgres" "main" {`, and nested blocks such as `timeouts` are
  indented within it;
- values the provider filled in are present and each carries the comment
  `# set by the provider`, e.g.
  `connection_string = "postgres://admin@localhost:5432/main" # set by the provider`;
- no `meta`, `depends_on` or `disabled` appears in any block;
- no `unable to show configuration` warning appears. Variables, outputs and
  modules produce no configuration and no warning;
- the configuration text is distinguishable from the log lines around it.

Then confirm it is suppressed when it should be:

```
XCL_LOG_LEVEL=warn go run ./example/configonly
```

**Expected result.** No configuration text at all, only warnings and errors.

**Who / when.** The implementer, before merge. Re-run when the pretty receiver
or the encoder's output shape changes.

---

## 2. Easy to use — a developer can convert following the docs alone

**What to measure.** That someone who has not read this plan can convert an
entity and an entity's saved data using only the published documentation.

**How.** Ask a developer who did not work on this feature to do the following
without reading the implementation, the plan or this test plan. Give them only:

- the README section **Converting to configuration text**, and
- the xcl.dev guide at `/configuration-text/`.

Their task:

1. Take an existing example, such as `example/configonly`, and after the apply
   print one entity's configuration using `xcl.EncodeEntity`.
2. Print the same entity from its saved data using `xcl.EncodeSavedEntity`,
   obtaining the record either from the state file or from an event.
3. Print it again including the values the provider filled in.
4. Say what happens, without running it, if the saved data names a type the
   registry does not know.

**Expected result.** All four completed from the documentation alone, with no
question that the documentation does not answer. Specifically they should not
need help to discover that:

- `EncodeSavedEntity` needs the registry, and works on one that no
  configuration has loaded;
- provider-filled values are off by default and `IncludeComputed()` adds them;
- events carry no data until `WithEventData` asks for it;
- the failure is `ErrUnregisteredType`, matched with `errors.Is`.

Record any point where they had to ask, and fix the documentation rather than
explaining it in person.

**Who / when.** A developer other than the implementer, before release.

---

## Note for whoever runs these

Both procedures exercise output that shows values exactly as they are held,
including passwords and any other secret. That is the documented behaviour
(Out of Scope, tracked in jumppad-labs/xcl#1). Run them against the bundled
examples' fixture data, not against a real configuration holding real
credentials, and do not paste the output into a ticket or a pull request.

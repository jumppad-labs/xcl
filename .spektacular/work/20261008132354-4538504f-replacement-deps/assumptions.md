### Unknown inputs: read with saved values, floor at Update (discovery)
- **Decision**: In the decide pass a resource whose configured values contain unknowns is still Read and asked Changed, with the saved value at each unknown path; core floors its outcome at Update, and the provider may raise it to Replace.
- **Rationale**: The design says every resource gets Read then Changed with its dependencies' decisions. Placeholders would mislead providers. Without a floor, a template whose input IP will change could answer NoChange and never re-render.
- **Rejected**: Skipping Read/Changed for unknown inputs (today's diff behaviour) stops the provider deciding Replace. Passing placeholders feeds providers bogus values.

### Decide failure leaves state untouched (discovery)
- **Decision**: A Read or Changed error in the decide pass fails the apply and leaves the previous state as it was. No resource is marked failed.
- **Rationale**: The spec says "a failing decision changes nothing". Nothing was attempted, so marking the resource failed would force a needless replace next time.
- **Rejected**: Keeping today's behaviour of marking a resource failed when its Read fails.

### Replace reason in the comment line (discovery, user decision)
- **Decision**: The rendered reason goes in the existing comment line above the header: `# <addr> will be replaced because <dep> is replaced`.
- **Rationale**: The user chose it, for consistency with the other actions.
- **Rejected**: A trailing comment on the header line, as in the design sketch.

### Look through provider-less entities for dependencies (discovery, user decision)
- **Decision**: A resource's dependencies are the provider-backed resources it reaches directly, or through outputs, variables, modules and registered config-only types.
- **Rationale**: The user chose it, so that module boundaries don't hide a replacement.
- **Rejected**: Listing only literal direct references to provider-backed resources.

### Tooling for generated code (discovery)
- **Decision**: Regenerate the proto with the local protoc and pinned `protoc-gen-go@v1.36.11` / `protoc-gen-go-grpc@v1.5.1` run through `go run`. Regenerate mocks with `go run mockery/v3@v3.8.0`.
- **Rationale**: These match the generator versions already in the repo, and neither mockery nor matching plugins are on PATH.
- **Rejected**: Using the installed protoc-gen-go-grpc 1.6.1 (needless churn), or editing the generated code by hand.
### Chosen direction: decide pass = promoted diff walk, act = destroyer then apply walk (architecture)
- **Decision**: Promote the existing diff walk into a shared decide pass with a per-entity decision record. Act runs the destroyer (reverse graph over replaced and removed resources), then the ordinary apply walk following the recorded decisions. The in-walk rebuild is removed. Delivery order: contract change, then decide/act, then reason and render, then example plugins, then docs.
- **Rationale**: This reuses the code that already decides without acting (refresh, the pending/unknown hooks), so plan and apply share one decision. The destroyer already gives dependents-first destroys and per-resource saves. The repository keeps building between phases.
- **Rejected**: Extending the in-walk rebuild (wrong destroy order). A separate decide path (plan/apply could disagree). Serial topological lists for the act phase (loses concurrency and duplicates context building). Effort: Medium-High.

### Unchanged resources keep the decide-pass copy (architecture)
- **Decision**: An entity decided NoChange is saved as its decide-pass read copy, with its status unchanged. An Update gets the freshly decoded configuration with computed values carried from the decide-pass Read.
- **Rationale**: This matches what read() saves and sends today, without calling Read twice.
- **Rejected**: Calling Read again in the act pass (doubles provider calls and could disagree with the plan).

### Plugin replace settings (architecture)
- **Decision**: Docker network: subnet means Replace. Docker container: image, command, environment and network blocks mean Replace, and a replaced dependency means Replace. Template: destination means Replace; source and variables mean Update. Person: first_name or last_name means Replace. The e2e fixtures keep DefaultChanged semantics (Update) because their fake providers can change every field in place.
- **Rationale**: These are the settings each provider really cannot change in place: Docker cannot change them without recreating, the template's Update cannot remove the old file, and the person ID derives from the name.
- **Rejected**: Leaving container fields as no-op updates, which is the drift the spec exists to remove.

### Conventions selected (architecture)
- **Decision**: Applied: code style, top-level public packages, testing and mocking, real-apply state, graph-parent ordering, shared test helpers, never modifying dependencies, the shared errors package, structured logging, the example-modules gotcha, the EncodeForState gotcha, and the entity glossary. Dropped: database, and HTTP/graceful shutdown.
- **Rationale**: These are the surfaces the work touches.
- **Rejected**: Listing every convention.

### Diff reason shape (data_structures)
- **Decision**: Add `diff.ReplaceReason` (failed, provider, dependency) and `ReplacedDeps []string` to `diff.Resource`, as JSON `reason` and `replaced_dependencies`, both omitted when empty.
- **Rationale**: The renderer needs three distinct phrases, and JSON consumers can then tell why a resource is replaced. The fields are additive to the diff design's Resource shape.
- **Rejected**: A single free-text reason string (not machine-readable). Deriving the reason at render time (the renderer has no state).

### Proto field numbering (data_structures)
- **Decision**: Make `ChangedResponse.change` field 3 and reserve 1, and add `ChangedRequest.dependencies` as field 5.
- **Rationale**: A stale plugin then fails loudly: it gets an unset change (NoChange) rather than having its bool misread. Breaking the protocol is allowed.
- **Rejected**: Reusing field 1 with a new type (wire-incompatible in confusing ways).

### Existing tests whose expectations change (testing_approach)
- **Decision**: Update the tests the design deliberately breaks (cross-resource event interleaving, never-read-with-unknown-inputs, a failed Read marking the resource failed) in the same task, and give the reason in that task.
- **Rationale**: These behaviours follow directly from decide-then-act and the unknown-input rule.
- **Rejected**: Keeping the old expectations through special cases.

### Docker success metric is behavioural but gated (testing_approach)
- **Decision**: Classify the no-drift metric as a behavioural Docker-gated test, plus a manual walkthrough on a machine with Docker.
- **Rationale**: The test skips without Docker, as every existing example test does.
- **Rejected**: Manual only (loses automation where Docker exists).

### e2e fixtures gain small replace rules (tasks)
- **Decision**: The in-process e2e PostgreSQL `location` and the external e2e Ingress `hostname` answer Replace. Every other field keeps the DefaultChanged answer.
- **Rationale**: The plan-versus-apply e2e scenarios need provider-decided replacements on both plugin kinds, and those two fields are identity-like.
- **Rejected**: Leaving all e2e fixtures on DefaultChanged (no e2e coverage of provider replace on the mixed graph).

### Newly created dependencies are not listed (tasks)
- **Decision**: A dependency decided create is not included in a dependent's list.
- **Rationale**: The design lists only Update and Replace. The Update floor for unknown inputs covers the dependent.
- **Rejected**: Reporting create as Replace (misleading to providers).

### Website page placement (tasks)
- **Decision**: Add a new page `src/pages/replacement.mdx`, linked from the Guides menu.
- **Rationale**: The site has no plugin-authoring page. The spec requires a section reachable from the guides.
- **Rejected**: Burying the explanation inside the plugin example page (not reachable as a guide).

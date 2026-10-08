### Milestone 1: Plugins answer unchanged, update or replace

#### - [ ] Task: Change contract through every plugin layer
**Id:** 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7
**Repo:** xclconfig
**Depends on:** none
**Execution:** agent

Introduces the `Change` answer (unchanged, update, replace) and the dependency list into the public plugin contract. Carries them through every layer between a provider and the core: the typed adapter, the in-process host, the gRPC protocol for external plugins, and the testing helpers. `DefaultChanged` and the recording test plugin are migrated, along with every caller that asserted a yes/no answer, so the whole repository and its examples keep building. The core honours a "replace" answer by destroying and recreating the resource on the existing failed-resource path, and passes no dependencies yet.

*Technical detail:* [context.md#task-change-contract-through-every-plugin-layer](./context.md#task-change-contract-through-every-plugin-layer)

**Acceptance criteria**:
- [ ] A provider can answer unchanged, update or replace, and the answer arrives unchanged at the core through both in-process and external plugins
- [ ] A dependency list given to an external plugin's change check arrives at the provider with the same addresses and outcomes
- [ ] `DefaultChanged` answers update when configured values differ and unchanged otherwise
- [ ] A "replace" answer during apply destroys the resource and creates it again
- [ ] The root module, every example module and the external test plugins build, and all their tests pass

### Milestone 2: Applies decide everything first, tell dependents, and replace in a safe order

#### - [ ] Task: Decision record and dependency resolver
**Id:** 56c89a24-1d35-469b-ac6f-f429f2de1ddb
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

Grows the diff recorder into a decision record that holds each entity's decided outcome, the dependencies it was told about, the reason for a replacement, and the copy read while deciding. Adds the resolver that turns an entity's references into the list of provider-backed resources it depends on that will update or replace, looking through outputs, variables, modules and config-only types. These are the building blocks the decide and act passes share.

*Technical detail:* [context.md#task-decision-record-and-dependency-resolver](./context.md#task-decision-record-and-dependency-resolver)

**Acceptance criteria**:
- [ ] A resource's dependency list contains only the provider-backed resources it references that will update or replace, each with its outcome
- [ ] A dependency reached through a module output or variable is listed as the provider-backed resource behind it
- [ ] The record lists every replaced and removed resource as the set to destroy
- [ ] Existing diff results are unchanged

#### - [ ] Task: Decide pass tells each resource about its dependencies
**Id:** a4febb1d-d732-4579-b6fd-651d403d21e0
**Repo:** xclconfig
**Depends on:**
- 56c89a24-1d35-469b-ac6f-f429f2de1ddb — Decision record and dependency resolver
**Execution:** agent

Turns the diff walk into the single decide pass shared by plans and applies. Every saved resource is read and asked whether it changed, together with the decisions already made for its dependencies, and its answer is recorded. A resource whose inputs are not yet known is read with its saved values and planned as at least an update. Any failure while deciding stops the operation before anything is touched. The recording test plugin learns to record the dependencies it is told.

*Technical detail:* [context.md#task-decide-pass-tells-each-resource-about-its-dependencies](./context.md#task-decide-pass-tells-each-resource-about-its-dependencies)

**Acceptance criteria**:
- [ ] A resource referencing a replaced dependency and an updated dependency is told about exactly those two, with their outcomes, and not about an unchanged one
- [ ] A plugin's replace answer is planned as a replacement and its update answer as an update
- [ ] A dependent whose plugin answers unchanged to a replaced dependency is planned as unchanged
- [ ] A resource whose inputs will only be known after the apply is planned as at least an update
- [ ] A failing change check fails the plan or apply without any create, update or destroy

#### - [ ] Task: Act pass destroys first, then creates and updates
**Id:** ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c
**Repo:** xclconfig
**Depends on:**
- a4febb1d-d732-4579-b6fd-651d403d21e0 — Decide pass tells each resource about its dependencies
**Execution:** agent

Makes apply run the decide pass first and then act on it. Every resource being replaced or removed is destroyed together, dependents first. The apply walk then creates new and replaced resources, updates updated ones, and leaves unchanged ones alone, in dependency order. The old in-walk rebuild goes away, so resources whose last apply failed are replaced the same way. A failed destroy or create is recorded so that the next apply replaces the resource again.

*Technical detail:* [context.md#task-act-pass-destroys-first-then-creates-and-updates](./context.md#task-act-pass-destroys-first-then-creates-and-updates)

**Acceptance criteria**:
- [ ] With a network and the container on it both replaced, the container is destroyed, then the network, then the network is created, then the container
- [ ] Every read and change check in an apply happens before its first destroy, create or update
- [ ] An update answer updates the resource in place without destroying it
- [ ] When creating a replaced resource fails, it is saved as failed and the next plan lists it as a replacement again
- [ ] When destroying a replaced resource fails, it is saved as failed to destroy and the apply stops
- [ ] Resources whose last apply failed are still replaced on the next apply

#### - [ ] Task: Person plugin replaces on a name change
**Id:** d6ddcc23-ebee-4c18-afbe-4c2723905f5b
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

The person provider in the plugin SDK example derives its ID from the first and last name, so it cannot rename a person in place. It now answers replace when either name changes and update for its other fields. This provider runs both in-process and as an external program, which makes it the fixture for proving that the two behave the same.

*Technical detail:* [context.md#task-person-plugin-replaces-on-a-name-change](./context.md#task-person-plugin-replaces-on-a-name-change)

**Acceptance criteria**:
- [ ] Changing a person's first or last name is answered as replace, through both the in-process and external plugin
- [ ] Changing any other field is answered as update, and an identical person as unchanged

#### - [ ] Task: Plans agree with applies for built-in and external plugins
**Id:** 960a7608-e9ce-4fdc-bcc6-c33a389a2d85
**Repo:** xclconfig
**Depends on:**
- ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c — Act pass destroys first, then creates and updates
- d6ddcc23-ebee-4c18-afbe-4c2723905f5b — Person plugin replaces on a name change
**Execution:** agent

Proves end to end, through public packages only, that a plan lists exactly what the following apply destroys, creates and updates, including replacements decided by a plugin and replacements caused by a dependency. The same change is applied once through a built-in plugin and once through the same plugin run as a separate program, and both must give the same plan and the same outcome. Root-package tests check that the public diff and apply methods expose the replacement.

*Technical detail:* [context.md#task-plans-agree-with-applies-for-built-in-and-external-plugins](./context.md#task-plans-agree-with-applies-for-built-in-and-external-plugins)

**Acceptance criteria**:
- [ ] For every e2e scenario, the resources a plan lists per action are exactly those the following apply acts on, with nothing extra and nothing missing
- [ ] The same change through an in-process and an external plugin produces the same plan and the same apply outcome
- [ ] The public diff reports a plugin-decided replacement, and the public apply performs it

### Milestone 3: Plans say why a resource is replaced

#### - [ ] Task: Diff carries and renders the replacement reason
**Id:** 6fa14271-8711-4d1d-8d7d-801eec77c80a
**Repo:** xclconfig
**Depends on:**
- a4febb1d-d732-4579-b6fd-651d403d21e0 — Decide pass tells each resource about its dependencies
**Execution:** agent

Adds the reason for a replacement to the diff result, along with the replaced dependencies behind it, so both code and the JSON form can say why. The rendered plan names the cause on the comment line above the resource, for example "will be replaced because docker.network.app is replaced". The layout and markers stay as they are.

*Technical detail:* [context.md#task-diff-carries-and-renders-the-replacement-reason](./context.md#task-diff-carries-and-renders-the-replacement-reason)

**Acceptance criteria**:
- [ ] A replacement caused by a replaced dependency renders "will be replaced because <dependency> is replaced", naming every replaced dependency
- [ ] A replacement decided by the plugin itself renders "will be replaced, it cannot be updated in place"
- [ ] A replacement of a resource whose last apply failed still renders "will be replaced, its last apply failed"
- [ ] The JSON form of a replacement includes its reason and replaced dependencies, and other actions include neither
- [ ] Values a replaced or updated dependency only learns after the apply render as "(known after apply)"

### Milestone 4: The plugin example rebuilds its network, and the docs explain replacement

#### - [ ] Task: Docker and template providers decide replacements
**Id:** be24536e-59ff-4655-8b6a-6c1be1a5261b
**Repo:** xclconfig
**Depends on:**
- 3b2fa7e8-6be3-41ba-a40f-21932ead4eb7 — Change contract through every plugin layer
**Execution:** agent

Gives the example's providers real change rules:
- The Docker network answers replace when its address range changes.
- The container answers replace when its image, command, environment or networks change, or when a resource it depends on is replaced.
- The template answers replace when its destination changes, and update for its source and variables.

Each rule has its own test, so every setting a provider cannot change in place is shown to answer replace.

*Technical detail:* [context.md#task-docker-and-template-providers-decide-replacements](./context.md#task-docker-and-template-providers-decide-replacements)

**Acceptance criteria**:
- [ ] A network with a different address range is answered as replace
- [ ] A container with a different image, command, environment or network is answered as replace, as is a container whose network is replaced
- [ ] A container whose dependencies are only updated, with an identical configuration, is answered as unchanged
- [ ] A template with a different destination is answered as replace, and one with different source or variables as update

#### - [ ] Task: Plugin example ships its address-range change configuration
**Id:** 863d7ec9-41fe-4245-8ef7-a3f40e2004a3
**Repo:** xclconfig
**Depends on:**
- be24536e-59ff-4655-8b6a-6c1be1a5261b — Docker and template providers decide replacements
- ad317131-f5a2-4b3b-a5eb-e3bbd5230d0c — Act pass destroys first, then creates and updates
- 6fa14271-8711-4d1d-8d7d-801eec77c80a — Diff carries and renders the replacement reason
**Execution:** agent

Moves the second configuration, which changes the network's address range, out of the main configuration directory into its own directory beside it, so the main configuration applies on its own again. The example's tests then show the change end to end against real Docker: the plan marks the network and container as replaced and gives the reason, the apply rebuilds the network on the new range with a new container and a re-rendered template, and a following plan shows no changes.

*Technical detail:* [context.md#task-plugin-example-ships-its-address-range-change-configuration](./context.md#task-plugin-example-ships-its-address-range-change-configuration)

**Acceptance criteria**:
- [ ] Applying the main configuration on its own creates the network, container and template, and the example's existing tests pass
- [ ] The plan for the address-range configuration shows the network replaced, the container replaced because of the network, and the template updated
- [ ] After applying the address-range configuration, the Docker network has the new range, a new container is attached to it, and the template contains the new container's address
- [ ] A plan straight after that apply reports no changes

#### - [ ] Task: Core guides, README and changelog explain replacement
**Id:** cdd6c390-c856-443e-8257-f3354a8358ef
**Repo:** xclconfig
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Updates the project's own guides for plugin authors and maintainers to the new contract. They explain the unchanged/update/replace answer and the dependency list, describe how apply now decides everything and then destroys before creating, and show a plan with a replacement and its reason. The changelog gains an entry for the change that lists the breaking changes to the plugin interface, the protocol and saved state.

*Technical detail:* [context.md#task-core-guides-readme-and-changelog-explain-replacement](./context.md#task-core-guides-readme-and-changelog-explain-replacement)

**Acceptance criteria**:
- [ ] The plugin developer guide explains how to decide unchanged, update or replace using the dependency list, with an example override
- [ ] The lifecycle and state guides describe decide-then-act and the destroy-first order
- [ ] The README's plugin example section shows the address-range plan with its replacements
- [ ] The changelog has an entry for this change naming every breaking change

#### - [ ] Task: Website guide on unchanged, update or replace
**Id:** bc5085bc-0568-4d88-8d95-eb23ae9d3971
**Repo:** xcl-website
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Adds a guide page to the documentation site for plugin authors. It explains how `Changed` answers unchanged, update or replace, what a resource is told about its dependencies, and how apply decides everything before destroying and then creating. The page includes the Docker network and container rules as a worked example, and is linked from the site's Guides menu.

*Technical detail:* [context.md#task-website-guide-on-unchanged-update-or-replace](./context.md#task-website-guide-on-unchanged-update-or-replace)

**Acceptance criteria**:
- [ ] The site has a guide page explaining unchanged, update and replace for plugin authors, reachable from the Guides menu
- [ ] The page shows a provider override that reads its dependency list
- [ ] The site builds

#### - [ ] Task: Website diff and plugin example pages show replacements
**Id:** fee32990-a561-4ded-b71e-6f69966adab9
**Repo:** xcl-website
**Depends on:**
- 863d7ec9-41fe-4245-8ef7-a3f40e2004a3 — Plugin example ships its address-range change configuration
**Execution:** agent

Updates the diff guide so that "replace" covers all three reasons, with a rendered sample that shows a replacement and its reason. Updates the plugin example page to walk through the address-range change configuration: the plan with the network and container replaced, then the apply that rebuilds them. The existing walkthrough misleadingly showed an image change as an update, so it is corrected too.

*Technical detail:* [context.md#task-website-diff-and-plugin-example-pages-show-replacements](./context.md#task-website-diff-and-plugin-example-pages-show-replacements)

**Acceptance criteria**:
- [ ] The diff page explains each reason a resource is replaced and shows a rendered replacement naming its dependency
- [ ] The plugin example page shows the address-range plan and apply with the network and container replaced
- [ ] No page still shows a container image change as an in-place update
- [ ] The site builds

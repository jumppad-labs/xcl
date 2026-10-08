### Milestone 1: Plugins answer unchanged, update or replace

**What changes**: When xcl asks a plugin whether a resource has changed, the plugin now answers that it is unchanged, needs updating in place, or must be replaced, rather than just yes or no. A "replace" answer already destroys the resource and creates it again, using the same path that failed resources take today. Every plugin in the repository, including the examples, the test plugins and plugins that run as separate programs, moves to the new answer in the same step, so the repository builds and every test passes throughout. Plugin authors see the new contract straight away. Dependents are not yet told anything about their dependencies.

**Validation point**: The full test suite, every example's tests and the external test plugins build and pass. A provider answering "replace" through both an in-process and an external plugin causes a destroy followed by a create.

### Milestone 2: Applies decide everything first, tell dependents, and replace in a safe order

**What changes**: Applying now decides the outcome of every resource before it touches any of them. Each resource's plugin is told which of the resources it references will be updated or replaced, and decides its own outcome from that. Applying then destroys everything being replaced or removed, dependents first, and only then creates and updates in dependency order. A failure while deciding changes nothing. A failed replacement is recorded as failed and is retried next time. Plans use the same decision pass, so a plan lists exactly what the apply then does.

**Validation point**: Behavioural tests show the following:
- dependents are told exactly their changing dependencies;
- a dependent may stay unchanged;
- with a network and its container both replaced, the container is destroyed, then the network, then the network is created, then the container;
- a failing decision causes no provider action;
- a failed replacement is planned as a replacement again;
- plan and apply agree in the e2e scenarios.

### Milestone 3: Plans say why a resource is replaced

**What changes**: A plan or diff now says why each replacement happens: its last apply failed, its plugin cannot update it in place, or a resource it depends on is being replaced, naming that resource. For example: `# docker.container.web will be replaced because docker.network.app is replaced`. The same reason is in the diff's Go types and JSON, so tools built on xcl can show or act on it. Values that a replaced or updated dependency only learns after the apply are shown as "(known after apply)".

**Validation point**: Render and JSON tests pass for each reason, and a parser diff of a dependent replacement names the dependency.

### Milestone 4: The plugin example rebuilds its network, and the docs explain replacement

**What changes**: In the plugin example:
- changing the network's address range replaces the Docker network and the container attached to it, and updates the template that reads them, so real Docker matches the configuration;
- the main configuration applies on its own again;
- the address-range change ships as a separate configuration that the documentation uses to show a plan and an apply with replacements.

The project guides and the documentation site explain how plugin authors decide unchanged, update or replace from what they are told about their dependencies, and show how plans display replacements and their reasons.

**Validation point**:
- On a machine with Docker, the example's tests pass and show the new subnet, a new container and a re-rendered template, followed by a plan with no changes.
- Without Docker, the provider rule tests pass.
- The website builds with the new guide reachable from the Guides menu.

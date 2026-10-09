- [ ] **A ready-to-build starting plugin**
  Plugin authors can create a new plugin from the template that builds, passes its tests and applies its sample configuration with no changes and no external service running.
- [ ] **Runs in-process and as a separate program**
  The plugin created from the template can be used both registered inside an application and run as a separate plugin program, from the same layout.
- [ ] **An explicit change decision**
  The template's provider defines its own decision between update and replace, with the settings that need a replace visible and easy to edit, rather than relying only on the built-in default.
- [ ] **Updates work only from what they are told**
  The template's in-place update acts only on the changed settings and dependencies it is given, without extra calls to rediscover the resource's previous state.
- [ ] **How to build, test and register it**
  The template's README and build targets show authors how to build, test, regenerate test doubles, and register the plugin both in-process and as a separate program.
- [ ] **A standard plugin layout**
  The project defines one standard layout for a plugin: where entity types, providers, the backend client and its test doubles, and the plugin's entry points live. The template, the plugin example and the documentation all follow it.
- [ ] **Entity types stand alone**
  An application that reads a plugin's resources from state can use the plugin's entity types without depending on the plugin's providers or on the backend's libraries.
- [ ] **Consistent file order**
  In the template and the plugin example, each file holds its public types and methods together at the top, with the exported provider methods in lifecycle order, followed by unexported helpers.
- [ ] **Tests in the project's style**
  The template includes unit tests of its provider against a strict test double, including the change decision and the in-place update. It also includes an end-to-end test that applies the sample configuration, confirms the next plan reports no changes, and destroys it.
- [ ] **Checked on every change**
  The template repository builds, vets and runs its tests automatically on every change.
- [ ] **The plugin example follows the standard**
  The Docker plugin example is rebuilt to the standard layout and file order with no change in behaviour, and the example application no longer pulls the Docker providers or libraries in only to read entity types.
- [ ] **Kept up to date with the project**
  The template repository is part of the project's registered repositories, so later changes to the plugin contract are planned and made in the template too.
- [ ] **Documentation shows the standard and the template**
  The project's guides and the documentation site explain the standard layout and how to start a plugin from the template, and the plugin example page reflects the example's new layout.

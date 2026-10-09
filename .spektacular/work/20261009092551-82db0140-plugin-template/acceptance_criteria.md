- [ ] **A new plugin builds and applies out of the box**
  A repository created from the template, with nothing changed and no external service running, builds, passes all its tests, applies its sample configuration, then plans with no changes, then destroys cleanly.
- [ ] **The same plugin runs in-process and as a separate program**
  Following the README, the plugin created from the template applies its sample configuration both when registered inside an application and when run as a separate plugin program.
- [ ] **Editing a setting that needs a replace replaces the resource**
  Changing a setting the template marks as needing a replace makes the plan show the resource replaced. Changing any other setting makes it show the resource updated.
- [ ] **The update acts only on what it is told**
  The template's unit tests drive its update against a strict test double that allows only the calls the reported changes require. A call made to rediscover previous state fails the test.
- [ ] **The README covers the whole workflow**
  Following only the README, an author can build, test, regenerate the test doubles, and register the plugin in-process and as a separate program, with every command shown succeeding.
- [ ] **The standard layout is written down**
  The project's guides describe the standard plugin layout. The template and the plugin example both match it, with every location the guide names present in each.
- [ ] **Reading state needs only the entity types**
  The plugin example's application reads the example's resources from state without its build including the Docker providers or the Docker libraries they use. The template's sample shows the same split.
- [ ] **Files list public members first**
  In the template and the plugin example, every source file shows its exported types and methods before any unexported helper. Provider methods appear in the order Init, Create, Read, Changed, Update, Destroy, Functions.
- [ ] **The template checks itself**
  A change pushed to the template repository runs a build, a vet and the full test suite, and a failure in any of them is reported on the change.
- [ ] **The plugin example behaves as before**
  After the rebuild, the plugin example's scenario tests (network swap, subnet rebuild, init-script rebuild and content edit, network removal, dangling reference) pass unchanged, as do its unit tests.
- [ ] **The template is a registered project repository**
  The project's repository list includes the template repository, with a description and role, and a spec or plan in this project can attribute work to it.
- [ ] **The documentation site covers the template**
  The documentation site has a page explaining the standard layout and how to start a plugin from the template. The plugin example page shows the example's new file locations, and the site builds.

- [ ] **Plain data class applies end to end**
  A program registers a data class that has no XCL base class and no class label, then applies, validates, looks up, diffs and destroys a configuration that uses it. Each operation succeeds, and the lookup returns an instance of that class with the configured values.
- [ ] **Type and subtype come from registration**
  A data class registered as type `database` with subtype `postgres` matches a `database "postgres" "main" {}` block and is found at `database.postgres.main`. Registered as type `app` with no subtype, it matches `app "web" {}` and is found at `app.web`. The same holds when the class is registered on a GitHub registry.
- [ ] **Removed API fails with guidance**
  Importing the old base class or the old class label from the package raises an error. The error message names the replacement: a plain data class with the type name given at registration.
- [ ] **Metadata only when declared**
  When a class declares a metadata field, a looked-up instance has it set to the block's address, name, type, file and line. When a class doesn't declare one, the same configuration applies and looks up without error, and the instance has no metadata attribute.
- [ ] **Disabled and dependencies mirror configuration**
  For a block with `disabled = true` and `depends_on = ["x.y"]`, a class that declares those fields reads them back as true and `["x.y"]`. A class that doesn't declare them accepts the same block without a validation error.
- [ ] **Untyped lookup returns the registered class**
  Listing all entities, or finding one without naming a class, returns instances of the class registered on that configuration for each type. A plugin type with no registered class comes back as the generic entity.
- [ ] **Two configurations, two classes**
  Two configurations in one process register different classes for type `app`. An untyped lookup of `app.web` on each returns an instance of that configuration's own class.
- [ ] **Event entity uses the configuration's class**
  An event handler on a configuration that carries entity data on its events gets back, from the event's entity, an instance of the class registered on that configuration.
- [ ] **Errors are XCL errors**
  Registering a class that isn't a data class, registering with no type name, and looking up with an unregistered class or with no class where one is required each raise an exception under the package's base error class. The message names the class or the missing type name.
- [ ] **Unmarked fields infer their shape**
  A data class with no field markup, holding a required string, a defaulted integer, an optional string, a nested data class, a list of data classes and a dict of data classes, validates configuration as a required attribute, an optional attribute with a default, an optional attribute, a nested block, a repeated block and a map of named blocks respectively. A field marked with a configuration name of `image` on the attribute `images` reads `image = "…"` from configuration.
- [ ] **Python and Go schemas match**
  The comparison harness passes for every shared fixture, with the Python side using plain data classes registered by name and the Go side using plain structs.
- [ ] **No removed API in the docs**
  Searching the Python package's README and the website's Python examples finds no use of the removed base class or class label, and each Python example registers its types with a type name.
- [ ] **Release notes cover the migration**
  The release notes contain a breaking-change entry naming the removed base class and label, with a before-and-after example of migrating one class.

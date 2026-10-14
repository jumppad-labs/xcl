- [ ] **Plain data classes as configuration types**
  Users can register an ordinary data class as a block type, with no XCL base class and no XCL label on the class.
- [ ] **Type named at registration**
  Users give the block's type, and optionally its subtype, when they register the class on a local registry or a GitHub registry. The class itself carries no type name.
- [ ] **Old base class and label removed**
  The XCL base class and the class label are no longer part of the Python package. Code that uses them fails with an error that tells the user how to register a plain data class instead.
- [ ] **Block metadata on request**
  A class that declares a metadata field receives the block's metadata (its address, name, type, file and line) on every lookup. A class that doesn't declare one works the same in every other respect.
- [ ] **Disabled and dependencies on request**
  A class can declare fields for a block's disabled flag and its explicit dependencies, and they hold the values set in configuration. Configuration can set both on any block whether or not the class declares them.
- [ ] **Untyped lookups use the configuration's own types**
  A lookup that doesn't name a class returns an instance of the class registered on that configuration for the entity's type. An entity whose type has no registered class is returned as the generic entity.
- [ ] **Configurations are independent**
  Two configurations in one process can register different classes for the same type name, and each configuration's lookups return its own class.
- [ ] **Events hydrate with the configuration's types**
  An entity carried on an event comes back as the class registered on the configuration that emitted it.
- [ ] **Clear errors for bad registrations and lookups**
  Registering something that isn't a data class, registering without a type name, or looking up with a class that isn't registered raises an XCL error naming the problem, not an unrelated Python error.
- [ ] **Field markup stays optional**
  A field's configuration name, and whether it's an attribute, an optional attribute, a nested block, a repeated block or a map of named blocks, all still come from the field's name, type and default. Users need field-level markup only to rename a field, to treat a data class field as an object value instead of a block, or to mark a block's name label.
- [ ] **Same schema as Go**
  For each shared comparison fixture, a Python plain data class and a Go plain struct describing the same block type produce the same schema.
- [ ] **Documentation shows plain data classes**
  The Python package's documentation and every Python example on the website use plain data classes with type names given at registration, and none mention the removed base class or label.
- [ ] **Breaking change recorded**
  The release notes record the removal as a breaking change and show how to migrate an existing class.

---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
designs:
    - source: design
      path: plugin-layout.md
epic: 20261010160411-19b965db-language-reference
---

# Feature: 20261011095738-fad05e41-language-surface

## Overview

XCL currently exposes everything its HCL-derived parser happens to support, and in several places it doesn't do what its documentation says: overriding variables from the environment does nothing, applications have no way to add their own functions, plugins can't offer functions to configuration, and named child blocks can't be collected by name. This spec settles what the XCL language is: a declarative language that keeps references, expressions and functions, drops string templating from the core, and collects named child blocks by name. It also fixes the library to match, so the language reference can describe real behaviour. It benefits people writing XCL configuration, developers embedding XCL in their applications, and plugin authors who want to offer functions to configuration.

## Requirements

- [x] **Environment overrides use the XCL prefix**
  Users can override a variable's value by setting an environment variable named with the `XCL_VAR_` prefix followed by the variable's name.
- [x] **No accidental environment overrides**
  Environment variables that don't carry the `XCL_VAR_` prefix never become variables or override one.
- [x] **Applications can register their own functions**
  Developers embedding XCL can make a function of their own callable from configuration, the same way they make an entity type available, without writing a plugin.
- [x] **Plugins can offer functions**
  Plugin authors can offer functions from a plugin, and configuration can call them.
- [x] **Functions are evaluated while configuration is read**
  Every function call in configuration, whether built-in, application-registered or plugin-hosted, is evaluated while the configuration is read. Its result is filled into the entity before the entity reaches any plugin lifecycle, so a plugin only ever sees the resulting value.
- [x] **Function name clashes are reported**
  The system reports an error, naming both sources, when two functions would share a name: two plugins, a plugin and an application function, or either of them and a built-in.
- [x] **Named child blocks collect into a map**
  When the Go field is a map, named child blocks of the same type are collected into it, keyed by each block's name.
- [x] **Unnamed blocks can't go into a map**
  A child block without a name, written where a map is expected, is reported as a configuration error that points at the block.
- [x] **Duplicate block names are reported**
  Two child blocks of the same type with the same name inside one entity are reported as a configuration error.
- [x] **Lists accept named and unnamed blocks**
  When the Go field is a list, child blocks are collected whether or not they have a name, and a block's name is kept when the Go type has a field for the name.
- [x] **Unsupported field types are errors, not crashes**
  A Go field that can't hold child blocks is reported as an error, never as a crash of the embedding application.
- [x] **Named blocks can be referenced by name**
  Configuration can refer to a value inside a named child block by the block's name, where that block is collected into a map.
- [x] **Optional fields are honoured inside map and object values**
  Fields a Go type marks optional can be left out of map and object values written as attributes, just as they can in blocks.
- [x] **References inside `for` expressions resolve**
  An entity referenced inside a `for` expression is recognised as a dependency and its values are filled in before the expression is evaluated.
- [x] **Template directives are rejected**
  Configuration that uses string template directives (`%{ … }`) is rejected with an error explaining that templating isn't part of XCL and pointing to functions instead. References in strings (`${ … }`) keep working.
- [x] **Variables can be built from other values**
  A variable's default can refer to other values in the configuration and is resolved in dependency order, so no separate `local` block is needed.
- [x] **Editor highlighting matches the language**
  The XCL editor extension no longer highlights template directives as valid syntax.
- [x] **The site matches the language**
  Pages and feature cards on xcl.dev, and the xcl README, that describe these behaviours (the environment prefix, the function count, functions, templating) state what the library now does.
- [x] **The plugin template supports functions**
  The plugin template shows how a plugin offers a function, and the example Docker plugin builds against the changed plugin interface.

## Constraints

- Plugins, and the functions they host, are written in Go only for now.
- A plugin-hosted function runs inside its plugin and is called over the plugin's gRPC protocol. It is never re-implemented or run on the host side.
- Configuration reaches into a module only through the module's outputs. The existing module boundary stays exactly as it is.
- Breaking changes to the language and the Go API are allowed, and each one is listed under **Breaking:** in the changelog. No deprecation period or compatibility shims are needed.
- The plugin template and the example Docker plugin keep to the standard plugin layout settled in the `design` source at `plugin-layout.md`.

## Acceptance Criteria

Unless a criterion says otherwise, criteria refer to this entity:

```hcl
app "web" {
  image = "nginx:1.27"
  host  = "web.local"

  listener "http" {
    port = 80
  }

  listener "https" {
    port = 443
  }
}
```

- [x] **Prefixed override applies**
  With `variable "region" { default = "eu" }` and `XCL_VAR_region=us` set, the applied entity that uses the variable shows `us`.
- [x] **Unprefixed and old-prefix variables are ignored**
  With the same config and `region=us` or `HCL_VAR_region=us` set (but no `XCL_VAR_region`), the entity shows `eu`. Referring to `variable.PATH` in config is reported as an unknown variable.
- [x] **Application function callable**
  An application registers a function named `slugify` without any plugin. Config `name = slugify("My App")` applies, and the entity holds the function's result.
- [x] **Plugin function callable**
  A plugin built from the plugin template offers a function. Config that loads the plugin and calls the function applies, and the entity holds the function's result.
- [x] **Plugin receives the result, not the call**
  When an entity handled by a plugin has an attribute set by a function call, the value the plugin receives in its create step is the function's result.
- [x] **Function clash reported**
  Registering an application function named `upper` (a built-in), registering an application function with the same name as a function a loaded plugin offers, or loading two plugins that offer the same function name, makes creating the config fail with an error naming the function and both sources.
- [x] **Named blocks fill a map**
  `listener "http" { port = 80 }` and `listener "https" { port = 443 }`, read into a map field, give a map with keys `http` and `https` holding ports 80 and 443.
- [x] **Unnamed block into a map fails**
  `listener { port = 80 }` read into a map field fails validation with an error at that block's line saying a name is required.
- [x] **Duplicate name fails**
  Two `listener "http"` blocks in one entity fail validation with an error naming `http` and pointing at the second block.
- [x] **List takes both kinds**
  `listener "http" { port = 80 }` and `listener { port = 8080 }`, read into a list field whose type keeps the name, give two items: one named `http` and one with an empty name.
- [x] **No crash on a bad field type**
  Registering a type whose child-block field is a type that can't hold blocks (for example a string) returns an error from creating or validating the config. The application doesn't panic.
- [x] **Reference by name works**
  With listeners collected into a map, another entity's `port = app.web.listener.https.port` applies and holds 443.
- [x] **Optional field omitted in a map value**
  `listeners = { http = { port = 80 } }`, where the Go type marks `tls` optional, applies with `tls` left at its zero value.
- [x] **for over an entity resolves**
  With listeners collected into a map, `ports = { for name, l in app.web.listener : name => l.port }` applies and holds `{ http = 80, https = 443 }`.
- [x] **Template directive rejected**
  A string containing `%{ if … }` or `%{ for … }` fails validation with an error at its position that mentions functions. A string containing `${app.web.image}` still applies with the value filled in.
- [x] **Variable default refers to an entity**
  `variable "url" { default = "http://${app.web.host}" }`, used by another entity, applies with the URL built from the applied `app.web.host`.
- [x] **Highlighter drops directives**
  In the XCL editor extension, `%{ … }` inside a string is no longer coloured as template syntax. `${ … }` still is.
- [x] **Site claims are correct**
  On xcl.dev and in the xcl README, nothing mentions `HCL_VAR_` or "around sixty" functions. The environment prefix shown is `XCL_VAR_`, and any function count matches the number of built-ins.
- [x] **Template and example plugin build**
  The plugin template builds with a working example function, and the example Docker plugin builds and passes its tests against the changed plugin interface.

## Technical Approach

- An application registers a function the same way it registers an entity type, so the two look alike to a Go developer. The planner chooses the exact surface, for example the registry or a config option.
- Plugin-hosted functions should build on the plugin function interfaces that already exist but were never wired up, rather than introduce a second mechanism.
- Function names stay bare, like the built-ins, with no namespacing. A clash is reported when the config is created, in the same place type clashes are reported today.
- The HCL-derived parser is a stopgap that XCL is expected to move away from. Language changes, such as rejecting template directives and collecting named blocks, should be made in XCL's own handling of the language rather than bolted on around it, so they survive a future parser change.
- Documentation leads with named blocks.

## Success Metrics

- The language reference can document every feature it covers as working behaviour, with no "not yet supported" caveats for environment overrides, application functions, named blocks into maps, and references inside `for`.
- Every claim on xcl.dev and in the README about the environment prefix, functions and templating matches what the latest release does when tried.
- No configuration or Go type can make the library panic while reading block definitions. Every such problem reaches the user as an error pointing at a position.
- A plugin author can add a working function to a plugin built from the template by following the template alone.

## Non-Goals

- No `for_each`, dynamic blocks or any other way of generating blocks from a value. This could be reconsidered later if the community asks for it.
- No `local` block.
- No templating function or plugin is provided.
- No removal of map and object attributes, and no warning when they're used. They're only not encouraged in documentation.
- No other changes to the expression language: operators, conditionals, splat, indexing, `for` and the built-in functions behave as they do today, apart from the `for` reference fix.
- The language reference and the "Using XCL with Go" guide aren't written here. They're covered by their own specs in the epic. This spec only corrects existing site claims.
- No API rename (for example `types.ResourceBase` → entity naming).
- No formatting support in the editor extension. Its only change here is dropping template directive highlighting.

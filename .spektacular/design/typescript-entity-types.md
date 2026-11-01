---
created_date: "2026-10-31"
document_status: draft
spec: 20261031080136-264e0eab-typescript-core-sdk
specs:
    - 20261031080136-264e0eab-typescript-core-sdk
---

# TypeScript SDK: defining entity types

**Status:** proposal, for review  
**Decision needed:** how a TypeScript program describes its xcl entity types  
**Proposal:** decorated entity classes (Option 2)

The TypeScript SDK (`xcl-lang` on npm) runs the same engine as Go and Python. A program registers its entity types, the engine checks configuration against them, and lookups return typed values. This document settles how those types are written in TypeScript. Everything else about the SDK (operations, lookups, events, state, packaging) follows the Python package and is covered by the SDK's main design.

## Contents

1. [The problem](#the-problem)
2. [How comparable tools solve it](#how-comparable-tools-solve-it)
3. [The example](#the-example)
4. [Proposal: decorated entity classes](#proposal-decorated-entity-classes)
5. [Alternatives considered](#alternatives-considered)
6. [Comparison](#comparison)
7. [Open questions](#open-questions)

## The problem

xcl needs an entity type's shape **at runtime**. It sends a schema to the engine, which checks configuration against it, and it turns the engine's JSON back into typed values.

TypeScript types are erased when the code is compiled, so an interface gives xcl nothing to read:

```ts
interface App {
  image: string;
  replicas: number;
}
```

When the program runs there is no `App` left. Every TypeScript library that needs a type at runtime solves this in one of four ways:

| Approach | Used by | Where the TypeScript type comes from |
|---|---|---|
| Decorated classes | TypeORM, MikroORM, Typegoose, NestJS | The class is the type; decorators carry the extra facts |
| Runtime builder | Drizzle ORM, Zod, Mongoose, TypeBox | Inferred from the builder |
| Schema file with code generation | Prisma, GraphQL Code Generator, protobuf | Generated from the schema file by a CLI |
| Compiler transform | typia | Read from plain interfaces by a compiler plugin |

## How comparable tools solve it

### ORMs are the closest analogue

xcl entity types are closest to ORM entities. Each has:
- a name;
- labels that identify it;
- fields with defaults;
- values computed by something other than the user;
- sensitive values;
- references to other blocks.

ORMs have the same kinds of concepts (tables, keys, nullability, database defaults, relations).

None of the major ORMs define entities with a general validation library such as Zod. Zod describes what a valid plain value looks like; it has no vocabulary for a primary key, a generated column or a relation. Each ORM instead has a domain-specific way to declare entities, and the most established is the decorated class:

```ts
import { Column, Entity, PrimaryGeneratedColumn } from "typeorm";

@Entity("users")
export class User {
  @PrimaryGeneratedColumn()
  id!: number;

  @Column()
  name!: string;

  @Column({ nullable: true })
  age?: number;
}
```

Where Zod appears alongside an ORM, it is derived from the entity, not used to write it: `drizzle-zod` builds a Zod schema from a Drizzle table, and Prisma has generators that do the same. The entity is defined in the domain's terms, and Zod validates untrusted input against it.

### Where Zod sits in a React application

React has no runtime types: component props are TypeScript types and vanish at compile time. Zod appears in React applications at boundaries, where data arrives from somewhere the compiler cannot see:

- form input, through `react-hook-form` and `zodResolver`;
- responses from an API;
- environment variables;
- arguments to server actions.

The application's data model comes from its ORM. xcl entity types are a data model, not a boundary, which is why they fit the ORM approaches better than Zod.

## The example

Every option below defines the same types:

- `app`, with:
  - a required `image`;
  - `replicas`, an integer defaulting to 2;
  - an optional `region`;
  - an optional, sensitive `password`;
  - an `endpoint` computed by a provider;
  - an optional nested `health` block;
  - a list of `volume` blocks;
  - named `listener` blocks;
  - the block's metadata.
- `database "postgres"`, a type with a subtype.
- `slugify`, a function callable from configuration.

The configuration it reads:

```hcl
app "web" {
  image    = "nginx:1.27-alpine"
  password = "s3cret"

  health {
    path = "/healthz"
  }

  volume "data" {
    source = "/var/lib/app"
  }

  listener "http" {
    port = 8080
  }
}

database "postgres" "main" {
  location = "eu-west-1"
}
```

## Proposal: decorated entity classes

An entity type is a class. `@Entity` names its type and subtype, and a decorator on each field says what it is in configuration. Lookups return instances of the class.

### Defining types

```ts
import {
  Attribute,
  Block,
  Blocks,
  Computed,
  Entity,
  Int,
  Label,
  Meta,
  NamedBlocks,
  Sensitive,
} from "xcl-lang";

export class Health {
  @Attribute(String)
  path = "/";

  @Attribute(Int)
  interval = 10;
}

export class Volume {
  @Label()
  name!: string;

  @Attribute(String)
  source!: string;
}

export class Listener {
  @Label()
  name!: string;

  @Attribute(Int)
  port!: number;
}

@Entity("app")
export class App {
  @Meta()
  meta!: Meta;

  @Attribute(String)
  image!: string;

  @Attribute(Int)
  replicas = 2;

  @Attribute(String, { optional: true })
  region?: string;

  @Sensitive(String, { optional: true })
  password?: Sensitive<string>;

  @Computed(String)
  endpoint?: string;

  @Block(() => Health, { optional: true })
  health?: Health;

  @Blocks(() => Volume)
  volume: Volume[] = [];

  @NamedBlocks(() => Listener)
  listener: Record<string, Listener> = {};
}

@Entity("database", "postgres")
export class Postgres {
  @Meta()
  meta!: Meta;

  @Attribute(String)
  location!: string;

  @Attribute(Int)
  port = 5432;
}
```

### The rules

| Field | Meaning in configuration |
|---|---|
| No initializer (`image!: string`) | Required |
| An initializer (`replicas = 2`) | Optional, with the initializer as its default |
| `{ optional: true }` and no initializer | Optional, with no default |
| `@Attribute(String)`, `Number`, `Boolean`, `Int` | A string, number, bool or integer attribute |
| `@Attribute([String])` | A list attribute |
| `@Attribute({ map: String })` | A map attribute |
| `@Sensitive(T)` | A sensitive attribute: masked in events and state |
| `@Computed(T)` | Set by a provider, never in configuration |
| `@Block(() => T)` | A nested block |
| `@Blocks(() => T)` | A list of blocks, in order |
| `@NamedBlocks(() => T)` | Named blocks, keyed by their label |
| `@Label()` | Receives the block's label; a type with a subtype takes two |
| `@Meta()` | Receives the block's metadata: its address, name, type and so on |

- **Types come from JavaScript's own constructors** (`String`, `Number`, `Boolean`), the convention Mongoose and Typegoose use, with `Int` from xcl for integers.
- **Defaults come from initializers.** xcl reads them by constructing the class once at registration, so an entity class takes no constructor arguments. This matches Python, where a dataclass field's default makes it optional.
- **Block types are passed as functions** (`() => Health`), as TypeORM does for relations, so a class can refer to one declared further down the file.
- **Field names are camelCase.** `connectionString` is written `connection_string` in configuration, converted by one rule in both directions.
- **A type with no labels is a singleton.** A type with neither a `@Label()` field nor a `@Meta()` field cannot read a label, so its block is written without one and it is found by its type alone, as in Go and Python.

### Registering and reading

```ts
import { createConfig, LocalRegistry } from "xcl-lang";
import { App, Postgres } from "./types.js";

const local = new LocalRegistry();
local.register(App, Postgres);

await using config = await createConfig({
  registries: [local],
  statePath: ".xcl",
});

await config.apply("./infra");

const web = await config.find(App, "app.web");

console.log(web instanceof App);   // true
console.log(web.image);            // "nginx:1.27-alpine"
console.log(web.replicas);         // 2
console.log(web.meta.id);          // "app.web"
console.log(`${web.password}`);    // "(sensitive)"
```

`find`, `findByType`, `findOne` and `all` take the class and return instances of it. An entity whose type has no registered class, such as one from a Go plugin, comes back as a generic `Entity` with its metadata and attributes.

### Hand-built entities

Because the class carries its type, an entity built in code can be encoded as configuration without saying what it is:

```ts
const web = new App();
web.image = "nginx:1.27-alpine";
web.meta = { name: "web" };

console.log(await config.encodeEntity(web));
```

```hcl
app "web" {
  image    = "nginx:1.27-alpine"
  replicas = 2
}
```

### Functions

A function callable from configuration is a method with a `@Func` decorator giving its parameter and return types:

```ts
import { Func } from "xcl-lang";

export class Functions {
  @Func([String], String)
  slugify(title: string): string {
    return title.toLowerCase().replaceAll(" ", "-");
  }

  @Func([String], String)
  async resolve(host: string): Promise<string> {
    const { address } = await lookup(host);
    return address;
  }
}

local.registerFunctions(new Functions());
```

```hcl
app "web" {
  image  = "nginx:1.27-alpine"
  region = slugify("US East")
}
```

The method name is the function's name in configuration; `@Func([String], String, { name: "slug" })` overrides it.

### Why repeat the type in the decorator

Each field states its type twice, `@Attribute(String)` and `: string`, because modern TypeScript decorators receive no type information at runtime.

TypeORM avoids the repetition with the older `experimentalDecorators` and `emitDecoratorMetadata` compiler settings, which record a field's type for decorators to read. They are not a good foundation here:

- they record only simple types: an array's element type and an optional field's type are lost;
- esbuild, and so tsx and Vite, cannot emit the metadata, so the types would silently be missing in a large share of projects;
- they are the legacy decorator model; TypeScript's standard decorators, the default since TypeScript 5.0, do not use them.

The constructor style keeps the repetition short. xcl checks at registration that a field's decorator and its initializer agree (an `@Attribute(Int)` field initialised with `"2"` is refused), which catches the common drift.

### Compiler support

The decorators follow the TC39 standard that TypeScript 5.0 and later compile without any setting, and that esbuild, SWC and Bun support. Many existing projects, NestJS ones in particular, still compile with `experimentalDecorators`, which calls decorators in the legacy form. xcl's decorators should accept both forms, so the SDK works in either kind of project without the user changing a setting.

## Alternatives considered

### Option 1: entity builder

A builder of xcl's own, in the style of Drizzle's table builder, with the TypeScript type inferred from it:

```ts
import { block, entity, int, label, string, type InferEntity } from "xcl-lang/schema";

export const volume = block({
  name: label(),
  source: string(),
});

export const app = entity("app", {
  image: string(),
  replicas: int().default(2),
  password: string().sensitive().optional(),
  endpoint: string().computed(),
  volume: volume.list(),
});

export type App = InferEntity<typeof app>;
```

The type is declared once, with no build step or compiler setting. But types read as builder expressions rather than declarations, lookups return plain objects rather than instances, and the TypeScript type is a computed one that has to be named separately.

### Option 3: schema files with generated types

Types declared in xcl's own syntax, in the style of Prisma's schema language, with a CLI generating the TypeScript:

```hcl
entity "app" {
  image    = string
  replicas = optional(int, 2)
  password = optional(sensitive(string))
  endpoint = computed(string)
  volume   = list(volume)
}
```

```sh
npx xcl-lang generate types/ --out src/types.xcl.ts
```

This is language-neutral, so one schema could serve Go, Python and TypeScript. But it needs a schema-file feature in the xcl core that does not exist today, adds a generate step to every project, and gives Go and Python a third way to define types. It is worth revisiting on its own merits later, not as a prerequisite for the TypeScript SDK.

### Zod

Zod schemas, with xcl's markers attached:

```ts
export const App = z.object({
  image: z.string(),
  replicas: z.number().int().default(2),
  password: sensitive(z.string()).optional(),
  endpoint: computed(z.string()),
  volume: z.array(Volume).default([]),
  listener: z.record(z.string(), Listener).default({}),
  meta: meta(),
});

export type App = z.infer<typeof App>;
```

It describes xcl's concepts in a general validation library's terms: named blocks become `z.record(z.string(), ...)`, labels and metadata become fields, and the type name is given separately at registration. As with the ORMs, a `toZod()` helper can derive a Zod schema from an entity class later, for anyone validating input with Zod.

## Comparison

| | Decorated classes | Entity builder | Schema files | Zod |
|---|---|---|---|---|
| Familiar from | TypeORM, MikroORM | Drizzle | Prisma | Zod |
| Speaks xcl's vocabulary | Yes | Yes | Yes | No |
| Reads as a type declaration | Yes | No | Yes | No |
| Lookups return class instances | Yes | No | No | No |
| Hand-built entities know their type | Yes | No | Yes | No |
| Type stated once | No | Yes | Yes | Yes |
| Build step | No | No | Yes | No |
| New dependency for users | No | No | No | Zod |
| Changes needed in the xcl core | No | No | Yes | No |

## Open questions

1. **Instances or plain objects.** Lookups returning class instances is the point of the proposal, but it means a class must be constructible with no arguments. Is that acceptable for every entity type we expect?
2. **Both decorator forms.** Supporting both standard and legacy decorators makes the SDK work in NestJS-style projects unchanged, at the cost of a second code path in each decorator. Is that worth it for the first release?
3. **Decorator names.** `@Attribute`, `@Block`, `@Blocks`, `@NamedBlocks` and `@Func` are proposals. `@Block`, `@List` and `@Map` would be shorter; `@Attribute` could be `@Field`.
4. **Sensitive values.** The proposal types a sensitive field as `Sensitive<string>`, whose text form is `(sensitive)` and whose `reveal()` returns the value, as in Python. Typing it as plain `string` would be simpler to use but easier to leak into a log.

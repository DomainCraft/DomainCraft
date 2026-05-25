# DomainCraft

**Define your domain model once in YAML. Get fully working code for any language.**

DomainCraft is a domain-driven code generator. You describe your entities, relations, permissions, and business rules in a single `domain.yaml` file, and DomainCraft produces a complete, production-ready project through pluggable **bridge** templates.

```yaml
# domain.yaml -- that's all you write
project:
  name: MyShop

entities:
  Product:
    features: [audit, soft_delete]
    fields:
      id: uuid [primary]
      title: string [required, min:3, max:200]
      price: decimal [required, gte:0]
      categoryId: relation(Category) [optional, on_delete:set_null]
    permissions:
      read: ["*"]
      create: [Admin]
      update: ["@Owner", Admin]
      delete: [Admin]
```

```bash
# That's all you run
domaincraft generate
```

The output is a complete, compilable project -- entities, repositories, controllers, configurations, Docker setup, everything.

## Installation

**Go** (any OS):

```bash
go install github.com/DomainCraft/DomainCraft/cmd/domaincraft@latest
```

**Or** download the binary for your platform from [Releases](https://github.com/DomainCraft/DomainCraft/releases), rename to `domaincraft` (or `domaincraft.exe` on Windows), and place it anywhere in your PATH.

**From source** (for contributors):

```bash
git clone https://github.com/DomainCraft/DomainCraft.git
cd DomainCraft
make install
```

That's it. Run `domaincraft` to get started.

## Quick Start

### Create a new project

```bash
domaincraft new
```

The interactive wizard guides you through:
- Project name
- Target bridge (C#, Java, TypeScript, ...)
- Database engine
- Authentication method
- API style

It creates a starter `domain.yaml` that you can edit to define your entities.

### Generate code

```bash
# Interactive -- select bridge from a menu
domaincraft generate

# Or specify directly
domaincraft generate --bridge csharp-restful --output ./generated

# Or use a local bridge directory
domaincraft generate --bridge ../my-bridge --output ./generated
```

### Validate

```bash
domaincraft validate --domain domain.yaml
```

## Why This Approach

**The problem:** Every new project starts with the same boilerplate -- CRUD endpoints, database configurations, authentication wiring, validation, permissions. Days of repetitive work that's error-prone and boring.

**The solution:** Describe *what* your domain looks like, not *how* to implement it. DomainCraft translates your domain model into idiomatic code for your target stack, handling all the plumbing automatically.

| Traditional Approach | DomainCraft |
|---------------------|-------------|
| Write 50+ files per entity by hand | Define 1 entity in ~10 lines of YAML |
| Repeat CRUD logic for every project | Generate it consistently every time |
| Fix bugs in boilerplate across projects | Bugs fixed in templates, fixed everywhere |
| Switch languages? Rewrite everything | Switch bridges, keep the same domain |
| Permissions scattered across codebase | Declared alongside entities, wired end-to-end |

## How It Works

```
domain.yaml --> Parser --> Lexer --> Validator --> IR Builder --> Renderer --> Generated Code
```

1. **Parser** reads your YAML and builds a structured schema
2. **Lexer** parses field definitions like `"string [required, max:255]"` into typed objects
3. **Validator** catches logical errors (missing primary keys, broken relations, invalid configurations)
4. **IR Builder** creates a fully linked intermediate representation with bidirectional relations
5. **Renderer** applies bridge templates to the IR and writes files to disk

The **Intermediate Representation (IR)** is the key design decision. It's a language-agnostic graph of your domain that templates consume. This means the core never needs to know about C#, Java, TypeScript, or any other language -- bridges handle all language-specific concerns.

## What You Can Define

### Fields and Types

```yaml
# Primitives
email: string [required, unique, email]
age: int [optional, gte:0, lte:150]
price: decimal [required, gte:0, default:0]
isActive: boolean [default:true]
createdAt: datetime [default:now()]

# Complex types
metadata: json
bio: text
avatar: url
ipAddress: ipv4
tags: array(string)
```

### Relations

```yaml
# Many-to-One (most common)
authorId: relation(User) [required, on_delete:restrict]

# One-to-One (unique)
profileId: relation(Profile) [unique, on_delete:cascade]

# One-to-Many (declared on the "one" side)
items: relation(OrderItem) [many]

# Many-to-Many (declared on either side)
tags: relation(Tag) [many]

# Self-referential
parentId: relation(Category) [optional, on_delete:set_null]
```

### Delete Behaviors

```yaml
cascade     # Delete dependents when parent is deleted
set_null    # Set FK to NULL (requires [optional])
restrict    # Block parent deletion if dependents exist
no_action   # Let the database handle it
```

### Entity Features (Auto-injected Fields)

```yaml
features:
  - audit              # createdAt, updatedAt
  - audit_log          # createdBy, updatedBy (uuid)
  - soft_delete         # deletedAt (nullable datetime)
  - optimistic_lock     # version (int, concurrency control)
```

### Permissions (RBAC + ABAC)

```yaml
permissions:
  read: ["*"]                    # Public
  create: [User, Admin]          # Role-based
  update: ["@Owner", Admin]      # Ownership-based
  delete: [Admin]                # Admin-only
```

### Indexes

```yaml
indexes:
  - fields: [categoryId, status]
    type: btree
  - fields: [slug]
    unique: true
```

### Seed Data

```yaml
seed:
  - { name: "Electronics", slug: "electronics", isActive: true }
  - { name: "Books", slug: "books", isActive: true }
```

## Bridge System

A **bridge** is a directory containing Go templates and configuration that tells DomainCraft how to generate code for a specific language and framework. Bridges are completely decoupled from the core -- you can create your own without modifying any Go code.

### Available Bridges

| Bridge | Language/Framework | Status |
|--------|-------------------|--------|
| [csharp-restful](https://github.com/DomainCraft/domaincraft-bridge-csharp) | C# / ASP.NET Core / EF Core / PostgreSQL | Ready |
| domaincraft-bridge-java | Java / Spring Boot | Planned |
| domaincraft-bridge-typescript | TypeScript / Express / Prisma | Planned |

List installed and available bridges:

```bash
domaincraft bridges
```

### Use a Bridge

```bash
# By registry ID (auto-downloads and caches)
domaincraft generate --bridge csharp-restful

# By local path
domaincraft generate --bridge ./my-bridge

# By GitHub shorthand
domaincraft generate --bridge DomainCraft/domaincraft-bridge-csharp
```

Bridges from the registry are cached in `~/.domaincraft/bridges/`.

### Create Your Own Bridge

See [CONTRIBUTING.md](./CONTRIBUTING.md) for a complete step-by-step guide on creating bridges for any language.

## Project Structure

```
DomainCraft/
├── cmd/
│   ├── parser/             # CLI entry point (Cobra)
│   │   ├── main.go
│   │   └── commands.go     # new, validate, generate, bridges commands
│   └── schema-gen/         # JSON schema generator for IDE autocomplete
├── internal/
│   ├── parser/             # YAML parsing -> ParsedSchema
│   ├── lexer/              # Field string parsing -> FieldDefinition
│   ├── validator/          # Logical consistency checks
│   ├── ir/                 # Intermediate Representation builder
│   ├── renderer/           # Template rendering engine
│   ├── bridge/             # Bridge registry and resolver
│   └── interactive/        # Interactive CLI prompts (huh)
├── scripts/
│   └── install.sh          # One-liner installer
├── pkg/
│   └── logger/             # Console output formatting
├── spec/
│   └── domain.schema.json  # JSON Schema for domain.yaml (auto-generated)
├── examples/
│   └── domain.yaml         # E-commerce example
├── Makefile
└── CONTRIBUTING.md
```

## CLI Reference

```
domaincraft new              # Create a new project (interactive wizard)
domaincraft generate         # Generate code from domain.yaml
domaincraft validate         # Validate domain.yaml
domaincraft bridges          # List available bridges

# Flags
--domain, -d     Path to domain.yaml (default: domain.yaml)
--bridge, -b     Bridge ID, path, or owner/repo
--output, -o     Output directory (default: generated)
--non-interactive  Disable interactive prompts (for CI/scripts)
--name           Project name (for 'new' command)
--database       Database type (postgresql, mysql, sqlite, mssql, mongodb)
--auth           Auth type (jwt, none)
--api-style      API style (rest, graphql, grpc)
```

## For Developers

If you want to work on DomainCraft itself:

```bash
git clone https://github.com/DomainCraft/DomainCraft.git
cd DomainCraft
make build          # Build binary to bin/domaincraft
make test           # Run all tests
make lint           # go vet
make fmt            # gofmt
```

### Programmatic Usage

```go
package main

import (
    "os"
    "domaincraft/internal/parser"
)

func main() {
    data, _ := os.ReadFile("domain.yaml")
    schema, _ := parser.ParseYAML(data)

    for _, entityName := range schema.EntityOrder {
        entity := schema.Entities[entityName]
        println("Entity:", entity.Name, "->", entity.NamePlural)

        for _, fieldName := range entity.FieldOrder {
            field := entity.Fields[fieldName]
            println("  -", field.Name, ":", field.Type)
        }
    }
}
```

## Full Example

See [`examples/domain.yaml`](./examples/domain.yaml) for a complete e-commerce domain with:
- 9 entities (User, Product, Category, Order, OrderItem, Tag, Review, Document, Folder)
- Enums, self-referential relations, many-to-many
- All feature types (audit, soft_delete, optimistic_lock)
- Complex RBAC permissions with ownership
- Composite indexes, seed data

## License

MIT

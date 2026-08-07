# Contributing to DomainCraft

This guide applies to the core parser and to any bridge template repository, regardless of target language.

## Getting Started

### Prerequisites

- Go 1.25+
- Git

### Clone and Build

```bash
git clone https://github.com/your-org/domaincraft.git
cd domaincraft
make install-deps   # go mod download && go mod tidy
make build          # builds to bin/domaincraft
```

### Run Tests

```bash
make test               # all tests
make test-verbose       # verbose output
make test-coverage      # generates coverage.html
make lint               # go vet ./...
make fmt                # go fmt ./...
```

### Validate a Domain YAML

```bash
make cli-validate DOMAIN=path/to/domain.yaml
# or directly:
go run ./cmd/domaincraft validate --domain domain.yaml
```

### Generate Code

```bash
make cli-generate DOMAIN=domain.yaml BRIDGE=../domaincraft-bridge-csharp OUTPUT=generated
# or directly:
go run ./cmd/domaincraft generate --domain domain.yaml --bridge /path/to/bridge --output generated
```

The `--bridge` flag accepts:
- A directory containing `bridge.yaml` (e.g. `../domaincraft-bridge-csharp`)
- A direct path to a `bridge.yaml` file

### Create a Starter Domain

Write a `domain.yaml` by hand or with the visual editor ([DomainCraft Studio](https://domaincraft.github.io/domaincraft-studio/)). An example lives in `examples/domain.yaml`.

---

## Architecture: Compiler Pipeline

```
domain.yaml --> Parser --> Lexer --> Validator --> IR Builder --> Renderer --> Generated Code
```

| Stage | Package | Output |
|-------|---------|--------|
| **Parser** | `internal/parser` | `ParsedSchema` — entities, fields, features, indexes, permissions, seeds |
| **Lexer** | `internal/lexer` | `FieldDefinition` — parsed field strings like `"string [required, max:255]"` |
| **Validator** | `internal/validator` | Validation errors (missing PKs, broken relations, etc.) |
| **IR Builder** | `internal/ir` | `IRProject` — fully linked graph with bidirectional relations, resolved navigation names |
| **Renderer** | `internal/renderer` | Files on disk + file manifest — reads `bridge.yaml`, loads templates, renders IR to output |
| **Snapshot/Migration** | `internal/snapshot` | `<output>/.domaincraft/snapshot.json` — persists IR + file manifest, diffs the next `domain.yaml`, drives delete/rename/type-change handling |

The **IR (Intermediate Representation)** is the contract between parsing and rendering. Templates receive a fully resolved `IRProject` and need no parsing logic.

---

## Creating a New Bridge

A bridge is a directory (or separate repository) containing templates and configuration that tell the renderer how to generate code for a target language/framework.

### Step 1: Create the Bridge Directory

```bash
mkdir my-bridge
cd my-bridge
```

### Step 2: Create `bridge.yaml`

This is the bridge manifest. It declares metadata, supported configurations, and the list of templates to render.

```yaml
name: my-language-api
description: "My Language REST API bridge"
version: "1.0.0"
output_dir: generated

# Declare what this bridge supports (informational)
supports:
  - api_style: rest
  - database: postgresql
  - auth: jwt

templates:
  # Per-entity template (rendered once per entity)
  - for: entity
    source: templates/entity.go.tmpl
    target: "models/{{ .Entity.Name | snakecase }}.go"

  # Project-level template (rendered once)
  - for: project
    source: templates/main.go.tmpl
    target: "main.go"

  # Conditional template (only rendered when condition is met)
  - for: project
    source: templates/enums.go.tmpl
    target: "models/enums.go"
    when: hasEnums
```

**Template spec fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `for` | yes | `entity` (per-entity) or `project` (once) |
| `source` | yes | Path to template file relative to bridge directory |
| `target` | yes | Output path pattern (supports Go template syntax) |
| `targets` | no | Multiple output path patterns (alternative to `target`) |
| `when` | no | Condition: `hasSeed`, `hasEnums`, `hasPermissions`, `hasOwnerTokens`, `hasAuth` |
| `overwrite` | no | `false` = scaffold once (skip if the file exists); default `true`. These files are tracked as **custom** (developer-owned) by the migration engine |
| `delimiters` | no | Custom template delimiters, e.g. `["<<", ">>"]` — use for bridges generating JSX/TSX where `{{ }}` conflicts with Go templates |

### Template conventions

- Use `{{-` and `-}}` whitespace trimming so output is clean.
- Feature fields (`createdAt`, `updatedAt`, `createdBy`, `updatedBy`, `deletedAt`, `version`) are auto-injected by features — skip them in field loops with `isFeatureField`.
- Iterate `range .Entity.Fields` with `if` filters instead of pre-computing slices; prefer clear names over clever control flow.
- **Split Core (always regenerated) and Custom (`overwrite: false`) code.** Generated logic belongs behind interfaces/base classes; the developer-owned implementation is scaffolded only once. The migration engine protects custom files when entities are deleted, renamed, or change types. The C# bridge (`domaincraft-bridge-csharp`) is the reference implementation: it ships the full **Generation Gap** pattern (`partial class` services split into an always-overwritten generated part with `partial void` hooks and a scaffold-once developer part).

### Schema migration (snapshots)

After every successful `domaincraft generate` the IR plus the file manifest is persisted to `<output>/.domaincraft/snapshot.json`. The next run diffs the new `domain.yaml` against that snapshot:

- **Deleted entities** — generated files are removed; custom files are listed in an interactive multi-select prompt (or a warning in non-interactive mode). Add `--prune` to apply cleanup automatically for CI. Non-interactive runs **without** `--prune` only warn and keep the previous snapshot, so a later `--prune` run can still find the orphaned files.
- **Renamed entities** — an entity declares its previous name via `old_name`:
  ```yaml
  entities:
    Item:
      old_name: Product   # rename hint
      fields: ...
  ```
  Custom files are renamed (replacing plural and singular forms in the path).
- **Field type changes** — the CLI prints a "manual refactoring" report listing changed fields and the custom files that may need fixes.

The snapshot is deterministic (sorted file lists) and language-agnostic. Bridges opt into custom-file protection by declaring `overwrite: false`.

### Step 3: Create `type_mappings.yaml`

This file maps domain/IR types to target language types. The renderer loads it at runtime.

```yaml
types:
  # Map every type the IR can produce
  string: "string"
  int: "int"
  int64: "long"
  float: "double"
  float64: "double"
  bool: "bool"
  boolean: "bool"
  date: "DateTime"
  datetime: "DateTime"
  "time.Time": "DateTime"
  uuid: "Guid"
  json: "JsonDocument"
  jsonb: "JsonDocument"
  decimal: "decimal"
  text: "string"
  "array(string)": "List<string>"
  "array(int)": "List<int>"

# Value types that need nullable suffix (?) when IsNullable=true
value_types:
  - "int"
  - "long"
  - "double"
  - "decimal"
  - "bool"
  - "DateTime"
  - "Guid"

# Foreign key delete behaviors
behaviors:
  cascade: "Cascade"
  set_null: "SetNull"
  restrict: "Restrict"
  no_action: "NoAction"
```

**Important:** Every type that `internal/ir/builder.go` can produce must have a mapping. The IR produces these types:
- `string`, `int`, `int64`, `float64`, `bool`, `decimal`
- `time.Time` (for date/datetime)
- `uuid`, `json`, `jsonb`, `text`
- Array types: `[]string`, `[]int`, `[]int64`, `[]float64`, `[]bool`

If a type is missing from the mapping, it passes through as-is (e.g. `float64` would appear literally in generated code).

### Step 4: Create Templates

Templates use Go's `text/template` syntax with [Sprig](https://masterminds.github.io/sprig/) functions plus these custom functions:

| Function | Description | Example |
|----------|-------------|---------|
| `pascalcase` | Convert to PascalCase | `{{ .Name \| pascalcase }}` |
| `camelcase` | Convert to camelCase | `{{ .Name \| camelcase }}` |
| `snakecase` | Convert to snake_case | `{{ .Name \| snakecase }}` |
| `lowercase` | Lowercase | `{{ .Name \| lowercase }}` |
| `pluralize` | Pluralize | `{{ pluralize .Entity.Name }}` |
| `languageType` | Map IR type to target language | `{{ languageType .DatabaseType .IsNullable }}` |
| `isValueType` | Check if type is a value type | `{{ if isValueType .DatabaseType }}` |
| `isFeatureField` | Check if field is auto-generated by features | `{{ if isFeatureField .Name }}` |
| `deleteBehaviorName` | Map delete behavior to target language | `{{ deleteBehaviorName .OnDeleteBehavior }}` |
| `jsonValue` | Format value for JSON output | `{{ jsonValue . }}` |

**Data available in templates:**

For `for: entity` templates, the context is:

```
.Entity.Name          - Entity name (e.g. "Product")
.Entity.NamePlural    - Plural name (e.g. "Products")
.Entity.Fields[]      - All fields
.Entity.RelationsOut  - Outgoing relations (FK to other entities)
.Entity.RelationsIn   - Incoming relations (other entities referencing this one)
.Entity.Indexes       - Indexes
.Entity.Seed          - Seed data rows
.Entity.Permissions   - RBAC permissions
.Entity.HasAudit      - Has audit feature
.Entity.HasSoftDelete - Has soft delete feature
.Entity.HasOptimisticLock - Has optimistic lock feature
.Project              - Project-level data (same as below)
```

For `for: project` templates:

```
.Project.Name         - Project name
.Project.Database     - Database type (e.g. "postgresql")
.Project.Auth         - Auth type (e.g. "jwt")
.Project.APIStyle     - API style (e.g. "rest")
.Project.Enums        - Map of enum name -> values
.Project.Entities[]   - All entities (same structure as above)
```

**IRField structure:**

```
.Name           - Field name
.DatabaseType   - IR type (e.g. "string", "float64", "uuid")
.IsPrimary      - Is primary key
.IsNullable     - Is nullable
.IsUnique       - Has unique constraint
.IsHidden       - Should be hidden from API (JsonIgnore)
.IsRelation     - Is a relation field
.IsMany         - Is a collection navigation (one-to-many)
.RelationTarget - Target entity name (for relations)
.DefaultValue   - Default value
.Validations[]  - Validation rules (Name, Value)
```

**IRRelation structure:**

```
.FieldName        - FK field name (e.g. "categoryId")
.TargetEntity     - Pointer to target IREntity
.NavigationName   - Navigation property name (e.g. "category")
.InverseNavName   - Inverse navigation name (e.g. "products")
.OnDeleteBehavior - Delete behavior (cascade, set_null, restrict)
.IsNullable       - Is the FK nullable
.IsMany           - Is this a collection side
.RelationType     - "many-to-one", "one-to-many", "many-to-many", "one-to-one"
.PairFieldName    - For a `[many]` reconciled with a single FK on the target: the FK field name (one-to-many, not a join)
```

Inverse navigation names are disambiguated when two relations from the same
source entity point to the same target: the second becomes
`<FieldName><EntityPlural>` (e.g. `EscrowContract.Buyer` → `EscrowContracts`,
`EscrowContract.Seller` → `SellerEscrowContracts`). A `[many]` relation whose
target declares a single FK back is reclassified as `one-to-many` with
`PairFieldName` set, so bridges render `HasMany(...).WithOne(...)` instead of a
many-to-many join table.

### Step 5: Test the Bridge

1. **Validate the domain YAML:**
   ```bash
   go run ./cmd/domaincraft validate --domain examples/domain.yaml
   ```

2. **Generate code:**
   ```bash
   go run ./cmd/domaincraft generate \
     --domain examples/domain.yaml \
     --bridge ./my-bridge \
     --output ./test-output
   ```

3. **Verify the output:**
   - Check that all expected files were generated
   - Open generated files and verify syntax
   - If targeting a compiled language, try to compile the output
   - Check that type mappings were applied (no raw IR types like `float64`)

4. **Add to CI (optional):**
   ```yaml
   # In .github/workflows/ci.yml
   - name: Test bridge
     run: |
       go run ./cmd/domaincraft generate \
         --domain examples/domain.yaml \
         --bridge ./my-bridge \
         --output ./test-output
       # Add language-specific compilation/linting here
   ```

### Step 6: Document the Bridge

Create a `README.md` in the bridge directory with:
- What the bridge generates (file list, architecture)
- Prerequisites (language version, frameworks)
- How to build/run the generated project
- What's supported and what's still TODO

---

## Bridge Template Checklist

- Support the full YAML surface area that the parser exposes.
- Map domain concepts to idiomatic constructs in the target language.
- Separate domain models from persistence/configuration concerns.
- Avoid generating migration scaffolding if the target ecosystem already owns it.
- Prefer configuration classes or metadata over duplicated inline boilerplate.
- Keep naming consistent with the target language's conventions.
- Add or update tests that cover the generated file set and key content.
- Document what the bridge generates and what still needs work.
- Split **Core** (always regenerated) from **Custom** (`overwrite: false`, scaffolded once) code so the migration engine can safely delete/rename files. Put generated logic behind interfaces or `partial` classes and scaffold the developer-owned implementation once.

### Bridge isolation and language mappings

- Bridges MUST live in their own repository or directory and provide any language-specific mappings there (e.g. `type_mappings.yaml`). The core renderer will load those mappings at runtime so the core remains language-agnostic.
- Do not add target-language types or helpers to the core. Keep the core focused on parsing, validation, IR, and generic rendering.

### Permissions and services

- Bridges should provide concrete implementations for runtime services referenced by generated code. For example, the C# bridge must include an `IPermissionService` interface and a default `PermissionService` implementation (templates live under `bridges/<bridge>/templates/`). Generated repositories will call `IPermissionService.HasAnyRole(...)` at runtime, so the bridge owns that contract.
- If the target language/framework has built-in authorization (e.g. ASP.NET Core policies), the bridge should wire generated code to those native mechanisms instead of inventing custom ones.

### Seeds and sample data

- Keep seed data in `domain.yaml` (the `seed:` block under each entity). The parser will pass seed rows to the IR and the renderer will supply them to templates that need it.
- Do not duplicate seed data inside bridge templates. If a bridge needs seed data in a different shape (e.g. SQL INSERTs, JSON fixtures), transform it from the IR seed rows in the template—do not hardcode it.

---

## Core Rules

- Keep changes small, testable, and aligned with the existing architecture.
- Prefer DRY, SOLID, and explicit templates over clever template tricks.
- Update docs and tests together with code changes.
- Preserve backward compatibility unless a change is intentionally breaking.
- When adding bridge features, make sure the bridge manifest, templates, and renderer expectations stay in sync.

---

## Review Checklist

When reviewing a pull request that touches bridge templates, verify:

1. The bridge manifest (`bridge.yaml`) lists every generated file and the correct output path pattern.
2. All new or changed templates are rendered for the expected scope (`for: entity` vs. `for: project`).
3. Type mappings cover every type the IR can produce for the target language (check `type_mappings.yaml`).
4. Generated code compiles (or passes a static check) for a representative domain like the BlogApp example.
5. Feature fields (audit, soft delete, etc.) are handled in the correct layer (entity, configuration, controller, etc.).
6. Permissions are wired end-to-end: YAML roles -> IR -> generated authorization attributes/policies.
7. Documentation is updated to reflect new or changed bridge capabilities.
8. `overwrite: false` is used only for scaffold-once, developer-owned files, and such files are never overwritten by a later run.
9. Delete/rename behavior is correct: re-running `generate --prune` after an entity is removed or renamed cleans up only the right files (verify against the snapshot at `<output>/.domaincraft/snapshot.json`).

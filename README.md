# DomainCraft

**Define your domain once in YAML. Compile your whole stack — API, web client, admin — from a single source of truth.**

DomainCraft is a domain-driven compiler. You describe entities, relations, permissions and rules in one `domain.yaml`. The tool compiles it into consistent code for every layer through pluggable **bridge** templates. Types, validation, queries, permissions and errors come from the same IR, so layers never drift.

```yaml
# domain.yaml — that's all you write
project:
  name: MyShop
auth:
  type: jwt
  roles: [Admin, User]
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
```

```bash
domaincraft generate --bridge csharp-rest
# → complete, compilable project — entities, repositories, controllers, Docker, k8s
```

## Installation

```bash
go install github.com/DomainCraft/DomainCraft/cmd/domaincraft@latest
# or download a binary from https://github.com/DomainCraft/DomainCraft/releases
```

## Quick start

**1. Create `domain.yaml`** — by hand or in [Studio](https://domaincraft.github.io/domaincraft-studio/) (visual canvas + Monaco editor, two-way sync).

Minimal example:

```yaml
project:
  name: My App
  version: 1.0.0
database: postgresql
auth:
  type: jwt
  roles: [Admin, User]
api_style: rest
entities:
  User:
    fields:
      id: uuid [primary]
      email: string [required, unique, email]
      name: string [required]
      password: string [required, hidden]
```

**2. Validate**

```bash
domaincraft validate --domain domain.yaml
```

**3. Generate**

```bash
domaincraft generate --bridge csharp-rest --output ./generated
domaincraft generate --bridge react-rest   --output ./frontend  # same model, typed client
```

Run the API: `cd generated && dotnet run --project src/WebApi`

Swap a layer without forking 71 templates:

```bash
domaincraft generate --bridge csharp-rest --replace persistence=csharp-dapper
```

## Why this approach

| Hand-written | DomainCraft |
|---|---|
| 50+ files per entity by hand | 1 entity in ~10 lines of YAML |
| CRUD repeated per project | Generated consistently |
| Bugs fixed in every copy | Fixed in the template, fixed everywhere |
| New language = rewrite | New bridge, same domain |
| Permissions scattered | Declared with entities, wired end-to-end |

You describe *what* the domain looks like, not *how* to implement it.

## How it works

```
domain.yaml → Parser → Lexer → Validator → IR Builder → Renderer → Generated Code
                                                      → Snapshot (.domaincraft/snapshot.json)
```

The **IR** is the contract. It's language-agnostic — the Go core never contains C# or TypeScript — and every bridge renders from it. Two runs on the same `domain.yaml` produce byte-identical output.

See [Architecture](https://domaincraft.github.io/domaincraft-site/docs/concepts/architecture/) for the full pipeline.

## What you can define

Fields: `string`, `int`, `bigint`, `float`, `decimal`, `boolean`, `date`, `datetime`, `uuid`, `text`, `json`/`jsonb`, `enum(Name)`, `array(Type)` — with traits `primary`, `required`, `unique`, `hidden`, `readonly`, `optional`, validations `min`/`max`/`email`/`url`/`ipv4`/`regex`/`gte`/`lt`, defaults `default:0`/`now()`/`uuid()` and `on_delete: cascade|set_null|restrict|no_action`.

Features: `audit` (`createdAt`/`updatedAt`), `audit_log` (`createdBy`/`updatedBy`), `soft_delete` (`deletedAt`), `optimistic_lock` (`version` → `409 Conflict`) — plus `event_sourced`/`cacheable`.

Permissions: `read`/`create`/`update`/`delete` with `*` (public), role names (RBAC) and `@Owner` (ABAC). `when:` in `bridge.yaml` gates templates on `hasAuth`/`hasSeed`/`hasAddon:dapr`.

Full language: [domain.yaml reference](https://domaincraft.github.io/domaincraft-site/docs/reference/language/)

## Axes, layers and replacement

One bridge = **one layer in one axis**. A layer is declared in `bridge.yaml` (`layer: domain|persistence|transport` for C#, `core|framework|offline` for web/mobile — open `^[a-z][a-z0-9_]*$`, duplicate `layer` in one chain fails).

```
backend (C#):  domain (core) → persistence (efcore/dapper) → transport (rest/grpc)
web:           core (ts-core) → framework (react/vue)
mobile:        core (dart-core) → framework (flutter) → offline (drift)
```

`extends: <base>` builds a linear chain (path / registry ID / `owner/repo`). `--replace` swaps one edge without forking:

```bash
--replace persistence=dapper        → core+dapper+rest
--replace transport=grpc            → core+efcore+grpc
# N×M = N+M repos, not N×M: 10×10=100 → 21 repos (1 core+10+10), 30×30=900 →61
```

`1000` bridges stay `1000` repos, not the product. See [Axes and layers](https://domaincraft.github.io/domaincraft-site/docs/concepts/three-axes/).

## Bridges

| ID | Layer | Targets |
|---|---|---|
| `csharp-core` | `domain` | Domain + Application |
| `csharp-efcore` | `persistence` — `extends: csharp-core` | EF Core + PostgreSQL + `migrations:` |
| `csharp-rest` | `transport` — `extends: csharp-efcore` | ASP.NET REST + JWT + k8s/tests |
| `csharp-restful` | — | *Archived* monolith (71 files) — use `csharp-rest` (byte-identical) |
| `ts-core` | `core` | TypeScript data layer (types, Zod, query DSL, permissions) |
| `react-rest` | `framework` — `extends: ts-core` | TanStack Query + auth |
| `appwrite` | — | Appwrite TablesDB (`appwrite.config.json`) |
| `admin-alpine` | — | Static admin (`--admin`) |

```bash
domaincraft generate --bridge csharp-rest
domaincraft generate --bridge csharp-rest --replace persistence=csharp-dapper
domaincraft bridges --check-updates
```

Bridges are cached in `~/.domaincraft/bridges/` and `~/.domaincraft/cache/` (24h TTL). Full registry: [Bridges](https://domaincraft.github.io/domaincraft-site/docs/reference/bridges/).

## CLI

```
domaincraft generate --domain domain.yaml --bridge csharp-rest --output ./generated
domaincraft validate --domain domain.yaml
domaincraft bridges --check-updates
domaincraft update --check
```

Flags: `--domain`/`-d`, `--bridge`/`-b`, `--output`/`-o`, `--admin`, `--replace left=right` (repeatable), `--prune`/`--migrate` (schema), `--addons dapr`, `--non-interactive`. See [CLI reference](https://domaincraft.github.io/domaincraft-site/docs/reference/cli/).

## Migration

The IR + manifest is saved to `.domaincraft/snapshot.json`. On the next run renames (`old_name: Product`) become `RenameTable`/`RenameColumn` (data kept), deletions show a prompt, type changes print a report. `--prune` applies renames/deletions and rewrites identifiers in `overwrite: false` files and runs `migrations:` best-effort. See [Migrations](https://domaincraft.github.io/domaincraft-site/docs/guides/migrations/) and [Generation Gap](https://domaincraft.github.io/domaincraft-site/docs/concepts/generation-gap/).

## Certification (TCK)

`DomainCraft/compliance-suite/` is the hand-written oracle. `kitchen-sink.yaml` (maximal domain) + `suite.js` (k6 over HTTP) — copy `certification.yml` to your bridge repo. TypeScript has its own `ts-core/tck/`. See [Certification](https://domaincraft.github.io/domaincraft-site/docs/guides/certification/).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) — `make build` / `make test` / `make lint`, bridge anatomy, `overwrite: false` and snapshot rules. For the full guide on writing a bridge: [Writing a bridge](https://domaincraft.github.io/domaincraft-site/docs/guides/writing-a-bridge/).

## License

MIT

# Spec

This folder contains the generated JSON Schema for `domain.yaml`.

## Regenerate schema and GUI types

```bash
make regenerate-spec
```

This runs `cmd/schema-gen` to produce `spec/domain.schema.json`, then regenerates `domaincraft-studio/src/types/domain.generated.ts`.

## Entity `old_name` (migration hint)

Entities may declare their previous name via `old_name`. It is a hint for the
schema migration engine: when the same entity reappears under a new key, the
CLI treats it as a *rename* (instead of a delete + create) and offers to rename
the orphaned custom files.

```yaml
entities:
  Item:
    old_name: Product   # rename hint — previous entity name
    fields:
      id: uuid [primary]
```

The GUI also writes `old_name` automatically when you rename an entity there.

## Schema migration engine (snapshots)

See the project documentation for the full design. Summary:

- After every successful `domaincraft generate`, the current domain model and
  the rendered file manifest are persisted to `<output>/.domaincraft/snapshot.json`.
- On the next run the new `domain.yaml` is diffed against that snapshot:
  - **Deleted entities** → the CLI offers to remove orphaned files.
  - **Renamed entities** (via `old_name`) → the CLI offers to rename files.
  - **Field type changes** → the CLI prints a smart "manual refactoring" report.
- `--prune` performs the cleanup automatically without prompts (CI-friendly).
- Without `--prune`, non-interactive runs only warn and **keep the previous
  snapshot**, so a later `--prune` run can still find the orphaned files.

## Build WASM validator for GUI

The GUI uses a Go WASM binary for full schema validation (same Go code as the CLI).

```bash
# Build WASM binary (requires GOOS=js GOARCH=wasm)
make build-wasm

# Or build and copy to GUI public directory
make build-wasm-gui
```

The WASM binary is served from `domaincraft-studio/public/wasm/validate.wasm`.
`wasm_exec.js` (Go's WASM runtime) is already in `public/wasm/`.

### When to rebuild

Rebuild WASM after changes to:
- `internal/parser/` (parsing logic)
- `internal/validator/` (validation rules)
- `internal/lexer/` (field definition parsing)
- `internal/specmeta/` (type definitions)
- `cmd/wasm-validator/` (WASM entry point)

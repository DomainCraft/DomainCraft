# Spec

This folder contains the generated JSON Schema for `domain.yaml`.

## Regenerate schema and GUI types

```bash
make regenerate-spec
```

This runs `cmd/schema-gen` to produce `spec/domain.schema.json`, then regenerates `DomainCraftGui/src/types/domain.generated.ts`.

## Build WASM validator for GUI

The GUI uses a Go WASM binary for full schema validation (same Go code as the CLI).

```bash
# Build WASM binary (requires GOOS=js GOARCH=wasm)
make build-wasm

# Or build and copy to GUI public directory
make build-wasm-gui
```

The WASM binary is served from `DomainCraftGui/public/wasm/validate.wasm`.
`wasm_exec.js` (Go's WASM runtime) is already in `public/wasm/`.

### When to rebuild

Rebuild WASM after changes to:
- `internal/parser/` (parsing logic)
- `internal/validator/` (validation rules)
- `internal/lexer/` (field definition parsing)
- `internal/specmeta/` (type definitions)
- `cmd/wasm-validator/` (WASM entry point)

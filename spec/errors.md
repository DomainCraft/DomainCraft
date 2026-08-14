# Error & Validation Contract

This document defines the **stable, language-agnostic** error and validation
contract that every bridge must serialize. The core produces the semantic data
(validation rule kinds + stable codes + default messages); a bridge maps them to
its framework's native mechanism. The contract fixes the *semantics* (codes,
status codes, message text), never the exact JSON byte layout — an ASP.NET
bridge returns `ValidationProblemDetails`, a FastAPI bridge returns its own
shape, but both expose the same machine-readable codes.

## 1. Validation rules (from the core)

`IRField.ValidationRules()` returns normalized rules. Each rule carries:

| field      | meaning                                                              |
|------------|----------------------------------------------------------------------|
| `Kind`     | semantic kind: `required`, `email`, `url`, `ipv4`, `regex`, `min_length`, `max_length`, `min_value`, `max_value` |
| `Code`     | stable wire code (see table below)                                   |
| `Value`    | the rule parameter (`""` for flag rules)                             |
| `Exclusive`| true when a numeric bound is strict (`gt`/`lt`)                      |
| `Message`  | stable default message, value already interpolated                   |

### Stable codes and default messages

| Kind          | Code        | Default message (N = `Value`)                                |
|---------------|-------------|---------------------------------------------------------------|
| `required`    | `REQUIRED`  | `is required`                                                 |
| `email`       | `EMAIL`     | `must be a valid email address`                               |
| `url`         | `URL`       | `must be a valid URL`                                         |
| `ipv4`        | `IPV4`      | `must be a valid IPv4 address`                                |
| `regex`       | `REGEX`     | `must match the required format`                              |
| `min_length`  | `MIN_LENGTH`| `must be at least N characters`                               |
| `max_length`  | `MAX_LENGTH`| `must be at most N characters`                                |
| `min_value`   | `MIN_VALUE` | `must be greater than or equal to N` (`> N` when `Exclusive`) |
| `max_value`   | `MAX_VALUE` | `must be less than or equal to N` (`< N` when `Exclusive`)    |

A bridge that carries per-rule codes natively (FluentValidation
`.WithErrorCode`, class-validator) uses `Code` directly; a DataAnnotations
bridge maps `Message` into `ErrorMessage` (DataAnnotations carry messages, not
codes — this is a framework limitation, not a contract violation).

## 2. Operation-error envelope

Non-validation operation errors (bad filter, bad patch, concurrency conflict)
use a minimal, consistent envelope:

```json
{ "code": "INVALID_FILTER", "message": "…" }
```

| `code`                  | HTTP | meaning                                        |
|-------------------------|------|------------------------------------------------|
| `INVALID_FILTER`        | 400  | malformed filter/sort expression (spec/query.md) |
| `INVALID_FILTER`        | 400  | a JSON-path ordered operator hit a non-numeric leaf |
| `INVALID_PATCH`         | 400  | merge-patch body is not a JSON object           |
| `CONCURRENCY_CONFLICT`  | 409  | optimistic-lock version mismatch                |
| `UNIQUE_CONFLICT`       | 409  | a unique constraint (single-field or index) was violated |
| `FOREIGN_KEY_CONFLICT`  | 409  | a foreign-key constraint was violated           |
| `CONSTRAINT_CONFLICT`   | 409  | any other database constraint violation         |

Unique constraints are core-owned: the core computes the constraint name
(`IRIndex.DatabaseName()`) and the field set (`IREntity.AllIndexes()`), so a
bridge can map a database unique violation (PostgreSQL `SqlState 23505`) back to
the offending field and report `{ code: "UNIQUE_CONFLICT", message: "<field> must
be unique" }` — never a bare 500.

Validation failures use the framework's standard validation response
(`ValidationProblemDetails` / RFC 9457 `problem+json` in ASP.NET) and are
**not** wrapped in the `{code, message}` envelope — they are a distinct,
framework-native concern. `404 Not Found` and `403 Forbid` likewise use the
framework's standard empty `ProblemDetails`.

## 3. Pagination contract

Paginated list endpoints return a `PagedResult<T>`-shaped payload:

```json
{
  "items": [ … ],
  "totalCount": 123,
  "page": 1,
  "pageSize": 20,
  "totalPages": 7,
  "hasNextPage": true,
  "hasPreviousPage": false,
  "nextCursor": null
}
```

Field names are part of the contract and must match across bridges (JSON
`camelCase`). Request normalization is defined in `spec/query.md`
(`IRPaginationConfig` / `ListQuery.FromQuery`): `limit` wins over `pageSize`,
both clamp to `1..max_page_size`, `page >= 1`. In keyset (cursor) mode
`nextCursor` carries the last row's primary key (or `null` at the end); offset
mode leaves it `null`.

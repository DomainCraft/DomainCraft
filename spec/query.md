# Query grammar (v1)

The core defines **one** list-query grammar — search, sort, filter and
pagination — that every bridge serializes into its own query layer (LINQ,
Prisma `where`/`orderBy`, Mongo `$match`/`$sort`, Appwrite `Query.*`, SQL
`WHERE`/`ORDER BY`, …). The core emits the *schema* a bridge's runtime validator
consumes — `IREntity.FilterablePaths()`, `SortableFields()`,
`SearchableFields()`, and the pagination policy on `IRProject.Pagination` — and
the bridge parses and validates the query strings at runtime against that
schema. The grammar below is the contract the bridge's parser must implement.

## Filter

A filter expression selects a subset of rows.

```
filter := or
or     := and ( "|" and )*            # "|" = OR (lowest precedence)
and    := term ( "," term )*          # "," = AND
term   := path ":" op ":" value
path   := segment ( "." segment )*    # relation hops / JSON key path
op     := "eq" | "ne" | "gt" | "gte" | "lt" | "lte"
        | "contains" | "startsWith" | "endsWith" | "in"
value  := bare | "(" bare ( "," bare )* ")"   # the list form is for `in`
```

- `path` is a dotted field reference (case-insensitive, camelCase like the JSON
  wire contract). It is one of:
  - a scalar field: `price`
  - a to-one relation hop + a scalar field on the target: `category.name`
  - a JSON field + one or more object keys: `meta.price`, `meta.a.b`
- `value` is a bare literal. It must not contain `:`, `,`, `|`, `(` or `)`.
  Free-text with those characters belongs to the separate `search` parameter.
- `in` takes a parenthesized, comma-separated list: `status:in:(Active,Pending)`.

### Operators per type

| Field type | Operators |
|------------|-----------|
| string, text | eq, ne, contains, startsWith, endsWith, in |
| boolean | eq, ne |
| int, bigint, float, decimal, date, datetime | eq, ne, gt, gte, lt, lte, in |
| uuid, enum | eq, ne, in |
| JSON path (jsonb) | eq, ne, gt, gte, lt, lte, in, contains, startsWith, endsWith |
| JSON path (json) | eq, ne, in, contains, startsWith, endsWith |

A relation path (`category.name`) inherits the operator set of the leaf field on
the target entity.

### Relation paths

A dotted path may cross **one** to-one relation, then end on a filterable scalar
field of the target entity:

```
category.name:eq:Books           # Product → Category.name
supplier.firstName:contains:Ana  # Product → User.firstName
```

- The relation segment matches either the relation's field name (`categoryId`)
  or its navigation name (`category`), case-insensitively.
- Only **to-one** relations are filterable. Collection (`many`) relations are
  rejected because the semantics (any vs all) are ambiguous.
- Relation nesting is limited to one hop. Deep chains (`a.b.c`) and
  self-referential cycles are rejected.

### JSON paths

A dotted path may enter a `json`/`jsonb` field and then name object keys:

```
meta.price:gte:100     # (meta->>'price')::numeric >= 100
meta.name:eq:Widget    # meta->>'name' = 'Widget'
meta.a.b:contains:xy   # meta#>>'{a,b}' LIKE '%xy%'
```

The value type of a JSON leaf is dynamic, so the core cannot coerce it
statically — values keep their string form. The operator set therefore depends
on the column type:

- `jsonb` — the full vocabulary, with the following serialization contract:
  - `eq`, `ne`, `in`, `contains`, `startsWith`, `endsWith` — **text** semantics:
    the bridge extracts the leaf as text (`->>` / `#>>` in PostgreSQL) and
    compares the string.
  - `gt`, `gte`, `lt`, `lte` — **numeric** semantics: the bridge casts the leaf
    to a number (`(col->>'key')::numeric`) before comparing. The JSON leaf must
    be numeric; a non-numeric leaf makes the bridge reject the request with
    HTTP 400 (mapped from the database cast error), never a 500.
- `json` — text-semantics operators only (`eq`, `ne`, `in`, `contains`,
  `startsWith`, `endsWith`). Ordered (numeric) operators are rejected by the
  core because a `json` column is an opaque, untyped document; use `jsonb` when
  you need ordered filtering on a JSON path.

### Filter examples

```
price:gte:100                        # price >= 100
status:eq:Active                     # exact enum match
name:contains:widget                 # substring
sku:in:(SKU-1,SKU-2)                 # set membership
price:gte:100,price:lt:500           # AND (comma)
status:eq:Active|status:eq:Archived  # OR (pipe)
category.name:eq:Books               # relation hop
meta.price:gte:100                   # JSON key path
```

### Free-text search

`search` is not a filter: it is a case-insensitive substring match across the
entity's `SearchableFields()` (string/text columns). It is expressed in the same
AST as `Or(Contains(f1, q), Contains(f2, q), …)` and AND-ed with any `filter`,
so bridges have a single serialization path for both.

## Sort

```
sort := key ( "," key )*
key  := [ "-" | "+" ] field        # "-" = descending, "+"/bare = ascending
```

- `field` is a scalar field name from `IREntity.SortableFields()`, matched
  case-insensitively and canonicalized to the IR field name.
- The primary key, relations, enums, arrays, JSON blobs, hidden and feature
  fields are **not** sortable (the PK is the implicit default order, not an
  explicit key).
- A bridge MUST reject an unknown/unsortable key with the same client error it
  uses for an invalid filter (HTTP 400 for HTTP bridges) — never silently ignore
  it or fall back to an arbitrary order.

```
price            # ascending by price
-price,name      # price descending, then name ascending
```

## Pagination

List endpoints accept two paging modes plus sizing controls:

- **Offset (default):** `page` + `pageSize` (and `limit`, an alias for
  `pageSize`; when both are given, `limit` wins). The effective page size is
  clamped to `1 .. MaxPageSize` (`IRProject.Pagination`), `page` is clamped to
  `>= 1`. Supports `sort`.
- **Keyset (cursor):** `cursor` + `pageSize`. Available only when the primary
  key is a monotonic integer (`int`/`bigint`) — `IREntity.CursorField()` returns
  the PK then, nil otherwise. `cursor` is the primary key of the last row seen;
  the bridge returns rows `WHERE pk > cursor ORDER BY pk` and sets `nextCursor`
  to the last row's PK (or `null` when no further rows follow). `sort` is
  ignored in cursor mode (order is by PK). Entities with a uuid/text PK ignore
  `cursor` and fall back to offset.

The response carries `items`, `totalCount`, `page`, `pageSize`, and — in cursor
mode — `nextCursor`. `hasNextPage`/`hasPreviousPage`/`totalPages` are
offset-mode only; cursor clients read `nextCursor` (null = end).

## Relation expansion

A bridge eager-loads the entity's relations (`IREntity.EagerLoadRelations()`).
List endpoints accept `include`, a comma-separated list of relation field names
(the JSON camelCase names); it restricts which `[many]` collections are
loaded:

- `include` omitted → load all relations (the default).
- `include=` (explicit empty) → load none.
- `include=items,tags` → load only those `[many]` collections.

Single to-one relations always expose their foreign-key id (no nested object),
so `include` does not apply to them.

## Serialization contract

A bridge MUST:

- `eq`/`ne` → equality/inequality.
- `gt`/`gte`/`lt`/`lte` → ordered comparison (numeric/date/datetime fields).
- `in` → set membership (SQL `IN`, LINQ `Contains`).
- `contains`/`startsWith`/`endsWith` → string predicates; case sensitivity
  follows the target database collation (documented, not normalized).
- `and`/`or` → logical conjunction/disjunction.
- preserve the sort key order (`-price,name` ≠ `name,-price`).
- reject unknown sort/filter fields with the bridge's client-error convention,
  never a 500.
- treat `search` and `filter` as orthogonal (AND-ed together).
- clamp pagination against the core's `Pagination` limits.

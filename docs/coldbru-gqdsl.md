# GraphQL Query DSL — Language Specification

**Version:** 0.1.0
**Date:** October 27, 2026
**Status:** Version 0.1 scope decided, not yet implemented. Decisions: [DECISIONS.md](../DECISIONS.md) D-007, D-038 – D-045. Where this document and DECISIONS.md disagree, DECISIONS.md wins.
**Implementation language:** Go
**Compilation target:** GraphQL

---

## 1. Overview

The GraphQL Query DSL is a declarative, JavaScript-inspired expression language for writing audience filters against the Customer service's GraphQL API.

Its primary use case is letting the LLM agent in the Proposal service express a segment rule in a compact, validated form. The agent never builds GraphQL documents or SQL statements directly.

Version 0.1 supports:

- Boolean and comparison expressions.
- Exact equality and inequality.
- Membership tests with `in`.
- Relative and absolute time values with `ago()` and `date()`.
- Boolean literals.
- Nested logical conditions.
- Relationship predicates with `some`, `none` and `every`.
- Compilation into GraphQL filter variables.

Approximate text matching (`~=`) and field-to-field comparisons are **reserved** in 0.1 (D-045).

The DSL deliberately adopts a subset of JavaScript syntax while assigning its own semantics to supported expressions.

**The DSL is not JavaScript.** It is a declarative query language and must not execute arbitrary JavaScript code.

### 1.1 Primary design objectives

**Expressiveness:** Allow complex filtering conditions without requiring the agent to understand GraphQL syntax.

**Type safety:** Validate referenced fields, relations and operators against a catalog generated from the GraphQL schema (D-044).

**Separation of concerns:** Keep expression parsing, query construction and database execution independent.

**Token efficiency:** A DSL filter is much shorter than the equivalent JSON or GraphQL filter (D-007).

**Backend independence:** The client-side compiler produces GraphQL rather than database-specific SQL.

**Controlled execution:** Reject unsupported code. The GraphQL server, not the client, is the security boundary (D-043).

### 1.2 Non-goals

Version 0.1 does not provide:

- General-purpose JavaScript execution.
- Direct SQL statement construction by the client.
- Arbitrary database join conditions.
- Data mutation or database administration.
- Aggregation functions.
- User-defined functions with executable bodies.
- Field selection or entity choice by the agent (D-041).
- Approximate text matching or field-to-field comparison (reserved, D-045).

---

## 2. System Architecture

```text
LLM agent (Proposal service)
          |  writes the filter expression only (D-041)
          v
DSL parser & semantic analyzer   — Proposal, Go, own Pratt parser (D-038)
          |  checks names and types against the generated catalog (D-044)
          v
Typed filter AST
          |
          v
GraphQL compiler                 — Proposal adds the fixed entity and selection
          |
          v
Customer service GraphQL API     — gqlgen; authoritative access control (D-043)
          |
          v
Whitelisted SQL compiler         — parameterized SQL, reachability condition always added (D-007)
          |
          v
PostgreSQL (customer_db)
```

The Proposal service parses the DSL, validates it against the catalog and compiles it into GraphQL variables.

The Customer service's GraphQL endpoint is authoritative for schema enforcement, access control, relational resolution and database interaction. Access management happens at this endpoint in every case (D-043):

- Each calling service sends its own token and is limited to its scopes.
- The filter input only contains catalog fields (D-042).
- The reachability and consent condition is added to every audience query (D-007).
- Depth and complexity limits are enforced.
- nginx does not expose the Customer service.

Client-side validation in Proposal is a convenience for fast, positioned error messages. It is not a security boundary.

DSL use is opt-in (D-007). A raw GraphQL filter can still be validated and sent without the DSL.

### 2.1 Query representation

A query has three logically separate components:

| Component | Purpose                                          | Set by (D-041)                    |
| --------- | ------------------------------------------------ | --------------------------------- |
| Entity    | Root GraphQL query resource                      | Always `customers`                |
| Filter    | Boolean predicate determining qualifying records | The LLM, as a DSL string          |
| Selection | Fields and nested relationships to return        | Proposal code, fixed per purpose  |

The LLM only writes the filter. It reaches Proposal as the single string field of the `propose_segment` tool (D-027).

Proposal builds the full query envelope internally:

```json
{
  "entity": "customers",
  "filter": "city === 'İstanbul' && totalSpendGbp > 1000",
  "purpose": "preview"
}
```

| Purpose    | Fixed selection                                               |
| ---------- | ------------------------------------------------------------- |
| Preview    | Audience count and channel coverage                           |
| Recipients | Customer id, first name, language, assigned channel's contact |

The agent cannot request contact data or any other field.

---

## 3. Lexical and Syntactic Specification

### 3.1 Basic syntax

The language adopts JavaScript-style expressions and operator precedence.

```javascript
ageYears >= 25 && totalSpendGbp > 1000
```

Parentheses may be used to explicitly group expressions.

```javascript
ageYears >= 25 &&
(city === "İstanbul" || city === "Ankara")
```

### 3.2 Operators

| Operator | Meaning                         | Status in 0.1                                   |
| -------- | ------------------------------- | ----------------------------------------------- |
| `===`    | Exact equality                  | Supported                                       |
| `!==`    | Exact inequality                | Supported                                       |
| `>`      | Greater than                    | Supported                                       |
| `>=`     | Greater than or equal           | Supported                                       |
| `<`      | Less than                       | Supported                                       |
| `<=`     | Less than or equal              | Supported                                       |
| `in`     | Membership in a literal list    | Supported                                       |
| `&&`     | Logical AND                     | Supported                                       |
| `\|\|`   | Logical OR                      | Supported                                       |
| `!`      | Logical NOT                     | Supported                                       |
| `~=`     | Search / approximate text match | Reserved: `UNSUPPORTED_OPERATOR`                |
| `==`     | No defined semantics            | Rejected: `UNSUPPORTED_OPERATOR`                |
| `!=`     | No defined semantics            | Rejected: `UNSUPPORTED_OPERATOR`                |

The lexer recognizes reserved and rejected operators so that it can report them with a source position.

No implicit JavaScript-style equality coercion is supported.

### 3.3 Exact equality

```javascript
city === "İstanbul"
ageYears === 25
loyaltyTier !== "Bronze"
```

Exact equality compares compatible typed values without implicit coercion.

For example:

```javascript
ageYears === "25"
```

Must produce `TYPE_MISMATCH`, because `ageYears` is numeric.

**City values (D-040).** `city` is a closed list. Both sides are compared through a canonical key: `ı`, `İ` and `I` become `i`, then NFD and lower case. So `"İstanbul"`, `"istanbul"` and `"ISTANBUL"` match the same customers. A value not in the known city list fails with `UNKNOWN_VALUE` and a list of suggestions. The rule is applied in the Customer service and does not depend on database collation.

Other enum-like string fields compare exactly, using the values in the catalog.

### 3.4 Membership

```javascript
city in ["İstanbul", "Ankara", "İzmir"]
loyaltyTier in ["Gold", "Platinum"]
```

The right-hand side must be a non-empty list of literals of the field's type; `city in []` fails with `SYNTAX_ERROR` (D-046). `city in [...]` uses the canonical key rule for every element.

### 3.5 Time values

```javascript
lastCartAddDate >= ago("30d")
signupDate < date("2026-01-01")
```

- `ago("<n><unit>")` is a relative point in time (D-039). The compiler sends it to GraphQL in relative form. The Customer service resolves it against `now()` when the query runs. The agent never does date arithmetic.
- `date("YYYY-MM-DD")` is an absolute date.
- Units are `d` (days) and `h` (hours): a positive integer plus the unit, e.g. `"30d"`, `"12h"` (D-058).

Because `ago()` is resolved at query time, the same rule can return a different audience at a later time. This is expected between the preview and the recipient list.

### 3.6 Boolean values

```javascript
hasOpenCart === true
retired === false
```

`true` and `false` are literals. A bare boolean field such as `hasOpenCart` or `!retired` is not part of the 0.1 grammar; write the comparison explicitly (D-046).

### 3.7 Approximate text matching (reserved)

```javascript
favouriteCategory ~= "electronics"
```

`~=` is reserved in 0.1 and returns `UNSUPPORTED_OPERATOR` (D-045). The catalog has no free-text field that needs it, and enabling it would require deciding the matching mode and a `pg_trgm` extension and index.

### 3.8 Logical expressions

Expressions may be combined recursively.

```javascript
city === "İstanbul" &&
(ageYears >= 25 || totalSpendGbp > 10000)
```

Negation is supported:

```javascript
!(loyaltyTier === "Bronze")
```

### 3.9 Operator precedence

Highest to lowest:

1. Parenthesized expressions, field access and function calls (`ago`, `date`).
2. Unary negation (`!`).
3. Comparisons and membership (`===`, `>`, `in`, etc.).
4. Logical AND (`&&`).
5. Logical OR (`||`).

Thus:

```javascript
a || b && c
```

Is interpreted as:

```javascript
a || (b && c)
```

Because `!` binds tighter than comparisons, negate a comparison with parentheses: `!(loyaltyTier === "Bronze")`. `!loyaltyTier === "Bronze"` is a `SYNTAX_ERROR` (D-065).

### 3.10 Identifiers and literals

Identifiers reference catalog fields or relationships. Names are camelCase (D-044).

```javascript
city
ageYears
totalSpendGbp
orders
```

Within relationship predicates, qualified names reference the current relationship variable:

```javascript
o.totalGbp
c.addedAt
```

Literals in 0.1:

- String literals, single- or double-quoted.
- Number literals.
- `true` and `false`.
- `ago("…")` and `date("…")`.
- List literals, only on the right-hand side of `in`.

There is no null literal in 0.1. `===` and `!==` are null-safe: `favouriteCategory !== "x"` also matches customers whose value is empty; ordering operators never match null (D-058). Money values are sent to GraphQL as decimal strings such as `"1234.50"` (D-058); numbers are written as `-?[0-9]+(\.[0-9]+)?` (no exponent, no leading `+`, no `_`) and kept as text, never as floats (D-065). Strings use `'` or `"`; the only escapes are `\\`, `\"` and `\'`, and a string cannot span lines. There are no comments. A literal on the left of a comparison (`1000 < totalSpendGbp`) is a `SYNTAX_ERROR` (D-065).

---

## 4. Expression Grammar

The following EBNF describes the 0.1 grammar.

```ebnf
expression      = or_expression ;

or_expression   = and_expression ,
                  { "||" , and_expression } ;

and_expression  = unary_expression ,
                  { "&&" , unary_expression } ;

unary_expression =
                    "!" , unary_expression
                  | "(" , expression , ")"
                  | relation_expression
                  | membership
                  | comparison ;

comparison      = field_reference ,
                  comparison_operator ,
                  value ;

comparison_operator =
                    "==="
                  | "!=="
                  | ">"
                  | ">="
                  | "<"
                  | "<=" ;

membership      = field_reference , "in" , list_literal ;

list_literal    = "[" , literal , { "," , literal } , "]" ;

relation_expression =
                    relation_reference ,
                    "." ,
                    quantifier ,
                    "(" ,
                    identifier ,
                    "=>" ,
                    expression ,
                    ")" ;

quantifier      = "some" | "none" | "every" ;

value           = literal ;

literal         = string_literal
                  | number_literal
                  | boolean_literal
                  | time_literal ;

boolean_literal = "true" | "false" ;

time_literal    = "ago" , "(" , string_literal , ")"
                  | "date" , "(" , string_literal , ")" ;

field_reference = identifier ,
                  { "." , identifier } ;

relation_reference =
                    identifier ,
                    { "." , identifier } ;
```

Notes:

- `value` is literal-only in 0.1. A field reference on the right-hand side is a field-to-field comparison and returns `UNSUPPORTED_OPERATOR` (D-045).
- `~=`, `==` and `!=` are lexed but not part of `comparison_operator`; they return `UNSUPPORTED_OPERATOR`.
- `field_reference` and `relation_reference` start the same way. The parser decides by lookahead: a path followed by `.some(`, `.none(` or `.every(` is a relation expression. `some`, `none`, `every`, `in`, `true`, `false`, `ago` and `date` are reserved words.
- Field and relationship references are resolved semantically using the current entity and lambda scopes.

Statements, variable declarations, assignments, loops, arbitrary function invocations and JavaScript object mutation are excluded.

---

## 5. Relationship Traversal and Join Semantics

### 5.1 Conceptual model

Relationships are navigable properties of the customer entity:

```text
Customer
  +-- orders[]    : Order
  +-- cartItems[] : CartItem
  +-- events[]    : Event
```

The agent does not specify physical foreign keys or SQL join conditions. It uses relationship predicates.

### 5.2 Existence predicate: `some`

```javascript
orders.some(o => o.totalGbp > 1000)
```

A customer qualifies if at least one related order satisfies the predicate.

GraphQL representation:

```graphql
orders: {
  some: {
    totalGbp: {
      gt: 1000
    }
  }
}
```

### 5.3 Non-existence predicate: `none`

```javascript
orders.none(o => o.orderStatus === "cancelled")
```

A customer qualifies if no related order satisfies the predicate. This is equivalent to negating a corresponding `some` condition.

### 5.4 Universal predicate: `every`

```javascript
orders.every(o => o.totalGbp > 0)
```

Every related order must satisfy the predicate. For an empty relationship, `every` returns true.

SQL null handling requires explicit server-side normalization so that three-valued logic does not change this meaning.

### 5.5 Nested relationships

Relationship predicates may be nested where the catalog defines nested relationships. In 0.1 the catalog exposes only the three customer relationships, so nesting is limited to one level. Whether order items and products become filterable relationships is open (D-042).

Lambda identifiers are local aliases used for field resolution. They are not executable functions. Every field inside a lambda body must be qualified with its variable (`o.totalGbp`); a bare customer field there is `INVALID_RELATION_SCOPE`, and a nested lambda cannot reuse an enclosing variable name (D-065).

### 5.6 Predicate scope

```javascript
orders.some(o => o.totalGbp > 1000) &&
orders.some(o => o.orderStatus === "delivered")
```

This requires an order above 1,000 and a delivered order. They may be different orders.

By contrast:

```javascript
orders.some(o =>
  o.totalGbp > 1000 &&
  o.orderStatus === "delivered"
)
```

Requires the same order to satisfy both conditions.

The compiler must preserve this distinction.

### 5.7 Relationship discovery

There is no runtime introspection (D-044). The Customer service's gqlgen schema (`.graphqls`) is the single source of truth. Proposal's catalog is generated at build time from that schema plus a small metadata file. The metadata file holds prompt descriptions, allowed operators per field and the quantifiers each relationship supports. Drift between the schema and the metadata must fail the build.

Arbitrary joins between unrelated fields or tables are outside the DSL.

---

## 6. Field-to-Field Comparisons (reserved)

```javascript
totalSpendGbp > openCartValueGbp
```

Field-to-field comparison is reserved in 0.1 and returns `UNSUPPORTED_OPERATOR` (D-045). Supporting it would need a dedicated `compare` input type in the GraphQL schema, because a simple `{ gt: 1000 }` input cannot express it.

The typed AST should still keep the right-hand operand as a sum type (`FieldReference | Literal`) so that the feature can be added later without restructuring.

---

## 7. Query Projection and Selection

Filtering and selection are independent concerns. In this project the selection is not part of the language exposed to the agent (D-041).

- A filter determines which customers qualify. The agent writes it.
- A selection determines which fields come back. Proposal code fixes it per purpose (§2.1).

Example of the GraphQL that Proposal generates for the preview purpose:

```graphql
query AudiencePreview($filter: CustomerFilter!) {
  audiencePreview(filter: $filter) {
    count
    coverage {
      both
      emailOnly
      smsOnly
      unreachable
    }
  }
}
```

The operation and type names above are illustrative. The final schema is defined in the Customer service (D-044).

Filtered nested selections are not used in 0.1.

---

## 8. Compilation to GraphQL

### 8.1 Compilation stages

**Stage 1: Lexing and parsing.** Convert the expression source into a syntactic AST with a dedicated Pratt parser (D-038).

**Stage 2: Name resolution.** Resolve fields, relationships and lambda variables against the generated catalog (D-044).

**Stage 3: Type checking.** Ensure operands, operators, `in` lists, time literals and relationship predicates are compatible.

**Stage 4: AST normalization.** Convert supported expressions into a typed, implementation-independent filter representation.

**Stage 5: GraphQL generation.** Create the GraphQL operation and variables from the normalized filter and the purpose's fixed selection.

**Stage 6: Execution.** Send the operation to the Customer service with Proposal's service token (D-043).

### 8.2 Example transformation

The sample brief "users in İstanbul who added to cart in the last 30 days and did not buy":

```javascript
city === "İstanbul" &&
cartItems.some(c => c.addedAt >= ago("30d")) &&
orders.none(o => o.orderDate >= ago("30d"))
```

Normalized GraphQL filter variable:

```json
{
  "and": [
    { "city": { "eq": "İstanbul" } },
    { "cartItems": { "some": { "addedAt": { "gte": { "ago": "30d" } } } } },
    { "orders": { "none": { "orderDate": { "gte": { "ago": "30d" } } } } }
  ]
}
```

The Customer service canonicalizes the city (D-040), resolves `ago` against `now()` (D-039) and adds the reachability condition (D-007) before running parameterized SQL.

The exact GraphQL input representation is defined by the Customer schema. The compiler uses the schema's actual input types, not an assumed universal filter standard.

### 8.3 Operator mapping

| DSL        | GraphQL representation (draft) |
| ---------- | ------------------------------ |
| `===`      | `eq`                           |
| `!==`      | `ne`                           |
| `>`        | `gt`                           |
| `>=`       | `gte`                          |
| `<`        | `lt`                           |
| `<=`       | `lte`                          |
| `in`       | `in`                           |
| `ago()`    | `{ ago: "…" }` value           |
| `date()`   | date string value              |
| `&&`       | `and`                          |
| `\|\|`     | `or`                           |
| `!`        | `not`                          |
| `.some()`  | Relation `some`                |
| `.none()`  | Relation `none`                |
| `.every()` | Relation `every`               |

The filter input types are defined in `internal/customer/graph/schema.graphqls` (D-058): `StringFilter { eq, ne, in }`, numeric, `Decimal`, `Date` and `DateTime` filters `{ eq, ne, gt, gte, lt, lte, in }`, `BooleanFilter { eq, ne }`, relation filters `{ some, none, every }`, and `TimeValue @oneOf { ago, date, at }` for time operands.

---

## 9. Type System and Catalog

The DSL uses a schema-derived, statically validated type system. Each identifier must resolve to a known field or relationship in the active scope.

### 9.1 Type categories

| Category     | Examples                       | Operations                           |
| ------------ | ------------------------------ | ------------------------------------ |
| String       | `city`, `loyaltyTier`          | `===`, `!==`, `in`                   |
| Numeric      | `ageYears`, `totalSpendGbp`    | `===`, `!==`, ordering, `in`         |
| Boolean      | `hasOpenCart`, `retired`       | `===`, `!==` with `true` / `false`   |
| Temporal     | `signupDate`, `lastOrderDate`  | Ordering with `ago()` / `date()`     |
| Relationship | `orders`, `cartItems`          | `some`, `none`, `every`              |

### 9.2 Catalog (D-042)

The filter catalog contains every non-personal field. Names below are the camelCase GraphQL names of current `customer_db` columns. The final list is generated from the schema (D-044).

**Customer fields in the catalog**

| Group | Fields |
| ----- | ------ |
| Location and language | `countryCode`, `city`, `language`, `timezone` |
| Activity | `signupDate`, `lastActiveDate`, `preferredPlatform`, `sessions90d` |
| Orders and spend | `totalOrders`, `totalSpendGbp`, `avgOrderValueGbp`, `firstOrderDate`, `lastOrderDate`, `favouriteCategory` |
| Cart | `lastCartAddDate`, `hasOpenCart`, `openCartValueGbp` |
| Loyalty and campaigns | `loyaltyTier`, `loyaltyPoints`, `campaignsReceived90d`, `emailsOpened90d`, `lastCampaignSentAt` |
| Demographics | `ageYears`, `sex`, `maritalStatus`, `dependants`, `yearlySalaryGbp`, `yearlyPensionGbp`, `retired`, `commuteDistanceKm` |
| Vehicle | `vehicleMake`, `vehicleModel`, `vehicleYear`, `vehicleType` |

**Relationship fields in the catalog**

| Relationship | Fields |
| ------------ | ------ |
| `orders` | `orderDate`, `orderStatus`, `itemsCount`, `subtotalGbp`, `discountCode`, `discountGbp`, `shippingGbp`, `totalGbp`, `paymentMethod`, `platform`, `shippingCity`, `countryCode` |
| `cartItems` | `addedAt`, `cartStatus`, `quantity`, `unitPriceGbp`, `convertedAt` |
| `events` | `eventType`, `eventTime`, `platform`, `category`, `valueGbp` |

**Not in the catalog**

| Reason | Fields |
| ------ | ------ |
| Personal identifiers | `firstName`, `lastName`, `email`, `phoneE164`, `addressStreet`, `addressCity`, `addressPostcode`, `dateOfBirth`, `employerCompany` |
| Reachability and consent: enforced by the server and the channel plan step (D-007, D-010) | `emailOptIn`, `smsOptIn`, `emailBounced`, `accountStatus`, `deletedAt`, `consentUpdatedAt` |

Using a field outside the catalog fails with `UNKNOWN_FIELD`, both in Proposal and, authoritatively, in the Customer service.

Fields like age, sex and salary allow sensitive targeting. Including them is a deliberate decision recorded in D-042.

### 9.3 Type checking

Valid:

```javascript
totalSpendGbp > 1000
```

Invalid (`TYPE_MISMATCH`):

```javascript
totalSpendGbp > "one thousand"
```

A relationship must be used with a valid quantifier:

```javascript
orders.some(o => o.totalGbp > 1000)
```

Any other method is rejected with `UNSUPPORTED_EXPRESSION`:

```javascript
orders.map(o => o.totalGbp)
```

### 9.4 Nullability

Null semantics (D-058): `===`/`!==` compile to `IS NOT DISTINCT FROM` / `IS DISTINCT FROM`, so an empty field matches `!==` but not `===`; `>`, `>=`, `<`, `<=` never match null; 0.1 has no `isNull` operator.

### 9.5 Numeric representation

GraphQL's built-in `Int` is a signed 32-bit integer. Money columns are `numeric(12,2)` in `customer_db` and travel as the custom `Decimal` scalar, a JSON string such as `"1234.50"` (D-058); `Date` is `YYYY-MM-DD` and `DateTime` is RFC 3339 UTC. The DSL's numeric type system follows the server schema rather than assuming every number is a GraphQL `Int` or `Float`.

---

## 10. Parser Implementation in Go

The grammar is JavaScript-inspired but not fully valid JavaScript. For example, the lexer must recognize `~=` to report it, and `~=` is not valid ECMAScript.

**Decision (D-038): a dedicated lexer and Pratt parser in Go.** This replaces the earlier plan in D-007 to parse with `expr-lang/expr`, whose syntax does not include `===`, `~=` or arrow lambdas.

Reasons:

- Complete control over syntax.
- No accidental JavaScript semantics.
- A small, predictable language surface.
- Source-positioned errors that the agent can use in a repair attempt (D-027).

Rejected: parsing with a JavaScript parser such as `goja` after preprocessing custom operators. It would need token-aware preprocessing and source-position mapping.

The parser produces a strongly typed AST:

```text
Expression
  ├── LogicalExpression
  │     ├── And
  │     └── Or
  ├── UnaryExpression
  │     └── Not
  ├── ComparisonExpression
  │     ├── Field
  │     ├── Operator
  │     └── Value (Literal; FieldReference reserved)
  ├── MembershipExpression
  │     ├── Field
  │     └── Values
  └── RelationExpression
        ├── Relationship
        ├── Quantifier
        ├── BoundVariable
        └── Predicate
```

A separate semantic pass resolves names and types before GraphQL generation. The parser should be covered by fuzz tests.

---

## 11. GraphQL Server Contract

The Proposal compiler and the Customer GraphQL server share a schema-defined contract. The schema is the single source of truth (D-044).

The server must expose:

- Root query fields with filter arguments, one per purpose (preview, recipients).
- Filter input types for the catalog fields and operators in §8.3.
- Relation filter types for `orders`, `cartItems` and `events`.
- Relative time values (`ago`) resolved at query time (D-039).

The server must enforce, independently of the client (D-043):

- Service tokens and scopes (D-063): each caller sends `Authorization: Bearer <token>`; a missing or unknown token gets HTTP 401 before GraphQL. Scopes are declared in the schema with `@hasScope`: Proposal has `audience:read` (preview, recipients), Notification has `consent:read` (consent check), and only an admin token has `consent:write` (`updateConsent`) plus `consent:read`. A field outside the caller's scopes returns `FORBIDDEN`.
- The field whitelist (D-042).
- The reachability and consent condition on every audience query (D-007).
- City canonicalization and known-value validation (D-040).
- Query depth and complexity limits, result-size limits and database timeouts.

The server never receives executable JavaScript. It receives GraphQL operations and variables.

### 11.1 Database execution

The Customer service translates the filter into parameterized SQL through a whitelisted compiler (D-007). Relational predicates become `EXISTS` / `NOT EXISTS` subqueries. Index choices are implementation details of the Customer service and follow the project's migration rules.

### 11.2 Relationship resolution

The client describes predicates, not the physical execution strategy. The server may use joins, `EXISTS` subqueries or batching.

---

## 12. Validation, Safety and Resource Limits

The compiler rejects syntactically valid JavaScript that falls outside the language:

```javascript
fetch("https://example.com")
```

```javascript
customer.loyaltyPoints = 9999
```

```javascript
eval("some code")
```

```javascript
orders.reduce((a, b) => a + b.totalGbp, 0)
```

Function calls are only permitted for `ago`, `date` and the relationship quantifiers.

The compiler enforces:

- Known field and relationship names.
- Allowed operators for each field type.
- Maximum expression length.
- Maximum AST depth and node count.
- Maximum relationship depth.
- Valid lambda scoping.
- Safe serialization into GraphQL variables.

The GraphQL server enforces the same limits again, plus access control (§11). Client validation is a convenience, not a security boundary.

### 12.1 Errors

| Error                    | Meaning                                                     |
| ------------------------ | ----------------------------------------------------------- |
| `SYNTAX_ERROR`           | Invalid expression grammar                                  |
| `UNKNOWN_FIELD`          | Field not in the catalog                                    |
| `UNKNOWN_RELATION`       | Invalid relationship path                                   |
| `UNKNOWN_VALUE`          | Value not in a closed list, e.g. an unknown city (D-040)    |
| `TYPE_MISMATCH`          | Incompatible operand types                                  |
| `UNSUPPORTED_OPERATOR`   | Reserved or rejected operator, or field-to-field comparison |
| `INVALID_RELATION_SCOPE` | Invalid lambda or traversal                                 |
| `COMPLEXITY_LIMIT`       | Query exceeds configured limits: nesting depth > 8, more than 100 condition nodes, or an `in` list over 100 values (D-062) |
| `UNSUPPORTED_EXPRESSION` | JavaScript construct outside the DSL                        |

The list is final (D-065). The parser (Proposal) emits `SYNTAX_ERROR`, `UNSUPPORTED_OPERATOR`, `UNSUPPORTED_EXPRESSION`, `INVALID_RELATION_SCOPE` and `COMPLEXITY_LIMIT` (source of at most 4096 bytes, depth 8, 100 nodes, 100 `in` values); the semantic pass emits `UNKNOWN_FIELD`, `UNKNOWN_RELATION`, `UNKNOWN_VALUE` and `TYPE_MISMATCH`. Parsing stops at the first error. Errors include the 1-based line, rune column, start-end range and a `near "…"` excerpt so that the agent can repair an invalid expression. The Customer service returns its own codes in GraphQL `extensions.code`; `UNKNOWN_VALUE` carries up to three `suggestions` (Levenshtein ≤ 2 or prefix on the canonical key), and `extensions.filterPath` points at the offending filter field. Empty `and`/`or` lists, a field without an operator and a relationship without a quantifier are rejected with `BAD_USER_INPUT` (D-062). The final list of error codes is open (D-045).

---

## 13. Query Examples

### Example A: Location and spend

```javascript
city === "İstanbul" && totalSpendGbp >= 1000
```

### Example B: Several cities

```javascript
city in ["İstanbul", "Ankara", "İzmir"] && loyaltyTier in ["Gold", "Platinum"]
```

### Example C: Relative time on summary fields

```javascript
lastCartAddDate >= ago("30d") && hasOpenCart === true
```

### Example D: Relationship existence

```javascript
orders.some(o => o.totalGbp > 1000)
```

### Example E: The sample brief

```javascript
city === "İstanbul" &&
cartItems.some(c => c.addedAt >= ago("30d")) &&
orders.none(o => o.orderDate >= ago("30d"))
```

### Example F: Negative relationship filtering

```javascript
orders.none(o => o.orderStatus === "cancelled")
```

### Example G: Universal relationship filtering

```javascript
orders.every(o => o.totalGbp > 0)
```

---

## 14. Decision Status

| Area                          | Status                                                            |
| ----------------------------- | ----------------------------------------------------------------- |
| Parser technology             | Decided: dedicated Pratt parser (D-038)                           |
| Relative time                 | Decided: `ago()` resolved by the server (D-039)                   |
| City matching                 | Decided: closed list and canonical key (D-040)                    |
| Entity and selection          | Decided: fixed in Proposal; the LLM writes the filter only (D-041) |
| Catalog                       | Decided: all non-personal fields (D-042)                          |
| Access control                | Decided: GraphQL endpoint, service tokens and scopes (D-043)      |
| Schema discovery and naming   | Decided: build-time catalog from the schema, camelCase (D-044)    |
| 0.1 operator scope            | Decided (D-045, D-046)                                            |
| `~=` behavior                 | Reserved; matching mode and `pg_trgm` need a new decision         |
| Field-to-field comparison     | Reserved; needs a `compare` input type                            |
| `==` and `!=`                 | Rejected                                                          |
| Null handling, number format  | Decided (D-058); DSL numeric literal syntax open                  |
| Filter input type shape       | Decided (D-058)                                                   |
| Time units                    | Decided: `d` and `h` (D-058)                                      |
| Order items and products      | Open (D-042)                                                      |
| Sorting, pagination           | Not designed                                                      |
| Aggregation                   | Outside the initial scope                                         |
| Language name                 | Not selected                                                      |

### 14.1 Potential language extensions

Future versions could support:

```javascript
// String matching
name.startsWith("Jo")

// Explicit range predicate
ageYears.between(25, 40)

// Text search
favouriteCategory ~= "electronics"

// Field-to-field comparison
totalSpendGbp > openCartValueGbp
```

These are proposals, not part of the 0.1 grammar. Extensions should keep the separation between expressions, GraphQL construction and server-side database execution.

---

## 15. Version 0.1 Scope (D-045)

1. Parse scalar comparisons and Boolean expressions with a dedicated Pratt parser.
2. Support `===`, `!==`, `>`, `>=`, `<`, `<=`, `in`, `&&`, `||`, `!` and parentheses.
3. Support string, number and boolean literals, `ago()` and `date()`.
4. Resolve field references against the catalog generated from the Customer schema.
5. Support the `orders`, `cartItems` and `events` relationships with `some`, `none` and `every`.
6. Return `UNSUPPORTED_OPERATOR` for `~=`, `==`, `!=` and field-to-field comparison.
7. Compile expressions into typed GraphQL filter variables.
8. Add the fixed entity and the purpose's selection in Proposal code.
9. Execute the operation with Proposal's service token; the Customer service enforces access, the catalog, reachability and limits.

---

## 16. Summary

The DSL is a restricted, typed expression language for audience filters on top of the Customer service's GraphQL API.

**Filter expression + relationship predicates → typed GraphQL filter**

It separates three responsibilities:

**The agent** writes the filter expression only.

**The Proposal compiler** validates it against the generated catalog and translates it into GraphQL.

**The Customer GraphQL server** enforces access and resolves the data through its whitelisted, parameterized database layer.

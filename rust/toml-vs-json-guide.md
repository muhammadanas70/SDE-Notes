# TOML vs JSON: A Complete Engineering Guide

*Data formats, grammars, semantics, tooling, and production implementations in Rust and Go*

---

## Table of Contents

1. [Origins and Design Philosophy](#1-origins-and-design-philosophy)
2. [Core Data Model](#2-core-data-model)
3. [Syntax Deep Dive: JSON](#3-syntax-deep-dive-json)
4. [Syntax Deep Dive: TOML](#4-syntax-deep-dive-toml)
5. [Side-by-Side Type Mapping](#5-side-by-side-type-mapping)
6. [Comments, Whitespace, and Human Editability](#6-comments-whitespace-and-human-editability)
7. [Nesting Models: Objects vs Tables](#7-nesting-models-objects-vs-tables)
8. [Dates, Times, and Native Types](#8-dates-times-and-native-types)
9. [Grammar and Parsing Theory](#9-grammar-and-parsing-theory)
10. [Schema Validation](#10-schema-validation)
11. [Architecture: Where Each Format Lives in a System](#11-architecture-where-each-format-lives-in-a-system)
12. [Rust Implementation](#12-rust-implementation)
13. [Go Implementation](#13-go-implementation)
14. [Error Handling Patterns](#14-error-handling-patterns)
15. [Round-Tripping and Comment Preservation](#15-round-tripping-and-comment-preservation)
16. [Performance Characteristics](#16-performance-characteristics)
17. [Security Considerations](#17-security-considerations)
18. [Ecosystem and Real-World Adoption](#18-ecosystem-and-real-world-adoption)
19. [Migration and Conversion](#19-migration-and-conversion)
20. [Decision Framework: Which to Use](#20-decision-framework-which-to-use)
21. [Quick Reference Summary](#21-quick-reference-summary)

---

## 1. Origins and Design Philosophy

### 1.1 JSON — JavaScript Object Notation

JSON was extracted from a subset of JavaScript's object literal syntax and standardized around 2001–2006 by Douglas Crockford. Its charter was narrow and deliberate: be a **data interchange format** — something a program emits and another program parses, with no ambiguity, no configuration knobs, and no room for a parser to "interpret" anything. It is now formalized in two competing-but-compatible standards:

- **RFC 8259** (IETF) — the wire-format standard used by HTTP APIs.
- **ECMA-404** — the minimal grammar standard.

JSON's philosophy in one sentence: **the grammar should be so small that every implementation produces byte-identical parse trees.** There is no place in the spec for comments, trailing commas, or multiple ways to write the same value, because every "convenience" feature is a potential source of interoperability bugs between independently-written parsers.

### 1.2 TOML — Tom's Obvious, Minimal Language

TOML was created in 2013 by Tom Preston-Werner (co-founder of GitHub) explicitly as a **configuration file format**, in reaction to the ambiguity of YAML (implicit typing surprises like the "Norway problem" where `NO` parses as boolean `false`) and the poor human-ergonomics of JSON as something a person hand-edits (no comments, fussy comma/bracket matching, awkward nesting).

TOML's charter: **"It should be obvious what the semantics are just by reading the file."** It borrows syntax heavily from INI files (`[section]` headers, `key = value` lines) but bolts on a real, unambiguous type system and formal spec. It reached a stable **v1.0.0** in January 2021, which is the version this guide targets throughout.

### 1.3 The Fundamental Distinction

| | JSON | TOML |
|---|---|---|
| **Primary purpose** | Machine-to-machine data interchange | Human-edited configuration |
| **Optimized for** | Parser simplicity, wire compactness | Human readability/editability, diff-friendliness |
| **Typical producer** | A program (serializer) | A person, in a text editor |
| **Typical consumer** | A program (deserializer) | A program, at startup |
| **Comments** | Not permitted | First-class |
| **Root value** | Any JSON value (object, array, string, etc.) | Always an implicit top-level table |

This one distinction — *"who writes this file, a human or a machine?"* — explains almost every syntactic difference between the two formats. Keep it in your head as the mental model for the rest of this guide.

---

## 2. Core Data Model

Both formats are built on a small set of primitive and composite types, but they diverge in exactly which primitives exist and how composites nest.

### 2.1 JSON's Data Model (from ECMA-404 / RFC 8259)

```
value    := object | array | string | number | "true" | "false" | "null"
object   := "{" (string ":" value ("," string ":" value)*)? "}"
array    := "[" (value ("," value)*)? "]"
```

JSON has exactly **six** value kinds: object, array, string, number, boolean, null. `number` is a single unified type — JSON does not distinguish integers from floats at the grammar level (that distinction is a decision each *implementation* makes when deserializing).

### 2.2 TOML's Data Model (from the TOML v1.0.0 spec)

TOML has a richer primitive set:

```
value := string | integer | float | boolean
       | offset-date-time | local-date-time | local-date | local-time
       | array | inline-table
```

Key differences from JSON's model:
- **Integers and floats are distinct types**, not a single "number."
- **Four separate date/time types** are native to the grammar (JSON has none — dates are just strings by convention).
- There is no `null`. TOML has no way to represent "key present, value absent." Omit the key instead.
- The document root is **always a table** (TOML's word for what JSON calls an object) — you cannot have a TOML document whose root is an array or a string.

---

## 3. Syntax Deep Dive: JSON

### 3.1 The Complete JSON Grammar (informal)

```json
{
  "string_example": "hello world",
  "escaped_string": "line1\nline2\ttabbed\u00e9",
  "integer_example": 42,
  "negative_example": -17,
  "float_example": 3.14159,
  "exponent_example": 6.022e23,
  "boolean_true": true,
  "boolean_false": false,
  "null_example": null,
  "array_example": [1, 2, 3, "mixed", true],
  "nested_object": {
    "inner_key": "inner_value",
    "deeply": {
      "nested": {
        "value": 100
      }
    }
  },
  "array_of_objects": [
    {"id": 1, "name": "first"},
    {"id": 2, "name": "second"}
  ]
}
```

### 3.2 Rules That Trip People Up

1. **No trailing commas.** `[1, 2, 3,]` is invalid JSON. This is the single most common hand-authored JSON syntax error.
2. **Keys must be double-quoted strings.** `{key: "value"}` is invalid; only `{"key": "value"}` is valid. Single quotes are never allowed anywhere in JSON.
3. **No comments, ever.** Not `//`, not `#`, not `/* */`. This is intentional — see §1.1. (JSONC and JSON5 are *non-standard supersets* some tools accept, but they are not JSON.)
4. **Numbers cannot have leading zeros** (`0123` is invalid), cannot have a trailing decimal point with nothing after it (`1.` is invalid — must be `1.0`), and cannot be `NaN` or `Infinity`.
5. **Only double quotes for strings**, and only a fixed set of escape sequences: `\"`, `\\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t`, `\uXXXX`.
6. **Root value can be anything**, not just an object — `42` alone, or `"hello"` alone, or `[1,2,3]` alone, are all valid complete JSON documents (per RFC 8259; ECMA-404 also permits this since its 2017 revision).

---

## 4. Syntax Deep Dive: TOML

### 4.1 Basic Key/Value Pairs

```toml
# This is a comment. TOML comments start with # and run to end of line.

title = "TOML Example"          # basic string
count = 42                       # integer
ratio = 3.14159                  # float
enabled = true                   # boolean
empty_array = []
tags = ["rust", "go", "systems"] # array

# Bare keys: letters, digits, underscore, dash — no quotes needed
bare-key123 = "value"

# Quoted keys: needed when the key contains special characters
"key with spaces" = "value"
"key.with.dots" = "value"        # the dots here are literal, part of ONE key

# Dotted keys: create implicit nested tables inline
physical.color = "orange"
physical.shape = "round"
# equivalent to:
# [physical]
# color = "orange"
# shape = "round"
```

### 4.2 String Types (TOML has four)

```toml
basic_string       = "Hello, \"world\"\nNewline and escapes work"
literal_string     = 'C:\Users\name\no_escapes_processed'
multiline_basic     = """
Roses are red
Violets are blue"""
multiline_literal   = '''
No \n escape processing here.
Raw backslashes stay raw.'''
```

- **Basic strings** (`"..."`) — double-quoted, support backslash escapes, similar to JSON strings.
- **Literal strings** (`'...'`) — single-quoted, **zero escaping**, whatever you type is what you get. Ideal for Windows paths and regexes.
- **Multi-line basic** (`"""..."""`) — basic strings that can span lines; a leading newline right after `"""` is trimmed.
- **Multi-line literal** (`'''...'''`) — literal strings across lines, still zero escaping.

This is a capability JSON simply does not have — there is no way to write a raw, unescaped multi-line string in JSON.

### 4.3 Integers and Floats (distinct types, richer literals)

```toml
dec_int      = 42
dec_explicit = +42
neg_int      = -17
hex_int      = 0xDEADBEEF
oct_int      = 0o755
bin_int      = 0b11010110
underscored  = 1_000_000        # underscores for readability, ignored by parser

float_basic  = 3.1415
float_exp    = 6.022e23
float_neg    = -1.5e-10
float_inf    = inf
float_neg_inf = -inf
float_nan    = nan
```

TOML natively supports hex/octal/binary integer literals, underscore digit separators, and IEEE 754 special floats (`inf`, `nan`) — none of which exist in JSON's number grammar.

### 4.4 Tables (TOML's word for "object")

```toml
# A table header. Everything below belongs to [server] until the next header.
[server]
host = "0.0.0.0"
port = 8080

[server.tls]              # nested table via dotted header
enabled = true
cert_path = "/etc/ssl/cert.pem"

[database]
url = "postgres://localhost/mydb"

# Inline table — compact, single-line, JSON-object-like syntax
point = { x = 1, y = 2 }
```

`[server.tls]` is exactly equivalent to writing `[server]` with a nested `tls = {...}`. TOML lets you choose the flat dotted-header style (better diffs, more readable for deeply nested config) or the inline-table style (better for small, tightly-coupled groups like a single coordinate pair).

### 4.5 Arrays of Tables (no JSON equivalent syntax)

This is TOML's most distinctive feature — a way to express a JSON `"array": [{...}, {...}]` pattern without repeating brackets:

```toml
[[products]]
name = "Widget"
sku = 1001

[[products]]
name = "Gadget"
sku = 1002
color = "blue"
```

is exactly equivalent to this JSON:

```json
{
  "products": [
    {"name": "Widget", "sku": 1001},
    {"name": "Gadget", "sku": 1002, "color": "blue"}
  ]
}
```

Each `[[products]]` header **appends a new table** to the `products` array. This reads far more naturally for config files with repeated blocks (think: multiple `[[servers]]`, multiple `[[dependencies]]`) than JSON's bracket-heavy nested-array-of-objects syntax.

---

## 5. Side-by-Side Type Mapping

| Concept | JSON | TOML |
|---|---|---|
| String | `"text"` | `"text"`, `'text'`, `"""..."""`, `'''...'''` |
| Integer | part of unified `number` | distinct `integer` type; dec/hex/oct/bin |
| Float | part of unified `number` | distinct `float` type; supports `inf`/`nan` |
| Boolean | `true` / `false` | `true` / `false` (identical) |
| Null / absence | `null` | **no null** — omit the key |
| Array | `[1, 2, 3]` | `[1, 2, 3]` (identical syntax) |
| Object / map | `{"k": "v"}` | inline table `{k = "v"}` or `[table]` header |
| Array of objects | `[{...}, {...}]` | `[[array-of-tables]]` or inline array of inline tables |
| Date/time | not native — string convention (ISO 8601) | 4 native types (§8) |
| Comments | none | `#` to end of line |
| Root value | any JSON value | always an implicit table |

---

## 6. Comments, Whitespace, and Human Editability

This is the crux of the human-vs-machine design split.

```toml
# Server configuration
[server]
port = 8080          # default port, override with $PORT env var
# host = "127.0.0.1" # uncomment to bind to localhost only
host = "0.0.0.0"
```

You cannot express "override with $PORT env var" as an inline note in JSON at all. Real-world workarounds people resort to in JSON configs:
- A convention like `"_comment": "..."` as a fake key (pollutes the actual data, consumers must filter it out).
- Switching to JSONC/JSON5 (non-standard; VS Code's `settings.json` uses JSONC, but a strict RFC 8259 parser will reject it).
- Maintaining separate documentation outside the file entirely.

TOML needs none of these hacks because comments are part of the grammar itself.

**Whitespace sensitivity**: neither format is whitespace-significant in the way Python or YAML are (no meaningful indentation). Both use explicit delimiters (`{}`/`[]` for JSON, `[headers]`/newlines for TOML) rather than indentation to denote structure — which is one reason both are less error-prone than YAML for machine generation, even though TOML is friendlier to hand-editing than JSON is.

---

## 7. Nesting Models: Objects vs Tables

JSON nesting is purely structural — braces inside braces, with no limit and no special syntax for "this is deeply nested":

```json
{
  "a": {
    "b": {
      "c": {
        "d": "deep value"
      }
    }
  }
}
```

TOML gives you the *same* capability via dotted table headers, but the deeper you go, the more it starts to look less like nesting and more like a flat namespace with dot-separated keys:

```toml
[a.b.c]
d = "deep value"
```

This is one line instead of five, and it is **exactly equivalent**. This is why TOML tends to stay flatter and more scannable than JSON for config-shaped data — most config never needs more than 2-3 levels, and TOML's table-header syntax keeps each level visually distinct with its own line rather than accumulating indentation.

**The trap**: you cannot redefine a table once declared, and you cannot mix a table header with an inline definition of the same path. This is invalid:

```toml
[fruit]
apple = "red"

[fruit.apple]   # ERROR: 'apple' is already defined as a string, not a table
texture = "smooth"
```

JSON has no equivalent trap because there's no concept of "declaring" a path twice — you just nest braces correctly the first time, and a JSON parser has no notion of "redeclaration" at all; it's a pure tree literal.

---

## 8. Dates, Times, and Native Types

This is TOML's standout feature relative to JSON. TOML has **four** native temporal types, unquoted, directly parseable to real datetime objects by a spec-compliant parser:

```toml
offset_datetime  = 1987-07-05T17:45:00Z          # RFC 3339, with timezone
offset_datetime2 = 1987-07-05T17:45:00-05:00     # explicit UTC offset
local_datetime   = 1987-07-05T17:45:00           # no timezone info
local_date       = 1987-07-05                    # date only
local_time       = 17:45:00                      # time only
```

In JSON, there is **no date type**. Every JSON-based system invents its own convention:
- ISO 8601 strings: `"date": "1987-07-05T17:45:00Z"` — the most common, but the parser has no idea it's a date; your application code must know to `Date.parse()` it.
- Unix epoch integers: `"date": 552065100`
- Two separate fields: `{"date": "1987-07-05", "time": "17:45:00"}`

None of these are wrong, but none of them are *interoperable by default* the way TOML's native date type is — a TOML parser hands your Rust/Go code an actual typed date object without any string-parsing step on your part.

---

## 9. Grammar and Parsing Theory

### 9.1 JSON Grammar Complexity

JSON's grammar is regular-ish and can be parsed with a simple recursive-descent parser in well under 500 lines in any language. It has:
- 6 value types
- No lookahead ambiguity (each token unambiguously starts exactly one production)
- No context sensitivity (a `{` always means "start object" regardless of where you are)

This is precisely why JSON parsers exist for nearly every language ever created, and why writing your *own* JSON parser is a common teaching exercise — it's genuinely one of the simplest non-trivial grammars in wide use.

### 9.2 TOML Grammar Complexity

TOML's official grammar is published as an **ABNF spec** (`toml.abnf` in the spec repo) and is meaningfully more complex than JSON's:
- Table headers require tracking a mutable "currently open table" parser state (which JSON's stateless tree grammar never needs).
- Dotted keys need path-resolution logic — `a.b.c = 1` must merge correctly with a prior `[a.b]` table header.
- Four string types and four datetime types multiply the tokenizer's state machine.
- Redefinition rules (§7) require the parser to track *which paths have already been defined* — genuine semantic validation beyond pure syntax.

This is the real cost of TOML's human-friendliness: parser authors do more work so that document authors do less. This tradeoff is invisible to you as a *consumer* of a mature TOML library (like the ones in §12–13) but explains why TOML parsers are newer, less numerous, and historically had more version-to-version compatibility churn (TOML went through breaking spec changes at v0.4, v0.5, v0.6 before stabilizing at v1.0.0 in 2021) than the rock-solid, unchanging JSON grammar.

### 9.3 ASCII: Parsing Pipeline Comparison

```
JSON PARSING PIPELINE
======================

  bytes ──> [ Lexer/Tokenizer ] ──> [ Recursive-Descent Parser ] ──> Value Tree
              (stateless)              (stateless, no symbol table)
              tokens: { } [ ] : , "str" num true false null

  Single pass. No mutable "current table" needed. No path resolution.


TOML PARSING PIPELINE
======================

  bytes ──> [ Lexer/Tokenizer ] ──> [ Parser w/ Table-Path State ] ──> Value Tree
              (stateful: tracks       (maintains: current table ptr,
               string-type context)    already-defined-paths set,
                                        dotted-key merge logic)
              tokens: [tbl] [[arr-tbl]] key = value  # comment  """ml"""

  Multi-concern pass. Must resolve dotted keys against prior table
  headers, enforce no-redefinition rules, distinguish 4 string kinds
  and 4 datetime kinds during lexing.
```

---

## 10. Schema Validation

### 10.1 JSON Schema

JSON has a mature, IETF-adjacent standard: **JSON Schema** (draft 2020-12 is current). It is itself written in JSON and can validate types, required fields, string patterns (regex), numeric ranges, enum constraints, conditional schemas (`if`/`then`/`else`), and cross-references (`$ref`).

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name", "port"],
  "properties": {
    "name": { "type": "string", "minLength": 1 },
    "port": { "type": "integer", "minimum": 1, "maximum": 65535 }
  }
}
```

Validator libraries exist in every major language (e.g., `jsonschema` in Python, `ajv` in JS, `jsonschema` crate in Rust).

### 10.2 TOML Validation

TOML has **no equivalent official schema standard**. The community leans on two alternate strategies instead:

1. **Deserialize straight into a strongly-typed struct** (Rust `struct` via `serde`, Go `struct` via tags) and let the *language's type system* be the schema. This is by far the dominant pattern in both ecosystems (see §12–13) — it means "validation" and "parsing" collapse into a single step.
2. **Convert TOML → JSON in-memory, then validate with JSON Schema** — some tools do this as a bridge, since TOML's data model maps losslessly onto JSON's *except* for the native date types and the int/float distinction (both of which JSON Schema can still express via `"format": "date-time"` and separate `"type": "integer"` vs `"type": "number"`).

---

## 11. Architecture: Where Each Format Lives in a System

```
TYPICAL BACKEND SERVICE — WHERE EACH FORMAT APPEARS
=====================================================

                         ┌─────────────────────────┐
                         │   config.toml            │   <- hand-edited by
                         │   (human-authored,        │      a human,
                         │    committed to git)      │      read ONCE at
                         └────────────┬─────────────┘      process start
                                      │
                                      │  parsed at startup
                                      ▼
                         ┌─────────────────────────┐
                         │   Config struct           │
                         │   (typed, in-memory)       │
                         └────────────┬─────────────┘
                                      │
                                      ▼
   ┌────────────┐   HTTP    ┌─────────────────────┐   SQL    ┌──────────┐
   │  Client A   │─────────▶│                       │─────────▶│ Database │
   │ (browser/   │  JSON     │   Your Service         │          └──────────┘
   │  mobile app)│◀─────────│   (Rust or Go binary)  │
   └────────────┘   JSON    │                       │
                              └──────────┬────────────┘
                                         │
                                         │  JSON over gRPC-gateway / REST
                                         ▼
                              ┌─────────────────────┐
                              │  Downstream Service B │
                              └─────────────────────┘

   config.toml  -> read ONCE, by the SERVICE ITSELF, at boot.
                   Optimized for a human maintaining it in a repo.

   *.json       -> exchanged CONTINUOUSLY, between machines, over
                   the wire, thousands of times per second.
                   Optimized for unambiguous machine parsing +
                   compactness on the wire.
```

This is the pattern you'll see in almost every real system: **TOML at the edges where a human touches the system (config, package manifests), JSON in the middle where machines talk to machines (APIs, message queues, RPC payloads).** Real examples:

- **Rust's Cargo** uses `Cargo.toml` for the human-edited manifest, but `Cargo.lock` — which is machine-generated and machine-consumed almost exclusively — is *also* TOML by convention (a case where TOML is used slightly outside its "for humans" sweet spot, largely for consistency with `Cargo.toml`).
- **Python's `pyproject.toml`** (PEP 518/621) — human-authored build configuration.
- **GitHub Actions**, by contrast, uses YAML, not TOML — showing this is convention, not universal law.
- Virtually **every public HTTP API** (Stripe, GitHub, Twitter/X, etc.) uses JSON for request/response bodies, never TOML — because the consumer is code, not a human editing the payload by hand.

---

## 12. Rust Implementation

Rust's ecosystem treats both formats through the same lens: **`serde`** (SERialize/DEserialize), a trait-based framework that separates "how to walk a Rust struct" from "how to encode/decode a specific format." You define your data shape once with `#[derive(Serialize, Deserialize)]`, and swap the backend crate to get JSON, TOML, YAML, etc., for free.

### 12.1 Cargo.toml Dependencies

```toml
[package]
name = "config-demo"
version = "0.1.0"
edition = "2021"

[dependencies]
serde = { version = "1", features = ["derive"] }
serde_json = "1"
toml = "0.8"
chrono = { version = "0.4", features = ["serde"] }
```

### 12.2 Defining a Shared Struct

```rust
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ServerConfig {
    pub host: String,
    pub port: u16,
    #[serde(default)]
    pub tls: TlsConfig,
}

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct TlsConfig {
    pub enabled: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cert_path: Option<String>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct AppConfig {
    pub name: String,
    pub server: ServerConfig,
    #[serde(default)]
    pub tags: Vec<String>,
}
```

### 12.3 Parsing TOML — Real Startup Config Loader

```rust
use std::fs;
use anyhow::{Context, Result};

fn load_config(path: &str) -> Result<AppConfig> {
    let raw = fs::read_to_string(path)
        .with_context(|| format!("failed to read config file: {path}"))?;

    let config: AppConfig = toml::from_str(&raw)
        .with_context(|| format!("failed to parse TOML in: {path}"))?;

    Ok(config)
}

fn main() -> Result<()> {
    let config = load_config("config.toml")?;
    println!("Starting {} on {}:{}",
        config.name, config.server.host, config.server.port);

    // Serialize back out, e.g. to write a normalized/default config
    let rendered = toml::to_string_pretty(&config)?;
    println!("---\n{rendered}");

    Ok(())
}
```

Matching `config.toml`:

```toml
name = "payments-service"
tags = ["prod", "eu-west"]

[server]
host = "0.0.0.0"
port = 8443

[server.tls]
enabled = true
cert_path = "/etc/ssl/certs/payments.pem"
```

### 12.4 Handling JSON — Real API Request/Response

```rust
use serde::{Deserialize, Serialize};
use serde_json::json;

#[derive(Debug, Deserialize)]
struct CreateOrderRequest {
    customer_id: String,
    items: Vec<OrderItem>,
    #[serde(default)]
    coupon_code: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
struct OrderItem {
    sku: String,
    quantity: u32,
    unit_price_cents: u64,
}

#[derive(Debug, Serialize)]
struct CreateOrderResponse {
    order_id: String,
    total_cents: u64,
    status: String,
}

fn handle_create_order(body: &str) -> Result<String, serde_json::Error> {
    // Deserialize incoming JSON body
    let req: CreateOrderRequest = serde_json::from_str(body)?;

    let total: u64 = req.items.iter()
        .map(|i| i.unit_price_cents * i.quantity as u64)
        .sum();

    let resp = CreateOrderResponse {
        order_id: "ord_9f3ac1".to_string(),
        total_cents: total,
        status: "confirmed".to_string(),
    };

    // Serialize outgoing JSON response
    serde_json::to_string(&resp)
}

fn main() {
    let incoming_body = r#"{
        "customer_id": "cus_123",
        "items": [
            {"sku": "WIDGET-1", "quantity": 2, "unit_price_cents": 500},
            {"sku": "GADGET-3", "quantity": 1, "unit_price_cents": 1200}
        ]
    }"#;

    match handle_create_order(incoming_body) {
        Ok(json_out) => println!("{json_out}"),
        Err(e) => eprintln!("bad request: {e}"),
    }
}
```

### 12.5 Working with `serde_json::Value` — Dynamic/Untyped JSON

When you don't know the shape ahead of time (e.g., proxying arbitrary webhook payloads):

```rust
use serde_json::Value;

fn extract_event_type(raw: &str) -> Option<String> {
    let v: Value = serde_json::from_str(raw).ok()?;
    v.get("event")?
     .get("type")?
     .as_str()
     .map(|s| s.to_string())
}

fn build_dynamic_payload() -> Value {
    serde_json::json!({
        "event": {
            "type": "order.created",
            "timestamp": chrono::Utc::now().to_rfc3339(),
        },
        "data": {
            "order_id": "ord_9f3ac1",
            "amount": 2200,
        }
    })
}
```

### 12.6 Native Dates in TOML with `chrono`

```rust
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize, Serialize)]
struct Deployment {
    version: String,
    deployed_at: DateTime<Utc>,   // maps directly to TOML's offset-date-time
}

// config.toml:
// version = "2.4.1"
// deployed_at = 2026-08-21T09:15:00Z
```

This is a case where `toml::from_str` gives you a real `DateTime<Utc>` with zero manual parsing — the equivalent JSON field would deserialize as a `String` unless you added a custom `serde` deserializer to parse the ISO 8601 text yourself.

### 12.7 Preserving Comments — `toml_edit`

Standard `toml`/`serde` round-trips *data*, not *formatting* — if you parse and re-serialize, comments and key ordering are lost. When you need to programmatically edit a TOML file while preserving a human's comments and layout (e.g., a `cargo add`-style tool), use `toml_edit`:

```rust
use toml_edit::{DocumentMut, value};

fn bump_port(raw: &str, new_port: i64) -> Result<String, toml_edit::TomlError> {
    let mut doc = raw.parse::<DocumentMut>()?;
    doc["server"]["port"] = value(new_port);
    Ok(doc.to_string())   // comments, spacing, and key order all preserved
}
```

---

## 13. Go Implementation

Go's standard library ships first-class JSON support (`encoding/json`) but has **no TOML support in the standard library** — you must pull in a third-party module. The two dominant choices are `BurntSushi/toml` (the long-time incumbent) and `pelletier/go-toml/v2` (newer, faster, actively maintained, TOML v1.0.0 compliant). Examples below use `pelletier/go-toml/v2`.

### 13.1 go.mod Dependencies

```
module config-demo

go 1.22

require (
    github.com/pelletier/go-toml/v2 v2.2.2
)
```

### 13.2 Defining Shared Structs with Tags

```go
package config

import "time"

type ServerConfig struct {
    Host string `json:"host" toml:"host"`
    Port int    `json:"port" toml:"port"`
    TLS  TLSConfig `json:"tls" toml:"tls"`
}

type TLSConfig struct {
    Enabled  bool   `json:"enabled" toml:"enabled"`
    CertPath string `json:"cert_path,omitempty" toml:"cert_path,omitempty"`
}

type AppConfig struct {
    Name       string       `json:"name" toml:"name"`
    Server     ServerConfig `json:"server" toml:"server"`
    Tags       []string     `json:"tags" toml:"tags"`
    DeployedAt time.Time    `json:"deployed_at" toml:"deployed_at"`
}
```

Note the dual struct tags — Go structs can carry *both* `json:` and `toml:` tags on the same field, so one struct type can be the deserialization target for either format without duplication.

### 13.3 Parsing TOML — Real Startup Config Loader

```go
package main

import (
    "fmt"
    "os"

    "config-demo/config"
    "github.com/pelletier/go-toml/v2"
)

func loadConfig(path string) (*config.AppConfig, error) {
    raw, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config file %s: %w", path, err)
    }

    var cfg config.AppConfig
    if err := toml.Unmarshal(raw, &cfg); err != nil {
        return nil, fmt.Errorf("parse TOML in %s: %w", path, err)
    }

    return &cfg, nil
}

func main() {
    cfg, err := loadConfig("config.toml")
    if err != nil {
        fmt.Fprintln(os.Stderr, "fatal:", err)
        os.Exit(1)
    }

    fmt.Printf("Starting %s on %s:%d\n", cfg.Name, cfg.Server.Host, cfg.Server.Port)

    // Serialize back out, e.g. to write a normalized/default config
    out, err := toml.Marshal(cfg)
    if err != nil {
        fmt.Fprintln(os.Stderr, "marshal error:", err)
        os.Exit(1)
    }
    fmt.Println(string(out))
}
```

### 13.4 Handling JSON — Real HTTP Handler (net/http, stdlib only)

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type CreateOrderRequest struct {
    CustomerID string      `json:"customer_id"`
    Items      []OrderItem `json:"items"`
    CouponCode *string     `json:"coupon_code,omitempty"`
}

type OrderItem struct {
    SKU            string `json:"sku"`
    Quantity       uint32 `json:"quantity"`
    UnitPriceCents uint64 `json:"unit_price_cents"`
}

type CreateOrderResponse struct {
    OrderID    string `json:"order_id"`
    TotalCents uint64 `json:"total_cents"`
    Status     string `json:"status"`
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
    var req CreateOrderRequest

    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields() // reject payloads with unexpected fields
    if err := decoder.Decode(&req); err != nil {
        http.Error(w, fmt.Sprintf("bad request: %v", err), http.StatusBadRequest)
        return
    }

    var total uint64
    for _, item := range req.Items {
        total += item.UnitPriceCents * uint64(item.Quantity)
    }

    resp := CreateOrderResponse{
        OrderID:    "ord_9f3ac1",
        TotalCents: total,
        Status:     "confirmed",
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    if err := json.NewEncoder(w).Encode(resp); err != nil {
        // Header already sent; log server-side, cannot change status now.
        fmt.Println("encode error:", err)
    }
}

func main() {
    http.HandleFunc("/orders", createOrderHandler)
    http.ListenAndServe(":8080", nil)
}
```

### 13.5 Working with `map[string]any` — Dynamic/Untyped JSON

```go
package main

import "encoding/json"

func extractEventType(raw []byte) (string, bool) {
    var v map[string]any
    if err := json.Unmarshal(raw, &v); err != nil {
        return "", false
    }
    event, ok := v["event"].(map[string]any)
    if !ok {
        return "", false
    }
    typ, ok := event["type"].(string)
    return typ, ok
}
```

Go's untyped JSON decoding lands in `map[string]any`, where numbers become `float64` by default — a common footgun for anyone expecting integers to stay integers (see §17.4 too). Use `json.Decoder.UseNumber()` to get `json.Number` instead, which preserves the original textual representation.

### 13.6 Native Dates in TOML

`pelletier/go-toml/v2` maps TOML's `offset-date-time` directly onto Go's `time.Time`:

```go
type Deployment struct {
    Version    string    `toml:"version"`
    DeployedAt time.Time `toml:"deployed_at"`
}

// config.toml:
// version = "2.4.1"
// deployed_at = 2026-08-21T09:15:00Z
```

Exactly like the Rust/`chrono` case — no manual string parsing needed, the parser hands you a real `time.Time`.

### 13.7 Struct Tags vs. JSON Struct Tags — Practical Note

Go's `encoding/json` tag options you'll use constantly in production:

```go
type User struct {
    ID        int64      `json:"id"`
    Email     string     `json:"email"`
    Password  string     `json:"-"`                    // never serialize
    Bio       string     `json:"bio,omitempty"`         // omit if zero value
    CreatedAt time.Time  `json:"created_at"`
    Internal  bool       `json:"-,"`                    // literal key "-", still serialized
}
```

---

## 14. Error Handling Patterns

### 14.1 Rust — `Result`-Based, Composable Errors

```rust
use thiserror::Error;

#[derive(Debug, Error)]
enum ConfigError {
    #[error("could not read config file: {0}")]
    Io(#[from] std::io::Error),

    #[error("invalid TOML syntax: {0}")]
    TomlParse(#[from] toml::de::Error),

    #[error("invalid JSON syntax: {0}")]
    JsonParse(#[from] serde_json::Error),
}

fn load_toml_config(path: &str) -> Result<AppConfig, ConfigError> {
    let raw = std::fs::read_to_string(path)?;   // io::Error auto-converts via `?`
    let cfg = toml::from_str(&raw)?;             // toml::de::Error auto-converts
    Ok(cfg)
}
```

`toml::de::Error` and `serde_json::Error` both carry **line and column information**, so error messages like `"invalid TOML syntax: expected newline, found '=' at line 4 column 12"` are automatic — you don't build this yourself.

### 14.2 Go — Explicit Multi-Value Returns

```go
func loadTomlConfig(path string) (*AppConfig, error) {
    raw, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("reading %s: %w", path, err)
    }

    var cfg AppConfig
    if err := toml.Unmarshal(raw, &cfg); err != nil {
        // pelletier/go-toml/v2 errors implement toml.DecodeError,
        // which exposes Position() for line/column info.
        var decodeErr *toml.DecodeError
        if errors.As(err, &decodeErr) {
            row, col := decodeErr.Position()
            return nil, fmt.Errorf("TOML syntax error at %d:%d: %w", row, col, err)
        }
        return nil, fmt.Errorf("parsing %s: %w", path, err)
    }

    return &cfg, nil
}
```

Both ecosystems converge on the same underlying idea — **rich, structured error types rather than bare strings** — but Rust surfaces it through `enum` + `?`-operator propagation, while Go surfaces it through `error` wrapping (`%w`) and `errors.As` for type-switching on the underlying cause.

---

## 15. Round-Tripping and Comment Preservation

A subtlety that matters once you go from "read config" to "write config programmatically":

| | JSON | TOML |
|---|---|---|
| Standard parse→serialize round-trip | Preserves data, **not** key order (unless using an ordered map) or whitespace | Preserves data, **not** comments or formatting, with plain `serde`/`encoding/json` |
| Comment-/format-preserving edit | Not applicable (no comments exist) | Requires a special "editing" library: `toml_edit` (Rust) or manual AST-level editing in Go |
| Practical implication | Round-tripping JSON is "free" — there's nothing to lose except key order | Round-tripping TOML with a plain struct-based parser **destroys comments** — dangerous if you programmatically rewrite a human-maintained file |

**Real-world consequence**: if you write a tool that reads `Cargo.toml`, changes one dependency version, and writes it back out — using plain `toml::from_str` / `toml::to_string` will silently delete every comment and reorder every key. This is exactly why `cargo add`/`cargo upgrade`-style tools use `toml_edit` instead: it parses into a format-preserving document tree, so only the touched value changes and everything else — comments, blank lines, key order — survives untouched.

---

## 16. Performance Characteristics

- **Parsing speed**: JSON parsers are generally faster than TOML parsers for equivalent data volume, because JSON's grammar is simpler (§9.1) and JSON parsing has received two decades of extreme optimization (SIMD-accelerated parsers like `simd-json` in Rust, `sonic` in Go/C, etc.). TOML parsing is fast enough that it is a complete non-issue for its intended use case (parsed once, at process startup) but you would not choose TOML for a hot-path, parse-millions-of-times-per-second workload.
- **Serialized size**: JSON is generally more compact for machine-generated payloads (no comments, minimal whitespace by default). TOML documents tend to be larger on disk because of the comments and formatting they're designed to carry — but this is irrelevant, since TOML files are read from local disk once at startup, not transmitted over a network repeatedly.
- **Memory**: both formats, when deserialized into typed structs (`#[derive(Deserialize)]` in Rust, tagged structs in Go), have essentially identical in-memory footprints — the *format* the data arrived in doesn't matter once it's a native struct; only the *shape* of the struct matters.

**Bottom line**: performance is essentially never the deciding factor between TOML and JSON — the deciding factor is always §1.3's "who is the primary author of this file, a human or a machine?"

---

## 17. Security Considerations

### 17.1 JSON

- **Prototype pollution** (JS-specific): a JSON payload containing `"__proto__"` as a key can, in poorly-guarded JavaScript deserializers, mutate `Object.prototype` globally. This is a JS-runtime issue, not a JSON-format issue, and does not affect Rust/Go deserializers, which have no prototype chain concept.
- **Unbounded nesting / stack exhaustion**: a maliciously deep JSON payload like `[[[[[[[[...]]]]]]]]` (millions of levels) can cause a naive recursive-descent parser to stack-overflow. Production parsers (`serde_json`, Go's `encoding/json`) impose depth or size limits; you should still enforce a max request-body size at the HTTP layer regardless.
- **Duplicate keys**: `{"role": "user", "role": "admin"}` — the JSON spec does not define which value "wins" on duplicate keys; different parsers pick differently (usually last-wins, but not guaranteed). This has caused real security bugs where a WAF/validator reads the *first* "role" while the backend deserializer reads the *last* — an inconsistency an attacker can exploit. **Mitigation**: reject duplicate keys explicitly if your parser supports strict mode, or ensure every service in a pipeline uses the same parser/library.

### 17.2 TOML

- **Redefinition ambiguity is actually stricter than JSON here** — TOML v1.0.0 explicitly makes it a **parse error** to define the same key/table path twice (§7), which closes the "duplicate key" class of bug JSON is vulnerable to.
- **Untrusted TOML input is rare** in practice (config files are typically operator-controlled, not attacker-controlled), so TOML's threat model is generally lower-stakes than JSON's — but if you ever do parse TOML from an untrusted source, apply the same discipline: size limits, and use a well-maintained, spec-compliant parser (`toml`/`toml_edit` in Rust; `pelletier/go-toml/v2` in Go) rather than a hand-rolled one.

### 17.3 General Practice for Both

- Always set a **maximum request/file size** before parsing, regardless of format.
- Use `DisallowUnknownFields()` (Go) / `#[serde(deny_unknown_fields)]` (Rust) on any struct that receives **untrusted** input, so unexpected fields are rejected rather than silently dropped — this prevents a whole class of "attacker smuggles an extra field the validator doesn't check but a downstream system does" bugs.
- Never deserialize untrusted data directly into a type that also controls authorization (e.g., don't let a JSON body set `"is_admin": true` unless every field is explicitly allow-listed).

### 17.4 The Integer-Precision Footgun (JSON-specific, both languages)

JSON's `number` type has no defined precision limit in the spec, but most JSON consumers (JavaScript, and by convention many other languages) parse numbers as IEEE 754 double-precision floats, which only exactly represent integers up to 2^53. A JSON payload with a 64-bit ID like `9223372036854775807` can silently lose precision when round-tripped through a naive parser.

```rust
// Rust: serde_json preserves i64/u64 precision correctly IF your struct
// field type is i64/u64 (not f64) — because serde_json's number parsing
// is type-directed by your target struct, not float-by-default.
#[derive(Deserialize)]
struct Event { id: u64 }  // safe: exact 64-bit integer parsing
```

```go
// Go: encoding/json into map[string]any DOES default to float64 and
// WILL lose precision on large integers. Use a typed struct field
// (int64) or json.Number to avoid this.
type Event struct {
    ID int64 `json:"id"` // safe
}
var m map[string]any
json.Unmarshal(data, &m)
m["id"] // UNSAFE if the source ID exceeds 2^53 — comes back as float64
```

TOML sidesteps this entirely — integers are a distinct grammar-level type from floats, so there's no "was this meant to be a float" ambiguity for a spec-compliant TOML parser to get wrong.

---

## 18. Ecosystem and Real-World Adoption

| Tool / Project | Format | Why |
|---|---|---|
| Cargo (Rust) | TOML | Human-edited package manifest |
| pip / Poetry / Hatch (Python, `pyproject.toml`) | TOML | Standardized by PEP 518/621 as the human-edited build config |
| Every public REST API (Stripe, GitHub, etc.) | JSON | Machine-to-machine, language-agnostic wire format |
| `package.json` (npm) | JSON | Predates TOML; JS ecosystem default |
| Kubernetes manifests | YAML (not covered here) | Chosen for comments + more compact nesting than JSON, predates TOML's popularity |
| GitHub Actions workflows | YAML | Same reasoning as Kubernetes |
| Hugo (static site generator) front-matter | TOML, YAML, or JSON (user's choice) | Illustrates that TOML competes directly with YAML for "human-edited config," not with JSON |
| gRPC / protobuf-JSON mapping | JSON | Canonical JSON encoding defined for protobuf messages, used at HTTP/JSON gateway boundaries |
| Terraform / HCL | Neither (HCL, a third format) | Worth knowing this exists as a third "human-config" contender alongside TOML and YAML |

The consistent pattern: **TOML directly competes with YAML** (both target "human writes this by hand"), while **JSON has essentially no competition** for "machine talks to machine over a wire" — that space stayed JSON's because of universal parser availability and its origin as literally native JavaScript syntax for the dominant client platform (the browser).

---

## 19. Migration and Conversion

### 19.1 Lossless Direction: TOML → JSON

Because TOML's data model is a strict superset-with-caveats of JSON's (extra types: dates, distinct int/float), converting TOML → JSON is always possible, with two type losses to be aware of:
- TOML dates become plain JSON strings (the "this is a date" type information is lost; downstream consumers must know to re-parse it).
- TOML's int/float distinction collapses into JSON's single `number` type (usually harmless, since JSON consumers infer intent from context anyway).

### 19.2 Lossy-ish Direction: JSON → TOML

Converting JSON → TOML is possible but has its own snag: **JSON's `null`** has no TOML equivalent (§2.2). A converter must choose a strategy:
- Drop the key entirely (closest semantic match to TOML's "omit if absent" philosophy), or
- Represent it as an empty string / sentinel (lossy and generally discouraged).

### 19.3 Rust Conversion Example

```rust
fn toml_to_json(toml_str: &str) -> Result<String, Box<dyn std::error::Error>> {
    let value: toml::Value = toml::from_str(toml_str)?;
    let json_value: serde_json::Value = serde_json::to_value(&value)?;
    Ok(serde_json::to_string_pretty(&json_value)?)
}
```

Because both `toml::Value` and `serde_json::Value` implement `serde::Serialize`/`Deserialize`, `serde_json::to_value()` on a `toml::Value` "just works" — this is the payoff of both crates sharing the `serde` data model.

### 19.4 Go Conversion Example

```go
func tomlToJSON(tomlStr string) (string, error) {
    var generic map[string]any
    if err := toml.Unmarshal([]byte(tomlStr), &generic); err != nil {
        return "", fmt.Errorf("toml decode: %w", err)
    }
    out, err := json.MarshalIndent(generic, "", "  ")
    if err != nil {
        return "", fmt.Errorf("json encode: %w", err)
    }
    return string(out), nil
}
```

Go's version routes through the generic `map[string]any` bridge rather than a shared `serde`-style trait system, since Go has no equivalent unifying serialization framework — this is a genuine ecosystem-level difference worth internalizing: **Rust's `serde` gives you one mental model across every format; Go gives you a separate, standalone package per format (`encoding/json`, `pelletier/go-toml`), unified only by the shared convention of struct tags.**

---

## 20. Decision Framework: Which to Use

```
                     ┌─────────────────────────────┐
                     │  Will a HUMAN routinely open  │
                     │  and hand-edit this file?     │
                     └───────────┬───────────────────┘
                          yes    │    no
                  ┌──────────────┴──────────────┐
                  ▼                              ▼
        ┌───────────────────┐        ┌─────────────────────────┐
        │   Choose TOML       │        │  Is this exchanged over  │
        │   (comments, clean   │        │  a network between       │
        │   nesting, no null   │        │  independently-deployed  │
        │   ambiguity)         │        │  services?                │
        └───────────────────┘        └───────────┬───────────────┘
                                            yes    │    no
                                    ┌───────────────┴───────────┐
                                    ▼                            ▼
                          ┌───────────────────┐      ┌────────────────────┐
                          │   Choose JSON        │      │  Either works —      │
                          │   (universal parser   │      │  prefer whichever    │
                          │   support, unambiguous │      │  your other tooling  │
                          │   grammar, streaming-   │      │  in this codebase     │
                          │   friendly)             │      │  already uses         │
                          └───────────────────┘      └────────────────────┘
```

**Concrete rules of thumb:**
- Writing a CLI tool's config file that a user edits by hand → **TOML**.
- Defining a REST/gRPC-gateway API request or response body → **JSON**.
- Package manifest / build tool configuration → **TOML** (unless the ecosystem convention is otherwise, e.g. `package.json`).
- Log lines, message-queue payloads, cache-serialized blobs → **JSON** (machine-only lifecycle end-to-end).
- A file that will never be read by a human and never leaves one process's disk (e.g., an internal cache/lockfile) → either is fine; JSON is usually marginally simpler to reach for since no extra crate/module is needed in Go, and parsing is faster.

---

## 21. Quick Reference Summary

```
┌──────────────────────┬───────────────────────────┬───────────────────────────┐
│ Property               │ JSON                       │ TOML                       │
├──────────────────────┼───────────────────────────┼───────────────────────────┤
│ Designed for           │ Machine data interchange   │ Human-edited config        │
│ Standardized as        │ RFC 8259 / ECMA-404         │ toml.io spec (v1.0.0)      │
│ Comments                │ ✗ none                     │ ✓ # to end of line          │
│ Trailing commas         │ ✗ invalid                  │ ✓ allowed in arrays/tables  │
│ Distinct int vs float   │ ✗ single `number` type      │ ✓ integer & float distinct  │
│ Native date/time        │ ✗ (string convention)       │ ✓ 4 native types            │
│ Null / absence          │ ✓ `null` literal            │ ✗ omit the key instead      │
│ Multi-line raw strings  │ ✗                           │ ✓ `'''...'''`               │
│ Duplicate-key handling  │ Undefined (parser choice)   │ Parse error (spec-mandated) │
│ Root value               │ any JSON value              │ always an implicit table    │
│ Schema standard          │ JSON Schema (mature)         │ none official               │
│ Rust crate                │ serde_json                  │ toml / toml_edit             │
│ Go module                 │ encoding/json (stdlib)       │ pelletier/go-toml/v2 (3p)    │
│ Typical file examples     │ package.json, API bodies     │ Cargo.toml, pyproject.toml   │
└──────────────────────┴───────────────────────────┴───────────────────────────┘
```

**The one idea to keep**: JSON and TOML are not really competing for the same job. JSON won the "machines talking to machines" job because of its grammar's radical simplicity and its native-JavaScript origin. TOML exists specifically to *not* be JSON for the "a person is going to open this file in `vim` and change a port number" job — comments, no-null, native dates, and forgiving trailing commas are all features that only make sense once you accept a human is the primary author. Once you internalize which job a given file is doing, the choice of format stops being a debate.

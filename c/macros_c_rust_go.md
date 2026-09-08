# Macros in C, Rust, and Go: A Complete In-Depth Guide

---

## Table of Contents

1. [What Are Macros and Why Do They Exist](#1-what-are-macros-and-why-do-they-exist)
2. [Mental Model: Compilation Stages and Where Macros Live](#2-mental-model-compilation-stages-and-where-macros-live)
3. [C Macros — The Preprocessor](#3-c-macros--the-preprocessor)
   - 3.1 [Preprocessing Pipeline](#31-preprocessing-pipeline)
   - 3.2 [Object-Like Macros](#32-object-like-macros)
   - 3.3 [Function-Like Macros](#33-function-like-macros)
   - 3.4 [Stringification (`#`)](#34-stringification-)
   - 3.5 [Token Pasting (`##`)](#35-token-pasting-)
   - 3.6 [Variadic Macros (`__VA_ARGS__`)](#36-variadic-macros-__va_args__)
   - 3.7 [Predefined Macros](#37-predefined-macros)
   - 3.8 [Conditional Compilation](#38-conditional-compilation)
   - 3.9 [Include Guards and `#pragma once`](#39-include-guards-and-pragma-once)
   - 3.10 [X-Macros Pattern](#310-x-macros-pattern)
   - 3.11 [Macro Hygiene Problems and Solutions](#311-macro-hygiene-problems-and-solutions)
   - 3.12 [Recursive Macros and Rescanning](#312-recursive-macros-and-rescanning)
   - 3.13 [Pitfalls, Edge Cases, Best Practices](#313-pitfalls-edge-cases-best-practices)
4. [Rust Macros — Hygienic, Typed Metaprogramming](#4-rust-macros--hygienic-typed-metaprogramming)
   - 4.1 [Macro System Overview and Phases](#41-macro-system-overview-and-phases)
   - 4.2 [Declarative Macros: `macro_rules!`](#42-declarative-macros-macro_rules)
   - 4.3 [Pattern Matching and Fragment Specifiers](#43-pattern-matching-and-fragment-specifiers)
   - 4.4 [Repetition in `macro_rules!`](#44-repetition-in-macro_rules)
   - 4.5 [Hygiene in Rust Macros](#45-hygiene-in-rust-macros)
   - 4.6 [Procedural Macros — Architecture](#46-procedural-macros--architecture)
   - 4.7 [Custom Derive Macros](#47-custom-derive-macros)
   - 4.8 [Attribute Macros](#48-attribute-macros)
   - 4.9 [Function-Like Procedural Macros](#49-function-like-procedural-macros)
   - 4.10 [`TokenStream`, `syn`, and `quote`](#410-tokenstream-syn-and-quote)
   - 4.11 [Macro Export, Import, and Scoping Rules](#411-macro-export-import-and-scoping-rules)
   - 4.12 [Macro Debugging Techniques](#412-macro-debugging-techniques)
5. [Go — No Macros, But Metaprogramming Exists](#5-go--no-macros-but-metaprogramming-exists)
   - 5.1 [Why Go Has No Macros](#51-why-go-has-no-macros)
   - 5.2 [`go:generate` — The Code Generation Gateway](#52-gogenerate--the-code-generation-gateway)
   - 5.3 [Build Tags and Conditional Compilation](#53-build-tags-and-conditional-compilation)
   - 5.4 [`text/template` and `go/ast` for Code Generation](#54-texttemplate-and-goast-for-code-generation)
   - 5.5 [Writing a Real Code Generator](#55-writing-a-real-code-generator)
   - 5.6 [Stringer, Mockgen, and Protocol Buffers](#56-stringer-mockgen-and-protocol-buffers)
   - 5.7 [Generics as a Macro Replacement](#57-generics-as-a-macro-replacement)
6. [Cross-Language Comparison and Mental Models](#6-cross-language-comparison-and-mental-models)
7. [Advanced Patterns Across All Three Languages](#7-advanced-patterns-across-all-three-languages)

---

## 1. What Are Macros and Why Do They Exist

A **macro** is a rule or pattern that specifies how a piece of code should be transformed into another piece of code before or during compilation. The transformation happens at **compile time**, not at runtime.

### The Core Problem Macros Solve

Programming has tasks that are structurally repetitive but not expressible as ordinary functions:

- **Code generation at compile time**: Generating boilerplate that varies only in types or names.
- **Compile-time computation**: Resolving constants, sizes, or configurations during compilation rather than at runtime.
- **Syntax extension**: Adding new syntax that the language itself doesn't provide (e.g., `assert!`, `println!` in Rust).
- **Conditional compilation**: Including or excluding code based on compile-time flags (target OS, debug mode, feature flags).
- **Zero-cost abstraction**: Achieving the same runtime performance as hand-written code while having a high-level authoring experience.

### A Taxonomy of Macro Systems

```
Macro Systems
├── Text/Token Substitution (C Preprocessor)
│   └── Operates on raw text before parsing
│       No type info, no AST, pure string replacement
│
├── Syntactic Macros (Rust macro_rules!)
│   └── Operates on token trees after tokenization
│       Pattern-matched, hygienic, type-safe fragments
│
├── Procedural/AST Macros (Rust proc macros)
│   └── Operates on the AST as a data structure
│       Full Rust code to inspect and transform tokens
│
└── Code Generation (Go go:generate)
    └── External programs invoked during build
        Operates on source files, generates new source files
```

---

## 2. Mental Model: Compilation Stages and Where Macros Live

Understanding where macros operate in the compilation pipeline is the single most important mental model.

### C Compilation Pipeline

```
  Source File (.c)
        |
        v
  +------------------+
  |   Preprocessor   |  <-- Macros live HERE
  |  (cpp / cc1)     |      Text substitution on raw tokens
  +------------------+
        |
        v  (translation unit: pure C, no macros)
  +------------------+
  |    Lexer/Parser  |  Tokenizes and builds AST
  +------------------+
        |
        v
  +------------------+
  |  Semantic Anal.  |  Type checking, symbol resolution
  +------------------+
        |
        v
  +------------------+
  |  IR Generation   |  LLVM IR / GCC GIMPLE
  +------------------+
        |
        v
  +------------------+
  |  Optimization    |
  +------------------+
        |
        v
  +------------------+
  |  Code Generation |  Assembly / Object file
  +------------------+
```

**Key insight**: The C preprocessor sees NO types, NO AST, NO symbol table. It's a glorified `sed`. Every macro expansion happens purely in text space.

### Rust Compilation Pipeline

```
  Source File (.rs)
        |
        v
  +------------------+
  |     Lexer        |  Produces Token Stream
  +------------------+
        |
        v  (token trees)
  +------------------+
  | Macro Expansion  |  <-- macro_rules! and proc macros live HERE
  |  (recursive)     |      Works on token trees, NOT raw text
  +------------------+
        |
        v  (fully expanded token stream)
  +------------------+
  |     Parser       |  Builds Concrete Syntax Tree (CST)
  +------------------+
        |
        v
  +------------------+
  |  HIR Lowering    |  High-level IR (desugaring)
  +------------------+
        |
        v
  +------------------+
  |  Type Inference  |  Hindley-Milner + extensions
  |  & Borrow Check  |
  +------------------+
        |
        v
  +------------------+
  |  MIR Lowering    |  Mid-level IR
  +------------------+
        |
        v
  +------------------+
  |  LLVM Backend    |
  +------------------+
```

**Key insight**: Rust macros operate on **token trees** — structured tokens with balanced delimiters. The macro expansion is recursive: macros can produce more macros. Hygiene is tracked in the token trees themselves via **syntax contexts** (span information).

### Go Compilation Pipeline

```
  Source File (.go)
        |
        v
  +------------------+
  |  go:generate     |  <-- "Macros" live HERE (external tools)
  |  (external cmd)  |      Runs BEFORE compilation
  +------------------+
        |
        v  (generates new .go files)
  +------------------+
  |     Lexer        |
  +------------------+
        |
        v
  +------------------+
  |     Parser       |  go/ast produces AST
  +------------------+
        |
        v
  +------------------+
  |  Type Checker    |  go/types
  +------------------+
        |
        v
  +------------------+
  |  SSA / IR        |  go/ssa
  +------------------+
        |
        v
  +------------------+
  |  Code Generation |  gc (Go compiler), gccgo, tinygo
  +------------------+
```

**Key insight**: Go has NO compile-time macro system. Metaprogramming is done through external code generators that are invoked by `go generate` and produce `.go` files that are then compiled normally. The `go/ast`, `go/types`, and `go/token` packages make this powerful.

---

## 3. C Macros — The Preprocessor

### 3.1 Preprocessing Pipeline

The C preprocessor (cpp) processes source files **before** the compiler proper sees them. It understands a small set of directives:

```
Directive Categories
├── File Inclusion     #include
├── Macro Definition   #define, #undef
├── Conditional Comp.  #if, #ifdef, #ifndef, #elif, #else, #endif
├── Line Control       #line
├── Error Generation   #error
├── Pragma             #pragma
└── Null Directive     # (bare hash)
```

The preprocessor operates on a **translation unit**: a stream of preprocessing tokens. These are NOT the same as C tokens — they're a simpler layer.

```
Preprocessing Token Types
├── Header names       <stdio.h>  "myfile.h"
├── Identifiers        foo, MY_MACRO, __FILE__
├── PP-numbers         0, 3.14, 0x1F, 1e-5
├── Character consts   'a', '\n'
├── String literals    "hello"
├── Punctuators        { } [ ] ( ) + - * / = ...
└── Other (whitespace, newlines are significant here)
```

### 3.2 Object-Like Macros

An **object-like macro** is a simple name-to-token-sequence substitution with no parameters.

```c
#define IDENTIFIER replacement-token-list
```

```c
/* Constants */
#define MAX_BUFFER_SIZE  4096
#define PI               3.14159265358979323846
#define NEWLINE          '\n'
#define NULL             ((void*)0)

/* Expression macros */
#define UINT32_MAX       0xFFFFFFFFU
#define CACHE_LINE_SIZE  64

/* Multi-token substitutions */
#define FOREVER          for(;;)
#define BEGIN            {
#define END              }

/* Usage */
char buf[MAX_BUFFER_SIZE];   /* expands to: char buf[4096]; */
double circumference = 2 * PI * r;  /* expands: 2 * 3.14159... * r */
```

**How substitution works internally:**

```
Source:   char buf[MAX_BUFFER_SIZE];
           |
           v  Preprocessor scans, finds 'MAX_BUFFER_SIZE' is a macro
           |  Replaces with its token list: 4096
           v
Output:   char buf[4096];
```

**Rescanning:** After substitution, the result is rescanned for further macro names. This is a fundamental rule.

```c
#define A B
#define B C
#define C 42

int x = A;    /* Step 1: A -> B, rescan -> C, rescan -> 42 */
              /* Result: int x = 42; */
```

### 3.3 Function-Like Macros

Function-like macros accept **parameters** and are invoked with argument lists.

```c
#define MACRO_NAME(param1, param2, ...) replacement-token-list
```

The parenthesis must immediately follow the macro name with **no whitespace**:

```c
#define FOO (x)   /* Object-like macro whose value is (x) */
#define BAR(x)    /* Function-like macro with parameter x */
```

**Basic examples:**

```c
#define SQUARE(x)       ((x) * (x))
#define MAX(a, b)       ((a) > (b) ? (a) : (b))
#define MIN(a, b)       ((a) < (b) ? (a) : (b))
#define ABS(x)          ((x) < 0 ? -(x) : (x))
#define SWAP(T, a, b)   do { T _tmp = (a); (a) = (b); (b) = _tmp; } while(0)
#define ARRAY_LEN(arr)  (sizeof(arr) / sizeof((arr)[0]))
#define STRINGIFY(x)    #x
#define OFFSET_OF(T, m) ((size_t)&((T*)0)->m)

/* Container of — fundamental in Linux kernel */
#define CONTAINER_OF(ptr, type, member) \
    ((type *)((char *)(ptr) - OFFSET_OF(type, member)))
```

**Argument substitution rules:**

1. Macro parameters in the replacement list are replaced by the corresponding argument.
2. Before substitution, each argument is **fully macro-expanded**, EXCEPT when adjacent to `#` or `##`.
3. After substitution, the result is rescanned for more macros.

```
SQUARE(3 + 4)
  |
  v  Parameter x = "3 + 4" (token sequence, not a value)
  v  Replacement: ((x) * (x))
  v  Substitute x: ((3 + 4) * (3 + 4))
  v  Rescan (no more macros)
Result: ((3 + 4) * (3 + 4))   --> evaluates to 49, correct

Without outer parens:
#define SQUARE_BAD(x)  x * x
SQUARE_BAD(3 + 4)  -->  3 + 4 * 3 + 4  -->  19 (WRONG)
```

**The `do { ... } while(0)` idiom:**

Any multi-statement macro MUST be wrapped in `do { ... } while(0)`:

```c
/* BAD: breaks with if/else */
#define LOG_ERROR(msg)  \
    fprintf(stderr, "Error: %s\n", msg); \
    error_count++;

if (condition)
    LOG_ERROR("something");
else
    do_something();
/* Expands to:
   if (condition)
       fprintf(stderr, "Error: %s\n", "something");
   error_count++;       <-- ALWAYS executes (outside the if)
   else                 <-- syntax error (else without if)
       do_something();
*/

/* GOOD: do-while wraps as single statement */
#define LOG_ERROR(msg)          \
    do {                        \
        fprintf(stderr, "Error: %s\n", (msg)); \
        error_count++;          \
    } while(0)

if (condition)
    LOG_ERROR("something");     /* Works correctly */
else
    do_something();
/* Expands to:
   if (condition)
       do { fprintf(...); error_count++; } while(0);
   else
       do_something();
   CORRECT.
*/
```

### 3.4 Stringification (`#`)

The `#` operator before a macro parameter converts the parameter tokens into a **string literal**.

```c
#define STRINGIFY(x)     #x
#define TOSTRING(x)      STRINGIFY(x)   /* needed for macro-expanded args */

STRINGIFY(hello world)     -->  "hello world"
STRINGIFY(3 + 4)           -->  "3 + 4"
STRINGIFY("quoted")        -->  "\"quoted\""

/* Indirection trick: */
#define VERSION 42
STRINGIFY(VERSION)         -->  "VERSION"      (x is not expanded before #)
TOSTRING(VERSION)          -->  "42"           (x is expanded first, then stringified)
```

**Why indirection is needed:**

When `#` appears before a parameter, the argument is NOT expanded before stringification. To stringify the *expansion* of a macro argument, you need an extra level of indirection:

```c
#define STR1(x)    #x          /* direct: does NOT expand x */
#define STR2(x)    STR1(x)     /* indirect: expands x, then stringifies */

#define FOO 123
STR1(FOO)  --> "FOO"
STR2(FOO)  --> "123"
```

**Practical application — compile-time assertion with message:**

```c
#define ASSERT_MSG(cond, msg)                          \
    do {                                               \
        if (!(cond)) {                                 \
            fprintf(stderr,                            \
                "Assertion failed: " #cond             \
                " at " __FILE__ ":" TOSTRING(__LINE__)  \
                " — " msg "\n");                       \
            abort();                                   \
        }                                              \
    } while(0)

ASSERT_MSG(x > 0, "x must be positive");
/* Expands to message: "Assertion failed: x > 0 at foo.c:42 — x must be positive" */
```

### 3.5 Token Pasting (`##`)

The `##` operator concatenates two tokens into one. Both adjacent tokens are joined into a single preprocessing token.

```c
#define CONCAT(a, b)    a ## b
#define PASTE3(a,b,c)   a ## b ## c

CONCAT(foo, bar)        -->  foobar
CONCAT(my_, function)   -->  my_function
CONCAT(x, 1)            -->  x1

/* Struct field access by name construction */
#define GETTER(type, field)                  \
    type get_ ## field(MyStruct *s) {        \
        return s->field;                     \
    }

GETTER(int, width)
/* Expands to:
   int get_width(MyStruct *s) {
       return s->width;
   }
*/
```

**Token pasting with numbers:**

```c
#define REG(n)   REGISTER_ ## n
REG(3)  -->  REGISTER_3
```

**Important rule:** The result of `##` must be a valid preprocessing token. Joining `foo` and `+bar` would be illegal because `foo+bar` as a single token is invalid. The preprocessor would emit a diagnostic.

**The indirection rule also applies to `##`:**

```c
#define XCAT(a, b)  a ## b
#define CAT(a, b)   XCAT(a, b)   /* expands args before pasting */

#define PRE prefix_
CAT(PRE, func)   -->  prefix_func    /* PRE expanded first */
XCAT(PRE, func)  -->  PRE_func  ... wait, actually:
/* XCAT(PRE, func): a=PRE, b=func, result = PRE ## func = PREfunc
   Weird — PRE is NOT expanded since ## suppresses expansion */
```

### 3.6 Variadic Macros (`__VA_ARGS__`)

C99 introduced variadic macros with `...` and `__VA_ARGS__`:

```c
#define debug_print(fmt, ...)   fprintf(stderr, fmt, __VA_ARGS__)
#define LOG(level, fmt, ...)    \
    fprintf(stderr, "[%s] " fmt "\n", level, __VA_ARGS__)
```

**Problem with zero variadic arguments:** In C99, `__VA_ARGS__` cannot be empty — the trailing comma causes an error.

```c
debug_print("no args")
/* Expands to: fprintf(stderr, "no args", )  <-- trailing comma: ERROR */
```

**Solutions:**

```c
/* GCC extension: ## before __VA_ARGS__ eats the comma if args are empty */
#define LOG(fmt, ...)   fprintf(stderr, fmt, ##__VA_ARGS__)

/* C23 / __VA_OPT__ (portable, standardized in C23) */
#define LOG(fmt, ...)   fprintf(stderr, fmt __VA_OPT__(,) __VA_ARGS__)

/* Explicit comma only if args present */
LOG("hello")               -->  fprintf(stderr, "hello")
LOG("val=%d", 42)          -->  fprintf(stderr, "val=%d", 42)
```

**Counting variadic arguments:**

```c
/* Classic trick — works for up to N args */
#define COUNT_ARGS(...)  COUNT_ARGS_(__VA_ARGS__, 5, 4, 3, 2, 1, 0)
#define COUNT_ARGS_(_1, _2, _3, _4, _5, n, ...) n

COUNT_ARGS(a, b, c)    -->  3
COUNT_ARGS(a)          -->  1
```

**Full logging macro with file/line:**

```c
#define LOG(level, fmt, ...)                                         \
    do {                                                             \
        fprintf(stderr, "[%s] %s:%d: " fmt "\n",                    \
                (level), __FILE__, __LINE__ __VA_OPT__(,) __VA_ARGS__); \
    } while(0)

#define DEBUG(fmt, ...)  LOG("DEBUG", fmt, ##__VA_ARGS__)
#define ERROR(fmt, ...)  LOG("ERROR", fmt, ##__VA_ARGS__)
#define FATAL(fmt, ...)  do { LOG("FATAL", fmt, ##__VA_ARGS__); abort(); } while(0)
```

### 3.7 Predefined Macros

The C standard mandates several predefined macros. Compilers add more.

```c
/* C Standard mandated */
__FILE__          /* String literal: current source file name */
__LINE__          /* Integer: current source line number */
__DATE__          /* String: compilation date "Mmm dd yyyy" */
__TIME__          /* String: compilation time "hh:mm:ss" */
__STDC__          /* 1 if conforming C implementation */
__STDC_VERSION__  /* Long: 199901L (C99), 201112L (C11), 201710L (C17) */

/* GCC/Clang extensions */
__FUNCTION__        /* Current function name (not standard, use __func__) */
__func__            /* C99: current function name, as string */
__PRETTY_FUNCTION__ /* GCC: function signature including parameters */
__COUNTER__         /* Unique integer, increments each use */
__INCLUDE_LEVEL__   /* Depth of #include nesting */
__BASE_FILE__       /* Top-level source file being compiled */

/* Compiler identification */
__GNUC__          /* GCC major version */
__clang__         /* Defined if clang */
__GNUC_MINOR__    /* GCC minor version */
__clang_major__   /* Clang major version */

/* Architecture */
__x86_64__        /* x86-64 target */
__aarch64__       /* ARM64 target */
__i386__          /* x86-32 target */
__arm__           /* ARM 32-bit target */

/* OS */
__linux__         /* Linux */
__APPLE__         /* macOS / iOS */
_WIN32            /* Windows (also defined on 64-bit) */
_WIN64            /* Windows 64-bit */
__FreeBSD__       /* FreeBSD */

/* Endianness (GCC/Clang) */
__BYTE_ORDER__              /* __ORDER_LITTLE_ENDIAN__ or __ORDER_BIG_ENDIAN__ */
__ORDER_LITTLE_ENDIAN__     /* 1234 */
__ORDER_BIG_ENDIAN__        /* 4321 */
```

**Usage in production code:**

```c
/* Compiler warning suppression */
#if defined(__GNUC__) || defined(__clang__)
#  define UNUSED(x)         __attribute__((unused)) x
#  define LIKELY(x)         __builtin_expect(!!(x), 1)
#  define UNLIKELY(x)       __builtin_expect(!!(x), 0)
#  define PACKED            __attribute__((packed))
#  define ALIGNED(n)        __attribute__((aligned(n)))
#  define NORETURN          __attribute__((noreturn))
#  define PRINTF_FMT(f, a)  __attribute__((format(printf, f, a)))
#else
#  define UNUSED(x)         x
#  define LIKELY(x)         (x)
#  define UNLIKELY(x)       (x)
#  define PACKED
#  define ALIGNED(n)
#  define NORETURN
#  define PRINTF_FMT(f, a)
#endif

/* Cross-platform export */
#if defined(_WIN32) || defined(_WIN64)
#  ifdef MYLIB_EXPORTS
#    define MYLIB_API __declspec(dllexport)
#  else
#    define MYLIB_API __declspec(dllimport)
#  endif
#else
#  define MYLIB_API __attribute__((visibility("default")))
#endif
```

### 3.8 Conditional Compilation

Conditional compilation directives control which code sections are compiled based on compile-time conditions.

```c
#if constant-expression       /* compile if expression != 0 */
#ifdef IDENTIFIER             /* compile if IDENTIFIER is defined */
#ifndef IDENTIFIER            /* compile if IDENTIFIER is NOT defined */
#elif constant-expression     /* else-if */
#else                         /* else */
#endif                        /* end of conditional block */
```

**Important distinction:**

```c
#define FOO 0

#ifdef FOO          /* TRUE — FOO is defined (its value is irrelevant) */
    ...
#endif

#if FOO             /* FALSE — FOO expands to 0 */
    ...
#endif
```

**`defined()` operator:**

```c
#if defined(FOO) && defined(BAR)    /* both must be defined */
#if defined(FOO) || !defined(BAR)   /* FOO defined OR BAR not defined */
```

**Feature flags and platform abstraction:**

```c
/* Compile-time feature selection */
#ifndef NDEBUG
#  define DCHECK(cond)   assert(cond)
#else
#  define DCHECK(cond)   ((void)0)
#endif

/* Platform-specific mutex */
#if defined(__linux__)
#  include <pthread.h>
   typedef pthread_mutex_t mutex_t;
#  define MUTEX_INIT(m)    pthread_mutex_init(&(m), NULL)
#  define MUTEX_LOCK(m)    pthread_mutex_lock(&(m))
#  define MUTEX_UNLOCK(m)  pthread_mutex_unlock(&(m))
#elif defined(_WIN32)
#  include <windows.h>
   typedef CRITICAL_SECTION mutex_t;
#  define MUTEX_INIT(m)    InitializeCriticalSection(&(m))
#  define MUTEX_LOCK(m)    EnterCriticalSection(&(m))
#  define MUTEX_UNLOCK(m)  LeaveCriticalSection(&(m))
#endif

/* Architecture-specific memory barrier */
#if defined(__x86_64__) || defined(__i386__)
#  define MEMORY_BARRIER()  __asm__ volatile("" ::: "memory")
#  define SFENCE()          __asm__ volatile("sfence" ::: "memory")
#  define LFENCE()          __asm__ volatile("lfence" ::: "memory")
#  define MFENCE()          __asm__ volatile("mfence" ::: "memory")
#elif defined(__aarch64__)
#  define MEMORY_BARRIER()  __asm__ volatile("dmb ish" ::: "memory")
#endif
```

### 3.9 Include Guards and `#pragma once`

**Include guards** prevent a header file from being included multiple times in a single translation unit:

```c
/* Traditional include guard */
#ifndef MY_HEADER_H
#define MY_HEADER_H

/* ... header content ... */

#endif /* MY_HEADER_H */

/* Pragma once — compiler-specific, widely supported, faster */
#pragma once
/* ... header content ... */
```

**Why include guards matter:**

```
main.c includes a.h and b.h.
Both a.h and b.h include common.h.

Without guards:
  main.c
    -> a.h -> common.h (struct Foo defined)
    -> b.h -> common.h (struct Foo defined AGAIN: error)

With guards in common.h:
  main.c
    -> a.h -> common.h (guard not set, include content, set guard)
    -> b.h -> common.h (guard IS set, skip entire file)
```

```
Include Guard Mechanism (internal):

First inclusion of common.h:
  Preprocessor checks: is COMMON_H defined?
  No -> include content, #define COMMON_H
  Result: guard macro is now in macro table

Second inclusion of common.h:
  Preprocessor checks: is COMMON_H defined?
  Yes -> skip everything until matching #endif
  Result: content never seen again by compiler
```

### 3.10 X-Macros Pattern

X-macros are one of the most powerful patterns in C macro programming. They allow you to maintain a single "data table" and generate multiple consistent code fragments from it.

**The core idea:** Define a list macro `X_LIST` that applies an `X()` macro to each item. Redefine `X` each time you want different output.

```c
/* Define the master table */
#define COLOR_LIST(X)     \
    X(RED,   0xFF0000)    \
    X(GREEN, 0x00FF00)    \
    X(BLUE,  0x0000FF)    \
    X(WHITE, 0xFFFFFF)    \
    X(BLACK, 0x000000)

/* Generate enum */
typedef enum {
#define X(name, value)  COLOR_ ## name,
    COLOR_LIST(X)
#undef X
    COLOR_COUNT
} Color;

/* Generate value array */
static const uint32_t color_values[] = {
#define X(name, value)  [COLOR_ ## name] = (value),
    COLOR_LIST(X)
#undef X
};

/* Generate string name array */
static const char *color_names[] = {
#define X(name, value)  [COLOR_ ## name] = #name,
    COLOR_LIST(X)
#undef X
};

/* Generate lookup function */
const char *color_to_string(Color c) {
    if (c < COLOR_COUNT) return color_names[c];
    return "UNKNOWN";
}
```

**Expansion visualized:**

```
COLOR_LIST(X) where X(name, value) = COLOR_ ## name,

Expands to:
    COLOR_RED,
    COLOR_GREEN,
    COLOR_BLUE,
    COLOR_WHITE,
    COLOR_BLACK,

Same data, different X definition:
X(name, value) = [COLOR_ ## name] = (value),
Expands to:
    [COLOR_RED]   = 0xFF0000,
    [COLOR_GREEN] = 0x00FF00,
    ...
```

**Error code table — production pattern:**

```c
#define ERROR_CODES(X)                                    \
    X(OK,            0,    "Success")                     \
    X(EINVAL,        1,    "Invalid argument")            \
    X(ENOMEM,        2,    "Out of memory")               \
    X(ENOENT,        3,    "No such entry")               \
    X(EACCES,        4,    "Permission denied")           \
    X(ETIMEDOUT,     5,    "Operation timed out")         \
    X(ECONNREFUSED,  6,    "Connection refused")

typedef enum {
#define X(name, code, msg)  ERR_ ## name = (code),
    ERROR_CODES(X)
#undef X
} ErrorCode;

static inline const char *error_message(ErrorCode e) {
    switch (e) {
#define X(name, code, msg)  case ERR_ ## name: return (msg);
        ERROR_CODES(X)
#undef X
        default: return "Unknown error";
    }
}
```

**Dispatch table / virtual table:**

```c
#define OPCODES(X)                 \
    X(NOP,  0x00, nop_handler)    \
    X(ADD,  0x01, add_handler)    \
    X(SUB,  0x02, sub_handler)    \
    X(MUL,  0x03, mul_handler)    \
    X(DIV,  0x04, div_handler)    \
    X(JMP,  0x10, jmp_handler)    \
    X(HALT, 0xFF, halt_handler)

/* Forward declare handlers */
#define X(name, opcode, handler)  void handler(VM *vm);
OPCODES(X)
#undef X

/* Build dispatch table */
typedef void (*OpcodeHandler)(VM *vm);
static OpcodeHandler dispatch_table[256] = {0};

void init_dispatch_table(void) {
#define X(name, opcode, handler)  dispatch_table[opcode] = handler;
    OPCODES(X)
#undef X
}
```

### 3.11 Macro Hygiene Problems and Solutions

C macros have NO hygiene. This is a fundamental design limitation.

**Problem 1: Multiple Evaluation**

```c
#define MAX(a, b)  ((a) > (b) ? (a) : (b))

int i = 5;
int result = MAX(i++, 3);
/* Expands to: ((i++) > (3) ? (i++) : (3))
   i is incremented TWICE if i > 3. Undefined behavior. */
```

**Problem 2: Variable Capture**

```c
#define SWAP(T, a, b)   { T tmp = a; a = b; b = tmp; }

/* If caller has a variable named 'tmp': */
int tmp = 10;
int x = 1, y = 2;
SWAP(int, x, tmp);
/* Expands to:
   { int tmp = x; x = tmp; tmp = tmp; }
   The inner 'tmp' shadows the outer 'tmp'.
   Behavior is not what's intended. */
```

**Problem 3: Scope Leakage**

```c
#define FOREACH(item, arr, len) \
    for (size_t _i = 0; _i < (len); _i++) { \
        __typeof__((arr)[0]) item = (arr)[_i];

/* Usage: */
FOREACH(elem, my_array, my_len)
    printf("%d\n", elem);
}  /* The brace from FOREACH must be closed manually — fragile */
```

**Mitigation strategies:**

```c
/* 1. Use GCC statement expressions to avoid multiple evaluation */
#define MAX(a, b)                  \
    ({                             \
        __typeof__(a) _a = (a);   \
        __typeof__(b) _b = (b);   \
        _a > _b ? _a : _b;        \
    })

/* Now MAX(i++, j++) evaluates each once, saves to _a, _b */

/* 2. Use unique variable names with __COUNTER__ or __LINE__ */
#define CONCAT_IMPL(a, b)   a ## b
#define CONCAT(a, b)        CONCAT_IMPL(a, b)
#define UNIQUE_VAR(name)    CONCAT(name, __LINE__)

#define SWAP_SAFE(T, a, b)              \
    do {                                \
        T UNIQUE_VAR(_tmp) = (a);       \
        (a) = (b);                      \
        (b) = UNIQUE_VAR(_tmp);         \
    } while(0)
/* UNIQUE_VAR(_tmp) expands to e.g. _tmp42, _tmp42 */
/* Still not perfect — __LINE__ same for both uses in same line */
```

### 3.12 Recursive Macros and Rescanning

The C preprocessor has a **paint-it-blue** rule: when a macro is being expanded, its own name is marked (painted blue) and will NOT be expanded again in the rescanning pass. This prevents infinite recursion.

```
Macro table:
  A -> A + 1

Expansion of A:
  1. We're expanding A. Paint A blue.
  2. Replacement: A + 1
  3. Rescan: find 'A' but it's blue — don't expand.
  4. Result: A + 1   (the A is the unexpanded token)
```

```c
#define EVIL  EVIL + 1

int x = EVIL;
/* Expands to: int x = EVIL + 1;
   The 'EVIL' in the output is NOT expanded (blue-painted).
   No infinite loop. */
```

**Consequences for indirect recursion:**

```c
#define A  B
#define B  A

int x = A;
/* Step 1: Expand A. Paint A blue. Replacement: B
   Step 2: Rescan. B is not blue. Expand B. Paint B blue. Replacement: A
   Step 3: Rescan. A is blue. Don't expand.
   Result: A
   The cycle stops because A is still blue. */
```

### 3.13 Pitfalls, Edge Cases, Best Practices

**Summary of common pitfalls:**

```
Pitfall                     Example                      Fix
-------------------------------------------------------------------
Missing parentheses         #define SQ(x) x*x            Always wrap params and whole expression
Multiple evaluation         MAX(f(), g())                 Use statement expressions or inline functions
Operator precedence         #define BITS 1<<3 (=8)        #define BITS (1<<3)
Semicolon swallowing        if(x) MACRO(); else ...       Use do{}while(0)
Variable capture            _tmp shadowing                Use __COUNTER__ / unique names
Accidental token join       #define SUFFIX _v2            Wrap in parens where possible
Stringification confusion   TOSTRING(__LINE__)            Use two-level indirection
Empty macro arg (C99)       LOG("msg") trailing comma     Use ##__VA_ARGS__ or __VA_OPT__
```

**Best practices:**

```c
/* 1. ALWAYS parenthesize parameters and the whole expression */
#define GOOD(x, y)  (((x) + (y)) * 2)
#define BAD(x, y)   x + y * 2

/* 2. Use UPPER_CASE for macro names to signal "I'm a macro" */
#define MAX_SIZE  1024       /* good */
#define maxSize   1024       /* bad: looks like a variable */

/* 3. Use inline functions instead of function-like macros when possible */
/* C99/C11 inline: same performance, type safety, no multiple-eval problem */
static inline int max(int a, int b) { return a > b ? a : b; }

/* 4. Use enum/const instead of #define for constants */
enum { BUFFER_SIZE = 4096 };       /* scoped, typed, debugger-visible */
static const int LIMIT = 100;      /* const, typed */
/* vs: #define BUFFER_SIZE 4096    no type, no scope, invisible in debugger */

/* 5. Undef macros that are only needed locally */
#define HELPER_MACRO(x)  ((x) * 2)
/* ... use HELPER_MACRO ... */
#undef HELPER_MACRO

/* 6. Document macro side effects */
/* MACRO: MAX(a, b)
 * WARNING: evaluates arguments twice. Do not use with side-effectful expressions. */
```

---

## 4. Rust Macros — Hygienic, Typed Metaprogramming

### 4.1 Macro System Overview and Phases

Rust has two distinct macro systems operating at different levels:

```
Rust Macro Systems
├── Declarative Macros (macro_rules!)
│   ├── Also called "macros by example"
│   ├── Pattern matching on token trees
│   ├── Hygienic by construction
│   ├── Compile in the same crate as usage
│   └── Cannot inspect types or do arbitrary computation
│
└── Procedural Macros (proc macros)
    ├── Regular Rust code running during compilation
    ├── Input: TokenStream -> Output: TokenStream
    ├── Three flavors:
    │   ├── #[derive(MyTrait)]  — impl generation
    │   ├── #[my_attr]          — attribute macros
    │   └── my_macro!(...)      — function-like proc macros
    └── Requires separate crate with proc-macro = true
```

**Token Trees:** Rust macros operate on **token trees**, not raw text. A token tree is either a single token or a delimited group:

```
Token Trees for:  foo(1 + 2, bar[3])

TokenTree::Ident("foo")
TokenTree::Group {
    delimiter: Parenthesis,
    tokens: [
        TokenTree::Literal(1),
        TokenTree::Punct('+'),
        TokenTree::Literal(2),
        TokenTree::Punct(','),
        TokenTree::Ident("bar"),
        TokenTree::Group {
            delimiter: Bracket,
            tokens: [TokenTree::Literal(3)]
        }
    ]
}
```

**Token trees are the fundamental unit.** Delimiters (`()`, `[]`, `{}`) MUST always be balanced for a token tree to be valid. This is why Rust macros can never cause "unmatched parenthesis" surprises.

### 4.2 Declarative Macros: `macro_rules!`

`macro_rules!` defines a macro by specifying match arms: pattern -> template pairs.

```
macro_rules! MACRO_NAME {
    ( PATTERN1 ) => { TEMPLATE1 };
    ( PATTERN2 ) => { TEMPLATE2 };
    ...
}
```

The macro expander tries arms in order, taking the first matching arm.

**Anatomy of a simple macro:**

```rust
macro_rules! say_hello {
    () => {
        println!("Hello!");
    };
    ($name:expr) => {
        println!("Hello, {}!", $name);
    };
}

say_hello!();           // matches first arm
say_hello!("Alice");    // matches second arm
say_hello!["Bob"];      // also matches second arm (any delimiter works)
say_hello! { "Carol" } // also matches second arm
```

**Note:** The invoking delimiter (`()`, `[]`, `{}`) is interchangeable for `macro_rules!` — all three are valid call sites. The conventional choice is `!()`.

### 4.3 Pattern Matching and Fragment Specifiers

Macro patterns use **metavariables** (`$name:specifier`) to capture parts of the input.

```
Fragment Specifiers (what $name can match):

$x:expr      Any expression: 1+2, foo(), x.bar(), if a { b } else { c }
$x:ident     Any identifier: foo, my_var, SomeType
$x:ty        Any type: i32, Vec<String>, &'a mut T, impl Trait
$x:pat       Any pattern: Some(x), (a, b), Foo { field }
$x:pat_param Same as pat but not top-level alternation (|)
$x:stmt      Any statement: let x = 1;  fn foo() {}  expr;
$x:block     A block expression: { ... }
$x:item      Any item: fn, struct, enum, impl, use, mod ...
$x:meta      A meta attribute item: #[derive(Debug)]'s inner: derive(Debug)
$x:literal   A literal: 42, "hello", 3.14, true, b'\n'
$x:lifetime  A lifetime: 'a, 'static
$x:vis       A visibility: pub, pub(crate), pub(super), (empty)
$x:tt        Any single token tree — the most flexible/escape hatch
```

**Fragment specifiers constrain BOTH what they match and what the result can be used as:**

```rust
macro_rules! demo {
    ($e:expr) => {
        // $e can be used wherever an expression is valid
        let x = $e;
        println!("{}", $e);  // $e is expanded each time — but hygienically
    };
    ($t:ty) => {
        // $t can be used wherever a type is valid
        let _: $t = Default::default();
    };
    ($i:ident) => {
        // $i can be used as an identifier — in function names, variable names, etc.
        let $i = 42;  // creates a variable with the given name
    };
}
```

**The `tt` specifier is the escape hatch:**

```rust
// $x:tt matches exactly ONE token tree — a single token OR a balanced group
// Useful when you don't know what type of fragment you'll receive

macro_rules! passthrough {
    ($x:tt) => { $x }  // returns whatever token tree was passed
}

passthrough!(42)        // -> 42
passthrough!(foo)       // -> foo
passthrough!({ let x = 1; x + 1 })  // -> { let x = 1; x + 1 }
```

**Multiple patterns and matching:**

```rust
macro_rules! my_vec {
    // Empty case
    () => {
        Vec::new()
    };
    // Single element - seed capacity from count
    ($elem:expr ; $n:expr) => {
        {
            let mut v = Vec::with_capacity($n);
            v.extend(::std::iter::repeat($elem).take($n));
            v
        }
    };
    // List of elements
    ($($x:expr),+ $(,)?) => {
        {
            let mut v = Vec::new();
            $(v.push($x);)+
            v
        }
    };
}

let a: Vec<i32> = my_vec![];           // Vec::new()
let b = my_vec![0; 5];                 // [0, 0, 0, 0, 0]
let c = my_vec![1, 2, 3];             // [1, 2, 3]
let d = my_vec![1, 2, 3,];            // trailing comma OK due to $(,)?
```

### 4.4 Repetition in `macro_rules!`

Repetitions allow matching and generating repeated patterns.

```
Repetition Syntax:
  $( PATTERN )SEPARATOR?  QUANTIFIER

  SEPARATOR: any single token (,  ;  =>  etc.)
  QUANTIFIER:
    *   zero or more
    +   one or more
    ?   zero or one (no separator allowed with ?)
```

**Repetition examples:**

```rust
// Comma-separated list, one or more
macro_rules! sum {
    ($($x:expr),+) => {
        0 $(+ $x)+
    };
}

sum!(1, 2, 3)     // expands to: 0 + 1 + 2 + 3
sum!(42)          // expands to: 0 + 42

// Nested repetitions — must match structure
macro_rules! matrix {
    ($(($($x:expr),+)),+) => {
        vec![$(vec![$($x),+]),+]
    };
}

matrix![(1, 2, 3), (4, 5, 6)]
// -> vec![vec![1, 2, 3], vec![4, 5, 6]]
```

**Counting with repetitions (a trick since macros have no native counter):**

```rust
macro_rules! count {
    () => (0usize);
    ($head:tt $($tail:tt)*) => (1usize + count!($($tail)*));
}

count!(a b c)  // -> 1 + 1 + 1 + 0 = 3
```

**TT muncher — a fundamental pattern for parsing arbitrary input:**

A TT muncher processes tokens one at a time through recursive macro invocation. It's the standard technique for parsing complex DSLs in `macro_rules!`.

```rust
// Example: parse a simple key: value DSL
macro_rules! config {
    // Base case: no more input
    (@parse {$($out:tt)*} ) => {
        { $($out)* }
    };
    // Match: key: value, rest...
    (@parse {$($out:tt)*} $key:ident : $val:expr , $($rest:tt)*) => {
        config!(@parse {
            $($out)*
            settings.insert(stringify!($key), $val);
        } $($rest)*)
    };
    // Entry point
    ($($input:tt)*) => {
        {
            let mut settings: std::collections::HashMap<&str, i64> = std::collections::HashMap::new();
            config!(@parse {} $($input)*);
            settings
        }
    };
}

let cfg = config!(timeout: 30, retries: 3, port: 8080,);
```

**The `@rule` convention:** Using `@name` as the first token of a macro arm is a conventional way to mark internal/recursive "helper arms" that shouldn't be called from outside the macro. It's a namespace convention, not a language feature.

**Push-down accumulator:** Accumulates output in a `{$($acc:tt)*}` group that grows with each step.

```
TT Muncher Execution Model:

macro! { A B C }
  -> macro! { @step { } A B C }   // init accumulator
     -> macro! { @step { out_A } B C }
        -> macro! { @step { out_A out_B } C }
           -> macro! { @step { out_A out_B out_C } }
              -> { out_A out_B out_C }  // done

Each recursive call processes one token, appends output to accumulator.
The accumulator is "pushed down" into deeper calls and "popped up"
as the recursion bottoms out.
```

### 4.5 Hygiene in Rust Macros

Hygiene is the property that macros cannot accidentally capture variables from their call site, and variables introduced by the macro cannot be captured by the code at the call site.

**How C macros fail hygiene:**

```c
/* C — unhygienic */
#define SWAP(a, b) { int tmp = a; a = b; b = tmp; }

int tmp = 99;
int x = 1, y = 2;
SWAP(x, tmp);
/* 'tmp' inside SWAP refers to the SAME 'tmp' as the caller. BUG. */
```

**How Rust macros achieve hygiene:**

Each identifier in a macro expansion is tagged with a **syntax context** — essentially a unique ID representing where (which macro invocation, at what expansion depth) the identifier came from. Two identifiers with different syntax contexts are different, even if they have the same name.

```
Syntax Context Model:

Call site context:    C0
macro_rules! swap:    defined at C1

In expansion of swap!(a, b) at call site C0:
  'tmp' introduced by the macro has context C1 (macro definition context)
  'a' and 'b' are metavariables — they inherit their context from call site C0

The binding: let tmp@C1 = ...
The reference: b@C0 = tmp@C1
  -> 'a' and 'b' at C0 refer to the caller's 'a' and 'b'
  -> 'tmp' at C1 is INVISIBLE to code at C0
  -> No capture possible
```

```rust
// Rust — hygienic swap
macro_rules! swap {
    ($a:expr, $b:expr) => {{
        let tmp = $a;    // 'tmp' has macro's syntax context
        $a = $b;
        $b = tmp;        // refers to the macro's 'tmp', not caller's
    }};
}

let tmp = 99;
let mut x = 1;
let mut y = 2;
swap!(x, tmp);   // Works correctly! Macro's 'tmp' != caller's 'tmp'
// x == 99, tmp == 1 (the original x)
// Caller's 'tmp' variable was used as the 'b' argument, not captured
```

**Intentional hygiene breaking with `$crate`:**

Sometimes you want a macro to refer to items from its defining crate, even when used from another crate:

```rust
// in my_crate:
#[macro_export]
macro_rules! my_vec {
    ($($x:expr),*) => {
        // Without $crate, 'Vec' might resolve to the caller's crate's Vec
        // With $crate, it always refers to this crate's Vec
        // For std types, $crate::vec::Vec is not needed, but for crate-local types:
        $crate::MyCustomType::new_from(vec![$($x),*])
    };
}
```

**Hygiene categories:**

```
Identifiers in macro expansions fall into three categories:

1. HYGIENIC (default for let/fn/variable bindings introduced by macro)
   - Invisible to call site
   - Cannot be referenced by name from call site

2. UNHYGIENIC (metavariables — $x, $y, etc.)
   - They carry the call site's syntax context
   - They resolve names as if written at the call site

3. ITEM-LEVEL (fn, struct, enum at module level)
   - Items defined by macros ARE accessible by name (they're in the module)
   - This is intentional — macros that generate functions should be usable
```

### 4.6 Procedural Macros — Architecture

Procedural macros are regular Rust programs that execute **at compile time** and transform `TokenStream` → `TokenStream`.

**The fundamental separation:** Proc macros must live in a separate crate with `proc-macro = true` in `Cargo.toml`. The proc macro crate is compiled for the **host** (the machine running the compiler), while the crate using the macro is compiled for the **target** (which might be a different architecture).

```
Proc Macro Architecture:

  my-proc-macro/ (Cargo.toml: [lib] proc-macro = true)
  ├── Compiled for HOST (build machine)
  ├── Loaded as a dynamic library by rustc
  └── Functions exported as proc macros

  my-application/ (depends on my-proc-macro)
  ├── Compiled for TARGET (could be different arch)
  ├── At compile time, rustc calls into the proc macro
  │   passing TokenStream, receiving TokenStream
  └── The returned tokens are inserted into the AST

  Compilation timeline:
    1. Compile my-proc-macro -> libmy_proc_macro.so (for host)
    2. Start compiling my-application
    3. Encounter #[derive(MyDerive)] on a struct
    4. Load libmy_proc_macro.so
    5. Call my_derive(item: TokenStream) -> TokenStream
    6. Insert returned TokenStream into AST
    7. Continue compilation with expanded code
```

**Cargo.toml for a proc macro crate:**

```toml
[package]
name = "my-proc-macro"
version = "0.1.0"
edition = "2021"

[lib]
proc-macro = true

[dependencies]
syn = { version = "2", features = ["full"] }
quote = "1"
proc-macro2 = "1"
```

**The `proc-macro2` distinction:**

`proc_macro` is the compiler's built-in crate — only available in proc macro crates. `proc_macro2` is a user-space crate that re-exports a compatible API but can be used in tests and regular code. Best practice: use `proc_macro2` internally, convert at the boundary.

```rust
// Entry point of proc macro: takes proc_macro::TokenStream
// Internal work: uses proc_macro2::TokenStream (testable)

use proc_macro::TokenStream;
use proc_macro2::TokenStream as TokenStream2;

#[proc_macro_derive(MyTrait)]
pub fn my_derive(input: TokenStream) -> TokenStream {
    // Convert to proc_macro2 for internal work
    let input2: TokenStream2 = input.into();
    // ... do work with syn/quote using TokenStream2 ...
    let output: TokenStream2 = impl_my_trait(input2);
    // Convert back for return
    output.into()
}
```

### 4.7 Custom Derive Macros

Custom derive macros are invoked with `#[derive(MyTrait)]` on a struct or enum. They receive the item as a `TokenStream` and return additional code (typically an `impl` block).

**Key constraint:** Derive macros can only ADD new items, they CANNOT modify the item they're derived on. If you need to modify the struct itself, use an attribute macro.

**Complete example — deriving a `Summary` trait:**

```rust
// === In my-derive crate (proc-macro = true) ===

use proc_macro::TokenStream;
use quote::quote;
use syn::{parse_macro_input, DeriveInput, Data, Fields};

#[proc_macro_derive(Summary, attributes(summary))]
pub fn derive_summary(input: TokenStream) -> TokenStream {
    let input = parse_macro_input!(input as DeriveInput);
    let name = &input.ident;

    // Extract fields from struct
    let fields = match &input.data {
        Data::Struct(data_struct) => {
            match &data_struct.fields {
                Fields::Named(fields) => {
                    fields.named.iter()
                        .map(|f| f.ident.as_ref().unwrap())
                        .collect::<Vec<_>>()
                }
                _ => panic!("Summary only works on structs with named fields"),
            }
        }
        _ => panic!("Summary only works on structs"),
    };

    // Generate the impl block
    let expanded = quote! {
        impl Summary for #name {
            fn summarize(&self) -> String {
                let mut parts = Vec::new();
                #(
                    parts.push(format!("{}: {:?}", stringify!(#fields), self.#fields));
                )*
                parts.join(", ")
            }
        }
    };

    TokenStream::from(expanded)
}

// === In user crate ===
use my_derive::Summary;

trait Summary {
    fn summarize(&self) -> String;
}

#[derive(Summary, Debug)]
struct Article {
    title: String,
    author: String,
    word_count: usize,
}

let a = Article {
    title: "BGP Internals".to_string(),
    author: "Alice".to_string(),
    word_count: 5000,
};
println!("{}", a.summarize());
// Output: title: "BGP Internals", author: "Alice", word_count: 5000
```

**Derive with helper attributes:**

```rust
// #[proc_macro_derive(MyDerive, attributes(my_attr))]
// Registers 'my_attr' as a known attribute for fields/variants of this derive

#[proc_macro_derive(Builder, attributes(builder))]
pub fn derive_builder(input: TokenStream) -> TokenStream {
    let input = parse_macro_input!(input as DeriveInput);
    // ... look for #[builder(default = "...")] on fields ...
    todo!()
}

// Usage:
#[derive(Builder)]
struct Config {
    host: String,
    #[builder(default = "8080")]
    port: u16,
    #[builder(default = "false")]
    tls: bool,
}
```

### 4.8 Attribute Macros

Attribute macros attach to any item (fn, struct, enum, impl, module) and can **replace** the entire item with generated code.

```rust
// #[proc_macro_attribute] functions take TWO TokenStreams:
//   attr:  the content of the attribute itself
//   item:  the item the attribute is attached to

#[proc_macro_attribute]
pub fn route(attr: TokenStream, item: TokenStream) -> TokenStream {
    // attr: GET "/path"
    // item: async fn handler(req: Request) -> Response { ... }
    // Returns: both the original fn + registration code
    todo!()
}

// Usage:
#[route(GET "/users")]
async fn list_users(req: Request) -> Response {
    // ...
}
```

**Real example — timing attribute:**

```rust
// In proc macro crate:
use proc_macro::TokenStream;
use quote::quote;
use syn::{parse_macro_input, ItemFn};

#[proc_macro_attribute]
pub fn timed(_attr: TokenStream, item: TokenStream) -> TokenStream {
    let mut func = parse_macro_input!(item as ItemFn);
    let func_name = &func.sig.ident;
    let func_name_str = func_name.to_string();
    let original_block = func.block.clone();

    // Replace the function body
    func.block = syn::parse_quote! {
        {
            let __start = ::std::time::Instant::now();
            let __result = (|| #original_block)();
            let __elapsed = __start.elapsed();
            eprintln!("[timing] {} took {:?}", #func_name_str, __elapsed);
            __result
        }
    };

    quote!(#func).into()
}

// Usage:
#[timed]
fn expensive_computation(n: u64) -> u64 {
    (0..n).sum()
}
// Automatically prints timing info when called
```

**Attribute macro replacing an item with multiple items:**

```rust
#[proc_macro_attribute]
pub fn test_with_logging(attr: TokenStream, item: TokenStream) -> TokenStream {
    let func = parse_macro_input!(item as ItemFn);
    let name = &func.sig.ident;

    // Wraps the function AND adds a logging test
    let expanded = quote! {
        #func   // original function preserved

        #[test]
        fn log_test_for_#name() {
            eprintln!("=== Running test: {} ===", stringify!(#name));
            #name();
            eprintln!("=== Test {} passed ===", stringify!(#name));
        }
    };
    expanded.into()
}
```

### 4.9 Function-Like Procedural Macros

Function-like proc macros look like `my_macro!(...)` but are implemented as full Rust programs. Unlike `macro_rules!`, they can do arbitrary computation, parse complex DSLs, and access the file system.

```rust
// In proc macro crate:
#[proc_macro]
pub fn sql(input: TokenStream) -> TokenStream {
    let sql_text = input.to_string();
    // Parse and validate SQL at compile time
    match parse_sql(&sql_text) {
        Ok(query) => generate_query_code(query),
        Err(e) => {
            // Emit a compile error with span info
            let error = syn::Error::new(
                proc_macro2::Span::call_site(),
                format!("Invalid SQL: {}", e)
            );
            error.to_compile_error().into()
        }
    }
}

// Usage:
let query = sql!(SELECT * FROM users WHERE id = ?);
// If the SQL is invalid, the compilation fails with a clear error
```

**Compile-time file inclusion with proc macro:**

```rust
#[proc_macro]
pub fn include_json_schema(input: TokenStream) -> TokenStream {
    // Parse filename from input
    let filename = parse_filename(input);
    // Read file at compile time (proc macros have filesystem access)
    let content = std::fs::read_to_string(&filename)
        .expect("Could not read schema file");
    // Parse JSON and generate Rust structs
    let schema: serde_json::Value = serde_json::from_str(&content).unwrap();
    generate_structs_from_schema(schema).into()
}

// Usage:
include_json_schema!("api_schema.json");
// At compile time, reads the file and generates struct types
```

### 4.10 `TokenStream`, `syn`, and `quote`

These three components are the backbone of proc macro development.

**`proc_macro2::TokenStream` — the raw material:**

```
TokenStream is an iterator of TokenTree.
TokenTree is:
  - Ident   (identifier: foo, my_var, Vec)
  - Punct   (punctuation: + - * / = < > ! . , ; : :: & | ^ ~ ? @)
  - Literal (number, string, char: 42, "hello", 'a', 3.14)
  - Group   (delimited: (...), [...], {...})
```

**`syn` — parsing TokenStream into a structured AST:**

`syn` can parse a `TokenStream` into rich Rust syntax types:

```rust
use syn::{
    // Top-level items
    DeriveInput,   // struct/enum/union that can have #[derive]
    ItemFn,        // fn declaration
    ItemStruct,    // struct declaration
    ItemEnum,      // enum declaration
    ItemImpl,      // impl block
    
    // Expressions
    Expr,          // any expression
    ExprCall,      // foo(args)
    ExprField,     // foo.bar
    
    // Types
    Type,          // any type
    TypePath,      // Vec<String>
    TypeReference, // &'a T
    
    // Patterns
    Pat,           // any pattern
    
    // Miscellaneous
    Ident,         // an identifier
    LitStr,        // string literal "hello"
    LitInt,        // integer literal 42
    Attribute,     // #[...] attribute
    Generics,      // <T: Trait, U>
    Visibility,    // pub, pub(crate), etc.
};

// Parse DeriveInput from TokenStream
let ast = syn::parse_macro_input!(input as DeriveInput);

// Navigate the AST
match &ast.data {
    syn::Data::Struct(s) => {
        for field in s.fields.iter() {
            let field_name = &field.ident;
            let field_type = &field.ty;
            let field_attrs = &field.attrs;
            // Process each field...
        }
    }
    syn::Data::Enum(e) => {
        for variant in e.variants.iter() {
            let variant_name = &variant.ident;
            // Process each variant...
        }
    }
    _ => {}
}
```

**`quote!` — generating TokenStream from Rust-like syntax:**

`quote!` is the inverse of `syn::parse` — it lets you write Rust-like syntax with interpolations and get back a `TokenStream`.

```rust
use quote::quote;

let name = quote::format_ident!("my_function");
let field_names = vec![
    quote::format_ident!("x"),
    quote::format_ident!("y"),
];
let field_types = vec![quote!(i32), quote!(f64)];

// Interpolation with #variable
let generated = quote! {
    struct #name {
        #(#field_names: #field_types),*    // repetition
    }

    impl #name {
        fn new(#(#field_names: #field_types),*) -> Self {
            Self { #(#field_names),* }
        }
    }
};
// 'generated' is a TokenStream ready to be returned from a proc macro
```

**`quote!` interpolation syntax:**

```
#var              — interpolate a single value (must impl ToTokens)
#(#vec_var),*     — interpolate each element of a vec, separated by ','
#(#vec_var);*     — interpolate each element separated by ';'
#(#a #b),*        — interpolate pairs from two vecs
##                — literal '#' in the output
```

**Error reporting with spans:**

```rust
use proc_macro2::Span;
use syn::Error;

// Error pointing to a specific piece of input
fn validate_field(field: &syn::Field) -> Result<(), Error> {
    if field.ident.is_none() {
        return Err(Error::new_spanned(
            field,
            "All fields must be named"
        ));
    }
    Ok(())
}

// In proc macro:
let errors: Vec<Error> = fields.iter()
    .filter_map(|f| validate_field(f).err())
    .collect();

// Combine all errors and emit them
if !errors.is_empty() {
    let combined = errors.into_iter()
        .reduce(|mut acc, e| { acc.combine(e); acc })
        .unwrap();
    return combined.to_compile_error().into();
}
```

### 4.11 Macro Export, Import, and Scoping Rules

**`macro_rules!` scoping:**

`macro_rules!` macros follow unusual scoping rules — they are visible **from the point of definition to the end of the module** (textual/sequential scope), not in the whole module.

```rust
// Works:
mod foo {
    macro_rules! my_macro { () => {} }
    my_macro!();   // defined above: visible
}

// Fails:
mod bar {
    my_macro!();   // defined below: NOT visible
    macro_rules! my_macro { () => {} }
}
```

**`#[macro_export]`:**

To make a macro available to other crates (or from the crate root within the same crate):

```rust
// my_crate/src/lib.rs or any module
#[macro_export]
macro_rules! my_macro {
    () => { println!("exported!"); }
}

// Exported macros are placed at the CRATE ROOT regardless of where defined.
// Users import with:
use my_crate::my_macro;
// or:
#[macro_use]
extern crate my_crate;   // legacy style, imports all #[macro_export] macros
```

**Proc macro scoping:**

Proc macros are items exported from a proc macro crate. They're imported like regular items:

```rust
use my_proc_macro::{MyDerive, my_attr, my_fn_macro};
```

**`macro_rules!` `use` imports (2018 edition):**

```rust
// After Rust 2018, use macro like any other item:
mod utils {
    #[macro_export]  // needed to be importable
    macro_rules! helper { () => {} }
}
use utils::helper;   // direct import
```

### 4.12 Macro Debugging Techniques

**1. `cargo expand`** — the most important tool:

```bash
# Install: cargo install cargo-expand
cargo expand                    # expand all macros in current crate
cargo expand my_module          # expand specific module
cargo expand --bin my_binary    # expand specific binary
```

**2. `dbg_macro` crate:**

```rust
// Like dbg! but also shows the file and line
dbg_macro::dbg!(my_complex_expression);
```

**3. `eprintln!` in proc macros:**

```rust
#[proc_macro_derive(Debug2)]
pub fn debug2(input: TokenStream) -> TokenStream {
    eprintln!("=== INPUT ===\n{}", input);  // prints during compilation
    // ...
    eprintln!("=== OUTPUT ===\n{}", output);
    output.into()
}
```

**4. `compile_error!` for condition diagnostics:**

```rust
macro_rules! only_debug {
    ($($t:tt)*) => {
        #[cfg(not(debug_assertions))]
        compile_error!("This macro only works in debug builds");
        $($t)*
    };
}
```

**5. `trace_macros!` nightly feature:**

```rust
#![feature(trace_macros)]
trace_macros!(true);
my_macro!(some input);  // prints each expansion step
trace_macros!(false);
```

**6. Testing proc macros with `proc-macro2` and `syn`:**

```rust
#[cfg(test)]
mod tests {
    use super::*;
    use proc_macro2::TokenStream;
    use quote::quote;

    #[test]
    fn test_my_derive() {
        let input = quote! {
            struct MyStruct {
                field1: String,
                field2: i32,
            }
        };
        // Call the internal implementation (not the proc_macro entry point)
        let output = impl_my_derive(input);
        // Parse and inspect the output
        let output_ast: syn::File = syn::parse2(output).unwrap();
        // Assert on the output...
    }
}
```

---

## 5. Go — No Macros, But Metaprogramming Exists

### 5.1 Why Go Has No Macros

Go deliberately omitted a macro system. The design philosophy:

```
Go's position on metaprogramming:

1. Simplicity: Macros add a "meta-language" on top of the language.
   Understanding a Go program requires understanding Go only — no macro DSL.

2. Readability: Go prioritizes reading code over writing code.
   "If you can't read it at a glance, it's not idiomatic Go."

3. Tool-friendliness: gofmt, go vet, gopls all work because Go syntax is
   completely regular. Macros would break these tools.

4. Code generation is explicit: Generated code is checked into version control
   and visible in code review. Macro-generated code is invisible.

5. Generics (added in Go 1.18) eliminate the most common macro use case
   (generic containers/algorithms) without introducing metaprogramming complexity.
```

**What Go has instead:**

```
C macros equivalent in Go
─────────────────────────────────────────────────────
Constants          -> const (typed, scoped)
Conditional comp.  -> Build tags (//go:build)
Code generation    -> //go:generate + external tools
Type generics      -> Go generics (1.18+)
Inline functions   -> Compiler inlining (automatic)
Debug code         -> Build tags or runtime checks
String constants   -> iota + const blocks
```

### 5.2 `go:generate` — The Code Generation Gateway

`//go:generate` is a comment directive that tells `go generate` to run a command. It's the official Go metaprogramming mechanism.

```go
// In source file:
//go:generate <command> [arguments...]

// Examples:
//go:generate stringer -type=Color
//go:generate mockgen -source=interface.go -destination=mock.go
//go:generate protoc --go_out=. api.proto
//go:generate go run gen/main.go
```

**Running generators:**

```bash
go generate ./...        # run all generators in all packages
go generate ./pkg/...    # specific package tree
go generate -v ./...     # verbose: print each command
go generate -n ./...     # dry run: print but don't execute
go generate -run regexp  # only run generators matching regexp
```

**Special variables in `//go:generate`:**

```go
$GOFILE     // current source file name
$GOLINE     // line number of the //go:generate directive
$GOPACKAGE  // package name of the current file
$GOARCH     // target architecture
$GOOS       // target operating system

//go:generate echo Processing $GOFILE in package $GOPACKAGE
```

**Example: Generating a complete enum with Stringer:**

```go
// color.go
package graphics

//go:generate stringer -type=Color -output=color_string.go

type Color int

const (
    Red Color = iota
    Green
    Blue
    Alpha
)
```

Running `go generate` produces `color_string.go`:

```go
// Code generated by "stringer -type=Color -output=color_string.go"; DO NOT EDIT.
package graphics

import "strconv"

func _() {
    // Compile-time check that iota values haven't changed
    var x [1]struct{}
    _ = x[Red-0]
    _ = x[Green-1]
    _ = x[Blue-2]
    _ = x[Alpha-3]
}

const _Color_name = "RedGreenBlueAlpha"

var _Color_index = [...]uint8{0, 3, 8, 12, 17}

func (i Color) String() string {
    if i < 0 || i >= Color(len(_Color_index)-1) {
        return "Color(" + strconv.FormatInt(int64(i), 10) + ")"
    }
    return _Color_name[_Color_index[i]:_Color_index[i+1]]
}
```

### 5.3 Build Tags and Conditional Compilation

Build tags are the Go equivalent of `#ifdef`. They control which files are included in a build.

**Modern syntax (Go 1.17+):**

```go
//go:build EXPRESSION

// Examples:
//go:build linux
//go:build linux && amd64
//go:build linux || darwin
//go:build !windows
//go:build (linux || darwin) && cgo
//go:build go1.18   // requires at minimum Go 1.18
```

**The old syntax (pre 1.17, still supported):**

```go
// +build linux
// +build linux,amd64
// +build !windows
```

**File naming conventions (automatic build constraints):**

```
Go also uses filename patterns as implicit build tags:

*_linux.go        -> only compiled on linux
*_windows.go      -> only compiled on windows
*_darwin.go       -> only compiled on darwin
*_amd64.go        -> only compiled for amd64 architecture
*_linux_amd64.go  -> only compiled on linux/amd64
*_test.go         -> only compiled for tests

These work WITHOUT any //go:build comment.
```

**Platform-specific implementation:**

```go
// file: mutex_linux.go
//go:build linux

package sync2

import "golang.org/x/sys/unix"

type Mutex struct {
    futex int32
}

func (m *Mutex) Lock() {
    // Linux futex-based implementation
    for !atomic.CompareAndSwapInt32(&m.futex, 0, 1) {
        unix.Futex(&m.futex, unix.FUTEX_WAIT, 1, nil, nil, 0)
    }
}
```

```go
// file: mutex_windows.go
//go:build windows

package sync2

import "syscall"

type Mutex struct {
    cs syscall.Handle
}

func (m *Mutex) Lock() {
    // Windows CRITICAL_SECTION implementation
    syscall.WaitForSingleObject(m.cs, syscall.INFINITE)
}
```

**Custom build tags for feature flags:**

```go
// file: feature_tracing.go
//go:build tracing

package main

import "github.com/opentelemetry/otel"

func initTracing() {
    otel.SetTracerProvider(...)
}
```

```go
// file: feature_tracing_stub.go
//go:build !tracing

package main

func initTracing() {
    // no-op when tracing not enabled
}
```

```bash
go build -tags tracing ./...      # with tracing
go build ./...                    # without tracing
```

### 5.4 `text/template` and `go/ast` for Code Generation

The Go standard library provides first-class tools for code generation.

**`text/template` for generating Go source:**

```go
// gen/main.go — a code generator

package main

import (
    "os"
    "text/template"
)

const typedSliceTmpl = `// Code generated by gen/main.go; DO NOT EDIT.
package {{.Package}}

// {{.TypeName}}Slice is a type-safe slice of {{.ElemType}}.
type {{.TypeName}}Slice []{{.ElemType}}

func (s {{.TypeName}}Slice) Filter(pred func({{.ElemType}}) bool) {{.TypeName}}Slice {
    out := {{.TypeName}}Slice{}
    for _, v := range s {
        if pred(v) {
            out = append(out, v)
        }
    }
    return out
}

func (s {{.TypeName}}Slice) Map(f func({{.ElemType}}) {{.ElemType}}) {{.TypeName}}Slice {
    out := make({{.TypeName}}Slice, len(s))
    for i, v := range s {
        out[i] = f(v)
    }
    return out
}

func (s {{.TypeName}}Slice) Reduce(init {{.ElemType}}, f func({{.ElemType}}, {{.ElemType}}) {{.ElemType}}) {{.ElemType}} {
    acc := init
    for _, v := range s {
        acc = f(acc, v)
    }
    return acc
}
`

type TemplateData struct {
    Package  string
    TypeName string
    ElemType string
}

func main() {
    tmpl := template.Must(template.New("typed_slice").Parse(typedSliceTmpl))

    types := []TemplateData{
        {"mypackage", "Int", "int"},
        {"mypackage", "Float64", "float64"},
        {"mypackage", "String", "string"},
    }

    for _, data := range types {
        filename := strings.ToLower(data.TypeName) + "_slice_gen.go"
        f, _ := os.Create(filename)
        defer f.Close()
        tmpl.Execute(f, data)
    }
}
```

**`go/ast` — inspecting existing Go code:**

The `go/ast` package parses Go source into an AST you can walk and transform:

```go
package main

import (
    "go/ast"
    "go/parser"
    "go/token"
    "fmt"
)

func main() {
    src := `
package example

type User struct {
    ID    int64  ` + "`json:\"id\"`" + `
    Name  string ` + "`json:\"name\"`" + `
    Email string ` + "`json:\"email\"`" + `
}
`
    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
    if err != nil {
        panic(err)
    }

    // Walk the AST
    ast.Inspect(f, func(n ast.Node) bool {
        typeSpec, ok := n.(*ast.TypeSpec)
        if !ok {
            return true
        }
        structType, ok := typeSpec.Type.(*ast.StructType)
        if !ok {
            return true
        }

        fmt.Printf("Struct: %s\n", typeSpec.Name.Name)
        for _, field := range structType.Fields.List {
            for _, name := range field.Names {
                fmt.Printf("  Field: %s (%s)\n",
                    name.Name,
                    // Print type as string
                    astTypeToString(field.Type))
            }
        }
        return true
    })
}
```

**`go/types` — type-aware analysis:**

```go
import (
    "go/types"
    "golang.org/x/tools/go/packages"
)

// Load a package with full type information
cfg := &packages.Config{
    Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
}
pkgs, _ := packages.Load(cfg, ".")

for _, pkg := range pkgs {
    for _, file := range pkg.Syntax {
        ast.Inspect(file, func(n ast.Node) bool {
            // With type info, we can look up the type of any expression
            if expr, ok := n.(ast.Expr); ok {
                t := pkg.TypesInfo.TypeOf(expr)
                if t != nil {
                    fmt.Printf("expr: %T -> type: %s\n", expr, t)
                }
            }
            return true
        })
    }
}
```

### 5.5 Writing a Real Code Generator

A complete generator that reads struct annotations and generates CRUD SQL:

```go
// cmd/sqlgen/main.go

package main

import (
    "flag"
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "os"
    "strings"
    "text/template"
)

type Field struct {
    Name    string
    Type    string
    Column  string
}

type Model struct {
    Package string
    Name    string
    Table   string
    Fields  []Field
}

const crudTemplate = `// Code generated by sqlgen; DO NOT EDIT.
// Source: {{.Package}}

package {{.Package}}

import "database/sql"

func Scan{{.Name}}(row *sql.Row) (*{{.Name}}, error) {
    m := &{{.Name}}{}
    err := row.Scan({{range $i, $f := .Fields}}{{if $i}}, {{end}}&m.{{$f.Name}}{{end}})
    return m, err
}

func Insert{{.Name}}(db *sql.DB, m *{{.Name}}) error {
    _, err := db.Exec(
        ` + "`" + `INSERT INTO {{.Table}} ({{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.Column}}{{end}}) VALUES ({{range $i, $f := .Fields}}{{if $i}}, {{end}}?{{end}})` + "`" + `,
        {{range $i, $f := .Fields}}{{if $i}}, {{end}}m.{{$f.Name}}{{end}},
    )
    return err
}

func Get{{.Name}}ByID(db *sql.DB, id int64) (*{{.Name}}, error) {
    row := db.QueryRow(` + "`" + `SELECT {{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.Column}}{{end}} FROM {{.Table}} WHERE id = ?` + "`" + `, id)
    return Scan{{.Name}}(row)
}
`

func main() {
    file := flag.String("file", "", "source file to process")
    flag.Parse()

    fset := token.NewFileSet()
    f, err := parser.ParseFile(fset, *file, nil, parser.ParseComments)
    if err != nil {
        fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
        os.Exit(1)
    }

    var models []Model
    ast.Inspect(f, func(n ast.Node) bool {
        genDecl, ok := n.(*ast.GenDecl)
        if !ok {
            return true
        }
        for _, spec := range genDecl.Specs {
            typeSpec, ok := spec.(*ast.TypeSpec)
            if !ok {
                continue
            }
            structType, ok := typeSpec.Type.(*ast.StructType)
            if !ok {
                continue
            }

            model := Model{
                Package: f.Name.Name,
                Name:    typeSpec.Name.Name,
                Table:   strings.ToLower(typeSpec.Name.Name) + "s",
            }

            for _, field := range structType.Fields.List {
                if len(field.Names) == 0 {
                    continue
                }
                col := strings.ToLower(field.Names[0].Name)
                if field.Tag != nil {
                    tag := field.Tag.Value
                    if idx := strings.Index(tag, `db:"`); idx != -1 {
                        start := idx + 4
                        end := strings.Index(tag[start:], `"`)
                        col = tag[start : start+end]
                    }
                }
                model.Fields = append(model.Fields, Field{
                    Name:   field.Names[0].Name,
                    Type:   fmt.Sprintf("%T", field.Type),
                    Column: col,
                })
            }
            models = append(models, model)
        }
        return true
    })

    tmpl := template.Must(template.New("crud").Parse(crudTemplate))
    for _, m := range models {
        outFile := strings.ToLower(m.Name) + "_gen.go"
        out, _ := os.Create(outFile)
        defer out.Close()
        tmpl.Execute(out, m)
    }
}
```

**In the user package:**

```go
// user.go
package models

//go:generate go run ../../cmd/sqlgen/main.go -file=user.go

type User struct {
    ID    int64  `db:"id"`
    Name  string `db:"name"`
    Email string `db:"email"`
}
```

### 5.6 Stringer, Mockgen, and Protocol Buffers

**`stringer`:** Generates `String()` methods for integer enums.

```go
//go:generate stringer -type=Weekday

type Weekday int

const (
    Sunday Weekday = iota
    Monday
    Tuesday
    Wednesday
    Thursday
    Friday
    Saturday
)
// After go generate: Sunday.String() == "Sunday"
```

**`mockgen`:** Generates mock implementations of interfaces.

```go
// repository.go
package storage

//go:generate mockgen -source=repository.go -destination=mock_repository.go -package=storage

type UserRepository interface {
    FindByID(id int64) (*User, error)
    Save(user *User) error
    Delete(id int64) error
}
```

Generated mock:

```go
// mock_repository.go — DO NOT EDIT
type MockUserRepository struct {
    ctrl     *gomock.Controller
    recorder *MockUserRepositoryMockRecorder
}

func (m *MockUserRepository) FindByID(id int64) (*User, error) {
    m.ctrl.T.Helper()
    ret := m.ctrl.Call(m, "FindByID", id)
    ret0, _ := ret[0].(*User)
    ret1, _ := ret[1].(error)
    return ret0, ret1
}
```

**Protocol Buffers:** The protoc compiler + Go plugins generate complete Go types from `.proto` files:

```protobuf
// api.proto
syntax = "proto3";
package api;

message User {
    int64  id    = 1;
    string name  = 2;
    string email = 3;
}
```

```go
//go:generate protoc --go_out=. --go-grpc_out=. api.proto
```

Generates `api.pb.go` with complete struct definitions, serialization, and gRPC service implementations.

### 5.7 Generics as a Macro Replacement

Go 1.18 introduced generics, which eliminate the most common reason to want macros: type-parameterized data structures and algorithms.

**Before generics (required code generation):**

```go
// Had to generate this for each type:
//go:generate gen-typed-list -type=int
//go:generate gen-typed-list -type=string

type IntList []int
func (l IntList) Map(f func(int) int) IntList { ... }

type StringList []string
func (l StringList) Map(f func(string) string) StringList { ... }
```

**After generics:**

```go
// One implementation works for all types
type List[T any] []T

func (l List[T]) Map[U any](f func(T) U) List[U] {
    out := make(List[U], len(l))
    for i, v := range l {
        out[i] = f(v)
    }
    return out
}

func (l List[T]) Filter(pred func(T) bool) List[T] {
    var out List[T]
    for _, v := range l {
        if pred(v) {
            out = append(out, v)
        }
    }
    return out
}

func (l List[T]) Reduce[U any](init U, f func(U, T) U) U {
    acc := init
    for _, v := range l {
        acc = f(acc, v)
    }
    return acc
}

// Usage:
ints := List[int]{1, 2, 3, 4, 5}
doubled := ints.Map(func(x int) int { return x * 2 })
evens := ints.Filter(func(x int) bool { return x%2 == 0 })
sum := ints.Reduce(0, func(acc, x int) int { return acc + x })
```

**Constraints — the type bound system:**

```go
import "golang.org/x/exp/constraints"

type Number interface {
    constraints.Integer | constraints.Float
}

func Sum[T Number](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}

Sum([]int{1, 2, 3})      // 6
Sum([]float64{1.1, 2.2}) // 3.3
```

---

## 6. Cross-Language Comparison and Mental Models

### Comparison Table

```
Feature                 C Preprocessor    Rust macro_rules!   Rust proc macro    Go
─────────────────────────────────────────────────────────────────────────────────────
Stage                   Before parsing    After tokenizing    After tokenizing   Codegen (external)
Input                   Raw text          Token trees         TokenStream        Source files
Output                  Text/tokens       Token trees         TokenStream        Source files
Type awareness          None              Fragment spec only  Full (via syn)     Full (go/types)
Hygiene                 None              Yes                 Yes (with care)    N/A (codegen)
Arbitrary computation   No                No                  Yes (full Rust)    Yes (full Go)
Cross-crate             #include          #[macro_export]     Normal crate dep   go:generate cmd
Recursive               Yes (limited)     Yes (via TT munch)  Yes               Yes
Debugging               cpp -E, gcc -E    cargo expand        eprintln!, expand  Inspect gen files
Error messages          Poor              Good                Excellent w/ spans N/A
Standard library use    No                No (const only)     Yes               Yes
Filesystem access       No                No                  Yes               Yes
Performance impact      Zero              Zero                Zero (compile)     Zero
Runtime overhead        Zero              Zero                Zero              Zero
```

### Mental Model: What Each System Really Is

```
C Preprocessor
──────────────
Think of it as: an extremely powerful "sed" that runs first.
It knows about:  tokens (roughly), but not C syntax
It CANNOT:       reason about types, scopes, or semantics
Its strength:    fast, universal, works with any C-like language
Its weakness:    no safety, no hygiene, easy to shoot yourself


macro_rules!
────────────
Think of it as: a pattern-matching rewrite system on token trees.
It knows about:  token trees, fragment types (expr, ty, ident, etc.)
It CANNOT:       do arbitrary computation, access types, read files
Its strength:    hygienic, zero dependencies, highly readable patterns
Its weakness:    complex patterns become unreadable, limited power


Proc Macros
───────────
Think of it as: a compiler plugin that transforms AST.
It knows about:  the full token stream, types (via syn), structure
It CAN:          do anything a Rust program can do at compile time
Its strength:    immense power, excellent error messages, full Rust
Its weakness:    complexity, separate crate needed, compile time cost


Go go:generate
──────────────
Think of it as: a Makefile step that generates source files.
It knows about:  everything (it's a full Go program)
It CANNOT:       run during compilation, be invisible
Its strength:    explicit, auditable, works with any tool
Its weakness:    not "zero friction", generated files need committing
```

### When to Use Which

```
Use C macros when:
  - Writing C (you have no choice for true compile-time constants/conditionals)
  - Cross-platform abstraction (different OS/arch code)
  - Performance-critical inlining (though inline functions are usually better)
  - X-macro tables (excellent for error code tables, dispatch tables)
  - Feature flags in C codebases

Use macro_rules! when:
  - Simple syntax extension (DSL-lite)
  - Variadic argument lists (vec![], format!-like macros)
  - Reducing repetition in patterns (matching on multiple types)
  - The transformation is purely structural / syntactic
  - You want zero dependencies

Use proc macros when:
  - Implementing derive traits (#[derive(Serialize)])
  - Framework attribute macros (#[route(GET "/")], #[tokio::test])
  - Type-aware code generation
  - Compile-time validation of external formats (SQL, regex, proto)
  - Heavy boilerplate reduction based on type structure

Use Go generate when:
  - You would use macros in other languages for boilerplate
  - String methods for enums (stringer)
  - Mock implementations of interfaces
  - Reading external schema files (proto, OpenAPI, graphql)
  - Any case where generics don't cover the needed abstraction
```

---

## 7. Advanced Patterns Across All Three Languages

### Pattern 1: Type-Safe Builder — Rust Proc Macro

Generating a builder pattern from a struct:

```rust
// Usage:
#[derive(Builder)]
pub struct ServerConfig {
    #[builder(required)]
    host: String,
    #[builder(default = "8080")]
    port: u16,
    #[builder(default = "30")]
    timeout_seconds: u64,
    #[builder(optional)]
    tls_cert_path: Option<String>,
}

// After macro expansion (equivalent to):
pub struct ServerConfigBuilder {
    host: Option<String>,
    port: Option<u16>,
    timeout_seconds: Option<u64>,
    tls_cert_path: Option<Option<String>>,
}

impl ServerConfigBuilder {
    pub fn new() -> Self {
        Self {
            host: None,
            port: Some(8080),
            timeout_seconds: Some(30),
            tls_cert_path: Some(None),
        }
    }
    pub fn host(mut self, v: String) -> Self { self.host = Some(v); self }
    pub fn port(mut self, v: u16) -> Self { self.port = Some(v); self }
    pub fn timeout_seconds(mut self, v: u64) -> Self { self.timeout_seconds = Some(v); self }
    pub fn tls_cert_path(mut self, v: Option<String>) -> Self { self.tls_cert_path = Some(v); self }
    
    pub fn build(self) -> Result<ServerConfig, String> {
        Ok(ServerConfig {
            host: self.host.ok_or("host is required")?,
            port: self.port.unwrap(),
            timeout_seconds: self.timeout_seconds.unwrap(),
            tls_cert_path: self.tls_cert_path.unwrap(),
        })
    }
}

impl ServerConfig {
    pub fn builder() -> ServerConfigBuilder {
        ServerConfigBuilder::new()
    }
}

// Proc macro implementation (proc-macro crate):
use proc_macro::TokenStream;
use syn::{parse_macro_input, DeriveInput, Data, Fields, Lit, Meta};
use quote::quote;

#[proc_macro_derive(Builder, attributes(builder))]
pub fn derive_builder(input: TokenStream) -> TokenStream {
    let input = parse_macro_input!(input as DeriveInput);
    let name = &input.ident;
    let builder_name = quote::format_ident!("{}Builder", name);

    let fields = match &input.data {
        Data::Struct(s) => match &s.fields {
            Fields::Named(f) => &f.named,
            _ => panic!("Only named fields supported"),
        },
        _ => panic!("Only structs supported"),
    };

    // Parse #[builder(...)] attributes on each field
    struct FieldInfo {
        name: syn::Ident,
        ty: syn::Type,
        required: bool,
        default: Option<String>,
        optional: bool,
    }

    let field_infos: Vec<FieldInfo> = fields.iter().map(|f| {
        let name = f.ident.clone().unwrap();
        let ty = f.ty.clone();
        let mut required = false;
        let mut default = None;
        let mut optional = false;

        for attr in &f.attrs {
            if !attr.path().is_ident("builder") { continue; }
            let _ = attr.parse_nested_meta(|meta| {
                if meta.path.is_ident("required") { required = true; }
                if meta.path.is_ident("optional") { optional = true; }
                if meta.path.is_ident("default") {
                    let value = meta.value()?;
                    let s: syn::LitStr = value.parse()?;
                    default = Some(s.value());
                }
                Ok(())
            });
        }

        FieldInfo { name, ty, required, default, optional }
    }).collect();

    // Generate builder struct fields
    let builder_fields = field_infos.iter().map(|fi| {
        let n = &fi.name;
        let t = &fi.ty;
        quote! { #n: Option<#t> }
    });

    // Generate default values in new()
    let field_defaults = field_infos.iter().map(|fi| {
        let n = &fi.name;
        let t = &fi.ty;
        if let Some(ref default_val) = fi.default {
            let default_expr: proc_macro2::TokenStream = default_val.parse().unwrap();
            quote! { #n: Some(#default_expr as #t) }
        } else if fi.optional {
            quote! { #n: Some(None) }
        } else {
            quote! { #n: None }
        }
    });

    // Generate setter methods
    let setters = field_infos.iter().map(|fi| {
        let n = &fi.name;
        let t = &fi.ty;
        quote! {
            pub fn #n(mut self, v: #t) -> Self {
                self.#n = Some(v);
                self
            }
        }
    });

    // Generate build() body
    let build_fields = field_infos.iter().map(|fi| {
        let n = &fi.name;
        let n_str = n.to_string();
        if fi.required {
            quote! { #n: self.#n.ok_or_else(|| format!("{} is required", #n_str))? }
        } else {
            quote! { #n: self.#n.unwrap() }
        }
    });

    let expanded = quote! {
        pub struct #builder_name {
            #(#builder_fields,)*
        }

        impl #builder_name {
            pub fn new() -> Self {
                Self {
                    #(#field_defaults,)*
                }
            }
            #(#setters)*
            pub fn build(self) -> Result<#name, String> {
                Ok(#name {
                    #(#build_fields,)*
                })
            }
        }

        impl #name {
            pub fn builder() -> #builder_name {
                #builder_name::new()
            }
        }
    };

    expanded.into()
}
```

### Pattern 2: State Machine DSL — Rust `macro_rules!`

A compile-time-verified state machine using `macro_rules!`:

```rust
macro_rules! state_machine {
    (
        name: $name:ident,
        state: $state_enum:ident { $($state:ident),+ },
        event: $event_enum:ident { $($event:ident),+ },
        transitions: {
            $($from:ident + $ev:ident => $to:ident),+ $(,)?
        },
        initial: $initial:ident
    ) => {
        #[derive(Debug, Clone, Copy, PartialEq, Eq)]
        pub enum $state_enum {
            $($state),+
        }

        #[derive(Debug, Clone, Copy, PartialEq, Eq)]
        pub enum $event_enum {
            $($event),+
        }

        pub struct $name {
            state: $state_enum,
        }

        impl $name {
            pub fn new() -> Self {
                Self { state: $state_enum::$initial }
            }

            pub fn state(&self) -> $state_enum {
                self.state
            }

            pub fn transition(&mut self, event: $event_enum) -> Result<$state_enum, String> {
                match (self.state, event) {
                    $(
                        ($state_enum::$from, $event_enum::$ev) => {
                            self.state = $state_enum::$to;
                            Ok(self.state)
                        }
                    )+
                    (s, e) => Err(format!("Invalid transition: {:?} + {:?}", s, e)),
                }
            }
        }
    };
}

// Usage:
state_machine! {
    name: TcpStateMachine,
    state: TcpState { Closed, Listen, SynSent, SynReceived, Established, FinWait1, Closed2 },
    event: TcpEvent { PassiveOpen, ActiveOpen, SynReceived, SynAck, Ack, Close, Fin },
    transitions: {
        Closed    + PassiveOpen  => Listen,
        Closed    + ActiveOpen   => SynSent,
        Listen    + SynReceived  => SynReceived,
        SynSent   + SynAck       => Established,
        SynReceived + Ack        => Established,
        Established + Close      => FinWait1,
        FinWait1  + Fin          => Closed2,
    },
    initial: Closed
}

let mut tcp = TcpStateMachine::new();
tcp.transition(TcpEvent::PassiveOpen).unwrap();
assert_eq!(tcp.state(), TcpState::Listen);
```

### Pattern 3: Generic Dispatch Table — C X-Macros

```c
/*
 * A complete, type-safe packet handler dispatch table generated with X-macros.
 * Adding a new protocol requires ONLY a new line in PROTOCOL_TABLE.
 */

#include <stdint.h>
#include <string.h>
#include <stdio.h>

/* Packet types with their EtherType and handler function names */
#define PROTOCOL_TABLE(X)                               \
    X(IP4,   0x0800, handle_ipv4,   "IPv4")            \
    X(IP6,   0x86DD, handle_ipv6,   "IPv6")            \
    X(ARP,   0x0806, handle_arp,    "ARP")             \
    X(VLAN,  0x8100, handle_vlan,   "VLAN 802.1Q")     \
    X(MPLS,  0x8847, handle_mpls,   "MPLS unicast")    \
    X(LLDP,  0x88CC, handle_lldp,   "LLDP")

/* 1. Generate enum */
typedef enum {
#define X(name, ethertype, handler, desc)  PROTO_ ## name,
    PROTOCOL_TABLE(X)
#undef X
    PROTO_UNKNOWN,
    PROTO_COUNT = PROTO_UNKNOWN
} Protocol;

/* 2. Forward declare handlers */
typedef struct Packet { uint8_t *data; size_t len; } Packet;
#define X(name, ethertype, handler, desc)  void handler(const Packet *pkt);
PROTOCOL_TABLE(X)
#undef X

/* 3. EtherType lookup table */
static const struct { uint16_t ethertype; Protocol proto; } ethertype_map[] = {
#define X(name, ethertype, handler, desc)  { ethertype, PROTO_ ## name },
    PROTOCOL_TABLE(X)
#undef X
    { 0, PROTO_UNKNOWN }
};

/* 4. Handler dispatch table */
static const void (*handler_table[])(const Packet *) = {
#define X(name, ethertype, handler, desc)  [PROTO_ ## name] = handler,
    PROTOCOL_TABLE(X)
#undef X
};

/* 5. Name table for logging */
static const char *proto_names[] = {
#define X(name, ethertype, handler, desc)  [PROTO_ ## name] = desc,
    PROTOCOL_TABLE(X)
#undef X
    [PROTO_UNKNOWN] = "Unknown"
};

/* 6. Lookup function */
Protocol ethertype_to_proto(uint16_t et) {
    for (int i = 0; ethertype_map[i].ethertype != 0; i++) {
        if (ethertype_map[i].ethertype == et)
            return ethertype_map[i].proto;
    }
    return PROTO_UNKNOWN;
}

/* 7. Dispatch function */
void dispatch_packet(const Packet *pkt, uint16_t ethertype) {
    Protocol proto = ethertype_to_proto(ethertype);
    if (proto != PROTO_UNKNOWN && handler_table[proto]) {
        fprintf(stderr, "[dispatch] %s (0x%04X)\n",
                proto_names[proto], ethertype);
        handler_table[proto](pkt);
    }
}

/* Stub implementations */
#define X(name, ethertype, handler, desc)              \
    void handler(const Packet *pkt) {                  \
        printf("Handling %s packet (%zu bytes)\n",     \
               desc, pkt->len);                        \
    }
PROTOCOL_TABLE(X)
#undef X
```

### Pattern 4: Compile-Time SQL Validation — Rust Proc Macro

```rust
// In sql-macro crate:
use proc_macro::TokenStream;
use syn::{LitStr, parse_macro_input};
use quote::quote;

#[proc_macro]
pub fn sql(input: TokenStream) -> TokenStream {
    let sql_str = parse_macro_input!(input as LitStr);
    let sql = sql_str.value();

    // Validate at compile time
    match validate_sql(&sql) {
        Ok(parsed) => {
            let col_names: Vec<String> = parsed.columns.clone();
            let param_count = parsed.param_count;
            quote! {
                {
                    // Embed metadata about the query
                    struct CompiledQuery {
                        sql: &'static str,
                        columns: &'static [&'static str],
                        param_count: usize,
                    }
                    CompiledQuery {
                        sql: #sql,
                        columns: &[#(#col_names),*],
                        param_count: #param_count,
                    }
                }
            }
        }
        Err(e) => {
            syn::Error::new(sql_str.span(), format!("SQL error: {}", e))
                .to_compile_error()
        }
    }.into()
}

// Usage:
let q = sql!("SELECT id, name, email FROM users WHERE id = $1");
// Compile error if SQL is malformed:
// let bad = sql!("SELCT * FORM users");  // ERROR at compile time
```

### Pattern 5: Interface Mock Generator — Go

```go
// cmd/mockgen/main.go — generates mock implementations

package main

import (
    "flag"
    "fmt"
    "go/ast"
    "go/parser"
    "go/token"
    "os"
    "strings"
    "text/template"
)

const mockTemplate = `// Code generated by mockgen; DO NOT EDIT.
package {{.Package}}

import (
    "sync"
    "fmt"
)

type Mock{{.Name}} struct {
    mu    sync.Mutex
    calls map[string][][]interface{}
    rets  map[string][]interface{}
}

func NewMock{{.Name}}() *Mock{{.Name}} {
    return &Mock{{.Name}}{
        calls: make(map[string][][]interface{}),
        rets:  make(map[string][]interface{}),
    }
}

func (m *Mock{{.Name}}) On(method string, returns ...interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.rets[method] = returns
}

func (m *Mock{{.Name}}) CallsTo(method string) int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.calls[method])
}

{{range .Methods}}
func (m *Mock{{$.Name}}) {{.Name}}({{.ParamsStr}}) ({{.ReturnsStr}}) {
    m.mu.Lock()
    args := []interface{}{ {{range .Params}}{{.Name}}, {{end}} }
    m.calls[{{printf "%q" .Name}}] = append(m.calls[{{printf "%q" .Name}}], args)
    rets := m.rets[{{printf "%q" .Name}}]
    m.mu.Unlock()

    {{range $i, $r := .Returns}}
    if len(rets) > {{$i}} {
        if v, ok := rets[{{$i}}].({{$r.Type}}); ok {
            {{$r.VarName}} = v
        }
    }
    {{end}}
    return {{range $i, $r := .Returns}}{{if $i}}, {{end}}{{$r.VarName}}{{end}}
}
{{end}}
`

type Param struct {
    Name string
    Type string
}

type Return struct {
    VarName string
    Type    string
}

type Method struct {
    Name       string
    Params     []Param
    Returns    []Return
    ParamsStr  string
    ReturnsStr string
}

type MockData struct {
    Package string
    Name    string
    Methods []Method
}

func main() {
    src := flag.String("source", "", "source file")
    iface := flag.String("interface", "", "interface name")
    flag.Parse()

    fset := token.NewFileSet()
    f, _ := parser.ParseFile(fset, *src, nil, 0)

    var data MockData
    data.Package = f.Name.Name

    ast.Inspect(f, func(n ast.Node) bool {
        typeSpec, ok := n.(*ast.TypeSpec)
        if !ok || typeSpec.Name.Name != *iface {
            return true
        }
        ifaceType, ok := typeSpec.Type.(*ast.InterfaceType)
        if !ok {
            return true
        }

        data.Name = typeSpec.Name.Name
        for i, method := range ifaceType.Methods.List {
            funcType, ok := method.Type.(*ast.FuncType)
            if !ok {
                continue
            }
            m := Method{Name: method.Names[0].Name}

            // Collect params
            if funcType.Params != nil {
                for j, param := range funcType.Params.List {
                    typeName := fmt.Sprintf("%v", param.Type)
                    for _, pname := range param.Names {
                        m.Params = append(m.Params, Param{
                            Name: pname.Name,
                            Type: typeName,
                        })
                    }
                    _ = j
                }
            }

            // Collect returns
            if funcType.Results != nil {
                for k, result := range funcType.Results.List {
                    typeName := fmt.Sprintf("%v", result.Type)
                    varName := fmt.Sprintf("ret%d", k)
                    m.Returns = append(m.Returns, Return{
                        VarName: varName,
                        Type:    typeName,
                    })
                }
            }

            // Build param/return strings
            var params []string
            for _, p := range m.Params {
                params = append(params, fmt.Sprintf("%s %s", p.Name, p.Type))
            }
            m.ParamsStr = strings.Join(params, ", ")

            var rets []string
            for _, r := range m.Returns {
                rets = append(rets, fmt.Sprintf("%s %s", r.VarName, r.Type))
            }
            m.ReturnsStr = strings.Join(rets, ", ")
            data.Methods = append(data.Methods, m)
            _ = i
        }
        return true
    })

    tmpl := template.Must(template.New("mock").Parse(mockTemplate))
    tmpl.Execute(os.Stdout, data)
}
```

### Pattern 6: Recursive Macro for Nested Data — Rust `macro_rules!`

Building deeply nested JSON-like structures with a recursive macro:

```rust
macro_rules! json {
    // null
    (null) => {
        JsonValue::Null
    };
    // boolean
    (true) => { JsonValue::Bool(true) };
    (false) => { JsonValue::Bool(false) };
    // number
    ($n:literal) => {
        JsonValue::Number($n as f64)
    };
    // string
    ($s:literal) => {
        JsonValue::Str($s.to_string())
    };
    // array
    ([$($elem:tt),* $(,)?]) => {
        JsonValue::Array(vec![$(json!($elem)),*])
    };
    // object
    ({$($key:literal : $val:tt),* $(,)?}) => {
        {
            let mut map = std::collections::HashMap::new();
            $(map.insert($key.to_string(), json!($val));)*
            JsonValue::Object(map)
        }
    };
    // variable interpolation
    ($var:expr) => {
        JsonValue::from($var)
    };
}

#[derive(Debug)]
enum JsonValue {
    Null,
    Bool(bool),
    Number(f64),
    Str(String),
    Array(Vec<JsonValue>),
    Object(std::collections::HashMap<String, JsonValue>),
}

// Usage:
let doc = json!({
    "name": "Alice",
    "age": 30,
    "active": true,
    "scores": [95, 87, 92],
    "address": {
        "city": "Wonderland",
        "zip": null
    }
});
```

---

## Summary: Core Mental Models

```
C Preprocessor
══════════════
1. Runs BEFORE the compiler sees your code.
2. Operates on TEXT/TOKENS — no types, no AST, no scopes.
3. Rescanning rule: substituted tokens are rescanned; self-references are "painted blue" to stop infinite loops.
4. Hygiene is YOUR problem — wrap in parens, use do-while, avoid side effects in args.
5. Power moves: X-macros for dispatch tables, conditional compilation for portability.
6. Gold standard tools: #define constants, include guards, conditional compilation, X-macros.
7. Avoid for: type-sensitive operations, complex logic (use inline functions instead).


macro_rules! (Rust)
═══════════════════
1. Runs AFTER tokenization, BEFORE parsing — operates on TOKEN TREES.
2. Pattern match on token tree structure → produce token tree.
3. Hygienic by default — variables introduced by macro are invisible at call site.
4. Fragment specifiers (expr, ty, ident, tt, ...) constrain input AND output context.
5. Repetitions ($(...)*) enable variadic behavior.
6. TT muncher + push-down accumulator = arbitrary DSL parsing.
7. @rule convention marks internal arms.
8. Gold standard tools: vec![], format!-like macros, simple DSLs, variadic helpers.
9. Avoid for: type inspection, file I/O, complex logic — use proc macros instead.


Proc Macros (Rust)
══════════════════
1. Full Rust programs that run AT COMPILE TIME as compiler plugins.
2. Input: TokenStream (from your code) → Output: TokenStream (injected back).
3. Three kinds: derive (adds impls), attribute (replaces items), function-like (replaces call).
4. Use syn to PARSE TokenStream into typed AST; use quote to GENERATE TokenStream.
5. Errors with spans give IDE-quality messages pointing to the right source location.
6. Lives in separate crate (proc-macro = true), compiled for host.
7. Gold standard tools: #[derive(Serialize/Deserialize)], #[tokio::main], #[test], builders.
8. Cost: separate crate, slightly longer compile times.


go:generate (Go)
════════════════
1. Metaprogramming is EXTERNAL — run a program, get .go files, compile those.
2. //go:generate <command> is a comment directive, not syntax.
3. Generated files are committed to version control — fully auditable.
4. go/ast + go/types = inspect existing code; text/template = generate new code.
5. Generics (1.18+) eliminate most of the "I wish I had macros" cases.
6. Build tags (//go:build) handle conditional compilation.
7. Gold standard tools: stringer, protoc, mockgen, sqlc, ent.
8. Philosophy: explicit > magical; readable > concise; auditable > automatic.
```

---

*This guide covers all major aspects of macro systems in C, Rust, and Go. Mastering these systems gives you precise control over code generation, zero-cost abstractions, and the mental model to know exactly what your compiler sees at each stage.*

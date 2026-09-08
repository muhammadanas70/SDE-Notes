# A Deep Dive into Function Types in Rust, C, and Go

This guide explores every facet of what we call a “function” across three languages: **Rust**, **C**, and **Go**.  
We will cover syntactic forms, type systems, memory representations, calling conventions, and the mental models that allow you to reason efficiently about code.  
Diagrams are given as ASCII art so you can always keep the architecture in view.

---

## Table of Contents
1. [Fundamental Concepts](#fundamental-concepts)
2. [Rust](#rust)
    - [Named Functions (fn Items)](#named-functions-fn-items)
    - [Function Pointers](#function-pointers-rust)
    - [Closures and the Fn* Traits](#closures-and-the-fn-traits)
    - [Methods and Associated Functions](#methods-and-associated-functions)
    - [Generic Functions and Const Generics](#generic-functions-and-const-generics)
    - [Diverging Functions (Never Type `!`)](#diverging-functions)
    - [Unsafe Functions](#unsafe-functions)
    - [Extern Functions and ABIs](#extern-functions-and-abis)
    - [Async Functions](#async-functions)
    - [Const Functions](#const-functions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-rust)
3. [C](#c)
    - [Function Definitions and Declarations](#function-definitions-and-declarations)
    - [Function Pointers](#function-pointers-c)
    - [Variadic Functions](#variadic-functions)
    - [Inline Functions](#inline-functions)
    - [Static and Extern Linkage](#static-and-extern-linkage)
    - [Nested Functions (GNU C)](#nested-functions-gnu-c)
    - [Signal Handlers and Callbacks](#signal-handlers-and-callbacks)
    - [`_Noreturn` and `noreturn`](#noreturn)
    - [Old-Style (K&R) Definitions](#old-style-kr-definitions)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-c)
4. [Go](#go)
    - [Named Functions](#named-functions-go)
    - [Anonymous Functions and Closures](#anonymous-functions-and-closures-go)
    - [Methods and Method Values/Expressions](#methods-and-method-valuesexpressions)
    - [Variadic Functions](#variadic-functions-go)
    - [Function Types and First-Class Citizens](#function-types-and-first-class-citizens)
    - [Multiple Return Values and Named Results](#multiple-return-values-and-named-results)
    - [Deferred Functions, Panic, and Recover](#deferred-functions-panic-and-recover)
    - [Init Functions](#init-functions)
    - [Built-in Functions](#built-in-functions)
    - [Generic Functions (Go 1.18+)](#generic-functions-go-1.18)
    - [Memory Layout and Call Architecture (ASCII)](#memory-layout-and-call-architecture-go)
5. [Comparative Summary](#comparative-summary)

---

## Fundamental Concepts
A **function** maps a tuple of arguments to a result, possibly modifying state. Each language gives us different tools to abstract and pass this mapping around.

Key dimensions:
- **Named vs anonymous** – can the function be referred to by an identifier?
- **First‑class vs second‑class** – can it be stored, passed, returned?
- **Closure vs plain** – does it capture an environment?
- **Method vs free function** – is it associated with a type?
- **Linkage / visibility** – static, extern, public/private.
- **Special modifiers** – inline, const, async, unsafe, variadic, etc.

We will explore all of these for each language.

---

## Rust

Rust’s function system is expression‑oriented and heavily integrated with ownership and traits.  
A “function” can mean a **fn item**, a **function pointer**, or a **closure**. They are distinct types.

### Named Functions (fn Items)
```rust
fn add(x: i32, y: i32) -> i32 {
    x + y
}
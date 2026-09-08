# Function Pointer Syntax — Complete Clarity Guide
# C · Go · Rust

> This guide is ONLY about syntax. Every construct is dissected character by
> character, every rule is stated explicitly, and every confusion is addressed
> head-on with before/after comparisons.

---

## Table of Contents

1. [C Syntax](#1-c-syntax)
   - 1.1 The One Rule That Explains Everything
   - 1.2 Reading Any C Declaration (the cdecl algorithm)
   - 1.3 Every Component Labelled
   - 1.4 typedef — Syntax Transformation
   - 1.5 Parameter Lists — What Goes Inside the Parentheses
   - 1.6 Arrays of Function Pointers
   - 1.7 Functions That Return Function Pointers
   - 1.8 Pointers to Functions That Return Pointers
   - 1.9 The Four Lookalike Declarations You Must Not Confuse
   - 1.10 How to Write Any Declaration from Scratch (the reverse algorithm)
   - 1.11 Quick-Reference Card
2. [Go Syntax](#2-go-syntax)
   - 2.1 Function Type Syntax
   - 2.2 Named Function Types
   - 2.3 Closure / Anonymous Function Syntax
   - 2.4 Passing and Returning Functions
   - 2.5 Method Values vs Method Expressions
   - 2.6 Nil and the Zero Value
   - 2.7 Quick-Reference Card
3. [Rust Syntax](#3-rust-syntax)
   - 3.1 The Four Callable Forms
   - 3.2 Bare Function Pointer `fn`
   - 3.3 Closure Syntax — Every Piece Explained
   - 3.4 Trait Bounds: `Fn`, `FnMut`, `FnOnce`
   - 3.5 `impl Fn` vs `dyn Fn` vs `fn` — The Critical Distinction
   - 3.6 Accepting a Callable as a Parameter
   - 3.7 Returning a Callable from a Function
   - 3.8 `move` Keyword Placement
   - 3.9 `extern "C"` Function Pointers
   - 3.10 `Option<fn>` — Nullable Function Pointers
   - 3.11 Quick-Reference Card
4. [Side-by-Side Cheat Sheet](#4-side-by-side-cheat-sheet)

---

## 1. C Syntax

### 1.1 The One Rule That Explains Everything

C declarations are **read from the variable name outward** using this priority:

```
Priority (highest → lowest):
  1. Parentheses ()    — function call or grouping
  2. Brackets   []    — array subscript
  3. Asterisk   *     — pointer
```

When you see `*` next to a name, ask: "Is there a `()` on the right side too?"
If yes, `()` wins — it is a *function*, not a pointer. To force *pointer* to win,
wrap `*name` in its own parentheses.

```
   int  *f  (int)         — f is a FUNCTION (int) returning int*
         ↑ () on right beats * on left

   int  (*f) (int)        — f is a POINTER to function (int) returning int
        ↑↑ parentheses force * to be read first
```

This single rule is the source of all C function-pointer syntax.

---

### 1.2 Reading Any C Declaration (the cdecl Algorithm)

Follow these four steps in order for **any** declaration, no matter how complex.

```
Step 1: Find the variable name.
Step 2: Look RIGHT — if you see () or [], consume them.
Step 3: Look LEFT  — if you see *, consume it ("pointer to").
Step 4: Repeat steps 2–3 until you hit a type keyword or an outer paren.
        If you hit an outer paren, exit it and continue outward.
```

Let us trace every example below using this algorithm.

**Example A — simple function pointer:**

```
  int  (*fp)  (int, double)
       ↑
  Step 1: name is fp

  Step 2: look right of fp  →  we see )  — stop, hit closing paren
  Step 3: look left of fp   →  we see *  — "fp is a pointer to…"

  Now exit the parentheses and continue outward:
  Step 2: look right        →  we see (int, double)  — "function taking (int, double)…"
  Step 3: look left         →  we see int             — "returning int"

  Final reading:
  "fp is a pointer to a function taking (int, double) returning int"
```

**Example B — array of function pointers:**

```
  void  (*arr[4])  (int)
         ↑
  Step 1: name is arr

  Step 2: look right → [4]   — "arr is an array of 4…"
  Step 3: look left  → *     — "…pointers to…"

  Exit parentheses:
  Step 2: look right → (int) — "…functions taking (int)…"
  Step 3: look left  → void  — "…returning void"

  Final reading:
  "arr is an array of 4 pointers to functions taking (int) returning void"
```

**Example C — function returning a function pointer:**

```
  int  (*make_op(char))  (int, int)
           ↑
  Step 1: name is make_op

  Step 2: look right → (char)  — "make_op is a function taking (char)…"
  Step 3: look left  → *       — "…returning a pointer to…"

  Exit parentheses:
  Step 2: look right → (int, int)  — "…a function taking (int, int)…"
  Step 3: look left  → int         — "…returning int"

  Final reading:
  "make_op is a function taking (char) returning a pointer to
   a function taking (int, int) returning int"
```

---

### 1.3 Every Component Labelled

Below every character of a function pointer declaration is annotated.

```
     ┌─ return type of the pointed-to function
     │
     │           ┌─ grouping parens: REQUIRED to make * bind to fp, not to int
     │           │  without them this would be "function returning int*"
     │           │
     │       ┌───┴────┐
     │       │        │         ┌─ parameter list of the pointed-to function
     │       │        │         │
     ▼       ▼        ▼         ▼
    int     (*       fp    )  (int  a,  double  b)
                     ▲                   ▲
                     │                   └─ parameter names (optional in declarations)
                     └─ variable name: this is where you name the pointer
```

Another angle — which parentheses do what:

```
    int   (*fp)   (int, double)
          ───┬──  ──────┬──────
             │          │
             │          └── "call signature" parens — describe what the function
             │               pointed-to takes as arguments
             │
             └── "grouping" parens — force * to bind to fp, not to int
                  Without these:  int *fp(int,double)  = function returning int*
                  With these:     int (*fp)(int,double) = pointer to function
```

---

### 1.4 typedef — Syntax Transformation

The `typedef` keyword turns a variable declaration into a **type alias**. The
rule is simple: whatever you would write for a variable, write the same thing,
but prepend `typedef` and the alias name replaces the variable name.

```
WITHOUT typedef (variable declaration):

    int   (*fp)   (int, int);
           ↑
        variable name

WITH typedef (type alias):

    typedef   int   (*BinaryOp)   (int, int);
                             ↑
                        alias name goes where the variable name was
                        Now BinaryOp IS the type.
```

Transformation diagram:

```
    VARIABLE DECLARATION              TYPE ALIAS (typedef)
    ──────────────────────            ─────────────────────────────────
    int (*fp)(int,int);               typedef int (*BinaryOp)(int,int);
         ↑                                          ↑
    "fp" is a variable                   "BinaryOp" is now a type name

    Usage of the type:
         BinaryOp fp;                 ← same as: int (*fp)(int,int);
         BinaryOp ops[4];             ← array of 4
         BinaryOp get_op(char);       ← function returning BinaryOp
```

**The typedef syntax for every pattern:**

```
Pattern                     Raw declaration              typedef form
───────────────────────────────────────────────────────────────────────────────

Simple fn ptr               int (*fp)(int,int)           typedef int (*Op)(int,int);

No params, no return        void (*fp)(void)             typedef void (*Action)(void);

Returning a pointer         char *(*fp)(char *)          typedef char *(*StrFn)(char *);

Taking a fn ptr param       void (*fp)(int (*)(int))     typedef int  (*IntFn)(int);
                                                         typedef void (*FnFn)(IntFn);

Array[N] of fn ptrs         int (*arr[4])(int)           typedef int  (*Op)(int);
                                                         Op arr[4];

Fn returning fn ptr         int (*fn(char))(int,int)     typedef int (*Op)(int,int);
                                                         Op fn(char);
```

---

### 1.5 Parameter Lists — What Goes Inside the Parentheses

The **call-signature parens** `(…)` describe the pointed-to function's parameters.

```
    int  (*fp)  (void)              — takes NO arguments  (explicit void)
    int  (*fp)  ()                  — takes UNSPECIFIED args (old C, avoid)
    int  (*fp)  (int)               — takes one int
    int  (*fp)  (int, double)       — takes int, then double
    int  (*fp)  (int a, double b)   — same; names are optional in the type
    int  (*fp)  (const char *)      — takes a const char pointer
    int  (*fp)  (int, ...)          — variadic (takes int, then anything)

Calling conventions (Windows / embedded):

    int  (__cdecl   *fp)(int)       — cdecl (default on x86)
    int  (__stdcall *fp)(int)       — stdcall (WinAPI)
    int  (__fastcall *fp)(int)      — fastcall
```

---

### 1.6 Arrays of Function Pointers

An array of function pointers holds `[N]` between the name and the `*`:

```
    int  (*arr[4])  (int)
             ↑
             └── [4] sits BETWEEN the name and the outer paren

    Reading (cdecl algorithm):
    Step 1: name = arr
    Step 2: look right → [4]   → "arr is an array of 4…"
    Step 3: look left  → *     → "…pointers to…"
    exit parens:
    Step 2: look right → (int) → "…functions taking (int)…"
    Step 3: look left  → int   → "…returning int"

    ↓ With typedef ↓

    typedef int (*Op)(int);
    Op arr[4];              ← cleaner, identical meaning
```

2D array:

```
    int  (*matrix[3][4])  (int)     — 3×4 array of fn pointers
             ↑↑
             └┴── [3][4] sits between name and outer paren
```

---

### 1.7 Functions That Return Function Pointers

The pattern for a function that **returns** a function pointer wraps the
function's own call signature inside the outer parens:

```
    int  (*make_op  (char op))  (int, int)
              ↑↑↑↑↑↑↑↑↑↑↑
              make_op(char op) — this is the outer function's signature
          ↑                              ↑
          └── * and return type are wrapping it on both sides


    Labelled:
    ┌── return type of inner function
    │
    │        ┌── * (the returned pointer)
    │        │
    │        │   ┌── outer function name + its own params
    │        │   │
    │        │   │                ┌── inner function's params
    │        │   │                │
    ▼        ▼   ▼────────────┐   ▼────────┐
    int     (*  make_op   (char op) )  (int, int)


    With typedef (MUCH cleaner):

    typedef int (*BinaryOp)(int, int);   ← name the return type
    BinaryOp make_op(char op);           ← now the declaration is readable
```

---

### 1.8 Pointers to Functions That Return Pointers

When the *pointed-to function itself* returns a pointer, the return-type slot
gains its own `*`:

```
    char  *(*fp)  (const char *)
    ────┬──
        └── the return type is char* (a pointer to char)
            The pointed-to function returns char*

    int  **(*fp)  (void)
    ─────┬──
         └── return type is int** (pointer to pointer to int)

    void *(*fp)(size_t)           — like malloc: takes size_t, returns void*
```

---

### 1.9 The Four Lookalike Declarations You Must Not Confuse

These four look similar but mean completely different things.

```
    ┌────────────────────────────────────────────────────────────────────────┐
    │  Declaration              What it actually is                          │
    ├────────────────────────────────────────────────────────────────────────┤
    │                                                                        │
    │  int  *fp (int)           FUNCTION named fp                           │
    │                           • takes int                                  │
    │                           • returns int* (pointer to int)              │
    │                           • NOT a pointer; fp is a function           │
    │                                                                        │
    │  int  (*fp)(int)          POINTER named fp                            │
    │                           • points to a function taking int            │
    │                           • that function returns int                  │
    │                           • fp IS a pointer variable                  │
    │                                                                        │
    │  int  *(*fp)(int)         POINTER named fp                            │
    │                           • points to a function taking int            │
    │                           • that function returns int*                 │
    │                                                                        │
    │  int  (*fp[4])(int)       ARRAY named fp                              │
    │                           • array of 4 pointers                        │
    │                           • each pointer → function(int)->int         │
    │                                                                        │
    └────────────────────────────────────────────────────────────────────────┘

    Memory usage:
    int *fp(int)       — fp is a function; lives in the text segment
    int (*fp)(int)     — fp is a pointer; sizeof(fp) == 8 on 64-bit
    int (*fp[4])(int)  — fp is an array; sizeof(fp) == 32 on 64-bit

    Test: is there a * before the name AND parentheses around it?
          YES → pointer to function
          NO  → function (that may return a pointer)
```

---

### 1.10 How to Write Any Declaration from Scratch

Reverse of the reading algorithm — build the declaration from the English:

```
"Write a variable op that holds a pointer to a
 function taking (int, int) and returning double"

Step 1: Write the variable name:
            op

Step 2: Add the pointer:
            *op
        It is a pointer, so wrap in parens to force binding:
            (*op)

Step 3: Add the pointed-to function's parameter list on the RIGHT:
            (*op)(int, int)

Step 4: Add the return type on the LEFT:
            double (*op)(int, int)

Done. ✓

─────────────────────────────────────────────────────────────────

"Write a variable get that holds a pointer to a
 function taking (char) returning a pointer to
 a function taking (int) returning int"

Step 1: name:                           get
Step 2: outermost pointer:              (*get)
Step 3: outer function's params:        (*get)(char)
Step 4: result so far is pointer to:    int (*get_result)(int)
        but get_result is not a name here;
        the whole (*get)(char) plays the role of the name:
            int (*  (*get)(char)  )(int)
Step 5: outer return type on left (int already there): done

    int  (*(*get)(char))  (int)

With typedef:
    typedef int (*InnerFn)(int);
    InnerFn (*get)(char);          ← much cleaner
```

---

### 1.11 C Quick-Reference Card

```
    FORM                            MEANING
    ────────────────────────────────────────────────────────────────────────

    int (*fp)(int, int)             pointer to function(int,int)->int
    void (*fp)(void)                pointer to function()->void
    char *(*fp)(char *)             pointer to function(char*)->char*
    int (*fp)(int, ...)             pointer to variadic function
    void (*fp[N])(int)              array[N] of pointers to function(int)->void
    int (*fn(char))(int,int)        function(char) returning ptr to function(int,int)->int

    TYPEDEF FORMS
    ────────────────────────────────────────────────────────────────────────

    typedef int (*Op)(int,int);
    Op fp;                          same as: int (*fp)(int,int)
    Op arr[4];                      array of 4 Op
    Op fn(char c);                  function(char) returning Op

    ASSIGNMENT
    ────────────────────────────────────────────────────────────────────────

    fp = some_function;             assign (function decays to pointer)
    fp = &some_function;            identical to above
    fp = NULL;                      null (no function)

    CALLING
    ────────────────────────────────────────────────────────────────────────

    fp(a, b)                        modern style (implicit deref)
    (*fp)(a, b)                     explicit deref style (identical result)

    GUARD
    ────────────────────────────────────────────────────────────────────────

    if (fp != NULL) fp(a, b);       always guard before calling
    if (fp) fp(a, b);               shorthand (same thing)
```

---

## 2. Go Syntax

### 2.1 Function Type Syntax

A function type in Go is written exactly like a function signature but **without
a name**:

```
    func  keyword → not a declaration; this is the TYPE
      │
      ▼
    func (int, string) (bool, error)
         ──────┬──────  ─────┬──────
               │             │
          parameter      return types
          types           (multiple allowed)

    Simpler forms:
    func()                       — no params, no return
    func(int) int                — one param, one return
    func(int, int) int           — two params, one return
    func(string) (int, error)    — one param, two returns
    func(...int) int             — variadic
```

**Variable declarations:**

```
    var f   func(int, int) int       — declares f with zero value nil
        ↑   ───────────────────
        │   the type: func(int,int)int
        variable name

    f := add                         — short form; type inferred

    f := func(a, b int) int {        — anonymous function assigned directly
        return a + b
    }
```

**Parameter names are optional in function types:**

```
    func(int, int) int               — types only (in type position)
    func(a, b int) int               — same type; names don't matter to the type
    func(a int, b int) int           — same type; expanded form
```

---

### 2.2 Named Function Types

A named type is created with `type`. It makes the type reusable and allows
methods to be defined on it.

```
    type  BinaryOp  func(int, int) int
          ────┬────  ────────────────────
              │      the underlying function type
              └── the new type name

    Usage:
    var op BinaryOp = add            — variable of named type
    ops := []BinaryOp{add, sub}      — slice of named type
    func (f BinaryOp) String() string { … }  — method on the named type
```

**Calling a value of named function type:**

```
    var op BinaryOp = add
    result := op(3, 4)               — same syntax as calling any function
                                       op is called like a normal function
```

---

### 2.3 Closure / Anonymous Function Syntax

An anonymous function is a `func` literal. It has the same syntax as a regular
function declaration but **without a name after the `func` keyword**:

```
NAMED function:          func  add  (a, b int) int  { return a + b }
                               ↑ name
ANONYMOUS function:      func         (a, b int) int  { return a + b }
                               ↑ no name here


    ANATOMY OF AN ANONYMOUS FUNCTION:

    func  (a, b int) int  {  return a + b  }
    ─┬──  ─────┬────  ─┬─   ───────────────
     │         │       │         │
     │    parameter    │       body
     │    list        return
     │                type(s)
     │
     keyword `func` begins the literal


    IMMEDIATELY INVOKED (IIFE):

    result := func(x int) int { return x * x }(5)
                                                ↑
                                         argument goes here, outside the braces
                                         This calls the function immediately
                                         result == 25

    ASSIGNED TO VARIABLE:

    square := func(x int) int { return x * x }
    result := square(5)    — called later

    INLINE AS ARGUMENT:

    sort.Slice(data, func(i, j int) bool {
                         return data[i] < data[j]
                     })
                     ↑ the closing ) of sort.Slice is AFTER the braces
```

---

### 2.4 Passing and Returning Functions

**Passing a function as a parameter:**

```
    func apply(f func(int) int, x int) int {
               ─────────────────
               parameter f has type func(int) int
               call it like: f(x)
    }

    apply(double, 5)      — pass named function
    apply(func(x int) int { return x * x }, 5)  — pass anonymous function
```

**Returning a function:**

```
    func makeAdder(base int) func(int) int {
                             ─────────────
                             return type: func(int) int
                             you return a function value

        return func(x int) int {
                   return base + x   — captures `base` from outer scope
               }
    }

    add5 := makeAdder(5)     — add5 has type func(int) int
    add5(3)                  — returns 8
```

**Multiple levels:**

```
    func outer() func() func() string {
                 ─────────────────────
                 return type: func() func() string
                   — a function that returns a function that returns string

        return func() func() string {
            return func() string {
                return "deep"
            }
        }
    }

    f := outer()    — f is func() func() string
    g := f()        — g is func() string
    s := g()        — s is "deep"
    — or: outer()()() == "deep"
```

---

### 2.5 Method Values vs Method Expressions

This is a common source of confusion in Go.

```
    type Rect struct { W, H float64 }
    func (r Rect) Area() float64 { return r.W * r.H }

    r := Rect{3, 4}

    ─── METHOD VALUE (bound to a specific receiver) ──────────────────────

    f := r.Area
         ──────
         syntax: instance.MethodName  (no call parens!)
         type:   func() float64       (receiver is already bound)
         call:   f()                  (no receiver needed)

    ─── METHOD EXPRESSION (receiver is the first parameter) ──────────────

    g := Rect.Area
         ─────────
         syntax: TypeName.MethodName  (capital T, type not instance)
         type:   func(Rect) float64   (receiver becomes first param)
         call:   g(r)                 (must supply receiver explicitly)

    ─── SIDE BY SIDE ─────────────────────────────────────────────────────

    r.Area       → type func() float64        → call: f()
    Rect.Area    → type func(Rect) float64    → call: g(r)

    The key: if it starts with a lowercase instance → method VALUE
             if it starts with an uppercase type   → method EXPRESSION
```

---

### 2.6 Nil and the Zero Value

```
    var f func(int) int          — f is nil (zero value of any func type)

    f == nil                     — true: f has not been assigned
    f(5)                         — PANIC: call of nil function

    Always guard optional function values:
    if f != nil {
        f(5)
    }

    Note: you cannot compare function values with ==
          (except to nil)
    f == g    — COMPILE ERROR (not allowed in Go)
    f == nil  — OK
```

---

### 2.7 Go Quick-Reference Card

```
    TYPE SYNTAX
    ─────────────────────────────────────────────────────────────────────────
    func()                       no params, no return
    func(int) int                one param, one return
    func(int, int) int           two params, one return
    func(string) (int, error)    two returns
    func(...int) int             variadic
    type F func(int) int         named type alias

    VARIABLE DECLARATION
    ─────────────────────────────────────────────────────────────────────────
    var f func(int) int          nil (zero value)
    f := add                     inferred type
    f := func(x int) int { … }  anonymous function
    f = nil                      reset to nil

    CALL
    ─────────────────────────────────────────────────────────────────────────
    f(arg)                       call through variable
    f(arg1, arg2)                multiple args
    f(slice...)                  spread slice into variadic

    METHOD VALUE vs METHOD EXPRESSION
    ─────────────────────────────────────────────────────────────────────────
    r.Method        → func(...) R         receiver r bound; call: f()
    Type.Method     → func(Type,...) R    receiver is first param; call: f(r)

    PASS / RETURN
    ─────────────────────────────────────────────────────────────────────────
    func take(f func(int) int)      parameter
    func give() func(int) int       return value
    []func(int) int                 slice of function values
    map[string]func(int) int        map of function values
```

---

## 3. Rust Syntax

### 3.1 The Four Callable Forms

Rust has four distinct syntactic forms for callable things. Knowing when to write
each one is the entire battle.

```
    Form                Example                     When to write it
    ─────────────────────────────────────────────────────────────────────────
    fn pointer          fn(i32) -> i32              Variable holding a fn address.
                                                    No captures allowed.
                                                    Always 8 bytes, always Copy.

    Closure             |x: i32| x + 1             Inline callable, may capture.
    (no trait written)                              Used as values, arguments, etc.

    impl Fn             impl Fn(i32) -> i32         Function/method PARAMETER type.
                                                    Static dispatch. Zero overhead.
                                                    Also used as RETURN type (→ static).

    dyn Fn              dyn Fn(i32) -> i32          Type-erased callable. Must be
                        Box<dyn Fn(i32)->i32>       behind a pointer (& or Box).
                                                    Dynamic dispatch (vtable).
```

---

### 3.2 Bare Function Pointer `fn`

```
    fn  (i32, i32)  ->  i32
    ─┬─  ─────┬────     ─┬─
     │         │          └── return type (after ->)
     │    parameter types
     keyword `fn`

    FULL LABELLED DIAGRAM:

         ┌── keyword (lowercase fn, NOT Fn the trait)
         │
         │            ┌── parameter types (no names, no mut)
         │            │
         │            │             ┌── return type
         │            │             │   (omit if ())
         ▼            ▼             ▼
        fn   (i32,  i32)   ->    i32
                                ──┬──
                                  └── if returning unit (nothing): omit -> i32
                                      or write -> ()   (they are the same)

    VARIABLE DECLARATIONS:

    let f: fn(i32) -> i32 = double;     — explicit type
    let f = double as fn(i32) -> i32;   — explicit cast (coercion)
    let f: fn(i32, i32) -> i32 = add;   — two params

    IN A STRUCT:
    struct Dispatch {
        op: fn(i32, i32) -> i32,        — field of function pointer type
    }

    IN AN ARRAY:
    let ops: [fn(i32, i32) -> i32; 3] = [add, sub, mul];

    CALLING:
    let result = f(3, 4);               — same syntax as a direct function call
    let result = (f)(3, 4);            — also valid (parens around f)
```

---

### 3.3 Closure Syntax — Every Piece Explained

```
    | x: i32, y: i32 |  ->  i32   {  x + y  }
    ─────────┬─────────  ────┬───    ────┬────
             │               │           └── body (expression or block)
             │               └── return type (OPTIONAL; usually inferred)
             └── parameter list between PIPES |…|
                 — type annotations optional when inferable


    ANATOMY:

                 ┌────── opening pipe
                 │                     ┌── closing pipe
                 │                     │
                 ▼                     ▼
                 |  x: i32 ,  y: i32  |  { x + y }
                    ──┬───    ──┬───
                      │         └── second param with optional type
                      └── first param with optional type

    FORMS (all equivalent if types are inferable):

    |x: i32, y: i32| -> i32 { x + y }   — fully annotated
    |x, y| -> i32 { x + y }              — types inferred
    |x, y| { x + y }                     — return type inferred
    |x, y| x + y                          — body is a single expression (no braces)

    SINGLE PARAM:
    |x| x * 2                             — one param, expression body
    |x: i32| x * 2                        — with type annotation
    |x: i32| -> i32 { x * 2 }            — fully explicit

    NO PARAMS:
    || 42                                  — no params, returns 42
    || { println!("hi"); }                — no params, unit return
    || -> i32 { 42 }                      — with explicit return type

    MULTI-STATEMENT BODY (needs braces):
    |x| {
        let y = x * 2;
        y + 1              — last expression is the return value
    }

    EXPLICIT return KEYWORD:
    |x| {
        if x > 0 { return x; }   — early return inside closure needs `return`
        -x
    }
```

**Where `move` goes:**

```
    move  |x| x + captured_var
    ─┬──  ─────────────────────
     │    the rest of the closure (params + body)
     │
     └── `move` comes BEFORE the | pipes, AFTER nothing else
         It forces all captures to be by value (owned)

    move || println!("{}", s)    — no params, move capture
    move |x| x + n              — one param, move capture
```

---

### 3.4 Trait Bounds: `Fn`, `FnMut`, `FnOnce`

These are **traits**, not types. They appear in angle brackets `<>` or `where`
clauses, and in `impl` / `dyn` position.

```
    SPELLING:  Fn  FnMut  FnOnce   (capital F, these are trait names)
               vs
               fn                  (lowercase, this is the function pointer TYPE)

    SYNTAX OF THE TRAIT BOUND:

    Fn(i32, i32) -> i32
    ─┬  ─────┬────   ─┬─
     │        │        └── return type after ->  (omit if ())
     │   argument types
     trait name (Fn / FnMut / FnOnce)

    IN GENERIC BOUNDS:

    fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 {
               ────────────────
               F must implement Fn(i32)->i32
    }

    fn apply<F>(f: F, x: i32) -> i32
    where
        F: Fn(i32) -> i32              — where clause, same meaning
    {
        f(x)
    }

    CHOOSING WHICH TRAIT:

    Fn      — closure called many times, read-only access to captures
              "I can call this as many times as I want, it has no side effects"

    FnMut   — closure called many times, may mutate captures
              "I may call this multiple times, and it might change state"
              The variable holding it must be mut: let mut f = ...

    FnOnce  — closure called AT MOST ONCE, may consume (move out of) captures
              "I will call this once and discard it"

    Hierarchy (supertrait relationship):

        Fn  ⊂  FnMut  ⊂  FnOnce

    Read: every Fn is also FnMut and FnOnce.
    So if you write FnOnce in a bound, you accept ALL three.
    If you write Fn in a bound, you only accept closures that implement Fn.

    RULE OF THUMB:
    — Use FnOnce when you call it once (e.g., thread::spawn, Option::map)
    — Use FnMut  when you call it multiple times and it may change state
    — Use Fn     when you call it multiple times and it must be side-effect-free
    — When in doubt: start with Fn, relax to FnMut or FnOnce only if the
      compiler complains
```

---

### 3.5 `impl Fn` vs `dyn Fn` vs `fn` — The Critical Distinction

This is the #1 syntax confusion in Rust.

```
    ┌─────────────────────────────────────────────────────────────────────┐
    │  impl Fn(T) -> U                                                     │
    ├─────────────────────────────────────────────────────────────────────┤
    │  Keyword: impl                                                       │
    │  Meaning: "some concrete type that implements Fn(T)->U"             │
    │           The compiler knows the EXACT type at compile time.        │
    │           Generates monomorphised (specialised) code.               │
    │  Size:    The underlying type's size (could be 0 for closures)      │
    │  Dispatch: STATIC — no vtable, no runtime overhead                  │
    │  Use in:  Parameter types, return types                             │
    │                                                                     │
    │  fn take(f: impl Fn(i32) -> i32) { … }                             │
    │  fn give() -> impl Fn(i32) -> i32 { |x| x + 1 }                   │
    └─────────────────────────────────────────────────────────────────────┘

    ┌─────────────────────────────────────────────────────────────────────┐
    │  dyn Fn(T) -> U                                                      │
    ├─────────────────────────────────────────────────────────────────────┤
    │  Keyword: dyn                                                        │
    │  Meaning: "erased type; we only know it implements Fn(T)->U"        │
    │           The EXACT type is NOT known at compile time.              │
    │           Dispatches via a vtable at runtime.                       │
    │  Size:    Dynamically sized (DST) — MUST be behind a pointer:       │
    │           &dyn Fn     Box<dyn Fn>     Arc<dyn Fn>                   │
    │  Dispatch: DYNAMIC — vtable lookup, small runtime cost              │
    │  Use in:  When you need to store different closure types together,  │
    │           or when you cannot use generics (trait objects)           │
    │                                                                     │
    │  fn take(f: &dyn Fn(i32) -> i32) { … }                             │
    │  fn give() -> Box<dyn Fn(i32) -> i32> { Box::new(|x| x + 1) }     │
    └─────────────────────────────────────────────────────────────────────┘

    ┌─────────────────────────────────────────────────────────────────────┐
    │  fn(T) -> U                                                          │
    ├─────────────────────────────────────────────────────────────────────┤
    │  Keyword: fn (lowercase)                                             │
    │  Meaning: Raw function pointer (like C). Holds an address only.     │
    │           Cannot hold a capturing closure.                          │
    │  Size:    Always 8 bytes (pointer-sized)                            │
    │  Dispatch: INDIRECT — loads address, then calls                     │
    │  Use in:  FFI, C callbacks, tables of function pointers             │
    │           When you need Copy semantics on a callable                │
    │                                                                     │
    │  let f: fn(i32) -> i32 = double;                                   │
    │  let ops: [fn(i32)->i32; 3] = [a, b, c];                           │
    └─────────────────────────────────────────────────────────────────────┘

    VISUAL COMPARISON:

         fn(i32)->i32           impl Fn(i32)->i32        dyn Fn(i32)->i32
         ─────────────          ─────────────────────    ──────────────────────
         pointer                concrete (mono.)         type-erased (vtable)
         8 bytes                0..N bytes               must be behind & or Box
         Copy + Clone           maybe Copy               not Sized
         no captures            any callable             any callable
         C interop              preferred for params     for heterogeneous/stored
         static/dyn dispatch    STATIC dispatch          DYNAMIC dispatch
```

---

### 3.6 Accepting a Callable as a Parameter

All three forms can appear as parameter types. Here is every syntax variant:

```
    ─── Using generic bound (impl Fn) — most common ─────────────────────────

    fn apply(f: impl Fn(i32) -> i32, x: i32) -> i32 {
        f(x)
    }

    ─── Using explicit generic parameter ────────────────────────────────────

    fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 {
        f(x)
    }

    ─── Using where clause ──────────────────────────────────────────────────

    fn apply<F>(f: F, x: i32) -> i32
    where
        F: Fn(i32) -> i32,
    {
        f(x)
    }

    ─── Using FnMut (closure that mutates captures) ─────────────────────────

    fn call_three_times<F: FnMut() -> i32>(mut f: F) {
                                           ↑
                                           Note: f must be declared mut
                                           because FnMut requires mutation
        f(); f(); f();
    }

    ─── Using FnOnce (consumed after one call) ──────────────────────────────

    fn call_once<F: FnOnce() -> String>(f: F) -> String {
        f()   — consumes f; cannot call again
    }

    ─── Using dyn Fn (dynamic dispatch, borrowed) ───────────────────────────

    fn apply_dyn(f: &dyn Fn(i32) -> i32, x: i32) -> i32 {
        f(x)
    }

    ─── Using dyn Fn (dynamic dispatch, owned) ──────────────────────────────

    fn apply_box(f: Box<dyn Fn(i32) -> i32>, x: i32) -> i32 {
        f(x)   — f is consumed (Box is moved in)
    }

    ─── Using bare fn pointer ───────────────────────────────────────────────

    fn apply_ptr(f: fn(i32) -> i32, x: i32) -> i32 {
        f(x)
    }

    ─── Multiple callable params ────────────────────────────────────────────

    fn map_filter<F, G>(
        data: &[i32],
        map_fn:    F,
        filter_fn: G,
    ) -> Vec<i32>
    where
        F: Fn(i32) -> i32,
        G: Fn(i32) -> bool,
    {
        data.iter()
            .map(|&x| map_fn(x))
            .filter(|&x| filter_fn(x))
            .collect()
    }
```

---

### 3.7 Returning a Callable from a Function

This is where most beginners stumble. There are exactly two syntactic choices:

```
    ─── Choice 1: impl Fn (preferred when possible) ─────────────────────────

    fn make_adder(n: i32) -> impl Fn(i32) -> i32 {
    //                       ───────────────────
    //                       return type: impl Fn(i32)->i32
    //                       — the compiler knows the exact type
    //                         but the caller does not
        move |x| x + n
    //  ↑ move is REQUIRED: n must be owned by the closure
    //    because the stack frame of make_adder is gone after return
    }

    ─── Choice 2: Box<dyn Fn> (when type must be erased) ───────────────────

    fn make_op(kind: &str) -> Box<dyn Fn(i32, i32) -> i32> {
    //                        ───────────────────────────────
    //                        return type wrapped in Box<dyn …>
    //                        — needed when different branches return
    //                          different closure types
        match kind {
            "add" => Box::new(|a, b| a + b),  // ← wrap each branch in Box::new(…)
            "mul" => Box::new(|a, b| a * b),
            _     => Box::new(|a, _| a),
        }
    }

    ─── WHY you cannot return a closure without impl or Box ─────────────────

    // This does NOT compile:
    fn bad() -> dyn Fn(i32) -> i32 {    // ERROR: dyn Fn is not Sized
        |x| x + 1
    }
    // dyn Trait is a dynamically-sized type (DST).
    // You cannot return DSTs directly; they must be behind a pointer.
    // Fix: Box<dyn Fn(i32)->i32>  or  impl Fn(i32)->i32

    ─── Returning with lifetime ─────────────────────────────────────────────

    fn make_greeter<'a>(name: &'a str) -> impl Fn() + 'a {
    //                                              ────
    //                                   + 'a means the returned closure
    //                                   borrows something that lives 'a
    //                                   (it borrows `name`)
        move || println!("Hello, {}!", name)
    }
```

---

### 3.8 `move` Keyword Placement

```
    move  |params|  body

    ↑ Always immediately BEFORE the opening |

    ─── All valid placements ─────────────────────────────────────────────────

    move || 42                          — no params
    move |x| x + 1                      — one param
    move |x, y| x + y                   — two params
    move |x: i32| -> i32 { x + 1 }     — with type annotations

    ─── move with let binding ───────────────────────────────────────────────

    let f = move |x| x + captured;      — assigned to variable

    ─── move as argument ────────────────────────────────────────────────────

    thread::spawn(move || { … });       — passed directly as argument

    ─── What move does ──────────────────────────────────────────────────────

    Without move:
        let n = 10;
        let f = |x| x + n;             — f borrows n (type: &i32 captured)

    With move:
        let n = 10;
        let f = move |x| x + n;        — f OWNS n (n is copied/moved in)
        // n is still usable here if it is Copy (i32 is Copy)
        // n is NOT usable here if it is non-Copy (String would be moved)
```

---

### 3.9 `extern "C"` Function Pointers

When doing FFI, function pointers carry a calling convention annotation:

```
    extern "C" fn(i32, i32) -> i32
    ─────────┬─
             └── calling convention: "C" uses the C ABI
                 "system" on Windows = stdcall, on Unix = C
                 "Rust" is the default (unstable between versions)

    DECLARING AN EXTERN FUNCTION:
    extern "C" {
        fn my_c_function(x: i32) -> i32;
        //           ↑ this is a declaration, not a definition
    }

    DEFINING A RUST FUNCTION CALLABLE FROM C:
    #[no_mangle]
    pub extern "C" fn my_rust_fn(x: i32) -> i32 {
        x * 2
    }

    STORING AN EXTERN FUNCTION POINTER:
    let f: unsafe extern "C" fn(i32) -> i32 = my_c_function;
    //     ────────────────────────────────
    //     unsafe because calling C code may have preconditions Rust cannot check
    //     extern "C" because C uses a different calling convention

    CALLING:
    let result = unsafe { f(21) };
    //           ──────
    //           unsafe block required when calling an extern fn
```

---

### 3.10 `Option<fn>` — Nullable Function Pointers

In Rust, `Option<fn(T)->U>` is the idiomatic way to express an optional
function pointer (equivalent to C's `NULL` check).

```
    let f: Option<fn(i32) -> i32> = Some(double);
    let g: Option<fn(i32) -> i32> = None;

    ─── Calling ─────────────────────────────────────────────────────────────

    if let Some(func) = f {
        func(5);               — call only if Some
    }

    f.map(|func| func(5));     — idiomatic: apply function if Some

    ─── In a struct ─────────────────────────────────────────────────────────

    struct Plugin {
        on_start: Option<fn()>,          — optional callback
        on_stop:  Option<fn()>,
    }

    impl Plugin {
        fn start(&self) {
            if let Some(f) = self.on_start { f(); }
        }
    }

    ─── Size guarantee ──────────────────────────────────────────────────────

    // Option<fn()> is GUARANTEED to be the same size as fn()
    // because Rust uses the null-pointer optimisation for non-nullable
    // function pointers.
    // None == null pointer; Some(f) == the function address.
    // std::mem::size_of::<Option<fn()>>() == std::mem::size_of::<fn()>() == 8
```

---

### 3.11 Rust Quick-Reference Card

```
    BARE FUNCTION POINTER (fn)
    ─────────────────────────────────────────────────────────────────────────
    fn(i32) -> i32                  type: pointer to function i32→i32
    fn(i32, i32) -> i32             two params
    fn() -> ()                      no params, unit return  (= fn())
    unsafe fn(i32) -> i32           unsafe function pointer
    extern "C" fn(i32) -> i32       C calling convention
    let f: fn(i32)->i32 = double;   variable declaration
    let f = double as fn(i32)->i32; explicit coercion

    CLOSURE
    ─────────────────────────────────────────────────────────────────────────
    |x| x + 1                       minimal closure
    |x: i32| x + 1                  with param type
    |x: i32| -> i32 { x + 1 }      fully annotated
    || 42                            no params
    move |x| x + n                  move capture
    move || work()                   no params, move capture

    TRAIT BOUNDS IN GENERICS
    ─────────────────────────────────────────────────────────────────────────
    F: Fn(i32) -> i32               read-only, call many times
    F: FnMut(i32) -> i32            mutating, call many times; f must be mut
    F: FnOnce(i32) -> i32           call once; f is consumed

    PARAMETER TYPES
    ─────────────────────────────────────────────────────────────────────────
    f: impl Fn(i32) -> i32          static dispatch (preferred)
    f: impl FnMut(i32) -> i32       static dispatch, mutable
    f: &dyn Fn(i32) -> i32          dynamic dispatch, borrowed
    f: Box<dyn Fn(i32) -> i32>      dynamic dispatch, owned
    f: fn(i32) -> i32               bare pointer (no captures)

    RETURN TYPES
    ─────────────────────────────────────────────────────────────────────────
    -> impl Fn(i32) -> i32          static (caller doesn't know exact type)
    -> Box<dyn Fn(i32) -> i32>      dynamic (heap, type-erased)
    -> fn(i32) -> i32               bare pointer (no captures in returned fn)

    OPTIONAL CALLABLE
    ─────────────────────────────────────────────────────────────────────────
    Option<fn(i32)->i32>            nullable fn pointer (None = no-op)
    Option<Box<dyn Fn(i32)->i32>>   nullable dyn closure

    CALLING SYNTAX (identical for all forms)
    ─────────────────────────────────────────────────────────────────────────
    f(arg)                          call with one arg
    f(a, b)                         two args
    f()                             no args
    (f)(arg)                        parens around f (also valid)
```

---

## 4. Side-by-Side Cheat Sheet

### 4.1 Declaring the Type

```
    Concept              C                           Go                    Rust
    ─────────────────────────────────────────────────────────────────────────────────

    fn ptr type          int (*)(int, int)           func(int, int) int    fn(i32,i32)->i32

    named type           typedef int (*Op)(int,int)  type Op func(int,int)int   (use trait bounds)
                         Op                          Op

    variable             int (*fp)(int,int)          var fp func(int,int)int    let f: fn(i32)->i32
                         Op fp                       fp := add                  let f = add as fn(i32)->i32

    no-capture closure   N/A (not in C)              func(x int) int { … }  |x: i32| x + 1
                                                     (anonymous function)

    capturing closure    struct+void*                func(x int) int { … }  |x| x + captured
                         (manual)                    with outer var captured    move |x| x + owned

    optional/null        int (*fp)(int) = NULL        var fp func(int) int   Option<fn(i32)->i32>
                         if (fp) fp(x)               if fp != nil { fp(x) }  f.map(|f| f(x))
```

### 4.2 Declaring a Parameter

```
    "accept a callable int→int"

    C:
        void apply(int (*f)(int), int x)      — raw pointer parameter
        void apply(Op f, int x)               — typedef'd
        void apply(int (*)(int), int)         — unnamed parameter (ok in prototype)

    Go:
        func apply(f func(int) int, x int) int   — function type inline
        func apply(f Op, x int) int              — named type

    Rust:
        fn apply(f: impl Fn(i32) -> i32, x: i32) -> i32   — static dispatch
        fn apply(f: &dyn Fn(i32) -> i32, x: i32) -> i32   — dynamic dispatch
        fn apply(f: fn(i32) -> i32, x: i32) -> i32         — bare fn pointer
        fn apply<F: Fn(i32)->i32>(f: F, x: i32) -> i32    — explicit generic
```

### 4.3 Declaring a Return Type

```
    "return a callable int→int"

    C:
        Op       make_op(char c)              — typedef (clearest)
        int    (*make_op(char c))(int)        — raw (hard to read)

    Go:
        func make_op(c byte) func(int) int   — return type is func(int)int

    Rust:
        fn make_op(c: char) -> impl Fn(i32) -> i32      — static (no heap)
        fn make_op(c: char) -> Box<dyn Fn(i32) -> i32>  — dynamic (heap)
        fn make_op(c: char) -> fn(i32) -> i32            — bare pointer (no capture)
```

### 4.4 Calling Through a Variable

```
    All three languages use identical call syntax:

    C:        fp(arg1, arg2)          (*fp)(arg1, arg2)    (both work)
    Go:       fp(arg1, arg2)
    Rust:     f(arg1, arg2)           (f)(arg1, arg2)      (both work)
```

### 4.5 Storing Multiple Callables

```
    C:
        typedef int (*Op)(int);
        Op table[4] = { add, sub, mul, div };    — array (all same type)

    Go:
        ops := []func(int, int) int{add, sub, mul}          — slice (all same type)
        ops := map[string]func(int,int)int{"add":add}        — map

    Rust:
        let ops: [fn(i32,i32)->i32; 3] = [add, sub, mul];  — array (all same fn ptr type)
        let ops: Vec<Box<dyn Fn(i32)->i32>> = vec![         — mixed closure types
            Box::new(|a,b| a+b),
            Box::new(move |a,b| a+b+offset),
        ];
```

### 4.6 Common Confusion Map

```
    CONFUSION                      RESOLUTION
    ──────────────────────────────────────────────────────────────────────────

    C: int *f(int)                 Function returning int*
       int (*f)(int)               Pointer to function returning int
    Rule: parens around *name = pointer; no parens = function

    C: arr[4] in declaration       Array of function pointers
       (*arr[4])(int)              The [4] goes INSIDE the outer parens

    Go: r.Method (no parens)       Method value (bound): type func()->R
        Type.Method (no parens)    Method expression (unbound): type func(Type)->R
    Rule: lowercase instance → value; Uppercase type → expression

    Go: func()                     Zero-value when declared with var
        nil                        You assign nil to unset it
        f == nil                   Only comparison allowed (not f == g)

    Rust: fn vs Fn                 fn = pointer type (lowercase)
                                   Fn = trait (uppercase)
          fn(i32)->i32             the type of a function pointer variable
          impl Fn(i32)->i32        parameter/return position: static dispatch
          dyn Fn(i32)->i32         parameter/return position: dynamic dispatch

    Rust: impl Fn                  no heap, no vtable, compiler specialises
          dyn Fn                   heap allocation needed, vtable at runtime

    Rust: FnMut param needs mut    let mut f = |x| { n+=1; x };
                                   mut f();
          forgetting mut on f      "cannot borrow f as mutable"

    Rust: move before ||           move |x| x + n     (not |move x|)
          not inside ||            |x| move x + n     WRONG placement
```

---

*Every syntax form in C, Go, and Rust — annotated, compared, and demystified.*

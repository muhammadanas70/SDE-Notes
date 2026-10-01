# The Complete C# Language Guide
### Syntax, Semantics, Memory Model, and Production Patterns

> A reference built for engineers coming from C/Go/Rust and systems programming.
> Every section maps a concept to *why it exists*, not just *how to write it*.

---

## Table of Contents

1. [CLR Architecture — The Execution Model](#1-clr-architecture)
2. [Compilation Pipeline](#2-compilation-pipeline)
3. [Program Structure & Syntax Basics](#3-program-structure)
4. [Type System Overview](#4-type-system-overview)
5. [Value Types vs Reference Types (Memory Model)](#5-value-vs-reference)
6. [Variables, Literals, Operators](#6-variables-literals-operators)
7. [Control Flow](#7-control-flow)
8. [Methods](#8-methods)
9. [Arrays & Collections](#9-arrays-collections)
10. [Strings](#10-strings)
11. [Classes & Objects](#11-classes-objects)
12. [Properties & Indexers](#12-properties-indexers)
13. [Inheritance & Polymorphism](#13-inheritance-polymorphism)
14. [Interfaces & Abstract Classes](#14-interfaces-abstract)
15. [Structs vs Classes — Deep Dive](#15-structs-vs-classes)
16. [Enums & Flags](#16-enums-flags)
17. [Generics](#17-generics)
18. [Delegates, Lambdas, Events](#18-delegates-lambdas-events)
19. [Exception Handling](#19-exception-handling)
20. [Memory Management & Garbage Collection](#20-memory-management-gc)
21. [IDisposable, `using`, Finalizers](#21-idisposable-using-finalizers)
22. [LINQ](#22-linq)
23. [Async/Await & Task-Based Concurrency](#23-async-await)
24. [Threading & Synchronization Primitives](#24-threading)
25. [Pattern Matching (Modern C#)](#25-pattern-matching)
26. [Records, Tuples, `with` expressions](#26-records-tuples)
27. [Nullable Reference Types](#27-nullable-reference-types)
28. [Extension Methods](#28-extension-methods)
29. [Operator Overloading](#29-operator-overloading)
30. [Attributes & Reflection](#30-attributes-reflection)
31. [Namespaces, Assemblies, Access Modifiers](#31-namespaces-assemblies)
32. [Span<T>, Memory<T>, and Low-Level/Unsafe C#](#32-span-memory-unsafe)
33. [Real-World Case Study: A TCP Proxy in C#](#33-case-study)
34. [Mental Model Cheat Sheet](#34-mental-model-cheat-sheet)

---

## 1. CLR Architecture

C# does not compile to native machine code directly (unless you use AOT/NativeAOT).
It compiles to **IL (Intermediate Language)** — conceptually similar to how Java
compiles to bytecode. The **CLR (Common Language Runtime)** is the virtual machine
that JITs IL into native code at run time and manages memory, threads, and exceptions.

If you know the Linux kernel's relationship to userspace processes, think of the
CLR as a *userspace runtime supervisor* — it doesn't own the CPU scheduler (the OS
still does), but it owns memory layout, type safety enforcement, and stack unwinding
for anything running inside it.

```
                     ┌─────────────────────────────────────────────┐
                     │              YOUR SOURCE CODE (.cs)          │
                     └───────────────────────┬───────────────────────┘
                                             │  Roslyn Compiler (csc)
                                             ▼
                     ┌─────────────────────────────────────────────┐
                     │   Assembly (.dll / .exe)                     │
                     │   ┌───────────────┐   ┌───────────────────┐  │
                     │   │  IL bytecode  │   │  Metadata / Types │  │
                     │   └───────────────┘   └───────────────────┘  │
                     └───────────────────────┬───────────────────────┘
                                             │  Loaded by CLR host (dotnet.exe)
                                             ▼
        ┌───────────────────────────────────────────────────────────────────┐
        │                        CLR (Common Language Runtime)               │
        │                                                                     │
        │  ┌────────────┐   ┌────────────────┐   ┌───────────────────────┐   │
        │  │  JIT/RyuJIT│   │  GC (Garbage   │   │  Type Loader /        │   │
        │  │  compiler  │   │  Collector)    │   │  Metadata resolver    │   │
        │  └─────┬──────┘   └───────┬────────┘   └───────────────────────┘   │
        │        │                  │                                        │
        │        ▼                  ▼                                        │
        │   Native machine     Managed Heap                                  │
        │   code (cached)      (Gen0/Gen1/Gen2, LOH)                          │
        └───────────────────────────────┬───────────────────────────────────┘
                                        │  syscalls (mmap, futex, epoll...)
                                        ▼
                     ┌─────────────────────────────────────────────┐
                     │           Linux Kernel / OS                  │
                     └─────────────────────────────────────────────┘
```

Key implications this diagram has for how you write code:

- **JIT compiles per-method, on first call**, then caches native code (Tiered
  Compilation: Tier0 fast/unoptimized → Tier1 fully optimized after enough calls).
  This is why the *first* request in a web service is often slower ("JIT warm-up"),
  analogous to page cache being cold after a fresh boot.
- **The GC owns your heap layout** — you don't control object placement the way
  you do with `malloc`/`mmap` in C. This has real consequences for anything you'd
  normally do with raw buffers (packet parsing, zero-copy) — see §32 (`Span<T>`).
- **Type metadata is embedded in the assembly.** This is what makes `reflection`
  (§30) and runtime type checks (`is`, `as`) possible without a separate symbol
  table, unlike a stripped C binary.

---

## 2. Compilation Pipeline

```
 file.cs ──► Roslyn (C# compiler) ──► IL + metadata ──► assembly (.dll/.exe)
                                                              │
                                                     dotnet run / dotnet exec
                                                              │
                                                              ▼
                                                CLR loads assembly, JITs hot methods,
                                                        executes native code
```

Two build/runtime models you'll actually choose between in production:

| Model | What happens | When to use |
|---|---|---|
| **JIT (default)** | IL shipped, JIT'd at startup/runtime | Normal services, fastest dev loop |
| **ReadyToRun (R2R)** | Native code pre-baked into assembly, JIT as fallback | Faster cold start (containers) |
| **NativeAOT** | Fully native binary, no CLR/JIT at run time, no reflection by default | CLI tools, sidecars, minimal attack surface, fast cold start — closest to what a Go binary gives you |

For anything you're deploying as a **cloud security sidecar or agent** (your
domain), NativeAOT is worth knowing: it removes the JIT and much of the
reflection-based metadata surface, shrinking both startup latency and attack
surface — conceptually closer to statically linking a Go binary than to a
traditional ".NET app".

---

## 3. Program Structure

Minimal C# 12 program (top-level statements — implicit `Main`):

```csharp
// Program.cs
Console.WriteLine("Hello, network.");
```

Compiler desugars this into:

```csharp
using System;

internal class Program
{
    private static void Main(string[] args)
    {
        Console.WriteLine("Hello, network.");
    }
}
```

Full explicit form, the way you'll see it in most production codebases and all
older code:

```csharp
namespace Acme.NetworkTools
{
    public class Program
    {
        public static void Main(string[] args)
        {
            Console.WriteLine("Hello, network.");
        }
    }
}
```

**Mental model**: a C# "program" is really "an assembly with an entry point
method". Everything — every class, struct, function — must live inside a type,
which lives inside a namespace (or the global namespace). There is no such
thing as a free function floating outside a type, unlike C.

---

## 4. Type System Overview

```
                              System.Object
                                    │
              ┌─────────────────────┴─────────────────────┐
              │                                             │
        Value Types                                   Reference Types
    (live on stack or inline                        (live on managed heap,
     in containing object;                            variable holds a
     copied by value)                                 pointer/reference)
              │                                             │
   ┌──────────┼──────────┐                     ┌────────────┼─────────────┐
   │          │           │                    │            │              │
 struct     enum      built-in            class          interface      delegate
 (Point,   (Color,    numerics           (string is        (IDisposable)  (Action,
  DateTime) DayOfWeek) (int, double,      actually a                       Func)
                        bool, char…)      reference
                                          type, special-
                                          cased!)
```

C# is **statically typed** with **type inference** (`var`) — inference happens
at compile time, it is not dynamic typing. `var x = 5;` is exactly `int x = 5;`
as far as the compiler and IL are concerned; `var` is a source-level convenience,
never a runtime concept (contrast with `dynamic`, which really is late-bound).

### Built-in type aliases

| C# keyword | .NET type | Size | Notes |
|---|---|---|---|
| `sbyte` | `System.SByte` | 1 byte | signed |
| `byte` | `System.Byte` | 1 byte | unsigned |
| `short` | `System.Int16` | 2 bytes | |
| `ushort` | `System.UInt16` | 2 bytes | |
| `int` | `System.Int32` | 4 bytes | default integer literal type |
| `uint` | `System.UInt32` | 4 bytes | |
| `long` | `System.Int64` | 8 bytes | suffix `L` |
| `ulong` | `System.UInt64` | 8 bytes | suffix `UL` |
| `float` | `System.Single` | 4 bytes | suffix `f` |
| `double` | `System.Double` | 8 bytes | default real literal type |
| `decimal` | `System.Decimal` | 16 bytes | base-10, financial precision, suffix `m` |
| `bool` | `System.Boolean` | 1 byte | `true`/`false` only, no int coercion |
| `char` | `System.Char` | 2 bytes | UTF-16 code unit, not a byte! |
| `string` | `System.String` | ref type | UTF-16, immutable |
| `object` | `System.Object` | ref type | root of everything |
| `nint`/`nuint` | native int | pointer-sized | for interop/pointer arithmetic |

`char` being UTF-16 (not ASCII/byte) is the single most common source of bugs
when C/network engineers move to C# and start parsing wire protocols — a
`string`'s `.Length` is a UTF-16 code-unit count, **not** a byte count. For wire
protocols, you almost always want `byte[]`/`Span<byte>`, never `string`, until
the final human-readable boundary.

---

## 5. Value vs Reference Types — Memory Model

This is the single most important mental model to get exactly right, because
your background (C/Rust/Go) makes you *assume* stack/heap rules that don't
transfer 1:1.

```
   int a = 10;                    Person p = new Person("Alice");
   int b = a;   // COPY           Person q = p;      // COPY OF REFERENCE

   STACK                          STACK                     HEAP
   ┌─────────┐                   ┌─────────┐              ┌────────────────┐
   │ a = 10  │                   │ p ──────┼─────────────►│ Person object   │
   ├─────────┤                   ├─────────┤         ┌───►│  Name: "Alice"  │
   │ b = 10  │  (independent)    │ q ──────┼─────────┘    │  Age: 0         │
   └─────────┘                   └─────────┘              └────────────────┘

   Mutating b never touches a.    Mutating via q.Name = "Bob"
                                  IS visible through p too —
                                  same object, two references.
```

Rules that follow from this model:

1. **Assignment for a value type copies the whole value.** Assignment for a
   reference type copies the *reference* (a pointer-like handle), not the
   object.
2. **Passing to a method** follows the same rule by default: value types are
   copied in, reference types pass the reference by value (so you can mutate
   the pointed-to object, but reassigning the parameter inside the method does
   not affect the caller's variable — unless you use `ref`).
3. `struct` = value type, `class` = reference type. That's the primary semantic
   difference between the two keywords — not "no inheritance" (though that's
   also true), the copy-vs-reference semantics is what actually bites you in
   production.
4. **Boxing**: when a value type is assigned to `object` (or an interface), the
   CLR heap-allocates a box containing a copy of the value and hands you a
   reference to that box. This is a hidden allocation — a classic hot-path bug:

```csharp
int i = 42;
object boxed = i;        // heap allocation happens here (boxing)
int back = (int)boxed;   // unboxing: copies value out of the box
```

If you've ever profiled a hot loop in C# and seen mystery Gen0 GC pressure with
no visible `new`, boxing inside a generic-less API (old-style `ArrayList`,
`string.Format` with value-type args in older runtimes, etc.) is a prime
suspect.

### `ref`, `out`, `in` — explicit pass-by-reference

```csharp
void Increment(ref int x) => x++;

int n = 5;
Increment(ref n);          // n is now 6 — caller's variable itself was modified
```

- `ref`  — parameter must already be initialized; callee may read and write it.
- `out`  — parameter need not be initialized before the call; callee **must**
  assign it before returning. Used for "return multiple values" patterns:

```csharp
bool TryParsePort(string s, out int port)
{
    return int.TryParse(s, out port) && port is > 0 and <= 65535;
}

if (TryParsePort("8443", out int p))
    Console.WriteLine($"Valid port {p}");
```

- `in`   — pass a (large) value type by reference for performance, but the
  callee gets a read-only view and cannot mutate it. Useful for big structs
  (e.g. a `struct` representing a fixed-size header) you don't want copied on
  every call.

```csharp
readonly struct PacketHeader { public readonly uint SrcIp, DstIp; public readonly ushort SrcPort, DstPort; }

void LogHeader(in PacketHeader hdr) => Console.WriteLine(hdr.SrcIp);
```

---

## 6. Variables, Literals, Operators

```csharp
int    a = 10;
double b = 3.14;
bool   flag = true;
char   c = 'x';
string s = "hello";
var    inferred = 42;      // still int, inferred at compile time

// Numeric literal forms
int hex   = 0xFF;
int bin   = 0b1010_1010;   // underscore = digit separator, purely cosmetic
long big  = 10_000_000_000L;

// Nullable value types — a value type that can also be null
int? maybePort = null;
if (maybePort.HasValue) Console.WriteLine(maybePort.Value);
int actual = maybePort ?? 8080;   // null-coalescing operator
```

### Operators worth knowing precisely (the ones that differ from C)

| Operator | Meaning | Notes |
|---|---|---|
| `??` | null-coalescing | `a ?? b` → `a` if not null, else `b` |
| `??=` | null-coalescing assignment | `a ??= b` → assign `b` to `a` only if `a` is null |
| `?.` | null-conditional | `obj?.Method()` short-circuits to `null` if `obj` is null, doesn't throw NRE |
| `?[]` | null-conditional index | `arr?[0]` |
| `is` | type/pattern test | `if (obj is string s)` — also binds `s` |
| `as` | safe cast | returns `null` on failure instead of throwing (reference types/nullable only) |
| `switch` expr | `x switch { 1 => "one", _ => "other" }` | expression form, not just statement |
| `..` | range | `arr[1..^1]` (skip first and last) |
| `^` | index-from-end | `arr[^1]` == last element |

`==` on reference types by default compares **reference identity** unless the
type overrides it (like `string` does, which overrides `==` for value
equality). This trips up C engineers coming from pointer-comparison intuitions
in the *opposite* direction — in C#, `==` is often value equality by
*convention* for types that choose to override it, which is the opposite
surprise from C/Go where `==` on non-primitives usually isn't even legal or is
always identity.

---

## 7. Control Flow

```csharp
// if / else
if (port < 0 || port > 65535)
    throw new ArgumentOutOfRangeException(nameof(port));
else if (port < 1024)
    Console.WriteLine("Privileged port");
else
    Console.WriteLine("Ephemeral/user port");

// switch statement — pattern-matching capable (C# 7+)
switch (proto)
{
    case ProtocolType.Tcp:
        Handshake();
        break;
    case ProtocolType.Udp when isConnectionless:
        SendDatagram();
        break;
    default:
        throw new NotSupportedException();
}

// switch EXPRESSION (C# 8+) — must be exhaustive or has a `_` discard
string Describe(int code) => code switch
{
    >= 200 and < 300 => "Success",
    >= 400 and < 500 => "Client error",
    >= 500           => "Server error",
    _                => "Unknown"
};

// loops
for (int i = 0; i < 10; i++) { }
foreach (var item in collection) { }         // uses IEnumerable<T>, calls Dispose on enumerator
while (cond) { }
do { } while (cond);

// loop control
foreach (var pkt in packets)
{
    if (pkt.IsMalformed) continue;
    if (pkt.IsTerminator) break;
}
```

`foreach` desugars to an explicit enumerator pattern — this matters because it
means `foreach` over anything implementing `IEnumerable<T>` calls
`GetEnumerator()`, then repeatedly `MoveNext()`/`Current`, then disposes the
enumerator in a `finally` block. Knowing this desugaring explains why you
**cannot modify a collection while foreach-ing over it** — the enumerator
tracks a version stamp and throws `InvalidOperationException` if it detects a
mutation (very similar to Go's map iteration guarantees, stricter than C's "do
whatever, good luck").


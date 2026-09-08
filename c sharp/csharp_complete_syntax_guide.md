# The Complete C# Guide — Syntax, Concepts & Architecture

A ground-up, in-depth reference for building a correct mental model of C# and the .NET runtime. Every section pairs the *why* with runnable code.

---

## Table of Contents

1. [The .NET / CLR Architecture](#1-the-net--clr-architecture)
2. [Compilation Pipeline](#2-compilation-pipeline)
3. [Program Structure & Entry Point](#3-program-structure--entry-point)
4. [Types, Variables & Literals](#4-types-variables--literals)
5. [Value Types vs Reference Types (Memory Model)](#5-value-types-vs-reference-types-memory-model)
6. [Operators & Expressions](#6-operators--expressions)
7. [Control Flow](#7-control-flow)
8. [Methods, Parameters & Overloading](#8-methods-parameters--overloading)
9. [Classes, Objects & Members](#9-classes-objects--members)
10. [Inheritance & Polymorphism](#10-inheritance--polymorphism)
11. [Interfaces & Abstract Classes](#11-interfaces--abstract-classes)
12. [Structs, Records & Value Semantics](#12-structs-records--value-semantics)
13. [Generics](#13-generics)
14. [Collections](#14-collections)
15. [Delegates, Events & Lambdas](#15-delegates-events--lambdas)
16. [LINQ](#16-linq)
17. [Exception Handling](#17-exception-handling)
18. [Garbage Collection & IDisposable](#18-garbage-collection--idisposable)
19. [Async/Await & the Task Model](#19-asyncawait--the-task-model)
20. [Pattern Matching & Modern C# Syntax](#20-pattern-matching--modern-c-syntax)
21. [Namespaces, Assemblies & Access Modifiers](#21-namespaces-assemblies--access-modifiers)
22. [Attributes & Reflection](#22-attributes--reflection)
23. [Nullable Reference Types](#23-nullable-reference-types)
24. [Real-World Composite Example](#24-real-world-composite-example)
25. [Mental Model Cheat Sheet](#25-mental-model-cheat-sheet)

---

## 1. The .NET / CLR Architecture

C# is not a standalone runtime language — it compiles to **Intermediate Language (IL)**, which the **Common Language Runtime (CLR)** executes. Understanding this pipeline is the single highest-leverage mental model for the entire language: it explains why C# has garbage collection, why generics behave the way they do, why `async` works, and why cross-language interop (F#, VB.NET) is seamless.

```
┌──────────────────────────────────────────────────────────────────────┐
│                          DEVELOPMENT MACHINE                         │
│                                                                        │
│   Program.cs  Class.cs  Interface.cs        (C# Source Files)        │
│        │           │           │                                     │
│        └───────────┴───────────┘                                     │
│                    │                                                  │
│                    ▼                                                  │
│         ┌────────────────────┐                                       │
│         │   Roslyn Compiler  │   (csc.exe / dotnet build)             │
│         │  Lex → Parse → AST │                                       │
│         │  Semantic Analysis │                                       │
│         └─────────┬──────────┘                                       │
│                    ▼                                                  │
│         ┌────────────────────┐                                       │
│         │   Assembly (.dll)  │                                       │
│         │  ┌──────────────┐  │                                       │
│         │  │  IL (CIL)    │  │  ← platform-independent bytecode       │
│         │  ├──────────────┤  │                                       │
│         │  │  Metadata    │  │  ← types, members, attributes          │
│         │  ├──────────────┤  │                                       │
│         │  │  Manifest    │  │  ← assembly identity, references       │
│         │  └──────────────┘  │                                       │
│         └─────────┬──────────┘                                       │
└────────────────────┼──────────────────────────────────────────────────┘
                      │  ships to / runs on
                      ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    CLR (Common Language Runtime)                     │
│                                                                        │
│   ┌────────────┐    ┌───────────────┐    ┌─────────────────────┐     │
│   │ Class      │    │  JIT Compiler │    │  Execution Engine   │     │
│   │ Loader     │───▶│ (RyuJIT)      │───▶│  runs native code   │     │
│   │            │    │  IL → x64/ARM │    │                     │     │
│   └────────────┘    └───────────────┘    └──────────┬──────────┘     │
│                                                       │                │
│   ┌───────────────────────────────────────────────────┘                │
│   ▼                                                                    │
│   ┌─────────────────────┐   ┌───────────────────┐   ┌───────────────┐ │
│   │  Garbage Collector  │   │  Type Safety /     │   │  Thread Pool /│ │
│   │  (Gen0/Gen1/Gen2,   │   │  Verification      │   │  Async Machin-│ │
│   │   LOH, background)  │   │                    │   │  ery (SynCtx) │ │
│   └─────────────────────┘   └───────────────────┘   └───────────────┘ │
│                                                                        │
│   Memory Layout per Process:                                          │
│   ┌────────────┬──────────────┬───────────────┬────────────────────┐ │
│   │   Stack    │  Small Object│  Large Object │   Loader Heap /     │ │
│   │ (value     │  Heap (SOH)  │  Heap (LOH)   │   Metadata          │ │
│   │  types,    │  (Gen0/1/2,  │  (>85,000     │   (JIT'd code,      │ │
│   │  frames,   │  reference   │  bytes,       │   type info)        │ │
│   │  refs)     │  type data)  │  arrays etc.) │                     │ │
│   └────────────┴──────────────┴───────────────┴────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

**Key takeaways from this diagram:**
- Your `.cs` file never runs directly. Roslyn turns it into **IL + metadata** packaged in an assembly (`.dll`/`.exe`).
- At runtime, the CLR's **JIT compiler** turns IL into native machine code *method-by-method, on first call* (there's also AOT/ReadyToRun compilation for faster startup, and full Native AOT in modern .NET).
- The **GC** owns the heap. You never `free()` — this single fact ripples into how you design types (see §18).
- Value types live on the stack (or inline in whatever contains them); reference types live on the heap and are accessed via a reference stored wherever the variable lives. This is the basis of §5.

---

## 2. Compilation Pipeline

```
Source (.cs)
    │
    ▼  Lexical Analysis
Tokens  (keywords, identifiers, literals, punctuation)
    │
    ▼  Parsing
Syntax Tree (AST)     — Roslyn exposes this tree; this is what enables
    │                    analyzers, refactorings, and source generators.
    ▼  Semantic Analysis
Bound Tree             — types resolved, overloads resolved, implicit
    │                     conversions inserted
    ▼  Lowering
Lowered Tree            — `foreach` → `while` + enumerator calls,
    │                     `async` → state machine, `using` → try/finally
    ▼  Emit
IL + Metadata → Assembly (.dll / .exe)
```

Two consequences engineers frequently miss:
- **`foreach`, `using`, `async`, LINQ query syntax, and `yield return` are all syntactic sugar.** The compiler *lowers* them into ordinary constructs (iterators become state machines implementing `IEnumerator<T>`; `async` methods become state machines implementing `IAsyncStateMachine`). Nothing magic happens at runtime that isn't expressible in plain C#.
- **Source generators** (Roslyn analyzers that run at compile time) can inspect the syntax/bound tree and inject additional code before emit — this is how libraries like `System.Text.Json` avoid reflection in trimmed/AOT scenarios.

---

## 3. Program Structure & Entry Point

Modern C# (C# 9+) supports **top-level statements** — no explicit `Main` required for simple programs.

```csharp
// Program.cs — top-level statements (implicit entry point)
Console.WriteLine("Hello, world");

var args_ = args; // 'args' is implicitly available
```

The classic explicit form (still what the compiler generates under the hood, and required when you need control over the signature):

```csharp
namespace MyApp;

internal class Program
{
    private static int Main(string[] args)
    {
        Console.WriteLine("Hello, world");
        return 0; // exit code
    }
}
```

**File-scoped namespaces** (C# 10+) — `namespace MyApp;` instead of `namespace MyApp { ... }` — remove a nesting level from every file.

---

## 4. Types, Variables & Literals

C# is **statically typed** — every variable's type is known at compile time — but offers `var` for compiler-inferred typing (not dynamic typing; the type is still fixed at compile time).

### Built-in Value Types

| Keyword   | CLR Type        | Size     | Notes                                 |
|-----------|-----------------|----------|----------------------------------------|
| `bool`    | `System.Boolean`| 1 byte   | `true` / `false`                       |
| `byte`    | `System.Byte`   | 1 byte   | 0–255                                   |
| `sbyte`   | `System.SByte`  | 1 byte   | -128–127                                |
| `short`   | `System.Int16`  | 2 bytes  |                                         |
| `int`     | `System.Int32`  | 4 bytes  | default integer literal type            |
| `long`    | `System.Int64`  | 8 bytes  | suffix `L`                              |
| `float`   | `System.Single`  | 4 bytes | suffix `f`                              |
| `double`  | `System.Double` | 8 bytes  | default real literal type               |
| `decimal` | `System.Decimal`| 16 bytes | suffix `m`; base-10, for money/finance  |
| `char`    | `System.Char`   | 2 bytes  | UTF-16 code unit                        |
| `nint`/`nuint` | native int | ptr-sized | platform pointer-sized integer     |

### Reference Types

`string`, arrays, delegates, all `class` instances, and interfaces are reference types.

```csharp
int age = 30;                       // value type, stack-allocated (if local)
string name = "Alice";               // reference type, heap-allocated object
double pi = 3.14159;
decimal price = 19.99m;              // exact decimal arithmetic (no binary rounding)
char grade = 'A';
bool isActive = true;

var inferred = 42;                   // compiler infers 'int' — still statically typed
const double Tau = 6.28318;          // compile-time constant, must be initialized
readonly int instanceId;             // runtime constant, settable only in constructor
```

### Literals

```csharp
int hex   = 0x1F;          // hexadecimal
int bin   = 0b1010;        // binary
int million = 1_000_000;   // digit separators for readability
string raw = """
    This is a raw string literal (C# 11+).
    No escaping needed for "quotes" or \backslashes\.
    """;
string interp = $"{name} is {age} years old";  // interpolation
```

### Nullability of value types: `Nullable<T>`

```csharp
int? maybeAge = null;              // Nullable<int>
if (maybeAge.HasValue) { Console.WriteLine(maybeAge.Value); }
int result = maybeAge ?? -1;       // null-coalescing default
```

---

## 5. Value Types vs Reference Types (Memory Model)

This is the single most important architectural distinction in C#, and where most bugs from other-language habits (JS, Python) originate.

```
VALUE TYPE ASSIGNMENT                    REFERENCE TYPE ASSIGNMENT
──────────────────────                   ──────────────────────────

int a = 5;                               var p1 = new Point(1, 2);   // class Point
int b = a;    // COPY                    var p2 = p1;     // COPY OF REFERENCE

Stack:                                   Stack:                 Heap:
┌───────┐                                ┌────────┐            ┌─────────────┐
│ a = 5 │                                │ p1  ●──┼───────────▶│ Point{1,2}  │
├───────┤                                ├────────┤            └─────────────┘
│ b = 5 │  (independent copy)            │ p2  ●──┼──────────────────┘
└───────┘                                └────────┘  (both point to SAME object)

b = 10;  →  a is still 5                 p2.X = 99;  →  p1.X is ALSO 99
```

```csharp
struct Point { public int X, Y; }   // value type
class PointBox { public int X, Y; } // reference type

Point p1 = new Point { X = 1, Y = 2 };
Point p2 = p1;          // full copy
p2.X = 99;
Console.WriteLine(p1.X); // 1 — unaffected

PointBox b1 = new PointBox { X = 1, Y = 2 };
PointBox b2 = b1;        // copies the reference, not the object
b2.X = 99;
Console.WriteLine(b1.X); // 99 — same object
```

**Passing to methods** follows the same rule: value types are copied into the parameter; reference types pass a copy of the *reference* (so mutating the object's fields affects the caller's object, but reassigning the parameter itself does not affect the caller's variable — unless you use `ref`):

```csharp
void Reset(Point p)      { p.X = 0; }              // no effect on caller
void Reset(PointBox p)   { p.X = 0; }               // mutates caller's object
void Rebind(ref PointBox p) { p = new PointBox(); } // rebinds caller's reference itself
void Modify(in Point p)  { /* p.X = 1; // compile error, read-only */ }
void Output(out int x)   { x = 42; }                // must assign before returning
```

- `ref` — pass by reference (caller's variable can be reassigned by callee).
- `out` — like `ref`, but the callee **must** assign it before returning; used for "return multiple values".
- `in` — pass by reference, read-only (avoids copying large structs without allowing mutation).

---

## 6. Operators & Expressions

```csharp
// Arithmetic
int sum = 3 + 4; int diff = 10 - 3; int mod = 10 % 3;

// Null-conditional / null-coalescing — critical for defensive code
string? city = person?.Address?.City;         // short-circuits to null if any link is null
string displayCity = city ?? "Unknown";       // default if null
person ??= new Person();                       // assign only if currently null

// Range & Index (C# 8+)
int[] numbers = { 0, 1, 2, 3, 4, 5 };
int last = numbers[^1];              // 5 — Index from end
int[] middle = numbers[2..4];        // { 2, 3 } — Range slice

// Pattern-based equality / comparisons covered in §20
// Bitwise
int flags = 0b0101 | 0b0010;   // OR
int masked = flags & 0b0110;   // AND
int shifted = 1 << 4;          // 16

// Checked/unchecked arithmetic contexts
checked
{
    int max = int.MaxValue;
    // int overflow = max + 1; // throws OverflowException here
}
```

---

## 7. Control Flow

```csharp
// if / else if / else
if (score >= 90) grade = 'A';
else if (score >= 80) grade = 'B';
else grade = 'F';

// switch statement with pattern matching (modern form)
string category = grade switch
{
    'A' or 'B' => "Pass with distinction",
    'C'        => "Pass",
    _          => "Needs improvement"          // discard = default case
};

// Loops
for (int i = 0; i < 10; i++) { }
foreach (var item in collection) { }
int j = 0;
while (j < 10) { j++; }
do { j++; } while (j < 20);

// Loop control
foreach (var n in numbers)
{
    if (n == 0) continue;
    if (n > 100) break;
}

// goto — rare, but valid, mostly used in generated code / fallthrough switch cases
```

---

## 8. Methods, Parameters & Overloading

```csharp
// Overloading — same name, different signature (parameter types/count)
int Add(int a, int b) => a + b;
double Add(double a, double b) => a + b;

// Optional parameters + named arguments
void Log(string message, string level = "INFO", bool timestamp = true) { }
Log("Startup complete", timestamp: false);   // named argument, skips 'level'

// params — variable-length argument list
int Sum(params int[] values) => values.Sum();
Sum(1, 2, 3, 4);

// Local functions — scoped helper functions, can capture enclosing variables
int Outer(int x)
{
    int Square(int n) => n * n;   // local function
    return Square(x) + 1;
}

// Expression-bodied members
int Square(int n) => n * n;
public int Age { get; }
public override string ToString() => $"Age: {Age}";
```

**Method resolution** at compile time picks the *most specific applicable* overload — this is resolved statically, not via runtime dispatch (unlike virtual method calls, see §10).

---

## 9. Classes, Objects & Members

```csharp
public class BankAccount
{
    // Fields — backing storage
    private decimal _balance;

    // Auto-property
    public string Owner { get; set; }

    // Property with custom logic
    public decimal Balance
    {
        get => _balance;
        private set => _balance = value < 0
            ? throw new ArgumentException("Balance cannot be negative")
            : value;
    }

    // Constructor
    public BankAccount(string owner, decimal initialBalance = 0)
    {
        Owner = owner;
        Balance = initialBalance;
    }

    // Instance method
    public void Deposit(decimal amount)
    {
        if (amount <= 0) throw new ArgumentOutOfRangeException(nameof(amount));
        Balance += amount;
    }

    public bool TryWithdraw(decimal amount)
    {
        if (amount > Balance) return false;
        Balance -= amount;
        return true;
    }

    // Static member — belongs to the type, not an instance
    public static BankAccount CreateDefault() => new("Unnamed", 0);

    // Indexer — lets the type be used like an array
    private readonly List<decimal> _history = new();
    public decimal this[int index] => _history[index];
}

var acct = new BankAccount("Priya", 1000m);
acct.Deposit(500m);
acct.TryWithdraw(200m);
```

Key member kinds: fields, properties, methods, constructors, indexers, events (§15), operators (operator overloading), and nested types.

---

## 10. Inheritance & Polymorphism

```csharp
public abstract class Shape
{
    public abstract double Area { get; }               // must be overridden
    public virtual string Describe() => $"Shape with area {Area:F2}";  // can be overridden
}

public class Circle : Shape
{
    public double Radius { get; init; }
    public override double Area => Math.PI * Radius * Radius;
    public override string Describe() => $"Circle: {base.Describe()}";
}

public class Square : Shape
{
    public double Side { get; init; }
    public override double Area => Side * Side;
}

Shape[] shapes = { new Circle { Radius = 2 }, new Square { Side = 3 } };
foreach (var s in shapes)
    Console.WriteLine(s.Describe()); // runtime (virtual) dispatch picks the right override
```

```
       Runtime dispatch for s.Describe() when s is declared as Shape:

       Shape shapeRef ──▶ [ Object Header | Circle's fields | vtable ptr ]
                                                                  │
                                                                  ▼
                                                      Circle's method table:
                                                      Describe → Circle.Describe
                                                      Area     → Circle.Area
```

- `virtual` / `override` → dynamic dispatch through a method table (vtable-like mechanism in the CLR).
- `new` (method hiding) → *shadows* a base member rather than overriding it; dispatch is based on the **static** type of the reference, not the runtime type — a common source of bugs when confused with `override`.
- `sealed` on a class prevents further inheritance; `sealed override` prevents further overriding of one member.
- All types implicitly derive from `System.Object` (`ToString`, `Equals`, `GetHashCode`, `GetType`).

---

## 11. Interfaces & Abstract Classes

```csharp
public interface IPayable
{
    decimal CalculatePayment();
    string PaymentDescription => "Standard payment";   // default interface method (C# 8+)
}

public interface IAuditable
{
    void RecordAudit(string action);
}

// A class can implement multiple interfaces but inherit only one base class
public class Invoice : IPayable, IAuditable
{
    public decimal Amount { get; set; }
    public decimal CalculatePayment() => Amount * 1.0m;
    public void RecordAudit(string action) => Console.WriteLine($"Audit: {action}");
}
```

**Interface vs abstract class — when to use which:**

| | Interface | Abstract class |
|---|---|---|
| Multiple inheritance | Yes — a class can implement many | No — single base class only |
| Can hold state (fields) | No (only auto-properties without backing field via default impl restrictions) | Yes |
| Constructors | No | Yes |
| Use when | Defining a *capability/contract* ("can do X") | Defining a *shared identity/hierarchy* ("is a X") with common code |

---

## 12. Structs, Records & Value Semantics

```csharp
// struct — value type, good for small, immutable, frequently-copied data
public readonly struct Vector2
{
    public double X { get; }
    public double Y { get; }
    public Vector2(double x, double y) { X = x; Y = y; }
    public double Length => Math.Sqrt(X * X + Y * Y);
}

// record class — reference type with value-based equality and concise syntax (C# 9+)
public record Person(string FirstName, string LastName)
{
    public int Age { get; init; }   // init-only property — settable only during construction
}

var p1 = new Person("Ada", "Lovelace") { Age = 36 };
var p2 = new Person("Ada", "Lovelace") { Age = 36 };
Console.WriteLine(p1 == p2);        // True — records compare by VALUE, not reference

var p3 = p1 with { Age = 37 };      // non-destructive mutation — copies, changes one field

// record struct (C# 10+) — value type + record semantics combined
public readonly record struct Point3D(double X, double Y, double Z);
```

`record` auto-generates `Equals`, `GetHashCode`, `ToString`, and the `with` expression — eliminating the boilerplate that used to be written by hand for immutable DTOs.

---

## 13. Generics

Generics give you compile-time type safety **and** avoid boxing/casting overhead, by letting the CLR generate specialized native code per value-type instantiation (`List<int>` and `List<string>` are genuinely different compiled types under the hood for value types; reference-type instantiations share code).

```csharp
public class Repository<T> where T : class, IEntity, new()
{
    private readonly List<T> _items = new();

    public void Add(T item) => _items.Add(item);

    public T? Find(Func<T, bool> predicate) => _items.FirstOrDefault(predicate);

    public T CreateDefault() => new T();   // requires 'new()' constraint
}

public interface IEntity { int Id { get; } }

// Generic method with its own type parameter, independent of any containing class
public static T Max<T>(T a, T b) where T : IComparable<T>
    => a.CompareTo(b) > 0 ? a : b;

Console.WriteLine(Max(3, 7));           // T inferred as int
Console.WriteLine(Max("abc", "abd"));   // T inferred as string
```

**Common constraints:** `where T : class`, `where T : struct`, `where T : new()`, `where T : BaseType`, `where T : IInterface`, `where T : notnull`, `where T : unmanaged`.

**Covariance/contravariance** in generic interfaces:

```csharp
IEnumerable<string> strings = new List<string>();
IEnumerable<object> objects = strings;   // OK — 'out T' makes IEnumerable<T> covariant

Action<object> objAction = o => Console.WriteLine(o);
Action<string> strAction = objAction;    // OK — 'in T' makes Action<T> contravariant
```

---

## 14. Collections

```csharp
// Arrays — fixed size, contiguous memory, fastest indexed access
int[] fixedArr = new int[5];
int[,] matrix = new int[3, 3];             // multidimensional
int[][] jagged = new int[3][];             // array of arrays (each row independent size)

// List<T> — dynamic array, most common general-purpose collection
var list = new List<int> { 1, 2, 3 };
list.Add(4);
list.RemoveAt(0);

// Dictionary<TKey, TValue> — hash table, O(1) average lookup
var scores = new Dictionary<string, int> { ["Alice"] = 95, ["Bob"] = 87 };
if (scores.TryGetValue("Alice", out int score)) Console.WriteLine(score);

// HashSet<T> — unique elements, O(1) average membership test
var seen = new HashSet<int> { 1, 2, 3 };
seen.Add(2);   // no-op, already present

// Queue<T> (FIFO) / Stack<T> (LIFO)
var queue = new Queue<int>(); queue.Enqueue(1); queue.Dequeue();
var stack = new Stack<int>(); stack.Push(1); stack.Pop();

// Immutable & concurrent variants
using System.Collections.Immutable;
var immutableList = ImmutableList.Create(1, 2, 3);
var concurrentDict = new System.Collections.Concurrent.ConcurrentDictionary<string, int>();

// Span<T> — a stack-only, allocation-free view over contiguous memory
// (arrays, stackalloc, or slices thereof) — critical for high-performance code
Span<int> span = stackalloc int[4] { 1, 2, 3, 4 };
Span<int> slice = span.Slice(1, 2);   // { 2, 3 } — no heap allocation at all
```

**Choosing a collection:** `List<T>` for general sequential data, `Dictionary<K,V>` for keyed lookup, `HashSet<T>` for uniqueness/membership tests, `Queue`/`Stack` for FIFO/LIFO semantics, `Span<T>`/`Memory<T>` when avoiding heap allocation matters (parsers, network buffers — directly relevant if you're writing high-throughput packet-processing code).

---

## 15. Delegates, Events & Lambdas

A **delegate** is a type-safe function pointer — an object that holds a reference to one or more methods with a matching signature.

```csharp
public delegate int MathOperation(int a, int b);

int Add(int a, int b) => a + b;
MathOperation op = Add;
Console.WriteLine(op(2, 3));            // 5

// Built-in generic delegates — almost always preferable to custom delegate types
Func<int, int, int> add = (a, b) => a + b;      // has return value
Action<string> log = msg => Console.WriteLine(msg);  // no return value
Predicate<int> isEven = n => n % 2 == 0;         // returns bool

// Multicast delegates — combine multiple method targets
Action pipeline = () => Console.Write("A");
pipeline += () => Console.Write("B");
pipeline();   // prints "AB"
```

**Events** are a controlled wrapper around delegates — the publisher can raise the event, but only the publisher (not subscribers) can invoke it directly from outside:

```csharp
public class Button
{
    public event EventHandler<EventArgs>? Clicked;

    public void SimulateClick() => Clicked?.Invoke(this, EventArgs.Empty);
}

var btn = new Button();
btn.Clicked += (sender, args) => Console.WriteLine("Button was clicked!");
btn.SimulateClick();
```

**Closures:** lambdas capture enclosing variables by reference (not by value at creation time) — a classic gotcha inside loops:

```csharp
var actions = new List<Action>();
for (int i = 0; i < 3; i++)
{
    int captured = i;                       // capture a fresh copy per iteration
    actions.Add(() => Console.WriteLine(captured));
}
// Since C# 5, 'foreach' loop variables are scoped per-iteration by default,
// but 'for' loop variables are NOT — this pattern above is still necessary for 'for'.
```

---

## 16. LINQ

Language Integrated Query — a unified, declarative query syntax over any `IEnumerable<T>` (in-memory) or `IQueryable<T>` (translated to SQL, e.g. via EF Core).

```csharp
var employees = new List<Employee>
{
    new("Alice", "Engineering", 95000),
    new("Bob", "Sales", 62000),
    new("Carol", "Engineering", 88000),
};

// Method syntax (chained extension methods) — most commonly used in practice
var topEngineers = employees
    .Where(e => e.Department == "Engineering")
    .OrderByDescending(e => e.Salary)
    .Select(e => new { e.Name, e.Salary })
    .ToList();

// Query syntax — equivalent, SQL-like
var topEngineers2 =
    from e in employees
    where e.Department == "Engineering"
    orderby e.Salary descending
    select new { e.Name, e.Salary };

// Aggregation
double avgSalary = employees.Average(e => e.Salary);
decimal totalPayroll = employees.Sum(e => e.Salary);
var byDept = employees.GroupBy(e => e.Department);

foreach (var group in byDept)
    Console.WriteLine($"{group.Key}: {group.Count()} employees");

record Employee(string Name, string Department, decimal Salary);
```

**Deferred execution** is the key architectural concept: most LINQ operators (`Where`, `Select`, `OrderBy`) build up an *expression* and don't actually run until you enumerate the result (via `foreach`, `.ToList()`, `.Count()`, etc.). This means the same query can return different results if the underlying data changes between definition and enumeration.

---

## 17. Exception Handling

```csharp
try
{
    int[] arr = { 1, 2, 3 };
    Console.WriteLine(arr[10]);
}
catch (IndexOutOfRangeException ex) when (ex.Message.Contains("index"))
{
    // 'when' clause — exception filter, only catches if condition is true
    Console.WriteLine($"Caught: {ex.Message}");
}
catch (Exception ex)
{
    Console.WriteLine($"Unexpected: {ex}");
    throw;              // re-throws preserving original stack trace
    // throw ex;        // AVOID — this resets the stack trace to this point
}
finally
{
    Console.WriteLine("Cleanup always runs, exception or not");
}

// Custom exception types — should carry meaningful context
public class InsufficientFundsException : Exception
{
    public decimal RequestedAmount { get; }
    public InsufficientFundsException(decimal amount)
        : base($"Cannot withdraw {amount:C} — insufficient funds")
    {
        RequestedAmount = amount;
    }
}
```

**Exception hierarchy essentials:** everything derives from `System.Exception`. `SystemException` covers runtime-thrown exceptions (`NullReferenceException`, `IndexOutOfRangeException`); `ApplicationException` was historically meant for user code but is largely ignored in favor of just deriving from `Exception` directly. Catch the **most specific** type you can meaningfully handle; avoid empty `catch (Exception) { }` blocks that swallow errors silently.

---

## 18. Garbage Collection & IDisposable

Because the CLR owns heap memory, C# has no `free()`/`delete`. But **unmanaged resources** (file handles, sockets, native memory, database connections) still need deterministic cleanup — that's what `IDisposable` and `using` are for.

```csharp
public class FileLogger : IDisposable
{
    private readonly StreamWriter _writer;
    private bool _disposed;

    public FileLogger(string path) => _writer = new StreamWriter(path);

    public void Log(string message) => _writer.WriteLine(message);

    public void Dispose()
    {
        if (_disposed) return;
        _writer.Dispose();          // release the unmanaged file handle
        _disposed = true;
        GC.SuppressFinalize(this);  // no need for the finalizer to run now
    }
}

// 'using' declaration (C# 8+) — Dispose() called automatically at end of scope
using var logger = new FileLogger("app.log");
logger.Log("Started");
// Dispose() called here, when 'logger' goes out of scope

// Classic 'using' block form — equivalent, explicit scope
using (var logger2 = new FileLogger("app2.log"))
{
    logger2.Log("Started");
} // Dispose() called here
```

```
GC Generational Model:

  Gen 0 ─── new short-lived objects allocated here
    │  (most objects die young — "generational hypothesis")
    │  survives a collection?
    ▼
  Gen 1 ─── buffer between short and long-lived
    │  survives again?
    ▼
  Gen 2 ─── long-lived objects (caches, static data)

  Large Object Heap (LOH) ─── objects ≥ 85,000 bytes, collected with Gen 2,
                               not compacted by default (fragmentation risk)
```

- The GC is generational and compacting (except LOH by default) — most allocations die in Gen 0, making short-lived-object allocation extremely cheap.
- `IDisposable`/`using` handles **deterministic** cleanup of unmanaged resources; the GC alone only handles **managed memory reclamation**, and its timing is non-deterministic.
- A **finalizer** (`~ClassName()`) is a safety net for when `Dispose()` isn't called — but it delays collection by at least one extra GC cycle, so it should only back up `IDisposable`, never replace it.

---

## 19. Async/Await & the Task Model

`async`/`await` is syntactic sugar over a compiler-generated **state machine** implementing `IAsyncStateMachine` — conceptually similar to how iterators (`yield return`) become `IEnumerator` state machines.

```csharp
public async Task<string> FetchDataAsync(string url)
{
    using var client = new HttpClient();
    // 'await' suspends this method (returns control to the caller) without
    // blocking the calling thread; when the I/O completes, execution resumes
    // here — possibly on a different thread pool thread.
    string result = await client.GetStringAsync(url);
    return result.ToUpperInvariant();
}

public async Task ProcessMultipleAsync()
{
    // Sequential awaits — each waits for the previous to finish
    var a = await FetchDataAsync("https://api.example.com/a");
    var b = await FetchDataAsync("https://api.example.com/b");

    // Concurrent awaits — start both, then wait for both
    Task<string> taskC = FetchDataAsync("https://api.example.com/c");
    Task<string> taskD = FetchDataAsync("https://api.example.com/d");
    await Task.WhenAll(taskC, taskD);
    Console.WriteLine($"{taskC.Result} {taskD.Result}");
}
```

```
Synchronous call:                        Asynchronous call (await):

Thread ──▶ [Method runs] ──▶ blocked      Thread ──▶ [Method starts]
           waiting on I/O                            │  hits 'await', I/O begins
           ──▶ result ──▶ continues                  ▼
                                          Thread returned to pool / caller,
                                          free to do other work
                                                      │
                                          I/O completes on background thread
                                                      ▼
                                          Continuation scheduled, method resumes
                                          (possibly on a different thread)
```

**Key rules:**
- `async` methods should return `Task`, `Task<T>`, or `ValueTask<T>` — never `void`, except top-level event handlers (`async void` swallows exceptions in a way that's hard to observe).
- `await` does **not** create a new thread — it frees the current thread to do other work while I/O is pending, then resumes the continuation (often on a thread pool thread).
- `ConfigureAwait(false)` avoids capturing the original synchronization context (relevant in UI apps / classic ASP.NET; largely a no-op concern in ASP.NET Core, but still good library-code hygiene).
- `CancellationToken` is the idiomatic cooperative-cancellation mechanism, threaded through async call chains.

```csharp
public async Task<string> FetchWithCancellationAsync(string url, CancellationToken ct)
{
    using var client = new HttpClient();
    return await client.GetStringAsync(url, ct);
}
```

---

## 20. Pattern Matching & Modern C# Syntax

```csharp
// Type patterns
object shape = new Circle { Radius = 3 };
if (shape is Circle { Radius: > 0 } c)
    Console.WriteLine($"Positive-radius circle: {c.Radius}");

// Switch expressions with property/positional patterns
string Describe(object obj) => obj switch
{
    int n when n < 0        => "negative number",
    int n                    => $"non-negative number: {n}",
    string { Length: 0 }     => "empty string",
    string s                 => $"string of length {s.Length}",
    Circle { Radius: var r } => $"circle with radius {r}",
    null                      => "null value",
    _                         => "unknown"
};

// Tuple deconstruction & patterns
(int x, int y) point = (3, 4);
var (px, py) = point;

// Record positional deconstruction
record Point(int X, int Y);
var p = new Point(1, 2);
if (p is (0, 0)) Console.WriteLine("origin");

// Relational & logical patterns (C# 9+)
static string Grade(int score) => score switch
{
    >= 90 and <= 100 => "A",
    >= 80            => "B",
    >= 70            => "C",
    _                => "F"
};

// 'not' pattern
if (obj is not null) { /* ... */ }
```

---

## 21. Namespaces, Assemblies & Access Modifiers

```csharp
namespace Company.Product.Module;   // file-scoped namespace (C# 10+)

public class Service { }            // accessible anywhere
internal class Helper { }           // accessible only within this assembly
public class Outer
{
    protected int shared;            // accessible to this class + derived classes
    private int secret;              // accessible only within this class
    private protected int x;         // derived classes, SAME assembly only
    protected internal int y;        // derived classes ANY assembly, or same assembly
}
```

**Assembly** = the unit of deployment and versioning (a `.dll` or `.exe`). Types are uniquely identified by namespace + assembly, which is why two different NuGet packages can each define `Company.Utils.Logger` without colliding, as long as they live in different assemblies with different strong names.

```csharp
using Company.Product.Module;         // brings a namespace into scope
using Json = System.Text.Json.JsonSerializer;  // alias, avoids ambiguity
global using System;                   // C# 10+ — applies project-wide from one file
```

---

## 22. Attributes & Reflection

**Attributes** attach declarative metadata to code elements, readable at runtime via reflection.

```csharp
[Serializable]
public class Config
{
    [Required]
    public string Name { get; set; } = string.Empty;

    [Obsolete("Use NewMethod instead")]
    public void OldMethod() { }
}

// Custom attribute
[AttributeUsage(AttributeTargets.Method)]
public class RetryAttribute : Attribute
{
    public int MaxAttempts { get; }
    public RetryAttribute(int maxAttempts) => MaxAttempts = maxAttempts;
}

public class ApiClient
{
    [Retry(3)]
    public void CallExternalService() { }
}
```

**Reflection** reads this metadata (and any type's shape) at runtime:

```csharp
Type type = typeof(Config);
foreach (var prop in type.GetProperties())
    Console.WriteLine($"{prop.Name}: {prop.PropertyType}");

MethodInfo method = typeof(ApiClient).GetMethod(nameof(ApiClient.CallExternalService))!;
var retryAttr = method.GetCustomAttribute<RetryAttribute>();
if (retryAttr is not null)
    Console.WriteLine($"Max attempts: {retryAttr.MaxAttempts}");

// Dynamic instantiation — the basis of DI containers, serializers, ORMs
object instance = Activator.CreateInstance(typeof(Config))!;
```

Reflection is powerful (it underlies dependency injection containers, ORMs like EF Core, and serializers), but has runtime cost and fights AOT/trimming — modern .NET favors **source generators** (compile-time codegen, see §2) for hot paths like JSON serialization.

---

## 23. Nullable Reference Types

Enabled per-project (`<Nullable>enable</Nullable>` in the `.csproj`) or per-file (`#nullable enable`), this feature makes the *reference type* nullability explicit and flow-analyzed at compile time — it does **not** change runtime behavior, only adds compiler warnings.

```csharp
#nullable enable

public class UserService
{
    public string Name { get; set; } = string.Empty;     // non-nullable — must be initialized
    public string? MiddleName { get; set; }               // explicitly nullable

    public string Greet(string? nickname)
    {
        // Compiler forces you to handle the null case before dereferencing
        if (nickname is null) return $"Hello, {Name}";
        return $"Hello, {nickname}";
    }

    public string GreetUnsafe(string? nickname)
        => $"Hello, {nickname!.ToUpper()}";   // '!' null-forgiving operator — "trust me"
}
```

This closes off an entire historical class of `NullReferenceException` bugs *at compile time*, by making "can this be null?" part of the type signature instead of an unstated assumption.

---

## 24. Real-World Composite Example

A small but realistic example tying together generics, interfaces, LINQ, async, exceptions, and records — a minimal in-memory rate limiter, the kind of utility you might actually write in a networking/services codebase:

```csharp
public interface IRateLimiter
{
    Task<bool> TryAcquireAsync(string key, CancellationToken ct = default);
}

public record RateLimitPolicy(int MaxRequests, TimeSpan Window);

public class SlidingWindowRateLimiter : IRateLimiter
{
    private readonly RateLimitPolicy _policy;
    private readonly Dictionary<string, List<DateTime>> _history = new();
    private readonly SemaphoreSlim _lock = new(1, 1);

    public SlidingWindowRateLimiter(RateLimitPolicy policy) => _policy = policy;

    public async Task<bool> TryAcquireAsync(string key, CancellationToken ct = default)
    {
        await _lock.WaitAsync(ct);
        try
        {
            var now = DateTime.UtcNow;
            var cutoff = now - _policy.Window;

            if (!_history.TryGetValue(key, out var timestamps))
            {
                timestamps = new List<DateTime>();
                _history[key] = timestamps;
            }

            // Drop entries outside the sliding window — LINQ used for clarity
            timestamps.RemoveAll(t => t < cutoff);

            if (timestamps.Count >= _policy.MaxRequests)
                return false;   // rate limit exceeded

            timestamps.Add(now);
            return true;
        }
        finally
        {
            _lock.Release();
        }
    }
}

// Usage
var limiter = new SlidingWindowRateLimiter(new RateLimitPolicy(MaxRequests: 5, Window: TimeSpan.FromSeconds(10)));

for (int i = 0; i < 7; i++)
{
    bool allowed = await limiter.TryAcquireAsync("client-42");
    Console.WriteLine($"Request {i}: {(allowed ? "ALLOWED" : "REJECTED")}");
}
```

This demonstrates: an interface as a contract (`IRateLimiter`), a `record` for an immutable value object (`RateLimitPolicy`), async coordination with `SemaphoreSlim` (since regular `lock` cannot wrap `await`), LINQ-style collection manipulation, and cancellation token plumbing — the everyday shape of production C# service code.

---

## 25. Mental Model Cheat Sheet

Use this as a fast recall map, not a replacement for the sections above:

- **Everything is IL first.** C# syntax is a lens onto IL + CLR services (GC, JIT, type system). When confused about behavior, ask "what would the lowered/IL form do?"
- **Value vs reference is about storage and copy semantics**, not about `struct` vs `class` in principle — but in C#, `struct` = value type, `class` = reference type, full stop.
- **Polymorphism = vtable dispatch on the runtime type**; overload resolution = compile-time dispatch on the static type. Never confuse `override` (dynamic) with `new`/hiding (static).
- **Generics are compile-time templates with runtime specialization** for value types, and shared code for reference types — you get type safety without the cost of boxing.
- **`async`/`await` never means "new thread."** It means "suspend here, free this thread, resume later via a continuation."
- **The GC frees managed memory; `IDisposable` frees everything else** (files, sockets, native handles) — deterministically, via `using`.
- **LINQ is lazy by default.** A query variable is a *recipe*, not a *result*, until you enumerate it.
- **Nullable reference types are a compile-time contract**, not a runtime check — `!` is you overriding the compiler's judgment, not a guarantee.

---

*End of guide.*

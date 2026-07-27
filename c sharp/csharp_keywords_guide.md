# C# Keywords — A Complete Engineering Reference

> Goal of this document: not just "what does `readonly` do" but **why the language needed it**, what problem it solves at the compiler/runtime level, and how it shows up in production code. Treat this like you'd treat learning a new instruction set — keywords are the ISA of the language; the CLR is the CPU.

---

## 0. Mental Model First — How to Organize 100+ Keywords in Your Head

Don't memorize a flat list. C# keywords fall into **8 functional buckets**. Once you see the bucket, the keyword's behavior becomes predictable.

```
                         ┌─────────────────────────────────────┐
                         │           C# KEYWORDS                │
                         └───────────────────┬───────────────────┘
                                              │
        ┌───────────────┬────────────────────┼────────────────────┬───────────────┐
        │               │                    │                    │               │
        ▼               ▼                    ▼                    ▼               ▼
   TYPE DECLARE     ACCESS/MODIFIER     CONTROL FLOW         DATA/MEMORY      META/OPERATOR
   class struct     public private     if else switch       ref out in       is as typeof
   interface enum   protected internal for foreach while     stackalloc       sizeof nameof
   record delegate  static readonly    do goto break         fixed unsafe     checked unchecked
   event            const sealed       continue return       new default
                    abstract virtual   try catch finally
                    override extern    throw yield
                    volatile partial

        ┌───────────────┬────────────────────┬────────────────────┐
        │               │                    │                    │
        ▼               ▼                    ▼                    ▼
   ASYNC/ITERATOR   GENERICS            LINQ QUERY           NAMESPACE/MODULE
   async await      where in out(variance) from select      namespace using
   yield return     T constraints       where group join    global alias
                                        orderby let into
```

Every time you meet a new keyword, ask: **"Which bucket, and what compiler-level or runtime-level problem does it solve?"** That question is the whole skill.

---

## 1. Type Declaration Keywords

### 1.1 `class` — reference type, heap-allocated

```csharp
public class TcpConnection
{
    public string RemoteEndpoint { get; set; }
    public int Port { get; set; }
}
```

A `class` instance lives on the **managed heap**. The variable you hold is a **reference** (a pointer the GC tracks and can move during compaction).

```
Stack frame                Managed Heap
┌────────────────┐         ┌───────────────────────────┐
│ conn (ref) ────┼────────▶│ TcpConnection object       │
│ 0x00A3F210     │         │  [MethodTable ptr]         │
└────────────────┘         │  [SyncBlock]               │
                            │  RemoteEndpoint -> "10.0.0.1"│
                            │  Port = 443                │
                            └───────────────────────────┘
```

**Why it matters for you:** every `class` field access is a pointer dereference. Passing a class instance around is passing an 8-byte reference (on x64) — cheap, but aliasing means two variables can mutate the same object. This is the C# analogue of passing a `struct sock *` in the Linux kernel vs. copying a `struct sockaddr` by value.

### 1.2 `struct` — value type, typically stack-allocated

```csharp
public struct PacketHeader
{
    public byte Version;
    public ushort Length;
    public uint Checksum;
}
```

```
Stack frame
┌─────────────────────────────┐
│ hdr                          │
│  Version  = 4                │
│  Length   = 1500              │
│  Checksum = 0xDEAD             │
└─────────────────────────────┘
```

No pointer indirection, no GC pressure, but **copy semantics**: assigning or passing a struct copies all its bytes. This is exactly like passing `struct iphdr` by value in C — cheap for small structs, expensive if you make it fat (>16-24 bytes is the usual "start worrying" threshold in .NET).

**Engineering rule of thumb:** use `struct` for small, immutable, frequently-allocated data that models a *value* (a header, a coordinate, a checksum) — not an *entity* with identity. If you find yourself asking "are these two the same object," you wanted a `class`.

### 1.3 `record` / `record struct` — value-based equality, immutability by convention

```csharp
public record ConnectionKey(string SrcIp, int SrcPort, string DstIp, int DstPort);
```

`record` is sugar over a `class` (or `struct` with `record struct`) that auto-generates:
- Value-based `Equals`/`GetHashCode` (compares field-by-field, not reference identity)
- `ToString()`
- Non-destructive mutation via `with`:

```csharp
var flow1 = new ConnectionKey("10.0.0.1", 5000, "10.0.0.2", 443);
var flow2 = flow1 with { SrcPort = 5001 }; // new object, only SrcPort changed
```

**Why this matters in networking code:** a 5-tuple flow key is a *value*, not an entity — two flow keys with the same fields represent the *same flow*. Using a plain `class` here is a classic bug: `Dictionary<FlowKey, FlowState>` lookups silently fail because default `class` equality is reference equality, not structural equality. `record` fixes this by construction.

### 1.4 `interface` — contract, no state

```csharp
public interface IPacketFilter
{
    bool ShouldAllow(ReadOnlySpan<byte> packet);
}
```

Interfaces define *capability*, not *implementation*. Think of it as a header file declaring a function pointer table — conceptually close to a `struct ops` in the Linux kernel (like `struct file_operations`), which is exactly how the CLR implements it under the hood: an interface method table (IMT) resolved via a dispatch stub.

```
        IPacketFilter (contract)
                 △
      ┌──────────┼──────────┐
      │          │          │
 GeoIpFilter  RateLimiter  StatefulFwFilter
 (impl A)     (impl B)      (impl C)
```

### 1.5 `enum` — named integral constants

```csharp
public enum TcpState : byte
{
    Closed = 0,
    Listen = 1,
    SynSent = 2,
    SynReceived = 3,
    Established = 4,
    FinWait1 = 5,
    // ...
}
```

Backed by an integral type (default `int`, but you should pin it — `byte` here saves 3 bytes per field in hot structs). This maps directly to the kernel's `enum tcp_state` — same idea, same purpose: a closed, checkable state space instead of magic numbers.

Add `[Flags]` when values are meant to be OR'd together (bitmask), which is common in security/permission code:

```csharp
[Flags]
public enum FirewallAction
{
    None    = 0,
    Log     = 1 << 0,
    Drop    = 1 << 1,
    Reject  = 1 << 2,
    Alert   = 1 << 3,
}
// var action = FirewallAction.Log | FirewallAction.Drop;
```

### 1.6 `delegate` — a typed function pointer

```csharp
public delegate void PacketHandler(ReadOnlySpan<byte> packet, IPEndPoint source);
```

A `delegate` type describes a method signature. An instance of it is an object holding a pointer to a method **and** (optionally) the target object — this is the managed equivalent of a C function pointer, but it's a first-class heap object supporting multicast (subscribing multiple handlers).

```
delegate PacketHandler instance
┌────────────────────────────┐
│ Invocation list:            │
│  -> Logger.OnPacket         │
│  -> Firewall.Inspect        │
│  -> StatsCollector.Count    │
└────────────────────────────┘
```

`event` (below) is almost always layered on top of `delegate`.

### 1.7 `event`

```csharp
public class NetworkInterface
{
    public event PacketHandler OnPacketReceived;

    public void Simulate(ReadOnlySpan<byte> data, IPEndPoint src)
        => OnPacketReceived?.Invoke(data, src);
}
```

`event` restricts a `delegate` field's external API to `+=`/`-=` only (subscribers can't call `Invoke` or clear the whole list from outside) — this is an encapsulation keyword, not a new dispatch mechanism. Same category of idea as making a struct field `private` and exposing only a getter.

---

## 2. Access Modifiers — Who Can See This?

```
                     Visibility scope (narrowest → widest)
private  <  private protected  <  protected  <  internal  <  protected internal  <  public
```

| Keyword | Visible from |
|---|---|
| `private` | same class/struct only |
| `protected` | same class + derived classes |
| `internal` | same assembly (compiled DLL/EXE) only |
| `protected internal` | same assembly **OR** derived classes (anywhere) |
| `private protected` | same assembly **AND** derived classes (intersection, C# 7.2+) |
| `public` | anywhere |
| `file` | same source **file** only (C# 11+, used for source-generator-emitted helper types) |

**Why `internal` matters in real architecture:** it's how you enforce module boundaries without physically separating projects. If you're writing a security library, your crypto primitives implementation classes should be `internal` — only your vetted public API surface (`public`) is callable by consumers. This is the C# analogue of `static` functions in a `.c` file (translation-unit scoping) vs. functions declared in a shared header.

---

## 3. Modifier Keywords — Behavior Contracts on Members

### 3.1 `static`
Belongs to the type, not an instance. No `this`. Shared across all usages.

```csharp
public static class ChecksumUtil
{
    public static ushort ComputeInternetChecksum(ReadOnlySpan<byte> data) { ... }
}
```
Use for pure functions and utility state that has no per-instance identity — this is the same instinct as putting a helper in a `.c` file with no associated struct.

### 3.2 `readonly`
Assignable only in the declaration or a constructor. Enforced at compile time for the *reference itself* — for a `readonly` field of a mutable class, the reference can't be reassigned, but the object it points to can still mutate internally.

```csharp
public class RateLimiter
{
    private readonly int _maxTokens;         // set once, in ctor
    public RateLimiter(int maxTokens) => _maxTokens = maxTokens;
}
```

`readonly struct` (type-level) goes further: **every field** must be immutable, and the compiler can skip defensive copies when passing it by `in` — a real performance-relevant guarantee, not just style.

### 3.3 `const`
Compile-time constant, inlined at every call site (no runtime memory location at all, unlike `static readonly`).

```csharp
public const int MaxPacketSize = 65535;
```

**Danger:** if `MaxPacketSize` lives in `LibraryA.dll` and you `const`-reference it from `LibraryB.dll`, and you update `LibraryA` without recompiling `LibraryB`, `LibraryB` keeps using the **old baked-in value** — because `const` is a textual substitution at compile time, like a C `#define`. Use `static readonly` across assembly boundaries instead.

### 3.4 `sealed`
Prevents further inheritance (on a class) or further override (on a virtual member overridden in a derived class).

```csharp
public sealed class Sha256Hasher : IHasher { ... }
```

Two engineering reasons to seal: (1) security — stop someone injecting behavior via inheritance in a crypto/auth path; (2) performance — the JIT can devirtualize calls to sealed types (no vtable indirection needed), similar in spirit to `final` in Java/C++.

### 3.5 `abstract`
A class that cannot be instantiated and/or a member with no implementation, forcing derived types to provide one.

```csharp
public abstract class PacketDecoder
{
    public abstract bool TryDecode(ReadOnlySpan<byte> raw, out Packet packet);
    public string Name { get; init; } // concrete, shared
}
```

### 3.6 `virtual` / `override` / `new` (member-hiding)

```csharp
public class Filter
{
    public virtual bool Matches(Packet p) => true;
}
public class TcpOnlyFilter : Filter
{
    public override bool Matches(Packet p) => p.Protocol == Protocol.Tcp;
}
```

- `virtual`: method is dispatched **dynamically** (vtable lookup based on the *runtime* type).
- `override`: replaces the base's virtual implementation — participates in the same vtable slot.
- `new` on a member: **hides** the base member instead of overriding — it creates a *separate* slot. This is a classic footgun:

```
Filter f = new TcpOnlyFilter();
f.Matches(p);       // -> TcpOnlyFilter.Matches (virtual dispatch, correct)

Filter f2 = new TcpOnlyFilter();
((TcpOnlyFilter)f2).SomeNewHiddenMethod(); // only reachable via static type
```

If the static (compile-time) type and runtime type differ and the member was declared with `new` instead of `override`, **which implementation runs depends on the variable's declared type**, not the object's actual type. This is the single most common "why is my override not being called" bug for engineers coming from C/Rust where dispatch is either always static or explicitly a vtable/trait object.

### 3.7 `extern`
Declares a member implemented **outside** the C# assembly — typically P/Invoke into native code, which is directly relevant to your world (calling into `libpcap`, raw sockets via native syscalls, etc.):

```csharp
[DllImport("libc", SetLastError = true)]
public static extern int socket(int domain, int type, int protocol);
```

### 3.8 `volatile`
Tells the JIT/CLR **not to cache this field in a register or reorder** reads/writes to it across threads (a subset of what a full memory barrier gives you).

```csharp
private volatile bool _shutdownRequested;
```

**Important nuance for someone from kernel/systems background:** `volatile` in C# is *not* the same weak guarantee as C's `volatile` (which is really just "don't optimize away this read/write, no threading semantics implied"). C#'s `volatile` actually implies **acquire/release semantics** on that field. But it is still **not** a substitute for proper synchronization (`lock`, `Interlocked`, `Channel<T>`) for compound operations (read-modify-write). Use `Interlocked.CompareExchange` for anything beyond a simple flag.

### 3.9 `partial`
Splits a single type's definition across multiple files. Used heavily by source generators (e.g., a generator emits `MyClass.Generated.cs`, you write `MyClass.cs`, both declare `partial class MyClass`).

```csharp
// File1.cs
public partial class PacketParser { public void ParseHeader() { ... } }
// File2.cs
public partial class PacketParser { public void ParsePayload() { ... } }
```

### 3.10 `unsafe`
Opens a scope where pointer arithmetic and unmanaged pointer types are legal. Required for raw memory access — this is your bridge from managed C# into "C-like" code.

```csharp
public unsafe void ParseRaw(byte* buffer, int length)
{
    ushort version = *(ushort*)buffer;
    byte* payload = buffer + sizeof(IpHeader);
}
```

Requires `<AllowUnsafeBlocks>true</AllowUnsafeBlocks>` in the project file. The GC will not move objects referenced by raw pointers while you're in an unsafe/fixed context (see `fixed` below) — this is exactly analogous to pinning a DMA buffer so the kernel's memory manager can't relocate it mid-transfer.

---

## 4. Memory & Data-Passing Keywords

This bucket is the one that matters most for someone doing systems-level work, because it's about **how data moves**, not just syntax sugar.

### 4.1 `ref`, `out`, `in` — parameter passing modes

```
                Parameter passing decision tree
                        │
          Does the callee need to WRITE back to caller?
                 ┌──────┴──────┐
                YES             NO
                 │               │
     Does caller need to    Pass by value (default)
     pass in a value too?   or `in` if large struct,
        ┌───┴───┐            read-only, avoid copy
       YES      NO
        │        │
      `ref`    `out`
```

```csharp
// ref: caller must initialize, callee may read+write
public void Swap(ref int a, ref int b) { (a, b) = (b, a); }

// out: caller need NOT initialize, callee MUST assign before returning
public bool TryParseIp(string s, out IPAddress addr)
{
    return IPAddress.TryParse(s, out addr);
}

// in: pass a (usually large) struct by reference, but read-only —
// avoids copying, and the compiler enforces no mutation
public double DistanceSquared(in Point3D a, in Point3D b) { ... }
```

Under the hood, `ref`/`out`/`in` all compile to passing a **managed pointer** — the difference is purely a set of **compiler-enforced rules** about definite assignment and mutability, there's no separate runtime mechanism. This is worth internalizing: it's the compiler doing borrow-checking-lite, not the CLR.

### 4.2 `ref` on locals and returns — aliasing without unsafe

```csharp
public ref int FindSlot(int[] buffer, int index) => ref buffer[index];

ref int slot = ref FindSlot(buffer, 3);
slot = 42; // mutates buffer[3] directly, no array re-indexing
```

This gives you C-pointer-like aliasing semantics **without** `unsafe` — useful for hot-path buffer manipulation (packet ring buffers, zero-copy parsing) where you want to avoid both copies and unsafe pointer arithmetic.

### 4.3 `stackalloc`
Allocates a block directly on the **stack**, not the managed heap — no GC involvement, freed automatically on scope exit. This is your `alloca()`.

```csharp
Span<byte> buffer = stackalloc byte[256];
socket.Receive(buffer);
```

```
Stack frame
┌─────────────────────────────┐
│ locals...                    │
│ buffer[0..255] (raw bytes)   │  <- stackalloc'd, no heap object header
│ return address                │
└─────────────────────────────┘
```

**Constraint:** stack space is small (default 1MB thread stack) — never `stackalloc` a size that depends on unbounded/attacker-controlled input without a hard cap, or you've built a stack-overflow DoS vector. Always bound it:

```csharp
if (len > 4096) throw new ArgumentOutOfRangeException();
Span<byte> buf = stackalloc byte[len];
```

### 4.4 `fixed`
Pins a managed object in place (prevents the GC from moving it during compaction) so you can take its address safely for the duration of the block.

```csharp
byte[] managedBuffer = new byte[1500];
fixed (byte* p = managedBuffer)
{
    ParseRaw(p, managedBuffer.Length); // safe: GC won't relocate managedBuffer here
}
```

Directly analogous to `pin_user_pages()` / DMA-safe buffer handling in kernel driver code — the *reason* you need it (a background mover invalidating your pointer) is conceptually the same class of problem, just GC-compaction instead of page reclaim.

### 4.5 `new` (object creation) vs `new()` (target-typed) vs `new` (member hiding, §3.6)
Three unrelated meanings sharing one keyword — a good example of why "keyword" isn't the same unit as "concept":

```csharp
var conn = new TcpConnection();        // allocation
TcpConnection conn2 = new();            // target-typed new (C# 9+), type inferred from LHS
```

### 4.6 `default`
Produces the default value of a type: `0`/`false`/`null` for reference types, all-zero-bits for structs.

```csharp
int x = default;              // 0
TcpConnection c = default;    // null
PacketHeader h = default;     // all fields zeroed
T value = default(T);         // generic context, pre-C#7.1 syntax still valid
```

---

## 5. Control Flow Keywords

Mostly familiar from C — the interesting parts are the newer pattern-matching-flavored forms.

### 5.1 `if` / `else`, `for`, `while`, `do` — standard, skipping the basics.

### 5.2 `foreach` — iterator protocol, not raw indexing

```csharp
foreach (var packet in packetQueue) { ... }
```

Desugars to calling `GetEnumerator()`, then repeated `MoveNext()`/`Current` — this is why `foreach` works uniformly over arrays, `List<T>`, `IEnumerable<T>`, and your own custom collections, as long as they implement (or duck-type) the enumerator pattern. It's the C# equivalent of the iterator protocol — same category of abstraction as Rust's `Iterator` trait.

### 5.3 `switch` statement vs `switch` expression

```csharp
// statement form
switch (tcpState)
{
    case TcpState.SynSent:
        HandleSynSent();
        break;
    case TcpState.Established when connectionIsIdle:
        HandleIdleTimeout();
        break;
    default:
        break;
}

// expression form (C# 8+) — returns a value, exhaustive-checked by the compiler
string Describe(TcpState s) => s switch
{
    TcpState.Closed      => "no connection",
    TcpState.Listen      => "awaiting SYN",
    TcpState.Established => "data flowing",
    _                    => "transitional"
};
```

`when` adds a **guard clause** to a case — the case only matches if the pattern matches *and* the boolean guard is true.

### 5.4 Pattern matching keywords: `is`, `and`, `or`, `not`

```csharp
if (packet is { Protocol: Protocol.Tcp, Flags: TcpFlags.Syn and not TcpFlags.Ack })
{
    // SYN without ACK => new connection attempt
}
```

This is **structural pattern matching** — destructuring + type check + conditional in one expression. `and`/`or`/`not` are *contextual* pattern-combinators (only meaningful inside a pattern), not general boolean operators (those are still `&&`, `||`, `!`).

### 5.5 `goto`
Rare, but legitimate for: breaking out of nested loops cleanly, or falling through `switch` cases explicitly (C# disallows implicit fallthrough — you must `goto case X;`).

```csharp
switch (protocol)
{
    case Protocol.Tcp:
    case Protocol.Udp:
        HandleTransportLayer();
        break;
}
```
```csharp
switch (result)
{
    case ParseResult.Retry:
        goto case ParseResult.Pending; // explicit fallthrough
    case ParseResult.Pending:
        Requeue();
        break;
}
```

### 5.6 `break`, `continue`, `return`
Standard — `break` exits the nearest enclosing loop/switch, `continue` skips to the next iteration, `return` exits the method (optionally with a value).

---

## 6. Exception Handling Keywords

```csharp
public bool TryConnect(IPEndPoint endpoint, out Socket socket)
{
    socket = null;
    try
    {
        socket = new Socket(SocketType.Stream, ProtocolType.Tcp);
        socket.Connect(endpoint);
        return true;
    }
    catch (SocketException ex) when (ex.SocketErrorCode == SocketError.TimedOut)
    {
        Log.Warn("connect timed out");
        return false;
    }
    catch (SocketException ex)
    {
        Log.Error($"connect failed: {ex.SocketErrorCode}");
        throw;               // re-throw preserving original stack trace
    }
    finally
    {
        // runs whether or not an exception occurred, and whether or not
        // we returned early — used for guaranteed cleanup
        if (socket != null && !socket.Connected) socket.Dispose();
    }
}
```

Key discipline points:
- `catch (Ex ex) when (condition)` — **exception filters**: the `catch` block is only entered if the filter is true; otherwise the exception keeps propagating past this handler as if it didn't match. This lets you branch on error *subtype* (like `errno` on a syscall failure) without catching-and-rethrowing.
- `throw;` (bare) preserves the original stack trace. `throw ex;` **resets** the stack trace to the rethrow point — almost always a bug when done accidentally; know the difference cold.
- `finally` executes even if `return` happened inside `try`/`catch` — this is your RAII-substitute in a GC'd language; pair it with `using`/`IDisposable` for anything that must be released deterministically (sockets, file handles, native memory).

**Systems mindset translation:** exceptions are *not* your error-handling mechanism for expected failure paths (a socket timeout in a hot loop, a malformed packet). Reserve exceptions for truly exceptional conditions; use `Try*`-pattern methods (`TryParse`, `TryConnect` above) or discriminated result types for expected failure — exceptions in .NET are comparatively expensive (stack unwinding cost), the same way you wouldn't use `panic!`/`longjmp` for routine control flow.

---

## 7. Async / Iterator Keywords

### 7.1 `async` / `await` — compiler-generated state machine, not a new thread

```csharp
public async Task<Packet> ReceivePacketAsync(Socket socket, CancellationToken ct)
{
    byte[] buffer = new byte[1500];
    int n = await socket.ReceiveAsync(buffer, ct);   // yields control here
    return Packet.Parse(buffer.AsSpan(0, n));
}
```

**Critical mental model correction for systems engineers:** `async`/`await` does **not** spin up a new OS thread. The compiler rewrites the method into a state machine (conceptually similar to how a kernel driver's continuation-passing / callback-based I/O completion works). At each `await`, the method's local state is captured into a heap-allocated struct/class, control returns to the caller, and when the awaited operation completes (often via an I/O completion port / epoll-equivalent under the hood), execution resumes — potentially on a different thread pool thread.

```
Synchronous mental model (WRONG for async):
   caller -> ReceivePacketAsync -> [blocks OS thread] -> returns

Actual model:
   caller -> ReceivePacketAsync (state machine object created)
                    │
                    ▼
             await socket.ReceiveAsync(...)
                    │  (method RETURNS to caller here, thread freed)
                    ▼
        [socket layer registers completion callback with epoll/IOCP]
                    │
          ... other work runs on this thread ...
                    │
        [I/O completes] -> thread pool schedules continuation
                    ▼
        state machine resumes exactly after the `await`
                    ▼
             returns Packet to original awaiter
```

This is why `async` scales to tens of thousands of concurrent connections on a handful of OS threads — it's the userland analogue of epoll-driven event loops, not thread-per-connection.

- `Task` = "a promise of a value or void, eventually" (like a `future`/`promise` in other ecosystems).
- `ValueTask<T>` = a struct-based alternative to avoid heap allocation when the result is frequently already available synchronously (a hot-path optimization, e.g., reading from a buffer that's already filled).
- **Never** `.Result` or `.Wait()` on a `Task` from synchronous code without understanding deadlock risk (classic ASP.NET/UI-context deadlock from blocking on the calling thread while the continuation needs that same thread).

### 7.2 `yield return` / `yield break` — building an iterator without hand-writing the state machine

```csharp
public IEnumerable<Packet> ReadAllFrames(Stream stream)
{
    while (true)
    {
        var header = ReadHeader(stream);
        if (header == null) yield break;      // stop iterating
        yield return ParseFrame(stream, header); // produce one item, suspend here
    }
}
```

Like `async`, `yield` triggers compiler-generated state-machine rewriting — but for **pull-based synchronous iteration** instead of asynchronous continuation. Each call to `MoveNext()` resumes exactly where the last `yield return` left off. This gives you **lazy evaluation** — nothing in the loop body executes until someone actually iterates:

```csharp
var frames = ReadAllFrames(networkStream); // nothing has run yet
foreach (var f in frames.Take(5)) { ... }   // only pulls 5 frames, stream read lazily
```

---

## 8. Generics Keywords

```csharp
public class RingBuffer<T> where T : struct
{
    private readonly T[] _buffer;
    private int _head, _tail;

    public RingBuffer(int capacity) => _buffer = new T[capacity];

    public void Enqueue(in T item) { _buffer[_tail] = item; _tail = (_tail + 1) % _buffer.Length; }
    public T Dequeue() { var item = _buffer[_head]; _head = (_head + 1) % _buffer.Length; return item; }
}
```

### `where` — generic constraints

| Constraint | Meaning |
|---|---|
| `where T : struct` | T must be a value type |
| `where T : class` | T must be a reference type |
| `where T : new()` | T must have a public parameterless constructor |
| `where T : SomeBaseClass` | T must derive from SomeBaseClass |
| `where T : ISomeInterface` | T must implement the interface |
| `where T : unmanaged` | T must be a value type with no reference-type fields anywhere in its layout (crucial for interop / `stackalloc T[]`) |
| `where T : notnull` | T cannot be a nullable type |

`unmanaged` is the one to know cold for your kind of work — it's the constraint that lets you write **generic code that can still be pinned, `stackalloc`'d, or passed to native interop**, because the compiler has proven there are no hidden references/GC-tracked fields inside T.

```csharp
public unsafe Span<byte> AsBytes<T>(ref T value) where T : unmanaged
    => new Span<byte>(Unsafe.AsPointer(ref value), sizeof(T));
```

### Variance: `in` / `out` on generic type parameters (different meaning from §4.1!)

```csharp
public interface IPacketProducer<out T> { T Produce(); }   // covariant: safe to return more-derived
public interface IPacketConsumer<in T>  { void Consume(T item); } // contravariant: safe to accept less-derived
```

- `out T` (covariant): if `Dog : Animal`, then `IPacketProducer<Dog>` can be used where `IPacketProducer<Animal>` is expected — because the interface only ever *produces* T, never accepts it as input.
- `in T` (contravariant): `IPacketConsumer<Animal>` can be used where `IPacketConsumer<Dog>` is expected — because it only ever *consumes* T.

This is the same variance/subtyping reasoning as Rust's variance rules for lifetimes/trait objects or C++ template covariance discussions — the direction of "is it safe to substitute" flips depending on whether the type appears in input or output position.

---

## 9. Operator & Introspection Keywords

| Keyword | Purpose | Example |
|---|---|---|
| `is` | runtime type/pattern check | `if (obj is TcpPacket tcp)` |
| `as` | safe cast, `null` on failure (reference types/nullable only) | `var tcp = obj as TcpPacket;` |
| `typeof` | compile-time `Type` token for a named type | `typeof(TcpPacket)` |
| `sizeof` | size in bytes of an unmanaged type (needs `unsafe` unless the type is a built-in primitive) | `sizeof(int) == 4` |
| `nameof` | compile-time string of an identifier's name — refactor-safe | `nameof(RemoteEndpoint)` |
| `checked` / `unchecked` | force (or suppress) overflow exceptions on integer arithmetic | `checked { var x = int.MaxValue + 1; }` throws `OverflowException` |
| `typeof` vs `GetType()` | `typeof(X)` resolves at compile time from a static type name; `obj.GetType()` resolves at runtime from the actual object — different tools for different jobs |

**`checked`/`unchecked` — why this matters for security-relevant code:** integer overflow is a classic vector for length-calculation bugs (the C/C++ world's "integer overflow leads to buffer overflow" class of CVE). By default C# arithmetic is `unchecked` (wraps silently, like C). In parsing/validation code — anything computing a buffer size or offset from untrusted input — wrap the arithmetic in `checked` so overflow throws instead of wrapping into a small/negative number that then under-allocates a buffer:

```csharp
checked
{
    int totalLen = header.PayloadLength + HeaderSize; // throws OverflowException instead of wrapping
}
```

---

## 10. Namespace / Module Keywords

```csharp
namespace Acme.NetworkSecurity.Filtering;   // C# 10+ file-scoped namespace, no braces needed

using System.Net.Sockets;
using Filter = Acme.NetworkSecurity.Filtering.IPacketFilter; // alias
global using System;  // (in a single file, usually GlobalUsings.cs) applies project-wide
```

- `namespace`: logical grouping / collision-avoidance — purely a compile-time construct, not something that exists at runtime (unlike, say, kernel namespaces which are a real isolation primitive — don't let the shared word mislead you).
- `using` (directive): imports a namespace's types into scope. **Different** from `using` (statement, below) — same keyword, two unrelated jobs.
- `using` (statement/declaration) — deterministic disposal:

```csharp
using (var socket = new Socket(...))
{
    // socket.Dispose() guaranteed to run at the closing brace, even on exception
}

// C# 8+ "using declaration" — disposes at end of enclosing scope, less nesting
using var socket2 = new Socket(...);
```

This is your closest thing to RAII in C# — pairs with `IDisposable`. Anything wrapping an OS handle (socket, file, native memory, crypto context) in a well-designed library implements `IDisposable`, and you should `using` it, full stop.

---

## 11. LINQ Query Keywords

LINQ has two equivalent forms — query syntax (SQL-like keywords) and method syntax (fluent chain). Know both; production code mostly uses method syntax, but query syntax is clearer for multi-source joins.

```csharp
var suspiciousFlows =
    from packet in capturedPackets
    where packet.Protocol == Protocol.Tcp && packet.Flags.HasFlag(TcpFlags.Syn)
    group packet by packet.SourceIp into g
    where g.Count() > 100          // SYN flood heuristic
    orderby g.Count() descending
    select new { SourceIp = g.Key, SynCount = g.Count() };
```

Equivalent method syntax:
```csharp
var suspiciousFlows = capturedPackets
    .Where(p => p.Protocol == Protocol.Tcp && p.Flags.HasFlag(TcpFlags.Syn))
    .GroupBy(p => p.SourceIp)
    .Where(g => g.Count() > 100)
    .OrderByDescending(g => g.Count())
    .Select(g => new { SourceIp = g.Key, SynCount = g.Count() });
```

| Query keyword | Method equivalent | Purpose |
|---|---|---|
| `from` | (source) | declares the range variable + source sequence |
| `where` | `.Where()` | filter predicate |
| `select` | `.Select()` | projection |
| `group ... by` | `.GroupBy()` | bucket elements by a key |
| `orderby` | `.OrderBy()`/`OrderByDescending()` | sort |
| `join` | `.Join()` | inner-join two sequences on a key |
| `let` | (compiler temp var) | introduce a named intermediate value in the query |
| `into` | (continuation) | re-scope a query after `group`/`select`, allowing further clauses |

**Performance note directly relevant to you:** LINQ over `IEnumerable<T>` is lazily evaluated and allocates iterator objects/closures — fine for control-plane/analysis code, but **avoid LINQ in a hot packet-processing path**. Use plain loops over `Span<T>`/arrays there. Know when you're in "throughput-critical" code vs. "occasional analysis" code, and choose accordingly — this judgment call is itself the engineering skill, not the syntax.

---

## 12. Boxing, `object`, `dynamic`, `var` — the "what even is the static type here" bucket

### `var` — compile-time type inference, **not** a dynamic type

```csharp
var count = 5;          // int, fixed at compile time
var packet = ParsePacket(buffer); // whatever ParsePacket's return type is
```
`var` is pure syntactic sugar resolved entirely by the compiler — the emitted IL is identical to writing the explicit type. Use it when the type is obvious from the right-hand side; avoid it when it hides something important (e.g., an interface vs. concrete type distinction that matters to the reader).

### `dynamic` — deferred, runtime-resolved typing (the real "different from var" keyword)

```csharp
dynamic config = JsonSerializer.Deserialize<ExpandoObject>(json);
Console.WriteLine(config.timeoutMs); // resolved at RUNTIME via the DLR, not compile time
```

Unlike `var`, `dynamic` **skips compile-time type checking entirely** — member resolution happens at runtime via the Dynamic Language Runtime, and a typo (`config.timeuotMs`) throws a `RuntimeBinderException` instead of failing to compile. Use sparingly — mostly for COM interop, dynamic JSON/config shapes, or scripting-host interop. It is the opposite philosophy from your day job (you want compile-time guarantees, not runtime surprises), so treat it as an escape hatch, not a default.

### Boxing/unboxing (`object`) — implicit, and a real performance/GC cost

```csharp
int x = 42;
object boxed = x;        // BOXING: heap-allocates a copy of x wrapped in an object header
int y = (int)boxed;      // UNBOXING: copies the value back out, with a runtime type check
```

```
Stack: x = 42 (4 bytes, no heap)

After boxing:
Stack: boxed (reference) ──▶ Heap: [MethodTable ptr][SyncBlock][42]
```

Every value type stuffed into an `object`-typed variable, an `ArrayList`, or a non-generic collection causes a heap allocation. This is invisible in the syntax — a classic hidden-cost bug for anyone whose instinct (correctly, from C/Rust) is "value types don't allocate." **Generics (`List<int>` instead of `ArrayList`) exist specifically to eliminate this** — internalize generics partly as "the boxing-elimination mechanism," not just "type-safe containers."

---

## 13. Full Reserved Keyword Reference Table

C# has **reserved keywords** (always keywords, can't be used as identifiers without `@` prefix) and **contextual keywords** (only special in specific syntactic positions, otherwise valid identifiers).

### Reserved keywords (partial, grouped)
```
abstract  as        base       bool      break     byte      case
catch     char      checked    class     const     continue  decimal
default   delegate  do         double    else      enum      event
explicit  extern    false      finally   fixed     float     for
foreach   goto      if         implicit  in        int       interface
internal  is        lock       long      namespace new       null
object    operator  out        override  params    private   protected
public    readonly  ref        return    sbyte     sealed    short
sizeof    stackalloc static    string    struct    switch    this
throw     true      try        typeof    uint      ulong     unchecked
unsafe    ushort    using      virtual   void      volatile  while
```

### Contextual keywords (only special in context — otherwise legal variable/method names)
```
add       alias      ascending  async      await      by         descending
dynamic   equals     from       get        global     group      into
join      let        nameof     nint       notnull    nuint      on
orderby   partial    record     remove     select     set        unmanaged
value     var        when       where      with       yield      file
required  scoped     init       managed    unmanaged
```

**Why the split exists:** contextual keywords were added in later language versions without breaking existing code that might already use `where`, `async`, `var`, etc. as identifiers — a real, deliberate backward-compatibility engineering decision. This is worth remembering as a *design pattern*, not just trivia: **when extending a stable public interface (a language, a protocol, an API), prefer additions that can't break existing valid usage.** Same principle as how you'd design a wire protocol's version negotiation or a syscall's flag bits (reserved-must-be-zero fields) to keep old clients working.

### `@identifier` escape
```csharp
int @class = 5; // legal: @ escapes a reserved word for use as an identifier
```
Rare, mostly seen when interop-generating code needs to use a name that happens to collide with a C# keyword.

---

## 14. `this`, `base`, `nameof`, `init`, `required` — construction & identity keywords

```csharp
public class TcpFilter : BaseFilter
{
    private readonly int _port;

    public TcpFilter(int port) : base(loggingEnabled: true) // call base ctor first
    {
        _port = port;
    }

    public override bool Matches(Packet p)
        => base.Matches(p) && p.DstPort == _port; // call base implementation, then extend
}
```

- `this`: refers to the current instance; also used for constructor chaining: `public Foo() : this(defaultValue) { }`
- `base`: refers to the immediate base class's member/constructor — required when you extend rather than replace inherited behavior.
- `init` (C# 9+): a property setter usable **only** during object initialization (constructor or object-initializer syntax), then becomes immutable — a middle ground between `readonly` fields and mutable properties:

```csharp
public class FirewallRule
{
    public string Name { get; init; }
    public FirewallAction Action { get; init; }
}
var rule = new FirewallRule { Name = "block-telnet", Action = FirewallAction.Drop };
// rule.Name = "x"; // compile error after construction
```

- `required` (C# 11+): forces callers to set a property/field during object initialization, enforced at compile time — catches "forgot to set a mandatory config field" bugs before runtime:

```csharp
public class TlsConfig
{
    public required string CertPath { get; init; }
    public required string KeyPath { get; init; }
    public int MinVersion { get; init; } = 12; // TLS 1.2 default, optional
}
// var cfg = new TlsConfig(); // COMPILE ERROR: CertPath and KeyPath not set
```

---

## 15. Putting It Together — A Realistic Production Example

A small but "real" piece: a token-bucket rate limiter guarding an API surface, written the way you'd actually ship it — touching type declaration, modifiers, generics, memory, async, and exception-safety keywords in one cohesive unit.

```csharp
namespace Acme.NetworkSecurity.RateLimiting;

using System;
using System.Threading;

/// <summary>
/// Thread-safe token-bucket rate limiter. One instance per (client, resource) key.
/// </summary>
public sealed class TokenBucketLimiter : IDisposable
{
    private readonly int _capacity;
    private readonly double _refillPerSecond;
    private readonly object _gate = new();     // lock object; 'new()' target-typed
    private double _tokens;
    private long _lastRefillTicks;
    private volatile bool _disposed;           // read across threads without a lock

    public TokenBucketLimiter(int capacity, double refillPerSecond)
    {
        if (capacity <= 0) throw new ArgumentOutOfRangeException(nameof(capacity));
        _capacity = capacity;
        _refillPerSecond = refillPerSecond;
        _tokens = capacity;
        _lastRefillTicks = DateTime.UtcNow.Ticks;
    }

    public bool TryAcquire(int cost = 1)
    {
        if (_disposed) throw new ObjectDisposedException(nameof(TokenBucketLimiter));

        lock (_gate)
        {
            Refill();
            if (_tokens < cost) return false;
            _tokens -= cost;
            return true;
        }
    }

    private void Refill()
    {
        long now = DateTime.UtcNow.Ticks;
        double elapsedSeconds = (now - _lastRefillTicks) / (double)TimeSpan.TicksPerSecond;
        if (elapsedSeconds <= 0) return;

        _tokens = Math.Min(_capacity, _tokens + elapsedSeconds * _refillPerSecond);
        _lastRefillTicks = now;
    }

    public void Dispose() => _disposed = true;
}
```

Every keyword choice here is deliberate, not decorative:
- `sealed` — this is a security-adjacent primitive; no one should be able to subclass and override `TryAcquire` to bypass the check.
- `readonly` on `_capacity`/`_refillPerSecond`/`_gate` — configuration and the lock object never change after construction; the compiler enforces it.
- `volatile` on `_disposed` — cheap cross-thread visibility for a simple flag; not used for `_tokens` because that needs full mutual exclusion (`lock`), not just visibility.
- `lock (_gate)` — protects the compound read-modify-write of `_tokens`/`_lastRefillTicks`; a private, dedicated object (not `this`) to avoid external code accidentally locking on the same monitor.
- `nameof(capacity)` — refactor-safe exception messages; renaming the parameter later can't silently desynchronize the string from the actual name.
- `IDisposable`/`Dispose()` — deterministic lifecycle signal, so callers can `using`/`using var` it if they manage limiter lifetime explicitly.

---

## 16. How to Actually Retain This (Study Method, Not Just Content)

1. **For every keyword you read above, ask two questions**: *(a)* what compiler-time rule does it enforce, and *(b)* what runtime behavior (if any) does it change. Some keywords are 100% compile-time (`const`, `nameof`, `readonly`'s enforcement), some are runtime-visible (`virtual` dispatch, `volatile` memory ordering, `async` state machines). Confusing these two categories is the #1 source of "why doesn't this do what I expected."
2. **Map every C# keyword to something you already know from C/Rust/kernel work.** You've done half of this document already by seeing the analogies (`struct` ↔ value type, `unsafe`/`fixed` ↔ pinning, `async`/`await` ↔ epoll+continuation, `sealed` ↔ devirtualization, `record` ↔ value semantics you'd hand-roll with `Eq`/`Hash` derives in Rust). Building the *mapping* is faster and stickier than memorizing C# in isolation.
3. **Write the 5 keywords that are least like anything in C/Rust from memory tomorrow, no reference:** `dynamic`, `yield return`, `event`, `in`/`out` variance on generics, `checked`/`unchecked`. These are the ones with no close analogue in your existing mental toolbox, so they're where forgetting will happen first.
4. **Build a second version of the rate-limiter example yourself** using a different keyword each time to intentionally break something (remove `sealed`, remove `lock`, change `volatile` to a plain field) and reason through *exactly* what class of bug you introduced. That's how you turn "I read what `volatile` does" into "I can debug a missing `volatile` in someone else's code at 2am."

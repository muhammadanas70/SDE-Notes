# Rust Keywords: Complete In-Depth Guide
## Mental Models, Real-World Usage, Fatal Mistakes, and Misinformation

> **Scope**: Every stable and reserved Rust keyword — semantics, ownership/borrow interaction,
> real production patterns, Linux kernel/cloud-native code, common fatal mistakes, and misinformation
> that traps experienced engineers. No hand-waving. No "just read the book" shortcuts.

---

## Summary (4–8 Lines)

Rust's keyword set is deliberately small (~40 active keywords) but each one encodes a **compiler
contract** — not merely syntax. The most dangerous mistakes come from treating keywords as
analogous to C/C++/Go equivalents when the semantics differ fundamentally (e.g., `mut`, `ref`,
`move`, `unsafe`, `static`, `dyn`). The second category of fatal issues stems from misunderstanding
how keywords interact with the borrow checker's lifetime inference, especially in async contexts
with `move`, `await`, and `Send`/`Sync` bounds. Misinformation around `unsafe` ("it disables the
borrow checker"), `static` ("it means the program lifetime in all contexts"), `extern` ("it turns
off safety"), and `dyn` vs `impl` costs causes both security vulnerabilities and performance
regressions. This guide builds a correct mental model for every keyword, with Linux-kernel-style
and cloud-native production code to anchor the abstract concepts.

---

## Table of Contents

1. [Keyword Taxonomy](#1-keyword-taxonomy)
2. [Ownership & Borrowing Keywords: `let`, `mut`, `ref`, `move`](#2-ownership--borrowing-keywords)
3. [Type System Keywords: `type`, `struct`, `enum`, `union`, `impl`, `trait`, `dyn`, `impl Trait`](#3-type-system-keywords)
4. [Control Flow Keywords: `if`, `else`, `match`, `loop`, `while`, `for`, `break`, `continue`, `return`](#4-control-flow-keywords)
5. [Function & Closure Keywords: `fn`, `const`, `static`, `async`, `await`, `move`](#5-function--closure-keywords)
6. [Module & Visibility Keywords: `mod`, `use`, `pub`, `crate`, `super`, `self`, `Self`](#6-module--visibility-keywords)
7. [FFI & Low-Level Keywords: `extern`, `unsafe`, `union`](#7-ffi--low-level-keywords)
8. [Generics & Lifetime Keywords: `where`, `for<'a>`, `impl Trait`, `dyn Trait`](#8-generics--lifetime-keywords)
9. [Special Value Keywords: `true`, `false`, `self`, `Self`](#9-special-value-keywords)
10. [Reserved but Unstable Keywords](#10-reserved-but-unstable-keywords)
11. [Keyword Interaction: The Dangerous Combinations](#11-keyword-interaction-the-dangerous-combinations)
12. [Complete Fatal Mistakes Reference](#12-complete-fatal-mistakes-reference)
13. [Misinformation Debunked](#13-misinformation-debunked)
14. [Cloud-Native & Linux Kernel Patterns](#14-cloud-native--linux-kernel-patterns)
15. [Threat Model: Keywords as Security Boundaries](#15-threat-model-keywords-as-security-boundaries)
16. [Next 3 Steps](#16-next-3-steps)

---

## 1. Keyword Taxonomy

```
RUST KEYWORD SPACE
══════════════════════════════════════════════════════════════════════════════

  OWNERSHIP / MEMORY          TYPE SYSTEM             CONTROL FLOW
  ┌─────────────────┐         ┌──────────────────┐    ┌──────────────────┐
  │ let             │         │ struct           │    │ if / else        │
  │ mut             │         │ enum             │    │ match            │
  │ ref             │         │ union            │    │ loop             │
  │ move            │         │ trait            │    │ while            │
  │ drop (fn, not   │         │ impl             │    │ for .. in        │
  │   keyword)      │         │ type (alias)     │    │ break [label]    │
  └─────────────────┘         │ dyn              │    │ continue [label] │
                              │ where            │    │ return           │
  FFI / UNSAFE                │ as               │    └──────────────────┘
  ┌─────────────────┐         └──────────────────┘
  │ unsafe          │                                 ASYNC
  │ extern          │         GENERICS / LIFETIME     ┌──────────────────┐
  │ union           │         ┌──────────────────┐    │ async            │
  └─────────────────┘         │ for<'a> (HRTB)   │    │ await            │
                              │ impl Trait       │    └──────────────────┘
  MODULE / VISIBILITY         │ dyn Trait        │
  ┌─────────────────┐         └──────────────────┘    FUNCTIONS / CONSTS
  │ mod             │                                 ┌──────────────────┐
  │ use             │         SPECIAL VALUES          │ fn               │
  │ pub             │         ┌──────────────────┐    │ const            │
  │ pub(crate)      │         │ true / false     │    │ static           │
  │ pub(super)      │         │ self             │    └──────────────────┘
  │ pub(in path)    │         │ Self             │
  │ crate (2018)    │         │ super            │    PATTERN MATCHING
  │ super           │         └──────────────────┘    ┌──────────────────┐
  │ self            │                                 │ ref (in pattern) │
  └─────────────────┘                                 │ @ bindings       │
                                                      │ _ (wildcard)     │
  RESERVED (FUTURE)                                   └──────────────────┘
  ┌─────────────────────────────────────────────────────────────────────┐
  │ abstract  become  box  do  final  macro  override  priv  try        │
  │ typeof  unsized  virtual  yield                                     │
  └─────────────────────────────────────────────────────────────────────┘

══════════════════════════════════════════════════════════════════════════════
```

---

## 2. Ownership & Borrowing Keywords

### `let`

**What it actually means**: `let` introduces a **binding** — it binds a name to a value in the
current scope. Crucially, `let` does **not** mean "declare a variable" in the C sense. It means
"move (or copy) the right-hand side value into this named slot, and the slot owns it unless a
borrow is taken."

**Compiler contract**: The binding takes ownership of the value. When the binding goes out of scope,
`Drop::drop` is called (if the type implements `Drop`). The borrow checker tracks the binding's
liveness from declaration to last use (NLL: Non-Lexical Lifetimes since Rust 2018).

**Shadowing**: `let` can shadow a prior binding of the same name. This is NOT rebinding — it
creates a **new** binding that hides the old one. The old value's destructor runs at the old
binding's scope end (NOT at the shadow point). This is a source of subtle bugs.

```rust
// Correct mental model of let
fn demonstrate_let_semantics() {
    // 'buf' owns the Vec allocation on heap
    let buf: Vec<u8> = Vec::with_capacity(4096);
    // buf's memory IS NOT freed here — it's moved into process_buffer
    process_buffer(buf);
    // buf is no longer accessible here — moved

    // Shadowing: new binding, same name, DIFFERENT type allowed
    let x: i32 = 42;
    let x: &str = "forty-two"; // new binding; old i32 dropped at end of scope
    println!("{}", x); // prints "forty-two"

    // Shadow with same type — common pattern for transformations
    let raw_bytes: Vec<u8> = read_packet();
    let raw_bytes: &[u8] = &raw_bytes; // shadow: now a borrow, not owned vec
    parse_packet(raw_bytes);
    // original Vec<u8> freed at end of this scope (the owned binding)
}

// Shadowing trap in security code:
fn security_trap_shadowing() {
    let secret = load_secret_key(); // owns key material
    // BAD: developer thinks old secret is "gone" after shadow
    let secret = derive_session_key(&secret); // old still alive until end of block!
    // The original key material is NOT wiped here — it lives until scope end
    // Fix: use explicit drop or a block scope
    {
        let original = load_secret_key();
        let session = derive_session_key(&original);
        zeroize(&original); // explicit wipe before drop
        drop(original);
        use_session_key(session);
    }
}
```

**`let else` (Rust 1.65+)**: Diverges the current control flow if pattern does not match. This is
crucial for early-return patterns without nesting.

```rust
// let-else: replaces nested if-let chains in parsers/validators
fn parse_netlink_msg(buf: &[u8]) -> Result<NetlinkMsg, ParseError> {
    let Some(header_bytes) = buf.get(..NLMSG_HDRLEN) else {
        return Err(ParseError::TooShort);
    };
    let Ok(header) = NetlinkHeader::from_bytes(header_bytes) else {
        return Err(ParseError::InvalidHeader);
    };
    // header is bound here, no nesting
    Ok(NetlinkMsg { header, payload: &buf[NLMSG_HDRLEN..] })
}
```

**`let` in `if let` / `while let`**: These are pattern-matching forms, not re-uses of binding
semantics. They test if a value matches a pattern AND destructure it simultaneously.

```rust
// if-let: option/result unwrapping without panic
fn drain_ring_buffer(rb: &mut RingBuffer) {
    while let Some(frame) = rb.pop_front() {
        process_frame(frame);
    }
    // When pop_front returns None, loop exits cleanly
}
```

---

### `mut`

**What it actually means**: `mut` is an annotation on a **binding** OR a **reference type**. These
are two completely different things and conflating them is the #1 source of confusion.

```
mut on binding:    let mut x = 5;      — the binding x can be reassigned / its value mutated
mut on reference:  &mut T              — this reference grants exclusive mutable access to T
```

**The exclusive access invariant**: `&mut T` is NOT just "a pointer that allows writes." It is a
**compile-time-enforced exclusive access token**. When a `&mut T` exists, the compiler guarantees:
1. No other reference (`&T` or `&mut T`) to the same data exists simultaneously
2. No other thread can access the data
3. You have full permission to rewrite the memory

This is the aliasing XOR mutation invariant — the foundation of Rust's memory safety model.

```rust
// mut on binding vs mut on reference — they are orthogonal
fn mut_orthogonality() {
    let mut owned = String::from("hello");
    // mut binding: can reassign 'owned' to point to different String
    owned = String::from("world"); // OK: reassignment of binding

    let immutable_owner = String::from("hello");
    // immutable binding: cannot reassign 'immutable_owner'
    // immutable_owner = String::from("world"); // ERROR

    // &mut T: exclusive mutable reference
    let r: &mut String = &mut owned;
    r.push_str("!"); // OK: mutating through &mut
    // let r2 = &owned; // ERROR: shared ref while &mut exists

    // &T: shared immutable reference
    let s1: &String = &immutable_owner;
    let s2: &String = &immutable_owner; // OK: multiple shared refs
}

// Real cloud pattern: mutable config hot-reload
pub struct DaemonConfig {
    pub listen_addr: SocketAddr,
    pub tls_cert_path: PathBuf,
    pub max_connections: usize,
}

pub fn reload_config(config: &mut DaemonConfig, new_cfg: DaemonConfig) {
    // &mut DaemonConfig: we have exclusive access, safe to overwrite
    *config = new_cfg; // atomic replace via move
}
```

**Fatal mistake: interior mutability confusion**

`mut` on a binding says nothing about whether the type allows mutation through shared references.
Types like `Cell<T>`, `RefCell<T>`, `Mutex<T>`, `RwLock<T>`, `AtomicXxx` provide **interior
mutability** — mutation through `&T` (shared reference). This is intentional and safe because these
types enforce their own invariants at runtime (or via hardware atomics).

```rust
use std::sync::{Arc, Mutex};

// The binding is NOT mut, but the data IS mutable — this confuses newcomers
fn interior_mutability_demo() {
    // No 'mut' on binding, yet we mutate through it
    let shared_counter: Arc<Mutex<u64>> = Arc::new(Mutex::new(0));
    let c = Arc::clone(&shared_counter);
    std::thread::spawn(move || {
        let mut guard = c.lock().unwrap(); // guard gives &mut u64
        *guard += 1;
    });
}

// FATAL MISTAKE: using mut where Mutex is needed in async
// This is DATA RACE territory if Send is incorrectly impl'd
struct BadSharedState {
    counter: u64, // NOT wrapped in Mutex
}
// impl Send for BadSharedState {} // NEVER do this manually for non-atomic fields
```

**`mut` in patterns**: When destructuring, `mut` binds a field as mutable.

```rust
struct Packet { seq: u32, payload: Vec<u8> }

fn process(mut pkt: Packet) {
    // 'mut pkt' means the local binding is mutable — we own it and can modify it
    pkt.seq += 1;
    pkt.payload.push(0xFF);
}

// Destructuring with mut
fn destructure(Packet { seq, mut payload }: Packet) {
    payload.push(0); // payload binding is mut, seq is not
}
```

---

### `ref`

**What it actually means**: `ref` is a **pattern modifier** used during **destructuring** to take
a reference to a value instead of moving it. It is almost exclusively used in `match`, `let`, and
function parameter patterns.

**Critical distinction**: `ref x` in a pattern IS NOT the same as `&x` in an expression.
- `&x` in an expression: creates a reference TO x (x must be accessible)
- `ref x` in a pattern: the pattern binds by reference rather than by move

```rust
// ref in match — prevents moving out of matched value
fn process_event(event: &NetworkEvent) {
    match event {
        NetworkEvent::PacketReceived { ref payload, seq } => {
            // 'payload' is &Vec<u8> — not moved, just borrowed
            // 'seq' is Copy so no ref needed
            analyze_payload(payload);
        }
        NetworkEvent::ConnectionClosed { ref peer } => {
            log_disconnection(peer);
        }
    }
}

// ref mut in pattern — mutable borrow during destructuring
fn modify_in_place(events: &mut Vec<NetworkEvent>) {
    for event in events.iter_mut() {
        if let NetworkEvent::PacketReceived { ref mut payload, .. } = event {
            payload.push(0); // mutate through ref mut
        }
    }
}
```

**When `ref` is rarely needed in modern Rust**: Since Rust 2018 match ergonomics ("match
ergonomics RFC"), patterns automatically dereference and infer `ref`/`ref mut` in many cases.
This means `ref` is mostly seen in pre-2018 code or in explicit disambiguation.

```rust
// Modern Rust — match ergonomics auto-infers ref
fn modern_match(opt: &Option<String>) {
    match opt {
        Some(s) => println!("{}", s), // s is &String, ref inferred automatically
        None => {}
    }
}

// Pre-2018 or explicit — need 'ref'
fn explicit_ref(opt: &Option<String>) {
    match opt {
        &Some(ref s) => println!("{}", s), // explicit deref + ref
        &None => {}
    }
}
```

**Fatal mistake with `ref` and ownership**: Using `ref` in a `let` binding on an owned value
creates a temporary borrow, not a moved binding. The original binding retains ownership.

```rust
fn ref_binding_trap() {
    let data = vec![1u8, 2, 3];
    let ref r = data; // r: &Vec<u8>, data still owns the Vec
    // drop(data); // ERROR: data is borrowed by r
    println!("{:?}", r);
    // data is dropped here after r's last use
}
```

---

### `move`

**What it actually means**: `move` forces a closure or async block to **take ownership** of all
captured variables from the enclosing scope, rather than borrowing them.

Without `move`: closures capture by the minimal borrow needed (borrow if possible, move if must).
With `move`: ALL captured variables are moved into the closure, regardless of whether borrowing
would suffice.

```rust
// ARCHITECTURE: Thread spawning with move
//
//  Main thread stack          Spawned thread stack
//  ┌─────────────┐            ┌─────────────────┐
//  │ config: Arc │──clone──►  │ config (owned)  │
//  │ socket: Fd  │──move──►   │ socket (owned)  │
//  │ name: String│──move──►   │ name (owned)    │
//  └─────────────┘            └─────────────────┘
//  (main cannot use socket    (thread has full ownership,
//   or name after spawn)       no dangling ref possible)

fn spawn_handler(socket: TcpStream, config: Arc<Config>, name: String) {
    // move: socket, config, name are ALL moved into the closure
    std::thread::spawn(move || {
        handle_connection(socket, &config, &name);
    });
    // socket and name cannot be used here — moved
    // config's Arc was cloned before spawn if needed elsewhere
}
```

**`move` in async blocks**: This is where most async bugs occur. An async block is essentially a
state machine (a `Future`). Without `move`, captured borrows must outlive the entire `Future`'s
lifetime, which is often impossible to prove.

```rust
use tokio::net::TcpListener;

// FATAL PATTERN WITHOUT move — borrow lifetime issue
async fn bad_server_loop() {
    let listener = TcpListener::bind("0.0.0.0:8080").await.unwrap();
    let config = Arc::new(load_config());

    loop {
        let (stream, peer) = listener.accept().await.unwrap();
        // WITHOUT move: config borrow must outlive the spawned task
        // Since the task can outlive this function's frame, borrow checker rejects it
        // tokio::spawn(async { handle(stream, &config).await }); // ERROR

        // WITH move: config (Arc clone) is moved into task — correct
        let cfg = Arc::clone(&config);
        tokio::spawn(async move {
            handle(stream, cfg).await;
        });
    }
}
```

**`move` does NOT deep-clone**: A critical misinformation. `move` moves the VALUE, not a clone.
For `Arc<T>`, moving an `Arc` means the Arc pointer is moved (reference count unchanged); you
must explicitly `Arc::clone()` if you need both the original and the moved copy.

```rust
fn move_arc_trap() {
    let shared = Arc::new(vec![1, 2, 3]);
    let task = {
        // WRONG: if you intend to use 'shared' after this block
        // move will MOVE the Arc — the original binding is invalid
        let s = Arc::clone(&shared); // explicit clone of Arc (cheap, just increments refcount)
        async move { process(s).await }
    };
    // shared is still valid here because we cloned before move
    println!("len: {}", shared.len());
}
```

---

## 3. Type System Keywords

### `struct`

**What it actually means**: `struct` defines a **product type** — a named composite of zero or more
fields. In Rust, structs are value types by default: they live on the stack unless boxed. The
compiler lays out struct fields in declaration order by default (subject to alignment padding), BUT
this is NOT guaranteed for `repr(Rust)` (the default). For FFI or hardware interfaces, always use
`#[repr(C)]`.

```rust
// repr(Rust) — default, compiler may reorder for optimization
struct NetworkStats {
    rx_bytes: u64,      // 8 bytes
    rx_packets: u32,    // 4 bytes + 4 padding (default repr may reorder)
    tx_bytes: u64,      // 8 bytes
    tx_packets: u32,    // 4 bytes + 4 padding
}

// repr(C) — stable layout, matches C struct, required for FFI/ioctl/kernel
#[repr(C)]
struct IfReq {
    ifr_name: [u8; 16], // IFNAMSIZ
    ifr_ifindex: i32,
}

// repr(packed) — no padding, use for protocol headers (careful with alignment)
#[repr(C, packed)]
struct EthernetHeader {
    dst_mac: [u8; 6],
    src_mac: [u8; 6],
    ether_type: u16,
}

// repr(transparent) — single-field newtype, same layout as inner type
// Used for zero-cost abstraction wrappers
#[repr(transparent)]
struct FileDescriptor(i32);

impl FileDescriptor {
    pub fn raw(&self) -> i32 { self.0 }
}
```

**Tuple structs**: Structs with unnamed positional fields. Common for newtypes.

```rust
struct Port(u16);      // newtype — prevents mixing Port with u16 directly
struct IpAddr([u8; 4]);

fn connect(addr: IpAddr, port: Port) { /* ... */ }
// connect(Port(80), IpAddr([127,0,0,1])); // ERROR: args in wrong order — newtype caught it
```

**Unit structs**: Zero-sized types (ZST). Used as marker types, phantom data, or singleton states.

```rust
struct TlsEstablished;   // ZST — zero runtime cost, compile-time state marker
struct Encrypted;
struct Plaintext;

// Typestate pattern using unit structs and generics
struct Connection<State> {
    fd: i32,
    _state: std::marker::PhantomData<State>,
}

impl Connection<Plaintext> {
    pub fn establish_tls(self) -> Connection<TlsEstablished> {
        do_tls_handshake(self.fd);
        Connection { fd: self.fd, _state: std::marker::PhantomData }
    }
}

impl Connection<TlsEstablished> {
    pub fn send(&mut self, data: &[u8]) { /* only callable after TLS */ }
}
```

---

### `enum`

**What it actually means**: `enum` defines a **sum type** — a value that is exactly one of several
named variants. Unlike C enums (which are just integer aliases), Rust enums are tagged unions that
can carry data per variant. The tag (discriminant) is stored alongside the data; the compiler
chooses the optimal size.

```rust
// Rich enum — each variant can have different data
#[derive(Debug)]
enum VxlanEvent {
    TunnelUp { vtep_ip: Ipv4Addr, vni: u32 },
    TunnelDown { vtep_ip: Ipv4Addr, reason: String },
    MacLearned { mac: [u8; 6], vtep_ip: Ipv4Addr, vni: u32 },
    Timeout,  // unit variant, no data
}

// Memory layout — compiler chooses discriminant size
// For VxlanEvent: size = tag + largest variant's data (with alignment)

// FATAL MISTAKE: assuming enum variants are separately addressable
// You CANNOT have a *VxlanEvent::TunnelUp — the variant is NOT a separate type
```

**`Option<T>` and `Result<T, E>` are just enums**: Understanding this is essential.
`Option::None` has the same type as `Option::Some(x)` — they are variants of the same type.
The compiler can optimize `Option<NonNull<T>>` to the size of a pointer (null pointer optimization).

```rust
// Result<T,E> patterns in systems code
fn read_netlink_socket(fd: i32, buf: &mut [u8]) -> Result<usize, std::io::Error> {
    let n = unsafe { libc::recv(fd, buf.as_mut_ptr() as *mut _, buf.len(), 0) };
    if n < 0 {
        Err(std::io::Error::last_os_error())
    } else {
        Ok(n as usize)
    }
}

// Chaining Results — the ? operator desugars to early return on Err
fn parse_and_send(fd: i32, raw: &[u8]) -> Result<(), Box<dyn std::error::Error>> {
    let msg = NetlinkMsg::parse(raw)?;   // ? returns Err early
    let encoded = msg.encode()?;
    send_netlink(fd, &encoded)?;
    Ok(())
}
```

**Non-exhaustive enums** (`#[non_exhaustive]`): Declares that future variants may be added (library
crates use this for stability). Downstream code MUST have a wildcard arm in match.

```rust
#[non_exhaustive]
pub enum CloudProvider {
    Aws,
    Gcp,
    Azure,
}
// Downstream:
// match provider {
//     CloudProvider::Aws => ...,
//     _ => ..., // REQUIRED because of #[non_exhaustive]
// }
```

---

### `union`

**What it actually means**: `union` defines a C-style union — all fields share the same memory.
Unlike C, Rust unions require `unsafe` to read fields because the compiler cannot guarantee which
variant is active. Unions are primarily for FFI, hardware register access, and performance-critical
low-level code.

```rust
// Kernel-style netlink attribute union
#[repr(C)]
union NlAttrData {
    u32_val: u32,
    u64_val: u64,
    raw: [u8; 8],
}

// Safe wrapper using enum (preferred over raw union in most cases)
fn read_nl_attr(attr: &NlAttr) -> u64 {
    match attr.attr_type {
        NLA_U32 => unsafe { attr.data.u32_val as u64 },
        NLA_U64 => unsafe { attr.data.u64_val },
        _ => 0,
    }
}

// Union for type punning (common in network packet parsing)
#[repr(C)]
union IpUnion {
    addr: u32,          // network byte order
    octets: [u8; 4],
}

fn ip_to_octets(ip: u32) -> [u8; 4] {
    unsafe { IpUnion { addr: ip }.octets }
}
```

**Fatal mistake**: Accessing a union field that was not the last one written is undefined behavior
in Rust (same as C). Never assume the compiler will "just read the bytes" correctly without the
correct variant being active.

---

### `trait`

**What it actually means**: `trait` defines an **interface contract** — a set of methods (and
associated types/constants) that a type must implement to satisfy the trait. Traits are NOT
abstract base classes (no vtable by default). They are **zero-cost** unless used as `dyn Trait`
(which introduces a vtable).

```rust
// Trait definition
pub trait PacketProcessor: Send + Sync {
    type Error: std::error::Error + Send + 'static;

    fn process(&mut self, packet: &[u8]) -> Result<ProcessedPacket, Self::Error>;

    // Default implementation
    fn process_batch(&mut self, packets: &[&[u8]]) -> Vec<Result<ProcessedPacket, Self::Error>> {
        packets.iter().map(|p| self.process(p)).collect()
    }

    // Required associated constant
    const MAX_PACKET_SIZE: usize;
}

// Trait implementation
struct XdpProcessor {
    map_fd: i32,
}

impl PacketProcessor for XdpProcessor {
    type Error = std::io::Error;
    const MAX_PACKET_SIZE: usize = 9000; // jumbo frames

    fn process(&mut self, packet: &[u8]) -> Result<ProcessedPacket, Self::Error> {
        if packet.len() > Self::MAX_PACKET_SIZE {
            return Err(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                "packet too large"
            ));
        }
        parse_ethernet_frame(packet)
    }
}
```

**Trait objects (`dyn Trait`)**: When you need runtime polymorphism (the concrete type is unknown
at compile time), use `dyn Trait`. This introduces a **vtable pointer** — a fat pointer (data ptr
+ vtable ptr). This is a runtime cost; prefer static dispatch (`impl Trait` or generics) when the
type is known at compile time.

```
Static dispatch (monomorphization):          Dynamic dispatch (vtable):
┌──────────────────────────────┐             ┌──────────────────────────────┐
│ fn process<P: Processor>(p)  │             │ fn process(p: &dyn Processor)│
│                              │             │                              │
│ Compiler generates:          │             │ Fat pointer layout:          │
│  process_XdpProcessor(p)     │             │ ┌──────────┬──────────────┐  │
│  process_EbpfProcessor(p)    │             │ │ data ptr │ vtable ptr   │  │
│  (separate compiled copies)  │             │ └──────────┴──────────────┘  │
│  ■ Zero runtime overhead     │             │ vtable: [drop, process, ...]  │
│  ■ Larger binary (code bloat)│             │ ■ One compiled copy          │
│  ■ Inlining possible         │             │ ■ Virtual dispatch overhead  │
└──────────────────────────────┘             │ ■ No inlining across dyn     │
                                             └──────────────────────────────┘
```

```rust
// Static dispatch — preferred for hot paths
fn process_packet_static<P: PacketProcessor>(processor: &mut P, pkt: &[u8]) {
    processor.process(pkt).unwrap();
}

// Dynamic dispatch — use when type varies at runtime (plugin systems, etc.)
fn process_packet_dynamic(processor: &mut dyn PacketProcessor<Error=std::io::Error>, pkt: &[u8]) {
    processor.process(pkt).unwrap();
}

// Box<dyn Trait> — heap-allocated trait object (owned)
fn make_processor(provider: &str) -> Box<dyn PacketProcessor<Error=std::io::Error>> {
    match provider {
        "xdp"  => Box::new(XdpProcessor::new()),
        "ebpf" => Box::new(EbpfProcessor::new()),
        _      => panic!("unknown provider"),
    }
}
```

---

### `impl`

**What it actually means**: `impl` has two distinct syntactic roles:
1. **`impl Type`**: Defines methods/associated functions for a type (inherent impl block)
2. **`impl Trait for Type`**: Implements a trait for a type
3. **`impl Trait` in position**: (see §8) — anonymous generic parameter

```rust
struct RingBuffer {
    data: Vec<u8>,
    head: usize,
    tail: usize,
    capacity: usize,
}

// Inherent impl block — methods that belong to RingBuffer regardless of traits
impl RingBuffer {
    pub fn new(capacity: usize) -> Self {
        RingBuffer {
            data: vec![0u8; capacity],
            head: 0,
            tail: 0,
            capacity,
        }
    }

    pub fn push(&mut self, byte: u8) -> bool {
        let next = (self.tail + 1) % self.capacity;
        if next == self.head { return false; } // full
        self.data[self.tail] = byte;
        self.tail = next;
        true
    }

    pub fn pop(&mut self) -> Option<u8> {
        if self.head == self.tail { return None; } // empty
        let val = self.data[self.head];
        self.head = (self.head + 1) % self.capacity;
        Some(val)
    }
}

// Trait impl — implements std::io::Read for RingBuffer
impl std::io::Read for RingBuffer {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        let mut written = 0;
        for slot in buf.iter_mut() {
            match self.pop() {
                Some(b) => { *slot = b; written += 1; }
                None => break,
            }
        }
        Ok(written)
    }
}
```

**`impl` blocks can be split across multiple blocks**: A type can have multiple `impl` blocks.
This is commonly used to group related methods or to conditionally compile methods.

```rust
impl RingBuffer {
    // Core methods
}

#[cfg(feature = "metrics")]
impl RingBuffer {
    // Metrics-gated methods only compiled when feature is enabled
    pub fn utilization(&self) -> f64 { /* ... */ }
}
```

---

### `type`

**What it actually means**: `type` creates a **type alias** — a new name for an existing type.
It does NOT create a new type (no newtype semantics, no new impl scope). The alias and the original
type are completely interchangeable. For a true new type, use a tuple struct: `struct Foo(Bar)`.

```rust
// Type alias — just a name
type Result<T> = std::result::Result<T, std::io::Error>;
type Fd = i32;         // NOT a new type — Fd and i32 are identical to the compiler
type IpPort = (u32, u16);

// Associated type in trait
trait NetworkAdapter {
    type Frame: AsRef<[u8]>;  // associated type — each impl specifies concrete type
    fn recv(&mut self) -> Option<Self::Frame>;
}

struct EthernetAdapter { /* ... */ }
impl NetworkAdapter for EthernetAdapter {
    type Frame = Vec<u8>;
    fn recv(&mut self) -> Option<Vec<u8>> { todo!() }
}

// FATAL: type alias does NOT create distinct types for safety
type Vni = u32;
type Vtep = u32;
fn create_tunnel(vni: Vni, vtep: Vtep) { /* ... */ }
// create_tunnel(vtep_id, vni_id); // compiles! alias provides NO type safety
// Use newtype struct instead:
struct Vni2(u32);
struct Vtep2(u32);
// fn create_tunnel2(vni: Vni2, vtep: Vtep2) — now swapped args give compile error
```

---

## 4. Control Flow Keywords

### `match`

**What it actually means**: `match` is exhaustive pattern matching. It forces you to handle every
possible value of a type. The compiler statically verifies exhaustiveness — you cannot forget a
case. Arms are tried top-to-bottom; the first matching arm executes.

**Mental model**: `match` is NOT a switch statement. It is a destructuring + branching operation
that simultaneously:
1. Tests which pattern matches the value
2. Destructures the value (binds inner components to names)
3. Executes the arm's body

```rust
// Exhaustive matching on enum
fn handle_syscall_result(result: Result<usize, Errno>) -> i64 {
    match result {
        Ok(n) => n as i64,
        Err(Errno::EINTR) => -1,  // interrupted — caller retries
        Err(Errno::EAGAIN) | Err(Errno::EWOULDBLOCK) => -11,  // or-pattern
        Err(e) => -(e.code() as i64),  // catch-all with binding
    }
}

// Match guards — additional conditions after pattern
fn classify_port(port: u16) -> &'static str {
    match port {
        0 => "reserved",
        1..=1023 => "privileged",          // range pattern
        1024..=49151 => "registered",
        p if p > 49151 => "ephemeral",     // match guard
        _ => unreachable!(),               // compiler may still require _
    }
}

// Destructuring nested structures
fn parse_vxlan_header(header: VxlanHeader) -> (u32, [u8; 6]) {
    match header {
        VxlanHeader {
            flags,
            vni,
            inner_mac: MacAddr { octets: mac },
        } if flags & VXLAN_VALID_BIT != 0 => (vni, mac),
        _ => panic!("invalid VXLAN header"),
    }
}
```

**`match` and ownership**: The matched value is consumed (moved) by the match arm that matches
it, UNLESS you match on a reference.

```rust
fn ownership_in_match(opt: Option<String>) {
    match opt {
        Some(s) => { /* s: String, moved out of opt */ println!("{}", s); }
        None => {}
    }
    // opt is consumed — cannot use after match

    // Match on reference: no move
    let opt2 = Some(String::from("hello"));
    match &opt2 {
        Some(s) => println!("{}", s), // s: &String
        None => {}
    }
    // opt2 still valid
    let _ = opt2;
}
```

---

### `loop`, `while`, `for`

**`loop`**: Infinite loop with explicit `break`. Unique Rust feature: `loop` can return a value.

```rust
// loop as expression — returns value via break
fn wait_for_connection(listener: &TcpListener) -> TcpStream {
    loop {
        match listener.accept() {
            Ok((stream, _)) => break stream,  // break with value
            Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                std::thread::sleep(Duration::from_millis(10));
                continue;
            }
            Err(e) => panic!("accept error: {}", e),
        }
    }
}

// Labeled loops — break/continue specific outer loop
'outer: for i in 0..N {
    'inner: for j in 0..M {
        if condition(i, j) {
            break 'outer;  // breaks outer loop, not inner
        }
        if skip(i, j) {
            continue 'outer; // advances outer loop's counter
        }
    }
}
```

**`while let`**: Loop while a pattern matches. Essential for iterators and option/result chains.

```rust
fn drain_channel<T>(rx: &mut Receiver<T>) -> Vec<T> {
    let mut items = Vec::new();
    while let Ok(item) = rx.try_recv() {
        items.push(item);
    }
    items
}
```

**`for` + `IntoIterator`**: `for x in expr` desugars to `IntoIterator::into_iter(expr)` then
repeated `Iterator::next()` calls. Understanding this is crucial for ownership:

```rust
// for consumes the iterator (takes ownership of it)
let v = vec![1u8, 2, 3];
for x in v { /* x: u8, v is consumed */ }
// v is invalid here

// for over reference — borrows
let v = vec![1u8, 2, 3];
for x in &v { /* x: &u8, v still valid */ }
for x in &mut v { /* x: &mut u8 */ }
```

---

### `break` and `continue`

Both support **labels** for nested loops (shown above) and `break` can carry a **value** from `loop`.

**`break` from `while`/`for` cannot carry a value** — only `loop` expressions can. This is a
common source of confusion.

```rust
// VALID: break with value from loop
let result: i32 = loop {
    if done() { break 42; }
};

// INVALID: break with value from while
// let result = while cond() { break 42; }; // ERROR
```

---

### `if` and `else`

`if` is an expression in Rust — it returns a value. Both arms must have the same type. The `else`
branch is required when `if` is used as an expression (unless both arms return `()`).

```rust
// if as expression
let max_conns: usize = if is_production { 10_000 } else { 100 };

// if-let chains (Rust 1.64+)
fn validate_packet(raw: &[u8]) -> bool {
    if let Some(eth) = parse_ethernet(raw)
        && let Some(ip) = parse_ip(eth.payload())
        && ip.version() == 4
    {
        true
    } else {
        false
    }
}
```

---

### `return`

`return` is an early exit from the current function. Rust idiom favors implicit returns (the last
expression in a block is the return value). Explicit `return` is used for early exits.

**Fatal mistake**: Putting a semicolon on the last expression changes its type to `()`.

```rust
fn add(a: i32, b: i32) -> i32 {
    a + b   // implicit return — no semicolon
}

fn broken_add(a: i32, b: i32) -> i32 {
    a + b;  // WRONG: semicolon makes this ()  — compile error
}

fn early_return(input: &[u8]) -> Option<Packet> {
    if input.len() < MIN_PACKET_SIZE {
        return None;  // early return
    }
    Some(parse(input))  // implicit return
}
```

---

## 5. Function & Closure Keywords

### `fn`

**What it actually means**: `fn` defines a function item. In Rust, functions are distinct from
closures — a function cannot capture its environment (it has no captures). A function pointer
`fn(Args) -> Ret` is a zero-sized type (you can take its address, but the function itself has no
data).

**`fn` vs `Fn`/`FnMut`/`FnOnce`**: These are DIFFERENT things.
- `fn`: keyword that defines a function
- `fn(T) -> U`: function pointer type (no captures)
- `Fn`, `FnMut`, `FnOnce`: traits that closures (and function pointers) implement

```rust
// Function pointer — no captures, coerceable to fn type
fn add_one(x: i32) -> i32 { x + 1 }

// Taking a function pointer parameter
fn apply(f: fn(i32) -> i32, x: i32) -> i32 { f(x) }
let result = apply(add_one, 5); // 6

// Closure — captures environment, implements Fn/FnMut/FnOnce
let offset = 10;
let add_offset = |x: i32| x + offset; // captures offset by ref
// apply(add_offset, 5); // ERROR: closure != fn pointer (unless no captures)

// Generic function that accepts any callable
fn apply_any<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 { f(x) }
apply_any(add_one, 5);     // works: fn implements Fn
apply_any(add_offset, 5);  // works: closure implements Fn
```

**`fn` in trait definitions**: A `fn` inside a `trait` without a body is required to be
implemented (abstract). A `fn` with a body is a default implementation (can be overridden).

---

### `const`

**What it actually means**: `const` defines a **compile-time constant**. Constants:
1. Must have a type annotation
2. Must be computed at compile time (only const-evaluable expressions)
3. Are inlined at every use site — they do NOT have a stable memory address
4. Live for the entire program (implicitly `'static`)

**`const fn`**: A function that CAN be called at compile time (but may also be called at runtime).
This is the foundation of compile-time computation in Rust.

```rust
const MAX_PACKET_SIZE: usize = 9000; // inlined everywhere it's used
const ETHERTYPE_IPV4: u16 = 0x0800;
const ETHERTYPE_IPV6: u16 = 0x86DD;

// const fn — evaluated at compile time when arguments are const
const fn compute_ring_size(base: usize) -> usize {
    base.next_power_of_two() // power-of-2 for efficient modulo
}

const RING_SIZE: usize = compute_ring_size(1000); // = 1024, at compile time

// Const generics — type-level constants
struct FixedBuf<const N: usize> {
    data: [u8; N],
    len: usize,
}

type EthernetFrame = FixedBuf<1514>;
type JumboFrame   = FixedBuf<9000>;
```

**`const` vs `static` — the critical difference**:

```
CONST                                   STATIC
┌───────────────────────────────┐       ┌────────────────────────────────────┐
│ - Inlined at use site         │       │ - Has a fixed memory address       │
│ - No memory address           │       │ - Initialized once                 │
│ - Can be any const-eval type  │       │ - Lives for program lifetime       │
│ - Cannot be mut               │       │ - Can be mut (unsafe to access)    │
│ - Cannot be referenced (&)    │       │ - Can be referenced (&'static T)   │
│   (no stable address)         │       │ - Thread-local variant available   │
└───────────────────────────────┘       └────────────────────────────────────┘
```

---

### `static`

**What it actually means**: `static` declares a **global variable** with `'static` lifetime. It
has a stable memory address. Unlike `const`, it is NOT inlined — all uses refer to the same
memory location.

**`static mut`** is `unsafe` to access because the compiler cannot prevent data races on it. It
should almost never be used; prefer `static` with interior mutability (`Mutex`, `AtomicXxx`) or
`OnceLock`/`LazyLock`.

```rust
// GOOD: static with atomic for lock-free global state
use std::sync::atomic::{AtomicU64, Ordering};
static PACKETS_RECEIVED: AtomicU64 = AtomicU64::new(0);
static PACKETS_DROPPED: AtomicU64 = AtomicU64::new(0);

fn record_packet_received() {
    PACKETS_RECEIVED.fetch_add(1, Ordering::Relaxed);
}

// GOOD: OnceLock for lazy initialization of complex statics
use std::sync::OnceLock;
static CONFIG: OnceLock<Config> = OnceLock::new();

fn get_config() -> &'static Config {
    CONFIG.get_or_init(|| Config::load_from_env())
}

// BAD: static mut — data race if accessed from multiple threads
static mut COUNTER: u64 = 0; // WRONG
// fn inc() { unsafe { COUNTER += 1; } } // DATA RACE

// Thread-local storage
thread_local! {
    static THREAD_BUFFER: std::cell::RefCell<Vec<u8>> =
        std::cell::RefCell::new(Vec::with_capacity(4096));
}
```

**`'static` lifetime** (the lifetime, not the keyword): A value with `'static` lifetime lives for
the entire program. This does NOT mean it was declared with `static`. String literals (`"hello"`)
have `'static` lifetime. `Box::leak()` produces `'static` references. `Arc<T>` where T: `'static`
satisfies `T: 'static`.

```rust
// 'static as a trait bound: "this type contains no non-static references"
fn spawn_task<F: FnOnce() + Send + 'static>(f: F) {
    std::thread::spawn(f);
}
// F: 'static means the closure cannot borrow from the current stack frame
// (which would be dangling after spawn returns)
```

---

### `async` and `await`

**What they actually mean**: `async` transforms a function or block into one that returns a `Future`.
The function body does NOT execute immediately; it executes when the future is polled by an executor.
`await` suspends the current `async` context until the awaited future completes.

**Critical mental model**: An `async fn` is a **state machine**. At each `.await` point, the
state machine can be suspended (its state saved) and resumed later. All local variables that live
across an `.await` point are stored in the state machine (the Future struct).

```
async fn example() COMPILES TO approximately:

enum ExampleFuture {
    State0 { /* variables before first await */ },
    State1 { /* variables between awaits */ },
    State2 { /* variables after last await */ },
    Completed,
}

impl Future for ExampleFuture {
    type Output = ReturnType;
    fn poll(&mut self, cx: &mut Context) -> Poll<ReturnType> {
        loop {
            match self {
                State0 { .. } => { /* run until first await */ }
                State1 { .. } => { /* run until second await */ }
                ...
            }
        }
    }
}
```

```rust
use tokio::io::{AsyncReadExt, AsyncWriteExt};

// async fn — returns impl Future<Output = Result<(), Error>>
async fn handle_tls_connection(
    mut stream: tokio_rustls::TlsStream<tokio::net::TcpStream>,
) -> Result<(), std::io::Error> {
    let mut header_buf = [0u8; 5]; // TLS record header
    stream.read_exact(&mut header_buf).await?; // suspend here if not ready

    let record_len = u16::from_be_bytes([header_buf[3], header_buf[4]]) as usize;
    let mut body_buf = vec![0u8; record_len];
    stream.read_exact(&mut body_buf).await?; // suspend again

    process_tls_record(&header_buf, &body_buf)?;
    stream.write_all(b"HTTP/1.1 200 OK\r\n\r\n").await?;
    Ok(())
}
```

**`Send` and `Sync` with async** — the most common async bugs:

A `Future` is `Send` (can be moved across threads) only if all values held across `.await` points
are `Send`. A `MutexGuard` is NOT `Send`. Holding a `MutexGuard` across an `.await` is a common
mistake that causes a compile error (or deadlock with `std::sync::Mutex`).

```rust
use std::sync::Mutex;
use tokio::sync::Mutex as TokioMutex; // async-aware mutex

// WRONG: std::sync::MutexGuard is !Send
async fn bad_async_mutex(lock: Arc<Mutex<State>>) {
    let guard = lock.lock().unwrap(); // std MutexGuard
    do_async_work().await; // ERROR or deadlock: guard held across await
    drop(guard);
}

// RIGHT: use tokio::sync::Mutex for guards across await
async fn good_async_mutex(lock: Arc<TokioMutex<State>>) {
    let mut guard = lock.lock().await; // tokio MutexGuard is Send
    do_async_work().await; // OK: tokio guard can be held across await
    guard.field = new_value;
}

// ALSO RIGHT: drop guard before await
async fn also_good(lock: Arc<Mutex<State>>) {
    {
        let mut guard = lock.lock().unwrap();
        guard.field = new_value;
    } // guard dropped here — before await
    do_async_work().await; // no guard held
}
```

---

## 6. Module & Visibility Keywords

### `mod`

**What it actually means**: `mod` declares a **module** — a namespace that groups items. A module
can be:
1. Inline: `mod foo { ... }` — defined in the current file
2. File-based: `mod foo;` — loaded from `foo.rs` or `foo/mod.rs`

The **module tree** determines visibility rules. Items are private by default (visible only within
their module and descendants).

```
Module tree for a daemon project:
crate root (main.rs or lib.rs)
├── mod config;         ← config.rs or config/mod.rs
│   ├── mod tls;        ← config/tls.rs
│   └── mod network;    ← config/network.rs
├── mod netlink;        ← netlink.rs
│   ├── mod socket;
│   └── mod message;
└── mod server;
    ├── mod handler;
    └── mod metrics;
```

```rust
// main.rs
mod config;
mod netlink;
mod server;

fn main() {
    let cfg = config::Config::load().unwrap();
    let nl_socket = netlink::Socket::new().unwrap();
    server::run(cfg, nl_socket).await;
}

// config/mod.rs
pub mod tls;
pub mod network;

pub struct Config {
    pub tls: tls::TlsConfig,
    pub net: network::NetworkConfig,
}
```

### `use`

**What it actually means**: `use` brings names into scope. It does NOT affect the module tree
structure — only what names are visible in the current scope without qualification.

```rust
use std::collections::HashMap;
use std::sync::{Arc, Mutex, RwLock};
use std::net::{IpAddr, Ipv4Addr, SocketAddr};

// Glob import (use sparingly — hurts readability)
use std::io::prelude::*;

// Renaming with 'as'
use std::io::Error as IoError;
use std::fmt::Error as FmtError;

// Re-exporting with pub use
pub use netlink::message::NlMsg; // consumers of this crate can use it without full path

// Nested paths
use std::{
    fs::File,
    io::{self, BufReader, Write}, // 'self' refers to std::io itself
};
```

### `pub`

**What it actually means**: `pub` makes an item **public** to the outside of its defining module.
There are fine-grained variants:

```rust
pub struct PublicToAll;                  // visible to anyone
pub(crate) struct CratePrivate;          // visible within this crate only
pub(super) struct SuperPrivate;          // visible to parent module
pub(in crate::config) struct ConfigOnly; // visible within config module tree

// pub on struct fields is independent of pub on the struct
pub struct Config {
    pub listen_addr: SocketAddr,     // pub: constructable from outside
    pub(crate) secret: Vec<u8>,      // crate-visible only
    tls_private_key: Vec<u8>,        // private: not accessible outside module
}
```

**Fatal mistake**: Making a struct `pub` but leaving fields private makes the struct impossible to
construct outside the module (no struct literal syntax for structs with private fields). You must
provide a constructor. This is actually a useful pattern for invariant enforcement.

### `crate`, `super`, `self` in paths

```rust
// crate — refers to the root of the current crate
use crate::config::Config;  // absolute path from crate root
use crate::netlink::Socket;

// super — refers to the parent module
// In src/server/handler.rs:
use super::metrics::Counter; // goes up to server, then into metrics
use super::super::config::Config; // goes up twice

// self — refers to current module (useful in use statements)
use self::inner::HelperType; // relative to current module
```

---

## 7. FFI & Low-Level Keywords

### `unsafe`

**What it actually means**: `unsafe` is a **scope that allows five specific operations** that the
compiler cannot verify for safety:
1. Dereferencing raw pointers (`*const T`, `*mut T`)
2. Calling `unsafe fn`
3. Accessing `static mut`
4. Implementing `unsafe trait`
5. Accessing fields of `union`

**The biggest misinformation**: "unsafe disables the borrow checker." FALSE. The borrow checker
is fully active inside `unsafe` blocks. Lifetimes, ownership, and borrows still apply. `unsafe`
only unlocks the five operations above.

**The second misinformation**: "unsafe means undefined behavior." FALSE. `unsafe` code can be
correct. The programmer is asserting: "I have verified the invariants that the compiler cannot
check." If those invariants are actually true, the code is safe.

```
What unsafe DOES and DOES NOT do:
┌────────────────────────────────────────────────────────────────────┐
│ DOES allow:                    DOES NOT disable:                   │
│  - raw pointer deref            - borrow checker                   │
│  - unsafe fn calls              - lifetime tracking                │
│  - static mut access            - type system                      │
│  - unsafe trait impl            - overflow checks (debug mode)     │
│  - union field access           - bounds checks (use unsafe fn)    │
└────────────────────────────────────────────────────────────────────┘
```

```rust
// Correct unsafe: wrapping a Linux system call
pub unsafe fn mmap(
    addr: *mut libc::c_void,
    length: usize,
    prot: i32,
    flags: i32,
    fd: i32,
    offset: i64,
) -> *mut libc::c_void {
    // Safety: caller guarantees addr is valid or null, fd is open, etc.
    libc::mmap(addr, length, prot, flags, fd, offset)
}

// Safe wrapper — encapsulates the unsafe in a narrow interface
pub struct MmapRegion {
    ptr: *mut u8,
    len: usize,
}

impl MmapRegion {
    pub fn new(len: usize) -> std::io::Result<Self> {
        let ptr = unsafe {
            libc::mmap(
                std::ptr::null_mut(),
                len,
                libc::PROT_READ | libc::PROT_WRITE,
                libc::MAP_PRIVATE | libc::MAP_ANONYMOUS,
                -1,
                0,
            ) as *mut u8
        };
        if ptr == libc::MAP_FAILED as *mut u8 {
            return Err(std::io::Error::last_os_error());
        }
        Ok(MmapRegion { ptr, len })
    }

    // Safe: we know ptr is valid for self.len bytes (invariant established in new())
    pub fn as_slice(&self) -> &[u8] {
        unsafe { std::slice::from_raw_parts(self.ptr, self.len) }
    }

    pub fn as_mut_slice(&mut self) -> &mut [u8] {
        unsafe { std::slice::from_raw_parts_mut(self.ptr, self.len) }
    }
}

impl Drop for MmapRegion {
    fn drop(&mut self) {
        unsafe { libc::munmap(self.ptr as *mut libc::c_void, self.len); }
    }
}
```

**`unsafe fn`**: Declares a function whose callers must uphold invariants that the compiler cannot
check. The function itself may or may not contain unsafe blocks.

```rust
// unsafe fn — callers must ensure preconditions
/// # Safety
/// - `ptr` must be non-null and aligned to T's alignment
/// - `ptr` must point to valid initialized T
/// - `ptr` must be valid for 'a lifetime
/// - No other mutable reference to *ptr may exist during 'a
pub unsafe fn ptr_to_ref<'a, T>(ptr: *const T) -> &'a T {
    &*ptr
}
```

**`unsafe trait`**: A trait where implementations must uphold invariants the compiler cannot check.

```rust
// Send and Sync are unsafe traits
// unsafe impl means: "I assert these invariants hold"
unsafe impl Send for MmapRegion {} // safe: ptr is not aliased
unsafe impl Sync for MmapRegion {} // safe: immutable access is fine concurrently
```

---

### `extern`

**What it actually means**: `extern` has two uses:
1. **`extern "ABI" { ... }`**: Declares functions/statics defined in another language (linking)
2. **`extern "ABI" fn`**: Specifies the calling convention for a function

The ABI string specifies the calling convention: `"C"` (C calling convention, most common),
`"system"` (platform-specific: `stdcall` on Windows, `C` elsewhere), `"Rust"` (default),
`"C-unwind"` (C ABI but allows unwinding across FFI boundary, Rust 1.71+).

```rust
// Declare C functions from Linux kernel syscall wrappers (via libc)
extern "C" {
    fn epoll_create1(flags: i32) -> i32;
    fn epoll_ctl(epfd: i32, op: i32, fd: i32, event: *mut EpollEvent) -> i32;
    fn epoll_wait(epfd: i32, events: *mut EpollEvent, maxevents: i32, timeout: i32) -> i32;
}

// All calls to extern "C" functions are unsafe
fn create_epoll() -> i32 {
    unsafe { epoll_create1(libc::EPOLL_CLOEXEC) }
}

// Calling convention on function definition (for callbacks exported to C)
// This function can be called from C code
#[no_mangle]
pub extern "C" fn rust_packet_callback(buf: *const u8, len: usize) -> i32 {
    if buf.is_null() || len == 0 { return -1; }
    let slice = unsafe { std::slice::from_raw_parts(buf, len) };
    match process_packet(slice) {
        Ok(_) => 0,
        Err(_) => -1,
    }
}

// Linking to a specific library
#[link(name = "xdp")]
extern "C" {
    fn xdp_socket_create(ifname: *const libc::c_char, queue_id: u32) -> i32;
    fn xdp_socket_recv(fd: i32, buf: *mut u8, len: usize) -> isize;
}
```

**`extern crate`** (pre-2018): In Rust 2015, external crates required `extern crate name;`. In
Rust 2018+, this is no longer needed — crates listed in `Cargo.toml` are automatically available.
The only remaining use of `extern crate` is for the `std`/`core`/`alloc` split in `#![no_std]`
crates.

```rust
// Rust 2018+: not needed (but still valid)
// extern crate serde; // unnecessary

// Still needed in no_std contexts
#![no_std]
extern crate alloc; // explicitly link alloc crate
use alloc::vec::Vec;
```

---

## 8. Generics & Lifetime Keywords

### `where`

**What it actually means**: `where` provides an alternate syntax for generic bounds, allowing
complex bounds to be written after the function/impl signature instead of inline with the generics.
It enables:
1. Bounds that reference associated types
2. Bounds that are too complex to fit inline
3. Better readability for complex signatures

```rust
// Inline bounds (limited readability for complex cases)
fn process<P: PacketProcessor + Send + 'static, E: std::error::Error>(p: P) { }

// where clause — same semantics, better readability
fn process<P, E>(processor: P) -> Result<(), E>
where
    P: PacketProcessor<Error = E> + Send + 'static,
    E: std::error::Error + Send + 'static,
{ /* ... */ }

// Where clause for associated type bounds (cannot be done inline)
impl<I> NetworkStats
where
    I: Iterator<Item = Packet>,
    I::Item: Clone + Send,
{ /* ... */ }
```

### Higher-Rank Trait Bounds (HRTB): `for<'a>`

**What it actually means**: `for<'a>` (read: "for any lifetime `'a`") specifies that a bound must
hold for ALL possible lifetimes. This is required when a function/closure must work with references
of ANY lifetime, not a specific one.

```rust
// HRTB: the closure must work for any lifetime 'a
fn apply_to_any<F>(f: F, data: &[u8]) -> usize
where
    F: for<'a> Fn(&'a [u8]) -> usize,
{
    f(data)
}

// Without HRTB, a specific lifetime would be tied to the function's parameters:
fn apply_specific<'b, F>(f: F, data: &'b [u8]) -> usize
where
    F: Fn(&'b [u8]) -> usize, // F only works for lifetime 'b
{
    f(data)
}

// Real example: trait objects with HRTB (common in async)
trait Handler {
    fn handle<'req>(&self, req: &'req Request) -> Response;
}
// dyn Handler requires HRTB: for<'req> fn(&'req Request)
```

### `dyn` Trait

**What it actually means**: `dyn Trait` is an **explicitly dynamic dispatch** type. A `dyn Trait`
value is a fat pointer: (data pointer, vtable pointer). The vtable contains function pointers for
all the trait's methods.

**`dyn` vs `impl` — the choice matters for performance**:

```rust
// impl Trait in argument position — static dispatch (monomorphized)
fn handle_impl(processor: &impl PacketProcessor) { /* zero runtime cost */ }

// &dyn Trait — dynamic dispatch (vtable lookup per method call)
fn handle_dyn(processor: &dyn PacketProcessor) { /* vtable dispatch */ }

// When to use dyn:
// 1. Heterogeneous collections
let processors: Vec<Box<dyn PacketProcessor<Error=std::io::Error>>> = vec![
    Box::new(XdpProcessor::new()),
    Box::new(EbpfProcessor::new()),
];

// 2. Return type polymorphism where impl Trait can't work
fn make_handler(kind: &str) -> Box<dyn Handler> {
    match kind {
        "http" => Box::new(HttpHandler),
        "grpc" => Box::new(GrpcHandler),
        _ => panic!(),
    }
}
// Note: 'impl Handler' in return position CAN work for single concrete type
// but when branches return DIFFERENT types, Box<dyn> is required
```

**`dyn` Trait object safety**: Not all traits can be used as `dyn Trait`. A trait is object-safe if:
- All methods are dispatchable (no generic methods)
- No `Self: Sized` constraint on the trait
- `where Self: Sized` methods are excluded from the vtable

```rust
// Object-safe: all methods dispatchable via vtable
trait Serialize {
    fn serialize(&self, buf: &mut Vec<u8>);
}

// NOT object-safe: generic method
trait BadTrait {
    fn convert<T: Clone>(&self) -> T; // generic — cannot put in vtable
}
// Box<dyn BadTrait> // ERROR: BadTrait is not object-safe
```

---

## 9. Special Value Keywords

### `self` and `Self`

`self` (lowercase) is the instance of the type in a method. `Self` (capitalized) is the type itself.

```rust
impl Config {
    // self: the Config instance
    pub fn listen_addr(&self) -> &SocketAddr { &self.listen_addr }

    // &mut self: mutable borrow of instance
    pub fn set_addr(&mut self, addr: SocketAddr) { self.listen_addr = addr; }

    // self (by value): consumes the instance
    pub fn into_tls_config(self) -> TlsConfig { self.tls }

    // Self: the type Config itself
    pub fn default() -> Self {
        Self {
            listen_addr: "0.0.0.0:8080".parse().unwrap(),
            tls: TlsConfig::default(),
            max_connections: 1000,
        }
    }

    // Builder pattern using Self
    pub fn with_addr(mut self, addr: SocketAddr) -> Self {
        self.listen_addr = addr;
        self
    }
}
```

**`self` in `use` statements** refers to the module itself:
```rust
use std::io::{self, Read}; // imports std::io AND std::io::Read
```

### `true` and `false`

These are literal values of type `bool`. They are keywords (not identifiers), so you cannot
shadow them. They participate in exhaustive match patterns.

### `as`

**What it actually means**: `as` performs a primitive type cast. It is NOT Rust's general
coercion — it is an explicit, potentially lossy, cast between primitive types.

```rust
let x: u32 = 300u32;
let y: u8 = x as u8;   // truncation: y = 44 (300 % 256)
let z: i8 = x as i8;   // truncation + reinterpret: z = 44 (300 fits, same bits)

let f: f64 = 3.99;
let i: i32 = f as i32; // truncation toward zero: i = 3

// as for pointer casting (common in unsafe/FFI)
let ptr: *mut u8 = buf.as_mut_ptr();
let c_ptr: *mut libc::c_void = ptr as *mut libc::c_void;
```

**`as` in patterns**: The `as` keyword in a pattern bind (`pat as name`) is NOT the same as
type-cast `as`. In patterns it's written as `pat @ binding` (the `@` binds the whole match).

**Fatal mistake**: Using `as` for integer truncation without validation:
```rust
let payload_len: u64 = parse_header_length(buf); // attacker-controlled
let slice = &buf[..payload_len as usize]; // DANGEROUS: if payload_len > buf.len(), panic
// Use safe conversion:
let len = usize::try_from(payload_len).map_err(|_| ParseError::LengthOverflow)?;
let slice = buf.get(..len).ok_or(ParseError::TooShort)?;
```

---

## 10. Reserved but Unstable Keywords

These are reserved for future use. You cannot use them as identifiers today.

| Keyword | Likely Future Use |
|---------|-------------------|
| `abstract` | Abstract types/methods (may never be added) |
| `become` | Tail-call optimization (RFC pending) |
| `box` | Placement new / Box construction (nightly only) |
| `do` | `do`-while loops or do-blocks |
| `final` | Prevent inheritance/override |
| `macro` | Declarative macro 2.0 syntax |
| `override` | Method overriding |
| `priv` | Private visibility (more granular than current) |
| `try` | `try { }` blocks (nightly: desugars `?` to Ok-wrap) |
| `typeof` | Type introspection |
| `unsized` | Explicitly unsized types |
| `virtual` | Virtual dispatch |
| `yield` | Generator/coroutine syntax |

**`try` blocks** are available on nightly and partially stabilized behavior through `?`:

```rust
// Nightly: try blocks (experimental)
// #![feature(try_blocks)]
// let result: Result<_, Error> = try {
//     let a = foo()?;
//     let b = bar(a)?;
//     b
// };
```

---

## 11. Keyword Interaction: The Dangerous Combinations

### `unsafe` + `static mut` — Race Condition Vector

```rust
// DANGEROUS: static mut + unsafe without synchronization
static mut GLOBAL_FD: i32 = -1;

fn init_socket() {
    unsafe { GLOBAL_FD = socket(AF_INET, SOCK_STREAM, 0); }
}

fn use_socket() {
    // Multiple threads reading GLOBAL_FD while another writes — DATA RACE
    let fd = unsafe { GLOBAL_FD };
    read(fd, buf);
}

// SAFE: use AtomicI32 instead
static GLOBAL_FD: AtomicI32 = AtomicI32::new(-1);
fn init_socket_safe() {
    GLOBAL_FD.store(socket(AF_INET, SOCK_STREAM, 0), Ordering::Release);
}
fn use_socket_safe() {
    let fd = GLOBAL_FD.load(Ordering::Acquire);
    // ...
}
```

### `async` + `move` + Captured Mutex Guards

```rust
// DEADLOCK PATTERN in async code
async fn deadlock_pattern(lock: Arc<std::sync::Mutex<State>>) {
    let guard = lock.lock().unwrap();
    // EITHER: compile error because MutexGuard: !Send
    // OR (with spawn_local / single-threaded runtime): deadlock if inner_fn
    // tries to lock the same mutex
    inner_async_fn_that_locks_same_mutex().await;
    drop(guard);
}
```

### `impl Trait` in Return Position — Lifetime Traps

```rust
// impl Trait in return position captures lifetimes from inputs
fn get_first(data: &[u8]) -> impl Iterator<Item = u8> + '_ {
    data.iter().copied()
}
// The '_ lifetime ties the returned iterator to 'data's lifetime
// Callers must ensure data outlives the iterator

// TRAP: two lifetime parameters, ambiguous capture
fn filter_data<'a, 'b>(
    data: &'a [u8],
    mask: &'b [u8],
) -> impl Iterator<Item = u8> + 'a {
    // Which lifetime is captured? 'a is explicit here
    // Without the '+ 'a, compiler may pick wrong lifetime
    data.iter().copied().filter(move |&b| mask.contains(&b))
}
```

### `const` Generics + `where` Bounds — Complexity Cliff

```rust
// Complex const generic bounds — compile times can explode
struct Matrix<T, const ROWS: usize, const COLS: usize> {
    data: [[T; COLS]; ROWS],
}

impl<T: Default + Copy, const R: usize, const C: usize> Matrix<T, R, C> {
    fn transpose(self) -> Matrix<T, C, R>
    where
        [(); R * C]: Sized, // const expression in where — nightly feature
    {
        todo!()
    }
}
```

---

## 12. Complete Fatal Mistakes Reference

### Mistake 1: Holding `MutexGuard` Across `.await`

```rust
// WRONG — causes compile error OR deadlock with tokio's cooperative scheduling
async fn wrong(state: Arc<Mutex<HashMap<String, Vec<u8>>>>) {
    let guard = state.lock().unwrap();
    let val = guard.get("key").cloned();
    do_io().await;          // guard still held! If do_io tries to acquire same lock → deadlock
    process(val);
}

// RIGHT
async fn right(state: Arc<Mutex<HashMap<String, Vec<u8>>>>) {
    let val = {
        let guard = state.lock().unwrap();
        guard.get("key").cloned()
    }; // guard dropped here
    do_io().await;
    process(val);
}
```

### Mistake 2: `as` Cast for Security-Critical Conversions

```rust
// WRONG: attacker controls length field in network packet
fn parse_bad(buf: &[u8]) -> &[u8] {
    let len = u16::from_be_bytes([buf[0], buf[1]]) as usize;
    &buf[2..2 + len] // PANIC if len > buf.len() - 2 (length mismatch attack)
}

// RIGHT: validate before casting
fn parse_good(buf: &[u8]) -> Result<&[u8], ParseError> {
    let len = u16::from_be_bytes(buf.get(0..2).ok_or(ParseError::TooShort)?.try_into().unwrap()) as usize;
    buf.get(2..2 + len).ok_or(ParseError::TooShort)
}
```

### Mistake 3: Using `type` Alias for Safety Guarantees

```rust
type Pid = i32;
type Uid = i32;
// fn kill(pid: Pid, uid: Uid) — WRONG: uid can be passed as pid, type alias provides zero safety

// RIGHT: newtype wrappers
struct Pid(i32);
struct Uid(i32);
// fn kill(pid: Pid, uid: Uid) — swapped args = compile error
```

### Mistake 4: `unsafe impl Send`/`Sync` Without Justification

```rust
struct MyHandle { raw_ptr: *mut InternalState } // *mut T is !Send + !Sync

// WRONG: asserting safety without analysis
unsafe impl Send for MyHandle {} // Is InternalState really shareable?
unsafe impl Sync for MyHandle {} // Are all operations thread-safe?

// RIGHT: analyze carefully
// Send: safe if MyHandle has exclusive ownership of raw_ptr (no aliasing)
// Sync: safe if all methods take &self and InternalState has internal sync
```

### Mistake 5: Forgetting `move` in Spawned Closures

```rust
fn bad_spawn(config: &Config) {
    std::thread::spawn(|| {
        // ERROR: closure borrows 'config' which lives on current stack frame
        // Thread may outlive this function → dangling reference
        println!("{}", config.listen_addr);
    });
}

fn good_spawn(config: Config) {
    std::thread::spawn(move || {
        println!("{}", config.listen_addr); // config moved into thread
    });
}

fn good_spawn_arc(config: Arc<Config>) {
    let c = Arc::clone(&config);
    std::thread::spawn(move || {
        println!("{}", c.listen_addr); // Arc clone moved in
    });
}
```

### Mistake 6: Shadowing Drops vs Actual Drops

```rust
fn zeroize_secret() {
    let key = load_key_material(); // Vec<u8> with secret bytes
    // WRONG: shadows 'key', but OLD key is NOT dropped until end of scope
    let key = encrypt_key(&key); // original key STILL ALIVE here
    // The secret bytes remain in memory until scope end

    // RIGHT: explicit zeroize before shadow or use block scope
    let session_key = {
        let original_key = load_key_material();
        let result = encrypt_key(&original_key);
        zeroize(original_key); // explicit wipe
        result
    }; // original_key dropped/wiped here
}
```

### Mistake 7: `pub` Struct with Private Fields — Unconstructable

```rust
pub mod config {
    pub struct Config {
        pub addr: SocketAddr,
        tls_key: Vec<u8>, // private field
    }
}

// Outside the module:
// config::Config { addr: "0.0.0.0:8080".parse().unwrap(), tls_key: vec![] }
// ERROR: cannot construct Config with private field tls_key

// Either: provide a constructor, OR make all fields pub
// This pattern is intentional for invariant enforcement:
impl Config {
    pub fn new(addr: SocketAddr, key_path: &Path) -> Result<Self, Error> {
        Ok(Config {
            addr,
            tls_key: std::fs::read(key_path)?, // validated on construction
        })
    }
}
```

### Mistake 8: `static` Lifetime Misunderstanding

```rust
// WRONG: thinking 'static means the data lives forever
fn wrong_static() {
    let s = String::from("hello");
    let r: &'static str = s.as_str(); // ERROR: s doesn't live long enough
    // 'static requires the data to actually live for the whole program
}

// RIGHT: string literals are 'static because they're in the binary
let r: &'static str = "hello"; // OK: in rodata section of binary

// RIGHT: Box::leak creates 'static (but leaks memory — use sparingly)
let boxed = Box::new(vec![1u8, 2, 3]);
let leaked: &'static [u8] = Box::leak(boxed);
```

### Mistake 9: Incorrect `extern "C"` for Callbacks

```rust
// WRONG: using Rust ABI for a callback registered with C code
fn my_callback(data: *const u8, len: usize) -> i32 { /* ... */ }
// If registered as C callback, calling convention mismatch → undefined behavior

// RIGHT: always use extern "C" for C callbacks
extern "C" fn my_callback(data: *const u8, len: usize) -> i32 { /* ... */ }
```

### Mistake 10: `dyn Trait` Without `Send`/`Sync` Bounds in Async

```rust
// WRONG in async context — Box<dyn Error> is !Send
async fn wrong() -> Result<(), Box<dyn std::error::Error>> {
    tokio::spawn(async {
        let x: Box<dyn std::error::Error> = get_error();
        Err(x) // Error: Box<dyn Error> is !Send
    }).await??;
    Ok(())
}

// RIGHT: add Send + 'static bounds
async fn right() -> Result<(), Box<dyn std::error::Error + Send + 'static>> {
    tokio::spawn(async {
        let x: Box<dyn std::error::Error + Send + 'static> = get_send_error();
        Err(x)
    }).await??;
    Ok(())
}
```

---

## 13. Misinformation Debunked

### Myth 1: "`unsafe` disables the borrow checker"

**False.** The borrow checker is fully active inside `unsafe` blocks. What `unsafe` enables is
dereferencing raw pointers — and even then, the compiler tracks lifetimes of references derived
from those pointers. You cannot create a dangling `&T` inside `unsafe` and have the compiler
ignore it.

### Myth 2: "`mut` means the value is on the heap"

**False.** `mut` has nothing to do with allocation location. A `let mut x = 5i32` is on the stack.
A `let x = Box::new(5i32)` is on the heap but the binding is immutable (can't move `x` to another
Box). Heap allocation is done by `Box`, `Vec`, `Arc`, etc. — not by `mut`.

### Myth 3: "`move` clones the captured values"

**False.** `move` MOVES (transfers ownership of) the captured values into the closure. No cloning
occurs. If you need both the original and the closure to have the value, you must explicitly clone
before the move.

### Myth 4: "`static` lifetime means the data is in static memory"

**Partially false.** `'static` lifetime means the data COULD live for the entire program. It does
NOT mean the data is in the BSS/rodata/data segment. A `Box::leak()`ed heap allocation has
`'static` lifetime but is on the heap. The bound `T: 'static` means "T contains no non-static
references" — it does NOT constrain where T's data lives.

### Myth 5: "`dyn Trait` is always slower than generics"

**Oversimplification.** For large monomorphized types (generic code compiled for 10+ concrete
types), `dyn Trait` can be faster due to less instruction cache pressure (one copy of code vs many).
For hot paths with a single concrete type, generics are faster (no vtable indirection, inlining
possible). Profile before deciding.

### Myth 6: "`async fn` blocks the current thread"

**False.** An `async fn` body does NOT run until polled. Even `tokio::spawn(async { ... })` is
non-blocking — it registers the future with the executor's work queue. The current thread is NOT
blocked. Blocking occurs only if you call blocking I/O inside an async context without
`tokio::task::spawn_blocking`.

### Myth 7: "You can return `impl Trait` from a function and have multiple different concrete types"

**False.** `impl Trait` in return position means ONE concrete type, determined at compile time.
If different branches return different types, you get a compile error. Use `Box<dyn Trait>` for
runtime polymorphism with multiple types.

```rust
fn wrong() -> impl Iterator<Item = i32> {
    if condition {
        vec![1, 2, 3].into_iter()
    } else {
        std::iter::empty() // ERROR: different concrete types
    }
}

fn right() -> Box<dyn Iterator<Item = i32>> {
    if condition {
        Box::new(vec![1, 2, 3].into_iter())
    } else {
        Box::new(std::iter::empty())
    }
}
```

### Myth 8: "`const fn` can do anything at compile time"

**False.** `const fn` has restrictions: no heap allocation, no raw pointer dereferencing (except
in specific cases), no I/O, no floating-point operations in some contexts, no calling non-const
functions. The set of allowed operations expands with each Rust release (const trait impls, etc.),
but it is NOT general-purpose computation.

### Myth 9: "`pub(crate)` is equivalent to C's `static` (file scope)"

**False.** C's `static` (on a non-local variable) gives file scope. `pub(crate)` gives the entire
crate access. Rust has no file-scope visibility. The closest is `pub(super)` (parent module only)
or just leaving items private (current module only).

### Myth 10: "`loop {}` will spin-wait and burn CPU"

**Context-dependent.** A bare `loop {}` with no `break` or yield points WILL burn CPU. But
`loop { some_future.await; }` does NOT spin — the executor suspends the task when the future is
not ready and wakes it only on the relevant I/O event.

---

## 14. Cloud-Native & Linux Kernel Patterns

### Pattern 1: Netlink Socket via `extern` + `unsafe` (Real eBPF/XDP Workflow)

```
Userspace (Rust daemon)       Kernel Space
┌─────────────────────┐       ┌──────────────────────────────────┐
│ extern "C" {        │       │                                  │
│   socket()          │◄─────►│  AF_NETLINK socket               │
│   bind()            │       │                                  │
│   sendmsg()         │       │  Netlink subsystem               │
│   recvmsg()         │       │  ├── NETLINK_ROUTE               │
│ }                   │       │  ├── NETLINK_GENERIC             │
│                     │       │  └── NETLINK_XFRM (IPsec)        │
│ unsafe {            │       │                                  │
│   struct NlMsgHdr   │       │  Kernel responds with:           │
│   (repr(C))         │◄─────►│  RTM_NEWLINK / RTM_NEWADDR       │
│ }                   │       │  RTM_NEWROUTE events             │
└─────────────────────┘       └──────────────────────────────────┘
```

```rust
use std::os::unix::io::RawFd;

// repr(C) is mandatory — kernel defines this layout
#[repr(C)]
struct NlMsgHdr {
    nlmsg_len:   u32,
    nlmsg_type:  u16,
    nlmsg_flags: u16,
    nlmsg_seq:   u32,
    nlmsg_pid:   u32,
}

#[repr(C)]
struct IfInfoMsg {
    ifi_family: u8,
    _pad:       u8,
    ifi_type:   u16,
    ifi_index:  i32,
    ifi_flags:  u32,
    ifi_change: u32,
}

const AF_NETLINK: i32 = 16;
const NETLINK_ROUTE: i32 = 0;
const RTM_GETLINK: u16 = 18;
const NLM_F_REQUEST: u16 = 0x0001;
const NLM_F_DUMP: u16    = 0x0300;

pub struct NetlinkSocket {
    fd: RawFd,
}

impl NetlinkSocket {
    pub fn new() -> std::io::Result<Self> {
        let fd = unsafe {
            libc::socket(AF_NETLINK, libc::SOCK_RAW | libc::SOCK_CLOEXEC, NETLINK_ROUTE)
        };
        if fd < 0 {
            return Err(std::io::Error::last_os_error());
        }
        let addr = libc::sockaddr_nl {
            nl_family: AF_NETLINK as u16,
            nl_pad: 0,
            nl_pid: 0,   // kernel auto-assigns
            nl_groups: 0,
        };
        let bind_result = unsafe {
            libc::bind(
                fd,
                &addr as *const _ as *const libc::sockaddr,
                std::mem::size_of::<libc::sockaddr_nl>() as u32,
            )
        };
        if bind_result < 0 {
            unsafe { libc::close(fd); }
            return Err(std::io::Error::last_os_error());
        }
        Ok(NetlinkSocket { fd })
    }

    pub fn dump_links(&self) -> std::io::Result<Vec<LinkInfo>> {
        // Build RTM_GETLINK + NLM_F_DUMP request
        let msg_len = (std::mem::size_of::<NlMsgHdr>() + std::mem::size_of::<IfInfoMsg>()) as u32;
        let hdr = NlMsgHdr {
            nlmsg_len:   msg_len,
            nlmsg_type:  RTM_GETLINK,
            nlmsg_flags: NLM_F_REQUEST | NLM_F_DUMP,
            nlmsg_seq:   1,
            nlmsg_pid:   0,
        };
        let ifi = IfInfoMsg {
            ifi_family: libc::AF_UNSPEC as u8,
            _pad: 0, ifi_type: 0, ifi_index: 0, ifi_flags: 0, ifi_change: 0,
        };

        // Build contiguous buffer for kernel
        let mut buf = vec![0u8; msg_len as usize];
        unsafe {
            std::ptr::copy_nonoverlapping(
                &hdr as *const NlMsgHdr as *const u8,
                buf.as_mut_ptr(),
                std::mem::size_of::<NlMsgHdr>(),
            );
            std::ptr::copy_nonoverlapping(
                &ifi as *const IfInfoMsg as *const u8,
                buf.as_mut_ptr().add(std::mem::size_of::<NlMsgHdr>()),
                std::mem::size_of::<IfInfoMsg>(),
            );
        }

        let sent = unsafe {
            libc::send(self.fd, buf.as_ptr() as *const libc::c_void, buf.len(), 0)
        };
        if sent < 0 { return Err(std::io::Error::last_os_error()); }

        self.recv_dump()
    }

    fn recv_dump(&self) -> std::io::Result<Vec<LinkInfo>> {
        let mut result = Vec::new();
        let mut recv_buf = vec![0u8; 65536];
        loop {
            let n = unsafe {
                libc::recv(self.fd, recv_buf.as_mut_ptr() as *mut libc::c_void, recv_buf.len(), 0)
            };
            if n < 0 { return Err(std::io::Error::last_os_error()); }
            let n = n as usize;
            let mut offset = 0;
            while offset + std::mem::size_of::<NlMsgHdr>() <= n {
                let hdr = unsafe {
                    &*(recv_buf.as_ptr().add(offset) as *const NlMsgHdr)
                };
                if hdr.nlmsg_type == libc::NLMSG_DONE as u16 { return Ok(result); }
                if hdr.nlmsg_type == libc::NLMSG_ERROR as u16 {
                    return Err(std::io::Error::new(std::io::ErrorKind::Other, "netlink error"));
                }
                result.push(parse_link_info(&recv_buf[offset..offset + hdr.nlmsg_len as usize])?);
                offset += nlmsg_align(hdr.nlmsg_len as usize);
            }
        }
    }
}

impl Drop for NetlinkSocket {
    fn drop(&mut self) {
        unsafe { libc::close(self.fd); }
    }
}

fn nlmsg_align(len: usize) -> usize { (len + 3) & !3 }
```

### Pattern 2: Async Cloud Agent with `async`/`await`/`move`/`Arc`

```
Cloud Agent Architecture:
┌─────────────────────────────────────────────────────────────┐
│                   Tokio Runtime (multi-thread)               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ gRPC server  │  │ Metrics task │  │ Config watch task│  │
│  │ async fn     │  │ async fn     │  │ async fn         │  │
│  │ Arc<State>   │  │ Arc<State>   │  │ Arc<State>       │  │
│  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
│         │                 │                    │             │
│         └─────────────────┼────────────────────┘             │
│                           │                                  │
│                    Arc<Mutex<State>>                         │
│                           │                                  │
│  ┌────────────────────────▼──────────────────────────────┐  │
│  │          Shared State                                  │  │
│  │   config: Config  metrics: Metrics  connections: Map  │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

```rust
use std::sync::Arc;
use tokio::sync::{Mutex, RwLock};
use std::collections::HashMap;

// Shared state with appropriate sync primitives
struct AgentState {
    config: RwLock<AgentConfig>,           // many readers, rare writes
    connections: Mutex<HashMap<u64, Conn>>, // exclusive access for mutation
    metrics: Arc<prometheus::Registry>,    // Arc because prometheus is thread-safe
}

struct Agent {
    state: Arc<AgentState>,
}

impl Agent {
    pub async fn run(self) -> Result<(), anyhow::Error> {
        let state = Arc::clone(&self.state);

        // Spawn independent tasks — each gets Arc clone
        let grpc_state = Arc::clone(&state);
        let grpc_task = tokio::spawn(async move {
            run_grpc_server(grpc_state).await
        });

        let metrics_state = Arc::clone(&state);
        let metrics_task = tokio::spawn(async move {
            run_metrics_server(metrics_state).await
        });

        let watcher_state = Arc::clone(&state);
        let watcher_task = tokio::spawn(async move {
            run_config_watcher(watcher_state).await
        });

        // Wait for any task to complete (or fail)
        tokio::select! {
            r = grpc_task    => { r??; }
            r = metrics_task => { r??; }
            r = watcher_task => { r??; }
        }

        Ok(())
    }
}

// Config reload: takes &mut via RwLock write guard
async fn run_config_watcher(state: Arc<AgentState>) -> Result<(), anyhow::Error> {
    let mut watcher = FileWatcher::new("/etc/agent/config.yaml")?;
    loop {
        watcher.wait_for_change().await?;
        let new_config = AgentConfig::load_from_file("/etc/agent/config.yaml")?;
        {
            let mut cfg = state.config.write().await; // exclusive write lock
            *cfg = new_config;
        } // write guard dropped immediately — readers unblocked
        tracing::info!("config reloaded");
    }
}

// gRPC handler: reads config frequently, no write needed
async fn handle_request(
    state: Arc<AgentState>,
    req: tonic::Request<AgentRequest>,
) -> Result<tonic::Response<AgentResponse>, tonic::Status> {
    let cfg = state.config.read().await; // shared read — no blocking
    let addr = cfg.upstream_addr.clone();
    drop(cfg); // drop read lock before any awaits that don't need it

    let conn = {
        let conns = state.connections.lock().await;
        conns.get(&req.get_ref().conn_id).cloned()
    }; // lock dropped

    // Now do I/O without holding any locks
    let result = forward_to_upstream(&addr, req.into_inner()).await?;
    Ok(tonic::Response::new(result))
}
```

### Pattern 3: Zero-Copy Packet Processing with `unsafe` + `ref` + `struct`

```
Packet Processing Pipeline:
┌──────────────┐   mmap'd    ┌──────────────────────────────────┐
│  NIC         │   ring buf  │  Userspace (AF_XDP / DPDK style) │
│  ┌─────────┐ │◄───────────►│                                  │
│  │ RX ring │ │             │  struct PacketRef<'a> {           │
│  │ [desc0] │ │             │    data: &'a [u8],  // no copy    │
│  │ [desc1] │ │             │    desc: &'a RxDesc,              │
│  │ [desc2] │ │             │  }                                │
│  └─────────┘ │             │                                  │
└──────────────┘             │  fn process<'a>(pkt: PacketRef<'a>│
                             │  // zero-copy: data stays in      │
                             │  // mmap'd region                 │
                             └──────────────────────────────────┘
```

```rust
// Zero-copy packet reference — lifetime ties to mmap region
pub struct PacketRef<'ring> {
    data: &'ring [u8],
    desc_idx: usize,
}

impl<'ring> PacketRef<'ring> {
    // Returns reference into the mmap region — no allocation
    pub fn ethernet_header(&self) -> Option<&'ring EthernetHeader> {
        if self.data.len() < std::mem::size_of::<EthernetHeader>() {
            return None;
        }
        // Safety: we've verified length, EthernetHeader is repr(C), aligned to 1
        Some(unsafe {
            &*(self.data.as_ptr() as *const EthernetHeader)
        })
    }

    pub fn payload(&self) -> &'ring [u8] {
        let hdr_len = std::mem::size_of::<EthernetHeader>();
        if self.data.len() <= hdr_len { &[] } else { &self.data[hdr_len..] }
    }
}

pub struct RxRing {
    mmap: MmapRegion,
    descs: Vec<RxDesc>,
    head: usize,
}

impl RxRing {
    // Lifetime annotation: returned PacketRef borrows from self
    pub fn recv(&mut self) -> Option<PacketRef<'_>> {
        if !self.descs[self.head].has_packet() { return None; }
        let desc = &self.descs[self.head];
        let offset = desc.offset as usize;
        let len    = desc.len    as usize;
        let data = &self.mmap.as_slice()[offset..offset + len]; // zero-copy borrow
        Some(PacketRef { data, desc_idx: self.head })
    }

    pub fn release(&mut self, pkt: PacketRef) {
        self.descs[pkt.desc_idx].mark_free();
        self.head = (self.head + 1) % self.descs.len();
    }
}
```

### Pattern 4: Typestate with `struct` + `impl` + `PhantomData` for TLS State Machine

```
TLS Connection State Machine (compile-time enforced):

  TCP Connected          TLS Handshake           Established
  ┌────────────┐         ┌────────────┐           ┌───────────────┐
  │Connection  │.tls()   │Connection  │.finish()  │Connection     │
  │<TcpOnly>  │────────►│<Handshake> │──────────►│<TlsReady>    │
  └────────────┘         └────────────┘           └───────┬───────┘
                                                          │
                                                    .send() / .recv()
                                                    (only available here)
```

```rust
use std::marker::PhantomData;

// Typestate markers — zero-size, zero cost
pub struct TcpOnly;
pub struct TlsHandshake;
pub struct TlsReady;

pub struct SecureConn<State> {
    fd: std::os::unix::io::RawFd,
    peer: std::net::SocketAddr,
    _state: PhantomData<State>,
}

// Methods only available in TcpOnly state
impl SecureConn<TcpOnly> {
    pub fn connect(addr: std::net::SocketAddr) -> std::io::Result<Self> {
        let fd = tcp_connect(addr)?;
        Ok(SecureConn { fd, peer: addr, _state: PhantomData })
    }

    // Consuming self — transitions to Handshake state
    pub fn begin_tls(self, config: &TlsConfig) -> std::io::Result<SecureConn<TlsHandshake>> {
        send_client_hello(self.fd, config)?;
        Ok(SecureConn { fd: self.fd, peer: self.peer, _state: PhantomData })
    }
}

// Methods only in TlsHandshake state
impl SecureConn<TlsHandshake> {
    pub fn finish_handshake(self) -> std::io::Result<SecureConn<TlsReady>> {
        complete_tls_handshake(self.fd)?;
        Ok(SecureConn { fd: self.fd, peer: self.peer, _state: PhantomData })
    }
}

// Methods only in TlsReady state
impl SecureConn<TlsReady> {
    pub fn send(&mut self, data: &[u8]) -> std::io::Result<usize> {
        tls_write(self.fd, data)
    }

    pub fn recv(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        tls_read(self.fd, buf)
    }
}

// conn.send() cannot be called before finish_handshake() — compile error, not runtime error
// No need for runtime state checks — the type system enforces the protocol
```

### Pattern 5: `const` + Const Generics for Network Protocol Header Parsing

```rust
// Zero-overhead protocol header parsing using const generics
trait Protocol {
    const HEADER_LEN: usize;
    type Header: Sized;

    fn parse_header(buf: &[u8; Self::HEADER_LEN]) -> Self::Header;
}

#[repr(C, packed)]
struct Ipv4HeaderRaw {
    version_ihl: u8,
    dscp_ecn:    u8,
    total_len:   u16,
    id:          u16,
    flags_frag:  u16,
    ttl:         u8,
    protocol:    u8,
    checksum:    u16,
    src_addr:    [u8; 4],
    dst_addr:    [u8; 4],
}

struct Ipv4Protocol;

impl Protocol for Ipv4Protocol {
    const HEADER_LEN: usize = 20; // IPv4 minimum header

    type Header = Ipv4HeaderRaw;

    fn parse_header(buf: &[u8; 20]) -> Ipv4HeaderRaw {
        // Safety: Ipv4HeaderRaw is repr(C, packed), same size as [u8; 20]
        unsafe { std::mem::transmute(*buf) }
    }
}

// Generic packet parser — header size known at compile time
fn parse_protocol_header<P: Protocol>(buf: &[u8]) -> Option<P::Header>
where
    [u8; P::HEADER_LEN]: Sized,
{
    let header_bytes: &[u8; P::HEADER_LEN] = buf.get(..P::HEADER_LEN)?.try_into().ok()?;
    Some(P::parse_header(header_bytes))
}

// Usage: zero-cost, header size checked at compile time
fn process_ipv4(buf: &[u8]) {
    if let Some(hdr) = parse_protocol_header::<Ipv4Protocol>(buf) {
        let dst = hdr.dst_addr;
        // route packet based on dst
    }
}
```

---

## 15. Threat Model: Keywords as Security Boundaries

```
RUST KEYWORD SECURITY BOUNDARY MAP
════════════════════════════════════════════════════════════════════

  MEMORY SAFETY BOUNDARY
  ┌────────────────────────────────────────────────────────────┐
  │  Safe Rust (no unsafe keyword)                             │
  │  ┌──────────────────────────────────────────────────────┐  │
  │  │ let / mut / ref / move / & / &mut                    │  │
  │  │ → Borrow checker enforces:                           │  │
  │  │   - No use-after-free                                │  │
  │  │   - No double-free                                   │  │
  │  │   - No data races (Send/Sync)                        │  │
  │  │   - No null pointer deref                            │  │
  │  │   - No buffer overflow (via safe indexing)           │  │
  │  └──────────────────────────────────────────────────────┘  │
  └────────────────────────────────────────────────────────────┘
         │ unsafe keyword: controlled breach of this boundary
         ▼
  UNSAFE BOUNDARY
  ┌────────────────────────────────────────────────────────────┐
  │  unsafe blocks/fns — programmer asserts invariants         │
  │  Threat: MUST audit all unsafe blocks for:                 │
  │   - Lifetime validity of raw pointers                      │
  │   - Alignment requirements                                 │
  │   - Aliasing rules (no &mut aliasing)                      │
  │   - Race conditions (extern static)                        │
  └────────────────────────────────────────────────────────────┘
         │ extern "C" — crosses language boundary
         ▼
  FFI BOUNDARY
  ┌────────────────────────────────────────────────────────────┐
  │  extern "C" {} — C/kernel ABI                              │
  │  Threat: untrusted data FROM C/kernel:                     │
  │   - Length fields may be attacker-controlled               │
  │   - Pointer validity not guaranteed                        │
  │   - Calling convention mismatch                            │
  │  Defense: validate ALL inputs at FFI boundary              │
  └────────────────────────────────────────────────────────────┘
         │ pub visibility — crosses module/crate boundary
         ▼
  API BOUNDARY
  ┌────────────────────────────────────────────────────────────┐
  │  pub / pub(crate) / pub(super)                             │
  │  Threat: public API is the attack surface                  │
  │   - Minimize pub surface                                   │
  │   - Use newtypes (struct Foo(Bar)) over type aliases       │
  │   - Typestate (PhantomData) to enforce protocol order      │
  │   - Constructor validation (no public fields + builder)    │
  └────────────────────────────────────────────────────────────┘

════════════════════════════════════════════════════════════════
```

**Keyword-specific security invariants**:

| Keyword | Security Invariant | Violation Consequence |
|---------|-------------------|----------------------|
| `unsafe` | Programmer-asserted pointer validity, aliasing | UAF, data race, memory corruption |
| `extern "C"` | Calling convention match; input validation | Stack corruption, logic bugs |
| `static mut` | External synchronization | Data race → UB |
| `union` | Correct variant tracking | Type confusion → logic bug |
| `as` (cast) | No truncation of security-critical values | Integer overflow in length calc |
| `pub` | Minimal surface | Increased attack surface |
| `move` + thread | All captures are `Send` | Data race if `unsafe impl Send` |
| `dyn Trait` | `Send + Sync + 'static` bounds in async | Memory unsafety across threads |

---

## 16. Next 3 Steps

**Step 1: Audit all `unsafe` blocks in your codebase with `cargo geiger`**
```bash
cargo install cargo-geiger
cargo geiger --all-features 2>&1 | grep -E "(unsafe|forbid|FORBID)"
# For each unsafe block: document the invariants it relies on as // Safety: comments
# Enforce with: #![deny(clippy::undocumented_unsafe_blocks)]
```

**Step 2: Run Miri to catch UB in unsafe code**
```bash
rustup toolchain install nightly
rustup component add miri --toolchain nightly
cargo +nightly miri test
# Miri detects: use-after-free, invalid raw ptr use, data races, alignment violations
# Run on unit tests that exercise unsafe code paths
```

**Step 3: Apply `#[deny]` lints to enforce keyword discipline**
```rust
// At crate root (lib.rs or main.rs)
#![deny(
    clippy::undocumented_unsafe_blocks,  // require Safety: on every unsafe
    clippy::cast_possible_truncation,    // flag lossy `as` casts
    clippy::cast_sign_loss,              // flag sign-losing casts
    clippy::wildcard_imports,            // no glob `use foo::*`
    missing_docs,                        // force documentation on pub items
)]

// Verify async Send bounds are correct:
fn assert_send<T: Send>(_: T) {}
fn assert_sync<T: Sync>(_: T) {}
// In tests: assert_send(your_future);
```

---

## References

- [Rust Reference: Keywords](https://doc.rust-lang.org/reference/keywords.html)
- [Rust Reference: Unsafety](https://doc.rust-lang.org/reference/unsafety.html)
- [Rust Reference: FFI](https://doc.rust-lang.org/reference/items/external-blocks.html)
- [Rust Nomicon: Unsafe Rust](https://doc.rust-lang.org/nomicon/)
- [Rustonomicon: Ownership and Lifetimes](https://doc.rust-lang.org/nomicon/ownership.html)
- [Async Book: Under the Hood](https://rust-lang.github.io/async-book/02_execution/01_chapter.html)
- [Tokio: Shared State](https://tokio.rs/tokio/tutorial/shared-state)
- [Linux Kernel Netlink: RFC 3549](https://datatracker.ietf.org/doc/html/rfc3549)
- [XDP Paper: The eXpress Data Path](https://dl.acm.org/doi/10.1145/3281411.3281443)
- [Rustc Dev Guide: NLL (Non-Lexical Lifetimes)](https://rustc-dev-guide.rust-lang.org/borrow_check/region_inference.html)
- [cargo-geiger: unsafe audit tool](https://github.com/geiger-rs/cargo-geiger)
- [Miri: UB detection](https://github.com/rust-lang/miri)

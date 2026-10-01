# Struct, Enum, and Union: A Complete In-Depth Guide
> C · Rust · Go · Linux Kernel · Network Subsystem

---

## Table of Contents

1. [Mental Model: Why These Three Constructs Exist](#1-mental-model)
2. [Memory Fundamentals: Alignment, Padding, and Layout](#2-memory-fundamentals)
3. [STRUCT — Heterogeneous Composite Types](#3-struct)
   - 3.1 C Structs
   - 3.2 Rust Structs
   - 3.3 Go Structs
   - 3.4 Linux Kernel Struct Patterns
   - 3.5 Network Subsystem Structs
4. [ENUM — Variant / Discriminated Types](#4-enum)
   - 4.1 C Enums
   - 4.2 Rust Enums (Algebraic Data Types)
   - 4.3 Go Enums via iota
   - 4.4 Linux Kernel Enums
5. [UNION — Overlapping Memory Types](#5-union)
   - 5.1 C Unions
   - 5.2 Rust Unions (unsafe)
   - 5.3 Go Unions (workarounds)
   - 5.4 Linux Kernel Union Usage
6. [Tagged Unions / Discriminated Unions](#6-tagged-unions)
7. [Memory Layout Deep Dive](#7-memory-layout-deep-dive)
8. [Bit Fields](#8-bit-fields)
9. [ABI, Packing, and `repr` in Rust](#9-abi-packing-repr)
10. [Pattern Matching and Exhaustiveness](#10-pattern-matching)
11. [Performance Implications](#11-performance-implications)
12. [Real-World Linux Kernel Case Studies](#12-linux-kernel-case-studies)
13. [Real-World Network Subsystem Case Studies](#13-network-subsystem-case-studies)
14. [Cross-Language Interoperability (FFI)](#14-cross-language-interoperability)
15. [Advanced Patterns and Idioms](#15-advanced-patterns)
16. [Summary: Decision Matrix](#16-summary-decision-matrix)

---

## 1. Mental Model

Before diving into syntax, build the right mental model. The three constructs solve three orthogonal problems:

```
PROBLEM                        CONSTRUCT      CORE IDEA
──────────────────────────────────────────────────────────────────
"I need several things at once" → struct  → fields laid out sequentially
"I need one of several things"  → enum    → variants with discriminant
"I need to reinterpret memory"  → union   → fields share the same bytes
```

Think of memory as a raw byte array. These three constructs are **interpretations** of that byte array:

```
Physical RAM (byte array)
┌───┬───┬───┬───┬───┬───┬───┬───┐
│ 0 │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │  ← raw bytes
└───┴───┴───┴───┴───┴───┴───┴───┘

struct:  field_a occupies [0..3], field_b occupies [4..7]  — both live simultaneously
union:   field_a AND field_b BOTH occupy [0..7]             — only one valid at a time
enum:    byte 0 = discriminant tag; [1..7] = variant data   — tag tells you which variant
```

### The Algebraic Analogy

| Construct | Algebra | Also Called |
|-----------|---------|-------------|
| struct | **Product type** (A × B × C) | Record, Tuple |
| enum | **Sum type** (A + B + C) | Variant, Discriminated union, ADT |
| union | **Unsafe overlay** | Overlapping storage |

A struct of 3 fields has `|A| × |B| × |C|` possible states.
An enum of 3 variants has `|A| + |B| + |C|` possible states.
This is why Rust/Haskell enums are called **algebraic data types**.

---

## 2. Memory Fundamentals

### 2.1 Alignment

Every type has an **alignment requirement** — the addresses at which it may be stored must be multiples of that alignment.

```
Type         Size    Alignment (typical x86-64)
─────────────────────────────────────────────────
char / u8     1       1
short / u16   2       2
int / u32     4       4
long / u64    8       8
float         4       4
double        8       8
pointer       8       8
```

Alignment exists because CPUs fetch aligned addresses in a single bus cycle. An unaligned 4-byte integer at address 0x03 would span two cache lines and require two loads + merge.

### 2.2 Padding

The compiler inserts **padding bytes** between struct fields so each field satisfies its alignment:

```
struct Foo {       // C
    char  a;       // 1 byte  @ offset 0
    // 3 bytes padding
    int   b;       // 4 bytes @ offset 4
    char  c;       // 1 byte  @ offset 8
    // 7 bytes padding
    long  d;       // 8 bytes @ offset 16
};
// sizeof(Foo) = 24

Memory layout:
Offset: 0    1    2    3    4    5    6    7    8    9   10   11   12   13   14   15   16   17 .. 23
        [  a  ][  padding  ][     b     ][  c  ][       padding       ][         d          ]
```

### 2.3 Struct Total Size Alignment (Tail Padding)

The struct's total size is rounded up to a multiple of its **largest member's alignment**:

```c
struct Bar {
    int  x;   // 4 bytes @ 0
    char y;   // 1 byte  @ 4
    // 3 bytes tail padding → total = 8
};
// sizeof(Bar) = 8, not 5
// This ensures Bar[] arrays keep each element aligned.
```

### 2.4 Cache Lines

Modern CPUs fetch 64-byte cache lines. Struct layout affects cache performance dramatically:

```
Cache line: 64 bytes
┌──────────────────────────────────────────────────────────────────┐
│  0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 ... 63       │
└──────────────────────────────────────────────────────────────────┘

A "hot" struct whose frequently-accessed fields fit in one cache line is fast.
A struct where hot field A is at offset 0 and hot field B is at offset 80 is slow.
```

---

## 3. STRUCT

A struct is a **named, ordered collection of named fields**. All fields exist simultaneously.

### 3.1 C Structs

#### Basic Declaration

```c
struct Point {
    int x;
    int y;
};

// C99+: typedef to avoid writing 'struct' everywhere
typedef struct {
    int x;
    int y;
} Point;

// Usage
Point p = { .x = 10, .y = 20 };  // designated initializers (C99)
Point q = { 10, 20 };             // positional
p.x = 5;
```

#### Nested Structs

```c
struct Rect {
    struct Point top_left;
    struct Point bottom_right;
};

// Memory layout of Rect (each Point = 8 bytes):
// [0..3] top_left.x
// [4..7] top_left.y
// [8..11] bottom_right.x
// [12..15] bottom_right.y
// sizeof(Rect) = 16
```

#### Flexible Array Members (C99)

```c
struct Packet {
    uint32_t length;
    uint8_t  data[];   // flexible array member — must be last field
};

// Allocate dynamically:
struct Packet *pkt = malloc(sizeof(struct Packet) + 256);
pkt->length = 256;
// pkt->data[0..255] are valid

// This pattern is used EXTENSIVELY in the Linux kernel
```

#### Forward Declaration and Opaque Structs

```c
// Forward declare — enables pointer-to-incomplete-type (opaque pointer idiom)
struct Node;

struct Node {
    int         value;
    struct Node *next;   // self-referential — only works with pointer
};

// Opaque handle (hide implementation from header):
// foo.h
typedef struct Foo_s Foo;         // incomplete type
Foo *foo_create(void);
void foo_destroy(Foo *f);

// foo.c
struct Foo_s {
    int  internal_state;
    char buffer[64];
};
```

#### Pointer Arithmetic and Struct Layout

```c
struct Header {
    uint32_t magic;
    uint16_t version;
    uint16_t flags;
    uint32_t length;
};

struct Header h;
// &h.magic   == (char*)&h + 0
// &h.version == (char*)&h + 4
// &h.flags   == (char*)&h + 6
// &h.length  == (char*)&h + 8
// sizeof(h)  == 12
```

#### Struct Assignment and Shallow Copy

```c
struct Point a = {1, 2};
struct Point b = a;      // memberwise copy — a and b are independent
b.x = 99;                // does not affect a.x
```

#### Self-Referential and Intrusive Data Structures

```c
// Intrusive linked list — used heavily in Linux kernel
struct list_node {
    struct list_node *prev;
    struct list_node *next;
};

struct Task {
    int              pid;
    char             name[16];
    struct list_node list;   // embedded link — intrusive
};

// To get outer struct from pointer to embedded field: container_of macro
// (explained in Section 12)
```

#### Packed Structs

```c
#pragma pack(1)           // MSVC
struct __attribute__((packed)) PackedHdr {  // GCC/Clang
    uint8_t  type;
    uint32_t length;       // at offset 1, NOT 4
    uint16_t checksum;     // at offset 5
};
#pragma pack()
// sizeof = 7 (no padding), but accessing length may be unaligned — UB risk
```

### 3.2 Rust Structs

Rust has three kinds of structs:

#### Named-Field Struct

```rust
struct Point {
    x: f64,
    y: f64,
}

let p = Point { x: 1.0, y: 2.0 };
println!("{}", p.x);

// Struct update syntax
let q = Point { x: 5.0, ..p };  // q.y == p.y
```

#### Tuple Struct (positional fields)

```rust
struct Color(u8, u8, u8);     // RGB
struct Meters(f64);            // newtype pattern

let red = Color(255, 0, 0);
let r = red.0;                  // access by index
let dist = Meters(42.0);
let raw: f64 = dist.0;
```

#### Unit Struct (zero-sized)

```rust
struct Marker;                  // zero-sized type (ZST)
struct PhantomData<T>;          // from std — carries type info, zero size

// ZSTs are used for:
// - type-level state machines
// - phantom type parameters
// - marker traits
let m = Marker;
assert_eq!(std::mem::size_of::<Marker>(), 0);
```

#### Visibility and Access

```rust
pub struct Config {
    pub host: String,      // public field
    pub port: u16,         // public field
    timeout_ms: u32,       // private field
}

impl Config {
    pub fn new(host: String, port: u16) -> Self {
        Config { host, port, timeout_ms: 5000 }
    }
    pub fn timeout(&self) -> u32 { self.timeout_ms }
}
```

#### Methods and impl Blocks

```rust
struct Rect {
    width: f64,
    height: f64,
}

impl Rect {
    // associated function (no self) — like static method
    pub fn new(w: f64, h: f64) -> Self {
        Rect { width: w, height: h }
    }

    // immutable method
    pub fn area(&self) -> f64 {
        self.width * self.height
    }

    // mutable method
    pub fn scale(&mut self, factor: f64) {
        self.width  *= factor;
        self.height *= factor;
    }

    // consuming method
    pub fn into_square(self) -> Rect {
        let side = self.width.min(self.height);
        Rect { width: side, height: side }
    }
}
```

#### Derived Traits

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
struct Point {
    x: i32,
    y: i32,
}
// Debug:    fmt with {:?}
// Clone:    explicit .clone()
// Copy:     implicit bitwise copy (stack types only)
// PartialEq/Eq: == operator
// Hash:     use as HashMap key
```

#### Rust repr Attributes (Critical for FFI/layout)

```rust
#[repr(C)]           // C-compatible layout — field order preserved, C padding rules
struct CStruct {
    a: u8,
    b: u32,
}

#[repr(packed)]      // no padding — potentially unaligned access
struct Packed {
    a: u8,
    b: u32,          // at offset 1
}

#[repr(align(16))]   // force minimum 16-byte alignment
struct Aligned {
    data: [u8; 8],
}

#[repr(transparent)] // same layout as single non-ZST field — for newtype FFI
struct Wrapper(u32);
```

#### Generic Structs

```rust
struct Stack<T> {
    items: Vec<T>,
}

impl<T> Stack<T> {
    pub fn new() -> Self { Stack { items: Vec::new() } }
    pub fn push(&mut self, item: T) { self.items.push(item); }
    pub fn pop(&mut self) -> Option<T> { self.items.pop() }
    pub fn peek(&self) -> Option<&T> { self.items.last() }
}

// Bounded generic
struct Pair<T: Clone + std::fmt::Debug> {
    first: T,
    second: T,
}
```

#### Lifetime Annotations in Structs

```rust
// Struct that borrows data — must carry lifetime
struct StrSplit<'a> {
    remainder: &'a str,
    delimiter: &'a str,
}
// The struct cannot outlive the string it borrows from.

struct ImportantExcerpt<'a> {
    part: &'a str,
}
```

### 3.3 Go Structs

#### Basic Syntax

```go
type Point struct {
    X float64
    Y float64
}

p := Point{X: 1.0, Y: 2.0}
q := Point{1.0, 2.0}   // positional — avoid in practice
fmt.Println(p.X)
```

#### Embedded Structs (Composition / Promotion)

```go
type Animal struct {
    Name string
}

func (a Animal) Speak() string { return a.Name + " speaks" }

type Dog struct {
    Animal          // embedded — fields and methods promoted
    Breed string
}

d := Dog{
    Animal: Animal{Name: "Rex"},
    Breed:  "Labrador",
}
fmt.Println(d.Name)      // promoted from Animal
fmt.Println(d.Speak())   // promoted method
fmt.Println(d.Animal.Name) // explicit access
```

#### Struct Tags (Metadata for Reflection)

```go
type User struct {
    ID       int    `json:"id"        db:"user_id"`
    Username string `json:"username"  db:"uname"   validate:"required,min=3"`
    Password string `json:"-"`        // omit from JSON
    Email    string `json:"email,omitempty"`
}

// Tags are read at runtime via reflect package:
import "reflect"
t := reflect.TypeOf(User{})
f, _ := t.FieldByName("Username")
fmt.Println(f.Tag.Get("json"))   // "username"
```

#### Anonymous Fields

```go
type Named struct {
    string   // anonymous field — accessed as .string
    int
}

n := Named{"hello", 42}
fmt.Println(n.string)  // "hello"
```

#### Pointer vs Value Receivers

```go
type Counter struct {
    count int
}

// Value receiver: operates on copy
func (c Counter) Value() int { return c.count }

// Pointer receiver: modifies original
func (c *Counter) Increment() { c.count++ }

c := Counter{}
c.Increment()     // Go auto-takes address
fmt.Println(c.Value())  // 1
```

#### Struct Initialization Patterns

```go
// Zero value: all fields zero-initialized
var p Point   // {0.0, 0.0}

// Constructor function (idiomatic Go)
func NewPoint(x, y float64) *Point {
    return &Point{X: x, Y: y}
}

// Options pattern for complex initialization
type Server struct {
    host    string
    port    int
    timeout time.Duration
}

type Option func(*Server)

func WithPort(p int) Option { return func(s *Server) { s.port = p } }
func WithTimeout(d time.Duration) Option { return func(s *Server) { s.timeout = d } }

func NewServer(host string, opts ...Option) *Server {
    s := &Server{host: host, port: 8080, timeout: 30 * time.Second}
    for _, opt := range opts { opt(s) }
    return s
}
```

#### Interface Satisfaction

```go
type Shape interface {
    Area() float64
    Perimeter() float64
}

type Rectangle struct { Width, Height float64 }

// Rectangle implicitly satisfies Shape (no 'implements' keyword)
func (r Rectangle) Area() float64      { return r.Width * r.Height }
func (r Rectangle) Perimeter() float64 { return 2 * (r.Width + r.Height) }

var s Shape = Rectangle{Width: 10, Height: 5}
```

### 3.4 Linux Kernel Struct Patterns

The Linux kernel uses C structs extensively with several distinctive patterns:

#### task_struct — The Process Descriptor

```c
// include/linux/sched.h (simplified)
struct task_struct {
    // ──── State ────────────────────────────────────────────────
    volatile long              state;          // TASK_RUNNING, TASK_INTERRUPTIBLE, etc.
    void                      *stack;          // kernel stack pointer

    // ──── Scheduling ───────────────────────────────────────────
    int                        prio;
    int                        static_prio;
    int                        normal_prio;
    unsigned int               rt_priority;
    const struct sched_class  *sched_class;
    struct sched_entity        se;             // CFS scheduler entity
    struct sched_rt_entity     rt;
    struct sched_dl_entity     dl;

    // ──── Process Identity ─────────────────────────────────────
    pid_t                      pid;
    pid_t                      tgid;
    struct task_struct        *real_parent;
    struct task_struct        *parent;
    struct list_head           children;      // list of children
    struct list_head           sibling;       // next/prev sibling

    // ──── Memory ───────────────────────────────────────────────
    struct mm_struct          *mm;            // user-space memory descriptor
    struct mm_struct          *active_mm;

    // ──── Files ────────────────────────────────────────────────
    struct fs_struct          *fs;
    struct files_struct       *files;

    // ──── Signals ──────────────────────────────────────────────
    struct signal_struct      *signal;
    struct sighand_struct     *sighand;
    sigset_t                   blocked;
    struct sigpending          pending;

    // ──── Credentials ──────────────────────────────────────────
    const struct cred         *real_cred;
    const struct cred         *cred;

    char                       comm[TASK_COMM_LEN];  // process name

    // ──── Namespaces ───────────────────────────────────────────
    struct nsproxy            *nsproxy;

    // ... (hundreds more fields)
};

/*
 Layout concept (simplified):

 task_struct
 ┌───────────────────────────────────────────────────────┐
 │  state (8)                                            │
 │  stack ptr (8)                                        │
 ├───────────────────────────────────────────────────────┤
 │  sched_entity se        ← CFS scheduling info         │
 │    ├── rb_node node     ← position in red-black tree  │
 │    ├── u64 exec_start                                 │
 │    └── u64 vruntime                                   │
 ├───────────────────────────────────────────────────────┤
 │  pid, tgid (4+4)                                      │
 │  list_head children  → [prev ptr][next ptr]           │
 │  list_head sibling   → [prev ptr][next ptr]           │
 ├───────────────────────────────────────────────────────┤
 │  mm_struct *mm  (8) → points to page tables           │
 │  files_struct *  (8) → open file descriptors          │
 └───────────────────────────────────────────────────────┘
*/
```

#### list_head — The Intrusive Linked List

```c
// include/linux/types.h
struct list_head {
    struct list_head *next;
    struct list_head *prev;
};

// This is embedded INTO other structs:
struct my_item {
    int              data;
    struct list_head list;   // intrusive node
};

// To get the containing struct from a list_head pointer:
#define container_of(ptr, type, member) ({              \
    const typeof(((type *)0)->member) *__mptr = (ptr);  \
    (type *)((char *)__mptr - offsetof(type, member)); })

// Usage:
struct list_head *pos;
struct my_item *item = container_of(pos, struct my_item, list);

/*
 Why intrusive lists?
 - No extra heap allocation for a 'node wrapper'
 - One item can be on MULTIPLE lists simultaneously (multiple list_head members)
 - Better cache locality — data and link pointers adjacent

 Memory diagram:
 my_item A             my_item B             my_item C
 ┌──────────┐          ┌──────────┐          ┌──────────┐
 │ data: 1  │          │ data: 2  │          │ data: 3  │
 │ list:    │          │ list:    │          │ list:    │
 │  next ───┼─────────►│  next ───┼─────────►│  next ─┐ │
 │  prev  ◄─┼──────────┤  prev   │◄──────────┤  prev  │ │
 └──────────┘          └──────────┘          └────────┼─┘
      ▲                                               │
      └───────────────────────────────────────────────┘
*/
```

#### kobject — The Kernel Object Base

```c
struct kobject {
    const char              *name;
    struct list_head         entry;
    struct kobject          *parent;
    struct kset             *kset;
    struct kobj_type        *ktype;
    struct kernfs_node      *sd;     // sysfs directory entry
    struct kref              kref;   // reference count
    unsigned int             state_initialized:1;
    unsigned int             state_in_sysfs:1;
    unsigned int             state_add_uevent_sent:1;
    unsigned int             state_remove_uevent_sent:1;
    unsigned int             uevent_suppress:1;
};

// kobject is embedded in every kernel object:
struct device {
    struct kobject  kobj;   // FIRST member — allows upcast
    struct device  *parent;
    // ...
};
```

### 3.5 Network Subsystem Structs

#### sk_buff — The Socket Buffer (most important network struct)

```c
// include/linux/skbuff.h (simplified)
struct sk_buff {
    // ──── Transport/routing ────────────────────────────────────
    union {
        struct net_device  *dev;     // outgoing device
        unsigned long       dev_scratch;
    };

    // ──── Data pointers (the core layout) ──────────────────────
    unsigned char          *head;    // start of allocated buffer
    unsigned char          *data;    // start of actual data
    unsigned char          *tail;    // end of actual data
    unsigned char          *end;     // end of allocated buffer

    /*
     Buffer layout:
     head                 data           tail       end
      │                    │              │           │
      ▼                    ▼              ▼           ▼
     ┌────────────────────┬──────────────┬───────────┐
     │   headroom         │  packet data │  tailroom │
     │ (for prepending    │  (L2 header  │  (for     │
     │  headers)          │  + payload)  │  appending│
     └────────────────────┴──────────────┴───────────┘

     skb_headroom(skb) = data - head
     skb_tailroom(skb) = end  - tail
     skb->len          = tail - data (in linear area)
    */

    // ──── Length fields ────────────────────────────────────────
    unsigned int            len;         // total length of all data
    unsigned int            data_len;    // length of paged data
    __u16                   mac_len;     // MAC header length
    __u16                   hdr_len;     // copy of full header length

    // ──── Checksum ─────────────────────────────────────────────
    __wsum                  csum;
    union {
        __u16               csum_start;
        __u16               csum_bad;
    };
    __u16                   csum_offset;

    // ──── Protocol / type ──────────────────────────────────────
    __be16                  protocol;    // ETH_P_IP, ETH_P_IPV6, etc.
    __u16                   transport_header;
    __u16                   network_header;
    __u16                   mac_header;

    // ──── Queue management ─────────────────────────────────────
    struct sk_buff         *next;
    struct sk_buff         *prev;
    struct rb_node          rbnode;

    // ──── Socket association ───────────────────────────────────
    struct sock            *sk;

    // ──── Timestamps ───────────────────────────────────────────
    ktime_t                 tstamp;

    // ──── Misc flags ───────────────────────────────────────────
    __u8                    pkt_type:3;  // PACKET_HOST, PACKET_BROADCAST, etc.
    __u8                    ignore_df:1;
    __u8                    ip_summed:2;
    __u8                    cloned:1;
    __u8                    nohdr:1;
};

/*
 sk_buff traversal through the network stack:

 NIC (hardware)
    │  skb allocated, data filled
    ▼
 netif_receive_skb()         ← L2 entry point
    │  mac_header set
    ▼
 eth_type_trans()            ← determine protocol
    │  protocol = ETH_P_IP
    ▼
 ip_rcv()                    ← IP layer
    │  network_header set, IP header parsed
    ▼
 tcp_v4_rcv() / udp_rcv()   ← Transport layer
    │  transport_header set
    ▼
 sock_queue_rcv_skb()        ← deliver to socket
    │
    ▼
 Application (read/recv)
*/
```

#### sock / socket — The Socket Struct Hierarchy

```c
/*
 The socket struct hierarchy uses struct embedding for polymorphism:

 socket (VFS layer)
   └─ sock (generic socket)
        ├─ inet_sock (IP-specific)
        │    ├─ inet_connection_sock (connection-oriented)
        │    │    ├─ tcp_sock (TCP)
        │    │    └─ dccp_sock (DCCP)
        │    └─ udp_sock (UDP)
        └─ unix_sock (Unix domain)

 Memory layout (TCP socket):
 ┌──────────────────────────────────────────────┐
 │  tcp_sock                                    │
 │  ├── inet_connection_sock icsk               │
 │  │   ├── inet_sock inet    ← cast to this    │
 │  │   │   ├── sock sk       ← cast to this    │
 │  │   │   │   ├── sock_common __sk_common     │
 │  │   │   │   │   ├── skc_family              │
 │  │   │   │   │   └── skc_state              │
 │  │   │   │   ├── sk_rcvbuf                   │
 │  │   │   │   └── sk_sndbuf                  │
 │  │   │   └── inet_daddr  (dest IP)           │
 │  │   └── icsk_retransmit_timer               │
 │  ├── tcp_send_head                           │
 │  ├── snd_una, snd_nxt, rcv_nxt              │
 │  └── tcp_options_received                   │
 └──────────────────────────────────────────────┘
*/

struct sock {
    struct sock_common  __sk_common;
    socket_lock_t       sk_lock;
    int                 sk_rcvbuf;
    int                 sk_sndbuf;
    struct sk_buff_head sk_receive_queue;
    struct sk_buff_head sk_write_queue;
    struct proto       *sk_prot;
    // ...
};

struct inet_sock {
    struct sock         sk;          // MUST be first
    __be32              inet_saddr;
    __be32              inet_daddr;
    __be16              inet_sport;
    __be16              inet_dport;
    // ...
};

// Type punning via casting (safe because struct is first member):
struct inet_sock *inet = (struct inet_sock *)sk;
```

#### netdev — Network Device Struct

```c
struct net_device {
    char                    name[IFNAMSIZ];   // "eth0", "lo", etc.
    unsigned long           mem_end;
    unsigned long           mem_start;
    unsigned long           base_addr;
    int                     irq;

    unsigned long           state;
    struct list_head        dev_list;
    struct list_head        napi_list;

    unsigned int            flags;          // IFF_UP, IFF_BROADCAST, etc.
    unsigned int            priv_flags;
    unsigned short          type;           // ARPHRD_ETHER, etc.
    unsigned short          hard_header_len;
    unsigned int            mtu;
    unsigned int            min_mtu;
    unsigned int            max_mtu;

    unsigned char           perm_addr[MAX_ADDR_LEN];
    unsigned char           dev_addr[MAX_ADDR_LEN];  // current MAC

    const struct net_device_ops   *netdev_ops;   // vtable
    const struct ethtool_ops      *ethtool_ops;

    struct netdev_rx_queue  *_rx;           // RX queues
    unsigned int             num_rx_queues;
    struct netdev_queue     *_tx;           // TX queues
    unsigned int             num_tx_queues;

    struct Qdisc            *qdisc;         // TX queue discipline
};

// Driver operations vtable (function pointer struct):
struct net_device_ops {
    int  (*ndo_open)(struct net_device *dev);
    int  (*ndo_stop)(struct net_device *dev);
    netdev_tx_t (*ndo_start_xmit)(struct sk_buff *skb, struct net_device *dev);
    void (*ndo_set_rx_mode)(struct net_device *dev);
    int  (*ndo_set_mac_address)(struct net_device *dev, void *addr);
    int  (*ndo_ioctl)(struct net_device *dev, struct ifreq *ifr, int cmd);
    // ...
};
```

---

## 4. ENUM

An enum defines a type that can hold **exactly one of several variants** at any given time.

### 4.1 C Enums

C enums are simply **named integer constants**. They carry no data and are not type-safe.

#### Basic C Enum

```c
enum Direction {
    NORTH = 0,
    EAST  = 1,
    SOUTH = 2,
    WEST  = 3,
};

enum Direction d = NORTH;

// C enums are just ints — no type safety:
int n = NORTH;           // OK in C (implicit conversion)
d = 999;                 // OK — no bounds checking
```

#### Enum with Explicit Values

```c
enum HttpStatus {
    HTTP_OK           = 200,
    HTTP_CREATED      = 201,
    HTTP_NO_CONTENT   = 204,
    HTTP_BAD_REQUEST  = 400,
    HTTP_UNAUTHORIZED = 401,
    HTTP_FORBIDDEN    = 403,
    HTTP_NOT_FOUND    = 404,
    HTTP_SERVER_ERROR = 500,
};

// Automatic increment from last explicit value:
enum Flags {
    FLAG_NONE  = 0,
    FLAG_READ  = 1 << 0,   // 1
    FLAG_WRITE = 1 << 1,   // 2
    FLAG_EXEC  = 1 << 2,   // 4
};
```

#### Size of C Enum

```c
// The compiler chooses the underlying integer type (typically int)
enum Color { RED, GREEN, BLUE };
printf("%zu\n", sizeof(enum Color));  // usually 4 (sizeof int)
```

#### Enum in Switch

```c
// C compilers warn about missing enum cases in switch (with -Wswitch)
enum State { IDLE, RUNNING, STOPPED };

void handle(enum State s) {
    switch (s) {
        case IDLE:    printf("idle\n");    break;
        case RUNNING: printf("running\n"); break;
        case STOPPED: printf("stopped\n"); break;
        // no default: compiler warns if new variant added and not handled
    }
}
```

#### C Enum Limitations

```
Limitation 1: No data attached to variants
    enum Msg { TEXT, NUMBER };      // which? we don't know the value
    // Need a separate union to carry data — ugly

Limitation 2: No type safety
    enum A { FOO = 1 };
    enum B { BAR = 1 };
    enum A a = BAR;  // compiles in C (warning in C++)

Limitation 3: Namespace pollution
    enum Color { RED, GREEN, BLUE };
    enum Error { RED_ERROR, GREEN_ERROR };  // RED already defined!
    // In C: must prefix: COLOR_RED, ERROR_RED
```

### 4.2 Rust Enums (Algebraic Data Types)

Rust enums are **sum types** — each variant can carry its own data. This is fundamentally more powerful than C enums.

#### Unit Variants (C-style)

```rust
enum Direction {
    North,
    East,
    South,
    West,
}

let d = Direction::North;
```

#### Variants with Data

```rust
enum Message {
    Quit,                          // unit variant — no data
    Move { x: i32, y: i32 },      // struct variant
    Write(String),                 // tuple variant
    ChangeColor(u8, u8, u8),       // tuple variant, multiple fields
}

let m1 = Message::Quit;
let m2 = Message::Move { x: 10, y: 20 };
let m3 = Message::Write(String::from("hello"));
let m4 = Message::ChangeColor(255, 0, 0);
```

#### Memory Layout of Rust Enums

```rust
// Rust lays out an enum as: discriminant tag + largest variant data
// The discriminant is just enough bits to distinguish variants

enum Coin {
    Penny,      // 0
    Nickel,     // 1
    Dime,       // 2
    Quarter,    // 3
}
// size = 1 byte (tag only, no data needed)

enum Msg {
    A,                         // tag=0, no data
    B(u64),                    // tag=1, 8 bytes data
    C { x: u32, y: u32 },     // tag=2, 8 bytes data
}
// Memory layout:
// ┌───┬─────────────────────┐
// │tag│     data (8 bytes)  │
// └───┴─────────────────────┘
// size = 1 (tag) + 7 (padding) + 8 (data) = 16 bytes
```

#### Option<T> — The Null-Safety Enum

```rust
// Defined in std:
pub enum Option<T> {
    None,       // no value
    Some(T),    // has a value of type T
}

fn divide(a: f64, b: f64) -> Option<f64> {
    if b == 0.0 { None } else { Some(a / b) }
}

let result = divide(10.0, 2.0);

// Pattern matching — exhaustive
match result {
    Some(v) => println!("Result: {}", v),
    None    => println!("Division by zero"),
}

// Combinator methods on Option
let doubled = result.map(|v| v * 2.0);
let or_zero = result.unwrap_or(0.0);
let mapped  = result.and_then(|v| if v > 0.0 { Some(v) } else { None });

// Null pointer optimization:
// sizeof(Option<&T>)  == sizeof(*mut T)  (None == null pointer)
// sizeof(Option<Box<T>>) == sizeof(Box<T>)
assert_eq!(std::mem::size_of::<Option<&i32>>(), std::mem::size_of::<&i32>());
```

#### Result<T, E> — The Error-Handling Enum

```rust
pub enum Result<T, E> {
    Ok(T),
    Err(E),
}

#[derive(Debug)]
enum ParseError {
    InvalidChar(char),
    UnexpectedEof,
    Overflow,
}

fn parse_number(s: &str) -> Result<i64, ParseError> {
    if s.is_empty() {
        return Err(ParseError::UnexpectedEof);
    }
    s.parse::<i64>().map_err(|_| ParseError::Overflow)
}

// The ? operator propagates errors
fn process(input: &str) -> Result<String, ParseError> {
    let n = parse_number(input)?;   // returns Err if parse fails
    Ok(format!("Parsed: {}", n))
}

// Combinators
let doubled = parse_number("42").map(|n| n * 2);
let default_ = parse_number("bad").unwrap_or(0);
```

#### Enums as State Machines

```rust
enum TcpState {
    Closed,
    Listen,
    SynSent    { seq_num: u32 },
    SynReceived { seq_num: u32, ack_num: u32 },
    Established { local_seq: u32, remote_seq: u32 },
    FinWait1   { local_seq: u32 },
    FinWait2,
    CloseWait,
    Closing,
    LastAck,
    TimeWait   { timer_expiry: u64 },
}

struct Connection {
    state:       TcpState,
    local_port:  u16,
    remote_port: u16,
}

impl Connection {
    fn transition(&mut self, event: TcpEvent) {
        self.state = match (&self.state, event) {
            (TcpState::Closed, TcpEvent::Listen)  => TcpState::Listen,
            (TcpState::Listen, TcpEvent::SynRecv { seq }) =>
                TcpState::SynReceived { seq_num: seq, ack_num: seq + 1 },
            (TcpState::SynReceived { ack_num, .. }, TcpEvent::AckRecv) =>
                TcpState::Established { local_seq: 1000, remote_seq: *ack_num },
            // ... etc.
            _ => panic!("Invalid transition"),
        };
    }
}
```

#### Recursive Enums and Box

```rust
// Direct recursion doesn't work — infinite size:
// enum List { Cons(i32, List), Nil }  // ERROR: recursive type has infinite size

// Fix: use Box to break the recursion (heap allocation)
enum List {
    Cons(i32, Box<List>),
    Nil,
}

let list = List::Cons(1, Box::new(List::Cons(2, Box::new(List::Nil))));

// Binary tree:
enum Tree<T> {
    Leaf(T),
    Node(Box<Tree<T>>, Box<Tree<T>>),
}

/*
 Memory layout of List::Cons(1, Box<...>):
 ┌─────┬───────────────────┐
 │ tag │ i32 │   Box ptr   │
 │  1  │  1  │ 0xdeadbeef  │
 └─────┴───────────────────┘
                │
                ▼ heap
          ┌─────┬───────────────────┐
          │ tag │ i32 │   Box ptr   │
          │  1  │  2  │ 0xcafebabe  │
          └─────┴───────────────────┘
                        │
                        ▼ heap
                  ┌─────┐
                  │ tag │  (Nil)
                  │  0  │
                  └─────┘
*/
```

#### Enum Methods and Pattern Matching

```rust
#[derive(Debug)]
enum Shape {
    Circle { radius: f64 },
    Rectangle { width: f64, height: f64 },
    Triangle { base: f64, height: f64 },
}

impl Shape {
    fn area(&self) -> f64 {
        match self {
            Shape::Circle { radius } => std::f64::consts::PI * radius * radius,
            Shape::Rectangle { width, height } => width * height,
            Shape::Triangle { base, height } => 0.5 * base * height,
        }
    }

    fn is_circle(&self) -> bool {
        matches!(self, Shape::Circle { .. })
    }
}

// if let — for single variant
let s = Shape::Circle { radius: 5.0 };
if let Shape::Circle { radius } = s {
    println!("radius = {}", radius);
}

// while let
let mut stack = vec![1, 2, 3];
while let Some(top) = stack.pop() {
    println!("{}", top);
}
```

### 4.3 Go Enums via iota

Go has no `enum` keyword. The idiomatic approach uses `const` + `iota`:

#### Basic iota

```go
type Direction int

const (
    North Direction = iota  // 0
    East                    // 1
    South                   // 2
    West                    // 3
)
```

#### iota with Expressions

```go
type ByteSize float64

const (
    _           = iota               // discard 0
    KB ByteSize = 1 << (10 * iota)  // 1024
    MB                               // 1048576
    GB                               // 1073741824
    TB
    PB
)

// Bit flags:
type FileMode uint

const (
    Read    FileMode = 1 << iota  // 1
    Write                          // 2
    Execute                        // 4
)
```

#### String Method (Stringer interface)

```go
type State int

const (
    Idle State = iota
    Running
    Stopped
)

func (s State) String() string {
    switch s {
    case Idle:    return "Idle"
    case Running: return "Running"
    case Stopped: return "Stopped"
    default:      return fmt.Sprintf("State(%d)", int(s))
    }
}
```

#### Typed vs Untyped Constants

```go
// Typed — type-safe
type Color int
const Red Color = 0

// Untyped — flexible
const Pi = 3.14159  // untyped constant — adapts to context
```

#### Go's Interface-Based Polymorphism (replaces enum with data)

```go
// Where Rust uses: enum Shape { Circle(f64), Rect(f64, f64) }
// Go uses an interface + concrete types:

type Shape interface {
    Area() float64
}

type Circle struct { Radius float64 }
type Rect   struct { W, H   float64 }

func (c Circle) Area() float64 { return math.Pi * c.Radius * c.Radius }
func (r Rect)   Area() float64 { return r.W * r.H }

// To get variant-like behavior, use type switch:
func describe(s Shape) {
    switch v := s.(type) {
    case Circle: fmt.Printf("Circle r=%.2f\n", v.Radius)
    case Rect:   fmt.Printf("Rect %gx%g\n", v.W, v.H)
    default:     fmt.Printf("Unknown shape: %T\n", v)
    }
}
```

### 4.4 Linux Kernel Enums

```c
// TCP state machine (net/tcp_states.h)
enum tcp_ca_state {
    TCP_CA_Open     = 0,
    TCP_CA_Disorder = 1,
    TCP_CA_CWR      = 2,
    TCP_CA_Recovery = 3,
    TCP_CA_Loss     = 4,
};

// Socket states (include/linux/net.h)
enum sock_type {
    SOCK_STREAM    = 1,
    SOCK_DGRAM     = 2,
    SOCK_RAW       = 3,
    SOCK_RDM       = 4,
    SOCK_SEQPACKET = 5,
    SOCK_DCCP      = 6,
    SOCK_PACKET    = 10,
};

// NF hook points (include/linux/netfilter.h)
enum nf_inet_hooks {
    NF_INET_PRE_ROUTING,   // 0 — before routing decision
    NF_INET_LOCAL_IN,      // 1 — for local processes
    NF_INET_FORWARD,       // 2 — forwarded packets
    NF_INET_LOCAL_OUT,     // 3 — locally generated
    NF_INET_POST_ROUTING,  // 4 — after routing decision
    NF_INET_NUMHOOKS,
};

/*
 Netfilter hook points in packet flow:

 ┌─────────────────────────────────────────────────────────────────┐
 │                         Network Stack                           │
 │                                                                 │
 │  NIC → PRE_ROUTING → [routing decision]                         │
 │                          │              │                       │
 │                    LOCAL_IN          FORWARD                    │
 │                          │              │                       │
 │                       Process       POST_ROUTING → NIC          │
 │                                                                 │
 │  LOCAL_OUT ─────────────────────────► POST_ROUTING → NIC        │
 └─────────────────────────────────────────────────────────────────┘
*/

// Device operational states (include/linux/netdevice.h)
enum netdev_state_t {
    __LINK_STATE_START,
    __LINK_STATE_PRESENT,
    __LINK_STATE_NOCARRIER,
    __LINK_STATE_LINKWATCH_PENDING,
    __LINK_STATE_DORMANT,
    __LINK_STATE_TESTING,
};
```

---

## 5. UNION

A union allows multiple fields to **occupy the same memory location**. Only one field is valid at any time. The union's size equals its **largest member's size**.

### 5.1 C Unions

#### Basic Union

```c
union Data {
    int    i;
    float  f;
    char   str[20];
};

union Data d;
d.i = 42;         // write as int
printf("%d\n", d.i);   // read as int — OK

d.f = 3.14f;      // write as float — overwrites d.i
printf("%f\n", d.f);   // read as float — OK
printf("%d\n", d.i);   // UNDEFINED BEHAVIOR — reading wrong variant

/*
 Memory layout of union Data:
 ┌──────────────────────────────────────────────┐
 │  0  1  2  3  4  5  6  7  8 ... 19            │
 │  ◄──────── str (20 bytes) ────────────────►   │
 │  ◄─── i (4 bytes) ───►                        │
 │  ◄─── f (4 bytes) ───►                        │
 └──────────────────────────────────────────────┘
 sizeof(union Data) = 20 (largest member: str[20])
*/
```

#### Union Alignment

```c
union Mix {
    char  c;    // 1 byte, align 1
    int   i;    // 4 bytes, align 4
    double d;   // 8 bytes, align 8
};

// sizeof(union Mix) = 8  (size of largest member: double)
// alignof(union Mix) = 8 (alignment of largest member: double)
```

#### Type Punning with Unions (Defined Behavior in C99+)

```c
// C99/C11: reading a union member different from last written is defined behavior
// (as long as the access doesn't violate aliasing rules in other contexts)

union FloatBits {
    float    f;
    uint32_t u;
};

// Examine IEEE 754 float bit pattern:
union FloatBits fb;
fb.f = -1.0f;
printf("bits: 0x%08X\n", fb.u);  // 0xBF800000

// Bit manipulation (e.g., fast inverse square root trick):
float fast_inv_sqrt(float number) {
    union FloatBits fb;
    fb.f = number;
    fb.u = 0x5f3759df - (fb.u >> 1);
    fb.f *= 1.5f - (number * 0.5f * fb.f * fb.f);
    return fb.f;
}
```

#### Packed Tagged Union in C (Manual Discriminated Union)

```c
// C has no built-in tagged union, so you build one manually:
enum ValueTag { TAG_INT, TAG_FLOAT, TAG_STRING };

struct Value {
    enum ValueTag tag;
    union {
        int    i;
        double d;
        char  *s;
    } data;
};

struct Value v;
v.tag    = TAG_INT;
v.data.i = 42;

// Reading requires checking tag first:
void print_value(const struct Value *v) {
    switch (v->tag) {
        case TAG_INT:    printf("%d\n",   v->data.i); break;
        case TAG_FLOAT:  printf("%f\n",   v->data.d); break;
        case TAG_STRING: printf("%s\n",   v->data.s); break;
    }
}
```

#### Anonymous Unions

```c
struct Config {
    int type;
    union {          // anonymous union — members accessed directly
        int   int_val;
        float float_val;
        char *str_val;
    };               // no field name
};

struct Config cfg = { .type = 0, .int_val = 42 };
printf("%d\n", cfg.int_val);  // no .data prefix needed
```

### 5.2 Rust Unions

Rust unions require `unsafe` to read because the compiler cannot verify which variant is active.

#### Basic Rust Union

```rust
#[repr(C)]
union MyUnion {
    int_val:   i32,
    float_val: f32,
}

let u = MyUnion { int_val: 42 };
let v: i32 = unsafe { u.int_val };    // safe access — wrote i32
let w: f32 = unsafe { u.float_val };  // UB! read different variant
```

#### Union with Copy Types

```rust
// All fields in a Rust union must implement Copy, OR
// the union must not implement Drop, OR ManuallyDrop is used

use std::mem::ManuallyDrop;

union StringOrInt {
    s: ManuallyDrop<String>,   // non-Copy type — need ManuallyDrop
    n: i64,
}

let mut u = StringOrInt { n: 0 };
unsafe {
    u.s = ManuallyDrop::new(String::from("hello"));
    println!("{}", &*u.s);
    ManuallyDrop::drop(&mut u.s);  // must drop manually!
}
```

#### repr(C) Union for FFI

```rust
// Matching a C union exactly for FFI
#[repr(C)]
union CValue {
    as_int:   i32,
    as_float: f32,
    as_bytes: [u8; 4],
}

// Reading bytes of a float:
let f: f32 = std::f32::consts::PI;
let u = CValue { as_float: f };
let bytes: [u8; 4] = unsafe { u.as_bytes };
println!("{:?}", bytes);  // [219, 15, 73, 64] (little-endian)
```

#### Pattern Matching on Unions (Limited)

```rust
// Rust doesn't support pattern matching on union variants directly
// You must maintain your own discriminant
struct TaggedUnion {
    tag: u8,
    data: RawUnion,
}

union RawUnion {
    as_u32: u32,
    as_f32: f32,
}

impl TaggedUnion {
    fn read_u32(&self) -> Option<u32> {
        if self.tag == 0 {
            Some(unsafe { self.data.as_u32 })
        } else {
            None
        }
    }
}
```

### 5.3 Go Unions (Workarounds)

Go has no native union. The idiomatic approaches:

#### interface{} / any

```go
type Value any  // alias for interface{}

// Store any type:
var v Value = 42
v = "hello"
v = 3.14

// Type assertion to extract:
if n, ok := v.(int); ok {
    fmt.Println("int:", n)
} else if s, ok := v.(string); ok {
    fmt.Println("string:", s)
}
```

#### Typed Union via Interface

```go
// Restrict to specific types using sealed interface
type IntOrString interface {
    intOrString()  // unexported method — only types in this package implement it
}

type IntVal struct { V int }
type StrVal struct { V string }

func (IntVal) intOrString() {}
func (StrVal) intOrString() {}

func process(v IntOrString) {
    switch x := v.(type) {
    case IntVal: fmt.Println("int:", x.V)
    case StrVal: fmt.Println("str:", x.V)
    }
}
```

#### unsafe Union in Go

```go
import "unsafe"

// Reinterpret float64 bits as uint64 (like C union)
func floatBits(f float64) uint64 {
    return *(*uint64)(unsafe.Pointer(&f))
}

// WARNING: breaks Go's type system — use only when necessary
```

### 5.4 Linux Kernel Union Usage

The Linux kernel uses unions pervasively for:

#### Overlapping Interpretations in sk_buff

```c
// From include/linux/skbuff.h
struct sk_buff {
    union {
        __be16      inner_protocol;
        __u8        inner_ipproto;
    };

    union {
        struct {
            unsigned long  _skb_refdst;
            void          (*destructor)(struct sk_buff *skb);
        };
        struct list_head   tcp_tsorted_anchor;
    };

    union {
        __u32   mark;
        __u32   reserved_tailroom;
    };

    union {
        __be16  inner_transport_header;
        __be16  inner_network_header;
    };
};
```

#### IP Header Overlapping Fields

```c
// include/uapi/linux/ip.h
struct iphdr {
    __u8    ihl:4;        // IHL (header length in 32-bit words)
    __u8    version:4;    // IP version
    __u8    tos;          // type of service
    __be16  tot_len;      // total length
    __be16  id;           // identification
    __be16  frag_off;     // fragment offset + flags
    __u8    ttl;          // time to live
    __u8    protocol;     // protocol (TCP=6, UDP=17, ICMP=1)
    __sum16 check;        // header checksum
    __be32  saddr;        // source address
    __be32  daddr;        // destination address
};

// For IP options, the option field reinterprets the trailer:
union {
    struct iphdr   iph;
    u8             raw[60];  // max IP header = 60 bytes
} ip_buf;
```

#### NLATTR (Netlink Attribute) Union

```c
// Netlink uses a type-length-value (TLV) scheme
struct nlattr {
    __u16   nla_len;     // total attribute length (including header)
    __u16   nla_type;    // attribute type
    // payload follows immediately
};

// To access payload as different types:
void *nla_data(const struct nlattr *nla) {
    return (char *)nla + NLA_HDRLEN;
}

// Kernel helper macros:
#define nla_get_u32(nla)  (*(u32 *)nla_data(nla))
#define nla_get_u64(nla)  get_unaligned((u64 *)nla_data(nla))
```

---

## 6. Tagged Unions / Discriminated Unions

A **tagged union** (also: discriminated union, variant record) combines a union with a discriminant field that indicates which variant is active. This is the safe way to use unions.

```
Tagged Union Structure:
┌──────────────────────────────────────────────┐
│  discriminant / tag                          │  ← tells you which variant is active
├──────────────────────────────────────────────┤
│                                              │
│  union data (size = largest variant)         │  ← payload, interpret via tag
│                                              │
└──────────────────────────────────────────────┘
```

### C Implementation

```c
typedef enum { KIND_NONE, KIND_INT, KIND_FLOAT, KIND_STR } ValueKind;

typedef struct {
    ValueKind kind;
    union {
        int    as_int;
        double as_float;
        struct {
            char  *ptr;
            size_t len;
        } as_str;
    };
} Value;

// Factory functions:
Value value_int(int n)        { return (Value){ .kind = KIND_INT,   .as_int   = n };  }
Value value_float(double d)   { return (Value){ .kind = KIND_FLOAT, .as_float = d };  }
Value value_str(char *p, size_t l) {
    return (Value){ .kind = KIND_STR, .as_str = { p, l } };
}

// Safe accessor:
int value_get_int(const Value *v, int *out) {
    if (v->kind != KIND_INT) return -1;
    *out = v->as_int;
    return 0;
}

// Print:
void value_print(const Value *v) {
    switch (v->kind) {
        case KIND_NONE:  printf("nil\n");                               break;
        case KIND_INT:   printf("%d\n",   v->as_int);                  break;
        case KIND_FLOAT: printf("%g\n",   v->as_float);                break;
        case KIND_STR:   printf("%.*s\n", (int)v->as_str.len,
                                          v->as_str.ptr);              break;
    }
}
```

### Rust Implementation (Enum IS a tagged union)

```rust
// Rust's enum is a tagged union with full compiler support:
enum Value {
    None,
    Int(i64),
    Float(f64),
    Str(String),
}

// The compiler:
// 1. Assigns a discriminant to each variant
// 2. Lays out the union of all variant data
// 3. Enforces exhaustive matching
// 4. Prevents reading wrong variant (no unsafe needed)

impl Value {
    fn print(&self) {
        match self {
            Value::None          => println!("nil"),
            Value::Int(n)        => println!("{}", n),
            Value::Float(f)      => println!("{}", f),
            Value::Str(s)        => println!("{}", s),
        }
    }
}
```

### Go Implementation

```go
type ValueKind int

const ( KindNone ValueKind = iota; KindInt; KindFloat; KindStr )

type Value struct {
    kind     ValueKind
    intVal   int64
    floatVal float64
    strVal   string
    // Note: all fields present, only one is "active" by convention
    // This wastes memory compared to a real union
}
```

### Memory Comparison: C, Rust, Go

```
C tagged union (Value):
┌─────────────────────────────────────────────────┐
│  kind (4)  │  padding (4)  │  largest union (16) │  = 24 bytes
└─────────────────────────────────────────────────┘

Rust enum (Value):
┌────────────────────────────────────────────────────────┐
│  tag (1+padding)  │  String internals (24 bytes)       │  = 32 bytes
│                   │  (ptr=8, len=8, cap=8)             │
└────────────────────────────────────────────────────────┘
// Note: Rust also lays out as a tagged union internally

Go struct (emulated union):
┌───────────────────────────────────────────────────────────────────────┐
│  kind (8)  │  intVal (8)  │  floatVal (8)  │  strVal (24: ptr+len+cap)│  = 48 bytes
└───────────────────────────────────────────────────────────────────────┘
// All fields always present — wastes memory
```

---

## 7. Memory Layout Deep Dive

### 7.1 Computing Struct Layout by Hand

```
ALGORITHM for struct layout:
1. Start at offset 0.
2. For each field:
   a. Round current offset UP to field's alignment.
   b. Place field at that offset.
   c. Advance offset by field's size.
3. Round final offset UP to struct's alignment (= largest field alignment).

Example:
struct Example {
    char  a;     // size=1, align=1
    int   b;     // size=4, align=4
    char  c;     // size=1, align=1
    short d;     // size=2, align=2
    long  e;     // size=8, align=8
};

Step-by-step:
offset=0: place a (size=1, align=1) → [0]       ; offset becomes 1
offset=1: align to 4 → offset=4; place b (size=4) → [4..7]  ; offset becomes 8
offset=8: place c (size=1, align=1) → [8]        ; offset becomes 9
offset=9: align to 2 → offset=10; place d (size=2) → [10..11] ; offset becomes 12
offset=12: align to 8 → offset=16; place e (size=8) → [16..23] ; offset becomes 24
struct align = 8 (largest); 24 is already multiple of 8.
sizeof(Example) = 24

Memory map:
 0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18 19 20 21 22 23
[a][P  P  P][     b     ][c][P][  d  ][P  P  P  P][         e          ]
 P = padding
```

### 7.2 Optimizing Struct Layout

Sort fields **largest to smallest** to minimize padding:

```c
// BAD — 24 bytes due to padding:
struct Bad {
    char   a;   // 1 byte
    int    b;   // 4 bytes  (3 bytes padding before)
    char   c;   // 1 byte
    double d;   // 8 bytes  (7 bytes padding before)
};
// layout: a[0] pad[1-3] b[4-7] c[8] pad[9-15] d[16-23] = 24 bytes

// GOOD — 16 bytes, no internal padding:
struct Good {
    double d;   // 8 bytes @ 0
    int    b;   // 4 bytes @ 8
    char   a;   // 1 byte  @ 12
    char   c;   // 1 byte  @ 13
    // 2 bytes tail padding
};
// Total: 16 bytes
```

### 7.3 Rust Field Reordering

By default, Rust may **reorder struct fields** for optimal layout:

```rust
struct Rust {
    a: u8,    // 1 byte
    b: u32,   // 4 bytes
    c: u8,    // 1 byte
}
// Rust may reorder to: b(4), a(1), c(1), pad(2) = 8 bytes
// Or:                  b(4), a(1), c(1)          = 6 bytes + 2 tail pad = 8 bytes
// vs C equivalent:     a(1), pad(3), b(4), c(1), pad(3) = 12 bytes!

// Use #[repr(C)] to prevent reordering:
#[repr(C)]
struct ExactC {
    a: u8,
    b: u32,
    c: u8,
}
// Now identical to C: 12 bytes
```

### 7.4 Union Size and Alignment

```
Union size  = size of LARGEST member (rounded up to union's alignment)
Union align = alignment of MOST STRICTLY ALIGNED member

union Example {
    char   c;    // size=1,  align=1
    int    i;    // size=4,  align=4
    double d;    // size=8,  align=8
};
// size  = 8  (= sizeof double, rounded to alignof double)
// align = 8  (= alignof double)

All members overlay the same bytes:
┌───┬───┬───┬───┬───┬───┬───┬───┐
│ 0 │ 1 │ 2 │ 3 │ 4 │ 5 │ 6 │ 7 │
│◄─────────── d ──────────────►  │
│◄─── i ───►                     │
│ c │                             │
└───┴───┴───┴───┴───┴───┴───┴───┘
```

### 7.5 offsetof and Pointer Arithmetic

```c
#include <stddef.h>

struct S { int x; char y; double z; };
size_t off_x = offsetof(struct S, x);   // 0
size_t off_y = offsetof(struct S, y);   // 4
size_t off_z = offsetof(struct S, z);   // 8

// Given a pointer to a field, find the containing struct:
struct S *s = ...;
char *y_ptr = &s->y;
struct S *back = (struct S *)((char *)y_ptr - offsetof(struct S, y));
// This is exactly what container_of does in the Linux kernel
```

---

## 8. Bit Fields

Bit fields pack multiple small values into integer storage.

### 8.1 C Bit Fields

```c
struct IpFlags {
    unsigned int reserved : 1;   // bit 0
    unsigned int df       : 1;   // bit 1 — don't fragment
    unsigned int mf       : 1;   // bit 2 — more fragments
    unsigned int offset   : 13;  // bits 3..15 — fragment offset
};

// IP header fragment field:
struct FragField {
    __be16  frag_off;   // in network code, often stored as __be16
    // bits: [0]=reserved [1]=DF [2]=MF [3..15]=fragment offset
};

// TCP flags:
struct TcpFlags {
    unsigned int cwr : 1;
    unsigned int ece : 1;
    unsigned int urg : 1;
    unsigned int ack : 1;
    unsigned int psh : 1;
    unsigned int rst : 1;
    unsigned int syn : 1;
    unsigned int fin : 1;
};
```

#### Important Bit Field Caveats

```
1. BIT ORDER: The compiler decides which bit is 'first'. 
   On x86 with GCC, bit fields fill from LSB to MSB.
   This is IMPLEMENTATION DEFINED in the C standard.

2. CROSSING STORAGE UNITS: If a bit field can't fit in the current
   storage unit, it either straddles it or starts a new one (impl-defined).

3. NO ADDRESS-OF: You cannot take the address of a bit field.

4. NETWORK PROTOCOLS: Never use bit fields for network headers — 
   endianness and bit ordering are not portable.
   The Linux kernel defines its own byte-order-safe macros instead.

struct Unsafe {
    unsigned int a : 4;   // bits 0..3
    unsigned int b : 4;   // bits 4..7
    // On little-endian x86: a is lower bits
    // On big-endian MIPS:   a might be higher bits
    // Portable network code: use explicit masks instead
};

// Portable network bit manipulation:
// DON'T:  header->flags_frag & DF_BIT   (if using bit fields)
// DO:     ntohs(header->frag_off) & IP_DF  (explicit mask)
#define IP_DF 0x4000  // Don't fragment
#define IP_MF 0x2000  // More fragments
```

### 8.2 Rust Bit Fields (via crates)

Rust has no native bit fields, but crates like `bitflags` and `modular-bitfield` provide them:

```rust
// bitflags crate — for flag sets
use bitflags::bitflags;

bitflags! {
    #[derive(Debug, Clone, Copy)]
    struct TcpFlags: u8 {
        const FIN = 0b0000_0001;
        const SYN = 0b0000_0010;
        const RST = 0b0000_0100;
        const PSH = 0b0000_1000;
        const ACK = 0b0001_0000;
        const URG = 0b0010_0000;
        const ECE = 0b0100_0000;
        const CWR = 0b1000_0000;
    }
}

let flags = TcpFlags::SYN | TcpFlags::ACK;
assert!(flags.contains(TcpFlags::SYN));
assert!(!flags.contains(TcpFlags::FIN));

// modular-bitfield crate — struct-like bit field
use modular_bitfield::prelude::*;

#[bitfield]
pub struct IpHeader {
    pub version:  B4,
    pub ihl:      B4,
    pub dscp:     B6,
    pub ecn:      B2,
    pub tot_len:  B16,
    // ...
}
```

### 8.3 Linux Kernel Bit Fields in Structs

```c
// kobject state flags use bit fields:
struct kobject {
    // ...
    unsigned int state_initialized:1;
    unsigned int state_in_sysfs:1;
    unsigned int state_add_uevent_sent:1;
    unsigned int state_remove_uevent_sent:1;
    unsigned int uevent_suppress:1;
};

// sk_buff uses bit fields extensively:
struct sk_buff {
    // ...
    __u8  pkt_type:3;      // PACKET_HOST, BROADCAST, etc.
    __u8  pfmemalloc:1;
    __u8  ignore_df:1;
    __u8  ip_summed:2;
    __u8  ooo_okay:1;
    __u8  l4_hash:1;
    __u8  sw_hash:1;
    // ...
};
```

---

## 9. ABI, Packing, and `repr` in Rust

### 9.1 ABI (Application Binary Interface)

The ABI defines how structs are passed at the machine code level:
- Field offsets
- Calling conventions (how function arguments are passed: registers vs stack)
- Name mangling

```
x86-64 Linux System V ABI (for struct passing):
- Structs ≤ 16 bytes: passed in registers (rdi, rsi, rdx, rcx, r8, r9)
- Structs > 16 bytes: passed on the stack by pointer
- Returned:
  - ≤ 8 bytes → rax
  - 9-16 bytes → rax:rdx
  - > 16 bytes → hidden pointer in rdi, caller allocates space

struct Small { int a; int b; };  // 8 bytes → passed in rdi
struct Large { int arr[10]; };   // 40 bytes → passed by pointer
```

### 9.2 Rust #[repr(...)] in Detail

```rust
// 1. Default (repr Rust) — compiler may reorder, optimize layout
struct Default {
    a: u8,
    b: u64,
    c: u16,
}
// Likely layout: b(8), c(2), a(1), pad(5) = 16 bytes — compiler decides

// 2. repr(C) — C-compatible, field order preserved
#[repr(C)]
struct CCompat {
    a: u8,    // offset 0
    // pad 3
    b: u32,   // offset 4
    c: u16,   // offset 8
    // pad 2
    // total: 12
}

// 3. repr(packed) — no padding, potentially unaligned
#[repr(packed)]
struct Packed {
    a: u8,   // offset 0
    b: u32,  // offset 1 — UNALIGNED on most architectures
}
// sizeof = 5

// Accessing packed fields requires unsafe (potential unaligned access)
let p = Packed { a: 1, b: 42 };
let b_val = unsafe { std::ptr::read_unaligned(&p.b) };

// 4. repr(align(N)) — force minimum alignment
#[repr(align(64))]  // cache-line aligned
struct CacheAligned {
    data: [u8; 64],
}

// 5. repr(transparent) — same layout as single non-ZST field
#[repr(transparent)]
struct Wrapper(u32);
// sizeof(Wrapper) == sizeof(u32) == 4
// Can safely transmute between Wrapper and u32
// Used for newtype FFI patterns

// 6. repr(u8/u16/u32/u64) — for enums: control discriminant size
#[repr(u8)]
enum SmallEnum {
    A = 0,
    B = 1,
    C = 255,
}
// sizeof(SmallEnum) == 1

#[repr(C, u8)]  // C-layout enum with u8 discriminant
enum CEnum {
    Integer(u32),
    Float(f32),
}
```

### 9.3 Enum Memory Layout Optimizations

```rust
// Null pointer optimization (NPO):
// Option<&T>, Option<Box<T>>, Option<NonNull<T>> all have same
// size as the inner type — None is represented as null pointer

use std::num::NonZeroU32;
assert_eq!(
    std::mem::size_of::<Option<NonZeroU32>>(),
    std::mem::size_of::<u32>()  // 4 bytes, not 8!
);
// None  → 0x00000000
// Some(n) → n (always nonzero)

// General enum layout:
enum Foo {
    A(u32),       // variant A: tag=0, data=u32
    B(u64),       // variant B: tag=1, data=u64
    C,            // variant C: tag=2, no data
}
// Rust chooses smallest discriminant that fits all variants
// Likely: tag=u8 (3 values fit in 1 byte), data=u64 (largest)
// Layout: 1 byte tag + 7 bytes pad + 8 bytes data = 16 bytes
// OR with niche optimization: depends on variants

// Check actual sizes:
println!("{}", std::mem::size_of::<Foo>());     // likely 16
println!("{}", std::mem::size_of::<Option<Foo>>());  // 16 (niche if possible)
```

---

## 10. Pattern Matching

### 10.1 Rust Match — The Full Power

```rust
#[derive(Debug)]
enum Packet {
    Tcp { src_port: u16, dst_port: u16, flags: u8, payload: Vec<u8> },
    Udp { src_port: u16, dst_port: u16, payload: Vec<u8> },
    Icmp { type_: u8, code: u8 },
    Unknown(u8),
}

fn handle(pkt: &Packet) {
    match pkt {
        // Struct pattern with field binding
        Packet::Tcp { dst_port: 80 | 443, flags, payload, .. } => {
            println!("HTTP(S) flags={flags}, len={}", payload.len());
        }

        // Guard clause
        Packet::Tcp { src_port, .. } if *src_port < 1024 => {
            println!("TCP from privileged port {src_port}");
        }

        // Binding with @
        Packet::Tcp { dst_port: p @ 8000..=8999, .. } => {
            println!("TCP to dev port {p}");
        }

        // Simple struct pattern
        Packet::Udp { src_port, dst_port, payload } => {
            println!("UDP {src_port} → {dst_port}, {}", payload.len());
        }

        // Multiple variants
        Packet::Icmp { type_: 8, code: 0 } => println!("Ping request"),
        Packet::Icmp { type_: 0, code: 0 } => println!("Ping reply"),
        Packet::Icmp { type_, code }        => println!("ICMP {type_}/{code}"),

        // Wildcard
        Packet::Unknown(proto) => println!("Unknown proto {proto}"),

        // Remaining TCP patterns
        Packet::Tcp { .. } => println!("Other TCP"),
    }
}

// Nested patterns
fn deep_match(opt: Option<Result<i32, &str>>) -> &'static str {
    match opt {
        None                   => "nothing",
        Some(Ok(n)) if n > 0   => "positive",
        Some(Ok(_))            => "non-positive",
        Some(Err(e))           => "error",
    }
}

// Destructuring in let
let Point { x, y } = Point { x: 1, y: 2 };
let (a, b, c) = (1, 2, 3);
let [first, .., last] = [1, 2, 3, 4, 5];
```

### 10.2 C Switch (Limited Pattern Matching)

```c
// C switch only on integer types — no structural decomposition
enum State { IDLE, RUNNING, STOPPED };

void dispatch(enum State s, int priority) {
    switch (s) {
        case IDLE:
            if (priority > 5) { /* nested condition */ }
            break;
        case RUNNING:
        case STOPPED:   // fall-through — both cases same code
            cleanup();
            break;
        default:
            abort();
    }
}

// For tagged unions in C, you manually decompose:
void handle_value(const Value *v) {
    switch (v->kind) {
        case KIND_INT:   use_int(v->as_int);   break;
        case KIND_FLOAT: use_float(v->as_float); break;
        case KIND_STR:   use_str(v->as_str.ptr, v->as_str.len); break;
        default:         /* unhandled — no compiler warning unless -Wswitch-enum */
    }
}
```

### 10.3 Go Type Switch

```go
func process(v interface{}) {
    switch x := v.(type) {
    case int:
        fmt.Printf("int: %d\n", x)
    case string:
        fmt.Printf("string: %q\n", x)
    case []byte:
        fmt.Printf("bytes: %d\n", len(x))
    case nil:
        fmt.Println("nil")
    default:
        fmt.Printf("unknown: %T\n", x)
    }
}

// Multiple types in one case (Go 1.18+):
switch x := v.(type) {
case int, int64:
    fmt.Printf("integer\n")
}
```

---

## 11. Performance Implications

### 11.1 Struct Size and Cache Efficiency

```
Cache line = 64 bytes on x86-64 / ARM64

COLD PATH — struct larger than cache line:
struct Fat {
    int     hot_a;      // accessed 10000x/sec
    char    cold[120];  // accessed 1x/sec
    int     hot_b;      // accessed 10000x/sec
};
// hot_a and hot_b are separated by 120 bytes → different cache lines
// Every access to hot_a brings in cold[] → cache pollution

HOT/COLD SPLIT:
struct HotPart {
    int hot_a;
    int hot_b;
};
struct ColdPart {
    char cold[120];
};
struct Efficient {
    struct HotPart hot;
    struct ColdPart *cold;  // pointer — only loaded when needed
};
```

### 11.2 False Sharing (Multi-Core)

```
Two threads, each writing to different fields in the same struct,
but fields share a cache line → cache line bounces between cores

struct Counter {
    int count_a;   // thread A writes this
    int count_b;   // thread B writes this
};
// Both at offsets 0 and 4 — same 64-byte cache line
// Thread A write invalidates thread B's cache line — SLOW

FIX: pad to separate cache lines
struct Counter {
    int count_a;
    char _pad[60];   // pad to 64 bytes
    int count_b;
};

In Linux kernel:
struct __cacheline_aligned_in_smp Counter {
    int count_a ____cacheline_aligned;
    int count_b ____cacheline_aligned;
};

#define ____cacheline_aligned  __attribute__((__aligned__(SMP_CACHE_BYTES)))
```

### 11.3 Enum Dispatch vs vtable

```rust
// Enum dispatch — monomorphic, no indirection
enum Animal { Cat, Dog, Fish }
impl Animal {
    fn speak(&self) -> &str {
        match self {    // compiled to a jump table or conditionals
            Animal::Cat  => "meow",
            Animal::Dog  => "woof",
            Animal::Fish => "...",
        }
    }
}

// Trait object (vtable) — dynamic dispatch, pointer indirection
trait Animal { fn speak(&self) -> &str; }
struct Cat; impl Animal for Cat { fn speak(&self) -> &str { "meow" } }
struct Dog; impl Animal for Dog { fn speak(&self) -> &str { "woof" } }

let a: &dyn Animal = &Cat;
a.speak();  // dereferences vtable pointer — one extra indirection

// Enum dispatch: known at compile time, inlinable
// vtable dispatch: known at runtime, cannot inline
//
// Use enum when variants are known and finite.
// Use trait objects for open-ended extensibility.
```

### 11.4 Union vs Struct for Overlapping Data

```c
// If you need to interpret the same data multiple ways:

// APPROACH 1: struct with conversion functions — two copies in memory
struct ConvertibleVal {
    int    int_view;
    float  float_view;  // always kept in sync — wasteful
};

// APPROACH 2: union — single copy, reinterpret in-place
union Val {
    int   as_int;
    float as_float;
};
// sizeof = 4 (not 8)
// No conversion needed — just access the right member
```

### 11.5 Rust Enum Size and Performance Tips

```rust
// Avoid large variants bloating enum size
enum Bad {
    Small(u8),
    Large([u8; 1024]),  // entire enum is 1025 bytes!
}

// Fix: box the large variant
enum Good {
    Small(u8),
    Large(Box<[u8; 1024]>),  // enum is now 9 bytes (tag + ptr)
}

// Check with:
use std::mem::size_of;
println!("{}", size_of::<Bad>());   // 1025
println!("{}", size_of::<Good>());  // 16 (on 64-bit)
```

---

## 12. Linux Kernel Case Studies

### 12.1 container_of Macro — Reverse Lookup

```c
// One of the most important macros in the Linux kernel:
#define container_of(ptr, type, member) ({                        \
    void *__mptr = (void *)(ptr);                                 \
    BUILD_BUG_ON_MSG(!__same_type(*(ptr), ((type *)0)->member) && \
                     !__same_type(*(ptr), void),                  \
                     "pointer type mismatch in container_of()");  \
    ((type *)(__mptr - offsetof(type, member))); })

// Example usage:
struct Timer {
    int         id;
    struct list_head list;   // embedded at some offset
    unsigned long    expires;
};

// When you have a pointer to the list member, get back to Timer:
void timer_callback(struct list_head *entry) {
    struct Timer *t = container_of(entry, struct Timer, list);
    // Now t points to the full Timer struct
    printf("Timer %d expired\n", t->id);
}

/*
 Memory diagram:
 Timer struct starts at address X:
 ┌────────────────────────────────────────┐
 │ id      @ X + 0                        │
 │ list    @ X + 8   ← we have this ptr   │
 │   .prev @ X + 8                        │
 │   .next @ X + 16                       │
 │ expires @ X + 24                       │
 └────────────────────────────────────────┘

 container_of(list_ptr, Timer, list) = list_ptr - offsetof(Timer, list)
                                     = list_ptr - 8
                                     = X  ← start of Timer
*/
```

### 12.2 RCU (Read-Copy-Update) and Struct Design

```c
// RCU requires careful struct design: readers hold no locks,
// writers copy, modify, then publish atomically.

struct Config {
    struct rcu_head rcu;    // must be embedded for call_rcu()
    int             value;
    char            name[32];
};

// Reader:
rcu_read_lock();
struct Config *cfg = rcu_dereference(global_cfg);  // read barrier
use(cfg->value);   // cfg is valid for the duration of RCU read lock
rcu_read_unlock();

// Writer:
struct Config *old_cfg = global_cfg;
struct Config *new_cfg = kmalloc(sizeof(*new_cfg), GFP_KERNEL);
*new_cfg = *old_cfg;           // copy
new_cfg->value = new_value;    // modify copy
rcu_assign_pointer(global_cfg, new_cfg);  // publish (write barrier)
call_rcu(&old_cfg->rcu, free_config);     // free after grace period

// rcu_head is a struct embedded in the object:
struct rcu_head {
    struct rcu_head *next;
    void (*func)(struct rcu_head *head);
};
```

### 12.3 Per-CPU Variables and Struct Padding

```c
// DEFINE_PER_CPU creates one copy of the variable per CPU core
// to avoid cache line contention:

struct pcpu_stats {
    unsigned long tx_packets;
    unsigned long tx_bytes;
    unsigned long rx_packets;
    unsigned long rx_bytes;
} __percpu;

DEFINE_PER_CPU(struct pcpu_stats, net_stats);

// On CPU 2:
this_cpu_inc(net_stats.tx_packets);

// Read aggregate:
unsigned long total_tx = 0;
for_each_possible_cpu(cpu)
    total_tx += per_cpu(net_stats.tx_packets, cpu);

// Each per-CPU copy is cache-line padded to prevent false sharing
```

### 12.4 The wait_queue — Struct Embedding for Sleeping

```c
struct wait_queue_entry {
    unsigned int          flags;
    void                 *private;  // typically task_struct *
    wait_queue_func_t     func;
    struct list_head      entry;
};

struct wait_queue_head {
    spinlock_t            lock;
    struct list_head      head;
};

// Usage in a driver waiting for data:
DECLARE_WAIT_QUEUE_HEAD(my_wait_queue);

// Thread waiting:
wait_event_interruptible(my_wait_queue, condition_is_true());
// This:
// 1. Creates a wait_queue_entry on the stack
// 2. Adds it to my_wait_queue.head
// 3. Sets task state to TASK_INTERRUPTIBLE
// 4. Calls schedule() — gives up CPU
// 5. When woken, re-checks condition

// Thread waking:
wake_up(&my_wait_queue);
// Walks the list_head, calling each entry's func,
// which sets the sleeping task's state to TASK_RUNNING
```

---

## 13. Network Subsystem Case Studies

### 13.1 Packet Journey: struct relationships

```
                    PACKET RECEIVE PATH
═══════════════════════════════════════════════════════════════════════

  NIC Hardware
    │  DMA → ring buffer (struct e1000_rx_ring / struct igb_rx_buffer)
    │
    ▼
  net_device->netdev_ops->ndo_start_xmit (TX) / NAPI poll (RX)
    │  alloc_skb() → sk_buff created
    │  skb->dev = net_device*
    │  skb_put() → fill data region
    │
    ▼
  netif_receive_skb(skb)
    │  ptype_all handlers called (packet sockets, tcpdump)
    │  eth_type_trans() → skb->protocol = ETH_P_IP (0x0800)
    │  skb->mac_header set
    │
    ▼
  ip_rcv(skb, dev, pt, orig_dev)         ← net/ipv4/ip_input.c
    │  struct iphdr *iph = ip_hdr(skb)
    │    ip_hdr(skb) = (struct iphdr *)(skb->head + skb->network_header)
    │  Verify checksum, TTL, length
    │  skb->network_header set
    │  NF_HOOK(NFPROTO_IPV4, NF_INET_PRE_ROUTING, ...)
    │
    ▼
  ip_rcv_finish(net, sk, skb)
    │  ip_route_input_noref() → rt_dst = skb_dst(skb)  (routing lookup)
    │    struct rtable { struct dst_entry dst; ... }
    │    dst_entry has .input function pointer
    │
    ▼
  dst->input(skb) → ip_local_deliver(skb) or ip_forward(skb)
    │
    │  ip_local_deliver:
    │    NF_HOOK(NF_INET_LOCAL_IN, ...)
    │    ip_local_deliver_finish()
    │      inet_protos[iph->protocol]->handler(skb)
    │        → tcp_v4_rcv(skb)    for TCP (protocol=6)
    │        → udp_rcv(skb)       for UDP (protocol=17)
    │
    ▼
  tcp_v4_rcv(skb)                         ← net/ipv4/tcp_ipv4.c
    │  struct tcphdr *th = tcp_hdr(skb)
    │  sk = __inet_lookup_skb(...)        ← hash table lookup for socket
    │    → struct sock* (actually tcp_sock*)
    │  tcp_v4_do_rcv(sk, skb)
    │    tcp_rcv_established(sk, skb, th)
    │      or tcp_rcv_state_process(sk, skb)
    │
    ▼
  sock_queue_rcv_skb(sk, skb)
    │  Enqueue to sk->sk_receive_queue (sk_buff_head)
    │  sk->sk_data_ready(sk)  ← wake up waiting process
    │
    ▼
  Process: recv(fd, buf, len, 0)
    │  tcp_recvmsg() → copy from sk_receive_queue to user buf
    └─ skb_copy_datagram_iter() → copy sk_buff data to iovec


KEY STRUCTS AND THEIR RELATIONSHIPS:

  net_device ←── skb->dev
      │
      └── netdev_ops* ──► function table

  sk_buff
      ├── sock *sk ──────────────────────────────► sock
      │                                             ├── inet_sock
      │                                             │    └── inet_connection_sock
      │                                             │         └── tcp_sock
      │                                             └── sk_receive_queue (sk_buff_head)
      │
      ├── dst_entry *_skb_refdst ────────────────► rtable / rt6_info
      │                                             └── dst_entry
      │                                                  └── input/output fn ptrs
      │
      └── [head][data .... tail][end]              ← raw packet bytes
```

### 13.2 Netfilter Hook System — Struct-Based Plugin Architecture

```c
// Registering a netfilter hook:
struct nf_hook_ops {
    nf_hookfn          *hook;         // callback function pointer
    struct net_device  *dev;          // device filter (or NULL)
    void               *priv;         // private data for hook
    u_int8_t            pf;           // protocol family (NFPROTO_IPV4)
    unsigned int        hooknum;      // which hook point (NF_INET_PRE_ROUTING, etc.)
    int                 priority;     // order among hooks at same point
};

// Hook callback signature:
typedef unsigned int nf_hookfn(void *priv,
                                struct sk_buff *skb,
                                const struct nf_hook_state *state);

// Return values:
// NF_ACCEPT  (1) — continue processing
// NF_DROP    (0) — drop packet
// NF_STOLEN  (2) — hook took ownership of skb
// NF_QUEUE   (3) — queue for userspace
// NF_REPEAT  (4) — call hook again

// Example — a simple firewall rule struct:
struct fw_rule {
    __be32             src_ip;
    __be32             src_mask;
    __be32             dst_ip;
    __be32             dst_mask;
    __be16             dst_port;
    __u8               protocol;
    enum fw_action     action;         // ACCEPT or DROP
    struct list_head   list;           // intrusive list
};

static unsigned int my_hook(void *priv, struct sk_buff *skb,
                             const struct nf_hook_state *state)
{
    struct iphdr *iph = ip_hdr(skb);
    struct fw_rule *rule;
    list_for_each_entry(rule, &fw_rules, list) {
        if ((iph->saddr & rule->src_mask) == rule->src_ip &&
            iph->protocol == rule->protocol) {
            return (rule->action == FW_ACCEPT) ? NF_ACCEPT : NF_DROP;
        }
    }
    return NF_ACCEPT;
}
```

### 13.3 Socket Buffer Pool — Slab Allocator with Structs

```c
/*
 sk_buff allocation is critical-path — uses slab cache for speed.

 Slab cache hierarchy for sk_buff:
 ┌─────────────────────────────────────────────────────┐
 │  skbuff_head_cache  — for struct sk_buff only       │
 │  skbuff_fclone_cache — for sk_buff + clone pair     │
 │                                                     │
 │  Each sk_buff points to a separate data buffer:     │
 │  - skb_shared_info sits at the END of the data buf  │
 │  - Contains frag list, GSO info, reference count    │
 └─────────────────────────────────────────────────────┘

 struct skb_shared_info {
     __u8            flags;
     __u8            meta_len;
     __u8            nr_frags;        // number of page fragments
     __u8            tx_flags;
     unsigned short  gso_size;        // GSO segment size
     unsigned short  gso_segs;        // number of GSO segments
     struct sk_buff *frag_list;       // linked list of fragments
     struct skb_shared_hwtstamps hwtstamps;
     atomic_t        dataref;         // reference count for data
     skb_frag_t      frags[MAX_SKB_FRAGS];  // page fragments
 };

 // Accessed via:
 #define skb_shinfo(SKB) ((struct skb_shared_info *)(skb_end_pointer(SKB)))
 // skb_end_pointer(skb) = skb->head + skb->end
*/
```

### 13.4 TCP Control Block — Deep struct hierarchy

```c
/*
 TCP socket struct nesting (each level adds TCP-specific state):

 sock (generic)
 ├── sk_common (hash table linkage, port, IP, family)
 ├── sk_rcvbuf, sk_sndbuf
 ├── sk_receive_queue (incoming data)
 ├── sk_write_queue   (outgoing data)
 └── sk_prot → tcp_prot (protocol operations vtable)

 inet_sock (IP-specific)
 ├── sock sk              ← base
 ├── inet_saddr, inet_daddr
 ├── inet_sport, inet_dport
 └── inet_id (IP ID counter)

 inet_connection_sock (connection-oriented)
 ├── inet_sock inet       ← base
 ├── icsk_accept_queue    ← SYN backlog + completed connections
 ├── icsk_bind_hash       ← port binding
 ├── icsk_retransmit_timer
 ├── icsk_delack_timer
 └── icsk_ca_ops → tcp_congestion_ops (congestion control vtable)

 tcp_sock (TCP-specific)
 ├── inet_connection_sock inet_conn ← base
 ├── snd_una, snd_nxt     ← send sequence numbers
 ├── rcv_nxt, rcv_wnd     ← receive window
 ├── snd_cwnd             ← congestion window
 ├── srtt_us              ← smoothed RTT
 ├── rto                  ← retransmission timeout
 ├── out_of_order_queue   ← rb_root of out-of-order segments
 └── tcp_header_len       ← header length including options

 Accessing: from a sock*, cast to tcp_sock* is safe because
 sock is the first member of inet_sock, which is first member
 of inet_connection_sock, which is first member of tcp_sock.
*/

// Type-safe access pattern in kernel:
static inline struct tcp_sock *tcp_sk(const struct sock *sk)
{
    return (struct tcp_sock *)sk;
}

// Congestion control vtable — struct of function pointers:
struct tcp_congestion_ops {
    u32  (*ssthresh)(struct sock *sk);           // get slow start threshold
    void (*cong_avoid)(struct sock *sk, u32 ack, u32 acked);
    void (*set_state)(struct sock *sk, u8 new_state);
    void (*cwnd_event)(struct sock *sk, enum tcp_ca_event ev);
    void (*in_ack_event)(struct sock *sk, u32 flags);
    u32  (*undo_cwnd)(struct sock *sk);
    void (*pkts_acked)(struct sock *sk, const struct ack_sample *sample);
    char         name[TCP_CA_NAME_MAX];
    struct module *owner;
    struct list_head list;
};
// CUBIC, BBR, Reno each fill in this struct differently.
```

---

## 14. Cross-Language Interoperability (FFI)

### 14.1 C to Rust FFI

```rust
// Calling C functions from Rust:

// 1. Declare the C struct in Rust with #[repr(C)]
#[repr(C)]
pub struct CPoint {
    pub x: f64,
    pub y: f64,
}

// 2. Declare C functions
extern "C" {
    fn c_distance(a: *const CPoint, b: *const CPoint) -> f64;
    fn c_create_point(x: f64, y: f64) -> *mut CPoint;
    fn c_free_point(p: *mut CPoint);
}

// 3. Wrap in safe Rust interface
pub fn distance(a: &CPoint, b: &CPoint) -> f64 {
    unsafe { c_distance(a as *const _, b as *const _) }
}

// 4. For enums shared with C:
#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CStatus {
    Ok    = 0,
    Error = 1,
    Timeout = 2,
}

// 5. Opaque C types:
#[repr(C)]
pub struct OpaqueHandle {
    _private: [u8; 0],  // zero-size, non-instantiable
}

extern "C" {
    fn handle_create() -> *mut OpaqueHandle;
    fn handle_destroy(h: *mut OpaqueHandle);
}
```

### 14.2 Rust to C FFI (exposing Rust to C)

```rust
// Make a Rust function callable from C:
#[no_mangle]
pub extern "C" fn rust_add(a: i32, b: i32) -> i32 {
    a + b
}

// Expose a Rust struct to C:
#[repr(C)]
pub struct RustBuffer {
    data: *mut u8,
    len:  usize,
    cap:  usize,
}

#[no_mangle]
pub extern "C" fn rust_buffer_new(cap: usize) -> *mut RustBuffer {
    let mut v = Vec::with_capacity(cap);
    let ptr = v.as_mut_ptr();
    let len = v.len();
    let cap = v.capacity();
    std::mem::forget(v);  // don't drop — C owns it now
    Box::into_raw(Box::new(RustBuffer { data: ptr, len, cap }))
}

#[no_mangle]
pub extern "C" fn rust_buffer_free(buf: *mut RustBuffer) {
    if buf.is_null() { return; }
    unsafe {
        let b = Box::from_raw(buf);    // re-own box
        Vec::from_raw_parts(b.data, b.len, b.cap);  // re-own vec
        // both drop here
    }
}
```

### 14.3 Go CGo — Calling C from Go

```go
// #include <stdint.h>
// #include <stdlib.h>
//
// typedef struct {
//     int32_t x;
//     int32_t y;
// } CPoint;
//
// double distance(CPoint a, CPoint b);
import "C"
import "unsafe"

type Point struct {
    X, Y float64
}

func Distance(a, b Point) float64 {
    ca := C.CPoint{x: C.int32_t(a.X), y: C.int32_t(a.Y)}
    cb := C.CPoint{x: C.int32_t(b.X), y: C.int32_t(b.Y)}
    return float64(C.distance(ca, cb))
}

// Passing Go structs to C via unsafe:
type GoData struct {
    Value int64
}

func passToC(d *GoData) {
    // Must pin Go object so GC doesn't move it:
    // (Go 1.21+ has runtime/pinner for this)
    cptr := (*C.int64_t)(unsafe.Pointer(&d.Value))
    C.some_c_function(cptr)
}
```

### 14.4 Memory Layout Compatibility Matrix

```
Platform: x86-64 Linux

Type          C size  C align  Rust size  Rust align  Go size  Go align
──────────────────────────────────────────────────────────────────────────
bool/u8         1       1        1          1           1        1
i16/u16         2       2        2          2           2        2
i32/u32         4       4        4          4           4        4
i64/u64         8       8        8          8           8        8
f32             4       4        4          4           4        4
f64             8       8        8          8           8        8
*T (pointer)    8       8        8          8           8        8
size_t/usize    8       8        8          8          (int=8)   8

Struct rules:
- C:    fields in declared order, natural alignment, tail padding
- Rust: default reorders for optimal layout; #[repr(C)] matches C
- Go:   fields in declared order, natural alignment, tail padding (same as C)
```

---

## 15. Advanced Patterns and Idioms

### 15.1 Builder Pattern (Rust)

```rust
// When structs have many optional fields, use a builder
struct Request {
    url:     String,
    method:  String,
    headers: Vec<(String, String)>,
    body:    Option<Vec<u8>>,
    timeout: std::time::Duration,
}

struct RequestBuilder {
    url:     String,
    method:  String,
    headers: Vec<(String, String)>,
    body:    Option<Vec<u8>>,
    timeout: std::time::Duration,
}

impl RequestBuilder {
    pub fn new(url: impl Into<String>) -> Self {
        RequestBuilder {
            url:     url.into(),
            method:  String::from("GET"),
            headers: Vec::new(),
            body:    None,
            timeout: std::time::Duration::from_secs(30),
        }
    }
    pub fn method(mut self, m: impl Into<String>) -> Self { self.method = m.into(); self }
    pub fn header(mut self, k: &str, v: &str) -> Self {
        self.headers.push((k.into(), v.into())); self
    }
    pub fn body(mut self, b: Vec<u8>) -> Self { self.body = Some(b); self }
    pub fn timeout(mut self, t: std::time::Duration) -> Self { self.timeout = t; self }
    pub fn build(self) -> Request {
        Request { url: self.url, method: self.method, headers: self.headers,
                  body: self.body, timeout: self.timeout }
    }
}

let req = RequestBuilder::new("https://example.com")
    .method("POST")
    .header("Content-Type", "application/json")
    .body(b"{\"key\":\"value\"}".to_vec())
    .timeout(std::time::Duration::from_secs(10))
    .build();
```

### 15.2 Typestate Pattern (Rust — Compile-Time State Machines)

```rust
// Use phantom type parameters to encode state in the type system
use std::marker::PhantomData;

struct Disconnected;
struct Connected;
struct Authenticated;

struct Connection<State> {
    addr:    String,
    stream:  Option<std::net::TcpStream>,
    _state:  PhantomData<State>,
}

impl Connection<Disconnected> {
    pub fn new(addr: &str) -> Self {
        Connection { addr: addr.to_string(), stream: None, _state: PhantomData }
    }
    pub fn connect(self) -> Result<Connection<Connected>, std::io::Error> {
        let stream = std::net::TcpStream::connect(&self.addr)?;
        Ok(Connection { addr: self.addr, stream: Some(stream), _state: PhantomData })
    }
}

impl Connection<Connected> {
    pub fn authenticate(self, token: &str) -> Result<Connection<Authenticated>, String> {
        // send token...
        Ok(Connection { addr: self.addr, stream: self.stream, _state: PhantomData })
    }
}

impl Connection<Authenticated> {
    pub fn send(&mut self, data: &[u8]) -> Result<(), std::io::Error> {
        // can only send when authenticated
        use std::io::Write;
        self.stream.as_mut().unwrap().write_all(data)
    }
}

// Compile error: cannot call send() on non-authenticated connection
// let mut conn = Connection::new("...").connect().unwrap();
// conn.send(b"data");  // ERROR: no method `send` on Connection<Connected>
```

### 15.3 Enum as Protocol State Machine (Rust + Networking)

```rust
#[derive(Debug)]
enum HttpParseState {
    ReadingRequestLine { buf: Vec<u8> },
    ReadingHeaders { method: String, path: String, headers: Vec<(String, String)> },
    ReadingBody { request: PartialRequest, remaining: usize },
    Complete(HttpRequest),
    Error(ParseError),
}

struct Parser {
    state: HttpParseState,
}

impl Parser {
    pub fn feed(&mut self, data: &[u8]) -> Option<HttpRequest> {
        // Transition state based on input
        self.state = match std::mem::replace(&mut self.state, HttpParseState::Error(ParseError::Internal)) {
            HttpParseState::ReadingRequestLine { mut buf } => {
                buf.extend_from_slice(data);
                if let Some(pos) = buf.windows(2).position(|w| w == b"\r\n") {
                    let line = String::from_utf8_lossy(&buf[..pos]).to_string();
                    let parts: Vec<&str> = line.splitn(3, ' ').collect();
                    HttpParseState::ReadingHeaders {
                        method:  parts[0].to_string(),
                        path:    parts[1].to_string(),
                        headers: Vec::new(),
                    }
                } else {
                    HttpParseState::ReadingRequestLine { buf }
                }
            }
            // ... other states
            state => state,
        };
        if let HttpParseState::Complete(req) = &self.state {
            // Return cloned request
        }
        None
    }
}
```

### 15.4 Union for SIMD / Low-Level Bit Manipulation (C)

```c
// Access SIMD vector as different types using union
#include <immintrin.h>

union Vec128 {
    __m128i  as_m128i;    // SSE2 128-bit integer
    __m128   as_m128;     // SSE 128-bit float
    __m128d  as_m128d;    // SSE2 128-bit double
    uint64_t as_u64[2];
    uint32_t as_u32[4];
    uint16_t as_u16[8];
    uint8_t  as_u8[16];
};

void print_vector(union Vec128 v) {
    printf("u8: ");
    for (int i = 0; i < 16; i++)
        printf("%02x ", v.as_u8[i]);
    printf("\n");
}

// Used in network crypto (AES-NI), hashing (SSE4.2 CRC32), etc.
```

### 15.5 Go Struct Embedding for Middleware Pattern

```go
type Handler interface {
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Embed a handler to wrap it
type LoggingHandler struct {
    inner  Handler
    logger *log.Logger
}

func (h *LoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    h.inner.ServeHTTP(w, r)
    h.logger.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
}

// Chain middlewares using struct embedding:
type RateLimitedHandler struct {
    LoggingHandler          // embed — promotes ServeHTTP
    limiter *rate.Limiter
}

func (h *RateLimitedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if !h.limiter.Allow() {
        http.Error(w, "Too Many Requests", 429)
        return
    }
    h.LoggingHandler.ServeHTTP(w, r)  // delegate up
}
```

### 15.6 C Struct Versioning (Kernel ABI Stability)

```c
// The Linux kernel maintains struct compatibility across kernel versions.
// New fields are added at the END only:

// Version 1 (kernel 4.x)
struct ifreq {
    char ifr_name[IFNAMSIZ];
    union {
        struct sockaddr ifr_addr;
        struct sockaddr ifr_dstaddr;
        struct sockaddr ifr_broadaddr;
        struct sockaddr ifr_netmask;
        struct sockaddr ifr_hwaddr;
        short           ifr_flags;
        int             ifr_ifindex;
        int             ifr_metric;
        int             ifr_mtu;
        struct ifmap    ifr_map;
        char            ifr_slave[IFNAMSIZ];
        char            ifr_newname[IFNAMSIZ];
        char           *ifr_data;
    };
};

// Version 2 (kernel 5.x) — new fields added at end:
struct ifreq_v2 {
    // ... all v1 fields ...
    uint32_t ifr_new_field;  // NEW — old code ignoring this is fine
};

// Versioned structs with explicit size:
struct nla_policy {
    __u16 type;
    __u16 len;
    // kernel 5.10+:
    __u16 validation_type;
    union {
        __s64 min, max;
        const __u8 *bitfield32_valid;
    };
};
```

---

## 16. Summary: Decision Matrix

### When to Use What

```
┌───────────────────────────────────────────────────────────────────────────┐
│                         STRUCT                                            │
├───────────────────────────────────────────────────────────────────────────┤
│ USE WHEN:                                                                 │
│  • You need ALL fields simultaneously                                     │
│  • Modeling a "thing" with multiple properties (a record)                 │
│  • Data for a function (parameter grouping)                               │
│  • Implementing a vtable (struct of function pointers)                    │
│  • FFI / hardware register mapping                                        │
│  • Embedding for composition / code reuse                                 │
│                                                                           │
│ EXAMPLES: Point, TcpHeader, Config, task_struct, sk_buff                  │
└───────────────────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────┐
│                          ENUM                                             │
├───────────────────────────────────────────────────────────────────────────┤
│ USE WHEN:                                                                 │
│  • A value can be exactly ONE of several variants                         │
│  • Each variant may carry different data                                  │
│  • You need exhaustive handling (compiler-checked)                        │
│  • Modeling optional values (Option<T>)                                   │
│  • Modeling error types (Result<T,E>)                                     │
│  • Encoding state machines                                                │
│  • Named integer constants (C-style, or Go iota)                         │
│                                                                           │
│ EXAMPLES: Option, Result, TcpState, ParseError, Shape, Message            │
└───────────────────────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────────────────────┐
│                          UNION                                            │
├───────────────────────────────────────────────────────────────────────────┤
│ USE WHEN:                                                                 │
│  • You need to reinterpret the same bytes as different types              │
│  • Memory efficiency is critical (only one variant at a time)             │
│  • Implementing a tagged union manually (C)                               │
│  • FFI with C unions                                                      │
│  • Hardware register overlays                                             │
│  • Type punning (with care about aliasing rules)                          │
│                                                                           │
│ EXAMPLES: sk_buff overlapping fields, FloatBits, network header overlaps  │
│                                                                           │
│ CAUTION: Only one active variant at a time. Always check tag first.       │
│  In Rust: unsafe. In C: UB if you read wrong variant.                     │
└───────────────────────────────────────────────────────────────────────────┘
```

### Language Quick Reference

```
Feature                    C          Rust         Go
────────────────────────────────────────────────────────────────────────────
Struct named fields        YES        YES          YES
Struct method binding      NO *       YES (impl)   YES (method syntax)
Struct embedding           NO         NO (field)   YES (promoted fields)
Struct generics            NO         YES          YES (1.18+ generics)
Struct visibility          .h/.c      pub/priv     export via capitalization
Struct zero-value          YES        YES          YES (zero-init)
Struct field reorder       NO         YES (default) NO
Struct repr control        __attrib__ #[repr]      struct tags (metadata only)

Enum with data             NO         YES          NO (use interface)
Enum type safety           WEAK       STRONG       MEDIUM (iota is typed)
Exhaustive match           WARNING    COMPILE ERR  type switch (not forced)
Enum discriminant size     int        configurable int (iota)
Option type                pointer+null Option<T> interface/pointer

Union memory overlap        YES        YES (unsafe) NO (use interface/unsafe)
Union type safety          NONE       UNSAFE       N/A
Tagged union built-in      NO         YES (enum)   NO
Union in struct             YES        YES          NO

Pattern matching           switch     match        type switch, switch
Nested destructuring        NO         YES          limited
Guard clauses               NO         YES (if let) NO
Binding variables in match  NO         YES          NO

sizeof                      sizeof()   size_of::<T>() unsafe.Sizeof()
offsetof                    offsetof() offset_of!()   unsafe.Offsetof()
```

### The Fundamental Safety Spectrum

```
LEAST SAFE                                             MOST SAFE
─────────────────────────────────────────────────────────────────
C union  →  C tagged union  →  Rust union  →  Rust enum
(no tag,    (manual tag,        (unsafe read,   (tag enforced,
 no check)   manual check)      compiler aware) compiler checks)
```

### Key Takeaways

1. **Struct** = product type. All fields co-exist. Use for records and composite data.
2. **Enum** = sum type. One variant active. Use for choices, states, optional and error values.
3. **Union** = overlay type. Multiple interpretations of the same bytes. Use only when you need bit-level reinterpretation or memory compactness for a known variant.
4. **Rust enums are tagged unions** — they unify enum and union concepts with full type safety.
5. **Linux kernel** relies on intrusive structs (`list_head`, `kobject`), container_of for navigation, and careful struct layout for performance and ABI stability.
6. **sk_buff** is the central data structure for networking — understanding its `head/data/tail/end` layout is essential for kernel network programming.
7. **Memory layout matters**: padding, alignment, and cache-line placement directly affect performance. Profile before optimizing; then place hot fields together.
8. **Go has no union, no algebraic enum** — it uses interfaces + type switches to achieve similar expressiveness, at the cost of heap allocation and runtime dispatch.
9. **ABI stability** in the kernel means struct fields are never reordered and new fields go at the end only — public structs are a contract.
10. **Pattern matching exhaustiveness** is the killer feature of Rust enums: adding a variant forces you to handle it everywhere — the compiler is your refactoring safety net.

# Working with Structs in C, Go, and Rust

I'm assuming you meant "how to **handle** structs" — here's a comparison of struct handling across these three languages:

## C: Manual Everything

```c
// Definition
struct Person {
    char name[50];
    int age;
    float height;
};

// Stack allocation (automatic cleanup)
struct Person p1 = {"Alice", 30, 5.8};

// Heap allocation (manual cleanup required)
struct Person *p2 = malloc(sizeof(struct Person));
p2->name = "Bob";
p2->age = 25;
free(p2);  // Must manually free

// Accessing members
p1.age;    // Stack: use dot notation
p2->age;   // Heap: use arrow notation

// No methods - use functions instead
void print_person(struct Person *p) {
    printf("%s is %d years old\n", p->name, p->age);
}
```

**Key points:**
- You manage memory explicitly (malloc/free)
- No methods — use functions with struct pointers
- Copy semantics by default (can be expensive)
- Zero abstraction overhead

---

## Go: Simplicity with Automatic Cleanup

```go
// Definition
type Person struct {
    Name   string
    Age    int
    Height float64
}

// Stack allocation (scope-based cleanup)
p1 := Person{"Alice", 30, 5.8}

// Pointers are optional (no null dereference safety)
p2 := &Person{Name: "Bob", Age: 25}

// Accessing members (automatic pointer dereferencing)
p1.Age        // Works with value
p2.Age        // Pointers auto-deref for fields
p2.Name = "Charlie"

// Methods attached to struct (receivers)
func (p Person) PrintInfo() {
    fmt.Printf("%s is %d years old\n", p.Name, p.Age)
}

// Pointer receiver for mutations
func (p *Person) HaveBirthday() {
    p.Age++
}

p1.PrintInfo()      // Works even though PrintInfo uses value receiver
p2.HaveBirthday()   // Pointer receiver for mutation
```

**Key points:**
- Automatic memory management (garbage collected)
- Methods use receiver syntax `func (p Person) Method()`
- Pointers optional — Go decides when needed
- Simple, pragmatic design

---

## Rust: Safety + Control

```rust
// Definition
struct Person {
    name: String,
    age: u32,
    height: f64,
}

// Stack allocation (ownership-based cleanup)
let p1 = Person {
    name: String::from("Alice"),
    age: 30,
    height: 5.8,
};

// Mutable binding for mutations
let mut p2 = Person {
    name: String::from("Bob"),
    age: 25,
    height: 5.9,
};

p2.age = 26;  // Must be mutable

// Borrowing (references without ownership transfer)
fn print_info(person: &Person) {
    println!("{} is {} years old", person.name, person.age);
}

// Mutable borrows for mutations
fn have_birthday(person: &mut Person) {
    person.age += 1;
}

print_info(&p1);
have_birthday(&mut p2);

// Methods using impl blocks
impl Person {
    // Takes ownership
    fn into_name(self) -> String {
        self.name
    }
    
    // Borrows immutably
    fn age(&self) -> u32 {
        self.age
    }
    
    // Borrows mutably
    fn celebrate(&mut self) {
        self.age += 1;
    }
}

p2.celebrate();
let name = p1.into_name();  // p1 is moved, can't use it again
```

**Key points:**
- Automatic cleanup via ownership (stack-based RAII)
- Must explicitly declare mutability
- Borrowing prevents data races at compile-time
- Methods specify how they access `self`
- Heap allocations use `Box`, `Vec`, `String`, etc.

---

## Quick Comparison Table

| Feature | C | Go | Rust |
|---------|---|----|----|
| Memory | Manual (malloc/free) | Automatic (GC) | Automatic (ownership) |
| Safety | Unsafe | Safe (nil panics) | Safe (compile-time) |
| Methods | No, use functions | Yes (receivers) | Yes (impl blocks) |
| Pointers | Explicit & necessary | Optional | Explicit (borrowing) |
| Mutability | Default mutable | Default mutable | Explicit `mut` |
| Performance | Maximum | Good | Maximum (no GC) |

Would you like me to dive deeper into any of these, like memory layout, heap allocation patterns, or method design?
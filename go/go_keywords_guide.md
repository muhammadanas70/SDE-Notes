# The Complete Go Keywords Reference
## Deep Architecture, Real-World Usage, Fatal Mistakes & Mental Models

> **Philosophy**: Go has exactly 25 reserved keywords. Every single one was chosen deliberately.
> Understanding them at the compiler and runtime level changes how you architect systems.
> This guide treats you as someone who wants to think in Go, not just write Go.

---

## Table of Contents

1. [Go's Keyword Philosophy](#gos-keyword-philosophy)
2. [Mental Model: The Go Runtime Stack](#mental-model-the-go-runtime-stack)
3. [package](#1-package)
4. [import](#2-import)
5. [var](#3-var)
6. [const](#4-const)
7. [type](#5-type)
8. [struct](#6-struct)
9. [interface](#7-interface)
10. [func](#8-func)
11. [return](#9-return)
12. [if / else](#10-if--else)
13. [for](#11-for)
14. [range](#12-range)
15. [switch / case / default / fallthrough](#13-switch--case--default--fallthrough)
16. [break / continue](#14-break--continue)
17. [goto](#15-goto)
18. [go](#16-go)
19. [chan](#17-chan)
20. [select](#18-select)
21. [defer](#19-defer)
22. [map](#20-map)
23. [Fatal Patterns: The Danger Matrix](#fatal-patterns-the-danger-matrix)
24. [Misinformation Hall of Fame](#misinformation-hall-of-fame)
25. [Cloud & Linux Kernel Integration Patterns](#cloud--linux-kernel-integration-patterns)
26. [Mental Model Summary](#mental-model-summary)

---

## Go's Keyword Philosophy

Go was designed by Rob Pike, Ken Thompson, and Robert Griesemer at Google in 2007.
The language specification contains exactly **25 keywords**. Compare: C has 32, C++ has 95+,
Java has 50+. This minimalism is intentional.

```
The 25 Go Keywords
══════════════════════════════════════════════════════════
  break        default      func         interface    select
  case         defer        go           map          struct
  chan         else         goto         package      switch
  const        fallthrough  if           range        type
  continue     for          import       return       var
══════════════════════════════════════════════════════════
```

**Why does keyword count matter?** Every keyword is a reserved token the compiler treats
specially. More keywords = more parser complexity = more cognitive load for the programmer.
Go's 25 keywords map almost 1-to-1 to the runtime concepts you actually need.

---

## Mental Model: The Go Runtime Stack

Before diving into keywords, internalize this architecture. Every keyword interacts with
some layer of this stack.

```
┌─────────────────────────────────────────────────────────────┐
│                    YOUR GO PROGRAM                          │
│                                                             │
│   package  import  var  const  type  struct  interface      │
│   func  return  if  else  for  range  switch  break         │
│   continue  goto  defer  map  fallthrough  case  default    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│                   GO RUNTIME (runtime pkg)                  │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Goroutine   │  │     GC       │  │   Stack Mgmt     │  │
│  │  Scheduler   │  │  (tri-color  │  │  (segmented →    │  │
│  │  (go, chan,  │  │   mark-sweep)│  │   contiguous)    │  │
│  │   select)    │  └──────────────┘  └──────────────────┘  │
│  └──────────────┘                                           │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │    defer     │  │   panic/     │  │   Channel        │  │
│  │   stack per  │  │   recover    │  │   runtime        │  │
│  │   goroutine  │  │   mechanism  │  │   (hchan struct) │  │
│  └──────────────┘  └──────────────┘  └──────────────────┘  │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│                   OS THREAD POOL (M)                        │
│                                                             │
│   M0    M1    M2    M3    ... (GOMAXPROCS threads)          │
│    │     │     │     │                                      │
├────┴─────┴─────┴─────┴──────────────────────────────────────┤
│                    LINUX KERNEL                             │
│                                                             │
│   epoll   futex   clone   mmap   sched   signal            │
│   (I/O)  (mutex) (thread) (heap) (CPU)   (async)           │
└─────────────────────────────────────────────────────────────┘
```

**The GMP Model** (critical to understand goroutines):

```
G = Goroutine  (logical unit of execution, ~2KB stack initially)
M = Machine    (OS thread, mapped 1:1 to kernel thread)
P = Processor  (logical CPU, holds run queue, GOMAXPROCS count)

                P0 run queue          P1 run queue
               ┌──────────────┐      ┌──────────────┐
               │ G3 G4 G5 G6  │      │ G7 G8 G9     │
               └──────┬───────┘      └──────┬───────┘
                      │                     │
               ┌──────▼───────┐      ┌──────▼───────┐
               │  M0 running  │      │  M1 running  │
               │      G1      │      │      G2      │
               └──────────────┘      └──────────────┘
                      │                     │
               ┌──────▼─────────────────────▼───────┐
               │           Linux Kernel              │
               │    thread1 (tid=1234)  thread2      │
               └─────────────────────────────────────┘
```

---

## 1. `package`

### What It Is

`package` is the foundational unit of code organization and compilation in Go.
Every `.go` file must begin with a package declaration. It is **not optional**.

```go
package main      // executable: must have func main()
package server    // library package
package _         // blank import (side-effects only)
package internal  // restricted: only parent tree can import
```

### Internal Mechanics

The Go compiler compiles packages, not files. All `.go` files in a directory with the
same package name are compiled into a single object file (`.a` archive). This is why:
- Two files in the same directory with different package names = compile error
- Circular imports = compile error (compile-time dependency graph is a DAG)

```
Directory Layout → Compilation Units

myapp/
├── main.go          ─┐
├── config.go         ├─ package main → main.a
├── routes.go        ─┘
│
├── server/
│   ├── server.go    ─┐
│   └── handler.go    ├─ package server → server.a
│                    ─┘
└── internal/
    └── db/
        └── db.go    ── package db → db.a (only myapp/* can import)

Import DAG (must be acyclic):
  main → server → db
  main → db
  server ✗→ main   (would be circular)
```

### Real-World Usage: Cloud Service Package Layout

```go
// File: /cmd/apiserver/main.go
// Cloud API server following standard Go project layout
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/mycompany/platform/internal/api"
    "github.com/mycompany/platform/internal/config"
    "github.com/mycompany/platform/internal/telemetry"
)

func main() {
    cfg := config.Load()
    tel := telemetry.New(cfg.OTELEndpoint)
    defer tel.Shutdown()

    srv := api.NewServer(cfg, tel)

    // Linux signal handling - production critical
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        syscall.SIGTERM, // Kubernetes sends this on pod deletion
        syscall.SIGINT,  // Ctrl+C
        syscall.SIGHUP,  // reload config
    )
    defer cancel()

    if err := srv.Run(ctx); err != nil {
        log.Fatalf("server error: %v", err)
        os.Exit(1)
    }
}
```

```go
// File: /internal/api/server.go
// internal/ prevents external packages from importing this
package api

// Package api implements the HTTP API layer.
// Internal packages are enforced by the Go toolchain —
// any import from outside the module root's subtree
// produces: "use of internal package not allowed"
```

### Common Mistakes

**Mistake 1: Confusing package name with import path**

```go
// WRONG mental model:
import "github.com/foo/bar/v2"
// People assume: bar.Something()

// REALITY: the package NAME is what's in the file:
// file bar/somefile.go might say: package baz
// So you call: baz.Something()

// Fix: always use explicit aliases when ambiguous
import barclient "github.com/foo/bar/v2"
barclient.Something()
```

**Mistake 2: package main in test files**

```go
// This is valid but confusing:
// file: mypkg_test.go
package mypkg_test  // black-box testing, different package
// vs
package mypkg       // white-box testing, same package
// Both can exist in same directory. Most people don't know this.
```

**Mistake 3: init() ordering assumptions**

```go
// init() runs ONCE per package, in file-alphabetical order
// within a file, top-to-bottom
// You cannot control init order across packages reliably

package main

func init() {
    // DANGEROUS: assuming db is initialized here
    // because another init() in another file might not have run yet
    db.Query("SELECT 1") // potential nil panic
}
```

### Fatal Issues

**Issue: init() side effects in library packages**

```go
// Package registers itself with global state in init()
// This is the "self-registering" anti-pattern used by database drivers
package mydriver

func init() {
    sql.Register("mydriver", &Driver{})
}

// Blank import triggers init():
import _ "github.com/lib/pq" // registers postgres driver

// FATAL RISK: if two init()s register the same driver name,
// the second panics with: "sql: Register called twice for driver X"
// This happens silently when you add a new dependency.
```

---

## 2. `import`

### What It Is

`import` declares dependencies on other packages. The import path is a string that
the toolchain resolves to a directory on disk (via GOPATH or module cache).

```go
import "fmt"                          // stdlib
import "os"                           // stdlib
import "github.com/user/pkg"          // module path
import . "math"                       // dot import: Sqrt() not math.Sqrt()
import _ "github.com/lib/pq"          // blank: side-effects only
import myname "github.com/foo/bar"    // alias
```

### Internal Mechanics: How Import Resolution Works

```
Source Code           Toolchain Resolution
─────────────         ──────────────────────────────────────────
import "fmt"    →     $GOROOT/src/fmt/
import "os"     →     $GOROOT/src/os/
import          →     $GOMODCACHE/github.com/user/pkg@v1.2.3/
  "github.com/        (verified via go.sum SHA-256 hash)
   user/pkg"

go.mod controls the module graph:
┌─────────────────────────────────────────────┐
│ module github.com/myco/myapp               │
│ go 1.22                                    │
│                                            │
│ require (                                  │
│   github.com/user/pkg v1.2.3               │
│   golang.org/x/net v0.20.0                 │
│ )                                          │
└─────────────────────────────────────────────┘
         │
         ▼
go.sum (cryptographic pins):
github.com/user/pkg v1.2.3 h1:abc123...
github.com/user/pkg v1.2.3/go.mod h1:def456...
```

### Real-World Usage: Dependency Injection via Interface + Import

```go
// internal/storage/storage.go
package storage

// Define behavior, not implementation
type BlobStore interface {
    Put(ctx context.Context, key string, data []byte) error
    Get(ctx context.Context, key string) ([]byte, error)
    Delete(ctx context.Context, key string) error
}
```

```go
// internal/storage/gcs/gcs.go
package gcs

import (
    "context"
    "fmt"
    "io"

    "cloud.google.com/go/storage"   // GCS SDK
    "google.golang.org/api/option"
)

type Client struct {
    bucket *storage.BucketHandle
}

func New(ctx context.Context, projectID, bucketName string) (*Client, error) {
    c, err := storage.NewClient(ctx, option.WithoutAuthentication())
    if err != nil {
        return nil, fmt.Errorf("gcs.New: %w", err)
    }
    return &Client{bucket: c.Bucket(bucketName)}, nil
}

func (c *Client) Put(ctx context.Context, key string, data []byte) error {
    w := c.bucket.Object(key).NewWriter(ctx)
    if _, err := w.Write(data); err != nil {
        return fmt.Errorf("gcs.Put write: %w", err)
    }
    return w.Close()
}
```

### Common Mistakes

**Mistake 1: Dot imports in production code**

```go
// NEVER do this in production:
import . "math"

Sqrt(4.0)   // Where does this come from? math.Sqrt? Some local func?

// Dot imports:
// 1. Pollute the namespace
// 2. Make grep/tooling harder
// 3. Create hidden name collisions
// 4. Are only appropriate in test files for DSL-style testing
```

**Mistake 2: Importing for side-effects without documenting why**

```go
// BAD:
import _ "net/http/pprof"

// GOOD: document exactly what side-effect you want
// Import pprof to register /debug/pprof/* HTTP handlers
// for production profiling. Remove before GA release.
import _ "net/http/pprof"
```

**Mistake 3: Version suffix confusion**

```go
// github.com/foo/bar/v2 is a DIFFERENT module from github.com/foo/bar
// You CAN import both simultaneously (major version compatibility)
import (
    bar "github.com/foo/bar"      // v1.x
    barv2 "github.com/foo/bar/v2" // v2.x - breaking API changes
)
// This is intentional Go design, not a bug.
// Most people think it's broken when both appear in go.mod.
```

### Misinformation

> "Unused imports are just warnings"

**FALSE.** Unused imports are **compile errors** in Go. This is a hard rule.
The compiler forces this because unused imports increase binary size and compilation time.
If you need to temporarily disable an import, use `_ "pkg"` or comment it out.

---

## 3. `var`

### What It Is

`var` declares one or more variables with an explicit type, an initializer expression,
or both. Variables declared with `var` are zero-initialized if no initializer is given.

```go
var x int              // x = 0
var s string           // s = ""
var p *int             // p = nil
var b bool             // b = false
var f float64          // f = 0.0
var sl []int           // sl = nil (not empty slice!)
var m map[string]int   // m = nil (not empty map!)

// Multiple:
var (
    host string = "localhost"
    port int    = 8080
    tls  bool
)

// Short declaration (inside functions only):
x := 42        // inferred type int
y := "hello"   // inferred type string
```

### Zero Value: The Most Important Concept in Go Variables

```
Type              Zero Value    Ready to use?
──────────────    ──────────    ─────────────────────────────
bool              false         yes
int/int64/etc     0             yes
float32/64        0.0           yes
complex64/128     0+0i          yes
string            ""            yes
[]T               nil           NO — must make() before append
map[K]V           nil           NO — must make() before write
chan T             nil           NO — send/recv blocks forever
*T                nil           NO — dereferencing panics
interface{}       nil           yes (but type assertion will fail)
struct{}          all fields     yes (fields individually zero'd)
                  zeroed
func              nil           NO — calling nil func panics
```

### Internal Mechanics: Stack vs Heap Allocation

```go
// Go compiler decides: stack or heap?
// Rule: if a variable's lifetime exceeds its scope, it escapes to heap

func stackVar() int {
    x := 42        // stays on stack: doesn't escape
    return x       // value copied out, x's storage freed
}

func heapVar() *int {
    x := 42        // ESCAPES to heap: address outlives function
    return &x      // x survives because pointer is returned
}

// Check escape analysis:
// go build -gcflags='-m -m' ./...
// Output: "./main.go:8:2: x escapes to heap"
```

```
Stack Frame (goroutine's stack):
┌────────────────────────────────────┐
│  main() frame                      │
│  ┌─────────────┐                   │
│  │ x = 42      │ ← lives here      │
│  │ y = "hello" │   freed on return │
│  └─────────────┘                   │
├────────────────────────────────────┤
│  heapVar() frame                   │
│  ┌─────────────┐                   │
│  │ ptr → ─────┼──────────────────────→ heap: [42]
│  └─────────────┘                   │
└────────────────────────────────────┘
         Goroutine Stack              GC-managed Heap
         (2KB → 1GB, grows)          (shared, GC scans)
```

### Real-World Usage: Configuring a Linux Networking Daemon

```go
package main

import (
    "log"
    "net"
    "os"
    "strconv"
    "syscall"
    "time"
)

// Package-level vars: initialized before main(), once per process
var (
    // Read from environment — cloud-native config pattern (12-factor)
    listenAddr = envOrDefault("LISTEN_ADDR", "0.0.0.0:8080")
    maxConns   = envIntOrDefault("MAX_CONNS", 10000)
    readTimeout = envDurationOrDefault("READ_TIMEOUT", 30*time.Second)
)

func envOrDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func envIntOrDefault(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return def
}

func envDurationOrDefault(key string, def time.Duration) time.Duration {
    if v := os.Getenv(key); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return def
}

// Using SO_REUSEPORT for multi-process listening (Linux kernel feature)
// Allows multiple goroutines/processes to bind same port → kernel load balances
func newReusePortListener(addr string) (net.Listener, error) {
    lc := net.ListenConfig{
        Control: func(network, address string, c syscall.RawConn) error {
            var setsockoptErr error
            err := c.Control(func(fd uintptr) {
                // SO_REUSEPORT: Linux 3.9+
                // Allows multiple sockets on same port
                // Kernel distributes incoming connections
                setsockoptErr = syscall.SetsockoptInt(
                    int(fd),
                    syscall.SOL_SOCKET,
                    syscall.SO_REUSEPORT,
                    1,
                )
            })
            if err != nil {
                return err
            }
            return setsockoptErr
        },
    }
    return lc.Listen(nil /* context */, "tcp", addr)
}

func main() {
    // Local var with short declaration
    ln, err := newReusePortListener(listenAddr)
    if err != nil {
        log.Fatalf("listen %s: %v", listenAddr, err)
    }
    defer ln.Close()

    log.Printf("listening on %s (max_conns=%d)", listenAddr, maxConns)

    // Semaphore pattern using buffered channel + var
    var sem = make(chan struct{}, maxConns)

    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Printf("accept error: %v", err)
            continue
        }

        sem <- struct{}{} // acquire slot
        go func(c net.Conn) {
            defer func() { <-sem }() // release slot
            defer c.Close()
            handleConn(c, readTimeout)
        }(conn)
    }
}

func handleConn(c net.Conn, timeout time.Duration) {
    c.SetDeadline(time.Now().Add(timeout))
    // ... handle connection
}
```

### Common Mistakes

**Mistake 1: nil map write (most common runtime panic)**

```go
// FATAL:
var m map[string]int
m["key"] = 1  // panic: assignment to entry in nil map

// CORRECT:
m := make(map[string]int)
m["key"] = 1

// OR:
var m = map[string]int{}
m["key"] = 1
```

**Mistake 2: nil slice vs empty slice distinction**

```go
var s1 []int          // nil slice:   s1 == nil → true,  len=0, cap=0
s2 := []int{}         // empty slice: s2 == nil → false, len=0, cap=0
s3 := make([]int, 0)  // empty slice: s3 == nil → false, len=0, cap=0

// json.Marshal treats nil slice as null, empty slice as []
// This causes API contract bugs:
json.Marshal(s1)  // → null
json.Marshal(s2)  // → []

// Both append() and range work on nil slices, but JSON doesn't
```

**Mistake 3: Shadow declarations hiding outer variables**

```go
x := 10
if condition {
    x := 20           // NEW variable x, shadows outer x
    fmt.Println(x)    // prints 20
}
fmt.Println(x)        // prints 10 — outer x unchanged!

// This is legal Go. Many bugs come from := inside blocks.
// Use = to assign to existing variable:
if condition {
    x = 20            // modifies outer x
}
```

### Fatal Issues

**Issue: var in goroutine closures — data race**

```go
// RACE CONDITION:
var results []string
for _, url := range urls {
    go func() {
        resp := fetch(url)        // url captured by reference!
        results = append(results, resp) // concurrent write: DATA RACE
    }()
}

// CORRECT:
var mu sync.Mutex
var results []string
for _, url := range urls {
    url := url  // shadow: new variable per iteration (Go 1.21 fixes loop var, but be explicit)
    go func() {
        resp := fetch(url)
        mu.Lock()
        results = append(results, resp)
        mu.Unlock()
    }()
}
```

---

## 4. `const`

### What It Is

`const` declares constants — values computed at compile time. They can be typed or
untyped. Untyped constants have a "kind" (integer, float, string, bool, rune, complex)
but no committed Go type until they are used in a context that requires one.

```go
const Pi = 3.14159265358979    // untyped float constant
const MaxUint = ^uint(0)       // untyped integer
const Greeting = "hello"       // untyped string

const (
    KB = 1024
    MB = 1024 * KB
    GB = 1024 * MB
    TB = 1024 * GB
)
```

### Iota: Go's Enumerator

`iota` is a predeclared identifier in a `const` block that represents the index of
the current constant specification (starts at 0, increments by 1 per line).

```go
const (
    // Linux file permission bits — real kernel values
    S_IXOTH = 1 << iota   // 0001 = 1  (execute, others)
    S_IWOTH               // 0010 = 2  (write, others)
    S_IROTH               // 0100 = 4  (read, others)
    // etc.
)

const (
    // iota resets to 0 in each new const block
    _  = iota             // skip 0
    KB = 1 << (10 * iota) // 1 << 10 = 1024
    MB                    // 1 << 20
    GB                    // 1 << 30
    TB                    // 1 << 40
    PB                    // 1 << 50
)

// Bit flags pattern — used everywhere in Linux-style APIs:
type Permission uint32

const (
    PermRead    Permission = 1 << iota // 0b001
    PermWrite                          // 0b010
    PermExecute                        // 0b100
)

func (p Permission) String() string {
    var s string
    if p&PermRead != 0    { s += "r" } else { s += "-" }
    if p&PermWrite != 0   { s += "w" } else { s += "-" }
    if p&PermExecute != 0 { s += "x" } else { s += "-" }
    return s
}
```

### Internal Mechanics: Why Constants Are Different

```
Variable (var):                    Constant (const):
─────────────────────────────      ──────────────────────────────────
Allocated at runtime               Exists only at compile time
Has an address (&x is valid)       Has NO address (&c is compile error)
Can change (mutable)               Can never change (immutable)
Stored in memory at runtime        Folded into instructions by compiler
Type is committed on declaration   Untyped: adapts to context
```

```go
// Untyped constant behavior:
const Big = 1 << 62      // fine: untyped integer constant
var x int = Big          // fine: fits in int64
var y int8 = Big         // compile error: constant 4611686018427387904
                         //   overflows int8

// This is why stdlib uses untyped constants for math.MaxInt etc.
const MaxInt = int(^uint(0) >> 1) // typed: platform-specific
```

### Real-World Usage: Linux Syscall Constants for Cloud Networking

```go
package netutil

import (
    "fmt"
    "syscall"
    "unsafe"
)

// Linux socket option constants (mirrors linux/socket.h)
// These supplement what syscall package provides
const (
    TCP_KEEPIDLE  = 4  // Seconds before first keepalive probe
    TCP_KEEPINTVL = 5  // Seconds between keepalive probes
    TCP_KEEPCNT   = 6  // Max failed probes before giving up
    TCP_USER_TIMEOUT = 18 // Timeout for unacknowledged data (ms)

    // SO_REUSEPORT for multi-process/goroutine listeners
    SO_REUSEPORT = 15

    // TCP_FASTOPEN for reduced connection latency (Linux 3.7+)
    TCP_FASTOPEN = 23
)

// HTTP/2 and gRPC connections need aggressive keepalives
// to detect broken connections through NAT/firewalls
func SetKeepalive(fd int, idle, interval, count int) error {
    if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET,
        syscall.SO_KEEPALIVE, 1); err != nil {
        return fmt.Errorf("SO_KEEPALIVE: %w", err)
    }
    if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP,
        TCP_KEEPIDLE, idle); err != nil {
        return fmt.Errorf("TCP_KEEPIDLE: %w", err)
    }
    if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP,
        TCP_KEEPINTVL, interval); err != nil {
        return fmt.Errorf("TCP_KEEPINTVL: %w", err)
    }
    if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP,
        TCP_KEEPCNT, count); err != nil {
        return fmt.Errorf("TCP_KEEPCNT: %w", err)
    }
    return nil
}

// epoll constants for high-performance I/O
const (
    EPOLLIN      = 0x001 // available for read
    EPOLLOUT     = 0x004 // available for write
    EPOLLRDHUP   = 0x2000 // peer closed writing half
    EPOLLET      = 0x80000000 // edge-triggered mode
    EPOLLONESHOT = 0x40000000 // one-shot mode
)

// Typed constant for state machine
type ConnState uint8

const (
    ConnStateNew ConnState = iota
    ConnStateActive
    ConnStateIdle
    ConnStateHijacked
    ConnStateClosed
)

func (s ConnState) String() string {
    return [...]string{
        "new", "active", "idle", "hijacked", "closed",
    }[s]
}
```

### Common Mistakes

**Mistake 1: iota skipping confusion**

```go
const (
    A = iota     // 0
    B            // 1
    C = "hello"  // "hello" — iota is 2 but not used
    D            // "hello" — repeats last expression, iota is 3 but not used
    E = iota     // 4 — resumes iota
)
// D = "hello" surprises almost everyone.
// iota continues incrementing even when not used in the expression.
```

**Mistake 2: Constants are not addressable**

```go
const MaxRetries = 3

ptr := &MaxRetries  // compile error: cannot take address of constant
// There is no memory location for a constant.
// If you need a pointer to a typed value, use a var.
```

**Mistake 3: Typed vs untyped constant in arithmetic**

```go
const c = 1.5    // untyped float
var i int = 3
_ = i + c        // compile error: mismatched types int and float constant

const typed float64 = 1.5
_ = i + int(typed) // OK with explicit conversion
```

---

## 5. `type`

### What It Is

`type` creates a new named type. This is one of Go's most powerful features.
A new type has the same underlying structure as its source type but is a distinct type
for the compiler — you cannot mix them without explicit conversion.

```go
type MyInt int          // new type, not alias
type Celsius float64    // semantic type
type UserID string      // prevents mixing with other string types
type Handler func(http.ResponseWriter, *http.Request)
type StringMap map[string]string

// Type alias (Go 1.9+): truly the same type, interchangeable
type Alias = ExistingType
```

### Internal Mechanics: Type Identity

```
Named Type:                         Underlying Type:
────────────────────                ────────────────
type Celsius float64                float64
type Fahrenheit float64             float64

Celsius and Fahrenheit are DIFFERENT types even though
both have underlying type float64.

var c Celsius    = 100.0  // OK
var f Fahrenheit = 100.0  // OK
c = f                     // COMPILE ERROR: cannot use f (Fahrenheit) as Celsius
c = Celsius(f)            // OK: explicit conversion

type Handler func(http.ResponseWriter, *http.Request)
var h Handler             // OK
h = someFunc              // OK IF someFunc has matching signature
```

### Type Methods: Adding Behavior to Any Type

```go
// You can add methods to ANY named type, including built-in type aliases
type Duration int64   // nanoseconds, like time.Duration

func (d Duration) Hours() float64   { return float64(d) / float64(hour) }
func (d Duration) Minutes() float64 { return float64(d) / float64(minute) }
func (d Duration) String() string   { ... }

const (
    nanosecond  Duration = 1
    microsecond          = 1000 * nanosecond
    millisecond          = 1000 * microsecond
    second               = 1000 * millisecond
    minute               = 60 * second
    hour                 = 60 * minute
)
```

### Real-World Usage: Domain-Driven Types for Cloud APIs

```go
package cloud

import (
    "fmt"
    "regexp"
    "time"
)

// Strong domain types prevent a class of bugs at compile time.
// You CANNOT accidentally pass an InstanceID where a ClusterID is expected.

type InstanceID string
type ClusterID  string
type RegionID   string
type ProjectID  string

// Newtypes carry validation in constructors
var instanceIDRe = regexp.MustCompile(`^i-[a-f0-9]{8,17}$`)

func NewInstanceID(s string) (InstanceID, error) {
    if !instanceIDRe.MatchString(s) {
        return "", fmt.Errorf("invalid instance ID: %q", s)
    }
    return InstanceID(s), nil
}

// Function signature is now self-documenting and type-safe
func TerminateInstance(ctx context.Context,
    project ProjectID,
    region RegionID,
    instance InstanceID,
) error {
    // Can't mix up parameters — compiler catches it
    return nil
}

// Functional options pattern — uses type to encode config
type ServerOption func(*serverConfig)

type serverConfig struct {
    maxConns    int
    readTimeout time.Duration
    tlsEnabled  bool
}

func WithMaxConns(n int) ServerOption {
    return func(c *serverConfig) { c.maxConns = n }
}

func WithReadTimeout(d time.Duration) ServerOption {
    return func(c *serverConfig) { c.readTimeout = d }
}

func WithTLS() ServerOption {
    return func(c *serverConfig) { c.tlsEnabled = true }
}

type Server struct{ cfg serverConfig }

func NewServer(opts ...ServerOption) *Server {
    cfg := serverConfig{
        maxConns:    1000,
        readTimeout: 30 * time.Second,
    }
    for _, opt := range opts {
        opt(&cfg)
    }
    return &Server{cfg: cfg}
}

// Usage:
// s := NewServer(WithMaxConns(5000), WithTLS(), WithReadTimeout(60*time.Second))
```

### Type Embedding vs Inheritance

```go
// Go has NO inheritance. It has EMBEDDING.
// Embedding promotes methods and fields — it is NOT "is-a".

type Logger struct {
    prefix string
}
func (l *Logger) Log(msg string) { fmt.Printf("[%s] %s\n", l.prefix, msg) }

type Server struct {
    Logger      // embedded: NOT inherited. Server "has-a" Logger.
    addr string
}

// Server gets Log() promoted:
s := Server{Logger: Logger{prefix: "server"}, addr: ":8080"}
s.Log("starting")       // calls s.Logger.Log("starting")
s.Logger.Log("starting") // equivalent, explicit

// Embedding an interface embeds the method set:
type ReadWriter interface {
    io.Reader   // embedded: has Read()
    io.Writer   // embedded: has Write()
}
```

### Fatal Issues

**Issue: type assertion on nil interface**

```go
type Animal interface{ Sound() string }

var a Animal    // nil interface (type=nil, value=nil)

// This panics:
dog := a.(Dog)  // panic: interface conversion: interface is nil, not Dog

// Safe:
dog, ok := a.(Dog)
if !ok { /* handle */ }
```

---

## 6. `struct`

### What It Is

`struct` defines a composite type — a collection of named fields. Structs are the
primary mechanism for data modeling in Go. There are no classes.

```go
type Point struct {
    X, Y float64
}

type Server struct {
    addr     string
    port     int
    listener net.Listener
    mu       sync.Mutex  // zero value is valid unlocked mutex
    conns    map[string]net.Conn
}
```

### Internal Mechanics: Memory Layout

```
struct Point { X, Y float64 }

Memory layout (little-endian 64-bit):
Offset  Size   Field
──────  ────   ─────
0       8      X (float64)
8       8      Y (float64)
                         Total: 16 bytes

struct BadLayout {
    a bool    // 1 byte
    b int64   // 8 bytes, but must be 8-byte aligned!
    c bool    // 1 byte
}

Actual memory with padding:
Offset  Size   Content
──────  ────   ────────────────────────────────────
0       1      a (bool)
1       7      [PADDING — wasted]
8       8      b (int64)
16      1      c (bool)
17      7      [PADDING — wasted]
               Total: 24 bytes (worst case!)

Optimized layout (largest fields first):
struct GoodLayout {
    b int64   // 8 bytes at offset 0
    a bool    // 1 byte at offset 8
    c bool    // 1 byte at offset 9
              // 6 bytes padding at 10-15
}
             Total: 16 bytes (25% saving)

Use: fieldalignment tool:  go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
```

### Real-World Usage: Linux cgroup v2 Statistics Struct (Container Monitoring)

```go
package cgroup

import (
    "bufio"
    "fmt"
    "os"
    "path/filepath"
    "strconv"
    "strings"
)

// Mirrors Linux cgroup v2 memory.stat interface
// Used by container runtimes (containerd, cri-o) for pod metrics
type MemoryStat struct {
    // Ordering optimized: 8-byte fields first
    Anon              uint64 // anonymous memory
    File              uint64 // page cache
    KernelStack       uint64 // kernel stacks
    Slab              uint64 // slab allocator memory
    Sock              uint64 // network buffer memory
    Shmem             uint64 // shared memory
    FileMapped        uint64 // file-backed mapped memory
    FileDirty         uint64 // dirty page cache
    FileWriteback     uint64 // pages under writeback
    AnonThp           uint64 // anonymous transparent hugepages
    InactiveAnon      uint64
    ActiveAnon        uint64
    InactiveFile      uint64
    ActiveFile        uint64
    Unevictable       uint64
    SwapCached        uint64
    // Totals
    Usage             uint64 // from memory.current
    SwapUsage         uint64 // from memory.swap.current
    Limit             uint64 // from memory.max
    SwapLimit         uint64 // from memory.swap.max
}

// CPUStat mirrors cgroup v2 cpu.stat
type CPUStat struct {
    UsageUsec     uint64 // total CPU time (microseconds)
    UserUsec      uint64 // user-space CPU time
    SystemUsec    uint64 // kernel CPU time
    NrPeriods     uint64 // number of throttle periods
    NrThrottled   uint64 // number of times throttled
    ThrottledUsec uint64 // total time throttled
}

// ContainerMetrics represents a single container's resource usage
type ContainerMetrics struct {
    ContainerID string
    PodUID      string
    Namespace   string
    Memory      MemoryStat
    CPU         CPUStat
    // Anonymous struct for network — used when type won't be reused
    Network struct {
        RxBytes   uint64
        TxBytes   uint64
        RxPackets uint64
        TxPackets uint64
        RxDropped uint64
        TxDropped uint64
    }
}

// ReadMemoryStat reads from /sys/fs/cgroup/<path>/memory.stat
func ReadMemoryStat(cgroupPath string) (MemoryStat, error) {
    var stat MemoryStat

    // memory.current (total usage)
    if b, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current")); err == nil {
        stat.Usage, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
    }

    // memory.max (limit, "max" = unlimited)
    if b, err := os.ReadFile(filepath.Join(cgroupPath, "memory.max")); err == nil {
        s := strings.TrimSpace(string(b))
        if s != "max" {
            stat.Limit, _ = strconv.ParseUint(s, 10, 64)
        } else {
            stat.Limit = ^uint64(0) // unlimited: max uint64
        }
    }

    // memory.stat (detailed breakdown)
    f, err := os.Open(filepath.Join(cgroupPath, "memory.stat"))
    if err != nil {
        return stat, fmt.Errorf("open memory.stat: %w", err)
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := scanner.Text()
        fields := strings.Fields(line)
        if len(fields) != 2 {
            continue
        }
        key, valStr := fields[0], fields[1]
        val, _ := strconv.ParseUint(valStr, 10, 64)

        // Use a struct field map for clean parsing
        switch key {
        case "anon":            stat.Anon = val
        case "file":            stat.File = val
        case "kernel_stack":    stat.KernelStack = val
        case "slab":            stat.Slab = val
        case "sock":            stat.Sock = val
        case "shmem":           stat.Shmem = val
        case "file_mapped":     stat.FileMapped = val
        case "file_dirty":      stat.FileDirty = val
        case "file_writeback":  stat.FileWriteback = val
        case "anon_thp":        stat.AnonThp = val
        case "inactive_anon":   stat.InactiveAnon = val
        case "active_anon":     stat.ActiveAnon = val
        case "inactive_file":   stat.InactiveFile = val
        case "active_file":     stat.ActiveFile = val
        case "unevictable":     stat.Unevictable = val
        case "swapcached":      stat.SwapCached = val
        }
    }
    return stat, scanner.Err()
}

// MemoryPressure returns [0.0, 1.0] fraction of limit in use
func (m *MemoryStat) MemoryPressure() float64 {
    if m.Limit == 0 || m.Limit == ^uint64(0) {
        return 0
    }
    return float64(m.Usage) / float64(m.Limit)
}
```

### Common Mistakes

**Mistake 1: Copying structs containing mutexes or channels**

```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func process(c SafeCounter) {  // WRONG: copies the mutex!
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}
// A copied mutex is a new, independent mutex.
// The original and copy are not synchronized.
// go vet will warn: "passes lock by value"

// CORRECT: pass pointer
func process(c *SafeCounter) {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}
```

**Mistake 2: Comparing structs with incomparable fields**

```go
type A struct {
    Name string
    Tags []string   // slices are NOT comparable
}
a1 := A{"x", []string{"t"}}
a2 := A{"x", []string{"t"}}
_ = a1 == a2  // compile error: struct containing []string cannot be compared

// Use reflect.DeepEqual or define your own Equal method
```

**Mistake 3: Unexported struct fields in JSON/encoding**

```go
type Config struct {
    Host string  // exported: JSON key "Host"
    port int     // unexported: INVISIBLE to json.Marshal
    tls  bool    // unexported: INVISIBLE to json.Marshal
}

// json.Marshal sees only Host.
// port and tls will always be zero after json.Unmarshal.
// Fix: export them (Port, TLS) or add json tags.

type Config struct {
    Host string `json:"host"`
    Port int    `json:"port"`
    TLS  bool   `json:"tls,omitempty"`
}
```

---

## 7. `interface`

### What It Is

`interface` defines a set of method signatures. Any type that implements all the methods
satisfies the interface — **implicitly**, without declaring it. This is structural typing
(duck typing with compile-time checking).

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

type ReadWriter interface {
    Reader  // embedded interface
    Writer
}

// Empty interface: every type satisfies it
type any = interface{}   // Go 1.18+ alias
```

### Internal Mechanics: Interface Values

An interface value is a **pair**: (type pointer, value pointer).

```
Interface value layout in memory:
┌─────────────────┬─────────────────┐
│   *itab         │   *data         │
│   (type info)   │   (value ptr)   │
└─────────────────┴─────────────────┘

itab contains:
┌──────────────────────────────────┐
│ *_type  (type descriptor)        │
│ *interfacetype                   │
│ fun[0] → method 0 address        │
│ fun[1] → method 1 address        │
│ ...                              │
└──────────────────────────────────┘

A nil interface: both pointers are nil
A non-nil interface with nil value:
  *itab = &concreteTypeInfo
  *data = nil
  → This is the "nil interface trap"!
```

### The Nil Interface Trap (Fatal Issue)

```go
// This is THE most insidious Go bug:
func getError() error {
    var p *MyError = nil    // typed nil pointer
    if someCondition {
        p = &MyError{msg: "oops"}
    }
    return p  // WRONG: returns non-nil interface with nil value!
}

err := getError()
if err != nil {  // TRUE — interface has type info even though value is nil!
    // This branch executes even when no error occurred!
    fmt.Println(err.Error())  // panic: nil pointer dereference
}

// CORRECT: return untyped nil for no-error case
func getError() error {
    var p *MyError = nil
    if someCondition {
        p = &MyError{msg: "oops"}
        return p   // non-nil interface, non-nil value: OK
    }
    return nil     // truly nil interface: (nil, nil)
}
```

```
nil interface vs non-nil interface with nil value:

  var err error = nil
  ┌─────────┬─────────┐
  │  nil    │  nil    │   → err == nil  is TRUE
  └─────────┴─────────┘

  var p *MyError = nil
  var err error = p
  ┌─────────┬─────────┐
  │ *MyError│  nil    │   → err == nil  is FALSE!
  └─────────┴─────────┘
```

### Real-World Usage: Cloud Provider Abstraction

```go
package cloud

import (
    "context"
    "io"
    "time"
)

// Define the contract, not the implementation
// This lets you swap AWS S3, GCS, Azure Blob at compile time
// or in tests with a mock

type ObjectStore interface {
    // PutObject uploads data. Implementations handle retries/multipart.
    PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64) error

    // GetObject downloads an object. Caller must close the returned reader.
    GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)

    // HeadObject returns metadata without downloading.
    HeadObject(ctx context.Context, bucket, key string) (*ObjectMeta, error)

    // DeleteObject removes an object.
    DeleteObject(ctx context.Context, bucket, key string) error

    // ListObjects lists objects with a prefix.
    ListObjects(ctx context.Context, bucket, prefix string) ([]ObjectMeta, error)
}

type ObjectMeta struct {
    Key          string
    Size         int64
    ETag         string
    LastModified time.Time
    ContentType  string
}

// Compile-time interface satisfaction check.
// This line will fail to compile if s3Client doesn't implement ObjectStore.
// Put this in the same file as the concrete type.
var _ ObjectStore = (*S3Client)(nil)
var _ ObjectStore = (*GCSClient)(nil)
var _ ObjectStore = (*MemStore)(nil)   // for tests

// MemStore: in-memory implementation for unit tests
// No AWS credentials, no network, fast.
type MemStore struct {
    mu      sync.RWMutex
    buckets map[string]map[string][]byte
}

func NewMemStore() *MemStore {
    return &MemStore{
        buckets: make(map[string]map[string][]byte),
    }
}

func (m *MemStore) PutObject(ctx context.Context, bucket, key string,
    body io.Reader, size int64) error {
    data, err := io.ReadAll(body)
    if err != nil {
        return err
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.buckets[bucket] == nil {
        m.buckets[bucket] = make(map[string][]byte)
    }
    m.buckets[bucket][key] = data
    return nil
}

func (m *MemStore) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    b, ok := m.buckets[bucket]
    if !ok {
        return nil, fmt.Errorf("bucket %q not found", bucket)
    }
    data, ok := b[key]
    if !ok {
        return nil, fmt.Errorf("object %q/%q not found", bucket, key)
    }
    return io.NopCloser(bytes.NewReader(data)), nil
}

// ... other methods

// Interface used in service layer — doesn't care which cloud:
type Archiver struct {
    store ObjectStore
    bucket string
}

func NewArchiver(store ObjectStore, bucket string) *Archiver {
    return &Archiver{store: store, bucket: bucket}
}

func (a *Archiver) Archive(ctx context.Context, name string, data []byte) error {
    return a.store.PutObject(ctx, a.bucket, name,
        bytes.NewReader(data), int64(len(data)))
}
```

### Misinformation

> "interface{} / any is like Object in Java — it's dynamic typing"

**FALSE.** Interface values are still statically typed at runtime (the type pointer
in the interface pair). The difference is that the check happens at runtime via type
assertion, not at compile time. Go is always statically typed — there is no dynamic
dispatch without explicit type assertions.

> "Interfaces with many methods are better because they're more complete"

**FALSE.** The Go idiom is the opposite:

```
io.Reader:   1 method  (Read)
io.Writer:   1 method  (Write)
io.Closer:   1 method  (Close)
io.ReadWriter: 2 methods (composed)
io.ReadWriteCloser: 3 methods (composed)

"The bigger the interface, the weaker the abstraction."
                                    — Rob Pike
```

Small interfaces compose better, are easier to implement (including mocks), and lead
to more decoupled code.

---

## 8. `func`

### What It Is

`func` declares a function. Functions are first-class values in Go — they can be
assigned to variables, passed as arguments, and returned from other functions.

```go
// Basic function
func add(a, b int) int { return a + b }

// Multiple return values
func divide(a, b float64) (float64, error) {
    if b == 0 {
        return 0, errors.New("division by zero")
    }
    return a / b, nil
}

// Named return values (use sparingly)
func minmax(a, b int) (min, max int) {
    if a < b { return a, b }
    return b, a
}

// Variadic
func sum(nums ...int) int {
    total := 0
    for _, n := range nums {
        total += n
    }
    return total
}

// Function type
type Handler func(ctx context.Context, req Request) (Response, error)

// Method (function with receiver)
func (s *Server) Serve(ctx context.Context) error { ... }

// Closure (captures outer scope variables)
func counter() func() int {
    n := 0
    return func() int {
        n++    // n is captured by reference
        return n
    }
}
```

### Internal Mechanics: Function Calls and the Call Stack

```
Go function call mechanics:

Caller                   Callee (func add(a, b int) int)
──────────               ──────────────────────────────
push args on stack  →    a = arg1, b = arg2 (on caller's frame)
CALL instruction    →    save return address
                         allocate local vars (return values slot)
                    ←    return value in designated slot
POP return value         restore registers
                         RET instruction

Go uses "goroutine stacks" (not OS stacks):
  - Initially: 2KB (Go 1.4+: 8KB)
  - Grows dynamically: up to 1GB (GOARCH-dependent)
  - Uses stack copying (not segmented stacks since Go 1.4)

Stack growth:
  goroutine stack fills up
        ↓
  runtime detects (via stack guard pointer check on every call)
        ↓
  allocates 2x larger stack on heap
        ↓
  copies all stack frames
        ↓
  updates all pointers to new stack locations
        ↓
  resumes execution
```

### Closures: The Capture Semantics

```go
// Closures capture VARIABLES (by reference), not VALUES

funcs := make([]func(), 5)
for i := 0; i < 5; i++ {
    funcs[i] = func() {
        fmt.Println(i)  // captures variable i, not its value
    }
}
for _, f := range funcs {
    f() // prints 5, 5, 5, 5, 5 — all see final i value
}

// FIX 1: shadow variable
for i := 0; i < 5; i++ {
    i := i  // new i per iteration
    funcs[i] = func() { fmt.Println(i) }
}

// FIX 2: pass as argument
for i := 0; i < 5; i++ {
    funcs[i] = func(n int) func() {
        return func() { fmt.Println(n) }
    }(i)
}

// Note: Go 1.22 changed loop variable semantics — i is
// now per-iteration by default. Still document your intent.
```

### Real-World Usage: Middleware Chain (HTTP + gRPC pattern)

```go
package middleware

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "go.opentelemetry.io/otel/trace"
    "golang.org/x/time/rate"
)

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares from left to right:
// Chain(A, B, C)(handler) = A(B(C(handler)))
func Chain(mws ...Middleware) Middleware {
    return func(next http.Handler) http.Handler {
        // Apply in reverse so request flows left-to-right
        for i := len(mws) - 1; i >= 0; i-- {
            next = mws[i](next)
        }
        return next
    }
}

// RateLimit returns a middleware that limits to `rps` requests/second
func RateLimit(rps float64, burst int) Middleware {
    limiter := rate.NewLimiter(rate.Limit(rps), burst)
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// Tracing adds OpenTelemetry span around each request
func Tracing(tracer trace.Tracer) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, span := tracer.Start(r.Context(),
                fmt.Sprintf("%s %s", r.Method, r.URL.Path))
            defer span.End()
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Recovery catches panics and returns 500
func Recovery(logger Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if rec := recover(); rec != nil {
                    logger.Error("panic recovered", "panic", rec)
                    http.Error(w, "Internal Server Error", 500)
                }
            }()
            next.ServeHTTP(w, r)
        })
    }
}

// Logging logs request duration
func Logging(logger Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            rw := &responseWriter{ResponseWriter: w, code: 200}
            next.ServeHTTP(rw, r)
            logger.Info("request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", rw.code,
                "duration", time.Since(start),
            )
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    code int
}
func (rw *responseWriter) WriteHeader(code int) {
    rw.code = code
    rw.ResponseWriter.WriteHeader(code)
}

// Wire everything together:
// handler := Chain(Recovery(log), Tracing(tracer), Logging(log), RateLimit(1000, 50))(myHandler)
```

### Fatal Issues

**Issue: goroutine leak via unclosed function**

```go
// LEAK: goroutine runs forever, holding references
func startWorker(input <-chan Request) {
    go func() {
        for req := range input {
            process(req)
        }
        // goroutine exits when input is closed — OK
    }()
}
// BUG: if input is never closed, goroutine leaks

// FIX: use context for cancellation
func startWorker(ctx context.Context, input <-chan Request) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return  // exits when context cancelled
            case req, ok := <-input:
                if !ok { return }
                process(req)
            }
        }
    }()
}
```

---

## 9. `return`

### What It Is

`return` exits the current function, optionally returning values to the caller.
In Go, `return` can be used with zero values (bare return in named return funcs),
explicit values, or multiple values.

```go
func nothing() {
    return  // optional in last statement, but explicit is cleaner
}

func sum(a, b int) int {
    return a + b
}

// Named returns + bare return:
func divide(a, b float64) (result float64, err error) {
    if b == 0 {
        err = errors.New("zero divisor")
        return  // bare return: returns current values of result, err
    }
    result = a / b
    return
}
```

### Named Returns: When They Help and When They Hurt

```go
// GOOD USE: complex function with many early returns
// Named returns document what each return value means
func parseConfig(path string) (cfg *Config, err error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("open: %w", err)  // explicit is clearer
    }
    defer f.Close()

    // Bare return + defer modify = the one legitimate pattern:
    defer func() {
        if err != nil {
            err = fmt.Errorf("parseConfig: %w", err)
        }
    }()

    cfg = &Config{}
    if err = json.NewDecoder(f).Decode(cfg); err != nil {
        return  // bare return: cfg=nil, err=decode error (wrapped by defer)
    }
    return  // cfg=loaded config, err=nil
}

// BAD USE: named return hiding logic
func calculate(x int) (result int) {
    result = x * 2   // What does "result" mean here?
    result += 10     // Reader must track "result" through whole function
    return           // Bare return in short function: just write the value!
}

// GOOD: just return the value
func calculate(x int) int {
    return x*2 + 10
}
```

### Fatal Issues

**Issue: defer modifying return value unexpectedly**

```go
// This is a well-known gotcha:
func returnOne() (result int) {
    defer func() { result++ }()  // modifies the named return
    return 1                      // sets result=1, then defer runs → result=2
}
// Returns 2, not 1!

// When using named returns + defer:
// the defer sees the current value of the named return
// AFTER the return statement has set it.

// This feature is occasionally useful (wrapping errors):
func doSomething() (err error) {
    defer func() {
        if err != nil {
            err = fmt.Errorf("doSomething: %w", err)
        }
    }()
    return riskyOperation()
}
// If riskyOperation returns an error, the defer wraps it.
```

---

## 10. `if` / `else`

### What It Is

`if` evaluates a boolean expression and conditionally executes a block.
Go's `if` has a unique feature: an optional initialization statement before
the condition, scoped to the `if`/`else` blocks.

```go
if condition {
    // ...
}

// Init statement — scoped to if/else:
if err := doThing(); err != nil {
    log.Fatal(err)
}
// err is NOT accessible here — scoped to the if block

if x := compute(); x > 0 {
    use(x)
} else {
    useNeg(x)  // x is accessible in else too
}
// x is NOT accessible here
```

### Mental Model: Error Handling as Control Flow

Go uses `if err != nil` instead of exceptions. This is a deliberate design decision.
Understanding WHY changes how you read and write Go.

```
Exception-based (Java/Python):     Go error-as-value:
───────────────────────────────    ──────────────────────────────
try {                              result, err := doA()
    doA();                         if err != nil {
    doB();                             return fmt.Errorf("a: %w", err)
    doC();                         }
} catch (IOException e) {          result2, err := doB(result)
    // Where did this come from?   if err != nil {
    // doA? doB? doC?                  return fmt.Errorf("b: %w", err)
    handle(e);                     }
}                                  result3, err := doC(result2)
                                   if err != nil {
                                       return fmt.Errorf("c: %w", err)
                                   }

// Go: every failure point is explicit.
// Exceptions: failure paths are hidden.
// Go's version is more verbose but more honest.
```

### Real-World Usage: Graceful Degradation in Cloud Services

```go
package service

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "time"
)

// Sentinel errors for structured error handling
var (
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrQuotaExceeded = errors.New("quota exceeded")
    ErrTimeout       = errors.New("timeout")
)

// errors.Is traverses error chains: errors.Is(wrappedErr, ErrNotFound)
type NotFoundError struct {
    Resource string
    ID       string
}
func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}
func (e *NotFoundError) Is(target error) bool {
    return target == ErrNotFound
}

// GetUser demonstrates the layered if/err pattern
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
    // Try cache first
    if user, err := s.cache.Get(ctx, "user:"+userID); err == nil {
        return user.(*User), nil
    }

    // Cache miss or error — try database
    user, err := s.db.QueryUser(ctx, userID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            // Not found is a valid business case, not a system error
            return nil, &NotFoundError{Resource: "user", ID: userID}
        }
        if errors.Is(err, context.DeadlineExceeded) {
            // Database timeout — degrade gracefully
            return nil, fmt.Errorf("GetUser: db timeout: %w", ErrTimeout)
        }
        return nil, fmt.Errorf("GetUser: db error: %w", err)
    }

    // Populate cache asynchronously — don't block on cache errors
    go func() {
        cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        if err := s.cache.Set(cacheCtx, "user:"+userID, user, 5*time.Minute); err != nil {
            s.log.Warn("cache set failed", "user_id", userID, "error", err)
        }
    }()

    return user, nil
}

// HTTP handler converting errors to status codes
func (s *Service) handleGetUser(w http.ResponseWriter, r *http.Request) {
    userID := r.PathValue("userID")

    user, err := s.GetUser(r.Context(), userID)
    if err != nil {
        // Map domain errors to HTTP status codes
        switch {
        case errors.Is(err, ErrNotFound):
            http.Error(w, err.Error(), http.StatusNotFound)
        case errors.Is(err, ErrUnauthorized):
            http.Error(w, "unauthorized", http.StatusUnauthorized)
        case errors.Is(err, ErrTimeout):
            http.Error(w, "service timeout", http.StatusGatewayTimeout)
        case errors.Is(err, ErrQuotaExceeded):
            w.Header().Set("Retry-After", "60")
            http.Error(w, "quota exceeded", http.StatusTooManyRequests)
        default:
            s.log.Error("unexpected error", "error", err)
            http.Error(w, "internal server error", http.StatusInternalServerError)
        }
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}
```

### Common Mistakes

**Mistake 1: else after return is redundant**

```go
// UNNECESSARY else:
if err != nil {
    return err
} else {
    doMoreWork()  // else not needed: if err returned, we never reach here
}

// IDIOMATIC Go (early return / guard clause):
if err != nil {
    return err
}
doMoreWork()
```

**Mistake 2: Accidental assignment in condition**

```go
// Go does NOT allow assignment in if condition (unlike C):
if x = 5 {     // compile error: x = 5 is not boolean
}

// The initialization form IS supported:
if x := getValue(); x > 0 {
    use(x)
}
// Prevents accidental = instead of == from C habits
```

---

## 11. `for`

### What It Is

`for` is Go's **only** looping keyword. It replaces `while`, `do-while`, `for`,
and `foreach` from other languages. This is one of Go's best simplifications.

```go
// Classic for
for i := 0; i < 10; i++ { }

// While-style
for condition { }

// Infinite loop
for { }

// Range-based
for i, v := range slice { }
for k, v := range aMap { }
for i, r := range "string" { }  // i=byte index, r=rune
for v := range channel { }
for range n { }                  // Go 1.22+: loop n times

// Labels for nested loops:
outer:
for i := 0; i < 5; i++ {
    for j := 0; j < 5; j++ {
        if j == 3 { break outer }
    }
}
```

### Internal Mechanics: Range Iteration

```go
// What the compiler generates for range:

// Source:
for i, v := range slice { use(i, v) }

// Compiler generates approximately:
{
    _len := len(slice)
    _ptr := &slice[0]
    for _i := 0; _i < _len; _i++ {
        i, v := _i, *(_ptr + _i)  // v is a COPY
        use(i, v)
    }
}

// CRITICAL: v is a copy of the element, not a reference!
// Modifying v does NOT modify the slice element.
```

### Real-World Usage: Event Loop with Linux epoll

```go
package reactor

import (
    "fmt"
    "log"
    "net"
    "syscall"
    "unsafe"
)

// Reactor pattern using Linux epoll — used by Redis, nginx, Node.js internally
// Go's net package uses this internally, but here we expose the primitives
// for educational purposes (e.g., writing a custom protocol server)

type EventLoop struct {
    epollFD int
    events  []syscall.EpollEvent
    conns   map[int]net.Conn
    done    chan struct{}
}

func NewEventLoop(maxEvents int) (*EventLoop, error) {
    // epoll_create1: Linux's scalable I/O multiplexer
    // O_CLOEXEC: fd closed on exec (prevents fd leak in child processes)
    epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
    if err != nil {
        return nil, fmt.Errorf("epoll_create1: %w", err)
    }
    return &EventLoop{
        epollFD: epfd,
        events:  make([]syscall.EpollEvent, maxEvents),
        conns:   make(map[int]net.Conn),
        done:    make(chan struct{}),
    }, nil
}

func (el *EventLoop) Add(conn net.Conn) error {
    fd := connFD(conn)
    event := syscall.EpollEvent{
        Events: syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLET,
        Fd:     int32(fd),
    }
    if err := syscall.EpollCtl(el.epollFD, syscall.EPOLL_CTL_ADD, fd, &event); err != nil {
        return fmt.Errorf("epoll_ctl ADD fd=%d: %w", fd, err)
    }
    el.conns[fd] = conn
    return nil
}

// Run is the event loop: for { epoll_wait → handle events }
func (el *EventLoop) Run(handler func(net.Conn)) {
    defer syscall.Close(el.epollFD)

    for {
        // epoll_wait: blocks until events ready, or 100ms timeout
        // The for loop is the reactor core — process events as they arrive
        n, err := syscall.EpollWait(el.epollFD, el.events, 100 /*ms*/)
        if err != nil {
            if err == syscall.EINTR {
                continue  // interrupted by signal, retry
            }
            log.Printf("epoll_wait: %v", err)
            return
        }

        // Check if we should stop (non-blocking)
        select {
        case <-el.done:
            return
        default:
        }

        // Process each ready event
        for i := 0; i < n; i++ {
            ev := el.events[i]
            fd := int(ev.Fd)

            conn, ok := el.conns[fd]
            if !ok {
                continue
            }

            if ev.Events&(syscall.EPOLLRDHUP|syscall.EPOLLHUP|syscall.EPOLLERR) != 0 {
                // Connection closed or error
                el.remove(fd)
                conn.Close()
                continue
            }

            if ev.Events&syscall.EPOLLIN != 0 {
                // Data available to read
                handler(conn)
            }
        }
    }
}

func (el *EventLoop) Stop() { close(el.done) }

func (el *EventLoop) remove(fd int) {
    syscall.EpollCtl(el.epollFD, syscall.EPOLL_CTL_DEL, fd, nil)
    delete(el.conns, fd)
}

func connFD(conn net.Conn) int {
    // Extract file descriptor from net.Conn via SyscallConn
    rc, _ := conn.(syscall.Conn)
    var fd int
    raw, _ := rc.SyscallConn()
    raw.Control(func(f uintptr) { fd = int(f) })
    return fd
}
```

### Common Mistakes

**Mistake 1: Modifying slice/map while ranging**

```go
// Deleting from map while ranging: SAFE (Go spec allows it)
for k, v := range m {
    if v == 0 { delete(m, k) }  // safe
}

// Appending to slice while ranging: TRICKY
// The range captures len(slice) at start
s := []int{1, 2, 3}
for i, v := range s {
    s = append(s, v*2)  // doesn't affect the current range iteration
    // s grows but range still runs 3 times
    fmt.Println(i, v)
}
```

**Mistake 2: Range over string iterates runes, not bytes**

```go
s := "café"  // 5 bytes (é is 2 bytes in UTF-8), but 4 runes

for i, b := range []byte(s) {
    fmt.Printf("%d: %x\n", i, b)  // i = 0,1,2,3,4 — byte indices
}

for i, r := range s {
    fmt.Printf("%d: %c\n", i, r)  // i = 0,1,2,3 (byte offset of each rune)
}                                  // NOT 0,1,2,3 by rune count!
// s[3] is the start of 'é', which is bytes 3,4
```

**Mistake 3: for-range variable reuse in Go < 1.22**

```go
// Pre-Go 1.22: i and v are THE SAME variable each iteration
for i, v := range items {
    go func() {
        fmt.Println(i, v)  // captures variable, not value!
    }()
}
// All goroutines print the LAST value of i and v

// Go 1.22 fixed this: each iteration creates new variables.
// But if your go.mod says "go 1.21" or earlier, the old behavior applies.
// Always check your go directive.
```

---

## 12. `range`

### What It Is

`range` is a keyword used exclusively with `for`. It iterates over:
- arrays, slices → (index, value)
- strings → (byte-index, rune)
- maps → (key, value) in **random order**
- channels → values until closed
- integers (Go 1.22+) → 0 to n-1

```go
// Discard index:
for _, v := range slice { }

// Discard value (or use index only):
for i := range slice { }

// Range channel (blocks until closed):
for msg := range ch { process(msg) }

// Range integer (Go 1.22+):
for i := range 5 { fmt.Println(i) }  // 0 1 2 3 4
```

### Internal Mechanics: Map Iteration Randomness

Go deliberately randomizes map iteration order on every run. This prevents programs
from accidentally depending on map ordering, which would be non-portable and fragile.
The Go runtime seeds the iteration start with `runtime.fastrand()` per range operation.

```go
m := map[string]int{"a": 1, "b": 2, "c": 3}

// Run 1: a b c
// Run 2: c a b
// Run 3: b c a
// Completely random, intentional, and different even between two loops on same map

// If you need sorted map iteration:
keys := make([]string, 0, len(m))
for k := range m { keys = append(keys, k) }
sort.Strings(keys)
for _, k := range keys {
    fmt.Println(k, m[k])
}
```

### Real-World Usage: Fan-out Worker Pool

```go
package workerpool

import (
    "context"
    "sync"
)

type Job[T, R any] struct {
    Input  T
    Result R
    Err    error
}

// Pool runs n workers consuming from jobs channel
// Uses range on channel — workers exit when jobs closed
func Pool[T, R any](
    ctx context.Context,
    workers int,
    jobs <-chan T,
    fn func(context.Context, T) (R, error),
) <-chan Job[T, R] {
    results := make(chan Job[T, R], workers)

    var wg sync.WaitGroup
    for range workers { // Go 1.22 syntax
        wg.Add(1)
        go func() {
            defer wg.Done()
            // range channel: blocks waiting for jobs, exits when jobs closed
            for input := range jobs {
                result, err := fn(ctx, input)
                select {
                case results <- Job[T, R]{Input: input, Result: result, Err: err}:
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    // Close results when all workers done
    go func() {
        wg.Wait()
        close(results)
    }()

    return results
}

// Usage: scan S3 bucket and checksum all objects
func checksumBucket(ctx context.Context, store ObjectStore, bucket string) error {
    objects, err := store.ListObjects(ctx, bucket, "")
    if err != nil {
        return err
    }

    jobs := make(chan string, len(objects))
    for _, obj := range objects {
        jobs <- obj.Key
    }
    close(jobs) // workers will drain and exit

    type CheckResult struct{ key, hash string }

    results := Pool(ctx, 10, jobs, func(ctx context.Context, key string) (CheckResult, error) {
        body, err := store.GetObject(ctx, bucket, key)
        if err != nil {
            return CheckResult{}, err
        }
        defer body.Close()
        h := sha256.New()
        if _, err := io.Copy(h, body); err != nil {
            return CheckResult{}, err
        }
        return CheckResult{key: key, hash: fmt.Sprintf("%x", h.Sum(nil))}, nil
    })

    for r := range results { // range on results channel
        if r.Err != nil {
            log.Printf("error checksumming %s: %v", r.Input, r.Err)
            continue
        }
        log.Printf("ok: %s %s", r.Result.key, r.Result.hash)
    }
    return nil
}
```

---

## 13. `switch` / `case` / `default` / `fallthrough`

### What It Is

`switch` is a multi-way branch. Unlike C/Java, Go's switch:
- Does **NOT** fall through by default (no forgotten `break` bugs)
- Can switch on any comparable type
- Can have no switch expression (acts like if/else chain)
- Cases can have multiple values
- Cases can be expressions, not just constants

```go
// Value switch
switch x {
case 1:
    fmt.Println("one")
case 2, 3:          // multiple values
    fmt.Println("two or three")
default:
    fmt.Println("other")
}

// No expression (like if/else):
switch {
case x < 0:  fmt.Println("negative")
case x == 0: fmt.Println("zero")
default:     fmt.Println("positive")
}

// Type switch:
switch v := i.(type) {
case int:    fmt.Printf("int: %d\n", v)
case string: fmt.Printf("string: %q\n", v)
case nil:    fmt.Println("nil")
default:     fmt.Printf("unknown: %T\n", v)
}

// With init statement:
switch err := doThing(); {
case err == nil: fmt.Println("ok")
case errors.Is(err, ErrNotFound): fmt.Println("not found")
default: fmt.Println("error:", err)
}
```

### `fallthrough`: The Explicit Permission

```go
// fallthrough explicitly passes control to next case
// Unlike C: you MUST write fallthrough to fall through
switch n {
case 1:
    fmt.Println("one")
    fallthrough     // explicitly falls to case 2
case 2:
    fmt.Println("at least one")
    // no fallthrough: stops here
case 3:
    fmt.Println("three")
}

// For n=1: prints "one" then "at least one"
// For n=2: prints "at least one"
// For n=3: prints "three"

// IMPORTANT: fallthrough bypasses the case condition check!
// It executes the BODY of the next case regardless of whether
// the next case's value would have matched.
```

### Real-World Usage: Protocol Message Dispatcher

```go
package protocol

import (
    "encoding/binary"
    "fmt"
    "io"
)

// MessageType identifies protocol messages (e.g., custom TCP protocol)
type MessageType uint8

const (
    MsgHeartbeat  MessageType = 0x01
    MsgAuth       MessageType = 0x02
    MsgData       MessageType = 0x03
    MsgAck        MessageType = 0x04
    MsgClose      MessageType = 0x05
    MsgError      MessageType = 0xFF
)

type Message struct {
    Type    MessageType
    Length  uint32
    Payload []byte
}

// Dispatcher: type switch for clean protocol handling
type Dispatcher struct {
    auth    AuthHandler
    data    DataHandler
    metrics MetricsHandler
}

func (d *Dispatcher) Dispatch(msg *Message, conn io.Writer) error {
    switch msg.Type {
    case MsgHeartbeat:
        // Respond with ACK immediately
        return d.sendAck(conn, msg)

    case MsgAuth:
        token, err := d.auth.Authenticate(msg.Payload)
        if err != nil {
            return d.sendError(conn, fmt.Sprintf("auth failed: %v", err))
        }
        return d.sendAck(conn, &Message{Payload: []byte(token)})

    case MsgData:
        // Process data asynchronously
        go func() {
            if err := d.data.Process(msg.Payload); err != nil {
                // can't send error back in goroutine — log it
                d.metrics.RecordError("data_process", err)
            }
        }()
        return d.sendAck(conn, msg)

    case MsgClose:
        return io.EOF  // signal caller to close connection

    case MsgError:
        // Client reported an error — log and continue
        d.metrics.RecordClientError(string(msg.Payload))
        return nil

    default:
        return fmt.Errorf("unknown message type: 0x%02X", msg.Type)
    }
}

// Type switch: handling different concrete types from interface
func processEvent(event interface{}) string {
    switch e := event.(type) {
    case *CreateEvent:
        return fmt.Sprintf("create: resource=%s id=%s", e.Resource, e.ID)
    case *UpdateEvent:
        return fmt.Sprintf("update: resource=%s id=%s fields=%v",
            e.Resource, e.ID, e.UpdatedFields)
    case *DeleteEvent:
        return fmt.Sprintf("delete: resource=%s id=%s", e.Resource, e.ID)
    case *ErrorEvent:
        return fmt.Sprintf("error: code=%d msg=%s", e.Code, e.Message)
    case nil:
        return "nil event"
    default:
        return fmt.Sprintf("unknown event type: %T", e)
    }
}
```

### Misinformation

> "switch is just syntactic sugar for if/else"

**PARTIALLY FALSE.** For expression switches, the compiler can generate jump tables
(O(1) dispatch) for dense integer ranges — far more efficient than O(n) if/else chains.
For type switches, the runtime uses interface type pointers for O(1) dispatch.
The generated assembly can be significantly different.

> "You need break in Go switch cases"

**FALSE.** Cases in Go switch automatically break at the end of each case body.
You only need `break` to exit early from inside a case (or to break from a labeled
outer loop). `fallthrough` is required to continue to the next case.

---

## 14. `break` / `continue`

### What They Are

- `break` exits the innermost `for`, `switch`, or `select`
- `continue` skips to the next iteration of the innermost `for`
- Both support **labels** to control outer loops

```go
// Basic break:
for i := 0; i < 10; i++ {
    if i == 5 { break }
    fmt.Println(i)
}

// Basic continue:
for i := 0; i < 10; i++ {
    if i%2 == 0 { continue }
    fmt.Println(i)  // odd numbers only
}

// Labeled break:
search:
for _, row := range matrix {
    for _, col := range row {
        if col == target {
            found = true
            break search  // exits BOTH loops
        }
    }
}

// Labeled continue (outer loop):
outer:
for i := range rows {
    for j := range cols {
        if skip(i, j) {
            continue outer  // goes to next i, skipping rest of j loop
        }
        process(i, j)
    }
}
```

### Real-World Usage: HTTP Polling with Backoff

```go
package poller

import (
    "context"
    "math"
    "net/http"
    "time"
)

type PollResult struct {
    Data []byte
    Err  error
}

// Poll with exponential backoff — cloud API polling pattern
func PollWithBackoff(ctx context.Context, url string, maxAttempts int) PollResult {
    client := &http.Client{Timeout: 10 * time.Second}
    base := 100 * time.Millisecond

    for attempt := range maxAttempts {
        // Check context before each attempt
        select {
        case <-ctx.Done():
            return PollResult{Err: ctx.Err()}
        default:
        }

        resp, err := client.Get(url)
        if err != nil {
            goto backoff  // error → wait and retry
        }
        defer resp.Body.Close()

        switch resp.StatusCode {
        case http.StatusOK:
            data, err := io.ReadAll(resp.Body)
            return PollResult{Data: data, Err: err}  // success: break out

        case http.StatusTooManyRequests:
            // Rate limited: use Retry-After header if present
            if ra := resp.Header.Get("Retry-After"); ra != "" {
                if secs, err := strconv.Atoi(ra); err == nil {
                    wait := time.Duration(secs) * time.Second
                    select {
                    case <-time.After(wait):
                        continue  // retry without exponential backoff
                    case <-ctx.Done():
                        return PollResult{Err: ctx.Err()}
                    }
                }
            }
            goto backoff

        case http.StatusNotFound,
             http.StatusUnauthorized,
             http.StatusForbidden:
            // Non-retriable errors: break immediately
            return PollResult{
                Err: fmt.Errorf("poll failed with status %d", resp.StatusCode),
            }

        default:
            goto backoff
        }

    backoff:
        if attempt == maxAttempts-1 {
            break  // last attempt: don't wait
        }
        // Exponential backoff with jitter
        wait := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
        jitter := time.Duration(rand.Int63n(int64(wait / 4)))
        select {
        case <-time.After(wait + jitter):
        case <-ctx.Done():
            return PollResult{Err: ctx.Err()}
        }
    }

    return PollResult{Err: fmt.Errorf("max attempts (%d) reached", maxAttempts)}
}
```

---

## 15. `goto`

### What It Is

`goto` unconditionally jumps to a labeled statement within the same function.
Go's `goto` is restricted:
- Cannot jump over variable declarations
- Cannot jump into a block (only to labels at the same or outer scope)
- Label must be in the same function

```go
func scan(data []byte) error {
    i := 0
loop:
    if i >= len(data) {
        goto done
    }
    if !isValid(data[i]) {
        goto errOut
    }
    i++
    goto loop
done:
    return nil
errOut:
    return fmt.Errorf("invalid byte at position %d: 0x%02x", i, data[i])
}
```

### When goto Is Legitimate

```go
// Go stdlib actually uses goto in hot paths (crypto, encoding)
// for state machine performance. Example pattern from real parsers:

func parseHTTPRequest(data []byte) (*Request, error) {
    var req Request
    i := 0

    // State machine with goto — avoids function call overhead
    // in tight parsing loops
parseMethod:
    for i < len(data) && data[i] != ' ' { i++ }
    req.Method = string(data[:i])
    if i >= len(data) { goto malformed }
    i++ // skip space

parseURL:
    start := i
    for i < len(data) && data[i] != ' ' { i++ }
    req.URL = string(data[start:i])
    if i >= len(data) { goto malformed }
    i++ // skip space

parseProto:
    req.Proto = string(data[i:])
    return &req, nil

malformed:
    return nil, fmt.Errorf("malformed HTTP request")
}
```

### Misinformation

> "goto is never acceptable in Go"

**FALSE.** The Go standard library uses `goto` in specific performance-critical
state machines (look at `src/crypto/aes/`, `src/encoding/json/`). The rule is:
use it only when it genuinely clarifies a state machine that would be more complex
with nested loops and flags. It is rare, not forbidden.

---

## 16. `go`

### What It Is

`go` starts a new **goroutine** — a concurrently executing function.
Goroutines are managed by the Go runtime, not the OS directly.
Starting a goroutine is extremely cheap (~2KB stack, a few hundred nanoseconds).

```go
go someFunction()
go someObject.Method()
go func() { /* anonymous */ }()
go func(x int) { fmt.Println(x) }(42)  // pass args immediately
```

### Internal Mechanics: Goroutine Lifecycle

```
go fn()  →  runtime.newproc(fn, args)

                ┌─────────────────────────────────────────┐
                │          Goroutine States                │
                │                                         │
                │  ┌──────┐                               │
                │  │Runnable├──→ P run queue               │
                │  └───┬───┘       ↑                      │
                │      │           │ M picks up            │
                │      ▼           │                      │
                │  ┌─────────┐     │                      │
                │  │ Running ├─────┘                      │
                │  └────┬────┘                            │
                │       │                                 │
                │  ┌────┴────┐   ┌─────────┐             │
                │  │ Waiting │   │  Dead   │             │
                │  │(blocked)│   │(exited) │             │
                │  └─────────┘   └─────────┘             │
                │                                         │
                │  Waiting triggers:                      │
                │  - channel send/recv (no receiver/sender)│
                │  - sync.Mutex.Lock (contended)          │
                │  - time.Sleep                           │
                │  - network I/O (syscall.read/write)     │
                │  - select with no ready case            │
                └─────────────────────────────────────────┘

Cost comparison:
  OS Thread:     ~8MB stack, ~10μs to create, preempted by kernel
  Goroutine:     ~2KB stack, ~0.2μs to create, scheduled by Go runtime
  Goroutine max: millions possible (memory-limited)
```

### Real-World Usage: Kubernetes Controller Pattern

```go
package controller

import (
    "context"
    "log/slog"
    "sync"
    "time"
)

// Controller is the pattern used by Kubernetes controllers, operators
// and cloud resource reconcilers.
// It runs a control loop: observe desired state → compute diff → apply changes

type Controller struct {
    log      *slog.Logger
    queue    *WorkQueue
    store    StateStore
    client   APIClient
    mu       sync.RWMutex
    handlers map[string]Handler
    wg       sync.WaitGroup
}

func (c *Controller) Run(ctx context.Context, workers int) {
    c.log.Info("starting controller", "workers", workers)

    // Start informer goroutine: watches API server for changes
    // and feeds the work queue
    c.wg.Add(1)
    go func() {
        defer c.wg.Done()
        c.runInformer(ctx)
    }()

    // Start worker goroutines: process work queue
    for range workers {
        c.wg.Add(1)
        go func() {
            defer c.wg.Done()
            c.runWorker(ctx)
        }()
    }

    // Start metrics goroutine: emits controller metrics every 30s
    c.wg.Add(1)
    go func() {
        defer c.wg.Done()
        c.runMetrics(ctx)
    }()

    // Wait for shutdown signal
    <-ctx.Done()
    c.log.Info("shutdown signal received, draining queue")

    // Drain: wait for inflight operations to complete
    drainCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    done := make(chan struct{})
    go func() {
        c.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        c.log.Info("controller stopped cleanly")
    case <-drainCtx.Done():
        c.log.Warn("controller drain timeout: some work may be incomplete")
    }
}

func (c *Controller) runInformer(ctx context.Context) {
    // Reconnect loop — cloud APIs can disconnect
    for {
        if err := c.watchResources(ctx); err != nil {
            if ctx.Err() != nil {
                return  // context cancelled, clean shutdown
            }
            c.log.Error("watch error, reconnecting", "error", err)
            select {
            case <-time.After(5 * time.Second):
            case <-ctx.Done():
                return
            }
        }
    }
}

func (c *Controller) runWorker(ctx context.Context) {
    for {
        key, shutdown := c.queue.Get()
        if shutdown {
            return
        }

        if err := c.reconcile(ctx, key.(string)); err != nil {
            c.log.Error("reconcile failed, requeuing", "key", key, "error", err)
            c.queue.AddAfter(key, 5*time.Second)
        }
        c.queue.Done(key)
    }
}

func (c *Controller) reconcile(ctx context.Context, key string) error {
    // Fetch current state
    desired, err := c.store.GetDesired(ctx, key)
    if err != nil {
        return fmt.Errorf("get desired: %w", err)
    }

    actual, err := c.client.GetActual(ctx, key)
    if err != nil && !IsNotFound(err) {
        return fmt.Errorf("get actual: %w", err)
    }

    // Diff and apply
    if IsNotFound(err) {
        return c.client.Create(ctx, desired)
    }
    if needsUpdate(desired, actual) {
        return c.client.Update(ctx, desired)
    }
    return nil // already in sync
}

func (c *Controller) runMetrics(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            c.emitMetrics()
        case <-ctx.Done():
            return
        }
    }
}
```

### Fatal Issues

**Issue 1: goroutine leak — the most common Go production bug**

```
Symptoms:
- Memory grows slowly over hours
- go tool pprof shows goroutines: 50000 (abnormal)
- Each goroutine holds references, preventing GC

Common leak patterns:

1. Channel send with no receiver:
   go func() {
       result := compute()
       ch <- result   // if no one reads ch, goroutine stuck forever
   }()

2. http.Get without timeout (network goroutine leaks):
   go func() {
       resp, _ := http.Get(url)  // hangs if server never responds
       // goroutine stuck in net.(*netFD).Read
   }()

3. Ticker not stopped:
   go func() {
       ticker := time.NewTicker(1*time.Second)
       // forgot: defer ticker.Stop()
       for range ticker.C { doWork() }
       // if goroutine exits another way, ticker still runs
   }()

Detection:
  import _ "net/http/pprof"
  // GET http://localhost:6060/debug/pprof/goroutine?debug=2
```

**Issue 2: go in a loop without proper variable capture (pre-1.22)**

```go
for _, item := range items {
    go func() {
        process(item)  // item is the LOOP VARIABLE — race condition!
        // All goroutines may see the final value of item
    }()
}

// Go 1.22 fix (if go.mod says go 1.22+): each iteration creates new item
// Pre-1.22 fix:
for _, item := range items {
    item := item  // shadow: new variable per iteration
    go func() {
        process(item)  // this item is captured correctly
    }()
}
```

---

## 17. `chan`

### What It Is

`chan` declares a channel type — a typed conduit for communication between goroutines.
Channels implement CSP (Communicating Sequential Processes) semantics.

```go
ch := make(chan int)          // unbuffered: sync handoff
ch := make(chan int, 10)      // buffered: async up to 10
ch := make(chan struct{})      // signal channel (zero size)

var ch <-chan int              // receive-only channel type
var ch chan<- int              // send-only channel type

// Channel operations:
ch <- value     // send (blocks if full/no receiver)
v := <-ch       // receive (blocks if empty/no sender)
v, ok := <-ch   // receive with closed check (ok=false if closed+empty)
close(ch)       // close: signals no more sends; receivers drain then get zero
len(ch)         // current buffered items
cap(ch)         // buffer capacity
```

### Internal Mechanics: hchan Structure

```
make(chan T, n) allocates an hchan:

┌────────────────────────────────────────────────────────────┐
│                     hchan struct                           │
├────────────────────────────────────────────────────────────┤
│  qcount    uint    → current items in buffer               │
│  dataqsiz  uint    → buffer capacity (n)                   │
│  buf       unsafe.Pointer → ring buffer [n]T               │
│  elemsize  uint16  → sizeof(T)                             │
│  closed    uint32  → 1 if closed                           │
│  sendx     uint    → next send index in ring buffer        │
│  recvx     uint    → next recv index in ring buffer        │
│  recvq     waitq   → goroutines blocked on receive         │
│  sendq     waitq   → goroutines blocked on send            │
│  lock      mutex   → protects all fields                   │
└────────────────────────────────────────────────────────────┘

Ring buffer for buffered channel (cap=3):
                      recvx          sendx
                        │              │
                        ▼              ▼
buf: [ item0 | item1 | item2 | empty | empty | ... ]
       ↑ already received    ↑ waiting to be received

Goroutine blocking:
  SEND on full chan:  G added to sendq, G.status = Gwaiting, M picks up next G
  RECV on empty chan: G added to recvq, G.status = Gwaiting, M picks up next G
  When space/item available: matching G is made Grunnable
```

### Channel Axioms (Must Memorize)

```
Operation       nil chan    open chan     closed chan
─────────────   ─────────   ─────────     ───────────
Send            blocks      works         PANIC
Receive         blocks      works         zero value immediately
Close           PANIC       works         PANIC (double close)
Range           blocks      iterates      stops when empty

Rules:
1. Only the sender should close a channel
2. Never close from the receiver
3. Only close when you're SURE no more sends will happen
4. A closed channel always returns zero values with ok=false
5. Reading from nil channel blocks forever (useful in select)
6. len(ch) is unreliable for synchronization — use select instead
```

### Real-World Usage: Pipeline Pattern for ETL in Cloud

```go
package pipeline

import (
    "context"
    "fmt"
    "io"
    "log"
)

// Pipeline: GCS → Parse → Validate → Transform → BigQuery
// Each stage is a goroutine communicating via channels

type Record struct {
    ID     string
    Fields map[string]interface{}
    Source string
    Line   int
}

// Stage 1: read raw objects from GCS
func readFromGCS(ctx context.Context, store ObjectStore, bucket, prefix string,
) <-chan []byte {
    out := make(chan []byte, 100) // buffered: decouple I/O from parsing
    go func() {
        defer close(out) // always close: signals downstream
        objects, err := store.ListObjects(ctx, bucket, prefix)
        if err != nil {
            log.Printf("list objects: %v", err)
            return
        }
        for _, obj := range objects {
            body, err := store.GetObject(ctx, bucket, obj.Key)
            if err != nil {
                log.Printf("get object %s: %v", obj.Key, err)
                continue // skip failed objects, don't stop pipeline
            }
            data, _ := io.ReadAll(body)
            body.Close()
            select {
            case out <- data:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}

// Stage 2: parse raw bytes into Records
func parseRecords(ctx context.Context, in <-chan []byte) <-chan Record {
    out := make(chan Record, 50)
    go func() {
        defer close(out)
        for data := range in { // range blocks until in is closed or item available
            records, err := parseCSV(data)
            if err != nil {
                log.Printf("parse error: %v", err)
                continue
            }
            for _, r := range records {
                select {
                case out <- r:
                case <-ctx.Done():
                    return
                }
            }
        }
    }()
    return out
}

// Stage 3: validate
func validateRecords(ctx context.Context, in <-chan Record) (<-chan Record, <-chan error) {
    out := make(chan Record, 50)
    errs := make(chan error, 100) // separate error channel
    go func() {
        defer close(out)
        defer close(errs)
        for r := range in {
            if err := validate(r); err != nil {
                select {
                case errs <- fmt.Errorf("record %s: %w", r.ID, err):
                default: // non-blocking: don't let errors slow pipeline
                }
                continue
            }
            select {
            case out <- r:
            case <-ctx.Done():
                return
            }
        }
    }()
    return out, errs
}

// Stage 4: batch insert into BigQuery
func insertToBigQuery(ctx context.Context, in <-chan Record, bqClient BQClient,
    batchSize int) <-chan error {
    errs := make(chan error, 10)
    go func() {
        defer close(errs)
        batch := make([]Record, 0, batchSize)
        flush := func() {
            if len(batch) == 0 { return }
            if err := bqClient.Insert(ctx, batch); err != nil {
                select {
                case errs <- fmt.Errorf("bq insert batch: %w", err):
                default:
                }
            }
            batch = batch[:0]
        }
        for r := range in {
            batch = append(batch, r)
            if len(batch) >= batchSize { flush() }
        }
        flush() // flush remaining
    }()
    return errs
}

// Wire the pipeline:
func RunPipeline(ctx context.Context, store ObjectStore, bqClient BQClient,
    bucket, prefix string) error {
    raw := readFromGCS(ctx, store, bucket, prefix)
    parsed := parseRecords(ctx, raw)
    valid, parseErrs := validateRecords(ctx, parsed)
    insertErrs := insertToBigQuery(ctx, valid, bqClient, 500)

    // Drain error channels
    var errCount int
    for {
        select {
        case err, ok := <-parseErrs:
            if !ok { parseErrs = nil; continue }
            log.Printf("validation error: %v", err)
            errCount++
        case err, ok := <-insertErrs:
            if !ok { insertErrs = nil; continue }
            log.Printf("insert error: %v", err)
            errCount++
        }
        if parseErrs == nil && insertErrs == nil { break }
    }

    if errCount > 0 {
        return fmt.Errorf("pipeline completed with %d errors", errCount)
    }
    return nil
}
```

### Common Mistakes

**Mistake 1: Closing channel from receiver**

```go
// WRONG:
func consumer(ch <-chan int) {
    for v := range ch {
        if v == 0 { close(ch) }  // compile error: cannot close receive-only chan
    }
}
// Even if writable: closing from consumer can race with sender
// and cause panic in the sender goroutine.

// RULE: Only the goroutine that WRITES closes the channel.
```

**Mistake 2: Reading from closed channel without ok check**

```go
ch := make(chan int, 1)
ch <- 42
close(ch)

v := <-ch   // 42 (buffered data drains first)
v = <-ch    // 0  (channel empty+closed: zero value, NO panic)
v, ok := <-ch // ok=false signals closure

// Don't confuse with nil channel:
var nilch chan int
v = <-nilch  // blocks forever (never returns)
```

**Mistake 3: Using channel as mutex (wrong tool)**

```go
// WRONG: using 1-buffered channel as mutex
lock := make(chan struct{}, 1)
lock <- struct{}{}  // "lock"
// ... critical section
<-lock              // "unlock"

// This works but:
// 1. It's slower than sync.Mutex
// 2. Not composable (can't RLock, TryLock, etc.)
// 3. Harder to read
// USE sync.Mutex for mutual exclusion.
// USE channels for communication and signaling.
```

---

## 18. `select`

### What It Is

`select` is like a `switch` for channel operations. It blocks until one of its cases
can proceed, then executes that case. If multiple cases are ready simultaneously,
one is chosen **at random** (uniform distribution).

```go
select {
case v := <-ch1:
    use(v)
case ch2 <- val:
    fmt.Println("sent")
case <-time.After(5 * time.Second):
    fmt.Println("timeout")
case <-ctx.Done():
    return ctx.Err()
default:
    // non-blocking: execute if no case is ready
}
```

### Internal Mechanics: How select Works

```
select implementation (runtime/select.go):

1. All cases evaluated (channel and value expressions)
2. Lock ALL channels involved (in address order to prevent deadlock)
3. Check each case for readiness:
   - Is channel ready for send/recv?
   - If any ready: pick one at random, execute, unlock all, done.
   - If default: execute default, unlock all, done.
4. If no case ready and no default:
   - Add current goroutine to each channel's send/recv queue
   - Block goroutine (Gwaiting)
   - When any channel becomes ready, goroutine wakes
   - Execute the case that unblocked it

Randomness:
  Multiple ready cases → random selection
  Uses runtime.fastrand() — not crypto-secure
  Purpose: prevent starvation (no case is always preferred)
```

### Real-World Usage: Circuit Breaker

```go
package breaker

import (
    "context"
    "errors"
    "sync"
    "sync/atomic"
    "time"
)

// Circuit Breaker — prevents cascading failures in microservices
// States: Closed (normal) → Open (failing) → Half-Open (testing)

type State int32

const (
    StateClosed   State = iota // requests pass through
    StateOpen                   // requests fail immediately (fast fail)
    StateHalfOpen              // one request passes to test recovery
)

type CircuitBreaker struct {
    maxFailures  int64
    resetTimeout time.Duration

    failures  atomic.Int64
    successes atomic.Int64
    state     atomic.Int32
    openedAt  atomic.Int64 // unix nano

    mu   sync.Mutex
    done chan struct{}
}

var ErrCircuitOpen = errors.New("circuit breaker open")

func New(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
    cb := &CircuitBreaker{
        maxFailures:  int64(maxFailures),
        resetTimeout: resetTimeout,
        done:         make(chan struct{}),
    }
    go cb.monitor()
    return cb
}

// Do executes fn through the circuit breaker
// select is used to implement timeout + cancellation
func (cb *CircuitBreaker) Do(ctx context.Context,
    fn func(context.Context) error) error {

    // Check state before executing
    switch State(cb.state.Load()) {
    case StateOpen:
        // Check if reset timeout elapsed
        openedAt := time.Unix(0, cb.openedAt.Load())
        if time.Since(openedAt) < cb.resetTimeout {
            return ErrCircuitOpen // fast fail
        }
        // Try half-open: transition atomically
        if cb.state.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
            cb.log("→ half-open")
        }

    case StateHalfOpen:
        return ErrCircuitOpen // only one request gets through
    }

    // Execute with timeout via select
    type result struct{ err error }
    resultCh := make(chan result, 1)

    go func() {
        resultCh <- result{err: fn(ctx)}
    }()

    select {
    case r := <-resultCh:
        cb.record(r.err)
        return r.err

    case <-ctx.Done():
        // Context cancelled: don't record as failure (client cancelled, not server error)
        return ctx.Err()

    case <-time.After(30 * time.Second):
        // Hard timeout beyond context
        cb.record(errors.New("timeout"))
        return errors.New("circuit breaker timeout")
    }
}

func (cb *CircuitBreaker) record(err error) {
    if err == nil {
        cb.successes.Add(1)
        cb.failures.Store(0)
        if State(cb.state.Load()) == StateHalfOpen {
            cb.state.Store(int32(StateClosed))
            cb.log("→ closed (recovered)")
        }
        return
    }

    count := cb.failures.Add(1)
    if count >= cb.maxFailures && State(cb.state.Load()) == StateClosed {
        cb.state.Store(int32(StateOpen))
        cb.openedAt.Store(time.Now().UnixNano())
        cb.log("→ open (too many failures)")
    }
}

// monitor uses select to reset failure count on timer
func (cb *CircuitBreaker) monitor() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // Decay failure count over time (leaky bucket)
            if f := cb.failures.Load(); f > 0 {
                cb.failures.CompareAndSwap(f, f/2)
            }
        case <-cb.done:
            return
        }
    }
}

func (cb *CircuitBreaker) Close() { close(cb.done) }
func (cb *CircuitBreaker) log(msg string) { /* structured log */ }
```

### select nil Channel Trick

```go
// Disabling a case in select by setting channel to nil:
// A nil channel in select is NEVER ready — effectively disables that case

func mergeChannels(ctx context.Context, ch1, ch2 <-chan int) <-chan int {
    out := make(chan int, 10)
    go func() {
        defer close(out)
        for ch1 != nil || ch2 != nil {
            select {
            case v, ok := <-ch1:
                if !ok {
                    ch1 = nil  // disable this case: ch1 exhausted
                    continue
                }
                out <- v

            case v, ok := <-ch2:
                if !ok {
                    ch2 = nil  // disable this case: ch2 exhausted
                    continue
                }
                out <- v

            case <-ctx.Done():
                return
            }
        }
    }()
    return out
}
// When ch1 closes, ch1 = nil disables that select case.
// Loop continues until BOTH are nil.
```

---

## 19. `defer`

### What It Is

`defer` schedules a function call to run when the surrounding function returns —
whether normally, via explicit `return`, or due to `panic`. Deferred calls run
in **LIFO** (last-in, first-out) order.

```go
func example() {
    defer fmt.Println("third")   // runs last
    defer fmt.Println("second")  // runs second
    defer fmt.Println("first")   // runs first
    fmt.Println("during")
}
// Output: during, first, second, third
```

### Internal Mechanics: defer Stack

```
Each goroutine has a defer list (a linked list of _defer structs):

goroutine G:
┌────────────────────────────────────────────────────────┐
│  _defer stack (LIFO)                                   │
│                                                        │
│  defer fn3() → defer fn2() → defer fn1() → nil        │
│  (newest)                              (oldest)        │
│                                                        │
│  On function return (normal or panic):                 │
│  1. Pop fn3 → execute → pop fn2 → execute → pop fn1   │
│  2. If panic: each defer runs, if any calls recover()  │
│     and returns normally, panic is stopped             │
└────────────────────────────────────────────────────────┘

defer argument evaluation:
  defer fn(expr)
  expr is evaluated IMMEDIATELY when defer is reached,
  not when the deferred function runs.

  x := 10
  defer fmt.Println(x)  // x evaluated NOW: prints 10
  x = 20                // too late, defer already captured x=10
```

### defer in Loops: A Critical Trap

```go
// WRONG: defers accumulate, resources not released until function returns
func processFiles(paths []string) error {
    for _, path := range paths {
        f, err := os.Open(path)
        if err != nil { return err }
        defer f.Close()  // ALL files deferred until processFiles returns!
        // If paths has 10,000 items: 10,000 open file descriptors!
    }
    return nil
}

// CORRECT: use a closure or inner function
func processFiles(paths []string) error {
    for _, path := range paths {
        if err := processFile(path); err != nil {
            return err
        }
    }
    return nil
}

func processFile(path string) error {
    f, err := os.Open(path)
    if err != nil { return err }
    defer f.Close()  // deferred within processFile scope — released each iteration
    // process f
    return nil
}

// OR: use anonymous function
func processFiles(paths []string) error {
    for _, path := range paths {
        err := func() error {
            f, err := os.Open(path)
            if err != nil { return err }
            defer f.Close()  // released at end of anonymous func, each iteration
            return process(f)
        }()
        if err != nil { return err }
    }
    return nil
}
```

### Real-World Usage: Lock/Unlock, Connection Management, Linux file ops

```go
package resource

import (
    "context"
    "fmt"
    "os"
    "syscall"
    "time"
)

// Mutex with defer — the canonical pattern
func (s *Store) updateRecord(key string, fn func(*Record)) error {
    s.mu.Lock()
    defer s.mu.Unlock()   // ALWAYS unlocks: on return OR panic
    r, ok := s.records[key]
    if !ok {
        return fmt.Errorf("record %q not found", key)
    }
    fn(r)
    return nil
}

// File operations with proper cleanup
func writeAtomically(path string, data []byte) (err error) {
    // Write to temp file, then rename for atomicity (POSIX guarantee)
    tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
    if err != nil {
        return fmt.Errorf("create temp: %w", err)
    }
    tmpPath := tmp.Name()

    // Deferred cleanup: runs even on panic
    defer func() {
        tmp.Close()
        if err != nil {
            // If we failed, remove the temp file
            os.Remove(tmpPath)
        }
    }()

    // Set permissions before writing
    if err = tmp.Chmod(0644); err != nil {
        return fmt.Errorf("chmod: %w", err)
    }

    if _, err = tmp.Write(data); err != nil {
        return fmt.Errorf("write: %w", err)
    }

    // Sync to disk before rename (data durability)
    if err = tmp.Sync(); err != nil {
        return fmt.Errorf("sync: %w", err)
    }

    if err = tmp.Close(); err != nil {
        return fmt.Errorf("close: %w", err)
    }

    // Atomic rename: either old or new, never partial
    if err = os.Rename(tmpPath, path); err != nil {
        return fmt.Errorf("rename: %w", err)
    }
    return nil
}

// Linux file lock with defer
func withFileLock(path string, exclusive bool, fn func() error) error {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
    if err != nil {
        return fmt.Errorf("open lock file: %w", err)
    }
    defer f.Close()

    how := syscall.LOCK_SH
    if exclusive { how = syscall.LOCK_EX }

    // flock: Linux advisory lock
    if err := syscall.Flock(int(f.Fd()), how); err != nil {
        return fmt.Errorf("flock acquire: %w", err)
    }
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) // always release

    return fn()
}

// Span/trace timing with defer
func doOperationWithTracing(ctx context.Context, name string) (err error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        // err is the named return: defer sees final value after return
        status := "ok"
        if err != nil { status = "error" }
        metrics.Record(name, duration, status)
    }()

    return actualOperation(ctx)
}
```

### Misinformation

> "defer has no performance cost"

**FALSE.** defer has measurable overhead:
- Pre-Go 1.14: every defer allocated a `_defer` struct on the heap (~100ns)
- Go 1.14+: open-coded defers (compiler inlines simple defers): ~1-3ns overhead
- defer in loops or under certain conditions still allocates heap structs
- For hot paths called millions of times, measure before using defer

> "defer runs when the goroutine exits"

**FALSE.** defer runs when the **function** that contains it returns — not the goroutine.
If you need cleanup when a goroutine exits, put the goroutine's code in a named function
or use a top-level defer in the goroutine's function body.

---

## 20. `map`

### What It Is

`map` declares a hash map type — an unordered collection of key-value pairs.
Maps must be initialized before use. The zero value (nil) supports reads (returns zero)
but panics on writes.

```go
m := make(map[string]int)         // empty map
m := map[string]int{"a": 1}       // map literal with initial values
m := make(map[string]int, 1000)   // hint: pre-allocate buckets for ~1000 items

// Operations:
m["key"] = 42          // write
v := m["key"]          // read (zero value if not present)
v, ok := m["key"]      // read with existence check
delete(m, "key")       // delete (no error if missing)
len(m)                 // count of key-value pairs
```

### Internal Mechanics: hmap Structure

```
Go map is a hash map with chaining (not open addressing):

hmap struct:
┌───────────────────────────────────────────────────┐
│  count     int        → number of elements         │
│  flags     uint8      → state flags                │
│  B         uint8      → log2(num buckets)          │
│  noverflow uint16     → approx overflow bucket cnt │
│  hash0     uint32     → hash seed (random per map) │
│  buckets   unsafe.Ptr → array of 2^B bmap          │
│  oldbuckets unsafe.Ptr → during map growth         │
│  nevacuate uintptr    → evacuation progress        │
└───────────────────────────────────────────────────┘

Each bucket (bmap) holds 8 key-value pairs:
┌─────────────────────────────────────────────────────┐
│  tophash [8]uint8   → top 8 bits of hash per slot  │
│  keys    [8]K       → 8 keys                        │
│  values  [8]V       → 8 values                      │
│  overflow *bmap     → pointer to overflow bucket    │
└─────────────────────────────────────────────────────┘

Lookup: hash(key) → bucket index → tophash scan → key compare
Load factor trigger for growth: 6.5 items/bucket average
Map grows by 2x → incremental evacuation over multiple writes
Random hash seed: prevents hash-collision DoS attacks
```

### Real-World Usage: LRU Cache for API Responses

```go
package cache

import (
    "container/list"
    "sync"
    "time"
)

type entry struct {
    key       string
    value     interface{}
    expiresAt time.Time
    elem      *list.Element
}

// LRUCache: bounded cache with TTL eviction
// Used for API response caching, DNS cache, connection pools
type LRUCache struct {
    mu       sync.RWMutex
    items    map[string]*entry  // map: O(1) lookup
    evict    *list.List         // LRU ordering: O(1) move to front/back
    capacity int
    ttl      time.Duration
}

func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
    return &LRUCache{
        items:    make(map[string]*entry, capacity),
        evict:    list.New(),
        capacity: capacity,
        ttl:      ttl,
    }
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    e, ok := c.items[key]
    c.mu.RUnlock()

    if !ok {
        return nil, false
    }

    // Check TTL
    if time.Now().After(e.expiresAt) {
        c.mu.Lock()
        delete(c.items, key)
        c.evict.Remove(e.elem)
        c.mu.Unlock()
        return nil, false
    }

    // Move to front (most recently used)
    c.mu.Lock()
    c.evict.MoveToFront(e.elem)
    c.mu.Unlock()

    return e.value, true
}

func (c *LRUCache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Update existing
    if e, ok := c.items[key]; ok {
        e.value = value
        e.expiresAt = time.Now().Add(c.ttl)
        c.evict.MoveToFront(e.elem)
        return
    }

    // Evict LRU if at capacity
    if len(c.items) >= c.capacity {
        oldest := c.evict.Back()
        if oldest != nil {
            e := oldest.Value.(*entry)
            delete(c.items, e.key)
            c.evict.Remove(oldest)
        }
    }

    // Insert new
    e := &entry{
        key:       key,
        value:     value,
        expiresAt: time.Now().Add(c.ttl),
    }
    e.elem = c.evict.PushFront(e)
    c.items[key] = e
}

func (c *LRUCache) Delete(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if e, ok := c.items[key]; ok {
        delete(c.items, key)
        c.evict.Remove(e.elem)
    }
}

// Stats for monitoring
func (c *LRUCache) Stats() map[string]int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return map[string]int{
        "size":     len(c.items),
        "capacity": c.capacity,
    }
}
```

### Common Mistakes

**Mistake 1: Concurrent map access (DATA RACE — fatal in production)**

```go
// FATAL: concurrent reads+writes on same map → DATA RACE → crash
var m = make(map[string]int)

go func() { m["a"] = 1 }()  // writer goroutine
go func() { _ = m["a"] }()  // reader goroutine → CONCURRENT READ+WRITE → panic

// Go 1.6+ detects concurrent map access and panics with:
// "concurrent map read and map write"

// FIX 1: sync.RWMutex
var mu sync.RWMutex
var m = make(map[string]int)
// write: mu.Lock(); m["a"] = 1; mu.Unlock()
// read:  mu.RLock(); v := m["a"]; mu.RUnlock()

// FIX 2: sync.Map (built-in concurrent map, optimized for certain patterns)
var m sync.Map
m.Store("a", 1)
v, ok := m.Load("a")
m.Delete("a")
m.Range(func(k, v interface{}) bool {
    // iterate; return false to stop
    return true
})
// sync.Map is optimized for: read-heavy, write-once-read-many
// For write-heavy: use sharded maps or RWMutex
```

**Mistake 2: Using map value as receiver for struct methods**

```go
type Counter struct{ count int }

counters := map[string]Counter{"a": {0}}

// WRONG:
counters["a"].count++  // compile error: cannot take address of map element

// CORRECT:
c := counters["a"]
c.count++
counters["a"] = c

// OR use pointer values:
counters2 := map[string]*Counter{"a": {0}}
counters2["a"].count++  // works: pointer is addressable
```

**Mistake 3: map growth invalidates iteration**

```go
// DO NOT add keys to a map while ranging it and expect consistent behavior
m := map[string]int{"a": 1, "b": 2}
for k, v := range m {
    m[k+"_copy"] = v  // modifying map during range
    // Go spec: new keys may or may not appear in the range
    // existing keys may disappear — spec only guarantees each key seen ≤ once
}
```

---

## Fatal Patterns: The Danger Matrix

```
╔══════════════════════════════════════════════════════════════════════════╗
║                    FATAL PATTERNS REFERENCE                              ║
╠══════════════╦═══════════════════════════════════╦════════════════════╣
║ Pattern      ║ Trigger                           ║ Symptom            ║
╠══════════════╬═══════════════════════════════════╬════════════════════╣
║ nil map write║ var m map[K]V; m[k]=v             ║ panic at runtime   ║
║ nil ptr deref║ var p *T; p.Field                 ║ panic at runtime   ║
║ closed chan  ║ close(ch); ch <- v                ║ panic at runtime   ║
║ double close ║ close(ch); close(ch)              ║ panic at runtime   ║
║ nil chan send║ var ch chan T; ch <- v             ║ goroutine blocks ∞ ║
║ map data race║ concurrent rw without sync        ║ crash + corruption ║
║ nil iface    ║ (*T)(nil) returned as error       ║ logic bug          ║
║ defer in loop║ defer f.Close() in for{}           ║ fd/resource leak   ║
║ goroutine leak║ go func blocking forever          ║ OOM, slowdown      ║
║ slice out of ║ s[i] where i>=len(s)              ║ panic at runtime   ║
║ bounds       ║                                   ║                    ║
║ integer ovfl ║ var u uint8 = 255; u++            ║ wraps to 0         ║
║ stack blow   ║ infinite recursion                ║ stack overflow     ║
╚══════════════╩═══════════════════════════════════╩════════════════════╝
```

### Panic vs Fatal: Understanding the Difference

```
panic("message")
  → Unwinds stack of CURRENT goroutine
  → Runs all deferred functions in that goroutine
  → If recover() in a deferred func: panic stopped, program continues
  → If no recover(): program crashes with stack trace

recover()
  → Only meaningful inside a deferred function
  → Returns the panic value; returns nil if no panic
  → CANNOT recover from another goroutine's panic

log.Fatal(...)
  → Calls os.Exit(1) immediately
  → Deferred functions DO NOT RUN
  → Use only in main() for fatal startup errors

os.Exit(code)
  → Immediate termination
  → Deferred functions DO NOT RUN
  → Buffered I/O NOT flushed
  → NEVER use in libraries

runtime.Goexit()
  → Terminates current goroutine only
  → Runs deferred functions in that goroutine
  → Other goroutines continue
  → Used in testing.T.Fatal()
```

```go
// Correct panic recovery pattern:
func safeHandler(h http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if rec := recover(); rec != nil {
                // Log full stack trace
                buf := make([]byte, 64<<10)
                n := runtime.Stack(buf, false)
                log.Printf("panic: %v\n%s", rec, buf[:n])
                http.Error(w, "internal server error", 500)
            }
        }()
        h(w, r)
    }
}
```

---

## Misinformation Hall of Fame

### 1. "Goroutines are green threads"

**Partially misleading.** Goroutines are multiplexed M:N onto OS threads. True green
threads are always 1:N (one OS thread). Goroutines can run in parallel on multiple
CPU cores. The runtime can park and resume goroutines across different OS threads
(work stealing). They are more powerful than traditional green threads.

### 2. "Go doesn't have generics"

**Was true until Go 1.18 (2022).** Go now has full parametric polymorphism.
Type parameters work on functions, methods (via type), and types:

```go
func Map[T, U any](s []T, fn func(T) U) []U {
    result := make([]U, len(s))
    for i, v := range s {
        result[i] = fn(v)
    }
    return result
}
```

### 3. "defer is always safe to use for cleanup"

**FALSE.** As shown: defer in loops accumulates. defer with named returns can modify
the return value unexpectedly. defer has measurable overhead in tight loops.
defer with os.Exit or log.Fatal doesn't run.

### 4. "Go garbage collector stops the world significantly"

**No longer true (since ~Go 1.5).** Go's GC is concurrent (tri-color mark-and-sweep).
Stop-the-world pauses are typically < 1ms. The GC runs mostly concurrently with
your goroutines. For most applications, GC is not the bottleneck.

### 5. "Channels are always better than mutexes"

**FALSE. Wrong tool for wrong job:**

```
Use channels when:           Use mutexes when:
──────────────────────       ──────────────────────────
Transferring data ownership  Protecting shared state
Signaling events             Counter increments
Pipeline patterns            Cache protection
Goroutine coordination       Connection pool management
Fan-out/fan-in               Simple struct field access

"Do not communicate by sharing memory;
 instead, share memory by communicating."
 — Go proverb

The proverb describes a PREFERENCE for certain patterns,
not a prohibition on mutexes. sync.Mutex is faster and
simpler than channels for pure mutual exclusion.
```

### 6. "interface{} / any means Go is dynamically typed"

**FALSE.** Type information is always preserved in the interface's type pointer.
Runtime type assertions are type-safe. If the asserted type doesn't match, you get
a panic (or false in the `value, ok` form) — not silent type coercion.

### 7. "Go doesn't support OOP"

**FALSE — it supports a different model.** Go has:
- Encapsulation (exported/unexported)
- Methods on types
- Composition via embedding
- Polymorphism via interfaces

It does NOT have:
- Inheritance (by design — prefer composition)
- Method overriding
- Constructors (use `New*` functions by convention)
- Operator overloading

### 8. "Make and new are interchangeable"

**FALSE:**

```go
// new(T): allocates zeroed T, returns *T
// Used for types where zero value is useful (sync.Mutex, sync.WaitGroup)
p := new(sync.Mutex)  // *sync.Mutex, zero value = unlocked

// make(T, ...): allocates AND initializes reference types
// Only for: slice, map, channel
// Returns T (not *T), because T itself is already a reference
s := make([]int, 10, 100)        // slice: len=10, cap=100
m := make(map[string]int)        // initialized, ready to use
ch := make(chan struct{}, 10)    // buffered channel

// new of a map is almost never what you want:
mp := new(map[string]int)   // *map[string]int — pointer to nil map
(*mp)["a"] = 1              // panic: assignment to nil map!
```

---

## Cloud & Linux Kernel Integration Patterns

### Complete Production-Ready HTTP Server

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "net"
    "net/http"
    "os"
    "os/signal"
    "runtime"
    "syscall"
    "time"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Linux: Set process title for better visibility in ps/top
    // (requires prctl syscall, Go doesn't expose it directly — use setproctitle)

    // Set GOMAXPROCS based on cgroup CPU quota (for containers)
    // In Kubernetes: default GOMAXPROCS = number of host CPUs, NOT pod CPU limit
    // Use automaxprocs: import _ "go.uber.org/automaxprocs"
    logger.Info("runtime info",
        "gomaxprocs", runtime.GOMAXPROCS(0),
        "goversion", runtime.Version(),
        "goos", runtime.GOOS,
        "goarch", runtime.GOARCH,
    )

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, `{"status":"ok"}`)
    })

    // Build listener with Linux socket options
    lc := net.ListenConfig{
        Control: func(network, address string, c syscall.RawConn) error {
            return c.Control(func(fd uintptr) {
                // SO_REUSEPORT: multiple listeners on same port (kernel load balancing)
                syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEPORT, 1)
                // TCP_NODELAY: disable Nagle's algorithm for low-latency APIs
                syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
            })
        },
        KeepAlive: 30 * time.Second, // Enable TCP keepalive
    }

    addr := envOrDefault("LISTEN_ADDR", ":8080")
    ln, err := lc.Listen(context.Background(), "tcp", addr)
    if err != nil {
        logger.Error("listen failed", "addr", addr, "error", err)
        os.Exit(1)
    }

    srv := &http.Server{
        Handler:           mux,
        ReadTimeout:       10 * time.Second,
        ReadHeaderTimeout: 5 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       60 * time.Second, // keep-alive timeout
        MaxHeaderBytes:    1 << 20,          // 1MB
    }

    // Graceful shutdown:
    // Kubernetes sends SIGTERM → pod gets 30s to drain connections
    // SIGINT = Ctrl+C in development

    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGTERM,
        syscall.SIGINT,
    )
    defer stop()

    // Start server in goroutine
    errCh := make(chan error, 1)
    go func() {
        logger.Info("server starting", "addr", addr)
        if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
        close(errCh)
    }()

    // Wait for signal or server error
    select {
    case <-ctx.Done():
        logger.Info("shutdown signal received")
    case err := <-errCh:
        if err != nil {
            logger.Error("server error", "error", err)
            os.Exit(1)
        }
        return
    }

    // Graceful shutdown: wait up to 25s for inflight requests
    // (5s margin for Kubernetes terminationGracePeriodSeconds=30)
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
    defer cancel()

    logger.Info("draining connections")
    if err := srv.Shutdown(shutdownCtx); err != nil {
        logger.Error("shutdown error", "error", err)
    }
    logger.Info("server stopped")
}

func envOrDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

### Memory-Mapped File Processing (Linux mmap)

```go
package mmap

import (
    "fmt"
    "os"
    "syscall"
    "unsafe"
)

// MMapFile wraps a memory-mapped file.
// Used for: log file processing, database storage engines,
// large file search (grep-like tools), shared memory between processes

type MMapFile struct {
    f    *os.File
    data []byte
    size int64
}

func Open(path string) (*MMapFile, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, fmt.Errorf("open: %w", err)
    }

    fi, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("stat: %w", err)
    }
    size := fi.Size()
    if size == 0 {
        f.Close()
        return &MMapFile{f: f, size: 0}, nil
    }

    // mmap: map file into process address space
    // PROT_READ: read-only
    // MAP_SHARED: changes visible to other processes mapping same file
    // MAP_PRIVATE: copy-on-write, changes not written to file
    data, err := syscall.Mmap(
        int(f.Fd()),
        0,             // offset: start of file
        int(size),
        syscall.PROT_READ,
        syscall.MAP_SHARED,
    )
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("mmap: %w", err)
    }

    return &MMapFile{f: f, data: data, size: size}, nil
}

func (m *MMapFile) Bytes() []byte { return m.data }
func (m *MMapFile) Size() int64   { return m.size }

// CountLines counts newlines without reading file into Go heap
// The kernel handles page faults to load file data on demand
func (m *MMapFile) CountLines() int {
    count := 0
    for _, b := range m.data {
        if b == '\n' {
            count++
        }
    }
    return count
}

// Advise kernel about access pattern (improves readahead)
func (m *MMapFile) AdviseSequential() error {
    // MADV_SEQUENTIAL: expect sequential access → kernel reads ahead
    return syscall.Madvise(m.data, syscall.MADV_SEQUENTIAL)
}

func (m *MMapFile) AdviseRandom() error {
    // MADV_RANDOM: expect random access → kernel disables readahead
    return syscall.Madvise(m.data, syscall.MADV_RANDOM)
}

func (m *MMapFile) Close() error {
    if m.data != nil {
        if err := syscall.Munmap(m.data); err != nil {
            return fmt.Errorf("munmap: %w", err)
        }
    }
    return m.f.Close()
}

// Read a struct directly from mapped memory (zero copy)
// T must be fixed-size, no pointers
func ReadStruct[T any](m *MMapFile, offset int64) (*T, error) {
    size := int64(unsafe.Sizeof(*new(T)))
    if offset+size > m.size {
        return nil, fmt.Errorf("offset %d + size %d exceeds file size %d",
            offset, size, m.size)
    }
    // Direct cast: no copy, no allocation
    return (*T)(unsafe.Pointer(&m.data[offset])), nil
}
```

---

## Mental Model Summary

### The Keyword → Concept Map

```
╔═══════════════════════════════════════════════════════════════════════╗
║              GO KEYWORD MENTAL MODEL SUMMARY                         ║
╠══════════════════════╦════════════════════════════════════════════════╣
║ KEYWORD              ║ THINK OF IT AS                                 ║
╠══════════════════════╬════════════════════════════════════════════════╣
║ package              ║ Compilation unit + namespace                   ║
║ import               ║ Dependency declaration + type availability     ║
║ var                  ║ Named memory location (stack or heap)          ║
║ const                ║ Compile-time value (no memory address)         ║
║ type                 ║ New type contract (distinct from base type)    ║
║ struct               ║ Composite data layout (value type)             ║
║ interface            ║ Behavioral contract (implicit satisfaction)    ║
║ func                 ║ First-class callable + closure scope           ║
║ return               ║ Exit current function, optionally with values  ║
║ if/else              ║ Conditional control flow (errors as values)    ║
║ for                  ║ The ONLY loop (replaces while/foreach/do-while)║
║ range                ║ Iteration over collections/channels            ║
║ switch/case          ║ Multi-way branch (no fall-through by default)  ║
║ default              ║ Catch-all branch in switch/select              ║
║ fallthrough          ║ Explicit permission to execute next case       ║
║ break                ║ Exit loop/switch/select (supports labels)      ║
║ continue             ║ Next iteration (supports labels)               ║
║ goto                 ║ Unconditional jump (state machines only)       ║
║ go                   ║ Spawn goroutine (cheap, managed by runtime)    ║
║ chan                  ║ Typed goroutine communication conduit          ║
║ select               ║ Wait for first ready channel operation         ║
║ defer                ║ Register cleanup to run on function exit       ║
║ map                  ║ Hash map (reference type, not thread-safe)     ║
╚══════════════════════╩════════════════════════════════════════════════╝
```

### The Three Rules That Prevent 90% of Bugs

```
RULE 1: Zero values
  Always ask: "What happens if this is the zero value?"
  nil map write → panic
  nil pointer deref → panic
  nil chan → block forever
  nil interface with type → not == nil

RULE 2: Ownership
  "Who is responsible for closing this channel/file/connection?"
  Sender closes channels. Opener closes files. Caller checks errors.
  Use defer immediately after acquisition.

RULE 3: Sharing
  "Is this being accessed from multiple goroutines?"
  Maps need sync.RWMutex or sync.Map.
  Slices need external sync.
  Channels ARE the synchronization.
  Atomics for single-value counters.
  sync.Once for one-time initialization.
```

### Error Handling Flow

```
Every function that can fail returns (T, error):

  ┌──────────────────────────────────────────────────────────────────┐
  │  result, err := operation()                                      │
  │                   │                                              │
  │           ┌───────┴─────────┐                                   │
  │           ▼                 ▼                                    │
  │         err==nil         err!=nil                               │
  │           │                 │                                    │
  │        use result      classify error                           │
  │                              │                                   │
  │                   ┌──────────┼──────────┐                       │
  │                   ▼          ▼          ▼                        │
  │              sentinel     type       wrap+return                 │
  │              errors.Is()  errors.As()  fmt.Errorf("%w")         │
  │                   │          │          │                        │
  │              handle      handle     caller handles               │
  │              domain      domain     unwrapped chain              │
  │              case        case                                    │
  └──────────────────────────────────────────────────────────────────┘
```

### Concurrency Decision Tree

```
Need shared state between goroutines?
         │
    ┌────┴────┐
    │  Yes    │  No → plain goroutines + return channels
    └────┬────┘
         │
    Simple counter/flag?
         │
    ┌────┴────┐
    │  Yes    │  No
    └────┬────┘    │
   sync/atomic     │
                   │
              Passing data?
                   │
         ┌─────────┴──────────┐
         │  Yes               │  No
         └────┬───────────    └──── Protecting existing struct?
         Channels                        │
         (pipeline,                 sync.RWMutex
          fan-out,                  (read-heavy → RLock)
          ownership                 (write-heavy → plain Mutex)
          transfer)
```

---

*This guide reflects Go 1.22+ behavior. Key version milestones:*
- *Go 1.14: open-coded defers (defer performance improved)*
- *Go 1.18: generics (type parameters)*
- *Go 1.21: log/slog (structured logging stdlib)*
- *Go 1.22: loop variable per-iteration semantics; range over integers*

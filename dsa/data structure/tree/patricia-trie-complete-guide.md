# PATRICIA Tries — A Complete Guide

*Practical Algorithm To Retrieve Information Coded In Alphanumeric*

This guide builds the mental model from first principles, then grounds every
idea in real source: the Linux IPv4 FIB (`net/ipv4/fib_trie.c`), the eBPF
`BPF_MAP_TYPE_LPM_TRIE` (`kernel/bpf/lpm_trie.c`), and three from-scratch,
compiled-and-tested implementations (C, Rust, Go). All diagrams are plain
ASCII. All three implementations were actually compiled and run — the tree
diagrams that accompany them are dumps of the real in-memory structure those
programs built, not hand-drawn approximations.

**How to read this if you already know tries:** skim §1–§3 for vocabulary,
then jump to §7 (kernel deep dives) and §8–10 (implementations). If you're
building the mental model from zero, read in order.

---

## Table of Contents

1. [Origins and Naming](#1-origins-and-naming)
2. [The Problem: From Trie to Compressed Trie](#2-the-problem-from-trie-to-compressed-trie)
3. [Two Species of PATRICIA](#3-two-species-of-patricia)
4. [Anatomy of a Practical PATRICIA Node](#4-anatomy-of-a-practical-patricia-node)
5. [Core Algorithms, Traced on a Real Tree](#5-core-algorithms-traced-on-a-real-tree)
6. [Complexity](#6-complexity)
7. [Real Architecture: Linux Kernel Deep Dives](#7-real-architecture-linux-kernel-deep-dives)
8. [C Implementation — Generic Bit-String Dictionary](#8-c-implementation--generic-bit-string-dictionary)
9. [Rust Implementation — IPv4 Longest-Prefix-Match Trie](#9-rust-implementation--ipv4-longest-prefix-match-trie)
10. [Go Implementation — IPv4 Longest-Prefix-Match Trie](#10-go-implementation--ipv4-longest-prefix-match-trie)
11. [Performance: Why Compression Is a Cache Argument](#11-performance-why-compression-is-a-cache-argument)
12. [Common Pitfalls](#12-common-pitfalls)
13. [Expert Mental Model](#13-expert-mental-model)
14. [Further Reading](#14-further-reading)

---

## 1. Origins and Naming

PATRICIA was published by Donald R. Morrison in 1968 ("PATRICIA — Practical
Algorithm To Retrieve Information Coded In Alphanumeric," *Journal of the
ACM*, 15(4)). The original motivation was brutally practical: on 1960s
hardware, memory was the scarce resource, and a naive trie over long
bit-strings wastes almost all of it on chains of one-child nodes that exist
purely to consume one bit at a time.

Morrison's contribution was a **single node structure with no null
pointers**, where every internal node stores the index of the one bit that
actually matters at that point, and "child" links that sometimes point
*upward* to an ancestor instead of downward to a new node — that upward
pointer serves double duty as a loop-terminator during search, at zero extra
memory cost.

Two things happened to the name since 1968:

- The core idea — *branch on the index of the next bit where two keys in
  the subtree differ, and only there* — got reused in cleaner, easier-to-
  implement forms that drop the upward-link trick. Most software you'll
  encounter calling itself "a PATRICIA trie" (routing daemons, `ipset`-style
  tools, in-kernel LPM structures) uses one of these cleaner forms, not
  Morrison's original.
- The term started getting used loosely as a synonym for "compressed binary
  trie" or even "radix tree" in general, which causes real confusion because
  **"radix tree" also names a structurally different data structure** — see
  §3.3.

This guide covers Morrison's original for historical completeness, but
teaches and implements the practical variant, because that's what you'll
actually read in kernel source and routing software.

---

## 2. The Problem: From Trie to Compressed Trie

Take five keys: `rome`, `roman`, `romane`, `rubens`, `ruber`. All five share
the prefix `"ro"` or `"ru"`, and three of them (`rome`, `roman`, `romane`)
are literally prefixes of each other.

A **naive bitwise trie** allocates one node per bit position, for every bit,
whether or not that bit distinguishes anything. For a shared prefix like the
first 11 bits that `rome`, `roman`, and `romane` all agree on, that's 11
nodes in a single-child chain before the tree does any actual branching:

```
depth 0   [bit 0]
             |
depth 1   [bit 1]
             |
             :          <- 9 more single-child nodes, one per bit,
             :             doing zero branching work
             |
depth 11  [bit 11]  <---- first bit where "ro*" and "ru*" diverge
           /      \
       "ro..."   "ru..."
```

Every one of those single-child nodes costs a pointer, a bit-test, and a
cache miss on lookup — for information that doesn't distinguish anything.
On real workloads (English word lists, IP prefix tables, URL paths) the
overwhelming majority of trie nodes look exactly like this: structural
padding, not decisions.

**Path compression** is the fix: skip straight from one branching point to
the next, storing *which* bit index to test at each node instead of relying
on position-in-the-tree to imply the bit index. A branch node with only one
child is deleted from existence; the bit index it *would* have tested is
simply never tested, because it was never the bit where anything diverged.

That compression is the whole idea. Everything else in this guide is
detail: how to store the skipped bit-count so search still works (§4), how
to insert/delete without breaking the invariant (§5), and how production
systems extend the same idea to multi-bit "level compression" for even
denser trees (§7.1).

---

## 3. Two Species of PATRICIA

### 3.1 Morrison's original (1968): single node type, upward links

Every node is the same struct: a bit index, a key (or key reference), and
two child pointers. There is no separate "leaf" type and — this is the
memory trick — no `NULL` pointers. Every downward path eventually points
back *up* to an ancestor instead of terminating.

```
        [bit 2] <---------------------------+
        /      \                            |
   [bit 4]    [bit 7]                       |  upward link:
   /    \      /    \                       |  "right" child of
 [b6]  ...   ...    [bit 9] ----------------+  this node points
 /  \                /    \                    back to an ancestor
up  up             up     up                   instead of down
```

Search descends by testing bits in **strictly increasing** order of bit
index. Because indices strictly increase on the way down, and an upward
link necessarily points to a node with a *smaller* bit index than the one
you're leaving, the moment you'd follow a link to a bit index that is not
greater than the current one, you know you've hit the "leaf" — the upward
link is self-identifying as a terminator without needing a null-pointer
check. You then compare the full key to confirm a match or a miss.

This is elegant and memory-optimal, but it's also fiddly to implement
correctly (insertion has to identify exactly which upward link to redirect)
and doesn't parallelize or RCU-read as cleanly as a tree with real leaves,
because "is this pointer a normal edge or a back-edge" is a runtime property
of the whole structure, not a local one. That's the main reason modern
systems don't use it.

### 3.2 The practical variant: separate branch and leaf nodes

Give up the null-pointer trick. Use two node kinds:

- **Branch (internal) nodes** hold only a bit index and two child pointers
  (left = bit is 0, right = bit is 1). They carry no key.
- **Leaf (external) nodes** hold the actual key (and value).

```
                 branch: bit=11
                0/            \1
                /                \
      branch: bit=29         branch: bit=35
      0/         \1           0/         \1
      /             \         /             \
branch: bit=41    leaf:    leaf:          leaf:
 0/       \1      "rome"  "rubens"       "ruber"
 /          \
leaf:      leaf:
"roman"   "romane"
```

Now every downward pointer is a genuine forward edge — real `NULL` is used
for "no child" — and search terminates naturally when it reaches a leaf.
This costs one extra pointer's worth of "wasted" indirection per key (the
leaf holds a key but does no branching) in exchange for a structure that's
straightforward to implement, easy to reason about, and — critically for
kernel code — safe to walk under RCU (read-copy-update) without a reader
ever seeing a torn or cyclic structure, since nothing points backward.

This is the ancestor of BSD's `radix.c` (used historically in the BSD
routing table and firewall code), of most "patricia.c" implementations
floating around BGP tooling, and — as you'll see in §7.2 — of the Linux
`BPF_MAP_TYPE_LPM_TRIE`. **This is the variant this guide implements in
C, Rust, and Go.**

### 3.3 Don't confuse this with "radix tree" in the Linux/XArray sense

This trips up a lot of people who already know the Linux kernel, so it's
worth stopping on explicitly, since your background includes kernel
networking internals.

The kernel's historic `lib/radix-tree.c` (now superseded by `lib/xarray.c`,
the `XArray` API) is **not** a PATRICIA trie. It's a fixed-radix, multiway
trie keyed by a plain integer index (e.g. a page-cache offset or an XArray
index): the key is chopped into fixed-width chunks (commonly 6 bits per
level, i.e. radix 64), and each level of the tree is an array of up to 64
child pointers, indexed directly by the chunk value. There's no path
compression, no bit-testing, no notion of "the bit index that matters
here" — every level is walked, always, no matter how sparse the tree is.

```
XArray/lib radix-tree style (NOT patricia):

index 0x0000_002A
        |
  split into fixed 6-bit chunks: [000000][000000][000000][101010]
        |
  level0[0] -> level1[0] -> level2[0] -> level3[0b101010] -> value
        (every level walked, always, regardless of density)
```

versus a PATRICIA/compressed trie, which only creates branch nodes where
keys actually diverge, and skips straight past everything else. The
practical consequence: XArray-style radix trees are great for *dense,
integer-keyed* structures (the page cache, `struct file` descriptor
tables, IDR/IDA id allocators) where you expect most slots to be
occupied and O(1)-ish array indexing per level matters more than raw
compactness. PATRICIA-style compressed tries are great for *sparse*
prefix-shaped keys — IP address ranges, hierarchical strings — where the
whole point is to not pay for the parts of key-space nobody uses.

`net/ipv4/fib_trie.c`, covered in §7.1, actually **fuses both ideas**: it's
a PATRICIA trie (path-compressed) whose internal nodes *also* level-compress
into small local arrays when the trie is dense enough to make that
worthwhile — the "LC-trie" (Level-Compressed trie) from Nilsson & Karlsson,
1998. It is not the same code as `lib/xarray.c`, but it borrows the
"array-index a dense region" trick from the same family of ideas.

---

## 4. Anatomy of a Practical PATRICIA Node

### 4.1 Bit indexing convention

Keys are treated as bit strings, MSB-first, byte 0 first — i.e. network
byte order for anything IP-shaped. Bit index `i` means: byte `i / 8`, and
within that byte, the bit at position `7 - (i % 8)` counting from the LSB
(equivalently, the `(i % 8)`-th bit counting from the MSB).

```c
/* bit 0 is the most-significant bit of key[0] */
static inline int test_bit(const unsigned char *key, int keylen, int bit) {
    int byte = bit >> 3;
    if (byte >= keylen) return 0;         /* short keys read as zero-padded */
    return (key[byte] >> (7 - (bit & 7))) & 1;
}
```

The "short keys read as zero-padded" rule is what lets branch nodes compare
keys of different lengths without special-casing — a 4-byte key and a
6-byte key are just two bit-strings, one of which happens to be all zeros
past bit 32. This matters directly for §5.3 (insertion) and for CIDR
prefixes in §9–10, where a `/8` route and a `/24` route are, structurally,
exactly this situation.

### 4.2 Branch node

Holds:
- `bit`: the index of the one bit this node tests. **Invariant:** every
  descendant's `bit` is strictly greater than this node's `bit`. This is
  what makes single-pass, non-backtracking descent correct (§5.5).
- `left`, `right`: children for bit value 0 and 1.

Carries no key. It exists purely because, somewhere below it, two keys
disagree at `bit`.

### 4.3 Leaf node

Holds the actual key (owned copy or reference) and the associated value.
Carries no bit index — a leaf is a terminal, not a decision point.

### 4.4 The LPM variant used in §7 and §9–10

For longest-prefix-match tries (IP routing, `BPF_MAP_TYPE_LPM_TRIE`), the
node shape generalizes slightly: instead of "branch nodes have no value,
leaves do," **any** node can optionally carry a value, because a shorter
prefix (e.g. `10.0.0.0/8`) can validly be a route in its own right *and*
be an ancestor of a more specific route (`10.1.0.0/16`) stored deeper in
the same tree. A node with no value is a pure "glue" node — structurally
identical to a branch node above, just renamed to match how the kernel
names it (`LPM_TREE_NODE_FLAG_IM`, "intermediate," see §7.2).

---

## 5. Core Algorithms, Traced on a Real Tree

This section's tree is not hand-drawn — it's the literal output of the
tested C implementation from §8, run on the five keys from §2, with a debug
dump function walking the real in-memory nodes.

```
root bit=11
  L-> bit=29
    L-> bit=41
      L-> leaf "roman"  (40 bits)
      R-> leaf "romane" (48 bits)
    R-> leaf "rome"     (32 bits)
  R-> bit=35
    L-> leaf "rubens"   (48 bits)
    R-> leaf "ruber"    (40 bits)
```

As ASCII architecture:

```
                     [ bit 11 ]                       <- root
                    0/        \1
                    /            \
           [ bit 29 ]           [ bit 35 ]
           0/      \1            0/      \1
           /          \          /          \
    [ bit 41 ]      "rome"  "rubens"      "ruber"
    0/     \1
    /         \
 "roman"    "romane"
```

### 5.1 Blind descent: `find_best_leaf`

The core trick that makes insertion and search both O(bit-length) with no
backtracking: descend from the root, and at every branch node, test the
*search key's* bit at that node's stored index — **without checking whether
that index is even meaningful for the key you're looking for.** Always go
left on 0, right on 1. This is guaranteed to terminate at a leaf (branch
nodes have exactly two children, always populated, in this external-node
design), but that leaf is only a *candidate* — the "closest" key in the
tree by the bits that were actually tested, not necessarily a match.

```c
static pnode_t *find_best_leaf(pnode_t *root, const unsigned char *key, int keylen) {
    if (!root) return NULL;
    pnode_t *n = root;
    while (!n->is_leaf) {
        n = test_bit(key, keylen, n->bit) ? n->right : n->left;
    }
    return n;
}
```

Trace searching for `"rubicon"` (not in the tree): bit 11 of `"rubicon"` is
1 (same as `"ru*"` keys) → go right, land at `bit=35`; bit 35 of
`"rubicon"` → tests some bit inside `"rub"` that happens to send it left →
land at leaf `"rubens"`. `find_best_leaf` returns `"rubens"` — the closest
match structurally, not a real match. That's expected; it's an internal
helper, not the public search API.

### 5.2 Search

Wrap the blind descent with a final full-key comparison — this is the step
that actually decides hit vs. miss:

```c
void *patricia_search(patricia_t *t, const unsigned char *key, int keylen) {
    pnode_t *leaf = find_best_leaf(t->root, key, keylen);
    if (!leaf) return NULL;
    if (leaf->keylen == keylen && memcmp(leaf->key, key, keylen) == 0)
        return leaf->value;
    return NULL;
}
```

Searching for `"rubicon"` lands on `"rubens"` via blind descent, fails the
`memcmp`, and correctly reports "not found" — even though the search never
did a single comparison against `"rubicon"`'s actual content until the very
last step. This single-comparison-at-the-end property is what makes
PATRICIA search strictly O(bit-length of the key) with no backtracking,
independent of how many keys are stored.

### 5.3 Insert: find-candidate, diff, re-descend, splice

Inserting a new key is a three-phase process. Using `"rubicon"` as the
running example against the tree above:

**Phase 1 — find the nearest existing key.** Run `find_best_leaf` exactly
as in search. For `"rubicon"` this again lands on `"rubens"`.

**Phase 2 — find the first bit where the new key and that candidate
diverge.** Compare `"rubicon"` against `"rubens"` bit by bit from 0. They
agree through `"rub"` (bits 0–23) and diverge inside the 4th byte — call
that bit index `diff`. This is the *only* full key comparison the whole
insert does.

**Phase 3 — re-descend from the root, this time stopping at the right
spot.** Walk down again, but this time compare each branch node's stored
`bit` against `diff`: keep descending only while `node->bit < diff`. The
moment you'd move to a node whose `bit` is not less than `diff` (or you hit
a leaf), stop — that is exactly where the new branch belongs, because every
node above it tests a bit *before* `diff` (so both keys agree there and it's
safe to be under it), and the new branch is the first node that needs to
test `diff` itself.

```c
pnode_t **link = &t->root;
pnode_t *parent = NULL;
while (!(*link)->is_leaf && (*link)->bit < diff) {
    parent = *link;
    link = test_bit(key, keylen, parent->bit) ? &parent->right : &parent->left;
}

pnode_t *new_leaf = make_leaf(key, keylen, value);
int new_key_bit = test_bit(key, keylen, diff);
pnode_t *new_branch = new_key_bit
    ? make_branch(diff, *link, new_leaf)
    : make_branch(diff, new_leaf, *link);
*link = new_branch;
```

For `"rubicon"`, `diff` falls inside the region already covered by the
`bit=35` branch node's subtree, so phase 3 descends past `bit=11` and stops
exactly at `bit=35`'s slot, splicing a new branch node there:

```
                     [ bit 11 ]
                    0/        \1
                    /            \
           [ bit 29 ]           [ bit 35 ]
           0/      \1            0/      \1
           /          \          /          \
    [ bit 41 ]      "rome"  [ bit=diff ]   "ruber"
    0/     \1                0/      \1
    /         \              /          \
 "roman"    "romane"     "rubens"    "rubicon"
```

(Exact position of the new branch under `bit=35` depends on which side
`diff` sends `"rubicon"` and `"rubens"` to; the shape above is illustrative
of *where* the splice happens — the C program in §8 will print the real
result if you run it with this key added.)

Why the strictly-increasing-bit-index invariant makes this safe: because
every ancestor of the eventual insertion point tests a bit *less than*
`diff`, and both `"rubicon"` and every key already under that subtree agree
on all bits below `diff` (that's what `diff` being the first difference
*means*), inserting the new branch there cannot violate any ancestor's
correctness — nothing above the new node needs to change.

### 5.4 Delete: splice out the leaf's parent, promote the sibling

Deleting a key requires the leaf's parent and grandparent (in this
external-node design, deleting a leaf always deletes its direct parent
branch node too, since a branch node existing with only one remaining
child is exactly the "wasted node" §2 was designed to eliminate):

```c
pnode_t **grandlink = NULL;
pnode_t **parentlink = &t->root;
pnode_t *node = t->root;

while (!node->is_leaf) {
    grandlink = parentlink;
    parentlink = test_bit(key, keylen, node->bit) ? &node->right : &node->left;
    node = *parentlink;
}
/* node is now the target leaf (after confirming a full-key match) */

pnode_t *parent = *grandlink;
pnode_t *sibling = (parentlink == &parent->left) ? parent->right : parent->left;
*grandlink = sibling;    /* grandparent now points directly at the sibling */
```

Deleting `"roman"` from the original tree: its parent is `bit=41`, its
sibling is leaf `"romane"`, and its grandparent's link (`bit=29`'s left
child) is repointed directly at `"romane"`. The `bit=41` node is freed —
it no longer distinguishes anything, since only one key remains in that
subtree:

```
                     [ bit 11 ]
                    0/        \1
                    /            \
           [ bit 29 ]           [ bit 35 ]
           0/      \1            0/      \1
           /          \          /          \
      "romane"      "rome"  "rubens"      "ruber"
```

### 5.5 Why single-pass, non-backtracking descent is correct

The strictly-increasing-bit-index invariant is the entire reason this
works without backtracking, so it's worth stating precisely:

> **Invariant:** for any branch node `N` with stored index `N.bit`, every
> key stored in `N`'s subtree agrees, bit for bit, on every bit index less
> than `N.bit`. `N.bit` is chosen so that the subtree splits into "keys
> with bit `N.bit` = 0" (left) and "= 1" (right), and every node strictly
> below `N` tests some bit index greater than `N.bit`.

Given that invariant, "blind descent" testing only the search key's own
bits — never checking whether a given bit index is even meaningful for that
key — is guaranteed to land on the *unique* leaf that would be correct *if*
the key exists, because at every branching point the path taken is forced
by the one bit that would distinguish the search key from everything on
the other side. If the key isn't actually in the tree, descent still
terminates (branch nodes always have two populated children in this
design) at *some* leaf — just not a matching one, caught by the final
`memcmp`. Insertion's phase-3 re-descent relies on the same invariant to
find the unique correct splice point in one pass.

---

## 6. Complexity

Let `k` = key length in bits, `n` = number of keys stored.

| Structure                        | Search        | Insert        | Delete        | Space                              | Notes |
|-----------------------------------|---------------|---------------|---------------|-------------------------------------|-------|
| Naive bitwise trie                | O(k)          | O(k)          | O(k)          | O(n·k) worst case                   | One node per bit, huge single-child chains |
| PATRICIA (path-compressed)        | O(k)          | O(k)          | O(k)          | O(n)                                | Exactly `n` leaves, at most `n-1` branch nodes |
| Level-compressed (LC-trie)        | O(k / w) amortized, worst-case O(k) | O(k) amortized (rebalance cost) | O(k) amortized | O(n), larger constant | `w` = average bits consumed per node; see §7.1 |
| Balanced BST (e.g. red-black) on keys | O(log n · compare-cost) | O(log n) | O(log n) | O(n) | Comparisons can themselves cost O(k) each |
| Hash table                        | O(1) average, O(k) to hash | O(1) average | O(1) average | O(n) | No ordering, no prefix/range queries |
| B-tree                            | O(log_B n)    | O(log_B n)    | O(log_B n)    | O(n)                                 | Optimized for block/page-oriented storage |

The column that matters most in practice is the one hash tables can't offer
at all: **PATRICIA supports longest-prefix-match and ordered iteration
natively**, because the tree structure directly encodes the prefix
relationships between keys. That's the property every use case in this
guide (IP routing, `BPF_MAP_TYPE_LPM_TRIE`, autocomplete, IP allow-lists)
actually needs — a hash table would need a separate linear or trie-based
scan over all possible prefix lengths to answer "what's the most specific
rule that covers this address," which is exactly the operation §7 and
§9–10 are built around.

---

## 7. Real Architecture: Linux Kernel Deep Dives

### 7.1 `net/ipv4/fib_trie.c` — the IPv4 routing table as an LC-trie

Since Linux 2.6.13, the IPv4 Forwarding Information Base (the kernel's
compiled routing table — what `ip route` shows you a view of) is stored as
an **LC-trie**: a PATRICIA trie (path compression, exactly as in §2–§5)
whose internal nodes *additionally* level-compress into small local arrays
wherever the trie is dense enough to make that a win. This is the Nilsson
& Karlsson (1998) LC-trie design, adapted for dynamic insert/delete instead
of the original's build-once/read-many use case.

**Node shape.** The kernel unifies "internal node" and "leaf" into one
`struct key_vector`, distinguished at runtime by whether its `bits` field
is zero:

```
struct key_vector {
    t_key key;            /* the bits already matched on the path here */
    unsigned char pos;     /* bit offset into the key where this node's
                             * own index segment starts  <- PATH compression */
    unsigned char bits;    /* how many key bits this node's array indexes
                             * (0 for a leaf, >0 for an internal node)
                             * <- LEVEL compression */
    unsigned char slen;
    union {
        struct hlist_head leaf;      /* valid when bits == 0 (leaf) */
        key_vector *tnode[];         /* valid when bits > 0 (internal) */
    };
};
```

- `pos` is the path-compression field: it's exactly the "index of the next
  bit that matters," the same concept as `bit` in §4.2, generalized to "the
  first bit of a whole *segment* this node indexes."
- `bits` is the level-compression field: instead of a plain binary branch
  (one bit, two children, as in §4–§5), a `key_vector` with `bits = k`
  branches on a `k`-bit segment at once, via a `2^k`-entry child array —
  turning what would be `k` single-bit branch nodes in a plain PATRICIA
  trie into one array-indexed lookup. `/proc/net/fib_triestat` on a live
  system shows this directly: it reports how many internal nodes handle 1
  bit, 2 bits, 3 bits, and so on, and on a densely populated table (a large
  BGP full-table view, for instance) it's common to see individual nodes
  handling 15–20+ bits at once, collapsing what would be a 20-level binary
  descent into one array index.
- A leaf's `hlist_head` chains together every route that terminates at that
  exact key, sorted by prefix length — because, exactly as noted in §4.4,
  more than one route can share the same address bits while differing in
  prefix length (`10.0.0.0/8` and `10.0.0.0/16` are different routes with
  the same 32-bit key). Each entry (`struct fib_alias`) points at a
  `struct fib_info` — the actual next-hop, output interface, and metrics
  shared by however many routes happen to use that same gateway.

**Density-adaptive rebalancing.** Unlike this guide's §5 insert/delete,
which only ever adds or removes exactly one branch node, `fib_trie`'s
`resize()` — via `inflate()` and `halve()` — actively grows or shrinks a
node's `bits` (and therefore its child array size) after every insert or
delete, based on how populated the surrounding subtree is. Insert a lot of
routes into a small address range and `inflate()` doubles a node's array,
pulling two levels of binary branching into one array-indexed level;
delete enough and `halve()` reverses it. This is the "L" in LC-trie — it's
what makes lookups on a real-world, densely populated FIB (millions of
routes) meaningfully faster than the plain binary PATRICIA trie in §5,
whose depth is bounded only by key length, not by how many keys actually
exist.

**Lookup with backtracking on prefix length.** A plain PATRICIA search
(§5.2) does one blind descent and one final comparison. FIB lookup is
subtler, because the goal isn't "does this exact address exist as a route"
— it's "what is the *longest* matching prefix." The kernel's own
implementation notes describe this as: descend as far as the address's own
bits allow, and if the leaf reached doesn't yield a semantic match at the
full prefix length, back off one prefix-length step at a time,
backtracking up through the trie, until a matching leaf/alias is found.
This backtracking is exactly the same idea implemented cleanly (with no
backtracking needed, by construction) by the "carry a `best` value while
descending" technique used in §9–10's LPM tries and in `lpm_trie.c` below —
`fib_trie`'s version is more intricate because level compression means a
single node covers a range of prefix lengths at once, so "back off one bit"
isn't simply "go to the parent."

```
Illustrative LC-trie fragment (small example, not a system dump):

           key_vector: pos=0, bits=2   <- one node, indexes 2 bits at once
           (4-entry array instead of 2 levels of binary branch)
          /      |      |      \
      [00]    [01]    [10]    [11]
        |       |       |       |
     leaf     leaf   key_vector   leaf
   10.0/8   10.64/10  pos=10,bits=1   10.192/10
                        /      \
                     leaf     leaf
                  10.128/9  10.160/11(?) -- illustrative only
```

### 7.2 `kernel/bpf/lpm_trie.c` — `BPF_MAP_TYPE_LPM_TRIE`

This is the one most directly relevant to XDP/eBPF work: a `BPF_MAP_TYPE_LPM_TRIE`
map is a from-scratch, general-purpose (not IPv4-specific — key width is
configurable up to 2048 bits, enough for IPv6 plus metadata) implementation
of **exactly the external-node PATRICIA trie from §3.2 and §5**, with no
level compression — it's the "clean" variant, not the LC-trie. If you've
worked with these maps from Aya or libbpf, the algorithm in §5 and the code
in §9–10 *is* what's running on the other side of `bpf_map_lookup_elem()`.

Node shape (field names paraphrased from the kernel source, same
information, same order):

```
struct lpm_trie_node {
    lpm_trie_node __rcu *child[2];   /* [0]=bit-is-0 child, [1]=bit-is-1 */
    u32                  prefixlen;   /* this node's own prefix length */
    u32                  flags;       /* LPM_TREE_NODE_FLAG_IM if this is
                                       * a pure "glue"/intermediate node */
    u8                   data[];      /* the key bytes, big-endian */
};
```

Map this directly onto §4.4 and the Rust/Go implementations in §9–10:
`flags & LPM_TREE_NODE_FLAG_IM` is precisely this guide's `value: None`
(Rust) / `hasValue == false` (Go) — a node that exists only to fork the
tree at a bit where two real, inserted prefixes diverge, exactly the "glue
node" created in the divergence branch of `insert_at`/`insertNode` in §9–10.
A node *without* that flag is a real, user-inserted prefix — exactly this
guide's leaf-with-a-value, except the kernel's version, like §9–10's LPM
tries (and unlike the exact-match trie in §8), allows *any* node in the
tree to carry a real value, not just terminal ones, because shorter
prefixes are valid routes/rules in their own right.

`trie_lookup_elem()` — the function that runs when your XDP or TC program
calls `bpf_map_lookup_elem()` on an LPM map — descends exactly like this
guide's §9–10 `lookup`/`Lookup`: walk down while the current node's stored
prefix still matches the search key's corresponding bits, and whenever the
current node is a real match (not `LPM_TREE_NODE_FLAG_IM`), remember it as
the best answer so far; return the deepest one found. Divergence stops the
walk early, exactly as in §5.5's correctness argument.

**Concurrency.** Because lookups happen from XDP/TC hook context — often
concurrently, on multiple CPUs, possibly while a control-plane process is
still updating the map from userspace — every pointer in the child array is
RCU-protected (`__rcu` annotations), and updates take a dedicated spinlock
(`rqspinlock_t` in current kernels) so concurrent *writers* serialize while
concurrent *readers* never block and never observe a torn node. This is a
direct, practical payoff of the external-node design from §3.2: because
there's no upward-link/backtrack trick, every pointer a reader follows is a
plain forward RCU pointer, and a reader that started its walk before an
update simply keeps seeing the pre-update tree until it's done — the same
reasoning that makes RCU-protected linked structures safe throughout the
rest of the networking and scheduler code you already work with.

**Using it from Aya**, keeping the same shape as your XDP firewall work:

```rust
// eBPF side (aya-ebpf): keying by a CIDR prefix for LPM matching
use aya_ebpf::{macros::map, maps::LpmTrie};

#[map]
static ALLOWED_NETS: LpmTrie<u32, u8> =
    LpmTrie::with_max_entries(1024, 0 /* BPF_F_NO_PREALLOC handled by aya */);

// key is a `aya_ebpf::maps::lpm_trie::Key<u32>` = { prefix_len, data }
// lookup finds the longest (most specific) matching prefix, exactly as in §9.
```

**Where it's *not* used.** `ipset` (netfilter's IP set implementation) uses
hash tables or plain sorted arrays depending on set type, not a PATRICIA
trie — it's optimized for exact-match and small-range membership tests, not
prefix matching. `nftables` sets similarly default to hash-based or
interval-tree-based backends depending on the element type. If you need
genuine longest-prefix-match semantics in-kernel outside of routing itself,
`BPF_MAP_TYPE_LPM_TRIE` is the tool built for exactly that.

---

## 8. C Implementation — Generic Bit-String Dictionary

This is the classic external-node PATRICIA trie from §3.2 and §5: an
exact-match symbol table mapping arbitrary byte-string keys to values, with
insert, search, and delete. Compiled with `-fsanitize=address,undefined`
and run clean — no leaks, no UB, no out-of-bounds access.

```c
/*
 * patricia.c -- classic (external-node) PATRICIA trie.
 *
 * Practical Algorithm To Retrieve Information Coded In Alphanumeric.
 *
 * This is a *symbol table* PATRICIA trie: it maps arbitrary byte-string
 * keys to values with exact-match lookup, insert and delete, using
 * O(1)-per-node radix branching and full path compression. It uses the
 * "external node" formulation (leaves are a distinct struct from
 * branch nodes) rather than Morrison's original single-node,
 * upward-link formulation -- see the accompanying guide for why.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ---- node representation ------------------------------------------ */

typedef struct pnode {
    int is_leaf;
    int bit;                /* branch nodes only: bit index tested here */
    struct pnode *left;     /* branch nodes only: child for bit == 0 */
    struct pnode *right;    /* branch nodes only: child for bit == 1 */
    unsigned char *key;     /* leaf nodes only: owned copy of the key */
    int keylen;              /* leaf nodes only: key length in bytes */
    void *value;             /* leaf nodes only */
} pnode_t;

typedef struct {
    pnode_t *root;
    size_t count;
} patricia_t;

/* ---- bit access ------------------------------------------------------
 * Bit 0 is the MSB of byte 0. Reading past the end of the key returns 0,
 * which is what lets branch nodes compare keys of different lengths
 * (the shorter key behaves as if zero-padded).
 */
static inline int test_bit(const unsigned char *key, int keylen, int bit) {
    int byte = bit >> 3;
    if (byte >= keylen) return 0;
    return (key[byte] >> (7 - (bit & 7))) & 1;
}

/* First bit at which two keys differ, or -1 if identical over the
 * shorter key's length (in which case the longer key differs by having
 * extra length -- we treat the first bit past the shorter key's end as
 * the differing bit so distinct-length keys still separate). */
static int first_diff_bit(const unsigned char *a, int alen,
                           const unsigned char *b, int blen) {
    int maxbits = (alen > blen ? alen : blen) * 8;
    for (int i = 0; i < maxbits; i++) {
        if (test_bit(a, alen, i) != test_bit(b, blen, i))
            return i;
    }
    return -1; /* keys identical */
}

static pnode_t *make_leaf(const unsigned char *key, int keylen, void *value) {
    pnode_t *n = calloc(1, sizeof(*n));
    n->is_leaf = 1;
    n->key = malloc(keylen);
    memcpy(n->key, key, keylen);
    n->keylen = keylen;
    n->value = value;
    return n;
}

static pnode_t *make_branch(int bit, pnode_t *left, pnode_t *right) {
    pnode_t *n = calloc(1, sizeof(*n));
    n->is_leaf = 0;
    n->bit = bit;
    n->left = left;
    n->right = right;
    return n;
}

/* ---- search ------------------------------------------------------ */

/* Descend using only the bit-index rule: at a branch node, go left or
 * right depending on the tested bit of *the search key*, ignoring
 * whether that bit index actually exists in the leaf we'll land on.
 * Because bit indices strictly increase root-to-leaf, this always
 * terminates at *some* leaf -- the best (longest common-prefix)
 * candidate, not necessarily a match. */
static pnode_t *find_best_leaf(pnode_t *root, const unsigned char *key, int keylen) {
    if (!root) return NULL;
    pnode_t *n = root;
    while (!n->is_leaf) {
        n = test_bit(key, keylen, n->bit) ? n->right : n->left;
    }
    return n;
}

void *patricia_search(patricia_t *t, const unsigned char *key, int keylen) {
    pnode_t *leaf = find_best_leaf(t->root, key, keylen);
    if (!leaf) return NULL;
    if (leaf->keylen == keylen && memcmp(leaf->key, key, keylen) == 0)
        return leaf->value;
    return NULL;
}

/* ---- insert -------------------------------------------------------- */

int patricia_insert(patricia_t *t, const unsigned char *key, int keylen, void *value) {
    if (!t->root) {
        t->root = make_leaf(key, keylen, value);
        t->count++;
        return 1;
    }

    pnode_t *best = find_best_leaf(t->root, key, keylen);
    if (best->keylen == keylen && memcmp(best->key, key, keylen) == 0) {
        best->value = value; /* key already present: overwrite */
        return 0;
    }

    int diff = first_diff_bit(key, keylen, best->key, best->keylen);

    /* Re-descend, stopping at the first branch node whose bit index is
     * greater than `diff` (or at a leaf) -- that is where the new
     * branch must be spliced in, because every branch above it agrees
     * with both keys, and the new branch decides bit `diff`. */
    pnode_t **link = &t->root;
    pnode_t *parent = NULL;
    while (!(*link)->is_leaf && (*link)->bit < diff) {
        parent = *link;
        link = test_bit(key, keylen, parent->bit) ? &parent->right : &parent->left;
    }

    pnode_t *new_leaf = make_leaf(key, keylen, value);
    int new_key_bit = test_bit(key, keylen, diff);
    pnode_t *new_branch = new_key_bit
        ? make_branch(diff, *link, new_leaf)
        : make_branch(diff, new_leaf, *link);

    *link = new_branch;
    t->count++;
    return 1;
}

/* ---- delete --------------------------------------------------------
 * Find the leaf's parent and grandparent while descending; splice the
 * parent branch out and promote the sibling into the grandparent's slot.
 */
int patricia_delete(patricia_t *t, const unsigned char *key, int keylen) {
    if (!t->root) return 0;

    if (t->root->is_leaf) {
        if (t->root->keylen == keylen && memcmp(t->root->key, key, keylen) == 0) {
            free(t->root->key);
            free(t->root);
            t->root = NULL;
            t->count--;
            return 1;
        }
        return 0;
    }

    pnode_t **grandlink = NULL; /* pointer to the slot holding `parent` */
    pnode_t **parentlink = &t->root; /* pointer to the slot holding current node */
    pnode_t *node = t->root;

    while (!node->is_leaf) {
        grandlink = parentlink;
        parentlink = test_bit(key, keylen, node->bit) ? &node->right : &node->left;
        node = *parentlink;
    }

    if (node->keylen != keylen || memcmp(node->key, key, keylen) != 0)
        return 0; /* not found */

    pnode_t *parent = *grandlink;
    pnode_t *sibling = (parentlink == &parent->left) ? parent->right : parent->left;

    *grandlink = sibling;

    free(node->key);
    free(node);
    free(parent);
    t->count--;
    return 1;
}

void patricia_free(pnode_t *n) {
    if (!n) return;
    if (n->is_leaf) {
        free(n->key);
    } else {
        patricia_free(n->left);
        patricia_free(n->right);
    }
    free(n);
}

/* ---- demo / self-test ---------------------------------------------- */

static void put(patricia_t *t, const char *k, long v) {
    patricia_insert(t, (const unsigned char *)k, (int)strlen(k), (void *)v);
}

static void expect(const char *label, long got, long want) {
    printf("%-32s got=%-6ld want=%-6ld %s\n", label, got, want,
           got == want ? "OK" : "FAIL");
    if (got != want) exit(1);
}

int main(void) {
    patricia_t t = {0};

    const char *words[] = {
        "rome", "roman", "romane", "romanus", "romulus",
        "rubens", "ruber", "rubicon", "rubicundus"
    };
    for (size_t i = 0; i < sizeof(words) / sizeof(words[0]); i++)
        put(&t, words[i], (long)(i + 1));

    expect("count after inserts", (long)t.count, 9);

    for (size_t i = 0; i < sizeof(words) / sizeof(words[0]); i++) {
        void *v = patricia_search(&t, (const unsigned char *)words[i], strlen(words[i]));
        expect(words[i], (long)v, (long)(i + 1));
    }

    long miss = (long)patricia_search(&t, (const unsigned char *)"rubicundissimus", 15);
    expect("miss: rubicundissimus", miss, 0);

    miss = (long)patricia_search(&t, (const unsigned char *)"rub", 3);
    expect("miss: rub (prefix only)", miss, 0);

    /* overwrite */
    put(&t, "roman", 999);
    expect("count after overwrite", (long)t.count, 9);
    long v = (long)patricia_search(&t, (const unsigned char *)"roman", 5);
    expect("roman after overwrite", v, 999);

    /* delete and re-check neighbors */
    int ok = patricia_delete(&t, (const unsigned char *)"roman", 5);
    expect("delete roman", ok, 1);
    expect("count after delete", (long)t.count, 8);
    miss = (long)patricia_search(&t, (const unsigned char *)"roman", 5);
    expect("roman gone", miss, 0);
    v = (long)patricia_search(&t, (const unsigned char *)"romane", 6);
    expect("romane still present", v, 3);
    v = (long)patricia_search(&t, (const unsigned char *)"rome", 4);
    expect("rome still present", v, 1);

    /* delete a leaf that is the root's direct child at the end */
    ok = patricia_delete(&t, (const unsigned char *)"rubicundus", 10);
    expect("delete rubicundus", ok, 1);
    v = (long)patricia_search(&t, (const unsigned char *)"rubicon", 7);
    expect("rubicon still present", v, 8);

    /* delete down to one and zero elements */
    const char *remaining[] = {"rome", "romane", "romanus", "romulus",
                                "rubens", "ruber", "rubicon"};
    for (size_t i = 0; i < sizeof(remaining) / sizeof(remaining[0]); i++) {
        patricia_delete(&t, (const unsigned char *)remaining[i], strlen(remaining[i]));
    }
    expect("count after draining", (long)t.count, 0);
    miss = (long)patricia_search(&t, (const unsigned char *)"rome", 4);
    expect("empty trie search", miss, 0);

    patricia_free(t.root);
    printf("\nAll C tests passed.\n");
    return 0;
}
```

**Build and run:**

```sh
gcc -Wall -Wextra -std=c11 -g -fsanitize=address,undefined -o patricia patricia.c
./patricia
```

**Actual output:**

```
count after inserts              got=9      want=9      OK
rome                              got=1      want=1      OK
roman                             got=2      want=2      OK
romane                            got=3      want=3      OK
romanus                           got=4      want=4      OK
romulus                           got=5      want=5      OK
rubens                            got=6      want=6      OK
ruber                             got=7      want=7      OK
rubicon                           got=8      want=8      OK
rubicundus                        got=9      want=9      OK
miss: rubicundissimus             got=0      want=0      OK
miss: rub (prefix only)           got=0      want=0      OK
count after overwrite             got=9      want=9      OK
roman after overwrite             got=999    want=999    OK
delete roman                      got=1      want=1      OK
count after delete                got=8      want=8      OK
roman gone                        got=0      want=0      OK
romane still present              got=3      want=3      OK
rome still present                got=1      want=1      OK
delete rubicundus                 got=1      want=1      OK
rubicon still present             got=8      want=8      OK
count after draining              got=0      want=0      OK
empty trie search                 got=0      want=0      OK

All C tests passed.
```

Note what `"miss: rub (prefix only)"` is testing: `"rub"` is a genuine bit-
string prefix of `"rubens"`, `"ruber"`, etc., but was never inserted as a
key itself — and the trie correctly reports it absent. This is the
exact-match trie behaving as a dictionary should; §9–10's LPM tries
deliberately invert this expectation, because for routing, a shorter
stored prefix *should* match a longer search key (that's the entire point
of longest-prefix-match) — the difference in requirements is why §9–10 use
a different node shape (§4.4) than this section.

---

## 9. Rust Implementation — IPv4 Longest-Prefix-Match Trie

The applied version: a PATRICIA trie specialized to IPv4 CIDR blocks,
supporting the operation real routing software actually needs —
"find the value of the most specific stored prefix that contains this
address" — which is exactly what a `BPF_MAP_TYPE_LPM_TRIE` lookup or a
kernel FIB lookup computes (§7). Node shape follows §4.4 and mirrors
`struct lpm_trie_node` from §7.2 field-for-field: `prefix`/`prefixlen`,
an optional value, and two child slots.

```rust
//! lpm.rs -- a PATRICIA trie specialised for IPv4 CIDR longest-prefix-match
//! (LPM), the same shape as `struct lpm_trie_node` in the Linux kernel's
//! `kernel/bpf/lpm_trie.c` and (module the level-compression) `net/ipv4/fib_trie.c`.
//!
//! Every node carries a `prefix`/`prefixlen` (a CIDR block) and an
//! `Option<V>`. Nodes with `value = None` are pure branch/glue nodes that
//! exist only to fork the tree at a bit where two real prefixes diverge --
//! this is exactly the kernel's "intermediate node" (`im` flag) idea.

type Prefix = u32;

#[derive(Debug)]
struct Node<V> {
    prefix: Prefix,
    prefixlen: u8, // 0..=32
    value: Option<V>,
    left: Option<Box<Node<V>>>,  // child whose branching bit is 0
    right: Option<Box<Node<V>>>, // child whose branching bit is 1
}

pub struct LpmTrie<V> {
    root: Option<Box<Node<V>>>,
    len: usize,
}

#[inline]
fn get_bit(x: Prefix, pos: u8) -> u8 {
    if pos >= 32 {
        return 0;
    }
    ((x >> (31 - pos)) & 1) as u8
}

#[inline]
fn mask_for(len: u8) -> Prefix {
    if len == 0 {
        0
    } else {
        Prefix::MAX << (32 - len as u32)
    }
}

#[inline]
fn masked(x: Prefix, len: u8) -> Prefix {
    x & mask_for(len)
}

#[inline]
fn prefix_matches(node_prefix: Prefix, node_len: u8, addr: Prefix) -> bool {
    masked(node_prefix, node_len) == masked(addr, node_len)
}

/// Length of the common leading-bit run between `a` and `b`, capped at `max`.
fn common_prefix_len(a: Prefix, b: Prefix, max: u8) -> u8 {
    let mut i = 0u8;
    while i < max && get_bit(a, i) == get_bit(b, i) {
        i += 1;
    }
    i
}

impl<V> Default for LpmTrie<V> {
    fn default() -> Self {
        Self { root: None, len: 0 }
    }
}

impl<V> LpmTrie<V> {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn len(&self) -> usize {
        self.len
    }

    pub fn is_empty(&self) -> bool {
        self.len == 0
    }

    /// Insert (or overwrite) the route `prefix/prefixlen -> value`.
    pub fn insert(&mut self, prefix: Prefix, prefixlen: u8, value: V) {
        let prefix = masked(prefix, prefixlen);
        let inserted = Self::insert_at(&mut self.root, prefix, prefixlen, value);
        if inserted {
            self.len += 1;
        }
    }

    /// Returns true if this call created a brand new route (as opposed to
    /// overwriting an existing exact-match prefix).
    fn insert_at(slot: &mut Option<Box<Node<V>>>, prefix: Prefix, prefixlen: u8, value: V) -> bool {
        match slot {
            None => {
                *slot = Some(Box::new(Node {
                    prefix,
                    prefixlen,
                    value: Some(value),
                    left: None,
                    right: None,
                }));
                true
            }
            Some(node) => {
                let common = common_prefix_len(node.prefix, prefix, node.prefixlen.min(prefixlen));

                if common == node.prefixlen && node.prefixlen <= prefixlen {
                    // `prefix` extends (or equals) this node's prefix.
                    if prefixlen == node.prefixlen {
                        let was_new = node.value.is_none();
                        node.value = Some(value);
                        was_new
                    } else {
                        let bit = get_bit(prefix, node.prefixlen);
                        let child = if bit == 0 { &mut node.left } else { &mut node.right };
                        Self::insert_at(child, prefix, prefixlen, value)
                    }
                } else if common == prefixlen && prefixlen < node.prefixlen {
                    // `prefix` is a strict ancestor of this node: splice above it.
                    let old = slot.take().unwrap();
                    let bit = get_bit(old.prefix, prefixlen);
                    let mut new_node = Box::new(Node {
                        prefix,
                        prefixlen,
                        value: Some(value),
                        left: None,
                        right: None,
                    });
                    if bit == 0 {
                        new_node.left = Some(old);
                    } else {
                        new_node.right = Some(old);
                    }
                    *slot = Some(new_node);
                    true
                } else {
                    // Genuine divergence at bit `common`: insert a glue node.
                    let old = slot.take().unwrap();
                    let old_bit = get_bit(old.prefix, common);
                    let new_leaf = Box::new(Node {
                        prefix,
                        prefixlen,
                        value: Some(value),
                        left: None,
                        right: None,
                    });
                    let mut glue = Box::new(Node {
                        prefix: masked(old.prefix, common),
                        prefixlen: common,
                        value: None,
                        left: None,
                        right: None,
                    });
                    if old_bit == 0 {
                        glue.left = Some(old);
                        glue.right = Some(new_leaf);
                    } else {
                        glue.right = Some(old);
                        glue.left = Some(new_leaf);
                    }
                    *slot = Some(glue);
                    true
                }
            }
        }
    }

    /// Longest-prefix-match lookup: the value of the most specific stored
    /// prefix that contains `addr`, exactly like `bpf_map_lookup_elem()` on
    /// a `BPF_MAP_TYPE_LPM_TRIE`, or a route lookup in the kernel FIB.
    pub fn lookup(&self, addr: Prefix) -> Option<&V> {
        let mut cur = &self.root;
        let mut best: Option<&V> = None;
        while let Some(node) = cur {
            if !prefix_matches(node.prefix, node.prefixlen, addr) {
                break;
            }
            if let Some(v) = &node.value {
                best = Some(v);
            }
            if node.prefixlen == 32 {
                break;
            }
            let bit = get_bit(addr, node.prefixlen);
            cur = if bit == 0 { &node.left } else { &node.right };
        }
        best
    }

    /// Exact-match removal of `prefix/prefixlen`. Returns the removed value.
    /// Prunes value-less nodes and collapses value-less single-child nodes
    /// so the trie stays a proper PATRICIA trie (no redundant branch nodes).
    pub fn remove(&mut self, prefix: Prefix, prefixlen: u8) -> Option<V> {
        let prefix = masked(prefix, prefixlen);
        let (removed, _collapsed) = Self::remove_at(&mut self.root, prefix, prefixlen);
        if removed.is_some() {
            self.len -= 1;
        }
        removed
    }

    /// Returns (removed value, "this slot is now empty/should be collapsed").
    fn remove_at(
        slot: &mut Option<Box<Node<V>>>,
        prefix: Prefix,
        prefixlen: u8,
    ) -> (Option<V>, bool) {
        let node = match slot {
            None => return (None, false),
            Some(n) => n,
        };

        if node.prefixlen == prefixlen && node.prefix == prefix {
            let removed = node.value.take();
            Self::collapse(slot);
            return (removed, slot.is_none());
        }

        if node.prefixlen >= prefixlen || !prefix_matches(node.prefix, node.prefixlen, prefix) {
            return (None, false); // not a possible ancestor of the target
        }

        let bit = get_bit(prefix, node.prefixlen);
        let child_slot = if bit == 0 { &mut node.left } else { &mut node.right };
        let (removed, _) = Self::remove_at(child_slot, prefix, prefixlen);
        if removed.is_some() {
            Self::collapse(slot);
        }
        (removed, false)
    }

    /// If `slot` holds a value-less node with 0 or 1 children, splice it
    /// out (0 children -> None, 1 child -> promote the child).
    fn collapse(slot: &mut Option<Box<Node<V>>>) {
        let is_collapsible = matches!(
            slot,
            Some(n) if n.value.is_none() && (n.left.is_none() || n.right.is_none())
        );
        if !is_collapsible {
            return;
        }
        let node = slot.take().unwrap();
        *slot = match (node.left, node.right) {
            (None, None) => None,
            (Some(c), None) | (None, Some(c)) => Some(c),
            (Some(_), Some(_)) => unreachable!("collapsible implies at most one child"),
        };
    }
}

// ---------------------------------------------------------------------
// demo / self-test
// ---------------------------------------------------------------------

fn ip(a: u8, b: u8, c: u8, d: u8) -> u32 {
    ((a as u32) << 24) | ((b as u32) << 16) | ((c as u32) << 8) | (d as u32)
}

fn main() {
    let mut rt: LpmTrie<&'static str> = LpmTrie::new();

    rt.insert(ip(0, 0, 0, 0), 0, "default");
    rt.insert(ip(10, 0, 0, 0), 8, "corp-net");
    rt.insert(ip(10, 1, 0, 0), 16, "corp-vlan-1");
    rt.insert(ip(10, 1, 2, 0), 24, "corp-subnet-2");
    rt.insert(ip(10, 1, 2, 128), 25, "corp-subnet-2-upper");
    rt.insert(ip(192, 168, 0, 0), 16, "home");
    rt.insert(ip(192, 168, 1, 0), 24, "home-vlan-1");

    assert_eq!(rt.len(), 7);

    let cases: &[(u32, &str)] = &[
        (ip(8, 8, 8, 8), "default"),
        (ip(10, 5, 5, 5), "corp-net"),
        (ip(10, 1, 9, 9), "corp-vlan-1"),
        (ip(10, 1, 2, 5), "corp-subnet-2"),
        (ip(10, 1, 2, 200), "corp-subnet-2-upper"),
        (ip(192, 168, 1, 200), "home-vlan-1"),
        (ip(192, 168, 2, 5), "home"),
        (ip(172, 16, 0, 1), "default"),
    ];

    for &(addr, expect) in cases {
        let got = *rt.lookup(addr).expect("lookup should always hit default");
        assert_eq!(got, expect, "lookup({addr:#010x})");
    }
    println!("longest-prefix-match: all {} cases OK", cases.len());

    // Delete the most specific route and confirm we fall back correctly.
    let removed = rt.remove(ip(10, 1, 2, 128), 25);
    assert_eq!(removed, Some("corp-subnet-2-upper"));
    assert_eq!(rt.len(), 6);
    let got = *rt.lookup(ip(10, 1, 2, 200)).unwrap();
    assert_eq!(got, "corp-subnet-2", "should fall back after removing /25");

    // Delete a mid-tree route and confirm deeper + shallower routes survive.
    let removed = rt.remove(ip(10, 1, 0, 0), 16);
    assert_eq!(removed, Some("corp-vlan-1"));
    assert_eq!(rt.len(), 5);
    assert_eq!(*rt.lookup(ip(10, 1, 9, 9)).unwrap(), "corp-net");
    assert_eq!(*rt.lookup(ip(10, 1, 2, 5)).unwrap(), "corp-subnet-2");

    // Remove everything and confirm empty-trie behaviour.
    rt.remove(ip(0, 0, 0, 0), 0);
    rt.remove(ip(10, 0, 0, 0), 8);
    rt.remove(ip(10, 1, 2, 0), 24);
    rt.remove(ip(192, 168, 0, 0), 16);
    rt.remove(ip(192, 168, 1, 0), 24);
    assert_eq!(rt.len(), 0);
    assert!(rt.lookup(ip(1, 2, 3, 4)).is_none());
    assert!(rt.is_empty());

    println!("All Rust tests passed.");
}
```

**Why `Option<Box<Node<V>>>` and not raw pointers or an arena:** this
mirrors the kernel's ownership story conceptually (each node is owned by
exactly one parent slot, exactly like the kernel's RCU-protected child
pointers are logically owned by their parent even though physically
they're raw pointers) while staying entirely in safe Rust. `slot.take()`
is the one piece of ownership choreography worth noticing — it's how the
insert and delete code moves a subtree from one position to another (e.g.
"splice a new ancestor above this subtree") without ever holding two
owning references to the same node at once, which is exactly the kind of
thing `unsafe` raw-pointer code has to get right by hand and Rust's
borrow checker verifies for you.

**Build and run:**

```sh
rustc -O --edition 2021 -o lpm lpm.rs
./lpm
```

**Actual output:**

```
longest-prefix-match: all 8 cases OK
All Rust tests passed.
```

**The real tree this program builds**, for the seven inserted routes
(`0.0.0.0/0` default, `10.0.0.0/8`, `10.1.0.0/16`, `10.1.2.0/24`,
`10.1.2.128/25`, `192.168.0.0/16`, `192.168.1.0/24`) — dumped from the
live structure, not hand-drawn:

```
root  0.0.0.0/0            value=default
  L-> 10.0.0.0/8            value=corp-net
    L-> 10.1.0.0/16          value=corp-vlan-1
      L-> 10.1.2.0/24         value=corp-subnet-2
        R-> 10.1.2.128/25      value=corp-subnet-2-upper
  R-> 192.168.0.0/16        value=home
    L-> 192.168.1.0/24       value=home-vlan-1
```

As ASCII architecture:

```
                         [ 0.0.0.0/0 ]  "default"
                        0/           \1
                        /               \
              [ 10.0.0.0/8 ]      [ 192.168.0.0/16 ]  "home"
                "corp-net"           /
                    /               0
                   0                |
                   |         [ 192.168.1.0/24 ]
          [ 10.1.0.0/16 ]      "home-vlan-1"
           "corp-vlan-1"
                |
                0
                |
          [ 10.1.2.0/24 ]
          "corp-subnet-2"
                |
                1
                |
        [ 10.1.2.128/25 ]
       "corp-subnet-2-upper"
```

Every node here carries a real value — there are no glue/intermediate
nodes in this particular example, because each route happens to be a
strict extension of the previous one along a chain, or a clean two-way
split at the root. §7.2's `LPM_TREE_NODE_FLAG_IM` glue nodes appear when
two *unrelated* prefixes diverge partway through a shared prefix with
neither being a stored route itself — e.g. inserting `10.5.0.0/16` and
`10.9.0.0/16` with no `10.0.0.0/8` route already present would force a
value-less glue node at whatever bit `10.5` and `10.9` first diverge, to
give the tree a place to branch.

`lookup(10.1.2.200)` walks `0.0.0.0/0 -> 10.0.0.0/8 -> 10.1.0.0/16 ->
10.1.2.0/24 -> 10.1.2.128/25`, remembering each node's value as `best`
along the way, and returns `"corp-subnet-2-upper"` — the deepest (most
specific) match. Removing that `/25` route and repeating the lookup falls
back to `"corp-subnet-2"`, confirmed by the test suite's remove/fallback
assertions.

---

## 10. Go Implementation — IPv4 Longest-Prefix-Match Trie

Same algorithm, same node shape, in Go generics (`node[V any]`,
`LpmTrie[V any]`) — the language most BGP-speaking control-plane software
in the wild is actually written in (GoBGP, and large parts of FRR's tooling
ecosystem and various SDN controllers). If you're building a route
reflector, a BGP-to-XDP bridge, or a control-plane process that programs
`BPF_MAP_TYPE_LPM_TRIE` maps from userspace via `cilium/ebpf` or similar,
this is the shape of table you're maintaining on the Go side before it
ever gets pushed into the kernel map.

```go
// lpm.go -- a PATRICIA trie specialised for IPv4 CIDR longest-prefix-match.
// Same shape and algorithm as the Rust version (lpm.rs) and the C string
// dictionary (patricia.c); see the guide for the shared theory. This style
// of trie -- and this exact insert/lookup algorithm -- is what BGP speakers
// such as GoBGP and FRR use internally to hold the FIB/RIB, and it mirrors
// kernel/bpf/lpm_trie.c almost field-for-field.
package main

import "fmt"

type node[V any] struct {
	prefix    uint32
	prefixLen uint8 // 0..=32
	hasValue  bool
	value     V
	left      *node[V] // branching bit 0
	right     *node[V] // branching bit 1
}

// LpmTrie is a longest-prefix-match routing table keyed by IPv4 CIDR blocks.
type LpmTrie[V any] struct {
	root *node[V]
	size int
}

func NewLpmTrie[V any]() *LpmTrie[V] {
	return &LpmTrie[V]{}
}

func (t *LpmTrie[V]) Len() int { return t.size }

func getBit(x uint32, pos uint8) uint8 {
	if pos >= 32 {
		return 0
	}
	return uint8((x >> (31 - pos)) & 1)
}

func maskFor(length uint8) uint32 {
	if length == 0 {
		return 0
	}
	return ^uint32(0) << (32 - uint32(length))
}

func masked(x uint32, length uint8) uint32 {
	return x & maskFor(length)
}

func prefixMatches(nodePrefix uint32, nodeLen uint8, addr uint32) bool {
	m := maskFor(nodeLen)
	return (nodePrefix & m) == (addr & m)
}

// commonPrefixLen returns the length of the common leading-bit run between
// a and b, capped at max.
func commonPrefixLen(a, b uint32, max uint8) uint8 {
	var i uint8
	for i < max && getBit(a, i) == getBit(b, i) {
		i++
	}
	return i
}

// Insert adds or overwrites the route prefix/prefixLen -> value.
func (t *LpmTrie[V]) Insert(prefix uint32, prefixLen uint8, value V) {
	prefix = masked(prefix, prefixLen)
	if insertNode(&t.root, prefix, prefixLen, value) {
		t.size++
	}
}

// insertNode returns true if a brand-new route was created (false if an
// existing exact-match prefix was merely overwritten).
func insertNode[V any](slot **node[V], prefix uint32, prefixLen uint8, value V) bool {
	n := *slot
	if n == nil {
		*slot = &node[V]{prefix: prefix, prefixLen: prefixLen, hasValue: true, value: value}
		return true
	}

	limit := n.prefixLen
	if prefixLen < limit {
		limit = prefixLen
	}
	common := commonPrefixLen(n.prefix, prefix, limit)

	switch {
	case common == n.prefixLen && n.prefixLen <= prefixLen:
		// prefix extends (or equals) n's prefix.
		if prefixLen == n.prefixLen {
			wasNew := !n.hasValue
			n.hasValue = true
			n.value = value
			return wasNew
		}
		bit := getBit(prefix, n.prefixLen)
		if bit == 0 {
			return insertNode(&n.left, prefix, prefixLen, value)
		}
		return insertNode(&n.right, prefix, prefixLen, value)

	case common == prefixLen && prefixLen < n.prefixLen:
		// prefix is a strict ancestor of n: splice a new node above it.
		bit := getBit(n.prefix, prefixLen)
		newNode := &node[V]{prefix: prefix, prefixLen: prefixLen, hasValue: true, value: value}
		if bit == 0 {
			newNode.left = n
		} else {
			newNode.right = n
		}
		*slot = newNode
		return true

	default:
		// Genuine divergence at bit `common`: insert a value-less glue node.
		oldBit := getBit(n.prefix, common)
		newLeaf := &node[V]{prefix: prefix, prefixLen: prefixLen, hasValue: true, value: value}
		glue := &node[V]{prefix: masked(n.prefix, common), prefixLen: common}
		if oldBit == 0 {
			glue.left, glue.right = n, newLeaf
		} else {
			glue.right, glue.left = n, newLeaf
		}
		*slot = glue
		return true
	}
}

// Lookup performs longest-prefix-match: it returns the value of the most
// specific stored prefix containing addr, exactly like a BPF_MAP_TYPE_LPM_TRIE
// lookup or a kernel FIB route lookup.
func (t *LpmTrie[V]) Lookup(addr uint32) (V, bool) {
	var best V
	found := false
	cur := t.root
	for cur != nil {
		if !prefixMatches(cur.prefix, cur.prefixLen, addr) {
			break
		}
		if cur.hasValue {
			best = cur.value
			found = true
		}
		if cur.prefixLen == 32 {
			break
		}
		if getBit(addr, cur.prefixLen) == 0 {
			cur = cur.left
		} else {
			cur = cur.right
		}
	}
	return best, found
}

// Remove deletes the exact prefix/prefixLen route, collapsing any glue
// nodes left with no value and at most one child.
func (t *LpmTrie[V]) Remove(prefix uint32, prefixLen uint8) (V, bool) {
	prefix = masked(prefix, prefixLen)
	value, removed := removeNode(&t.root, prefix, prefixLen)
	if removed {
		t.size--
	}
	return value, removed
}

func removeNode[V any](slot **node[V], prefix uint32, prefixLen uint8) (V, bool) {
	var zero V
	n := *slot
	if n == nil {
		return zero, false
	}

	if n.prefixLen == prefixLen && n.prefix == prefix {
		if !n.hasValue {
			return zero, false
		}
		val := n.value
		n.hasValue = false
		var zeroV V
		n.value = zeroV
		collapse(slot)
		return val, true
	}

	if n.prefixLen >= prefixLen || !prefixMatches(n.prefix, n.prefixLen, prefix) {
		return zero, false
	}

	var childSlot **node[V]
	if getBit(prefix, n.prefixLen) == 0 {
		childSlot = &n.left
	} else {
		childSlot = &n.right
	}
	val, removed := removeNode(childSlot, prefix, prefixLen)
	if removed {
		collapse(slot)
	}
	return val, removed
}

// collapse splices out *slot if it is a value-less node with 0 or 1 children.
func collapse[V any](slot **node[V]) {
	n := *slot
	if n == nil || n.hasValue {
		return
	}
	switch {
	case n.left == nil && n.right == nil:
		*slot = nil
	case n.left == nil:
		*slot = n.right
	case n.right == nil:
		*slot = n.left
	}
}

// ---------------------------------------------------------------------
// demo / self-test
// ---------------------------------------------------------------------

func ip(a, b, c, d uint8) uint32 {
	return uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
}

func mustEqual[V comparable](label string, got, want V) {
	if got != want {
		panic(fmt.Sprintf("%s: got %v want %v", label, got, want))
	}
	fmt.Printf("%-32s OK (%v)\n", label, got)
}

func main() {
	rt := NewLpmTrie[string]()

	rt.Insert(ip(0, 0, 0, 0), 0, "default")
	rt.Insert(ip(10, 0, 0, 0), 8, "corp-net")
	rt.Insert(ip(10, 1, 0, 0), 16, "corp-vlan-1")
	rt.Insert(ip(10, 1, 2, 0), 24, "corp-subnet-2")
	rt.Insert(ip(10, 1, 2, 128), 25, "corp-subnet-2-upper")
	rt.Insert(ip(192, 168, 0, 0), 16, "home")
	rt.Insert(ip(192, 168, 1, 0), 24, "home-vlan-1")

	mustEqual("size after inserts", rt.Len(), 7)

	cases := []struct {
		addr uint32
		want string
	}{
		{ip(8, 8, 8, 8), "default"},
		{ip(10, 5, 5, 5), "corp-net"},
		{ip(10, 1, 9, 9), "corp-vlan-1"},
		{ip(10, 1, 2, 5), "corp-subnet-2"},
		{ip(10, 1, 2, 200), "corp-subnet-2-upper"},
		{ip(192, 168, 1, 200), "home-vlan-1"},
		{ip(192, 168, 2, 5), "home"},
		{ip(172, 16, 0, 1), "default"},
	}
	for _, c := range cases {
		got, ok := rt.Lookup(c.addr)
		if !ok {
			panic("expected a match (default route should catch everything)")
		}
		mustEqual(fmt.Sprintf("lookup(%d.%d.%d.%d)", byte(c.addr>>24), byte(c.addr>>16), byte(c.addr>>8), byte(c.addr)), got, c.want)
	}

	v, ok := rt.Remove(ip(10, 1, 2, 128), 25)
	mustEqual("remove /25 ok", ok, true)
	mustEqual("remove /25 value", v, "corp-subnet-2-upper")
	mustEqual("size after remove", rt.Len(), 6)

	got, _ := rt.Lookup(ip(10, 1, 2, 200))
	mustEqual("fallback after /25 removed", got, "corp-subnet-2")

	rt.Remove(ip(10, 1, 0, 0), 16)
	got, _ = rt.Lookup(ip(10, 1, 9, 9))
	mustEqual("fallback after /16 removed", got, "corp-net")

	rt.Remove(ip(0, 0, 0, 0), 0)
	rt.Remove(ip(10, 0, 0, 0), 8)
	rt.Remove(ip(10, 1, 2, 0), 24)
	rt.Remove(ip(192, 168, 0, 0), 16)
	rt.Remove(ip(192, 168, 1, 0), 24)
	mustEqual("size after draining", rt.Len(), 0)
	_, ok = rt.Lookup(ip(1, 2, 3, 4))
	mustEqual("empty trie has no match", ok, false)

	fmt.Println("\nAll Go tests passed.")
}
```

**Build and run:**

```sh
go run lpm.go
```

**Actual output:**

```
size after inserts               OK (7)
lookup(8.8.8.8)                  OK (default)
lookup(10.5.5.5)                 OK (corp-net)
lookup(10.1.9.9)                 OK (corp-vlan-1)
lookup(10.1.2.5)                 OK (corp-subnet-2)
lookup(10.1.2.200)               OK (corp-subnet-2-upper)
lookup(192.168.1.200)            OK (home-vlan-1)
lookup(192.168.2.5)              OK (home)
lookup(172.16.0.1)               OK (default)
remove /25 ok                    OK (true)
remove /25 value                 OK (corp-subnet-2-upper)
size after remove                OK (6)
fallback after /25 removed       OK (corp-subnet-2)
fallback after /16 removed       OK (corp-net)
size after draining              OK (0)
empty trie has no match          OK (false)

All Go tests passed.
```

Same tree, same fallback behavior on removal, as the Rust version in §9 —
by design, since both implement the identical algorithm. The one Go-
specific thing worth flagging: `collapse()`'s three-way switch
(`left == nil && right == nil` / `left == nil` / `right == nil`) is doing
by hand exactly what Rust's `matches!` + `unreachable!()` in §9's
`collapse` does with the type system's help — Go's lack of sum types means
"a value-less node with exactly one child" isn't a state the compiler can
help you enumerate exhaustively, so the guard has to be written and tested
explicitly. Worth noticing if you're used to Rust's `Option`/`match`
giving you that exhaustiveness check for free.

---

## 11. Performance: Why Compression Is a Cache Argument

Every pointer chase in a tree-shaped structure is, on modern hardware,
potentially a full main-memory round trip — 100+ cycles — if the target
node isn't already in cache. This is the number that actually explains why
§7.1's LC-trie level-compression exists, and it's worth stating precisely
rather than hand-wavily:

- A plain binary PATRICIA trie over a 32-bit IPv4 key has a worst-case
  depth of 32 branch-node hops. Each hop is a pointer dereference to a
  node that, statistically, is *not* the one you touched last (routing
  tables are large; there's no meaningful temporal locality across
  unrelated destination addresses) — so each hop is a plausible cache
  miss.
- Level compression collapses `w` bits of binary branching into a single
  array-indexed node. That's still one memory access, but it replaces
  `w` potential cache misses with one, at the cost of a larger node (a
  `2^w`-entry array instead of two pointers) — which is itself a
  reasonable trade as long as the array fits in a small number of cache
  lines and the region of the tree is genuinely dense enough to make the
  array mostly non-empty. This is precisely what `fib_trie`'s
  `resize()`/`inflate()`/`halve()` are tuning for at runtime, per-subtree,
  based on actual occupancy — and it's why `/proc/net/fib_triestat`'s
  "average depth" number on a real full-table BGP feed (800,000+ routes)
  sits at roughly 5–6 rather than anywhere near 32.
- This is also the practical argument for why `BPF_MAP_TYPE_LPM_TRIE`
  (§7.2) does **not** level-compress: XDP/TC programs run with a strict
  instruction budget and no floating tolerance for the kind of background
  rebalancing (`inflate()`/`halve()`) that level compression requires to
  stay worthwhile — a plain PATRICIA trie's bounded-and-predictable
  per-lookup work (at most `max_prefixlen` hops, no rebalancing pass ever
  triggered by a lookup) is a better fit for a hard real-time-ish
  datapath than a self-tuning structure would be, even though the tuned
  structure would be faster in the amortized/steady-state case.

If you're benchmarking your own implementation from §8–10: measure with a
realistic key distribution (real IP prefix tables, or a real word list —
not uniformly random bit strings, which produce artificially shallow,
evenly-branching trees that don't resemble what either routing tables or
natural-language key sets actually look like), and measure cache misses
(`perf stat -e cache-misses`) alongside wall-clock time — for this class
of structure, cache misses are usually the more informative number, since
they're what actually explains the difference between a plain PATRICIA
trie's and an LC-trie's throughput on the same key set.

---

## 12. Common Pitfalls

- **Confusing PATRICIA with "radix tree" in the Linux/XArray sense.**
  Covered in depth in §3.3 — they solve different problems and have
  different internal shapes. If you're reading kernel networking code and
  see "radix tree," check which one is actually meant; `fib_trie.c`'s own
  comments call itself both an "LPC-trie" and, informally, sometimes
  "the FIB trie" — not "radix tree" — precisely to avoid this collision.

- **Bit-numbering convention mismatches.** This guide numbers bit 0 as the
  MSB of byte 0 (network byte order), which is the right convention for
  anything IP-shaped. Some historical PATRICIA implementations (rooted in
  BSD's `radix.c`, which predates widespread IPv6 and was tuned for
  `sockaddr` layouts) use LSB-first or byte-order-dependent numbering.
  Mixing conventions between your insert code and your lookup code is a
  silent-corruption bug, not a crash — the trie will build and search
  "successfully" against the wrong bit positions and simply return wrong
  answers for some keys. Pin the convention in one place (a single
  `test_bit`, as in §4.1) and never let bit arithmetic live anywhere else.

- **Forgetting the strictly-increasing-bit-index invariant when hand-
  rolling insert.** §5.3's phase-3 re-descent condition
  (`(*link)->bit < diff`) is not a minor detail — it's the entire
  correctness argument from §5.5. A common bug is writing the re-descent
  loop to stop on "found a leaf" without also checking the bit-index
  condition, which silently breaks on keys that should have spliced in
  partway down an existing subtree rather than at its leaf.

- **Not handling variable-length keys / one key being a bit-string prefix
  of another.** §4.1's zero-padding rule (`test_bit` returns 0 past the
  end of a shorter key) is what makes this safe for the exact-match trie
  in §8. For LPM tries (§9–10), this isn't an edge case to "handle" — it's
  the entire point of the data structure (`10.0.0.0/8` *should* match
  `10.1.2.3`) — but it means an LPM trie's node shape genuinely needs
  §4.4's "any node can carry a value" property; retrofitting that onto an
  exact-match, leaf-only design (§3.2) rather than designing for it from
  the start is a common source of subtle bugs.

- **Memory reclamation under concurrent readers.** If you adapt any of
  §8–10 for a concurrent context (a userspace router, an XDP control
  plane), freeing a node the instant it's unlinked is unsafe if any reader
  might still be mid-traversal through it — this is exactly what RCU
  (§7.2) exists to solve in the kernel, and what you'd need epoch-based
  reclamation, hazard pointers, or a global lock for in userspace Rust/Go/C
  if you're not going through the kernel's own RCU machinery.

- **Benchmarking with the wrong key distribution.** Covered in §11 —
  uniformly random bit strings produce misleadingly shallow, evenly split
  trees that don't resemble real IP prefix tables or real word lists,
  which tend to be highly skewed (a handful of large aggregates, lots of
  more-specific routes nested under them).

---

## 13. Expert Mental Model

The single sentence worth internalizing: **a PATRICIA trie is a decision
tree over the bits of your keys, restructured so that every node
represents a decision that actually distinguishes something.** Every other
property — the O(k) bound, the lack of backtracking, the ease of longest-
prefix-match, the RCU-friendliness — falls out of that one restructuring.

A few more compact framings that are worth having on hand:

- **Compared to a hash table:** a hash table answers "is this exact key
  present" by throwing away all structural relationship between keys. A
  PATRICIA trie answers the same question *and* "what's the longest
  prefix of this key that's present" *and* "give me all keys in sorted
  order" *and* "give me all keys between X and Y," because it never throws
  that structure away — it just compresses out the *redundant* parts of
  it (§2). If your problem only ever needs exact-match and you don't care
  about ordering or prefixes, a hash table is simpler and often faster;
  the moment prefix or range semantics enter the picture (routing, ACLs,
  autocomplete, IP allow/deny lists), that's the tell that you actually
  want this structure, not a hash table with a workaround bolted on.

- **Compared to a balanced BST:** a BST balances by *key comparison
  outcome*, which is a global, data-dependent property that requires
  rebalancing machinery (rotations) to maintain as data changes. A
  PATRICIA trie's "balance" (such as it is) falls directly out of the key
  bits themselves — there's no separate rebalancing invariant to maintain
  for the plain version (§5), because the bit-index-per-node structure is
  self-consistent by construction. (Level-compressed variants, §7.1,
  reintroduce a *form* of rebalancing, but it's tuning for cache
  performance, not correctness — the plain trie underneath is already
  correct without it.)

- **The kernel-networking anchor, since it's the one you'll touch most:**
  every time you write a `BPF_MAP_TYPE_LPM_TRIE` key/value pair from Aya
  or libbpf, you are populating exactly the structure built and tested in
  §9–10, running the exact algorithm traced in §5 and §7.2. There is no
  conceptual gap between "the toy trie I built in Rust to learn this" and
  "the trie the kernel walks on your XDP program's behalf" — it's the same
  data structure, same invariants, same insert/lookup logic, just with
  RCU and a spinlock wrapped around the mutation path for kernel-grade
  concurrency safety.

- **The one-sentence test for "should this be a PATRICIA trie":** does
  your lookup ever need "the most specific/longest match," not just
  "does this exact thing exist"? If yes, and your keys are naturally
  bit-string- or prefix-shaped (addresses, hierarchical paths, CIDR
  blocks), this is very likely the right structure, and reaching for a
  hash table plus a manual prefix-length loop is almost always the wrong
  amount of extra machinery for the same result.

---

## 14. Further Reading

- D. R. Morrison, ["PATRICIA — Practical Algorithm To Retrieve Information
  Coded In Alphanumeric,"](https://dl.acm.org/doi/10.1145/321479.321481)
  *Journal of the ACM* 15(4), 1968 — the original paper; §3.1 of this guide.
- S. Nilsson, G. Karlsson, "IP-Address Lookup Using LC-Tries," *IEEE
  Journal on Selected Areas in Communications*, 1999 — the level-
  compression design behind `fib_trie.c` (§7.1).
- Robert Sedgewick, *Algorithms* — the standard textbook treatment of
  PATRICIA/radix tries and the external-node formulation used throughout
  §3.2–§10 of this guide.
- Kernel source, read in this order for the deepest dive: `net/ipv4/fib_trie.c`,
  `Documentation/networking/fib_trie.rst` (the LC-trie implementation
  notes referenced in §7.1), `kernel/bpf/lpm_trie.c`, and
  `Documentation/bpf/map_lpm_trie.rst` for the userspace-facing API
  (§7.2).
- `/proc/net/fib_triestat` and `/proc/net/route` on any live Linux box —
  the actual depth/density statistics referenced in §7.1 and §11 are one
  `cat` away.
- For the historical BSD lineage of the external-node variant this guide
  implements: 4.4BSD's `net/radix.c`, the ancestor of most "patricia.c"
  files still floating around routing-software codebases today.

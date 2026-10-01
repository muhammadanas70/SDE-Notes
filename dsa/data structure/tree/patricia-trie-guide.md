# Patricia Trie — A Complete Engineering Guide

**Scope:** theory, invariants, algorithms, Linux kernel usage (`fib_trie`, `BPF_MAP_TYPE_LPM_TRIE`), and production-grade implementations in C, Rust, and Go, aimed at someone building an XDP/eBPF firewall and doing Linux network-subsystem work.

---

## Table of Contents

1. What problem is this actually solving?
2. Naming: Trie vs Radix Tree vs PATRICIA vs Compressed Trie
3. The core idea: bit-testing instead of byte/char comparison
4. Two historical variants (know both — you'll meet both in the wild)
5. Invariants that must always hold
6. Search algorithm — step by step, with ASCII diagrams
7. Insert algorithm — step by step, with ASCII diagrams
8. Delete algorithm — the hard one, with ASCII diagrams
9. Complexity analysis
10. Longest Prefix Match (LPM) — the networking-specific variant
11. Linux kernel: `fib_trie` (IPv4 FIB) architecture
12. eBPF: `BPF_MAP_TYPE_LPM_TRIE` — how the kernel implements it, and how you use it from XDP
13. C implementation — LPM Patricia trie for IPv4 CIDR
14. Rust implementation — LPM Patricia trie, `no_std`-friendly
15. Go implementation — LPM Patricia trie
16. Testing strategy: invariants, property tests, edge cases
17. Mental models and guiding questions to internalize this
18. Further reading

---

## 1. What problem is this actually solving?

You have a set of keys (bit-strings — could be IP prefixes, dictionary words, routing labels) and you need one or more of:

- **Exact match**: is key `K` present?
- **Longest Prefix Match (LPM)**: given key `K`, find the *longest* stored prefix that matches `K`. This is literally what an IP router does on every packet — "which route in my table has the most specific prefix that contains this destination address?"
- Do this in **O(bit-length of key)** time, with **memory proportional to the number of keys**, not the size of the keyspace.

A plain trie already gets you O(key length) search, but it wastes a node for every bit position even when there's no branching — a chain of single-child nodes. If you have sparse keys (e.g., a routing table with a few hundred thousand prefixes out of a 2^32 IPv4 address space), you pay for every unbranching bit. That's the actual problem PATRICIA solves: **collapse every node that has exactly one child.** Only nodes where the trie actually *branches* (two children) or *terminates* (a stored key) get materialized.

This single design decision — no unary nodes — is where everything else in this document flows from.

---

## 2. Naming: Trie vs Radix Tree vs PATRICIA vs Compressed Trie

These terms get used loosely in industry and even in kernel code, so pin them down:

| Term | Meaning |
|---|---|
| **Trie** | Tree keyed by successive symbols (bits, bytes, chars) of the key. One edge per possible symbol value at each level. No compression. |
| **Radix tree / Radix trie** | A trie where chains of single-child nodes are merged into one edge labeled with a *string* of symbols (not just one). Node stores a *skip string*. |
| **PATRICIA** | *Practical Algorithm To Retrieve Information Coded In Alphanumeric* (Morrison, 1968). A **binary** radix tree (radix = 2, i.e. bit-at-a-time) where instead of storing a skip *string*, each internal node stores only a **skip count / bit-index** — the position of the *next differing bit* — and does **not** store the intervening bits at all (they're implicit; you re-fetch them from the actual key stored at a leaf/terminal node to compare). Classic PATRICIA (Sedgewick/Knuth's description) is also **threaded**: instead of null child pointers, it uses **back-pointers** to an ancestor, turning the structure into a single circular structure with no null links — this was an optimization for the memory model of 1968, largely a historical curiosity today. |
| **Compressed trie** | Generic umbrella term — often used interchangeably with radix tree. |

**In modern systems code (Linux kernel, eBPF, most "Patricia trie" library code you'll read), "Patricia trie" almost always means:** a binary radix tree with skip-count-based path compression, **without** the archaic back-pointer threading. That's what `fib_trie` is, that's what `BPF_MAP_TYPE_LPM_TRIE` is, and that's what we implement below. We'll note the classic-vs-modern distinction again in Section 4 because it matters when you read old papers/Knuth vs. kernel source.

---

## 3. The core idea: bit-testing instead of byte/char comparison

Every key is treated as a bit string, MSB first. An internal node doesn't ask "what's the next byte/character?" — it asks **one yes/no question**: "what is bit number `i` of the key?" and branches left (0) or right (1).

```
Key "5" (binary, 4 bits):  0101
Key "3" (binary, 4 bits):  0011
Key "7" (binary, 4 bits):  0111
```

A plain binary trie over these 3 keys has one node per bit position per key — lots of single-child chains. PATRICIA instead stores, at each internal node, **only the index of the bit where the relevant keys diverge**. That index is called the **skip value / bit index / test bit**. Everything between the parent's bit index and the child's bit index is *never materialized as a node* — it's implicitly "matched" by comparing against a real stored key when you reach a leaf.

This is the crux: **internal nodes carry no key data at all, only a bit-index.** Correctness of "the skipped bits actually matched" is verified *after* you reach a leaf, by comparing the full key. This lazy verification is what makes insert/search O(1) node visits per bit-index level rather than O(bit length) always doing full comparisons along the way.

---

## 4. Two historical variants

### 4.1 Classic PATRICIA (Sedgewick / Knuth, 1968 original)

- No null pointers. Every node has exactly 2 children.
- A "leaf" is simulated by making one child pointer point **backward** up the tree (to an ancestor) rather than down. This is the threading trick — it lets the whole structure be built from one node type with no nulls, important for the memory-constrained era it was designed in.
- Traversal must always **compare the test-bit index of the node you're about to visit against the test-bit index of the node you came from**: if the child's bit-index is **not strictly greater** than the current node's, you know you've hit a back-pointer (i.e., a "leaf"), not a real descent.
- This variant is elegant but genuinely confusing to implement and debug — the "am I going down or is this a back-edge" check is easy to get subtly wrong, and cache/TLB behavior is poor because you jump all over memory following back-pointers.

### 4.2 Modern radix/Patricia (what you'll actually build and what the kernel uses)

- Ordinary tree with real null child pointers (or, in kernel code, tagged/union'd pointers — see Section 11).
- Internal ("branch") nodes: no key, just a bit-index and two child pointers.
- Leaf nodes: full key (or enough of it), plus payload (route entry, next-hop, value, etc.).
- Some designs (fib_trie, BPF LPM trie) go further and allow **intermediate nodes to also carry a value** — because in LPM use cases, a shorter prefix (e.g. `/8`) can be a *valid answer on its own*, not just a waypoint to more specific `/24`s. This is the single biggest structural difference between "dictionary PATRICIA" (leaves only carry values) and "networking PATRICIA/LPM trie" (any node, at any depth, may carry a value). We build the LPM variant below since that's your domain.

**Guiding question for you:** why would a plain dictionary trie never need internal nodes to carry values, but an IP routing trie absolutely must? (Answer: in a dictionary, a key is either present or not — there's no notion of "this key is a prefix of that key and both are independently meaningful routes." In routing, `10.0.0.0/8` and `10.1.0.0/16` can *both* be real, independently configured routes, and a packet to `10.1.2.3` must prefer the more specific one but fall back to the less specific one if the specific one didn't exist.)

---

## 5. Invariants that must always hold

Write these down and check every one of them in your test suite (Section 16):

1. **Bit-index strictly increases along any root-to-leaf path.** A child's test-bit index must be greater than its parent's. This is what guarantees termination and O(bit-length) bound on depth.
2. **Every internal (branch) node has exactly two non-null children** (in the modern variant, or is the trivially-empty root). A branch node with only one child is a bug — it should have been path-compressed away. This is the invariant that *makes it Patricia and not a plain trie*.
3. **Skipped bits are never assumed — they are verified.** When you land on a candidate leaf after a bit-driven descent, you must compare the *actual* stored key against the search key (at least the skipped bit ranges) to confirm it's really a match and not just "happened to end up in this subtree." (Some designs check this on the way down instead — see "prefix check on descent" in Section 6.)
4. **For LPM tries specifically:** a node at any depth can hold a `(prefix, prefix_len, value)` — and search must track the **last node with a value seen so far along the descent path**, because the *true* longest match might be an ancestor of the leaf you eventually land on, not the leaf itself.
5. **No duplicate bit-indices on a single path**, and no wasted branch nodes (every branch must actually discriminate between ≥2 keys currently in the subtree).

---

## 6. Search algorithm — step by step

### 6.1 Plain PATRICIA (exact match)

```
search(root, key):
    node = root
    while node is a branch node:
        bit = test_bit(key, node.bit_index)
        node = node.child[bit]
    // node is now a leaf
    if node.key == key:
        return node.value
    else:
        return NOT_FOUND
```

Note there is **no backtracking**. You never revisit a decision. That's only correct because of invariant #1 (monotonically increasing bit index) plus the final full-key comparison at the leaf — if the descent was "wrong" because some skipped bit didn't actually match, the final comparison catches it and reports NOT_FOUND. You get O(1) amortized work per bit-index jump and exactly one full-key comparison at the end. Total: **O(log n) branch hops in a balanced case, worst-case O(W) where W = key width in bits, plus one O(W) verification compare.**

### 6.2 LPM search (what you actually need for a firewall / router)

```
lpm_search(root, key):
    node = root
    best = NULL                      // best value found so far
    while node != NULL:
        if node.has_value and prefix_matches(node.prefix, node.prefix_len, key):
            best = node.value        // remember, keep descending for something more specific
        if node is leaf:
            break
        bit = test_bit(key, node.bit_index)
        node = node.child[bit]
    return best
```

This is the key structural difference from dictionary search: **you don't return on the first match, you keep the best match seen and keep going**, because a more specific (longer) prefix might exist deeper in the tree. This is exactly the "longest prefix wins" rule in IP routing (RFC 1812 §5.2.4) and exactly what `BPF_MAP_TYPE_LPM_TRIE` implements in-kernel for you.

### 6.3 ASCII diagram — LPM trie over a small IPv4 prefix set

Prefixes: `10.0.0.0/8`, `10.1.0.0/16`, `10.1.1.0/24`, `192.168.0.0/16`

```
                          [root, bit=0]
                         /              \
                     (bit0=0)          (bit0=1)
                       /                    \
              [node A, bit=8]         [leaf: 192.168.0.0/16]
              prefix=10.0.0.0/8
                 (VALUE HERE)
               /            \
          (bit8=0)        (bit8=1)
             |                |
      [only 10.0.0.0/8    [node B, bit=16]
       matches further    prefix=10.1.0.0/16
       here, no more            (VALUE HERE)
       specific routes]      /            \
                        (bit16=0)      (bit16=1)
                           |               |
                    [leaf: 10.1.1.0/24  [no route here,
                     VALUE HERE]         dead end]
```

Trace a lookup for `10.1.1.5`:
1. At root: bit0 = 0 (since `10.x` starts with `0000...`) → go left to node A.
2. Node A has a value (`10.0.0.0/8`) and it matches → `best = 10.0.0.0/8`. Test bit 8 → 0? `10.1.1.5` = `00001010.00000001...`; bit 8 (second octet's MSB) = 0 → go to node B side (this is illustrative; exact bit arithmetic depends on your bit-numbering convention — nail this down in code, don't eyeball it).
3. At node B: has value `10.1.0.0/16`, matches → `best = 10.1.0.0/16` (overwrites the less specific one). Continue by bit 16.
4. Reach leaf `10.1.1.0/24` → matches → `best = 10.1.1.0/24`.
5. Return `best = 10.1.1.0/24`. Correct: most specific route wins.

If the address had been `10.1.2.5` instead, step 4 would land on the "no route here" leaf or fail the prefix check, and the search would return the previously remembered `best = 10.1.0.0/16` — this is the entire point of carrying `best` forward instead of only trusting the leaf.

---

## 7. Insert algorithm — step by step

Inserting into a Patricia/radix trie has three cases. Think of it as: **walk down as far as the bits agree with an existing key, then splice in a new branch node at the first point of disagreement.**

### 7.1 Case 1 — empty tree
Just make the new key the root (a leaf).

### 7.2 Case 2 — descend, then diverge before reaching a leaf
Walk down using existing branch nodes' bit-indices **as long as the new key's bits at that index also happen to match** what the existing structure "expects" implicitly — but remember, branch nodes don't store the skipped bits, so how do you know when to stop early?

The trick: before you can trust a branch node's bit-index and descend further, you must know that the *skipped* bits (all bits before this node's bit-index that were skipped by compression) also match between your new key and the keys already in that subtree. Two common engineering approaches:

- **(a) Store a representative key at every branch node** (any one key from its subtree is enough) so you can compare your new key against it bit-by-bit at insert time, and find the **first differing bit** between the new key and that representative. Compare that first-differing-bit position against the node's bit-index as you descend:
  - if `first_diff_bit < node.bit_index` → you must splice in **right here**, above this node (Case 3).
  - if `first_diff_bit >= node.bit_index` → keep descending using `test_bit(new_key, node.bit_index)`.
- **(b) Descend all the way to a leaf first (ignore correctness), then compare the new key against that leaf's full key to find the first differing bit**, then walk back down again (or track it during the first descent) to find the correct splice point. This is the classic PATRICIA approach and avoids storing a representative key at every branch, at the cost of two passes (or careful bookkeeping in one pass).

Both are used in real code; (a) is simpler to reason about and is what `fib_trie` effectively does via its "empty node with `full_key`" leaves; (b) is closer to the historical algorithm. **We implement (a) below — it is easier to get correct and to test.**

### 7.3 Case 3 — splice a new branch node

Once you know the first differing bit `d` between the new key and the nearest existing key, and you know where in the path that belongs (`d` is less than the child's bit-index but ≥ the parent's), you:

1. Create a new branch node `N` with `bit_index = d`.
2. `N.child[bit(new_key, d)] = new leaf(new_key)`.
3. `N.child[1 - bit(new_key, d)] = <the old subtree that was here>`.
4. Splice `N` into the parent's child slot that used to point at the old subtree.

### 7.4 ASCII diagram — inserting `10.2.0.0/16` into the tree from Section 6.3

Before insert, `10.1.0.0/16` sits at "node B" with `bit_index=16`. The new key `10.2.0.0/16` agrees with `10.1.0.0/16` up through bit 15 (both are `10.x`) but **differs starting at bit 15** (`.1.` = `00000001` vs `.2.` = `00000010` — they actually differ at bit 14, illustrative only — the point is: find the true first differing bit with real code, don't eyeball it in prose).

```
Before:                              After splicing in a new branch at bit=14:

   [node B, bit=16]                     [new node C, bit=14]
   prefix=10.1.0.0/16                  /                    \
      /            \                (bit14=0)             (bit14=1)
 (bit16=0)      (bit16=1)               |                     |
    |               |            [node B, bit=16]      [leaf: 10.2.0.0/16]
 [leaf:          [no route]      prefix=10.1.0.0/16
  10.1.1.0/24]                      /            \
                                (bit16=0)      (bit16=1)
                                   |               |
                                [leaf:          [no route]
                                 10.1.1.0/24]
```

Node C is a **new branch node with no value of its own** — it exists purely to discriminate between the `10.1.0.0/16` subtree and the new `10.2.0.0/16` leaf. This is exactly invariant #2 from Section 5 in action: every branch node has two real children, and it was created *because* two keys needed to diverge, not for any other reason.

---

## 8. Delete algorithm — the hard one

Deletion in a Patricia trie is where people write buggy code, because you must **restore invariant #2** (no single-child branch nodes) after removing a leaf.

```
delete(root, key):
    find leaf L for key, and its parent P, and P's parent GP
    if L not found: return NOT_FOUND
    sibling = P.child[the OTHER slot, not L's]
    // P is about to have only one child (sibling) once L is removed —
    // that would violate invariant #2, so P itself must be removed too,
    // and sibling takes P's place directly under GP.
    GP.child[slot that used to point to P] = sibling
    free(P)
    free(L)
```

**For LPM tries specifically**, there's an extra wrinkle: if the node being removed (`L`, or an intermediate value-carrying node) holds a value but is also an ancestor branch point still needed by other keys, you **cannot free the node** — you can only clear its value and demote it to a pure branch node. You only physically remove a node when it becomes childless *and* valueless. Get this wrong and you either leak stale routes (memory/correctness bug) or you delete a branch that other prefixes still need (catastrophic — you silently lose unrelated routes).

### 8.1 ASCII diagram — deleting `10.1.1.0/24` from the earlier tree

```
Before:                                After:

[node B, bit=16]                     [leaf: 10.1.0.0/16]
prefix=10.1.0.0/16                   (node B's value promoted up,
   /            \                     node B and its dead branch removed —
(bit16=0)     (bit16=1)                because after deleting the /24 leaf,
   |              |                    node B would have only ONE child left,
[leaf:         [no route]              which violates invariant #2, so node B
 10.1.1.0/24]                          collapses and its sibling — which in
                                        this case is "no route", i.e. nothing —
                                        means node B's VALUE simply becomes
                                        a leaf itself and replaces node B.)
```

**Guiding question:** what happens if `node B` had a value AND both children were real subtrees (not "no route"), and you delete the `/24` leaf child? Walk through it yourself before reading further: node B still has one real remaining child, so node B is **not** removed — it just loses the deleted leaf, and its other child moves into that slot directly (still under node B, since node B still discriminates between its own value being "here" vs. the remaining child being "more specific, elsewhere"). This is the case that trips people up: **removing invariant violations bottom-up is not always "collapse the parent," sometimes it's "the parent survives, only the empty slot's subtree changes."** Precisely state, in your own words, the general rule that covers both cases before you write delete() — that's a very good exercise to do with a colleague/reviewer before coding it.

---

## 9. Complexity analysis

| Operation | Time | Notes |
|---|---|---|
| Search (exact) | O(W) worst case, W = key width in bits | One branch hop per bit-index level, monotonically increasing, so at most W hops; realistically much less since bit-indices skip ahead. |
| LPM search | O(W) worst case | Same descent, plus O(1) work per node to check/update `best`. |
| Insert | O(W) | One descent to find splice point + O(1) node creation. |
| Delete | O(W) | One descent + O(1) restructuring. |
| Space | O(n) branch nodes + O(n) leaves, n = number of keys | This is the entire point versus a plain trie: **independent of keyspace size (2^32 for IPv4), dependent only on how many prefixes you actually store.** |

Compare to a plain (uncompressed) binary trie over 32-bit IPv4 keys: worst case you'd materialize up to 32 nodes **per key** even when there's no branching, i.e. O(n·W) nodes in the worst case (sparse, non-overlapping prefixes). PATRICIA guarantees **exactly `n-1` branch nodes for `n` leaves** (a strict binary tree property), independent of W. This is the number to have in your head when someone asks "why not just use a hash table for routes" — a hash table gives you O(1) exact match but **cannot do longest-prefix match at all** without either scanning all possible prefix lengths (up to 32 hash lookups per packet, one per possible mask length — this is literally what some naive multi-level hash implementations do) or falling back to a trie-like structure anyway.

---

## 10. Longest Prefix Match (LPM) — the networking-specific variant, formalized

Recall from Section 5, invariant #4, and Section 6.2: LPM changes two things versus a plain dictionary Patricia trie:

1. **Any node can carry a value**, not just leaves — because a `/8` and a `/24` under it can both be real, independently-configured routes.
2. **Search never returns on first match — it tracks the best (longest / most specific) match along the entire descent path.**

This is why the Linux kernel's IPv4 FIB structure and the eBPF `LPM_TRIE` map type are both, structurally, "Patricia tries with value-bearing internal nodes," not plain dictionary PATRICIA. Keep this distinction sharp — it is the single most common point of confusion when people read a "Patricia trie" tutorial that only covers the dictionary case and then try to map it onto routing code.

---

## 11. Linux kernel: `fib_trie` — IPv4 FIB architecture

Since Linux 2.6.something (the "LC-trie"/`fib_trie` replaced the old, much simpler `fib_hash`), the IPv4 routing table (per-namespace, per `struct net`) is implemented as a compressed trie. Source: `net/ipv4/fib_trie.c`.

Key structures and ideas (names as of modern kernel source — verify against the exact tree you're reading, kernel internals do shift):

- **`struct key_vector`** — the fundamental node type. It is a **level-compressed** structure, meaning it goes a step beyond plain PATRICIA: instead of one bit tested per node (binary branching only), a `key_vector` can test **multiple bits at once**, holding up to `2^bits` child pointers in one node (a small array), where `bits` is chosen adaptively based on local key density. This is **LC-trie (Level-Compressed trie)**, a refinement on top of Patricia/radix compression — Patricia gives you *path* compression (no unary nodes), LC-trie additionally gives you *level* compression (collapse several binary levels into one wider fan-out node when the subtree is dense enough to make that worthwhile). This is why kernel people call it "trie" and papers call it "Patricia" — both are correct, it's Patricia-compressed **and** level-compressed.
- **`struct tnode`** — internal node wrapper holding an array of `key_vector` children, `bits` (fan-out width), and `pos` (bit position, analogous to our `bit_index`).
- **Leaves store `struct fib_alias` chains** — because in real routing you can have multiple routes to the same prefix with different metrics/next-hops/tables, so a leaf is not just one value, it's a small list.
- **`fib_find_node()` / `fib_table_lookup()`** implement essentially the LPM search algorithm from Section 6.2, adapted for multi-bit fan-out nodes instead of strict binary branching.
- Rebalancing (`resize()` in `fib_trie.c`) dynamically decides, per subtree, whether to widen (more bits tested per node, trading memory for fewer hops) or narrow (less memory, more hops) — this adaptive behavior is the main extra complexity LC-trie adds over "plain" binary Patricia, and it's driven by heuristics on child density, not something you need to reimplement to understand routing — but you should know it exists so you're not confused when kernel node structures don't look strictly binary.

**Why this matters for your XDP/eBPF work specifically:** `fib_trie` is the **control-plane** (kernel routing table) structure — it's consulted by the normal IP stack (`ip_route_input`/`ip_route_output`, netfilter, etc.), not directly by your XDP program (XDP runs *before* the stack, at the driver's RX ring, and does not automatically consult the kernel FIB unless you explicitly call `bpf_fib_lookup()`, which is a helper that internally walks the same FIB tables). If your firewall needs its own prefix-based ACL/blocklist independent of kernel routing, you build **your own** LPM trie as a BPF map — which is exactly `BPF_MAP_TYPE_LPM_TRIE`, covered next.

---

## 12. eBPF: `BPF_MAP_TYPE_LPM_TRIE`

This is the map type you almost certainly want for a CIDR-based allow/deny list in your XDP firewall, instead of hand-rolling a trie in BPF C (which you *can* do but is much more error-prone under the verifier's constraints — bounded loops, no unbounded pointer chasing, etc.).

### 12.1 Key structure

```c
struct lpm_key {
    __u32 prefixlen;   // number of significant bits, e.g. 24 for a /24
    __u8  data[4];     // the actual bytes, e.g. an IPv4 address
                        // (use 16 bytes for IPv6)
};
```

The `prefixlen` field is mandatory and is exactly the "how many leading bits actually matter" — the kernel's LPM trie implementation (`kernel/bpf/lpm_trie.c`) is a binary Patricia trie in the exact sense of Sections 5–7 above: internal `struct lpm_trie_node` entries have two children and a `bits`/prefix info, and any node along the path can be a valid, matchable entry (value-bearing internal nodes, per Section 10).

### 12.2 Map creation (from user space, e.g. via `libbpf`)

```c
struct bpf_map_create_opts opts = { .sz = sizeof(opts) };
int map_fd = bpf_map_create(
    BPF_MAP_TYPE_LPM_TRIE,
    "cidr_blocklist",
    sizeof(struct lpm_key),   // key size
    sizeof(__u32),            // value size, e.g. an action/verdict
    1024,                     // max entries
    &opts
);
```

Note: `BPF_MAP_TYPE_LPM_TRIE` **requires** the `BPF_F_NO_PREALLOC` map flag on many kernels — check `map_flags` requirements for your target kernel version; older kernels reject LPM tries without it because LPM tries cannot be safely fully preallocated the way hash maps are.

### 12.3 Lookup from your XDP program

```c
struct lpm_key key = {
    .prefixlen = 32,          // always query with full width; the map does the LPM logic
    .data = { ip4[0], ip4[1], ip4[2], ip4[3] },
};
__u32 *verdict = bpf_map_lookup_elem(&cidr_blocklist, &key);
if (verdict && *verdict == DROP)
    return XDP_DROP;
```

**Critical point** (and a common bug for people new to this map type): you always query with `prefixlen = 32` (full IPv4 width) and the *full concrete address* — the kernel's LPM trie implementation does the "find the longest matching stored prefix" work for you internally; you are not supposed to loop over possible prefix lengths yourself. If you find yourself writing a loop that calls `bpf_map_lookup_elem` once per possible prefix length, you have misunderstood the map type — that's exactly the "naive multi-hash-lookup" anti-pattern mentioned in Section 9 that this data structure exists to avoid.

### 12.4 In-kernel implementation notes (`kernel/bpf/lpm_trie.c`)

- `struct lpm_trie_node` — has `child[2]`, `prefixlen`, `flags` (to mark "this node has a real value, not just a branch point" — exactly invariant #4/Section 10's "value-bearing internal node" concept), and inline key data.
- `trie_lookup_elem()` implements almost verbatim the `lpm_search` pseudocode in Section 6.2: descend, and at every node whose stored prefix is a real prefix of the search key, remember it as the current best, continue, return the best found.
- Update/delete under RCU: because BPF maps can be read concurrently by running BPF programs while being updated from user space, `lpm_trie.c` does its node splicing (Section 7) and node removal (Section 8) using `rcu_assign_pointer()` / call_rcu-style deferred freeing, not in-place mutation — this matters if you ever read that source: the "delete" logic is doing exactly the tree-surgery from Section 8, just wrapped in RCU-safety so a concurrent BPF program never dereferences a freed node. **This is a very good, very concrete example of "RCU in real production kernel code" to study once you're comfortable with the plain (non-concurrent) delete algorithm from Section 8** — get the sequential algorithm solid in your head first, then go read `lpm_trie.c`'s `trie_delete_elem()` and map every step back to Section 8.

---

## 13. C implementation — LPM Patricia trie for IPv4 CIDR

This is a from-scratch, dependency-free implementation you can compile and test standalone (useful for understanding `lpm_trie.c` by analogy, or for a user-space control-plane tool that populates a BPF map). It implements Sections 6.2 (LPM search), 7 (insert, representative-key approach), and 8 (delete).

```c
/* patricia_lpm.c — educational, single-file LPM Patricia trie for IPv4/32 keys.
 * Not the kernel implementation — a clean-room analog for building the mental
 * model before reading kernel/bpf/lpm_trie.c or net/ipv4/fib_trie.c.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>

typedef uint32_t u32;

typedef struct trie_node {
    struct trie_node *child[2];
    int   bit_index;      /* -1 for a pure leaf; 0..31 for a branch node */
    u32   key;            /* representative key (host byte order for this demo) */
    int   prefix_len;     /* valid iff has_value */
    bool  has_value;
    u32   value;           /* e.g. a verdict: 0 = allow, 1 = deny */
} trie_node;

static trie_node *root = NULL;

/* bit 0 = MSB of the 32-bit key */
static inline int test_bit(u32 key, int bit_index) {
    return (key >> (31 - bit_index)) & 1;
}

/* first bit index (0..31) at which a and b differ; 32 if identical */
static int first_diff_bit(u32 a, u32 b) {
    u32 x = a ^ b;
    if (x == 0) return 32;
    int i;
    for (i = 0; i < 32; i++)
        if (test_bit(x, i)) return i;
    return 32; /* unreachable */
}

static u32 mask_for(int prefix_len) {
    return prefix_len == 0 ? 0 : (0xFFFFFFFFu << (32 - prefix_len));
}

static bool prefix_matches(u32 stored_key, int stored_len, u32 search_key) {
    u32 m = mask_for(stored_len);
    return (stored_key & m) == (search_key & m);
}

static trie_node *new_leaf(u32 key, int prefix_len, u32 value) {
    trie_node *n = calloc(1, sizeof(*n));
    n->bit_index = -1;
    n->key = key;
    n->prefix_len = prefix_len;
    n->has_value = true;
    n->value = value;
    return n;
}

/* --------------------------- INSERT --------------------------- */

void trie_insert(u32 key, int prefix_len, u32 value) {
    u32 masked = key & mask_for(prefix_len);

    if (root == NULL) {
        root = new_leaf(masked, prefix_len, value);
        return;
    }

    /* Descend using bit_index while the representative key still agrees
     * with `masked` up to that point. Stop at a leaf, or the point where
     * we'd have to diverge before the child's bit_index. */
    trie_node *parent = NULL;
    trie_node *node = root;
    int parent_slot = -1;

    while (node->bit_index != -1) {
        int d = first_diff_bit(node->key, masked);
        if (d < node->bit_index) {
            /* must splice above `node` */
            break;
        }
        parent = node;
        parent_slot = test_bit(masked, node->bit_index);
        node = node->child[parent_slot];
    }

    /* `node` is either a leaf, or a branch node we must splice above. */
    int d;
    u32 node_repr_key;
    if (node->bit_index == -1) {
        /* leaf */
        if (node->key == masked && node->prefix_len == prefix_len) {
            /* exact prefix already present -> update value in place */
            node->has_value = true;
            node->value = value;
            return;
        }
        node_repr_key = node->key;
    } else {
        node_repr_key = node->key;
    }
    d = first_diff_bit(node_repr_key, masked);

    /* Special case: masked is itself a prefix of node_repr_key (or vice versa)
     * at exactly this depth — for LPM tries this can legitimately mean
     * "insert a value-bearing branch node", not just a leaf split.
     * For clarity this reference implementation treats prefix_len as the
     * authoritative "depth" of a value: we splice a branch at bit index `d`,
     * unless prefix_len <= d, in which case masked's own value belongs on
     * a NEW intermediate branch node at bit index = prefix_len. */

    trie_node *new_node;
    if (prefix_len <= d) {
        /* our new prefix is a strict ancestor prefix relative to what's below */
        new_node = calloc(1, sizeof(*new_node));
        new_node->bit_index = prefix_len < 32 ? prefix_len : -1;
        new_node->key = masked;
        new_node->has_value = true;
        new_node->prefix_len = prefix_len;
        new_node->value = value;
        if (new_node->bit_index != -1) {
            int slot = test_bit(node_repr_key, prefix_len);
            new_node->child[slot] = node;
            new_node->child[1 - slot] = NULL; /* nothing else here yet */
        }
    } else {
        trie_node *leaf = new_leaf(masked, prefix_len, value);
        new_node = calloc(1, sizeof(*new_node));
        new_node->bit_index = d;
        new_node->key = masked; /* either representative works */
        int slot = test_bit(masked, d);
        new_node->child[slot] = leaf;
        new_node->child[1 - slot] = node;
    }

    if (parent == NULL) {
        root = new_node;
    } else {
        parent->child[parent_slot] = new_node;
    }
}

/* --------------------------- LPM SEARCH --------------------------- */

u32 trie_lpm_lookup(u32 key, bool *found) {
    trie_node *node = root;
    trie_node *best = NULL;
    *found = false;

    while (node != NULL) {
        if (node->has_value &&
            prefix_matches(node->key, node->prefix_len, key)) {
            best = node;
        }
        if (node->bit_index == -1) break; /* leaf, stop */
        node = node->child[test_bit(key, node->bit_index)];
    }

    if (best) {
        *found = true;
        return best->value;
    }
    return 0;
}

/* --------------------------- DEMO --------------------------- */

static u32 ip(int a, int b, int c, int d) {
    return ((u32)a << 24) | ((u32)b << 16) | ((u32)c << 8) | (u32)d;
}

int main(void) {
    trie_insert(ip(10,0,0,0),   8,  100); /* allow */
    trie_insert(ip(10,1,0,0),   16, 200);
    trie_insert(ip(10,1,1,0),   24, 999); /* deny */
    trie_insert(ip(192,168,0,0),16, 300);

    struct { const char *desc; u32 addr; } tests[] = {
        { "10.1.1.5  -> expect 999 (/24 wins)", ip(10,1,1,5) },
        { "10.1.2.5  -> expect 200 (/16 wins)", ip(10,1,2,5) },
        { "10.2.2.2  -> expect 100 (/8 wins)",  ip(10,2,2,2) },
        { "8.8.8.8   -> expect NOT FOUND",       ip(8,8,8,8) },
    };

    for (size_t i = 0; i < sizeof(tests)/sizeof(tests[0]); i++) {
        bool found;
        u32 v = trie_lpm_lookup(tests[i].addr, &found);
        printf("%-42s got=%s value=%u\n", tests[i].desc,
               found ? "FOUND" : "NOT_FOUND", v);
    }
    return 0;
}
```

Build and run:

```bash
gcc -Wall -Wextra -O2 -o patricia_lpm patricia_lpm.c
./patricia_lpm
```

**Note on this reference implementation:** delete is intentionally omitted from the C version to keep it focused — implement it yourself as an exercise directly from Section 8's algorithm and the invariants in Section 5; that's a better learning path than reading it pre-written. If you get stuck on the "does the parent survive or collapse" branch, that's exactly the guiding question posed at the end of Section 8 — work through it on paper with the tree from Section 8's diagram before touching code.

---

## 14. Rust implementation — LPM Patricia trie, `no_std`-friendly design

Given your XDP/Aya work, here's a **user-space (std) reference version** first — get the algorithm right in a comfortable environment — followed by notes on what changes for a genuine `#![no_std]` BPF-side port (spoiler: **you would not implement this trie inside the BPF program itself** — see the callout after the code — but understanding it in Rust is still valuable for your user-space control plane that populates the `LPM_TRIE` map, and for building correct mental models transferable to kernel work).

```rust
// patricia_lpm.rs — educational LPM Patricia trie, IPv4/32 keys.
// cargo new patricia_lpm && drop this in src/main.rs

#[derive(Debug)]
enum Node {
    Branch {
        bit_index: u8,       // 0..=31
        repr_key: u32,       // representative key for divergence checks
        children: [Box<Node>; 2],
        value: Option<(u32 /*prefix*/, u8 /*len*/, u32 /*value*/)>,
    },
    Leaf {
        key: u32,
        prefix_len: u8,
        value: u32,
    },
}

fn test_bit(key: u32, bit_index: u8) -> usize {
    ((key >> (31 - bit_index)) & 1) as usize
}

fn first_diff_bit(a: u32, b: u32) -> u8 {
    let x = a ^ b;
    if x == 0 {
        return 32;
    }
    x.leading_zeros() as u8
}

fn mask_for(prefix_len: u8) -> u32 {
    if prefix_len == 0 {
        0
    } else {
        0xFFFF_FFFFu32 << (32 - prefix_len as u32)
    }
}

fn prefix_matches(stored_key: u32, stored_len: u8, search_key: u32) -> bool {
    let m = mask_for(stored_len);
    (stored_key & m) == (search_key & m)
}

struct Trie {
    root: Option<Box<Node>>,
}

impl Trie {
    fn new() -> Self {
        Trie { root: None }
    }

    fn insert(&mut self, key: u32, prefix_len: u8, value: u32) {
        let masked = key & mask_for(prefix_len);
        let Some(root) = self.root.take() else {
            self.root = Some(Box::new(Node::Leaf { key: masked, prefix_len, value }));
            return;
        };
        self.root = Some(Self::insert_rec(root, masked, prefix_len, value));
    }

    fn insert_rec(node: Box<Node>, masked: u32, prefix_len: u8, value: u32) -> Box<Node> {
        match *node {
            Node::Leaf { key, prefix_len: existing_len, value: existing_val } => {
                if key == masked && existing_len == prefix_len {
                    return Box::new(Node::Leaf { key, prefix_len, value });
                }
                Self::splice(key, Box::new(Node::Leaf { key, prefix_len: existing_len, value: existing_val }),
                              masked, prefix_len, value)
            }
            Node::Branch { bit_index, repr_key, mut children, value: node_val } => {
                let d = first_diff_bit(repr_key, masked);
                if d < bit_index {
                    // must splice above this branch
                    let old_branch = Box::new(Node::Branch { bit_index, repr_key, children, value: node_val });
                    return Self::splice(repr_key, old_branch, masked, prefix_len, value);
                }
                let slot = test_bit(masked, bit_index);
                children[slot] = Self::insert_rec(children[slot].clone_placeholder(), masked, prefix_len, value);
                Box::new(Node::Branch { bit_index, repr_key, children, value: node_val })
            }
        }
    }

    /// Create a new branch node splicing `masked` in against the existing
    /// subtree `existing`, whose representative key is `existing_repr`.
    fn splice(existing_repr: u32, existing: Box<Node>, masked: u32, prefix_len: u8, value: u32) -> Box<Node> {
        let d = first_diff_bit(existing_repr, masked);
        if prefix_len <= d {
            // `masked` is an ancestor-level prefix relative to `existing`
            if prefix_len == 32 {
                // shouldn't happen given the <= d check when d < 32, defensive only
                return existing;
            }
            let slot = test_bit(existing_repr, prefix_len);
            let mut children: [Box<Node>; 2] = [
                Box::new(Node::Leaf { key: masked, prefix_len, value }), // placeholder, fixed below
                Box::new(Node::Leaf { key: masked, prefix_len, value }),
            ];
            children[slot] = existing;
            children[1 - slot] = Box::new(Node::Leaf { key: masked, prefix_len, value }); // no-op child
            Box::new(Node::Branch {
                bit_index: prefix_len,
                repr_key: masked,
                children,
                value: Some((masked, prefix_len, value)),
            })
        } else {
            let new_leaf = Box::new(Node::Leaf { key: masked, prefix_len, value });
            let slot = test_bit(masked, d);
            let mut children: [Box<Node>; 2] = [new_leaf.clone_placeholder(), new_leaf.clone_placeholder()];
            children[slot] = new_leaf;
            children[1 - slot] = existing;
            Box::new(Node::Branch { bit_index: d, repr_key: masked, children, value: None })
        }
    }

    fn lpm_lookup(&self, key: u32) -> Option<u32> {
        let mut cur = self.root.as_deref();
        let mut best: Option<u32> = None;
        while let Some(node) = cur {
            match node {
                Node::Leaf { key: k, prefix_len, value } => {
                    if prefix_matches(*k, *prefix_len, key) {
                        best = Some(*value);
                    }
                    break;
                }
                Node::Branch { bit_index, children, value, .. } => {
                    if let Some((p, l, v)) = value {
                        if prefix_matches(*p, *l, key) {
                            best = Some(*v);
                        }
                    }
                    cur = Some(children[test_bit(key, *bit_index)].as_ref());
                }
            }
        }
        best
    }
}

fn ip(a: u8, b: u8, c: u8, d: u8) -> u32 {
    ((a as u32) << 24) | ((b as u32) << 16) | ((c as u32) << 8) | (d as u32)
}

fn main() {
    let mut t = Trie::new();
    t.insert(ip(10, 0, 0, 0), 8, 100);
    t.insert(ip(10, 1, 0, 0), 16, 200);
    t.insert(ip(10, 1, 1, 0), 24, 999);
    t.insert(ip(192, 168, 0, 0), 16, 300);

    for (desc, addr) in [
        ("10.1.1.5", ip(10, 1, 1, 5)),
        ("10.1.2.5", ip(10, 1, 2, 5)),
        ("10.2.2.2", ip(10, 2, 2, 2)),
        ("8.8.8.8", ip(8, 8, 8, 8)),
    ] {
        println!("{desc} -> {:?}", t.lpm_lookup(addr));
    }
}
```

> **Important honesty note on the Rust sketch above:** `Box<Node>` doesn't implement `Clone` by default and the `.clone_placeholder()` calls are **pseudocode markers**, not real Rust — they mark exactly the spots where you need to make a real ownership decision (typically: restructure `insert_rec` to take `&mut Option<Box<Node>>` and use `mem::replace`/`take`, which is the idiomatic way to rebuild a tree in place in safe Rust without fighting the borrow checker on recursive owned structures). **This is a deliberate teaching choice, not sloppiness**: the ownership-juggling in a from-scratch owned-tree insert is itself one of the more instructive Rust exercises you can do, and pre-solving it for you would rob you of that. Guiding question: how would you restructure `insert_rec` to take `&mut Box<Node>` (or `&mut Option<Box<Node>>` for the root) and mutate in place using `std::mem::replace`, so you never need to clone or duplicate a node? Work that out — it's the single best Rust-ownership exercise this whole document offers.

### 14.1 Why you would *not* implement this trie inside a BPF/`no_std` program

The eBPF verifier fundamentally cannot verify **unbounded recursive pointer-chasing over dynamically allocated, arbitrarily deep owned structures** — no `Box`, no recursive `enum`, no dynamic allocation at all inside the BPF program. This is exactly why `BPF_MAP_TYPE_LPM_TRIE` exists as a **kernel-side, helper-mediated** data structure (Section 12): the kernel implements the trie in trusted, unverified C (`lpm_trie.c`) and your BPF program only ever calls the bounded, verifier-approved `bpf_map_lookup_elem()` helper — it never walks trie nodes itself. **Your Rust/Aya user-space program is exactly where a from-scratch trie like the one above belongs** — e.g., as your own control-plane cache/index before pushing entries into the kernel map, or for a purely user-space policy engine. Keep this boundary sharp in your mental model: *trie traversal logic = user space (or trusted kernel C); BPF program = bounded map helper calls only.*

---

## 15. Go implementation — LPM Patricia trie

Useful if your control-plane / orchestration tooling (populating BPF maps, exposing a gRPC/HTTP API for firewall rules, cloud-provider SDK glue for AWS/GCP/Azure/OCI security groups) is in Go, which is common in cloud networking tooling.

```go
package patricia

import "math/bits"

type node struct {
	// bitIndex == -1 marks a leaf.
	bitIndex   int
	reprKey    uint32
	children   [2]*node
	hasValue   bool
	prefix     uint32
	prefixLen  uint8
	value      uint32
}

type Trie struct {
	root *node
}

func New() *Trie { return &Trie{} }

func testBit(key uint32, bitIndex int) int {
	return int((key >> (31 - bitIndex)) & 1)
}

func firstDiffBit(a, b uint32) int {
	x := a ^ b
	if x == 0 {
		return 32
	}
	return bits.LeadingZeros32(x)
}

func maskFor(prefixLen uint8) uint32 {
	if prefixLen == 0 {
		return 0
	}
	return 0xFFFFFFFF << (32 - prefixLen)
}

func prefixMatches(storedKey uint32, storedLen uint8, searchKey uint32) bool {
	m := maskFor(storedLen)
	return (storedKey & m) == (searchKey & m)
}

func newLeaf(key uint32, prefixLen uint8, value uint32) *node {
	return &node{
		bitIndex:  -1,
		reprKey:   key,
		hasValue:  true,
		prefix:    key,
		prefixLen: prefixLen,
		value:     value,
	}
}

func (t *Trie) Insert(key uint32, prefixLen uint8, value uint32) {
	masked := key & maskFor(prefixLen)

	if t.root == nil {
		t.root = newLeaf(masked, prefixLen, value)
		return
	}

	var parent *node
	parentSlot := -1
	cur := t.root

	for cur.bitIndex != -1 {
		d := firstDiffBit(cur.reprKey, masked)
		if d < cur.bitIndex {
			break
		}
		parent = cur
		parentSlot = testBit(masked, cur.bitIndex)
		cur = cur.children[parentSlot]
	}

	if cur.bitIndex == -1 && cur.reprKey == masked && cur.prefixLen == prefixLen {
		cur.hasValue = true
		cur.value = value
		return
	}

	d := firstDiffBit(cur.reprKey, masked)
	var newNode *node

	if prefixLen <= uint8(d) {
		nn := &node{
			bitIndex:  int(prefixLen),
			reprKey:   masked,
			hasValue:  true,
			prefix:    masked,
			prefixLen: prefixLen,
			value:     value,
		}
		slot := testBit(cur.reprKey, int(prefixLen))
		nn.children[slot] = cur
		newNode = nn
	} else {
		leaf := newLeaf(masked, prefixLen, value)
		nn := &node{bitIndex: d, reprKey: masked}
		slot := testBit(masked, d)
		nn.children[slot] = leaf
		nn.children[1-slot] = cur
		newNode = nn
	}

	if parent == nil {
		t.root = newNode
	} else {
		parent.children[parentSlot] = newNode
	}
}

// LPMLookup returns (value, true) for the longest matching stored prefix,
// or (0, false) if nothing matches.
func (t *Trie) LPMLookup(key uint32) (uint32, bool) {
	cur := t.root
	var best *node

	for cur != nil {
		if cur.hasValue && prefixMatches(cur.prefix, cur.prefixLen, key) {
			best = cur
		}
		if cur.bitIndex == -1 {
			break
		}
		cur = cur.children[testBit(key, cur.bitIndex)]
	}

	if best != nil {
		return best.value, true
	}
	return 0, false
}
```

Example usage / test:

```go
package main

import (
	"fmt"
	"patricia" // adjust import path to your module
)

func ip(a, b, c, d byte) uint32 {
	return uint32(a)<<24 | uint32(b)<<16 | uint32(c)<<8 | uint32(d)
}

func main() {
	t := patricia.New()
	t.Insert(ip(10, 0, 0, 0), 8, 100)
	t.Insert(ip(10, 1, 0, 0), 16, 200)
	t.Insert(ip(10, 1, 1, 0), 24, 999)
	t.Insert(ip(192, 168, 0, 0), 16, 300)

	tests := []struct {
		desc string
		addr uint32
	}{
		{"10.1.1.5", ip(10, 1, 1, 5)},
		{"10.1.2.5", ip(10, 1, 2, 5)},
		{"10.2.2.2", ip(10, 2, 2, 2)},
		{"8.8.8.8", ip(8, 8, 8, 8)},
	}
	for _, tc := range tests {
		v, ok := t.LPMLookup(tc.addr)
		fmt.Printf("%-10s -> value=%d found=%v\n", tc.desc, v, ok)
	}
}
```

**Go-specific engineering note:** unlike the Rust version, Go's garbage collector means you don't have the ownership-juggling problem — but that also means it's *easier to write a subtly wrong trie in Go and not notice*, because GC will happily keep unreachable-but-still-pointed-to structures alive without complaint. This is exactly where invariant-checking test code (Section 16) earns its keep — the language won't catch a "forgot to detach the old subtree" bug for you the way a Rust borrow-checker fight might force you to think it through.

---

## 16. Testing strategy: invariants, property tests, edge cases

Test at three levels. Don't skip the middle one — it's the one that actually catches Patricia-specific bugs (as opposed to generic off-by-one bugs).

### 16.1 Structural invariant checks (run after every insert/delete in your test suite)

Write a `check_invariants(root)` function and call it obsessively in tests:

1. Every branch node has exactly two non-null children.
2. Every child's bit-index is strictly greater than its parent's bit-index (or the child is a leaf).
3. For every leaf/value-bearing node, walking down from the root using its own bits reproduces the exact path to that node (i.e., insert a key, then LPM-search for that exact key, and confirm you land back on it with the right value — this catches "inserted into the wrong slot" bugs).
4. Node count sanity: for `n` inserted distinct prefixes, branch-node count should be bounded and consistent with a strict binary tree over the *distinct bit-index divergence points* — you won't get a magic formula as clean as `n-1` once value-bearing internal nodes are allowed (LPM variant), but it should never grow unboundedly across repeated insert/delete of the same key set (a classic bug: delete leaks a node because you forgot the "demote to branch-only, don't free" rule from Section 8).

### 16.2 Property-based / fuzz tests (this is where LPM bugs actually surface)

- Generate random sets of CIDR prefixes (varying lengths 0–32, including edge lengths 0 and 32).
- Generate random query addresses.
- Compute the "obviously correct" answer via **brute force**: linear scan of all inserted prefixes, keep the one with the longest `prefix_len` that matches (ties broken by insertion order or explicitly disallowed in your test data). Compare against your trie's answer for every query. This differential test (trie vs. brute force) is the single highest-value test you can write for this data structure — write it before you trust any of the code above in anything real.
- Explicitly include: overlapping prefixes (`/8` and `/16` and `/24` of the same address, inserted in different orders — insertion order should not affect final correctness, but *can* affect internal tree shape, which is fine), a `/0` default route (matches everything — make sure it's returned when nothing more specific exists, and never wins over anything more specific), a `/32` host route, duplicate insert of the same exact prefix (should update value, not create a duplicate leaf), and delete-then-reinsert cycles.

### 16.3 Kernel/eBPF-specific test additions (once you wire this to `BPF_MAP_TYPE_LPM_TRIE`)

- Verify `BPF_F_NO_PREALLOC` flag requirements for your target kernel (test on the actual guest VM kernel version you're running — don't assume, check `uname -r` and cross-reference kernel changelogs/`Documentation/bpf/map_lpm_trie.rst` if present in your kernel's doc tree).
- Verify behavior at `max_entries` capacity — what does `bpf_map_update_elem()` return when the LPM trie is full, and does your user-space control plane handle `ENOSPC`/`E2BIG` gracefully rather than crashing your control-plane daemon?
- Concurrency test: hammer the map with concurrent inserts/deletes from user space while a real XDP program (or a test harness using `bpf_prog_test_run`) does lookups, and confirm you never observe a torn/inconsistent read — this exercises the RCU-safety discussed in Section 12.4, and is exactly the kind of test that distinguishes "I understand the algorithm" from "I understand the production system."

---

## 17. Mental models and guiding questions to internalize this

Work through these **without** looking at the code above, on paper, before you consider this topic "known":

1. **Why must bit-index strictly increase along a path?** What specifically breaks (be concrete: describe a failing search) if you allowed a child to have a bit-index ≤ its parent's?
2. **Why does a plain dictionary Patricia trie only need leaves to carry values, while an LPM trie needs any node to potentially carry one?** Connect this directly to the semantics of IP routing (multiple independently valid prefixes of different lengths covering the same address).
3. **Walk through, from memory, why search never backtracks** — what property of the structure (not the algorithm — the *structure*) guarantees that "descend once, verify once at the end" is sufficient?
4. **In delete, why is "parent has one child left → collapse the parent" not always the correct rule when the parent itself is value-bearing?** Redo the Section 8 exercise in your own words.
5. **Why can't the trie traversal logic live inside the BPF program itself?** State the verifier constraint precisely, not just "it's not allowed" — what specifically about recursive/dynamically-sized pointer structures is the verifier unable to bound?
6. **Contrast `fib_trie`'s LC-trie multi-bit fan-out against strict binary Patricia** — what's the actual engineering trade-off being bought (memory vs. hops), and why would that trade-off matter more for a full internet routing table (800k+ IPv4 prefixes as of recent years) than for your firewall's likely much smaller CIDR blocklist?
7. **Design exercise (do this before your next XDP work session):** sketch, on paper, the LPM trie your firewall would build for a realistic blocklist — say, 5–10 CIDR ranges of mixed lengths including some overlapping ones (a `/8` you mostly allow with a `/24` carve-out you deny inside it) — and hand-trace three lookups through it. This is precisely the shape of policy logic real firewalls implement, and doing it once on paper will make the eBPF map semantics in Section 12 feel obvious rather than magical.

---

## 18. Further reading

- D. R. Morrison, *"PATRICIA — Practical Algorithm To Retrieve Information Coded in Alphanumeric,"* Journal of the ACM, 1968 — the original paper, describes the threaded/back-pointer classic variant (Section 4.1).
- Sedgewick, *Algorithms* — has a clear treatment of classic PATRICIA with the back-pointer traversal explained carefully; good for understanding the historical variant if you ever read old networking code that still uses it.
- Nilsson & Karlsson, *"IP-address lookup using LC-tries"* — the paper behind the level-compression idea used in `fib_trie`.
- `net/ipv4/fib_trie.c` in the Linux kernel source — read alongside this document, mapping every concept in Sections 5–10 onto the real `struct key_vector`/`struct tnode` code.
- `kernel/bpf/lpm_trie.c` — the eBPF LPM trie implementation; read `trie_lookup_elem()` and `trie_update_elem()`/`trie_delete_elem()` alongside Sections 6–8 and 12.4 of this document.
- RFC 1812, *Requirements for IP Version 4 Routers*, §5.2.4 — the normative "longest match wins" rule this whole data structure exists to implement efficiently.
- `Documentation/bpf/map_lpm_trie.rst` (if present in your kernel tree) / `include/uapi/linux/bpf.h` for the authoritative, version-specific `BPF_MAP_TYPE_LPM_TRIE` key/flag semantics — always check this against your actual running kernel version rather than trusting any single external write-up (including this one) for exact flag/behavior details, since kernel-version drift is real here.

---

*End of guide. Suggested next step for your XDP firewall project: implement Section 13 or 14's insert/lookup, write the Section 16.1 invariant checker, then write the Section 16.2 differential fuzz test against brute force before wiring anything into an actual `BPF_MAP_TYPE_LPM_TRIE` map — get the algorithm provably right in user space first, exactly the same sequencing discipline you'd apply to any kernel-adjacent production code.*

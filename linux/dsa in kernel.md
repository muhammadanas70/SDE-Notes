The Linux kernel provides a rich set of optimized, generic data structures for use in kernel code. Kernel developers are strongly encouraged to use these standard implementations instead of creating their own. The primary ones include:

### 📜 Linked Lists
The kernel primarily uses a **circular, doubly-linked list** implemented via the `struct list_head`. This is an *intrusive* list where the `list_head` structure is embedded within the data structure itself. Key characteristics include:
*   **Header**: `<linux/list.h>`.
*   **Implementation**: It's a circular list, meaning the last node points back to the first, and the first node's `prev` points to the last.
*   **Variants**: The kernel also supports **singly-linked** lists (`hlist`) and **hash lists** (`hash.h`, `hashtable.h`) which are optimized for use in hash tables.

### 🗂️ Hash Tables
Hash tables are used for fast key-based lookups. They are often implemented as an array of **buckets**, with each bucket containing a linked list (or hash list) of entries that hash to the same index.

### 🌳 Binary Trees
The kernel employs several types of tree structures:
*   **Red-Black Trees (rbtree)**: A self-balancing binary search tree that provides O(log n) search, insert, and delete operations. They are widely used for in-memory caches, such as the Virtual Memory Area (VMA) tree.
*   **Radix Trees**: A space-efficient tree that maps integer keys to pointers. It's designed for fast lookups and is commonly used for storing page cache information.
*   **Binary Trees**: The kernel also includes a generic implementation of binary trees.

### 🏗️ Other Important Structures
*   **Queues (kfifo)**: A lockless, generic FIFO implementation, useful for producer/consumer scenarios.
*   **Maps (IDR/IDA)**: Used for managing mappings of integer IDs to pointers (IDR) or for allocating unique integer IDs (IDA).
*   **XArray**: A modern, efficient API that is replacing the older radix tree for many use cases. It is a sparse array that can hold arbitrary pointers and provides a simpler and safer interface.
*   **Min-Heaps**: A priority queue implementation where the smallest element is always at the root.
*   **Bit Arrays**: The kernel provides efficient operations for managing and manipulating arrays of bits.

Each of these structures is designed with concurrency and performance in mind, often integrating with kernel synchronization primitives like RCU (Read-Copy-Update) for lockless access.
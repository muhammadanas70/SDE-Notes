# Operator Precedence and Associativity: Complete Guide for C, Rust, and Go

## Table of Contents

1. [Fundamentals](#fundamentals)
2. [How Operator Precedence Works](#how-operator-precedence-works)
3. [Associativity Rules](#associativity-rules)
4. [Memory and Evaluation Architecture](#memory-and-evaluation-architecture)
5. [Complete Precedence Tables](#complete-precedence-tables)
6. [C Language: Detailed Analysis](#c-language-detailed-analysis)
7. [Rust Language: Detailed Analysis](#rust-language-detailed-analysis)
8. [Go Language: Detailed Analysis](#go-language-detailed-analysis)
9. [Practical Implications](#practical-implications)
10. [Common Pitfalls](#common-pitfalls)
11. [Performance Considerations](#performance-considerations)

---

## Fundamentals

### What is Operator Precedence?

Operator precedence determines the order in which operators are evaluated in an expression that contains multiple operators. It defines which operators have higher priority and will be evaluated first.

**Simple Example:**
```
Expression: 2 + 3 * 4
Without precedence: (2 + 3) * 4 = 5 * 4 = 20  ❌ Wrong
With precedence:    2 + (3 * 4) = 2 + 12 = 14 ✓ Correct
```

Multiplication has higher precedence than addition, so it's evaluated first.

### What is Associativity?

Associativity determines the direction in which operators of the same precedence are evaluated. There are two types:

1. **Left-to-Right (Left Associativity)**: Operators are evaluated from left to right
2. **Right-to-Left (Right Associativity)**: Operators are evaluated from right to left

**Example - Left Associativity:**
```
Expression: 10 - 5 - 2
Left-to-right (correct): (10 - 5) - 2 = 5 - 2 = 3
Right-to-left (wrong):   10 - (5 - 2) = 10 - 3 = 7
```

**Example - Right Associativity:**
```
Expression: 2 ^ 3 ^ 2  (power/exponentiation operator)
Left-to-right (wrong):  (2 ^ 3) ^ 2 = 8 ^ 2 = 64
Right-to-left (correct): 2 ^ (3 ^ 2) = 2 ^ 9 = 512
```

### Why Does This Matter?

Understanding operator precedence and associativity is crucial for:
1. **Correctness**: Writing code that does what you intend
2. **Readability**: Making code self-documenting
3. **Debugging**: Understanding why expressions evaluate unexpectedly
4. **Performance**: Being aware of evaluation order affecting caching and side effects
5. **Cross-language Development**: Knowing differences between languages prevents bugs

---

## How Operator Precedence Works

### The Parsing Process

When a compiler/interpreter encounters an expression, it builds an Abstract Syntax Tree (AST) based on precedence rules.

```
Expression: a + b * c - d

Step 1: Identify all operators and their precedence
        + (precedence 2)
        * (precedence 3)  ← Higher precedence
        - (precedence 2)

Step 2: Build AST from highest to lowest precedence
        
                    -
                   / \
                  +   d
                 / \
                a   *
                   / \
                  b   c

Step 3: Evaluate from leaf nodes upward
        b * c → result1
        a + result1 → result2
        result2 - d → final result
```

### The AST (Abstract Syntax Tree) Mechanism

The Abstract Syntax Tree is the internal representation the compiler uses:

```
Visual representation of: 2 + 3 * 4

        [+]
       /   \
      2     [*]
           /   \
          3     4

Evaluation order:
1. Leaf nodes first: 3, 4
2. Internal nodes: 3 * 4 = 12
3. Root: 2 + 12 = 14
```

---

## Associativity Rules

### Left Associativity (Most Common)

Most operators are left-associative. This means operators of the same precedence are grouped from left to right.

```
Expression: a - b - c
Grouped as: (a - b) - c

Evaluation steps:
1. a - b → temp1
2. temp1 - c → result
```

### Right Associativity (Less Common)

Some operators are right-associative, grouping from right to left.

```
Expression: a = b = c = 5
Grouped as: a = (b = (c = 5))

Evaluation steps:
1. c = 5 → returns 5, assigns to c
2. b = 5 → returns 5, assigns to b
3. a = 5 → returns 5, assigns to a
```

### Mixed Precedence and Associativity

Complex expressions use both rules together:

```
Expression: 2 + 3 * 4 - 5 / 2

Step 1: Identify operators with their precedence and associativity
        + (precedence 2, left-to-right)
        * (precedence 3, left-to-right)
        - (precedence 2, left-to-right)
        / (precedence 3, left-to-right)

Step 2: Group by precedence (highest first)
        3 * 4 and 5 / 2 first

Step 3: Group remaining operators of same precedence left-to-right
        (2 + 12) - 2.5

Step 4: Evaluate
        14 - 2.5 = 11.5
```

---

## Memory and Evaluation Architecture

### Runtime Stack During Expression Evaluation

Understanding how expressions are evaluated in memory helps grasp precedence:

```
Evaluating: int result = 2 + 3 * 4;

┌─────────────────────────────────────────────────────────────────┐
│                         EVALUATION STACK                         │
├─────────────────────────────────────────────────────────────────┤

Phase 1: Parse and Build AST
┌──────────┐
│   AST    │  
│    +     │
│   / \    │
│  2   *   │
│     / \  │
│    3   4 │
└──────────┘

Phase 2: Evaluate bottom-up
┌──────────────────────────────────┐
│ Stack Frame 1: Push operands 3,4 │
│ [3] [4]                          │
│ Operator: *                      │
│ Pop 4, Pop 3, Calculate 3*4=12   │
│ Push 12                          │
└──────────────────────────────────┘

┌──────────────────────────────────┐
│ Stack Frame 2: Push operands 2,12│
│ [2] [12]                         │
│ Operator: +                      │
│ Pop 12, Pop 2, Calculate 2+12=14 │
│ Push 14                          │
└──────────────────────────────────┘

Final Result: 14
```

### Memory Layout During Evaluation

```
Program Memory During: a = 5; b = 3; int c = a + b * 2;

┌─────────────────────────────────────────────────────────────┐
│                       MEMORY LAYOUT                          │
├─────────────────────────────────────────────────────────────┤
│  Heap                                                        │
│  ┌──────────────────────────────────────────────────────┐   │
│  │                    (Dynamic Memory)                   │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Stack                                                       │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  [Return Addr ]        ← Function return address      │   │
│  │  [Frame Ptr   ]        ← Previous frame pointer       │   │
│  │  [Local: a=5  ]        ← Variable 'a'                 │   │
│  │  [Local: b=3  ]        ← Variable 'b'                 │   │
│  │  [Local: c    ]        ← Variable 'c' (uninitialized) │   │
│  │                                                       │   │
│  │  During evaluation of (b * 2):                        │   │
│  │  ┌──────────────────────────────────────────────┐    │   │
│  │  │ Temp1 = b       (read from memory)          │    │   │
│  │  │ Temp2 = 2       (constant)                  │    │   │
│  │  │ Temp3 = Temp1 * Temp2 = 3 * 2 = 6         │    │   │
│  │  └──────────────────────────────────────────────┘    │   │
│  │                                                       │   │
│  │  During evaluation of (a + result):                   │   │
│  │  ┌──────────────────────────────────────────────┐    │   │
│  │  │ Temp4 = a       (read from memory)          │    │   │
│  │  │ Temp5 = Temp3   (from previous calc)        │    │   │
│  │  │ Temp6 = Temp4 + Temp5 = 5 + 6 = 11        │    │   │
│  │  │ c = Temp6       (assign to memory)          │    │   │
│  │  └──────────────────────────────────────────────┘    │   │
│  │                                                       │   │
│  │  [c = 11]       ← Final value written                │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  Code Segment (Read-only)                                   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Instructions: load, mul, add, store                  │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Register Allocation During Evaluation

Most modern CPUs evaluate expressions in registers:

```
Evaluating: int result = (a + b) * (c - d);

CPU Registers during execution:

RAX  │  5  │  Holds value of 'a'
RBX  │  3  │  Holds value of 'b'
RCX  │  8  │  Temporary: a + b = 8
RDX  │ 10  │  Holds value of 'c'
RSI  │  2  │  Holds value of 'd'
RDI  │  8  │  Temporary: c - d = 8
R8   │ 64  │  Final: 8 * 8 = 64

Execution timeline:
┌─────────────────────────────────────────────────────┐
│ T0:  Load a → RAX  (RAX = 5)                        │
│ T1:  Load b → RBX  (RBX = 3)                        │
│ T2:  ADD RAX, RBX  (RCX = 8) // a + b             │
│ T3:  Load c → RDX  (RDX = 8)                        │
│ T4:  Load d → RSI  (RSI = 2)                        │
│ T5:  SUB RDX, RSI  (RDI = 8) // c - d             │
│ T6:  MUL RCX, RDI  (R8 = 64) // (a+b) * (c-d)    │
│ T7:  Store R8 → result                             │
└─────────────────────────────────────────────────────┘
```

---

## Complete Precedence Tables

### Unified Comparison Table

```
┌────────────┬──────────────────────┬─────────────┬──────────────┐
│ Precedence │      Operator        │   Language  │ Associativity│
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    1 (H)   │ () [] . ->           │ C/Rust/Go   │ Left→Right   │
│            │ Function call, Index │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    2       │ ! ~ ++ -- + - * &    │ C/Rust/Go   │ Right→Left   │
│            │ Unary operators      │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    3       │ * / % << >>          │ C/Rust/Go   │ Left→Right   │
│            │ Multiplicative       │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    4       │ + -                  │ C/Rust/Go   │ Left→Right   │
│            │ Additive             │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    5       │ << >> >>> (Rust)     │ Rust/Go     │ Left→Right   │
│            │ Bit shift            │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    6       │ & (bitwise AND)      │ C/Rust/Go   │ Left→Right   │
│            │                      │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    7       │ ^                    │ C/Rust/Go   │ Left→Right   │
│            │ Bitwise XOR          │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    8       │ | (bitwise OR)       │ C/Rust/Go   │ Left→Right   │
│            │                      │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│    9       │ < > <= >= == !=      │ C/Rust/Go   │ Left→Right   │
│            │ Relational/Equality  │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│   10       │ &&                   │ C/Rust/Go   │ Left→Right   │
│            │ Logical AND          │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│   11       │ ||                   │ C/Rust/Go   │ Left→Right   │
│            │ Logical OR           │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│   12       │ ?:                   │ C/Rust/Go   │ Right→Left   │
│            │ Ternary conditional  │             │              │
├────────────┼──────────────────────┼─────────────┼──────────────┤
│   13 (L)   │ = += -= *= /= %=     │ C/Rust/Go   │ Right→Left   │
│            │ Assignment           │             │              │
└────────────┴──────────────────────┴─────────────┴──────────────┘
```

---

## C Language: Detailed Analysis

### C Operator Precedence Table (Complete)

```
┌──────────┬────────────────────────────────┬──────────────┬─────────────────┐
│ Priority │         Operators              │Associativity │     Details     │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   1      │ () [] -> .                     │ L → R        │ Postfix ops     │
│          │ Postfix increment/decrement    │              │ Post ++ Post -- │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   2      │ ! ~ + - ++ --                 │ R → L        │ Unary prefix    │
│          │ * & (dereference/address)     │              │ Pre ++ Pre --   │
│          │ sizeof (type)                  │              │ (type) cast     │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   3      │ * / %                          │ L → R        │ Multiply, Div   │
│          │ (multiplicative)               │              │ Modulo          │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   4      │ + -                            │ L → R        │ Add, Subtract   │
│          │ (additive)                     │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   5      │ << >>                          │ L → R        │ Bitwise shifts  │
│          │ (shift operators)              │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   6      │ < > <= >=                      │ L → R        │ Relational ops  │
│          │ (relational)                   │              │ Less/Greater    │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   7      │ == !=                          │ L → R        │ Equality ops    │
│          │ (equality)                     │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   8      │ &                              │ L → R        │ Bitwise AND     │
│          │ (bitwise AND)                  │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   9      │ ^                              │ L → R        │ Bitwise XOR     │
│          │ (bitwise XOR)                  │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   10     │ |                              │ L → R        │ Bitwise OR      │
│          │ (bitwise OR)                   │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   11     │ &&                             │ L → R        │ Logical AND     │
│          │ (logical AND)                  │              │ Short-circuit   │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   12     │ ||                             │ L → R        │ Logical OR      │
│          │ (logical OR)                   │              │ Short-circuit   │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   13     │ ?:                             │ R → L        │ Ternary cond.   │
│          │ (ternary conditional)         │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   14     │ = += -= *= /= %=               │ R → L        │ Assignment ops  │
│          │ <<= >>= &= ^= |=               │              │                 │
├──────────┼────────────────────────────────┼──────────────┼─────────────────┤
│   15     │ ,                              │ L → R        │ Comma operator  │
│          │ (comma)                        │              │ Lowest priority │
└──────────┴────────────────────────────────┴──────────────┴─────────────────┘
```

### C Language Code Examples

#### Example 1: Basic Arithmetic Precedence

```c
#include <stdio.h>

int main() {
    int a = 2, b = 3, c = 4, d = 5;
    
    // Example 1: Multiplication before addition
    // Expression: a + b * c - d
    // Evaluated as: a + (b * c) - d = 2 + (3 * 4) - 5 = 2 + 12 - 5 = 9
    int result1 = a + b * c - d;
    printf("a + b * c - d = %d\n", result1);  // Output: 9
    
    // Example 2: Same operators, left-to-right
    // Expression: a - b - c
    // Evaluated as: (a - b) - c = (2 - 3) - 4 = -1 - 4 = -5
    int result2 = a - b - c;
    printf("a - b - c = %d\n", result2);      // Output: -5
    
    // Example 3: Mixed with parentheses
    // Expression: (a + b) * (c - d)
    // Parentheses override precedence
    // Evaluated as: (2 + 3) * (4 - 5) = 5 * (-1) = -5
    int result3 = (a + b) * (c - d);
    printf("(a + b) * (c - d) = %d\n", result3);  // Output: -5
    
    return 0;
}
```

#### Example 2: Bitwise Operations Precedence

```c
#include <stdio.h>

int main() {
    unsigned int a = 12;  // Binary: 1100
    unsigned int b = 10;  // Binary: 1010
    unsigned int c = 5;   // Binary: 0101
    
    // Example 1: Bitwise AND has higher precedence than bitwise OR
    // Expression: a | b & c
    // Evaluated as: a | (b & c)
    // b & c = 1010 & 0101 = 0000 = 0
    // a | 0 = 1100 | 0000 = 1100 = 12
    unsigned int result1 = a | b & c;
    printf("a | b & c = %u\n", result1);  // Output: 12
    
    // Compare with explicit parentheses
    // Expression: (a | b) & c
    // a | b = 1100 | 1010 = 1110 = 14
    // 14 & c = 1110 & 0101 = 0100 = 4
    unsigned int result2 = (a | b) & c;
    printf("(a | b) & c = %u\n", result2);  // Output: 4
    
    // Example 2: Shift operators with arithmetic
    // Expression: a + b << 2
    // Evaluated as: a + (b << 2)
    // b << 2 = 1010 << 2 = 101000 = 40
    // a + 40 = 12 + 40 = 52
    unsigned int result3 = a + b << 2;
    printf("a + b << 2 = %u\n", result3);  // Output: 52
    
    return 0;
}
```

#### Example 3: Unary Operator Precedence

```c
#include <stdio.h>

int main() {
    int a = 5;
    int b = 10;
    int c = 3;
    
    // Example 1: Unary negation vs binary subtraction
    // Expression: -a + b
    // Evaluated as: (-a) + b = -5 + 10 = 5
    int result1 = -a + b;
    printf("-a + b = %d\n", result1);  // Output: 5
    
    // Compare with: a - b
    // Evaluated as: a - b = 5 - 10 = -5
    int result2 = a - b;
    printf("a - b = %d\n", result2);   // Output: -5
    
    // Example 2: Increment/decrement precedence
    // Post-increment has higher precedence than multiplication
    // Expression: a++ * b
    // a++ evaluates to 5 (old value), then a becomes 6
    // Result: 5 * 10 = 50
    a = 5;  // Reset
    int result3 = a++ * b;
    printf("a++ * b = %d (a is now %d)\n", result3, a);  // Output: 50 (a is now 6)
    
    // Example 3: Type casting has higher precedence than multiplication
    // Expression: (int) 3.5 * c
    // Evaluated as: ((int) 3.5) * c = 3 * 3 = 9
    double d = 3.5;
    int result4 = (int) d * c;
    printf("(int) d * c = %d\n", result4);  // Output: 9
    
    return 0;
}
```

#### Example 4: Logical Operators and Short-Circuit Evaluation

```c
#include <stdio.h>

int side_effect_false() {
    printf("  [side_effect_false() called]\n");
    return 0;
}

int side_effect_true() {
    printf("  [side_effect_true() called]\n");
    return 1;
}

int main() {
    // Example 1: && and || with short-circuit evaluation
    // Expression: 0 && side_effect_true()
    // && has left-to-right associativity
    // Since 0 is false, side_effect_true() is NOT evaluated (short-circuit)
    printf("Expression: 0 && side_effect_true()\n");
    int result1 = 0 && side_effect_true();
    printf("Result: %d\n\n", result1);  // side_effect_true() NOT called
    
    // Example 2: 1 || side_effect_false()
    // || has left-to-right associativity
    // Since 1 is true, side_effect_false() is NOT evaluated (short-circuit)
    printf("Expression: 1 || side_effect_false()\n");
    int result2 = 1 || side_effect_false();
    printf("Result: %d\n\n", result2);  // side_effect_false() NOT called
    
    // Example 3: Multiple logical operators
    // Expression: 1 && 0 || 1
    // && has same precedence as ||, both left-to-right
    // Evaluated as: (1 && 0) || 1 = 0 || 1 = 1
    printf("Expression: 1 && 0 || 1\n");
    int result3 = 1 && 0 || 1;
    printf("Result: %d\n\n", result3);  // Output: 1
    
    // Example 4: Relational before logical
    // Expression: 5 < 10 && 3 > 1
    // Relational ops (< >) have higher precedence than logical (&&)
    // Evaluated as: (5 < 10) && (3 > 1) = 1 && 1 = 1
    printf("Expression: 5 < 10 && 3 > 1\n");
    int result4 = 5 < 10 && 3 > 1;
    printf("Result: %d\n\n", result4);  // Output: 1
    
    return 0;
}
```

#### Example 5: Ternary Operator and Right Associativity

```c
#include <stdio.h>

int main() {
    int a = 5, b = 10, c = 3;
    
    // Example 1: Simple ternary
    // Expression: a > b ? a : b
    // Evaluated as: (a > b) ? a : b = (5 > 10) ? 5 : 10 = 10
    int result1 = a > b ? a : b;
    printf("a > b ? a : b = %d\n", result1);  // Output: 10
    
    // Example 2: Ternary is right-associative
    // Expression: a < b ? b < c ? c : b : a
    // Grouped as: a < b ? (b < c ? c : b) : a
    // a < b → 5 < 10 → true
    //   b < c → 10 < 3 → false
    //   Return b → 10
    // Result: 10
    int result2 = a < b ? b < c ? c : b : a;
    printf("a < b ? b < c ? c : b : a = %d\n", result2);  // Output: 10
    
    // Example 3: Ternary mixed with arithmetic
    // Expression: a + 5 > b ? a * 2 : b * 3
    // a + 5 > b → 10 > 10 → false
    // Return b * 3 → 10 * 3 = 30
    int result3 = a + 5 > b ? a * 2 : b * 3;
    printf("a + 5 > b ? a * 2 : b * 3 = %d\n", result3);  // Output: 30
    
    return 0;
}
```

#### Example 6: Assignment Operator Right Associativity

```c
#include <stdio.h>

int main() {
    int a, b, c;
    
    // Example 1: Assignment is right-associative
    // Expression: a = b = c = 5
    // Grouped as: a = (b = (c = 5))
    // c = 5 → assigns 5 to c, returns 5
    // b = 5 → assigns 5 to b, returns 5
    // a = 5 → assigns 5 to a, returns 5
    a = b = c = 5;
    printf("After a = b = c = 5:\n");
    printf("a = %d, b = %d, c = %d\n\n", a, b, c);  // All are 5
    
    // Example 2: Assignment with arithmetic
    // Expression: a = b + 3
    // Evaluated as: a = (b + 3) = 5 + 3 = 8
    a = b + 3;
    printf("After a = b + 3:\n");
    printf("a = %d\n\n", a);  // Output: 8
    
    // Example 3: Chained assignment with expression
    // Expression: a = b = c + 2
    // Grouped as: a = (b = (c + 2))
    // c + 2 = 5 + 2 = 7
    // b = 7 → assigns 7 to b, returns 7
    // a = 7 → assigns 7 to a, returns 7
    c = 5;
    a = b = c + 2;
    printf("After a = b = c + 2 (c was 5):\n");
    printf("a = %d, b = %d, c = %d\n\n", a, b, c);  // a=7, b=7, c=5
    
    return 0;
}
```

#### Example 7: Complex Expression Analysis

```c
#include <stdio.h>

int main() {
    int x = 10, y = 5, z = 2;
    
    // Complex expression: x + y * z - x / z + y % z
    // Step-by-step evaluation:
    // 1. * and / and % (multiplicative, left-to-right)
    //    y * z = 5 * 2 = 10
    //    x / z = 10 / 2 = 5
    //    y % z = 5 % 2 = 1
    // 2. + and - (additive, left-to-right)
    //    x + 10 = 10 + 10 = 20
    //    20 - 5 = 15
    //    15 + 1 = 16
    
    int result = x + y * z - x / z + y % z;
    printf("x + y * z - x / z + y %% z = %d\n", result);
    
    // Let's verify step by step
    int temp1 = y * z;        // 5 * 2 = 10
    int temp2 = x / z;        // 10 / 2 = 5
    int temp3 = y % z;        // 5 % 2 = 1
    int temp4 = x + temp1;    // 10 + 10 = 20
    int temp5 = temp4 - temp2; // 20 - 5 = 15
    int temp6 = temp5 + temp3; // 15 + 1 = 16
    
    printf("Step-by-step verification: %d\n", temp6);
    
    return 0;
}
```

### C Language: Key Points

1. **Precedence is crucial**: Affects correctness of calculations
2. **Parentheses override**: Always available to disambiguate
3. **Assignment is right-associative**: Unlike most operators
4. **Short-circuit evaluation**: Prevents unnecessary computation
5. **Unary operators bind tightly**: Higher than binary operators

---

## Rust Language: Detailed Analysis

### Rust Operator Precedence Table (Complete)

```
┌──────────┬────────────────────────────────┬──────────────┬──────────────────┐
│ Priority │         Operators              │Associativity │      Details     │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   1      │ () [] . ?                      │ L → R        │ Postfix ops      │
│          │ Function call, Index           │              │ ? (try op)       │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   2      │ ! ~ - * &mut (unary)          │ R → L        │ Unary prefix ops │
│          │ - (negation)                   │              │ Deref & borrow   │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   3      │ * / %                          │ L → R        │ Multiplicative   │
│          │ (multiplicative)               │              │ Multiply/Divide  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   4      │ + -                            │ L → R        │ Additive         │
│          │ (additive)                     │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   5      │ << >>                          │ L → R        │ Bitwise shift    │
│          │ (shift operators)              │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   6      │ &                              │ L → R        │ Bitwise AND      │
│          │ (bitwise AND)                  │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   7      │ ^                              │ L → R        │ Bitwise XOR      │
│          │ (bitwise XOR)                  │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   8      │ |                              │ L → R        │ Bitwise OR       │
│          │ (bitwise OR)                   │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   9      │ == != < > <= >=                │ L → R        │ Comparison       │
│          │ (relational/equality)         │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   10     │ &&                             │ L → R        │ Logical AND      │
│          │ (logical AND)                  │              │ Short-circuit    │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   11     │ ||                             │ L → R        │ Logical OR       │
│          │ (logical OR)                   │              │ Short-circuit    │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   12     │ .. ..=                         │ L → R        │ Range operators  │
│          │ (range inclusive/exclusive)   │              │ Rust-specific    │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   13     │ ?:                             │ R → L        │ Ternary cond.    │
│          │ (if expression - Rust style)  │              │ Not an operator  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   14     │ =  +=  -=  *=  /=  %=         │ R → L        │ Assignment ops   │
│          │ &=  |=  ^=  <<=  >>=          │              │                  │
└──────────┴────────────────────────────────┴──────────────┴──────────────────┘
```

### Rust Language Code Examples

#### Example 1: Basic Arithmetic and Precedence

```rust
fn main() {
    let a = 2;
    let b = 3;
    let c = 4;
    let d = 5;
    
    // Example 1: Multiplication before addition
    // Expression: a + b * c - d
    // Evaluated as: a + (b * c) - d = 2 + (3 * 4) - 5 = 2 + 12 - 5 = 9
    let result1 = a + b * c - d;
    println!("a + b * c - d = {}", result1);  // Output: 9
    
    // Example 2: Same operators, left-to-right
    // Expression: a - b - c
    // Evaluated as: (a - b) - c = (2 - 3) - 4 = -1 - 4 = -5
    let result2 = a - b - c;
    println!("a - b - c = {}", result2);      // Output: -5
    
    // Example 3: Parentheses override precedence
    // Expression: (a + b) * (c - d)
    // Evaluated as: (2 + 3) * (4 - 5) = 5 * (-1) = -5
    let result3 = (a + b) * (c - d);
    println!("(a + b) * (c - d) = {}", result3);  // Output: -5
}
```

#### Example 2: Bitwise Operations in Rust

```rust
fn main() {
    let a = 12u32;  // Binary: 1100
    let b = 10u32;  // Binary: 1010
    let c = 5u32;   // Binary: 0101
    
    // Example 1: Bitwise AND has higher precedence than bitwise OR
    // Expression: a | b & c
    // Evaluated as: a | (b & c)
    // b & c = 1010 & 0101 = 0000 = 0
    // a | 0 = 1100 | 0000 = 1100 = 12
    let result1 = a | b & c;
    println!("a | b & c = {}", result1);  // Output: 12
    
    // Example 2: Explicit parentheses change result
    // Expression: (a | b) & c
    // a | b = 1100 | 1010 = 1110 = 14
    // 14 & c = 1110 & 0101 = 0100 = 4
    let result2 = (a | b) & c;
    println!("(a | b) & c = {}", result2);  // Output: 4
    
    // Example 3: Shift operators with arithmetic
    // Expression: a + b << 2
    // Evaluated as: a + (b << 2)
    // b << 2 = 1010 << 2 = 101000 = 40
    // a + 40 = 12 + 40 = 52
    let result3 = a + b << 2;
    println!("a + b << 2 = {}", result3);  // Output: 52
    
    // Example 4: XOR with OR
    // a ^ b | c
    // Evaluated as: (a ^ b) | c
    // a ^ b = 1100 ^ 1010 = 0110 = 6
    // 6 | c = 0110 | 0101 = 0111 = 7
    let result4 = a ^ b | c;
    println!("a ^ b | c = {}", result4);  // Output: 7
}
```

#### Example 3: Ownership and Reference Operations

```rust
fn main() {
    // Unary operators: * (dereference), & (borrow), &mut (mutable borrow)
    let mut x = 5;
    let y = &mut x;
    
    // * dereference has high precedence
    // *y + 3 = dereference y first, then add 3
    let result = *y + 3;  // 5 + 3 = 8
    println!("*y + 3 = {}", result);  // Output: 8
    
    // & borrow has high precedence as unary operator
    let a = 10;
    let reference_to_a = &a;  // Create reference
    println!("reference_to_a = {:p}", reference_to_a);
    
    // Chained references
    let b = 20;
    let ref_b = &b;
    let ref_ref_b = &ref_b;
    // To dereference twice: **ref_ref_b
    println!("**ref_ref_b = {}", **ref_ref_b);  // Output: 20
}
```

#### Example 4: Try Operator (?) Precedence

```rust
fn divide(a: i32, b: i32) -> Result<i32, String> {
    if b == 0 {
        Err("Division by zero".to_string())
    } else {
        Ok(a / b)
    }
}

fn main() {
    // The ? operator has very high precedence (postfix, like .)
    // It propagates the Result/Option immediately
    
    fn safe_operation() -> Result<i32, String> {
        let a = 10;
        let b = 2;
        
        // a / b with ? operator
        // divide(a, b)? returns the Ok value or propagates the Err
        let result = divide(a, b)?;  // Result: 5
        
        // Chaining ? operations
        let result2 = divide(divide(a, b)?, 2)?;  // ((10 / 2) / 2) = 2.5, but integer division = 2
        
        Ok(result2)
    }
    
    match safe_operation() {
        Ok(value) => println!("Result: {}", value),
        Err(e) => println!("Error: {}", e),
    }
}
```

#### Example 5: Range Operators

```rust
fn main() {
    // Rust has range operators with specific precedence
    // .. (exclusive range) and ..= (inclusive range)
    
    // Range operators are low precedence
    // Expression: 1..5 + 1 is interpreted as 1..(5 + 1)
    let r1 = 1..5 + 1;  // Range from 1 to 6 (exclusive)
    for i in r1 {
        print!("{} ", i);  // Prints: 1 2 3 4 5
    }
    println!();
    
    // Inclusive range
    let r2 = 1..=5;  // Range from 1 to 5 (inclusive)
    for i in r2 {
        print!("{} ", i);  // Prints: 1 2 3 4 5
    }
    println!();
    
    // Using in slicing (where they have different semantics)
    let array = [1, 2, 3, 4, 5];
    let slice1 = &array[1..4];  // Elements at indices 1, 2, 3
    println!("Slice [1..4]: {:?}", slice1);  // Output: [2, 3, 4]
    
    let slice2 = &array[1..=3]; // Elements at indices 1, 2, 3
    println!("Slice [1..=3]: {:?}", slice2);  // Output: [2, 3, 4]
}
```

#### Example 6: Logical Operators and Short-Circuit Evaluation

```rust
fn side_effect_false() -> bool {
    println!("  [side_effect_false() called]");
    false
}

fn side_effect_true() -> bool {
    println!("  [side_effect_true() called]");
    true
}

fn main() {
    // Example 1: && with short-circuit
    println!("Expression: false && side_effect_true()");
    let result1 = false && side_effect_true();
    println!("Result: {}\n", result1);  // side_effect_true() NOT called
    
    // Example 2: || with short-circuit
    println!("Expression: true || side_effect_false()");
    let result2 = true || side_effect_false();
    println!("Result: {}\n", result2);  // side_effect_false() NOT called
    
    // Example 3: Multiple logical operators (left-to-right)
    println!("Expression: true && false || true");
    let result3 = true && false || true;
    println!("Result: {}\n", result3);  // Output: true
    
    // Example 4: Comparison before logical
    println!("Expression: 5 < 10 && 3 > 1");
    let result4 = 5 < 10 && 3 > 1;
    println!("Result: {}\n", result4);  // Output: true
}
```

#### Example 7: Type Conversion and Operators

```rust
fn main() {
    // Type conversion has lower precedence in most contexts
    // But as prefix operator (cast), it's high precedence
    
    let a = 3u32;
    let b = 5u32;
    
    // as (type cast) has high precedence
    // Expression: a as i64 + 2
    // Evaluated as: (a as i64) + 2 = 3 + 2 = 5
    let result1 = a as i64 + 2;
    println!("a as i64 + 2 = {}", result1);  // Output: 5
    
    // Example with division
    let x = 10u32;
    let y = 3u32;
    
    // Integer division truncates
    let result2 = x / y;  // 10 / 3 = 3
    println!("x / y = {}", result2);  // Output: 3
    
    // Cast to float for precise division
    let result3 = x as f64 / y as f64;  // 10.0 / 3.0 = 3.333...
    println!("x as f64 / y as f64 = {}", result3);  // Output: 3.333...
}
```

#### Example 8: Complex Expression Analysis

```rust
fn main() {
    let x = 10;
    let y = 5;
    let z = 2;
    
    // Complex expression: x + y * z - x / z + y % z
    // Step-by-step:
    // 1. y * z = 5 * 2 = 10
    // 2. x / z = 10 / 2 = 5
    // 3. y % z = 5 % 2 = 1
    // 4. x + 10 = 10 + 10 = 20
    // 5. 20 - 5 = 15
    // 6. 15 + 1 = 16
    
    let result = x + y * z - x / z + y % z;
    println!("x + y * z - x / z + y % z = {}", result);  // Output: 16
    
    // Rust way: with more explicit intermediate steps
    let mult = y * z;
    let div = x / z;
    let rem = y % z;
    let step1 = x + mult;
    let step2 = step1 - div;
    let final_result = step2 + rem;
    
    println!("Step-by-step verification: {}", final_result);  // Output: 16
}
```

#### Example 9: Comparison Chaining (Common Pitfall)

```rust
fn main() {
    // Rust does NOT support comparison chaining like Python
    // This is a common mistake:
    
    let x = 5;
    
    // ❌ WRONG: This compiles but does something unexpected
    // x > 3 > 1 is evaluated as (x > 3) > 1
    // x > 3 = 5 > 3 = true (bool)
    // true > 1 → comparisons convert bool to u8 (1 for true, 0 for false)
    // 1 > 1 = false
    let wrong = x > 3 > 1;
    println!("x > 3 > 1 = {}", wrong);  // Output: false (UNEXPECTED!)
    
    // ✓ CORRECT: Use && to chain comparisons
    let correct = x > 3 && x < 10;
    println!("x > 3 && x < 10 = {}", correct);  // Output: true
    
    // Another example of the pitfall:
    let a = 5;
    let b = 5;
    let c = 5;
    
    // ❌ This doesn't work as you might expect:
    // a == b == c is evaluated as (a == b) == c
    // a == b = 5 == 5 = true
    // true == c = true == 5 → ERROR: can't compare bool to i32
    
    // ✓ CORRECT: Use &&
    let result = a == b && b == c;
    println!("a == b && b == c = {}", result);  // Output: true
}
```

### Rust Language: Key Points

1. **Try operator (?) has postfix precedence**: Similar to . and function calls
2. **Range operators (..) are lower precedence**: 1..5+1 means 1..(5+1)
3. **Type casting (as) is high precedence**: Binds tighter than arithmetic
4. **No comparison chaining**: Must use logical operators (&&, ||)
5. **Ownership operators (&, *) are unary**: High precedence, right-associative
6. **Short-circuit evaluation**: Prevents unnecessary computation like C

---

## Go Language: Detailed Analysis

### Go Operator Precedence Table (Complete)

```
┌──────────┬────────────────────────────────┬──────────────┬──────────────────┐
│ Priority │         Operators              │Associativity │      Details     │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   1      │ () [] {} . ->                  │ L → R        │ Postfix ops      │
│ (High)   │ Function call, Index, Slice    │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   2      │ ! ^ <- (unary)                 │ R → L        │ Unary prefix ops │
│          │ + - * & (unary)                │              │ Channel receive  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   3      │ * / % << >> & &^               │ L → R        │ Multiplicative   │
│          │ (multiplicative)               │              │ & (AND)          │
│          │ &^ (AND NOT)                   │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   4      │ + -                            │ L → R        │ Additive         │
│          │ | ^                            │              │ | (OR), ^ (XOR)  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   5      │ == != < <= > >=                │ L → R        │ Comparison       │
│          │ (relational/equality)         │              │                  │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   6      │ &&                             │ L → R        │ Logical AND      │
│          │ (logical AND)                  │              │ Short-circuit    │
├──────────┼────────────────────────────────┼──────────────┼──────────────────┤
│   7      │ ||                             │ L → R        │ Logical OR       │
│ (Low)    │ (logical OR)                   │              │ Short-circuit    │
└──────────┴────────────────────────────────┴──────────────┴──────────────────┘
```

**Note**: Go has NO ternary operator (? :), NO comma operator, NO range operator.

### Go Language Code Examples

#### Example 1: Basic Arithmetic and Precedence

```go
package main

import "fmt"

func main() {
    a := 2
    b := 3
    c := 4
    d := 5
    
    // Example 1: Multiplication before addition
    // Expression: a + b * c - d
    // Evaluated as: a + (b * c) - d = 2 + (3 * 4) - 5 = 2 + 12 - 5 = 9
    result1 := a + b*c - d
    fmt.Printf("a + b * c - d = %d\n", result1)  // Output: 9
    
    // Example 2: Same operators, left-to-right
    // Expression: a - b - c
    // Evaluated as: (a - b) - c = (2 - 3) - 4 = -1 - 4 = -5
    result2 := a - b - c
    fmt.Printf("a - b - c = %d\n", result2)      // Output: -5
    
    // Example 3: Parentheses override precedence
    // Expression: (a + b) * (c - d)
    // Evaluated as: (2 + 3) * (4 - 5) = 5 * (-1) = -5
    result3 := (a + b) * (c - d)
    fmt.Printf("(a + b) * (c - d) = %d\n", result3)  // Output: -5
}
```

#### Example 2: Bitwise Operations in Go

```go
package main

import "fmt"

func main() {
    a := uint32(12)  // Binary: 1100
    b := uint32(10)  // Binary: 1010
    c := uint32(5)   // Binary: 0101
    
    // Example 1: Bitwise AND has higher precedence than bitwise OR
    // Expression: a | b & c
    // Evaluated as: a | (b & c)
    // b & c = 1010 & 0101 = 0000 = 0
    // a | 0 = 1100 | 0000 = 1100 = 12
    result1 := a | b & c
    fmt.Printf("a | b & c = %d\n", result1)  // Output: 12
    
    // Example 2: Explicit parentheses change result
    // Expression: (a | b) & c
    // a | b = 1100 | 1010 = 1110 = 14
    // 14 & c = 1110 & 0101 = 0100 = 4
    result2 := (a | b) & c
    fmt.Printf("(a | b) & c = %d\n", result2)  // Output: 4
    
    // Example 3: Shift operators with arithmetic
    // Expression: a + b << 2
    // Evaluated as: a + (b << 2)
    // b << 2 = 1010 << 2 = 101000 = 40
    // a + 40 = 12 + 40 = 52
    result3 := a + b<<2
    fmt.Printf("a + b << 2 = %d\n", result3)  // Output: 52
    
    // Example 4: AND NOT operator (&^)
    // a &^ b clears bits in a that are set in b
    // a = 1100, b = 1010
    // a &^ b = 1100 &^ 1010 = 0100 = 4
    result4 := a &^ b
    fmt.Printf("a &^ b = %d\n", result4)  // Output: 4
    
    // Example 5: XOR with OR
    // a ^ b | c
    // Evaluated as: (a ^ b) | c
    // a ^ b = 1100 ^ 1010 = 0110 = 6
    // 6 | c = 0110 | 0101 = 0111 = 7
    result5 := a ^ b | c
    fmt.Printf("a ^ b | c = %d\n", result5)  // Output: 7
}
```

#### Example 3: Pointer and Reference Operations

```go
package main

import "fmt"

func main() {
    // Unary * (dereference) and & (address-of) operators
    x := 5
    
    // & creates a pointer to x
    ptr := &x
    fmt.Printf("Address of x: %p\n", ptr)
    
    // * dereferences the pointer
    value := *ptr
    fmt.Printf("Value at ptr: %d\n", value)  // Output: 5
    
    // Example: Precedence of * and +
    // *ptr + 3 = dereference ptr first, then add 3
    result := *ptr + 3  // 5 + 3 = 8
    fmt.Printf("*ptr + 3 = %d\n", result)  // Output: 8
    
    // Modifying through pointer
    *ptr = 10
    fmt.Printf("x after *ptr = 10: %d\n", x)  // Output: 10
}
```

#### Example 4: Logical Operators and Short-Circuit Evaluation

```go
package main

import "fmt"

func sideEffectFalse() bool {
    fmt.Println("  [sideEffectFalse() called]")
    return false
}

func sideEffectTrue() bool {
    fmt.Println("  [sideEffectTrue() called]")
    return true
}

func main() {
    // Example 1: && with short-circuit
    fmt.Println("Expression: false && sideEffectTrue()")
    result1 := false && sideEffectTrue()
    fmt.Printf("Result: %v\n\n", result1)  // sideEffectTrue() NOT called
    
    // Example 2: || with short-circuit
    fmt.Println("Expression: true || sideEffectFalse()")
    result2 := true || sideEffectFalse()
    fmt.Printf("Result: %v\n\n", result2)  // sideEffectFalse() NOT called
    
    // Example 3: Multiple logical operators (left-to-right)
    fmt.Println("Expression: true && false || true")
    result3 := true && false || true
    fmt.Printf("Result: %v\n\n", result3)  // Output: true
    
    // Example 4: Comparison before logical
    fmt.Println("Expression: 5 < 10 && 3 > 1")
    result4 := 5 < 10 && 3 > 1
    fmt.Printf("Result: %v\n\n", result4)  // Output: true
}
```

#### Example 5: Conditional Logic Without Ternary Operator

```go
package main

import "fmt"

func main() {
    // Go does NOT have a ternary operator (?:)
    // Use if-else statements instead
    
    a := 5
    b := 10
    
    // To find maximum, use if-else
    var max int
    if a > b {
        max = a
    } else {
        max = b
    }
    fmt.Printf("Maximum of %d and %d: %d\n", a, b, max)  // Output: 10
    
    // Inline if-else can be used in some contexts
    // But this is NOT a ternary operator - it's a different construct
    
    // Example: Swapping with logical operators (clever but not recommended)
    // Using AND/OR as conditional: (condition && value1) || value2
    // This works because:
    // - true && x evaluates to x
    // - false && x evaluates to false
    // - false || x evaluates to x
    // - true || x evaluates to true
    
    condition := a > b
    result := func() int {
        if condition {
            return a
        }
        return b
    }()
    fmt.Printf("Result: %d\n", result)  // Output: 10
}
```

#### Example 6: Channel Operations

```go
package main

import (
    "fmt"
    "time"
)

func main() {
    // Channel receive operator <-
    // It's a unary operator with high precedence
    
    ch := make(chan int, 1)
    
    // Send to channel
    ch <- 42
    
    // Receive from channel
    // <-ch has high precedence, similar to function calls
    value := <-ch
    fmt.Printf("Received: %d\n", value)  // Output: 42
    
    // Example: Arithmetic with channel receive
    ch <- 10
    result := <-ch + 5  // Receive 10, then add 5
    fmt.Printf("Result: %d\n", result)  // Output: 15
    
    // Example: Using in conditions
    ch <- true
    if <-ch {
        fmt.Println("Channel received true")
    }
}
```

#### Example 7: Type Assertion and Interfaces

```go
package main

import "fmt"

func main() {
    // Type assertion: value.(Type)
    // Has high precedence (postfix operator)
    
    var i interface{} = 42
    
    // Type assertion
    value, ok := i.(int)
    if ok {
        fmt.Printf("Value is int: %d\n", value)
        
        // Type assertion in expression
        // value.(int) + 10 = type assertion first, then add
        result := value + 10  // 42 + 10 = 52
        fmt.Printf("value + 10 = %d\n", result)  // Output: 52
    }
    
    // Map with interface values
    m := make(map[string]interface{})
    m["count"] = 100
    m["name"] = "test"
    
    // Type assertion when accessing map
    if count, ok := m["count"].(int); ok {
        fmt.Printf("Count: %d\n", count)  // Output: 100
    }
}
```

#### Example 8: Slice and Array Indexing

```go
package main

import "fmt"

func main() {
    // Array indexing and slicing have high precedence
    
    arr := []int{1, 2, 3, 4, 5}
    
    // Example 1: Index operation (high precedence)
    // arr[0] + arr[1] * arr[2]
    // Evaluated as: arr[0] + (arr[1] * arr[2])
    // 1 + (2 * 3) = 1 + 6 = 7
    result1 := arr[0] + arr[1]*arr[2]
    fmt.Printf("arr[0] + arr[1]*arr[2] = %d\n", result1)  // Output: 7
    
    // Example 2: Slicing (returns a new slice)
    slice := arr[1:4]  // Elements at indices 1, 2, 3
    fmt.Printf("arr[1:4] = %v\n", slice)  // Output: [2 3 4]
    
    // Example 3: Nested indexing
    matrix := [][]int{
        {1, 2, 3},
        {4, 5, 6},
        {7, 8, 9},
    }
    
    // matrix[1][2] + 10
    // Evaluated as: (matrix[1][2]) + 10 = 6 + 10 = 16
    result2 := matrix[1][2] + 10
    fmt.Printf("matrix[1][2] + 10 = %d\n", result2)  // Output: 16
}
```

#### Example 9: Complex Expression Analysis

```go
package main

import "fmt"

func main() {
    x := 10
    y := 5
    z := 2
    
    // Complex expression: x + y * z - x / z + y % z
    // Step-by-step evaluation:
    // 1. * / % (multiplicative, left-to-right)
    //    y * z = 5 * 2 = 10
    //    x / z = 10 / 2 = 5
    //    y % z = 5 % 2 = 1
    // 2. + - (additive, left-to-right)
    //    x + 10 = 10 + 10 = 20
    //    20 - 5 = 15
    //    15 + 1 = 16
    
    result := x + y*z - x/z + y%z
    fmt.Printf("x + y * z - x / z + y %% z = %d\n", result)  // Output: 16
    
    // Verify step-by-step
    temp1 := y * z        // 5 * 2 = 10
    temp2 := x / z        // 10 / 2 = 5
    temp3 := y % z        // 5 % 2 = 1
    temp4 := x + temp1    // 10 + 10 = 20
    temp5 := temp4 - temp2 // 20 - 5 = 15
    temp6 := temp5 + temp3 // 15 + 1 = 16
    
    fmt.Printf("Step-by-step verification: %d\n", temp6)  // Output: 16
}
```

#### Example 10: Comparison with Go's Specific Rules

```go
package main

import "fmt"

func main() {
    // Go-specific: No ternary operator
    // Go-specific: No comparison chaining
    // Go-specific: & in multiplication context has same precedence as *
    
    a := 5
    b := 3
    
    // Unlike other languages:
    // - No (condition) ? trueVal : falseVal syntax
    // - Must use if-else
    
    // Address of operator & in multiplicative context
    // &x has high precedence, used for pointers
    
    // Bitwise AND also uses &
    x := 12    // 1100
    y := 10    // 1010
    
    // x & y = 1100 & 1010 = 1000 = 8
    bitwiseResult := x & y
    fmt.Printf("x & y = %d\n", bitwiseResult)  // Output: 8
    
    // Address-of creates a pointer
    ptr := &a
    fmt.Printf("Address of a: %p\n", ptr)
    
    // Dereferencing
    derefValue := *ptr
    fmt.Printf("Dereferenced value: %d\n", derefValue)  // Output: 5
}
```

### Go Language: Key Points

1. **No ternary operator**: Use if-else instead
2. **No comma operator**: Statements must be separated by newlines or braces
3. **No range operator**: Different syntax for loops
4. **AND NOT operator (&^)**: Clears bits - unique to Go
5. **Channel receive (<-)**: Unary operator, high precedence
6. **Short-circuit evaluation**: && and || work like other languages
7. **No comparison chaining**: Must use logical operators

---

## Practical Implications

### Real-World Memory Access Pattern

```
Scenario: Evaluating a + b[i] * c[j] - d

┌────────────────────────────────────────────────────────────────┐
│                    MEMORY ACCESS TIMELINE                      │
├────────────────────────────────────────────────────────────────┤

T0: Load variable 'a'
    Read from memory address [rbp-8] → RAX = a

T1: Calculate array index access b[i]
    Load i value → RCX
    Load array base address of b → RBX
    Address = RBX + (RCX * element_size)
    Load from address → RDX = b[i]

T2: Load variable 'c'
    Read from memory address [rbp-24] → RSI

T3: Calculate array index access c[j]
    Load j value → R8
    Load array base address of c → R9
    Address = R9 + (R8 * element_size)
    Load from address → R10 = c[j]

T4: Multiply b[i] * c[j]
    RDX * R10 → R11 (product in R11)

T5: Add a + product
    RAX + R11 → R12

T6: Load variable 'd'
    Read from memory address [rbp-32] → R13

T7: Subtract
    R12 - R13 → Result

Total memory accesses: 6 (a, b[i], c[j], d, and 2 for array addressing)
```

### CPU Cache Impact

```
Expression: sum = arr[0] + arr[1] + arr[2] + arr[3]

First execution (cold cache):
┌──────────────────────────────────────────────────┐
│        L1 Cache (very fast, 4 cycles)            │
├──────────────────────────────────────────────────┤
│ Miss ──→ Load arr[0] from memory (100+ cycles)  │
│          Fill cache line with arr[0..3]          │
│ Hit  ←─ arr[1], arr[2], arr[3] in cache (fast)  │
└──────────────────────────────────────────────────┘

Subsequent executions (warm cache):
┌──────────────────────────────────────────────────┐
│        L1 Cache (very fast, 4 cycles)            │
├──────────────────────────────────────────────────┤
│ Hit  ←─ All array elements already in cache      │
│        Each access: ~4 cycles                    │
└──────────────────────────────────────────────────┘

Total time improvement: ~25x faster for warm cache
```

### Order of Evaluation and Side Effects

```
Critical Example: Function Evaluation Order

Expression: foo(a) + bar(b) * baz(c)

┌─────────────────────────────────────────────────┐
│ Multiplication has higher precedence than add   │
│ So: foo(a) + (bar(b) * baz(c))                 │
│                                                 │
│ But what's the order of function CALLS?         │
│ This is UNSPECIFIED in C and Rust!              │
│                                                 │
│ Possible execution orders:                      │
│                                                 │
│ Order 1: foo(a), bar(b), baz(c)                │
│ Order 2: foo(a), baz(c), bar(b)                │
│ Order 3: bar(b), foo(a), baz(c)                │
│ Order 4: bar(b), baz(c), foo(a)                │
│ Order 5: baz(c), foo(a), bar(b)                │
│ Order 6: baz(c), bar(b), foo(a)                │
│                                                 │
│ All are valid! Compiler can choose any.         │
└─────────────────────────────────────────────────┘

In Go, left-to-right evaluation is guaranteed:
├─ foo(a) is evaluated first
├─ bar(b) is evaluated second
└─ baz(c) is evaluated third

Never write code that depends on evaluation order
in languages where it's unspecified!
```

---

## Common Pitfalls

### Pitfall 1: Confusion Between Arithmetic and Bitwise Operators

```c
// C example
int x = 5 + 3 << 2;  // What's the result?

// Incorrect understanding:
// (5 + 3) << 2 = 8 << 2 = 32 ❌ WRONG!

// Correct understanding:
// + has precedence 2 (lower than << which is 3)
// So: 5 + (3 << 2) = 5 + 12 = 17 ✓ CORRECT!
```

### Pitfall 2: Assignment as Expression

```c
// C pitfall
if (x = 5) {  // This assigns 5 to x, not comparing!
    // x is now 5, and if evaluates to true
}

// Correct:
if (x == 5) {  // This compares x with 5
    // Only enters if x equals 5
}
```

### Pitfall 3: Unary Minus Precedence

```c
// C pitfall
int x = 2 ^ 3 * -4;  // What does this evaluate to?

// Correct understanding:
// -4 is evaluated first (unary - is high precedence)
// 3 * (-4) = -12
// 2 ^ (-12) = bitwise XOR... wait, this probably isn't what you intended!

int correct = 2 ^ (3 * (-4));  // More explicit
```

### Pitfall 4: Order of Operations with Mixed Shifts

```go
// Go pitfall
x := 8 + 4 >> 2  // What's the result?

// Incorrect: (8 + 4) >> 2 = 12 >> 2 = 3
// Correct: 8 + (4 >> 2) = 8 + 1 = 9

// Reason: >> has higher precedence than +
// The >> operator in Go is multiplicative level
```

### Pitfall 5: Pointer Dereferencing Confusion

```rust
// Rust pitfall
let ptr = &5;
let result = *ptr + 3;  // What is this?

// Correct: Dereference first (*ptr), then add
// result = 5 + 3 = 8

// The * is unary dereference, not multiplication here
// It has high precedence and is right-associative
```

### Pitfall 6: Boolean as Integer Subtlety

```rust
// Rust: No implicit bool-to-int conversion
let x = 5;
let y = 3;

// ❌ This compiles to a comparison
// (x > y) > 1 means:
// x > y returns bool (true = 1 in C)
// true > 1 is comparing bool to int... wait, can't do this in Rust!

// But in C:
int result = 5 > 3 > 1;  // (5 > 3) > 1 = 1 > 1 = 0 (false)
```

---

## Performance Considerations

### Branch Prediction and Operator Evaluation

```
Evaluating: (unlikely_condition && expensive_function())

CPU Pipeline:
┌────────────────────────────────────────────────────┐
│         BRANCH PREDICTION IMPACT                   │
├────────────────────────────────────────────────────┤

Scenario 1: Short-circuit works (condition is false)
Time: ~2 cycles (just one comparison)
└────────────────────────────────────────────────────┘

Scenario 2: Without short-circuit (always evaluate)
Time: ~2 cycles + expensive_function() execution time
└────────────────────────────────────────────────────┘

With good branch prediction:
- CPU predicts unlikely_condition as false
- Speculatively skips expensive_function()
- Saves thousands of cycles

With bad branch prediction:
- CPU mispredicts and executes expensive_function()
- Then flushes the pipeline
- Costs 10+ cycles per misprediction
```

### Operator Selection for Performance

```
Register allocation for: a + b * c - d / e

┌──────────────────────────────────────────────────┐
│           REGISTER EFFICIENCY                    │
├──────────────────────────────────────────────────┤

Option 1: Evaluate left-to-right
  temp1 = b * c      (uses 2 registers)
  temp2 = d / e      (uses 2 registers)
  temp3 = a + temp1  (uses 2 registers)
  result = temp3 - temp2 (uses 2 registers)
  
  Peak registers used: 4

Option 2: Better grouping with parentheses
  ((a + b) * (c - d)) / e
  Allows intermediate results to be reused
  
  Peak registers used: 3

Modern compilers optimize this automatically,
but explicit parentheses can guide optimization.
```

### Cache-Friendly Operator Usage

```
Array access patterns:

Pattern 1: Non-sequential access (SLOW)
┌─────────────────────────────────────┐
│ for i in 0..n:                      │
│   sum += arr[i*stride]              │
│   // Stride causes cache misses     │
│   // Each access misses L1 cache    │
└─────────────────────────────────────┘
Effective bandwidth: Low (many cache misses)

Pattern 2: Sequential access (FAST)
┌─────────────────────────────────────┐
│ for i in 0..n:                      │
│   sum += arr[i]                     │
│   // Sequential access               │
│   // Prefetcher fills cache          │
│   // One cache line serves multiple  │
└─────────────────────────────────────┘
Effective bandwidth: High (few cache misses)

Performance ratio: 10-100x difference possible!
```

### Compiler Optimization Based on Precedence

```
Source Code:
  int x = a + b + c;
  int y = a + (b + c);

Compiled Result:
  Both compile to same machine code due to
  addition being associative. Compiler recognizes
  this and optimizes accordingly.

But with non-associative operations:
  double x = a - b - c;
  double y = a - (b - c);
  
Compiled to DIFFERENT machine code:
  x = (a - b) - c   // First subtract b, then c
  y = a - (b - c)   // First compute b-c, then subtract from a
  
Different results due to floating-point rounding!
This is NOT operator precedence; it's associativity.
```

---

## Summary Comparison Table

```
┌──────────────────┬────────────────────┬────────────────┬─────────────────┐
│  Feature         │        C           │      Rust      │        Go       │
├──────────────────┼────────────────────┼────────────────┼─────────────────┤
│ Operator Count   │        46          │        50      │        35       │
│ Precedence Levels│        15          │        13      │         7       │
│ Ternary Op       │  Yes (? :)         │  Yes (? :)     │  No             │
│ Comma Operator   │  Yes               │  No            │  No             │
│ Assignment Op    │  Right-assoc       │  Right-assoc   │  Right-assoc    │
│ Try Operator     │  N/A               │  Yes (?)       │  N/A            │
│ Channel Op       │  N/A               │  N/A           │  Yes (<-)       │
│ AND NOT Op (&^)  │  No                │  No            │  Yes            │
│ Comparison Chain │  No                │  No            │  No             │
│ Short-circuit    │  Yes               │  Yes           │  Yes            │
│ Eval Order       │  Unspecified       │  Unspecified   │  Left-to-right  │
└──────────────────┴────────────────────┴────────────────┴─────────────────┘
```

---

## Final Recommendations

### Best Practices

1. **Use Parentheses Liberally**
   ```c
   // Bad: Relies on precedence knowledge
   int result = a + b * c - d / e;
   
   // Good: Clear intent
   int result = a + (b * c) - (d / e);
   ```

2. **Know Your Language's Rules**
   - C: 15 precedence levels, right-assoc assignment
   - Rust: 13 levels, has ? operator, no ternary chaining
   - Go: 7 levels, no ternary, guaranteed left-to-right

3. **Test Complex Expressions**
   ```c
   // When in doubt, break it down:
   int a = expression_part_1;
   int b = expression_part_2;
   int result = a + b;
   ```

4. **Avoid Side Effects in Operator Evaluation**
   ```c
   // Bad: Order of function calls is unspecified
   int x = foo() + bar() * baz();
   
   // Good: Explicit order
   int a = foo();
   int b = bar();
   int c = baz();
   int x = a + b * c;
   ```

5. **Document Non-Obvious Precedence**
   ```c
   // When precedence isn't obvious, add a comment:
   // Bitwise AND binds tighter than bitwise OR
   int mask = x & 0xFF | y & 0xFFFF;
   ```

---

## References and Further Reading

- C Standard: ISO/IEC 9899:2018 (C18)
- Rust Reference: https://doc.rust-lang.org/reference/
- Go Specification: https://golang.org/ref/spec
- Implementation-specific behavior in each language

---

## Conclusion

Understanding operator precedence and associativity is fundamental to writing correct, efficient, and maintainable code across C, Rust, and Go. While these concepts are similar across languages, each has nuances:

- **C** offers maximum flexibility with complex precedence rules
- **Rust** adds modern features while maintaining compatibility
- **Go** simplifies with fewer precedence levels and guaranteed evaluation order

The mental model to internalize is:
1. Precedence determines grouping (AST construction)
2. Associativity determines grouping direction for same-precedence operators
3. Evaluation order may be unspecified (except in Go)
4. Side effects and memory access can impact performance significantly

Master these concepts, and you'll write more reliable, performant code across all three languages.

# Operator Precedence and Associativity in C, Rust, and Go
## A Complete, In-Depth Technical Reference

---

> **Why This Matters**
> Operator precedence and associativity are not merely syntax trivia — they form the foundation of how compilers interpret every expression you write. A wrong mental model here produces silent logical errors, undefined behavior (in C), surprising type coercions, and security vulnerabilities. Mastering these rules to the point of intuition gives you the ability to read, write, and reason about systems-level code with precision, speed, and confidence.

---

## Table of Contents

1. [Foundational Concepts](#1-foundational-concepts)
2. [How Compilers Parse Expressions](#2-how-compilers-parse-expressions)
3. [Mental Models](#3-mental-models)
4. [C — Complete Reference](#4-c--complete-reference)
5. [Rust — Complete Reference](#5-rust--complete-reference)
6. [Go — Complete Reference](#6-go--complete-reference)
7. [Cross-Language Comparison](#7-cross-language-comparison)
8. [Common Pitfalls and Bugs](#8-common-pitfalls-and-bugs)
9. [Best Practices](#9-best-practices)

---

## 1. Foundational Concepts

### 1.1 What Is Operator Precedence?

Operator precedence defines the **binding strength** of each operator relative to all others. When an expression contains multiple different operators, precedence determines which operands "belong to" which operator — i.e., how the expression is implicitly parenthesized before evaluation.

The rule is absolute: **Higher precedence = tighter binding = that operator claims its operands first.**

```c
int x = 2 + 3 * 4;   // Result: 14, not 20
//                        Because * binds tighter than +
//                        Implicit grouping: 2 + (3 * 4) = 2 + 12 = 14
```

Think of precedence as **gravitational pull**: operators with higher precedence exert stronger pull on adjacent operands. The `*` operator pulls `3` and `4` to itself before `+` can pull them.

Precedence is a **purely syntactic/parse-time rule**. It shapes the Abstract Syntax Tree (AST) the compiler builds. It says nothing about which computation happens first at runtime — that is a separate concept.

### 1.2 What Is Associativity?

Associativity resolves the grouping ambiguity when **multiple operators of the same precedence level** appear consecutively without explicit parentheses.

#### Left-to-Right (Left Associative)

The chain groups from left to right. The leftmost operation "forms" first. This is by far the most common.

```
a OP b OP c  →  (a OP b) OP c
```

```c
5 - 3 - 1   →  (5 - 3) - 1  =  1     (NOT  5 - (3-1) = 3)
8 / 4 / 2   →  (8 / 4) / 2  =  1     (NOT  8 / (4/2) = 4)
a.b.c       →  (a.b).c              (access member b of a, then member c of that)
```

ASCII tree for `5 - 3 - 1` (left-associative):
```
      -            ← outermost (last to form)
     / \
    -   1
   / \
  5   3            ← innermost (first to form)
```

#### Right-to-Left (Right Associative)

The chain groups from right to left. The rightmost operation "forms" first. Common for assignment, unary prefix, and some special operators.

```
a OP b OP c  →  a OP (b OP c)
```

```c
a = b = c   →  a = (b = c)      // assignment is right-associative
a += b += 1 →  a += (b += 1)    // compound assignment too
-~x         →  -(~x)             // unary prefix chain: right-to-left
!!x         →  !(!x)             // double logical NOT
```

ASCII tree for `a = b = 5` (right-associative):
```
      =            ← outermost (processed last)
     / \
    a   =          ← inner (processed first)
       / \
      b   5
```

Evaluation of `a = b = 5`:
```
Step 1: b = 5   → b is now 5; expression value is 5
Step 2: a = 5   → a is now 5; expression value is 5
```

#### Non-Associative

Some languages declare certain operators as non-associative. Chaining them at the same level is a **compile-time error**. This prevents silent logical bugs.

```rust
// Rust: comparison operators are non-associative
let x = 1 < 2 < 3;   // COMPILE ERROR: chained comparison not allowed
let x = 1 < 2 && 2 < 3; // CORRECT: use && to chain comparisons
```

```go
// Go: also non-associative for comparisons
var x = 1 < 2 < 3  // COMPILE ERROR
```

### 1.3 Operator Arity

Operators are classified by the number of operands they take:

```
UNARY    — 1 operand:   -x,  !x,  ~x,  &x,  *x,  ++x,  x++
BINARY   — 2 operands:  a+b, a*b, a&&b, a=b
TERNARY  — 3 operands:  condition ? true_expr : false_expr  (C only among the three)
```

Arity affects how precedence rules interact. Unary operators are almost always higher precedence than binary operators (they bind tighter to the single adjacent token).

### 1.4 The Critical Distinction: Precedence ≠ Evaluation Order

This is the single most misunderstood concept in expression semantics. Many experienced programmers confuse these.

```
Precedence / Associativity  →  determines GROUPING (AST structure, parse-time)
Evaluation Order            →  determines which SUBEXPRESSION runs FIRST at runtime
```

These are orthogonal concerns.

```c
int f1() { printf("f1 "); return 1; }
int f2() { printf("f2 "); return 2; }

int x = f1() + f2();
//      ^^^^^^^^^^^^  grouping: (f1() + f2()) assigned to x
//                    + has higher precedence than =: ✓ clear
//                    BUT: is f1() or f2() called first? UNSPECIFIED in C!
```

The compiler is free to evaluate `f1()` or `f2()` first. The precedence of `+` over `=` only tells us the grouping: `(f1() + f2())` is computed, then stored in `x`. But the relative order of evaluating `f1()` and `f2()` is not guaranteed.

```
GROUPING (determined by precedence):        EVALUATION ORDER (NOT by precedence):
         =                                         ?  f1() before f2() ?
        / \                                        ?  f2() before f1() ?
       x   +                                       The C standard says: UNSPECIFIED.
          / \                                       Rust: left-to-right guaranteed.
        f1() f2()                                  Go: left-to-right for most contexts.
```

This distinction is one of the roots of undefined behavior in C. Rust and Go largely eliminate it with explicit evaluation-order rules.

### 1.5 Short-Circuit Evaluation

The logical AND (`&&`) and OR (`||`) operators in all three languages have a special property: they **do not necessarily evaluate the right operand**.

```
&&:  If left is FALSE → right is NOT evaluated; result is false
||:  If left is TRUE  → right is NOT evaluated; result is true
```

This is **not** a precedence rule — it is a semantic rule that provides a **sequence point** (C) / defined evaluation order (Rust/Go). It is relied upon heavily for safe pointer access and performance.

```c
// Safe null check (short-circuit protects against null dereference):
if (node != NULL && node->value > 0) { ... }
// If node is NULL, node->value is NEVER accessed ← short-circuit saves us

// Side-effect-free optimization:
int expensive_check();
if (quick_check() || expensive_check()) { ... }
// expensive_check() is only called if quick_check() returns false
```

---

## 2. How Compilers Parse Expressions

### 2.1 From Source Code to AST — The Full Pipeline

```
  Source Code: "2 + 3 * 4 - 1"
        │
        ▼
  ┌────────────────────────────────────────┐
  │  LEXER (Tokenizer)                     │
  │  Splits input into atomic tokens       │
  └────────────────────────────────────────┘
        │
        ▼
  Token Stream: [INT:2] [PLUS] [INT:3] [STAR] [INT:4] [MINUS] [INT:1]
        │
        ▼
  ┌────────────────────────────────────────┐
  │  PARSER                                │
  │  Builds tree respecting precedence     │
  │  and associativity rules               │
  │  (Recursive descent or Pratt parsing)  │
  └────────────────────────────────────────┘
        │
        ▼
  Abstract Syntax Tree (AST):
             -
            / \
           +   1
          / \
         2   *
            / \
           3   4
        │
        ▼
  ┌────────────────────────────────────────┐
  │  SEMANTIC ANALYSIS                     │
  │  Type checking, name resolution        │
  │  Constant folding, optimization passes │
  └────────────────────────────────────────┘
        │
        ▼
  ┌────────────────────────────────────────┐
  │  CODE GENERATION                       │
  │  Intermediate Representation (IR)      │
  │  → LLVM IR / GCC RTL / etc.            │
  └────────────────────────────────────────┘
        │
        ▼
  Machine Code / Object File
```

### 2.2 How Grammar Encodes Precedence

The language grammar itself encodes precedence by **nesting rules**. Each rule calls the next-higher-precedence rule for its operands. This nesting ensures higher-precedence operators claim their operands before lower-precedence ones can.

A simplified C-like expression grammar (higher-precedence rules are deeper):

```
expression     := assignment

assignment     := conditional (( '=' | '+=' | '-=' | ... ) assignment)?
                  ^── right-recursive → right-associative

conditional    := logical_or ( '?' expression ':' conditional )?

logical_or     := logical_and ( '||' logical_and )*

logical_and    := bitwise_or ( '&&' bitwise_or )*

bitwise_or     := bitwise_xor ( '|' bitwise_xor )*

bitwise_xor    := bitwise_and ( '^' bitwise_and )*

bitwise_and    := equality ( '&' equality )*

equality       := relational ( ( '==' | '!=' ) relational )*

relational     := shift ( ( '<' | '>' | '<=' | '>=' ) shift )*

shift          := additive ( ( '<<' | '>>' ) additive )*

additive       := multiplicative ( ( '+' | '-' ) multiplicative )*
                  ^── loop + left-recursive calls → left-associative

multiplicative := unary ( ( '*' | '/' | '%' ) unary )*

unary          := ( '+' | '-' | '!' | '~' | '*' | '&' | '++' | '--' ) unary
               |  postfix
               ^── right-recursive → right-associative for prefix operators

postfix        := primary ( '++' | '--' | '[' expr ']' | '(' args ')' | '.' ID | '->' ID )*
                  ^── loop with no recursion → left-associative

primary        := NUMBER | STRING | IDENTIFIER | '(' expression ')'
```

**Reading this grammar:**
- `additive` calls `multiplicative` for operands → `*` and `/` are applied **before** `+` and `-`
- `logical_and` calls `bitwise_or` for operands → `|` is applied **before** `&&`
- `assignment` right-recurses on itself → right-associativity
- The loops `( op operand )*` in `additive`, `multiplicative`, etc. create left-associativity

### 2.3 Detailed AST Examples

#### Example A: `2 + 3 * 4 - 1`

```
Tokens: 2  +  3  *  4  -  1

Parse:
  multiplicative at level 3:  3*4 groups first
  additive at level 4:        left-associative → (2 + 12) - 1

                  -           ← last operation
                 / \
                +   1
               / \
              2   *           ← first operation (highest precedence)
                 / \
                3   4

Post-order evaluation:
  1. 3 * 4 = 12
  2. 2 + 12 = 14
  3. 14 - 1 = 13
```

#### Example B: `a = b = c = 5` (right-associative assignment)

```
Tokens: a  =  b  =  c  =  5

Parse (right-recursive grammar):
  assignment sees: a = (rest)
  rest: b = (rest)
  rest: c = 5

              =           ← assigned last (outermost)
             / \
            a   =         ← assigned second
               / \
              b   =       ← assigned first (innermost)
                 / \
                c   5

Evaluation (right-to-left, post-order):
  1. c = 5  → c is 5; expression yields 5
  2. b = 5  → b is 5; expression yields 5
  3. a = 5  → a is 5; expression yields 5
```

#### Example C: `*p++` — The Classic C Pointer Confusion

```
Tokens: *  p  ++

Postfix ++ has HIGHER precedence than unary *
Parse: *(p++)   ← NOT (*p)++

          *         ← dereference (lower precedence, outer)
          |
         ++ (postfix)   ← applied to p first (higher precedence, inner)
          |
          p

Semantics:
  1. p++ returns CURRENT value of p (say, addr 0x1000), then increments p
  2. * dereferences that original address
  Net: read value at original p, then advance pointer

To get (*p)++, must write explicitly:
         ++ (postfix)    ← outer
          |
          *              ← inner dereference
          |
          p
```

#### Example D: `a & b == c` — The Dangerous C Trap

```
C: == has HIGHER precedence than &
Parse: a & (b == c)

          &          ← bitwise AND (lower precedence in C)
         / \
        a   ==       ← equality (higher precedence in C)
           / \
          b   c

The famous BUG:
  if (x & 0xFF == 0) { ... }
  → Parsed as: x & (0xFF == 0)
             = x & 0             (0xFF==0 is false=0)
             = 0                 always false! Never enters the if.

CORRECT (need explicit parens in C):
  if ((x & 0xFF) == 0) { ... }

NOTE: Rust and Go FIXED this — in those languages, & has HIGHER precedence than ==
  so x & 0xFF == 0 parses as (x & 0xFF) == 0 — the intuitive result!
```

#### Example E: Ternary Chaining `a ? x : b ? y : z` (right-to-left)

```
Tokens: a  ?  x  :  b  ?  y  :  z

Right-to-left associativity:
  a ? x : (b ? y : z)

         ?:          ← outer ternary
        /  |  \
       a   x   ?:    ← inner ternary (right operand of outer)
              /  |  \
             b   y   z

Evaluation:
  1. Evaluate a
  2. If a is truthy → return x (inner ternary never evaluated)
  3. If a is falsy  → evaluate b
     3a. If b is truthy → return y
     3b. If b is falsy  → return z
```

#### Example F: `a || b && c` (&&  has higher precedence than ||)

```
Tokens: a  ||  b  &&  c

Parse: a || (b && c)

          ||          ← outer (lower precedence)
         /  \
        a    &&       ← inner (higher precedence)
            / \
           b   c

For a=1, b=0, c=1:
  b && c = 0          (false)
  a || 0 = 1          (true — a is truthy, short-circuits)
```

---

## 3. Mental Models

### 3.1 The Binding Strength Model

Visualize each operator as a magnet attracting its operands. Stronger = higher precedence.

```
Expression: 2 + 3 * 4

   2       +       3       *       4
         weak            strong
        (adds)         (multiplies)

  * pulls 3 and 4 together first:    2 + [12]
  + then pulls 2 and [12]:           [14]
```

```
Expression: !a && b || c

  !a  →  (!a) is formed first (unary prefix, highest)
  Then (&& higher than ||):  (!a && b) || c
```

### 3.2 The Parenthesization Algorithm

A systematic procedure for parenthesizing any expression:

```
ALGORITHM:
  1. Find the operator with the HIGHEST precedence (not yet parenthesized)
  2. Apply associativity if there are ties (multiple operators at same level)
  3. Parenthesize that operation
  4. Replace the parenthesized group with a single "token"
  5. Repeat until only one token remains

EXAMPLE: a + b * c - d / e + f

Step 1: * and / are highest (same level): (b*c) and (d/e)
  → a + (b*c) - (d/e) + f

Step 2: +, +, - are next (same level, left-associative): left-to-right grouping
  → ((a + (b*c)) - (d/e)) + f

Step 3: final result:
  → (((a + (b*c)) - (d/e)) + f)
```

### 3.3 The Tree-Building Mental Model

Transform "which operator is the ROOT of the tree?"

- The operator with the **lowest precedence** in an expression is the **root node**
- Its left and right subtrees recursively follow the same rule
- The **highest-precedence** operators are the **deepest leaves**

```
Expression: a = b + c * d - e

Lowest precedence: = → ROOT
  Left of =: a
  Right of =: b + c * d - e
    Lowest in "b + c * d - e": + and - (same level, left-associative)
    Leftmost lowest: +, but wait, it's left-associative so group left first
    Actually: (b + (c*d)) - e
      Lowest is -: ROOT of right subtree
        Left of -: b + (c*d)
          Lowest is +: root of this
            Left: b
            Right: c*d
              Lowest is *: root
                Left: c, Right: d
        Right of -: e
  
            =
           / \
          a   -
             / \
            +   e
           / \
          b   *
             / \
            c   d
```

### 3.4 Mnemonics for C Operator Precedence

A common mnemonic covering C's 15 levels (left to right = high to low precedence):

```
"Please  Unary  Monkeys  Attack  Squirrels   Really   Easily   And   XOR   Or  
  └─1─┘  └─2─┘  └─3─┘  └─4─┘   └─5─┘     └─6─┘   └─7─┘   └─8─┘ └─9─┘ └10─┘

Always   Love   Logical  Ternaries  Assigned  Commas
 └─11─┘ └─12─┘  └─13─┘   └─14─┘    └─15─┘   └─wait that's not right┘"

More practically: remember the pattern from high to low precedence:
  Postfix → Prefix → Mult → Add → Shift → Compare → Equal → &→^→| → &&→|| → ?: → Assign → ,
```

---

## 4. C — Complete Reference

### 4.1 Complete C Operator Precedence Table

C has **15 distinct precedence levels**. Level 1 is the HIGHEST (tightest binding); Level 15 is the LOWEST.

```
╔═══════╦═══════════════════════════════════════════════════╦══════════════════════╦═══════════════════╗
║ Level ║ Operators                                         ║ Name / Category       ║ Associativity     ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   1   ║ ()  []  .  ->  expr++  expr--  (type){list}       ║ Postfix               ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   2   ║ ++expr  --expr  +expr  -expr  !  ~                 ║ Unary prefix          ║ Right → Left      ║
║       ║ (type)  *  &  sizeof  _Alignof                    ║ Cast, deref, addr-of  ║                   ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   3   ║ *   /   %                                         ║ Multiplicative        ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   4   ║ +   -                                             ║ Additive              ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   5   ║ <<   >>                                           ║ Bitwise shift         ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   6   ║ <   <=   >   >=                                   ║ Relational            ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   7   ║ ==   !=                                           ║ Equality              ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   8   ║ &                                                 ║ Bitwise AND           ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║   9   ║ ^                                                 ║ Bitwise XOR           ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  10   ║ |                                                 ║ Bitwise OR            ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  11   ║ &&                                                ║ Logical AND           ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  12   ║ ||                                                ║ Logical OR            ║ Left → Right      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  13   ║ ?:                                                ║ Ternary conditional   ║ Right → Left      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  14   ║ =  +=  -=  *=  /=  %=  <<=  >>=  &=  ^=  |=      ║ Assignment            ║ Right → Left      ║
╠═══════╬═══════════════════════════════════════════════════╬══════════════════════╬═══════════════════╣
║  15   ║ ,                                                 ║ Comma (sequencing)    ║ Left → Right      ║
╚═══════╩═══════════════════════════════════════════════════╩══════════════════════╩═══════════════════╝

CRITICAL SURPRISE: Bitwise & (level 8) is LOWER precedence than == (level 7)!
This means: x & mask == 0  →  x & (mask == 0)  ← almost always wrong!
Always parenthesize: (x & mask) == 0
```

### 4.2 Level 1: Postfix Operators

All postfix operators bind with **left-to-right** associativity and have the **highest precedence** in C.

**Operators:** `()` `[]` `.` `->` `expr++` `expr--` `(type){...}`

#### Function Call: `f(args)`

```c
// Function call is postfix — highest precedence
result = f(x) + g(y);   // Parses as: (f(x)) + (g(y))

// Chaining: factory() returns a function pointer, then call it
typedef int (*fn_t)(int);
fn_t factory(void);
factory()(42);           // (factory())(42): first call factory, then call result
                         // Parses left-to-right: (factory()) then (42)

// Method-style chaining with function pointers:
obj->get_handler()(data);  // ((obj->get_handler)())(data)
// Step 1: obj->get_handler  (member access)
// Step 2: (...)()           (call to get function pointer)
// Step 3: (...)(data)       (call the function pointer with data)
```

#### Array Subscript: `arr[idx]`

```c
// Multi-dimensional arrays — left-to-right subscript application
int matrix[3][4];
matrix[1][2] = 5;   // (matrix[1])[2] = 5
// matrix[1] → pointer to second row (int(*)[4] decays to int*)
// [2]       → element 2 of that row

// Equivalent pointer form (understanding the underlying model):
// matrix[1][2]  ≡  *(*(matrix + 1) + 2)
// Deref chain applied right-to-left, but subscripts are left-to-right

// Pointer arithmetic via subscript:
int arr[10];
int *p = arr + 3;
p[-1]  // arr[2]: negative subscripts are legal with valid addresses
p[0]   // arr[3]
p[1]   // arr[4]
```

#### Member Access: `.` and `->`

```c
struct Point { int x; int y; };
struct Line  { struct Point start; struct Point end; };

struct Line line = {{1,2},{3,4}};
struct Line *lp = &line;

// Direct access (.)  — left-to-right chaining:
int sx = line.start.x;   // (line.start).x — access start member of line, then x

// Pointer access (->)  — (*ptr).member
int ex = lp->end.x;      // (lp->end).x
                          //  = ((lp->end)).x
                          //  = ((*lp).end).x

// Mixing () and ->:
struct Point *get_origin(void);
get_origin()->x;   // (get_origin())->x: call, then access member

// Understanding -> vs *:
// ptr->field  ≡  (*ptr).field  (exactly equivalent, -> just more convenient)
lp->end.y;         // ((lp->end)).y  =  ((*lp).end).y  =  (*lp).end.y
(*lp).end.y;       // same thing with explicit dereference
```

#### Postfix Increment/Decrement: `expr++` / `expr--`

**Rule:** Returns the **current value**, THEN modifies the variable.

```c
int i = 5;
int j = i++;   // j = 5 (old value captured), then i becomes 6
// Internally: {temp = i; i = i+1; return temp;}

int k = i--;   // k = 6 (old value), then i becomes 5

// In array/pointer context:
int arr[5] = {10, 20, 30, 40, 50};
int *p = arr;

int val1 = *p++;
// Postfix ++ is higher precedence than *
// Parse: *(p++)
// 1. p++ returns current p (addr of arr[0]), advances p to arr[1]
// 2. * dereferences original address: arr[0] = 10
// Result: val1=10, p now points to arr[1]

int val2 = *p++;   // val2=20, p now points to arr[2]
int val3 = *p;     // val3=30, p unchanged

// To increment the VALUE at p (not the pointer):
(*p)++;   // Explicit parentheses required!
// Parse: (*p)++ — dereference first, then increment the int value
```

```
ASCII: Comparing *p++ vs (*p)++

*p++                         (*p)++
─────                        ──────
    *    (outer, low prec)        ++   (outer, postfix)
    │                             │
   ++    (inner, high prec)       *    (inner, deref)
    │                             │
    p                             p

*p++ → read arr[i], advance pointer p
(*p)++ → read arr[i], increment the integer value at that address
```

#### Compound Literal: `(type){initializer}` (C99+)

```c
// Compound literals are postfix — can be used anywhere a struct/array is needed
struct Point *p = &(struct Point){.x=3, .y=4};
// Creates a temporary struct, takes its address

// With member access:
(struct Point){.x=3, .y=4}.x   // = 3: access member of compound literal
```

### 4.3 Level 2: Unary/Prefix Operators (Right-to-Left)

**Right-to-left associativity** means when multiple unary prefix operators appear, the rightmost (closest to the operand) applies first.

**Operators:** `++expr` `--expr` `+expr` `-expr` `!expr` `~expr` `(type)expr` `*expr` `&expr` `sizeof` `_Alignof`

```c
int x = 5;

// Right-to-left unary chaining:
int a = -~x;    // -(~x): apply ~ first (closer to x), then -
                // ~5 = -6 (two's complement), -(-6) = 6

int b = !!x;    // !(!x): apply rightmost ! first
                // !5 = 0, !0 = 1
                // !! is the C idiom for "normalize to 0 or 1" (bool conversion)

int c = --++x;  // Unusual: ++(x) first (right-to-left), then --(result)
                // But this is generally bad style

// The "double-not" idiom:
int val = 42;
int is_set = !!val;   // 1 (any non-zero becomes 1)
int is_zero = !val;   // 0 (42 is truthy, !truthy = 0)
```

#### Prefix Increment/Decrement: `++expr` / `--expr`

**Rule:** Modifies the variable FIRST, then returns the **new value**.

```c
int i = 5;
int j = ++i;   // i becomes 6 first, j = 6 (new value)
int k = --i;   // i becomes 5 first, k = 5 (new value)

// Contrast with postfix:
int a = 5, b;
b = a++;  // b=5, a=6  (postfix: return old, then modify)
b = ++a;  // b=7, a=7  (prefix:  modify first, return new)

// In loop: both achieve "loop N times" but differ when value is used:
for (int n=0; n < 10; n++) { }   // postfix: n incremented after body check
for (int n=0; n < 10; ++n) { }   // prefix: same effect here (value not used)

// The difference matters when used as an expression value:
int arr[10];
int idx = 0;
arr[idx++] = 100;   // Stores at arr[0], then idx=1
arr[++idx] = 200;   // idx=2 first, then stores at arr[2]
```

#### Unary Plus and Minus: `+expr` `-expr`

```c
int x = 5;
int neg = -x;   // Arithmetic negation: -5
int pos = +x;   // Unary plus: +5 (same value but triggers integer promotion)

// Integer promotion via unary +:
short s = 255;
// sizeof(+s) == sizeof(int)  ← +s promotes short to int!
// sizeof(s)  == sizeof(short)

// Unary minus on unsigned: defined behavior (wraps)
unsigned int u = 5;
unsigned int neg_u = -u;   // = UINT_MAX - 4  (wraps, well-defined)
```

#### Logical NOT: `!expr`

```c
int x = 42;
int a = !x;       // !42 = 0 (42 is truthy → not truthy = 0)
int b = !0;       // 1 (0 is falsy → not falsy = 1)
int c = !(x > 5); // !(42>5) = !(1) = 0

// Double negation idiom:
int bool_val = !!x;  // Normalize: any non-zero → 1, zero → 0

// In conditions:
if (!error_code) { /* no error */ }
if (!ptr) { /* ptr is NULL */ }
```

#### Bitwise NOT: `~expr`

```c
// Flips every bit in the binary representation
unsigned char x = 0b10110011;  // = 179
unsigned char y = ~x;          // = 0b01001100 = 76

int a = 0;
int b = ~a;   // = -1 (two's complement: all bits set)

// Critical use: creating bit masks
unsigned int mask = ~0u;         // All bits set: 0xFFFFFFFF (unsigned!)
unsigned int low8  = 0xFFu;      // Low 8 bits
unsigned int clear = ~low8;      // High 24 bits set, low 8 cleared: 0xFFFFFF00
unsigned int x32   = someVal & ~(7u << 4);  // Clear bits 4,5,6 of someVal

// Trick: ~x + 1 == -x (two's complement negation)
int negated = ~x + 1;   // = -x (for signed integers)
```

#### Type Cast: `(type)expr`

```c
// Cast has same precedence level as other unary prefix operators (level 2)
// Right-to-left: cast applies to whatever unary expression follows

double d = 3.7;
int i = (int)d;         // Truncation: i = 3

// GOTCHA: Cast vs multiply — cast binds tighter than *
double x = (double)5 / 2;    // = 5.0 / 2 = 2.5  ✓
double y = (double)(5 / 2);  // = (double)2 = 2.0  ✗ (int division first)

// GOTCHA: Cast applies to the immediate operand only
char *p = (char *)0x1000 + 8;
// = ((char *)0x1000) + 8
// = address 0x1000 cast to char*, then advance by 8 bytes (byte arithmetic)
// NOT: (char *)(0x1000 + 8) which would also work here but may differ for expressions

// Common safe cast pattern:
int size = some_long_value;
// If some_long_value might be negative, cast safely:
size_t safe_size = (size > 0) ? (size_t)size : 0;

// Pointer casts:
void *raw = malloc(100);
int  *ip = (int *)raw;      // Cast void* to int* (valid)
char *cp = (char *)ip;      // Cast int* to char* (always valid in C)
// float *fp = (float *)ip; // Cast int* to float* (valid only via memcpy!)
```

#### Dereference: `*expr`

```c
int val = 42;
int *p = &val;
int *q = p;

*p = 100;    // Write through pointer: val is now 100
int x = *p;  // Read through pointer: x = 100

// Multiple levels of indirection:
int **pp = &p;
int ***ppp = &pp;

***ppp;   // *(*(*ppp)) — three dereferences
          // *ppp = pp, *pp = p, *p = val → = 100

// Pointer arithmetic with dereference:
int arr[5] = {10, 20, 30, 40, 50};
int *a = arr;

*(a + 2) == a[2] == 30   // These are EXACTLY equivalent in C

// Common misread:
const char *s = "hello";
char c1 = *s + 1;    // (*s) + 1 = 'h' + 1 = 'i'  (add 1 to char value)
char c2 = *(s + 1);  // *('e' address) = 'e'        (advance pointer, then read)
```

#### Address-Of: `&expr`

```c
int x = 42;
int *p = &x;    // Address of x

// &expr requires an lvalue (something with a memory location):
// &42          ERROR: integer literal has no address
// &(x + 1)    ERROR: result of addition has no address

// Arrays decay to pointers, but & gives the array's own address:
int arr[5];
int *elem_ptr = arr;          // arr decays: points to first element, type int*
int (*arr_ptr)[5] = &arr;     // &arr: address of the array itself, type int(*)[5]
// arr == &arr[0] == (int*)&arr  (same numeric address but different types!)

// Pointer to pointer:
int *pp;
int **ppp = &pp;   // Address of the pointer variable

// Function pointer:
void func(int);
void (*fp)(int) = &func;  // Address of function (& optional in C for functions)
fp(42);  // Call through function pointer
```

#### `sizeof` Operator

`sizeof` is a **compile-time operator** (for non-VLA types), not a function. It does not evaluate its expression operand.

```c
// sizeof with type (parentheses required):
sizeof(int)       // = 4 (typically)
sizeof(double)    // = 8
sizeof(char)      // = 1 (always)
sizeof(void *)    // = 8 on 64-bit

// sizeof with expression (parentheses optional):
int x;
sizeof x          // = sizeof(int) = 4
sizeof(x)         // same

// Array sizeof — total bytes:
int arr[10];
sizeof(arr)               // = 40 (10 * sizeof(int))
sizeof(arr)/sizeof(arr[0]) // = 10 ← portable array element count macro

// CRITICAL GOTCHA: sizeof after array-to-pointer decay
void process(int arr[]) {
    // arr is a POINTER here, not an array!
    sizeof(arr)  // = 8 (pointer size), NOT 40!
    // The array type information is lost when passed to functions
}

// sizeof does NOT evaluate its operand:
int i = 0;
sizeof(i++);    // i is NOT incremented! sizeof skips evaluation
sizeof(arr[i]); // arr not accessed, i unchanged

// Exception: VLA (Variable Length Arrays, C99):
int n = 10;
int vla[n];
sizeof(vla);    // = 40: n IS evaluated (VLA size must be computed at runtime)

// sizeof in expressions:
char buf[sizeof(struct Header) + 256];  // Allocate header + 256 bytes
ptrdiff_t element_offset = (char*)&obj.field - (char*)&obj;  // Field offset
// Or use offsetof macro:
#include 
size_t off = offsetof(struct MyStruct, field);  // Cleaner
```

### 4.4 Level 3: Multiplicative Operators

**Operators:** `*` `/` `%` — Left-to-right

```c
// Integer division: truncates toward zero (C99 and later)
7 / 2    =  3    (truncated, not rounded)
-7 / 2   = -3    (toward zero, not -4)
7 / -2   = -3

// Modulo: sign always matches the DIVIDEND (C99+)
7  %  3  =  1    (7 = 3*2 + 1)
-7 %  3  = -1    (-7 = 3*(-2) + (-1), sign of -7)
7  % -3  =  1    (7 = (-3)*(-2) + 1, sign of 7)
-7 % -3  = -1

// Invariant: (a/b)*b + a%b == a  (when b != 0)

// Left-to-right associativity:
12 / 3 / 2   = (12/3)/2 = 4/2 = 2    (NOT 12/(3/2) = 12/1 = 12!)
6 * 2 / 3    = (6*2)/3  = 12/3 = 4   (NOT 6*(2/3) = 6*0 = 0)

// Integer promotions apply: char/short → int before arithmetic
short a = 1000, b = 1000;
int product = a * b;  // a and b promoted to int, then multiply: 1,000,000
short overflow = a * b; // Warning: product may overflow short (32767 max)

// Floating-point:
10.0 / 3.0   // = 3.333...
10 / 3.0     // 10 promoted to 10.0, result: 3.333...
10.0 / 3     // 3 promoted to 3.0, result: 3.333...
10 / 3       // INTEGER division: 3 (NOT 3.333)

// Overflow (UNDEFINED BEHAVIOR for signed):
INT_MAX * 2   // UB for signed! Use unsigned if you need wrap:
UINT_MAX * 2u // = UINT_MAX-1 (unsigned wraps, well-defined)
```

### 4.5 Level 4: Additive Operators

**Operators:** `+` `-` — Left-to-right

```c
// Pointer arithmetic (valid ONLY within the same array/object):
int arr[10];
int *p = arr;     // p → arr[0]
int *q = p + 3;   // q → arr[3]  (advances by 3 * sizeof(int) bytes)

*(p + 2)   // = arr[2] (equivalent to arr[2])
q - p      // = 3 (ptrdiff_t: element count, not bytes)
p - q      // = -3

// Pointer + integer: advance by N elements of pointed-to type
// int *:   +1 advances by 4 bytes (sizeof int)
// char *:  +1 advances by 1 byte
// double *: +1 advances by 8 bytes

// String traversal with pointer arithmetic:
const char *str = "hello";
while (*str != '\0') {
    process(*str);
    str++;   // str = str + 1: advance by 1 char
}

// Signed integer overflow: UNDEFINED BEHAVIOR
int max = INT_MAX;
int overflow = max + 1;   // UB — compiler may assume this never happens!
// Use unsigned for wrapping arithmetic:
unsigned int u = UINT_MAX;
unsigned int wrap = u + 1;  // = 0, well-defined (modular arithmetic)

// Subtraction with pointers to different objects: UB
int a, b;
ptrdiff_t diff = &a - &b;   // UB: not in the same array
```

### 4.6 Level 5: Shift Operators

**Operators:** `<<` `>>` — Left-to-right

```c
// Left shift: x << n = x * 2^n  (for non-negative x with no overflow)
1 << 0   // = 1
1 << 1   // = 2
1 << 4   // = 16
5 << 3   // = 40  (5 * 8)

// Right shift:
// Unsigned: logical shift (zeros shift in from left)
// Signed: implementation-defined (usually arithmetic shift = sign extension)
unsigned int u = 0xFF00;
u >> 8      // = 0x00FF (zeros shift in)

int neg = -8;   // Likely 0b...11111000
neg >> 1        // = -4 on most platforms (arithmetic shift: sign bit propagates)
                // But officially: implementation-defined for negative values!

// UNDEFINED BEHAVIOR with shifts:
1 << -1     // UB: negative shift amount
1 << 32     // UB: shift amount >= bit width (int is 32 bits)
-1 << 1     // UB: left shift of negative signed value (C standard)
1u << 31    // DEFINED: unsigned, within range

// Best practice: use unsigned types for bit manipulation
#define BIT(n)          (1u << (n))
#define SET_BIT(x,n)    ((x) |  BIT(n))
#define CLEAR_BIT(x,n)  ((x) & ~BIT(n))
#define TOGGLE_BIT(x,n) ((x) ^  BIT(n))
#define CHECK_BIT(x,n)  (((x) >> (n)) & 1u)

// Left-to-right associativity:
1 << 2 << 3  = (1<<2) << 3 = 4 << 3 = 32
// (NOT 1 << (2<<3) = 1 << 8 = 256)
```

### 4.7 Level 6: Relational Operators

**Operators:** `<` `<=` `>` `>=` — Left-to-right (but AVOID chaining!)

```c
// Result is 0 or 1 (int in C, not _Bool):
int a = 5, b = 3, c = 7;

// Valid uses:
int gt = a > b;    // = 1 (true)
int lt = a < b;    // = 0 (false)

// LEGAL IN C BUT USUALLY A BUG — chaining via left-associativity:
int x = a > b > c;    // (a > b) > c = 1 > 7 = 0  (not "5 > 3 > 7"!)
int y = 1 < 2 < 3;    // (1 < 2) < 3 = 1 < 3 = 1  (always true — meaningless test)
int z = 5 < 4 < 3;    // (5 < 4) < 3 = 0 < 3 = 1  (true even though 5 < 4 is false!)

// What you actually want (range check):
int in_range = (a > 0) && (a < 100);   // CORRECT range check
int sorted   = (a < b) && (b < c);     // CORRECT chain comparison

// Pointer comparison (within same array or one past end):
int arr[5];
int *p = arr;
int *q = arr + 3;
if (p < q) { /* p is before q in the array */ }   // DEFINED

// Comparing pointers from different objects: UB
int x1, y1;
if (&x1 < &y1) { }  // UB: different objects, not in same array

// Strings: compare bytes, not value
const char *s1 = "abc";
const char *s2 = "abc";
if (s1 == s2) { }   // WRONG: compares pointer addresses, not content!
if (strcmp(s1, s2) == 0) { }  // CORRECT: use strcmp
```

### 4.8 Level 7: Equality Operators

**Operators:** `==` `!=` — Left-to-right

```c
// = (assignment) vs == (equality) — most common C bug:
int x = 5;
if (x = 3) { }    // BUG: assigns 3 to x, then tests 3 (truthy) → always runs
if (x == 3) { }   // CORRECT: tests equality

// Yoda conditions (constant on left) to catch accidental assignment:
if (3 == x) { }   // Safe: "3 = x" would be a compile error

// Equality with pointers:
int *p = malloc(4);
int *q = p;
if (p == q) { /* same address */ }
if (p != NULL) { /* p is not null */ }

// Floating-point equality: almost always wrong
double a = 0.1 + 0.2;    // = 0.30000000000000004 (binary representation)
if (a == 0.3) { }        // FALSE! Floating-point rounding
// Use epsilon comparison:
#define EPSILON 1e-9
if (fabs(a - 0.3) < EPSILON) { }  // Safer

// char vs EOF — CRITICAL BUG:
char c = getchar();
if (c == EOF) { }  // BUG if char is unsigned (can never be -1/EOF)
// EOF is -1; unsigned char can hold 0-255, never -1
int ic = getchar();       // Read as int!
if (ic == EOF) { }        // CORRECT

// String equality: use strcmp, NOT ==
const char *s = "hello";
if (s == "hello") { }       // WRONG: pointer comparison (may or may not work)
if (strcmp(s, "hello") == 0) { }  // CORRECT: content comparison
```

### 4.9 Levels 8, 9, 10: Bitwise AND, XOR, OR

**THE HISTORICAL MISTAKE:** In C, bitwise `&`, `^`, `|` are **lower** precedence than `==` and `!=`.  
This is widely considered one of C's design flaws (the operators existed before `&&` and `||`).

**Rust and Go fixed this:** in those languages, `&`, `^`, `|` are **higher** precedence than `==`.

```
C Hierarchy (counterintuitive):
  ... > == > != > & > ^ > | > && > || > ...
  (levels 7, 8, 9, 10 from high to low)

Rust/Go Hierarchy (intuitive):
  ... > & > ^ > | > == > != > && > || > ...
```

#### Level 8: `&` — Bitwise AND

```c
// COMMON BUG in C:
int x = 0x12;
if (x & 0x0F == 0) {     // Parses as: x & (0x0F == 0) = x & 0 = 0 → ALWAYS false!
    printf("never runs\n");
}
// CORRECT — always parenthesize bitwise in C:
if ((x & 0x0F) == 0) { printf("lower nibble is zero\n"); }

// Masking — extract specific bits:
unsigned char byte = 0b10110101;   // 181
unsigned char lo4  = byte & 0x0F;  // 0b00000101 = 5  (low nibble)
unsigned char hi4  = (byte >> 4) & 0x0F;  // 0b00001011 = 11 (high nibble)

// Testing if a number is even (bit 0 is the parity bit):
int n = 42;
if ((n & 1) == 0) { printf("%d is even\n", n); }

// Alignment check — is address aligned to N bytes (N must be power of 2):
void *ptr = some_address();
int aligned = ((uintptr_t)ptr & (sizeof(int)-1)) == 0;  // Aligned to int?
// If sizeof(int)==4: mask 0x3, check all 2 low bits are zero

// Clear specific bits:
unsigned flags = 0xFF;
flags &= ~(1u << 3);   // Clear bit 3
```

#### Level 9: `^` — Bitwise XOR

```c
// XOR properties:
// x ^ 0 = x     (XOR with 0 is identity)
// x ^ x = 0     (XOR with self is zero)
// x ^ y = y ^ x (commutative)
// x ^ y ^ y = x (XOR twice cancels: useful for encryption)

// Toggle specific bits:
unsigned flags = 0b10110010;
flags ^= (1u << 1);  // Toggle bit 1: 0b10110000

// XOR swap (classic interview trick, but avoid in practice):
int a = 5, b = 3;
a ^= b;   // a = 5^3 = 6  (a now holds a^b)
b ^= a;   // b = 3^6 = 5  (b = b^(a^b) = a)
a ^= b;   // a = 6^5 = 3  (a = (a^b)^a = b)
// Now a=3, b=5 (swapped without temp variable)
// WARNING: UNDEFINED BEHAVIOR if a and b alias the same variable!
// if (&a == &b): a ^= b makes a=0, then everything stays 0

// Simple encryption/obfuscation (NOT cryptographically secure):
const char key = 0x5A;
char buf[] = "secret";
for (int i = 0; buf[i]; i++) buf[i] ^= key;  // Encrypt
for (int i = 0; buf[i]; i++) buf[i] ^= key;  // Decrypt (XOR twice = identity)
```

#### Level 10: `|` — Bitwise OR

```c
// Set specific bits:
unsigned flags = 0;
flags |= (1u << 0);  // Set bit 0: READ flag
flags |= (1u << 1);  // Set bit 1: WRITE flag
flags |= (1u << 2);  // Set bit 2: EXEC flag

// Combine flags (canonical use of |):
int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
// The | combines flag values: O_WRONLY=1, O_CREAT=64, O_TRUNC=512
// 1 | 64 | 512 = 577

// ANOTHER C BUG (same as & bug):
if (flags | FLAG_VALUE == FLAG_VALUE) {  // Parses as: flags | (FLAG_VALUE==FLAG_VALUE)
    // = flags | 1  which tests flags, not "is FLAG_VALUE set"!
}
// CORRECT:
if ((flags | FLAG_VALUE) == FLAG_VALUE) { }
// Or better: test with &
if (flags & FLAG_VALUE) { }  // Check if the specific bit is set
```

#### Hierarchy of `&`, `^`, `|` (same within C, Rust, Go)

```
Within bitwise operators, all three languages agree:
  & (AND) > ^ (XOR) > | (OR)

Expression: a & b ^ c | d
Parse: ((a & b) ^ c) | d

              |          ← lowest (level 10 in C)
             / \
            ^   d
           / \
          &   c
         / \
        a   b           ← highest (level 8 in C)
```

### 4.10 Levels 11 & 12: Logical AND and OR

**`&&` (Logical AND, Level 11)** — Left-to-right, short-circuit  
**`||` (Logical OR, Level 12)** — Left-to-right, short-circuit

These are critical because they:
1. Have **defined left-to-right evaluation order** (sequence point between operands)
2. **Short-circuit**: may skip evaluating the right operand
3. The operands are implicitly converted to boolean (0 = false, non-zero = true)
4. Return **int** (0 or 1), not the actual operand values (unlike Rust's `&&`/`||`)

```c
// Short-circuit saves from null dereference:
Node *node = find_node(key);
if (node != NULL && node->value > 0) {
    // Safe: if node is NULL, node->value is NEVER accessed
    use(node->value);
}

// Short-circuit for lazy evaluation (only call if needed):
bool is_valid_and_ready(Item *item) {
    return item != NULL
        && item->initialized
        && item->check() != NULL;  // check() only called if initialized
}

// Short-circuit with side effects:
int counter = 0;
int inc(void) { return ++counter; }

int r1 = 0 && inc();   // inc() NOT called: left is false, whole && is false
int r2 = 1 || inc();   // inc() NOT called: left is true, whole || is true
int r3 = 1 && inc();   // inc() IS called: left is true, need to evaluate right
int r4 = 0 || inc();   // inc() IS called: left is false, need right to decide

// && has HIGHER precedence than ||:
// a || b && c  →  a || (b && c)
int x = 0 || 1 && 0;   // 0 || (1 && 0) = 0 || 0 = 0
int y = 1 || 0 && 0;   // 1 || (0 && 0) = 1 || 0 = 1 (short-circuits!)

// Practical: boolean flag chains
int is_valid = check_range(val)
            && check_type(val)
            && check_consistency(val);
// Stops at first false (efficient)

// RETURN VALUE: always 0 or 1
int a_or_b = 5 || 0;   // = 1  (not 5! unlike Python's "or")
// In Python: 5 or 0 = 5. In C: 5 || 0 = 1. DIFFERENT!
```

### 4.11 Level 13: Conditional (Ternary) Operator `?:`

**Right-to-left** associativity. Only ternary operator in C.

```
condition ? true_expr : false_expr

- Evaluate condition
- If non-zero (true): evaluate and return true_expr; false_expr is NOT evaluated
- If zero (false): evaluate and return false_expr; true_expr is NOT evaluated
- Like short-circuit: only one branch executes
```

```c
// Basic usage — expression form of if/else:
int x = (a > b) ? a : b;         // max of a and b
int abs_val = (x >= 0) ? x : -x; // absolute value

// Ternary IS an expression (unlike if/else), usable inline:
printf("Value is %s\n", (x > 0) ? "positive" : "non-positive");
int result = some_flag ? compute_a() : compute_b();  // Only one function called!

// Right-to-left chaining (reads naturally when formatted):
const char *label =
    n < 0   ? "negative"
  : n == 0  ? "zero"
  : n < 10  ? "small"
  : n < 100 ? "medium"
  :           "large";
// Parsed as: n<0 ? "negative" : (n==0 ? "zero" : (n<10 ? "small" : (...)))
```

ASCII tree for `a ? x : b ? y : z`:
```
          ?:              ← outer (right-to-left means inner is the false branch)
         /  |  \
        a   x   ?:        ← inner (nested in false branch of outer)
               /  |  \
              b   y   z
```

```c
// Ternary with assignment (valid, but controversial style):
int val;
flag ? (val = 10) : (val = 20);  // One of the assignments happens

// GOTCHA — ternary is an expression, not a statement:
// flag ? x++ : y++;    ← valid (increments only one)
// flag ? f() : ;       ← INVALID syntax (false branch must be an expression)

// Ternary vs if for lvalues (C does NOT allow ternary as lvalue):
// (flag ? a : b) = 5;  ← INVALID in C
// In C++, this CAN be valid under specific conditions (not relevant here)

// Macros using ternary:
#define MAX(a, b) ((a) > (b) ? (a) : (b))
#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define CLAMP(x, lo, hi) ((x) < (lo) ? (lo) : ((x) > (hi) ? (hi) : (x)))

// Type of ternary result: usual arithmetic conversions apply to true/false branches
int a = 1;
double b = 2.0;
typeof(a > 0 ? a : b)  // double: int is converted to double (UAC)
```

### 4.12 Level 14: Assignment Operators (Right-to-Left)

**Operators:** `=` `+=` `-=` `*=` `/=` `%=` `<<=` `>>=` `&=` `^=` `|=`

Assignment in C is an **expression** (has a value), unlike Go where it is a statement.

```c
// Assignment is an expression whose value is the stored value:
int a, b, c;
a = b = c = 0;    // Right-to-left: c=0 (yields 0), b=0 (yields 0), a=0
// Same as: a = (b = (c = 0))

// Using assignment expression value:
int len;
while ((len = strlen(str)) > 0) {  // Assigns AND tests in one expression
    process(str, len);
    // Advance str...
}

// Common pattern: reading input in loops
char buf[256];
while (fgets(buf, sizeof(buf), stdin) != NULL) {
    process(buf);
}

// Compound assignments:
int n = 10;
n += 5;    // n = n + 5 = 15
n -= 3;    // n = n - 3 = 12
n *= 2;    // n = n * 2 = 24
n /= 4;    // n = n / 4 = 6
n %= 4;    // n = n % 4 = 2

n <<= 2;   // n = n << 2 = 8   (multiply by 4)
n >>= 1;   // n = n >> 1 = 4   (divide by 2, implementation-defined for negatives)
n &= 0x0F; // n = n & 0x0F     (mask to low nibble)
n ^= 0xFF; // n = n ^ 0xFF     (flip low 8 bits)
n |= 0x80; // n = n | 0x80     (set bit 7)

// Right-to-left with compound assignment:
int x = 5;
x += x += 2;  // x += (x += 2) = x += 7 → but this has undefined behavior!
              // x appears twice as a modified lvalue without sequence point
// AVOID: modifying the same variable twice in one expression
```

### 4.13 Level 15: Comma Operator (Lowest Precedence)

**Operator:** `,` — Left-to-right, evaluates left, discards result, returns right.

```c
// The comma OPERATOR (vs comma as separator in calls/declarations):
int x = (1, 2, 3);   // x = 3: evaluates 1 (discarded), 2 (discarded), returns 3

// ONLY valid use: for loops with multiple update expressions:
for (int i = 0, j = 10; i < j; i++, j--) {
    // i++, j-- uses comma operator to update two variables per iteration
}

// The commas in "int i=0, j=10" and in "f(a, b)" are NOT comma operators!
// - Declaration: "int i=0, j=10" — comma separates declarations
// - Function call: "f(a, b)" — comma separates arguments (unspecified eval order)
// Only (expr, expr) is the comma operator

// GOTCHA — assignment has higher precedence than comma!
int y;
y = 1, 2;   // Parses as (y = 1), 2 — y gets 1, not 2!
y = (1, 2); // y gets 2 — explicit comma operator with parens

// Using comma for side effects:
int a = 0, b = 0;
int c = (a++, b++, a + b);  // a becomes 1, b becomes 1, c = 1+1 = 2
// Sequence: a++ → b++ → a+b (each step is sequenced by commas)

// Rare legitimate use: assertion-like macros
#define ASSERT_AND_RETURN(cond, val) ((assert(cond)), (val))
```

### 4.14 Sequence Points and Undefined Behavior

Understanding sequence points is essential for writing correct C code. A **sequence point** is a point where all previous side effects are complete and none from subsequent operations have started.

**Sequence points in C:**
```
1. End of a full expression (semicolon ;)
2. After evaluating the left operand of && and ||
3. After evaluating the condition of ?: (the ? part)
4. After each comma in the comma OPERATOR (NOT in function args)
5. At the entry of a function call (all args fully evaluated)
6. At the return from a function call
7. After each declarator's initializer in a list
```

**Undefined Behavior:** Modifying an object more than once between sequence points, OR reading it for a value other than determining which value to write.

```c
// UNDEFINED BEHAVIOR examples:
int i = 0;

i = i++;         // UB: i is both written (by =) and incremented (by ++) with no sequence point
i++ + i++;       // UB: two modifications to i in the same expression
a[i] = i++;      // UB: read i (index) and write i (increment) — order undefined
printf("%d %d", i++, i++);  // UB: function argument evaluation order unspecified
                              //      AND two modifications to i

// DEFINED — a sequence point separates the modifications:
i++;
i = i + 1;      // Only one modification to i per statement
x = a[0];       // Only reads i, doesn't modify

// DEFINED — comma operator provides sequence point:
i = (i++, i);   // i++ happens first (sequence point), then i is read → defined

// DEFINED — short-circuit provides sequence point:
i = 0;
(i++ > 0) && (i++ > 0);
// Left of && is evaluated, sequenced, THEN right (if needed)
// i=0: i++ returns 0 (i becomes 1), 0>0 is false → short-circuit!
// Right side NOT evaluated: i stays 1
// Result: i == 1

// GOTCHA — function arg evaluation order (unspecified, not UB if no shared state):
int f(int x) { return x; }
int g(int x) { return x; }
foo(f(a), g(b));  // Not UB, but order of f(a) vs g(b) is unspecified
foo(i++, i++);    // UB: two modifications to i, no sequence point between args
```

### 4.15 Integer Promotions and Usual Arithmetic Conversions

These implicit conversions interact with operator precedence.

```c
// INTEGER PROMOTION: char and short → int before most arithmetic
char c = 200;    // Often stored as signed: -56
int i = c + 0;  // c promoted to int(-56), then + : result is -56 (not 200+0=200!)

// USUAL ARITHMETIC CONVERSIONS (for binary operators):
// If either operand is double → both double
// Else if either is float → both float
// Else if either is unsigned long long → both unsigned long long
// ... (integer rank rules)
// Else both → int (integer promotion applied to both)

// CRITICAL: mixing signed and unsigned
int    s = -1;
unsigned u = 1u;
if (s < u) { printf("negative < positive\n"); }
// WRONG! -1 is converted to UNSIGNED: -1 → UINT_MAX
// UINT_MAX < 1 is FALSE → this prints nothing!
// FIX: cast to same type explicitly: (int)u > s OR (unsigned)s < u (carefully)

// Promotion affects result type:
short a = 1000, b = 1000;
int c = a * b;      // = 1,000,000 — promoted to int first, no overflow
short d = a * b;    // WARNING: may overflow short if stored in short

// sizeof is not affected by promotions (it's a compile-time size, not arithmetic)
```

### 4.16 Complete C Code Examples

```c
#include 
#include 
#include 
#include 
#include 

// ── Example 1: The Bitwise Precedence Bug ──
void bitwise_bug_demo(void) {
    int x = 0x12;  // 0001 0010

    // BUG: & has LOWER precedence than == in C!
    if (x & 0x0F == 0) {          // x & (0x0F == 0) = x & 0 = 0 → always false
        printf("Bug: will never print\n");
    }

    if ((x & 0x0F) == 0) {        // Correct: test if lower nibble is zero
        printf("Lower nibble is zero\n");  // Not printed (0x12 & 0x0F = 2)
    }

    if ((x & 0xF0) == 0x10) {     // Correct: test upper nibble
        printf("Upper nibble is 1\n");    // Printed!
    }
}

// ── Example 2: Pointer Operator Precedence ──
void pointer_precedence_demo(void) {
    int arr[5] = {10, 20, 30, 40, 50};
    int *p = arr;

    // Various pointer + operator interactions:
    printf("*p      = %d\n", *p);      // 10: dereference p
    printf("*p+1    = %d\n", *p+1);    // 11: (*p)+1 — adds to VALUE
    printf("*(p+1)  = %d\n", *(p+1));  // 20: pointer advance, then deref

    int v1 = *p++;  // *(p++) → reads arr[0]=10, advances p to arr[1]
    printf("v1=%d, *p=%d\n", v1, *p);  // v1=10, *p=20

    int v2 = ++*p;  // ++(*p) → increments value at arr[1]: 20→21, returns 21
    printf("v2=%d, arr[1]=%d\n", v2, arr[1]);  // v2=21, arr[1]=21

    int v3 = (*p)++;  // (*p)++ → reads arr[1]=21, then increments it to 22
    printf("v3=%d, arr[1]=%d\n", v3, arr[1]);  // v3=21, arr[1]=22
}

// ── Example 3: Short-Circuit in Linked List ──
typedef struct Node { int val; struct Node *next; } Node;

int safe_sum(Node *head) {
    int sum = 0;
    for (Node *n = head; n != NULL && n->val > 0; n = n->next) {
        // n != NULL checked first: if NULL, n->val is never accessed
        sum += n->val;
    }
    return sum;
}

// ── Example 4: Ternary Chaining ──
const char *classify_int(int n) {
    return n < 0   ? "negative"
         : n == 0  ? "zero"
         : n < 10  ? "small"
         : n < 100 ? "medium"
         :           "large";
    // Parses as: n<0 ? "neg" : (n==0 ? "zero" : (n<10 ? "small" : ...))
}

// ── Example 5: Bit Manipulation (Hardware Register Access) ──
#define REG_CTRL    ((volatile uint32_t *)0x40000000)
#define CTRL_ENABLE  (1u << 0)
#define CTRL_MODE    (0x3u << 4)   // 2-bit field at bits 4:5
#define CTRL_SPEED   (0x7u << 8)   // 3-bit field at bits 8:10

void config_hardware(void) {
    uint32_t reg = 0;

    // Set enable bit and mode=2, speed=5:
    reg = CTRL_ENABLE | (2u << 4) | (5u << 8);
    // Precedence: << > | so shifts apply first, then ORs combine

    // Read mode field:
    int mode = (reg & CTRL_MODE) >> 4;   // (reg & mask) first, then shift
    // NOT: reg & (CTRL_MODE >> 4) — wrong masking position

    // Modify speed field only:
    reg &= ~CTRL_SPEED;        // Clear the 3-bit speed field
    reg |= (3u << 8);          // Set new speed = 3

    printf("reg=0x%08X, mode=%d\n", reg, mode);
}

// ── Example 6: sizeof and Pointer Gotcha ──
void sizeof_gotcha(void) {
    int arr[10];
    int *ptr = arr;

    size_t sz_arr = sizeof(arr);    // 40: array size
    size_t sz_ptr = sizeof(ptr);    // 8: pointer size (64-bit)
    size_t sz_val = sizeof(*ptr);   // 4: element size
    size_t count  = sizeof(arr)/sizeof(arr[0]);  // 10: element count

    printf("sizeof arr=%zu, ptr=%zu, *ptr=%zu, count=%zu\n",
           sz_arr, sz_ptr, sz_val, count);
}

// ── Example 7: Compound Assignment ──
void bit_field_ops(uint32_t *reg) {
    *reg |= (1u << 3);      // Set bit 3
    *reg &= ~(1u << 5);     // Clear bit 5
    *reg ^= (1u << 7);      // Toggle bit 7
    *reg <<= 1;              // Shift entire register left by 1
    *reg = (*reg & 0xFFFF0000u) | 0x1234u;  // Replace lower 16 bits
}

int main(void) {
    bitwise_bug_demo();
    printf("\n");
    pointer_precedence_demo();
    printf("\n");
    printf("classify(7)=%s, classify(-3)=%s, classify(150)=%s\n",
           classify_int(7), classify_int(-3), classify_int(150));
    printf("\n");
    config_hardware();
    sizeof_gotcha();
    return 0;
}
```

---

## 5. Rust — Complete Reference

Rust's operator precedence is informed by C/C++ but with deliberate improvements:

- **Bitwise `&`, `^`, `|` are HIGHER precedence than `==`** (fixes C's counterintuitive design)
- **Comparison operators are non-associative** (chaining is a compile error)
- **`as` casting** is an explicit, defined operator (no UB like C casts)
- **No ternary `?:`** — use `if-else` expressions instead (everything is an expression in Rust)
- **No comma operator** — multiple things require explicit syntax
- **The `?` error propagation operator** is a postfix operator
- **Range operators `..` and `..=`** are low-precedence operators

### 5.1 Complete Rust Operator Precedence Table

```
╔══════════╦════════════════════════════════════════════════════╦════════════════════════╦══════════════════╗
║ Priority ║ Operators                                          ║ Category                ║ Associativity    ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║ Highest  ║ expr.method(args)   expr.field   expr[idx]         ║ Method call, field, idx ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ expr?                                              ║ Error propagation        ║ (postfix unary)  ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ -expr   !expr   *expr   &expr   &mut expr          ║ Unary prefix             ║ Right → Left     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ expr as Type                                       ║ Type cast                ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ *   /   %                                          ║ Multiplicative           ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ +   -                                              ║ Additive                 ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ <<   >>                                            ║ Bitwise shift            ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ &                                                  ║ Bitwise AND              ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ ^                                                  ║ Bitwise XOR              ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ |                                                  ║ Bitwise OR               ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ ==   !=   <   >   <=   >=                          ║ Comparison (NON-ASSOC)  ║ NON-ASSOCIATIVE  ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ &&                                                 ║ Logical AND (lazy)       ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ ||                                                 ║ Logical OR (lazy)        ║ Left → Right     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ ..   ..=                                           ║ Range operators          ║ NON-ASSOCIATIVE  ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║          ║ =  +=  -=  *=  /=  %=  &=  |=  ^=  <<=  >>=      ║ Assignment / compound    ║ Right → Left     ║
╠══════════╬════════════════════════════════════════════════════╬════════════════════════╬══════════════════╣
║  Lowest  ║ return   break   continue   |args| body (closure)  ║ Control flow / closures ║ —                ║
╚══════════╩════════════════════════════════════════════════════╩════════════════════════╩══════════════════╝

KEY DIFFERENCE FROM C: Bitwise & ^ | are ABOVE == != (opposite of C!)
x & mask == 0  →  (x & mask) == 0  in Rust  ← mathematically intuitive!
x & mask == 0  →  x & (mask == 0)  in C     ← counterintuitive, bug-prone!
```

### 5.2 Method Calls, Field Access, and Indexing

All are postfix with highest precedence and left-to-right associativity.

```rust
struct Point { x: f64, y: f64 }

impl Point {
    fn distance(&self) -> f64 { (self.x*self.x + self.y*self.y).sqrt() }
    fn scale(&self, factor: f64) -> Point { Point { x: self.x*factor, y: self.y*factor } }
}

let p = Point { x: 3.0, y: 4.0 };

// Method chaining — left-to-right:
p.scale(2.0).distance();
// (p.scale(2.0)).distance()
// Step 1: p.scale(2.0) → Point{x:6.0, y:8.0}
// Step 2: .distance()  → 10.0

// Field access and method together:
p.x.abs();   // (p.x).abs() — access field x (f64), then call abs() method

// Indexing:
let v = vec![1, 2, 3, 4, 5];
v[2];          // Third element: 3
v[2..4];       // Slice: [3, 4] (note: range has lower precedence!)

// Method chains with auto-deref:
// Rust automatically dereferences through smart pointers when calling methods:
let boxed = Box::new(Point { x: 1.0, y: 2.0 });
boxed.distance();   // Rust auto-derefs Box to Point, calls .distance()
// Equivalent to: (*boxed).distance()

// Complex chaining in real code:
fn process(data: &str) -> String {
    data.trim()         // &str → &str
        .to_lowercase() // &str → String
        .replace("_", " ")  // String → String
        .split_whitespace()  // → SplitWhitespace iterator
        .collect::<Vec>() // → Vec
        .join(", ")          // → String
}
```

### 5.3 The `?` Error Propagation Operator

`?` is a **postfix unary operator** that short-circuits on errors. It has very high precedence (just below method calls).

```rust
use std::fs;
use std::io;
use std::num::ParseIntError;

// What ? does (desugared):
// expr?  ≡  match expr {
//               Ok(val)  => val,
//               Err(e)   => return Err(e.into()),
//           }

// Without ?:
fn read_number_verbose(path: &str) -> Result> {
    let content = match fs::read_to_string(path) {
        Ok(s) => s,
        Err(e) => return Err(Box::new(e)),
    };
    let trimmed = content.trim();
    let number = match trimmed.parse::() {
        Ok(n) => n,
        Err(e) => return Err(Box::new(e)),
    };
    Ok(number)
}

// With ? (clean chaining):
fn read_number(path: &str) -> Result> {
    let number = fs::read_to_string(path)? // if Err → return early with Err
                     .trim()              // .trim() called on the Ok String
                     .parse::()?;   // if Err → return early with Err
    Ok(number)
}

// Precedence of ? relative to method calls:
// fs::read_to_string(path)?.trim()  is:
// (fs::read_to_string(path)?).trim()
// Step 1: ? extracts the Ok value (or returns early)
// Step 2: .trim() is called on the extracted String
// NOT: fs::read_to_string(path)?.trim() could not be "?"(read.trim())
//      because ? is postfix — applies immediately to its left expression

// Chaining multiple ?:
fn complex_operation() -> Result> {
    let a = "42".parse::()?;    // ? after parse
    let b = "100".parse::()?;   // ? after parse
    Ok((a + b) as u64)
}
```

### 5.4 Unary Operators in Rust

**Right-to-left associativity.**

```rust
let x: i32 = 5;

// Negation:
let neg = -x;    // -5
let nn  = -(-x); // 5 — right-to-left: innermost first

// Logical NOT:
let t = true;
let f = !t;          // false
let tt = !!t;        // !(!true) = !false = true

// For integers, ! is BITWISE NOT (unlike C's ~ for integers):
let n: u8 = 0b10110011;  // 179
let flipped = !n;         // 0b01001100 = 76  ← ! is bitwise NOT for integers in Rust!
// In C you'd use ~n, but Rust uses ! for both logical NOT (bool) and bitwise NOT (int)

// Dereference:
let val = 42;
let r = &val;    // r is &i32
let v = *r;      // v = 42: explicit dereference

// Rust auto-derefs in many contexts (method calls, assignments):
let mut v_ref = &mut vec![1, 2, 3];
v_ref.push(4);   // Auto-derefs: (*v_ref).push(4)
*v_ref = vec![]; // Explicit dereference needed for full replacement

// Borrow operators:
let a = String::from("hello");
let r1: &String = &a;         // Shared (immutable) borrow
// let r2: &mut String = &mut a; // Mutable borrow (requires mut)

// Chaining unary (right-to-left):
let x: i32 = -5;
let y = -!x;    // -(!(x))... wait, !i32 is bitwise NOT
// !(-5) = bitwise NOT of -5 = 4 (for i32: ~(-5) = ~(0b...11111011) = 0b...00000100 = 4)
// -4
```

### 5.5 The `as` Casting Operator

`as` is a left-associative binary operator for type casting. It has higher precedence than arithmetic but lower than method calls.

```rust
// Basic casts:
let x: f64 = 3.7;
let i = x as i32;     // Truncation: i = 3

let big: i64 = 1000;
let small = big as i8;  // Wraps: 1000 % 256 = 232, then interpreted as i8 = -24

// as is left-associative:
let v: u64 = 1000;
let result = v as u16 as i16;
// ((v as u16) as i16): first truncate to u16 (1000), then reinterpret as i16 (1000)

// as vs explicit conversion functions:
let n: u32 = 300;
// n as u8   → 44 (wraps: 300 % 256 = 44)
// u8::try_from(n) → Err (300 doesn't fit in u8, safe conversion)
// Use try_from/into for safe conversions in production code

// Precedence of as — binds tighter than arithmetic:
let x: f32 = 1.5;
let y: f32 = 2.5;
let sum = x as i32 + y as i32;  // (x as i32) + (y as i32) = 1 + 2 = 3
// NOT: x as (i32 + y) as i32 — nonsensical, not how it parses

// as with pointers:
let p: *const i32 = &42 as *const i32;
let u = p as usize;  // Pointer as integer (unsafe context may be needed)

// Floating-point to integer via as:
f64::NAN as i32        // = 0 (implementation-defined until Rust 1.45, now saturating)
f64::INFINITY as i32   // = i32::MAX (saturating cast since Rust 1.45)
-f64::INFINITY as i32  // = i32::MIN
1.9f64 as u8           // = 1 (truncation toward zero)
-1.0f64 as u8          // = 0 (saturating: negative → 0 for unsigned)
256.0f64 as u8         // = 255 (saturating: overflow → max for unsigned)
```

### 5.6 Bitwise Operators — Fixed Relative to C

In Rust, bitwise `&`, `^`, `|` are **higher precedence** than comparison operators. This fixes C's counterintuitive design.

```rust
// In Rust: x & mask == 0  →  (x & mask) == 0  ✓ INTUITIVE
// In C:    x & mask == 0  →  x & (mask == 0)  ✗ COUNTERINTUITIVE

let x: u8 = 0x12;
let result = x & 0x0F == 0;     // (x & 0x0F) == 0 = 2 == 0 = false ← correct parse
let result2 = x & 0xF0 == 0x10; // (x & 0xF0) == 0x10 = 0x10 == 0x10 = true ← correct

// Bit operations — same semantics as C, but ! is bitwise NOT:
let flags: u32 = 0;
let flags = flags | (1 << 0);  // Set bit 0
let flags = flags | (1 << 1);  // Set bit 1
let is_set = (flags & (1 << 1)) != 0;  // Test bit 1
let flags = flags & !(1 << 0); // Clear bit 0: note ! for complement in Rust

// Precedence among & ^ |: & > ^ > | (same as C):
let a: u32 = 0xFF;
let b: u32 = 0x0F;
let c: u32 = 0xF0;
let r = a & b ^ c;   // (a & b) ^ c = 0x0F ^ 0xF0 = 0xFF
let r2 = a ^ b | c;  // (a ^ b) | c = 0xF0 | 0xF0 = 0xF0

// For integers, ! is bitwise NOT (Rust uses ! for both logical and bitwise NOT):
let n: u8 = 0b10110011;
let inv = !n;  // 0b01001100 = 76
// In C this would be ~n
```

### 5.7 Non-Associative Comparison Operators

Rust makes comparison operators **non-associative** to eliminate the silent bug class that plagues C.

```rust
// COMPILE ERROR: cannot chain comparison operators in Rust
// let x = 1 < 2 < 3;    // Error: non-associative: use parens or &&

// CORRECT: use && to chain comparisons
let in_range = 1 < 5 && 5 < 10;   // true
let sorted   = a < b && b < c;    // proper range check

// Non-associativity also prevents this confusing expression:
// let x = 1 == 1 == true;  // Error! Can't chain == either
// Instead:
let x = (1 == 1) == true;  // OK: explicit parens, true == true

// The error message:
// error[E0308]: chained comparison operators require parentheses
// 1 < 2 < 3
//     ^ --- this is a non-associative operator

// In practice, Rust forces you to write clear, unambiguous comparisons:
fn in_range(n: i32, lo: i32, hi: i32) -> bool {
    n >= lo && n <= hi   // Clear, explicit, unambiguous
}
```

### 5.8 Logical AND/OR — Short Circuit and Expression Values

```rust
// Rust's && and || also short-circuit, same as C
// But unlike C, they work on bool types (not integers)
// And the whole expression is bool, not int

let a = true;
let b = false;
let c = true;

let r1 = a && b;   // false (b evaluated: a is true, need b)
let r2 = b && c;   // false (c NOT evaluated: b is false, short-circuit)
let r3 = a || b;   // true  (b NOT evaluated: a is true, short-circuit)
let r4 = b || c;   // true  (c evaluated: b is false, need c)

// Short-circuit with side effects:
fn side_effect() -> bool {
    println!("evaluated!");
    true
}
let _ = false && side_effect();  // "evaluated!" NOT printed
let _ = true  || side_effect();  // "evaluated!" NOT printed
let _ = true  && side_effect();  // "evaluated!" IS printed
let _ = false || side_effect();  // "evaluated!" IS printed

// Real use case: safe access pattern
fn find_item(items: &[i32], target: i32) -> bool {
    !items.is_empty() && items.contains(&target)
    // If empty, contains() not called
}

// Unlike C, && and || require bool operands:
// 1 && 2    // COMPILE ERROR: expected bool, found integer
// if x { } // COMPILE ERROR if x is not bool (Rust doesn't auto-convert to bool)
```

### 5.9 Range Operators `..` and `..=`

Range operators are unique to Rust among the three languages. They are **non-associative** and have **lower precedence than arithmetic and bitwise operators** but **higher than assignment**.

```rust
// Exclusive range (excludes end):
let r1 = 1..5;   // Represents 1, 2, 3, 4 (NOT 5)

// Inclusive range (includes end):
let r2 = 1..=5;  // Represents 1, 2, 3, 4, 5

// Using in for loops:
for i in 0..10 { print!("{} ", i); }   // 0 1 2 3 4 5 6 7 8 9
for i in 0..=10 { print!("{} ", i); }  // 0 1 2 3 4 5 6 7 8 9 10

// Range types:
// 1..5   → std::ops::Range
// 1..=5  → std::ops::RangeInclusive
// ..     → std::ops::RangeFull
// 1..    → std::ops::RangeFrom
// ..5    → std::ops::RangeTo
// ..=5   → std::ops::RangeToInclusive

// Precedence: range is LOWER than arithmetic:
let r = 1 + 2..5 * 2;
// = (1+2)..(5*2) = 3..10  ← arithmetic applies first!

// Slicing with ranges (very common):
let arr = [10, 20, 30, 40, 50];
let slice = &arr[1..4];   // [20, 30, 40] — positions 1, 2, 3
let all   = &arr[..];     // All elements
let first3 = &arr[..3];   // [10, 20, 30]
let last2  = &arr[3..];   // [40, 50]

// Pattern matching with ranges:
let ch = 'g';
match ch {
    'a'..='z' => println!("lowercase"),
    'A'..='Z' => println!("uppercase"),
    '0'..='9' => println!("digit"),
    _          => println!("other"),
}
// Note: in patterns, only ..= (inclusive) is allowed; .. is not valid in patterns
```

### 5.10 Assignment in Rust — Statement, Not Expression

Unlike C, assignment in Rust is a **statement**, not an expression. It returns `()` (unit), not the assigned value.

```rust
let mut x = 0;

// Assignment is a statement, returns ():
let _ = (x = 5);   // () — not 5

// CONSEQUENCE: cannot do C's pattern of:
// while ((n = getchar()) != EOF) { }
// Instead, Rust uses while-let or other patterns:

let mut n;
loop {
    n = get_value();
    if n == SENTINEL { break; }
    process(n);
}

// Or:
while let Some(item) = iterator.next() {
    process(item);
}

// Chained assignment is not valid in Rust:
// a = b = 5;  // ERROR: b = 5 returns (), not 5; a = () is a type error (unless a is ())

// Compound assignment operators DO exist:
let mut n = 10;
n += 5;   // 15
n -= 3;   // 12
n *= 2;   // 24
n /= 4;   // 6
n %= 5;   // 1
n <<= 3;  // 8
n >>= 1;  // 4
n &= 0x0F; // 4
n ^= 0xFF; // 251 (if u8) or large negative (if i32)
n |= 0x80; // Set high bit
```

### 5.11 `return`, `break`, `continue` — Lowest Precedence

In Rust, `return`, `break`, and `continue` are expressions with very low precedence (lower than assignment). This means they can be used in surprising places.

```rust
// return as an expression:
let x = return 5;  // Expression: return early with 5; x is never bound

// break with value (from loop{}):
let result = loop {
    let val = compute();
    if val > 100 {
        break val;  // Break out of loop and yield 'val' as the result
    }
};
// result = val from break

// Their very low precedence means:
fn example(x: i32) -> i32 {
    // return has lower precedence than all operators:
    return x * 2 + 1;
    // Parses as: return (x * 2 + 1)
    // NOT: (return x) * 2 + 1 — that doesn't even make sense syntactically

    // But with assignment:
    let y = return x;
    // Assignment > return, so: let y = (return x)
    // return executes first (never reaches let completion)
}

// Closure expressions also have very low precedence:
let add = |x: i32, y: i32| x + y;
// The body "x + y" extends as far right as possible
// Not: (|x, y| x) + y
```

### 5.12 Operator Overloading in Rust via Traits

Rust allows overloading operators via `std::ops` traits. Precedence is fixed by the language; only behavior is overloaded.

```rust
use std::ops::{Add, Sub, Mul, Neg, BitAnd, BitOr, BitXor, Not, Shl, Shr};
use std::fmt;

#[derive(Debug, Clone, Copy, PartialEq)]
struct Vec2 { x: f64, y: f64 }

impl Add for Vec2 {
    type Output = Vec2;
    fn add(self, rhs: Vec2) -> Vec2 {
        Vec2 { x: self.x + rhs.x, y: self.y + rhs.y }
    }
}

impl Mul for Vec2 {
    type Output = Vec2;
    fn mul(self, scalar: f64) -> Vec2 {
        Vec2 { x: self.x * scalar, y: self.y * scalar }
    }
}

impl Neg for Vec2 {
    type Output = Vec2;
    fn neg(self) -> Vec2 { Vec2 { x: -self.x, y: -self.y } }
}

let a = Vec2 { x: 1.0, y: 2.0 };
let b = Vec2 { x: 3.0, y: 4.0 };
let c = a + b * 2.0;
// * has higher precedence than + → a + (b * 2.0)
// b * 2.0 → Vec2{6.0, 8.0}
// a + Vec2{6.0, 8.0} → Vec2{7.0, 10.0}
// Precedence is fixed even though the types are custom!

// Comparison operators overloading: PartialOrd, Ord, PartialEq, Eq
// Index: Index, IndexMut
// Deref: Deref, DerefMut
// Addition assignment: AddAssign, etc.
```

### 5.13 Complete Rust Code Examples

```rust
use std::fs;
use std::io;

// ── Example 1: Bitwise Operations (Fixed Precedence vs C) ──
fn bitwise_demo() {
    let x: u32 = 0x12;

    // Rust: & has HIGHER precedence than == — no bug here!
    if x & 0x0F == 0 {       // (x & 0x0F) == 0: correct parse in Rust
        println!("lower nibble is zero");
    }

    // Bit manipulation:
    let flags: u32 = 0b00000000;
    let flags = flags | (1 << 0);  // Set bit 0 (READ)
    let flags = flags | (1 << 1);  // Set bit 1 (WRITE)
    let has_write = (flags & (1 << 1)) != 0;  // Test bit 1
    let flags = flags & !(1 << 0); // Clear bit 0 using ! (Rust's bitwise NOT)

    println!("has_write={}, flags={:08b}", has_write, flags);
}

// ── Example 2: Error Propagation Chain with ? ──
fn read_and_parse(path: &str) -> Result> {
    // ? extracts Ok value or returns Err early
    // Method calls chain naturally on the Ok value:
    let n = fs::read_to_string(path)?  // ? applied after read call
               .trim()                 // .trim() on Ok(String)
               .parse::()?;       // ? applied after parse call
    Ok(n * 2)
}

// ── Example 3: Range Operators and Slices ──
fn range_demo() {
    let data = vec![10, 20, 30, 40, 50, 60, 70, 80, 90, 100];

    // Range creates iterator:
    let sum: i32 = (0..10).map(|i| i * i).sum();  // 0²+1²+...+9² = 285
    println!("Sum of squares 0-9: {}", sum);

    // Range slicing — range has lower precedence than * and +:
    let start = 1 + 1;
    let end   = 3 * 2;
    let slice = &data[start..end];  // &data[(1+1)..(3*2)] = &data[2..6]
    println!("slice={:?}", slice);  // [30, 40, 50, 60]

    // Inclusive range in match:
    for &val in &data {
        let label = match val {
            1..=25   => "very low",
            26..=50  => "low",
            51..=75  => "medium",
            76..=100 => "high",
            _        => "out of range",
        };
        print!("{} ", label);
    }
    println!();
}

// ── Example 4: Non-Associative Comparisons (Forced Clarity) ──
fn comparison_demo() {
    let values = vec![3, 1, 4, 1, 5, 9, 2, 6];

    // Must use && to chain comparisons:
    let in_bounds: Vec = values.iter()
        .filter(|&&v| v >= 2 && v <= 6)  // Explicit &&, cannot chain <
        .collect();
    println!("in_bounds={:?}", in_bounds);

    // Rust prevents this common C bug by refusing to compile:
    // let bad = 1 < 2 < 3;  // ERROR: non-associative
    // Must write:
    let correct = 1 < 2 && 2 < 3;  // true — explicit and clear
    println!("1 < 2 < 3 = {}", correct);
}

// ── Example 5: Operator Overloading Respecting Precedence ──
#[derive(Debug, Clone, Copy)]
struct Bits(u32);

impl std::ops::BitAnd for Bits {
    type Output = Self;
    fn bitand(self, rhs: Self) -> Self { Bits(self.0 & rhs.0) }
}
impl std::ops::BitOr for Bits {
    type Output = Self;
    fn bitor(self, rhs: Self) -> Self { Bits(self.0 | rhs.0) }
}
impl std::ops::Not for Bits {
    type Output = Self;
    fn not(self) -> Self { Bits(!self.0) }
}
impl PartialEq for Bits {
    fn eq(&self, other: &Self) -> bool { self.0 == other.0 }
}

fn overloaded_ops() {
    let a = Bits(0b1010);
    let b = Bits(0b1100);
    let c = Bits(0b0001);

    // Precedence of & over | holds even with overloaded operators:
    let r = a & b | c;
    // = (a & b) | c = Bits(0b1000) | Bits(0b0001) = Bits(0b1001)
    println!("a&b|c = {:04b}", r.0);  // 1001

    // Precedence of & over ==:
    let equal = a & b == Bits(0b1000);  // (a & b) == Bits(0b1000)
    println!("a&b == 0b1000: {}", equal);  // true
}

// ── Example 6: as Casting and Numeric Conversions ──
fn casting_demo() {
    let f: f64 = 3.99;
    let i = f as i32;      // Truncation: 3 (not rounded!)
    let u = -1i32 as u32;  // Wrap: u32::MAX

    println!("3.99f64 as i32 = {}", i);   // 3
    println!("-1i32 as u32 = {}", u);      // 4294967295

    // as is left-associative:
    let chain = 300u32 as u16 as i16;
    // (300u32 as u16) = 300 (fits in u16)
    // 300u16 as i16 = 300 (fits in i16)
    println!("300u32 as u16 as i16 = {}", chain);  // 300

    // Saturating cast (since Rust 1.45):
    let inf = f64::INFINITY as i32;
    let nan = f64::NAN as i32;
    println!("INFINITY as i32 = {}", inf);  // 2147483647 (i32::MAX)
    println!("NAN as i32 = {}", nan);       // 0
}

fn main() {
    bitwise_demo();
    range_demo();
    comparison_demo();
    overloaded_ops();
    casting_demo();
}
```

---

## 6. Go — Complete Reference

Go deliberately chose **simplicity** in operator precedence. It has only **5 levels of binary operator precedence** and a set of unary operators. This makes it much easier to reason about expressions without reference material.

Key Go-specific features:
- **`++` and `--` are statements, not expressions** — eliminates an entire class of C bugs
- **`^` is both XOR (binary) and bitwise NOT (unary)** — no separate `~` operator
- **`&^` (AND NOT)** is a unique bitwise clear operator
- **`<-` channel receive** is a unary operator at statement level
- **No ternary operator**
- **No comma operator**
- **No implicit type conversions** — all conversions are explicit

### 6.1 Complete Go Operator Precedence Table

```
╔════════════╦═════════════════════════════════════════════════╦═════════════════════════╦═══════════════════╗
║ Precedence ║ Operators                                       ║ Category                ║ Associativity     ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  Unary     ║ +  -  !  ^  *  &  <-                           ║ Unary (highest overall) ║ Right → Left      ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  5(highest)║ *   /   %   <<   >>   &   &^                   ║ Multiplicative + bitwise║ Left → Right      ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  4         ║ +   -   |   ^                                   ║ Additive + bitwise      ║ Left → Right      ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  3         ║ ==   !=   <   <=   >   >=                       ║ Comparison              ║ NON-ASSOCIATIVE   ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  2         ║ &&                                              ║ Logical AND             ║ Left → Right      ║
╠════════════╬═════════════════════════════════════════════════╬═════════════════════════╬═══════════════════╣
║  1(lowest) ║ ||                                              ║ Logical OR              ║ Left → Right      ║
╚════════════╩═════════════════════════════════════════════════╩═════════════════════════╩═══════════════════╝

IMPORTANT NOTES:
1. Bitwise & is at level 5 — HIGHER than comparison (level 3). Fixes C's bug!
2. Bitwise ^ (XOR) and | are at level 4 — also HIGHER than comparison.
3. Only 5 levels of binary operators — far simpler than C's 12 binary levels.
4. Unary operators always have higher precedence than binary operators.
5. ++ and -- are STATEMENTS (not operators/expressions). No a[i++] in Go!

KEY DIFFERENCE FROM C: In Go, x & mask == 0 → (x & mask) == 0 — CORRECT!
(because & is level 5 and == is level 3; higher number = higher precedence in Go)
```

### 6.2 Unary Operators

```go
package main

import "fmt"

func unary_demo() {
    x := 5
    y := -x    // Unary minus: -5
    z := +x    // Unary plus: 5 (no-op for ints, triggers conversions)
    
    // Logical NOT:
    b := true
    nb := !b   // false
    
    // Bitwise NOT (Go uses ^ for this, C uses ~):
    n := uint8(0b10110011)  // 179
    inv := ^n               // 0b01001100 = 76 — bitwise complement
    // In C: ~n. In Go: ^n (both unary XOR and unary complement)
    
    // Dereference:
    p := &x
    v := *p    // Dereference: 5
    
    // Address-of:
    addr := &x  // *int pointing to x
    
    fmt.Println(y, z, nb, inv, v, addr)
}
```

### 6.3 Binary Operator Levels in Detail

#### Level 5 (Highest Binary): `*` `/` `%` `<<` `>>` `&` `&^`

Note that Go groups bitwise AND (`&`) and bitwise clear (`&^`) at the SAME level as multiplication. This is different from C (where `&` is far below `*`) and Rust.

```go
// All level-5 operators have the same precedence, left-to-right:
a := 10
b := 3
c := 2

r1 := a * b / c   // (a*b)/c = 30/2 = 15
r2 := a % b * c   // (a%b)*c = 1*2 = 2
r3 := a << b >> c // (a<>c = 80>>2 = 20

// Bitwise AND at same level as multiplication:
flags := uint32(0xFF00)
mask  := uint32(0x0F00)
// These have equal precedence — left-to-right:
r4 := flags & mask * 2   // Parses as: flags & (mask * 2)?? No!
                          // All level 5, left-to-right:
                          // (flags & mask) * 2 = 0x0F00 * 2 = 0x1E00
// Actually: * and & are the SAME level in Go → left-to-right
// (flags & mask) * 2

// In practice: always parenthesize to be clear
r5 := (flags & mask) * 2  // Explicit and clear

// Shifts:
n := uint32(1)
n <<= 4   // n = 16: shift left 4 (multiply by 16)
n >>= 2   // n = 4:  shift right 2 (divide by 4)

// In Go, shift counts must be unsigned or untyped constant:
var shift uint = 3
n = 1 << shift  // OK: shift is uint
// n = 1 << -1   // Compile error: negative shift
// n = 1 << 64   // Compile error: too large for uint32
```

#### Level 5 Special: `&^` — Bitwise Clear (AND NOT)

`&^` is Go's unique operator. `a &^ b` clears the bits in `a` that are set in `b`.

```go
// a &^ b  means: a AND (NOT b)  =  a & (^b) in most languages
// But in Go, ^b is bitwise NOT, so they're equivalent
// &^ is provided as a single operator for clarity and to avoid precedence issues

var flags uint32 = 0b11111111

// Clear specific bits using &^:
const BIT3 = uint32(1 << 3)
const BIT5 = uint32(1 << 5)

flags &^= BIT3        // Clear bit 3: flags = 0b11110111
flags &^= BIT5        // Clear bit 5: flags = 0b11010111
flags &^= BIT3 | BIT5 // Clear bits 3 and 5 in one operation

// Equivalent without &^:
flags &= ^(BIT3 | BIT5)  // Explicit NOT then AND: same result
// But &^ is cleaner and avoids the precedence question of ^ relative to &

// Practical example: flag management
const (
    FlagRead    = uint32(1 << 0)
    FlagWrite   = uint32(1 << 1)
    FlagExec    = uint32(1 << 2)
    FlagHidden  = uint32(1 << 3)
)

perms := FlagRead | FlagWrite | FlagExec | FlagHidden
perms &^= FlagHidden  // Remove hidden flag
fmt.Printf("perms: %04b\n", perms)  // 0111
```

#### Level 4: `+` `-` `|` `^`

Note: Go puts bitwise OR (`|`) and XOR (`^`) at the same level as addition/subtraction. This is different from C (where `|` and `^` are at very different levels from `+`).

```go
// All level-4 operators have same precedence, left-to-right:
a := 10
b := 3
c := 5

r1 := a + b - c   // (a+b)-c = 13-5 = 8
r2 := a - b + c   // (a-b)+c = 7+5  = 12

// Bitwise OR at same level as +, -:
flags := uint32(0)
bit1  := uint32(0x01)
bit2  := uint32(0x02)
val   := uint32(0x10)

// Tricky — all level 4, left-to-right!
r3 := flags + val | bit1 | bit2
// = ((flags + val) | bit1) | bit2
// = (0x10 | 0x01) | 0x02 = 0x11 | 0x02 = 0x13

// In practice: always parenthesize! Don't mix + and | without parens
combined := flags | bit1 | bit2  // Clear intent

// XOR at level 4:
x := uint32(0b1010)
y := uint32(0b1100)
z := uint32(0b0001)
r4 := x ^ y | z   // (x^y) | z = 0b0110 | 0b0001 = 0b0111
r5 := x | y ^ z   // (x|y) ^ z = 0b1110 ^ 0b0001 = 0b1111

// String concatenation + at level 4:
s1 := "hello"
s2 := " "
s3 := "world"
result := s1 + s2 + s3  // "hello world" — left-to-right
```

#### Level 3: Comparison Operators (Non-Associative)

```go
// Go comparison operators are non-associative:
// var x = 1 < 2 < 3  // COMPILE ERROR

// Must use explicit &&:
x := 5
in_range := x > 0 && x < 10   // Correct range check
equal_check := x == 5 && x != 3  // Multiple conditions

// Note: & (level 5) > == (level 3)
// So: x & mask == 0 → (x & mask) == 0 — correct in Go!
n := uint32(0x12)
if n & 0x0F == 0 {      // Parses as (n & 0x0F) == 0 ✓
    fmt.Println("lower nibble is zero")
}

// String comparison:
s1, s2 := "apple", "banana"
if s1 < s2 {
    fmt.Println("apple comes before banana")  // Lexicographic comparison
}
```

#### Levels 2 and 1: `&&` and `||` (Short-Circuit)

```go
// Same short-circuit semantics as C and Rust
// && = level 2 (higher than ||)
// || = level 1 (lowest binary)

func mayPanic() bool {
    fmt.Println("evaluated!")
    return true
}

// Short-circuit:
false && mayPanic()  // "evaluated!" NOT printed (left is false)
true  || mayPanic()  // "evaluated!" NOT printed (left is true)

// Null/nil check pattern (Go uses nil instead of NULL):
type Node struct {
    Val  int
    Next *Node
}
var node *Node = nil
if node != nil && node.Val > 0 {  // node.Val safe: nil checked first
    fmt.Println(node.Val)
}

// Complex boolean expression:
a, b, c, d := true, false, true, false
result := a || b && c || d
//       = a || (b && c) || d   (&&=level2 > ||=level1)
//       = true || false || false
//       = true (short-circuits after first ||)

// In Go, boolean values must be bool — no int-to-bool:
// if 1 { }     // COMPILE ERROR: not bool
// if x { }     // COMPILE ERROR if x is int
if x > 0 { }    // CORRECT: x > 0 is a bool expression
```

### 6.4 Channel Operator `<-`

The `<-` operator is used for channel communication and appears as a unary operator for receive.

```go
// Channel basics:
ch := make(chan int, 10)

// SEND (statement form — <- is part of the send statement):
ch <- 42       // Send 42 to channel ch (statement, not expression)
ch <- x + 1   // Send computed value

// RECEIVE (unary operator — value of expression is received item):
v := <-ch      // Receive from ch, assign to v
fmt.Println(<-ch)  // Receive and print immediately

// Close check (two-value receive):
val, ok := <-ch   // val = received value, ok = false if channel closed and empty

// In select statement:
select {
case msg := <-ch1:
    process(msg)
case ch2 <- data:
    // sent data to ch2
default:
    // no channel ready
}

// Direction in type declarations:
func producer(out chan<- int) {  // out: send-only channel
    out <- 42
}
func consumer(in <-chan int) {   // in: receive-only channel
    v := <-in
    fmt.Println(v)
}

// Receive has high unary precedence:
// <-ch.Field   would be WRONG: <-(ch.Field) if ch is a struct?
// Actually: <- applies to the expression after it: <-ch
// ch.method() returns a channel: (<-ch).method() needs parens for method on received val
```

### 6.5 `++` and `--` Are Statements (Not Expressions)

This is one of Go's most significant differences from C. In Go, `i++` and `i--` are **statements**, not expressions. They do not have a value and cannot be embedded in other expressions.

```go
// VALID: i++ and i-- as statements
i := 0
i++   // i = 1
i++   // i = 2
i--   // i = 1

// INVALID in Go (would be valid in C):
// j := i++    // COMPILE ERROR: i++ is a statement, not an expression
// a[i++] = x  // COMPILE ERROR: same reason
// fmt.Println(i++) // COMPILE ERROR

// INVALID: prefix ++ and -- don't exist in Go:
// ++i  // COMPILE ERROR
// --i  // COMPILE ERROR

// Correct Go equivalents for C patterns:
// C: arr[i++] = val;
// Go:
arr[i] = val
i++

// C: for (int i=0; i < n; i++) { arr[i++] = compute(i); }  // complex and UB-prone in C
// Go (clean, unambiguous):
for i := 0; i < n; i++ {
    arr[i] = compute(i)  // i is used only as index, no modification
}

// This design eliminates ALL of C's undefined behavior from:
//   i++ + i++, arr[i] = i++, etc.
// In Go, these are simply compile errors.
```

### 6.6 No Ternary, No Comma Operator

```go
// Go has no ternary operator (condition ? a : b)
// Must use if-else:
x := 5
var label string
if x > 0 {
    label = "positive"
} else {
    label = "non-positive"
}

// Or as an initialization expression (Go's approach):
label2 := func() string {
    if x > 0 { return "positive" }
    return "non-positive"
}()

// Go has no comma operator for evaluating multiple expressions inline
// Multiple return values serve the common use cases:
func divmod(a, b int) (int, int) {
    return a / b, a % b
}
q, r := divmod(10, 3)  // q=3, r=1

// For loop with multiple variables:
for i, j := 0, 10; i < j; i, j = i+1, j-1 {
    fmt.Printf("i=%d, j=%d\n", i, j)
}
// The "i, j = i+1, j-1" is a multi-assignment, not a comma operator
```

### 6.7 Type Conversions in Go

Go has no implicit type conversions between numeric types. All conversions are explicit using conversion expressions.

```go
// Explicit conversion syntax (looks like function call):
var i int = 42
var f float64 = float64(i)   // int → float64
var u uint = uint(i)          // int → uint (wraps for negatives)
var b byte = byte(i)          // int → uint8 (truncates)

// Conversions have HIGH precedence (like function calls):
x := 3
y := 4
dist := float64(x*x + y*y)   // float64(25) = 25.0
// NOT: (float64(x)*x + y*y) — conversion only applies to its immediate argument

// This differs from C cast which applies to entire right expression:
// (double)x * x + y * y  in C = (double)(x)*x+... = x promoted then multiplied
// float64(x) in Go ONLY converts x, not x*x

// String conversions:
n := 65
s := string(n)           // = "A": int → string (rune interpretation)
str := fmt.Sprintf("%d", n)  // = "65": int → decimal string representation
// GOTCHA: string(65) = "A" (treats as rune), not "65"!
// Use strconv.Itoa(

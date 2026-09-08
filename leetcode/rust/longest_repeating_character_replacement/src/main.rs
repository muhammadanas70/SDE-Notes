use std::collections::HashMap;

struct Solution;

impl Solution {
    pub fn character_replacement(s: String, k: i32) -> i32 {

        let mut count: HashMap<u8, i32> = HashMap::new();

    }
}

fn main() {
    println!("Hello, world!");

    let tests = vec![
        ("ABAB".to_string(), 2, 4),
        ("AABABBA".to_string(), 1, 4),
        ("AAAA".to_string(), 0, 4),
        ("ABCDE".to_string(), 1, 2),
        ("ABBB".to_string(), 2, 4),
        ("AABA".to_string(), 0, 2),
    ]

    for (s, k, expected) in tests {
        let result = Solution::character_replacement(s.clone(), k);
        println!("s={}, k={} -> got={}, expected={}", s, k, result, expected);
    }
}

struct Solution;

impl Solution {
    pub fn maximum_candies(candies: Vec<i32>, k: i64) -> i32 {
        _ = candies;
        _ = k;
        -1
    }
}

fn main() {
    println!("Hello, world!");
    let candies = vec![5,8,6];
    let k = 3;
    
    let result = Solution::maximum_candies(candies, k);
    println!("{}", result);

}

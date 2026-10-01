package main

func majorityElement(nums []int) int {

	result := 0
	returnKey := 0
	dict := make(map[int]int)

	for _, value := range nums {

		if _, ok := dict[value]; ok {
			dict[value] += 1
		} else {
			dict[value] = 1
		}
	}

	for k, v := range dict {
		if v > result {
			result = v
			returnKey = k
		}
	}
	return returnKey

}

func main() {

	var nums = []int{2, 2, 1, 1, 1, 2, 2}
	println(majorityElement(nums))
}

//Input: nums = [3,2,3]
//Output: 3
//Example 2:
//
//Input: nums = [2,2,1,1,1,2,2]
//Output: 2

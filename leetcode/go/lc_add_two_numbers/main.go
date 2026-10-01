package main

import "fmt"

func removeElement(nums []int, val int) int {

	k := 0

	for _, value := range nums {

		if value != val {
			nums[k] = value
			k++

		}



	}
	fmt.Println(len(nums))

	return k

}


func main() {


	nums := []int{3, 2, 2, 3}
	val := 3
	fmt.Println(removeElement(nums, val))


}

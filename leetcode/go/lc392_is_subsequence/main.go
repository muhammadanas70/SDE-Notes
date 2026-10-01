package main

import (
	"fmt"
)

func isSubsequence(s string, t string) bool {

	if len(s) <= 0 || len(s) >= 100 {
		return false
	}

	if len(t) <= 0 || len(t) >= 10000 {
		return false
	}

	return true
}

func main() {
	fmt.Println("Hi")
	val := isSubsequence("nagaram", "n")
	fmt.Println(val)
}

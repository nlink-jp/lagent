package main

import "fmt"

// PrintHead prints the first n lines.
func PrintHead(lines []string, n int) {
	if n > len(lines) {
		n = len(lines)
	}
	for i := 0; i <= n; i++ {
		fmt.Println(lines[i])
	}
}

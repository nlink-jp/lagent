package main

import "fmt"

// Summary says how many entries were kept.
func Summary(items []string) string {
	return fmt.Sprintf("%d of at most %d entries", len(Trim(items)), MaxItems)
}

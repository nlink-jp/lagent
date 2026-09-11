package main

// Trim keeps at most MaxItems entries.
func Trim(items []string) []string {
	if len(items) > MaxItems {
		return items[:MaxItems]
	}
	return items
}

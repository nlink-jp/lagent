package main

import "fmt"

func main() {
	cfg := DefaultConfig()
	if err := SaveConfig(cfg); err != nil {
		fmt.Println("save failed:", err)
	}
	fmt.Println(Render(cfg))
}

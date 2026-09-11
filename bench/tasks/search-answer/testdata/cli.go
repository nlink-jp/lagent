package main

import "fmt"

// Config is what the program remembers between runs.
type Config struct {
	Name  string `json:"name"`
	Width int    `json:"width"`
}

func DefaultConfig() Config { return Config{Name: "demo", Width: 80} }

// Render formats the configuration for the terminal.
func Render(c Config) string { return fmt.Sprintf("%s (%d)", c.Name, c.Width) }

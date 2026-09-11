package main

import (
	"encoding/json"
	"os"
)

const configPath = "app.json"

// SaveConfig writes the configuration to disk.
func SaveConfig(c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0o644)
}

// LoadConfig reads the configuration back.
func LoadConfig() (Config, error) {
	var c Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

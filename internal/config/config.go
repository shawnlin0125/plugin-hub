package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// PluginDef defines a plugin's metadata and test configuration.
type PluginDef struct {
	ID          string `yaml:"id"          json:"id"`
	Name        string `yaml:"name"        json:"name"`
	Version     string `yaml:"version"     json:"version"`
	Description string `yaml:"description" json:"description"`
	Repo        string `yaml:"repo"        json:"repo"`
	TestCommand string `yaml:"test_command" json:"test_command"`
	Enabled     bool   `yaml:"enabled"     json:"-"`
}

// Config is the top-level configuration file.
type Config struct {
	Plugins []PluginDef `yaml:"plugins"`
}

// Load reads and parses the plugins config YAML.
func Load(path string) (*Config, error) {
	if path == "" {
		// Try default locations
		for _, p := range []string{
			"/etc/plugin-hub/plugins.yaml",
			"./config/plugins.yaml",
			"config/plugins.yaml",
		} {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

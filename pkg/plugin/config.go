package plugin

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ManagerConfig describes how the plugin manager should behave.
type ManagerConfig struct {
	PluginDir string                  `yaml:"pluginDir"`
	Defaults  IsolationPolicy         `yaml:"defaults"`
	Plugins   map[string]PluginConfig `yaml:"plugins"`
}

// PluginConfig is the configuration block for a single plugin instance.
type PluginConfig struct {
	Enabled bool             `yaml:"enabled"`
	Path    string           `yaml:"path"`
	Config  map[string]any   `yaml:"config"`
	Policy  *IsolationPolicy `yaml:"policy"`
}

// IsolationPolicy governs the security restrictions enforced for a plugin.
type IsolationPolicy struct {
	AllowedCapabilities []Capability `yaml:"allowedCapabilities"`
	DeniedCapabilities  []Capability `yaml:"deniedCapabilities"`
}

// Merge returns a new policy using values from other when not present.
func (p IsolationPolicy) Merge(other IsolationPolicy) IsolationPolicy {
	if len(p.AllowedCapabilities) == 0 {
		p.AllowedCapabilities = other.AllowedCapabilities
	}
	if len(p.DeniedCapabilities) == 0 {
		p.DeniedCapabilities = other.DeniedCapabilities
	}
	return p
}

// LoadManagerConfig reads a YAML file into a ManagerConfig.
func LoadManagerConfig(path string) (ManagerConfig, error) {
	var cfg ManagerConfig
	if path == "" {
		return cfg, errors.New("config path cannot be empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read plugin config: %w", err)
	}
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal plugin config: %w", err)
	}
	if cfg.Plugins == nil {
		cfg.Plugins = map[string]PluginConfig{}
	}
	return cfg, nil
}

// Validate ensures the manager configuration is internally consistent.
func (c ManagerConfig) Validate() error {
	for id, plugin := range c.Plugins {
		if id == "" {
			return errors.New("plugin id cannot be empty")
		}
		if !plugin.Enabled {
			continue
		}
		if plugin.Path == "" {
			return fmt.Errorf("plugin %s path cannot be empty when enabled", id)
		}
	}
	return nil
}

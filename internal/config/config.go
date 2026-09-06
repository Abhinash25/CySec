// Package config manages CySec.env user configuration.
// Configuration is stored as YAML in the platform-specific config directory.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the CySec.env configuration.
type Config struct {
	v       *viper.Viper
	cfgFile string
}

// Defaults returns the default configuration values.
func Defaults() map[string]interface{} {
	return map[string]interface{}{
		"banner.show_on_start":   true,
		"banner.first_run_only":  true,
		"install.confirm":        true,
		"install.verify_checksum": true,
		"install.health_check":   true,
		"cache.max_size_mb":      5120,       // 5 GB
		"logging.level":          "info",
		"logging.redact_secrets": true,
		"registry.auto_update":   false,
		"registry.update_interval_hours": 24,
	}
}

// Load reads the configuration from disk, creating defaults if necessary.
func Load(configDir string) (*Config, error) {
	cfgFile := filepath.Join(configDir, "config.yaml")

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(cfgFile)

	// Set defaults
	for key, val := range Defaults() {
		v.SetDefault(key, val)
	}

	// Environment variable overrides: CYSEC_<KEY> with dots replaced by underscores
	v.SetEnvPrefix("CYSEC")
	v.AutomaticEnv()

	// Read existing config (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// If the file exists but can't be read, check if it actually exists
			if _, statErr := os.Stat(cfgFile); statErr == nil {
				return nil, fmt.Errorf("failed to read config %s: %w", cfgFile, err)
			}
			// File doesn't exist, that's fine — we'll use defaults
		}
	}

	return &Config{v: v, cfgFile: cfgFile}, nil
}

// Save writes the current configuration to disk.
func (c *Config) Save() error {
	dir := filepath.Dir(c.cfgFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	return c.v.WriteConfigAs(c.cfgFile)
}

// Get returns a configuration value.
func (c *Config) Get(key string) interface{} {
	return c.v.Get(key)
}

// GetString returns a string configuration value.
func (c *Config) GetString(key string) string {
	return c.v.GetString(key)
}

// GetBool returns a boolean configuration value.
func (c *Config) GetBool(key string) bool {
	return c.v.GetBool(key)
}

// GetInt returns an integer configuration value.
func (c *Config) GetInt(key string) int {
	return c.v.GetInt(key)
}

// Set updates a configuration value.
func (c *Config) Set(key string, value interface{}) {
	c.v.Set(key, value)
}

// IsFirstRun returns true if no config file exists on disk.
func (c *Config) IsFirstRun() bool {
	_, err := os.Stat(c.cfgFile)
	return os.IsNotExist(err)
}

// MarkFirstRunDone persists the config (creating the file) to signal first run is complete.
func (c *Config) MarkFirstRunDone() error {
	c.Set("banner.first_run_done", true)
	return c.Save()
}

// ConfigFile returns the path to the configuration file.
func (c *Config) ConfigFile() string {
	return c.cfgFile
}

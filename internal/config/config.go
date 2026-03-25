package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	APIs map[string]APIConfig `json:"apis"`
}

type APIConfig struct {
	Name        string     `json:"name"`
	SpecURL     string     `json:"specURL"`
	BaseURL     string     `json:"baseURL"`
	Type        string     `json:"type"` // "openapi3", "swagger2", "graphql"
	Auth        AuthConfig `json:"auth"`
}

type AuthConfig struct {
	Type   string `json:"type"`  // "none", "api_key", "bearer"
	Header string `json:"header"` // header name
	Value  string `json:"value"`  // token value
}

func getConfigPath() (string, error) {
	dir := os.Getenv("AURL_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config", "aurl")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIs: make(map[string]APIConfig)}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.APIs == nil {
		cfg.APIs = make(map[string]APIConfig)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Backup existing file
	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		os.Rename(path, backup)
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) AddAPI(api APIConfig) error {
	if c.APIs == nil {
		c.APIs = make(map[string]APIConfig)
	}
	c.APIs[api.Name] = api
	return Save(c)
}

func (c *Config) GetAPI(name string) (APIConfig, bool) {
	api, ok := c.APIs[name]
	return api, ok
}

func (c *Config) ListAPIs() []string {
	var names []string
	for name := range c.APIs {
		names = append(names, name)
	}
	return names
}

// Error types
type ConfigError struct {
	Msg string
}

func (e *ConfigError) Error() string {
	return e.Msg
}

func NewConfigError(format string, args ...interface{}) *ConfigError {
	return &ConfigError{Msg: fmt.Sprintf(format, args...)}
}

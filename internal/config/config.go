package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Alias struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
	Thinking string `toml:"thinking"`
}

type OpenCodeDefaults struct {
	Plan  string `toml:"plan"`
	Build string `toml:"build"`
}

type OpenCodeConfig struct {
	Defaults OpenCodeDefaults      `toml:"defaults"`
	Aliases  map[string]Alias      `toml:"aliases"`
}

type AppConfig struct {
	AllowedUsers       []int64        `toml:"allowed_users"`
	DefaultOpenCodeDir string         `toml:"default_opencode_dir"`
	OpenCode           OpenCodeConfig `toml:"opencode"`
}

func Load(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg AppConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *AppConfig) Validate() error {
	if len(c.AllowedUsers) == 0 {
		return fmt.Errorf("allowed_users must not be empty")
	}
	if c.DefaultOpenCodeDir == "" {
		return fmt.Errorf("default_opencode_dir must be set")
	}
	if len(c.OpenCode.Aliases) == 0 {
		return fmt.Errorf("at least one opencode alias must be defined")
	}
	return nil
}

func (c *AppConfig) IsAllowed(userID int64) bool {
	for _, id := range c.AllowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}

func (c *AppConfig) AliasNames() []string {
	names := make([]string, 0, len(c.OpenCode.Aliases))
	for name := range c.OpenCode.Aliases {
		names = append(names, name)
	}
	return names
}

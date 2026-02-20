package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all bot configuration
type Config struct {
	Nick       string `yaml:"nick"`
	NickPass   string `yaml:"nick_pass"`
	Alternate  string `yaml:"alternate"`
	Server     string `yaml:"server"`
	Port       int    `yaml:"port"`
	ServerPass string `yaml:"server_pass"`
	IRCName    string `yaml:"irc_name"`
	Username   string `yaml:"username"`
	OperNick   string `yaml:"oper_nick"`
	OperPass   string `yaml:"oper_pass"`
	AdminPass  string `yaml:"admin_pass"`
	DataDir    string `yaml:"data_dir"`
}

// Load reads and parses a YAML configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}

	return &cfg, nil
}

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configurable settings for oak-go-cli.
type Config struct {
	SystemManagerIP  string `json:"system_manager_ip"`
	ClusterManagerIP string `json:"cluster_manager_ip"`
	ClusterName      string `json:"cluster_name"`
	ClusterLocation  string `json:"cluster_location"`
	MainOakRepoPath  string `json:"main_oak_repo_path"`
	// Login credentials for the Oakestra API (defaults to "Admin"/"Admin").
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// URL to the Oakestra troubleshoot skill for Claude CLI.
	TroubleshootSkillURL string `json:"troubleshoot_skill_url,omitempty"`
	// Path to the FLOps addon repository (used by oak addon flops).
	FlopsRepoPath string `json:"flops_repo_path,omitempty"`
}

// DefaultTroubleshootSkillURL is the default URL for the Oakestra troubleshoot skill.
const DefaultTroubleshootSkillURL = "https://raw.githubusercontent.com/oakestra/oakestra/refs/heads/develop/SKILLS/troubleshoot-oakestra.md"

// GetTroubleshootSkillURL returns the configured skill URL or the default.
func (c *Config) GetTroubleshootSkillURL() string {
	if c.TroubleshootSkillURL == "" {
		return DefaultTroubleshootSkillURL
	}
	return c.TroubleshootSkillURL
}

// GetUsername returns the configured username or the default "Admin".
func (c *Config) GetUsername() string {
	if c.Username == "" {
		return "Admin"
	}
	return c.Username
}

// GetPassword returns the configured password or the default "Admin".
func (c *Config) GetPassword() string {
	if c.Password == "" {
		return "Admin"
	}
	return c.Password
}

const DefaultSystemManagerIP = "0.0.0.0"

// Dir returns the oak_cli user folder (~/oak_cli).
func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "oak_cli")
}

// Path returns the path to the JSON config file.
func Path() string {
	return filepath.Join(Dir(), ".oak_go_cli_config.json")
}

// SLAFolder returns the path to the SLA folder inside the user dir.
func SLAFolder() string {
	return filepath.Join(Dir(), "SLAs")
}

// Load reads the config file. Returns an empty Config on first run.
func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0o644)
}

// Set updates a single key by name and saves the config.
func Set(key, value string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	switch key {
	case "system_manager_ip":
		cfg.SystemManagerIP = value
	case "root_orchestrator_address": // alias supported to make it more user friendly
		cfg.SystemManagerIP = value
	case "cluster_manager_ip":
		cfg.ClusterManagerIP = value
	case "cluster_name":
		cfg.ClusterName = value
	case "cluster_location":
		cfg.ClusterLocation = value
	case "main_oak_repo_path":
		cfg.MainOakRepoPath = value
	case "troubleshoot_skill_url":
		cfg.TroubleshootSkillURL = value
	case "flops_repo_path":
		cfg.FlopsRepoPath = value
	default:
		return fmt.Errorf(
			"unknown config key %q.\nValid keys: system_manager_ip, root_orchestrator_address, cluster_manager_ip, cluster_name, cluster_location, main_oak_repo_path, troubleshoot_skill_url, flops_repo_path\n"+
				"For login credentials use: oak config credentials <username> [password]", key)
	}
	return Save(cfg)
}

// SetCredentials stores login credentials.
func SetCredentials(username, password string) error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Username = username
	cfg.Password = password
	return Save(cfg)
}

// URL returns the full base URL of the system manager for this config.
// Falls back to 0.0.0.0:10000 when no IP is configured.
func (c *Config) URL() string {
	ip := c.SystemManagerIP
	if ip == "" {
		ip = DefaultSystemManagerIP
	}
	return fmt.Sprintf("http://%s:10000", ip)
}

// SystemManagerURL returns the full base URL of the system manager.
// Falls back to 0.0.0.0:10000 when no IP is configured.
func SystemManagerURL() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.URL(), nil
}

// Keys returns all configurable general key names.
func Keys() []string {
	return []string{
		"system_manager_ip",
		"cluster_manager_ip",
		"cluster_name",
		"cluster_location",
		"main_oak_repo_path",
		"troubleshoot_skill_url",
		"flops_repo_path",
	}
}

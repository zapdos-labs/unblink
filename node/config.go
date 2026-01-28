package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Config represents the v2 node configuration
type Config struct {
	RelayAddress string        `json:"relay_address"`
	NodeID       string        `json:"node_id"`
	Token        string        `json:"token"`
	Reconnect    ReconnectConf `json:"reconnect"`
}

// ReconnectConf defines reconnection behavior
type ReconnectConf struct {
	Enabled        bool `json:"enabled"`
	MaxNumAttempts int  `json:"max_num_attempts"`
}

// ConfigFile bundles a config with its file path
type ConfigFile struct {
	Path   string
	Config *Config
}

// GetDefaultConfigPath returns the default config file path
func GetDefaultConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".unblink", "config.json"), nil
}

// Load creates a ConfigFile, resolving default path if customPath is empty
// If the file doesn't exist, creates a new config with a generated node ID
func Load(customPath string) (*ConfigFile, error) {
	configPath := customPath
	if configPath == "" {
		var err error
		configPath, err = GetDefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	// Try to read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}

		// File doesn't exist - create default config
		cfg := &Config{
			RelayAddress: "ws://localhost:9020",
			NodeID:       uuid.New().String(),
			Token:        "",
			Reconnect: ReconnectConf{
				Enabled:        true,
				MaxNumAttempts: 10,
			},
		}

		cf := &ConfigFile{
			Path:   configPath,
			Config: cfg,
		}

		// Save default config
		if err := cf.Save(); err != nil {
			return nil, fmt.Errorf("save default config: %w", err)
		}

		return cf, nil
	}

	// Parse existing config
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &ConfigFile{
		Path:   configPath,
		Config: &cfg,
	}, nil
}

// Save persists the config to its path
func (cf *ConfigFile) Save() error {
	configDir := filepath.Dir(cf.Path)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cf.Config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(cf.Path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Show displays the config
func (cf *ConfigFile) Show() error {
	fmt.Println()
	fmt.Println("Config File:")
	fmt.Println("  Path:", cf.Path)
	fmt.Println()
	fmt.Println("Config:")
	fmt.Printf("  relay_address: %s\n", cf.Config.RelayAddress)
	fmt.Printf("  node_id: %s\n", cf.Config.NodeID)
	fmt.Printf("  token: %s\n", cf.Config.Token)
	fmt.Println()

	if cf.Config.Token != "" {
		fmt.Println("  Status: Ready to connect")
	} else {
		fmt.Println("  Status: No token set")
	}

	return nil
}

// Delete removes the config file after confirmation
func (cf *ConfigFile) Delete() error {
	// Check if file exists
	if _, err := os.Stat(cf.Path); os.IsNotExist(err) {
		fmt.Printf("Config file does not exist: %s\n", cf.Path)
		return nil
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete the config file at %s? (yes/no): ", cf.Path)
	var response string
	fmt.Scanln(&response)

	if response != "yes" {
		fmt.Println("Deletion cancelled")
		return nil
	}

	// Delete the config file
	if err := os.Remove(cf.Path); err != nil {
		return fmt.Errorf("delete config file: %w", err)
	}

	fmt.Printf("Config file deleted: %s\n", cf.Path)
	return nil
}

// Logout clears the token from config and saves
func (cf *ConfigFile) Logout() error {
	if cf.Config.Token == "" {
		log.Println("[Node] No token set.")
		return nil
	}

	// Clear token
	cf.Config.Token = ""

	// Save config
	if err := cf.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	log.Println("[Node] Token cleared.")
	return nil
}

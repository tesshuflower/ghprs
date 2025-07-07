package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RepositoryConfig represents a single repository configuration
type RepositoryConfig struct {
	Name    string `yaml:"name"`
	Konflux bool   `yaml:"konflux,omitempty"`
}

// Config represents the application configuration
type Config struct {
	Repositories []RepositoryConfig `yaml:"repositories"`
	Defaults     struct {
		State string `yaml:"state"`
		Limit int    `yaml:"limit"`
	} `yaml:"defaults"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Repositories: []RepositoryConfig{},
		Defaults: struct {
			State string `yaml:"state"`
			Limit int    `yaml:"limit"`
		}{
			State: "open",
			Limit: 30,
		},
	}
}

// LoadConfig loads configuration from the config file
func LoadConfig() (*Config, error) {
	configPath := getConfigPath()

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfig saves the configuration to the config file
func SaveConfig(config *Config) error {
	configPath := getConfigPath()

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// getConfigPath returns the path to the configuration file
func getConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return ".ghprs.yaml"
	}
	return filepath.Join(homeDir, ".config", "ghprs", "config.yaml")
}

// GetConfigPath returns the configuration file path (exported for CLI commands)
func GetConfigPath() string {
	return getConfigPath()
}

// GetRepositories returns the appropriate repository list based on whether it's Konflux or not
func (c *Config) GetRepositories(isKonflux bool) []string {
	var repos []string
	for _, repo := range c.Repositories {
		if !isKonflux || repo.Konflux {
			repos = append(repos, repo.Name)
		}
	}
	return repos
}

// AddRepository adds a repository to the list
func (c *Config) AddRepository(repo string, isKonflux bool) bool {
	// Check if repo already exists
	for i, existingRepo := range c.Repositories {
		if existingRepo.Name == repo {
			// Update existing repo's Konflux flag if needed
			if isKonflux && !existingRepo.Konflux {
				c.Repositories[i].Konflux = true
				return true
			}
			return false // Already exists with same settings
		}
	}

	// Add new repository
	c.Repositories = append(c.Repositories, RepositoryConfig{
		Name:    repo,
		Konflux: isKonflux,
	})
	return true
}

// RemoveRepository removes a repository from the list
func (c *Config) RemoveRepository(repo string, isKonflux bool) bool {
	for i, existingRepo := range c.Repositories {
		if existingRepo.Name == repo {
			if isKonflux && existingRepo.Konflux {
				// Remove Konflux flag but keep repository if it's not exclusively Konflux
				c.Repositories[i].Konflux = false
				return true
			} else if !isKonflux {
				// Remove repository entirely
				c.Repositories = append(c.Repositories[:i], c.Repositories[i+1:]...)
				return true
			}
		}
	}
	return false
}

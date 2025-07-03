package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ghprs configuration",
	Long: `Manage ghprs configuration file.

The configuration file allows you to set repositories, states, and limits.
Repositories can be marked as Konflux repositories. 'ghprs list' shows all repositories,
while 'ghprs konflux' shows only repositories marked as Konflux.
Configuration is stored in ~/.config/ghprs/config.yaml`,
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration file contents and location.`,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration file: %s\n\n", GetConfigPath())

		fmt.Println("Current configuration:")
		fmt.Printf("  Default State: %s\n", config.Defaults.State)
		fmt.Printf("  Default Limit: %d\n", config.Defaults.Limit)

		if len(config.Repositories) > 0 {
			fmt.Println("  Repositories:")
			for _, repo := range config.Repositories {
				if repo.Konflux {
					fmt.Printf("    - %s (Konflux)\n", repo.Name)
				} else {
					fmt.Printf("    - %s\n", repo.Name)
				}
			}
		} else {
			fmt.Println("  Repositories: (none)")
		}
	},
}

// configInitCmd initializes a new configuration file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Long:  `Create a new configuration file with default values.`,
	Run: func(cmd *cobra.Command, args []string) {
		config := DefaultConfig()

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration file created at: %s\n", GetConfigPath())
		fmt.Println("\nDefault configuration:")
		fmt.Printf("  State: %s\n", config.Defaults.State)
		fmt.Printf("  Limit: %d\n", config.Defaults.Limit)
		fmt.Println("  Repositories: (none)")
		fmt.Println("\nEdit the file to add your repositories and customize settings.")
		fmt.Println("Use 'ghprs config add-repo owner/repo' to add regular repositories.")
		fmt.Println("Use 'ghprs config add-konflux-repo owner/repo' to add repositories for Konflux use.")
	},
}

// configAddRepoCmd adds a repository to the configuration
var configAddRepoCmd = &cobra.Command{
	Use:   "add-repo <owner/repo>",
	Short: "Add a repository to default list",
	Long:  `Add a repository to the default repositories list in the configuration.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

		// Validate repo format
		if !strings.Contains(repo, "/") || strings.Count(repo, "/") != 1 {
			fmt.Println("Repository must be in the format 'owner/repo'")
			os.Exit(1)
		}

		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Add the repository using the helper method
		if !config.AddRepository(repo, false) {
			fmt.Printf("Repository %s is already in the configuration\n", repo)
			return
		}

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added repository %s to configuration\n", repo)
	},
}

// configRemoveRepoCmd removes a repository from the configuration
var configRemoveRepoCmd = &cobra.Command{
	Use:   "remove-repo <owner/repo>",
	Short: "Remove a repository from default list",
	Long:  `Remove a repository from the default repositories list in the configuration.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Remove the repository using the helper method
		if !config.RemoveRepository(repo, false) {
			fmt.Printf("Repository %s not found in configuration\n", repo)
			return
		}

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed repository %s from configuration\n", repo)
	},
}

// configSetCmd sets configuration values
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value. Available keys:
  - state: default state filter (open, closed, all)
  - limit: default limit for number of results`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		switch key {
		case "state":
			if value != "open" && value != "closed" && value != "all" {
				fmt.Println("State must be one of: open, closed, all")
				os.Exit(1)
			}
			config.Defaults.State = value

		case "limit":
			var limit int
			if _, err := fmt.Sscanf(value, "%d", &limit); err != nil {
				fmt.Println("Limit must be a number")
				os.Exit(1)
			}
			if limit <= 0 {
				fmt.Println("Limit must be greater than 0")
				os.Exit(1)
			}
			config.Defaults.Limit = limit

		default:
			fmt.Printf("Unknown configuration key: %s\n", key)
			fmt.Println("Available keys: state, limit")
			os.Exit(1)
		}

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Set %s = %s\n", key, value)
	},
}

// configAddKonfluxRepoCmd adds a repository and marks it as a Konflux repository
var configAddKonfluxRepoCmd = &cobra.Command{
	Use:   "add-konflux-repo <owner/repo>",
	Short: "Add a repository and mark it as a Konflux repository",
	Long:  `Add a repository to the configuration and mark it as a Konflux repository. Konflux repositories will be included when running 'ghprs konflux' command.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

		// Validate repo format
		if !strings.Contains(repo, "/") || strings.Count(repo, "/") != 1 {
			fmt.Println("Repository must be in the format 'owner/repo'")
			os.Exit(1)
		}

		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Add the repository using the helper method
		if !config.AddRepository(repo, true) {
			fmt.Printf("Repository %s is already configured as a Konflux repository\n", repo)
			return
		}

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Added repository %s and marked it as a Konflux repository\n", repo)
	},
}

// configRemoveKonfluxRepoCmd removes the Konflux marking from a repository
var configRemoveKonfluxRepoCmd = &cobra.Command{
	Use:   "remove-konflux-repo <owner/repo>",
	Short: "Remove the Konflux marking from a repository",
	Long:  `Remove the Konflux marking from a repository in the configuration. The repository will remain in the list but will no longer be treated as a Konflux repository.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		repo := args[0]

		config, err := LoadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Remove the repository using the helper method
		if !config.RemoveRepository(repo, true) {
			fmt.Printf("Repository %s not found or not marked as a Konflux repository\n", repo)
			return
		}

		if err := SaveConfig(config); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed Konflux marking from repository %s\n", repo)
	},
}

func init() {
	RootCmd.AddCommand(configCmd)

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configAddRepoCmd)
	configCmd.AddCommand(configRemoveRepoCmd)
	configCmd.AddCommand(configAddKonfluxRepoCmd)
	configCmd.AddCommand(configRemoveKonfluxRepoCmd)
	configCmd.AddCommand(configSetCmd)
}

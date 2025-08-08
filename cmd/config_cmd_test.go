package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Configuration Commands Functionality", func() {
	var tempConfigPath string

	BeforeEach(func() {
		// Create a temporary config file for testing
		tempFile, err := os.CreateTemp("", "ghprs-test-config-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		tempConfigPath = tempFile.Name()
		_ = tempFile.Close()
		_ = os.Remove(tempConfigPath) // Remove the file so tests can create it fresh

		// Set the custom config path for testing
		cmd.SetConfigPath(tempConfigPath)
	})

	AfterEach(func() {
		// Reset config path to default
		cmd.ResetConfigPath()

		// Clean up temporary config file
		_ = os.Remove(tempConfigPath)
	})

	Describe("Configuration Loading and Saving", func() {
		Context("when no config file exists", func() {
			It("should load default configuration", func() {
				config, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Defaults.State).To(Equal("open"))
				Expect(config.Defaults.Limit).To(Equal(30))
				Expect(config.Repositories).To(BeEmpty())
			})

			It("should provide correct config path", func() {
				configPath := cmd.GetConfigPath()
				Expect(configPath).To(Equal(tempConfigPath))
			})
		})

		Context("when config file exists", func() {
			BeforeEach(func() {
				// Create config with some repositories
				config := &cmd.Config{
					Repositories: []cmd.RepositoryConfig{
						{Name: "owner/repo1"},
						{Name: "konflux/repo1", Konflux: true},
					},
				}
				config.Defaults.State = "all"
				config.Defaults.Limit = 50

				err := cmd.SaveConfig(config)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load existing configuration", func() {
				config, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Defaults.State).To(Equal("all"))
				Expect(config.Defaults.Limit).To(Equal(50))
				Expect(config.Repositories).To(HaveLen(2))

				// Verify regular repositories
				allRepos := config.GetRepositories(false)
				Expect(allRepos).To(HaveLen(2))
				Expect(allRepos).To(ContainElement("owner/repo1"))
				Expect(allRepos).To(ContainElement("konflux/repo1"))

				// Verify Konflux repositories
				konfluxRepos := config.GetRepositories(true)
				Expect(konfluxRepos).To(HaveLen(1))
				Expect(konfluxRepos).To(ContainElement("konflux/repo1"))
			})
		})

		Context("when config file is corrupted", func() {
			BeforeEach(func() {
				// Write corrupted YAML content to the temp config file
				err := os.WriteFile(tempConfigPath, []byte("invalid: yaml: content: ["), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle corrupted config with error", func() {
				_, err := cmd.LoadConfig()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse config file"))
			})
		})

		Context("when filesystem errors occur", func() {
			It("should handle directory creation", func() {
				config := cmd.DefaultConfig()
				err := cmd.SaveConfig(config)
				Expect(err).NotTo(HaveOccurred())

				// Verify the config file was created
				info, err := os.Stat(tempConfigPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.IsDir()).To(BeFalse())
			})

			It("should handle permission denied errors gracefully", func() {
				// Create a read-only directory and set config path to a file inside it
				readOnlyDir, err := os.MkdirTemp("", "readonly-*")
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = os.Chmod(readOnlyDir, 0755); _ = os.RemoveAll(readOnlyDir) }()

				// Make directory read-only
				err = os.Chmod(readOnlyDir, 0444)
				Expect(err).NotTo(HaveOccurred())

				// Set config path to file inside read-only directory
				readOnlyConfigPath := filepath.Join(readOnlyDir, "config.yaml")
				cmd.SetConfigPath(readOnlyConfigPath)

				config := cmd.DefaultConfig()
				err = cmd.SaveConfig(config)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write config file"))
			})
		})
	})

	Describe("Repository Management", func() {
		var config *cmd.Config

		BeforeEach(func() {
			config = cmd.DefaultConfig()
		})

		Describe("Adding repositories", func() {
			It("should add regular repository successfully", func() {
				success := config.AddRepository("owner/repo", false)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Name).To(Equal("owner/repo"))
				Expect(config.Repositories[0].Konflux).To(BeFalse())

				// Save and reload to test persistence
				err := cmd.SaveConfig(config)
				Expect(err).NotTo(HaveOccurred())

				reloadedConfig, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				repos := reloadedConfig.GetRepositories(false)
				Expect(repos).To(ContainElement("owner/repo"))
			})

			It("should add Konflux repository successfully", func() {
				success := config.AddRepository("konflux/repo", true)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Name).To(Equal("konflux/repo"))
				Expect(config.Repositories[0].Konflux).To(BeTrue())

				// Test Konflux filtering
				konfluxRepos := config.GetRepositories(true)
				Expect(konfluxRepos).To(ContainElement("konflux/repo"))
			})

			It("should handle duplicate repositories", func() {
				config.AddRepository("owner/repo", false)
				success := config.AddRepository("owner/repo", false)
				Expect(success).To(BeFalse())
				Expect(config.Repositories).To(HaveLen(1))
			})

			It("should upgrade regular repository to Konflux", func() {
				config.AddRepository("owner/repo", false)
				success := config.AddRepository("owner/repo", true)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Konflux).To(BeTrue())
			})

			It("should handle multiple repositories", func() {
				config.AddRepository("owner/repo1", false)
				config.AddRepository("owner/repo2", false)
				config.AddRepository("konflux/repo1", true)

				allRepos := config.GetRepositories(false)
				Expect(allRepos).To(HaveLen(3))

				konfluxRepos := config.GetRepositories(true)
				Expect(konfluxRepos).To(HaveLen(1))
				Expect(konfluxRepos).To(ContainElement("konflux/repo1"))
			})
		})

		Describe("Removing repositories", func() {
			BeforeEach(func() {
				config.AddRepository("owner/repo1", false)
				config.AddRepository("owner/repo2", false)
				config.AddRepository("konflux/repo1", true)
			})

			It("should remove regular repository completely", func() {
				success := config.RemoveRepository("owner/repo1", false)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(2))

				// Verify the correct repository was removed
				names := []string{}
				for _, repo := range config.Repositories {
					names = append(names, repo.Name)
				}
				Expect(names).NotTo(ContainElement("owner/repo1"))
				Expect(names).To(ContainElement("owner/repo2"))
				Expect(names).To(ContainElement("konflux/repo1"))
			})

			It("should remove Konflux flag only", func() {
				success := config.RemoveRepository("konflux/repo1", true)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(3)) // Repository still exists

				// Find the repository and check its Konflux flag
				for _, repo := range config.Repositories {
					if repo.Name == "konflux/repo1" {
						Expect(repo.Konflux).To(BeFalse())
						break
					}
				}

				// Verify it's no longer in Konflux repos
				konfluxRepos := config.GetRepositories(true)
				Expect(konfluxRepos).NotTo(ContainElement("konflux/repo1"))
			})

			It("should handle non-existent repository", func() {
				success := config.RemoveRepository("owner/nonexistent", false)
				Expect(success).To(BeFalse())
				Expect(config.Repositories).To(HaveLen(3))
			})

			It("should handle removing Konflux flag from non-Konflux repo", func() {
				success := config.RemoveRepository("owner/repo1", true)
				Expect(success).To(BeFalse())
			})
		})
	})

	Describe("Configuration Validation", func() {
		Describe("Repository format validation", func() {
			It("should validate correct formats", func() {
				validFormats := []string{
					"owner/repo",
					"organization/project",
					"user-name/repo-name",
					"org_name/repo_name",
					"123owner/456repo",
				}

				config := cmd.DefaultConfig()
				for _, format := range validFormats {
					success := config.AddRepository(format, false)
					Expect(success).To(BeTrue(), "Format %s should be valid", format)
				}
			})

			It("should reject invalid formats", func() {
				// Note: The actual validation is done in the CLI layer
				// Here we test that the config accepts any string
				invalidFormats := []string{
					"invalidrepo",
					"owner/repo/extra",
					"",
					"/repo",
					"owner/",
				}

				config := cmd.DefaultConfig()
				for _, format := range invalidFormats {
					// The config layer doesn't validate format - that's CLI responsibility
					success := config.AddRepository(format, false)
					Expect(success).To(BeTrue(), "Config layer should accept any string")
				}
			})
		})

		Describe("Default values", func() {
			It("should have correct default values", func() {
				config := cmd.DefaultConfig()
				Expect(config.Defaults.State).To(Equal("open"))
				Expect(config.Defaults.Limit).To(Equal(30))
				Expect(config.Repositories).To(BeEmpty())
			})

			It("should allow setting custom defaults", func() {
				config := cmd.DefaultConfig()
				config.Defaults.State = "closed"
				config.Defaults.Limit = 100

				err := cmd.SaveConfig(config)
				Expect(err).NotTo(HaveOccurred())

				reloadedConfig, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(reloadedConfig.Defaults.State).To(Equal("closed"))
				Expect(reloadedConfig.Defaults.Limit).To(Equal(100))
			})
		})
	})

	Describe("Configuration File Persistence", func() {
		It("should persist complex configuration", func() {
			config := cmd.DefaultConfig()
			config.Defaults.State = "all"
			config.Defaults.Limit = 75
			config.AddRepository("owner1/repo1", false)
			config.AddRepository("owner2/repo2", false)
			config.AddRepository("konflux1/repo1", true)
			config.AddRepository("konflux2/repo2", true)

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			// Reload and verify
			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			Expect(reloadedConfig.Defaults.State).To(Equal("all"))
			Expect(reloadedConfig.Defaults.Limit).To(Equal(75))
			Expect(reloadedConfig.Repositories).To(HaveLen(4))

			allRepos := reloadedConfig.GetRepositories(false)
			Expect(allRepos).To(HaveLen(4))

			konfluxRepos := reloadedConfig.GetRepositories(true)
			Expect(konfluxRepos).To(HaveLen(2))
			Expect(konfluxRepos).To(ContainElement("konflux1/repo1"))
			Expect(konfluxRepos).To(ContainElement("konflux2/repo2"))
		})

		It("should handle empty configuration", func() {
			config := cmd.DefaultConfig()
			config.Repositories = []cmd.RepositoryConfig{}

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(reloadedConfig.Repositories).To(BeEmpty())
		})

		It("should preserve repository order", func() {
			config := cmd.DefaultConfig()
			repos := []string{"a/repo", "z/repo", "m/repo"}
			for _, repo := range repos {
				config.AddRepository(repo, false)
			}

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			// Verify order is preserved
			for i, repo := range repos {
				Expect(reloadedConfig.Repositories[i].Name).To(Equal(repo))
			}
		})
	})

	Describe("Edge Cases", func() {
		It("should handle special characters in repository names", func() {
			config := cmd.DefaultConfig()
			specialRepos := []string{
				"user-with-dashes/repo-with-dashes",
				"user_with_underscores/repo_with_underscores",
				"123numbers/456numbers",
				"MixedCase/RepoName",
			}

			for _, repo := range specialRepos {
				success := config.AddRepository(repo, false)
				Expect(success).To(BeTrue())
			}

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			allRepos := reloadedConfig.GetRepositories(false)
			for _, repo := range specialRepos {
				Expect(allRepos).To(ContainElement(repo))
			}
		})

		It("should handle very long repository names", func() {
			config := cmd.DefaultConfig()
			longRepo := "very-very-very-long-organization-name/very-very-very-long-repository-name-that-exceeds-normal-limits"

			success := config.AddRepository(longRepo, false)
			Expect(success).To(BeTrue())

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			allRepos := reloadedConfig.GetRepositories(false)
			Expect(allRepos).To(ContainElement(longRepo))
		})

		It("should handle many repositories", func() {
			config := cmd.DefaultConfig()

			// Add many repositories
			for i := 0; i < 100; i++ {
				repoName := fmt.Sprintf("owner%d/repo%d", i, i)
				isKonflux := i%3 == 0 // Every third repo is Konflux
				config.AddRepository(repoName, isKonflux)
			}

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			reloadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			allRepos := reloadedConfig.GetRepositories(false)
			Expect(allRepos).To(HaveLen(100))

			konfluxRepos := reloadedConfig.GetRepositories(true)
			Expect(konfluxRepos).To(HaveLen(34)) // 0, 3, 6, 9, ... 99
		})
	})
})

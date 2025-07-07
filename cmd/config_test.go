package cmd_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Configuration", func() {
	var tempDir string
	var originalHome string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "ghprs-test")
		Expect(err).NotTo(HaveOccurred())

		originalHome = os.Getenv("HOME")
		_ = os.Setenv("HOME", tempDir)
	})

	AfterEach(func() {
		_ = os.Setenv("HOME", originalHome)
		_ = os.RemoveAll(tempDir)
	})

	Describe("DefaultConfig", func() {
		It("should create a config with proper defaults", func() {
			config := cmd.DefaultConfig()
			Expect(config.Defaults.State).To(Equal("open"))
			Expect(config.Defaults.Limit).To(Equal(30))
			Expect(config.Repositories).To(BeEmpty())
		})
	})

	Describe("LoadConfig", func() {
		Context("when config file doesn't exist", func() {
			It("should return default config", func() {
				config, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Defaults.State).To(Equal("open"))
				Expect(config.Defaults.Limit).To(Equal(30))
			})
		})

		Context("when config file exists", func() {
			BeforeEach(func() {
				configDir := filepath.Join(tempDir, ".config", "ghprs")
				err := os.MkdirAll(configDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				configContent := `repositories:
  - name: owner/repo1
  - name: owner/repo2
  - name: konflux/repo1
    konflux: true
  - name: konflux/repo2
    konflux: true
defaults:
  state: all
  limit: 50`

				configFile := filepath.Join(configDir, "config.yaml")
				err = os.WriteFile(configFile, []byte(configContent), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load the custom config", func() {
				config, err := cmd.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(config.Defaults.State).To(Equal("all"))
				Expect(config.Defaults.Limit).To(Equal(50))
				Expect(config.Repositories).To(HaveLen(4))

				// Check regular repositories
				regularRepos := config.GetRepositories(false)
				Expect(regularRepos).To(HaveLen(4))
				Expect(regularRepos).To(ContainElement("owner/repo1"))
				Expect(regularRepos).To(ContainElement("owner/repo2"))
				Expect(regularRepos).To(ContainElement("konflux/repo1"))
				Expect(regularRepos).To(ContainElement("konflux/repo2"))

				// Check Konflux repositories
				konfluxRepos := config.GetRepositories(true)
				Expect(konfluxRepos).To(HaveLen(2))
				Expect(konfluxRepos).To(ContainElement("konflux/repo1"))
				Expect(konfluxRepos).To(ContainElement("konflux/repo2"))
			})
		})

		Context("when config file is malformed", func() {
			BeforeEach(func() {
				configDir := filepath.Join(tempDir, ".config", "ghprs")
				err := os.MkdirAll(configDir, 0755)
				Expect(err).NotTo(HaveOccurred())

				configFile := filepath.Join(configDir, "config.yaml")
				err = os.WriteFile(configFile, []byte("invalid: yaml: content: ["), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error", func() {
				_, err := cmd.LoadConfig()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse config file"))
			})
		})
	})

	Describe("SaveConfig", func() {
		It("should save config to the correct location", func() {
			config := &cmd.Config{
				Repositories: []cmd.RepositoryConfig{
					{Name: "test/repo"},
				},
			}
			config.Defaults.State = "closed"
			config.Defaults.Limit = 100

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			// Verify the file was created
			configFile := filepath.Join(tempDir, ".config", "ghprs", "config.yaml")
			_, err = os.Stat(configFile)
			Expect(err).NotTo(HaveOccurred())

			// Verify the content by loading it back
			loadedConfig, err := cmd.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(loadedConfig.Defaults.State).To(Equal("closed"))
			Expect(loadedConfig.Defaults.Limit).To(Equal(100))
			Expect(loadedConfig.Repositories).To(HaveLen(1))
			Expect(loadedConfig.Repositories[0].Name).To(Equal("test/repo"))
		})

		It("should create directories if they don't exist", func() {
			config := cmd.DefaultConfig()

			err := cmd.SaveConfig(config)
			Expect(err).NotTo(HaveOccurred())

			// Verify the directory was created
			configDir := filepath.Join(tempDir, ".config", "ghprs")
			info, err := os.Stat(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})
	})

	Describe("GetConfigPath", func() {
		It("should return the correct config path", func() {
			path := cmd.GetConfigPath()
			expectedPath := filepath.Join(tempDir, ".config", "ghprs", "config.yaml")
			Expect(path).To(Equal(expectedPath))
		})
	})

	Describe("Config Structure", func() {
		It("should have the correct structure", func() {
			config := cmd.DefaultConfig()

			// Test that we can modify the config
			config.Repositories = append(config.Repositories, cmd.RepositoryConfig{Name: "owner/repo"})
			config.Repositories = append(config.Repositories, cmd.RepositoryConfig{Name: "konflux/repo", Konflux: true})
			config.Defaults.State = "closed"
			config.Defaults.Limit = 50

			Expect(config.Repositories).To(HaveLen(2))
			Expect(config.Repositories[0].Name).To(Equal("owner/repo"))
			Expect(config.Repositories[1].Name).To(Equal("konflux/repo"))
			Expect(config.Repositories[1].Konflux).To(BeTrue())
			Expect(config.Defaults.State).To(Equal("closed"))
			Expect(config.Defaults.Limit).To(Equal(50))
		})
	})

	Describe("Repository Management", func() {
		var config *cmd.Config

		BeforeEach(func() {
			config = cmd.DefaultConfig()
		})

		Describe("GetRepositories", func() {
			It("should return all repositories when isKonflux is false", func() {
				config.Repositories = []cmd.RepositoryConfig{
					{Name: "owner/repo1"},
					{Name: "owner/repo2"},
					{Name: "konflux/repo1", Konflux: true},
				}

				repos := config.GetRepositories(false)
				Expect(repos).To(HaveLen(3))
				Expect(repos).To(ContainElement("owner/repo1"))
				Expect(repos).To(ContainElement("owner/repo2"))
				Expect(repos).To(ContainElement("konflux/repo1"))
			})

			It("should return only Konflux repositories when isKonflux is true", func() {
				config.Repositories = []cmd.RepositoryConfig{
					{Name: "owner/repo1"},
					{Name: "owner/repo2"},
					{Name: "konflux/repo1", Konflux: true},
				}

				repos := config.GetRepositories(true)
				Expect(repos).To(HaveLen(1))
				Expect(repos).To(ContainElement("konflux/repo1"))
			})
		})

		Describe("AddRepository", func() {
			It("should add repository as regular when isKonflux is false", func() {
				success := config.AddRepository("owner/repo", false)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Name).To(Equal("owner/repo"))
				Expect(config.Repositories[0].Konflux).To(BeFalse())
			})

			It("should add repository as Konflux when isKonflux is true", func() {
				success := config.AddRepository("konflux/repo", true)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Name).To(Equal("konflux/repo"))
				Expect(config.Repositories[0].Konflux).To(BeTrue())
			})

			It("should not add duplicate repository", func() {
				config.Repositories = []cmd.RepositoryConfig{
					{Name: "owner/repo"},
				}
				success := config.AddRepository("owner/repo", false)
				Expect(success).To(BeFalse())
				Expect(config.Repositories).To(HaveLen(1))
			})

			It("should upgrade regular repository to Konflux", func() {
				config.Repositories = []cmd.RepositoryConfig{
					{Name: "owner/repo"},
				}
				success := config.AddRepository("owner/repo", true)
				Expect(success).To(BeTrue())
				Expect(config.Repositories).To(HaveLen(1))
				Expect(config.Repositories[0].Konflux).To(BeTrue())
			})
		})

		Describe("RemoveRepository", func() {
			BeforeEach(func() {
				config.Repositories = []cmd.RepositoryConfig{
					{Name: "owner/repo1"},
					{Name: "owner/repo2"},
					{Name: "konflux/repo1", Konflux: true},
				}
			})

			It("should remove repository entirely when isKonflux is false", func() {
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
			})

			It("should remove Konflux flag when isKonflux is true", func() {
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
			})

			It("should return false when removing non-existent repository", func() {
				success := config.RemoveRepository("owner/nonexistent", false)
				Expect(success).To(BeFalse())
				Expect(config.Repositories).To(HaveLen(3))
			})
		})
	})
})

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
  - owner/repo1
  - owner/repo2
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
				Expect(config.Repositories).To(HaveLen(2))
				Expect(config.Repositories).To(ContainElement("owner/repo1"))
				Expect(config.Repositories).To(ContainElement("owner/repo2"))
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
				Repositories: []string{"test/repo"},
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
			Expect(loadedConfig.Repositories).To(ContainElement("test/repo"))
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
			config.Repositories = append(config.Repositories, "owner/repo")
			config.Defaults.State = "closed"
			config.Defaults.Limit = 50

			Expect(config.Repositories).To(ContainElement("owner/repo"))
			Expect(config.Defaults.State).To(Equal("closed"))
			Expect(config.Defaults.Limit).To(Equal(50))
		})
	})
})

package cmd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Integration Tests", func() {

	var mockClient *cmd.MockRESTClient
	var owner, repo string
	var tempDir string

	BeforeEach(func() {
		mockClient = cmd.NewMockRESTClient()
		owner = "testowner"
		repo = "testrepo"

		// Create temporary directory for config tests
		var err error
		tempDir, err = os.MkdirTemp("", "ghprs-integration-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Clean up temporary directory
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("End-to-End PR Listing Workflow", func() {
		It("should handle complete PR listing with filtering and display", func() {
			// Setup comprehensive mock data
			prs := cmd.CreateMockPullRequests(10)

			// Make some PRs match specific criteria
			prs[0].Title = "SECURITY: Fix CVE-2023-1234"
			prs[1].Body = "⚠️[migration] Database schema change"
			prs[2].Labels = []cmd.Label{{Name: "do-not-merge/hold"}}
			prs[3].Labels = []cmd.Label{{Name: "konflux-nudge"}}
			prs[4].User.Login = "red-hat-konflux[bot]"
			prs[5].Base.Ref = "release/v2.0"

			mockClient.AddResponse("pulls", 200, prs)

			// Mock file responses for Tekton checking
			for _, pr := range prs {
				if pr.Number%2 == 0 {
					// Even PRs have only Tekton files
					files := cmd.CreateMockPRFiles(true)
					mockClient.AddResponse(fmt.Sprintf("pulls/%d/files", pr.Number), 200, files)
				} else {
					// Odd PRs have mixed files
					files := cmd.CreateMockPRFiles(false)
					mockClient.AddResponse(fmt.Sprintf("pulls/%d/files", pr.Number), 200, files)
				}

				// Mock reviews
				reviews := cmd.CreateMockReviews(pr.Number%3 == 0) // Every 3rd PR approved
				mockClient.AddResponse(fmt.Sprintf("pulls/%d/reviews", pr.Number), 200, reviews)

				// Mock PR details for cache testing
				fullPR := pr
				fullPR.MergeableState = []string{"clean", "dirty", "blocked"}[pr.Number%3]
				mockClient.AddResponse(fmt.Sprintf("pulls/%d", pr.Number), 200, fullPR)
			}

			// Test filtering functionality
			filteredPRs := cmd.FilterPRsTest(prs, mockClient, owner, repo, false)
			Expect(len(filteredPRs)).To(Equal(len(prs))) // No filters applied
		})

		It("should handle security-only filtering", func() {
			prs := cmd.CreateMockPullRequests(5)
			prs[0].Title = "SECURITY: Fix vulnerability"
			prs[1].Title = "Update CVE-2023-1234"
			prs[2].Title = "Regular feature update"

			mockClient.AddResponse("pulls", 200, prs)

			securityPRs := []cmd.PullRequest{}
			for _, pr := range prs {
				if cmd.HasSecurityTest(pr) {
					securityPRs = append(securityPRs, pr)
				}
			}

			Expect(len(securityPRs)).To(Equal(2)) // Only security PRs
			Expect(securityPRs[0].Title).To(ContainSubstring("SECURITY"))
			Expect(securityPRs[1].Title).To(ContainSubstring("CVE"))
		})

		It("should handle migration warning filtering", func() {
			prs := cmd.CreateMockPullRequests(5)
			prs[0].Body = "⚠️[migration] Database changes"
			prs[1].Body = "[migration] Schema update"
			prs[2].Body = "Regular update"

			migrationPRs := []cmd.PullRequest{}
			for _, pr := range prs {
				if cmd.HasMigrationWarningTest(pr) {
					migrationPRs = append(migrationPRs, pr)
				}
			}

			Expect(len(migrationPRs)).To(Equal(2)) // Only migration PRs
		})
	})

	Describe("Configuration Integration", func() {
		It("should create and use configuration correctly", func() {
			configPath := filepath.Join(tempDir, "config.yaml")

			// Create a test config
			config := cmd.Config{
				Defaults: struct {
					State string `yaml:"state"`
					Limit int    `yaml:"limit"`
				}{
					State: "all",
					Limit: 50,
				},
				Repositories: []cmd.RepositoryConfig{
					{Name: "owner1/repo1", Konflux: false},
					{Name: "owner2/repo2", Konflux: false},
					{Name: "konflux/repo1", Konflux: true},
				},
			}

			// Save config to temp file
			err := cmd.SaveConfigTest(config, configPath)
			Expect(err).NotTo(HaveOccurred())

			// Load config back
			loadedConfig, err := cmd.LoadConfigTest(configPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(loadedConfig.Defaults.State).To(Equal("all"))
			Expect(loadedConfig.Defaults.Limit).To(Equal(50))
			Expect(len(loadedConfig.Repositories)).To(Equal(3))

			// Check Konflux repositories
			konfluxRepos := 0
			for _, repo := range loadedConfig.Repositories {
				if repo.Konflux {
					konfluxRepos++
				}
			}
			Expect(konfluxRepos).To(Equal(1))
		})

		It("should handle missing config gracefully", func() {
			missingPath := filepath.Join(tempDir, "nonexistent.yaml")

			config, err := cmd.LoadConfigTest(missingPath)
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())

			// Should fall back to default config
			defaultConfig := cmd.DefaultConfig()
			Expect(defaultConfig.Defaults.State).To(Equal("open"))
			Expect(defaultConfig.Defaults.Limit).To(Equal(30))
		})
	})

	Describe("Sorting Integration", func() {
		It("should sort PRs correctly by different criteria", func() {
			prs := []cmd.PullRequest{
				{Number: 3, Title: "Third", CreatedAt: "2023-01-03T10:00:00Z", UpdatedAt: "2023-01-05T10:00:00Z"},
				{Number: 1, Title: "First", CreatedAt: "2023-01-01T10:00:00Z", UpdatedAt: "2023-01-02T10:00:00Z"},
				{Number: 2, Title: "Second", CreatedAt: "2023-01-02T10:00:00Z", UpdatedAt: "2023-01-06T10:00:00Z"},
			}

			// Test number sorting
			numberSorted := make([]cmd.PullRequest, len(prs))
			copy(numberSorted, prs)
			cmd.SortPullRequestsTest(numberSorted, "number")
			Expect(numberSorted[0].Number).To(Equal(1))
			Expect(numberSorted[1].Number).To(Equal(2))
			Expect(numberSorted[2].Number).To(Equal(3))

			// Test oldest sorting
			oldestSorted := make([]cmd.PullRequest, len(prs))
			copy(oldestSorted, prs)
			cmd.SortPullRequestsTest(oldestSorted, "oldest")
			Expect(oldestSorted[0].CreatedAt).To(Equal("2023-01-01T10:00:00Z"))

			// Test updated sorting
			updatedSorted := make([]cmd.PullRequest, len(prs))
			copy(updatedSorted, prs)
			cmd.SortPullRequestsTest(updatedSorted, "updated")
			Expect(updatedSorted[0].UpdatedAt).To(Equal("2023-01-06T10:00:00Z")) // Most recent first
		})

		It("should handle priority sorting with security and migration", func() {
			prs := []cmd.PullRequest{
				{Number: 1, Title: "Regular update", CreatedAt: "2023-01-01T10:00:00Z"},
				{Number: 2, Title: "SECURITY fix", CreatedAt: "2023-01-02T10:00:00Z"},
				{Number: 3, Body: "⚠️[migration] changes", CreatedAt: "2023-01-03T10:00:00Z"},
				{Number: 4, Title: "CVE fix", Body: "⚠️[migration] also", CreatedAt: "2023-01-04T10:00:00Z"},
			}

			cmd.SortPullRequestsTest(prs, "priority")

			// Security should come first, then migration
			Expect(prs[0].Title).To(ContainSubstring("CVE"))      // Security + migration
			Expect(prs[1].Title).To(ContainSubstring("SECURITY")) // Security only
		})
	})

	Describe("Caching Integration", func() {
		It("should cache PR details across multiple operations", func() {
			cache := cmd.NewPRDetailsCacheTest()

			originalPR := cmd.PullRequest{
				Number:         1,
				MergeableState: "unknown",
			}

			freshPR := cmd.PullRequest{
				Number:         1,
				MergeableState: "clean",
			}
			mockClient.AddResponse("pulls/1", 200, freshPR)

			// First call should fetch from API
			result1 := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
			Expect(result1.MergeableState).To(Equal("clean"))
			Expect(mockClient.GetRequestCount("pulls")).To(Equal(1))

			// Second call should use cache
			result2 := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
			Expect(result2.MergeableState).To(Equal("clean"))
			Expect(mockClient.GetRequestCount("pulls")).To(Equal(1)) // Still 1, cached

			// Test rebase checking with cache
			needsRebase1, _ := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
			needsRebase2, _ := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
			Expect(needsRebase1).To(Equal(needsRebase2))
			Expect(mockClient.GetRequestCount("pulls")).To(Equal(1)) // Still cached
		})
	})

	Describe("Error Handling Integration", func() {
		It("should handle API failures gracefully across operations", func() {
			prs := cmd.CreateMockPullRequests(3)
			mockClient.AddResponse("pulls", 200, prs)

			// Mock API failures for detailed operations with full URL patterns
			mockClient.AddErrorResponse(fmt.Sprintf("repos/%s/%s/pulls/1", owner, repo), fmt.Errorf("API rate limit exceeded"))
			mockClient.AddErrorResponse(fmt.Sprintf("repos/%s/%s/pulls/1/files", owner, repo), fmt.Errorf("Not found"))
			mockClient.AddErrorResponse(fmt.Sprintf("repos/%s/%s/pulls/1/reviews", owner, repo), fmt.Errorf("Permission denied"))

			// Operations should not panic and handle errors gracefully
			cache := cmd.NewPRDetailsCacheTest()

			// Fetch PR details should handle error
			result, err := cmd.FetchPRDetailsTest(mockClient, owner, repo, 1)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())

			// Cache should handle API errors
			originalPR := prs[0]
			originalPR.MergeableState = "unknown"
			cachedResult := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
			Expect(cachedResult.MergeableState).To(Equal("unknown")) // Falls back to original

			// Review checking should handle error
			isReviewed := cmd.IsReviewedTest(mockClient, owner, repo, 1, []cmd.Label{})
			Expect(isReviewed).To(BeFalse()) // Defaults to false on error

			// Tekton file checking should handle error
			onlyTekton, files, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).To(HaveOccurred())
			Expect(onlyTekton).To(BeFalse())
			Expect(files).To(BeNil())
		})

		It("should handle malformed API responses", func() {
			// Test with malformed JSON responses
			invalidResponse := "invalid json {"
			mockClient.AddResponse("pulls", 200, invalidResponse)

			// Should handle gracefully without panicking
			Expect(func() {
				var prs []cmd.PullRequest
				_ = mockClient.Get("repos/owner/repo/pulls", &prs)
			}).NotTo(Panic())
		})
	})

	Describe("String Processing Integration", func() {
		It("should handle complex string processing workflows", func() {
			// Test with various Unicode, ANSI, and special characters
			testStrings := []string{
				"Simple ASCII text",
				"Unicode: 世界 🌟 αβγ",
				"\033[31mANSI colored\033[0m text",
				"Mixed \033[1mformatted\033[0m 🚀 unicode",
				strings.Repeat("Very long string ", 100),
				"",
				"   \t\n   ", // Whitespace only
			}

			for _, str := range testStrings {
				// Full processing pipeline
				stripped := cmd.StripANSISequencesTest(str)
				width := cmd.DisplayWidthTest(stripped)
				truncated := cmd.TruncateStringTest(stripped, 50)
				padded := cmd.PadStringTest(truncated, 60)

				// Verify integrity
				Expect(func() { _ = stripped }).NotTo(Panic())
				Expect(width).To(BeNumerically(">=", 0))
				Expect(cmd.DisplayWidthTest(truncated)).To(BeNumerically("<=", 50))
				Expect(len(padded)).To(BeNumerically(">=", len(truncated)))
			}
		})
	})

	Describe("Mock Client Integration", func() {
		It("should handle complex request/response patterns", func() {
			// Setup complex response patterns
			cmd.SetupMockResponses(mockClient, owner, repo)

			// Test multiple concurrent-style operations
			for i := 1; i <= 5; i++ {
				// Simulate checking multiple PRs
				var pr cmd.PullRequest
				err := mockClient.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, i), &pr)
				Expect(err).NotTo(HaveOccurred())
				Expect(pr.Number).To(Equal(i))

				// Check files
				var files []cmd.PRFile
				err = mockClient.Get(fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, i), &files)
				Expect(err).NotTo(HaveOccurred())

				// Check reviews
				var reviews []cmd.Review
				err = mockClient.Get(fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, i), &reviews)
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify request tracking
			Expect(mockClient.GetRequestCount("pulls")).To(BeNumerically(">=", 5))
			Expect(mockClient.GetRequestCount("files")).To(Equal(5))
			Expect(mockClient.GetRequestCount("reviews")).To(Equal(5))
		})

		It("should handle request history and patterns", func() {
			// Make various requests
			_ = mockClient.Get("repos/owner/repo/pulls", nil)
			_ = mockClient.Post("repos/owner/repo/issues/1/comments", bytes.NewReader([]byte("test")), nil)
			_ = mockClient.Get("repos/owner/repo/pulls/1/files", nil)

			// Verify request history
			Expect(len(mockClient.Requests)).To(Equal(3))
			Expect(mockClient.Requests[0].Method).To(Equal("GET"))
			Expect(mockClient.Requests[1].Method).To(Equal("POST"))
			Expect(mockClient.Requests[1].Body).To(Equal("test"))

			lastRequest := mockClient.GetLastRequest()
			Expect(lastRequest).NotTo(BeNil())
			Expect(lastRequest.Method).To(Equal("GET"))
			Expect(lastRequest.URL).To(ContainSubstring("files"))

			// Test clearing requests
			mockClient.ClearRequests()
			Expect(len(mockClient.Requests)).To(Equal(0))
		})
	})

	Describe("Performance Integration", func() {
		It("should handle moderately large datasets efficiently", func() {
			// Test with 100 PRs
			largePRList := cmd.CreateMockPullRequests(100)

			// Should handle sorting without issues
			Expect(func() {
				cmd.SortPullRequestsTest(largePRList, "number")
			}).NotTo(Panic())

			// Verify sorting worked
			Expect(largePRList[0].Number).To(Equal(1))
			Expect(largePRList[99].Number).To(Equal(100))

			// Test string processing on large text
			largeTitle := strings.Repeat("Long PR title with Unicode 🚀 ", 50)
			processed := cmd.TruncateStringTest(largeTitle, 100)
			Expect(cmd.DisplayWidthTest(processed)).To(BeNumerically("<=", 100))
		})

		It("should handle rapid cache operations", func() {
			cache := cmd.NewPRDetailsCacheTest()

			// Simulate rapid cache access
			for i := 1; i <= 50; i++ {
				pr := cmd.PullRequest{
					Number:         i,
					MergeableState: "clean",
				}

				// Should handle rapid access without issues
				result := cache.GetOrFetchTest(mockClient, owner, repo, i, pr)
				Expect(result.Number).To(Equal(i))
			}
		})
	})
})

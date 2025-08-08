package cmd_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("API-Dependent Functions (Previously Skipped)", func() {

	var mockClient *cmd.MockRESTClient
	var owner, repo string

	BeforeEach(func() {
		mockClient = cmd.NewMockRESTClient()
		owner = "testowner"
		repo = "testrepo"
	})

	Describe("isReviewed Function", func() {
		It("should detect approved reviews", func() {
			// Mock approved reviews
			reviews := cmd.CreateMockReviews(true)
			mockClient.AddResponse("reviews", 200, reviews)

			labels := []cmd.Label{}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeTrue())
		})

		It("should detect not approved reviews", func() {
			// Mock non-approved reviews
			reviews := cmd.CreateMockReviews(false)
			mockClient.AddResponse("reviews", 200, reviews)

			labels := []cmd.Label{}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeFalse())
		})

		It("should detect approved via labels when API fails", func() {
			// Mock API error
			mockClient.AddErrorResponse("reviews", fmt.Errorf("API error"))

			// But has approved label
			labels := []cmd.Label{{Name: "approved"}}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeTrue())
		})

		It("should detect lgtm via labels", func() {
			// Mock API error
			mockClient.AddErrorResponse("reviews", fmt.Errorf("API error"))

			// But has lgtm label
			labels := []cmd.Label{{Name: "lgtm"}}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeTrue())
		})

		It("should return false when API fails and no approved labels", func() {
			// Mock API error
			mockClient.AddErrorResponse("reviews", fmt.Errorf("API error"))

			// No approved labels
			labels := []cmd.Label{{Name: "bug"}}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeFalse())
		})

		It("should handle empty reviews", func() {
			// Mock empty reviews
			mockClient.AddResponse("reviews", 200, []cmd.Review{})

			labels := []cmd.Label{}
			result := cmd.IsReviewedTest(mockClient, owner, repo, 1, labels)
			Expect(result).To(BeFalse())
		})
	})

	Describe("fetchPRDetails Function", func() {
		It("should fetch PR details successfully", func() {
			pr := cmd.PullRequest{
				Number:         1,
				Title:          "Test PR",
				MergeableState: "clean",
			}
			mockClient.AddResponse("pulls/1", 200, pr)

			result, err := cmd.FetchPRDetailsTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Number).To(Equal(1))
			Expect(result.Title).To(Equal("Test PR"))
			Expect(result.MergeableState).To(Equal("clean"))
		})

		It("should handle API errors", func() {
			mockClient.AddErrorResponse("pulls/1", fmt.Errorf("Not found"))

			result, err := cmd.FetchPRDetailsTest(mockClient, owner, repo, 1)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

		It("should handle different mergeable states", func() {
			states := []string{"clean", "dirty", "blocked", "behind", "unstable", "unknown"}

			for _, state := range states {
				pr := cmd.PullRequest{
					Number:         1,
					MergeableState: state,
				}
				mockClient.AddResponse(fmt.Sprintf("pulls/%d", 1), 200, pr)

				result, err := cmd.FetchPRDetailsTest(mockClient, owner, repo, 1)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.MergeableState).To(Equal(state))

				// Clear responses for next iteration
				mockClient = cmd.NewMockRESTClient()
			}
		})
	})

	Describe("checkTektonFilesDetailed Function", func() {
		It("should detect PRs with only Tekton files", func() {
			files := cmd.CreateMockPRFiles(true) // Only Tekton files
			mockClient.AddResponse("files", 200, files)

			onlyTekton, tektonFiles, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(onlyTekton).To(BeTrue())
			Expect(tektonFiles).NotTo(BeEmpty())
		})

		It("should detect PRs with mixed files", func() {
			files := cmd.CreateMockPRFiles(false) // Mixed files
			mockClient.AddResponse("files", 200, files)

			onlyTekton, _, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(onlyTekton).To(BeFalse())
			// May still have some Tekton files, but not exclusively
		})

		It("should handle API errors", func() {
			mockClient.AddErrorResponse("files", fmt.Errorf("API error"))

			onlyTekton, tektonFiles, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).To(HaveOccurred())
			Expect(onlyTekton).To(BeFalse())
			Expect(tektonFiles).To(BeNil())
		})

		It("should handle PRs with no files", func() {
			files := []cmd.PRFile{} // No files
			mockClient.AddResponse("files", 200, files)

			onlyTekton, tektonFiles, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(onlyTekton).To(BeFalse())
			Expect(tektonFiles).To(BeEmpty())
		})

		It("should correctly identify Tekton file patterns", func() {
			// Test specific Tekton file patterns
			tektonFiles := []cmd.PRFile{
				{Filename: ".tekton/pipeline-pull-request.yaml", Status: "modified"},
				{Filename: ".tekton/pipeline-push.yaml", Status: "added"},
				{Filename: ".tekton/custom-pull-request.yaml", Status: "modified"},
				{Filename: ".tekton/build-push.yaml", Status: "added"},
			}
			mockClient.AddResponse("files", 200, tektonFiles)

			onlyTekton, foundFiles, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(onlyTekton).To(BeTrue())
			Expect(foundFiles).To(HaveLen(4))
		})

		It("should reject non-matching Tekton files", func() {
			// Files in .tekton/ but don't match patterns
			nonMatchingFiles := []cmd.PRFile{
				{Filename: ".tekton/config.yaml", Status: "modified"},      // Doesn't match pattern
				{Filename: ".tekton/README.md", Status: "added"},           // Doesn't match pattern
				{Filename: ".tekton/scripts/build.sh", Status: "modified"}, // Doesn't match pattern
			}
			mockClient.AddResponse("files", 200, nonMatchingFiles)

			onlyTekton, foundFiles, err := cmd.CheckTektonFilesDetailedTest(mockClient, owner, repo, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(onlyTekton).To(BeFalse()) // No matching Tekton files
			Expect(foundFiles).To(BeEmpty()) // No files match the pattern
		})
	})

	Describe("Cache Functions", func() {
		var cache *cmd.PRDetailsCache

		BeforeEach(func() {
			cache = cmd.NewPRDetailsCacheTest()
		})

		Describe("needsRebaseWithCache Function", func() {
			It("should use original PR when it has valid mergeable state", func() {
				pr := cmd.PullRequest{
					Number:         1,
					MergeableState: "dirty",
				}

				needsRebase, hasState := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, pr)
				Expect(hasState).To(BeTrue())
				Expect(needsRebase).To(BeTrue())

				// Should not have made any API calls since original PR had valid state
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(0))
			})

			It("should fetch from API when original PR has unknown state", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "unknown",
				}

				// Mock API response with clean state
				freshPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "clean",
				}
				mockClient.AddResponse("pulls/1", 200, freshPR)

				needsRebase, hasState := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
				Expect(hasState).To(BeTrue())
				Expect(needsRebase).To(BeFalse())

				// Should have made API call
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(1))
			})

			It("should handle API errors gracefully", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "",
				}

				mockClient.AddErrorResponse("pulls/1", fmt.Errorf("API error"))

				needsRebase, hasState := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
				Expect(hasState).To(BeFalse())
				Expect(needsRebase).To(BeFalse())
			})

			It("should cache results to avoid duplicate API calls", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "unknown",
				}

				freshPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "dirty",
				}
				mockClient.AddResponse("pulls/1", 200, freshPR)

				// First call should fetch from API
				needsRebase1, hasState1 := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
				Expect(hasState1).To(BeTrue())
				Expect(needsRebase1).To(BeTrue())
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(1))

				// Second call should use cache
				needsRebase2, hasState2 := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, originalPR)
				Expect(hasState2).To(BeTrue())
				Expect(needsRebase2).To(BeTrue())
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(1)) // Still 1, no additional API call
			})
		})

		Describe("isBlockedWithCache Function", func() {
			It("should detect blocked state", func() {
				pr := cmd.PullRequest{
					Number:         1,
					MergeableState: "blocked",
				}

				isBlocked, hasState := cmd.IsBlockedWithCacheTest(cache, mockClient, owner, repo, pr)
				Expect(hasState).To(BeTrue())
				Expect(isBlocked).To(BeTrue())
			})

			It("should detect non-blocked state", func() {
				pr := cmd.PullRequest{
					Number:         1,
					MergeableState: "clean",
				}

				isBlocked, hasState := cmd.IsBlockedWithCacheTest(cache, mockClient, owner, repo, pr)
				Expect(hasState).To(BeTrue())
				Expect(isBlocked).To(BeFalse())
			})

			It("should fetch fresh data when needed", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "",
				}

				freshPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "blocked",
				}
				mockClient.AddResponse("pulls/1", 200, freshPR)

				isBlocked, hasState := cmd.IsBlockedWithCacheTest(cache, mockClient, owner, repo, originalPR)
				Expect(hasState).To(BeTrue())
				Expect(isBlocked).To(BeTrue())
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(1))
			})
		})

		Describe("GetOrFetch Method", func() {
			It("should return original PR when it has valid mergeable state", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "clean",
				}

				result := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
				Expect(result.MergeableState).To(Equal("clean"))

				// Should not have made API call
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(0))
			})

			It("should fetch from API when mergeable state is unknown", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "unknown",
				}

				freshPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "clean",
				}
				mockClient.AddResponse("pulls/1", 200, freshPR)

				result := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
				Expect(result.MergeableState).To(Equal("clean"))

				// Should have made API call
				Expect(mockClient.GetRequestCount("pulls")).To(Equal(1))
			})

			It("should return original PR when API fails", func() {
				originalPR := cmd.PullRequest{
					Number:         1,
					MergeableState: "unknown",
				}

				mockClient.AddErrorResponse("pulls/1", fmt.Errorf("API error"))

				result := cache.GetOrFetchTest(mockClient, owner, repo, 1, originalPR)
				Expect(result.MergeableState).To(Equal("unknown"))
			})
		})
	})
})

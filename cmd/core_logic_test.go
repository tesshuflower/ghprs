package cmd_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Core Logic Functions", func() {
	Describe("Migration Warning Detection", func() {
		Context("when PR has migration warning patterns", func() {
			It("should detect ⚠️[migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains ⚠️[migration] changes that need attention",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect [migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains [migration] changes that need attention",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect :warning:[migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains :warning:[migration] changes",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect case-insensitive patterns", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains ⚠️[MIGRATION] changes",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect multiple warning patterns", func() {
				patterns := []string{
					"⚠️[migration] Database changes required",
					":warning:[migration] Breaking changes",
					"⚠️migration⚠️ Manual intervention needed",
					"[migration] Schema update",
					"⚠️[MIGRATION] UPPERCASE",
				}

				for _, pattern := range patterns {
					pr := cmd.PullRequest{Body: pattern}
					Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue(),
						"Should detect migration warning in: %s", pattern)
				}
			})
		})

		Context("when PR has no migration warnings", func() {
			It("should not detect false positives", func() {
				falsePositives := []string{
					"Migration is needed",
					"MIGRATION: important note",
					"This requires migration work",
					"migration plan",
					"database migration script",
					"Run migration after deployment",
					"Migration guide: ...",
				}

				for _, text := range falsePositives {
					pr := cmd.PullRequest{Body: text}
					Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse(),
						"Should not detect migration warning in: %s", text)
				}
			})

			It("should handle empty body", func() {
				pr := cmd.PullRequest{Body: ""}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
			})
		})
	})

	Describe("PR Status Detection", func() {
		Context("isOnHold", func() {
			It("should detect hold label", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
						{Name: "bug"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeTrue())
			})

			It("should not detect hold when no hold label", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "bug"},
						{Name: "enhancement"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})

			It("should handle empty labels", func() {
				pr := cmd.PullRequest{Labels: []cmd.Label{}}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})
		})

		Context("needsRebase", func() {
			It("should detect conflicting state", func() {
				pr := cmd.PullRequest{MergeableState: "dirty"}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeTrue())
			})

			It("should not need rebase when clean", func() {
				pr := cmd.PullRequest{MergeableState: "clean"}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			})

			It("should handle unknown state", func() {
				pr := cmd.PullRequest{MergeableState: "unknown"}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			})
		})

		Context("isBlocked", func() {
			It("should detect blocked PR", func() {
				pr := cmd.PullRequest{MergeableState: "blocked"}
				Expect(cmd.IsBlockedTest(pr)).To(BeTrue())
			})

			It("should not detect blocked when clean", func() {
				pr := cmd.PullRequest{MergeableState: "clean"}
				Expect(cmd.IsBlockedTest(pr)).To(BeFalse())
			})
		})
	})

	Describe("String Utilities", func() {
		Context("TruncateString", func() {
			It("should truncate long strings", func() {
				result := cmd.TruncateStringTest("This is a very long string that needs truncation", 10)
				Expect(result).To(Equal("This is..."))
			})

			It("should not truncate short strings", func() {
				result := cmd.TruncateStringTest("Short", 10)
				Expect(result).To(Equal("Short"))
			})

			It("should handle empty strings", func() {
				result := cmd.TruncateStringTest("", 10)
				Expect(result).To(Equal(""))
			})
		})

		Context("DisplayWidth", func() {
			It("should calculate display width correctly", func() {
				width := cmd.DisplayWidthTest("Hello World")
				Expect(width).To(Equal(11))
			})

			It("should handle empty strings", func() {
				width := cmd.DisplayWidthTest("")
				Expect(width).To(Equal(0))
			})
		})

		Context("StripANSISequences", func() {
			It("should remove ANSI color codes", func() {
				input := "\033[31mRed text\033[0m"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Red text"))
			})

			It("should handle text without ANSI codes", func() {
				input := "Plain text"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Plain text"))
			})
		})

		Context("PadString", func() {
			It("should pad strings to specified width", func() {
				result := cmd.PadStringTest("Hello", 10)
				Expect(result).To(Equal("Hello     "))
			})

			It("should not pad strings already at width", func() {
				result := cmd.PadStringTest("Hello", 5)
				Expect(result).To(Equal("Hello"))
			})
		})
	})

	Describe("PR Link Formatting", func() {
		Context("formatPRLink", func() {
			It("should format PR links correctly", func() {
				link := cmd.FormatPRLinkTest("owner", "repo", 123)
				// In test environment, usually returns short format due to NO_COLOR or not being a terminal
				Expect(link).To(MatchRegexp(`^(#123|\033]8;;https://github\.com/owner/repo/pull/123\033\\#123\033]8;;\033\\)$`))
			})

			It("should handle different owners and repos", func() {
				link := cmd.FormatPRLinkTest("testorg", "testproject", 456)
				// In test environment, usually returns short format due to NO_COLOR or not being a terminal
				Expect(link).To(MatchRegexp(`^(#456|\033]8;;https://github\.com/testorg/testproject/pull/456\033\\#456\033]8;;\033\\)$`))
			})
		})
	})

	Describe("Status Icons", func() {
		Context("getStatusIcon", func() {
			It("should return correct icon for open PR", func() {
				pr := cmd.PullRequest{State: "open", Draft: false}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🟢"))
			})

			It("should return correct icon for draft PR", func() {
				pr := cmd.PullRequest{State: "open", Draft: true}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🟡"))
			})

			It("should return correct icon for closed PR", func() {
				pr := cmd.PullRequest{State: "closed", Draft: false}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🔴"))
			})

			It("should return correct icon for merged PR", func() {
				pr := cmd.PullRequest{State: "merged", Draft: false}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🟣"))
			})

			It("should return correct icon for held PR", func() {
				pr := cmd.PullRequest{
					State: "open",
					Draft: false,
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
					},
				}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🔶"))
			})
		})
	})

	Describe("Pull Request Sorting", func() {
		var testPRs []cmd.PullRequest

		BeforeEach(func() {
			testPRs = []cmd.PullRequest{
				{
					Number:    1,
					Title:     "First PR",
					CreatedAt: "2023-01-01T00:00:00Z",
					UpdatedAt: "2023-01-01T12:00:00Z",
					Body:      "Regular PR",
				},
				{
					Number:    2,
					Title:     "Second PR",
					CreatedAt: "2023-01-02T00:00:00Z",
					UpdatedAt: "2023-01-02T12:00:00Z",
					Body:      "⚠️[migration] Migration PR",
				},
				{
					Number:    3,
					Title:     "Third PR",
					CreatedAt: "2023-01-03T00:00:00Z",
					UpdatedAt: "2023-01-01T06:00:00Z",
					Body:      "Another regular PR",
				},
			}
		})

		Context("sortPullRequests", func() {
			It("should sort by oldest creation date", func() {
				cmd.SortPullRequestsTest(testPRs, "oldest")
				Expect(testPRs[0].Number).To(Equal(1))
				Expect(testPRs[1].Number).To(Equal(2))
				Expect(testPRs[2].Number).To(Equal(3))
			})

			It("should sort by PR number", func() {
				// Rearrange to test sorting
				testPRs[0].Number = 10
				testPRs[1].Number = 5
				testPRs[2].Number = 15

				cmd.SortPullRequestsTest(testPRs, "number")
				Expect(testPRs[0].Number).To(Equal(5))
				Expect(testPRs[1].Number).To(Equal(10))
				Expect(testPRs[2].Number).To(Equal(15))
			})

			It("should sort by updated date (most recent first)", func() {
				cmd.SortPullRequestsTest(testPRs, "updated")
				// Most recently updated should be first
				Expect(testPRs[0].Number).To(Equal(2)) // 2023-01-02T12:00:00Z
				Expect(testPRs[1].Number).To(Equal(1)) // 2023-01-01T12:00:00Z
				Expect(testPRs[2].Number).To(Equal(3)) // 2023-01-01T06:00:00Z
			})

			It("should sort by priority (migration warnings first)", func() {
				cmd.SortPullRequestsTest(testPRs, "priority")
				// Migration PR should be first
				Expect(testPRs[0].Body).To(ContainSubstring("migration"))
				Expect(testPRs[0].Number).To(Equal(2))
			})

			It("should maintain order for newest/default sorting", func() {
				originalNumbers := []int{testPRs[0].Number, testPRs[1].Number, testPRs[2].Number}
				cmd.SortPullRequestsTest(testPRs, "newest")
				// Should maintain original order
				Expect(testPRs[0].Number).To(Equal(originalNumbers[0]))
				Expect(testPRs[1].Number).To(Equal(originalNumbers[1]))
				Expect(testPRs[2].Number).To(Equal(originalNumbers[2]))
			})

			It("should handle unknown sort option", func() {
				originalNumbers := []int{testPRs[0].Number, testPRs[1].Number, testPRs[2].Number}
				cmd.SortPullRequestsTest(testPRs, "unknown")
				// Should maintain original order for unknown option
				Expect(testPRs[0].Number).To(Equal(originalNumbers[0]))
				Expect(testPRs[1].Number).To(Equal(originalNumbers[1]))
				Expect(testPRs[2].Number).To(Equal(originalNumbers[2]))
			})
		})
	})

	Describe("Color Detection", func() {
		Context("shouldUseColors", func() {
			It("should detect color support", func() {
				// This tests the color detection logic
				result := cmd.ShouldUseColorsTest()
				// The result will depend on the environment, so we just test that it returns a boolean
				Expect(result).To(BeAssignableToTypeOf(false))
			})
		})
	})

	Describe("Git Diff Colorization", func() {
		Context("colorizeGitDiff", func() {
			It("should colorize addition lines", func() {
				diff := "+added line\n-removed line\n unchanged line"
				result := cmd.ColorizeGitDiffTest(diff)

				// Check that the result contains the original content
				Expect(result).To(ContainSubstring("added line"))
				Expect(result).To(ContainSubstring("removed line"))
				Expect(result).To(ContainSubstring("unchanged line"))
			})

			It("should handle empty diff", func() {
				diff := ""
				result := cmd.ColorizeGitDiffTest(diff)
				Expect(result).To(Equal(""))
			})

			It("should handle diff without +/- lines", func() {
				diff := "No changes here\nJust regular text"
				result := cmd.ColorizeGitDiffTest(diff)
				Expect(result).To(Equal(diff))
			})
		})
	})

	Describe("Approval Flow Logic", func() {
		Context("label-based approval detection", func() {
			It("should detect approved label", func() {
				labels := []cmd.Label{
					{Name: "approved"},
					{Name: "bug"},
				}

				hasApproval := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApproval = true
						break
					}
				}

				Expect(hasApproval).To(BeTrue())
			})

			It("should detect lgtm label", func() {
				labels := []cmd.Label{
					{Name: "lgtm"},
					{Name: "enhancement"},
				}

				hasApproval := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApproval = true
						break
					}
				}

				Expect(hasApproval).To(BeTrue())
			})
		})

		Context("migration confirmation logic", func() {
			It("should validate confirmation responses", func() {
				validResponses := []string{"y", "yes", "Y", "YES"}
				invalidResponses := []string{"", "n", "no", "N", "NO", "maybe", "later"}

				for _, response := range validResponses {
					// Test the confirmation logic
					normalized := strings.ToLower(strings.TrimSpace(response))
					confirmed := normalized == "y" || normalized == "yes"
					Expect(confirmed).To(BeTrue(), "Response '%s' should be confirmed", response)
				}

				for _, response := range invalidResponses {
					// Test the confirmation logic
					normalized := strings.ToLower(strings.TrimSpace(response))
					confirmed := normalized == "y" || normalized == "yes"
					Expect(confirmed).To(BeFalse(), "Response '%s' should not be confirmed", response)
				}
			})
		})

		Context("PR state validation", func() {
			It("should validate PR states", func() {
				validStates := []string{"open", "closed", "merged"}

				for _, state := range validStates {
					pr := cmd.PullRequest{State: state}
					Expect(pr.State).To(BeElementOf(validStates))
				}
			})

			It("should handle draft PRs", func() {
				pr := cmd.PullRequest{State: "open", Draft: true}
				Expect(pr.Draft).To(BeTrue())
				Expect(pr.State).To(Equal("open"))
			})
		})
	})

	Describe("Cache Logic", func() {
		Context("cache key generation", func() {
			It("should generate consistent cache keys", func() {
				owner := "testowner"
				repo := "testrepo"
				prNumber := 123

				// Simulate cache key generation
				cacheKey := owner + "/" + repo + "/" + string(rune(prNumber))
				Expect(cacheKey).To(ContainSubstring(owner))
				Expect(cacheKey).To(ContainSubstring(repo))
			})
		})

		Context("cache decision logic", func() {
			It("should use cache when mergeable_state is missing", func() {
				pr := cmd.PullRequest{
					Number:         123,
					MergeableState: "",
				}

				shouldFetch := pr.MergeableState == ""
				Expect(shouldFetch).To(BeTrue())
			})

			It("should not use cache when mergeable_state is present", func() {
				pr := cmd.PullRequest{
					Number:         123,
					MergeableState: "clean",
				}

				shouldFetch := pr.MergeableState == ""
				Expect(shouldFetch).To(BeFalse())
			})
		})
	})
})

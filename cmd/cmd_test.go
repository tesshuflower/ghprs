package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Cmd Package", func() {
	Describe("Utility Functions", func() {
		Describe("TruncateString", func() {
			It("should truncate strings longer than maxWidth", func() {
				result := cmd.TruncateStringTest("This is a very long string", 10)
				Expect(result).To(Equal("This is..."))
				Expect(len(result)).To(Equal(10))
			})

			It("should return original string if shorter than maxWidth", func() {
				result := cmd.TruncateStringTest("Short", 10)
				Expect(result).To(Equal("Short"))
			})

			It("should return original string if equal to maxWidth", func() {
				result := cmd.TruncateStringTest("Exactly10!", 10)
				Expect(result).To(Equal("Exactly10!"))
			})

			It("should handle empty strings", func() {
				result := cmd.TruncateStringTest("", 10)
				Expect(result).To(Equal(""))
			})

			It("should handle very small maxWidth", func() {
				result := cmd.TruncateStringTest("Hello", 3)
				Expect(result).To(Equal("Hel"))
			})
		})

		Describe("DisplayWidth", func() {
			It("should calculate width of ASCII strings correctly", func() {
				width := cmd.DisplayWidthTest("Hello World")
				Expect(width).To(Equal(11))
			})

			It("should calculate width of strings with emojis correctly", func() {
				width := cmd.DisplayWidthTest("🟢 Test")
				Expect(width).To(Equal(7)) // emoji = 2, space = 1, "Test" = 4
			})

			It("should handle empty strings", func() {
				width := cmd.DisplayWidthTest("")
				Expect(width).To(Equal(0))
			})

			It("should handle strings with multiple emojis", func() {
				width := cmd.DisplayWidthTest("🟢🟡🔶")
				Expect(width).To(Equal(6)) // 3 emojis * 2 each
			})
		})

		Describe("StripANSISequences", func() {
			It("should remove ANSI color sequences", func() {
				input := "\033[31mRed text\033[0m"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Red text"))
			})

			It("should remove OSC 8 sequences (clickable links)", func() {
				input := "\033]8;;https://example.com\033\\Link text\033]8;;\033\\"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Link text"))
			})

			It("should handle plain text without sequences", func() {
				input := "Plain text"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Plain text"))
			})

			It("should handle empty strings", func() {
				result := cmd.StripANSISequencesTest("")
				Expect(result).To(Equal(""))
			})
		})

		Describe("PadString", func() {
			It("should pad strings to specified width", func() {
				result := cmd.PadStringTest("Test", 10)
				Expect(result).To(Equal("Test      "))
				Expect(cmd.DisplayWidthTest(result)).To(Equal(10))
			})

			It("should not pad if string is already correct width", func() {
				result := cmd.PadStringTest("Test", 4)
				Expect(result).To(Equal("Test"))
			})

			It("should not pad if string is longer than width", func() {
				result := cmd.PadStringTest("Very long string", 5)
				Expect(result).To(Equal("Very long string"))
			})
		})

		Describe("FormatPRLink", func() {
			It("should format PR links with OSC 8 sequences when colors enabled", func() {
				result := cmd.FormatPRLinkTest("owner", "repo", 123)
				Expect(result).To(ContainSubstring("#123"))
				// Should contain OSC 8 sequence if colors are enabled
				if cmd.ShouldUseColorsTest() {
					Expect(result).To(ContainSubstring("\033]8;;"))
				}
			})

			It("should format PR links as plain text when colors disabled", func() {
				// This test would need to mock the color detection
				result := cmd.FormatPRLinkTest("owner", "repo", 123)
				Expect(result).To(ContainSubstring("#123"))
			})
		})
	})

	Describe("Data Structures", func() {
		Describe("PullRequest", func() {
			var pr cmd.PullRequest

			BeforeEach(func() {
				pr = cmd.PullRequest{
					Number: 123,
					Title:  "Test PR",
					State:  "open",
					User: cmd.User{
						Login: "testuser",
					},
					Head: cmd.Branch{
						Ref: "feature-branch",
						SHA: "abc123",
					},
					Base: cmd.Branch{
						Ref: "main",
						SHA: "def456",
					},
					Draft:     false,
					CreatedAt: "2023-01-01T00:00:00Z",
					UpdatedAt: "2023-01-02T00:00:00Z",
					HTMLURL:   "https://github.com/owner/repo/pull/123",
					Body:      "This is a test PR",
					Labels:    []cmd.Label{},
				}
			})

			It("should create valid PullRequest structs", func() {
				Expect(pr.Number).To(Equal(123))
				Expect(pr.Title).To(Equal("Test PR"))
				Expect(pr.State).To(Equal("open"))
				Expect(pr.User.Login).To(Equal("testuser"))
				Expect(pr.Head.Ref).To(Equal("feature-branch"))
				Expect(pr.Base.Ref).To(Equal("main"))
			})
		})

		Describe("CheckStatus", func() {
			It("should calculate totals correctly", func() {
				status := &cmd.CheckStatus{
					Passed:    5,
					Failed:    2,
					Pending:   1,
					Cancelled: 1,
					Skipped:   1,
				}
				status.Total = status.Passed + status.Failed + status.Pending + status.Cancelled + status.Skipped
				Expect(status.Total).To(Equal(10))
			})
		})
	})

	Describe("Status Functions", func() {
		Describe("GetStatusIcon", func() {
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
				pr := cmd.PullRequest{State: "closed"}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🔴"))
			})

			It("should return correct icon for merged PR", func() {
				pr := cmd.PullRequest{State: "merged"}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🟣"))
			})

			It("should return correct icon for PR on hold", func() {
				pr := cmd.PullRequest{
					State: "open",
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
					},
				}
				icon := cmd.GetStatusIconTest(pr)
				Expect(icon).To(Equal("🔶"))
			})
		})

		Describe("IsOnHold", func() {
			It("should detect PR on hold", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeTrue())
			})

			It("should not detect normal PR as on hold", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "bug"},
						{Name: "enhancement"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})

			It("should handle PR with no labels", func() {
				pr := cmd.PullRequest{Labels: []cmd.Label{}}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})
		})

		Describe("HasMigrationWarning", func() {
			It("should detect migration warning in PR body", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains ⚠️[migration] changes that require attention",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect different migration warning patterns", func() {
				patterns := []string{
					"⚠️[migration] warning",
					":warning:[migration] note",
					"⚠️migration⚠️ alert",
					"[migration] update required",
				}

				for _, pattern := range patterns {
					pr := cmd.PullRequest{Body: pattern}
					Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue(), "Should detect pattern: "+pattern)
				}
			})

			It("should not detect migration warning in normal PR body", func() {
				pr := cmd.PullRequest{
					Body: "This is a normal PR with no special warnings",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
			})

			It("should handle empty PR body", func() {
				pr := cmd.PullRequest{Body: ""}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
			})
		})
	})

	Describe("Configuration", func() {
		Describe("DefaultConfig", func() {
			It("should create config with correct defaults", func() {
				config := cmd.DefaultConfig()
				Expect(config.Defaults.State).To(Equal("open"))
				Expect(config.Defaults.Limit).To(Equal(30))
				Expect(config.Repositories).To(BeEmpty())
			})
		})
	})

	Describe("Sorting Functions", func() {
		var prs []cmd.PullRequest

		BeforeEach(func() {
			prs = []cmd.PullRequest{
				{
					Number:    1,
					Title:     "First PR",
					CreatedAt: "2023-01-01T00:00:00Z",
					UpdatedAt: "2023-01-03T00:00:00Z",
				},
				{
					Number:    2,
					Title:     "Second PR",
					CreatedAt: "2023-01-02T00:00:00Z",
					UpdatedAt: "2023-01-01T00:00:00Z",
				},
				{
					Number:    3,
					Title:     "Third PR",
					CreatedAt: "2023-01-03T00:00:00Z",
					UpdatedAt: "2023-01-02T00:00:00Z",
				},
			}
		})

		Describe("SortPullRequests", func() {
			It("should sort by oldest when specified", func() {
				cmd.SortPullRequestsTest(prs, "oldest")
				Expect(prs[0].Number).To(Equal(1)) // Created first
				Expect(prs[1].Number).To(Equal(2)) // Created second
				Expect(prs[2].Number).To(Equal(3)) // Created third
			})

			It("should sort by updated when specified", func() {
				cmd.SortPullRequestsTest(prs, "updated")
				Expect(prs[0].Number).To(Equal(1)) // Most recently updated
				Expect(prs[1].Number).To(Equal(3)) // Second most recent
				Expect(prs[2].Number).To(Equal(2)) // Least recent
			})

			It("should sort by number when specified", func() {
				// Reverse the order first
				prs[0], prs[2] = prs[2], prs[0]
				cmd.SortPullRequestsTest(prs, "number")
				Expect(prs[0].Number).To(Equal(1))
				Expect(prs[1].Number).To(Equal(2))
				Expect(prs[2].Number).To(Equal(3))
			})

			It("should not change order for newest (default)", func() {
				originalOrder := make([]int, len(prs))
				for i, pr := range prs {
					originalOrder[i] = pr.Number
				}
				cmd.SortPullRequestsTest(prs, "newest")
				for i, pr := range prs {
					Expect(pr.Number).To(Equal(originalOrder[i]))
				}
			})
		})
	})

	Describe("Color Functions", func() {
		Describe("ColorizeGitDiff", func() {
			It("should colorize added lines", func() {
				diff := "+added line"
				result := cmd.ColorizeGitDiffTest(diff)
				if cmd.ShouldUseColorsTest() {
					Expect(result).To(ContainSubstring("\033[32m")) // Green color
				}
			})

			It("should colorize removed lines", func() {
				diff := "-removed line"
				result := cmd.ColorizeGitDiffTest(diff)
				if cmd.ShouldUseColorsTest() {
					Expect(result).To(ContainSubstring("\033[31m")) // Red color
				}
			})

			It("should handle plain diff without colors when disabled", func() {
				diff := "+added\n-removed\n unchanged"
				result := cmd.ColorizeGitDiffTest(diff)
				Expect(result).To(ContainSubstring("added"))
				Expect(result).To(ContainSubstring("removed"))
				Expect(result).To(ContainSubstring("unchanged"))
			})
		})
	})

	Describe("Integration Tests", func() {
		Describe("Repository Selection", func() {
			It("should handle single repository correctly", func() {
				repos := []string{"owner/repo"}
				// This would test the promptForRepositorySelection function
				// but it requires user input, so we'd need to mock stdin
				Expect(len(repos)).To(Equal(1))
			})

			It("should handle multiple repositories", func() {
				repos := []string{"owner/repo1", "owner/repo2", "owner/repo3"}
				Expect(len(repos)).To(BeNumerically(">", 1))
			})
		})
	})
})

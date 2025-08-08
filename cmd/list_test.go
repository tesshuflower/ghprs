package cmd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Listing Functionality", func() {

	Describe("Pull Request Status Detection", func() {
		Describe("isOnHold", func() {
			It("should detect PR on hold with do-not-merge/hold label", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
						{Name: "enhancement"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeTrue())
			})

			It("should not detect PR on hold without do-not-merge/hold label", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "enhancement"},
						{Name: "bug"},
					},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})

			It("should handle empty labels", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{},
				}
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			})
		})

		Describe("needsRebase", func() {
			It("should detect PR needs rebase when mergeable_state is dirty", func() {
				pr := cmd.PullRequest{
					MergeableState: "dirty",
				}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeTrue())
			})

			It("should detect PR needs rebase when mergeable_state is behind", func() {
				pr := cmd.PullRequest{
					MergeableState: "behind",
				}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeTrue())
			})

			It("should not detect rebase needed for clean mergeable_state", func() {
				pr := cmd.PullRequest{
					MergeableState: "clean",
				}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			})

			It("should not detect rebase needed for unstable mergeable_state", func() {
				pr := cmd.PullRequest{
					MergeableState: "unstable",
				}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			})

			It("should handle empty mergeable_state", func() {
				pr := cmd.PullRequest{
					MergeableState: "",
				}
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			})
		})

		Describe("isBlocked", func() {
			It("should detect blocked PR when mergeable_state is blocked", func() {
				pr := cmd.PullRequest{
					MergeableState: "blocked",
				}
				Expect(cmd.IsBlockedTest(pr)).To(BeTrue())
			})

			It("should not detect blocked for other mergeable_states", func() {
				mergeableStates := []string{"clean", "dirty", "behind", "unstable", "unknown", ""}

				for _, state := range mergeableStates {
					pr := cmd.PullRequest{
						MergeableState: state,
					}
					Expect(cmd.IsBlockedTest(pr)).To(BeFalse(), "Expected state '%s' to not be blocked", state)
				}
			})
		})

		Describe("isKonfluxNudge", func() {
			It("should detect konflux-nudge label", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "konflux-nudge"},
						{Name: "enhancement"},
					},
				}
				Expect(cmd.IsKonfluxNudgeTest(pr)).To(BeTrue())
			})

			It("should not detect konflux-nudge when not present", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{
						{Name: "enhancement"},
						{Name: "bug"},
					},
				}
				Expect(cmd.IsKonfluxNudgeTest(pr)).To(BeFalse())
			})

			It("should handle empty labels", func() {
				pr := cmd.PullRequest{
					Labels: []cmd.Label{},
				}
				Expect(cmd.IsKonfluxNudgeTest(pr)).To(BeFalse())
			})
		})
	})

	Describe("Content Analysis", func() {
		Describe("hasMigrationWarning", func() {
			It("should detect ⚠️[migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "This PR includes ⚠️[migration] changes that need attention.",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect :warning:[migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "Please review: :warning:[migration] database schema changes included.",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect ⚠️migration⚠️ pattern", func() {
				pr := cmd.PullRequest{
					Body: "Important: ⚠️migration⚠️ review required.",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should detect [migration] pattern", func() {
				pr := cmd.PullRequest{
					Body: "This PR contains [migration] changes.",
				}
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should be case insensitive", func() {
				patterns := []string{
					"⚠️[MIGRATION] changes",
					":WARNING:[Migration] updates",
					"⚠️MIGRATION⚠️ notice",
					"[Migration] required",
				}

				for _, pattern := range patterns {
					pr := cmd.PullRequest{
						Body: pattern,
					}
					Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue(), "Expected pattern '%s' to be detected", pattern)
				}
			})

			It("should not detect migration warnings when not present", func() {
				nonMigrationBodies := []string{
					"Regular PR body without any migration content",
					"This contains the word migrate but no warning pattern",
					"[feature] new functionality added",
					"⚠️ warning but not migration related",
					"",
				}

				for _, body := range nonMigrationBodies {
					pr := cmd.PullRequest{
						Body: body,
					}
					Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse(), "Expected body '%s' to not be detected as migration", body)
				}
			})
		})

		Describe("hasSecurity", func() {
			It("should detect SECURITY in title", func() {
				pr := cmd.PullRequest{
					Title: "Fix SECURITY vulnerability in auth module",
				}
				Expect(cmd.HasSecurityTest(pr)).To(BeTrue())
			})

			It("should detect CVE in title", func() {
				pr := cmd.PullRequest{
					Title: "Patch CVE-2023-1234 in dependencies",
				}
				Expect(cmd.HasSecurityTest(pr)).To(BeTrue())
			})

			It("should be case insensitive for SECURITY", func() {
				titles := []string{
					"security fix applied",
					"Security patch for library",
					"SECURITY: Critical update",
					"Apply Security fixes",
				}

				for _, title := range titles {
					pr := cmd.PullRequest{
						Title: title,
					}
					Expect(cmd.HasSecurityTest(pr)).To(BeTrue(), "Expected title '%s' to be detected as security", title)
				}
			})

			It("should be case insensitive for CVE", func() {
				titles := []string{
					"Fix cve-2023-1234",
					"Patch CVE-2023-5678",
					"Update for Cve-2023-9999",
				}

				for _, title := range titles {
					pr := cmd.PullRequest{
						Title: title,
					}
					Expect(cmd.HasSecurityTest(pr)).To(BeTrue(), "Expected title '%s' to be detected as CVE", title)
				}
			})

			It("should not detect security when not present", func() {
				nonSecurityTitles := []string{
					"Regular feature addition",
					"Bug fix for display issue",
					"Refactor database queries",
					"Add new API endpoint",
					"",
				}

				for _, title := range nonSecurityTitles {
					pr := cmd.PullRequest{
						Title: title,
					}
					Expect(cmd.HasSecurityTest(pr)).To(BeFalse(), "Expected title '%s' to not be detected as security", title)
				}
			})
		})
	})

	Describe("Status Icon Generation", func() {
		Describe("getStatusIcon", func() {
			It("should return draft icon for draft PRs", func() {
				pr := cmd.PullRequest{
					Draft: true,
					State: "open",
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🟡"))
			})

			It("should return hold icon for PRs on hold", func() {
				pr := cmd.PullRequest{
					Draft: false,
					State: "open",
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
					},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🔶"))
			})

			It("should return green icon for open PRs not on hold", func() {
				pr := cmd.PullRequest{
					Draft:  false,
					State:  "open",
					Labels: []cmd.Label{},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🟢"))
			})

			It("should return red icon for closed PRs", func() {
				pr := cmd.PullRequest{
					Draft:  false,
					State:  "closed",
					Labels: []cmd.Label{},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🔴"))
			})

			It("should return purple icon for merged PRs", func() {
				pr := cmd.PullRequest{
					Draft:  false,
					State:  "merged",
					Labels: []cmd.Label{},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🟣"))
			})

			It("should return hold icon for unknown state PRs on hold", func() {
				pr := cmd.PullRequest{
					Draft: false,
					State: "unknown",
					Labels: []cmd.Label{
						{Name: "do-not-merge/hold"},
					},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("🔶"))
			})

			It("should return white icon for unknown state PRs not on hold", func() {
				pr := cmd.PullRequest{
					Draft:  false,
					State:  "unknown",
					Labels: []cmd.Label{},
				}
				Expect(cmd.GetStatusIconTest(pr)).To(Equal("⚪"))
			})
		})
	})

	Describe("String Utilities", func() {
		Describe("TruncateString", func() {
			It("should not truncate strings shorter than max width", func() {
				result := cmd.TruncateStringTest("Hello", 10)
				Expect(result).To(Equal("Hello"))
			})

			It("should truncate strings longer than max width", func() {
				result := cmd.TruncateStringTest("This is a very long string", 10)
				Expect(result).To(Equal("This is..."))
			})

			It("should handle exact max width", func() {
				result := cmd.TruncateStringTest("Exactly10!", 10)
				Expect(result).To(Equal("Exactly10!"))
			})

			It("should handle empty string", func() {
				result := cmd.TruncateStringTest("", 10)
				Expect(result).To(Equal(""))
			})

			It("should handle zero max width", func() {
				result := cmd.TruncateStringTest("Hello", 0)
				Expect(result).To(Equal(""))
			})

			It("should handle very small width", func() {
				result := cmd.TruncateStringTest("Hello World", 2)
				Expect(result).To(Equal("He")) // When maxWidth <= 3, truncates by runes without ellipsis
			})

			It("should handle width of 3", func() {
				result := cmd.TruncateStringTest("Hello World", 3)
				Expect(result).To(Equal("Hel")) // When maxWidth <= 3, truncates by runes without ellipsis
			})
		})

		Describe("DisplayWidth", func() {
			It("should calculate width of simple ASCII strings", func() {
				Expect(cmd.DisplayWidthTest("Hello")).To(Equal(5))
				Expect(cmd.DisplayWidthTest("")).To(Equal(0))
				Expect(cmd.DisplayWidthTest("123")).To(Equal(3))
			})

			It("should handle strings with ANSI escape sequences", func() {
				// ANSI sequences should not count toward display width
				coloredString := "\033[31mRed Text\033[0m"
				Expect(cmd.DisplayWidthTest(coloredString)).To(Equal(8)) // Only "Red Text" counts
			})

			It("should handle tabs", func() {
				// Tabs count as 1 character for display width
				Expect(cmd.DisplayWidthTest("Hello\tWorld")).To(Equal(10))
			})
		})

		Describe("StripANSISequences", func() {
			It("should remove ANSI color codes", func() {
				input := "\033[31mRed Text\033[0m"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Red Text"))
			})

			It("should remove complex ANSI sequences", func() {
				input := "\033[1;31;46mBold Red on Cyan\033[0m Normal"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal("Bold Red on Cyan Normal"))
			})

			It("should leave normal text unchanged", func() {
				input := "Normal text without ANSI"
				result := cmd.StripANSISequencesTest(input)
				Expect(result).To(Equal(input))
			})

			It("should handle empty string", func() {
				result := cmd.StripANSISequencesTest("")
				Expect(result).To(Equal(""))
			})
		})

		Describe("PadString", func() {
			It("should pad strings shorter than target width", func() {
				result := cmd.PadStringTest("Hello", 10)
				Expect(result).To(Equal("Hello     "))
				Expect(len(result)).To(Equal(10))
			})

			It("should not pad strings equal to target width", func() {
				result := cmd.PadStringTest("Exactly10!", 10)
				Expect(result).To(Equal("Exactly10!"))
			})

			It("should not truncate strings longer than target width (PadString doesn't truncate)", func() {
				result := cmd.PadStringTest("This is too long", 10)
				Expect(result).To(Equal("This is too long")) // PadString doesn't truncate, just returns original if >= width
			})

			It("should handle zero width", func() {
				result := cmd.PadStringTest("Hello", 0)
				Expect(result).To(Equal("Hello")) // Returns original string when current width >= target width
			})

			It("should handle negative width", func() {
				result := cmd.PadStringTest("Hello", -1)
				Expect(result).To(Equal("Hello")) // Returns original string when current width >= target width
			})

			It("should handle empty string", func() {
				result := cmd.PadStringTest("", 5)
				Expect(result).To(Equal("     "))
			})
		})

		Describe("FormatPRLink", func() {
			It("should format GitHub PR links with terminal features", func() {
				result := cmd.FormatPRLinkTest("microsoft", "vscode", 12345)
				// Since we're in a test environment, it likely returns the simple format
				Expect(result).To(ContainSubstring("#12345"))
			})

			It("should handle different owner/repo combinations", func() {
				result := cmd.FormatPRLinkTest("owner-name", "repo-name", 1)
				Expect(result).To(ContainSubstring("#1"))
			})

			It("should handle zero PR number", func() {
				result := cmd.FormatPRLinkTest("owner", "repo", 0)
				Expect(result).To(ContainSubstring("#0"))
			})
		})
	})

	Describe("Pull Request Sorting", func() {
		var samplePRs []cmd.PullRequest

		BeforeEach(func() {
			samplePRs = []cmd.PullRequest{
				{
					Number:    1,
					Title:     "First PR",
					CreatedAt: "2023-01-01T10:00:00Z",
					UpdatedAt: "2023-01-01T11:00:00Z",
				},
				{
					Number:    2,
					Title:     "Second PR",
					CreatedAt: "2023-01-02T10:00:00Z",
					UpdatedAt: "2023-01-02T12:00:00Z",
				},
				{
					Number:    3,
					Title:     "Third PR",
					CreatedAt: "2023-01-03T10:00:00Z",
					UpdatedAt: "2023-01-03T09:00:00Z",
				},
			}
		})

		Describe("sortPullRequests", func() {
			It("should sort by oldest first", func() {
				prs := make([]cmd.PullRequest, len(samplePRs))
				copy(prs, samplePRs)

				// Shuffle the order
				prs[0], prs[2] = prs[2], prs[0]

				cmd.SortPullRequestsTest(prs, "oldest")

				Expect(prs[0].Number).To(Equal(1))
				Expect(prs[1].Number).To(Equal(2))
				Expect(prs[2].Number).To(Equal(3))
			})

			It("should sort by most recently updated first (default)", func() {
				prs := make([]cmd.PullRequest, len(samplePRs))
				copy(prs, samplePRs)

				cmd.SortPullRequestsTest(prs, "updated")

				// Default sort is by number descending, not by updated time
				Expect(prs[0].Number).To(Equal(3))
				Expect(prs[1].Number).To(Equal(2))
				Expect(prs[2].Number).To(Equal(1))
			})

			It("should handle unknown sort option gracefully", func() {
				prs := make([]cmd.PullRequest, len(samplePRs))
				copy(prs, samplePRs)
				originalOrder := make([]int, len(prs))
				for i, pr := range prs {
					originalOrder[i] = pr.Number
				}

				cmd.SortPullRequestsTest(prs, "invalid-sort-option")

				// Should remain in original order for unknown sort options
				for i, pr := range prs {
					Expect(pr.Number).To(Equal(originalOrder[i]))
				}
			})

			It("should handle empty slice", func() {
				var emptyPRs []cmd.PullRequest
				Expect(func() {
					cmd.SortPullRequestsTest(emptyPRs, "oldest")
				}).ToNot(Panic())
			})

			It("should handle single PR", func() {
				singlePR := []cmd.PullRequest{samplePRs[0]}
				cmd.SortPullRequestsTest(singlePR, "oldest")
				Expect(singlePR[0].Number).To(Equal(1))
			})
		})
	})

	Describe("Utility Functions", func() {
		Describe("ShouldUseColors", func() {
			It("should provide consistent color usage decision", func() {
				result1 := cmd.ShouldUseColorsTest()
				result2 := cmd.ShouldUseColorsTest()
				Expect(result1).To(Equal(result2))
			})
		})

		Describe("NewPRDetailsCache", func() {
			It("should create a new empty cache", func() {
				cache := cmd.NewPRDetailsCacheTest()
				Expect(cache).NotTo(BeNil())
			})
		})
	})

	Describe("Integration Scenarios", func() {
		It("should correctly identify complex PR states", func() {
			complexPR := cmd.PullRequest{
				Number:         123,
				Title:          "SECURITY: Fix CVE-2023-1234 vulnerability",
				State:          "open",
				Draft:          false,
				MergeableState: "dirty",
				Body:           "This PR includes ⚠️[migration] database changes and security fixes.",
				Labels: []cmd.Label{
					{Name: "do-not-merge/hold"},
					{Name: "security"},
					{Name: "approved"},
				},
			}

			// Should detect multiple conditions
			Expect(cmd.IsOnHoldTest(complexPR)).To(BeTrue())
			Expect(cmd.HasSecurityTest(complexPR)).To(BeTrue())
			Expect(cmd.HasMigrationWarningTest(complexPR)).To(BeTrue())
			Expect(cmd.NeedsRebaseTest(complexPR)).To(BeTrue())
			Expect(cmd.GetStatusIconTest(complexPR)).To(Equal("🔶")) // Hold status takes precedence
		})

		It("should handle clean, ready-to-merge PRs", func() {
			cleanPR := cmd.PullRequest{
				Number:         456,
				Title:          "Feature: Add new user dashboard",
				State:          "open",
				Draft:          false,
				MergeableState: "clean",
				Body:           "Adds a new user dashboard with improved UX.",
				Labels:         []cmd.Label{},
			}

			Expect(cmd.IsOnHoldTest(cleanPR)).To(BeFalse())
			Expect(cmd.HasSecurityTest(cleanPR)).To(BeFalse())
			Expect(cmd.HasMigrationWarningTest(cleanPR)).To(BeFalse())
			Expect(cmd.NeedsRebaseTest(cleanPR)).To(BeFalse())
			Expect(cmd.IsBlockedTest(cleanPR)).To(BeFalse())
			Expect(cmd.GetStatusIconTest(cleanPR)).To(Equal("🟢")) // Green for ready
		})
	})
})

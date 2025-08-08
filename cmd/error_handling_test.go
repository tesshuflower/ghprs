package cmd_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Error Handling and Edge Cases", func() {

	Describe("Data Validation and Edge Cases", func() {
		Describe("Malformed PR Data", func() {
			It("should handle PRs with missing fields gracefully", func() {
				pr := cmd.PullRequest{
					// Missing most fields
					Number: 123,
					// Title, State, etc. are empty
				}

				// Functions should not panic with minimal data
				Expect(func() { cmd.IsOnHoldTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.NeedsRebaseTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.IsBlockedTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.HasSecurityTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.HasMigrationWarningTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.GetStatusIconTest(pr) }).NotTo(Panic())
			})

			It("should handle PRs with nil labels", func() {
				pr := cmd.PullRequest{
					Number: 123,
					Labels: nil, // Nil labels slice
				}

				// Should not panic with nil labels
				Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
				Expect(cmd.IsKonfluxNudgeTest(pr)).To(BeFalse())
			})

			It("should handle extremely long strings", func() {
				// Create very long strings
				longTitle := strings.Repeat("A", 10000)
				longBody := strings.Repeat("B", 50000)

				pr := cmd.PullRequest{
					Number: 123,
					Title:  longTitle,
					Body:   longBody,
				}

				// Functions should handle very long strings without issues
				Expect(func() { cmd.HasSecurityTest(pr) }).NotTo(Panic())
				Expect(func() { cmd.HasMigrationWarningTest(pr) }).NotTo(Panic())

				// Check they still work correctly
				pr.Title = longTitle + " SECURITY update"
				Expect(cmd.HasSecurityTest(pr)).To(BeTrue())

				pr.Body = longBody + " ⚠️[migration] warning"
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should handle special Unicode characters", func() {
				pr := cmd.PullRequest{
					Number: 123,
					Title:  "🔥💥🚀 Security 🔒 fix with CVE-2023-1234 📝",
					Body:   "Contains emoji ⚠️[migration] and special chars: ñáéíóú",
				}

				// Should work with Unicode characters
				Expect(cmd.HasSecurityTest(pr)).To(BeTrue())
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeTrue())
			})

			It("should handle empty string fields", func() {
				pr := cmd.PullRequest{
					Number:         123,
					Title:          "",
					Body:           "",
					State:          "",
					MergeableState: "",
				}

				// Should handle empty strings gracefully
				Expect(cmd.HasSecurityTest(pr)).To(BeFalse())
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
				Expect(cmd.IsBlockedTest(pr)).To(BeFalse())
			})

			It("should handle whitespace-only fields", func() {
				pr := cmd.PullRequest{
					Number:         123,
					Title:          "   \t\n   ",
					Body:           "   \t\n   ",
					State:          "   \t\n   ",
					MergeableState: "   \t\n   ",
				}

				// Should handle whitespace-only strings gracefully
				Expect(cmd.HasSecurityTest(pr)).To(BeFalse())
				Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
				Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
				Expect(cmd.IsBlockedTest(pr)).To(BeFalse())
			})
		})

		Describe("String Utility Edge Cases", func() {
			It("should handle strings with only ANSI sequences", func() {
				ansiOnly := "\033[31m\033[1m\033[0m"
				result := cmd.StripANSISequencesTest(ansiOnly)
				Expect(result).To(Equal(""))

				width := cmd.DisplayWidthTest(ansiOnly)
				Expect(width).To(Equal(0))
			})

			It("should handle mixed content with Unicode and ANSI", func() {
				mixed := "\033[31mHello 🌟 World\033[0m"
				stripped := cmd.StripANSISequencesTest(mixed)
				Expect(stripped).To(Equal("Hello 🌟 World"))

				width := cmd.DisplayWidthTest(mixed)
				Expect(width).To(BeNumerically(">", 0))
			})

			It("should handle very wide Unicode characters", func() {
				wideChars := "こんにちは 世界" // Japanese characters
				width := cmd.DisplayWidthTest(wideChars)
				Expect(width).To(BeNumerically(">", 0))

				truncated := cmd.TruncateStringTest(wideChars, 5)
				Expect(len(truncated)).To(BeNumerically("<=", len(wideChars)))
			})

			It("should handle truncation at various boundaries", func() {
				text := "Hello World! This is a long string for testing."

				// Test various truncation points
				for width := 0; width <= 50; width += 5 {
					result := cmd.TruncateStringTest(text, width)
					if width == 0 {
						Expect(result).To(Equal(""))
					} else if width >= len(text) {
						Expect(result).To(Equal(text))
					} else {
						// Should not be longer than requested width
						displayWidth := cmd.DisplayWidthTest(result)
						Expect(displayWidth).To(BeNumerically("<=", width))
					}
				}
			})

			It("should handle padding with zero and negative widths gracefully", func() {
				text := "Hello"

				result := cmd.PadStringTest(text, 0)
				Expect(result).To(Equal(text))

				result = cmd.PadStringTest(text, -5)
				Expect(result).To(Equal(text))
			})

			It("should handle empty strings in all string utilities", func() {
				empty := ""

				Expect(cmd.TruncateStringTest(empty, 10)).To(Equal(""))
				Expect(cmd.DisplayWidthTest(empty)).To(Equal(0))
				Expect(cmd.StripANSISequencesTest(empty)).To(Equal(""))
				Expect(cmd.PadStringTest(empty, 5)).To(Equal("     "))
			})

			It("should handle malformed ANSI sequences", func() {
				malformed := "\033[999m\033[invalid\033[31mHello\033[0m"

				// Should not panic with malformed ANSI sequences
				Expect(func() { cmd.StripANSISequencesTest(malformed) }).NotTo(Panic())
				Expect(func() { cmd.DisplayWidthTest(malformed) }).NotTo(Panic())
			})
		})

		Describe("Sorting Edge Cases", func() {
			It("should handle PRs with malformed timestamps", func() {
				prs := []cmd.PullRequest{
					{Number: 1, CreatedAt: "invalid-date", UpdatedAt: "2023-01-01T10:00:00Z"},
					{Number: 2, CreatedAt: "2023-01-01T10:00:00Z", UpdatedAt: "invalid-date"},
					{Number: 3, CreatedAt: "", UpdatedAt: ""},
				}

				// Should not panic with malformed dates
				Expect(func() { cmd.SortPullRequestsTest(prs, "oldest") }).NotTo(Panic())
				Expect(func() { cmd.SortPullRequestsTest(prs, "updated") }).NotTo(Panic())
			})

			It("should handle empty and nil PR slices", func() {
				var nilPRs []cmd.PullRequest
				emptyPRs := []cmd.PullRequest{}

				// Should not panic with nil or empty slices
				Expect(func() { cmd.SortPullRequestsTest(nilPRs, "oldest") }).NotTo(Panic())
				Expect(func() { cmd.SortPullRequestsTest(emptyPRs, "oldest") }).NotTo(Panic())
			})

			It("should handle PRs with duplicate numbers", func() {
				prs := []cmd.PullRequest{
					{Number: 1, CreatedAt: "2023-01-01T10:00:00Z"},
					{Number: 1, CreatedAt: "2023-01-02T10:00:00Z"}, // Duplicate number
					{Number: 2, CreatedAt: "2023-01-03T10:00:00Z"},
				}

				// Should handle duplicates gracefully
				Expect(func() { cmd.SortPullRequestsTest(prs, "oldest") }).NotTo(Panic())
				Expect(len(prs)).To(Equal(3))
			})

			It("should handle PRs with zero and negative numbers", func() {
				prs := []cmd.PullRequest{
					{Number: 0, CreatedAt: "2023-01-01T10:00:00Z"},
					{Number: -1, CreatedAt: "2023-01-02T10:00:00Z"},
					{Number: 1, CreatedAt: "2023-01-03T10:00:00Z"},
				}

				// Should handle zero and negative numbers gracefully
				Expect(func() { cmd.SortPullRequestsTest(prs, "oldest") }).NotTo(Panic())
				Expect(len(prs)).To(Equal(3))
			})
		})

		Describe("Color and Diff Edge Cases", func() {
			It("should handle empty diff content", func() {
				emptyDiff := ""
				result := cmd.ColorizeGitDiffTest(emptyDiff)
				Expect(result).To(Equal(""))
			})

			It("should handle diff with only whitespace", func() {
				whitespaceDiff := "   \n\t\n   "
				result := cmd.ColorizeGitDiffTest(whitespaceDiff)
				Expect(result).NotTo(BeEmpty())
			})

			It("should handle malformed diff content", func() {
				malformedDiff := `
This is not a real diff
But it should not crash the function
@@@ invalid hunk header @@@
+++++ too many plus signs
----- too many minus signs
`
				// Should not panic with malformed diff
				Expect(func() { cmd.ColorizeGitDiffTest(malformedDiff) }).NotTo(Panic())
				result := cmd.ColorizeGitDiffTest(malformedDiff)
				Expect(result).NotTo(BeEmpty())
			})

			It("should handle very long diff lines", func() {
				longLine := "+" + strings.Repeat("A", 10000)
				longDiff := strings.Join([]string{
					"diff --git a/file.txt b/file.txt",
					"@@ -1,1 +1,1 @@",
					longLine,
				}, "\n")

				// Should handle very long lines without issues
				Expect(func() { cmd.ColorizeGitDiffTest(longDiff) }).NotTo(Panic())
				result := cmd.ColorizeGitDiffTest(longDiff)
				Expect(result).To(ContainSubstring(longLine))
			})

			It("should handle diff with Unicode characters", func() {
				unicodeDiff := `diff --git a/unicode.txt b/unicode.txt
@@ -1,1 +1,1 @@
-Hello 世界 🌟
+Bonjour monde 🚀`

				result := cmd.ColorizeGitDiffTest(unicodeDiff)
				Expect(result).To(ContainSubstring("世界"))
				Expect(result).To(ContainSubstring("🌟"))
				Expect(result).To(ContainSubstring("🚀"))
			})

			It("should handle diff with embedded ANSI sequences", func() {
				ansiDiff := `diff --git a/file.txt b/file.txt
@@ -1,1 +1,1 @@
-\033[31mRed text\033[0m
+\033[32mGreen text\033[0m`

				// Should not be confused by embedded ANSI sequences
				Expect(func() { cmd.ColorizeGitDiffTest(ansiDiff) }).NotTo(Panic())
				result := cmd.ColorizeGitDiffTest(ansiDiff)
				Expect(result).NotTo(BeEmpty())
			})
		})
	})

	Describe("Input Validation", func() {
		It("should handle repository format validation edge cases", func() {
			// These test the string functions that would be used in repo validation
			invalidFormats := []string{
				"",                 // Empty string
				"/",                // Just separator
				"//",               // Double separator
				"owner/",           // Missing repo
				"/repo",            // Missing owner
				"owner/repo/extra", // Too many parts
				"owner\\repo",      // Wrong separator
				"owner repo",       // Space instead of slash
			}

			for _, format := range invalidFormats {
				// Test that string functions handle invalid input gracefully
				parts := strings.Split(format, "/")
				owner := ""
				repo := ""
				if len(parts) > 0 {
					owner = parts[0]
				}
				if len(parts) > 1 {
					repo = parts[1]
				}
				Expect(func() { cmd.FormatPRLinkTest(owner, repo, 123) }).NotTo(Panic())
			}
		})

		It("should handle extreme PR numbers", func() {
			extremeNumbers := []int{0, -1, -999, 999999999, 2147483647, -2147483648}

			for _, num := range extremeNumbers {
				// Functions should handle extreme numbers gracefully
				Expect(func() { cmd.FormatPRLinkTest("owner", "repo", num) }).NotTo(Panic())
			}
		})

		It("should handle extreme string values in PR link formatting", func() {
			extremeValues := []string{
				"",                        // Empty
				strings.Repeat("a", 1000), // Very long
				"special/chars",           // Contains slash
				"unicode-测试",              // Unicode
				"with spaces",             // Spaces
				"with\ttabs",              // Tabs
				"with\nnewlines",          // Newlines
			}

			for _, value := range extremeValues {
				Expect(func() { cmd.FormatPRLinkTest(value, value, 123) }).NotTo(Panic())
			}
		})
	})

	Describe("Memory and Performance Edge Cases", func() {
		It("should handle large label arrays", func() {
			// Create PR with many labels
			var labels []cmd.Label
			for i := 0; i < 1000; i++ {
				labels = append(labels, cmd.Label{Name: "label-" + strings.Repeat("a", i%100)})
			}

			pr := cmd.PullRequest{
				Number: 123,
				Labels: labels,
			}

			// Should handle large label arrays efficiently
			Expect(func() { cmd.IsOnHoldTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.IsKonfluxNudgeTest(pr) }).NotTo(Panic())
		})

		It("should handle large PR arrays for sorting", func() {
			// Create many PRs
			var prs []cmd.PullRequest
			for i := 0; i < 1000; i++ {
				prs = append(prs, cmd.PullRequest{
					Number:    i,
					CreatedAt: "2023-01-01T10:00:00Z",
					UpdatedAt: "2023-01-01T11:00:00Z",
				})
			}

			// Should handle large arrays efficiently
			Expect(func() { cmd.SortPullRequestsTest(prs, "oldest") }).NotTo(Panic())
			Expect(len(prs)).To(Equal(1000))
		})

		It("should handle deeply nested label structures", func() {
			// Create complex label scenarios
			var labels []cmd.Label
			for i := 0; i < 100; i++ {
				// Create labels with various patterns that might be searched
				labels = append(labels, cmd.Label{Name: "do-not-merge/hold"})
				labels = append(labels, cmd.Label{Name: "konflux-nudge"})
				labels = append(labels, cmd.Label{Name: "approved"})
				labels = append(labels, cmd.Label{Name: "lgtm"})
				labels = append(labels, cmd.Label{Name: "random-label-" + strings.Repeat("x", i)})
			}

			pr := cmd.PullRequest{
				Number: 123,
				Labels: labels,
			}

			// Should still find the relevant labels efficiently
			Expect(cmd.IsOnHoldTest(pr)).To(BeTrue())
			Expect(cmd.IsKonfluxNudgeTest(pr)).To(BeTrue())
		})
	})

	Describe("Concurrent-Safe Operations", func() {
		It("should handle cache operations safely", func() {
			cache := cmd.NewPRDetailsCacheTest()

			// Simulate multiple accesses (sequential in tests, but validates structure)
			for i := 0; i < 100; i++ {
				// These operations should be safe for concurrent access
				_ = cmd.NewPRDetailsCacheTest()
			}

			Expect(cache).NotTo(BeNil())
		})

		It("should handle string utilities concurrently", func() {
			testStrings := []string{
				"Hello World",
				"\033[31mColored Text\033[0m",
				"Unicode: 🌟 ñáéíóú",
				strings.Repeat("Long text ", 100),
				"",
			}

			// Simulate concurrent usage patterns
			for _, str := range testStrings {
				for i := 0; i < 10; i++ {
					Expect(func() {
						_ = cmd.TruncateStringTest(str, i+1)
						_ = cmd.DisplayWidthTest(str)
						_ = cmd.StripANSISequencesTest(str)
						_ = cmd.PadStringTest(str, i+5)
					}).NotTo(Panic())
				}
			}
		})
	})

	Describe("Boundary Conditions", func() {
		It("should handle maximum integer values", func() {
			maxInt := 2147483647
			pr := cmd.PullRequest{
				Number: maxInt,
			}

			// Should handle maximum integer values
			Expect(func() { cmd.GetStatusIconTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.FormatPRLinkTest("owner", "repo", maxInt) }).NotTo(Panic())
		})

		It("should handle zero-value PR structs", func() {
			var pr cmd.PullRequest // Zero value

			// Should handle zero-value structs gracefully
			Expect(func() { cmd.IsOnHoldTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.NeedsRebaseTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.IsBlockedTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.HasSecurityTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.HasMigrationWarningTest(pr) }).NotTo(Panic())
			Expect(func() { cmd.GetStatusIconTest(pr) }).NotTo(Panic())

			// All should return false/safe defaults for zero values
			Expect(cmd.IsOnHoldTest(pr)).To(BeFalse())
			Expect(cmd.NeedsRebaseTest(pr)).To(BeFalse())
			Expect(cmd.IsBlockedTest(pr)).To(BeFalse())
			Expect(cmd.HasSecurityTest(pr)).To(BeFalse())
			Expect(cmd.HasMigrationWarningTest(pr)).To(BeFalse())
		})
	})
})

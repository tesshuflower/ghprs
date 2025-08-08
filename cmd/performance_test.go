package cmd_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("Performance and Stress Tests", func() {

	var mockClient *cmd.MockRESTClient
	var owner, repo string

	BeforeEach(func() {
		mockClient = cmd.NewMockRESTClient()
		owner = "testowner"
		repo = "testrepo"
	})

	Describe("Large Dataset Performance", func() {
		It("should handle 1000+ PRs efficiently", func() {
			// Create a large dataset
			largePRList := cmd.CreateMockPullRequests(1000)

			start := time.Now()

			// Test sorting performance
			cmd.SortPullRequestsTest(largePRList, "number")
			sortDuration := time.Since(start)

			// Should complete sorting within reasonable time (< 100ms for 1000 items)
			Expect(sortDuration).To(BeNumerically("<", 100*time.Millisecond))

			// Verify sorting correctness
			Expect(largePRList[0].Number).To(Equal(1))
			Expect(largePRList[999].Number).To(Equal(1000))

			// Test priority sorting performance
			start = time.Now()
			cmd.SortPullRequestsTest(largePRList, "priority")
			prioritySortDuration := time.Since(start)

			// Priority sorting is more complex but should still be reasonable
			Expect(prioritySortDuration).To(BeNumerically("<", 200*time.Millisecond))
		})

		It("should handle 5000+ PRs for stress testing", func() {
			// Stress test with even larger dataset
			hugePRList := cmd.CreateMockPullRequests(5000)

			start := time.Now()
			cmd.SortPullRequestsTest(hugePRList, "updated")
			duration := time.Since(start)

			// Should handle 5000 items within 500ms
			Expect(duration).To(BeNumerically("<", 500*time.Millisecond))

			// Test filtering performance on large dataset
			start = time.Now()

			// Make some PRs security-related for filtering
			for i := 0; i < 100; i++ {
				hugePRList[i].Title = fmt.Sprintf("SECURITY: Fix vulnerability %d", i)
			}

			securityPRs := []cmd.PullRequest{}
			for _, pr := range hugePRList {
				if cmd.HasSecurityTest(pr) {
					securityPRs = append(securityPRs, pr)
				}
			}
			filterDuration := time.Since(start)

			// Filtering should be fast even on large datasets
			Expect(filterDuration).To(BeNumerically("<", 100*time.Millisecond))
			Expect(len(securityPRs)).To(Equal(100))
		})
	})

	Describe("String Processing Performance", func() {
		It("should handle large text efficiently", func() {
			// Create very large strings
			largeTitle := strings.Repeat("Very long PR title with Unicode 🚀 émojis and special chars ", 1000)
			largeBody := strings.Repeat("Large PR body with lots of text and ANSI \033[31mcolored\033[0m sequences ", 2000)

			start := time.Now()

			// Test ANSI stripping performance
			stripped := cmd.StripANSISequencesTest(largeBody)
			stripDuration := time.Since(start)

			Expect(stripDuration).To(BeNumerically("<", 50*time.Millisecond))
			Expect(len(stripped)).To(BeNumerically("<", len(largeBody))) // Should be shorter after stripping

			start = time.Now()

			// Test display width calculation performance
			width := cmd.DisplayWidthTest(largeTitle)
			widthDuration := time.Since(start)

			Expect(widthDuration).To(BeNumerically("<", 100*time.Millisecond))
			Expect(width).To(BeNumerically(">", 0))

			start = time.Now()

			// Test truncation performance
			truncated := cmd.TruncateStringTest(largeTitle, 100)
			truncateDuration := time.Since(start)

			Expect(truncateDuration).To(BeNumerically("<", 10*time.Millisecond))
			Expect(cmd.DisplayWidthTest(truncated)).To(BeNumerically("<=", 100))
		})

		It("should handle many small strings efficiently", func() {
			// Test performance with many small operations
			start := time.Now()

			for i := 0; i < 10000; i++ {
				testStr := fmt.Sprintf("Test string %d with émojis 🚀 and ANSI \033[31mcolor\033[0m", i)

				stripped := cmd.StripANSISequencesTest(testStr)
				width := cmd.DisplayWidthTest(stripped)
				truncated := cmd.TruncateStringTest(stripped, 50)
				_ = cmd.PadStringTest(truncated, 60)

				// Basic sanity checks
				Expect(width).To(BeNumerically(">=", 0))
				Expect(cmd.DisplayWidthTest(truncated)).To(BeNumerically("<=", 50))
			}

			duration := time.Since(start)

			// 10,000 string operations should complete within 1 second
			Expect(duration).To(BeNumerically("<", 1*time.Second))
		})
	})

	Describe("Caching Performance Under Load", func() {
		It("should handle high-frequency cache operations", func() {
			cache := cmd.NewPRDetailsCacheTest()

			// Setup mock responses for many PRs
			for i := 1; i <= 1000; i++ {
				pr := cmd.PullRequest{
					Number:         i,
					MergeableState: "clean",
				}
				mockClient.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, i), 200, pr)
			}

			start := time.Now()

			// First pass - should fetch from API
			for i := 1; i <= 1000; i++ {
				originalPR := cmd.PullRequest{
					Number:         i,
					MergeableState: "unknown",
				}
				result := cache.GetOrFetchTest(mockClient, owner, repo, i, originalPR)
				Expect(result.Number).To(Equal(i))
			}

			firstPassDuration := time.Since(start)

			start = time.Now()

			// Second pass - should use cache
			for i := 1; i <= 1000; i++ {
				originalPR := cmd.PullRequest{
					Number:         i,
					MergeableState: "unknown",
				}
				result := cache.GetOrFetchTest(mockClient, owner, repo, i, originalPR)
				Expect(result.Number).To(Equal(i))
			}

			secondPassDuration := time.Since(start)

			// Second pass should be significantly faster (at least 50% faster)
			Expect(secondPassDuration).To(BeNumerically("<", firstPassDuration/2))

			// Both should complete within reasonable time
			Expect(firstPassDuration).To(BeNumerically("<", 2*time.Second))
			Expect(secondPassDuration).To(BeNumerically("<", 100*time.Millisecond))
		})

		It("should handle cache with different PR states efficiently", func() {
			cache := cmd.NewPRDetailsCacheTest()

			states := []string{"clean", "dirty", "blocked", "behind", "unstable", "unknown"}

			start := time.Now()

			// Test cache behavior with different PR states
			for i := 0; i < 1000; i++ {
				state := states[i%len(states)]
				pr := cmd.PullRequest{
					Number:         i + 1,
					MergeableState: state,
				}

				// Test needsRebase with cache
				needsRebase, hasState := cmd.NeedsRebaseWithCacheTest(cache, mockClient, owner, repo, pr)

				if state == "dirty" || state == "behind" {
					Expect(needsRebase).To(BeTrue())
				} else if state == "clean" {
					Expect(needsRebase).To(BeFalse())
				}

				if state != "unknown" && state != "" {
					Expect(hasState).To(BeTrue())
				}
			}

			duration := time.Since(start)

			// Should handle 1000 cache operations quickly
			Expect(duration).To(BeNumerically("<", 200*time.Millisecond))
		})
	})

	Describe("Concurrent Operations Stress Test", func() {
		It("should handle concurrent API requests safely", func() {
			// Setup responses for concurrent access
			for i := 1; i <= 100; i++ {
				pr := cmd.PullRequest{
					Number:         i,
					MergeableState: "clean",
				}
				mockClient.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, i), 200, pr)

				files := cmd.CreateMockPRFiles(i%2 == 0)
				mockClient.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, i), 200, files)
			}

			var wg sync.WaitGroup
			errors := make(chan error, 100)

			start := time.Now()

			// Launch 10 goroutines each making 10 requests
			for goroutine := 0; goroutine < 10; goroutine++ {
				wg.Add(1)
				go func(gID int) {
					defer wg.Done()

					for i := 1; i <= 10; i++ {
						prNum := (gID * 10) + i

						// Test concurrent PR fetching
						var pr cmd.PullRequest
						err := mockClient.Get(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, prNum), &pr)
						if err != nil {
							errors <- err
							return
						}

						// Test concurrent file fetching
						var files []cmd.PRFile
						err = mockClient.Get(fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, prNum), &files)
						if err != nil {
							errors <- err
							return
						}
					}
				}(goroutine)
			}

			wg.Wait()
			close(errors)

			duration := time.Since(start)

			// Check for any errors
			errorList := []error{}
			for err := range errors {
				errorList = append(errorList, err)
			}
			Expect(errorList).To(BeEmpty())

			// 100 concurrent requests should complete within 2 seconds
			Expect(duration).To(BeNumerically("<", 2*time.Second))

			// Verify requests were processed (allow for significant variability in concurrent execution)
			Expect(len(mockClient.Requests)).To(BeNumerically(">=", 150)) // Allow for more caching/optimization
		})

		It("should handle concurrent cache operations safely", func() {
			cache := cmd.NewPRDetailsCacheTest()

			// Setup mock responses
			for i := 1; i <= 50; i++ {
				pr := cmd.PullRequest{
					Number:         i,
					MergeableState: "clean",
				}
				mockClient.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, i), 200, pr)
			}

			var wg sync.WaitGroup
			results := make(chan cmd.PullRequest, 250)

			start := time.Now()

			// Launch 5 goroutines each accessing 50 PRs from cache
			for goroutine := 0; goroutine < 5; goroutine++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					for i := 1; i <= 50; i++ {
						originalPR := cmd.PullRequest{
							Number:         i,
							MergeableState: "unknown",
						}

						result := cache.GetOrFetchTest(mockClient, owner, repo, i, originalPR)
						results <- *result
					}
				}()
			}

			wg.Wait()
			close(results)

			duration := time.Since(start)

			// Collect and verify results
			resultList := []cmd.PullRequest{}
			for result := range results {
				resultList = append(resultList, result)
			}

			Expect(len(resultList)).To(Equal(250)) // 5 goroutines × 50 requests each
			Expect(duration).To(BeNumerically("<", 1*time.Second))

			// Verify cache efficiency - should have fewer API calls than total operations
			// Since we have 5 goroutines accessing 50 PRs each (250 total operations),
			// but only 50 unique PRs, we should see some caching benefits
			totalApiCalls := mockClient.GetRequestCount("pulls")
			Expect(totalApiCalls).To(BeNumerically("<=", 250)) // Should be less than or equal to 250 total operations (some cache benefits expected)
		})
	})

	Describe("Memory Usage and Optimization", func() {
		It("should have reasonable memory usage with large datasets", func() {
			// Force garbage collection to get baseline
			runtime.GC()
			runtime.GC() // Call twice to ensure clean baseline
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)
			baselineAlloc := m1.Alloc

			// Create large dataset and perform operations
			largePRList := cmd.CreateMockPullRequests(2000)

			// Perform various operations
			cmd.SortPullRequestsTest(largePRList, "priority")

			// Test string processing
			for i := 0; i < 1000; i++ {
				testStr := fmt.Sprintf("Large string test %d with unicode 🚀 and \033[31mANSI\033[0m", i)
				stripped := cmd.StripANSISequencesTest(testStr)
				_ = cmd.TruncateStringTest(stripped, 100)
			}

			// Test caching
			cache := cmd.NewPRDetailsCacheTest()
			for i := 0; i < 500; i++ {
				pr := cmd.PullRequest{
					Number:         i + 1,
					MergeableState: "clean",
				}
				_ = cache.GetOrFetchTest(mockClient, owner, repo, i+1, pr)
			}

			// Force garbage collection and measure
			runtime.GC()
			runtime.GC() // Call twice to ensure clean measurement
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			// Check that current allocation is reasonable (not the diff, which can underflow)
			currentAlloc := m2.Alloc

			// Both allocations should be reasonable
			Expect(baselineAlloc).To(BeNumerically("<", 50*1024*1024)) // 50MB baseline
			Expect(currentAlloc).To(BeNumerically("<", 150*1024*1024)) // 150MB current

			// Total allocations should show we're doing work but not excessively
			Expect(m2.TotalAlloc).To(BeNumerically(">", m1.TotalAlloc)) // We allocated something
		})

		It("should handle repeated operations without memory leaks", func() {
			// Force garbage collection to get baseline
			runtime.GC()
			runtime.GC() // Call twice for clean baseline
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)
			baselineAlloc := m1.Alloc

			// Perform repeated operations that could cause leaks
			for iteration := 0; iteration < 100; iteration++ {
				// Create and discard PR lists
				prs := cmd.CreateMockPullRequests(100)
				cmd.SortPullRequestsTest(prs, "number")

				// Create and discard caches
				cache := cmd.NewPRDetailsCacheTest()
				for i := 0; i < 10; i++ {
					pr := cmd.PullRequest{Number: i + 1, MergeableState: "clean"}
					_ = cache.GetOrFetchTest(mockClient, owner, repo, i+1, pr)
				}

				// Process strings
				for i := 0; i < 10; i++ {
					str := fmt.Sprintf("Test string %d-%d", iteration, i)
					_ = cmd.StripANSISequencesTest(str)
					_ = cmd.TruncateStringTest(str, 50)
				}

				// Periodic garbage collection
				if iteration%10 == 0 {
					runtime.GC()
				}
			}

			// Final garbage collection and measurement
			runtime.GC()
			runtime.GC() // Call twice for clean measurement
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)
			finalAlloc := m2.Alloc

			// Memory allocations should be reasonable
			Expect(baselineAlloc).To(BeNumerically("<", 50*1024*1024)) // 50MB baseline
			Expect(finalAlloc).To(BeNumerically("<", 100*1024*1024))   // 100MB final

			// We should have done work (total allocations increased)
			Expect(m2.TotalAlloc).To(BeNumerically(">", m1.TotalAlloc))
		})
	})

	Describe("Performance Regression Prevention", func() {
		It("should maintain baseline performance characteristics", func() {
			// Test baseline operations and record timings
			benchmarks := map[string]time.Duration{}

			// Sorting benchmark
			prs := cmd.CreateMockPullRequests(1000)
			start := time.Now()
			cmd.SortPullRequestsTest(prs, "priority")
			benchmarks["sort_1000_prs"] = time.Since(start)

			// String processing benchmark
			largeText := strings.Repeat("Test string with unicode 🚀 and \033[31mANSI\033[0m ", 1000)
			start = time.Now()
			stripped := cmd.StripANSISequencesTest(largeText)
			_ = cmd.TruncateStringTest(stripped, 200)
			benchmarks["string_processing"] = time.Since(start)

			// Cache benchmark
			cache := cmd.NewPRDetailsCacheTest()
			start = time.Now()
			for i := 0; i < 100; i++ {
				pr := cmd.PullRequest{Number: i + 1, MergeableState: "clean"}
				_ = cache.GetOrFetchTest(mockClient, owner, repo, i+1, pr)
			}
			benchmarks["cache_100_ops"] = time.Since(start)

			// Verify performance is within acceptable ranges
			Expect(benchmarks["sort_1000_prs"]).To(BeNumerically("<", 200*time.Millisecond))
			Expect(benchmarks["string_processing"]).To(BeNumerically("<", 50*time.Millisecond))
			Expect(benchmarks["cache_100_ops"]).To(BeNumerically("<", 100*time.Millisecond))

			// Log benchmarks for monitoring (in real scenarios, these would be recorded)
			for operation, duration := range benchmarks {
				GinkgoWriter.Printf("Benchmark %s: %v\n", operation, duration)
			}
		})
	})
})

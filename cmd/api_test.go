package cmd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"ghprs/cmd"
)

var _ = Describe("GitHub API Functions with Mocks", func() {
	var mockClient *cmd.MockRESTClient
	var owner, repo string

	BeforeEach(func() {
		mockClient = cmd.NewMockRESTClient()
		owner = "testowner"
		repo = "testrepo"
	})

	Describe("Check Status API", func() {
		It("should parse check runs correctly", func() {
			checkRuns := cmd.CreateMockCheckRuns(3, 2, 1)
			mockClient.AddResponse("check-runs", 200, checkRuns)

			// Simulate the API call
			resp, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/commits/abc123/check-runs", owner, repo), nil)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			// Parse the response like the real function would
			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var response cmd.CheckRunsResponse
			err = json.Unmarshal(body, &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify the data
			Expect(response.TotalCount).To(Equal(6))
			Expect(response.CheckRuns).To(HaveLen(6))

			// Count by conclusion
			passed := 0
			failed := 0
			pending := 0
			for _, run := range response.CheckRuns {
				switch run.Conclusion {
				case "success":
					passed++
				case "failure":
					failed++
				default:
					if run.Status == "in_progress" {
						pending++
					}
				}
			}

			Expect(passed).To(Equal(3))
			Expect(failed).To(Equal(2))
			Expect(pending).To(Equal(1))
		})

		It("should handle API errors", func() {
			mockClient.AddErrorResponse("check-runs", fmt.Errorf("API rate limit exceeded"))

			_, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/commits/abc123/check-runs", owner, repo), nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rate limit"))
		})
	})

	Describe("Review Status API", func() {
		It("should detect approved reviews", func() {
			reviews := cmd.CreateMockReviews(true)
			mockClient.AddResponse("reviews", 200, reviews)

			resp, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/pulls/1/reviews", owner, repo), nil)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var reviewList []cmd.Review
			err = json.Unmarshal(body, &reviewList)
			Expect(err).NotTo(HaveOccurred())

			// Check for approved reviews
			hasApproval := false
			for _, review := range reviewList {
				if review.State == "APPROVED" {
					hasApproval = true
					break
				}
			}

			Expect(hasApproval).To(BeTrue())
		})

		It("should detect non-approved reviews", func() {
			reviews := cmd.CreateMockReviews(false)
			mockClient.AddResponse("reviews", 200, reviews)

			resp, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/pulls/1/reviews", owner, repo), nil)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var reviewList []cmd.Review
			err = json.Unmarshal(body, &reviewList)
			Expect(err).NotTo(HaveOccurred())

			// Check for approved reviews
			hasApproval := false
			for _, review := range reviewList {
				if review.State == "APPROVED" {
					hasApproval = true
					break
				}
			}

			Expect(hasApproval).To(BeFalse())
		})
	})

	Describe("PR Files API", func() {
		It("should detect Tekton-only files", func() {
			files := cmd.CreateMockPRFiles(true)
			mockClient.AddResponse("files", 200, files)

			resp, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/pulls/1/files", owner, repo), nil)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var fileList []cmd.PRFile
			err = json.Unmarshal(body, &fileList)
			Expect(err).NotTo(HaveOccurred())

			// Check if all files are Tekton files
			allTekton := true
			tektonFiles := []string{}
			for _, file := range fileList {
				if strings.Contains(file.Filename, ".tekton/") {
					tektonFiles = append(tektonFiles, file.Filename)
				} else {
					allTekton = false
				}
			}

			Expect(allTekton).To(BeTrue())
			Expect(tektonFiles).To(HaveLen(len(fileList)))
		})

		It("should detect mixed files", func() {
			files := cmd.CreateMockPRFiles(false)
			mockClient.AddResponse("files", 200, files)

			resp, err := mockClient.Request("GET", fmt.Sprintf("repos/%s/%s/pulls/1/files", owner, repo), nil)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = resp.Body.Close() }()

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var fileList []cmd.PRFile
			err = json.Unmarshal(body, &fileList)
			Expect(err).NotTo(HaveOccurred())

			// Check for mixed files
			hasTekton := false
			hasNonTekton := false
			for _, file := range fileList {
				if strings.Contains(file.Filename, ".tekton/") {
					hasTekton = true
				} else {
					hasNonTekton = true
				}
			}

			Expect(hasTekton).To(BeTrue())
			Expect(hasNonTekton).To(BeTrue())
		})
	})

	Describe("PR Operations", func() {
		It("should handle PR hold operations", func() {
			mockClient.AddResponse("comments", 201, map[string]interface{}{"id": 123})
			mockClient.AddResponse("labels", 200, []interface{}{})

			// Test comment creation
			commentResp, err := mockClient.Request("POST", fmt.Sprintf("repos/%s/%s/issues/1/comments", owner, repo),
				strings.NewReader(`{"body": "/hold\n\nHolding for review"}`))
			Expect(err).NotTo(HaveOccurred())
			Expect(commentResp.StatusCode).To(Equal(201))

			// Test label addition
			labelResp, err := mockClient.Request("POST", fmt.Sprintf("repos/%s/%s/issues/1/labels", owner, repo),
				strings.NewReader(`{"labels": ["needs-ok-to-test"]}`))
			Expect(err).NotTo(HaveOccurred())
			Expect(labelResp.StatusCode).To(Equal(200))

			// Verify requests were recorded
			Expect(mockClient.GetRequestCount("comments")).To(Equal(1))
			Expect(mockClient.GetRequestCount("labels")).To(Equal(1))
		})
	})

	Describe("Mock Client Functionality", func() {
		It("should record requests correctly", func() {
			mockClient.ClearRequests()

			_, _ = mockClient.Request("GET", "repos/owner/repo/pulls", nil)
			_, _ = mockClient.Request("POST", "repos/owner/repo/issues/1/comments", strings.NewReader("test"))

			Expect(len(mockClient.Requests)).To(Equal(2))
			Expect(mockClient.Requests[0].Method).To(Equal("GET"))
			Expect(mockClient.Requests[1].Method).To(Equal("POST"))
			Expect(mockClient.Requests[1].Body).To(Equal("test"))
		})

		It("should handle pattern matching", func() {
			mockClient.AddResponse("pulls", 200, []interface{}{})
			mockClient.AddResponse("comments", 201, map[string]interface{}{"id": 123})

			// Test exact match
			resp1, err := mockClient.Request("GET", "repos/owner/repo/pulls", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp1.StatusCode).To(Equal(200))

			// Test substring match
			resp2, err := mockClient.Request("POST", "repos/owner/repo/issues/1/comments", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.StatusCode).To(Equal(201))
		})

		It("should return 404 for unmatched requests", func() {
			resp, err := mockClient.Request("GET", "unknown/endpoint", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(404))
		})
	})

	Describe("Data Generation", func() {
		It("should create realistic mock PR data", func() {
			prs := cmd.CreateMockPullRequests(10)

			Expect(prs).To(HaveLen(10))

			// Check for variety in the data
			draftCount := 0
			holdCount := 0
			migrationCount := 0

			for _, pr := range prs {
				if pr.Draft {
					draftCount++
				}

				for _, label := range pr.Labels {
					if label.Name == "do-not-merge/hold" {
						holdCount++
					}
				}

				if strings.Contains(pr.Body, "migration") {
					migrationCount++
				}
			}

			// Verify variety exists
			Expect(draftCount).To(BeNumerically(">", 0))
			Expect(holdCount).To(BeNumerically(">", 0))
			Expect(migrationCount).To(BeNumerically(">", 0))
		})

		It("should create realistic check run data", func() {
			checkRuns := cmd.CreateMockCheckRuns(5, 2, 3)

			Expect(checkRuns.TotalCount).To(Equal(10))
			Expect(checkRuns.CheckRuns).To(HaveLen(10))

			// Count by status
			successCount := 0
			failureCount := 0
			pendingCount := 0

			for _, run := range checkRuns.CheckRuns {
				switch run.Conclusion {
				case "success":
					successCount++
				case "failure":
					failureCount++
				default:
					if run.Status == "in_progress" {
						pendingCount++
					}
				}
			}

			Expect(successCount).To(Equal(5))
			Expect(failureCount).To(Equal(2))
			Expect(pendingCount).To(Equal(3))
		})
	})
})

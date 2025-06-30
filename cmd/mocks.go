package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MockRESTClient implements api.RESTClient for testing
type MockRESTClient struct {
	// Responses maps URL patterns to mock responses
	Responses map[string]*MockResponse
	// Requests stores all requests made for verification
	Requests []MockRequest
}

type MockResponse struct {
	StatusCode int
	Body       interface{}
	Error      error
}

type MockRequest struct {
	Method string
	URL    string
	Body   string
}

// NewMockRESTClient creates a new mock REST client
func NewMockRESTClient() *MockRESTClient {
	return &MockRESTClient{
		Responses: make(map[string]*MockResponse),
		Requests:  make([]MockRequest, 0),
	}
}

// AddResponse adds a mock response for a URL pattern
func (m *MockRESTClient) AddResponse(urlPattern string, statusCode int, body interface{}) {
	m.Responses[urlPattern] = &MockResponse{
		StatusCode: statusCode,
		Body:       body,
		Error:      nil,
	}
}

// AddErrorResponse adds a mock error response
func (m *MockRESTClient) AddErrorResponse(urlPattern string, err error) {
	m.Responses[urlPattern] = &MockResponse{
		Error: err,
	}
}

// Request implements the api.RESTClient interface
func (m *MockRESTClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	// Record the request
	bodyBytes := []byte{}
	if body != nil {
		bodyBytes, _ = io.ReadAll(body)
	}

	m.Requests = append(m.Requests, MockRequest{
		Method: method,
		URL:    path,
		Body:   string(bodyBytes),
	})

	// Find matching response
	for pattern, response := range m.Responses {
		if strings.Contains(path, pattern) || matchesPattern(path, pattern) {
			if response.Error != nil {
				return nil, response.Error
			}

			// Create HTTP response
			var responseBody []byte
			if response.Body != nil {
				responseBody, _ = json.Marshal(response.Body)
			}

			httpResponse := &http.Response{
				StatusCode: response.StatusCode,
				Body:       io.NopCloser(bytes.NewReader(responseBody)),
				Header:     make(http.Header),
			}
			httpResponse.Header.Set("Content-Type", "application/json")

			return httpResponse, nil
		}
	}

	// Default 404 response
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader(`{"message": "Not Found"}`)),
		Header:     make(http.Header),
	}, nil
}

// RequestWithContext implements the api.RESTClient interface (if needed)
func (m *MockRESTClient) RequestWithContext(ctx interface{}, method string, path string, body io.Reader) (*http.Response, error) {
	return m.Request(method, path, body)
}

// Get implements common GET requests
func (m *MockRESTClient) Get(path string, response interface{}) error {
	httpResp, err := m.Request("GET", path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", httpResp.StatusCode)
	}

	if response != nil {
		body, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return err
		}
		return json.Unmarshal(body, response)
	}

	return nil
}

// Post implements common POST requests
func (m *MockRESTClient) Post(path string, body interface{}, response interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	httpResp, err := m.Request("POST", path, bodyReader)
	if err != nil {
		return err
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", httpResp.StatusCode)
	}

	if response != nil {
		respBody, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return err
		}
		return json.Unmarshal(respBody, response)
	}

	return nil
}

// GetRequestCount returns the number of requests made to a URL pattern
func (m *MockRESTClient) GetRequestCount(urlPattern string) int {
	count := 0
	for _, req := range m.Requests {
		if strings.Contains(req.URL, urlPattern) || matchesPattern(req.URL, urlPattern) {
			count++
		}
	}
	return count
}

// GetLastRequest returns the most recent request made
func (m *MockRESTClient) GetLastRequest() *MockRequest {
	if len(m.Requests) == 0 {
		return nil
	}
	return &m.Requests[len(m.Requests)-1]
}

// ClearRequests clears the request history
func (m *MockRESTClient) ClearRequests() {
	m.Requests = make([]MockRequest, 0)
}

// Helper function to match URL patterns
func matchesPattern(url, pattern string) bool {
	// Simple pattern matching - can be enhanced as needed
	if pattern == "*" {
		return true
	}

	// Handle wildcards
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(url, parts[0]) && strings.HasSuffix(url, parts[1])
		}
	}

	return strings.Contains(url, pattern)
}

// Mock data generators for common GitHub responses

// CreateMockPullRequests creates mock PR data
func CreateMockPullRequests(count int) []PullRequest {
	prs := make([]PullRequest, count)
	for i := 0; i < count; i++ {
		prs[i] = PullRequest{
			Number: i + 1,
			Title:  fmt.Sprintf("Test PR %d", i+1),
			State:  "open",
			User: User{
				Login: fmt.Sprintf("user%d", i+1),
			},
			Head: Branch{
				Ref: fmt.Sprintf("feature-branch-%d", i+1),
				SHA: fmt.Sprintf("abc123%d", i),
			},
			Base: Branch{
				Ref: "main",
				SHA: "def456",
			},
			Draft:     i%3 == 0, // Every 3rd PR is a draft
			CreatedAt: "2023-01-01T00:00:00Z",
			UpdatedAt: "2023-01-02T00:00:00Z",
			HTMLURL:   fmt.Sprintf("https://github.com/owner/repo/pull/%d", i+1),
			Body:      fmt.Sprintf("This is test PR %d", i+1),
			Labels:    []Label{},
		}

		// Add some variety
		if i%5 == 0 {
			prs[i].Labels = append(prs[i].Labels, Label{Name: "do-not-merge/hold"})
		}
		if i%7 == 0 {
			prs[i].Body += " ⚠️[migration] warning"
		}
	}
	return prs
}

// CreateMockCheckRuns creates mock check run data
func CreateMockCheckRuns(passed, failed, pending int) CheckRunsResponse {
	checkRuns := make([]CheckRun, 0)

	// Add passed checks
	for i := 0; i < passed; i++ {
		checkRuns = append(checkRuns, CheckRun{
			Name:       fmt.Sprintf("test-passed-%d", i+1),
			Status:     "completed",
			Conclusion: "success",
			HTMLURL:    fmt.Sprintf("https://github.com/owner/repo/runs/%d", i+1),
		})
	}

	// Add failed checks
	for i := 0; i < failed; i++ {
		checkRuns = append(checkRuns, CheckRun{
			Name:       fmt.Sprintf("test-failed-%d", i+1),
			Status:     "completed",
			Conclusion: "failure",
			HTMLURL:    fmt.Sprintf("https://github.com/owner/repo/runs/%d", passed+i+1),
		})
	}

	// Add pending checks
	for i := 0; i < pending; i++ {
		checkRuns = append(checkRuns, CheckRun{
			Name:       fmt.Sprintf("test-pending-%d", i+1),
			Status:     "in_progress",
			Conclusion: "",
			HTMLURL:    fmt.Sprintf("https://github.com/owner/repo/runs/%d", passed+failed+i+1),
		})
	}

	return CheckRunsResponse{
		TotalCount: len(checkRuns),
		CheckRuns:  checkRuns,
	}
}

// CreateMockPRFiles creates mock PR file data
func CreateMockPRFiles(tektonOnly bool) []PRFile {
	if tektonOnly {
		return []PRFile{
			{Filename: ".tekton/pipeline.yaml", Status: "modified"},
			{Filename: ".tekton/task.yaml", Status: "added"},
		}
	}

	return []PRFile{
		{Filename: "main.go", Status: "modified"},
		{Filename: "README.md", Status: "modified"},
		{Filename: ".tekton/pipeline.yaml", Status: "added"},
	}
}

// CreateMockReviews creates mock review data
func CreateMockReviews(approved bool) []Review {
	if approved {
		return []Review{
			{
				State: "APPROVED",
				User:  User{Login: "reviewer1"},
			},
		}
	}

	return []Review{
		{
			State: "COMMENTED",
			User:  User{Login: "reviewer1"},
		},
	}
}

// SetupMockResponses configures common mock responses for testing
func SetupMockResponses(client *MockRESTClient, owner, repo string) {
	// Mock PR list
	prs := CreateMockPullRequests(5)
	client.AddResponse(fmt.Sprintf("repos/%s/%s/pulls", owner, repo), 200, prs)

	// Mock individual PR details
	for _, pr := range prs {
		client.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, pr.Number), 200, pr)

		// Mock check runs
		checkRuns := CreateMockCheckRuns(3, 1, 1)
		client.AddResponse(fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, pr.Head.SHA), 200, checkRuns)

		// Mock PR files
		files := CreateMockPRFiles(pr.Number%4 == 0) // Every 4th PR has only Tekton files
		client.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, pr.Number), 200, files)

		// Mock reviews
		reviews := CreateMockReviews(pr.Number%3 == 0) // Every 3rd PR is approved
		client.AddResponse(fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number), 200, reviews)
	}

	// Mock diff endpoint
	client.AddResponse(".diff", 200, "+added line\n-removed line\n unchanged line")
}

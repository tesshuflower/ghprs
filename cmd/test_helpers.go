package cmd

import "github.com/cli/go-gh/v2/pkg/api"

// Test helper functions that expose internal functionality for testing

// Exported utility functions for testing
func TruncateStringTest(s string, maxWidth int) string {
	return TruncateString(s, maxWidth)
}

func DisplayWidthTest(s string) int {
	return DisplayWidth(s)
}

func StripANSISequencesTest(s string) string {
	return StripANSISequences(s)
}

func PadStringTest(s string, width int) string {
	return PadString(s, width)
}

func FormatPRLinkTest(owner, repo string, prNumber int) string {
	return formatPRLink(owner, repo, prNumber)
}

func ShouldUseColorsTest() bool {
	return shouldUseColors()
}

func GetStatusIconTest(pr PullRequest) string {
	return getStatusIcon(pr)
}

func IsOnHoldTest(pr PullRequest) bool {
	return isOnHold(pr)
}

func HasMigrationWarningTest(pr PullRequest) bool {
	return hasMigrationWarning(pr)
}

func IsKonfluxNudgeTest(pr PullRequest) bool {
	return isKonfluxNudge(pr)
}

func NeedsRebaseTest(pr PullRequest) bool {
	return needsRebase(pr)
}

func IsBlockedTest(pr PullRequest) bool {
	return isBlocked(pr)
}

func IsReviewedTest(client api.RESTClient, owner, repo string, prNumber int, labels []Label) bool {
	return isReviewed(client, owner, repo, prNumber, labels)
}

func FetchPRDetailsTest(client api.RESTClient, owner, repo string, prNumber int) (*PullRequest, error) {
	return fetchPRDetails(client, owner, repo, prNumber)
}

func NewPRDetailsCacheTest() *PRDetailsCache {
	return NewPRDetailsCache()
}

func (c *PRDetailsCache) GetOrFetchTest(client api.RESTClient, owner, repo string, prNumber int, originalPR PullRequest) *PullRequest {
	return c.GetOrFetch(client, owner, repo, prNumber, originalPR)
}

func ColorizeGitDiffTest(diff string) string {
	return colorizeGitDiff(diff)
}

func SortPullRequestsTest(prs []PullRequest, sortBy string) {
	sortPullRequests(prs, sortBy)
}

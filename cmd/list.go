package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ghprs",
	Short: "A CLI tool for GitHub Pull Requests",
	Long: `A CLI application built with Cobra for managing and working with 
GitHub Pull Requests. This tool provides various commands to interact 
with GitHub repositories and pull requests.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Welcome to ghprs!")
		fmt.Println("Use 'ghprs --help' to see available commands.")
	},
}

// PullRequest represents a GitHub pull request
type PullRequest struct {
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	State          string  `json:"state"`
	User           User    `json:"user"`
	Head           Branch  `json:"head"`
	Base           Branch  `json:"base"`
	Draft          bool    `json:"draft"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
	HTMLURL        string  `json:"html_url"`
	Body           string  `json:"body"`
	MergeableState string  `json:"mergeable_state"`
	Labels         []Label `json:"labels"`
}

type User struct {
	Login string `json:"login"`
}

type Branch struct {
	Ref string `json:"ref"`
}

type Label struct {
	Name string `json:"name"`
}

// ReviewRequest represents a pull request review request
type ReviewRequest struct {
	Body  string `json:"body"`
	Event string `json:"event"`
}

// CommentRequest represents a pull request comment request
type CommentRequest struct {
	Body string `json:"body"`
}

// Review represents a pull request review
type Review struct {
	State string `json:"state"`
	User  User   `json:"user"`
}

// PRFile represents a file changed in a pull request
type PRFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"` // "added", "modified", "removed", etc.
}

var (
	state         string
	limit         int
	approve       bool
	current       bool
	tektonOnly    bool
	migrationOnly bool
	sortBy        string
	showFiles     bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [owner/repo]",
	Short: "List pull requests for a repository",
	Long: `List pull requests for a GitHub repository.

If no repository is specified, configured default repositories will be used.
If no default repositories are configured, the current repository will be detected from git remotes.
You can also specify a repository in the format "owner/repo".

Examples:
  ghprs list
  ghprs list microsoft/vscode
  ghprs list --state closed
  ghprs list --limit 5
  ghprs list --current                       # Force use current repo, bypass config
  ghprs list --sort-by oldest               # Show oldest PRs first
  ghprs list --sort-by updated               # Sort by last update`,
	Run: func(cmd *cobra.Command, args []string) {
		listPullRequests(args, "", false)
	},
}

// konfluxCmd represents the konflux command
var konfluxCmd = &cobra.Command{
	Use:   "konflux [owner/repo]",
	Short: "List Konflux pull requests (authored by red-hat-konflux[bot])",
	Long: `List pull requests authored by "red-hat-konflux[bot]" for a GitHub repository.

If no repository is specified, configured default repositories will be used.
If no default repositories are configured, the current repository will be detected from git remotes.
You can also specify a repository in the format "owner/repo".

Examples:
  ghprs konflux
  ghprs konflux microsoft/vscode
  ghprs konflux --state closed
  ghprs konflux --limit 5
  ghprs konflux --current                    # Force use current repo, bypass config
  ghprs konflux --approve                    # Interactively approve Konflux PRs (review + /lgtm comment)
  ghprs konflux --tekton-only                # Show only PRs that EXCLUSIVELY modify Tekton files
  ghprs konflux --migration-only             # Show only PRs with migration warnings
  ghprs konflux --sort-by priority           # Sort by priority (migration warnings first)
  ghprs konflux --sort-by oldest             # Show oldest PRs first
  ghprs konflux --approve --show-files       # Approve with detailed file lists
  ghprs konflux --approve                    # Interactive approval (use 'f' to view files)
  ghprs konflux owner/repo --approve         # Approve Konflux PRs in specific repo`,
	Run: func(cmd *cobra.Command, args []string) {
		listPullRequests(args, "red-hat-konflux[bot]", true)
	},
}

func listPullRequests(args []string, authorFilter string, isKonflux bool) {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Printf("Warning: Could not load config: %v", err)
		config = DefaultConfig()
	}

	// Use config defaults if no explicit values were set
	if state == "open" && config.Defaults.State != "open" {
		state = config.Defaults.State
	}
	if limit == 30 && config.Defaults.Limit != 30 {
		limit = config.Defaults.Limit
	}

	var repositories []string

	if len(args) > 0 {
		// Use specified repository
		repositories = []string{args[0]}
	} else if current {
		// Force use of current repository when --current flag is set
		if currentRepo, err := repository.Current(); err == nil {
			repositories = []string{fmt.Sprintf("%s/%s", currentRepo.Owner, currentRepo.Name)}
		} else {
			log.Fatal("Could not detect current repository. Make sure you're in a git repository.")
		}
	} else {
		// Use configured repositories first, then fall back to auto-detection
		if len(config.Repositories) > 0 {
			repositories = config.Repositories
		} else if currentRepo, err := repository.Current(); err == nil {
			repositories = []string{fmt.Sprintf("%s/%s", currentRepo.Owner, currentRepo.Name)}
		} else {
			log.Fatal("No repositories specified and no default repositories configured. Please specify owner/repo manually, configure default repositories with 'ghprs config add-repo owner/repo', or run from a git repository.")
		}
	}

	// Process each repository
	for i, repoSpec := range repositories {
		if len(repositories) > 1 {
			if i > 0 {
				fmt.Println() // Add spacing between repositories
			}
			fmt.Printf("=== %s ===\n", repoSpec)
		}

		// Parse owner/repo from repository spec
		parts := strings.Split(repoSpec, "/")
		if len(parts) != 2 {
			log.Printf("Invalid repository format '%s', skipping. Must be 'owner/repo'", repoSpec)
			continue
		}
		owner := parts[0]
		repo := parts[1]

		// Create REST API client
		client, err := api.DefaultRESTClient()
		if err != nil {
			log.Printf("Failed to create GitHub client for %s: %v", repoSpec, err)
			continue
		}

		// Prepare API request
		path := fmt.Sprintf("repos/%s/%s/pulls", owner, repo)

		// Add query parameters
		params := []string{}
		if state != "" {
			params = append(params, "state="+state)
		}
		if limit > 0 {
			params = append(params, "per_page="+strconv.Itoa(limit))
		}

		if len(params) > 0 {
			path += "?" + strings.Join(params, "&")
		}

		// Make API request
		var allPullRequests []PullRequest
		err = client.Get(path, &allPullRequests)
		if err != nil {
			log.Printf("Failed to fetch pull requests for %s: %v", repoSpec, err)
			continue
		}

		// Filter by author if specified
		var pullRequests []PullRequest
		if authorFilter != "" {
			for _, pr := range allPullRequests {
				if pr.User.Login == authorFilter {
					pullRequests = append(pullRequests, pr)
				}
			}
		} else {
			pullRequests = allPullRequests
		}

		// Sort PRs based on the specified sort option
		if sortBy != "" {
			sortPullRequests(pullRequests, sortBy)

			// For Konflux PRs with priority sorting, do a more comprehensive sort
			if isKonflux && sortBy == "priority" {
				sortPullRequestsWithContext(pullRequests, *client, owner, repo, sortBy)
			}
		}

		// Display results
		if len(pullRequests) == 0 {
			if isKonflux {
				fmt.Printf("No Konflux pull requests found for %s\n", repoSpec)
			} else {
				fmt.Printf("No %s pull requests found for %s\n", state, repoSpec)
			}
			continue
		}

		if len(repositories) == 1 {
			// Single repository - show full header
			if isKonflux {
				fmt.Printf("Konflux pull requests for %s:\n\n", repoSpec)
			} else {
				fmt.Printf("Pull requests for %s:\n\n", repoSpec)
			}
		}

		// Handle approval if requested
		if approve && isKonflux {
			approveKonfluxPRs(*client, owner, repo, pullRequests)
			continue
		}

		// Display PR list
		for _, pr := range pullRequests {
			// Check for Tekton files if this is a Konflux PR
			onlyTektonFiles := false
			var tektonFiles []string
			if isKonflux {
				var err error
				onlyTektonFiles, tektonFiles, err = checkTektonFilesDetailed(*client, owner, repo, pr.Number)
				if err != nil {
					fmt.Printf("⚠️  Could not check files for PR #%d: %v\n", pr.Number, err)
				}
			}

			// Check for migration warnings
			hasMigration := false
			if isKonflux {
				hasMigration = hasMigrationWarning(pr)
			}

			// Skip PRs that don't exclusively modify Tekton files if --tekton-only flag is set
			if tektonOnly && !onlyTektonFiles {
				continue
			}

			// Skip PRs that don't have migration warnings if --migration-only flag is set
			if migrationOnly && !hasMigration {
				continue
			}

			// Color-code based on state, draft status, hold status, and Tekton files
			var icon string
			if isKonflux {
				icon = getStatusIconWithTekton(pr, onlyTektonFiles)
			} else {
				icon = getStatusIcon(pr)
			}

			fmt.Printf("%s #%-4d %s\n", icon, pr.Number, pr.Title)
			fmt.Printf("        %s → %s by @%s\n", pr.Head.Ref, pr.Base.Ref, pr.User.Login)
			if isKonflux && onlyTektonFiles && len(tektonFiles) > 0 {
				fmt.Printf("        📁 Tekton-only files: %s\n", strings.Join(tektonFiles, ", "))
			}
			if isKonflux && hasMigration {
				fmt.Printf("        🚨 Contains migration warnings\n")
			}
			fmt.Printf("        %s\n\n", pr.HTMLURL)
		}
	}
}

// promptForApproval prompts the user to approve a specific PR
func promptForApproval(pr PullRequest, owner, repo string, client api.RESTClient) bool {
	fmt.Printf("\n🔍 Review PR #%d:\n", pr.Number)
	fmt.Printf("   Title: %s\n", pr.Title)
	fmt.Printf("   Author: @%s\n", pr.User.Login)
	fmt.Printf("   Branch: %s → %s\n", pr.Head.Ref, pr.Base.Ref)
	fmt.Printf("   URL: %s\n", pr.HTMLURL)

	// Get file count (and optionally display files if --show-files is used)
	filesPath := fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, pr.Number)
	var allFiles []PRFile
	err := client.Get(filesPath, &allFiles)
	if err != nil {
		fmt.Printf("   ⚠️  Could not fetch file list: %v\n", err)
	} else {
		if showFiles {
			fmt.Printf("   📁 Files changed (%d):\n", len(allFiles))
			displayFileList(allFiles)
		} else {
			fmt.Printf("   📁 Files changed: %d (press 'f' during approval to view)\n", len(allFiles))
		}
	}

	// Check for Tekton files
	onlyTektonFiles, tektonFiles, err := checkTektonFilesDetailed(client, owner, repo, pr.Number)
	if err != nil {
		fmt.Printf("   ⚠️  Could not check Tekton files: %v\n", err)
	} else if onlyTektonFiles {
		fmt.Printf("   ✅ ONLY modifies Tekton files: %s\n", strings.Join(tektonFiles, ", "))
	} else {
		fmt.Printf("   ❌ Does NOT exclusively modify target Tekton files\n")
	}

	// Check for migration warnings
	if hasMigrationWarning(pr) {
		fmt.Printf("   🚨 MIGRATION WARNING: This PR contains migration notes - review carefully!\n")
	}

	// Show hold status if applicable
	if isOnHold(pr) {
		fmt.Printf("   ⚠️  Status: ON HOLD (has 'do-not-merge/hold' label)\n")
	}

	for {
		if showFiles {
			fmt.Printf("\nApprove this PR? [y/N/q]: ")
		} else {
			fmt.Printf("\nApprove this PR? [y/N/q/f] (f=show files): ")
		}

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			// Handle EOF gracefully (e.g., when input is piped and runs out)
			if err == io.EOF {
				fmt.Printf("(EOF - exiting approval process)\n")
				os.Exit(0)
			}
			fmt.Printf("Error reading input: %v (skipping PR)\n", err)
			return false
		}

		response = strings.TrimSpace(strings.ToLower(response))

		switch response {
		case "y", "yes":
			return true
		case "q", "quit":
			fmt.Println("Quitting approval process.")
			os.Exit(0)
			return false // This won't be reached but satisfies the compiler
		case "f", "files":
			if showFiles {
				fmt.Printf("\n📁 File list already shown above.\n")
			} else {
				// Show detailed file list
				fmt.Printf("\n📁 Detailed file list for PR #%d:\n", pr.Number)
				filesPath := fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, pr.Number)
				var files []PRFile
				err := client.Get(filesPath, &files)
				if err != nil {
					fmt.Printf("   ❌ Could not fetch file list: %v\n", err)
				} else {
					displayFileList(files)
					fmt.Printf("\nTotal: %d files changed\n", len(files))
				}
			}
			// Continue the loop to ask again
			continue
		case "", "n", "no":
			fmt.Printf("Skipping PR #%d\n", pr.Number)
			return false
		default:
			fmt.Printf("Invalid option '%s'. Please choose y/N/q/f.\n", response)
			// Continue the loop to ask again
			continue
		}
	}
}

func approveKonfluxPRs(client api.RESTClient, owner, repo string, pullRequests []PullRequest) {
	approvedCount := 0
	skippedCount := 0
	alreadyApprovedCount := 0
	userSkippedCount := 0

	fmt.Printf("\n🎯 Interactive approval mode for %d Konflux PRs\n", len(pullRequests))
	if showFiles {
		fmt.Printf("Commands: [y]es to approve, [N]o to skip (default), [q]uit\n")
	} else {
		fmt.Printf("Commands: [y]es to approve, [N]o to skip (default), [q]uit, [f]iles to view\n")
	}
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	for _, pr := range pullRequests {
		// Only approve open PRs
		if pr.State != "open" {
			fmt.Printf("⏭️  Auto-skipping #%d (state: %s): %s\n", pr.Number, pr.State, pr.Title)
			skippedCount++
			continue
		}

		// Skip draft PRs
		if pr.Draft {
			fmt.Printf("⏭️  Auto-skipping #%d (draft): %s\n", pr.Number, pr.Title)
			skippedCount++
			continue
		}

		// Skip PRs that are on hold
		if isOnHold(pr) {
			fmt.Printf("⏭️  Auto-skipping #%d (on hold): %s\n", pr.Number, pr.Title)
			skippedCount++
			continue
		}

		// Check if PR is already approved by current user
		reviewsPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number)
		var reviews []Review
		err := client.Get(reviewsPath, &reviews)
		if err != nil {
			fmt.Printf("⚠️  Could not check existing reviews for #%d: %v\n", pr.Number, err)
			// Continue with prompt despite error
		} else {
			// Check if we already have an approval from the current user
			alreadyApproved := false
			for _, review := range reviews {
				if review.State == "APPROVED" {
					// We could check if it's from the current user, but for simplicity
					// we'll consider any approval as sufficient
					alreadyApproved = true
					break
				}
			}

			if alreadyApproved {
				fmt.Printf("✅ Already approved #%d: %s\n", pr.Number, pr.Title)
				alreadyApprovedCount++
				continue
			}
		}

		// Prompt user for approval decision
		if !promptForApproval(pr, owner, repo, client) {
			userSkippedCount++
			continue
		}

		// Create approval review
		reviewPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number)
		review := ReviewRequest{
			Body:  "Approved by ghprs CLI tool for Konflux automation",
			Event: "APPROVE",
		}

		// Convert review to JSON
		reviewJSON, err := json.Marshal(review)
		if err != nil {
			fmt.Printf("❌ Failed to marshal review for #%d: %v\n", pr.Number, err)
			continue
		}

		fmt.Printf("✅ Approving #%d: %s\n", pr.Number, pr.Title)

		// First, add the approval review
		err = client.Post(reviewPath, bytes.NewReader(reviewJSON), nil)
		if err != nil {
			fmt.Printf("❌ Failed to approve #%d: %v\n", pr.Number, err)
			continue
		}

		// Second, add a "/lgtm" comment
		commentPath := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, pr.Number)
		comment := CommentRequest{
			Body: "/lgtm",
		}

		commentJSON, err := json.Marshal(comment)
		if err != nil {
			fmt.Printf("⚠️  Failed to marshal comment for #%d: %v\n", pr.Number, err)
			// Continue since approval was successful
		} else {
			err = client.Post(commentPath, bytes.NewReader(commentJSON), nil)
			if err != nil {
				fmt.Printf("⚠️  Failed to add /lgtm comment to #%d: %v\n", pr.Number, err)
				// Continue since approval was successful
			} else {
				fmt.Printf("   ✓ Added /lgtm comment to #%d\n", pr.Number)
			}
		}

		approvedCount++
		fmt.Printf("   ✓ Successfully approved #%d\n", pr.Number)
	}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   ✅ Approved: %d\n", approvedCount)
	fmt.Printf("   ✅ Already approved: %d\n", alreadyApprovedCount)
	fmt.Printf("   👤 User skipped: %d\n", userSkippedCount)
	fmt.Printf("   ⏭️  Auto-skipped: %d\n", skippedCount)
	fmt.Printf("   📝 Total: %d\n", len(pullRequests))
}

func getStateIcon(state string, isDraft bool) string {
	if isDraft {
		return "🟡 (draft)"
	}

	switch state {
	case "open":
		return "🟢 (open)"
	case "closed":
		return "🔴 (closed)"
	case "merged":
		return "🟣 (merged)"
	default:
		return "⚪ (" + state + ")"
	}
}

// isOnHold checks if a PR has the "do-not-merge/hold" label
func isOnHold(pr PullRequest) bool {
	for _, label := range pr.Labels {
		if label.Name == "do-not-merge/hold" {
			return true
		}
	}
	return false
}

// checkTektonFilesDetailed checks if a PR ONLY modifies specific Tekton files and returns the list
func checkTektonFilesDetailed(client api.RESTClient, owner, repo string, prNumber int) (bool, []string, error) {
	filesPath := fmt.Sprintf("repos/%s/%s/pulls/%d/files", owner, repo, prNumber)
	var files []PRFile
	err := client.Get(filesPath, &files)
	if err != nil {
		return false, nil, err
	}

	var tektonFiles []string
	var nonTektonFiles []string

	for _, file := range files {
		// Check if file is in .tekton/ directory and matches our patterns
		if strings.HasPrefix(file.Filename, ".tekton/") {
			if strings.HasSuffix(file.Filename, "-pull-request.yaml") || strings.HasSuffix(file.Filename, "-push.yaml") {
				tektonFiles = append(tektonFiles, file.Filename)
			} else {
				// File is in .tekton/ but doesn't match our patterns
				nonTektonFiles = append(nonTektonFiles, file.Filename)
			}
		} else {
			// File is not in .tekton/ directory
			nonTektonFiles = append(nonTektonFiles, file.Filename)
		}
	}

	// Return true only if we have target Tekton files AND no other files
	onlyTektonFiles := len(tektonFiles) > 0 && len(nonTektonFiles) == 0
	return onlyTektonFiles, tektonFiles, nil
}

// hasMigrationWarning checks if a PR body contains migration warnings
func hasMigrationWarning(pr PullRequest) bool {
	if pr.Body == "" {
		return false
	}

	// Look for migration warning patterns in the PR body
	// Common patterns: ⚠️[migration]..., :warning:[migration], ⚠️migration⚠️
	migrationPatterns := []string{
		"⚠️[migration]",
		":warning:[migration]",
		"⚠️migration⚠️",
		"[migration]",
	}

	body := strings.ToLower(pr.Body)
	for _, pattern := range migrationPatterns {
		if strings.Contains(body, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// getStatusIcon returns the appropriate icon and status for a PR
func getStatusIcon(pr PullRequest) string {
	onHold := isOnHold(pr)

	if pr.Draft {
		if onHold {
			return "🟡 (draft, on hold)"
		}
		return "🟡 (draft)"
	}

	switch pr.State {
	case "open":
		if onHold {
			return "🔶 (open, on hold)"
		}
		return "🟢 (open)"
	case "closed":
		return "🔴 (closed)"
	case "merged":
		return "🟣 (merged)"
	default:
		if onHold {
			return "⚪ (" + pr.State + ", on hold)"
		}
		return "⚪ (" + pr.State + ")"
	}
}

// getStatusIconWithTekton returns the appropriate icon and status for a PR, including Tekton and migration info
func getStatusIconWithTekton(pr PullRequest, hasTektonFiles bool) string {
	onHold := isOnHold(pr)
	hasMigration := hasMigrationWarning(pr)

	var indicators []string
	if hasTektonFiles {
		indicators = append(indicators, "tekton")
	}
	if hasMigration {
		indicators = append(indicators, "migration")
	}

	indicatorStr := ""
	if len(indicators) > 0 {
		indicatorStr = ", " + strings.Join(indicators, ", ")
	}

	if pr.Draft {
		if onHold {
			return fmt.Sprintf("🟡 (draft, on hold%s)", indicatorStr)
		}
		return fmt.Sprintf("🟡 (draft%s)", indicatorStr)
	}

	switch pr.State {
	case "open":
		if onHold {
			return fmt.Sprintf("🔶 (open, on hold%s)", indicatorStr)
		}
		if hasTektonFiles || hasMigration {
			return fmt.Sprintf("🟢 (open%s)", indicatorStr)
		}
		return "🟢 (open)"
	case "closed":
		return fmt.Sprintf("🔴 (closed%s)", indicatorStr)
	case "merged":
		return fmt.Sprintf("🟣 (merged%s)", indicatorStr)
	default:
		if onHold {
			return fmt.Sprintf("⚪ (%s, on hold%s)", pr.State, indicatorStr)
		}
		return fmt.Sprintf("⚪ (%s%s)", pr.State, indicatorStr)
	}
}

// sortPullRequests sorts PRs based on the specified sort option
func sortPullRequests(prs []PullRequest, sortBy string) {
	switch sortBy {
	case "oldest":
		// Sort by creation date ascending (oldest first)
		sort.Slice(prs, func(i, j int) bool {
			return prs[i].CreatedAt < prs[j].CreatedAt
		})
	case "updated":
		// Sort by last update descending (most recently updated first)
		sort.Slice(prs, func(i, j int) bool {
			return prs[i].UpdatedAt > prs[j].UpdatedAt
		})
	case "number":
		// Sort by PR number ascending (lowest numbers first)
		sort.Slice(prs, func(i, j int) bool {
			return prs[i].Number < prs[j].Number
		})
	case "priority":
		// Custom priority sorting: migration warnings first, then others by creation date
		sort.Slice(prs, func(i, j int) bool {
			iMigration := hasMigrationWarning(prs[i])
			jMigration := hasMigrationWarning(prs[j])

			// Migration warnings have highest priority
			if iMigration && !jMigration {
				return true
			}
			if !iMigration && jMigration {
				return false
			}

			// If both have same migration status, sort by creation date (newest first)
			return prs[i].CreatedAt > prs[j].CreatedAt
		})
	case "newest":
		fallthrough
	default:
		// Default: Sort by creation date descending (newest first) - GitHub's default
		// No sorting needed as this is already the API default
		return
	}
}

// sortPullRequestsWithContext sorts PRs with full context including Tekton file information
func sortPullRequestsWithContext(prs []PullRequest, client api.RESTClient, owner, repo string, sortBy string) {
	if sortBy != "priority" {
		return // Only apply context-aware sorting for priority mode
	}

	// Create a slice of PR info with additional context
	type prInfo struct {
		pr              PullRequest
		hasMigration    bool
		onlyTektonFiles bool
	}

	var prInfos []prInfo
	for _, pr := range prs {
		info := prInfo{
			pr:           pr,
			hasMigration: hasMigrationWarning(pr),
		}

		// Check Tekton files (this makes API calls, so only do it for priority sorting)
		onlyTekton, _, err := checkTektonFilesDetailed(client, owner, repo, pr.Number)
		if err == nil {
			info.onlyTektonFiles = onlyTekton
		}

		prInfos = append(prInfos, info)
	}

	// Sort by priority: migration warnings first, then Tekton-only, then others
	sort.Slice(prInfos, func(i, j int) bool {
		iInfo := prInfos[i]
		jInfo := prInfos[j]

		// 1. Migration warnings have highest priority
		if iInfo.hasMigration && !jInfo.hasMigration {
			return true
		}
		if !iInfo.hasMigration && jInfo.hasMigration {
			return false
		}

		// 2. If both have same migration status, Tekton-only PRs come next
		if iInfo.onlyTektonFiles && !jInfo.onlyTektonFiles {
			return true
		}
		if !iInfo.onlyTektonFiles && jInfo.onlyTektonFiles {
			return false
		}

		// 3. If both have same migration and Tekton status, sort by creation date (newest first)
		return iInfo.pr.CreatedAt > jInfo.pr.CreatedAt
	})

	// Copy back the sorted PRs
	for i, info := range prInfos {
		prs[i] = info.pr
	}
}

// displayFileList shows a formatted list of files with status indicators
func displayFileList(files []PRFile) {
	for _, file := range files {
		status := ""
		statusColor := ""
		switch file.Status {
		case "added":
			status = "+"
			statusColor = "🟢"
		case "modified":
			status = "~"
			statusColor = "🟡"
		case "removed":
			status = "-"
			statusColor = "🔴"
		case "renamed":
			status = "→"
			statusColor = "🔵"
		default:
			status = "?"
			statusColor = "⚪"
		}
		fmt.Printf("      %s %s %s\n", statusColor, status, file.Filename)
	}
}

func init() {
	RootCmd.AddCommand(listCmd)
	RootCmd.AddCommand(konfluxCmd)

	// Add flags to both commands
	listCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	listCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
	listCmd.Flags().BoolVarP(&current, "current", "c", false, "Use current repository, bypass config")
	listCmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort PRs by: newest (default), oldest, updated, number, priority")
	listCmd.Flags().BoolVarP(&showFiles, "show-files", "f", false, "Show detailed file list during approval process")

	konfluxCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	konfluxCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
	konfluxCmd.Flags().BoolVarP(&current, "current", "c", false, "Use current repository, bypass config")
	konfluxCmd.Flags().BoolVarP(&approve, "approve", "a", false, "Interactively approve Konflux pull requests (review + /lgtm comment)")
	konfluxCmd.Flags().BoolVarP(&tektonOnly, "tekton-only", "t", false, "Show only PRs that EXCLUSIVELY modify Tekton files (.tekton/*-pull-request.yaml or *-push.yaml)")
	konfluxCmd.Flags().BoolVarP(&migrationOnly, "migration-only", "m", false, "Show only PRs that contain migration warnings")
	konfluxCmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort PRs by: newest (default), oldest, updated, number, priority")
	konfluxCmd.Flags().BoolVarP(&showFiles, "show-files", "f", false, "Show detailed file list during approval process")
}

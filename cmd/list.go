package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	SHA string `json:"sha"`
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

// LabelRequest represents a request to add labels to an issue/PR
type LabelRequest struct {
	Labels []string `json:"labels"`
}

// CheckRun represents a GitHub check run
type CheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // "queued", "in_progress", "completed"
	Conclusion string `json:"conclusion"` // "success", "failure", "neutral", "cancelled", "timed_out", "action_required", "skipped"
	HTMLURL    string `json:"html_url"`
}

// CheckRunsResponse represents the response from the check runs API
type CheckRunsResponse struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []CheckRun `json:"check_runs"`
}

// StatusCheck represents a GitHub status check (legacy)
type StatusCheck struct {
	State       string `json:"state"` // "pending", "success", "error", "failure"
	Description string `json:"description"`
	Context     string `json:"context"`
	TargetURL   string `json:"target_url"`
}

// CheckStatus represents the combined status of all checks
type CheckStatus struct {
	Passed    int
	Failed    int
	Pending   int
	Cancelled int
	Skipped   int
	Total     int
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
	showDiff      bool
	noColor       bool
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
  ghprs list --sort-by updated               # Sort by last update
  ghprs list --approve                       # Interactively approve PRs (review + /lgtm comment)
  ghprs list --approve --show-files          # Approve with detailed file lists
  ghprs list --approve --show-diff           # Approve with detailed diff display
  ghprs list --approve                       # Interactive approval (use 'f' to view files, 'd' to view diff, 'c' to view checks)`,
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
  ghprs konflux --approve --show-diff        # Approve with detailed diff display
  ghprs konflux --approve --show-diff --no-color  # Approve with diff but no colors
  ghprs konflux --approve                    # Interactive approval (use 'f' to view files, 'd' to view diff, 'c' to view checks)
  ghprs konflux owner/repo --approve         # Approve Konflux PRs in specific repo`,
	Run: func(cmd *cobra.Command, args []string) {
		listPullRequests(args, "red-hat-konflux[bot]", true)
	},
}

// ApprovalConfig controls the behavior of the approval process
type ApprovalConfig struct {
	IsKonflux bool
}

// promptForRepositorySelection prompts the user to select a repository from a list
func promptForRepositorySelection(repositories []string) string {
	fmt.Printf("\n📂 Multiple repositories configured (%d):\n", len(repositories))
	for i, repo := range repositories {
		fmt.Printf("  %d. %s\n", i+1, repo)
	}
	fmt.Printf("  %d. All repositories\n", len(repositories)+1)
	fmt.Printf("  0. Cancel\n")

	for {
		fmt.Printf("\nSelect repository (1-%d, %d for all, 0 to cancel): ", len(repositories), len(repositories)+1)

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf("\n")
				return "" // User cancelled or input ended
			}
			fmt.Printf("Error reading input: %v\n", err)
			return "" // Exit on any read error
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		choice, err := strconv.Atoi(input)
		if err != nil {
			fmt.Printf("Invalid input '%s'. Please enter a number.\n", input)
			continue
		}

		if choice == 0 {
			return "" // User cancelled
		} else if choice >= 1 && choice <= len(repositories) {
			return repositories[choice-1]
		} else if choice == len(repositories)+1 {
			return "ALL" // Special value to indicate all repositories
		} else {
			fmt.Printf("Invalid choice %d. Please select a number between 0 and %d.\n", choice, len(repositories)+1)
		}
	}
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
		configRepos := config.GetRepositories(isKonflux)
		if len(configRepos) > 0 {
			// If there are multiple repositories, prompt the user to select which repository they want to see
			if len(configRepos) > 1 {
				selectedRepo := promptForRepositorySelection(configRepos)
				if selectedRepo == "" {
					fmt.Println("No repository selected. Exiting.")
					return
				}
				if selectedRepo == "ALL" {
					repositories = configRepos
				} else {
					repositories = []string{selectedRepo}
				}
			} else {
				repositories = configRepos
			}
		} else if currentRepo, err := repository.Current(); err == nil {
			repositories = []string{fmt.Sprintf("%s/%s", currentRepo.Owner, currentRepo.Name)}
		} else {
			if isKonflux {
				log.Fatal("No repositories specified and no Konflux repositories configured. Please specify owner/repo manually, configure Konflux repositories with 'ghprs config add-konflux-repo owner/repo', or run from a git repository.")
			} else {
				log.Fatal("No repositories specified and no default repositories configured. Please specify owner/repo manually, configure default repositories with 'ghprs config add-repo owner/repo', or run from a git repository.")
			}
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
		if approve {
			config := ApprovalConfig{
				IsKonflux: false,
			}

			if isKonflux {
				config = ApprovalConfig{
					IsKonflux: true,
				}
			}

			// Start approval flow - table will be displayed there
			approvePRsWithConfig(*client, owner, repo, pullRequests, config, nil)
			continue
		}

		// Display PR list in table format
		_ = displayPRTable(pullRequests, owner, repo, client, isKonflux, nil)
	}
}

// promptForApproval prompts the user to approve a specific PR with configurable behavior
// ApprovalResult represents the result of the approval prompt
type ApprovalResult int

const (
	ApprovalResultSkip ApprovalResult = iota
	ApprovalResultApprove
	ApprovalResultHold
	ApprovalResultQuit
	ApprovalResultComment
)

// promptForApprovalWithCache prompts the user to approve a specific PR with configurable behavior and optional cache
func promptForApprovalWithCache(pr PullRequest, owner, repo string, client api.RESTClient, config ApprovalConfig, cache *PRDetailsCache) ApprovalResult {
	fmt.Printf("\n🔍 Review PR %s:\n", formatPRLink(owner, repo, pr.Number))
	fmt.Printf("   Title: %s\n", pr.Title)
	fmt.Printf("   Author: @%s\n", pr.User.Login)
	fmt.Printf("   Branch: %s → %s\n", pr.Head.Ref, pr.Base.Ref)

	// Use provided cache or create a new one for PR details to avoid duplicate API calls
	if cache == nil {
		cache = NewPRDetailsCache()
	}

	// Show rebase status - fetch full details if needed
	if needsRebaseWithCache(cache, client, owner, repo, pr) {
		fmt.Printf("   🔄 Rebase needed: PR is behind the target branch or has conflicts\n")
	}
	// Only show if there's an issue, otherwise it's assumed to be up to date

	// Show blocked status - fetch full details if needed
	if isBlockedWithCache(cache, client, owner, repo, pr) {
		fmt.Printf("   🚫 Blocked: PR is blocked from merging (failed checks, missing reviews, etc.)\n")
	}
	// Only show if blocked, otherwise it's assumed to be ready for merge

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

	// Display check status
	if pr.Head.SHA != "" {
		displayCheckStatus(client, owner, repo, pr.Number, pr.Head.SHA)
	}

	// Optionally display diff if --show-diff is used
	if showDiff {
		err := displayDiff(owner, repo, pr.Number)
		if err != nil {
			fmt.Printf("   ⚠️  Could not fetch diff: %v\n", err)
		}
	}

	// Konflux-specific checks
	if config.IsKonflux {
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
	}

	// Show hold status if applicable
	if isOnHold(pr) {
		fmt.Printf("   ⚠️  Status: ON HOLD (has 'do-not-merge/hold' label)\n")
	}

	for {
		// Build prompt based on what's already shown
		promptOptions := []string{"y/N/q/h/m"}
		promptHelp := []string{"h=hold", "m=comment"}

		if !showFiles {
			promptOptions = append(promptOptions, "f")
			promptHelp = append(promptHelp, "f=show files")
		}
		if !showDiff {
			promptOptions = append(promptOptions, "d")
			promptHelp = append(promptHelp, "d=show diff")
		}

		// Always show check option if we have a head SHA
		if pr.Head.SHA != "" {
			promptOptions = append(promptOptions, "c")
			promptHelp = append(promptHelp, "c=show checks")
		}

		promptStr := fmt.Sprintf("\nApprove this PR? [%s]", strings.Join(promptOptions, "/"))
		if len(promptHelp) > 0 {
			promptStr += fmt.Sprintf(" (%s)", strings.Join(promptHelp, ", "))
		}
		promptStr += ": "

		fmt.Print(promptStr)

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			// Handle EOF gracefully (e.g., when input is piped and runs out)
			if err == io.EOF {
				fmt.Printf("(EOF - exiting approval process)\n")
				os.Exit(0)
			}
			fmt.Printf("Error reading input: %v (skipping PR)\n", err)
			return ApprovalResultSkip
		}

		response = strings.TrimSpace(strings.ToLower(response))

		switch response {
		case "y", "yes":
			return ApprovalResultApprove
		case "q", "quit":
			fmt.Println("Quitting approval process.")
			return ApprovalResultQuit
		case "h", "hold":
			// Prompt for additional comment
			fmt.Printf("Enter an optional comment to add with /hold (or press Enter for none): ")
			reader := bufio.NewReader(os.Stdin)
			additionalComment, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading comment: %v\n", err)
				additionalComment = ""
			}
			additionalComment = strings.TrimSpace(additionalComment)

			// Hold the PR
			err = holdPR(client, owner, repo, pr.Number, additionalComment)
			if err != nil {
				fmt.Printf("❌ Failed to hold PR %s: %v\n", formatPRLink(owner, repo, pr.Number), err)
				continue // Let user try again
			}

			fmt.Printf("⏸️  Put PR %s on hold\n", formatPRLink(owner, repo, pr.Number))
			return ApprovalResultHold
		case "m", "comment":
			// Prompt for comment
			fmt.Printf("Enter your comment: ")
			reader := bufio.NewReader(os.Stdin)
			commentText, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading comment: %v\n", err)
				continue // Let user try again
			}
			commentText = strings.TrimSpace(commentText)

			if commentText == "" {
				fmt.Printf("Empty comment, skipping.\n")
				continue // Let user try again
			}

			// Add the comment
			err = addCommentToPR(client, owner, repo, pr.Number, commentText)
			if err != nil {
				fmt.Printf("❌ Failed to add comment to PR %s: %v\n", formatPRLink(owner, repo, pr.Number), err)
				continue // Let user try again
			}

			fmt.Printf("💬 Added comment to PR %s\n", formatPRLink(owner, repo, pr.Number))
			return ApprovalResultComment
		case "f", "files":
			if showFiles {
				fmt.Printf("\n📁 File list already shown above.\n")
			} else {
				// Show detailed file list
				fmt.Printf("\n📁 Detailed file list for PR %s:\n", formatPRLink(owner, repo, pr.Number))
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
		case "d", "diff":
			if showDiff {
				fmt.Printf("\n📄 Diff already shown above.\n")
			} else {
				// Show diff
				err := displayDiff(owner, repo, pr.Number)
				if err != nil {
					fmt.Printf("   ❌ Could not fetch diff: %v\n", err)
				}
			}
			// Continue the loop to ask again
			continue
		case "c", "checks":
			if pr.Head.SHA != "" {
				displayDetailedCheckStatus(client, owner, repo, pr.Number, pr.Head.SHA)
			} else {
				fmt.Printf("   ❌ No commit SHA available for check status\n")
			}
			// Continue the loop to ask again
			continue
		case "", "n", "no":
			fmt.Printf("Skipping PR %s\n", formatPRLink(owner, repo, pr.Number))
			return ApprovalResultSkip
		default:
			fmt.Printf("Invalid option '%s'. Please choose from the available options.\n", response)
			// Continue the loop to ask again
			continue
		}
	}
}

func approvePRsWithConfig(client api.RESTClient, owner, repo string, pullRequests []PullRequest, config ApprovalConfig, cache *PRDetailsCache) {
	fmt.Printf("\n🎯 Interactive approval mode for %d PRs\n", len(pullRequests))

	// Keep track of processed PRs to remove them from subsequent displays
	processedPRs := make(map[int]bool)
	approvedCount := 0
	skippedCount := 0
	heldCount := 0
	commentedCount := 0

	for {
		// Filter out PRs that can't be approved (closed, draft, on hold) and already processed
		var approvablePRs []PullRequest
		var displayPRs []PullRequest
		var prIndexMap = make(map[int]int) // Maps PR number to index in approvablePRs

		for _, pr := range pullRequests {
			// Skip already processed PRs
			if processedPRs[pr.Number] {
				continue
			}

			// Add to display list (for table)
			displayPRs = append(displayPRs, pr)

			// Add to approvable list if eligible
			if pr.State == "open" && !pr.Draft && !isOnHold(pr) {
				prIndexMap[pr.Number] = len(approvablePRs)
				approvablePRs = append(approvablePRs, pr)
			}
		}

		// Check if we have any PRs left to display
		if len(displayPRs) == 0 {
			fmt.Printf("\n✅ All PRs have been processed!\n")
			break
		}

		// Display the PR table (excluding processed PRs)
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		cache = displayPRTable(displayPRs, owner, repo, &client, config.IsKonflux, cache)
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")

		// Check if we have any approvable PRs left
		if len(approvablePRs) == 0 {
			fmt.Printf("❌ No more PRs available for approval (remaining are closed, draft, or on hold)\n")
			break
		}

		// Prompt for PR selection
		fmt.Printf("\n📝 Select PR to approve:\n")
		fmt.Printf("   Enter PR number (default: %d for first approvable PR)\n", approvablePRs[0].Number)
		fmt.Printf("   Or press 'q' to quit\n")
		fmt.Printf("   Available for approval: ")

		var availableNumbers []string
		for _, pr := range approvablePRs {
			availableNumbers = append(availableNumbers, fmt.Sprintf("#%d", pr.Number))
		}
		fmt.Printf("%s\n", strings.Join(availableNumbers, ", "))

		fmt.Print("\nPR to approve: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Printf("(EOF - exiting approval process)\n")
				break
			}
			fmt.Printf("Error reading input: %v\n", err)
			break
		}

		input = strings.TrimSpace(input)

		// Handle quit
		if strings.ToLower(input) == "q" || strings.ToLower(input) == "quit" {
			fmt.Println("Exiting approval process.")
			break
		}

		// Determine which PR to approve
		var selectedPR *PullRequest

		if input == "" {
			// Default to first approvable PR
			selectedPR = &approvablePRs[0]
			fmt.Printf("Using default PR: #%d\n", selectedPR.Number)
		} else {
			// Parse the PR number (remove # prefix if present)
			input = strings.TrimPrefix(input, "#")

			prNumber, err := strconv.Atoi(input)
			if err != nil {
				fmt.Printf("❌ Invalid PR number: %s\n", input)
				fmt.Printf("Press Enter to continue or 'q' to quit.\n")
				continue
			}

			// Find the PR in our approvable list
			index, exists := prIndexMap[prNumber]
			if !exists {
				fmt.Printf("❌ PR #%d is not available for approval (may be closed, draft, on hold, or not exist)\n", prNumber)
				fmt.Printf("   Available PRs: %s\n", strings.Join(availableNumbers, ", "))
				fmt.Printf("Press Enter to continue or 'q' to quit.\n")
				continue
			}

			selectedPR = &approvablePRs[index]
			fmt.Printf("Selected PR: #%d\n", selectedPR.Number)
		}

		// Now proceed with the approval flow for the selected PR - reuse the cache
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		result := approveSinglePRWithCache(client, owner, repo, *selectedPR, config, cache)

		// Mark this PR as processed and update counters
		processedPRs[selectedPR.Number] = true
		switch result {
		case ApprovalResultApprove:
			approvedCount++
		case ApprovalResultSkip:
			skippedCount++
		case ApprovalResultHold:
			heldCount++
		case ApprovalResultComment:
			commentedCount++
		case ApprovalResultQuit:
			fmt.Println("Exiting approval process.")
			goto exitLoop
		}

		fmt.Printf("\n")
	}

exitLoop:
	// Print final summary
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")
	fmt.Printf("📊 Final Approval Summary:\n")
	fmt.Printf("   ✅ Approved: %d\n", approvedCount)
	fmt.Printf("   ❌ Skipped: %d\n", skippedCount)
	fmt.Printf("   ⏸️  Put on hold: %d\n", heldCount)
	fmt.Printf("   💬 Commented: %d\n", commentedCount)
	fmt.Printf("   📊 Total processed: %d\n", approvedCount+skippedCount+heldCount+commentedCount)
}

// approveSinglePRWithCache handles the approval process for a single PR with cache reuse
func approveSinglePRWithCache(client api.RESTClient, owner, repo string, pr PullRequest, config ApprovalConfig, cache *PRDetailsCache) ApprovalResult {
	// Build help message based on what's already shown
	helpOptions := []string{"[y]es to approve", "[N]o to skip (default)", "[h]old", "[q]uit"}
	if !showFiles {
		helpOptions = append(helpOptions, "[f]iles to view")
	}
	if !showDiff {
		helpOptions = append(helpOptions, "[d]iff to view")
	}
	helpOptions = append(helpOptions, "[c]hecks to view")

	fmt.Printf("Commands: %s\n", strings.Join(helpOptions, ", "))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	// Check if PR is already approved by current user
	reviewsPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number)
	var reviews []Review
	err := client.Get(reviewsPath, &reviews)
	if err != nil {
		fmt.Printf("⚠️  Could not check existing reviews for %s: %v\n", formatPRLink(owner, repo, pr.Number), err)
		// Continue with prompt despite error
	} else {
		// Check if we already have an approval from any user
		alreadyApproved := false
		for _, review := range reviews {
			if review.State == "APPROVED" {
				alreadyApproved = true
				break
			}
		}

		if alreadyApproved {
			fmt.Printf("✅ PR %s is already approved: %s\n", formatPRLink(owner, repo, pr.Number), pr.Title)
			fmt.Printf("Do you want to continue anyway? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil || strings.ToLower(strings.TrimSpace(response)) != "y" {
				fmt.Printf("Skipping already approved PR.\n")
				return ApprovalResultSkip
			}
		}
	}

	// Prompt user for approval decision - reuse the provided cache
	result := promptForApprovalWithCache(pr, owner, repo, client, config, cache)
	switch result {
	case ApprovalResultSkip:
		fmt.Printf("❌ Skipped PR %s\n", formatPRLink(owner, repo, pr.Number))
		return ApprovalResultSkip
	case ApprovalResultHold:
		fmt.Printf("⏸️  Put PR %s on hold\n", formatPRLink(owner, repo, pr.Number))
		return ApprovalResultHold
	case ApprovalResultQuit:
		return ApprovalResultQuit
	case ApprovalResultComment:
		fmt.Printf("💬 Added comment to PR %s\n", formatPRLink(owner, repo, pr.Number))
		return ApprovalResultComment
	case ApprovalResultApprove:
		// Check for migration warnings and ask for additional confirmation
		if hasMigrationWarning(pr) {
			fmt.Printf("\n🚨 ⚠️  MIGRATION WARNING DETECTED ⚠️  🚨\n")
			fmt.Printf("This PR contains migration warnings which may indicate breaking changes or\n")
			fmt.Printf("require special attention during deployment.\n\n")
			fmt.Printf("Are you sure you want to approve this PR with migration warnings? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			confirmResponse, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading confirmation: %v (skipping PR)\n", err)
				return ApprovalResultSkip
			}

			confirmResponse = strings.TrimSpace(strings.ToLower(confirmResponse))
			if confirmResponse != "y" && confirmResponse != "yes" {
				fmt.Printf("❌ Approval cancelled due to migration warnings. Skipping PR %s\n", formatPRLink(owner, repo, pr.Number))
				return ApprovalResultSkip
			}

			fmt.Printf("✅ Confirmed - proceeding with approval despite migration warnings.\n")
		}
		// Continue with approval process below
	}

	// Create approval review
	reviewPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number)
	review := ReviewRequest{
		Body:  "/lgtm",
		Event: "APPROVE",
	}

	// Convert review to JSON
	reviewJSON, err := json.Marshal(review)
	if err != nil {
		fmt.Printf("❌ Failed to marshal review for %s: %v\n", formatPRLink(owner, repo, pr.Number), err)
		return ApprovalResultSkip
	}

	fmt.Printf("✅ Approving %s: %s\n", formatPRLink(owner, repo, pr.Number), pr.Title)

	// Add the approval review
	err = client.Post(reviewPath, bytes.NewReader(reviewJSON), nil)
	if err != nil {
		fmt.Printf("❌ Failed to approve %s: %v\n", formatPRLink(owner, repo, pr.Number), err)
		return ApprovalResultSkip
	}

	fmt.Printf("   ✓ Successfully approved %s\n", formatPRLink(owner, repo, pr.Number))
	return ApprovalResultApprove
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

// needsRebase checks if a PR needs a rebase based on mergeable_state
func needsRebase(pr PullRequest) bool {
	switch pr.MergeableState {
	case "dirty", "behind":
		return true
	default:
		return false
	}
}

// isBlocked checks if a PR is blocked from merging based on mergeable_state
func isBlocked(pr PullRequest) bool {
	return pr.MergeableState == "blocked"
}

// PRDetailsCache caches fetched PR details to avoid duplicate API calls
type PRDetailsCache struct {
	cache map[int]*PullRequest
}

// NewPRDetailsCache creates a new PR details cache
func NewPRDetailsCache() *PRDetailsCache {
	return &PRDetailsCache{
		cache: make(map[int]*PullRequest),
	}
}

// GetOrFetch gets PR details from cache or fetches them if not cached
func (c *PRDetailsCache) GetOrFetch(client api.RESTClient, owner, repo string, prNumber int, originalPR PullRequest) *PullRequest {
	// If the original PR already has mergeable_state populated, use it
	if originalPR.MergeableState != "" {
		return &originalPR
	}

	// Check cache first
	if cachedPR, exists := c.cache[prNumber]; exists {
		return cachedPR
	}

	// Fetch from API and cache the result
	var pr PullRequest
	prPath := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, prNumber)
	err := client.Get(prPath, &pr)
	if err != nil {
		// If we can't fetch details, cache the original PR to avoid retrying
		c.cache[prNumber] = &originalPR
		return &originalPR
	}

	// Cache the fetched PR details
	c.cache[prNumber] = &pr
	return &pr
}

// fetchPRDetails fetches full PR details including mergeable_state
func fetchPRDetails(client api.RESTClient, owner, repo string, prNumber int) (*PullRequest, error) {
	var pr PullRequest
	prPath := fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, prNumber)
	err := client.Get(prPath, &pr)
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

// needsRebaseWithCache checks if a PR needs a rebase using cached details
func needsRebaseWithCache(cache *PRDetailsCache, client api.RESTClient, owner, repo string, pr PullRequest) bool {
	fullPR := cache.GetOrFetch(client, owner, repo, pr.Number, pr)
	return needsRebase(*fullPR)
}

// isBlockedWithCache checks if a PR is blocked using cached details
func isBlockedWithCache(cache *PRDetailsCache, client api.RESTClient, owner, repo string, pr PullRequest) bool {
	fullPR := cache.GetOrFetch(client, owner, repo, pr.Number, pr)
	return isBlocked(*fullPR)
}

// isReviewed checks if a PR has any approved reviews or approved/lgtm labels
func isReviewed(client api.RESTClient, owner, repo string, prNumber int, labels []Label) bool {
	// First check for approved/lgtm labels
	for _, label := range labels {
		if label.Name == "approved" || label.Name == "lgtm" {
			return true
		}
	}

	// Then check for approved reviews
	reviewsPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)
	var reviews []Review
	err := client.Get(reviewsPath, &reviews)
	if err != nil {
		// If we can't fetch reviews, assume not reviewed
		return false
	}

	// Check if we have any approved reviews
	for _, review := range reviews {
		if review.State == "APPROVED" {
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

// hasMigrationWarning checks if a PR contains migration warnings
func hasMigrationWarning(pr PullRequest) bool {
	// Check for migration warning patterns in the PR body
	// ⚠️[migration] or :warning:[migration] or ⚠️migration⚠️ or [migration]
	bodyLower := strings.ToLower(pr.Body)

	// Look for various migration warning patterns
	migrationPatterns := []string{
		"⚠️[migration]",
		":warning:[migration]",
		"⚠️migration⚠️",
		"[migration]",
	}

	for _, pattern := range migrationPatterns {
		if strings.Contains(bodyLower, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// isKonfluxNudge checks if a PR has the "konflux-nudge" label
func isKonfluxNudge(pr PullRequest) bool {
	for _, label := range pr.Labels {
		if label.Name == "konflux-nudge" {
			return true
		}
	}
	return false
}

// getCheckStatus fetches and analyzes the status of all checks for a PR
func getCheckStatus(client api.RESTClient, owner, repo string, prNumber int, headSHA string) (*CheckStatus, error) {
	status := &CheckStatus{}

	// Get check runs (newer GitHub checks API)
	checkRunsPath := fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, headSHA)
	var checkRunsResp CheckRunsResponse
	err := client.Get(checkRunsPath, &checkRunsResp)
	if err != nil {
		// If check runs API fails, we'll try the legacy status API below
		fmt.Printf("   ⚠️  Could not fetch check runs: %v\n", err)
	} else {
		for _, checkRun := range checkRunsResp.CheckRuns {
			status.Total++
			switch checkRun.Status {
			case "completed":
				switch checkRun.Conclusion {
				case "success":
					status.Passed++
				case "failure", "timed_out", "action_required":
					status.Failed++
				case "cancelled":
					status.Cancelled++
				case "skipped", "neutral":
					status.Skipped++
				}
			case "queued", "in_progress":
				status.Pending++
			}
		}
	}

	// Get legacy status checks
	statusPath := fmt.Sprintf("repos/%s/%s/commits/%s/status", owner, repo, headSHA)
	var statusResp struct {
		State    string        `json:"state"`
		Statuses []StatusCheck `json:"statuses"`
	}
	err = client.Get(statusPath, &statusResp)
	if err != nil {
		fmt.Printf("   ⚠️  Could not fetch status checks: %v\n", err)
	} else {
		for _, statusCheck := range statusResp.Statuses {
			status.Total++
			switch statusCheck.State {
			case "success":
				status.Passed++
			case "failure", "error":
				status.Failed++
			case "pending":
				status.Pending++
			}
		}
	}

	return status, nil
}

// displayCheckStatus shows the status of checks for a PR
func displayCheckStatus(client api.RESTClient, owner, repo string, prNumber int, headSHA string) {
	checkStatus, err := getCheckStatus(client, owner, repo, prNumber, headSHA)
	if err != nil {
		fmt.Printf("   ⚠️  Could not fetch check status: %v\n", err)
		return
	}

	if checkStatus.Total == 0 {
		fmt.Printf("   ✅ No checks configured\n")
		return
	}

	// Build status summary
	statusParts := []string{}
	if checkStatus.Passed > 0 {
		statusParts = append(statusParts, fmt.Sprintf("✅ %d passed", checkStatus.Passed))
	}
	if checkStatus.Failed > 0 {
		statusParts = append(statusParts, fmt.Sprintf("❌ %d failed", checkStatus.Failed))
	}
	if checkStatus.Pending > 0 {
		statusParts = append(statusParts, fmt.Sprintf("🟡 %d pending", checkStatus.Pending))
	}
	if checkStatus.Cancelled > 0 {
		statusParts = append(statusParts, fmt.Sprintf("⚫ %d cancelled", checkStatus.Cancelled))
	}
	if checkStatus.Skipped > 0 {
		statusParts = append(statusParts, fmt.Sprintf("⚪ %d skipped", checkStatus.Skipped))
	}

	// Show overall status with appropriate icon
	var overallIcon string
	if checkStatus.Failed > 0 {
		overallIcon = "❌"
	} else if checkStatus.Pending > 0 {
		overallIcon = "🟡"
	} else if checkStatus.Passed > 0 {
		overallIcon = "✅"
	} else {
		overallIcon = "⚪"
	}

	fmt.Printf("   %s Checks (%d total): %s (press 'c' during approval to view details)\n", overallIcon, checkStatus.Total, strings.Join(statusParts, ", "))
}

// displayDetailedCheckStatus shows detailed information about all checks for a PR
func displayDetailedCheckStatus(client api.RESTClient, owner, repo string, prNumber int, headSHA string) {
	fmt.Printf("\n🔍 Detailed check status for PR %s:\n", formatPRLink(owner, repo, prNumber))

	// Get check runs (newer GitHub checks API)
	checkRunsPath := fmt.Sprintf("repos/%s/%s/commits/%s/check-runs", owner, repo, headSHA)
	var checkRunsResp CheckRunsResponse
	err := client.Get(checkRunsPath, &checkRunsResp)
	if err == nil && len(checkRunsResp.CheckRuns) > 0 {
		fmt.Printf("\n📋 Check Runs:\n")
		for _, checkRun := range checkRunsResp.CheckRuns {
			var icon string
			var status string

			switch checkRun.Status {
			case "completed":
				switch checkRun.Conclusion {
				case "success":
					icon = "✅"
					status = "passed"
				case "failure", "timed_out", "action_required":
					icon = "❌"
					status = fmt.Sprintf("failed (%s)", checkRun.Conclusion)
				case "cancelled":
					icon = "⚫"
					status = "cancelled"
				case "skipped", "neutral":
					icon = "⚪"
					status = fmt.Sprintf("skipped (%s)", checkRun.Conclusion)
				default:
					icon = "❓"
					status = checkRun.Conclusion
				}
			case "queued":
				icon = "🟡"
				status = "queued"
			case "in_progress":
				icon = "🟡"
				status = "running"
			default:
				icon = "❓"
				status = checkRun.Status
			}

			fmt.Printf("   %s %s: %s\n", icon, checkRun.Name, status)
		}
	}

	// Get legacy status checks
	statusPath := fmt.Sprintf("repos/%s/%s/commits/%s/status", owner, repo, headSHA)
	var statusResp struct {
		State    string        `json:"state"`
		Statuses []StatusCheck `json:"statuses"`
	}
	err = client.Get(statusPath, &statusResp)
	if err == nil && len(statusResp.Statuses) > 0 {
		fmt.Printf("\n📋 Status Checks:\n")
		for _, statusCheck := range statusResp.Statuses {
			var icon string
			switch statusCheck.State {
			case "success":
				icon = "✅"
			case "failure", "error":
				icon = "❌"
			case "pending":
				icon = "🟡"
			default:
				icon = "❓"
			}

			description := statusCheck.Description
			if description == "" {
				description = statusCheck.State
			}

			fmt.Printf("   %s %s: %s\n", icon, statusCheck.Context, description)
		}
	}

	fmt.Printf("\n")
}

// holdPR puts a PR on hold by commenting /hold and adding the "needs-ok-to-test" label
func holdPR(client api.RESTClient, owner, repo string, prNumber int, additionalComment string) error {
	// Build the comment body
	commentBody := "/hold"
	if additionalComment != "" {
		commentBody += "\n\n" + additionalComment
	}

	// Add the /hold comment
	commentPath := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, prNumber)
	comment := CommentRequest{
		Body: commentBody,
	}

	commentJSON, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment: %v", err)
	}

	err = client.Post(commentPath, bytes.NewReader(commentJSON), nil)
	if err != nil {
		return fmt.Errorf("failed to add /hold comment: %v", err)
	}

	// Add the "needs-ok-to-test" label
	labelPath := fmt.Sprintf("repos/%s/%s/issues/%d/labels", owner, repo, prNumber)
	labelRequest := LabelRequest{
		Labels: []string{"needs-ok-to-test"},
	}

	labelJSON, err := json.Marshal(labelRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal label request: %v", err)
	}

	err = client.Post(labelPath, bytes.NewReader(labelJSON), nil)
	if err != nil {
		return fmt.Errorf("failed to add label: %v", err)
	}

	return nil
}

// addCommentToPR adds a comment to a pull request
func addCommentToPR(client api.RESTClient, owner, repo string, prNumber int, commentText string) error {
	commentPath := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, prNumber)
	comment := CommentRequest{
		Body: commentText,
	}

	// Convert comment to JSON
	commentJSON, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("failed to marshal comment: %v", err)
	}

	// Add the comment
	err = client.Post(commentPath, bytes.NewReader(commentJSON), nil)
	if err != nil {
		return fmt.Errorf("failed to post comment: %v", err)
	}

	return nil
}

// getStatusIcon returns the appropriate icon and status for a PR
func getStatusIcon(pr PullRequest) string {
	onHold := isOnHold(pr)

	if pr.Draft {
		return "🟡"
	}

	switch pr.State {
	case "open":
		if onHold {
			return "🔶"
		}
		return "🟢"
	case "closed":
		return "🔴"
	case "merged":
		return "🟣"
	default:
		if onHold {
			return "🔶"
		}
		return "⚪"
	}
}

// getStatusIconWithTekton returns the appropriate icon and status for a PR, including Tekton and migration info
func getStatusIconWithTekton(pr PullRequest, hasTektonFiles bool) string {
	onHold := isOnHold(pr)

	if pr.Draft {
		return "🟡"
	}

	switch pr.State {
	case "open":
		if onHold {
			return "🔶"
		}
		return "🟢"
	case "closed":
		return "🔴"
	case "merged":
		return "🟣"
	default:
		if onHold {
			return "🔶"
		}
		return "⚪"
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

// displayDiff shows the diff content for a PR with color coding
func displayDiff(owner, repo string, prNumber int) error {
	// The go-gh REST client doesn't expose direct HTTP methods for custom Accept headers,
	// so we use a direct approach: use the .diff URL directly with authentication
	// We'll construct the URL and use Go's http package but with authentication from go-gh
	diffURL := fmt.Sprintf("https://github.com/%s/%s/pull/%d.diff", owner, repo, prNumber)

	// Create an HTTP request
	req, err := http.NewRequest("GET", diffURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create diff request: %v", err)
	}

	// Try to get authentication token from environment (same as go-gh uses)
	if token := os.Getenv("GH_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	} else if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	// Make the request
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch diff: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to fetch diff: HTTP %d", resp.StatusCode)
	}

	// Read the diff content
	diffContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read diff: %v", err)
	}

	// Display the diff with color coding
	fmt.Printf("\n📄 Diff for PR %s:\n", formatPRLink(owner, repo, prNumber))
	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	// Apply color coding to the diff (unless colors are disabled)
	if shouldUseColors() {
		colorizedDiff := colorizeGitDiff(string(diffContent))
		fmt.Print(colorizedDiff)
	} else {
		fmt.Print(string(diffContent))
	}

	fmt.Printf("═══════════════════════════════════════════════════════════════\n")

	return nil
}

// colorizeGitDiff adds ANSI color codes to diff output similar to git diff
func colorizeGitDiff(diff string) string {
	// ANSI color codes
	const (
		reset   = "\033[0m"
		bold    = "\033[1m"
		red     = "\033[31m"
		green   = "\033[32m"
		yellow  = "\033[33m"
		blue    = "\033[34m"
		magenta = "\033[35m"
		cyan    = "\033[36m"
		white   = "\033[37m"
		dimGray = "\033[90m"
	)

	lines := strings.Split(diff, "\n")
	var colorizedLines []string

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			// File header - bold white
			colorizedLines = append(colorizedLines, bold+white+line+reset)
		case strings.HasPrefix(line, "index "):
			// Index line - dim gray
			colorizedLines = append(colorizedLines, dimGray+line+reset)
		case strings.HasPrefix(line, "--- "):
			// Old file - red
			colorizedLines = append(colorizedLines, red+line+reset)
		case strings.HasPrefix(line, "+++ "):
			// New file - green
			colorizedLines = append(colorizedLines, green+line+reset)
		case strings.HasPrefix(line, "@@"):
			// Hunk header - cyan
			colorizedLines = append(colorizedLines, cyan+line+reset)
		case strings.HasPrefix(line, "+"):
			// Added lines - green
			colorizedLines = append(colorizedLines, green+line+reset)
		case strings.HasPrefix(line, "-"):
			// Removed lines - red
			colorizedLines = append(colorizedLines, red+line+reset)
		case strings.HasPrefix(line, "new file mode"):
			// New file mode - green
			colorizedLines = append(colorizedLines, green+line+reset)
		case strings.HasPrefix(line, "deleted file mode"):
			// Deleted file mode - red
			colorizedLines = append(colorizedLines, red+line+reset)
		case strings.HasPrefix(line, "rename from") || strings.HasPrefix(line, "rename to"):
			// Rename operations - yellow
			colorizedLines = append(colorizedLines, yellow+line+reset)
		case strings.HasPrefix(line, "similarity index") || strings.HasPrefix(line, "dissimilarity index"):
			// Similarity index - dim gray
			colorizedLines = append(colorizedLines, dimGray+line+reset)
		default:
			// Context lines - no color
			colorizedLines = append(colorizedLines, line)
		}
	}

	return strings.Join(colorizedLines, "\n")
}

// shouldUseColors determines if we should colorize output
func shouldUseColors() bool {
	// If user explicitly disabled colors, respect that
	if noColor {
		return false
	}

	// Check if NO_COLOR environment variable is set (standard convention)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if output is going to a terminal
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// formatPRLink creates a clickable link for a PR number using OSC 8 escape sequences
func formatPRLink(owner, repo string, prNumber int) string {
	// Check if we should use terminal features (similar to color check)
	if noColor || os.Getenv("NO_COLOR") != "" || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Sprintf("#%d", prNumber)
	}

	url := fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNumber)
	return fmt.Sprintf("\033]8;;%s\033\\#%d\033]8;;\033\\", url, prNumber)
}

// truncateString truncates a string to a maximum display width with ellipsis
func TruncateString(s string, maxWidth int) string {
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		// If maxWidth is very small, just truncate by runes
		runes := []rune(s)
		if len(runes) <= maxWidth {
			return s
		}
		return string(runes[:maxWidth])
	}

	// Truncate to fit within maxWidth - 3 (for "...")
	targetWidth := maxWidth - 3
	runes := []rune(s)
	currentWidth := 0

	for i, r := range runes {
		charWidth := 1
		if r >= 0x1F600 && r <= 0x1F64F || // Emoticons
			r >= 0x1F300 && r <= 0x1F5FF || // Misc Symbols and Pictographs
			r >= 0x1F680 && r <= 0x1F6FF || // Transport and Map
			r >= 0x1F1E0 && r <= 0x1F1FF || // Regional indicators
			r >= 0x2600 && r <= 0x26FF || // Misc symbols
			r >= 0x2700 && r <= 0x27BF { // Dingbats
			charWidth = 2
		}

		if currentWidth+charWidth > targetWidth {
			return string(runes[:i]) + "..."
		}
		currentWidth += charWidth
	}

	return s
}

// displayWidth calculates the visual width of a string in the terminal
func DisplayWidth(s string) int {
	// Remove ANSI escape sequences (including OSC 8 sequences for links)
	cleanString := StripANSISequences(s)

	width := 0
	for _, r := range cleanString {
		// Most emojis and some Unicode characters take 2 character widths
		if r >= 0x1F600 && r <= 0x1F64F || // Emoticons
			r >= 0x1F300 && r <= 0x1F5FF || // Misc Symbols and Pictographs
			r >= 0x1F680 && r <= 0x1F6FF || // Transport and Map
			r >= 0x1F7E0 && r <= 0x1F7EB || // Geometric Shapes Extended (colored circles)
			r >= 0x1F1E0 && r <= 0x1F1FF || // Regional indicators
			r >= 0x2600 && r <= 0x26FF || // Misc symbols
			r >= 0x2700 && r <= 0x27BF || // Dingbats
			r == 0x200D || // Zero width joiner
			r >= 0xFE0F && r <= 0xFE0F { // Variation selectors
			width += 2
		} else if r >= 0x20 { // Printable ASCII and most Unicode
			width += 1
		}
		// Control characters (< 0x20) don't add width
	}
	return width
}

// stripANSISequences removes ANSI escape sequences from a string
func StripANSISequences(s string) string {
	result := strings.Builder{}
	i := 0
	runes := []rune(s)

	for i < len(runes) {
		if runes[i] == '\033' && i+1 < len(runes) { // ESC character
			i++ // Skip the ESC

			if i < len(runes) && runes[i] == ']' { // OSC sequence (like ]8;;URL\033\\)
				i++ // Skip the ]
				// Skip everything until we find the terminator
				for i < len(runes) {
					if runes[i] == '\007' { // BEL terminator
						i++
						break
					} else if runes[i] == '\033' && i+1 < len(runes) && runes[i+1] == '\\' { // ST terminator
						i += 2 // Skip \033\
						break
					}
					i++
				}
			} else if i < len(runes) && runes[i] == '[' { // CSI sequence (like [31m)
				i++ // Skip the [
				// Skip until we find the final byte (@ to ~)
				for i < len(runes) {
					if runes[i] >= 0x40 && runes[i] <= 0x7E {
						i++
						break
					}
					i++
				}
			} else {
				// Other escape sequences, skip until final byte
				for i < len(runes) {
					if runes[i] >= 0x40 && runes[i] <= 0x7E {
						i++
						break
					}
					i++
				}
			}
		} else {
			result.WriteRune(runes[i])
			i++
		}
	}

	return result.String()
}

// padString pads a string to a specific width, accounting for actual display width
func PadString(s string, width int) string {
	currentWidth := DisplayWidth(s)
	if currentWidth >= width {
		return s
	}
	padding := width - currentWidth
	return s + strings.Repeat(" ", padding)
}

// displayLegend shows what the various emojis and symbols mean in the table
func displayLegend(isKonflux bool) {
	fmt.Println("Legend:")
	fmt.Println("  Status: 🟢 open  🟡 draft  🔶 on hold  🔴 closed  🟣 merged")
	fmt.Println("  Reviewed: ✅ approved  ❌ not approved")
	fmt.Println("  Rebase: 🔄 needs rebase  - N/A (on hold)  (empty = up to date)")
	fmt.Println("  Blocked: 🚫 blocked from merging  - N/A (on hold)  (empty = not blocked)")
	fmt.Println("  Nudge: 👉 konflux nudge PR  (empty = not a nudge)")
	if isKonflux {
		fmt.Println("  Tekton: ✅ exclusively Tekton files  ❌ mixed/other files")
		fmt.Println("  🚨 = migration warning")
	}
	fmt.Println()
}

// displayPRTableWithCache displays PRs in a table format using an optional existing cache
func displayPRTable(pullRequests []PullRequest, owner, repo string, client *api.RESTClient, isKonflux bool, cache *PRDetailsCache) *PRDetailsCache {
	// Use existing cache or create a new one
	if cache == nil {
		cache = NewPRDetailsCache()
	}

	if len(pullRequests) == 0 {
		return cache
	}

	// Display legend first
	displayLegend(isKonflux)

	// Define column widths - compact but readable
	const (
		statusWidth   = 2  // Emoji width
		prWidth       = 6  // "#1234"
		titleWidth    = 41 // Full title width
		authorWidth   = 16 // Author names
		branchWidth   = 14 // Source branch names
		targetWidth   = 12 // Target branch names
		stateWidth    = 10 // "STATUS"
		reviewedWidth = 8  // "REVIEWED"
		rebaseWidth   = 6  // "REBASE"
		blockedWidth  = 7  // "BLOCKED"
		nudgeWidth    = 5  // "NUDGE"
		tektonWidth   = 6  // "TEKTON"
	)

	// Print table header
	fmt.Printf("%s %s %s %s %s %s %s %s %s %s %s",
		PadString("ST", statusWidth),
		PadString("PR", prWidth),
		PadString("TITLE", titleWidth),
		PadString("AUTHOR", authorWidth),
		PadString("BRANCH", branchWidth),
		PadString("TARGET", targetWidth),
		PadString("STATUS", stateWidth),
		PadString("REVIEWED", reviewedWidth),
		PadString("REBASE", rebaseWidth),
		PadString("BLOCKED", blockedWidth),
		PadString("NUDGE", nudgeWidth))
	if isKonflux {
		fmt.Printf(" %s", PadString("TEKTON", tektonWidth))
	}
	fmt.Printf("\n")

	// Print separator line
	fmt.Printf("%s %s %s %s %s %s %s %s %s %s %s",
		PadString(strings.Repeat("-", statusWidth), statusWidth),
		PadString(strings.Repeat("-", prWidth), prWidth),
		PadString(strings.Repeat("-", titleWidth), titleWidth),
		PadString(strings.Repeat("-", authorWidth), authorWidth),
		PadString(strings.Repeat("-", branchWidth), branchWidth),
		PadString(strings.Repeat("-", targetWidth), targetWidth),
		PadString(strings.Repeat("-", stateWidth), stateWidth),
		PadString(strings.Repeat("-", reviewedWidth), reviewedWidth),
		PadString(strings.Repeat("-", rebaseWidth), rebaseWidth),
		PadString(strings.Repeat("-", blockedWidth), blockedWidth),
		PadString(strings.Repeat("-", nudgeWidth), nudgeWidth))
	if isKonflux {
		fmt.Printf(" %s", PadString(strings.Repeat("-", tektonWidth), tektonWidth))
	}
	fmt.Printf("\n")

	// Display each PR as a table row
	for _, pr := range pullRequests {
		// Check for Tekton files if this is a Konflux PR
		onlyTektonFiles := false
		if isKonflux {
			var err error
			onlyTektonFiles, _, err = checkTektonFilesDetailed(*client, owner, repo, pr.Number)
			if err != nil {
				// Silently continue if we can't check Tekton files for table display
				// Error is intentionally ignored for display purposes
				_ = err
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

		// Get status icon
		var icon string
		if isKonflux {
			icon = getStatusIconWithTekton(pr, onlyTektonFiles)
		} else {
			icon = getStatusIcon(pr)
		}

		// Prepare table data
		prLink := formatPRLink(owner, repo, pr.Number)
		title := TruncateString(pr.Title, titleWidth)
		author := TruncateString(pr.User.Login, authorWidth)
		branch := TruncateString(pr.Head.Ref, branchWidth)
		target := TruncateString(pr.Base.Ref, targetWidth)

		// Determine status text
		status := ""
		if pr.Draft {
			status = "draft"
		} else if isOnHold(pr) {
			status = "on hold"
		} else {
			status = pr.State
		}
		if hasMigration {
			status += " 🚨"
		}
		status = TruncateString(status, stateWidth)

		// Determine reviewed status
		reviewedStatus := ""
		if isReviewed(*client, owner, repo, pr.Number, pr.Labels) {
			reviewedStatus = "✅"
		} else {
			reviewedStatus = "❌"
		}

		// Determine rebase status - skip API call if PR is on hold
		rebaseStatus := ""
		if isOnHold(pr) {
			rebaseStatus = "-" // N/A for PRs on hold
		} else if needsRebaseWithCache(cache, *client, owner, repo, pr) {
			rebaseStatus = "🔄"
		}
		// Leave empty if no rebase needed

		// Determine blocked status - skip API call if PR is on hold
		blockedStatus := ""
		if isOnHold(pr) {
			blockedStatus = "-" // N/A for PRs on hold
		} else if isBlockedWithCache(cache, *client, owner, repo, pr) {
			blockedStatus = "🚫"
		}
		// Leave empty if not blocked

		// Determine nudge status
		nudgeStatus := ""
		if isKonfluxNudge(pr) {
			nudgeStatus = "👉"
		}

		// Print the row with proper padding
		fmt.Printf("%s %s %s %s %s %s %s %s %s %s %s",
			PadString(icon, statusWidth),
			PadString(prLink, prWidth),
			PadString(title, titleWidth),
			PadString(author, authorWidth),
			PadString(branch, branchWidth),
			PadString(target, targetWidth),
			PadString(status, stateWidth),
			PadString(reviewedStatus, reviewedWidth),
			PadString(rebaseStatus, rebaseWidth),
			PadString(blockedStatus, blockedWidth),
			PadString(nudgeStatus, nudgeWidth))

		if isKonflux {
			tektonStatus := ""
			if onlyTektonFiles {
				tektonStatus = "✅"
			} else {
				tektonStatus = "❌"
			}
			fmt.Printf(" %s", PadString(tektonStatus, tektonWidth))
		}

		fmt.Printf("\n")
	}

	// Return the cache for potential reuse in approval flow
	return cache
}

func init() {
	RootCmd.AddCommand(listCmd)
	RootCmd.AddCommand(konfluxCmd)

	// Add flags to both commands
	listCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	listCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
	listCmd.Flags().BoolVarP(&current, "current", "c", false, "Use current repository, bypass config")
	listCmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort PRs by: newest (default), oldest, updated, number, priority")
	listCmd.Flags().BoolVarP(&approve, "approve", "a", false, "Interactively approve pull requests (review + /lgtm comment)")
	listCmd.Flags().BoolVarP(&showFiles, "show-files", "f", false, "Show detailed file list during approval process")
	listCmd.Flags().BoolVarP(&showDiff, "show-diff", "d", false, "Show detailed diff during approval process")
	listCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable color output in diff display")

	konfluxCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	konfluxCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
	konfluxCmd.Flags().BoolVarP(&current, "current", "c", false, "Use current repository, bypass config")
	konfluxCmd.Flags().BoolVarP(&approve, "approve", "a", false, "Interactively approve Konflux pull requests (review + /lgtm comment)")
	konfluxCmd.Flags().BoolVarP(&tektonOnly, "tekton-only", "t", false, "Show only PRs that EXCLUSIVELY modify Tekton files (.tekton/*-pull-request.yaml or *-push.yaml)")
	konfluxCmd.Flags().BoolVarP(&migrationOnly, "migration-only", "m", false, "Show only PRs that contain migration warnings")
	konfluxCmd.Flags().StringVar(&sortBy, "sort-by", "", "Sort PRs by: newest (default), oldest, updated, number, priority")
	konfluxCmd.Flags().BoolVarP(&showFiles, "show-files", "f", false, "Show detailed file list during approval process")
	konfluxCmd.Flags().BoolVarP(&showDiff, "show-diff", "d", false, "Show detailed diff during approval process")
	konfluxCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable color output in diff display")
}

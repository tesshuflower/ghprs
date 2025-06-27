package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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
	Number         int    `json:"number"`
	Title          string `json:"title"`
	State          string `json:"state"`
	User           User   `json:"user"`
	Head           Branch `json:"head"`
	Base           Branch `json:"base"`
	Draft          bool   `json:"draft"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	HTMLURL        string `json:"html_url"`
	Body           string `json:"body"`
	MergeableState string `json:"mergeable_state"`
}

type User struct {
	Login string `json:"login"`
}

type Branch struct {
	Ref string `json:"ref"`
}

// ReviewRequest represents a pull request review request
type ReviewRequest struct {
	Body  string `json:"body"`
	Event string `json:"event"`
}

// Review represents a pull request review
type Review struct {
	State string `json:"state"`
	User  User   `json:"user"`
}

var (
	state   string
	limit   int
	approve bool
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list [owner/repo]",
	Short: "List pull requests for a repository",
	Long: `List pull requests for a GitHub repository.

If no repository is specified, the current repository will be detected from git remotes.
You can also specify a repository in the format "owner/repo".

Examples:
  ghprs list
  ghprs list microsoft/vscode
  ghprs list --state closed
  ghprs list --limit 5`,
	Run: func(cmd *cobra.Command, args []string) {
		listPullRequests(args, "", false)
	},
}

// konfluxCmd represents the konflux command
var konfluxCmd = &cobra.Command{
	Use:   "konflux [owner/repo]",
	Short: "List Konflux pull requests (authored by red-hat-konflux[bot])",
	Long: `List pull requests authored by "red-hat-konflux[bot]" for a GitHub repository.

If no repository is specified, the current repository will be detected from git remotes.
You can also specify a repository in the format "owner/repo".

Examples:
  ghprs konflux
  ghprs konflux microsoft/vscode
  ghprs konflux --state closed
  ghprs konflux --limit 5
  ghprs konflux --approve                    # Approve all open Konflux PRs
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
			// Color-code based on state and draft status
			icon := getStateIcon(pr.State, pr.Draft)
			fmt.Printf("%s #%-4d %s\n", icon, pr.Number, pr.Title)
			fmt.Printf("        %s → %s by @%s\n", pr.Head.Ref, pr.Base.Ref, pr.User.Login)
			fmt.Printf("        %s\n\n", pr.HTMLURL)
		}
	}
}

func approveKonfluxPRs(client api.RESTClient, owner, repo string, pullRequests []PullRequest) {
	approvedCount := 0
	skippedCount := 0
	alreadyApprovedCount := 0

	for _, pr := range pullRequests {
		// Only approve open PRs
		if pr.State != "open" {
			fmt.Printf("⏭️  Skipping #%d (state: %s): %s\n", pr.Number, pr.State, pr.Title)
			skippedCount++
			continue
		}

		// Skip draft PRs
		if pr.Draft {
			fmt.Printf("⏭️  Skipping #%d (draft): %s\n", pr.Number, pr.Title)
			skippedCount++
			continue
		}

		// Check if PR is already approved by current user
		reviewsPath := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, pr.Number)
		var reviews []Review
		err := client.Get(reviewsPath, &reviews)
		if err != nil {
			fmt.Printf("⚠️  Could not check existing reviews for #%d: %v\n", pr.Number, err)
			// Continue with approval attempt despite error
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

		err = client.Post(reviewPath, bytes.NewReader(reviewJSON), nil)
		if err != nil {
			fmt.Printf("❌ Failed to approve #%d: %v\n", pr.Number, err)
			continue
		}

		approvedCount++
		fmt.Printf("   ✓ Successfully approved #%d\n", pr.Number)
	}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   ✅ Approved: %d\n", approvedCount)
	fmt.Printf("   ✅ Already approved: %d\n", alreadyApprovedCount)
	fmt.Printf("   ⏭️  Skipped: %d\n", skippedCount)
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

func init() {
	RootCmd.AddCommand(listCmd)
	RootCmd.AddCommand(konfluxCmd)

	// Add flags to both commands
	listCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	listCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")

	konfluxCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	konfluxCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
	konfluxCmd.Flags().BoolVarP(&approve, "approve", "a", false, "Approve all open Konflux pull requests")
}

package cmd

import (
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

var (
	state string
	limit int
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
		var owner, repo string

		if len(args) > 0 {
			// Parse owner/repo from argument
			parts := strings.Split(args[0], "/")
			if len(parts) != 2 {
				log.Fatal("Repository must be in the format 'owner/repo'")
			}
			owner = parts[0]
			repo = parts[1]
		} else {
			// Auto-detect repository from current directory
			currentRepo, err := repository.Current()
			if err != nil {
				log.Fatal("Could not detect repository. Please run from a git repository or specify owner/repo manually.")
			}

			owner = currentRepo.Owner
			repo = currentRepo.Name
		}

		// Create REST API client
		client, err := api.DefaultRESTClient()
		if err != nil {
			log.Fatal("Failed to create GitHub client. Make sure you're authenticated with 'gh auth login' or set GH_TOKEN.")
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
		var pullRequests []PullRequest
		err = client.Get(path, &pullRequests)
		if err != nil {
			log.Fatalf("Failed to fetch pull requests: %v", err)
		}

		// Display results
		if len(pullRequests) == 0 {
			fmt.Printf("No %s pull requests found for %s/%s\n", state, owner, repo)
			return
		}

		fmt.Printf("Pull requests for %s/%s:\n\n", owner, repo)

		for _, pr := range pullRequests {
			// Color-code based on state and draft status
			icon := getStateIcon(pr.State, pr.Draft)
			fmt.Printf("%s #%-4d %s\n", icon, pr.Number, pr.Title)
			fmt.Printf("        %s → %s by @%s\n", pr.Head.Ref, pr.Base.Ref, pr.User.Login)
			fmt.Printf("        %s\n\n", pr.HTMLURL)
		}
	},
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

	listCmd.Flags().StringVarP(&state, "state", "s", "open", "Filter by state: open, closed, all")
	listCmd.Flags().IntVarP(&limit, "limit", "l", 30, "Maximum number of pull requests to show")
}

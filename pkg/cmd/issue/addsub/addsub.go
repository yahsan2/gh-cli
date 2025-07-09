package addsub

import (
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/issue/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type AddSubOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	BaseRepo   func() (ghrepo.Interface, error)

	ParentIssueArg string
	SubIssueArg    string
}

func NewCmdAddSub(f *cmdutil.Factory, runF func(*AddSubOptions) error) *cobra.Command {
	opts := &AddSubOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		BaseRepo:   f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "add-sub <parent-issue> <sub-issue>",
		Short: "Add a sub-issue to an issue",
		Long: heredoc.Doc(`
			Add a sub-issue to a parent issue, creating a hierarchical relationship.
			
			Both issues must exist in the same repository. The sub-issue will be
			linked to the parent issue, allowing for better organization of related work.
		`),
		Example: heredoc.Doc(`
			# Add issue #456 as a sub-issue of #123
			$ gh issue add-sub 123 456
			
			# Using URLs
			$ gh issue add-sub https://github.com/owner/repo/issues/123 456
			
			# With repository specification
			$ gh issue add-sub 123 456 --repo owner/repo
		`),
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// support `-R, --repo` override
			opts.BaseRepo = f.BaseRepo

			opts.ParentIssueArg = args[0]
			opts.SubIssueArg = args[1]

			if runF != nil {
				return runF(opts)
			}
			return addSubRun(opts)
		},
	}

	return cmd
}

func addSubRun(opts *AddSubOptions) error {
	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}

	baseRepo, err := opts.BaseRepo()
	if err != nil {
		return err
	}

	// Parse parent issue
	parentIssueNumber, parentRepo, err := shared.ParseIssueFromArg(opts.ParentIssueArg)
	if err != nil {
		return fmt.Errorf("invalid parent issue: %w", err)
	}
	if parentRepo.IsSome() {
		baseRepo = parentRepo.Unwrap()
	}

	// Parse sub-issue
	subIssueNumber, subRepo, err := shared.ParseIssueFromArg(opts.SubIssueArg)
	if err != nil {
		return fmt.Errorf("invalid sub-issue: %w", err)
	}
	if subRepo.IsSome() {
		if !ghrepo.IsSame(baseRepo, subRepo.Unwrap()) {
			return fmt.Errorf("parent issue and sub-issue must be in the same repository")
		}
	}

	// Check if issues are the same
	if parentIssueNumber == subIssueNumber {
		return fmt.Errorf("an issue cannot be its own sub-issue")
	}

	// Get issue IDs for GraphQL
	parentIssueID, err := getIssueID(httpClient, baseRepo, parentIssueNumber)
	if err != nil {
		return fmt.Errorf("failed to get parent issue ID: %w", err)
	}

	subIssueID, err := getIssueID(httpClient, baseRepo, subIssueNumber)
	if err != nil {
		return fmt.Errorf("failed to get sub-issue ID: %w", err)
	}

	// Execute GraphQL mutation
	err = addSubIssue(httpClient, parentIssueID, subIssueID)
	if err != nil {
		return fmt.Errorf("failed to add sub-issue: %w", err)
	}

	fmt.Fprintf(opts.IO.Out, "✓ Added issue #%d as a sub-issue of #%d\n", subIssueNumber, parentIssueNumber)
	return nil
}

func getIssueID(httpClient *http.Client, repo ghrepo.Interface, issueNumber int) (string, error) {
	query := `
		query($owner: String!, $repo: String!, $number: Int!) {
			repository(owner: $owner, name: $repo) {
				issueOrPullRequest(number: $number) {
					... on Issue {
						id
					}
				}
			}
		}
	`

	variables := map[string]interface{}{
		"owner":  repo.RepoOwner(),
		"repo":   repo.RepoName(),
		"number": issueNumber,
	}

	var response struct {
		Repository struct {
			IssueOrPullRequest struct {
				ID string `json:"id"`
			} `json:"issueOrPullRequest"`
		} `json:"repository"`
	}

	client := api.NewClientFromHTTP(httpClient)
	err := client.GraphQL(repo.RepoHost(), query, variables, &response)
	if err != nil {
		return "", err
	}

	if response.Repository.IssueOrPullRequest.ID == "" {
		return "", fmt.Errorf("issue #%d not found", issueNumber)
	}

	return response.Repository.IssueOrPullRequest.ID, nil
}

func addSubIssue(httpClient *http.Client, parentIssueID, subIssueID string) error {
	mutation := `
		mutation($issueId: ID!, $subIssueId: ID!) {
			addSubIssue(input: {issueId: $issueId, subIssueId: $subIssueId}) {
				clientMutationId
				issue {
					id
					number
					title
				}
				subIssue {
					id
					number
					title
				}
			}
		}
	`

	variables := map[string]interface{}{
		"issueId":    parentIssueID,
		"subIssueId": subIssueID,
	}

	var response struct {
		AddSubIssue struct {
			Issue struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"issue"`
			SubIssue struct {
				ID     string `json:"id"`
				Number int    `json:"number"`
				Title  string `json:"title"`
			} `json:"subIssue"`
		} `json:"addSubIssue"`
	}

	client := api.NewClientFromHTTP(httpClient)
	return client.GraphQL("github.com", mutation, variables, &response)
}

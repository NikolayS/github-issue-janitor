package main

import (
	"context"
	"fmt"
	"os"
	"time"

	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const GITHUB_ORG_NAME = "dotmesh-io"
const GITHUB_PROJECT_ID = 1
const GITHUB_BACKLOG_COLUMN = 1527643

func main() {
	GITHUB_IGNORED_REPOS := map[string]struct{}{
		"roadmap":             struct{}{},
		"moby-counter-issues": struct{}{},
	}

	GITHUB_IGNORED_LABELS := map[string]struct{}{
		"hypothesis": struct{}{},
	}

	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := gh.NewClient(tc)

	fmt.Printf("#### FINDING ISSUES IN REPOSITORIES\n")

	repos, _, err := client.Repositories.ListByOrg(ctx, GITHUB_ORG_NAME, &gh.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		fmt.Printf("Error fetching repository list: %+v\n", err)
		os.Exit(1)
	}

skipRepo:
	for _, repo := range repos {
		fmt.Printf("Got a repo: %+v\n", *(repo.Name))

		_, ignoredRepo := GITHUB_IGNORED_REPOS[*(repo.Name)]
		if ignoredRepo {
			fmt.Printf("Ignoring that one!\n")
			continue skipRepo
		}

		page := 1
		for {
			fmt.Printf("Listing issues for %s/%s\n", *(repo.Owner.Login), *(repo.Name))
			time.Sleep(3 * time.Second) // 30 requests per minute is the rate limit...
			issues, resp, err := client.Search.Issues(ctx, fmt.Sprintf(
				"no:project is:open repo:dotmesh-io/%s",
				*(repo.Name),
			), &gh.SearchOptions{
				ListOptions: gh.ListOptions{
					PerPage: 100,
					Page:    page},
			})

			if err != nil {
				fmt.Printf("Error fetching issue list: %+v\n", err)
				os.Exit(1)
			}

		skipIssue:
			for _, issue := range issues.Issues {
				issueTag := fmt.Sprintf("%s#%d", *(repo.Name), *(issue.Number))

				for _, label := range issue.Labels {
					_, ignoredLabel := GITHUB_IGNORED_LABELS[*(label.Name)]
					if ignoredLabel {
						fmt.Printf("Ignoring issue %s due to label %s\n", issueTag, *(label.Name))
						continue skipIssue
					}
				}

				fmt.Printf("Issue not in project: %s: %s\n", issueTag, *(issue.Title))

				client.Projects.CreateProjectCard(ctx, GITHUB_BACKLOG_COLUMN, &gh.ProjectCardOptions{
					ContentType: "Issue",
					ContentID:   *(issue.ID),
				})
			}

			if resp.NextPage == 0 {
				break
			} else {
				page = resp.NextPage
				fmt.Printf("Next page!\n")
			}
		}
	}
}

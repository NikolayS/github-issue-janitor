package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const GITHUB_ORG_NAME = "dotmesh-io"
const GITHUB_PROJECT_ID = 1
const GITHUB_BACKLOG_COLUMN = 1527643

func main() {
	GITHUB_COLUMN_IDS := []int64{
		1527643, // Backlog
		2044199, // Ready
		2620854, // Epics
		2620949, // Sprint
		2159385, // Sprint
		1527644, // In Progress
		2330460, // Blocked
		2591940, // Done
	}

	GITHUB_IGNORED_REPOS := map[string]struct{}{
		"roadmap": struct{}{},
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

	fmt.Printf("#### FINDING ISSUES ALREADY IN PROJECT\n")

	// Keys are "repo#id"
	issuesInProject := map[string]struct{}{}
	for _, column := range GITHUB_COLUMN_IDS {
		page := 1
		for {
			cards, resp, err := client.Projects.ListProjectCards(ctx, column, &gh.ListOptions{
				PerPage: 100,
				Page:    page,
			})
			if err != nil {
				fmt.Printf("Error fetching card list: %+v\n", err)
				return
			}
			for _, card := range cards {
				// Parse ContentURL to get issues already in project:
				// https://api.github.com/repos/dotmesh-io/dotmesh/issues/117
				// {repo}/issues/{id}
				if card.ContentURL != nil {
					parts := strings.Split(*(card.ContentURL), "/")
					repo := parts[5]
					id, err := strconv.Atoi(parts[7])
					if err == nil {
						fmt.Printf("Found issue %s#%d in project\n", repo, id)
						issuesInProject[fmt.Sprintf("%s#%d", repo, id)] = struct{}{}
					} else {
						fmt.Printf("Can't parse id %s from url %s: %+v\n", parts[7], parts, err)
					}
				}
			}

			if resp.NextPage == 0 {
				break
			} else {
				page = resp.NextPage
				fmt.Printf("Next page!\n")
			}
		}
	}

	fmt.Printf("#### FINDING ISSUES IN REPOSITORIES\n")

	repos, _, err := client.Repositories.ListByOrg(ctx, GITHUB_ORG_NAME, &gh.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		fmt.Printf("Error fetching repository list: %+v\n", err)
		return
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
			issues, resp, err := client.Issues.ListByRepo(ctx, *(repo.Owner.Login), *(repo.Name), &gh.IssueListByRepoOptions{
				ListOptions: gh.ListOptions{
					PerPage: 100,
					Page:    page},
			})

			if err != nil {
				fmt.Printf("Error fetching issue list: %+v\n", err)
				return
			}

		skipIssue:
			for _, issue := range issues {
				issueTag := fmt.Sprintf("%s#%d", *(repo.Name), *(issue.Number))

				for _, label := range issue.Labels {
					_, ignoredLabel := GITHUB_IGNORED_LABELS[*(label.Name)]
					if ignoredLabel {
						fmt.Printf("Ignoring issue %s due to label %s\n", issueTag, *(label.Name))
						continue skipIssue
					}
				}

				_, inProject := issuesInProject[issueTag]
				if inProject {
					fmt.Printf("Issue already in project: %s: %s\n", issueTag, *(issue.Title))
				} else {
					fmt.Printf("Issue not in project: %s: %s\n", issueTag, *(issue.Title))

					client.Projects.CreateProjectCard(ctx, GITHUB_BACKLOG_COLUMN, &gh.ProjectCardOptions{
						ContentType: "Issue",
						ContentID:   *(issue.ID),
					})
				}
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

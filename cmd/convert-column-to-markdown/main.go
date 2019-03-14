package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dotmesh-io/github-issue-janitor/pkg/utils"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Github API docs: https://godoc.org/github.com/google/go-github/github

const GITHUB_ORG_NAME = "dotmesh-io"

// Find all the issues in a given column (given the column URL from
// "Copy column link" in the column menu in the web UI) that aren't in
// an epic or theme, and generate Markdown suitable for putting in an
// epic/theme description listing them all.

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("USAGE: %s column-URL\n", os.Args[0])
		fmt.Printf("Get the column URL from the Github web UI by clicking the menu button for a column and selecting \"Copy column link\", eg https://github.com/orgs/dotmesh-io/projects/8#column-4716294\n")
		os.Exit(1)
	}

	inputColumn := os.Args[1]

	dashPos := strings.LastIndex(inputColumn, "-")
	if dashPos == -1 {
		fmt.Printf("Invalid input column URL %q\n", inputColumn)
	}
	columnId, err := strconv.ParseInt(inputColumn[dashPos+1:], 10, 64)
	if err != nil {
		fmt.Printf("Invalid input column URL %q: %s\n", inputColumn, err.Error())
	}

	GITHUB_IGNORED_REPOS := map[string]struct{}{
		"roadmap":             struct{}{},
		"moby-counter-issues": struct{}{},
	}

	GITHUB_IGNORED_LABELS := map[string]struct{}{
		"hypothesis": struct{}{},
	}

	// Labels that make an issue count as an epic
	EPIC_LABELS := map[string]struct{}{
		"epic":  struct{}{},
		"theme": struct{}{},
	}

	ctx := context.Background()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_AUTH_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := gh.NewClient(tc)

	repos, _, err := client.Repositories.ListByOrg(ctx, GITHUB_ORG_NAME, &gh.RepositoryListByOrgOptions{
		Type:        "all",
		ListOptions: gh.ListOptions{PerPage: 100},
	})
	if err != nil {
		fmt.Printf("Error fetching repository list: %s\n", err.Error())
		os.Exit(1)
	}

	// repo#number strings for issues that are mentioned in epics, mapped to the last epic it was spotted in
	issuesMentionedInEpics := map[string]string{}

	// Scan all repos for epics/themes and build up issuesMentionedInEpics
skipRepo:
	for idx, repo := range repos {
		rn := *(repo.Name)

		fmt.Printf("%s(%d/%d) ", rn, idx+1, len(repos))

		_, ignoredRepo := GITHUB_IGNORED_REPOS[rn]
		if ignoredRepo || *repo.Archived {
			continue skipRepo
		}

		// Process epics
		page := 1
		for {
			issues, resp, err := client.Search.Issues(ctx, fmt.Sprintf(
				"is:open repo:dotmesh-io/%s",
				rn,
			), &gh.SearchOptions{
				ListOptions: gh.ListOptions{
					PerPage: 100,
					Page:    page},
			})

			if err != nil {
				fmt.Printf("Error fetching issue list: %s %#v\n", err.Error(), resp)
				os.Exit(1)
			}

			if resp.Rate.Remaining < 10 {
				delay := time.Until(resp.Rate.Reset.Time)
				fmt.Printf("[rl] %s ", delay.String())
				time.Sleep(delay)
			}

		skipIssue:
			for _, issue := range issues.Issues {
				issueTag := fmt.Sprintf("%s#%d", rn, *(issue.Number))
				isEpic := false

				for _, label := range issue.Labels {
					_, ignoredLabel := GITHUB_IGNORED_LABELS[*(label.Name)]
					if ignoredLabel {
						continue skipIssue
					}
					_, isEpicLabel := EPIC_LABELS[*(label.Name)]
					if isEpicLabel {
						isEpic = true
					}
				}

				if isEpic {
					body := *(issue.Body)

					mentionedIssues := utils.ParseBodyForIssueLinks(body, GITHUB_ORG_NAME, rn)
					for _, mi := range mentionedIssues {
						issuesMentionedInEpics[mi] = issueTag
					}
				}
			}

			if resp.NextPage == 0 {
				break
			} else {
				page = resp.NextPage
			}
		}
	}

	// Scan the column, ignoring issues in issuesMentionedInEpics, and make markdown

	column, _, err := client.Projects.GetProjectColumn(ctx, columnId)
	if err != nil {
		fmt.Printf("Error fetching column: %s\n", err.Error())
		os.Exit(1)
	}

	fmt.Printf("\n### EXAMINING COLUMN %d: %s...\n", columnId, *(column.Name))

	page := 1
	for {
		pcs, resp, err := client.Projects.ListProjectCards(ctx, columnId, &gh.ListOptions{
			PerPage: 100,
			Page:    page,
		})
		if err != nil {
			fmt.Printf("Error fetching column contents: %s %#v\n", err.Error(), resp)
			os.Exit(1)
		}

		if resp.Rate.Remaining <= 5 {
			delay := time.Until(resp.Rate.Reset.Time)
			fmt.Printf("[rl] %s\n", delay.String())
			time.Sleep(delay)
		}

		for _, pc := range pcs {
			if pc.ContentURL != nil {
				issueUrl := *(pc.ContentURL)
				// issueUrl is like "https://api.github.com/repos/dotmesh-io/dotmesh/issues/386"
				parts := strings.Split(issueUrl, "/")
				repo := parts[len(parts)-3]
				issueNum, err := strconv.Atoi(parts[len(parts)-1])
				if err != nil {
					fmt.Printf("Error parsing issue URL: %s\n", err.Error())
					os.Exit(1)
				}

				issue, resp, err := client.Issues.Get(ctx, GITHUB_ORG_NAME, repo, issueNum)
				if err != nil {
					fmt.Printf("Error fetching issue %s#%d: %s %#v\n", repo, issueNum, err.Error(), resp)
					os.Exit(1)
				}

				if resp.Rate.Remaining < 10 {
					delay := time.Until(resp.Rate.Reset.Time)
					fmt.Printf("[rl] %s\n", delay.String())
					time.Sleep(delay)
				}

				labels := ""

				for _, l := range issue.Labels {
					if labels == "" {
						labels = *(l.Name)
					} else {
						labels = labels + " " + *(l.Name)
					}
				}

				issueLink := fmt.Sprintf("[%s#%d](https://github.com/%s/%s/issues/%d): %s (%s)", repo, issueNum, GITHUB_ORG_NAME, repo, issueNum, *(issue.Title), labels)

				epic, alreadyInEpic := issuesMentionedInEpics[fmt.Sprintf("%s#%d", repo, issueNum)]
				if !alreadyInEpic {
					if issue.ClosedAt == nil {
						fmt.Printf("- [ ] %s\n", issueLink)
					} else {
						// fmt.Printf("%s is closed\n", issueLink)
					}
				} else {
					_ = epic
					// fmt.Printf("%s is mentioned in %s\n", issueLink, epic)
				}
			} else {
				// fmt.Printf("Card has no content\n")
			}
		}

		if resp.NextPage == 0 {
			break
		} else {
			page = resp.NextPage
		}
	}
	fmt.Printf("Done.\n")
}

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
const GITHUB_TRIAGE_COLUMN = 1527643

func main() {
	GITHUB_IGNORED_REPOS := map[string]struct{}{
		"roadmap":             struct{}{},
		"moby-counter-issues": struct{}{},
	}

	GITHUB_IGNORED_LABELS := map[string]struct{}{
		"hypothesis": struct{}{},
	}

	DESIRED_LABELS := map[string]string{
		"task":               "84b6eb",
		"bug":                "ee0701",
		"debt":               "dbba69",
		"epic":               "7744aa",
		"theme":              "7744aa",
		"support":            "77f252",
		"ready-for-sign-off": "e89db5",
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
		fmt.Printf("Error fetching repository list: %+v\n", err)
		os.Exit(1)
	}

skipRepo:
	for _, repo := range repos {
		fmt.Printf("### EXAMINING REPO: %s\n", *(repo.Name))

		_, ignoredRepo := GITHUB_IGNORED_REPOS[*(repo.Name)]
		if ignoredRepo {
			fmt.Printf("Ignoring that one!\n")
			continue skipRepo
		}

		// Process labels

		labels, _, err := client.Issues.ListLabels(ctx, GITHUB_ORG_NAME, *(repo.Name), &gh.ListOptions{})
		if err != nil {
			fmt.Printf("Error fetching label list: %+v\n", err)
			os.Exit(1)
		}
		missingLabels := map[string]struct{}{}
		for n, _ := range DESIRED_LABELS {
			missingLabels[n] = struct{}{}
		}
		for _, l := range labels {
			desiredColor, known := DESIRED_LABELS[*(l.Name)]
			if known {
				if desiredColor != *(l.Color) {
					fmt.Printf("Label %s has colour %s, should be %s\n", *(l.Name), *(l.Color), desiredColor)
					l.Color = &desiredColor
					_, _, err := client.Issues.EditLabel(ctx, GITHUB_ORG_NAME, *(repo.Name), *(l.Name), l)
					if err != nil {
						fmt.Printf("Error updating label: %s\n", err.Error())
						os.Exit(1)
					}
				} else {
					fmt.Printf("Label %s found with correct colour\n", *(l.Name))
					delete(missingLabels, *(l.Name))
				}
			} else {
				fmt.Printf("Ignoring unknown label %d: %s / %s / %s\n", *(l.ID), *(l.Name), *(l.Color), *(l.NodeID))
			}
		}

		for ml, _ := range missingLabels {
			colour := DESIRED_LABELS[ml]
			fmt.Printf("Label %s / %s was missing\n", ml, colour)
			_, _, err := client.Issues.CreateLabel(ctx, GITHUB_ORG_NAME, *(repo.Name), &gh.Label{
				Name:  &ml,
				Color: &colour,
			})
			if err != nil {
				fmt.Printf("Error creating label: %s\n", err.Error())
				os.Exit(1)
			}
		}

		// Process issues

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

				client.Projects.CreateProjectCard(ctx, GITHUB_TRIAGE_COLUMN, &gh.ProjectCardOptions{
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

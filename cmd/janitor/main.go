package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/dotmesh-io/github-issue-janitor/pkg/utils"
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
		fmt.Printf("Error fetching repository list: %+v\n", err)
		os.Exit(1)
	}

	// repo#number strings -> github IDs for issues that aren't in projects
	issuesNotInProjects := map[string]int64{}

	// repo#number strings for issues that are mentioned in epics
	issuesMentionedInEpics := map[string]struct{}{}

skipRepo:
	for idx, repo := range repos {
		rn := *(repo.Name)
		fmt.Printf("### EXAMINING REPO %d/%d: %s\n", idx+1, len(repos), rn)

		_, ignoredRepo := GITHUB_IGNORED_REPOS[rn]
		if ignoredRepo || *repo.Archived {
			fmt.Printf("Ignoring that one!\n")
			continue skipRepo
		}

		// Process labels

		labels, _, err := client.Issues.ListLabels(ctx, GITHUB_ORG_NAME, rn, &gh.ListOptions{})
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
					_, _, err := client.Issues.EditLabel(ctx, GITHUB_ORG_NAME, rn, *(l.Name), l)
					if err != nil {
						fmt.Printf("Error updating label: %s\n", err.Error())
						os.Exit(1)
					}
				}
				delete(missingLabels, *(l.Name))
			} else {
				fmt.Printf("Ignoring unknown label %s / %s\n", *(l.Name), *(l.Color))
			}
		}

		for ml, _ := range missingLabels {
			colour := DESIRED_LABELS[ml]
			fmt.Printf("Label %s / %s was missing\n", ml, colour)
			_, _, err := client.Issues.CreateLabel(ctx, GITHUB_ORG_NAME, rn, &gh.Label{
				Name:  &ml,
				Color: &colour,
			})
			if err != nil {
				fmt.Printf("Error creating label: %s\n", err.Error())
				os.Exit(1)
			}
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
				fmt.Printf("Error fetching issue list: %+v\n", err)
				os.Exit(1)
			}

			if resp.Rate.Remaining <= 5 {
				delay := time.Until(resp.Rate.Reset.Time)
				fmt.Printf("[rl] %s\n", delay.String())
				time.Sleep(delay)
			}

		skipIssue:
			for _, issue := range issues.Issues {
				issueTag := fmt.Sprintf("%s#%d", rn, *(issue.Number))
				isEpic := false

				for _, label := range issue.Labels {
					_, ignoredLabel := GITHUB_IGNORED_LABELS[*(label.Name)]
					if ignoredLabel {
						fmt.Printf("Ignoring issue %s due to label %s\n", issueTag, *(label.Name))
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
					fmt.Printf("Issue %s is an epic, mentioning these issues: %v!\n", issueTag, mentionedIssues)
					for _, mi := range mentionedIssues {
						issuesMentionedInEpics[mi] = struct{}{}
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

		// Process issues not in projects

		page = 1
		for {
			issues, resp, err := client.Search.Issues(ctx, fmt.Sprintf(
				"no:project is:open repo:dotmesh-io/%s",
				rn,
			), &gh.SearchOptions{
				ListOptions: gh.ListOptions{
					PerPage: 100,
					Page:    page},
			})

			if err != nil {
				fmt.Printf("Error fetching issue list: %+v / %+v\n", err, resp)
				os.Exit(1)
			}

			if resp.Rate.Remaining <= 5 {
				delay := time.Until(resp.Rate.Reset.Time)
				fmt.Printf("[rl] %s\n", delay.String())
				time.Sleep(delay)
			}

		skipIssueNotInProject:
			for _, issue := range issues.Issues {
				issueTag := fmt.Sprintf("%s#%d", rn, *(issue.Number))

				for _, label := range issue.Labels {
					_, ignoredLabel := GITHUB_IGNORED_LABELS[*(label.Name)]
					if ignoredLabel {
						fmt.Printf("Ignoring issue %s due to label %s\n", issueTag, *(label.Name))
						continue skipIssueNotInProject
					}
				}

				fmt.Printf("Issue not in project: %s: %s\n", issueTag, *(issue.Title))

				issuesNotInProjects[issueTag] = *(issue.ID)
			}

			if resp.NextPage == 0 {
				break
			} else {
				page = resp.NextPage
				fmt.Printf("Next page!\n")
			}
		}
	}

	// Now put the issues not in projects into the triage column, unless they're in an epic
	for tag, id := range issuesNotInProjects {
		_, mentionedInEpic := issuesMentionedInEpics[tag]
		if mentionedInEpic {
			fmt.Printf("Issue %s is mentioned in an epic, so isn't lost\n", tag)
		} else {
			fmt.Printf("Issue %s isn't mentioned in an epic or a project, putting it into triage...\n", tag)

			_, resp, err := client.Projects.CreateProjectCard(ctx, GITHUB_TRIAGE_COLUMN, &gh.ProjectCardOptions{
				ContentType: "Issue",
				ContentID:   id,
			})

			if err != nil {
				fmt.Printf("Error putting issue in triage: %+v / %+v\n", err, resp)
				os.Exit(1)
			}

			if resp.Rate.Remaining <= 5 {
				delay := time.Until(resp.Rate.Reset.Time)
				fmt.Printf("[rl] %s\n", delay.String())
				time.Sleep(delay)
			}
		}
	}

	fmt.Printf("Done.\n")
}

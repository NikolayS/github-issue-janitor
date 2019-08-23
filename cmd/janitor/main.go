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
	// Ignore these repos
	GITHUB_IGNORED_REPOS := map[string]struct{}{
		"roadmap":             struct{}{},
		"moby-counter-issues": struct{}{},
	}

	// Ignore issues with these labels
	GITHUB_IGNORED_LABELS := map[string]struct{}{
		"hypothesis": struct{}{},
		"bot":        struct{}{},
	}

	// Labels that should be renamed. Renames happen *before*
	// DESIRED_LABELS / UNDESIRED_LABELS are considered.
	RENAME_THESE_LABELS := map[string]string{
		/* Proposed rename to make it clear you pick exactly one of these (and to make them sort consistently against importance:* and urgency:*

		If we adopt this, update the corresponding names in DESIRED_LABELS.

		"task":  "type:task",
		"bug":   "type:bug",
		"debt":  "type:debt",
		"epic":  "type:epic",
		"theme": "type:theme",
		*/
	}

	// Labels we want in every repo, with their colours
	DESIRED_LABELS := map[string]string{
		// Issue types
		"task":  "84b6eb",
		"bug":   "f03838",
		"debt":  "dbba69",
		"epic":  "7744aa",
		"theme": "7744aa",
		"bot":   "EEF5DB",

		// Flag to mark an issue as coming from the support team, so they
		// can find them and update users
		"support": "77f252",

		// Urgency: How soon do we need it?
		"urgency:high":   "c40000",
		"urgency:medium": "ffff00",
		"urgency:low":    "00ba00",

		// Importance: How much do we need it?
		"importance:high":   "ffaaaa",
		"importance:medium": "ffffaa",
		"importance:low":    "aaffaa",
	}

	// Labels we want to delete if found
	UNDESIRED_LABELS := map[string]struct{}{
		"P0":                 struct{}{},
		"P1":                 struct{}{},
		"P2":                 struct{}{},
		"P3":                 struct{}{},
		"code-review":        struct{}{},
		"ready-for-sign-off": struct{}{},
		"duplicate":          struct{}{},
		"enhancement":        struct{}{},
		"good first issue":   struct{}{},
		"help wanted":        struct{}{},
		"invalid":            struct{}{},
		"question":           struct{}{},
		"wontfix":            struct{}{},
	}

	// Labels that make an issue count as an epic
	EPIC_LABELS := map[string]struct{}{
		"epic":  struct{}{},
		"theme": struct{}{},
	}

	// Set to true to prevent any actual changes happening on Github
	DRY_RUN := false

	if DRY_RUN {
		fmt.Printf("DRY RUN MODE - not actually changing anything!\n")
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

	// Maps we build up as we scan through every issue in every repo:

	// repo#number strings -> github IDs for issues that aren't in projects
	issuesNotInProjects := map[string]int64{}

	// repo#number strings for issues that are mentioned in epics
	issuesMentionedInEpics := map[string]struct{}{}

	// Scan through every repo
skipRepo:
	for idx, repo := range repos {
		rn := *(repo.Name)
		fmt.Printf("### EXAMINING REPO %d/%d: %s\n", idx+1, len(repos), rn)

		_, ignoredRepo := GITHUB_IGNORED_REPOS[rn]
		if ignoredRepo || *repo.Archived {
			fmt.Printf("Ignoring that one!\n")
			continue skipRepo
		}

		// Process labels in this repo

		labels, _, err := client.Issues.ListLabels(ctx, GITHUB_ORG_NAME, rn, &gh.ListOptions{})
		if err != nil {
			fmt.Printf("Error fetching label list: %+v\n", err)
			os.Exit(1)
		}

		// Track what desired labels are missing; start with all of them in the list, then remove ones we find.
		missingLabels := map[string]struct{}{}
		for n, _ := range DESIRED_LABELS {
			missingLabels[n] = struct{}{}
		}

		for _, l := range labels {
			ln := *(l.Name)

			// Check for renames
			newName, renameNeeded := RENAME_THESE_LABELS[ln]
			if renameNeeded {
				fmt.Printf("ACTION: Label %s must be renamed to %s\n", ln, newName)
				l.Name = &newName
				if !DRY_RUN {
					_, _, err := client.Issues.EditLabel(ctx, GITHUB_ORG_NAME, rn, ln, l)
					if err != nil {
						fmt.Printf("Error updating label: %s\n", err.Error())
						os.Exit(1)
					}
				}

				// Update the name in ln, as we still need to process it for
				// colour changes - or being deleted
				ln = newName
			}

			// Check for undesired labels and remove them
			_, undesired := UNDESIRED_LABELS[ln]
			if undesired {
				fmt.Printf("ACTION: Removing undesired label %s / %s\n", ln, *(l.Color))
				if !DRY_RUN {
					_, err = client.Issues.DeleteLabel(ctx, GITHUB_ORG_NAME, rn, ln)
					if err != nil {
						fmt.Printf("Error deleting label: %s\n", err.Error())
						os.Exit(1)
					}
				}
			} else {
				desiredColor, known := DESIRED_LABELS[ln]
				if known {
					// Fix colour of desired labels that exist but have the wrong colour
					if desiredColor != *(l.Color) {
						fmt.Printf("ACTION: Label %s has colour %s, should be %s\n", ln, *(l.Color), desiredColor)
						l.Color = &desiredColor
						if !DRY_RUN {
							_, _, err := client.Issues.EditLabel(ctx, GITHUB_ORG_NAME, rn, ln, l)
							if err != nil {
								fmt.Printf("Error updating label: %s\n", err.Error())
								os.Exit(1)
							}
						}
					}
					// We found this label so it's not missing
					delete(missingLabels, ln)
				} else {
					fmt.Printf("Ignoring unknown label %s / %s\n", ln, *(l.Color))
				}
			}
		}

		// Create desired labels we didn't find
		for ml, _ := range missingLabels {
			colour := DESIRED_LABELS[ml]
			fmt.Printf("ACTION: Label %s / %s was missing\n", ml, colour)
			if !DRY_RUN {
				_, _, err := client.Issues.CreateLabel(ctx, GITHUB_ORG_NAME, rn, &gh.Label{
					Name:  &ml,
					Color: &colour,
				})
				if err != nil {
					fmt.Printf("Error creating label: %s\n", err.Error())
					os.Exit(1)
				}
			}
		}

		// Find all the epics in the repo
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

				// Find the issues mentioned in the epic
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

		// Find issues not in projects

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
	} // End of iteration over all repos

	// Now we have built up two maps: issuesNotInProjects and
	// issuesMentionedIn Epics. Process them to find issues not in a
	// project or epic, so we can put them in the triage column.

	// Now put the issues not in projects into the triage column, unless they're in an epic
	for tag, id := range issuesNotInProjects {
		_, mentionedInEpic := issuesMentionedInEpics[tag]
		if mentionedInEpic {
			fmt.Printf("Issue %s is mentioned in an epic, so isn't lost\n", tag)
		} else {
			fmt.Printf("ACTION: Issue %s isn't mentioned in an epic or a project, putting it into triage...\n", tag)

			if !DRY_RUN {
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
	}

	fmt.Printf("Done.\n")
}

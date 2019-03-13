package utils

import (
	"regexp"
)

func ParseBodyForIssueLinks(body, org, currentRepo string) []string {
	links := []string{}

	var longLink = regexp.MustCompile(`https://github.com/` + org + `/([^/]+)/issues/([0-9]+)`)
	var shortLink = regexp.MustCompile(`#([0-9]+)`)

	longLinks := longLink.FindAllStringSubmatch(body, -1)
	for _, ll := range longLinks {
		repo := ll[1]
		issue := ll[2]
		links = append(links, repo+"#"+issue)
	}

	shortLinks := shortLink.FindAllStringSubmatch(body, -1)
	for _, sl := range shortLinks {
		issue := sl[1]
		links = append(links, currentRepo+"#"+issue)
	}

	return links
}

This repo contains the github issue janitor and, for lack of a better place, a few related utilities for issue management.

# Github Issue Janitor [![Build Status](https://drone.app.cloud.dotscience.net/api/badges/dotmesh-io/github-issue-janitor/status.svg)](https://drone.app.cloud.dotscience.net/dotmesh-io/github-issue-janitor)

This scans our project boards and does various things to keep house in order (read the source if you want to know what, it'll be more up to date than anything we write in here).

You can run it manually if you have a Github API auth token:

```shell
GITHUB_AUTH_TOKEN=... go run cmd/janitor/main.go
```

...its standard output is full of detail on what it's doing. It's
normally run automatically from a Kubernetes cronjob, see
`github-janitor-cronjob.yaml` in the `saas-manifests` repo. That runs
whatever is tagged as latest, so if you run `rebuild.sh` from this
repo, you'll update what we're running on our live system. Be careful!
There's no UNDO.

# Convert Column To Markdown

This tool takes a column of issues in a Github project board, and
generates a reasonably formatted Markdown list of links to those
issues. It doesn't change anything in Github, so it's safe to run it
without knowing what you're doing. You can run it like so:

```shell
GITHUB_AUTH_TOKEN=... go run cmd/convert-column-to-markdown/main.go 'https://github.com/orgs/dotmesh-io/projects/8#column-4716294'
```

The URL is the URL of the column, as found through the web UI.

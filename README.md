# perch

A self-hosted engineering dashboard that surfaces GitHub (and GitHub Enterprise) activity in real time: open PRs, workflow runs, branch build matrices, and assigned issues — all pushed live to your browser via SSE.

## Why "perch"?

A **perch** is the elevated spot a bird returns to in order to watch the world below — a branch, a ledge, a high post. *perch* is your perch above your repos: you sit in one place and watch PRs and Actions runs flow past underneath. Quiet, observant, bird-of-prey energy. Pairs nicely with GitHub's Octocat (also bird-ish).

## Quick start (Docker)

```bash
cp config.example.yaml config.yaml
# edit config.yaml — replace instance URLs, set env var names for tokens
export GITHUB_TOKEN=ghp_...
export GHE_TOKEN=ghp_...
docker compose up
```

Open http://localhost:8080.

`config.yaml` is gitignored — your instances, repos, and token env-var
references stay local. Commit changes via `config.example.yaml` only.

### Local overrides

Need to inject extra environment variables (e.g. additional GHE instance
tokens) without editing the committed `docker-compose.yml`?

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
# add any extra ${VAR}: ${VAR:-} passthroughs
```

`docker-compose.override.yml` is gitignored and auto-merged by Compose.

## Local development

Requires Go 1.23+, Node 18+ (for Tailwind CLI).

```bash
cp config.example.yaml config.yaml
# edit config.yaml
export GITHUB_TOKEN=ghp_...
make dev          # starts Tailwind watcher + go run in parallel
```

Other targets:

```
make build    # minify CSS + compile binary → dist/perch
make test     # go test ./...
make tidy     # go mod tidy
make docker   # docker build -t perch .
```

## Supported widget types

| type | description |
|---|---|
| `pr_list` | Open pull requests, filterable by author / state / label |
| `workflow_status` | Latest N runs of a workflow file |
| `branch_matrix` | Build-status grid across repos × branches |
| `my_issues` | Issues assigned to or created by the configured author |
| `repo_health` | Per-repo summary: open PR count, failing checks, stale-branch days |

## Architecture

```
Browser
  │  SSE (text/event-stream)
  ▼
Fiber HTTP server  ─── embedded html/template (layout + widget partials)
  │
  ├── Widget scheduler (per-widget poll goroutine)
  │     └── GitHub REST / GraphQL client (ETag caching, rate-limit aware)
  │           └── widget_snapshot table (SQLite / GORM)
  │
  └── Broadcaster (fan-out widget HTML fragments → SSE clients)
```

Config is a single YAML file; tokens come from environment variables (`${GITHUB_TOKEN}`). SQLite lives in the `/data` volume (Docker) or `./data/` locally.

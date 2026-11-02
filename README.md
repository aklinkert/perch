# perch

[![test](https://github.com/aklinkert/perch/actions/workflows/test.yml/badge.svg)](https://github.com/aklinkert/perch/actions/workflows/test.yml)

A self-hosted, real-time engineering dashboard for one developer juggling
**multiple GitHub instances** (e.g. github.com + a corporate GitHub
Enterprise). Surfaces open PRs, workflow runs, branch build matrices, and
issues live in your browser via Server-Sent Events.

Single binary, single SQLite file, no JavaScript build chain on the server,
no external dependencies once running. Drop into a `docker compose up` and
forget.

## Why "perch"?

A **perch** is the elevated spot a bird returns to in order to watch the
world below — a branch, a ledge, a high post. *perch* is your perch above
your repos: you sit in one place and watch PRs and Actions runs flow past
underneath. Quiet, observant, bird-of-prey energy. Pairs nicely with
GitHub's Octocat (also bird-ish).

## Highlights

- **Multi-instance** — one dashboard for `api.github.com` and any number of
  GitHub Enterprise hosts side-by-side. Each widget binds to a named
  instance.
- **Live updates over SSE** — widgets push HTML fragments to the browser
  whenever a refresh tick produces new data. No polling from the client.
- **ETag-aware caching + rate-limit accounting** built into the GitHub
  client layer; conditional requests don't burn quota.
- **Persistent drag-and-drop layout** — rearrange widget boxes across
  columns; the order survives page reload + container restart.
- **Composable widget types** — PR lists, workflow runs, branch×repo build
  matrix, assigned issues, repo health.
- **Single SQLite file** for ETag cache, widget snapshots, and saved
  layout. No Postgres / Redis to operate.
- **Server-side rendering** — `html/template` + HTMX 2 + Tailwind. No
  React, no client bundle, fast first paint.
- **Public-repo friendly** — your real `config.yaml` stays gitignored;
  ship clean defaults via `config.example.yaml`.

## Quick start (Docker)

```bash
git clone https://github.com/aklinkert/perch.git
cd perch
cp config.example.yaml config.yaml
$EDITOR config.yaml         # set instance URLs, repos, widgets

export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
export GHE_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx   # optional second instance

docker compose up -d --build
open http://localhost:8080
```

`config.yaml` is gitignored — your instances, repos, and token env-var
references stay local. Commit changes via `config.example.yaml` only.

### Prebuilt image

Every release publishes a multi-arch (`linux/amd64`, `linux/arm64`) image to
GitHub Container Registry, tagged with the release version and `latest`:

```bash
docker pull ghcr.io/aklinkert/perch:latest
```

To run it instead of building locally, set the image in
`docker-compose.override.yml` and start without `--build`:

```yaml
services:
  perch:
    image: ghcr.io/aklinkert/perch:latest
    pull_policy: always
```

## Configuration

Single YAML file (`config.yaml` by default; override with `--config` or
`DASHBOARD_CONFIG` env var).

### Top-level shape

```yaml
instances: [ ... ]    # one entry per GitHub host
defaults:
  poll_interval: 5m   # fallback widget refresh interval
db_path: /data/perch.db
listen: :8080
widgets: [ ... ]      # widget definitions
layout:
  columns: 3
  order: [ ... ]      # widget IDs in render order
```

### Instances

Each instance has a name (referenced from widgets), a REST API URL, an
optional GraphQL URL, and a token. Tokens are read from environment
variables via `${VAR}` syntax — never put real tokens in this file.

```yaml
instances:
  - name: github
    api_url: https://api.github.com
    graphql_url: https://api.github.com/graphql
    token: ${GITHUB_TOKEN}

  - name: corp
    api_url: https://github.example.com/api/v3
    graphql_url: https://github.example.com/api/graphql
    token: ${CORP_GHE_TOKEN}
```

### Widget types

| type              | what it shows                                                       |
|-------------------|---------------------------------------------------------------------|
| `pr_list`         | Open pull requests, filterable by author / state / label / repos    |
| `my_issues`       | Issues you're assigned to or authored, scoped to a set of repos     |
| `workflow_status` | Latest N runs of a single workflow file in a single repo            |
| `branch_matrix`   | Build-status grid across `repos[] × branches[]`                     |
| `repo_health`     | Per-repo summary: open PRs, failing checks, stale branches          |

### Common widget fields

```yaml
- id: my-prs              # required, unique
  type: pr_list           # required, must match a registered widget type
  title: My Pull Requests
  instance: github        # required, must match an entry in `instances`
  refresh: 2m             # optional; falls back to defaults.poll_interval
  last_n: 30              # max items to fetch (pr_list / my_issues / workflow_status)
  filters:                # subset depends on widget type
    author: "@me"
    state: open
    label: "needs-review"
    target_branch: main
    repos:                # scope to specific repos
      - org/service-a
      - org/service-b
  repos: [ ... ]          # for branch_matrix / repo_health
  branches: [ ... ]       # for branch_matrix
  repo: org/repo          # for workflow_status
  workflow: deploy.yml    # for workflow_status
```

`filters.repos` and the top-level `repos` field both work for the widgets
that take a list — pick whichever reads cleaner.

### Layout

```yaml
layout:
  columns: 3              # number of columns at md+ breakpoints
  order:                  # widget IDs, top-to-bottom in declared order
    - tta-my-issues
    - tta-my-prs
    - gh-team-prs
    - ...
```

The renderer takes that linear order and bin-packs it into `columns`
columns by rendered HTML size, so heavy widgets don't stack on top of
each other in the same column. Once you drag a box, the saved positions
override the auto-packing — see below.

### Drag-and-drop persistence

Each box has a small `⋮⋮ drag` handle in the top-left corner. Drag a box
within a column to reorder, or across columns to move it. The layout is
posted to `POST /layout` on drop and stored in SQLite (`layout_state`
table). Reload preserves it; container restart preserves it.

To reset to the auto-packed default:

```bash
curl -X POST -H 'Content-Type: application/json' \
  -d '{"cols":[]}' http://localhost:8080/layout
```

If you add a new widget to `config.yaml` after saving a layout, it gets
appended to whichever column is shortest at render time.

## Tokens

You'll need one Personal Access Token per instance. Required scopes:

- `repo`        — read PRs / issues / commit statuses on private repos
- `read:org`    — see org-scoped PRs and team membership
- `workflow`    — read Actions runs and workflow definitions

Classic PATs with lifetimes >90 days are blocked by some org policies (you'll
see a warning in the logs); generate short-lived tokens or use fine-grained
PATs scoped per-org.

## Local overrides (Docker)

`docker-compose.yml` is committed and intentionally generic — it only knows
about `GITHUB_TOKEN` and `GHE_TOKEN`. To pass extra environment variables
into the container without editing the tracked file:

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
$EDITOR docker-compose.override.yml
```

Compose auto-merges `docker-compose.override.yml` (gitignored) on every
`up`. Use it for tokens belonging to additional GHE instances, alternative
listen ports, custom volume bindings, etc.

## Local development

Requires Go 1.27+, Node 18+ (for the Tailwind CLI), and SQLite headers
(macOS: built-in; Linux: `apt install libsqlite3-dev`).

```bash
cp config.example.yaml config.yaml
$EDITOR config.yaml
export GITHUB_TOKEN=ghp_...
mkdir -p data        # SQLite path used by config.example.yaml's local db_path
make dev             # starts Tailwind watcher + `go run` in parallel
```

Other Make targets:

```
make build    # minify CSS + compile binary → dist/perch
make test     # go test ./...
make tidy     # go mod tidy
make docker   # docker build -t perch .
```

If you only edit Go code, you can `docker compose up -d --build` instead;
the multi-stage Dockerfile builds Tailwind and the binary together.

## Architecture

```
Browser
  │  SSE (text/event-stream)            HTMX swaps fragments by widget ID
  ▼
Fiber HTTP server ── embedded html/template (layout + widgets/*.html)
  │                  embedded /static/   (htmx, sortable.js, app.css)
  │
  ├── Widget scheduler         (one goroutine per widget, ticks on Refresh interval)
  │     └── widget.Refresh()
  │           └── GitHub clients (REST v3 + GraphQL v4, ETag-aware transport, rate-limit accounting)
  │                 └── cache.Store    (SQLite via gorm: api_caches, widget_snapshots, layout_state)
  │
  └── Broadcaster              (fan-out rendered HTML fragments → all SSE subscribers)
```

Each widget owns:

- a snapshot of its current view data, persisted to `widget_snapshot`
- a render method that emits an HTML fragment using its own template
- a refresh method that fetches fresh data and asks the broadcaster to
  push the new fragment to connected SSE clients

New widget types register themselves in `init()` via
`widget.MustRegister`; `cmd/perch/main.go` blank-imports each type
package so the registry is populated before config validation.

### Directory layout

```
cmd/perch/             entrypoint (urfave/cli/v3)
internal/config/       YAML loader + validator
internal/cache/        gorm models, ETag store, snapshot store, layout store
internal/github/       multi-instance clients, ETag transport, rate limiter
internal/widget/       Widget interface, registry, scheduler, broadcaster
internal/widget/<type> per-widget implementations (pr_list, my_issues, …)
internal/views/        embedded html/template files (layout + widgets/*.html)
internal/server/       Fiber routes + SSE handler + column-pack logic
web/                   tailwind input + built static assets (embedded)
```

### Stack

- Go 1.27, [Fiber v2](https://github.com/gofiber/fiber)
- `html/template` (stdlib) + `embed.FS` for SSR
- HTMX 2.x + [SortableJS](https://github.com/SortableJS/Sortable) for
  drag-and-drop, Tailwind CSS for styling
- SQLite via `gorm.io/gorm`
- DI via `samber/do/v2`, functional utils via `samber/lo`
- Validation via `go-playground/validator/v10`
- GitHub clients: `shurcooL/githubv4` (GraphQL) + `google/go-github/v66`
  (REST)

## Operations

### Logs

Perch writes structured logs (`slog`, text handler) to stderr. Useful
filters:

```bash
docker compose logs -f perch | grep -iE 'error|widget refresh failed'
```

### Persistent state

Everything that needs to survive a restart lives in the SQLite file at
`db_path`:

- `api_caches`       — ETag + body per `(instance, key)`
- `widget_snapshots` — last-rendered view-data per widget ID
- `layout_state`     — saved drag-and-drop column ordering

Back it up with a plain file copy.

### Updating the app

```bash
git pull
docker compose up -d --build
```

Schemas are auto-migrated on startup via `gorm.AutoMigrate`.

## Troubleshooting

| Symptom                                              | Likely cause                                                                |
|------------------------------------------------------|------------------------------------------------------------------------------|
| `connecting…` never flips to `connected` (top right) | Reverse proxy buffering SSE; set `proxy_buffering off;` (nginx) or equivalent |
| `unknown widget type: <name>`                        | New widget package not blank-imported in `cmd/perch/main.go`                |
| `403 The 'X' organization forbids … PAT`             | Org policy bans long-lived classic PATs; rotate to ≤90-day or fine-grained  |
| Widgets keep showing `loading …`                     | `widget refresh failed` errors in logs — usually 4xx/5xx from the API host  |
| Empty trailing column on standard screens            | Make sure you're on a build that includes server-side column packing        |

## Contributing

- Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`,
  `ci:`, `test:`)
- Don't mock the DB — integration tests use real SQLite
- New widget type? Add `internal/widget/<type>/`, register via
  `widget.MustRegister`, and blank-import the package from
  `cmd/perch/main.go`.

## License

MIT — see `LICENSE`.

# work-dashboard — Claude Instructions

## Implementation workflow

**Use an agent team for non-trivial implementation work.** When a task spans
multiple files / layers (server + widgets + infra), spawn parallel sub-agents
(`backend-engineer`, `frontend-engineer`, `devops`, `e2e-tester`,
`code-reviewer`) instead of doing everything in the main loop. Coordinate by
splitting the work into file-disjoint slices so agents can run concurrently
without conflicting.

## Stack

- Go (fiber/v2) + `html/template` (stdlib) + `embed.FS` for SSR
- HTMX 2.x + Tailwind (CLI build) for the frontend
- SQLite via `gorm.io/gorm` (single-node, single-user)
- DI via `samber/do/v2`, functional utils via `samber/lo`
- Validation via `go-playground/validator/v10`
- Multi-instance GitHub clients (github.com + GHE) — `shurcooL/githubv4` + `google/go-github/v66`

## Directory layout

```
cmd/dashboard/         entrypoint (urfave/cli/v3)
internal/config/       YAML loader + validator
internal/cache/        gorm models, ETag store
internal/github/       multi-instance clients, ETag transport, rate limiter
internal/widget/       interface, registry, scheduler, broadcaster
internal/widget/<type> per-widget implementations
internal/views/        embedded html/template files (layout + widgets/*.html)
internal/server/       Fiber routes + SSE
web/                   tailwind input + built static assets (embedded)
```

## Conventions

- Conventional Commits (feat:, fix:, refactor:, docs:, chore:, ci:, test:)
- No mocked DB in integration tests — use real SQLite
- Each widget: GraphQL/REST fetch → store snapshot in `widget_snapshot` →
  publish HTML fragment via Broadcaster → SSE fan-out to clients
- New widget types register themselves in `internal/widget/registry.go`

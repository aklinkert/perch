---
name: agent-team-workflow
description: For multi-file/layer implementation tasks in work-dashboard, spawn parallel sub-agents instead of building serially in the main loop
type: feedback
---

For non-trivial implementation work in this repo, use an agent team — spawn
parallel sub-agents (`backend-engineer`, `frontend-engineer`, `devops`,
`e2e-tester`, `code-reviewer`) split along file-disjoint slices.

**Why:** User explicitly requested this during the initial dashboard build
("Use an agent team for the implementation. Remember this at CLAUDE.md").
Parallelizes work and matches the project's CLAUDE.md.

**How to apply:** When a task touches >2 layers (e.g. server + widget impls +
docker), divide work so each agent owns disjoint files, dispatch in a single
message with multiple Agent tool calls, then reconcile (go mod tidy, build,
lint) in the main loop.

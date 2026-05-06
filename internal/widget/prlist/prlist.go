package prlist

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/shurcooL/githubv4"

	"github.com/aklinkert/perch/internal/cache"
	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/views"
	"github.com/aklinkert/perch/internal/widget"
)

const widgetType = "pr_list"

func init() {
	widget.MustRegister(func(r *widget.Registry) {
		r.Register(widgetType, New)
	})
}

type item struct {
	Repo    string
	Number  int
	Title   string
	Author  string
	URL     string
	Updated string
	State   string
}

type viewData struct {
	Title   string
	Updated string
	Items   []item
	Error   string
}

type Widget struct {
	cfg  config.Widget
	deps widget.Deps
	tmpl *template.Template
	mu   sync.RWMutex
	data viewData
}

func New(cfg config.Widget, deps widget.Deps) (widget.Widget, error) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/pr_list.html")
	if err != nil {
		return nil, fmt.Errorf("parse pr_list template: %w", err)
	}
	w := &Widget{
		cfg:  cfg,
		deps: deps,
		tmpl: tmpl,
		data: viewData{Title: cfg.Title},
	}
	if deps.Store != nil {
		if snap, err := deps.Store.GetSnapshot(cfg.ID); err == nil && snap != nil {
			var d viewData
			if err := json.Unmarshal(snap.Payload, &d); err == nil {
				w.data = d
			}
		}
	}
	return w, nil
}

func (w *Widget) ID() string              { return w.cfg.ID }
func (w *Widget) Type() string            { return w.cfg.Type }
func (w *Widget) Title() string           { return w.cfg.Title }
func (w *Widget) Interval() time.Duration { return w.cfg.Refresh }

func (w *Widget) Render(out io.Writer) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tmpl.ExecuteTemplate(out, "pr_list.html", w.data)
}

// buildSearchQuery constructs a GitHub search query from cfg.Filters.
func buildSearchQuery(cfg config.Widget) string {
	parts := []string{"is:pr"}
	state := strings.ToLower(cfg.Filters.State)
	switch state {
	case "open", "closed", "merged":
		parts = append(parts, "is:"+state)
	case "":
		parts = append(parts, "is:open")
	}
	if a := strings.TrimSpace(cfg.Filters.Author); a != "" {
		parts = append(parts, "author:"+a)
	}
	if l := strings.TrimSpace(cfg.Filters.Label); l != "" {
		parts = append(parts, fmt.Sprintf("label:%q", l))
	}
	if tb := strings.TrimSpace(cfg.Filters.TargetBranch); tb != "" {
		parts = append(parts, "base:"+tb)
	}
	for _, r := range cfg.Filters.Repos {
		if r != "" {
			parts = append(parts, "repo:"+r)
		}
	}
	parts = append(parts, "sort:updated-desc")
	return strings.Join(parts, " ")
}

type searchQuery struct {
	Search struct {
		Nodes []struct {
			PullRequest struct {
				Number     int
				Title      string
				URL        string
				State      string
				UpdatedAt  time.Time
				Author     struct{ Login string }
				Repository struct{ NameWithOwner string }
			} `graphql:"... on PullRequest"`
		}
	} `graphql:"search(query: $q, type: ISSUE, first: $n)"`
}

func (w *Widget) Refresh(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	w.mu.Lock()
	w.data.Title = w.cfg.Title
	w.mu.Unlock()

	c := w.deps.Clients.Get(w.cfg.Instance)
	if c == nil {
		return w.fail(fmt.Errorf("instance %q not found", w.cfg.Instance), now)
	}

	q := buildSearchQuery(w.cfg)
	vars := map[string]any{
		"q": githubv4.String(q),
		"n": githubv4.Int(30),
	}
	var resp searchQuery
	if err := c.GQL.Query(ctx, &resp, vars); err != nil {
		return w.fail(fmt.Errorf("search prs: %w", err), now)
	}

	items := make([]item, 0, len(resp.Search.Nodes))
	for _, n := range resp.Search.Nodes {
		pr := n.PullRequest
		if pr.URL == "" {
			continue
		}
		items = append(items, item{
			Repo:    pr.Repository.NameWithOwner,
			Number:  pr.Number,
			Title:   pr.Title,
			Author:  pr.Author.Login,
			URL:     pr.URL,
			Updated: pr.UpdatedAt.UTC().Format(time.RFC3339),
			State:   pr.State,
		})
	}

	w.mu.Lock()
	w.data.Items = items
	w.data.Updated = now
	w.data.Error = ""
	snap := w.data
	w.mu.Unlock()

	w.persist(snap)
	return nil
}

func (w *Widget) fail(err error, now string) error {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	w.mu.Lock()
	w.data.Title = w.cfg.Title
	w.data.Error = msg
	w.data.Updated = now
	snap := w.data
	w.mu.Unlock()
	w.persist(snap)
	return err
}

func (w *Widget) persist(d viewData) {
	if w.deps.Store == nil {
		return
	}
	payload, err := json.Marshal(d)
	if err != nil {
		return
	}
	_ = w.deps.Store.PutSnapshot(&cache.WidgetSnapshot{
		WidgetID:  w.cfg.ID,
		Payload:   payload,
		UpdatedAt: time.Now(),
	})
}

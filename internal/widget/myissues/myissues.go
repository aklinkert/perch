package myissues

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"github.com/shurcooL/githubv4"

	"github.com/aklinkert/work-dashboard/internal/cache"
	"github.com/aklinkert/work-dashboard/internal/config"
	"github.com/aklinkert/work-dashboard/internal/views"
	"github.com/aklinkert/work-dashboard/internal/widget"
)

const widgetType = "my_issues"

func init() {
	widget.MustRegister(func(r *widget.Registry) {
		r.Register(widgetType, New)
	})
}

type item struct {
	Repo    string
	Number  int
	Title   string
	URL     string
	State   string
	Updated string
	Labels  []string
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
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/my_issues.html")
	if err != nil {
		return nil, fmt.Errorf("parse my_issues template: %w", err)
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
	return w.tmpl.ExecuteTemplate(out, "my_issues.html", w.data)
}

func buildSearchQuery(cfg config.Widget) string {
	parts := []string{"is:issue"}
	state := strings.ToLower(cfg.Filters.State)
	switch state {
	case "open", "closed":
		parts = append(parts, "is:"+state)
	case "":
		parts = append(parts, "is:open")
	}
	author := strings.TrimSpace(cfg.Filters.Author)
	if author == "" {
		author = "@me"
	}
	// "involved" matches issues where the user is assignee or author.
	parts = append(parts, "involves:"+author)
	if l := strings.TrimSpace(cfg.Filters.Label); l != "" {
		parts = append(parts, fmt.Sprintf("label:%q", l))
	}
	for _, r := range cfg.Filters.Repos {
		if r != "" {
			parts = append(parts, "repo:"+r)
		}
	}
	for _, r := range cfg.Repos {
		if r != "" {
			parts = append(parts, "repo:"+r)
		}
	}
	parts = append(parts, "sort:updated-desc")
	return strings.Join(parts, " ")
}

type issueSearchQuery struct {
	Search struct {
		Nodes []struct {
			Issue struct {
				Number     int
				Title      string
				URL        string
				State      string
				UpdatedAt  time.Time
				Repository struct{ NameWithOwner string }
				Labels     struct {
					Nodes []struct{ Name string }
				} `graphql:"labels(first: 10)"`
			} `graphql:"... on Issue"`
		}
	} `graphql:"search(query: $q, type: ISSUE, first: $n)"`
}

func (w *Widget) Refresh(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	c := w.deps.Clients.Get(w.cfg.Instance)
	if c == nil {
		return w.fail(fmt.Errorf("instance %q not found", w.cfg.Instance), now)
	}
	q := buildSearchQuery(w.cfg)
	vars := map[string]any{
		"q": githubv4.String(q),
		"n": githubv4.Int(30),
	}
	var resp issueSearchQuery
	if err := c.GQL.Query(ctx, &resp, vars); err != nil {
		return w.fail(fmt.Errorf("search issues: %w", err), now)
	}
	items := make([]item, 0, len(resp.Search.Nodes))
	for _, n := range resp.Search.Nodes {
		is := n.Issue
		if is.URL == "" {
			continue
		}
		labels := lo.Map(is.Labels.Nodes, func(l struct{ Name string }, _ int) string { return l.Name })
		items = append(items, item{
			Repo:    is.Repository.NameWithOwner,
			Number:  is.Number,
			Title:   is.Title,
			URL:     is.URL,
			State:   is.State,
			Updated: is.UpdatedAt.UTC().Format(time.RFC3339),
			Labels:  labels,
		})
	}

	w.mu.Lock()
	w.data.Title = w.cfg.Title
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

package repohealth

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

	"github.com/aklinkert/work-dashboard/internal/cache"
	"github.com/aklinkert/work-dashboard/internal/config"
	"github.com/aklinkert/work-dashboard/internal/views"
	"github.com/aklinkert/work-dashboard/internal/widget"
)

const widgetType = "repo_health"

func init() {
	widget.MustRegister(func(r *widget.Registry) {
		r.Register(widgetType, New)
	})
}

type item struct {
	Repo          string
	OpenPRs       int
	FailingChecks int
	StaleDays     int
	URL           string
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
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/repo_health.html")
	if err != nil {
		return nil, fmt.Errorf("parse repo_health template: %w", err)
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
	return w.tmpl.ExecuteTemplate(out, "repo_health.html", w.data)
}

// repoQuery is one repo's slice of the batched GraphQL query. We assemble the
// final query manually because alias names need to be unique per repo.
type repoQuery struct {
	URL       githubv4.URI
	PullRequests struct {
		TotalCount int
	} `graphql:"pullRequests(states: OPEN)"`
	DefaultBranchRef *struct {
		Target struct {
			Commit struct {
				CommittedDate time.Time
				StatusCheckRollup *struct {
					State string
				}
			} `graphql:"... on Commit"`
		}
	}
}

func (w *Widget) Refresh(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	c := w.deps.Clients.Get(w.cfg.Instance)
	if c == nil {
		return w.fail(fmt.Errorf("instance %q not found", w.cfg.Instance), now)
	}
	if len(w.cfg.Repos) == 0 {
		return w.fail(fmt.Errorf("no repos configured"), now)
	}

	// Build a dynamic struct with one alias per repo. Since Go is statically
	// typed and shurcool/githubv4 needs a struct, we run one query per repo
	// to keep things simple. Five small queries is fine for this widget.
	items := make([]item, 0, len(w.cfg.Repos))
	var firstErr error
	for _, fullRepo := range w.cfg.Repos {
		owner, repo, ok := splitRepo(fullRepo)
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid repo %q", fullRepo)
			}
			continue
		}
		var q struct {
			Repository repoQuery `graphql:"repository(owner: $owner, name: $name)"`
		}
		vars := map[string]any{
			"owner": githubv4.String(owner),
			"name":  githubv4.String(repo),
		}
		if err := c.GQL.Query(ctx, &q, vars); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", fullRepo, err)
			}
			continue
		}
		it := item{
			Repo:    fullRepo,
			OpenPRs: q.Repository.PullRequests.TotalCount,
			URL:     q.Repository.URL.String(),
		}
		if br := q.Repository.DefaultBranchRef; br != nil {
			cd := br.Target.Commit.CommittedDate
			if !cd.IsZero() {
				it.StaleDays = int(time.Since(cd).Hours() / 24)
			}
			if rollup := br.Target.Commit.StatusCheckRollup; rollup != nil && rollup.State == "FAILURE" {
				it.FailingChecks = 1
			}
		}
		items = append(items, it)
	}

	if firstErr != nil && len(items) == 0 {
		return w.fail(firstErr, now)
	}

	w.mu.Lock()
	w.data.Title = w.cfg.Title
	w.data.Items = items
	w.data.Updated = now
	if firstErr != nil {
		msg := firstErr.Error()
		if len(msg) > 200 {
			msg = msg[:200]
		}
		w.data.Error = msg
	} else {
		w.data.Error = ""
	}
	snap := w.data
	w.mu.Unlock()
	w.persist(snap)
	if firstErr != nil {
		return firstErr
	}
	return nil
}

func splitRepo(s string) (owner, repo string, ok bool) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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

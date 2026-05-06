package branchmatrix

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"
	"time"

	gogithub "github.com/google/go-github/v66/github"

	"github.com/aklinkert/work-dashboard/internal/cache"
	"github.com/aklinkert/work-dashboard/internal/config"
	"github.com/aklinkert/work-dashboard/internal/views"
	"github.com/aklinkert/work-dashboard/internal/widget"
)

const widgetType = "branch_matrix"

func init() {
	widget.MustRegister(func(r *widget.Registry) {
		r.Register(widgetType, New)
	})
}

type Cell struct {
	Conclusion string
	URL        string
}

type viewData struct {
	Title    string
	Updated  string
	Repos    []string
	Branches []string
	Cells    map[string]map[string]Cell
	Error    string
}

type Widget struct {
	cfg  config.Widget
	deps widget.Deps
	tmpl *template.Template
	mu   sync.RWMutex
	data viewData
}

func New(cfg config.Widget, deps widget.Deps) (widget.Widget, error) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/branch_matrix.html")
	if err != nil {
		return nil, fmt.Errorf("parse branch_matrix template: %w", err)
	}
	w := &Widget{
		cfg:  cfg,
		deps: deps,
		tmpl: tmpl,
		data: viewData{
			Title:    cfg.Title,
			Repos:    append([]string(nil), cfg.Repos...),
			Branches: append([]string(nil), cfg.Branches...),
			Cells:    map[string]map[string]Cell{},
		},
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
	return w.tmpl.ExecuteTemplate(out, "branch_matrix.html", w.data)
}

func splitRepo(s string) (owner, repo string, ok bool) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (w *Widget) Refresh(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339)
	c := w.deps.Clients.Get(w.cfg.Instance)
	if c == nil {
		return w.fail(fmt.Errorf("instance %q not found", w.cfg.Instance), now)
	}
	cells := map[string]map[string]Cell{}
	var firstErr error
	for _, fullRepo := range w.cfg.Repos {
		owner, repo, ok := splitRepo(fullRepo)
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("invalid repo %q", fullRepo)
			}
			continue
		}
		row := map[string]Cell{}
		for _, branch := range w.cfg.Branches {
			opts := &gogithub.ListWorkflowRunsOptions{
				Branch:      branch,
				ListOptions: gogithub.ListOptions{PerPage: 1},
			}
			runs, _, err := c.REST.Actions.ListRepositoryWorkflowRuns(ctx, owner, repo, opts)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s@%s: %w", fullRepo, branch, err)
				}
				row[branch] = Cell{}
				continue
			}
			if len(runs.WorkflowRuns) == 0 {
				row[branch] = Cell{}
				continue
			}
			r := runs.WorkflowRuns[0]
			row[branch] = Cell{
				Conclusion: stringDeref(r.Conclusion),
				URL:        stringDeref(r.HTMLURL),
			}
		}
		cells[fullRepo] = row
	}

	if firstErr != nil {
		// Keep partial data and surface the error message.
		return w.failPartial(firstErr, now, cells)
	}

	w.mu.Lock()
	w.data.Title = w.cfg.Title
	w.data.Repos = append([]string(nil), w.cfg.Repos...)
	w.data.Branches = append([]string(nil), w.cfg.Branches...)
	w.data.Cells = cells
	w.data.Updated = now
	w.data.Error = ""
	snap := w.data
	w.mu.Unlock()
	w.persist(snap)
	return nil
}

func stringDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

func (w *Widget) failPartial(err error, now string, cells map[string]map[string]Cell) error {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	w.mu.Lock()
	w.data.Title = w.cfg.Title
	w.data.Repos = append([]string(nil), w.cfg.Repos...)
	w.data.Branches = append([]string(nil), w.cfg.Branches...)
	if len(cells) > 0 {
		w.data.Cells = cells
	}
	w.data.Updated = now
	w.data.Error = msg
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

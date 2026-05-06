package workflowstatus

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

	"github.com/aklinkert/perch/internal/cache"
	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/views"
	"github.com/aklinkert/perch/internal/widget"
)

const widgetType = "workflow_status"

func init() {
	widget.MustRegister(func(r *widget.Registry) {
		r.Register(widgetType, New)
	})
}

type item struct {
	Conclusion string
	Status     string
	URL        string
	RunNumber  int
	CreatedAt  string
	HeadBranch string
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
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/workflow_status.html")
	if err != nil {
		return nil, fmt.Errorf("parse workflow_status template: %w", err)
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
	return w.tmpl.ExecuteTemplate(out, "workflow_status.html", w.data)
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
	owner, repo, ok := splitRepo(w.cfg.Repo)
	if !ok {
		return w.fail(fmt.Errorf("invalid repo %q (want owner/name)", w.cfg.Repo), now)
	}
	if w.cfg.Workflow == "" {
		return w.fail(fmt.Errorf("workflow file required"), now)
	}
	n := w.cfg.LastN
	if n <= 0 {
		n = 10
	}
	opts := &gogithub.ListWorkflowRunsOptions{
		ListOptions: gogithub.ListOptions{PerPage: n},
	}
	if w.cfg.Branch != "" {
		opts.Branch = w.cfg.Branch
	}
	runs, _, err := c.REST.Actions.ListWorkflowRunsByFileName(ctx, owner, repo, w.cfg.Workflow, opts)
	if err != nil {
		return w.fail(fmt.Errorf("list workflow runs: %w", err), now)
	}
	items := make([]item, 0, len(runs.WorkflowRuns))
	for i, r := range runs.WorkflowRuns {
		if i >= n {
			break
		}
		items = append(items, item{
			Conclusion: stringDeref(r.Conclusion),
			Status:     stringDeref(r.Status),
			URL:        stringDeref(r.HTMLURL),
			RunNumber:  intDeref(r.RunNumber),
			CreatedAt:  formatTime(r.CreatedAt),
			HeadBranch: stringDeref(r.HeadBranch),
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

func stringDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intDeref(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func formatTime(t *gogithub.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
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

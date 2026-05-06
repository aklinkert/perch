package repohealth

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/aklinkert/perch/internal/views"
)

func TestRender(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/repo_health.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	w := &Widget{
		tmpl: tmpl,
		data: viewData{
			Title:   "Repos",
			Updated: "now",
			Items: []item{
				{Repo: "octo/cat", OpenPRs: 3, FailingChecks: 0, StaleDays: 5, URL: "https://x/octo/cat"},
				{Repo: "octo/dog", OpenPRs: 1, FailingChecks: 1, StaleDays: 60, URL: "https://x/octo/dog"},
			},
		},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Repos", "octo/cat", "octo/dog", "3 PRs", "1 failing", "60d stale", "bg-yellow-700", "bg-red-600"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
}

func TestSplitRepo(t *testing.T) {
	o, r, ok := splitRepo("foo/bar")
	if !ok || o != "foo" || r != "bar" {
		t.Errorf("got %q %q %v", o, r, ok)
	}
}

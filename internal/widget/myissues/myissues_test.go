package myissues

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/aklinkert/work-dashboard/internal/config"
	"github.com/aklinkert/work-dashboard/internal/views"
)

func TestRender(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/my_issues.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	w := &Widget{
		tmpl: tmpl,
		data: viewData{
			Title:   "Issues",
			Updated: "now",
			Items: []item{{
				Repo:    "octo/cat",
				Number:  42,
				Title:   "fix bug",
				URL:     "https://example/issue/42",
				State:   "OPEN",
				Updated: "2025-01-01",
				Labels:  []string{"bug", "p0"},
			}},
		},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Issues", "octo/cat", "#42", "fix bug", "bug", "p0"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
}

func TestBuildSearchQuery(t *testing.T) {
	q := buildSearchQuery(config.Widget{
		Filters: config.Filters{Author: "alice", Label: "bug"},
		Repos:   []string{"a/b"},
	})
	for _, want := range []string{"is:issue", "is:open", "involves:alice", "label:\"bug\"", "repo:a/b"} {
		if !strings.Contains(q, want) {
			t.Errorf("missing %q in %s", want, q)
		}
	}
}

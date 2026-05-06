package prlist

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/views"
)

func TestRender(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/pr_list.html")
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	w := &Widget{
		cfg:  config.Widget{ID: "x", Type: widgetType, Title: "My PRs"},
		tmpl: tmpl,
		data: viewData{
			Title:   "My PRs",
			Updated: "2025-01-01T00:00:00Z",
			Items: []item{{
				Repo:   "octo/cat",
				Number: 7,
				Title:  "Add feature",
				Author: "alice",
				URL:    "https://example/pr/7",
				State:  "OPEN",
			}},
		},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"My PRs", "octo/cat", "#7", "Add feature", "alice", "https://example/pr/7"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output: %s", want, out)
		}
	}
}

func TestBuildSearchQuery(t *testing.T) {
	q := buildSearchQuery(config.Widget{
		Filters: config.Filters{
			Author:       "@me",
			State:        "open",
			Repos:        []string{"a/b", "c/d"},
			Label:        "bug",
			TargetBranch: "main",
		},
	})
	for _, want := range []string{"is:pr", "is:open", "author:@me", "repo:a/b", "repo:c/d", "label:\"bug\"", "base:main"} {
		if !strings.Contains(q, want) {
			t.Errorf("missing %q in query: %s", want, q)
		}
	}
}

func TestRenderError(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/pr_list.html")
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	w := &Widget{
		tmpl: tmpl,
		data: viewData{Title: "T", Error: "boom"},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Errorf("expected error in output: %s", buf.String())
	}
}

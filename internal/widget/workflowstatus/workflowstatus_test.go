package workflowstatus

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/aklinkert/perch/internal/views"
)

func TestRender(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/workflow_status.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	w := &Widget{
		tmpl: tmpl,
		data: viewData{
			Title:   "CI",
			Updated: "now",
			Items: []item{
				{Conclusion: "success", Status: "completed", URL: "https://x/run/1", RunNumber: 11, HeadBranch: "main", CreatedAt: "2025-01-01"},
				{Conclusion: "failure", Status: "completed", URL: "https://x/run/2", RunNumber: 12, HeadBranch: "main", CreatedAt: "2025-01-02"},
			},
		},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"CI", "#11", "#12", "main", "bg-green-500", "bg-red-500"} {
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
	if _, _, ok := splitRepo("invalid"); ok {
		t.Error("expected !ok for invalid")
	}
}

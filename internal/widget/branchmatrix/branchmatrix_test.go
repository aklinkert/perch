package branchmatrix

import (
	"bytes"
	"html/template"
	"strings"
	"testing"

	"github.com/aklinkert/work-dashboard/internal/views"
)

func TestRender(t *testing.T) {
	tmpl, err := template.New("").ParseFS(views.FS, "widgets/branch_matrix.html")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	w := &Widget{
		tmpl: tmpl,
		data: viewData{
			Title:    "Matrix",
			Updated:  "now",
			Repos:    []string{"a/b", "c/d"},
			Branches: []string{"main", "develop"},
			Cells: map[string]map[string]Cell{
				"a/b": {
					"main":    {Conclusion: "success", URL: "https://x/1"},
					"develop": {Conclusion: "failure", URL: "https://x/2"},
				},
				"c/d": {
					"main":    {Conclusion: "", URL: ""},
					"develop": {Conclusion: "success", URL: "https://x/3"},
				},
			},
		},
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Matrix", "a/b", "c/d", "main", "develop", "bg-green-600", "bg-red-600"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
}

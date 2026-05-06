package views

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed *.html widgets/*.html
var FS embed.FS

func Templates() (*template.Template, error) {
	t := template.New("").Funcs(funcs)
	sub, err := fs.Sub(FS, ".")
	if err != nil {
		return nil, err
	}
	return t.ParseFS(sub, "*.html", "widgets/*.html")
}

var funcs = template.FuncMap{
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "…"
	},
}

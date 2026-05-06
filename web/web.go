package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var embedded embed.FS

// Static is the static assets filesystem rooted at the `static` directory.
// Tailwind output (`app.css`), htmx, and the SSE extension live here.
var Static fs.FS = mustSub(embedded, "static")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"github.com/aklinkert/perch/internal/widget"
	"github.com/aklinkert/perch/web"
)

type layoutWidget struct {
	ID          string
	InitialHTML template.HTML
}

type layoutData struct {
	Columns int
	Widgets []layoutWidget
}

func (s *Server) registerRoutes() {
	s.app.Get("/", s.handleIndex)
	s.app.Get("/widget/:id", s.handleWidget)
	s.app.Get("/events", s.handleSSE)
	s.app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(web.Static),
	}))
}

// orderedWidgets returns widgets in the configured layout order, falling back
// to the registration order when Layout.Order is empty.
func (s *Server) orderedWidgets() []widget.Widget {
	if len(s.cfg.Layout.Order) == 0 {
		return s.widgets
	}
	out := make([]widget.Widget, 0, len(s.cfg.Layout.Order))
	for _, id := range s.cfg.Layout.Order {
		if w, ok := s.byID[id]; ok {
			out = append(out, w)
		}
	}
	return out
}

func (s *Server) handleIndex(c *fiber.Ctx) error {
	ws := s.orderedWidgets()
	data := layoutData{
		Columns: s.cfg.Layout.Columns,
		Widgets: make([]layoutWidget, 0, len(ws)),
	}
	for _, w := range ws {
		var buf bytes.Buffer
		if err := w.Render(&buf); err != nil {
			s.logger.Warn("initial render failed", "id", w.ID(), "error", err)
			buf.WriteString(fmt.Sprintf(`<div class="text-sm text-gray-500">loading %s…</div>`, template.HTMLEscapeString(w.Title())))
		}
		data.Widgets = append(data.Widgets, layoutWidget{
			ID:          w.ID(),
			InitialHTML: template.HTML(buf.String()),
		})
	}

	var out bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&out, "layout.html", data); err != nil {
		s.logger.Error("layout render failed", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "render failed")
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Send(out.Bytes())
}

func (s *Server) handleWidget(c *fiber.Ctx) error {
	id := c.Params("id")
	w, ok := s.byID[id]
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "unknown widget")
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		s.logger.Warn("widget render failed", "id", id, "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "render failed")
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Send(buf.Bytes())
}

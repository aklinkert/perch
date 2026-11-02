package server

import (
	"bytes"
	"encoding/json"
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
	ColSpan     int
	RowSpan     int
}

type layoutData struct {
	Columns int
	Widgets []layoutWidget
	Cols    [][]layoutWidget
}

func (s *Server) registerRoutes() {
	s.app.Get("/", s.handleIndex)
	s.app.Get("/widget/:id", s.handleWidget)
	s.app.Get("/events", s.handleSSE)
	s.app.Post("/layout", s.handleSaveLayout)
	s.app.Use("/static", filesystem.New(filesystem.Config{
		Root: http.FS(web.Static),
	}))
}

type savedLayout struct {
	Cols [][]string `json:"cols"`
}

// loadSavedLayout returns the persisted column ordering, or nil when none has
// been saved (or the saved order references widgets that no longer exist).
func (s *Server) loadSavedLayout() *savedLayout {
	if s.store == nil {
		return nil
	}
	ls, err := s.store.GetLayout()
	if err != nil || ls == nil {
		return nil
	}
	var sl savedLayout
	if err := json.Unmarshal(ls.Payload, &sl); err != nil {
		return nil
	}
	return &sl
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
			fmt.Fprintf(&buf, `<div class="text-sm text-gray-500">loading %s…</div>`, template.HTMLEscapeString(w.Title()))
		}
		lw := layoutWidget{
			ID:          w.ID(),
			InitialHTML: template.HTML(buf.String()),
		}
		if cw := s.cfg.WidgetByID(w.ID()); cw != nil {
			lw.ColSpan = cw.ColSpan
			lw.RowSpan = cw.RowSpan
		}
		data.Widgets = append(data.Widgets, lw)
	}

	if saved := s.loadSavedLayout(); saved != nil {
		data.Cols = applySavedLayout(data.Widgets, saved.Cols, data.Columns)
	} else {
		data.Cols = splitColumns(data.Widgets, data.Columns)
	}

	var out bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&out, "layout.html", data); err != nil {
		s.logger.Error("layout render failed", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "render failed")
	}
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Send(out.Bytes())
}

// splitColumns balances widgets across n columns by greedy bin-packing on
// rendered-HTML length, scanning in declared order and assigning each widget
// to whichever column currently has the smallest total weight. This keeps
// roughly equal column heights when one widget renders far longer than the
// rest, while preserving relative order within each column.
func splitColumns(ws []layoutWidget, n int) [][]layoutWidget {
	if n < 1 {
		n = 1
	}
	cols := make([][]layoutWidget, n)
	weights := make([]int, n)
	for _, w := range ws {
		target := 0
		for c := 1; c < n; c++ {
			if weights[c] < weights[target] {
				target = c
			}
		}
		cols[target] = append(cols[target], w)
		weights[target] += len(w.InitialHTML)
	}
	return cols
}

// applySavedLayout reorders widgets to match the persisted layout. Any widgets
// not referenced in the saved layout are appended into the lightest column,
// and any saved IDs that no longer correspond to a widget are skipped.
func applySavedLayout(ws []layoutWidget, savedCols [][]string, n int) [][]layoutWidget {
	if n < 1 {
		n = 1
	}
	byID := make(map[string]layoutWidget, len(ws))
	for _, w := range ws {
		byID[w.ID] = w
	}
	cols := make([][]layoutWidget, n)
	weights := make([]int, n)
	placed := make(map[string]bool, len(ws))
	for c, ids := range savedCols {
		if c >= n {
			break
		}
		for _, id := range ids {
			w, ok := byID[id]
			if !ok || placed[id] {
				continue
			}
			cols[c] = append(cols[c], w)
			weights[c] += len(w.InitialHTML)
			placed[id] = true
		}
	}
	for _, w := range ws {
		if placed[w.ID] {
			continue
		}
		target := 0
		for c := 1; c < n; c++ {
			if weights[c] < weights[target] {
				target = c
			}
		}
		cols[target] = append(cols[target], w)
		weights[target] += len(w.InitialHTML)
	}
	return cols
}

func (s *Server) handleSaveLayout(c *fiber.Ctx) error {
	if s.store == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "no store")
	}
	var sl savedLayout
	if err := json.Unmarshal(c.Body(), &sl); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid json")
	}
	known := s.byID
	cleaned := make([][]string, len(sl.Cols))
	for i, ids := range sl.Cols {
		for _, id := range ids {
			if _, ok := known[id]; ok {
				cleaned[i] = append(cleaned[i], id)
			}
		}
	}
	payload, err := json.Marshal(savedLayout{Cols: cleaned})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "encode")
	}
	if err := s.store.PutLayout(payload); err != nil {
		s.logger.Warn("save layout", "error", err)
		return fiber.NewError(fiber.StatusInternalServerError, "save")
	}
	return c.SendStatus(fiber.StatusNoContent)
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

package server

import (
	"context"
	"html/template"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/aklinkert/perch/internal/cache"
	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/widget"
)

// Server wraps a Fiber app together with the dependencies needed to render
// the dashboard layout and stream live widget updates over SSE.
type Server struct {
	app     *fiber.App
	cfg     *config.Config
	tmpl    *template.Template
	bcast   *widget.Broadcaster
	widgets []widget.Widget
	byID    map[string]widget.Widget
	store   *cache.Store
	logger  *slog.Logger
}

// New constructs the server, registers routes, and returns it ready to Listen.
func New(cfg *config.Config, tmpl *template.Template, bcast *widget.Broadcaster, widgets []widget.Widget, store *cache.Store) *Server {
	app := fiber.New(fiber.Config{
		AppName:               "perch",
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
		// Long-lived SSE streams need WriteTimeout disabled.
		WriteTimeout: 0,
		IdleTimeout:  0,
	})

	byID := make(map[string]widget.Widget, len(widgets))
	for _, w := range widgets {
		byID[w.ID()] = w
	}

	s := &Server{
		app:     app,
		cfg:     cfg,
		tmpl:    tmpl,
		bcast:   bcast,
		widgets: widgets,
		byID:    byID,
		store:   store,
		logger:  slog.Default().With("component", "server"),
	}
	s.registerRoutes()
	return s
}

// Listen starts the HTTP server. Blocks until the listener returns.
func (s *Server) Listen(addr string) error {
	s.logger.Info("listening", "addr", addr)
	return s.app.Listen(addr)
}

// Shutdown gracefully drains in-flight requests and stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}

// App exposes the underlying Fiber app — handy for tests / middleware wiring.
func (s *Server) App() *fiber.App { return s.app }

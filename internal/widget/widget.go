package widget

import (
	"context"
	"io"
	"time"

	"github.com/aklinkert/perch/internal/cache"
	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/github"
)

type Widget interface {
	ID() string
	Type() string
	Title() string
	Refresh(ctx context.Context) error
	Render(w io.Writer) error
	Interval() time.Duration
}

type Deps struct {
	Clients     *github.Clients
	Store       *cache.Store
	Broadcaster *Broadcaster
}

type Factory func(cfg config.Widget, deps Deps) (Widget, error)

type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: map[string]Factory{}}
}

func (r *Registry) Register(typ string, f Factory) {
	r.factories[typ] = f
}

func (r *Registry) Build(cfg config.Widget, deps Deps) (Widget, error) {
	f, ok := r.factories[cfg.Type]
	if !ok {
		return nil, ErrUnknownType{Type: cfg.Type}
	}
	return f(cfg, deps)
}

type ErrUnknownType struct{ Type string }

func (e ErrUnknownType) Error() string { return "unknown widget type: " + e.Type }

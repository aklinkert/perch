package widget

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"
)

type Scheduler struct {
	widgets []Widget
	bcast   *Broadcaster
	wg      sync.WaitGroup
}

func NewScheduler(widgets []Widget, bcast *Broadcaster) *Scheduler {
	return &Scheduler{widgets: widgets, bcast: bcast}
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, w := range s.widgets {
		s.wg.Add(1)
		go s.runOne(ctx, w)
	}
}

func (s *Scheduler) Wait() { s.wg.Wait() }

func (s *Scheduler) runOne(ctx context.Context, w Widget) {
	defer s.wg.Done()
	// Initial fetch ASAP
	s.tick(ctx, w)
	t := time.NewTicker(w.Interval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx, w)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, w Widget) {
	if err := w.Refresh(ctx); err != nil {
		slog.Warn("widget refresh failed", "id", w.ID(), "err", err)
	}
	var buf bytes.Buffer
	if err := w.Render(&buf); err != nil {
		slog.Warn("widget render failed", "id", w.ID(), "err", err)
		return
	}
	s.bcast.Publish(Update{WidgetID: w.ID(), HTML: buf.String()})
}

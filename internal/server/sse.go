package server

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

// sseHeartbeat is how often we send a comment line to keep proxies and
// browsers from giving up on idle SSE streams.
const sseHeartbeat = 25 * time.Second

func (s *Server) handleSSE(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/event-stream")
	c.Set(fiber.HeaderCacheControl, "no-cache")
	c.Set(fiber.HeaderConnection, "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	ctx := c.UserContext()
	bcast := s.bcast
	logger := s.logger

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ch := bcast.Subscribe()
		defer bcast.Unsubscribe(ch)

		// Initial comment so the client transitions to "open" immediately.
		if _, err := w.WriteString(": connected\n\n"); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		ticker := time.NewTicker(sseHeartbeat)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.WriteString(": ping\n\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case u, ok := <-ch:
				if !ok {
					return
				}
				if _, err := fmt.Fprintf(w, "event: widget-%s\n", u.WidgetID); err != nil {
					return
				}
				// Per the SSE spec, each line of payload needs its own
				// `data:` prefix. Splitting on newlines (CR/LF/CRLF) and
				// re-prefixing keeps multi-line HTML fragments framed
				// correctly.
				for _, line := range splitLines(u.HTML) {
					if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
						return
					}
				}
				if _, err := w.WriteString("\n"); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					logger.Debug("sse client gone", "widget", u.WidgetID, "error", err)
					return
				}
			}
		}
	})

	return nil
}

// splitLines splits on CRLF, LF, or CR boundaries without leaving stray empty
// trailing lines that would prematurely end an SSE event.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

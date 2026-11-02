package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/widget"
)

// TestHandleSSE streams over a real listener because app.Test buffers the
// whole body and never returns for a long-lived event stream.
func TestHandleSSE(t *testing.T) {
	bcast := widget.NewBroadcaster()
	s := New(&config.Config{}, nil, bcast, nil, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = s.App().Listener(ln) }()
	t.Cleanup(func() {
		// The stream goroutine only notices the closed client on its next
		// write, so do not wait for it to drain.
		_ = s.App().ShutdownWithTimeout(time.Second)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ln.Addr().String()+"/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get /events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	r := bufio.NewReader(resp.Body)
	if got := readEvent(t, r); !reflect.DeepEqual(got, []string{": connected"}) {
		t.Fatalf("initial event = %q, want [\": connected\"]", got)
	}

	// The handler subscribes before writing ": connected", so the publish
	// below cannot be dropped for lack of a subscriber.
	if n := bcast.ClientCount(); n != 1 {
		t.Fatalf("ClientCount = %d, want 1", n)
	}
	bcast.Publish(widget.Update{WidgetID: "prs", HTML: "<ul>\r\n<li>a</li>\n</ul>"})

	want := []string{
		"event: widget-prs",
		"data: <ul>",
		"data: <li>a</li>",
		"data: </ul>",
	}
	if got := readEvent(t, r); !reflect.DeepEqual(got, want) {
		t.Fatalf("broadcast event = %q, want %q", got, want)
	}
}

// readEvent reads lines up to the blank line that terminates an SSE event.
func readEvent(t *testing.T, r *bufio.Reader) []string {
	t.Helper()
	var lines []string
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream after %q: %v", lines, err)
		}
		line = strings.TrimSuffix(line, "\n")
		if line == "" {
			return lines
		}
		lines = append(lines, line)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{""}},
		{"single", "a", []string{"a"}},
		{"lf", "a\nb", []string{"a", "b"}},
		{"crlf", "a\r\nb", []string{"a", "b"}},
		{"cr", "a\rb", []string{"a", "b"}},
		{"mixed", "a\r\nb\nc\rd", []string{"a", "b", "c", "d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitLines(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitLines(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

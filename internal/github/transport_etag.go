package github

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/aklinkert/work-dashboard/internal/cache"
)

type etagTransport struct {
	instance string
	store    *cache.Store
	rl       *RateLimiter
	next     http.RoundTripper
}

func (t *etagTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	if req.Method != http.MethodGet || t.store == nil {
		return t.do(req, next)
	}
	key := req.URL.String()
	cached, _ := t.store.GetAPI(t.instance, key)
	if cached != nil && cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}
	resp, err := t.do(req, next)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotModified && cached != nil {
		resp.Body.Close()
		resp.StatusCode = http.StatusOK
		resp.Status = "200 OK (cached)"
		resp.Body = io.NopCloser(bytes.NewReader(cached.Body))
		resp.ContentLength = int64(len(cached.Body))
		return resp, nil
	}
	if resp.StatusCode == http.StatusOK {
		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr == nil {
			etag := resp.Header.Get("ETag")
			if etag != "" {
				_ = t.store.PutAPI(&cache.APICache{
					Instance: t.instance, Key: key, ETag: etag,
					Body: body, FetchedAt: time.Now(),
				})
			}
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
		} else {
			resp.Body = io.NopCloser(bytes.NewReader(nil))
		}
	}
	return resp, nil
}

func (t *etagTransport) do(req *http.Request, next http.RoundTripper) (*http.Response, error) {
	if err := t.rl.Wait(req.Context()); err != nil {
		return nil, err
	}
	resp, err := next.RoundTrip(req)
	if resp != nil {
		t.rl.Update(resp.Header)
	}
	return resp, err
}

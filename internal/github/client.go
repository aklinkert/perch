package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/aklinkert/work-dashboard/internal/cache"
	"github.com/aklinkert/work-dashboard/internal/config"
	gogithub "github.com/google/go-github/v66/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type Client struct {
	Name       string
	APIURL     string
	GraphQLURL string
	REST       *gogithub.Client
	GQL        *githubv4.Client
	HTTP       *http.Client
	rl         *RateLimiter
}

type Clients struct {
	mu sync.RWMutex
	m  map[string]*Client
}

func NewClients(cfgs []config.Instance, store *cache.Store) (*Clients, error) {
	cs := &Clients{m: make(map[string]*Client)}
	for _, inst := range cfgs {
		c, err := newClient(inst, store)
		if err != nil {
			return nil, fmt.Errorf("instance %s: %w", inst.Name, err)
		}
		cs.m[inst.Name] = c
	}
	return cs, nil
}

func (cs *Clients) Get(name string) *Client {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.m[name]
}

func (cs *Clients) All() map[string]*Client {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make(map[string]*Client, len(cs.m))
	for k, v := range cs.m {
		out[k] = v
	}
	return out
}

func newClient(inst config.Instance, store *cache.Store) (*Client, error) {
	rl := NewRateLimiter()
	tokenSrc := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: inst.Token})
	base := oauth2.NewClient(context.Background(), tokenSrc)
	base.Transport = &etagTransport{
		instance: inst.Name,
		store:    store,
		rl:       rl,
		next:     base.Transport,
	}

	rest, err := gogithub.NewClient(base).WithEnterpriseURLs(inst.APIURL, inst.APIURL)
	if err != nil {
		// fall back to default if api_url invalid for enterprise builder
		rest = gogithub.NewClient(base)
	}

	gqlURL := inst.GraphQLURL
	if gqlURL == "" {
		gqlURL = deriveGraphQL(inst.APIURL)
	}
	var gql *githubv4.Client
	if strings.Contains(inst.APIURL, "api.github.com") {
		gql = githubv4.NewClient(base)
	} else {
		gql = githubv4.NewEnterpriseClient(gqlURL, base)
	}

	return &Client{
		Name:       inst.Name,
		APIURL:     inst.APIURL,
		GraphQLURL: gqlURL,
		REST:       rest,
		GQL:        gql,
		HTTP:       base,
		rl:         rl,
	}, nil
}

func deriveGraphQL(apiURL string) string {
	u, err := url.Parse(apiURL)
	if err != nil {
		return apiURL
	}
	if strings.HasSuffix(u.Host, "api.github.com") {
		return "https://api.github.com/graphql"
	}
	// GHE: https://ghe.example.com/api/v3 -> https://ghe.example.com/api/graphql
	u.Path = strings.TrimSuffix(u.Path, "/v3") + "/graphql"
	if u.Path == "/graphql" {
		u.Path = "/api/graphql"
	}
	return u.String()
}

func (c *Client) RateLimit() RateLimitSnapshot {
	return c.rl.Snapshot()
}

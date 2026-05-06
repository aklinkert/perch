package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Minimal(t *testing.T) {
	t.Setenv("GH_TOKEN", "tok123")
	p := writeTemp(t, `
instances:
  - name: gh
    api_url: https://api.github.com
    token: ${GH_TOKEN}
widgets:
  - id: w1
    type: pr_list
    instance: gh
layout:
  order: [w1]
`)
	c, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Instances[0].Token != "tok123" {
		t.Fatalf("env not expanded: %q", c.Instances[0].Token)
	}
	if c.Widgets[0].Refresh != 5*time.Minute {
		t.Fatalf("default refresh not applied: %v", c.Widgets[0].Refresh)
	}
}

func TestLoad_DuplicateWidgetID(t *testing.T) {
	t.Setenv("T", "x")
	p := writeTemp(t, `
instances:
  - {name: gh, api_url: https://api.github.com, token: ${T}}
widgets:
  - {id: w1, type: pr_list, instance: gh}
  - {id: w1, type: pr_list, instance: gh}
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoad_UnknownInstance(t *testing.T) {
	t.Setenv("T", "x")
	p := writeTemp(t, `
instances:
  - {name: gh, api_url: https://api.github.com, token: ${T}}
widgets:
  - {id: w1, type: pr_list, instance: ghe}
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected unknown instance error")
	}
}

func TestLoad_LayoutMismatch(t *testing.T) {
	t.Setenv("T", "x")
	p := writeTemp(t, `
instances:
  - {name: gh, api_url: https://api.github.com, token: ${T}}
widgets:
  - {id: w1, type: pr_list, instance: gh}
layout:
  order: [w1, w2]
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected unknown layout id error")
	}
}

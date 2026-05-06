package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Instance struct {
	Name       string `yaml:"name" validate:"required"`
	APIURL     string `yaml:"api_url" validate:"required,url"`
	GraphQLURL string `yaml:"graphql_url"`
	Token      string `yaml:"token" validate:"required"`
}

type Defaults struct {
	PollInterval time.Duration `yaml:"poll_interval"`
}

type Filters struct {
	Author       string   `yaml:"author"`
	State        string   `yaml:"state"`
	Repos        []string `yaml:"repos"`
	Label        string   `yaml:"label"`
	TargetBranch string   `yaml:"target_branch"`
}

type Widget struct {
	ID       string        `yaml:"id" validate:"required"`
	Type     string        `yaml:"type" validate:"required"`
	Title    string        `yaml:"title"`
	Instance string        `yaml:"instance" validate:"required"`
	Refresh  time.Duration `yaml:"refresh"`
	Filters  Filters       `yaml:"filters"`
	Repos    []string      `yaml:"repos"`
	Repo     string        `yaml:"repo"`
	Branches []string      `yaml:"branches"`
	Branch   string        `yaml:"branch"`
	Workflow string        `yaml:"workflow"`
	LastN    int           `yaml:"last_n"`
}

type Layout struct {
	Columns int      `yaml:"columns"`
	Order   []string `yaml:"order"`
}

type Config struct {
	Instances []Instance `yaml:"instances" validate:"min=1,dive"`
	Defaults  Defaults   `yaml:"defaults"`
	Widgets   []Widget   `yaml:"widgets" validate:"dive"`
	Layout    Layout     `yaml:"layout"`
	DBPath    string     `yaml:"db_path"`
	Listen    string     `yaml:"listen"`
}

var envRE = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

func expandEnv(b []byte) []byte {
	return envRE.ReplaceAllFunc(b, func(m []byte) []byte {
		key := string(m[2 : len(m)-1])
		return []byte(os.Getenv(key))
	})
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	raw = expandEnv(raw)
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Defaults.PollInterval == 0 {
		c.Defaults.PollInterval = 5 * time.Minute
	}
	if c.DBPath == "" {
		c.DBPath = "./data/perch.db"
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.Layout.Columns == 0 {
		c.Layout.Columns = 2
	}
	for i := range c.Widgets {
		if c.Widgets[i].Refresh == 0 {
			c.Widgets[i].Refresh = c.Defaults.PollInterval
		}
	}
}

func (c *Config) validate() error {
	v := validator.New()
	if err := v.Struct(c); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}
	instances := map[string]bool{}
	for _, i := range c.Instances {
		if instances[i.Name] {
			return fmt.Errorf("duplicate instance %q", i.Name)
		}
		instances[i.Name] = true
	}
	ids := map[string]bool{}
	for _, w := range c.Widgets {
		if ids[w.ID] {
			return fmt.Errorf("duplicate widget id %q", w.ID)
		}
		ids[w.ID] = true
		if !instances[w.Instance] {
			return fmt.Errorf("widget %q references unknown instance %q", w.ID, w.Instance)
		}
	}
	for _, id := range c.Layout.Order {
		if !ids[id] {
			return fmt.Errorf("layout order references unknown widget %q", id)
		}
	}
	if len(c.Instances) == 0 {
		return errors.New("at least one instance required")
	}
	return nil
}

func (c *Config) WidgetByID(id string) *Widget {
	for i := range c.Widgets {
		if c.Widgets[i].ID == id {
			return &c.Widgets[i]
		}
	}
	return nil
}

func (c *Config) InstanceByName(name string) *Instance {
	for i := range c.Instances {
		if c.Instances[i].Name == name {
			return &c.Instances[i]
		}
	}
	return nil
}

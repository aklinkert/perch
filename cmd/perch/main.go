package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/samber/do/v2"
	"github.com/urfave/cli/v3"

	"github.com/aklinkert/perch/internal/cache"
	"github.com/aklinkert/perch/internal/config"
	"github.com/aklinkert/perch/internal/github"
	"github.com/aklinkert/perch/internal/server"
	"github.com/aklinkert/perch/internal/views"
	"github.com/aklinkert/perch/internal/widget"

	_ "github.com/aklinkert/perch/internal/widget/branchmatrix"
	_ "github.com/aklinkert/perch/internal/widget/myissues"
	_ "github.com/aklinkert/perch/internal/widget/prlist"
	_ "github.com/aklinkert/perch/internal/widget/repohealth"
	_ "github.com/aklinkert/perch/internal/widget/workflowstatus"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	app := &cli.Command{
		Name:  "dashboard",
		Usage: "Personal multi-instance GitHub work dashboard",
		Commands: []*cli.Command{
			runCmd(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func runCmd() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Run the dashboard server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to YAML config",
				Value:   "./config.yaml",
				Sources: cli.EnvVars("DASHBOARD_CONFIG"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return runDashboard(ctx, cmd.String("config"))
		},
	}
}

func runDashboard(parent context.Context, configPath string) error {
	logger := slog.Default().With("component", "main")

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Info("config loaded", "path", configPath, "widgets", len(cfg.Widgets), "instances", len(cfg.Instances))

	store, err := cache.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open cache: %w", err)
	}

	clients, err := github.NewClients(cfg.Instances, store)
	if err != nil {
		return fmt.Errorf("build github clients: %w", err)
	}

	tmpl, err := views.Templates()
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	bcast := widget.NewBroadcaster()

	registry := widget.NewRegistry()
	widget.RegisterAll(registry)

	deps := widget.Deps{
		Clients:     clients,
		Store:       store,
		Broadcaster: bcast,
	}

	widgets := make([]widget.Widget, 0, len(cfg.Widgets))
	for _, wcfg := range cfg.Widgets {
		w, err := registry.Build(wcfg, deps)
		if err != nil {
			return fmt.Errorf("build widget %q: %w", wcfg.ID, err)
		}
		widgets = append(widgets, w)
	}

	srv := server.New(cfg, tmpl, bcast, widgets, store)

	// DI container — registered for downstream consumers / tests.
	injector := do.New()
	do.ProvideValue(injector, cfg)
	do.ProvideValue(injector, store)
	do.ProvideValue(injector, clients)
	do.ProvideValue(injector, bcast)
	do.ProvideValue(injector, tmpl)
	do.ProvideValue(injector, srv)
	defer func() {
		if err := injector.Shutdown(); err != nil {
			logger.Warn("injector shutdown", "error", err)
		}
	}()

	// Signal-aware context for the scheduler and any blocking work.
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	scheduler := widget.NewScheduler(widgets, bcast)
	scheduler.Start(ctx)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Listen(cfg.Listen)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error("server crashed", "error", err)
			stop()
			scheduler.Wait()
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("server shutdown", "error", err)
	}
	stop()
	scheduler.Wait()
	logger.Info("bye")
	return nil
}

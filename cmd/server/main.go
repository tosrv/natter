package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tosrv/pmess/internal/api"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/jobs"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/realtime"
	"github.com/tosrv/pmess/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	if cfg.E2EFakeLogin {
		slog.Warn("!!! E2E_FAKE_LOGIN=1: POST /api/test/login is ENABLED — anyone can log in as anyone. NEVER use in production !!!")
	}

	now := time.Now
	msgLim := ratelimit.New(20, 10*time.Second, now)
	guestLim := ratelimit.New(cfg.GuestLimitPerHour(), time.Hour, now)
	searchLim := ratelimit.New(30, time.Minute, now)

	svc := chat.New(st, cfg.DeveloperEmail)
	hub := realtime.NewHub(svc, msgLim, cfg.AllowedOrigins)
	router := api.NewRouter(api.Deps{
		Auth:           auth.New(st, svc, cfg, guestLim),
		Chat:           svc,
		SearchLimiter:  searchLim,
		WS:             hub.Handler(),
		Notifier:       hub,
		StaticDir:      cfg.StaticDir,
		AllowedOrigins: cfg.AllowedOrigins,
	})

	jobsDone := make(chan struct{})
	go func() {
		defer close(jobsDone)
		jobs.Run(ctx, st, hub, []*ratelimit.Limiter{msgLim, guestLim, searchLim}, now)
	}()

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("listening", "addr", cfg.ListenAddr)

	select {
	case err := <-errc:
		stop()
		<-jobsDone
		return err
	case <-ctx.Done():
	}
	slog.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var shutErr error
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		shutErr = err
		_ = srv.Close()
	}
	<-jobsDone
	return shutErr
}

var _ api.Notifier = (*realtime.Hub)(nil)

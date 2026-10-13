package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/alert"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/config"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/mcpserver"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/server"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "kinhaven-server:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logOut io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: cfg.LogLevel}))

	st, err := store.Open(filepath.Join(cfg.DataDir, "kinhaven.json"))
	if err != nil {
		return err
	}
	mcpHandler := mcpserver.Handler(mcpserver.New(st, mcpserver.Config{
		Grace:    cfg.MissedDoseGrace,
		Location: cfg.Location,
		Now:      time.Now,
	}), logger)

	sender, err := newSender(ctx, cfg.Alerts, logger)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	if sender != nil {
		watcher := &alert.Watcher{Store: st, Sender: sender, Grace: cfg.MissedDoseGrace, Now: time.Now, Logger: logger}
		wg.Go(func() { watcher.Run(ctx, cfg.AlertInterval) })
	}

	err = server.Serve(ctx, ln, server.NewHandler(mcpHandler, cfg.AllowedOrigins), logger)
	cancel()
	wg.Wait()
	return err
}

func newSender(ctx context.Context, mode string, logger *slog.Logger) (alert.Sender, error) {
	switch mode {
	case "sns":
		return alert.NewSNSSender(ctx)
	case "log":
		return alert.LogSender{Logger: logger}, nil
	}
	return nil, nil
}

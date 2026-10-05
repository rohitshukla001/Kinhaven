package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/config"
)

func TestRunRejectsBadConfig(t *testing.T) {
	getenv := func(k string) string {
		if k == config.EnvAddr {
			return "0.0.0.0:8080"
		}
		return ""
	}
	err := run(context.Background(), getenv, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "load config") {
		t.Fatalf("run() error = %v, want a config error", err)
	}
}

func TestRunStartsAndStops(t *testing.T) {
	getenv := func(k string) string {
		if k == config.EnvAddr {
			return "127.0.0.1:0"
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var logs bytes.Buffer
	if err := run(ctx, getenv, &logs); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(logs.String(), "server listening") {
		t.Errorf("logs do not show startup:\n%s", logs.String())
	}
}

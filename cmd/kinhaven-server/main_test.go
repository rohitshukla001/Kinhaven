package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestRunRejectsBadConfig(t *testing.T) {
	getenv := func(k string) string {
		if k == "KINHAVEN_ADDR" {
			return "0.0.0.0:8080"
		}
		return ""
	}
	if err := run(context.Background(), getenv, io.Discard); err == nil {
		t.Fatal("want a config error")
	}
}

func TestRunStopsWhenContextEnds(t *testing.T) {
	dataDir := t.TempDir()
	getenv := func(k string) string {
		switch k {
		case "KINHAVEN_ADDR":
			return "127.0.0.1:0"
		case "KINHAVEN_DATA_DIR":
			return dataDir
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var logs bytes.Buffer
	if err := run(ctx, getenv, &logs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "listening") {
		t.Errorf("no startup log line:\n%s", logs.String())
	}
}

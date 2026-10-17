package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/mcpserver"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

func TestConsoleWithMockModel(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "Asia/Kolkata"}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mcpserver.Handler(mcpserver.New(st, mcpserver.Config{Grace: time.Hour, Location: time.UTC, Now: time.Now}), nil))
	defer srv.Close()

	getenv := func(k string) string {
		if k == "KINHAVEN_SIM_MCP_URL" {
			return srv.URL
		}
		return ""
	}
	var out bytes.Buffer
	in := strings.NewReader("What is due today?\nI feel great\n\n")
	if err := run(context.Background(), []string{"console"}, getenv, in, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"with 10 tools, model mock", "[get_schedule]", "no doses are scheduled", "[record_checkin]", "you feel well today"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestConsoleNeedsServer(t *testing.T) {
	getenv := func(k string) string {
		if k == "KINHAVEN_SIM_MCP_URL" {
			return "http://127.0.0.1:1/mcp"
		}
		return ""
	}
	if err := run(context.Background(), []string{"console"}, getenv, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "connect to") {
		t.Errorf("err = %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := run(context.Background(), []string{"serve"}, func(string) string { return "" }, nil, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("err = %v", err)
	}
}

func TestWebSimulatorStartsAndStops(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mcpserver.Handler(mcpserver.New(st, mcpserver.Config{Grace: time.Hour, Location: time.UTC, Now: time.Now}), nil))
	defer srv.Close()

	getenv := func(k string) string {
		switch k {
		case "KINHAVEN_SIM_MCP_URL":
			return srv.URL
		case "KINHAVEN_SIM_ADDR":
			return "127.0.0.1:0"
		}
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if err := run(ctx, nil, getenv, nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "simulator ready") {
		t.Errorf("logs:\n%s", out.String())
	}
}

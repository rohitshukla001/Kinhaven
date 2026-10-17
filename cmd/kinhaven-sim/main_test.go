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
	if err := run(context.Background(), getenv, in, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"with 10 tools, model mock", "[get_schedule]", "no doses are scheduled", "[record_checkin]", "you feel good today"} {
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
	if err := run(context.Background(), getenv, strings.NewReader(""), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "connect to") {
		t.Errorf("err = %v", err)
	}
}

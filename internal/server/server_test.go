package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var (
	allowed = []string{"http://localhost:8090"}
	echoMCP = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
)

func TestMCPOriginCheck(t *testing.T) {
	for _, tt := range []struct {
		origin string
		want   int
	}{
		{"", http.StatusTeapot},
		{"http://localhost:8090", http.StatusTeapot},
		{"http://LOCALHOST:8090", http.StatusTeapot},
		{"https://evil.example", http.StatusForbidden},
		{"http://localhost:9999", http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if tt.origin != "" {
			req.Header.Set("Origin", tt.origin)
		}
		rec := httptest.NewRecorder()
		NewHandler(echoMCP, allowed).ServeHTTP(rec, req)
		if rec.Code != tt.want {
			t.Errorf("Origin %q: status = %d, want %d", tt.origin, rec.Code, tt.want)
		}
		if tt.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), `"code":-32600`) {
			t.Errorf("Origin %q: body = %s, want a JSON-RPC error", tt.origin, rec.Body)
		}
	}
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	NewHandler(echoMCP, allowed).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" || body["name"] != "kinhaven" || body["version"] == "" {
		t.Errorf("body = %v", body)
	}
}

func TestUnknownRoutes(t *testing.T) {
	for _, tt := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
		{http.MethodGet, "/nope", http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		NewHandler(echoMCP, allowed).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
		if rec.Code != tt.want {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, rec.Code, tt.want)
		}
	}
}

func TestServeShutsDownOnCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + ln.Addr().String() + "/healthz"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, NewHandler(echoMCP, allowed), slog.New(slog.DiscardHandler)) }()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
	if _, err := http.Get(url); err == nil {
		t.Error("server still accepts connections")
	}
}

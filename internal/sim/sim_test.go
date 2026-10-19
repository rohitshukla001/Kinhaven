package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/Kinhaven/internal/agent"
	"github.com/rohitshukla001/Kinhaven/internal/care"
	"github.com/rohitshukla001/Kinhaven/internal/mcpserver"
	"github.com/rohitshukla001/Kinhaven/internal/store"
)

type fixture struct {
	store *store.Store
	url   string
	med   care.Medication
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := st.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	today, _ := care.DayBounds(time.Now(), time.UTC)
	med, err := st.AddMedication(care.Medication{RecipientID: r.ID, Name: "Metformin", Times: []care.ClockTime{{Hour: 23, Minute: 59}}, CreatedAt: today})
	if err != nil {
		t.Fatal(err)
	}
	mcpSrv := httptest.NewServer(mcpserver.Handler(mcpserver.New(st, mcpserver.Config{Grace: time.Hour, Location: time.UTC, Now: time.Now}), nil))
	t.Cleanup(mcpSrv.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "sim-test", Version: "v0"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpSrv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	a, err := agent.New(context.Background(), agent.Mock{}, session, agent.SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}
	web := httptest.NewServer(New("mock", a, session, slog.New(slog.DiscardHandler)).Handler())
	t.Cleanup(web.Close)
	return &fixture{store: st, url: web.URL, med: med}
}

func (f *fixture) post(t *testing.T, path string, body any, headers ...string) (*http.Response, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, f.url+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	return do(t, req)
}

func (f *fixture) get(t *testing.T, path string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, f.url+path, nil)
	return do(t, req)
}

func do(t *testing.T, req *http.Request) (*http.Response, map[string]any) {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestIndexPage(t *testing.T) {
	f := newFixture(t)
	resp, err := http.Get(f.url + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(page, `"ui/initialize"`) || !strings.Contains(page, `sandbox", "allow-scripts"`) {
		t.Errorf("status %d, page has no MCP Apps host code", resp.StatusCode)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Errorf("CSP = %q", csp)
	}
	if r, _ := http.Get(f.url + "/nope"); r.StatusCode != http.StatusNotFound {
		t.Errorf("unknown page = %d", r.StatusCode)
	}
}

func TestAskKeepsTheConversation(t *testing.T) {
	f := newFixture(t)
	resp, first := f.post(t, "/api/ask", map[string]string{"text": "What is due today?"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %v", resp.StatusCode, first)
	}
	calls := first["calls"].([]any)
	call := calls[0].(map[string]any)
	if call["tool"] != "get_schedule" || call["resource_uri"] != "ui://kinhaven/card.html" || call["result"] == nil {
		t.Errorf("call = %v", call)
	}
	if !strings.Contains(first["reply"].(string), "Next: Metformin at 23:59") {
		t.Errorf("reply = %v", first["reply"])
	}

	id := first["session_id"].(string)
	_, second := f.post(t, "/api/ask", map[string]string{"session_id": id, "text": "I feel fine"})
	if second["session_id"] != id || !strings.Contains(second["reply"].(string), "okay") {
		t.Errorf("second turn = %v", second)
	}
	_, info := f.get(t, "/api/info")
	if info["model"] != "mock" || info["tools"].(float64) != 10 {
		t.Errorf("info = %v", info)
	}
}

func TestRejectsBadRequests(t *testing.T) {
	f := newFixture(t)
	for _, tt := range []struct {
		name string
		do   func() *http.Response
		want int
	}{
		{"empty text", func() *http.Response { r, _ := f.post(t, "/api/ask", map[string]string{"text": "  "}); return r }, http.StatusBadRequest},
		{"long text", func() *http.Response {
			r, _ := f.post(t, "/api/ask", map[string]string{"text": strings.Repeat("a", 501)})
			return r
		}, http.StatusBadRequest},
		{"foreign origin", func() *http.Response {
			r, _ := f.post(t, "/api/ask", map[string]string{"text": "hi"}, "Origin", "https://evil.example")
			return r
		}, http.StatusForbidden},
		{"form post", func() *http.Response {
			resp, err := http.Post(f.url+"/api/ask", "text/plain", strings.NewReader(`{"text":"hi"}`))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			return resp
		}, http.StatusUnsupportedMediaType},
		{"not JSON", func() *http.Response {
			req, _ := http.NewRequest(http.MethodPost, f.url+"/api/ask", strings.NewReader("{"))
			req.Header.Set("Content-Type", "application/json")
			r, _ := do(t, req)
			return r
		}, http.StatusBadRequest},
		{"unknown tool", func() *http.Response { r, _ := f.post(t, "/api/tool", map[string]any{"name": "rm_rf"}); return r }, http.StatusNotFound},
		{"non-ui resource", func() *http.Response { r, _ := f.get(t, "/api/resource?uri=kinhaven://people/x/care-plan"); return r }, http.StatusBadRequest},
	} {
		if got := tt.do(); got.StatusCode != tt.want {
			t.Errorf("%s: status %d, want %d", tt.name, got.StatusCode, tt.want)
		}
	}
}

func TestCardHostEndpoints(t *testing.T) {
	f := newFixture(t)
	resp, card := f.get(t, "/api/resource?uri=ui://kinhaven/card.html")
	if resp.StatusCode != http.StatusOK || card["mimeType"] != "text/html;profile=mcp-app" || !strings.Contains(card["text"].(string), "Mark taken") {
		t.Fatalf("card: status %d mime %v", resp.StatusCode, card["mimeType"])
	}

	today, _ := care.DayBounds(time.Now(), time.UTC)
	slot := today.Add(23*time.Hour + 59*time.Minute).Format(time.RFC3339)
	_, logged := f.post(t, "/api/tool", map[string]any{"name": "log_dose", "arguments": map[string]any{"medication_id": f.med.ID, "status": "taken", "scheduled_at": slot}})
	if logged["isError"] == true || logged["structuredContent"].(map[string]any)["status"] != "taken" {
		t.Errorf("log_dose via card = %v", logged)
	}
}

func TestTrimKeepsWholeTurns(t *testing.T) {
	var history []agent.Message
	for range 10 {
		history = append(history,
			agent.Message{Role: agent.User, Parts: []agent.Part{{Text: "q"}}},
			agent.Message{Role: agent.Assistant, Parts: []agent.Part{{ToolUse: &agent.ToolUse{ID: "t"}}}},
			agent.Message{Role: agent.User, Parts: []agent.Part{{ToolResult: &agent.ToolResult{ToolUseID: "t"}}}},
			agent.Message{Role: agent.Assistant, Parts: []agent.Part{{Text: "a"}}},
		)
	}
	got := trim(history, 10)
	if len(got) != 8 || got[0].Parts[0].Text != "q" {
		t.Errorf("trimmed to %d messages starting with %+v", len(got), got[0])
	}
	if got := trim(history[:6], 10); len(got) != 6 {
		t.Errorf("short history changed: %d", len(got))
	}
}

func TestSessionsAreCapped(t *testing.T) {
	s := &Server{byID: map[string]*conversation{}}
	first, _ := s.conversation("")
	for range maxSessions {
		s.conversation("")
	}
	if len(s.byID) != maxSessions {
		t.Errorf("sessions = %d, want %d", len(s.byID), maxSessions)
	}
	if _, ok := s.byID[first]; ok {
		t.Error("oldest session was not dropped")
	}
	again, c := s.conversation(s.ordered[3])
	if again != s.ordered[3] || c != s.byID[again] {
		t.Error("existing session not reused")
	}
}

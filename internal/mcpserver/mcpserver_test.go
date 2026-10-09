package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/server"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

type fixture struct {
	store *store.Store
	loc   *time.Location
	day   time.Time
	now   time.Time
	url   string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1)
	f := &fixture{
		store: st,
		loc:   loc,
		day:   time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc),
	}
	f.now = f.day.Add(10 * time.Hour)

	h := server.NewHandler(Handler(New(st, Config{Grace: time.Hour, Location: loc, Now: func() time.Time { return f.now }}), nil), []string{"http://localhost:8090"})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	f.url = srv.URL + "/mcp"
	return f
}

func (f *fixture) connect(t *testing.T, protocol string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "kinhaven-test", Version: "v0"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: f.url}, &mcp.ClientSessionOptions{ProtocolVersion: protocol})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func (f *fixture) addPerson(t *testing.T, name string) care.Recipient {
	t.Helper()
	r, err := f.store.AddRecipient(care.Recipient{Name: name, Timezone: "Asia/Kolkata"})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *fixture) addMedication(t *testing.T, m care.Medication) care.Medication {
	t.Helper()
	m, err := f.store.AddMedication(m)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func clocks(t *testing.T, ss ...string) []care.ClockTime {
	t.Helper()
	out := make([]care.ClockTime, len(ss))
	for i, s := range ss {
		c, err := care.ParseClock(s)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = c
	}
	return out
}

func call[Out any](t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (Out, *mcp.CallToolResult) {
	t.Helper()
	var out Out
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		return out, res
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("%s: decode %s: %v", tool, b, err)
	}
	return out, res
}

func errorText(res *mcp.CallToolResult) string {
	if !res.IsError || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func TestHandshakeAndToolList(t *testing.T) {
	f := newFixture(t)
	for _, protocol := range []string{"2025-11-25", ""} {
		cs := f.connect(t, protocol)
		init := cs.InitializeResult()
		if init == nil || init.ServerInfo == nil || init.ServerInfo.Name != "kinhaven" {
			t.Fatalf("protocol %q: initialize result = %+v", protocol, init)
		}
		if init.ProtocolVersion < "2025-11-25" {
			t.Errorf("protocol %q: negotiated %s, want 2025-11-25 or later", protocol, init.ProtocolVersion)
		}
		if protocol != "" && init.ProtocolVersion != protocol {
			t.Errorf("negotiated %s, want %s", init.ProtocolVersion, protocol)
		}

		res, err := cs.ListTools(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		var readOnly, writes []string
		for _, tool := range res.Tools {
			if tool.Annotations != nil && tool.Annotations.ReadOnlyHint {
				readOnly = append(readOnly, tool.Name)
			} else {
				writes = append(writes, tool.Name)
			}
		}
		slices.Sort(readOnly)
		slices.Sort(writes)
		if want := []string{"get_adherence_summary", "get_schedule", "list_medications", "list_people"}; !slices.Equal(readOnly, want) {
			t.Errorf("read-only tools = %v, want %v", readOnly, want)
		}
		if want := []string{"add_contact", "add_medication", "add_person", "log_dose", "record_checkin", "stop_medication"}; !slices.Equal(writes, want) {
			t.Errorf("write tools = %v, want %v", writes, want)
		}
	}
}

func TestGetSchedule(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	metformin := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Dosage: "500 mg", Instructions: "after food", Times: clocks(t, "08:00", "20:30")})
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Thyroxine", Times: clocks(t, "07:00")})
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Amlodipine", Times: clocks(t, "09:30")})
	if _, err := f.store.RecordDose(care.DoseEvent{MedicationID: metformin.ID, Slot: f.day.Add(8 * time.Hour), Status: care.DoseTaken, RecordedAt: f.day.Add(8*time.Hour + 5*time.Minute)}); err != nil {
		t.Fatal(err)
	}

	cs := f.connect(t, "")
	out, res := call[getScheduleOutput](t, cs, "get_schedule", nil)
	if res.IsError {
		t.Fatal(errorText(res))
	}

	if out.Name != "Kamala" || out.Date != f.day.Format(time.DateOnly) || out.Timezone != "Asia/Kolkata" {
		t.Errorf("header = %s %s %s", out.Name, out.Date, out.Timezone)
	}
	if want := (scheduleSummary{Taken: 1, Missed: 1, Due: 1, Upcoming: 1}); out.Summary != want {
		t.Errorf("summary = %+v, want %+v", out.Summary, want)
	}
	var got []string
	for _, d := range out.Doses {
		got = append(got, d.Time+" "+d.Medication+" "+d.Status)
	}
	want := []string{"07:00 Thyroxine missed", "08:00 Metformin taken", "09:30 Amlodipine due", "20:30 Metformin upcoming"}
	if !slices.Equal(got, want) {
		t.Errorf("doses = %q, want %q", got, want)
	}
	taken := out.Doses[1]
	if taken.Dosage != "500 mg" || taken.Instructions != "after food" || !strings.HasSuffix(taken.RecordedAt, "+05:30") {
		t.Errorf("taken dose = %+v", taken)
	}
	if at, err := time.Parse(time.RFC3339, taken.ScheduledAt); err != nil || !at.Equal(f.day.Add(8*time.Hour)) {
		t.Errorf("scheduled_at = %q", taken.ScheduledAt)
	}
}

func TestGetScheduleForAnotherDate(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Times: clocks(t, "08:00")})
	cs := f.connect(t, "")

	next := f.day.AddDate(0, 0, 1).Format(time.DateOnly)
	out, res := call[getScheduleOutput](t, cs, "get_schedule", map[string]any{"date": next})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if out.Date != next || len(out.Doses) != 1 || out.Doses[0].Status != "upcoming" {
		t.Errorf("got %+v", out)
	}

	_, res = call[getScheduleOutput](t, cs, "get_schedule", map[string]any{"date": "05/10/2026"})
	if !strings.Contains(errorText(res), "YYYY-MM-DD") {
		t.Errorf("bad date: IsError=%v text=%q", res.IsError, errorText(res))
	}
}

func TestListMedications(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	vitD := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Vitamin D", Times: clocks(t, "09:00"), Days: []time.Weekday{time.Sunday}})
	aspirin := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Aspirin", Times: clocks(t, "21:00")})
	if err := f.store.StopMedication(aspirin.ID); err != nil {
		t.Fatal(err)
	}
	cs := f.connect(t, "")

	out, res := call[listMedicationsOutput](t, cs, "list_medications", nil)
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if len(out.Medications) != 1 || out.Medications[0].ID != vitD.ID ||
		!slices.Equal(out.Medications[0].Days, []string{"sunday"}) || !slices.Equal(out.Medications[0].Times, []string{"09:00"}) {
		t.Errorf("active medications = %+v", out.Medications)
	}

	out, _ = call[listMedicationsOutput](t, cs, "list_medications", map[string]any{"include_stopped": true})
	if len(out.Medications) != 2 {
		t.Errorf("with stopped = %+v", out.Medications)
	}
}

func TestChoosingThePerson(t *testing.T) {
	f := newFixture(t)
	cs := f.connect(t, "")

	_, res := call[getScheduleOutput](t, cs, "get_schedule", nil)
	if !strings.Contains(errorText(res), "no one is set up") {
		t.Errorf("empty store: %q", errorText(res))
	}

	kamala := f.addPerson(t, "Kamala")
	f.addPerson(t, "Gopal")

	_, res = call[getScheduleOutput](t, cs, "get_schedule", nil)
	if text := errorText(res); !strings.Contains(text, "Kamala ("+kamala.ID+")") || !strings.Contains(text, "Gopal") {
		t.Errorf("two people without id: %q", text)
	}

	out, res := call[getScheduleOutput](t, cs, "get_schedule", map[string]any{"recipient_id": kamala.ID})
	if res.IsError || out.Name != "Kamala" {
		t.Errorf("with id: %+v %q", out, errorText(res))
	}

	_, res = call[getScheduleOutput](t, cs, "get_schedule", map[string]any{"recipient_id": "rcp_missing"})
	if !strings.Contains(errorText(res), "not found") {
		t.Errorf("unknown id: %q", errorText(res))
	}

	people, _ := call[listPeopleOutput](t, cs, "list_people", nil)
	if len(people.People) != 2 || people.People[0].Timezone != "Asia/Kolkata" {
		t.Errorf("people = %+v", people)
	}
}

func TestTransportRejectsBadRequests(t *testing.T) {
	f := newFixture(t)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`

	for _, tt := range []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"foreign origin", map[string]string{"Origin": "https://evil.example", "Mcp-Protocol-Version": "2025-11-25"}, http.StatusForbidden},
		{"unknown protocol version", map[string]string{"Mcp-Protocol-Version": "1999-01-01"}, http.StatusBadRequest},
	} {
		req, err := http.NewRequest(http.MethodPost, f.url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		for k, v := range tt.headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.name, resp.StatusCode, tt.want)
		}
	}
}

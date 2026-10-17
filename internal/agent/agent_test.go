package agent

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/mcpserver"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

type kinhaven struct {
	store   *store.Store
	session *mcp.ClientSession
	day     time.Time
	now     time.Time
}

func startKinhaven(t *testing.T) *kinhaven {
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
	k := &kinhaven{store: st, day: time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc)}
	k.now = k.day.Add(8*time.Hour + 10*time.Minute)

	srv := mcpserver.New(st, mcpserver.Config{Grace: time.Hour, Location: loc, Now: func() time.Time { return k.now }})
	ts := httptest.NewServer(mcpserver.Handler(srv, nil))
	t.Cleanup(ts.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "agent-test", Version: "v0"}, nil)
	k.session, err = client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: ts.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k.session.Close() })
	return k
}

func (k *kinhaven) seed(t *testing.T) care.Recipient {
	t.Helper()
	r, err := k.store.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "Asia/Kolkata"})
	if err != nil {
		t.Fatal(err)
	}
	m := care.Medication{RecipientID: r.ID, Name: "Metformin", Dosage: "500 mg"}
	for _, s := range []string{"08:00", "20:30"} {
		c, _ := care.ParseClock(s)
		m.Times = append(m.Times, c)
	}
	if _, err := k.store.AddMedication(m); err != nil {
		t.Fatal(err)
	}
	return r
}

type scripted struct {
	replies []Message
	seen    [][]Message
	err     error
}

func (s *scripted) Converse(_ context.Context, _ string, _ []Tool, history []Message) (Message, error) {
	s.seen = append(s.seen, append([]Message(nil), history...))
	if s.err != nil {
		return Message{}, s.err
	}
	if len(s.replies) == 0 {
		return use("again", "get_schedule", map[string]any{}), nil
	}
	next := s.replies[0]
	s.replies = s.replies[1:]
	return next, nil
}

func TestAskRunsTheToolLoop(t *testing.T) {
	k := startKinhaven(t)
	k.seed(t)
	model := &scripted{replies: []Message{
		use("t1", "get_schedule", map[string]any{}),
		say("Kamala has not taken her 8 o'clock Metformin yet."),
	}}
	a, err := New(context.Background(), model, k.session, SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Tools()) != 10 {
		t.Errorf("tools = %d, want 10", len(a.Tools()))
	}

	history, turn, err := a.Ask(context.Background(), nil, "Has Mom taken her tablets?")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Reply != "Kamala has not taken her 8 o'clock Metformin yet." || len(history) != 4 {
		t.Errorf("reply %q, history %d", turn.Reply, len(history))
	}
	if len(turn.Calls) != 1 || turn.Calls[0].ResourceURI != "ui://kinhaven/card.html" || turn.Calls[0].Result == nil {
		t.Fatalf("calls = %+v", turn.Calls)
	}
	result := model.seen[1][2].Parts[0].ToolResult
	if result == nil || result.ToolUseID != "t1" || !strings.Contains(result.Text, "Due now: Metformin at 08:00") {
		t.Errorf("tool result sent to the model = %+v", result)
	}
}

func TestToolErrorsGoBackToTheModel(t *testing.T) {
	k := startKinhaven(t)
	model := &scripted{replies: []Message{
		use("t1", "no_such_tool", map[string]any{}),
		use("t2", "get_schedule", map[string]any{}),
		say("Nobody is set up yet."),
	}}
	a, err := New(context.Background(), model, k.session, SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}
	_, turn, err := a.Ask(context.Background(), nil, "Has Mom taken her tablets?")
	if err != nil {
		t.Fatal(err)
	}
	unknown := model.seen[1][2].Parts[0].ToolResult
	empty := model.seen[2][4].Parts[0].ToolResult
	if !unknown.IsError || !empty.IsError || !strings.Contains(empty.Text, "no one is set up") {
		t.Errorf("results = %+v, %+v", unknown, empty)
	}
	if turn.Reply != "Nobody is set up yet." || len(turn.Calls) != 2 {
		t.Errorf("turn = %+v", turn)
	}
}

func TestFailedTurnLeavesHistoryUnchanged(t *testing.T) {
	k := startKinhaven(t)
	k.seed(t)
	a, err := New(context.Background(), &scripted{}, k.session, SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}
	earlier := []Message{say("Hello.")}
	history, _, err := a.Ask(context.Background(), earlier, "Loop forever")
	if !errors.Is(err, ErrTooManySteps) || len(history) != 1 {
		t.Errorf("err = %v, history = %d", err, len(history))
	}

	a.model = &scripted{err: errors.New("throttled")}
	history, _, err = a.Ask(context.Background(), earlier, "Hi")
	if err == nil || len(history) != 1 {
		t.Errorf("err = %v, history = %d", err, len(history))
	}
}

func TestMockConversation(t *testing.T) {
	k := startKinhaven(t)
	r := k.seed(t)
	a, err := New(context.Background(), Mock{}, k.session, SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}

	var history []Message
	for _, step := range []struct {
		say, want string
	}{
		{"Has Mom taken her tablets?", "0 of 2 doses taken. Due now: Metformin at 08:00. Next: Metformin at 20:30."},
		{"She just took her Metformin", "Done. I marked the 08:00 dose of Metformin as taken."},
		{"She took her vitamins", "Which medicine was it? I know about Metformin."},
		{"How was her week?", "Kamala took 1 of 1 doses (100%)"},
		{"I feel a bit tired today", "I noted it for the family."},
	} {
		var turn Turn
		history, turn, err = a.Ask(context.Background(), history, step.say)
		if err != nil {
			t.Fatalf("%q: %v", step.say, err)
		}
		if !strings.Contains(turn.Reply, step.want) {
			t.Errorf("%q: reply %q, want it to contain %q", step.say, turn.Reply, step.want)
		}
	}

	if got := k.store.CheckIns(r.ID, k.day, k.day.AddDate(0, 0, 1)); len(got) != 1 || got[0].Mood != care.MoodPoor {
		t.Errorf("check-ins = %+v", got)
	}
}

func TestClassify(t *testing.T) {
	for q, want := range map[string]intent{
		"Has Mom taken her tablets?":   askSchedule,
		"What is due now":              askSchedule,
		"She took her Metformin":       logDose,
		"Mom skipped the evening dose": logDose,
		"How was her week?":            askSummary,
		"Give me the weekly update":    askSummary,
		"I'm feeling fine":             checkIn,
	} {
		if got := classify(q); got != want {
			t.Errorf("classify(%q) = %d, want %d", q, got, want)
		}
	}
	if moodOf("I feel okay") != "okay" || moodOf("I feel great") != "good" || moodOf("my knee pain is back") != "poor" {
		t.Error("moodOf mismatch")
	}
}

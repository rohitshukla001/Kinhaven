package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/Kinhaven/internal/care"
)

func TestToolsLinkToTheCard(t *testing.T) {
	f := newFixture(t)
	cs := f.connect(t, "")
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		ui, _ := tool.Meta["ui"].(map[string]any)
		uri, _ := ui["resourceUri"].(string)
		wantCard := tool.Name == "get_schedule" || tool.Name == "get_adherence_summary"
		if wantCard && uri != cardURI {
			t.Errorf("%s: ui.resourceUri = %q, want %q", tool.Name, uri, cardURI)
		}
		if !wantCard && ui != nil {
			t.Errorf("%s has UI metadata %v", tool.Name, ui)
		}
	}
}

func TestCardResource(t *testing.T) {
	f := newFixture(t)
	cs := f.connect(t, "")

	list, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Resources) != 1 || list.Resources[0].URI != cardURI || list.Resources[0].MIMEType != "text/html;profile=mcp-app" {
		t.Fatalf("resources = %+v", list.Resources)
	}

	read, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: cardURI})
	if err != nil {
		t.Fatal(err)
	}
	c := read.Contents[0]
	if c.MIMEType != appMIMEType || !strings.Contains(c.Text, `"ui/initialize"`) || !strings.Contains(c.Text, "ui/notifications/tool-result") {
		t.Errorf("card contents: mime=%q len=%d", c.MIMEType, len(c.Text))
	}
	if strings.Contains(c.Text, "http://") || strings.Contains(c.Text, "https://") {
		t.Error("card loads external content; MCP Apps hosts block it by default")
	}
	if ui, _ := c.Meta["ui"].(map[string]any); ui["prefersBorder"] != true {
		t.Errorf("content _meta = %v", c.Meta)
	}
}

func TestScheduleIncludesSpokenSummary(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Times: clocks(t, "08:00", "20:30")})
	cs := f.connect(t, "")

	_, res := call[getScheduleOutput](t, cs, "get_schedule", nil)
	if len(res.Content) != 2 {
		t.Fatalf("content blocks = %d, want 2", len(res.Content))
	}
	speech := res.Content[0].(*mcp.TextContent).Text
	want := "Kamala, " + f.day.Format("Monday 2 January") + ": 0 of 2 doses taken. Missed: Metformin at 08:00. Next: Metformin at 20:30."
	if speech != want {
		t.Errorf("speech = %q\nwant     %q", speech, want)
	}
	var decoded getScheduleOutput
	if err := json.Unmarshal([]byte(res.Content[1].(*mcp.TextContent).Text), &decoded); err != nil || len(decoded.Doses) != 2 {
		t.Errorf("JSON block: %v, %+v", err, decoded)
	}
}

func TestSpeech(t *testing.T) {
	day := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	got := scheduleSpeech(getScheduleOutput{
		Name:    "Kamala",
		Summary: scheduleSummary{Taken: 1, Missed: 1, Due: 1, Upcoming: 2},
		Doses: []dose{
			{Medication: "Thyroxine", Time: "07:00", Status: "missed", FamilyAlertedAt: "2026-10-16T08:01:00+05:30"},
			{Medication: "Metformin", Time: "08:00", Status: "taken"},
			{Medication: "Amlodipine", Time: "09:30", Status: "due"},
			{Medication: "Metformin", Time: "20:30", Status: "upcoming"},
			{Medication: "Calcium", Time: "21:00", Status: "upcoming"},
		},
	}, day)
	want := "Kamala, Friday 16 October: 1 of 5 doses taken. Missed: Thyroxine at 07:00 (family alerted). Due now: Amlodipine at 09:30. Next: Metformin at 20:30."
	if got != want {
		t.Errorf("schedule speech = %q\nwant              %q", got, want)
	}
	if got := scheduleSpeech(getScheduleOutput{Name: "Kamala"}, day); got != "Kamala, Friday 16 October: no doses are scheduled." {
		t.Errorf("empty day = %q", got)
	}

	pct := 71
	got = adherenceSpeech(adherenceOutput{
		Name: "Kamala", From: "2026-10-10", To: "2026-10-16",
		doseCounts: doseCounts{Taken: 5, Skipped: 1, Missed: 1, Percent: &pct},
		Moods:      map[string]int{"good": 2, "okay": 0, "poor": 1},
	})
	want = "Kamala took 5 of 7 doses (71%) between 2026-10-10 and 2026-10-16. Missed 1, skipped 1. Check-ins: 2 good, 0 okay, 1 poor."
	if got != want {
		t.Errorf("adherence speech = %q\nwant               %q", got, want)
	}
	if got := adherenceSpeech(adherenceOutput{Name: "Kamala", From: "a", To: "b", Moods: map[string]int{}}); !strings.Contains(got, "no doses due") {
		t.Errorf("empty adherence = %q", got)
	}
}

func TestPrompts(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	cs := f.connect(t, "")

	list, err := cs.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Prompts) != 2 {
		t.Fatalf("prompts = %d, want 2", len(list.Prompts))
	}

	for name, want := range map[string][]string{
		"daily_briefing":       {"Kamala", r.ID, "get_schedule", f.now.Format("Monday 2 January")},
		"weekly_family_update": {"Kamala", r.ID, "get_adherence_summary", "days 7"},
	} {
		res, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: name})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		text := res.Messages[0].Content.(*mcp.TextContent).Text
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("%s does not mention %q:\n%s", name, w, text)
			}
		}
	}

	f.addPerson(t, "Gopal")
	if _, err := cs.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "daily_briefing"}); err == nil || !strings.Contains(err.Error(), "recipient_id") {
		t.Errorf("two people without id: %v", err)
	}
}

func TestCarePlanResource(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Dosage: "500 mg", Instructions: "after food", Times: clocks(t, "08:00", "20:30")})
	f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Vitamin D", Times: clocks(t, "09:00"), Days: []time.Weekday{time.Sunday}})
	stopped := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Aspirin", Times: clocks(t, "21:00")})
	if err := f.store.StopMedication(stopped.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.AddContact(care.Contact{RecipientID: r.ID, Name: "Ravi", Relation: "son", Phone: "+919876543210", NotifyMissedDose: true}); err != nil {
		t.Fatal(err)
	}
	cs := f.connect(t, "")

	templates, err := cs.ListResourceTemplates(context.Background(), nil)
	if err != nil || len(templates.ResourceTemplates) != 1 || templates.ResourceTemplates[0].URITemplate != carePlanTemplate {
		t.Fatalf("templates = %+v, %v", templates, err)
	}

	read, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "kinhaven://people/" + r.ID + "/care-plan"})
	if err != nil {
		t.Fatal(err)
	}
	text := read.Contents[0].Text
	for _, want := range []string{
		"# Care plan for Kamala",
		"- **Metformin** 500 mg at 08:00, 20:30 (after food)",
		"- **Vitamin D** at 09:00 on sunday",
		"- Ravi (son), SMS alerts on",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("care plan is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Aspirin") || strings.Contains(text, "9876543210") {
		t.Errorf("care plan shows a stopped medicine or a phone number:\n%s", text)
	}

	if _, err := cs.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "kinhaven://people/rcp_missing/care-plan"}); err == nil {
		t.Error("unknown person: want an error")
	}
}

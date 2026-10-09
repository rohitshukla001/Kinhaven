package mcpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/version"
)

const instructions = `Kinhaven helps a family look after an elderly person: their medicines, the doses they take and how they feel each day.
All times are in the cared-for person's own timezone.
When more than one person is cared for, call list_people first and pass recipient_id to the other tools.`

type Config struct {
	Grace    time.Duration
	Location *time.Location
	Now      func() time.Time
}

type tools struct {
	store *store.Store
	cfg   Config
}

func New(st *store.Store, cfg Config) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    version.Name,
		Title:   "Kinhaven",
		Version: version.Version,
	}, &mcp.ServerOptions{Instructions: instructions})

	t := &tools{store: st, cfg: cfg}
	no, yes := false, true
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &no}
	additive := &mcp.ToolAnnotations{DestructiveHint: &no, OpenWorldHint: &no}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_people",
		Title:       "List people",
		Description: "List the people whose care Kinhaven tracks, with their IDs and timezones.",
		Annotations: readOnly,
	}, t.listPeople)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_medications",
		Title:       "List medications",
		Description: "List a person's medicines with dosage, instructions, daily dose times and, for weekly medicines, the days they are taken.",
		Annotations: readOnly,
	}, t.listMedications)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_schedule",
		Title:       "Get dose schedule",
		Description: "Show every dose for one day with its status: upcoming, due, taken, skipped or missed. Use it to answer questions such as \"Has Mom taken her morning tablets?\".",
		Annotations: readOnly,
	}, t.getSchedule)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_adherence_summary",
		Title:       "Get adherence summary",
		Description: "Summarize the last few days: doses taken, skipped and missed per medicine, the adherence percentage, and the daily check-ins with mood. Use it for weekly family updates.",
		Annotations: readOnly,
	}, t.adherenceSummary)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_person",
		Title:       "Add a person",
		Description: "Start tracking care for a new person.",
		Annotations: additive,
	}, t.addPerson)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_contact",
		Title:       "Add a family contact",
		Description: "Add a family member who gets an SMS when a dose is missed.",
		Annotations: additive,
	}, t.addContact)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_medication",
		Title:       "Add a medication",
		Description: "Add a medicine with its daily dose times. Confirm the name, dosage and times with the user before calling.",
		Annotations: additive,
	}, t.addMedication)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "stop_medication",
		Title:       "Stop a medication",
		Description: "Stop a medicine so it no longer appears in the schedule. Confirm with the user before calling.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &yes, IdempotentHint: true, OpenWorldHint: &no},
	}, t.stopMedication)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_dose",
		Title:       "Log a dose",
		Description: "Record that a dose was taken or skipped. Logging the same dose again replaces the earlier record.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &no, IdempotentHint: true, OpenWorldHint: &no},
	}, t.logDose)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "record_checkin",
		Title:       "Record a daily check-in",
		Description: "Record how the person feels today, with an optional note in their own words.",
		Annotations: additive,
	}, t.recordCheckIn)

	return s
}

func Handler(s *mcp.Server, logger *slog.Logger) http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{
		Stateless: true,
		Logger:    logger,
	})
}

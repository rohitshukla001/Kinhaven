package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const carePlanTemplate = "kinhaven://people/{recipient_id}/care-plan"

func (t *tools) addPrompts(s *mcp.Server) {
	recipientArg := []*mcp.PromptArgument{{
		Name:        "recipient_id",
		Description: "ID from list_people. Leave it out when only one person is cared for.",
	}}
	s.AddPrompt(&mcp.Prompt{
		Name:        "daily_briefing",
		Title:       "Daily briefing",
		Description: "A short spoken briefing for the person in care: what is taken, what is due and what comes next.",
		Arguments:   recipientArg,
	}, t.dailyBriefing)
	s.AddPrompt(&mcp.Prompt{
		Name:        "weekly_family_update",
		Title:       "Weekly family update",
		Description: "A warm update for the family about the last seven days of medicines and check-ins.",
		Arguments:   recipientArg,
	}, t.weeklyFamilyUpdate)

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: carePlanTemplate,
		Name:        "care-plan",
		Title:       "Care plan",
		Description: "The person's medicines, dose times, instructions and family contacts as Markdown.",
		MIMEType:    "text/markdown",
	}, t.carePlan)
}

func (t *tools) dailyBriefing(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	r, err := t.recipient(req.Params.Arguments["recipient_id"])
	if err != nil {
		return nil, err
	}
	loc, err := r.Location()
	if err != nil {
		return nil, err
	}
	today := t.cfg.Now().In(loc).Format("Monday 2 January")
	return promptResult("Daily briefing for "+r.Name, fmt.Sprintf(`Give %[1]s a short, warm spoken briefing for today, %[2]s.
Call get_schedule with recipient_id %[3]q.
Say which medicines are already taken, which are due now and which were missed. If the family was alerted about a missed dose, say so gently.
Finish with the next dose, its time and how to take it.
Speak directly to %[1]s, use short sentences and keep it under 60 words.`, r.Name, today, r.ID)), nil
}

func (t *tools) weeklyFamilyUpdate(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	r, err := t.recipient(req.Params.Arguments["recipient_id"])
	if err != nil {
		return nil, err
	}
	return promptResult("Weekly family update for "+r.Name, fmt.Sprintf(`Write a weekly update about %[1]s for the family.
Call get_adherence_summary with recipient_id %[2]q and days 7.
Start with the overall adherence percentage, then name any medicine that was missed or skipped more than once.
Mention how %[1]s felt this week from the check-ins and quote a note if there is one.
End with one practical suggestion. Keep it under 120 words and avoid medical advice.`, r.Name, r.ID)), nil
}

func promptResult(description, text string) *mcp.GetPromptResult {
	return &mcp.GetPromptResult{
		Description: description,
		Messages:    []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}},
	}
}

func (t *tools) carePlan(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	id, ok := strings.CutPrefix(uri, "kinhaven://people/")
	if ok {
		id, ok = strings.CutSuffix(id, "/care-plan")
	}
	if !ok || id == "" {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	r, err := t.store.Recipient(id)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Care plan for %s\n\nTimezone: %s\n\n## Medicines\n\n", r.Name, r.Timezone)
	active := 0
	for _, m := range t.store.Medications(r.ID) {
		if m.Stopped {
			continue
		}
		active++
		med := toMedication(m)
		line := "- **" + med.Name + "**"
		if med.Dosage != "" {
			line += " " + med.Dosage
		}
		line += " at " + strings.Join(med.Times, ", ")
		if len(med.Days) > 0 {
			line += " on " + strings.Join(med.Days, ", ")
		}
		if med.Instructions != "" {
			line += " (" + med.Instructions + ")"
		}
		b.WriteString(line + "\n")
	}
	if active == 0 {
		b.WriteString("No medicines yet.\n")
	}

	b.WriteString("\n## Family contacts\n\n")
	contacts := t.store.Contacts(r.ID)
	for _, c := range contacts {
		alerts := "no SMS alerts"
		if c.NotifyMissedDose {
			alerts = "SMS alerts on"
		}
		relation := ""
		if c.Relation != "" {
			relation = " (" + c.Relation + ")"
		}
		fmt.Fprintf(&b, "- %s%s, %s\n", c.Name, relation, alerts)
	}
	if len(contacts) == 0 {
		b.WriteString("No contacts yet.\n")
	}

	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: "text/markdown", Text: b.String()}}}, nil
}

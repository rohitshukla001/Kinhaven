package mcpserver

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

const maxSummaryDays = 90

type adherenceInput struct {
	RecipientID string `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	Days        int    `json:"days,omitempty" jsonschema:"Number of days to cover, counting today. From 1 to 90. Defaults to 7."`
}

type doseCounts struct {
	Taken   int  `json:"taken"`
	Skipped int  `json:"skipped"`
	Missed  int  `json:"missed"`
	Percent *int `json:"adherence_percent,omitempty"`
}

func (c *doseCounts) add(status care.SlotStatus) {
	switch status {
	case care.SlotTaken:
		c.Taken++
	case care.SlotSkipped:
		c.Skipped++
	case care.SlotMissed:
		c.Missed++
	}
}

func (c *doseCounts) finish() {
	if total := c.Taken + c.Skipped + c.Missed; total > 0 {
		p := int(math.Round(float64(c.Taken) * 100 / float64(total)))
		c.Percent = &p
	}
}

type medicationAdherence struct {
	MedicationID string `json:"medication_id"`
	Medication   string `json:"medication"`
	doseCounts
}

type adherenceOutput struct {
	RecipientID string `json:"recipient_id"`
	Name        string `json:"name"`
	From        string `json:"from"`
	To          string `json:"to"`
	doseCounts
	Medications []medicationAdherence `json:"medications"`
	Moods       map[string]int        `json:"moods"`
	CheckIns    []checkIn             `json:"check_ins"`
}

func (t *tools) adherenceSummary(_ context.Context, _ *mcp.CallToolRequest, in adherenceInput) (*mcp.CallToolResult, adherenceOutput, error) {
	days := cmp.Or(in.Days, 7)
	if days < 1 || days > maxSummaryDays {
		return nil, adherenceOutput{}, fmt.Errorf("days must be from 1 to %d", maxSummaryDays)
	}
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, adherenceOutput{}, err
	}
	loc, err := r.Location()
	if err != nil {
		return nil, adherenceOutput{}, err
	}

	now := t.cfg.Now()
	from, _ := care.DayBounds(now.AddDate(0, 0, 1-days), loc)
	sched := care.Schedule{Location: loc, Grace: t.cfg.Grace}
	slots := sched.Slots(t.store.Medications(r.ID), t.store.Doses(r.ID, from, now), from, now, now)

	out := adherenceOutput{
		RecipientID: r.ID,
		Name:        r.Name,
		From:        from.Format(time.DateOnly),
		To:          now.In(loc).Format(time.DateOnly),
		Medications: []medicationAdherence{},
		Moods:       map[string]int{string(care.MoodGood): 0, string(care.MoodOkay): 0, string(care.MoodPoor): 0},
		CheckIns:    []checkIn{},
	}
	byMed := map[string]*medicationAdherence{}
	for _, s := range slots {
		out.add(s.Status)
		m, ok := byMed[s.Medication.ID]
		if !ok {
			m = &medicationAdherence{MedicationID: s.Medication.ID, Medication: s.Medication.Name}
			byMed[s.Medication.ID] = m
		}
		m.add(s.Status)
	}
	out.finish()
	for _, m := range byMed {
		m.finish()
		out.Medications = append(out.Medications, *m)
	}
	slices.SortFunc(out.Medications, func(a, b medicationAdherence) int { return cmp.Compare(a.Medication, b.Medication) })

	for _, c := range t.store.CheckIns(r.ID, from, now.Add(time.Nanosecond)) {
		out.Moods[string(c.Mood)]++
		out.CheckIns = append(out.CheckIns, checkIn{At: c.At.In(loc).Format(time.RFC3339), Mood: string(c.Mood), Note: c.Note})
	}
	return nil, out, nil
}

package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

type person struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

type listPeopleOutput struct {
	People []person `json:"people"`
}

func (t *tools) listPeople(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, listPeopleOutput, error) {
	out := listPeopleOutput{People: []person{}}
	for _, r := range t.store.Recipients() {
		out.People = append(out.People, person{ID: r.ID, Name: r.Name, Timezone: r.Timezone})
	}
	return nil, out, nil
}

type listMedicationsInput struct {
	RecipientID    string `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	IncludeStopped bool   `json:"include_stopped,omitempty" jsonschema:"Also list medicines that were stopped."`
}

type medication struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Dosage       string   `json:"dosage,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	Times        []string `json:"times"`
	Days         []string `json:"days,omitempty"`
	Stopped      bool     `json:"stopped,omitempty"`
}

type listMedicationsOutput struct {
	RecipientID string       `json:"recipient_id"`
	Medications []medication `json:"medications"`
}

func (t *tools) listMedications(_ context.Context, _ *mcp.CallToolRequest, in listMedicationsInput) (*mcp.CallToolResult, listMedicationsOutput, error) {
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, listMedicationsOutput{}, err
	}
	out := listMedicationsOutput{RecipientID: r.ID, Medications: []medication{}}
	for _, m := range t.store.Medications(r.ID) {
		if m.Stopped && !in.IncludeStopped {
			continue
		}
		med := medication{
			ID:           m.ID,
			Name:         m.Name,
			Dosage:       m.Dosage,
			Instructions: m.Instructions,
			Stopped:      m.Stopped,
		}
		for _, c := range m.Times {
			med.Times = append(med.Times, c.String())
		}
		for _, d := range m.Days {
			med.Days = append(med.Days, strings.ToLower(d.String()))
		}
		out.Medications = append(out.Medications, med)
	}
	return nil, out, nil
}

type getScheduleInput struct {
	RecipientID string `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	Date        string `json:"date,omitempty" jsonschema:"Day to show, as YYYY-MM-DD in the person's timezone. Defaults to today."`
}

type dose struct {
	MedicationID string `json:"medication_id"`
	Medication   string `json:"medication"`
	Dosage       string `json:"dosage,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Time         string `json:"time"`
	ScheduledAt  string `json:"scheduled_at"`
	Status       string `json:"status"`
	RecordedAt   string `json:"recorded_at,omitempty"`
	Note         string `json:"note,omitempty"`
}

type scheduleSummary struct {
	Taken    int `json:"taken"`
	Skipped  int `json:"skipped"`
	Missed   int `json:"missed"`
	Due      int `json:"due"`
	Upcoming int `json:"upcoming"`
}

type getScheduleOutput struct {
	RecipientID string          `json:"recipient_id"`
	Name        string          `json:"name"`
	Date        string          `json:"date"`
	Timezone    string          `json:"timezone"`
	Summary     scheduleSummary `json:"summary"`
	Doses       []dose          `json:"doses"`
}

func (t *tools) getSchedule(_ context.Context, _ *mcp.CallToolRequest, in getScheduleInput) (*mcp.CallToolResult, getScheduleOutput, error) {
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, getScheduleOutput{}, err
	}
	loc, err := r.Location()
	if err != nil {
		return nil, getScheduleOutput{}, err
	}

	now := t.now()
	day := now
	if in.Date != "" {
		if day, err = time.ParseInLocation(time.DateOnly, in.Date, loc); err != nil {
			return nil, getScheduleOutput{}, fmt.Errorf("date %q must be in YYYY-MM-DD format", in.Date)
		}
	}
	start, end := care.DayBounds(day, loc)
	sched := care.Schedule{Location: loc, Grace: t.grace}
	slots := sched.Day(t.store.Medications(r.ID), t.store.Doses(r.ID, start, end), start, now)

	out := getScheduleOutput{
		RecipientID: r.ID,
		Name:        r.Name,
		Date:        start.Format(time.DateOnly),
		Timezone:    r.Timezone,
		Doses:       make([]dose, 0, len(slots)),
	}
	for _, s := range slots {
		d := dose{
			MedicationID: s.Medication.ID,
			Medication:   s.Medication.Name,
			Dosage:       s.Medication.Dosage,
			Instructions: s.Medication.Instructions,
			Time:         s.At.Format("15:04"),
			ScheduledAt:  s.At.Format(time.RFC3339),
			Status:       string(s.Status),
		}
		if s.Dose != nil {
			d.RecordedAt = s.Dose.RecordedAt.In(loc).Format(time.RFC3339)
			d.Note = s.Dose.Note
		}
		out.Doses = append(out.Doses, d)
		out.Summary.add(s.Status)
	}
	return nil, out, nil
}

func (s *scheduleSummary) add(status care.SlotStatus) {
	switch status {
	case care.SlotTaken:
		s.Taken++
	case care.SlotSkipped:
		s.Skipped++
	case care.SlotMissed:
		s.Missed++
	case care.SlotDue:
		s.Due++
	case care.SlotUpcoming:
		s.Upcoming++
	}
}

func (t *tools) recipient(id string) (care.Recipient, error) {
	if id != "" {
		return t.store.Recipient(id)
	}
	people := t.store.Recipients()
	switch len(people) {
	case 0:
		return care.Recipient{}, errors.New("no one is set up in Kinhaven yet")
	case 1:
		return people[0], nil
	}
	names := make([]string, len(people))
	for i, p := range people {
		names[i] = fmt.Sprintf("%s (%s)", p.Name, p.ID)
	}
	return care.Recipient{}, fmt.Errorf("more than one person is cared for; pass recipient_id as one of: %s", strings.Join(names, ", "))
}

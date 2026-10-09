package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

type addPersonInput struct {
	Name     string `json:"name" jsonschema:"Name the family uses for the person, for example Mom or Kamala."`
	Timezone string `json:"timezone,omitempty" jsonschema:"IANA timezone such as Asia/Kolkata. Defaults to the server's timezone."`
}

func (t *tools) addPerson(_ context.Context, _ *mcp.CallToolRequest, in addPersonInput) (*mcp.CallToolResult, person, error) {
	tz := in.Timezone
	if tz == "" {
		tz = t.cfg.Location.String()
	}
	r, err := t.store.AddRecipient(care.Recipient{Name: strings.TrimSpace(in.Name), Timezone: tz})
	if err != nil {
		return nil, person{}, err
	}
	return nil, person{ID: r.ID, Name: r.Name, Timezone: r.Timezone}, nil
}

type addContactInput struct {
	RecipientID      string `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	Name             string `json:"name" jsonschema:"Family member's name."`
	Relation         string `json:"relation,omitempty" jsonschema:"How they are related, for example son or daughter."`
	Phone            string `json:"phone" jsonschema:"Mobile number in international format, for example +919876543210."`
	NotifyMissedDose *bool  `json:"notify_missed_dose,omitempty" jsonschema:"Send this person an SMS when a dose is missed. Defaults to true."`
}

type contact struct {
	ID               string `json:"id"`
	RecipientID      string `json:"recipient_id"`
	Name             string `json:"name"`
	Relation         string `json:"relation,omitempty"`
	Phone            string `json:"phone"`
	NotifyMissedDose bool   `json:"notify_missed_dose"`
}

func (t *tools) addContact(_ context.Context, _ *mcp.CallToolRequest, in addContactInput) (*mcp.CallToolResult, contact, error) {
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, contact{}, err
	}
	notify := in.NotifyMissedDose == nil || *in.NotifyMissedDose
	c, err := t.store.AddContact(care.Contact{
		RecipientID:      r.ID,
		Name:             strings.TrimSpace(in.Name),
		Relation:         strings.TrimSpace(in.Relation),
		Phone:            strings.ReplaceAll(in.Phone, " ", ""),
		NotifyMissedDose: notify,
	})
	if err != nil {
		return nil, contact{}, err
	}
	return nil, contact{ID: c.ID, RecipientID: c.RecipientID, Name: c.Name, Relation: c.Relation, Phone: c.Phone, NotifyMissedDose: c.NotifyMissedDose}, nil
}

type addMedicationInput struct {
	RecipientID  string   `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	Name         string   `json:"name" jsonschema:"Medicine name, for example Metformin."`
	Dosage       string   `json:"dosage,omitempty" jsonschema:"Amount per dose, for example 500 mg or 1 tablet."`
	Instructions string   `json:"instructions,omitempty" jsonschema:"How to take it, for example after food."`
	Times        []string `json:"times" jsonschema:"Daily dose times in 24-hour HH:MM format, for example [\"08:00\", \"20:30\"]."`
	Days         []string `json:"days,omitempty" jsonschema:"Days of the week for weekly medicines, for example [\"sunday\"]. Leave it out for every day."`
}

func (t *tools) addMedication(_ context.Context, _ *mcp.CallToolRequest, in addMedicationInput) (*mcp.CallToolResult, medication, error) {
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, medication{}, err
	}
	m := care.Medication{
		RecipientID:  r.ID,
		Name:         strings.TrimSpace(in.Name),
		Dosage:       strings.TrimSpace(in.Dosage),
		Instructions: strings.TrimSpace(in.Instructions),
	}
	for _, s := range in.Times {
		c, err := care.ParseClock(strings.TrimSpace(s))
		if err != nil {
			return nil, medication{}, err
		}
		m.Times = append(m.Times, c)
	}
	for _, s := range in.Days {
		d, err := care.ParseWeekday(s)
		if err != nil {
			return nil, medication{}, err
		}
		m.Days = append(m.Days, d)
	}
	if m, err = t.store.AddMedication(m); err != nil {
		return nil, medication{}, err
	}
	return nil, toMedication(m), nil
}

type stopMedicationInput struct {
	MedicationID string `json:"medication_id" jsonschema:"ID from list_medications."`
}

func (t *tools) stopMedication(_ context.Context, _ *mcp.CallToolRequest, in stopMedicationInput) (*mcp.CallToolResult, medication, error) {
	if err := t.store.StopMedication(in.MedicationID); err != nil {
		return nil, medication{}, err
	}
	m, err := t.store.Medication(in.MedicationID)
	if err != nil {
		return nil, medication{}, err
	}
	return nil, toMedication(m), nil
}

type logDoseInput struct {
	MedicationID string `json:"medication_id" jsonschema:"ID from get_schedule or list_medications."`
	Status       string `json:"status" jsonschema:"taken or skipped."`
	ScheduledAt  string `json:"scheduled_at,omitempty" jsonschema:"scheduled_at value from get_schedule. Leave it out to use today's unrecorded dose closest to now."`
	Note         string `json:"note,omitempty" jsonschema:"Optional note, for example the reason a dose was skipped."`
}

type logDoseOutput struct {
	MedicationID string `json:"medication_id"`
	Medication   string `json:"medication"`
	ScheduledAt  string `json:"scheduled_at"`
	Status       string `json:"status"`
	RecordedAt   string `json:"recorded_at"`
	Note         string `json:"note,omitempty"`
}

func (t *tools) logDose(_ context.Context, _ *mcp.CallToolRequest, in logDoseInput) (*mcp.CallToolResult, logDoseOutput, error) {
	m, err := t.store.Medication(in.MedicationID)
	if err != nil {
		return nil, logDoseOutput{}, err
	}
	r, err := t.store.Recipient(m.RecipientID)
	if err != nil {
		return nil, logDoseOutput{}, err
	}
	loc, err := r.Location()
	if err != nil {
		return nil, logDoseOutput{}, err
	}

	now := t.cfg.Now()
	slot, err := t.findSlot(m, loc, in.ScheduledAt, now)
	if err != nil {
		return nil, logDoseOutput{}, err
	}
	e, err := t.store.RecordDose(care.DoseEvent{
		MedicationID: m.ID,
		Slot:         slot,
		Status:       care.DoseStatus(strings.ToLower(strings.TrimSpace(in.Status))),
		RecordedAt:   now,
		Note:         strings.TrimSpace(in.Note),
	})
	if err != nil {
		return nil, logDoseOutput{}, err
	}
	return nil, logDoseOutput{
		MedicationID: m.ID,
		Medication:   m.Name,
		ScheduledAt:  slot.In(loc).Format(time.RFC3339),
		Status:       string(e.Status),
		RecordedAt:   e.RecordedAt.In(loc).Format(time.RFC3339),
		Note:         e.Note,
	}, nil
}

func (t *tools) findSlot(m care.Medication, loc *time.Location, scheduledAt string, now time.Time) (time.Time, error) {
	sched := care.Schedule{Location: loc, Grace: t.cfg.Grace}
	meds := []care.Medication{m}

	if scheduledAt != "" {
		at, err := time.Parse(time.RFC3339, scheduledAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("scheduled_at %q must be an RFC 3339 time such as 2026-10-05T08:00:00+05:30", scheduledAt)
		}
		start, end := care.DayBounds(at, loc)
		for _, s := range sched.Slots(meds, nil, start, end, now) {
			if s.At.Equal(at) {
				return s.At, nil
			}
		}
		return time.Time{}, fmt.Errorf("%s has no dose at %s; call get_schedule for the dose times", m.Name, at.In(loc).Format(time.RFC3339))
	}

	start, end := care.DayBounds(now, loc)
	var best time.Time
	for _, s := range sched.Slots(meds, t.store.Doses(m.RecipientID, start, end), start, end, now) {
		if s.Dose == nil && (best.IsZero() || distance(s.At, now) < distance(best, now)) {
			best = s.At
		}
	}
	if best.IsZero() {
		return time.Time{}, fmt.Errorf("every %s dose for today is already recorded; pass scheduled_at to change one", m.Name)
	}
	return best, nil
}

func distance(a, b time.Time) time.Duration {
	return max(a.Sub(b), b.Sub(a))
}

type recordCheckInInput struct {
	RecipientID string `json:"recipient_id,omitempty" jsonschema:"ID from list_people. Leave it out when only one person is cared for."`
	Mood        string `json:"mood" jsonschema:"good, okay or poor."`
	Note        string `json:"note,omitempty" jsonschema:"What the person said or anything the family should know."`
}

type checkIn struct {
	At   string `json:"at"`
	Mood string `json:"mood"`
	Note string `json:"note,omitempty"`
}

func (t *tools) recordCheckIn(_ context.Context, _ *mcp.CallToolRequest, in recordCheckInInput) (*mcp.CallToolResult, checkIn, error) {
	r, err := t.recipient(in.RecipientID)
	if err != nil {
		return nil, checkIn{}, err
	}
	loc, err := r.Location()
	if err != nil {
		return nil, checkIn{}, err
	}
	c, err := t.store.RecordCheckIn(care.CheckIn{
		RecipientID: r.ID,
		At:          t.cfg.Now(),
		Mood:        care.Mood(strings.ToLower(strings.TrimSpace(in.Mood))),
		Note:        strings.TrimSpace(in.Note),
	})
	if err != nil {
		return nil, checkIn{}, err
	}
	return nil, checkIn{At: c.At.In(loc).Format(time.RFC3339), Mood: string(c.Mood), Note: c.Note}, nil
}

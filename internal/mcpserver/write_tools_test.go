package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

func TestAddPersonAndContact(t *testing.T) {
	f := newFixture(t)
	cs := f.connect(t, "")

	p, res := call[person](t, cs, "add_person", map[string]any{"name": " Kamala "})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if p.Name != "Kamala" || p.Timezone != "Asia/Kolkata" || !strings.HasPrefix(p.ID, "rcp_") {
		t.Errorf("person = %+v", p)
	}
	_, res = call[person](t, cs, "add_person", map[string]any{"name": "Gopal", "timezone": "Mars/Base"})
	if !strings.Contains(errorText(res), "unknown timezone") {
		t.Errorf("bad timezone: %q", errorText(res))
	}

	c, res := call[contact](t, cs, "add_contact", map[string]any{"recipient_id": p.ID, "name": "Ravi", "relation": "son", "phone": "+91 98765 43210"})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if c.Phone != "+919876543210" || !c.NotifyMissedDose {
		t.Errorf("contact = %+v", c)
	}
	c, _ = call[contact](t, cs, "add_contact", map[string]any{"recipient_id": p.ID, "name": "Asha", "phone": "+14155550100", "notify_missed_dose": false})
	if c.NotifyMissedDose {
		t.Error("notify_missed_dose=false was ignored")
	}
	_, res = call[contact](t, cs, "add_contact", map[string]any{"recipient_id": p.ID, "name": "Ravi", "phone": "98765 43210"})
	if !strings.Contains(errorText(res), "E.164") {
		t.Errorf("local phone: %q", errorText(res))
	}
	if got := f.store.Contacts(p.ID); len(got) != 2 {
		t.Errorf("stored contacts = %d, want 2", len(got))
	}
}

func TestAddAndStopMedication(t *testing.T) {
	f := newFixture(t)
	f.addPerson(t, "Kamala")
	cs := f.connect(t, "")

	m, res := call[medication](t, cs, "add_medication", map[string]any{
		"name": "Vitamin D", "dosage": "60000 IU", "times": []string{"09:00"}, "days": []string{"Sun", "wednesday"},
	})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if !slices.Equal(m.Days, []string{"sunday", "wednesday"}) || !slices.Equal(m.Times, []string{"09:00"}) || m.Dosage != "60000 IU" {
		t.Errorf("medication = %+v", m)
	}

	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"name": "Metformin", "times": []string{"8am"}}, "HH:MM"},
		{map[string]any{"name": "Metformin", "times": []string{}}, "at least one dose time"},
		{map[string]any{"name": "Metformin", "times": []string{"08:00"}, "days": []string{"funday"}}, "not a day of the week"},
		{map[string]any{"name": "", "times": []string{"08:00"}}, "name is required"},
	} {
		_, res := call[medication](t, cs, "add_medication", tt.args)
		if !strings.Contains(errorText(res), tt.want) {
			t.Errorf("%v: error %q, want it to mention %q", tt.args, errorText(res), tt.want)
		}
	}

	stopped, res := call[medication](t, cs, "stop_medication", map[string]any{"medication_id": m.ID})
	if res.IsError || !stopped.Stopped {
		t.Fatalf("stop: %+v %q", stopped, errorText(res))
	}
	list, _ := call[listMedicationsOutput](t, cs, "list_medications", nil)
	if len(list.Medications) != 0 {
		t.Errorf("stopped medicine still listed: %+v", list.Medications)
	}
	_, res = call[medication](t, cs, "stop_medication", map[string]any{"medication_id": "med_missing"})
	if !strings.Contains(errorText(res), "not found") {
		t.Errorf("unknown id: %q", errorText(res))
	}
}

func TestLogDose(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	m := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Times: clocks(t, "08:00", "13:00", "20:30")})
	cs := f.connect(t, "")
	at := func(h, min int) string {
		return f.day.Add(time.Duration(h)*time.Hour + time.Duration(min)*time.Minute).Format(time.RFC3339)
	}

	f.now = f.day.Add(12*time.Hour + 40*time.Minute)
	got, res := call[logDoseOutput](t, cs, "log_dose", map[string]any{"medication_id": m.ID, "status": "Taken"})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if got.ScheduledAt != at(13, 0) || got.Status != "taken" || got.RecordedAt != at(12, 40) {
		t.Errorf("nearest slot: %+v", got)
	}

	got, _ = call[logDoseOutput](t, cs, "log_dose", map[string]any{"medication_id": m.ID, "status": "taken"})
	if got.ScheduledAt != at(8, 0) {
		t.Errorf("next nearest unrecorded slot = %s, want %s", got.ScheduledAt, at(8, 0))
	}

	got, res = call[logDoseOutput](t, cs, "log_dose", map[string]any{"medication_id": m.ID, "status": "skipped", "scheduled_at": at(8, 0), "note": "felt dizzy"})
	if res.IsError || got.Status != "skipped" || got.Note != "felt dizzy" {
		t.Errorf("relog: %+v %q", got, errorText(res))
	}

	sched, _ := call[getScheduleOutput](t, cs, "get_schedule", nil)
	if want := (scheduleSummary{Taken: 1, Skipped: 1, Upcoming: 1}); sched.Summary != want {
		t.Errorf("summary after logging = %+v, want %+v", sched.Summary, want)
	}

	for _, tt := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"medication_id": m.ID, "status": "taken", "scheduled_at": at(9, 0)}, "has no dose at"},
		{map[string]any{"medication_id": m.ID, "status": "taken", "scheduled_at": "8am"}, "RFC 3339"},
		{map[string]any{"medication_id": m.ID, "status": "missed", "scheduled_at": at(20, 30)}, "dose status must be"},
		{map[string]any{"medication_id": "med_missing", "status": "taken"}, "not found"},
	} {
		_, res := call[logDoseOutput](t, cs, "log_dose", tt.args)
		if !strings.Contains(errorText(res), tt.want) {
			t.Errorf("%v: error %q, want it to mention %q", tt.args, errorText(res), tt.want)
		}
	}

	call[logDoseOutput](t, cs, "log_dose", map[string]any{"medication_id": m.ID, "status": "taken"})
	_, res = call[logDoseOutput](t, cs, "log_dose", map[string]any{"medication_id": m.ID, "status": "taken"})
	if !strings.Contains(errorText(res), "already recorded") {
		t.Errorf("all recorded: %q", errorText(res))
	}
}

func TestRecordCheckIn(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	cs := f.connect(t, "")

	got, res := call[checkIn](t, cs, "record_checkin", map[string]any{"mood": "Okay", "note": "knee hurts a little"})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if got.Mood != "okay" || got.At != f.now.Format(time.RFC3339) {
		t.Errorf("check-in = %+v", got)
	}
	_, res = call[checkIn](t, cs, "record_checkin", map[string]any{"mood": "great"})
	if !strings.Contains(errorText(res), "mood must be") {
		t.Errorf("bad mood: %q", errorText(res))
	}
	if n := len(f.store.CheckIns(r.ID, f.day, f.day.AddDate(0, 0, 1))); n != 1 {
		t.Errorf("stored check-ins = %d, want 1", n)
	}
}

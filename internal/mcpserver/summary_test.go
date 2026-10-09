package mcpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

func TestAdherenceSummary(t *testing.T) {
	f := newFixture(t)
	r := f.addPerson(t, "Kamala")
	metformin := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Metformin", Times: clocks(t, "08:00", "20:00")})
	thyroxine := f.addMedication(t, care.Medication{RecipientID: r.ID, Name: "Thyroxine", Times: clocks(t, "07:00")})

	day1, day2, day3 := f.day, f.day.AddDate(0, 0, 1), f.day.AddDate(0, 0, 2)
	record := func(m care.Medication, at time.Time, status care.DoseStatus) {
		t.Helper()
		if _, err := f.store.RecordDose(care.DoseEvent{MedicationID: m.ID, Slot: at, Status: status, RecordedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	record(metformin, day1.Add(8*time.Hour), care.DoseTaken)
	record(metformin, day1.Add(20*time.Hour), care.DoseTaken)
	record(thyroxine, day1.Add(7*time.Hour), care.DoseTaken)
	record(metformin, day2.Add(8*time.Hour), care.DoseSkipped)
	record(thyroxine, day2.Add(7*time.Hour), care.DoseTaken)
	record(thyroxine, day3.Add(7*time.Hour), care.DoseTaken)

	for _, c := range []care.CheckIn{
		{RecipientID: r.ID, At: day1.Add(9 * time.Hour), Mood: care.MoodGood},
		{RecipientID: r.ID, At: day2.Add(9 * time.Hour), Mood: care.MoodPoor, Note: "did not sleep well"},
	} {
		if _, err := f.store.RecordCheckIn(c); err != nil {
			t.Fatal(err)
		}
	}

	f.now = day3.Add(8*time.Hour + 30*time.Minute)
	cs := f.connect(t, "")

	out, res := call[adherenceOutput](t, cs, "get_adherence_summary", map[string]any{"days": 3})
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if out.From != day1.Format(time.DateOnly) || out.To != day3.Format(time.DateOnly) {
		t.Errorf("range = %s..%s", out.From, out.To)
	}
	if out.Taken != 5 || out.Skipped != 1 || out.Missed != 1 || out.Percent == nil || *out.Percent != 71 {
		t.Errorf("totals = %+v percent=%v", out.doseCounts, out.Percent)
	}
	if len(out.Medications) != 2 || out.Medications[0].Medication != "Metformin" ||
		out.Medications[0].Taken != 2 || out.Medications[0].Skipped != 1 || out.Medications[0].Missed != 1 ||
		*out.Medications[1].Percent != 100 {
		t.Errorf("per medicine = %+v", out.Medications)
	}
	if out.Moods["good"] != 1 || out.Moods["poor"] != 1 || len(out.CheckIns) != 2 || out.CheckIns[1].Note != "did not sleep well" {
		t.Errorf("moods = %v check-ins = %+v", out.Moods, out.CheckIns)
	}

	out, _ = call[adherenceOutput](t, cs, "get_adherence_summary", map[string]any{"days": 1})
	if out.Taken != 1 || out.Missed != 0 || len(out.CheckIns) != 0 {
		t.Errorf("today only = %+v", out.doseCounts)
	}

	_, res = call[adherenceOutput](t, cs, "get_adherence_summary", map[string]any{"days": 91})
	if !strings.Contains(errorText(res), "from 1 to 90") {
		t.Errorf("days=91: %q", errorText(res))
	}
}

func TestAdherenceWithoutDosesHasNoPercent(t *testing.T) {
	f := newFixture(t)
	f.addPerson(t, "Kamala")
	cs := f.connect(t, "")

	out, res := call[adherenceOutput](t, cs, "get_adherence_summary", nil)
	if res.IsError {
		t.Fatal(errorText(res))
	}
	if out.Percent != nil || len(out.Medications) != 0 || out.Moods["okay"] != 0 {
		t.Errorf("empty summary = %+v", out)
	}
}

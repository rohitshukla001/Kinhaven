package care

import (
	"testing"
	"time"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func med(id, name string, times ...string) Medication {
	m := Medication{ID: id, Name: name}
	for _, s := range times {
		c, err := ParseClock(s)
		if err != nil {
			panic(err)
		}
		m.Times = append(m.Times, c)
	}
	return m
}

type want struct {
	name   string
	clock  string
	status SlotStatus
}

func checkSlots(t *testing.T, got []Slot, wants []want) {
	t.Helper()
	if len(got) != len(wants) {
		for _, s := range got {
			t.Logf("  %s %s %s", s.At.Format("Jan 2 15:04 MST"), s.Medication.Name, s.Status)
		}
		t.Fatalf("got %d slots, want %d", len(got), len(wants))
	}
	for i, w := range wants {
		s := got[i]
		if s.Medication.Name != w.name || s.At.Format("15:04") != w.clock || s.Status != w.status {
			t.Errorf("slot %d = %s %s %s, want %s %s %s",
				i, s.Medication.Name, s.At.Format("15:04"), s.Status, w.name, w.clock, w.status)
		}
	}
}

func TestDayStatuses(t *testing.T) {
	ist := mustLoad(t, "Asia/Kolkata")
	sched := Schedule{Location: ist, Grace: time.Hour}
	day := time.Date(2026, 10, 5, 0, 0, 0, 0, ist)
	at := func(h, m int) time.Time { return time.Date(2026, 10, 5, h, m, 0, 0, ist) }

	meds := []Medication{
		med("m1", "Metformin", "08:00", "20:30"),
		med("m2", "Thyroxine", "07:00"),
		med("m3", "Amlodipine", "09:30"),
		med("m4", "Calcium", "08:00"),
	}
	doses := []DoseEvent{
		{MedicationID: "m1", Slot: at(8, 0).UTC(), Status: DoseTaken},
		{MedicationID: "m4", Slot: at(8, 0), Status: DoseSkipped},
	}

	got := sched.Day(meds, doses, day, at(10, 0))
	checkSlots(t, got, []want{
		{"Thyroxine", "07:00", SlotMissed},
		{"Calcium", "08:00", SlotSkipped},
		{"Metformin", "08:00", SlotTaken},
		{"Amlodipine", "09:30", SlotDue},
		{"Metformin", "20:30", SlotUpcoming},
	})
	if got[2].Dose == nil || got[2].Dose.Status != DoseTaken {
		t.Errorf("taken slot has no dose attached: %+v", got[2])
	}
}

func TestGraceBoundaries(t *testing.T) {
	sched := Schedule{Location: time.UTC, Grace: 30 * time.Minute}
	slotAt := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	meds := []Medication{med("m1", "Metformin", "08:00")}

	for _, tt := range []struct {
		now  time.Time
		want SlotStatus
	}{
		{slotAt.Add(-time.Second), SlotUpcoming},
		{slotAt, SlotDue},
		{slotAt.Add(29 * time.Minute), SlotDue},
		{slotAt.Add(30 * time.Minute), SlotMissed},
	} {
		got := sched.Day(meds, nil, slotAt, tt.now)
		if len(got) != 1 || got[0].Status != tt.want {
			t.Errorf("now=%s: got %+v, want %s", tt.now.Format("15:04:05"), got, tt.want)
		}
	}
}

func TestWeeklyMedicationOnlyOnItsDays(t *testing.T) {
	sched := Schedule{Location: time.UTC, Grace: time.Hour}
	vitD := med("m1", "Vitamin D", "09:00")
	vitD.Days = []time.Weekday{time.Sunday}
	meds := []Medication{vitD}

	sunday := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	monday := sunday.AddDate(0, 0, 1)
	if got := sched.Day(meds, nil, sunday, monday); len(got) != 1 {
		t.Errorf("Sunday slots = %d, want 1", len(got))
	}
	if got := sched.Day(meds, nil, monday, monday); len(got) != 0 {
		t.Errorf("Monday slots = %d, want 0", len(got))
	}
}

func TestStoppedAndNewMedications(t *testing.T) {
	sched := Schedule{Location: time.UTC, Grace: time.Hour}
	day := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

	stopped := med("m1", "Aspirin", "08:00")
	stopped.Stopped = true
	added := med("m2", "Antibiotic", "08:00", "14:00", "20:00")
	added.CreatedAt = day.Add(13 * time.Hour)

	got := sched.Day([]Medication{stopped, added}, nil, day, day.Add(15*time.Hour))
	checkSlots(t, got, []want{
		{"Antibiotic", "14:00", SlotMissed},
		{"Antibiotic", "20:00", SlotUpcoming},
	})
}

func TestSlotsAcrossDaysIsHalfOpen(t *testing.T) {
	ist := mustLoad(t, "Asia/Kolkata")
	sched := Schedule{Location: ist, Grace: time.Hour}
	meds := []Medication{med("m1", "Metformin", "08:00", "20:00")}

	from := time.Date(2026, 10, 5, 8, 0, 0, 0, ist)
	to := time.Date(2026, 10, 8, 8, 0, 0, 0, ist)
	got := sched.Slots(meds, nil, from, to, to)
	if len(got) != 6 {
		t.Fatalf("got %d slots, want 6", len(got))
	}
	if !got[0].At.Equal(from) || got[5].At.Format("Jan 2 15:04") != "Oct 7 20:00" {
		t.Errorf("first %s, last %s", got[0].At, got[5].At)
	}
}

func TestDayFollowsRecipientTimezoneNotCaller(t *testing.T) {
	ist := mustLoad(t, "Asia/Kolkata")
	sched := Schedule{Location: ist, Grace: time.Hour}
	meds := []Medication{med("m1", "Metformin", "00:30", "23:30")}

	utcEvening := time.Date(2026, 10, 4, 20, 0, 0, 0, time.UTC)
	got := sched.Day(meds, nil, utcEvening, utcEvening)
	if len(got) != 2 || got[0].At.Format("2006-01-02 15:04") != "2026-10-05 00:30" {
		t.Errorf("got %+v", got)
	}
}

func TestDaylightSavingDays(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	sched := Schedule{Location: ny, Grace: time.Hour}
	meds := []Medication{med("m1", "Metformin", "01:30", "02:30", "09:00")}

	for _, day := range []time.Time{
		time.Date(2026, 3, 8, 12, 0, 0, 0, ny),
		time.Date(2026, 11, 1, 12, 0, 0, 0, ny),
	} {
		got := sched.Day(meds, nil, day, day)
		if len(got) != 3 {
			t.Errorf("%s: got %d slots, want 3", day.Format("Jan 2"), len(got))
			continue
		}
		for _, s := range got {
			if s.At.Day() != day.Day() {
				t.Errorf("%s: slot %s falls on another day", day.Format("Jan 2"), s.At)
			}
		}
	}
}

func TestRecipientLocation(t *testing.T) {
	loc, err := Recipient{Timezone: "Asia/Kolkata"}.Location()
	if err != nil || loc.String() != "Asia/Kolkata" {
		t.Errorf("got %v, %v", loc, err)
	}
	if _, err := (Recipient{Timezone: "Mars/Base"}).Location(); err == nil {
		t.Error("want an error for an unknown timezone")
	}
}

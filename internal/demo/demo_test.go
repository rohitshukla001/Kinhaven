package demo

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rohitshukla001/Kinhaven/internal/alert"
	"github.com/rohitshukla001/Kinhaven/internal/care"
	"github.com/rohitshukla001/Kinhaven/internal/store"
)

const grace = time.Hour

var kolkata = mustLoad("Asia/Kolkata")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func seeded(t *testing.T, now time.Time) (*store.Store, Result) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Seed(st, kolkata, grace, now)
	if err != nil {
		t.Fatal(err)
	}
	return st, res
}

func TestSeedBuildsAWeekOfHistory(t *testing.T) {
	now := time.Date(2026, 10, 16, 11, 30, 0, 0, kolkata)
	st, res := seeded(t, now)

	want := Result{Recipient: res.Recipient, Contacts: 2, Medications: 5, Doses: 35, CheckIns: 7, Alerts: 3}
	if res != want {
		t.Errorf("result = %+v, want %+v", res, want)
	}

	sched := care.Schedule{Location: kolkata, Grace: grace}
	today, _ := care.DayBounds(now, kolkata)
	since := today.AddDate(0, 0, -historyDays)
	meds := st.Medications(res.Recipient.ID)
	doses := st.Doses(res.Recipient.ID, since, now)

	counts := map[care.SlotStatus]int{}
	for _, s := range sched.Slots(meds, doses, since, today, now) {
		counts[s.Status]++
	}
	if counts[care.SlotTaken] != 31 || counts[care.SlotSkipped] != 2 || counts[care.SlotMissed] != 3 || len(counts) != 3 {
		t.Errorf("past week = %v", counts)
	}

	var got []string
	for _, s := range sched.Day(meds, doses, now, now) {
		got = append(got, s.Medication.Name+" "+s.At.Format("15:04")+" "+string(s.Status))
	}
	wantToday := "Thyroxine 07:00 taken, Metformin 08:00 taken, Amlodipine 09:00 missed, Metformin 20:00 upcoming, Atorvastatin 21:00 upcoming"
	if strings.Join(got, ", ") != wantToday {
		t.Errorf("today = %s", strings.Join(got, ", "))
	}

	if c := st.CheckIns(res.Recipient.ID, since, today); len(c) != 7 || c[2].Mood != care.MoodPoor {
		t.Errorf("check-ins = %+v", c)
	}
}

type recorder struct{ messages []string }

func (r *recorder) Send(_ context.Context, phone, text string) error {
	r.messages = append(r.messages, phone+": "+text)
	return nil
}

func TestSeedLeavesTheLatestMissedDoseForTheWatcher(t *testing.T) {
	now := time.Date(2026, 10, 16, 11, 30, 0, 0, kolkata)
	st, _ := seeded(t, now)

	sms := &recorder{}
	w := &alert.Watcher{Store: st, Sender: sms, Grace: grace, Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.Check(context.Background())
	w.Check(context.Background())

	want := "+919876543210: Kinhaven: Kamala has not taken Amlodipine 5 mg (09:00). Please check in with them."
	if len(sms.messages) != 1 || sms.messages[0] != want {
		t.Errorf("messages = %q", sms.messages)
	}
}

func TestSeedEarlyInTheDayRecordsNothingToday(t *testing.T) {
	now := time.Date(2026, 10, 18, 6, 0, 0, 0, kolkata)
	st, res := seeded(t, now)
	today, _ := care.DayBounds(now, kolkata)
	if d := st.Doses(res.Recipient.ID, today, now); len(d) != 0 {
		t.Errorf("doses today = %+v", d)
	}
	if res.Doses != 33 {
		t.Errorf("doses = %d, want 33", res.Doses)
	}
}

func TestSeedRefusesExistingData(t *testing.T) {
	now := time.Date(2026, 10, 16, 11, 30, 0, 0, kolkata)
	st, _ := seeded(t, now)
	if _, err := Seed(st, kolkata, grace, now); !errors.Is(err, ErrNotEmpty) {
		t.Errorf("err = %v, want ErrNotEmpty", err)
	}
	if n := len(st.Recipients()); n != 1 {
		t.Errorf("recipients = %d", n)
	}
}

func TestSeedLateEveningStillAlertsTheFamily(t *testing.T) {
	now := time.Date(2026, 10, 16, 21, 30, 0, 0, kolkata)
	st, _ := seeded(t, now)
	sms := &recorder{}
	w := &alert.Watcher{Store: st, Sender: sms, Grace: grace, Now: func() time.Time { return now }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w.Check(context.Background())
	if len(sms.messages) != 1 || !strings.Contains(sms.messages[0], "Metformin 500 mg (20:00)") {
		t.Errorf("messages = %q", sms.messages)
	}
}

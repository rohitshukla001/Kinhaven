package store

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

var (
	morning = care.ClockTime{Hour: 8}
	evening = care.ClockTime{Hour: 20, Minute: 30}
	slot    = time.Date(2026, 10, 5, 2, 30, 0, 0, time.UTC)
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data", "kinhaven.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func seed(t *testing.T, s *Store) (care.Recipient, care.Medication) {
	t.Helper()
	r, err := s.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "Asia/Kolkata"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.AddMedication(care.Medication{
		RecipientID: r.ID,
		Name:        "Metformin",
		Dosage:      "500 mg",
		Times:       []care.ClockTime{morning, evening},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r, m
}

func TestOpenMissingFileStartsEmpty(t *testing.T) {
	s, path := openTemp(t)
	if len(s.Recipients()) != 0 {
		t.Error("want no recipients")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open created the data file early: %v", err)
	}
}

func TestDataSurvivesReopen(t *testing.T) {
	s, path := openTemp(t)
	r, m := seed(t, s)
	if _, err := s.AddContact(care.Contact{RecipientID: r.ID, Name: "Ravi", Phone: "+919876543210", NotifyMissedDose: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordDose(care.DoseEvent{MedicationID: m.ID, Slot: slot, Status: care.DoseTaken}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RecordCheckIn(care.CheckIn{RecipientID: r.ID, At: slot, Mood: care.MoodGood}); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.Medication(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Metformin" || len(got.Times) != 2 || got.Times[1] != evening || got.CreatedAt.IsZero() {
		t.Errorf("medication = %+v", got)
	}
	if c := reopened.Contacts(r.ID); len(c) != 1 || !c[0].NotifyMissedDose {
		t.Errorf("contacts = %+v", c)
	}
	day := slot.Truncate(24 * time.Hour)
	if d := reopened.Doses(r.ID, day, day.Add(24*time.Hour)); len(d) != 1 || d[0].RecipientID != r.ID {
		t.Errorf("doses = %+v", d)
	}
	if c := reopened.CheckIns(r.ID, day, day.Add(24*time.Hour)); len(c) != 1 {
		t.Errorf("check-ins = %+v", c)
	}
}

func TestDataFileIsPrivate(t *testing.T) {
	s, path := openTemp(t)
	seed(t, s)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file mode = %v, want 0600", perm)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".kinhaven-*.tmp"))
	if len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}
}

func TestInvalidInputIsRejected(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.AddRecipient(care.Recipient{Name: "Kamala"}); !errors.Is(err, care.ErrInvalid) {
		t.Errorf("AddRecipient err = %v", err)
	}
	if _, err := s.AddMedication(care.Medication{RecipientID: "rcp_x", Name: "Metformin"}); !errors.Is(err, care.ErrInvalid) {
		t.Errorf("AddMedication err = %v", err)
	}
}

func TestUnknownReferencesAreNotFound(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.AddContact(care.Contact{RecipientID: "rcp_missing", Name: "Ravi", Phone: "+919876543210"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("AddContact err = %v", err)
	}
	if _, err := s.AddMedication(care.Medication{RecipientID: "rcp_missing", Name: "Metformin", Times: []care.ClockTime{morning}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("AddMedication err = %v", err)
	}
	if _, err := s.RecordDose(care.DoseEvent{MedicationID: "med_missing", Slot: slot, Status: care.DoseTaken}); !errors.Is(err, ErrNotFound) {
		t.Errorf("RecordDose err = %v", err)
	}
	if err := s.StopMedication("med_missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("StopMedication err = %v", err)
	}
	if _, err := s.Recipient("rcp_missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Recipient err = %v", err)
	}
}

func TestRecordDoseReplacesSameSlot(t *testing.T) {
	s, _ := openTemp(t)
	r, m := seed(t, s)

	first, err := s.RecordDose(care.DoseEvent{MedicationID: m.ID, Slot: slot, Status: care.DoseSkipped})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.RecordDose(care.DoseEvent{MedicationID: m.ID, Slot: slot.In(time.FixedZone("IST", 19800)), Status: care.DoseTaken})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Errorf("ids differ: %s vs %s", first.ID, second.ID)
	}
	doses := s.Doses(r.ID, slot, slot.Add(time.Minute))
	if len(doses) != 1 || doses[0].Status != care.DoseTaken {
		t.Errorf("doses = %+v", doses)
	}
}

func TestDosesRangeIsHalfOpen(t *testing.T) {
	s, _ := openTemp(t)
	r, m := seed(t, s)
	for _, at := range []time.Time{slot, slot.Add(12 * time.Hour)} {
		if _, err := s.RecordDose(care.DoseEvent{MedicationID: m.ID, Slot: at, Status: care.DoseTaken}); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Doses(r.ID, slot, slot.Add(12*time.Hour)); len(got) != 1 || !got[0].Slot.Equal(slot) {
		t.Errorf("doses = %+v", got)
	}
	if got := s.Doses("rcp_other", slot, slot.Add(24*time.Hour)); len(got) != 0 {
		t.Errorf("doses for another recipient = %+v", got)
	}
}

func TestStopMedication(t *testing.T) {
	s, path := openTemp(t)
	_, m := seed(t, s)
	if err := s.StopMedication(m.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := reopened.Medication(m.ID); !got.Stopped {
		t.Error("medication is not stopped after reopen")
	}
}

func TestCallersCannotMutateStoredMedication(t *testing.T) {
	s, _ := openTemp(t)
	times := []care.ClockTime{morning}
	r, err := s.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.AddMedication(care.Medication{RecipientID: r.ID, Name: "Aspirin", Times: times})
	if err != nil {
		t.Fatal(err)
	}

	times[0] = evening
	m.Times[0] = evening
	s.Medications(r.ID)[0].Times[0] = evening

	if got, _ := s.Medication(m.ID); got.Times[0] != morning {
		t.Errorf("stored time changed to %s", got.Times[0])
	}
}

func TestFailedSaveKeepsPreviousState(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	s, path := openTemp(t)
	r, _ := seed(t, s)

	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })

	if _, err := s.AddRecipient(care.Recipient{Name: "Gopal", Timezone: "UTC"}); err == nil {
		t.Fatal("want a save error")
	}
	if got := s.Recipients(); len(got) != 1 || got[0].ID != r.ID {
		t.Errorf("recipients after failed save = %+v", got)
	}
}

func TestCorruptFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kinhaven.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Error("want an error for a corrupt file")
	}
}

func TestConcurrentWrites(t *testing.T) {
	s, path := openTemp(t)
	r, _ := seed(t, s)

	const n = 40
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			at := slot.Add(time.Duration(i) * time.Minute)
			if _, err := s.RecordCheckIn(care.CheckIn{RecipientID: r.ID, At: at, Mood: care.MoodOkay}); err != nil {
				t.Error(err)
			}
			s.CheckIns(r.ID, slot, slot.Add(time.Hour))
		})
	}
	wg.Wait()

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.CheckIns(r.ID, slot, slot.Add(time.Hour)); len(got) != n {
		t.Errorf("check-ins after reopen = %d, want %d", len(got), n)
	}
}

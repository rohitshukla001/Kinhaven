package care

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func mustClock(t *testing.T, s string) ClockTime {
	t.Helper()
	c, err := ParseClock(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestParseClock(t *testing.T) {
	c := mustClock(t, "07:05")
	if c != (ClockTime{Hour: 7, Minute: 5}) || c.String() != "07:05" {
		t.Errorf("got %+v (%s)", c, c)
	}
	for _, bad := range []string{"7am", "24:00", "12:60", "", "8:00:00"} {
		if _, err := ParseClock(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseClock(%q) err = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestClockTimeJSON(t *testing.T) {
	in := []ClockTime{{8, 0}, {20, 30}}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `["08:00","20:30"]` {
		t.Errorf("marshal = %s", b)
	}
	var out []ClockTime
	if err := json.Unmarshal(b, &out); err != nil || len(out) != 2 || out[1] != in[1] {
		t.Errorf("round trip = %v, %v", out, err)
	}
	if err := json.Unmarshal([]byte(`["25:00"]`), &out); !errors.Is(err, ErrInvalid) {
		t.Errorf("unmarshal bad time err = %v", err)
	}
}

func TestValidate(t *testing.T) {
	at := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	morning := mustClock(t, "08:00")

	tests := []struct {
		name  string
		value interface{ Validate() error }
		ok    bool
	}{
		{"recipient", Recipient{Name: "Kamala", Timezone: "Asia/Kolkata"}, true},
		{"recipient without name", Recipient{Name: " ", Timezone: "UTC"}, false},
		{"recipient without timezone", Recipient{Name: "Kamala"}, false},
		{"recipient with bad timezone", Recipient{Name: "Kamala", Timezone: "Mars/Base"}, false},

		{"contact", Contact{Name: "Ravi", Phone: "+919876543210"}, true},
		{"contact without name", Contact{Phone: "+919876543210"}, false},
		{"contact with local phone", Contact{Name: "Ravi", Phone: "09876543210"}, false},

		{"medication", Medication{Name: "Metformin", Times: []ClockTime{morning}}, true},
		{"weekly medication", Medication{Name: "Vitamin D", Times: []ClockTime{morning}, Days: []time.Weekday{time.Sunday}}, true},
		{"medication without times", Medication{Name: "Metformin"}, false},
		{"medication with repeated time", Medication{Name: "Metformin", Times: []ClockTime{morning, morning}}, false},
		{"medication with bad day", Medication{Name: "Metformin", Times: []ClockTime{morning}, Days: []time.Weekday{7}}, false},
		{"medication with repeated day", Medication{Name: "Metformin", Times: []ClockTime{morning}, Days: []time.Weekday{1, 1}}, false},

		{"dose", DoseEvent{MedicationID: "med_1", Slot: at, Status: DoseTaken}, true},
		{"dose without slot", DoseEvent{MedicationID: "med_1", Status: DoseTaken}, false},
		{"dose with unknown status", DoseEvent{MedicationID: "med_1", Slot: at, Status: "missed"}, false},

		{"check-in", CheckIn{At: at, Mood: MoodOkay}, true},
		{"check-in without mood", CheckIn{At: at}, false},
	}
	for _, tt := range tests {
		err := tt.value.Validate()
		if tt.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tt.name, err)
		}
		if !tt.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", tt.name, err)
		}
	}
}

func TestParseWeekday(t *testing.T) {
	for in, want := range map[string]time.Weekday{"sunday": time.Sunday, "Mon": time.Monday, " SATURDAY ": time.Saturday} {
		if got, err := ParseWeekday(in); err != nil || got != want {
			t.Errorf("ParseWeekday(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "su", "funday"} {
		if _, err := ParseWeekday(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseWeekday(%q) err = %v", bad, err)
		}
	}
}

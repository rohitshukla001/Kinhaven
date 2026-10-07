package care

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid input")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

type Recipient struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
}

func (r Recipient) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return invalid("recipient name is required")
	}
	if r.Timezone == "" {
		return invalid("recipient timezone is required")
	}
	_, err := r.Location()
	return err
}

var e164 = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

type Contact struct {
	ID               string `json:"id"`
	RecipientID      string `json:"recipient_id"`
	Name             string `json:"name"`
	Relation         string `json:"relation,omitempty"`
	Phone            string `json:"phone"`
	NotifyMissedDose bool   `json:"notify_missed_dose"`
}

func (c Contact) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return invalid("contact name is required")
	}
	if !e164.MatchString(c.Phone) {
		return invalid("phone %q must be in E.164 format, for example +919876543210", c.Phone)
	}
	return nil
}

type Medication struct {
	ID           string         `json:"id"`
	RecipientID  string         `json:"recipient_id"`
	Name         string         `json:"name"`
	Dosage       string         `json:"dosage,omitempty"`
	Instructions string         `json:"instructions,omitempty"`
	Times        []ClockTime    `json:"times"`
	Days         []time.Weekday `json:"days,omitempty"`
	Stopped      bool           `json:"stopped,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

func (m Medication) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return invalid("medication name is required")
	}
	if len(m.Times) == 0 {
		return invalid("medication %q needs at least one dose time", m.Name)
	}
	seen := make(map[ClockTime]bool, len(m.Times))
	for _, t := range m.Times {
		if seen[t] {
			return invalid("dose time %s is listed twice", t)
		}
		seen[t] = true
	}
	seenDay := make(map[time.Weekday]bool, len(m.Days))
	for _, d := range m.Days {
		if d < time.Sunday || d > time.Saturday {
			return invalid("day %d is not a weekday number from 0 (Sunday) to 6", d)
		}
		if seenDay[d] {
			return invalid("%s is listed twice", d)
		}
		seenDay[d] = true
	}
	return nil
}

type DoseStatus string

const (
	DoseTaken   DoseStatus = "taken"
	DoseSkipped DoseStatus = "skipped"
)

type DoseEvent struct {
	ID           string     `json:"id"`
	RecipientID  string     `json:"recipient_id"`
	MedicationID string     `json:"medication_id"`
	Slot         time.Time  `json:"slot"`
	Status       DoseStatus `json:"status"`
	RecordedAt   time.Time  `json:"recorded_at"`
	Note         string     `json:"note,omitempty"`
}

func (d DoseEvent) Validate() error {
	if d.MedicationID == "" {
		return invalid("medication id is required")
	}
	if d.Slot.IsZero() {
		return invalid("dose slot time is required")
	}
	if d.Status != DoseTaken && d.Status != DoseSkipped {
		return invalid("dose status must be %q or %q", DoseTaken, DoseSkipped)
	}
	return nil
}

type Mood string

const (
	MoodGood Mood = "good"
	MoodOkay Mood = "okay"
	MoodPoor Mood = "poor"
)

type CheckIn struct {
	ID          string    `json:"id"`
	RecipientID string    `json:"recipient_id"`
	At          time.Time `json:"at"`
	Mood        Mood      `json:"mood"`
	Note        string    `json:"note,omitempty"`
}

func (c CheckIn) Validate() error {
	if c.At.IsZero() {
		return invalid("check-in time is required")
	}
	switch c.Mood {
	case MoodGood, MoodOkay, MoodPoor:
		return nil
	}
	return invalid("mood must be %q, %q or %q", MoodGood, MoodOkay, MoodPoor)
}

package care

import (
	"cmp"
	"slices"
	"time"
)

type SlotStatus string

const (
	SlotUpcoming SlotStatus = "upcoming"
	SlotDue      SlotStatus = "due"
	SlotTaken    SlotStatus = "taken"
	SlotSkipped  SlotStatus = "skipped"
	SlotMissed   SlotStatus = "missed"
)

type Slot struct {
	Medication Medication
	At         time.Time
	Status     SlotStatus
	Dose       *DoseEvent
}

type Schedule struct {
	Location *time.Location
	Grace    time.Duration
}

func (s Schedule) Day(meds []Medication, doses []DoseEvent, day, now time.Time) []Slot {
	start, end := DayBounds(day, s.Location)
	return s.Slots(meds, doses, start, end, now)
}

func DayBounds(t time.Time, loc *time.Location) (start, end time.Time) {
	start = startOfDay(t.In(loc))
	return start, start.AddDate(0, 0, 1)
}

func (s Schedule) Slots(meds []Medication, doses []DoseEvent, from, to, now time.Time) []Slot {
	recorded := make(map[slotKey]DoseEvent, len(doses))
	for _, d := range doses {
		recorded[slotKey{d.MedicationID, d.Slot.Unix()}] = d
	}

	var slots []Slot
	for day := startOfDay(from.In(s.Location)); day.Before(to); day = day.AddDate(0, 0, 1) {
		for _, m := range meds {
			if m.Stopped || !takenOn(m, day.Weekday()) {
				continue
			}
			for _, t := range m.Times {
				at := time.Date(day.Year(), day.Month(), day.Day(), t.Hour, t.Minute, 0, 0, s.Location)
				if at.Before(from) || !at.Before(to) || at.Before(m.CreatedAt) {
					continue
				}
				slot := Slot{Medication: m, At: at}
				if d, ok := recorded[slotKey{m.ID, at.Unix()}]; ok {
					slot.Dose = &d
				}
				slot.Status = s.status(slot, now)
				slots = append(slots, slot)
			}
		}
	}

	slices.SortStableFunc(slots, func(a, b Slot) int {
		return cmp.Or(a.At.Compare(b.At), cmp.Compare(a.Medication.Name, b.Medication.Name))
	})
	return slots
}

func (s Schedule) status(slot Slot, now time.Time) SlotStatus {
	switch {
	case slot.Dose != nil && slot.Dose.Status == DoseTaken:
		return SlotTaken
	case slot.Dose != nil:
		return SlotSkipped
	case now.Before(slot.At):
		return SlotUpcoming
	case now.Before(slot.At.Add(s.Grace)):
		return SlotDue
	default:
		return SlotMissed
	}
}

type slotKey struct {
	medicationID string
	unix         int64
}

func takenOn(m Medication, wd time.Weekday) bool {
	return len(m.Days) == 0 || slices.Contains(m.Days, wd)
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (r Recipient) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(r.Timezone)
	if err != nil {
		return nil, invalid("unknown timezone %q", r.Timezone)
	}
	return loc, nil
}

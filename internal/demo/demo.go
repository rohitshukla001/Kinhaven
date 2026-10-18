package demo

import (
	"errors"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

const historyDays = 7

var ErrNotEmpty = errors.New("the data store already has people in it")

type Result struct {
	Recipient   care.Recipient
	Contacts    int
	Medications int
	Doses       int
	CheckIns    int
	Alerts      int
}

type outcome struct {
	status care.DoseStatus
	note   string
}

type slotRef struct {
	daysAgo    int
	medication string
	at         string
}

var missed = outcome{}

var exceptions = map[slotRef]outcome{
	{6, "Metformin", "20:00"}:    missed,
	{5, "Amlodipine", "09:00"}:   {care.DoseSkipped, "Felt dizzy in the morning"},
	{4, "Thyroxine", "07:00"}:    missed,
	{3, "Atorvastatin", "21:00"}: missed,
	{2, "Metformin", "08:00"}:    {care.DoseSkipped, "Skipped breakfast"},
}

var checkIns = []struct {
	mood care.Mood
	note string
}{
	{care.MoodGood, "Walked in the garden with Priya"},
	{care.MoodOkay, "Slept badly"},
	{care.MoodPoor, "Felt dizzy in the morning"},
	{care.MoodOkay, ""},
	{care.MoodGood, "Video call with the grandchildren"},
	{care.MoodOkay, "Knees ached after the stairs"},
	{care.MoodGood, "Cooked lunch for the family"},
}

func medications(recipientID string, since time.Time) []care.Medication {
	med := func(name, dosage, instructions string, days []time.Weekday, times ...care.ClockTime) care.Medication {
		return care.Medication{
			RecipientID:  recipientID,
			Name:         name,
			Dosage:       dosage,
			Instructions: instructions,
			Times:        times,
			Days:         days,
			CreatedAt:    since,
		}
	}
	return []care.Medication{
		med("Thyroxine", "50 mcg", "Take on an empty stomach", nil, care.ClockTime{Hour: 7}),
		med("Metformin", "500 mg", "Take with food", nil, care.ClockTime{Hour: 8}, care.ClockTime{Hour: 20}),
		med("Amlodipine", "5 mg", "", nil, care.ClockTime{Hour: 9}),
		med("Vitamin D3", "60000 IU", "Take with milk", []time.Weekday{time.Sunday}, care.ClockTime{Hour: 10}),
		med("Atorvastatin", "10 mg", "Take at bedtime", nil, care.ClockTime{Hour: 21}),
	}
}

func Seed(st *store.Store, loc *time.Location, grace time.Duration, now time.Time) (Result, error) {
	if len(st.Recipients()) > 0 {
		return Result{}, ErrNotEmpty
	}

	var res Result
	r, err := st.AddRecipient(care.Recipient{Name: "Kamala", Timezone: loc.String()})
	if err != nil {
		return Result{}, err
	}
	res.Recipient = r

	contacts := []care.Contact{
		{RecipientID: r.ID, Name: "Priya", Relation: "daughter", Phone: "+919876543210", NotifyMissedDose: true},
		{RecipientID: r.ID, Name: "Arjun", Relation: "son", Phone: "+919812345678"},
	}
	var notify []care.Contact
	for _, c := range contacts {
		c, err := st.AddContact(c)
		if err != nil {
			return Result{}, err
		}
		if c.NotifyMissedDose {
			notify = append(notify, c)
		}
		res.Contacts++
	}

	today, _ := care.DayBounds(now, loc)
	since := today.AddDate(0, 0, -historyDays)
	var meds []care.Medication
	for _, m := range medications(r.ID, since) {
		m, err := st.AddMedication(m)
		if err != nil {
			return Result{}, err
		}
		meds = append(meds, m)
		res.Medications++
	}

	sched := care.Schedule{Location: loc, Grace: grace}
	var alerts []care.Alert
	slots := sched.Slots(meds, nil, since, now, now)
	open := lastMissedToday(slots, today)
	for i, s := range slots {
		o, recorded := planDose(s, today, i == open)
		if !recorded {
			if s.At.Before(today) {
				for _, c := range notify {
					alerts = append(alerts, care.Alert{RecipientID: r.ID, MedicationID: s.Medication.ID, Slot: s.At, ContactID: c.ID, SentAt: s.At.Add(grace + time.Minute)})
				}
			}
			continue
		}
		dose := care.DoseEvent{
			MedicationID: s.Medication.ID,
			Slot:         s.At,
			Status:       o.status,
			RecordedAt:   s.At.Add(time.Duration(2+i*7%25) * time.Minute),
			Note:         o.note,
		}
		if _, err := st.RecordDose(dose); err != nil {
			return Result{}, err
		}
		res.Doses++
	}
	if len(alerts) > 0 {
		if err := st.RecordAlerts(alerts); err != nil {
			return Result{}, err
		}
		res.Alerts = len(alerts)
	}

	for i, c := range checkIns {
		day := today.AddDate(0, 0, i-historyDays)
		at := time.Date(day.Year(), day.Month(), day.Day(), 19, 0, 0, 0, loc)
		if _, err := st.RecordCheckIn(care.CheckIn{RecipientID: r.ID, At: at, Mood: c.mood, Note: c.note}); err != nil {
			return Result{}, err
		}
		res.CheckIns++
	}
	return res, nil
}

func lastMissedToday(slots []care.Slot, today time.Time) int {
	open := -1
	for i, s := range slots {
		if !s.At.Before(today) && s.Status == care.SlotMissed {
			open = i
		}
	}
	return open
}

func planDose(s care.Slot, today time.Time, open bool) (outcome, bool) {
	taken := outcome{status: care.DoseTaken}
	if !s.At.Before(today) {
		return taken, s.Status == care.SlotMissed && !open
	}
	daysAgo := int(today.Sub(s.At).Hours()/24) + 1
	o, ok := exceptions[slotRef{daysAgo, s.Medication.Name, s.At.Format("15:04")}]
	if !ok {
		return taken, true
	}
	return o, o != missed
}

package alert

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rohitshukla001/Kinhaven/internal/care"
	"github.com/rohitshukla001/Kinhaven/internal/store"
)

const lookback = 12 * time.Hour

type Watcher struct {
	Store  *store.Store
	Sender Sender
	Grace  time.Duration
	Now    func() time.Time
	Logger *slog.Logger
}

func (w *Watcher) Run(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		w.Check(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Watcher) Check(ctx context.Context) {
	now := w.Now()
	for _, r := range w.Store.Recipients() {
		if err := w.checkRecipient(ctx, r, now); err != nil {
			w.Logger.Warn("missed-dose alert failed", "recipient", r.ID, "err", err)
		}
	}
}

type alertKey struct {
	contactID    string
	medicationID string
	slot         int64
}

func (w *Watcher) checkRecipient(ctx context.Context, r care.Recipient, now time.Time) error {
	var contacts []care.Contact
	for _, c := range w.Store.Contacts(r.ID) {
		if c.NotifyMissedDose {
			contacts = append(contacts, c)
		}
	}
	if len(contacts) == 0 {
		return nil
	}

	loc, err := r.Location()
	if err != nil {
		return err
	}
	from := now.Add(-lookback)
	sched := care.Schedule{Location: loc, Grace: w.Grace}
	var missed []care.Slot
	for _, s := range sched.Slots(w.Store.Medications(r.ID), w.Store.Doses(r.ID, from, now), from, now, now) {
		if s.Status == care.SlotMissed {
			missed = append(missed, s)
		}
	}
	if len(missed) == 0 {
		return nil
	}

	sent := map[alertKey]bool{}
	for _, a := range w.Store.Alerts(r.ID, from, now) {
		sent[alertKey{a.ContactID, a.MedicationID, a.Slot.Unix()}] = true
	}

	var errs []error
	for _, c := range contacts {
		var pending []care.Slot
		for _, s := range missed {
			if !sent[alertKey{c.ID, s.Medication.ID, s.At.Unix()}] {
				pending = append(pending, s)
			}
		}
		if len(pending) == 0 {
			continue
		}
		if err := w.Sender.Send(ctx, c.Phone, message(r.Name, pending, now.In(loc))); err != nil {
			errs = append(errs, fmt.Errorf("notify %s: %w", c.ID, err))
			continue
		}
		alerts := make([]care.Alert, len(pending))
		for i, s := range pending {
			alerts[i] = care.Alert{RecipientID: r.ID, MedicationID: s.Medication.ID, Slot: s.At, ContactID: c.ID, SentAt: now}
		}
		if err := w.Store.RecordAlerts(alerts); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func message(name string, slots []care.Slot, now time.Time) string {
	doses := make([]string, len(slots))
	for i, s := range slots {
		med := s.Medication.Name
		if s.Medication.Dosage != "" {
			med += " " + s.Medication.Dosage
		}
		layout := "15:04"
		if s.At.YearDay() != now.YearDay() || s.At.Year() != now.Year() {
			layout = "Jan 2 15:04"
		}
		doses[i] = fmt.Sprintf("%s (%s)", med, s.At.Format(layout))
	}
	return fmt.Sprintf("Kinhaven: %s has not taken %s. Please check in with them.", name, joinList(doses))
}

func joinList(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

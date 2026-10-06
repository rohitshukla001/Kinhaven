package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
)

var ErrNotFound = errors.New("not found")

type snapshot struct {
	Recipients  []care.Recipient  `json:"recipients"`
	Contacts    []care.Contact    `json:"contacts"`
	Medications []care.Medication `json:"medications"`
	Doses       []care.DoseEvent  `json:"doses"`
	CheckIns    []care.CheckIn    `json:"check_ins"`
}

func (s snapshot) clone() snapshot {
	return snapshot{
		Recipients:  slices.Clone(s.Recipients),
		Contacts:    slices.Clone(s.Contacts),
		Medications: slices.Clone(s.Medications),
		Doses:       slices.Clone(s.Doses),
		CheckIns:    slices.Clone(s.CheckIns),
	}
}

type Store struct {
	path string

	mu   sync.RWMutex
	data snapshot
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	s := &Store{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) update(apply func(*snapshot) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.data.clone()
	if err := apply(&next); err != nil {
		return err
	}
	if err := writeAtomic(s.path, next); err != nil {
		return fmt.Errorf("save %s: %w", s.path, err)
	}
	s.data = next
	return nil
}

func writeAtomic(path string, data snapshot) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kinhaven-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func newID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func (s *Store) AddRecipient(r care.Recipient) (care.Recipient, error) {
	if err := r.Validate(); err != nil {
		return care.Recipient{}, err
	}
	r.ID = newID("rcp")
	err := s.update(func(d *snapshot) error {
		d.Recipients = append(d.Recipients, r)
		return nil
	})
	if err != nil {
		return care.Recipient{}, err
	}
	return r, nil
}

func (s *Store) Recipient(id string) (care.Recipient, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return findRecipient(s.data, id)
}

func (s *Store) Recipients() []care.Recipient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.data.Recipients)
}

func findRecipient(d snapshot, id string) (care.Recipient, error) {
	i := slices.IndexFunc(d.Recipients, func(r care.Recipient) bool { return r.ID == id })
	if i < 0 {
		return care.Recipient{}, fmt.Errorf("recipient %q: %w", id, ErrNotFound)
	}
	return d.Recipients[i], nil
}

func (s *Store) AddContact(c care.Contact) (care.Contact, error) {
	if err := c.Validate(); err != nil {
		return care.Contact{}, err
	}
	c.ID = newID("con")
	err := s.update(func(d *snapshot) error {
		if _, err := findRecipient(*d, c.RecipientID); err != nil {
			return err
		}
		d.Contacts = append(d.Contacts, c)
		return nil
	})
	if err != nil {
		return care.Contact{}, err
	}
	return c, nil
}

func (s *Store) Contacts(recipientID string) []care.Contact {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []care.Contact
	for _, c := range s.data.Contacts {
		if c.RecipientID == recipientID {
			out = append(out, c)
		}
	}
	return out
}

func (s *Store) AddMedication(m care.Medication) (care.Medication, error) {
	if err := m.Validate(); err != nil {
		return care.Medication{}, err
	}
	m = cloneMedication(m)
	m.ID = newID("med")
	m.Stopped = false
	m.CreatedAt = time.Now().UTC()
	err := s.update(func(d *snapshot) error {
		if _, err := findRecipient(*d, m.RecipientID); err != nil {
			return err
		}
		d.Medications = append(d.Medications, m)
		return nil
	})
	if err != nil {
		return care.Medication{}, err
	}
	return cloneMedication(m), nil
}

func (s *Store) Medication(id string) (care.Medication, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, err := medicationIndex(s.data, id)
	if err != nil {
		return care.Medication{}, err
	}
	return cloneMedication(s.data.Medications[i]), nil
}

func (s *Store) Medications(recipientID string) []care.Medication {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []care.Medication
	for _, m := range s.data.Medications {
		if m.RecipientID == recipientID {
			out = append(out, cloneMedication(m))
		}
	}
	return out
}

func (s *Store) StopMedication(id string) error {
	return s.update(func(d *snapshot) error {
		i, err := medicationIndex(*d, id)
		if err != nil {
			return err
		}
		m := cloneMedication(d.Medications[i])
		m.Stopped = true
		d.Medications[i] = m
		return nil
	})
}

func medicationIndex(d snapshot, id string) (int, error) {
	i := slices.IndexFunc(d.Medications, func(m care.Medication) bool { return m.ID == id })
	if i < 0 {
		return -1, fmt.Errorf("medication %q: %w", id, ErrNotFound)
	}
	return i, nil
}

func cloneMedication(m care.Medication) care.Medication {
	m.Times = slices.Clone(m.Times)
	m.Days = slices.Clone(m.Days)
	return m
}

func (s *Store) RecordDose(e care.DoseEvent) (care.DoseEvent, error) {
	if err := e.Validate(); err != nil {
		return care.DoseEvent{}, err
	}
	err := s.update(func(d *snapshot) error {
		i, err := medicationIndex(*d, e.MedicationID)
		if err != nil {
			return err
		}
		e.RecipientID = d.Medications[i].RecipientID

		j := slices.IndexFunc(d.Doses, func(x care.DoseEvent) bool {
			return x.MedicationID == e.MedicationID && x.Slot.Equal(e.Slot)
		})
		if j >= 0 {
			e.ID = d.Doses[j].ID
			d.Doses[j] = e
			return nil
		}
		e.ID = newID("dose")
		d.Doses = append(d.Doses, e)
		return nil
	})
	if err != nil {
		return care.DoseEvent{}, err
	}
	return e, nil
}

func (s *Store) Doses(recipientID string, from, to time.Time) []care.DoseEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []care.DoseEvent
	for _, e := range s.data.Doses {
		if e.RecipientID == recipientID && within(e.Slot, from, to) {
			out = append(out, e)
		}
	}
	return out
}

func (s *Store) RecordCheckIn(c care.CheckIn) (care.CheckIn, error) {
	if err := c.Validate(); err != nil {
		return care.CheckIn{}, err
	}
	c.ID = newID("chk")
	err := s.update(func(d *snapshot) error {
		if _, err := findRecipient(*d, c.RecipientID); err != nil {
			return err
		}
		d.CheckIns = append(d.CheckIns, c)
		return nil
	})
	if err != nil {
		return care.CheckIn{}, err
	}
	return c, nil
}

func (s *Store) CheckIns(recipientID string, from, to time.Time) []care.CheckIn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []care.CheckIn
	for _, c := range s.data.CheckIns {
		if c.RecipientID == recipientID && within(c.At, from, to) {
			out = append(out, c)
		}
	}
	return out
}

func within(t, from, to time.Time) bool {
	return !t.Before(from) && t.Before(to)
}

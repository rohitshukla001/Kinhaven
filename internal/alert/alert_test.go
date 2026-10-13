package alert

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/care"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/store"
)

type sms struct{ phone, text string }

type fakeSender struct {
	mu   sync.Mutex
	sent []sms
	err  error
}

func (f *fakeSender) Send(_ context.Context, phone, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sms{phone, text})
	return nil
}

func (f *fakeSender) take() []sms {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.sent
	f.sent = nil
	return out
}

type fixture struct {
	store   *store.Store
	sender  *fakeSender
	watcher *Watcher
	person  care.Recipient
	day     time.Time
	now     time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "kinhaven.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := st.AddRecipient(care.Recipient{Name: "Kamala", Timezone: "Asia/Kolkata"})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := r.Location()
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1)
	f := &fixture{
		store:  st,
		sender: &fakeSender{},
		person: r,
		day:    time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, loc),
	}
	f.watcher = &Watcher{
		Store:  st,
		Sender: f.sender,
		Grace:  time.Hour,
		Now:    func() time.Time { return f.now },
		Logger: slog.New(slog.DiscardHandler),
	}
	return f
}

func (f *fixture) contact(t *testing.T, name, phone string, notify bool) care.Contact {
	t.Helper()
	c, err := f.store.AddContact(care.Contact{RecipientID: f.person.ID, Name: name, Phone: phone, NotifyMissedDose: notify})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (f *fixture) medication(t *testing.T, name, dosage string, times ...string) care.Medication {
	t.Helper()
	m := care.Medication{RecipientID: f.person.ID, Name: name, Dosage: dosage}
	for _, s := range times {
		c, err := care.ParseClock(s)
		if err != nil {
			t.Fatal(err)
		}
		m.Times = append(m.Times, c)
	}
	m, err := f.store.AddMedication(m)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func (f *fixture) at(h, m int) time.Time {
	return f.day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

func TestMissedDosesAlertFamilyOnce(t *testing.T) {
	f := newFixture(t)
	f.contact(t, "Ravi", "+919876543210", true)
	f.contact(t, "Asha", "+14155550100", false)
	f.medication(t, "Thyroxine", "", "07:00")
	f.medication(t, "Metformin", "500 mg", "08:00")
	calcium := f.medication(t, "Calcium", "", "08:30")
	f.medication(t, "Amlodipine", "", "09:30")
	if _, err := f.store.RecordDose(care.DoseEvent{MedicationID: calcium.ID, Slot: f.at(8, 30), Status: care.DoseTaken, RecordedAt: f.at(8, 31)}); err != nil {
		t.Fatal(err)
	}

	f.now = f.at(10, 0)
	f.watcher.Check(context.Background())
	sent := f.sender.take()
	if len(sent) != 1 || sent[0].phone != "+919876543210" {
		t.Fatalf("sent = %+v, want one SMS to Ravi", sent)
	}
	want := "Kinhaven: Kamala has not taken Thyroxine (07:00) and Metformin 500 mg (08:00). Please check in with them."
	if sent[0].text != want {
		t.Errorf("text = %q\nwant  %q", sent[0].text, want)
	}

	f.watcher.Check(context.Background())
	if sent := f.sender.take(); len(sent) != 0 {
		t.Errorf("repeat check sent %+v", sent)
	}

	f.now = f.at(11, 0)
	f.watcher.Check(context.Background())
	sent = f.sender.take()
	if len(sent) != 1 || !strings.Contains(sent[0].text, "has not taken Amlodipine (09:30).") {
		t.Errorf("later check sent %+v, want only Amlodipine", sent)
	}
	if got := f.store.Alerts(f.person.ID, f.day, f.day.AddDate(0, 0, 1)); len(got) != 3 {
		t.Errorf("stored alerts = %d, want 3", len(got))
	}
}

func TestFailedSendIsRetried(t *testing.T) {
	f := newFixture(t)
	f.contact(t, "Ravi", "+919876543210", true)
	f.medication(t, "Metformin", "", "08:00")
	f.now = f.at(10, 0)

	var logs bytes.Buffer
	f.watcher.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	f.sender.err = errors.New("throttled")
	f.watcher.Check(context.Background())
	if !strings.Contains(logs.String(), "throttled") {
		t.Errorf("failure not logged: %s", logs.String())
	}
	if got := f.store.Alerts(f.person.ID, f.day, f.now); len(got) != 0 {
		t.Fatalf("alert stored although sending failed: %+v", got)
	}

	f.sender.err = nil
	f.watcher.Check(context.Background())
	if sent := f.sender.take(); len(sent) != 1 {
		t.Errorf("retry sent %+v", sent)
	}
}

func TestOldMissesAreNotAlerted(t *testing.T) {
	f := newFixture(t)
	f.contact(t, "Ravi", "+919876543210", true)
	f.medication(t, "Metformin", "", "08:00", "20:00")

	f.now = f.at(22, 0)
	f.watcher.Check(context.Background())
	sent := f.sender.take()
	if len(sent) != 1 || !strings.Contains(sent[0].text, "Metformin (20:00).") {
		t.Errorf("sent = %+v, want only the 20:00 dose", sent)
	}
}

func TestNewContactGetsOpenMisses(t *testing.T) {
	f := newFixture(t)
	f.contact(t, "Ravi", "+919876543210", true)
	f.medication(t, "Metformin", "", "08:00")
	f.now = f.at(10, 0)
	f.watcher.Check(context.Background())
	f.sender.take()

	f.contact(t, "Meera", "+919812345678", true)
	f.watcher.Check(context.Background())
	sent := f.sender.take()
	if len(sent) != 1 || sent[0].phone != "+919812345678" {
		t.Errorf("sent = %+v, want one SMS to Meera", sent)
	}
}

func TestMessageDatesEarlierDays(t *testing.T) {
	loc := time.FixedZone("IST", 19800)
	now := time.Date(2026, 10, 6, 6, 0, 0, 0, loc)
	slots := []care.Slot{
		{Medication: care.Medication{Name: "Metformin"}, At: time.Date(2026, 10, 5, 20, 0, 0, 0, loc)},
		{Medication: care.Medication{Name: "Thyroxine"}, At: time.Date(2026, 10, 6, 5, 0, 0, 0, loc)},
		{Medication: care.Medication{Name: "Calcium"}, At: time.Date(2026, 10, 6, 5, 30, 0, 0, loc)},
	}
	want := "Kinhaven: Kamala has not taken Metformin (Oct 5 20:00), Thyroxine (05:00) and Calcium (05:30). Please check in with them."
	if got := message("Kamala", slots, now); got != want {
		t.Errorf("message = %q\nwant      %q", got, want)
	}
}

func TestRunStopsWhenContextEnds(t *testing.T) {
	f := newFixture(t)
	f.now = f.at(10, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.watcher.Run(ctx, 10*time.Millisecond)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

type fakePublisher struct{ input *sns.PublishInput }

func (p *fakePublisher) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	p.input = in
	return &sns.PublishOutput{}, nil
}

func TestSNSSenderPublishesTransactionalSMS(t *testing.T) {
	pub := &fakePublisher{}
	if err := (&SNSSender{client: pub}).Send(context.Background(), "+919876543210", "hello"); err != nil {
		t.Fatal(err)
	}
	in := pub.input
	if *in.PhoneNumber != "+919876543210" || *in.Message != "hello" {
		t.Errorf("input = %+v", in)
	}
	if v := in.MessageAttributes["AWS.SNS.SMS.SMSType"]; *v.StringValue != "Transactional" || *v.DataType != "String" {
		t.Errorf("SMSType attribute = %+v", v)
	}
}

func TestLogSenderMasksPhone(t *testing.T) {
	var logs bytes.Buffer
	LogSender{Logger: slog.New(slog.NewTextHandler(&logs, nil))}.Send(context.Background(), "+919876543210", "hi")
	if !strings.Contains(logs.String(), "+91******3210") || strings.Contains(logs.String(), "9876543210") {
		t.Errorf("log = %s", logs.String())
	}
}

func TestNewSNSSenderNeedsRegion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_REGION", "")
	if _, err := NewSNSSender(context.Background()); err == nil || !strings.Contains(err.Error(), "AWS_REGION") {
		t.Errorf("err = %v, want a hint about AWS_REGION", err)
	}

	t.Setenv("AWS_REGION", "ap-south-1")
	if _, err := NewSNSSender(context.Background()); err != nil {
		t.Errorf("with region: %v", err)
	}
}

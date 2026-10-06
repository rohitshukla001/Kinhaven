package care

import (
	"fmt"
	"time"
)

type ClockTime struct {
	Hour, Minute int
}

func ParseClock(s string) (ClockTime, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return ClockTime{}, invalid("time %q must be HH:MM in 24-hour format", s)
	}
	return ClockTime{Hour: t.Hour(), Minute: t.Minute()}, nil
}

func (c ClockTime) String() string {
	return fmt.Sprintf("%02d:%02d", c.Hour, c.Minute)
}

func (c ClockTime) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c *ClockTime) UnmarshalText(b []byte) error {
	parsed, err := ParseClock(string(b))
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

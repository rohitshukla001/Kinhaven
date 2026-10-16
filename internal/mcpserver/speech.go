package mcpserver

import (
	"fmt"
	"strings"
	"time"
)

func scheduleSpeech(out getScheduleOutput, day time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, %s: ", out.Name, day.Format("Monday 2 January"))
	if len(out.Doses) == 0 {
		b.WriteString("no doses are scheduled.")
		return b.String()
	}
	total := len(out.Doses)
	fmt.Fprintf(&b, "%d of %d doses taken.", out.Summary.Taken, total)

	var missed, due, upcoming []string
	for _, d := range out.Doses {
		label := d.Medication + " at " + d.Time
		switch d.Status {
		case "missed":
			if d.FamilyAlertedAt != "" {
				label += " (family alerted)"
			}
			missed = append(missed, label)
		case "due":
			due = append(due, label)
		case "upcoming":
			upcoming = append(upcoming, label)
		}
	}
	if len(missed) > 0 {
		fmt.Fprintf(&b, " Missed: %s.", strings.Join(missed, ", "))
	}
	if len(due) > 0 {
		fmt.Fprintf(&b, " Due now: %s.", strings.Join(due, ", "))
	}
	if len(upcoming) > 0 {
		fmt.Fprintf(&b, " Next: %s.", upcoming[0])
	}
	return b.String()
}

func adherenceSpeech(out adherenceOutput) string {
	counted := out.Taken + out.Skipped + out.Missed
	if counted == 0 {
		return fmt.Sprintf("%s has no doses due between %s and %s yet.", out.Name, out.From, out.To)
	}
	text := fmt.Sprintf("%s took %d of %d doses (%d%%) between %s and %s.", out.Name, out.Taken, counted, *out.Percent, out.From, out.To)
	if out.Missed > 0 || out.Skipped > 0 {
		text += fmt.Sprintf(" Missed %d, skipped %d.", out.Missed, out.Skipped)
	}
	if n := out.Moods["good"] + out.Moods["okay"] + out.Moods["poor"]; n > 0 {
		text += fmt.Sprintf(" Check-ins: %d good, %d okay, %d poor.", out.Moods["good"], out.Moods["okay"], out.Moods["poor"])
	}
	return text
}

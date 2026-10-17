package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Mock struct{}

type intent int

const (
	askSchedule intent = iota
	askSummary
	logDose
	checkIn
)

func (Mock) Converse(_ context.Context, _ string, _ []Tool, history []Message) (Message, error) {
	question, results := latestQuestion(history)
	if n := len(results); n > 0 && results[n-1].IsError {
		return say("Sorry, " + results[n-1].Text), nil
	}

	id := fmt.Sprintf("mock-%d", len(history))
	switch classify(question) {
	case askSummary:
		if len(results) == 0 {
			return use(id, "get_adherence_summary", map[string]any{"days": 7}), nil
		}
		return say(firstLine(results[0].Text)), nil

	case checkIn:
		if len(results) == 0 {
			return use(id, "record_checkin", map[string]any{"mood": moodOf(question), "note": question}), nil
		}
		return say(fmt.Sprintf("Thank you. I noted that you feel %s today.", moodOf(question))), nil

	case logDose:
		switch len(results) {
		case 0:
			return use(id, "list_medications", map[string]any{}), nil
		case 1:
			medID, names, err := findMedication(results[0].Text, question)
			if err != nil {
				return Message{}, err
			}
			if medID == "" {
				return say("Which medicine was it? I know about " + strings.Join(names, ", ") + "."), nil
			}
			status := "taken"
			if strings.Contains(strings.ToLower(question), "skip") {
				status = "skipped"
			}
			return use(id, "log_dose", map[string]any{"medication_id": medID, "status": status}), nil
		default:
			return say(loggedReply(results[1].Text)), nil
		}

	default:
		if len(results) == 0 {
			return use(id, "get_schedule", map[string]any{}), nil
		}
		return say(firstLine(results[0].Text)), nil
	}
}

func latestQuestion(history []Message) (string, []*ToolResult) {
	var results []*ToolResult
	for i := len(history) - 1; i >= 0; i-- {
		for _, p := range history[i].Parts {
			if p.ToolResult != nil {
				results = append([]*ToolResult{p.ToolResult}, results...)
			}
			if history[i].Role == User && p.Text != "" {
				return p.Text, results
			}
		}
	}
	return "", results
}

func classify(q string) intent {
	q = strings.ToLower(q)
	isQuestion := strings.HasSuffix(strings.TrimSpace(q), "?") || hasAnyPrefix(q, "has ", "did ", "have ", "is ", "what", "when")
	switch {
	case containsAny(q, "week", "summary", "update", "adherence", "how has", "how was"):
		return askSummary
	case containsAny(q, "feel", "feeling", "mood"):
		return checkIn
	case !isQuestion && containsAny(q, "took", "taken", "had her", "had his", "skipped", "skip"):
		return logDose
	}
	return askSchedule
}

func moodOf(q string) string {
	q = strings.ToLower(q)
	switch {
	case containsAny(q, "not well", "not good", "bad", "tired", "pain", "dizzy", "unwell", "poor", "sad"):
		return "poor"
	case containsAny(q, "okay", "alright", "fine", "so-so"):
		return "okay"
	}
	return "good"
}

func findMedication(listJSON, question string) (string, []string, error) {
	var list struct {
		Medications []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"medications"`
	}
	if err := json.Unmarshal([]byte(listJSON), &list); err != nil {
		return "", nil, fmt.Errorf("read list_medications result: %w", err)
	}
	q := strings.ToLower(question)
	names := make([]string, len(list.Medications))
	for i, m := range list.Medications {
		names[i] = m.Name
		if strings.Contains(q, strings.ToLower(m.Name)) {
			return m.ID, nil, nil
		}
	}
	return "", names, nil
}

func loggedReply(logJSON string) string {
	var logged struct {
		Medication  string `json:"medication"`
		ScheduledAt string `json:"scheduled_at"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(logJSON), &logged); err != nil {
		return "Done."
	}
	at, err := time.Parse(time.RFC3339, logged.ScheduledAt)
	if err != nil {
		return fmt.Sprintf("Done. I marked %s as %s.", logged.Medication, logged.Status)
	}
	return fmt.Sprintf("Done. I marked the %s dose of %s as %s.", at.Format("15:04"), logged.Medication, logged.Status)
}

func say(text string) Message {
	return Message{Role: Assistant, Parts: []Part{{Text: text}}}
}

func use(id, tool string, input map[string]any) Message {
	return Message{Role: Assistant, Parts: []Part{{ToolUse: &ToolUse{ID: id, Name: tool, Input: input}}}}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

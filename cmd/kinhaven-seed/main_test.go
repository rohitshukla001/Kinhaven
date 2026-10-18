package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSeedWritesDemoDataOnce(t *testing.T) {
	dir := t.TempDir()
	getenv := func(k string) string {
		switch k {
		case "KINHAVEN_DATA_DIR":
			return dir
		case "KINHAVEN_TIMEZONE":
			return "Asia/Kolkata"
		}
		return ""
	}
	now := time.Date(2026, 10, 16, 6, 0, 0, 0, time.UTC)

	var out bytes.Buffer
	if err := run(getenv, now, &out); err != nil {
		t.Fatal(err)
	}
	if want := "Seeded Kamala (Asia/Kolkata) into " + dir; !strings.HasPrefix(out.String(), want) {
		t.Errorf("output = %q, want prefix %q", out.String(), want)
	}

	err := run(getenv, now, &out)
	if err == nil || !strings.Contains(err.Error(), "already has people") {
		t.Errorf("second run err = %v", err)
	}
}

func TestSeedRejectsBadConfig(t *testing.T) {
	getenv := func(k string) string {
		if k == "KINHAVEN_TIMEZONE" {
			return "Mars/Olympus"
		}
		return ""
	}
	if err := run(getenv, time.Now(), &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "KINHAVEN_TIMEZONE") {
		t.Errorf("err = %v", err)
	}
}

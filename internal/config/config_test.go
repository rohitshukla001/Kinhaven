package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(envFrom(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Addr != "127.0.0.1:8080" || cfg.AllowPublicBind || cfg.DataDir != "./data" {
		t.Errorf("got Addr=%q AllowPublicBind=%v DataDir=%q", cfg.Addr, cfg.AllowPublicBind, cfg.DataDir)
	}
	if want := []string{"http://localhost:8090", "http://127.0.0.1:8090"}; !slices.Equal(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.Location.String() != "UTC" || cfg.LogLevel != slog.LevelInfo || cfg.MissedDoseGrace != time.Hour {
		t.Errorf("got Location=%v LogLevel=%v MissedDoseGrace=%v", cfg.Location, cfg.LogLevel, cfg.MissedDoseGrace)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(envFrom(map[string]string{
		"KINHAVEN_ADDR":              "0.0.0.0:9000",
		"KINHAVEN_ALLOW_PUBLIC_BIND": "true",
		"KINHAVEN_ALLOWED_ORIGINS":   " https://Demo.Example.com , http://localhost:3000/ ",
		"KINHAVEN_TIMEZONE":          "Asia/Kolkata",
		"KINHAVEN_LOG_LEVEL":         "debug",
		"KINHAVEN_MISSED_DOSE_GRACE": "45m",
	}))
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{"https://demo.example.com", "http://localhost:3000"}; !slices.Equal(cfg.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.Location.String() != "Asia/Kolkata" || cfg.LogLevel != slog.LevelDebug || cfg.MissedDoseGrace != 45*time.Minute {
		t.Errorf("got Location=%v LogLevel=%v MissedDoseGrace=%v", cfg.Location, cfg.LogLevel, cfg.MissedDoseGrace)
	}
}

func TestLoadAcceptsLoopbackHosts(t *testing.T) {
	for _, addr := range []string{"localhost:8080", "127.0.0.1:0", "[::1]:8080"} {
		if _, err := Load(envFrom(map[string]string{"KINHAVEN_ADDR": addr})); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	tests := []struct {
		key, value, wantErr string
	}{
		{"KINHAVEN_ADDR", "0.0.0.0:8080", "not a loopback host"},
		{"KINHAVEN_ADDR", ":8080", "not a loopback host"},
		{"KINHAVEN_ADDR", "127.0.0.1", "missing port"},
		{"KINHAVEN_ALLOW_PUBLIC_BIND", "maybe", "want true or false"},
		{"KINHAVEN_ALLOWED_ORIGINS", "http://localhost:8090/app", "not an origin"},
		{"KINHAVEN_ALLOWED_ORIGINS", "localhost:8090", "not an origin"},
		{"KINHAVEN_ALLOWED_ORIGINS", " , ", "no origins"},
		{"KINHAVEN_TIMEZONE", "Mars/Olympus", "unknown time zone"},
		{"KINHAVEN_LOG_LEVEL", "loud", "KINHAVEN_LOG_LEVEL"},
		{"KINHAVEN_MISSED_DOSE_GRACE", "soon", "positive duration"},
		{"KINHAVEN_MISSED_DOSE_GRACE", "-5m", "positive duration"},
	}
	for _, tt := range tests {
		_, err := Load(envFrom(map[string]string{tt.key: tt.value}))
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s=%q: err = %v, want it to mention %q", tt.key, tt.value, err, tt.wantErr)
		}
	}
}

func TestLoadReportsEveryError(t *testing.T) {
	_, err := Load(envFrom(map[string]string{
		"KINHAVEN_ADDR":      "0.0.0.0:8080",
		"KINHAVEN_TIMEZONE":  "Nowhere/City",
		"KINHAVEN_LOG_LEVEL": "loud",
	}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, key := range []string{"KINHAVEN_ADDR", "KINHAVEN_TIMEZONE", "KINHAVEN_LOG_LEVEL"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

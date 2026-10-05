package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != DefaultAddr {
		t.Errorf("Addr = %q, want %q", cfg.Addr, DefaultAddr)
	}
	if cfg.AllowPublicBind {
		t.Error("AllowPublicBind = true, want false")
	}
	if cfg.DataDir != DefaultDataDir {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, DefaultDataDir)
	}
	wantOrigins := []string{"http://localhost:8090", "http://127.0.0.1:8090"}
	if !slices.Equal(cfg.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, wantOrigins)
	}
	if cfg.Location == nil || cfg.Location.String() != "UTC" {
		t.Errorf("Location = %v, want UTC", cfg.Location)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		EnvAddr:            "0.0.0.0:9000",
		EnvAllowPublicBind: "true",
		EnvDataDir:         "/var/lib/carecircle",
		EnvAllowedOrigins:  " https://Demo.Example.com , http://localhost:3000/ ",
		EnvTimezone:        "Asia/Kolkata",
		EnvLogLevel:        "debug",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Addr != "0.0.0.0:9000" || !cfg.AllowPublicBind {
		t.Errorf("Addr = %q, AllowPublicBind = %v", cfg.Addr, cfg.AllowPublicBind)
	}
	if cfg.DataDir != "/var/lib/carecircle" {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
	wantOrigins := []string{"https://demo.example.com", "http://localhost:3000"}
	if !slices.Equal(cfg.AllowedOrigins, wantOrigins) {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, wantOrigins)
	}
	if cfg.Location.String() != "Asia/Kolkata" {
		t.Errorf("Location = %v", cfg.Location)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
}

func TestLoadLoopbackHosts(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8080", "localhost:8080", "[::1]:8080", "127.0.0.1:0"} {
		if _, err := Load(env(map[string]string{EnvAddr: addr})); err != nil {
			t.Errorf("Load(%q) error = %v, want nil", addr, err)
		}
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"public bind without opt-in", map[string]string{EnvAddr: "0.0.0.0:8080"}, "not a loopback address"},
		{"empty host without opt-in", map[string]string{EnvAddr: ":8080"}, "not a loopback address"},
		{"missing port", map[string]string{EnvAddr: "127.0.0.1"}, "must be host:port"},
		{"bad port", map[string]string{EnvAddr: "127.0.0.1:99999"}, "port must be"},
		{"bad bool", map[string]string{EnvAllowPublicBind: "maybe"}, EnvAllowPublicBind},
		{"origin with path", map[string]string{EnvAllowedOrigins: "http://localhost:8090/app"}, "invalid origin"},
		{"origin without scheme", map[string]string{EnvAllowedOrigins: "localhost:8090"}, "invalid origin"},
		{"empty origin list", map[string]string{EnvAllowedOrigins: " , "}, "at least one origin"},
		{"bad timezone", map[string]string{EnvTimezone: "Mars/Olympus"}, "unknown IANA timezone"},
		{"bad log level", map[string]string{EnvLogLevel: "loud"}, EnvLogLevel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(env(tt.env))
			if err == nil {
				t.Fatalf("Load() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Load() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := Load(env(map[string]string{
		EnvAddr:     "0.0.0.0:8080",
		EnvTimezone: "Nowhere/City",
		EnvLogLevel: "loud",
	}))
	if err == nil {
		t.Fatal("Load() error = nil, want three errors")
	}
	for _, key := range []string{EnvAddr, EnvTimezone, EnvLogLevel} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %s", err, key)
		}
	}
}

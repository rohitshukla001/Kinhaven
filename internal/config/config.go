// Package config loads and validates CareCircle server settings from
// environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Environment variable names.
const (
	EnvAddr            = "CARECIRCLE_ADDR"
	EnvAllowPublicBind = "CARECIRCLE_ALLOW_PUBLIC_BIND"
	EnvDataDir         = "CARECIRCLE_DATA_DIR"
	EnvAllowedOrigins  = "CARECIRCLE_ALLOWED_ORIGINS"
	EnvTimezone        = "CARECIRCLE_TIMEZONE"
	EnvLogLevel        = "CARECIRCLE_LOG_LEVEL"
)

// Default values.
const (
	DefaultAddr           = "127.0.0.1:8080"
	DefaultDataDir        = "./data"
	DefaultAllowedOrigins = "http://localhost:8090,http://127.0.0.1:8090"
	DefaultTimezone       = "UTC"
	DefaultLogLevel       = "info"
)

// Config is the validated server configuration.
type Config struct {
	// Addr is the TCP address that the HTTP server listens on.
	Addr string
	// AllowPublicBind permits a non-loopback Addr. The MCP spec recommends
	// a loopback bind for local servers, so this is off by default.
	AllowPublicBind bool
	// DataDir is the directory for persisted CareCircle data.
	DataDir string
	// AllowedOrigins is the list of browser origins that can call the MCP
	// endpoint. The server rejects other Origin header values with HTTP 403.
	AllowedOrigins []string
	// Location is the default timezone for care schedules.
	Location *time.Location
	// LogLevel is the minimum log level.
	LogLevel slog.Level
}

// Load reads the configuration through getenv and validates it.
// It returns all validation errors together.
func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	var (
		cfg  Config
		errs []error
	)

	cfg.Addr = get(EnvAddr, DefaultAddr)
	cfg.DataDir = get(EnvDataDir, DefaultDataDir)

	allowPublic, err := strconv.ParseBool(get(EnvAllowPublicBind, "false"))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: must be true or false", EnvAllowPublicBind))
	}
	cfg.AllowPublicBind = allowPublic

	if err := validateAddr(cfg.Addr, cfg.AllowPublicBind); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvAddr, err))
	}

	origins, err := parseOrigins(get(EnvAllowedOrigins, DefaultAllowedOrigins))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvAllowedOrigins, err))
	}
	cfg.AllowedOrigins = origins

	loc, err := time.LoadLocation(get(EnvTimezone, DefaultTimezone))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: unknown IANA timezone", EnvTimezone))
	}
	cfg.Location = loc

	if err := cfg.LogLevel.UnmarshalText([]byte(get(EnvLogLevel, DefaultLogLevel))); err != nil {
		errs = append(errs, fmt.Errorf("%s: must be debug, info, warn or error", EnvLogLevel))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// validateAddr checks that addr is host:port. Unless allowPublic is true,
// the host must be a loopback address or "localhost".
func validateAddr(addr string, allowPublic bool) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("must be host:port: %w", err)
	}
	if p, err := strconv.Atoi(port); err != nil || p < 0 || p > 65535 {
		return errors.New("port must be a number from 0 to 65535")
	}
	if allowPublic {
		return nil
	}
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("host %q is not a loopback address; set %s=true to allow it", host, EnvAllowPublicBind)
}

// parseOrigins parses a comma-separated list of origins such as
// "https://example.com,http://localhost:8090".
func parseOrigins(raw string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		o := strings.TrimSpace(part)
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return nil, fmt.Errorf("invalid origin %q: use scheme://host[:port]", o)
		}
		out = append(out, strings.ToLower(u.Scheme+"://"+u.Host))
	}
	if len(out) == 0 {
		return nil, errors.New("must contain at least one origin")
	}
	return out, nil
}

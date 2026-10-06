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

type Config struct {
	Addr            string
	AllowPublicBind bool
	DataDir         string
	AllowedOrigins  []string
	Location        *time.Location
	LogLevel        slog.Level
}

func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		Addr:    get("KINHAVEN_ADDR", "127.0.0.1:8080"),
		DataDir: get("KINHAVEN_DATA_DIR", "./data"),
	}
	var errs []error
	var err error

	if cfg.AllowPublicBind, err = strconv.ParseBool(get("KINHAVEN_ALLOW_PUBLIC_BIND", "false")); err != nil {
		errs = append(errs, errors.New("KINHAVEN_ALLOW_PUBLIC_BIND: want true or false"))
	}
	if err := checkBindHost(cfg.Addr, cfg.AllowPublicBind); err != nil {
		errs = append(errs, fmt.Errorf("KINHAVEN_ADDR: %w", err))
	}
	if cfg.AllowedOrigins, err = parseOrigins(get("KINHAVEN_ALLOWED_ORIGINS", "http://localhost:8090,http://127.0.0.1:8090")); err != nil {
		errs = append(errs, fmt.Errorf("KINHAVEN_ALLOWED_ORIGINS: %w", err))
	}
	if cfg.Location, err = time.LoadLocation(get("KINHAVEN_TIMEZONE", "UTC")); err != nil {
		errs = append(errs, fmt.Errorf("KINHAVEN_TIMEZONE: %w", err))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(get("KINHAVEN_LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("KINHAVEN_LOG_LEVEL: %w", err))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func checkBindHost(addr string, allowPublic bool) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if allowPublic || host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("%q is not a loopback host; set KINHAVEN_ALLOW_PUBLIC_BIND=true to listen on it", host)
}

func parseOrigins(raw string) ([]string, error) {
	var origins []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || strings.Trim(u.Path, "/") != "" {
			return nil, fmt.Errorf("%q is not an origin like https://example.com", s)
		}
		origins = append(origins, strings.ToLower(u.Scheme+"://"+u.Host))
	}
	if len(origins) == 0 {
		return nil, errors.New("no origins given")
	}
	return origins, nil
}

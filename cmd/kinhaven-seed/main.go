package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rohitshukla001/Kinhaven/internal/config"
	"github.com/rohitshukla001/Kinhaven/internal/demo"
	"github.com/rohitshukla001/Kinhaven/internal/store"
)

func main() {
	if err := run(os.Getenv, time.Now(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "kinhaven-seed:", err)
		os.Exit(1)
	}
}

func run(getenv func(string) string, now time.Time, out io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	path := filepath.Join(cfg.DataDir, "kinhaven.json")
	st, err := store.Open(path)
	if err != nil {
		return err
	}
	res, err := demo.Seed(st, cfg.Location, cfg.MissedDoseGrace, now)
	if err != nil {
		return fmt.Errorf("%s: %w; remove the file or set KINHAVEN_DATA_DIR to an empty folder", path, err)
	}
	fmt.Fprintf(out, "Seeded %s (%s) into %s: %d contacts, %d medicines, %d doses, %d check-ins, %d sent alerts.\n",
		res.Recipient.Name, res.Recipient.Timezone, path, res.Contacts, res.Medications, res.Doses, res.CheckIns, res.Alerts)
	return nil
}

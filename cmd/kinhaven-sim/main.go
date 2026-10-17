package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/agent"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/config"
	"github.com/rohitshukla001/AmazonDeveloperHackathon/internal/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "kinhaven-sim:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, in io.Reader, out io.Writer) error {
	cfg, err := config.LoadSim(getenv)
	if err != nil {
		return err
	}
	model, err := newModel(ctx, cfg.Model)
	if err != nil {
		return err
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "kinhaven-sim", Version: version.Version}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: cfg.MCPURL}, nil)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.MCPURL, err)
	}
	defer session.Close()

	a, err := agent.New(ctx, model, session, agent.SystemPrompt)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Connected to %s with %d tools, model %s. Type a request, or an empty line to quit.\n", cfg.MCPURL, len(a.Tools()), cfg.Model)

	var history []agent.Message
	lines := bufio.NewScanner(in)
	for {
		fmt.Fprint(out, "> ")
		if !lines.Scan() {
			return lines.Err()
		}
		text := strings.TrimSpace(lines.Text())
		if text == "" {
			return nil
		}
		var turn agent.Turn
		history, turn, err = a.Ask(ctx, history, text)
		for _, c := range turn.Calls {
			fmt.Fprintf(out, "  [%s]\n", c.Tool)
		}
		if err != nil {
			fmt.Fprintln(out, "  error:", err)
			continue
		}
		fmt.Fprintln(out, turn.Reply)
	}
}

func newModel(ctx context.Context, name string) (agent.Model, error) {
	if name == "mock" {
		return agent.Mock{}, nil
	}
	return agent.NewBedrock(ctx, name)
}

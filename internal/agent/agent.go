package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Role string

const (
	User      Role = "user"
	Assistant Role = "assistant"
)

type Message struct {
	Role  Role
	Parts []Part
}

type Part struct {
	Text       string
	ToolUse    *ToolUse
	ToolResult *ToolResult
}

type ToolUse struct {
	ID    string
	Name  string
	Input map[string]any
}

type ToolResult struct {
	ToolUseID string
	Text      string
	IsError   bool
}

type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

type Model interface {
	Converse(ctx context.Context, system string, tools []Tool, history []Message) (Message, error)
}

type Call struct {
	Tool        string              `json:"tool"`
	Input       map[string]any      `json:"input"`
	Result      *mcp.CallToolResult `json:"result"`
	ResourceURI string              `json:"resource_uri,omitempty"`
}

type Turn struct {
	Reply string `json:"reply"`
	Calls []Call `json:"calls"`
}

const maxSteps = 8

var ErrTooManySteps = errors.New("the assistant needed too many steps to answer")

type Agent struct {
	model   Model
	session *mcp.ClientSession
	system  string
	tools   []Tool
	ui      map[string]string
}

func New(ctx context.Context, model Model, session *mcp.ClientSession, system string) (*Agent, error) {
	a := &Agent{model: model, session: session, system: system, ui: map[string]string{}}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list MCP tools: %w", err)
		}
		schema, err := asMap(tool.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", tool.Name, err)
		}
		a.tools = append(a.tools, Tool{Name: tool.Name, Description: tool.Description, InputSchema: schema})
		if ui, ok := tool.Meta["ui"].(map[string]any); ok {
			if uri, ok := ui["resourceUri"].(string); ok {
				a.ui[tool.Name] = uri
			}
		}
	}
	return a, nil
}

func (a *Agent) Tools() []Tool { return a.tools }

func (a *Agent) Ask(ctx context.Context, history []Message, text string) ([]Message, Turn, error) {
	before := history
	history = append(history[:len(history):len(history)], Message{Role: User, Parts: []Part{{Text: text}}})
	var turn Turn

	for range maxSteps {
		reply, err := a.model.Converse(ctx, a.system, a.tools, history)
		if err != nil {
			return before, turn, err
		}
		history = append(history, reply)

		var uses []*ToolUse
		var said []string
		for _, p := range reply.Parts {
			if p.ToolUse != nil {
				uses = append(uses, p.ToolUse)
			} else if p.Text != "" {
				said = append(said, p.Text)
			}
		}
		if len(uses) == 0 {
			turn.Reply = strings.TrimSpace(strings.Join(said, " "))
			return history, turn, nil
		}

		results := Message{Role: User}
		for _, use := range uses {
			call, result := a.call(ctx, use)
			turn.Calls = append(turn.Calls, call)
			results.Parts = append(results.Parts, Part{ToolResult: result})
		}
		history = append(history, results)
	}
	return before, turn, ErrTooManySteps
}

func (a *Agent) call(ctx context.Context, use *ToolUse) (Call, *ToolResult) {
	call := Call{Tool: use.Name, Input: use.Input, ResourceURI: a.ui[use.Name]}
	res, err := a.session.CallTool(ctx, &mcp.CallToolParams{Name: use.Name, Arguments: use.Input})
	if err != nil {
		return call, &ToolResult{ToolUseID: use.ID, Text: err.Error(), IsError: true}
	}
	call.Result = res
	return call, &ToolResult{ToolUseID: use.ID, Text: resultText(res), IsError: res.IsError}
}

func resultText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func asMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type fakeBedrock struct {
	inputs  []*bedrockruntime.ConverseInput
	outputs []types.Message
}

func (f *fakeBedrock) Converse(_ context.Context, in *bedrockruntime.ConverseInput, _ ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	f.inputs = append(f.inputs, in)
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return &bedrockruntime.ConverseOutput{Output: &types.ConverseOutputMemberMessage{Value: out}}, nil
}

func TestBedrockToolUseRoundTrip(t *testing.T) {
	k := startKinhaven(t)
	k.seed(t)
	fake := &fakeBedrock{outputs: []types.Message{
		{Role: types.ConversationRoleAssistant, Content: []types.ContentBlock{
			&types.ContentBlockMemberText{Value: "Let me check."},
			&types.ContentBlockMemberToolUse{Value: types.ToolUseBlock{
				ToolUseId: aws.String("tu-1"),
				Name:      aws.String("get_adherence_summary"),
				Input:     document.NewLazyDocument(map[string]any{"days": 7}),
			}},
		}},
		{Role: types.ConversationRoleAssistant, Content: []types.ContentBlock{
			&types.ContentBlockMemberText{Value: "She took every dose this week."},
		}},
	}}
	a, err := New(context.Background(), &Bedrock{client: fake, modelID: "test-model"}, k.session, SystemPrompt)
	if err != nil {
		t.Fatal(err)
	}

	_, turn, err := a.Ask(context.Background(), nil, "How was her week?")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Reply != "She took every dose this week." {
		t.Errorf("reply = %q", turn.Reply)
	}
	if len(turn.Calls) != 1 || turn.Calls[0].Input["days"] == nil {
		t.Fatalf("calls = %+v", turn.Calls)
	}

	first := fake.inputs[0]
	if aws.ToString(first.ModelId) != "test-model" || len(first.ToolConfig.Tools) != 10 || *first.InferenceConfig.MaxTokens != 800 {
		t.Errorf("first input: model=%s tools=%d", aws.ToString(first.ModelId), len(first.ToolConfig.Tools))
	}
	if sys := first.System[0].(*types.SystemContentBlockMemberText).Value; !strings.Contains(sys, "Echo Show") {
		t.Errorf("system prompt = %q", sys)
	}
	spec := first.ToolConfig.Tools[0].(*types.ToolMemberToolSpec).Value
	if aws.ToString(spec.Name) == "" || spec.InputSchema == nil {
		t.Errorf("tool spec = %+v", spec)
	}

	second := fake.inputs[1].Messages
	if len(second) != 3 || second[1].Role != types.ConversationRoleAssistant || second[2].Role != types.ConversationRoleUser {
		t.Fatalf("second request roles = %+v", second)
	}
	result := second[2].Content[0].(*types.ContentBlockMemberToolResult).Value
	text := result.Content[0].(*types.ToolResultContentBlockMemberText).Value
	if aws.ToString(result.ToolUseId) != "tu-1" || result.Status != types.ToolResultStatusSuccess || !strings.Contains(text, "Kamala") {
		t.Errorf("tool result block = %+v (%s)", result, text)
	}
}

func TestNewBedrockNeedsRegion(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_REGION", "")
	if _, err := NewBedrock(context.Background(), "m"); err == nil || !strings.Contains(err.Error(), "AWS_REGION") {
		t.Errorf("err = %v", err)
	}
}

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

type converser interface {
	Converse(context.Context, *bedrockruntime.ConverseInput, ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
}

type Bedrock struct {
	client  converser
	modelID string
}

func NewBedrock(ctx context.Context, modelID string) (*Bedrock, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRetryMaxAttempts(6))
	if err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		return nil, errors.New("Amazon Bedrock needs an AWS region; set AWS_REGION")
	}
	return &Bedrock{client: bedrockruntime.NewFromConfig(cfg), modelID: modelID}, nil
}

func (b *Bedrock) Converse(ctx context.Context, system string, tools []Tool, history []Message) (Message, error) {
	in := &bedrockruntime.ConverseInput{
		ModelId:  aws.String(b.modelID),
		Messages: toBedrockMessages(history),
		System:   []types.SystemContentBlock{&types.SystemContentBlockMemberText{Value: system}},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(800),
			Temperature: aws.Float32(0.2),
		},
	}
	if len(tools) > 0 {
		in.ToolConfig = &types.ToolConfiguration{Tools: toBedrockTools(tools)}
	}

	out, err := b.client.Converse(ctx, in)
	if err != nil {
		return Message{}, fmt.Errorf("bedrock converse: %w", err)
	}
	msg, ok := out.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return Message{}, fmt.Errorf("bedrock converse: unexpected output %T", out.Output)
	}
	return fromBedrockMessage(msg.Value)
}

func toBedrockTools(tools []Tool) []types.Tool {
	out := make([]types.Tool, len(tools))
	for i, t := range tools {
		out[i] = &types.ToolMemberToolSpec{Value: types.ToolSpecification{
			Name:        aws.String(t.Name),
			Description: aws.String(t.Description),
			InputSchema: &types.ToolInputSchemaMemberJson{Value: document.NewLazyDocument(t.InputSchema)},
		}}
	}
	return out
}

func toBedrockMessages(history []Message) []types.Message {
	out := make([]types.Message, 0, len(history))
	for _, m := range history {
		msg := types.Message{Role: types.ConversationRoleUser}
		if m.Role == Assistant {
			msg.Role = types.ConversationRoleAssistant
		}
		for _, p := range m.Parts {
			switch {
			case p.ToolUse != nil:
				msg.Content = append(msg.Content, &types.ContentBlockMemberToolUse{Value: types.ToolUseBlock{
					ToolUseId: aws.String(p.ToolUse.ID),
					Name:      aws.String(p.ToolUse.Name),
					Input:     document.NewLazyDocument(p.ToolUse.Input),
				}})
			case p.ToolResult != nil:
				status := types.ToolResultStatusSuccess
				if p.ToolResult.IsError {
					status = types.ToolResultStatusError
				}
				msg.Content = append(msg.Content, &types.ContentBlockMemberToolResult{Value: types.ToolResultBlock{
					ToolUseId: aws.String(p.ToolResult.ToolUseID),
					Content:   []types.ToolResultContentBlock{&types.ToolResultContentBlockMemberText{Value: p.ToolResult.Text}},
					Status:    status,
				}})
			case p.Text != "":
				msg.Content = append(msg.Content, &types.ContentBlockMemberText{Value: p.Text})
			}
		}
		out = append(out, msg)
	}
	return out
}

func fromBedrockMessage(m types.Message) (Message, error) {
	msg := Message{Role: Assistant}
	for _, block := range m.Content {
		switch c := block.(type) {
		case *types.ContentBlockMemberText:
			msg.Parts = append(msg.Parts, Part{Text: c.Value})
		case *types.ContentBlockMemberToolUse:
			input, err := decodeInput(c.Value.Input)
			if err != nil {
				return Message{}, fmt.Errorf("decode input for %s: %w", aws.ToString(c.Value.Name), err)
			}
			msg.Parts = append(msg.Parts, Part{ToolUse: &ToolUse{
				ID:    aws.ToString(c.Value.ToolUseId),
				Name:  aws.ToString(c.Value.Name),
				Input: input,
			}})
		}
	}
	return msg, nil
}

func decodeInput(doc document.Interface) (map[string]any, error) {
	input := map[string]any{}
	if doc == nil {
		return input, nil
	}
	b, err := doc.MarshalSmithyDocument()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &input); err != nil {
		return nil, err
	}
	return input, nil
}

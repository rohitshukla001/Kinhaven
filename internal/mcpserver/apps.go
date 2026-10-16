package mcpserver

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	appMIMEType = "text/html;profile=mcp-app"
	cardURI     = "ui://kinhaven/card.html"
)

//go:embed ui/card.html
var cardHTML string

var cardMeta = mcp.Meta{"ui": map[string]any{"resourceUri": cardURI}}

func addApps(s *mcp.Server) {
	resourceMeta := mcp.Meta{"ui": map[string]any{"prefersBorder": true}}
	s.AddResource(&mcp.Resource{
		URI:         cardURI,
		Name:        "kinhaven-card",
		Title:       "Kinhaven care card",
		Description: "Echo Show style card for the day's doses and the adherence summary. Doses that are due or missed can be marked as taken from the card.",
		MIMEType:    appMIMEType,
		Meta:        resourceMeta,
	}, func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
			URI:      cardURI,
			MIMEType: appMIMEType,
			Text:     cardHTML,
			Meta:     resourceMeta,
		}}}, nil
	})
}

func spoken(text string, out any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: text},
		&mcp.TextContent{Text: string(b)},
	}}, nil
}

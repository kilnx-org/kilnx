package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

var llmClient = anthropic.NewClient(option.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))

type rowQuerier interface {
	QueryRowsWithParams(sqlStr string, params map[string]string) ([]database.Row, error)
}

func executeLLM(ctx context.Context, node parser.Node, db rowQuerier, params map[string]string) (string, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	model := node.LLMModel
	if model == "" {
		model = string(anthropic.ModelClaudeHaiku4_5)
	}

	messages, err := buildLLMMessages(node, db, params)
	if err != nil {
		return "", err
	}
	if len(messages) == 0 {
		return "Olá! Como posso ajudar?", nil
	}

	req := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 1024,
		Messages:  messages,
	}
	if node.LLMSystem != "" {
		req.System = []anthropic.TextBlockParam{{Text: node.LLMSystem}}
	}

	resp, err := llmClient.Messages.New(ctx, req)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	if len(resp.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}

	text := strings.TrimSpace(resp.Content[0].Text)
	fmt.Printf("  llm %s -> %d chars\n", model, len(text))
	return text, nil
}

func buildLLMMessages(node parser.Node, db rowQuerier, params map[string]string) ([]anthropic.MessageParam, error) {
	if node.LLMHistorySQL == "" {
		return nil, nil
	}
	rows, err := db.QueryRowsWithParams(node.LLMHistorySQL, params)
	if err != nil {
		return nil, fmt.Errorf("llm history query: %w", err)
	}

	var msgs []anthropic.MessageParam
	for _, row := range rows {
		conteudo := row["conteudo"]
		if conteudo == "" {
			continue
		}
		switch row["papel"] {
		case "assistente":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(conteudo)))
		default:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(conteudo)))
		}
	}
	return mergeConsecutiveRoles(msgs), nil
}

// mergeConsecutiveRoles collapses consecutive messages with the same role
// to satisfy Anthropic's alternating user/assistant requirement.
func mergeConsecutiveRoles(msgs []anthropic.MessageParam) []anthropic.MessageParam {
	if len(msgs) <= 1 {
		return msgs
	}
	out := []anthropic.MessageParam{msgs[0]}
	for i := 1; i < len(msgs); i++ {
		prev := &out[len(out)-1]
		if msgs[i].Role == prev.Role {
			prev.Content = append(prev.Content, msgs[i].Content...)
		} else {
			out = append(out, msgs[i])
		}
	}
	return out
}

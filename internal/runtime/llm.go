package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kilnx-org/kilnx/internal/database"
	"github.com/kilnx-org/kilnx/internal/parser"
)

var llmClient = &http.Client{Timeout: 60 * time.Second}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type rowQuerier interface {
	QueryRowsWithParams(sqlStr string, params map[string]string) ([]database.Row, error)
}

func executeLLM(node parser.Node, db rowQuerier, params map[string]string) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	model := node.LLMModel
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	var messages []anthropicMessage

	if node.LLMHistorySQL != "" {
		rows, err := db.QueryRowsWithParams(node.LLMHistorySQL, params)
		if err != nil {
			return "", fmt.Errorf("llm history query: %w", err)
		}
		for _, row := range rows {
			papel := row["papel"]
			conteudo := row["conteudo"]
			if conteudo == "" {
				continue
			}
			role := "user"
			switch papel {
			case "assistente":
				role = "assistant"
			case "sistema":
				role = "user"
			}
			messages = append(messages, anthropicMessage{Role: role, Content: conteudo})
		}
	}

	if len(messages) == 0 {
		return "Olá! Como posso ajudar?", nil
	}

	// Anthropic requires alternating user/assistant; deduplicate consecutive same-role messages.
	messages = mergeConsecutiveRoles(messages)

	req := anthropicRequest{
		Model:     model,
		MaxTokens: 1024,
		System:    node.LLMSystem,
		Messages:  messages,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("llm marshal: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := llmClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm http: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("llm read: %w", err)
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", fmt.Errorf("llm parse: %w", err)
	}
	if ar.Error != nil {
		return "", fmt.Errorf("anthropic: %s", ar.Error.Message)
	}
	if len(ar.Content) == 0 {
		return "", fmt.Errorf("anthropic: empty response")
	}

	text := strings.TrimSpace(ar.Content[0].Text)
	fmt.Printf("  llm %s -> %d chars\n", model, len(text))
	return text, nil
}

// mergeConsecutiveRoles collapses consecutive messages with the same role
// to satisfy Anthropic's alternating user/assistant requirement.
func mergeConsecutiveRoles(msgs []anthropicMessage) []anthropicMessage {
	if len(msgs) == 0 {
		return msgs
	}
	out := []anthropicMessage{msgs[0]}
	for i := 1; i < len(msgs); i++ {
		if msgs[i].Role == out[len(out)-1].Role {
			out[len(out)-1].Content += "\n" + msgs[i].Content
		} else {
			out = append(out, msgs[i])
		}
	}
	return out
}

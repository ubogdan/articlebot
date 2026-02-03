package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Claude API structures
type ClaudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	Messages    []ClaudeMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	System      string          `json:"system,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []ClaudeContent `json:"content"`
	Usage   ClaudeUsage     `json:"usage"`
}

type ClaudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ClaudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
type ClaudeAI struct {
	config     Config
	httpClient *http.Client
}

func NewClaudeAI(apiKey string, opts ...func(ai *Config)) *ClaudeAI {
	ai := ClaudeAI{
		httpClient: &http.Client{
			Timeout: 300 * time.Second,
		},
	}
	// Apply default config
	ai.defaultConfig(apiKey)

	// Apply options
	for _, opt := range opts {
		opt(&ai.config)
	}

	return &ai
}

func (ai *ClaudeAI) defaultConfig(apiKey string) {
	ai.config = Config{
		apiKey:      apiKey,
		model:       "claude-sonnet-4-20250514",
		maxTokens:   4000,
		temperature: 0.7,
	}
}

// Test verifies Claude API connection
func (ai *ClaudeAI) Test() error {
	response, err := ai.Call("You are a helpful assistant.", "Say 'Claude API connection successful!' in a friendly way.")
	if err != nil {
		return err
	}
	log.Printf("   Response: %s", response)
	return nil
}

// Call makes a request to Claude API
func (ai *ClaudeAI) Call(prompt, message string) (string, error) {
	reqBody := ClaudeRequest{
		Model:     ai.config.model,
		MaxTokens: ai.config.maxTokens,
		Messages: []ClaudeMessage{
			{Role: "user", Content: message},
		},
		Temperature: ai.config.temperature,
		System:      prompt,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", ai.config.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := ai.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	log.Printf("   📊 Tokens - Input: %d, Output: %d, Total: %d",
		claudeResp.Usage.InputTokens,
		claudeResp.Usage.OutputTokens,
		claudeResp.Usage.InputTokens+claudeResp.Usage.OutputTokens)

	return claudeResp.Content[0].Text, nil
}

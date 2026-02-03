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

// OpenAI API structures
type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAI struct {
	config     Config
	httpClient *http.Client
}

func NewOpenAI(apiKey string, opts ...func(ai *Config)) *OpenAI {
	ai := OpenAI{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
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

func (ai *OpenAI) defaultConfig(apiKey string) {
	ai.config = Config{
		apiKey:      apiKey,
		model:       "gpt-4o-mini",
		maxTokens:   4000,
		temperature: 0.7,
	}
}

// Test  tests OpenAI API connection
func (ai *OpenAI) Test() error {
	response, err := ai.Call("", "Say 'OpenAI API connection successful!' in a friendly way.")
	if err != nil {
		return err
	}

	log.Printf("   Response: %s", response)

	return nil
}

// Call makes a request to OpenAI API
func (ai *OpenAI) Call(prompt, message string) (string, error) {
	messages := []OpenAIMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: message},
	}

	if len(prompt) == 0 {
		messages = messages[1:]
	}

	reqBody := OpenAIChatRequest{
		Model:       ai.config.model,
		Messages:    messages,
		Temperature: ai.config.temperature,
		MaxTokens:   ai.config.maxTokens,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ai.config.apiKey)

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

	var chatResp OpenAIChatResponse
	err = json.Unmarshal(body, &chatResp)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	log.Printf("   📊 Tokens - Prompt: %d, Completion: %d, Total: %d",
		chatResp.Usage.PromptTokens,
		chatResp.Usage.CompletionTokens,
		chatResp.Usage.TotalTokens)

	return chatResp.Choices[0].Message.Content, nil
}

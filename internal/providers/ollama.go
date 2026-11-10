package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	defaultOllamaHost = "http://localhost:11434"
)

type OllamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

type Ollama struct {
	config     Config
	httpClient *http.Client
}

func NewOllama(baseURL, model string, opts ...func(ai *Config)) *Ollama {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOllamaHost
	}

	ai := &Ollama{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
	ai.defaultConfig(baseURL, model)
	for _, opt := range opts {
		opt(&ai.config)
	}
	return ai
}

func (ai *Ollama) defaultConfig(baseURL, model string) {
	ai.config = Config{
		baseURL:     baseURL,
		model:       model,
		maxTokens:   4000,
		temperature: 0.7,
	}
}

func (ai *Ollama) Test() error {
	_, err := ai.Call("You are a helpful assistant.", "Say 'Ollama connection successful!' in a friendly way.")
	return err
}

func (ai *Ollama) Call(prompt, message string) (string, error) {
	messages := []OllamaMessage{
		{Role: "system", Content: prompt},
		{Role: "user", Content: message},
	}
	if prompt == "" {
		messages = []OllamaMessage{{Role: "user", Content: message}}
	}

	payload := OllamaRequest{
		Model:    ai.config.model,
		Messages: messages,
		Stream:   false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	headers := map[string]string{"Content-Type": "application/json"}
	respBody, err := doJSONRequest(ai.httpClient, http.MethodPost, strings.TrimRight(ai.config.baseURL, "/")+"/api/chat", headers, body, 3)
	if err != nil {
		return "", err
	}

	var chatResp OllamaChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if strings.TrimSpace(chatResp.Message.Content) == "" {
		return "", fmt.Errorf("no content in response")
	}

	log.Printf("   📊 Model: %s", chatResp.Model)
	return chatResp.Message.Content, nil
}

func (ai *Ollama) callRaw(prompt, message string) (string, error) {
	payload := map[string]interface{}{
		"model": ai.config.model,
		"messages": []map[string]string{
			{"role": "system", "content": prompt},
			{"role": "user", "content": message},
		},
		"stream": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(ai.config.baseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ai.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

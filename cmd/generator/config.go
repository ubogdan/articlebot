package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ubogdan/articlebot/internal/providers"
)

// Config holds bot configuration
type Config struct {
	// AI Provider settings
	AIProvider   string  `json:"ai_provider"` // "openai", "claude", "ollama", "random", "alternate"
	OpenAIAPIKey string  `json:"openai_api_key"`
	ClaudeAPIKey string  `json:"claude_api_key"`
	OpenAIModel  string  `json:"openai_model"`
	ClaudeModel  string  `json:"claude_model"`
	OllamaHost   string  `json:"ollama_host"`
	OllamaModel  string  `json:"ollama_model"`
	Temperature  float64 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`

	// Bot settings
	OutputDir          string        `json:"output_dir"`
	ArticlesPerDay     int           `json:"articles_per_day"`
	GenerationInterval time.Duration `json:"generation_interval_hours"`
	AutoPublish        bool          `json:"auto_publish"`
}

func (c Config) HasClaude() bool {
	return c.ClaudeAPIKey != "" && c.ClaudeAPIKey != "your-claude-api-key-here"
}

func (c Config) HasOpenAI() bool {
	return c.OpenAIAPIKey != "" && c.OpenAIAPIKey != "your-openai-api-key-here"
}

func (c Config) HasOllama() bool {
	return c.OllamaHost != "" && c.OllamaModel != ""
}

func (c Config) Provider(provider AIProvider) (Provider, error) {
	switch provider {
	case ProviderClaude:
		if !c.HasClaude() {
			return nil, fmt.Errorf("ClaudeAI API key not configured")
		}
		return providers.NewClaudeAI(
			c.ClaudeAPIKey,
			providers.WithModel(c.ClaudeModel),
			providers.WithTemperature(c.Temperature),
			providers.WithMaxTokens(c.MaxTokens),
		), nil
	case ProviderOpenAI:
		if !c.HasOpenAI() {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}

		return providers.NewOpenAI(
			c.OpenAIAPIKey,
			providers.WithModel(c.OpenAIModel),
			providers.WithTemperature(c.Temperature),
			providers.WithMaxTokens(c.MaxTokens),
		), nil
	case ProviderOllama:
		if !c.HasOllama() {
			return nil, fmt.Errorf("Ollama host and model must be configured")
		}
		return providers.NewOllama(
			c.OllamaHost,
			c.OllamaModel,
			providers.WithTemperature(c.Temperature),
			providers.WithMaxTokens(c.MaxTokens),
		), nil
	}
	return nil, fmt.Errorf("invalid AI provider: %s (use 'openai', 'claude', 'ollama', 'random', or 'alternate')", c.AIProvider)
}

// Validate checks the runtime configuration for invalid or unsafe values.
func (c Config) Validate() error {
	if c.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if c.ArticlesPerDay <= 0 {
		return fmt.Errorf("articles_per_day must be greater than 0")
	}
	if c.GenerationInterval <= 0 {
		return fmt.Errorf("generation_interval_hours must be greater than 0")
	}
	if c.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be greater than 0")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	switch AIProvider(c.AIProvider) {
	case ProviderOpenAI:
		if c.OpenAIAPIKey == "" || c.OpenAIAPIKey == "your-openai-api-key-here" {
			return fmt.Errorf("OpenAI API key not configured")
		}
	case ProviderClaude:
		if c.ClaudeAPIKey == "" || c.ClaudeAPIKey == "your-claude-api-key-here" {
			return fmt.Errorf("Claude API key not configured")
		}
	case ProviderOllama:
		if c.OllamaHost == "" || c.OllamaModel == "" {
			return fmt.Errorf("Ollama host and model must be configured")
		}
	case ProviderRandom, ProviderAlternate:
		if (c.OpenAIAPIKey == "" || c.OpenAIAPIKey == "your-openai-api-key-here") &&
			(c.ClaudeAPIKey == "" || c.ClaudeAPIKey == "your-claude-api-key-here") {
			return fmt.Errorf("at least one API key must be configured for random/alternate mode")
		}
	default:
		return fmt.Errorf("invalid AI provider: %s (use 'openai', 'claude', 'ollama', 'random', or 'alternate')", c.AIProvider)
	}

	return nil
}

// getDefaultConfig returns default configuration
func getDefaultConfig() Config {
	return Config{
		AIProvider:         "random",
		OpenAIAPIKey:       "your-openai-api-key-here",
		ClaudeAPIKey:       "your-claude-api-key-here",
		OpenAIModel:        "gpt-4o-mini",
		ClaudeModel:        "claude-sonnet-4-20250514",
		OllamaHost:         "http://localhost:11434",
		OllamaModel:        "llama2",
		Temperature:        0.7,
		MaxTokens:          4000,
		OutputDir:          "content/posts",
		ArticlesPerDay:     3,
		GenerationInterval: 8 * time.Hour,
		AutoPublish:        false,
	}
}

// loadConfig loads configuration from file
func (ag *ArticleGenerator) loadConfig() error {
	data, err := os.ReadFile(ConfigFilename)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	config.GenerationInterval = config.GenerationInterval * time.Hour
	config.applyEnv()
	ag.config = config
	return nil
}

// applyEnv overrides API keys with environment variables when they are set
func (c *Config) applyEnv() {
	if key := os.Getenv(EnvClaudeAPIKey); key != "" {
		c.ClaudeAPIKey = key
	}
	if key := os.Getenv(EnvOpenAIAPIKey); key != "" {
		c.OpenAIAPIKey = key
	}
	if key := os.Getenv(EnvOllamaHost); key != "" {
		c.OllamaHost = key
	}
	if key := os.Getenv(EnvOllamaModel); key != "" {
		c.OllamaModel = key
	}
}

// createDefaultConfig creates a default configuration file
func (ag *ArticleGenerator) createDefaultConfig() {
	config := ag.config
	config.GenerationInterval = 8 // Store as hours for JSON

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Error creating config: %v", err)
		return
	}

	// The config may hold API keys, so keep it readable by the owner only
	if err := os.WriteFile(ConfigFilename, data, 0600); err != nil {
		log.Printf("Error writing config: %v", err)
		return
	}

	log.Printf("✅ Created %s", ConfigFilename)
	log.Println("")
	log.Println("📝 Configuration options:")
	log.Println("   ai_provider: 'openai', 'claude', 'ollama', 'random', or 'alternate'")
	log.Println("   openai_api_key: Get from https://platform.openai.com/api-keys")
	log.Println("   claude_api_key: Get from https://console.anthropic.com/")
	log.Println("   ollama_host: Host for Ollama API (default: http://localhost:11434)")
	log.Println("   ollama_model: Model name for Ollama")
	log.Printf("   API keys can also be set via %s and %s", EnvClaudeAPIKey, EnvOpenAIAPIKey)
}

// validateConfig checks if the configuration is valid
func (ag *ArticleGenerator) validateConfig() error {
	return ag.config.Validate()
}

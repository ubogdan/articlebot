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
	AIProvider   string  `json:"ai_provider"` // "openai", "claude", "random", "alternate"
	OpenAIAPIKey string  `json:"openai_api_key"`
	ClaudeAPIKey string  `json:"claude_api_key"`
	OpenAIModel  string  `json:"openai_model"`
	ClaudeModel  string  `json:"claude_model"`
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
	}
	return nil, fmt.Errorf("invalid AI provider: %s (use 'openai', 'claude', 'random', or 'alternate')", c.AIProvider)
}

// getDefaultConfig returns default configuration
func getDefaultConfig() Config {
	return Config{
		AIProvider:         "random",
		OpenAIAPIKey:       "your-openai-api-key-here",
		ClaudeAPIKey:       "your-claude-api-key-here",
		OpenAIModel:        "gpt-4o-mini",
		ClaudeModel:        "claude-sonnet-4-20250514",
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
	ag.config = config
	return nil
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

	if err := os.WriteFile(ConfigFilename, data, 0644); err != nil {
		log.Printf("Error writing config: %v", err)
		return
	}

	log.Println("✅ Created bot-config.json")
	log.Println("")
	log.Println("📝 Configuration options:")
	log.Println("   ai_provider: 'openai', 'claude', 'random', or 'alternate'")
	log.Println("   openai_api_key: Get from https://platform.openai.com/api-keys")
	log.Println("   claude_api_key: Get from https://console.anthropic.com/")
}

// validateConfig checks if the configuration is valid
func (ag *ArticleGenerator) validateConfig() error {
	switch AIProvider(ag.config.AIProvider) {
	case ProviderOpenAI:
		if ag.config.OpenAIAPIKey == "" || ag.config.OpenAIAPIKey == "your-openai-api-key-here" {
			return fmt.Errorf("OpenAI API key not configured")
		}
	case ProviderClaude:
		if ag.config.ClaudeAPIKey == "" || ag.config.ClaudeAPIKey == "your-claude-api-key-here" {
			return fmt.Errorf("Claude API key not configured")
		}
	case ProviderRandom, ProviderAlternate:
		if (ag.config.OpenAIAPIKey == "" || ag.config.OpenAIAPIKey == "your-openai-api-key-here") &&
			(ag.config.ClaudeAPIKey == "" || ag.config.ClaudeAPIKey == "your-claude-api-key-here") {
			return fmt.Errorf("at least one API key must be configured for random/alternate mode")
		}
	default:
		return fmt.Errorf("invalid AI provider: %s (use 'openai', 'claude', 'random', or 'alternate')", ag.config.AIProvider)
	}

	return nil
}

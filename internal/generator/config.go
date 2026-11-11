package generator

import (
	"encoding/json"
	"fmt"
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

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
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

// LoadConfig loads configuration from path, converting stored hours to a duration.
// Environment overrides and provider selection belong to the caller.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	defaults := DefaultConfig()
	config := Config{
		OllamaHost:  defaults.OllamaHost,
		OllamaModel: defaults.OllamaModel,
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}

	config.GenerationInterval = config.GenerationInterval * time.Hour
	return config, nil
}

// ApplyEnv overrides settings using the supplied environment lookup.
func (c *Config) ApplyEnv(getenv func(string) string) {
	if key := getenv(EnvClaudeAPIKey); key != "" {
		c.ClaudeAPIKey = key
	}
	if key := getenv(EnvOpenAIAPIKey); key != "" {
		c.OpenAIAPIKey = key
	}
	if key := getenv(EnvOllamaHost); key != "" {
		c.OllamaHost = key
	}
	if key := getenv(EnvOllamaModel); key != "" {
		c.OllamaModel = key
	}
}

// WriteDefaultConfig writes a template with the interval stored in hours.
func WriteDefaultConfig(path string) error {
	config := DefaultConfig()
	config.GenerationInterval /= time.Hour

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode default config: %w", err)
	}

	// The config may hold API keys, so keep it readable by the owner only
	return os.WriteFile(path, data, 0600)
}

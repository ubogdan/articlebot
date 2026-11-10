package main

import (
	"os"
	"testing"
)

func TestConfigApplyEnv(t *testing.T) {
	tests := []struct {
		name            string
		claudeEnv       string
		openAIEnv       string
		ollamaHost      string
		ollamaModel     string
		wantClaude      string
		wantOpenAI      string
		wantOllamaHost  string
		wantOllamaModel string
	}{
		{name: "env unset keeps file keys", wantClaude: "file-claude", wantOpenAI: "file-openai", wantOllamaHost: "file-host", wantOllamaModel: "file-model"},
		{name: "env overrides file keys", claudeEnv: "env-claude", openAIEnv: "env-openai", ollamaHost: "env-host", ollamaModel: "env-model", wantClaude: "env-claude", wantOpenAI: "env-openai", wantOllamaHost: "env-host", wantOllamaModel: "env-model"},
		{name: "partial override", claudeEnv: "env-claude", wantClaude: "env-claude", wantOpenAI: "file-openai", wantOllamaHost: "file-host", wantOllamaModel: "file-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvClaudeAPIKey, tt.claudeEnv)
			t.Setenv(EnvOpenAIAPIKey, tt.openAIEnv)
			t.Setenv(EnvOllamaHost, tt.ollamaHost)
			t.Setenv(EnvOllamaModel, tt.ollamaModel)

			cfg := Config{ClaudeAPIKey: "file-claude", OpenAIAPIKey: "file-openai", OllamaHost: "file-host", OllamaModel: "file-model"}
			cfg.applyEnv()

			if cfg.ClaudeAPIKey != tt.wantClaude || cfg.OpenAIAPIKey != tt.wantOpenAI || cfg.OllamaHost != tt.wantOllamaHost || cfg.OllamaModel != tt.wantOllamaModel {
				t.Fatalf("got claude=%q openai=%q ollama_host=%q ollama_model=%q, want claude=%q openai=%q ollama_host=%q ollama_model=%q",
					cfg.ClaudeAPIKey, cfg.OpenAIAPIKey, cfg.OllamaHost, cfg.OllamaModel, tt.wantClaude, tt.wantOpenAI, tt.wantOllamaHost, tt.wantOllamaModel)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid random config",
			config: Config{
				AIProvider:         "random",
				OpenAIAPIKey:       "openai-key",
				ClaudeAPIKey:       "claude-key",
				OutputDir:          "content/posts",
				ArticlesPerDay:     2,
				GenerationInterval: 4,
				MaxTokens:          2000,
				Temperature:        0.5,
			},
			wantErr: false,
		},
		{
			name: "missing output dir",
			config: Config{
				AIProvider:         "openai",
				OpenAIAPIKey:       "openai-key",
				OutputDir:          "",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        0.7,
			},
			wantErr: true,
		},
		{
			name: "invalid temperature",
			config: Config{
				AIProvider:         "claude",
				ClaudeAPIKey:       "claude-key",
				OutputDir:          "content/posts",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        3,
			},
			wantErr: true,
		},
		{
			name: "provider requires key",
			config: Config{
				AIProvider:         "claude",
				ClaudeAPIKey:       "your-claude-api-key-here",
				OutputDir:          "content/posts",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        0.7,
			},
			wantErr: true,
		},
		{
			name: "local provider with host and model",
			config: Config{
				AIProvider:         "ollama",
				OllamaHost:         "http://localhost:11434",
				OllamaModel:        "llama2",
				OutputDir:          "content/posts",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        0.7,
			},
			wantErr: false,
		},
		{
			name: "local provider missing host",
			config: Config{
				AIProvider:         "ollama",
				OllamaHost:         "",
				OllamaModel:        "llama2",
				OutputDir:          "content/posts",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        0.7,
			},
			wantErr: true,
		},
		{
			name: "local provider missing model",
			config: Config{
				AIProvider:         "ollama",
				OllamaHost:         "http://localhost:11434",
				OllamaModel:        "",
				OutputDir:          "content/posts",
				ArticlesPerDay:     1,
				GenerationInterval: 1,
				MaxTokens:          1000,
				Temperature:        0.7,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldSaveTopicsArgs(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{"articlebot", "topics", "--save"}
	if !shouldSaveTopics() {
		t.Fatal("expected --save to enable saving")
	}

	os.Args = []string{"articlebot", "topics", "--no-save"}
	if shouldSaveTopics() {
		t.Fatal("expected --no-save to disable saving")
	}
}

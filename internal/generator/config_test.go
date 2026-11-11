package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigOllamaDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ConfigFilename)

	tests := []struct {
		name      string
		fields    map[string]interface{}
		hostEnv   string
		modelEnv  string
		wantHost  string
		wantModel string
		wantErr   bool
	}{
		{name: "omitted fields", wantHost: "http://localhost:11434", wantModel: "llama2"},
		{name: "file values", fields: map[string]interface{}{"ollama_host": "http://ollama:11434", "ollama_model": "custom"}, wantHost: "http://ollama:11434", wantModel: "custom"},
		{name: "environment overrides defaults", hostEnv: "http://remote:11434", modelEnv: "env-model", wantHost: "http://remote:11434", wantModel: "env-model"},
		{name: "explicit empty host rejected", fields: map[string]interface{}{"ollama_host": ""}, wantModel: "llama2", wantErr: true},
		{name: "explicit empty model rejected", fields: map[string]interface{}{"ollama_model": ""}, wantHost: "http://localhost:11434", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{EnvOllamaHost: tt.hostEnv, EnvOllamaModel: tt.modelEnv}
			fields := map[string]interface{}{
				"ai_provider": "ollama", "output_dir": "content/posts",
				"articles_per_day": 1, "generation_interval_hours": 8,
				"max_tokens": 4000, "temperature": 0.7,
			}
			for key, value := range tt.fields {
				fields[key] = value
			}
			data, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			config, err := LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			config.ApplyEnv(func(key string) string { return env[key] })
			if config.OllamaHost != tt.wantHost || config.OllamaModel != tt.wantModel {
				t.Fatalf("got host=%q model=%q, want host=%q model=%q", config.OllamaHost, config.OllamaModel, tt.wantHost, tt.wantModel)
			}
			if err := config.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("validateConfig() = %v, wantErr %v", err, tt.wantErr)
			}
			if config.GenerationInterval != 8*time.Hour {
				t.Fatalf("unexpected generation interval: %v", config.GenerationInterval)
			}
		})
	}
}

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
			env := map[string]string{
				EnvClaudeAPIKey: tt.claudeEnv, EnvOpenAIAPIKey: tt.openAIEnv,
				EnvOllamaHost: tt.ollamaHost, EnvOllamaModel: tt.ollamaModel,
			}

			cfg := Config{ClaudeAPIKey: "file-claude", OpenAIAPIKey: "file-openai", OllamaHost: "file-host", OllamaModel: "file-model"}
			cfg.ApplyEnv(func(key string) string { return env[key] })

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

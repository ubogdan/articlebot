package main

import "testing"

func TestConfigApplyEnv(t *testing.T) {
	tests := []struct {
		name       string
		claudeEnv  string
		openAIEnv  string
		wantClaude string
		wantOpenAI string
	}{
		{name: "env unset keeps file keys", wantClaude: "file-claude", wantOpenAI: "file-openai"},
		{name: "env overrides file keys", claudeEnv: "env-claude", openAIEnv: "env-openai", wantClaude: "env-claude", wantOpenAI: "env-openai"},
		{name: "partial override", claudeEnv: "env-claude", wantClaude: "env-claude", wantOpenAI: "file-openai"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvClaudeAPIKey, tt.claudeEnv)
			t.Setenv(EnvOpenAIAPIKey, tt.openAIEnv)

			cfg := Config{ClaudeAPIKey: "file-claude", OpenAIAPIKey: "file-openai"}
			cfg.applyEnv()

			if cfg.ClaudeAPIKey != tt.wantClaude || cfg.OpenAIAPIKey != tt.wantOpenAI {
				t.Fatalf("got claude=%q openai=%q, want claude=%q openai=%q",
					cfg.ClaudeAPIKey, cfg.OpenAIAPIKey, tt.wantClaude, tt.wantOpenAI)
			}
		})
	}
}

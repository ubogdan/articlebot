package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestProviderOverride(t *testing.T) {
	dir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Error(err)
		}
	})
	t.Setenv(EnvOpenAIAPIKey, "")
	t.Setenv(EnvClaudeAPIKey, "")
	t.Setenv(EnvOllamaHost, "")
	t.Setenv(EnvOllamaModel, "")

	tests := []struct {
		name       string
		args       []string
		configured string
		openAIKey  string
		claudeKey  string
		want       string
		wantErr    string
	}{
		{name: "keeps configured provider", configured: "claude", claudeKey: "test-key", want: "claude"},
		{name: "overrides before validation", args: []string{"--provider", "openai"}, configured: "claude", openAIKey: "test-key", want: "openai"},
		{name: "equals syntax", args: []string{"--provider=claude"}, configured: "openai", claudeKey: "test-key", want: "claude"},
		{name: "local provider", args: []string{"--provider", "ollama"}, configured: "claude", want: "ollama"},
		{name: "random", args: []string{"--provider", "random"}, configured: "claude", openAIKey: "test-key", want: "random"},
		{name: "alternate", args: []string{"--provider", "alternate"}, configured: "claude", openAIKey: "test-key", want: "alternate"},
		{name: "unknown provider", args: []string{"--provider", "unknown"}, configured: "claude", claudeKey: "test-key", wantErr: "invalid AI provider"},
		{name: "empty provider", args: []string{"--provider="}, configured: "claude", claudeKey: "test-key", wantErr: "invalid AI provider"},
		{name: "missing credentials", args: []string{"--provider", "openai"}, configured: "claude", claudeKey: "test-key", wantErr: "OpenAI API key not configured"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getDefaultConfig()
			cfg.AIProvider = tt.configured
			cfg.OpenAIAPIKey = tt.openAIKey
			cfg.ClaudeAPIKey = tt.claudeKey
			cfg.GenerationInterval = 8
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ConfigFilename, data, 0600); err != nil {
				t.Fatal(err)
			}

			app := newApp()
			app.Commands = []*cli.Command{{
				Name: "check",
				Action: func(c *cli.Context) error {
					gen, err := loadGeneratorForCommand(c)
					if err != nil {
						return err
					}
					if gen == nil {
						t.Fatal("expected generator")
					}
					if gen.config.AIProvider != tt.want {
						t.Fatalf("provider = %q, want %q", gen.config.AIProvider, tt.want)
					}
					if tt.want != "random" && tt.want != "alternate" && string(gen.selectProvider()) != tt.want {
						t.Fatalf("selected provider does not match override %q", tt.want)
					}
					return nil
				},
			}}
			args := append([]string{"articlebot"}, tt.args...)
			err = app.Run(append(args, "check"))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile(ConfigFilename)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(saved, data) {
				t.Fatal("provider override modified config.json")
			}
		})
	}
}

func TestGlobalProviderWithCommandFlags(t *testing.T) {
	app := newApp()
	called := false
	for _, command := range app.Commands {
		if command.Name == "topics" {
			command.Action = func(c *cli.Context) error {
				called = true
				if c.String("provider") != "ollama" || !c.Bool("no-save") {
					t.Fatal("expected global provider and command flag")
				}
				if !reflect.DeepEqual(c.Args().Slice(), []string{"Go Development"}) {
					t.Fatalf("unexpected domains: %v", c.Args().Slice())
				}
				return nil
			}
		}
	}
	if err := app.Run([]string{"articlebot", "--provider", "ollama", "topics", "--no-save", "Go Development"}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("topics action was not called")
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ubogdan/articlebot/internal/generator"
	"github.com/urfave/cli/v2"
)

type testProvider struct {
	call func(string, string) (string, error)
}

func (p testProvider) Call(system, message string) (string, error) {
	return p.call(system, message)
}

func (p testProvider) Test() error { return nil }

func TestProviderOverride(t *testing.T) {
	for _, tt := range []struct {
		name       string
		args       []string
		configured string
		openAIKey  string
		claudeKey  string
		want       generator.AIProvider
		wantErr    string
	}{
		{name: "keeps configured provider", configured: "claude", claudeKey: "test-key", want: generator.ProviderClaude},
		{name: "overrides before validation", args: []string{"--provider", "openai"}, configured: "claude", openAIKey: "test-key", want: generator.ProviderOpenAI},
		{name: "equals syntax", args: []string{"--provider=claude"}, configured: "openai", claudeKey: "test-key", want: generator.ProviderClaude},
		{name: "local provider", args: []string{"--provider", "ollama"}, configured: "claude", want: generator.ProviderOllama},
		{name: "random", args: []string{"--provider", "random"}, configured: "claude", openAIKey: "test-key", want: generator.ProviderOpenAI},
		{name: "alternate", args: []string{"--provider", "alternate"}, configured: "claude", openAIKey: "test-key", want: generator.ProviderOpenAI},
		{name: "unknown provider", args: []string{"--provider", "unknown"}, configured: "claude", claudeKey: "test-key", wantErr: "invalid AI provider"},
		{name: "empty provider", args: []string{"--provider="}, configured: "claude", claudeKey: "test-key", wantErr: "invalid AI provider"},
		{name: "missing credentials", args: []string{"--provider", "openai"}, configured: "claude", claudeKey: "test-key", wantErr: "OpenAI API key not configured"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.json")
			topicsPath := filepath.Join(dir, "topics.json")
			cfg := generator.DefaultConfig()
			cfg.AIProvider = tt.configured
			cfg.OpenAIAPIKey = tt.openAIKey
			cfg.ClaudeAPIKey = tt.claudeKey
			cfg.GenerationInterval = 8
			data, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(topicsPath, []byte(`{"topics":[{"title":"Example"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := newApp(appOptions{
				configPath: configPath,
				getenv:     func(string) string { return "" },
				generator: generator.Options{
					Paths: generator.Paths{Topics: topicsPath, OutputDir: filepath.Join(dir, "posts")},
					NewProvider: func(name generator.AIProvider) (generator.Provider, error) {
						calls++
						if name != tt.want {
							t.Fatalf("provider = %q, want %q", name, tt.want)
						}
						return testProvider{call: func(string, string) (string, error) {
							return "## Conclusion\nFinished.", nil
						}}, nil
					},
				},
			})
			args := append([]string{"articlebot"}, tt.args...)
			err = app.Run(append(args, "generate", "--title", "Example"))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				if calls != 0 {
					t.Fatal("invalid configuration reached provider")
				}
			} else if err != nil {
				t.Fatal(err)
			} else if calls != 1 {
				t.Fatalf("provider calls = %d, want 1", calls)
			}
			saved, err := os.ReadFile(configPath)
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
	app := newApp(appOptions{})
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

func TestConfigInitializationUsesInjectedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	app := newApp(appOptions{
		configPath: path,
		getenv:     func(string) string { return "" },
	})
	if err := app.Run([]string{"articlebot", "generate"}); err != nil {
		t.Fatal(err)
	}
	config, err := generator.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.GenerationInterval != generator.DefaultConfig().GenerationInterval {
		t.Fatalf("unexpected default interval: %v", config.GenerationInterval)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions = %v, want 0600", info.Mode().Perm())
	}
}

func TestInvalidConfigIsNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	app := newApp(appOptions{
		configPath: path,
		getenv:     func(string) string { return "" },
	})
	if err := app.Run([]string{"articlebot", "generate"}); err == nil {
		t.Fatal("expected invalid config error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{" {
		t.Fatal("invalid config was overwritten")
	}
}

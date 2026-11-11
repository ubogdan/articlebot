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

func TestTopicsSavePolicy(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		input   string
		save    bool
		wantErr string
	}{
		{name: "save", args: []string{"--save"}, save: true},
		{name: "no save", args: []string{"--no-save"}},
		{name: "dry run", args: []string{"--dry-run"}},
		{name: "global provider", args: []string{"--no-save", "SRE"}},
		{name: "flag after domain", args: []string{"SRE", "--save"}, save: true},
		{name: "no save after domain", args: []string{"SRE", "--no-save"}},
		{name: "confirm", input: "yes\n", save: true},
		{name: "decline", input: "no\n"},
		{name: "closed input"},
		{name: "conflicting flags", args: []string{"--save", "--dry-run"}, wantErr: "cannot be combined"},
		{name: "conflict after domain", args: []string{"--save", "SRE", "--no-save"}, wantErr: "cannot be combined"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			app := newApp(appOptions{})
			app.Reader = strings.NewReader(tt.input)
			app.Writer = &bytes.Buffer{}
			called := false
			for _, command := range app.Commands {
				if command.Name == "topics" {
					command.Action = func(c *cli.Context) error {
						called = true
						got, err := shouldSaveTopics(c)
						if tt.wantErr != "" {
							if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
								t.Fatalf("error = %v, want %q", err, tt.wantErr)
							}
						} else if err != nil || got != tt.save {
							t.Fatalf("save = %v, error = %v, want %v", got, err, tt.save)
						}
						return nil
					}
				}
			}
			args := append([]string{"articlebot", "--provider", "ollama", "topics"}, tt.args...)
			if err := app.Run(args); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("topics command not called")
			}
		})
	}
}

func TestTopicsCommandInjectedDependencies(t *testing.T) {
	for _, save := range []bool{false, true} {
		save := save
		t.Run(map[bool]string{true: "save", false: "no-save"}[save], func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.json")
			topicsPath := filepath.Join(dir, "topics.json")
			if err := generator.WriteDefaultConfig(configPath); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := newApp(appOptions{
				configPath: configPath,
				getenv:     func(string) string { return "" },
				generator: generator.Options{
					Paths: generator.Paths{Topics: topicsPath},
					NewProvider: func(name generator.AIProvider) (generator.Provider, error) {
						if name != generator.ProviderOllama {
							t.Fatalf("provider = %q, want ollama", name)
						}
						return testProvider{call: func(_, message string) (string, error) {
							calls++
							if !strings.Contains(message, "1. Networking\n") || !strings.Contains(message, "10. Networking\n") {
								t.Fatal("missing explicit domain assignments")
							}
							topics := make([]generator.Topic, 10)
							for i := range topics {
								topics[i] = generator.Topic{Title: "Example", Category: "Networking"}
							}
							data, err := json.Marshal(map[string]interface{}{"topics": topics})
							return string(data), err
						}}, nil
					},
				},
			})
			app.Reader = strings.NewReader("")
			flag := "--no-save"
			if save {
				flag = "--save"
			}
			// Run twice through the same app to catch leaked process arguments or state.
			for i := 0; i < 2; i++ {
				if err := app.Run([]string{"articlebot", "--provider", "ollama", "topics", flag, "Networking"}); err != nil {
					t.Fatal(err)
				}
				flag = "--no-save"
			}
			if calls != 2 {
				t.Fatalf("provider calls = %d, want 2", calls)
			}
			data, err := os.ReadFile(topicsPath)
			if !save {
				if !os.IsNotExist(err) {
					t.Fatalf("no-save wrote topics: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var document struct {
				Topics []generator.Topic `json:"topics"`
			}
			if err := json.Unmarshal(data, &document); err != nil {
				t.Fatal(err)
			}
			if len(document.Topics) != 10 {
				t.Fatalf("got %d saved topics, want 10", len(document.Topics))
			}
		})
	}
}

func TestFilterTopicDomains(t *testing.T) {
	got := filterTopicDomains([]string{"--save", "Platform Engineering", "Kubernetes", " ", "--dry-run"})
	if !reflect.DeepEqual(got, []string{"Platform Engineering", "Kubernetes"}) {
		t.Fatalf("domains = %v", got)
	}
}

func TestTopicsNoninteractiveDefault(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	app := newApp(appOptions{})
	app.Reader = input
	for _, command := range app.Commands {
		if command.Name == "topics" {
			command.Action = func(c *cli.Context) error {
				save, err := shouldSaveTopics(c)
				if err != nil || !save {
					t.Fatalf("save = %v, error = %v, want noninteractive default true", save, err)
				}
				return nil
			}
		}
	}
	if err := app.Run([]string{"articlebot", "topics"}); err != nil {
		t.Fatal(err)
	}
}

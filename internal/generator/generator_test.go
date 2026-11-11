package generator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	config := DefaultConfig()
	gen := New(config, Options{})
	if gen.paths.Topics != TopicsFilename || gen.paths.OutputDir != config.OutputDir {
		t.Fatalf("unexpected default paths: %+v", gen.paths)
	}
	if gen.now == nil || gen.newProvider == nil || gen.usedTopics == nil {
		t.Fatal("missing default dependencies")
	}
}

func TestGenerationDependencies(t *testing.T) {
	for _, mode := range []string{"title", "batch"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			topicsPath := filepath.Join(dir, "source.json")
			outputDir := filepath.Join(dir, "output")
			if err := os.WriteFile(topicsPath, []byte(`{"topics":[{"title":"Example","category":"SRE"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			// A non-UTC date verifies the filename and front matter share one clock value.
			now := time.Date(2031, 2, 3, 0, 0, 1, 0, time.FixedZone("test", 2*60*60))
			config := DefaultConfig()
			config.AIProvider = "claude"
			config.ClaudeAPIKey = "test-claude"
			config.OutputDir = "must-not-be-used"
			providerCalls, clockCalls := 0, 0
			gen := New(config, Options{
				Paths: Paths{Topics: topicsPath, OutputDir: outputDir},
				Now: func() time.Time {
					clockCalls++
					return now
				},
				NewProvider: func(name AIProvider) (Provider, error) {
					if name != ProviderClaude {
						t.Fatalf("unexpected provider: %s", name)
					}
					return topicTestProvider{call: func(system, message string) (string, error) {
						providerCalls++
						if !strings.Contains(system, "technical writer") || !strings.Contains(message, `"Example"`) {
							t.Fatal("provider did not receive the article prompts")
						}
						return "## Conclusion\nThe article is complete.", nil
					}}, nil
				},
			})
			if mode == "title" {
				if err := gen.GenerateArticleByTitle("Example"); err != nil {
					t.Fatal(err)
				}
			} else {
				gen.GenerateArticles(1)
			}
			path := filepath.Join(outputDir, "2031-02-03-example.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "date: 2031-02-03T00:00:01+02:00\n") ||
				!strings.Contains(string(data), "draft: true\n") ||
				!strings.Contains(string(data), "## Conclusion") {
				t.Fatalf("unexpected article:\n%s", data)
			}
			if providerCalls != 1 || clockCalls != 1 {
				t.Fatalf("provider calls = %d, clock calls = %d, want one each", providerCalls, clockCalls)
			}
		})
	}
}

func TestProviderFactoryFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.json")
	if err := os.WriteFile(path, []byte(`{"topics":[{"title":"Example"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("provider construction failed")
	gen := New(DefaultConfig(), Options{
		Paths: Paths{Topics: path},
		NewProvider: func(AIProvider) (Provider, error) {
			return nil, wantErr
		},
	})
	if err := gen.GenerateArticleByTitle("Example"); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if _, err := gen.GenerateTopics([]string{"SRE"}, 1); !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestConnectionsUsesInjectedProviders(t *testing.T) {
	config := DefaultConfig()
	config.OpenAIAPIKey = "test-openai"
	config.ClaudeAPIKey = "test-claude"
	var names []AIProvider
	tests := 0
	gen := New(config, Options{
		NewProvider: func(name AIProvider) (Provider, error) {
			names = append(names, name)
			return connectionTestProvider{test: func() error {
				tests++
				return nil
			}}, nil
		},
	})
	gen.TestConnections()
	if !reflect.DeepEqual(names, []AIProvider{ProviderOpenAI, ProviderClaude}) || tests != 2 {
		t.Fatalf("provider names = %v, test calls = %d", names, tests)
	}
}

type connectionTestProvider struct {
	test func() error
}

func (p connectionTestProvider) Call(string, string) (string, error) {
	return "", errors.New("unexpected generation request")
}

func (p connectionTestProvider) Test() error { return p.test() }

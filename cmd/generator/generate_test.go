package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ubogdan/articlebot/internal/providers"
)

func TestGenerateTitle(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Error(err)
		}
	})
	t.Setenv(EnvOllamaHost, "")
	t.Setenv(EnvOllamaModel, "")
	target := Topic{
		Title: "Reliable incident response", Category: "SRE",
		Keywords:   []string{"incident-response", "reliability"},
		Difficulty: "Advanced", Description: "Build reliable response workflows.",
	}
	data, err := json.Marshal(map[string]interface{}{
		"topics": []Topic{{Title: "Unrelated topic", Category: "Go Development"}, target},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	failProvider := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request providers.OllamaRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if len(request.Messages) != 2 {
			t.Error("expected system and user messages")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		for _, want := range []string{target.Title, target.Category, target.Description, target.Difficulty, "incident-response"} {
			if !strings.Contains(request.Messages[1].Content, want) {
				t.Errorf("generation prompt missing %q", want)
			}
		}
		if failProvider {
			if err := json.NewEncoder(w).Encode(providers.OllamaChatResponse{}); err != nil {
				t.Error(err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(providers.OllamaChatResponse{
			Message: providers.OllamaMessage{Content: "## Conclusion\n\nIncident response requires preparation.\n\nPractice regularly."},
		}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cfg := getDefaultConfig()
	cfg.AIProvider = "claude"
	cfg.ClaudeAPIKey = ""
	cfg.OllamaHost = server.URL
	cfg.GenerationInterval = 8
	cfg.OutputDir = "posts"
	configData, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigFilename, configData, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name        string
		args        []string
		topics      []byte
		missingFile bool
		fail        bool
		wantErr     string
		wantCall    bool
	}{
		{name: "exact title", args: []string{"--title", target.Title}, topics: data, wantCall: true},
		{name: "not found", args: []string{"--title", "Missing"}, topics: data, wantErr: "not found"},
		{name: "case sensitive", args: []string{"--title", strings.ToLower(target.Title)}, topics: data, wantErr: "not found"},
		{name: "empty title", args: []string{"--title", " "}, topics: data, wantErr: "--title must not be empty"},
		{name: "count conflict", args: []string{"--title", target.Title, "2"}, topics: data, wantErr: "cannot be combined"},
		{name: "missing file", args: []string{"--title", target.Title}, missingFile: true, wantErr: "load topics.json"},
		{name: "invalid file", args: []string{"--title", target.Title}, topics: []byte("{"), wantErr: "load topics.json"},
		{name: "provider failure", args: []string{"--title", target.Title}, topics: data, fail: true, wantCall: true, wantErr: "generate article"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.missingFile {
				if err := os.Remove(TopicsFilename); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(TopicsFilename, tt.topics, 0600); err != nil {
				t.Fatal(err)
			}
			before := calls
			failProvider = tt.fail
			args := append([]string{"articlebot", "--provider", "ollama", "generate"}, tt.args...)
			err := newApp().Run(args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantCalls := 0
			if tt.wantCall {
				wantCalls = 1
			}
			if calls-before != wantCalls {
				t.Fatalf("provider calls = %d, want %d", calls-before, wantCalls)
			}
			if !tt.missingFile {
				saved, err := os.ReadFile(TopicsFilename)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(saved, tt.topics) {
					t.Fatal("topics.json was modified")
				}
			}
			if tt.wantErr == "" {
				files, err := filepath.Glob(filepath.Join(cfg.OutputDir, "*.md"))
				if err != nil || len(files) != 1 {
					t.Fatalf("output files = %v, error = %v", files, err)
				}
				content, err := os.ReadFile(files[0])
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{
					"title: " + quoteYAMLString(target.Title),
					"categories: [\"SRE\"]", "draft: true", target.Description,
					"incident-response", "## Conclusion",
				} {
					if !strings.Contains(string(content), want) {
						t.Errorf("saved article missing %q", want)
					}
				}
			}
		})
	}
}

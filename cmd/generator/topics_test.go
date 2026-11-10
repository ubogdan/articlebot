package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ubogdan/articlebot/internal/providers"
)

func TestSelectTopicDomains(t *testing.T) {
	for _, tt := range []struct {
		name    string
		domains []string
		count   int
		wantErr bool
	}{
		{name: "default domains", domains: []string{"Go Development", "DevOps", "SRE", "SecOps"}, count: 10},
		{name: "custom domain", domains: []string{"Networking"}, count: 3},
		{name: "no domains", count: 10, wantErr: true},
		{name: "empty domain", domains: []string{" "}, count: 10, wantErr: true},
		{name: "zero count", domains: []string{"SRE"}, wantErr: true},
		{name: "negative count", domains: []string{"SRE"}, count: -1, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectTopicDomains(tt.domains, tt.count)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != tt.count {
				t.Fatalf("got %d assignments, want %d", len(got), tt.count)
			}
			for _, domain := range got {
				if !contains(tt.domains, domain) {
					t.Fatalf("unexpected domain %q", domain)
				}
			}
		})
	}
}

func TestBuildTopicGenerationPrompt(t *testing.T) {
	gen := &ArticleGenerator{}
	assigned := []string{"SRE", "Networking", "DevOps"}
	prompt := gen.buildTopicGenerationPrompt(assigned)
	for _, want := range []string{
		"Generate exactly 3", "1. SRE\n2. Networking\n3. DevOps\n",
		"Category: Copy the assigned domain exactly",
		"without forcing Go implementations",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	for _, unwanted := range []string{"Generate 10", "Focus on Go programming language where applicable", "AI/ML integration with Go"} {
		if strings.Contains(prompt, unwanted) {
			t.Errorf("prompt contains %q", unwanted)
		}
	}
}

func TestValidateTopicAssignments(t *testing.T) {
	for _, tt := range []struct {
		name       string
		categories []string
		wantErr    bool
	}{
		{name: "matches", categories: []string{"SRE", "DevOps"}},
		{name: "wrong order", categories: []string{"DevOps", "SRE"}, wantErr: true},
		{name: "wrong category", categories: []string{"Go Development", "DevOps"}, wantErr: true},
		{name: "too few", categories: []string{"SRE"}, wantErr: true},
		{name: "too many", categories: []string{"SRE", "DevOps", "SecOps"}, wantErr: true},
		{name: "empty response", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			topics := make([]Topic, len(tt.categories))
			for i, category := range tt.categories {
				topics[i].Category = category
			}
			if err := validateTopicAssignments(topics, []string{"SRE", "DevOps"}); (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateTopicsDomainAssignments(t *testing.T) {
	oldArgs := os.Args
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Args = oldArgs
		if err := os.Chdir(oldDir); err != nil {
			t.Error(err)
		}
	})
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprintf("mismatched=%v", mismatch), func(t *testing.T) {
			var assigned []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request providers.OllamaRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if len(request.Messages) != 2 || request.Messages[0].Content != topicGenerationSystemPrompt {
					t.Error("expected topic-specific system prompt")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_, assignmentBlock, ok := strings.Cut(request.Messages[1].Content, "Domain assignments (one topic per entry, in this exact order):\n")
				if !ok {
					t.Error("missing domain assignments")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				assignmentBlock, _, _ = strings.Cut(assignmentBlock, "\n\n")
				var topics []Topic
				for _, line := range strings.Split(strings.TrimSpace(assignmentBlock), "\n") {
					_, domain, _ := strings.Cut(line, ". ")
					assigned = append(assigned, domain)
					topics = append(topics, Topic{Title: "Domain-native topic", Category: domain})
				}
				if mismatch {
					topics[0].Category = "Go Development"
				}
				content, err := json.Marshal(map[string]interface{}{"topics": topics})
				if err != nil {
					t.Error(err)
					return
				}
				if err := json.NewEncoder(w).Encode(providers.OllamaChatResponse{
					Message: providers.OllamaMessage{Content: string(content)},
				}); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			os.Args = []string{"articlebot", "topics", "--save"}
			gen := &ArticleGenerator{config: getDefaultConfig()}
			gen.config.AIProvider = "ollama"
			gen.config.OllamaHost = server.URL
			err := gen.generateTopicsWithAI([]string{"SRE", "DevOps", "SecOps"}, 3)
			if mismatch {
				if err == nil || !strings.Contains(err.Error(), "does not match assigned domain") {
					t.Fatalf("expected category mismatch, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				saved, err := gen.loadTopics()
				if err != nil {
					t.Fatal(err)
				}
				var categories []string
				for _, topic := range saved {
					categories = append(categories, topic.Category)
				}
				if len(saved) != 3 || !reflect.DeepEqual(categories, assigned) {
					t.Fatalf("saved categories %v, want %v", categories, assigned)
				}
			}
			if mismatch {
				saved, err := gen.loadTopics()
				if err != nil {
					t.Fatal(err)
				}
				if len(saved) != 3 {
					t.Fatal("invalid response changed saved topics")
				}
			}
		})
	}
}

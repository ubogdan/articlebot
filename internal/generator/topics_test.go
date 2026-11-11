package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
	gen := New(DefaultConfig(), Options{Now: func() time.Time {
		return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	}})
	assigned := []string{"SRE", "Networking", "DevOps"}
	prompt := gen.buildTopicGenerationPrompt(assigned)
	for _, want := range []string{
		"Generate exactly 3", "1. SRE\n2. Networking\n3. DevOps\n",
		"Category: Copy the assigned domain exactly",
		"without forcing Go implementations", "2030 trends",
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
	for _, mismatch := range []bool{false, true} {
		mismatch := mismatch
		t.Run(fmt.Sprintf("mismatched=%v", mismatch), func(t *testing.T) {
			t.Parallel()
			var assigned []string
			provider := topicTestProvider{call: func(system, message string) (string, error) {
				if system != topicGenerationSystemPrompt {
					t.Fatal("expected topic-specific system prompt")
				}
				_, assignmentBlock, ok := strings.Cut(message, "Domain assignments (one topic per entry, in this exact order):\n")
				if !ok {
					t.Fatal("missing domain assignments")
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
					return "", err
				}
				return string(content), nil
			}}
			now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			path := filepath.Join(t.TempDir(), "topics.json")
			gen := New(DefaultConfig(), Options{
				Paths:       Paths{Topics: path},
				Now:         func() time.Time { return now },
				NewProvider: func(AIProvider) (Provider, error) { return provider, nil },
			})
			generated, err := gen.GenerateTopics([]string{"SRE", "DevOps", "SecOps"}, 3)
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatal("generating topics must not write a file")
			}
			if mismatch {
				if err == nil || !strings.Contains(err.Error(), "does not match assigned domain") {
					t.Fatalf("expected category mismatch, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := gen.SaveTopics(generated); err != nil {
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
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var document struct {
					GeneratedAt time.Time `json:"generated_at"`
				}
				if err := json.Unmarshal(data, &document); err != nil {
					t.Fatal(err)
				}
				if !document.GeneratedAt.Equal(now) {
					t.Fatalf("generated_at = %v, want %v", document.GeneratedAt, now)
				}
			}
		})
	}
}

type topicTestProvider struct {
	call func(string, string) (string, error)
}

func (p topicTestProvider) Call(system, message string) (string, error) {
	return p.call(system, message)
}

func (p topicTestProvider) Test() error { return nil }

func TestSaveTopicsAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.json")
	gen := New(DefaultConfig(), Options{Paths: Paths{Topics: path}})
	for _, title := range []string{"First", "Second"} {
		if err := gen.SaveTopics([]Topic{{Title: title}}); err != nil {
			t.Fatal(err)
		}
	}
	topics, err := gen.loadTopics()
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 || topics[0].Title != "First" || topics[1].Title != "Second" {
		t.Fatalf("unexpected saved topics: %+v", topics)
	}
}

func TestSaveTopicsPreservesInvalidFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "topics.json")
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	gen := New(DefaultConfig(), Options{Paths: Paths{Topics: path}})
	if err := gen.SaveTopics([]Topic{{Title: "Example"}}); err == nil {
		t.Fatal("expected invalid topics error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{" {
		t.Fatal("invalid existing topics were overwritten")
	}
}

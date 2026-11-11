package generator

import (
	"fmt"
	"strings"
	"testing"
)

type articleTestProvider struct {
	calls int
	call  func(int, string) (string, error)
}

func (p *articleTestProvider) Call(system, message string) (string, error) {
	p.calls++
	return p.call(p.calls, message)
}

func (p *articleTestProvider) Test() error { return nil }

func TestArticleContinuation(t *testing.T) {
	complete := "## Conclusion\nThe article is complete."
	for _, tt := range []struct {
		name      string
		responses []string
		fail      bool
		wantCalls int
		wantErr   string
	}{
		{name: "complete", responses: []string{complete}, wantCalls: 1},
		{name: "continued", responses: []string{"Introduction.\n\n", complete}, wantCalls: 2},
		{name: "empty article", responses: []string{" \n"}, wantCalls: 1, wantErr: "empty article content"},
		{name: "empty continuation", responses: []string{"Introduction.\n\n", " "}, wantCalls: 2, wantErr: "empty article continuation"},
		{name: "duplicate continuation", responses: []string{"Introduction.", "Introduction."}, wantCalls: 2, wantErr: "repeated existing content"},
		{name: "provider failure", responses: []string{"Introduction."}, fail: true, wantCalls: 2, wantErr: "request article continuation"},
		{name: "limit", responses: []string{"Initial.", " More.", " Still more.", " Yet more."}, wantCalls: 1 + maxArticleContinuations, wantErr: "still incomplete after 3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := &articleTestProvider{call: func(call int, message string) (string, error) {
				if call > 1 && !strings.Contains(message, tt.responses[0]) {
					t.Error("continuation prompt lost existing article")
				}
				if tt.fail && call > 1 {
					return "", fmt.Errorf("provider unavailable")
				}
				if call > len(tt.responses) {
					t.Fatal("unexpected extra provider call")
				}
				return tt.responses[call-1], nil
			}}
			gen := New(DefaultConfig(), Options{})
			article, err := gen.generateArticleWithAI(provider, Topic{Title: "Example"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || article != nil {
					t.Fatalf("article=%v error=%v, want %q", article, err, tt.wantErr)
				}
			} else if err != nil || article == nil || !strings.Contains(article.Content, complete) {
				t.Fatalf("article=%v error=%v", article, err)
			}
			if provider.calls != tt.wantCalls {
				t.Fatalf("calls=%d, want %d", provider.calls, tt.wantCalls)
			}
		})
	}
}

func TestArticleCompleteness(t *testing.T) {
	gen := &ArticleGenerator{}
	for _, tt := range []struct {
		name       string
		content    string
		incomplete bool
	}{
		{name: "whitespace", content: " \n\t", incomplete: true},
		{name: "conclusion with adjacent paragraph", content: "## Conclusion\nFinished."},
		{name: "body mentions work in progress", content: "Avoid deploying work in progress.\n\n## Conclusion\nFinished."},
		{name: "truncation marker", content: "## Conclusion\nTo be continued.", incomplete: true},
		{name: "closed continuation", content: "To be continued.\n\n## Conclusion\nFinished."},
		{name: "unclosed fence", content: "## Conclusion\n```go\nfmt.Println()", incomplete: true},
		{name: "heading without body", content: strings.Repeat("word ", 1200) + "\n## Conclusion", incomplete: true},
		{name: "bold final sentence", content: "## Conclusion\nThis is the last sentence with **important advice.**"},
		{name: "italic final sentence", content: "## Conclusion\nThis is the last sentence with _important advice._"},
		{name: "unicode closing quote", content: "## Conclusion\nThe final recommendation is \u201ckeep it simple.\u201d"},
		{name: "resource link", content: "## Conclusion\nFinished.\n\n## Resources\n[OpenTofu documentation](https://opentofu.org/docs/)"},
		{name: "table ending", content: "## Conclusion\nFinished.\n\n| Tool | Purpose |\n| --- | --- |\n| OpenTofu | Provision infrastructure |"},
		{name: "inline fence literal", content: "## Conclusion\nUse the inline marker ` ``` ` only when demonstrating Markdown."},
		{name: "nested fences", content: "## Conclusion\n````markdown\n```go\nfmt.Println()\n```\n````"},
		{name: "tilde fence", content: "## Conclusion\n~~~go\nfmt.Println()\n~~~"},
		{name: "unclosed tilde fence", content: "## Conclusion\n~~~go\nfmt.Println()", incomplete: true},
		{name: "short closing fence", content: "## Conclusion\n````go\nfmt.Println()\n```", incomplete: true},
		{name: "closing fence with info", content: "## Conclusion\n```go\nfmt.Println()\n```go", incomplete: true},
		{name: "conclusion inside code", content: "```markdown\n## Conclusion\n```\nNot enough words.", incomplete: true},
		{name: "level one conclusion", content: "# Conclusion\nFinished."},
		{name: "unfinished prose", content: "## Conclusion\nThe remaining important consideration is", incomplete: true},
		{name: "unfinished bold prose", content: "## Conclusion\nThe remaining important **consideration is**", incomplete: true},
		{name: "unfinished bold list", content: "## Conclusion\n- **Remember the following:**", incomplete: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := gen.isArticleIncomplete(tt.content); got != tt.incomplete {
				t.Fatalf("incomplete=%v, want %v", got, tt.incomplete)
			}
		})
	}
}

func TestArticleContinuationBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name         string
		initial      string
		continuation string
		want         string
	}{
		{
			name:         "close existing fence",
			initial:      "## Conclusion\n```go\nfmt.Println()",
			continuation: "```\n\nFinished.",
			want:         "fmt.Println()\n```\n\nFinished.",
		},
		{
			name:         "heading after fence",
			initial:      "```go\nfmt.Println()\n```",
			continuation: "## Conclusion\nFinished.",
			want:         "```\n## Conclusion\nFinished.",
		},
		{
			name:         "partial word",
			initial:      "## Conclusion\nThe remaining important consideration is reliab",
			continuation: "ility.",
			want:         "The remaining important consideration is reliability.",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			reason := articleIncompleteReason(tt.initial)
			if reason == "" {
				t.Fatal("expected incomplete initial response")
			}
			provider := &articleTestProvider{call: func(call int, message string) (string, error) {
				switch call {
				case 1:
					if !strings.Contains(message, "1200-2000") || strings.Contains(message, "1500-2500") {
						t.Error("expected consistent word budget")
					}
					return tt.initial, nil
				case 2:
					if !strings.Contains(message, reason) || !strings.Contains(message, tt.initial) {
						t.Error("continuation prompt missing failure reason or article")
					}
					return tt.continuation, nil
				default:
					t.Fatal("unexpected extra continuation")
					return "", nil
				}
			}}
			gen := New(DefaultConfig(), Options{})
			article, err := gen.generateArticleWithAI(provider, Topic{Title: "Example"})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(article.Content, tt.want) {
				t.Fatalf("article content %q missing %q", article.Content, tt.want)
			}
		})
	}
}

func TestArticleContinuationLimitReason(t *testing.T) {
	provider := &articleTestProvider{call: func(call int, _ string) (string, error) {
		if call == 1 {
			return "## Conclusion\n```go\n", nil
		}
		return fmt.Sprintf("line%d\n", call), nil
	}}
	gen := New(DefaultConfig(), Options{})
	_, err := gen.generateArticleWithAI(provider, Topic{Title: "Example"})
	if err == nil || !strings.Contains(err.Error(), "after 3 continuation attempts: unclosed ``` code fence") {
		t.Fatalf("error = %v, want continuation limit with reason", err)
	}
}

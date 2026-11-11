package providers

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type claudeTestTransport func(*http.Request) (*http.Response, error)

func (f claudeTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClaudeCallTextBlocks(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "text", body: `{"content":[{"type":"text","text":"Article."}]}`, want: "Article."},
		{name: "thinking before text", body: `{"content":[{"type":"thinking","thinking":"Reasoning"},{"type":"text","text":"Article."}]}`, want: "Article."},
		{name: "multiple text blocks", body: `{"content":[{"type":"text","text":"First."},{"type":"tool_use"},{"type":"text","text":" Second."}]}`, want: "First. Second."},
		{name: "only thinking", body: `{"content":[{"type":"thinking","thinking":"Reasoning"}]}`, wantErr: true},
		{name: "blank text", body: `{"content":[{"type":"text","text":"  "}]}`, wantErr: true},
		{name: "empty blocks", body: `{"content":[]}`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ai := NewClaudeAI("test-key")
			ai.httpClient.Transport = claudeTestTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
			})
			got, err := ai.Call("System", "Prompt")
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("Call() = %q, %v; want %q, error=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

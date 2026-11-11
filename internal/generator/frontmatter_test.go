package generator

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewFrontMatterPreservesExistingOutput(t *testing.T) {
	topic := Topic{
		Title:       "Building Reliable Services",
		Category:    "Go Development",
		Keywords:    []string{"go", "reliability"},
		Description: "A practical guide to reliable services.",
	}

	got := NewFrontMatter(topic, "# Building Reliable Services\nArticle body.\n", true, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC))
	want := strings.Join([]string{
		"---",
		`title: "Building Reliable Services"`,
		"date: " + got.Date,
		"draft: true",
		`tags: ["go", "reliability"]`,
		`categories: ["Go Development"]`,
		`description: "A practical guide to reliable services."`,
		"---",
		"",
		"Article body.",
		"",
	}, "\n")

	if got.Content != want {
		t.Fatalf("NewFrontMatter output changed:\n got:\n%s\nwant:\n%s", got.Content, want)
	}
}

func TestNewFrontMatterEscapesMetadata(t *testing.T) {
	topic := Topic{
		Title:       "A \"quoted\" title\nwith a newline",
		Category:    "Guides: \"advanced\"",
		Keywords:    []string{`one "quote"`, `back\slash`, "line\nbreak", "comma, and colon: value", "[brackets] #hash"},
		Description: "Description with \"quotes\", \\slashes,\nand a newline.",
	}

	got := NewFrontMatter(topic, "Article body.", false, time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC))
	for _, expected := range []string{
		`title: "A \"quoted\" title\nwith a newline"`,
		`tags: ["one \"quote\"", "back\\slash", "line\nbreak", "comma, and colon: value", "[brackets] #hash"]`,
		`categories: ["Guides: \"advanced\""]`,
		`description: "Description with \"quotes\", \\slashes,\nand a newline."`,
	} {
		if !strings.Contains(got.Content, expected+"\n") {
			t.Errorf("front matter does not contain escaped line %q:\n%s", expected, got.Content)
		}
	}

	lines := strings.Split(got.Content, "\n")
	values := make(map[string]string, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "---") {
			continue
		}
		key, value, found := strings.Cut(line, ": ")
		if found {
			values[key] = value
		}
	}

	var title, description string
	if err := json.Unmarshal([]byte(values["title"]), &title); err != nil {
		t.Fatalf("title is not a valid quoted scalar: %v", err)
	}
	if err := json.Unmarshal([]byte(values["description"]), &description); err != nil {
		t.Fatalf("description is not a valid quoted scalar: %v", err)
	}
	var tags, categories []string
	if err := json.Unmarshal([]byte(values["tags"]), &tags); err != nil {
		t.Fatalf("tags are not a valid flow sequence: %v", err)
	}
	if err := json.Unmarshal([]byte(values["categories"]), &categories); err != nil {
		t.Fatalf("categories are not a valid flow sequence: %v", err)
	}

	if title != topic.Title || description != topic.Description ||
		strings.Join(tags, "\x00") != strings.Join(topic.Keywords, "\x00") ||
		len(categories) != 1 || categories[0] != topic.Category {
		t.Fatalf("metadata did not round-trip: title=%q description=%q tags=%q categories=%q",
			title, description, tags, categories)
	}
}

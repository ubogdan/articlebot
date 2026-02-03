package analyzer

import (
	"testing"

	"github.com/ubogdan/articlebot/internal/parser"
)

func TestAnalyzeTypos(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantTypo string
	}{
		{
			name:     "common typo - alot",
			content:  "There are alot of things to do.",
			wantTypo: "alot",
		},
		{
			name:     "preview vs previous",
			content:  "The preview settings need updating.",
			wantTypo: "preview",
		},
		{
			name:     "no typos",
			content:  "This sentence is correctly written.",
			wantTypo: "",
		},
	}

	cfg := DefaultConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := &parser.Article{
				Content: tt.content,
				FrontMatter: parser.FrontMatter{
					Title:       "Test",
					Date:        "2024-01-01",
					Description: "Test description",
				},
			}

			issues := Analyze(article, cfg)

			foundTypo := false
			for _, issue := range issues {
				if issue.Type == IssueTypo && tt.wantTypo != "" {
					foundTypo = true
					break
				}
			}

			if tt.wantTypo != "" && !foundTypo {
				t.Errorf("Expected to find typo '%s', but didn't", tt.wantTypo)
			}
			if tt.wantTypo == "" && foundTypo {
				t.Errorf("Expected no typos, but found some")
			}
		})
	}
}

func TestAnalyzeCodeTypos(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		language string
		wantTypo string
	}{
		{
			name:     "terraform vars typo",
			code:     `bucket = vars.bucket_name`,
			language: "terraform",
			wantTypo: "vars.",
		},
		{
			name:     "xen config typo",
			code:     `other-confi:auto_poweron=true`,
			language: "bash",
			wantTypo: "other-confi:",
		},
		{
			name:     "lsof typo",
			code:     `lsoff -p 1234`,
			language: "bash",
			wantTypo: "lsoff",
		},
		{
			name:     "correct code",
			code:     `var.bucket_name`,
			language: "terraform",
			wantTypo: "",
		},
	}

	cfg := DefaultConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := &parser.Article{
				Content: "```" + tt.language + "\n" + tt.code + "\n```",
				CodeBlocks: []parser.CodeBlock{
					{
						Language: tt.language,
						Content:  tt.code,
						Line:     1,
					},
				},
				FrontMatter: parser.FrontMatter{
					Title:       "Test",
					Date:        "2024-01-01",
					Description: "Test",
				},
			}

			issues := Analyze(article, cfg)

			foundTypo := false
			for _, issue := range issues {
				if issue.Type == IssueCodeSyntax && tt.wantTypo != "" {
					foundTypo = true
					break
				}
			}

			if tt.wantTypo != "" && !foundTypo {
				t.Errorf("Expected to find code typo '%s', but didn't", tt.wantTypo)
			}
			if tt.wantTypo == "" && foundTypo {
				t.Errorf("Expected no code typos, but found some")
			}
		})
	}
}

func TestAnalyzeMissingMetadata(t *testing.T) {
	cfg := DefaultConfig()

	article := &parser.Article{
		Content: "Some content here.",
		FrontMatter: parser.FrontMatter{
			Title: "Test Title",
			Date:  "2024-01-01",
			// Missing: Description
		},
	}

	issues := Analyze(article, cfg)

	foundMissingDesc := false
	for _, issue := range issues {
		if issue.Type == IssueMissingMeta && issue.Message == "Missing front matter field: description" {
			foundMissingDesc = true
			break
		}
	}

	if !foundMissingDesc {
		t.Error("Expected to find missing description issue")
	}
}

func TestAnalyzeLinks(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantIssue bool
		issueType IssueType
	}{
		{
			name:      "http link",
			content:   "Check out [this link](http://example.com)",
			wantIssue: true,
			issueType: IssueBrokenLink,
		},
		{
			name:      "https link",
			content:   "Check out [this link](https://example.com)",
			wantIssue: false,
		},
		{
			name:      "placeholder link",
			content:   "Check out [this link](#)",
			wantIssue: true,
			issueType: IssueBrokenLink,
		},
		{
			name:      "internal link with space",
			content:   "Check out [this](/path with space)",
			wantIssue: true,
			issueType: IssueBrokenLink,
		},
	}

	cfg := DefaultConfig()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article, _ := parser.Parse("test.md", []byte("---\ntitle: Test\ndate: 2024-01-01\ndescription: Test\n---\n"+tt.content))

			issues := Analyze(article, cfg)

			found := false
			for _, issue := range issues {
				if issue.Type == tt.issueType {
					found = true
					break
				}
			}

			if tt.wantIssue && !found {
				t.Errorf("Expected to find %s issue, but didn't", tt.issueType)
			}
			if !tt.wantIssue && found {
				t.Errorf("Expected no %s issue, but found one", tt.issueType)
			}
		})
	}
}

func TestAnalyzeOutdated(t *testing.T) {
	cfg := DefaultConfig()

	article := &parser.Article{
		Content: `
Here's a terraform config:

` + "```terraform" + `
terraform {
  required_version = ">= 0.14"
  required_providers {
    aws = {
      source = "hashicorp/aws"
      version = "~> 3.0"
    }
  }
}
` + "```",
		FrontMatter: parser.FrontMatter{
			Title:       "Test",
			Date:        "2024-01-01",
			Description: "Test",
		},
	}

	issues := Analyze(article, cfg)

	foundOutdated := false
	for _, issue := range issues {
		if issue.Type == IssueOutdated {
			foundOutdated = true
			break
		}
	}

	if !foundOutdated {
		t.Error("Expected to find outdated content issue")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MinWordCount <= 0 {
		t.Error("MinWordCount should be positive")
	}

	if len(cfg.CommonTypos) == 0 {
		t.Error("CommonTypos should not be empty")
	}

	if len(cfg.CodeTypos) == 0 {
		t.Error("CodeTypos should not be empty")
	}

	if len(cfg.ValidLanguages) == 0 {
		t.Error("ValidLanguages should not be empty")
	}
}

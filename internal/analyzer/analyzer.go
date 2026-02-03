package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ubogdan/articlebot/internal/parser"
)

// Severity levels for issues
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// IssueType categorizes issues
type IssueType string

const (
	IssueTypo           IssueType = "typo"
	IssueCodeSyntax     IssueType = "code_syntax"
	IssueMissingMeta    IssueType = "missing_meta"
	IssueMissingSection IssueType = "missing_section"
	IssueBrokenLink     IssueType = "broken_link"
	IssueOutdated       IssueType = "outdated"
	IssueStyle          IssueType = "style"
	IssueStructure      IssueType = "structure"
)

// Issue represents a detected problem
type Issue struct {
	Type        IssueType
	Severity    Severity
	Line        int
	Column      int
	Message     string
	Suggestion  string
	AutoFixable bool
	FixData     map[string]string // Data needed for auto-fix
}

// Config holds analyzer configuration
type Config struct {
	MinWordCount        int               `json:"min_word_count"`
	MaxWordCount        int               `json:"max_word_count"`
	RequiredSections    []string          `json:"required_sections"`
	RequiredFrontMatter []string          `json:"required_front_matter"`
	CommonTypos         map[string]string `json:"common_typos"`
	CodeTypos           map[string]string `json:"code_typos"`
	OutdatedPatterns    []OutdatedPattern `json:"outdated_patterns"`
	InternalLinkPrefix  string            `json:"internal_link_prefix"`
	ValidLanguages      []string          `json:"valid_languages"`
}

// OutdatedPattern defines patterns that indicate outdated content
type OutdatedPattern struct {
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
	Message     string `json:"message"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		MinWordCount:        300,
		MaxWordCount:        5000,
		RequiredSections:    []string{},
		RequiredFrontMatter: []string{"title", "date", "description"},
		CommonTypos: map[string]string{
			"teh":        "the",
			"taht":       "that",
			"recieve":    "receive",
			"occured":    "occurred",
			"seperate":   "separate",
			"definately": "definitely",
			"alot":       "a lot",
			"cant":       "can't",
			"wont":       "won't",
			"dont":       "don't",
			"doesnt":     "doesn't",
			"isnt":       "isn't",
			"arent":      "aren't",
			"wasnt":      "wasn't",
			"werent":     "weren't",
			"havent":     "haven't",
			"hasnt":      "hasn't",
			"hadnt":      "hadn't",
			"wouldnt":    "wouldn't",
			"couldnt":    "couldn't",
			"shouldnt":   "shouldn't",
			"preview":    "previous", // Common in tech docs
			"figure out": "discover", // Style suggestion
		},
		CodeTypos: map[string]string{
			// Go typos
			"fucn":     "func",
			"funcion":  "func",
			"reutrn":   "return",
			"retrun":   "return",
			"stirng":   "string",
			"strign":   "string",
			"interfce": "interface",
			"strcut":   "struct",
			"pakage":   "package",
			"improt":   "import",
			"ipmort":   "import",
			// Terraform typos
			"vars.":    "var.", // vars.bucket_name -> var.bucket_name
			"resoruce": "resource",
			"provder":  "provider",
			"varialbe": "variable",
			// Shell typos
			"lsoff": "lsof",
			"grpe":  "grep",
			"cta":   "cat",
			"ehco":  "echo",
			"mkidr": "mkdir",
			"chown": "chown",
			"chmdo": "chmod",
			// Xen specific
			"other-confi:": "other-config:",
		},
		OutdatedPatterns: []OutdatedPattern{
			{
				Pattern:     `version\s*=\s*"~>\s*3\.0"`,
				Replacement: `version = "~> 5.0"`,
				Message:     "AWS provider 3.x is outdated, consider upgrading to 5.x",
			},
			{
				Pattern:     `required_version\s*=\s*">=\s*0\.14"`,
				Replacement: `required_version = ">= 1.0"`,
				Message:     "Terraform 0.14 is outdated, minimum should be 1.0+",
			},
			{
				Pattern: `github\.com/gorilla/mux`,
				Message: "Consider noting that gorilla/mux is in maintenance mode, chi or stdlib might be alternatives",
			},
		},
		InternalLinkPrefix: "/",
		ValidLanguages: []string{
			"go", "golang", "bash", "sh", "shell", "python", "py",
			"javascript", "js", "typescript", "ts", "json", "yaml", "yml",
			"terraform", "tf", "hcl", "sql", "html", "css", "markdown", "md",
			"dockerfile", "docker", "nginx", "apache", "xml", "toml", "ini",
			"ruby", "rb", "rust", "rs", "java", "kotlin", "swift", "c", "cpp",
			"csharp", "cs", "php", "perl", "lua", "r", "scala", "groovy",
			"powershell", "ps1", "batch", "bat", "vim", "diff", "plaintext",
		},
	}
}

// LoadConfig loads config from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Analyze runs all analyzers on an article
func Analyze(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue

	// Run all analyzers
	issues = append(issues, analyzeMetadata(article, cfg)...)
	issues = append(issues, analyzeContent(article, cfg)...)
	issues = append(issues, analyzeCodeBlocks(article, cfg)...)
	issues = append(issues, analyzeLinks(article, cfg)...)
	issues = append(issues, analyzeStructure(article, cfg)...)
	issues = append(issues, analyzeOutdated(article, cfg)...)

	return issues
}

// analyzeMetadata checks front matter
func analyzeMetadata(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue

	// Check required fields
	for _, field := range cfg.RequiredFrontMatter {
		missing := false
		switch field {
		case "title":
			missing = article.FrontMatter.Title == ""
		case "date":
			missing = article.FrontMatter.Date == ""
		case "description":
			missing = article.FrontMatter.Description == ""
		case "tags":
			missing = len(article.FrontMatter.Tags) == 0
		case "categories":
			missing = len(article.FrontMatter.Categories) == 0
		case "author":
			missing = article.FrontMatter.Author == ""
		}

		if missing {
			issues = append(issues, Issue{
				Type:        IssueMissingMeta,
				Severity:    SeverityWarning,
				Line:        1,
				Message:     "Missing front matter field: " + field,
				Suggestion:  "Add '" + field + "' to your front matter",
				AutoFixable: field == "description", // We can auto-generate descriptions
			})
		}
	}

	// Check for lastmod if article is old
	if article.FrontMatter.Date != "" && article.FrontMatter.LastMod == "" {
		// Simple check - if year is before current year
		if strings.Contains(article.FrontMatter.Date, "2021") ||
			strings.Contains(article.FrontMatter.Date, "2022") ||
			strings.Contains(article.FrontMatter.Date, "2023") {
			issues = append(issues, Issue{
				Type:        IssueMissingMeta,
				Severity:    SeverityInfo,
				Line:        1,
				Message:     "Old article without lastmod date",
				Suggestion:  "Add 'lastmod' field to indicate when content was last reviewed",
				AutoFixable: false,
			})
		}
	}

	return issues
}

// analyzeContent checks for typos and style issues
func analyzeContent(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue
	lines := strings.Split(article.Content, "\n")
	inCodeBlock := false

	for i, line := range lines {
		lineNum := i + 1

		// Track code blocks to skip them for prose typos
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}

		if inCodeBlock {
			continue
		}

		// Check for common typos in prose
		lowerLine := strings.ToLower(line)
		for typo, correction := range cfg.CommonTypos {
			if strings.Contains(lowerLine, typo) {
				// Make sure it's a word boundary match
				pattern := `\b` + regexp.QuoteMeta(typo) + `\b`
				if matched, _ := regexp.MatchString("(?i)"+pattern, line); matched {
					issues = append(issues, Issue{
						Type:        IssueTypo,
						Severity:    SeverityWarning,
						Line:        lineNum,
						Message:     "Possible typo: '" + typo + "'",
						Suggestion:  "Did you mean '" + correction + "'?",
						AutoFixable: true,
						FixData: map[string]string{
							"find":    typo,
							"replace": correction,
						},
					})
				}
			}
		}

		// Check for repeated words (simple approach without backreferences)
		words := strings.Fields(line)
		for i := 1; i < len(words); i++ {
			// Clean words for comparison (remove punctuation)
			w1 := strings.ToLower(strings.Trim(words[i-1], ".,;:!?\"'()[]{}"))
			w2 := strings.ToLower(strings.Trim(words[i], ".,;:!?\"'()[]{}"))

			if w1 == w2 && len(w1) > 2 { // Skip very short words like "a a"
				issues = append(issues, Issue{
					Type:        IssueTypo,
					Severity:    SeverityWarning,
					Line:        lineNum,
					Message:     "Repeated word: '" + words[i] + "'",
					Suggestion:  "Remove the duplicate word",
					AutoFixable: true,
					FixData: map[string]string{
						"find":    words[i-1] + " " + words[i],
						"replace": words[i],
					},
				})
			}
		}
	}

	// Check word count
	wordCount := article.WordCount()
	if wordCount < cfg.MinWordCount {
		issues = append(issues, Issue{
			Type:        IssueStructure,
			Severity:    SeverityInfo,
			Line:        1,
			Message:     "Article may be too short",
			Suggestion:  fmt.Sprintf("Consider expanding the content. Current: %d words, recommended minimum: %d", wordCount, cfg.MinWordCount),
			AutoFixable: false,
		})
	}

	if cfg.MaxWordCount > 0 && wordCount > cfg.MaxWordCount {
		issues = append(issues, Issue{
			Type:        IssueStructure,
			Severity:    SeverityInfo,
			Line:        1,
			Message:     "Article may be too long",
			Suggestion:  "Consider splitting into multiple articles or adding a table of contents",
			AutoFixable: false,
		})
	}

	return issues
}

// analyzeCodeBlocks checks code syntax
func analyzeCodeBlocks(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue

	for _, block := range article.CodeBlocks {
		// Check for missing language specification
		if block.Language == "" {
			issues = append(issues, Issue{
				Type:        IssueCodeSyntax,
				Severity:    SeverityWarning,
				Line:        block.Line,
				Message:     "Code block without language specification",
				Suggestion:  "Add a language identifier after ``` for syntax highlighting",
				AutoFixable: false,
			})
		} else {
			// Check if language is valid
			isValid := false
			for _, lang := range cfg.ValidLanguages {
				if strings.EqualFold(block.Language, lang) {
					isValid = true
					break
				}
			}
			if !isValid {
				issues = append(issues, Issue{
					Type:        IssueCodeSyntax,
					Severity:    SeverityInfo,
					Line:        block.Line,
					Message:     "Unknown language identifier: " + block.Language,
					Suggestion:  "Verify this is a valid language for syntax highlighting",
					AutoFixable: false,
				})
			}
		}

		// Check for common code typos
		for typo, correction := range cfg.CodeTypos {
			if strings.Contains(block.Content, typo) {
				issues = append(issues, Issue{
					Type:        IssueCodeSyntax,
					Severity:    SeverityError,
					Line:        block.Line,
					Message:     "Possible code typo: '" + typo + "'",
					Suggestion:  "Did you mean '" + correction + "'?",
					AutoFixable: true,
					FixData: map[string]string{
						"find":    typo,
						"replace": correction,
						"scope":   "code",
					},
				})
			}
		}

		// Language-specific checks
		switch strings.ToLower(block.Language) {
		case "go", "golang":
			issues = append(issues, analyzeGoCode(block)...)
		case "bash", "sh", "shell":
			issues = append(issues, analyzeBashCode(block)...)
		case "terraform", "tf", "hcl":
			issues = append(issues, analyzeTerraformCode(block)...)
		}
	}

	return issues
}

// analyzeGoCode performs Go-specific code checks
func analyzeGoCode(block parser.CodeBlock) []Issue {
	var issues []Issue

	// Check for common Go issues
	if strings.Contains(block.Content, "func main") && !strings.Contains(block.Content, "package") {
		issues = append(issues, Issue{
			Type:        IssueCodeSyntax,
			Severity:    SeverityWarning,
			Line:        block.Line,
			Message:     "Go code with main func but missing package declaration",
			Suggestion:  "Add 'package main' at the start of the code block",
			AutoFixable: false,
		})
	}

	// Check for error handling
	if strings.Contains(block.Content, ", err :=") || strings.Contains(block.Content, ", err =") {
		if !strings.Contains(block.Content, "if err != nil") &&
			!strings.Contains(block.Content, "if err !=nil") {
			issues = append(issues, Issue{
				Type:        IssueCodeSyntax,
				Severity:    SeverityInfo,
				Line:        block.Line,
				Message:     "Error variable assigned but not checked",
				Suggestion:  "Consider adding error handling with 'if err != nil'",
				AutoFixable: false,
			})
		}
	}

	return issues
}

// analyzeBashCode performs shell-specific checks
func analyzeBashCode(block parser.CodeBlock) []Issue {
	var issues []Issue

	// Check for unsafe practices
	if strings.Contains(block.Content, "rm -rf /") {
		issues = append(issues, Issue{
			Type:        IssueCodeSyntax,
			Severity:    SeverityError,
			Line:        block.Line,
			Message:     "Dangerous command: rm -rf /",
			Suggestion:  "This command is dangerous and should not be in documentation",
			AutoFixable: false,
		})
	}

	// Check for sudo without explanation
	if strings.Contains(block.Content, "sudo") {
		// This is just informational
		issues = append(issues, Issue{
			Type:        IssueCodeSyntax,
			Severity:    SeverityInfo,
			Line:        block.Line,
			Message:     "Command requires sudo/root privileges",
			Suggestion:  "Consider noting that root privileges are required",
			AutoFixable: false,
		})
	}

	return issues
}

// analyzeTerraformCode performs Terraform-specific checks
func analyzeTerraformCode(block parser.CodeBlock) []Issue {
	var issues []Issue

	// Check for hardcoded credentials
	credentialPatterns := []string{
		`access_key\s*=\s*"[^"]+`,
		`secret_key\s*=\s*"[^"]+`,
		`password\s*=\s*"[^"]+`,
		`api_key\s*=\s*"[^"]+`,
	}

	for _, pattern := range credentialPatterns {
		if matched, _ := regexp.MatchString(pattern, block.Content); matched {
			issues = append(issues, Issue{
				Type:        IssueCodeSyntax,
				Severity:    SeverityError,
				Line:        block.Line,
				Message:     "Possible hardcoded credentials in Terraform code",
				Suggestion:  "Use variables or environment variables for sensitive values",
				AutoFixable: false,
			})
			break
		}
	}

	return issues
}

// analyzeLinks checks for broken or problematic links
func analyzeLinks(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue

	for _, link := range article.Links {
		if link.IsInCode {
			continue // Skip links inside code blocks
		}

		// Check for empty links
		if link.URL == "" {
			issues = append(issues, Issue{
				Type:        IssueBrokenLink,
				Severity:    SeverityError,
				Line:        link.Line,
				Message:     "Empty link URL",
				Suggestion:  "Add a valid URL or remove the link",
				AutoFixable: false,
			})
			continue
		}

		// Check for placeholder links
		if link.URL == "#" || link.URL == "TODO" || strings.Contains(link.URL, "example.com") {
			issues = append(issues, Issue{
				Type:        IssueBrokenLink,
				Severity:    SeverityWarning,
				Line:        link.Line,
				Message:     "Placeholder link: " + link.URL,
				Suggestion:  "Replace with actual URL",
				AutoFixable: false,
			})
		}

		// Check internal links format
		if strings.HasPrefix(link.URL, "/") {
			// Internal link - check for common issues
			if strings.Contains(link.URL, " ") {
				issues = append(issues, Issue{
					Type:        IssueBrokenLink,
					Severity:    SeverityError,
					Line:        link.Line,
					Message:     "Internal link contains spaces",
					Suggestion:  "URL encode spaces or use hyphens",
					AutoFixable: true,
					FixData: map[string]string{
						"find":    link.URL,
						"replace": strings.ReplaceAll(link.URL, " ", "-"),
					},
				})
			}
		}

		// Check for http (non-https) links
		if strings.HasPrefix(link.URL, "http://") && !strings.Contains(link.URL, "localhost") {
			issues = append(issues, Issue{
				Type:        IssueBrokenLink,
				Severity:    SeverityWarning,
				Line:        link.Line,
				Message:     "Non-HTTPS link: " + link.URL,
				Suggestion:  "Consider using HTTPS if available",
				AutoFixable: true,
				FixData: map[string]string{
					"find":    "http://",
					"replace": "https://",
				},
			})
		}
	}

	return issues
}

// analyzeStructure checks article structure
func analyzeStructure(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue

	// Check for required sections
	for _, required := range cfg.RequiredSections {
		if !article.HasSectionContaining(required) {
			issues = append(issues, Issue{
				Type:        IssueMissingSection,
				Severity:    SeverityWarning,
				Line:        1,
				Message:     "Missing recommended section: " + required,
				Suggestion:  "Consider adding a '" + required + "' section",
				AutoFixable: false,
			})
		}
	}

	// Check header hierarchy
	lastLevel := 0
	for _, section := range article.Sections {
		if lastLevel > 0 && section.Level > lastLevel+1 {
			issues = append(issues, Issue{
				Type:        IssueStructure,
				Severity:    SeverityWarning,
				Line:        section.Line,
				Message:     "Skipped heading level",
				Suggestion:  fmt.Sprintf("Headings should not skip levels (e.g., H2 -> H4). Use H%d instead of H%d", lastLevel+1, section.Level),
				AutoFixable: false,
			})
		}
		lastLevel = section.Level
	}

	// Check for very long sections without subheadings
	for _, section := range article.Sections {
		wordCount := len(strings.Fields(section.Content))
		if wordCount > 500 && section.Level < 3 {
			issues = append(issues, Issue{
				Type:        IssueStructure,
				Severity:    SeverityInfo,
				Line:        section.Line,
				Message:     fmt.Sprintf("Long section without subheadings (%d words)", wordCount),
				Suggestion:  "Consider breaking this section into smaller subsections",
				AutoFixable: false,
			})
		}
	}

	return issues
}

// analyzeOutdated checks for outdated content
func analyzeOutdated(article *parser.Article, cfg *Config) []Issue {
	var issues []Issue
	content := article.Content

	for _, pattern := range cfg.OutdatedPatterns {
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			continue
		}

		if loc := re.FindStringIndex(content); loc != nil {
			// Find line number
			lineNum := strings.Count(content[:loc[0]], "\n") + 1

			issue := Issue{
				Type:        IssueOutdated,
				Severity:    SeverityWarning,
				Line:        lineNum,
				Message:     pattern.Message,
				AutoFixable: pattern.Replacement != "",
			}

			if pattern.Replacement != "" {
				issue.Suggestion = "Update to: " + pattern.Replacement
				issue.FixData = map[string]string{
					"pattern":     pattern.Pattern,
					"replacement": pattern.Replacement,
				}
			}

			issues = append(issues, issue)
		}
	}

	return issues
}

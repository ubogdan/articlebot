package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HugoFrontMatter represents Hugo's front matter
type HugoFrontMatter struct {
	Title       string   `json:"title"`
	Date        string   `json:"date"`
	Draft       bool     `json:"draft"`
	Tags        []string `json:"tags"`
	Categories  []string `json:"categories"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	AIModel     string   `json:"ai_model"`
	Series      string   `json:"series,omitempty"`
	Content     string   `json:"-"`
}

// NewFrontMatter combines front matter and content at the supplied time.
func NewFrontMatter(topic Topic, content string, isDraft bool, now time.Time) *HugoFrontMatter {
	var builder strings.Builder

	frontMatter := HugoFrontMatter{
		Title:       topic.Title,
		Date:        now.Format(time.RFC3339),
		Draft:       isDraft,
		Tags:        topic.Keywords,
		Categories:  []string{topic.Category},
		Description: topic.Description,
		//		Author:      "TechBot AI",
		Series: detectSeries(topic),
	}

	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("title: %s\n", quoteYAMLString(frontMatter.Title)))
	builder.WriteString(fmt.Sprintf("date: %s\n", frontMatter.Date))
	builder.WriteString(fmt.Sprintf("draft: %t\n", frontMatter.Draft))
	builder.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(quoteStrings(frontMatter.Tags), ", ")))
	builder.WriteString(fmt.Sprintf("categories: [%s]\n", strings.Join(quoteStrings(frontMatter.Categories), ", ")))
	builder.WriteString(fmt.Sprintf("description: %s\n", quoteYAMLString(frontMatter.Description)))
	//builder.WriteString(fmt.Sprintf("author: \"%s\"\n", frontMatter.Author))
	if frontMatter.Series != "" {
		builder.WriteString(fmt.Sprintf("series: %s\n", quoteYAMLString(frontMatter.Series)))
	}
	builder.WriteString("---\n\n")

	// Remove any existing title from content
	content = strings.Replace(content, fmt.Sprintf("# %s\n", frontMatter.Title), "", 1)

	builder.WriteString(content)
	frontMatter.Content = builder.String()

	return &frontMatter
}

func quoteYAMLString(value string) string {
	// JSON double-quoted strings are a valid subset of YAML 1.2 scalars.
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// Save saves the article to a file
func (fm *HugoFrontMatter) Save(outputDir string) (string, error) {
	date, err := time.Parse(time.RFC3339, fm.Date)
	if err != nil {
		return "", fmt.Errorf("parse article date: %w", err)
	}
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}

	filename := filepath.Join(outputDir,
		fmt.Sprintf("%s-%s.md",
			date.Format("2006-01-02"),
			slugify(fm.Title),
		),
	)

	err = os.WriteFile(filename, []byte(fm.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("write article: %w", err)
	}

	return filename, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)

	// Replace special characters that could cause filesystem issues
	s = strings.ReplaceAll(s, "/", "-")   // CI/CD -> ci-cd
	s = strings.ReplaceAll(s, "\\", "-")  // backslash
	s = strings.ReplaceAll(s, " ", "-")   // spaces
	s = strings.ReplaceAll(s, ":", "")    // colons
	s = strings.ReplaceAll(s, ",", "")    // commas
	s = strings.ReplaceAll(s, "&", "and") // ampersand
	s = strings.ReplaceAll(s, "(", "")    // parentheses
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "'", "") // quotes
	s = strings.ReplaceAll(s, "\"", "")

	// Remove multiple consecutive dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}

	// Limit length
	if len(s) > 60 {
		s = s[:60]
	}

	// Clean up leading/trailing dashes
	s = strings.Trim(s, "-")

	return s
}

func quoteStrings(strs []string) []string {
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = quoteYAMLString(s)
	}
	return quoted
}

func detectSeries(topic Topic) string {
	switch topic.Category {
	case "SRE":
		return "Site Reliability Engineering"
	case "SecOps":
		return "Security Operations"
	}
	if contains(topic.Keywords, "kubernetes") {
		return "Kubernetes Mastery"
	}
	if contains(topic.Keywords, "microservices") {
		return "Microservices Architecture"
	}
	if contains(topic.Keywords, "security") {
		return "Security Best Practices"
	}
	if contains(topic.Keywords, "monitoring") || contains(topic.Keywords, "observability") {
		return "Observability & Monitoring"
	}
	if topic.Category == "Go Development" && topic.Difficulty == "Beginner" {
		return "Go Fundamentals"
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

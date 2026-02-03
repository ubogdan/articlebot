package main

import (
	"fmt"
	"log"
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

// formatWithFrontMatter combines front matter and content
func NewFrontMatter(topic Topic, content string, isDraft bool) *HugoFrontMatter {
	var builder strings.Builder

	frontMatter := HugoFrontMatter{
		Title:       topic.Title,
		Date:        time.Now().Format(time.RFC3339),
		Draft:       isDraft,
		Tags:        topic.Keywords,
		Categories:  []string{topic.Category},
		Description: topic.Description,
		//		Author:      "TechBot AI",
		Series: detectSeries(topic),
	}

	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("title: \"%s\"\n", frontMatter.Title))
	builder.WriteString(fmt.Sprintf("date: %s\n", frontMatter.Date))
	builder.WriteString(fmt.Sprintf("draft: %t\n", frontMatter.Draft))
	builder.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(quoteStrings(frontMatter.Tags), ", ")))
	builder.WriteString(fmt.Sprintf("categories: [%s]\n", strings.Join(quoteStrings(frontMatter.Categories), ", ")))
	builder.WriteString(fmt.Sprintf("description: \"%s\"\n", frontMatter.Description))
	//builder.WriteString(fmt.Sprintf("author: \"%s\"\n", frontMatter.Author))
	if frontMatter.Series != "" {
		builder.WriteString(fmt.Sprintf("series: \"%s\"\n", frontMatter.Series))
	}
	builder.WriteString("---\n\n")

	// Remove any existing title from content
	content = strings.Replace(content, fmt.Sprintf("# %s\n", frontMatter.Title), "", 1)

	builder.WriteString(content)
	frontMatter.Content = builder.String()

	return &frontMatter
}

// Save saves the article to a file
func (fm *HugoFrontMatter) Save(outputDir string) string {
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		log.Printf("❌ Error creating directory: %v", err)
		return ""
	}

	filename := filepath.Join(outputDir,
		fmt.Sprintf("%s-%s.md",
			time.Now().Format("2006-01-02"),
			slugify(fm.Title),
		),
	)

	err = os.WriteFile(filename, []byte(fm.Content), 0644)
	if err != nil {
		log.Printf("❌ Error writing file: %v", err)
		return ""
	}

	return filename
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

package fixer

import (
	"regexp"
	"strings"
	"time"

	"github.com/ubogdan/articlebot/internal/analyzer"
	"github.com/ubogdan/articlebot/internal/parser"
)

// Fix attempts to automatically fix issues in an article
// Returns the modified article and a list of issues that were fixed
func Fix(article *parser.Article, issues []analyzer.Issue) (*parser.Article, []analyzer.Issue) {
	var fixed []analyzer.Issue

	// Sort issues to fix code issues first, then content issues
	// This prevents line number shifts from affecting subsequent fixes
	codeIssues := filterIssues(issues, true)
	contentIssues := filterIssues(issues, false)

	// Fix code block issues
	for _, issue := range codeIssues {
		if !issue.AutoFixable {
			continue
		}

		if fixCodeIssue(article, issue) {
			fixed = append(fixed, issue)
		}
	}

	// Fix content issues
	for _, issue := range contentIssues {
		if !issue.AutoFixable {
			continue
		}

		if fixContentIssue(article, issue) {
			fixed = append(fixed, issue)
		}
	}

	// Fix metadata issues
	for _, issue := range issues {
		if issue.Type != analyzer.IssueMissingMeta || !issue.AutoFixable {
			continue
		}

		if fixMetadataIssue(article, issue) {
			fixed = append(fixed, issue)
		}
	}

	return article, fixed
}

func filterIssues(issues []analyzer.Issue, codeOnly bool) []analyzer.Issue {
	var result []analyzer.Issue
	for _, issue := range issues {
		isCode := issue.FixData != nil && issue.FixData["scope"] == "code"
		if isCode == codeOnly {
			result = append(result, issue)
		}
	}
	return result
}

func fixCodeIssue(article *parser.Article, issue analyzer.Issue) bool {
	if issue.FixData == nil {
		return false
	}

	find, hasFind := issue.FixData["find"]
	replace, hasReplace := issue.FixData["replace"]

	if !hasFind || !hasReplace {
		return false
	}

	// Fix in code blocks
	modified := false
	for i, block := range article.CodeBlocks {
		if strings.Contains(block.Content, find) {
			article.CodeBlocks[i].Content = strings.ReplaceAll(block.Content, find, replace)
			modified = true
		}
	}

	// Also fix in the raw content
	if modified {
		article.Content = strings.ReplaceAll(article.Content, find, replace)
	}

	return modified
}

func fixContentIssue(article *parser.Article, issue analyzer.Issue) bool {
	if issue.FixData == nil {
		return false
	}

	switch issue.Type {
	case analyzer.IssueTypo:
		return fixTypo(article, issue)
	case analyzer.IssueBrokenLink:
		return fixLink(article, issue)
	case analyzer.IssueOutdated:
		return fixOutdated(article, issue)
	}

	return false
}

func fixTypo(article *parser.Article, issue analyzer.Issue) bool {
	find := issue.FixData["find"]
	replace := issue.FixData["replace"]

	if find == "" || replace == "" {
		return false
	}

	// Use word boundary matching to avoid partial replacements
	pattern := `(?i)\b` + regexp.QuoteMeta(find) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	// Preserve original case
	newContent := re.ReplaceAllStringFunc(article.Content, func(match string) string {
		if match == strings.ToUpper(match) {
			return strings.ToUpper(replace)
		}
		if match[0] >= 'A' && match[0] <= 'Z' {
			return strings.ToUpper(string(replace[0])) + replace[1:]
		}
		return replace
	})

	if newContent == article.Content {
		return false
	}

	article.Content = newContent
	return true
}

func fixLink(article *parser.Article, issue analyzer.Issue) bool {
	find := issue.FixData["find"]
	replace := issue.FixData["replace"]

	if find == "" || replace == "" {
		return false
	}

	newContent := strings.ReplaceAll(article.Content, find, replace)
	if newContent == article.Content {
		return false
	}

	article.Content = newContent
	return true
}

func fixOutdated(article *parser.Article, issue analyzer.Issue) bool {
	pattern := issue.FixData["pattern"]
	replacement := issue.FixData["replacement"]

	if pattern == "" || replacement == "" {
		return false
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	newContent := re.ReplaceAllString(article.Content, replacement)
	if newContent == article.Content {
		return false
	}

	article.Content = newContent

	// Update lastmod when fixing outdated content
	article.FrontMatter.LastMod = time.Now().Format("2006-01-02")

	return true
}

func fixMetadataIssue(article *parser.Article, issue analyzer.Issue) bool {
	// Currently we can auto-generate descriptions
	if strings.Contains(issue.Message, "description") {
		if article.FrontMatter.Description == "" {
			desc := generateDescription(article)
			if desc != "" {
				article.FrontMatter.Description = desc
				return true
			}
		}
	}

	return false
}

// generateDescription creates a description from article content
func generateDescription(article *parser.Article) string {
	// Try to get the first paragraph that's not a heading
	lines := strings.Split(article.Content, "\n")
	var paragraph strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines, headings, code blocks, and images
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "```") || strings.HasPrefix(line, "![") {
			if paragraph.Len() > 0 {
				break
			}
			continue
		}

		paragraph.WriteString(line)
		paragraph.WriteString(" ")

		// Stop after getting enough content
		if paragraph.Len() > 150 {
			break
		}
	}

	desc := strings.TrimSpace(paragraph.String())

	// Truncate if too long
	if len(desc) > 160 {
		// Find a good break point
		desc = desc[:157]
		lastSpace := strings.LastIndex(desc, " ")
		if lastSpace > 100 {
			desc = desc[:lastSpace]
		}
		desc += "..."
	}

	return desc
}

// ApplyCustomFixes allows adding custom fix functions
type CustomFixer func(*parser.Article, analyzer.Issue) bool

// FixWithCustom runs fixes with additional custom fixers
func FixWithCustom(article *parser.Article, issues []analyzer.Issue, custom []CustomFixer) (*parser.Article, []analyzer.Issue) {
	article, fixed := Fix(article, issues)

	// Run custom fixers on remaining unfixed issues
	remaining := make([]analyzer.Issue, 0)
	for _, issue := range issues {
		wasFixed := false
		for _, f := range fixed {
			if f.Line == issue.Line && f.Message == issue.Message {
				wasFixed = true
				break
			}
		}
		if !wasFixed {
			remaining = append(remaining, issue)
		}
	}

	for _, issue := range remaining {
		for _, customFix := range custom {
			if customFix(article, issue) {
				fixed = append(fixed, issue)
				break
			}
		}
	}

	return article, fixed
}

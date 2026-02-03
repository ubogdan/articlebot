package reporter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ubogdan/articlebot/internal/analyzer"
	"github.com/ubogdan/articlebot/internal/parser"
)

// FileReport contains analysis results for a single file
type FileReport struct {
	Path       string
	Article    *parser.Article
	Issues     []analyzer.Issue
	Fixed      []analyzer.Issue
	WasChanged bool
}

// Summary contains aggregated statistics
type Summary struct {
	TotalFiles      int
	FilesWithIssues int
	TotalIssues     int
	FixedIssues     int
	ByType          map[analyzer.IssueType]int
	BySeverity      map[analyzer.Severity]int
}

// Generate creates a report in the specified format
func Generate(reports []FileReport, format string) string {
	switch format {
	case "json":
		return generateJSON(reports)
	case "markdown", "md":
		return generateMarkdown(reports)
	default:
		return generateText(reports)
	}
}

func generateText(reports []FileReport) string {
	var sb strings.Builder
	summary := calculateSummary(reports)

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("                    ARTICLE BOT ANALYSIS REPORT                 \n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	// Summary
	sb.WriteString(fmt.Sprintf("Files analyzed: %d\n", summary.TotalFiles))
	sb.WriteString(fmt.Sprintf("Files with issues: %d\n", summary.FilesWithIssues))
	sb.WriteString(fmt.Sprintf("Total issues: %d\n", summary.TotalIssues))
	sb.WriteString(fmt.Sprintf("Issues fixed: %d\n", summary.FixedIssues))
	sb.WriteString("\n")

	// Issues by severity
	sb.WriteString("By Severity:\n")
	for sev := analyzer.SeverityError; sev >= analyzer.SeverityInfo; sev-- {
		if count, ok := summary.BySeverity[sev]; ok && count > 0 {
			icon := severityIcon(sev)
			sb.WriteString(fmt.Sprintf("  %s %s: %d\n", icon, sev.String(), count))
		}
	}
	sb.WriteString("\n")

	// Issues by type
	sb.WriteString("By Type:\n")
	for issueType, count := range summary.ByType {
		sb.WriteString(fmt.Sprintf("  • %s: %d\n", issueType, count))
	}
	sb.WriteString("\n")

	// Per-file details
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("                         FILE DETAILS                          \n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	// Sort files by number of issues (most issues first)
	sort.Slice(reports, func(i, j int) bool {
		return len(reports[i].Issues) > len(reports[j].Issues)
	})

	for _, report := range reports {
		if len(report.Issues) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("📄 %s\n", filepath.Base(report.Path)))
		sb.WriteString(fmt.Sprintf("   Path: %s\n", report.Path))
		if report.Article.FrontMatter.Title != "" {
			sb.WriteString(fmt.Sprintf("   Title: %s\n", report.Article.FrontMatter.Title))
		}
		sb.WriteString(fmt.Sprintf("   Word count: ~%d\n", report.Article.WordCount()))
		sb.WriteString(fmt.Sprintf("   Issues: %d", len(report.Issues)))
		if len(report.Fixed) > 0 {
			sb.WriteString(fmt.Sprintf(" (%d fixed)", len(report.Fixed)))
		}
		sb.WriteString("\n\n")

		// Group issues by severity
		for sev := analyzer.SeverityError; sev >= analyzer.SeverityInfo; sev-- {
			sevIssues := filterBySeverity(report.Issues, sev)
			if len(sevIssues) == 0 {
				continue
			}

			for _, issue := range sevIssues {
				icon := severityIcon(issue.Severity)
				fixedMarker := ""
				if wasFixed(issue, report.Fixed) {
					fixedMarker = " ✓ FIXED"
				}

				sb.WriteString(fmt.Sprintf("   %s Line %d: %s%s\n", icon, issue.Line, issue.Message, fixedMarker))
				if issue.Suggestion != "" {
					sb.WriteString(fmt.Sprintf("      💡 %s\n", issue.Suggestion))
				}
			}
		}
		sb.WriteString("\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n\n")
	}

	// Files without issues
	cleanFiles := 0
	for _, report := range reports {
		if len(report.Issues) == 0 {
			cleanFiles++
		}
	}
	if cleanFiles > 0 {
		sb.WriteString(fmt.Sprintf("✅ %d file(s) with no issues detected\n", cleanFiles))
	}

	return sb.String()
}

func generateMarkdown(reports []FileReport) string {
	var sb strings.Builder
	summary := calculateSummary(reports)

	// Header
	sb.WriteString("# Article Bot Analysis Report\n\n")

	// Summary table
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Metric | Count |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Files analyzed | %d |\n", summary.TotalFiles))
	sb.WriteString(fmt.Sprintf("| Files with issues | %d |\n", summary.FilesWithIssues))
	sb.WriteString(fmt.Sprintf("| Total issues | %d |\n", summary.TotalIssues))
	sb.WriteString(fmt.Sprintf("| Issues fixed | %d |\n", summary.FixedIssues))
	sb.WriteString("\n")

	// Issues by severity
	sb.WriteString("### By Severity\n\n")
	sb.WriteString("| Severity | Count |\n")
	sb.WriteString("|----------|-------|\n")
	for sev := analyzer.SeverityError; sev >= analyzer.SeverityInfo; sev-- {
		if count, ok := summary.BySeverity[sev]; ok {
			sb.WriteString(fmt.Sprintf("| %s %s | %d |\n", severityEmoji(sev), sev.String(), count))
		}
	}
	sb.WriteString("\n")

	// Issues by type
	sb.WriteString("### By Type\n\n")
	sb.WriteString("| Type | Count |\n")
	sb.WriteString("|------|-------|\n")
	for issueType, count := range summary.ByType {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", issueType, count))
	}
	sb.WriteString("\n")

	// Per-file details
	sb.WriteString("## File Details\n\n")

	for _, report := range reports {
		if len(report.Issues) == 0 {
			continue
		}

		title := filepath.Base(report.Path)
		if report.Article.FrontMatter.Title != "" {
			title = report.Article.FrontMatter.Title
		}

		sb.WriteString(fmt.Sprintf("### %s\n\n", title))
		sb.WriteString(fmt.Sprintf("**Path:** `%s`\n\n", report.Path))
		sb.WriteString(fmt.Sprintf("**Word count:** ~%d\n\n", report.Article.WordCount()))

		if len(report.Issues) > 0 {
			sb.WriteString("| Line | Severity | Issue | Suggestion | Fixed |\n")
			sb.WriteString("|------|----------|-------|------------|-------|\n")

			for _, issue := range report.Issues {
				fixed := ""
				if wasFixed(issue, report.Fixed) {
					fixed = "✅"
				}
				suggestion := issue.Suggestion
				if len(suggestion) > 50 {
					suggestion = suggestion[:47] + "..."
				}
				sb.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s |\n",
					issue.Line,
					severityEmoji(issue.Severity),
					escapeMarkdown(issue.Message),
					escapeMarkdown(suggestion),
					fixed,
				))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func generateJSON(reports []FileReport) string {
	type JSONIssue struct {
		Type        string `json:"type"`
		Severity    string `json:"severity"`
		Line        int    `json:"line"`
		Message     string `json:"message"`
		Suggestion  string `json:"suggestion,omitempty"`
		AutoFixable bool   `json:"auto_fixable"`
		Fixed       bool   `json:"fixed"`
	}

	type JSONFileReport struct {
		Path       string      `json:"path"`
		Title      string      `json:"title,omitempty"`
		WordCount  int         `json:"word_count"`
		IssueCount int         `json:"issue_count"`
		FixedCount int         `json:"fixed_count"`
		Issues     []JSONIssue `json:"issues"`
	}

	type JSONReport struct {
		Summary struct {
			TotalFiles      int            `json:"total_files"`
			FilesWithIssues int            `json:"files_with_issues"`
			TotalIssues     int            `json:"total_issues"`
			FixedIssues     int            `json:"fixed_issues"`
			ByType          map[string]int `json:"by_type"`
			BySeverity      map[string]int `json:"by_severity"`
		} `json:"summary"`
		Files []JSONFileReport `json:"files"`
	}

	summary := calculateSummary(reports)

	jsonReport := JSONReport{}
	jsonReport.Summary.TotalFiles = summary.TotalFiles
	jsonReport.Summary.FilesWithIssues = summary.FilesWithIssues
	jsonReport.Summary.TotalIssues = summary.TotalIssues
	jsonReport.Summary.FixedIssues = summary.FixedIssues
	jsonReport.Summary.ByType = make(map[string]int)
	jsonReport.Summary.BySeverity = make(map[string]int)

	for t, c := range summary.ByType {
		jsonReport.Summary.ByType[string(t)] = c
	}
	for s, c := range summary.BySeverity {
		jsonReport.Summary.BySeverity[s.String()] = c
	}

	for _, report := range reports {
		fileReport := JSONFileReport{
			Path:       report.Path,
			Title:      report.Article.FrontMatter.Title,
			WordCount:  report.Article.WordCount(),
			IssueCount: len(report.Issues),
			FixedCount: len(report.Fixed),
		}

		for _, issue := range report.Issues {
			fileReport.Issues = append(fileReport.Issues, JSONIssue{
				Type:        string(issue.Type),
				Severity:    issue.Severity.String(),
				Line:        issue.Line,
				Message:     issue.Message,
				Suggestion:  issue.Suggestion,
				AutoFixable: issue.AutoFixable,
				Fixed:       wasFixed(issue, report.Fixed),
			})
		}

		jsonReport.Files = append(jsonReport.Files, fileReport)
	}

	data, _ := json.MarshalIndent(jsonReport, "", "  ")
	return string(data)
}

func calculateSummary(reports []FileReport) Summary {
	summary := Summary{
		TotalFiles: len(reports),
		ByType:     make(map[analyzer.IssueType]int),
		BySeverity: make(map[analyzer.Severity]int),
	}

	for _, report := range reports {
		if len(report.Issues) > 0 {
			summary.FilesWithIssues++
		}
		summary.TotalIssues += len(report.Issues)
		summary.FixedIssues += len(report.Fixed)

		for _, issue := range report.Issues {
			summary.ByType[issue.Type]++
			summary.BySeverity[issue.Severity]++
		}
	}

	return summary
}

func filterBySeverity(issues []analyzer.Issue, severity analyzer.Severity) []analyzer.Issue {
	var result []analyzer.Issue
	for _, issue := range issues {
		if issue.Severity == severity {
			result = append(result, issue)
		}
	}
	return result
}

func wasFixed(issue analyzer.Issue, fixed []analyzer.Issue) bool {
	for _, f := range fixed {
		if f.Line == issue.Line && f.Message == issue.Message {
			return true
		}
	}
	return false
}

func severityIcon(sev analyzer.Severity) string {
	switch sev {
	case analyzer.SeverityError:
		return "❌"
	case analyzer.SeverityWarning:
		return "⚠️"
	case analyzer.SeverityInfo:
		return "ℹ️"
	default:
		return "•"
	}
}

func severityEmoji(sev analyzer.Severity) string {
	switch sev {
	case analyzer.SeverityError:
		return "🔴"
	case analyzer.SeverityWarning:
		return "🟡"
	case analyzer.SeverityInfo:
		return "🔵"
	default:
		return "⚪"
	}
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"|", "\\|",
		"\n", " ",
	)
	return replacer.Replace(s)
}

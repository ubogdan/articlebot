package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ubogdan/articlebot/internal/analyzer"
	"github.com/ubogdan/articlebot/internal/fixer"
	"github.com/ubogdan/articlebot/internal/parser"
	"github.com/ubogdan/articlebot/internal/reporter"
)

func main() {
	// CLI flags
	contentDir := flag.String("dir", "content", "Hugo content directory")
	fix := flag.Bool("fix", false, "Automatically fix issues where possible")
	verbose := flag.Bool("verbose", false, "Show detailed output")
	outputFormat := flag.String("format", "text", "Output format: text, json, markdown")
	singleFile := flag.String("file", "", "Analyze a single file instead of directory")
	configFile := flag.String("linter", "", "Path to config file (optional)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Article Bot - Hugo Markdown Linter & Fixer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -dir ./content/posts\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -file ./content/posts/my-article.md -fix\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -dir ./content -format markdown > report.md\n", os.Args[0])
	}

	flag.Parse()

	// Load configuration
	cfg := loadConfig(*configFile)

	// Collect markdown files
	var files []string
	var err error

	if *singleFile != "" {
		if _, err := os.Stat(*singleFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", *singleFile)
			os.Exit(1)
		}
		files = []string{*singleFile}
	} else {
		files, err = collectMarkdownFiles(*contentDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning directory: %v\n", err)
			os.Exit(1)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "No markdown files found\n")
		os.Exit(1)
	}

	if *verbose {
		fmt.Fprintf(os.Stderr, "Found %d markdown files\n", len(files))
	}

	// Process each file
	var allResults []reporter.FileReport

	for _, file := range files {
		if *verbose {
			fmt.Fprintf(os.Stderr, "Processing: %s\n", file)
		}

		// Parse the file
		article, err := parser.ParseFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", file, err)
			continue
		}

		// Analyze
		issues := analyzer.Analyze(article, cfg)

		// Fix if requested
		var fixed []analyzer.Issue
		if *fix && len(issues) > 0 {
			article, fixed = fixer.Fix(article, issues)
			if len(fixed) > 0 {
				if err := parser.WriteFile(file, article); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", file, err)
				}
			}
		}

		allResults = append(allResults, reporter.FileReport{
			Path:       file,
			Article:    article,
			Issues:     issues,
			Fixed:      fixed,
			WasChanged: len(fixed) > 0,
		})
	}

	// Generate report
	report := reporter.Generate(allResults, *outputFormat)
	fmt.Print(report)

	// Exit with error code if issues found
	totalIssues := 0
	for _, r := range allResults {
		totalIssues += len(r.Issues) - len(r.Fixed)
	}
	if totalIssues > 0 {
		os.Exit(1)
	}
}

func collectMarkdownFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".md" || filepath.Ext(path) == ".markdown") {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func loadConfig(path string) *analyzer.Config {
	cfg := analyzer.DefaultConfig()

	if path != "" {
		if loaded, err := analyzer.LoadConfig(path); err == nil {
			cfg = loaded
		}
	}

	return cfg
}

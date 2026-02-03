package parser

import (
	"bufio"
	"bytes"
	"os"
	"regexp"
	"strings"
)

// Article represents a parsed Hugo markdown file
type Article struct {
	Path        string
	FrontMatter FrontMatter
	Content     string
	Sections    []Section
	CodeBlocks  []CodeBlock
	Links       []Link
	Images      []Image
	RawContent  []byte
}

// FrontMatter represents Hugo front matter
type FrontMatter struct {
	Title       string   `yaml:"title"`
	Date        string   `yaml:"date"`
	Draft       bool     `yaml:"draft"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	Categories  []string `yaml:"categories"`
	Author      string   `yaml:"author"`
	Image       string   `yaml:"image"`
	Aliases     []string `yaml:"aliases"`
	Thumbnail   string   `yaml:"thumbnail"`
	LastMod     string   `yaml:"lastmod"`
	Raw         map[string]interface{}
}

// Section represents a markdown section (header + content)
type Section struct {
	Level   int
	Title   string
	Content string
	Line    int
}

// CodeBlock represents a fenced code block
type CodeBlock struct {
	Language string
	Content  string
	Line     int
}

// Link represents a markdown link
type Link struct {
	Text     string
	URL      string
	Line     int
	IsImage  bool
	IsInCode bool
}

// Image represents a markdown image
type Image struct {
	Alt  string
	URL  string
	Line int
}

var (
	frontMatterRegex = regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`)
	headerRegex      = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	codeBlockRegex   = regexp.MustCompile("(?m)^```(\\w*)\\s*$")
	linkRegex        = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	imageRegex       = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
)

// parseFrontMatter parses YAML-like front matter without external dependencies
func parseFrontMatter(raw string) FrontMatter {
	fm := FrontMatter{
		Raw: make(map[string]interface{}),
	}

	lines := strings.Split(raw, "\n")
	var currentKey string
	var inList bool
	var listItems []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check if it's a list item
		if strings.HasPrefix(trimmed, "- ") {
			if inList && currentKey != "" {
				listItems = append(listItems, strings.TrimPrefix(trimmed, "- "))
			}
			continue
		}

		// Save previous list if we're starting a new key
		if inList && currentKey != "" {
			saveListToFrontMatter(&fm, currentKey, listItems)
			inList = false
			listItems = nil
		}

		// Parse key: value
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			// Remove quotes
			value = strings.Trim(value, `"'`)

			currentKey = key

			// Support inline YAML arrays: ["a", "b"]
			if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
				// Remove brackets
				arr := strings.TrimPrefix(value, "[")
				arr = strings.TrimSuffix(arr, "]")
				// Split by comma
				parts := strings.Split(arr, ",")
				var items []string
				for _, part := range parts {
					item := strings.TrimSpace(part)
					item = strings.Trim(item, `"'`)
					if item != "" {
						items = append(items, item)
					}
				}
				// Save as list
				saveListToFrontMatter(&fm, key, items)
				inList = false
				listItems = nil
				continue
			}

			if value == "" {
				// Could be start of a list
				inList = true
				listItems = nil
			} else {
				fm.Raw[key] = value
				switch key {
				case "title":
					fm.Title = value
				case "date":
					fm.Date = value
				case "draft":
					fm.Draft = value == "true"
				case "description":
					fm.Description = value
				case "author":
					fm.Author = value
				case "image":
					fm.Image = value
				case "thumbnail":
					fm.Thumbnail = value
				case "lastmod":
					fm.LastMod = value
				}
			}
		}
	}

	// Save final list
	if inList && currentKey != "" {
		saveListToFrontMatter(&fm, currentKey, listItems)
	}

	return fm
}

func saveListToFrontMatter(fm *FrontMatter, key string, items []string) {
	fm.Raw[key] = items
	switch key {
	case "tags":
		fm.Tags = items
	case "categories":
		fm.Categories = items
	case "aliases":
		fm.Aliases = items
	}
}

// ParseFile reads and parses a Hugo markdown file
func ParseFile(path string) (*Article, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return Parse(path, data)
}

// Parse parses markdown content
func Parse(path string, data []byte) (*Article, error) {
	article := &Article{
		Path:       path,
		RawContent: data,
	}

	content := string(data)

	// Extract front matter
	if matches := frontMatterRegex.FindStringSubmatch(content); len(matches) > 1 {
		article.FrontMatter = parseFrontMatter(matches[1])
		// Remove front matter from content
		content = frontMatterRegex.ReplaceAllString(content, "")
	}

	article.Content = content

	// Parse sections
	article.Sections = parseSections(content)

	// Parse code blocks
	article.CodeBlocks = parseCodeBlocks(content)

	// Parse links and images
	article.Links, article.Images = parseLinksAndImages(content, article.CodeBlocks)

	return article, nil
}

func parseSections(content string) []Section {
	var sections []Section
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	var currentSection *Section
	var sectionContent strings.Builder

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if matches := headerRegex.FindStringSubmatch(line); len(matches) > 0 {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = strings.TrimSpace(sectionContent.String())
				sections = append(sections, *currentSection)
			}

			// Start new section
			currentSection = &Section{
				Level: len(matches[1]),
				Title: matches[2],
				Line:  lineNum,
			}
			sectionContent.Reset()
		} else if currentSection != nil {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	// Don't forget the last section
	if currentSection != nil {
		currentSection.Content = strings.TrimSpace(sectionContent.String())
		sections = append(sections, *currentSection)
	}

	return sections
}

func parseCodeBlocks(content string) []CodeBlock {
	var blocks []CodeBlock
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	var currentBlock *CodeBlock
	var blockContent strings.Builder

	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				// Start of code block
				lang := strings.TrimPrefix(line, "```")
				lang = strings.TrimSpace(lang)
				currentBlock = &CodeBlock{
					Language: lang,
					Line:     i + 1,
				}
				blockContent.Reset()
				inCodeBlock = true
			} else {
				// End of code block
				currentBlock.Content = blockContent.String()
				blocks = append(blocks, *currentBlock)
				inCodeBlock = false
				currentBlock = nil
			}
		} else if inCodeBlock {
			blockContent.WriteString(line)
			blockContent.WriteString("\n")
		}
	}

	return blocks
}

func parseLinksAndImages(content string, codeBlocks []CodeBlock) ([]Link, []Image) {
	var links []Link
	var images []Image

	// Build a map of lines that are inside code blocks
	codeLines := make(map[int]bool)
	for _, block := range codeBlocks {
		lines := strings.Count(block.Content, "\n") + 2 // +2 for ``` markers
		for i := block.Line; i < block.Line+lines; i++ {
			codeLines[i] = true
		}
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNum := i + 1
		inCode := codeLines[lineNum]

		// Find images first (they have ! prefix)
		for _, match := range imageRegex.FindAllStringSubmatch(line, -1) {
			images = append(images, Image{
				Alt:  match[1],
				URL:  match[2],
				Line: lineNum,
			})
		}

		// Find links (excluding images)
		lineWithoutImages := imageRegex.ReplaceAllString(line, "")
		for _, match := range linkRegex.FindAllStringSubmatch(lineWithoutImages, -1) {
			links = append(links, Link{
				Text:     match[1],
				URL:      match[2],
				Line:     lineNum,
				IsInCode: inCode,
			})
		}
	}

	return links, images
}

// WriteFile writes the article back to disk
func WriteFile(path string, article *Article) error {
	var buf bytes.Buffer

	// Write front matter
	buf.WriteString("---\n")

	// Write known fields in a consistent order
	if article.FrontMatter.Title != "" {
		buf.WriteString("title: \"" + escapeYamlString(article.FrontMatter.Title) + "\"\n")
	}
	if article.FrontMatter.Date != "" {
		buf.WriteString("date: " + article.FrontMatter.Date + "\n")
	}
	if article.FrontMatter.LastMod != "" {
		buf.WriteString("lastmod: " + article.FrontMatter.LastMod + "\n")
	}
	if article.FrontMatter.Draft {
		buf.WriteString("draft: true\n")
	}
	if article.FrontMatter.Thumbnail != "" {
		buf.WriteString("thumbnail: \"" + escapeYamlString(article.FrontMatter.Thumbnail) + "\"\n")
	}
	if article.FrontMatter.Description != "" {
		buf.WriteString("description: \"" + escapeYamlString(article.FrontMatter.Description) + "\"\n")
	}
	if article.FrontMatter.Author != "" {
		buf.WriteString("author: \"" + escapeYamlString(article.FrontMatter.Author) + "\"\n")
	}
	if article.FrontMatter.Image != "" {
		buf.WriteString("image: \"" + article.FrontMatter.Image + "\"\n")
	}
	if len(article.FrontMatter.Categories) > 0 {
		buf.WriteString("categories:\n")
		for _, cat := range article.FrontMatter.Categories {
			buf.WriteString("  - " + cat + "\n")
		}
	}
	if len(article.FrontMatter.Tags) > 0 {
		buf.WriteString("tags:\n")
		for _, tag := range article.FrontMatter.Tags {
			buf.WriteString("  - " + tag + "\n")
		}
	}
	if len(article.FrontMatter.Aliases) > 0 {
		buf.WriteString("aliases:\n")
		for _, alias := range article.FrontMatter.Aliases {
			buf.WriteString("  - " + alias + "\n")
		}
	}

	buf.WriteString("---\n")

	// Write content
	buf.WriteString(article.Content)

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func escapeYamlString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// WordCount returns approximate word count of article content
func (a *Article) WordCount() int {
	// Remove code blocks for word count
	content := a.Content
	for _, block := range a.CodeBlocks {
		content = strings.Replace(content, block.Content, "", 1)
	}

	words := strings.Fields(content)
	return len(words)
}

// HasSection checks if article has a section with given title (case insensitive)
func (a *Article) HasSection(title string) bool {
	title = strings.ToLower(title)
	for _, s := range a.Sections {
		if strings.ToLower(s.Title) == title {
			return true
		}
	}
	return false
}

// HasSectionContaining checks if any section title contains the substring
func (a *Article) HasSectionContaining(substr string) bool {
	substr = strings.ToLower(substr)
	for _, s := range a.Sections {
		if strings.Contains(strings.ToLower(s.Title), substr) {
			return true
		}
	}
	return false
}

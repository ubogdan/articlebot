package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// generateArticleWithAI generates an article using the specified AI provider
func (ag *ArticleGenerator) generateArticleWithAI(provider Provider, topic Topic) (*HugoFrontMatter, error) {
	prompt := ag.buildPrompt(topic)

	log.Printf("📝 Generating article with AI...")
	content, err := provider.Call(ag.getSystemPrompt(), prompt)
	if err != nil {
		return nil, err
	}

	for {
		if !ag.isArticleIncomplete(content) {
			log.Printf("✅ Article appears complete")
			break
		}

		log.Println("   ⚠️  Article appears incomplete, requesting continuation...")

		continuation, err := ag.requestContinuation(provider, content)
		if err != nil {
			log.Printf("      ⚠️  Could not get continuation: %v", err)
			break
		}

		log.Println("      ✅ Continuation received")
		content = content + continuation
	}
	os.WriteFile("article.md", []byte(content), 0644)
	// Add Hugo front matter
	return NewFrontMatter(topic, content, !ag.config.AutoPublish), nil
}

// isArticleIncomplete checks if an article appears to be cut off or incomplete
func (ag *ArticleGenerator) isArticleIncomplete(content string) bool {
	if len(content) == 0 {
		return true
	}

	content = strings.TrimSpace(content)
	lowerContent := strings.ToLower(content)

	// Check for explicit truncation indicators
	truncationIndicators := []string{
		"to be continued",
		"[continued",
		"(continued)",
		"will be continued",
		"more coming soon",
		"to be completed",
		"under construction",
		"work in progress",
	}

	for _, indicator := range truncationIndicators {
		if strings.Contains(lowerContent, strings.ToLower(indicator)) {
			return true
		}
	}

	// Check for required conclusion sections
	hasConclusion := strings.Contains(lowerContent, "## conclusion") ||
		strings.Contains(lowerContent, "## summary") ||
		strings.Contains(lowerContent, "## wrapping up") ||
		strings.Contains(lowerContent, "## final thoughts") ||
		strings.Contains(lowerContent, "## key takeaways")

	// Count words to estimate article completeness
	wordCount := len(strings.Fields(content))

	// If no conclusion section found and word count is low, likely incomplete
	if !hasConclusion && wordCount < 1200 {
		return true
	}

	// Check if article ends abruptly - must end with proper punctuation
	lastChar := rune(content[len(content)-1])
	validEndChars := map[rune]bool{
		'.': true, '!': true, '?': true, // Sentence endings
		')': true, '"': true, '`': true, // Closing punctuation
		']': true, '}': true, // Bracket endings
		'\n': true, // Newline (for multiple paragraphs at end)
	}

	if !validEndChars[lastChar] {
		// Check if it ends with a partial code block or list
		lastLines := strings.Split(content, "\n")
		if len(lastLines) > 0 {
			lastLine := strings.TrimSpace(lastLines[len(lastLines)-1])
			// If last line is short and doesn't look like code/list continuation, it's incomplete
			if len(lastLine) > 20 && !strings.HasPrefix(lastLine, "-") &&
				!strings.HasPrefix(lastLine, "*") && !strings.HasPrefix(lastLine, "```") &&
				!strings.Contains(lastLine, "http") {
				return true
			}
		}
	}

	// Check for incomplete code blocks (unclosed fences)
	codeBlockCount := strings.Count(lowerContent, "```")
	if codeBlockCount%2 != 0 {
		return true
	}

	// Check if article ends with a heading without content
	lastLines := strings.Split(strings.TrimSpace(content), "\n")
	for i := len(lastLines) - 1; i >= len(lastLines)-3 && i >= 0; i-- {
		line := strings.TrimSpace(lastLines[i])
		if strings.HasPrefix(line, "#") && len(lastLines)-1-i < 2 {
			// Heading without following content
			return true
		}
	}

	// Check for incomplete list or bullet points at end
	lastNonEmptyLine := ""
	for i := len(lastLines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lastLines[i])
		if line != "" {
			lastNonEmptyLine = line
			break
		}
	}

	if lastNonEmptyLine != "" && (strings.HasPrefix(lastNonEmptyLine, "-") ||
		strings.HasPrefix(lastNonEmptyLine, "*")) {
		// Article ends with list item, check if it looks complete
		if strings.HasSuffix(lastNonEmptyLine, ",") || strings.HasSuffix(lastNonEmptyLine, ":") {
			return true
		}
	}

	return false
}

// requestContinuation requests the AI to continue an incomplete article
func (ag *ArticleGenerator) requestContinuation(provider Provider, existingContent string) (string, error) {
	continuationPrompt := `The previous article content was cut off. Please continue from where it stopped and complete the article. 

Include:
- Any remaining sections that were not completed
- The Conclusion section with key takeaways

Continue the article now (pick up exactly where the previous content ended):

`

	var continuation string
	var err error

	continuation, err = provider.Call("You are continuing a technical article that was cut off. Complete it professionally.", continuationPrompt+existingContent)

	return continuation, err
}

// getSystemPrompt returns the system prompt for AI
func (ag *ArticleGenerator) getSystemPrompt() string {
	return `You are an expert technical writer specializing in Go programming, DevOps, SRE, and SecOps. 
You write clear, comprehensive, and engaging technical articles with practical examples. 
Your articles are well-structured with proper headings, code examples, and best practices.
Write in markdown format suitable for Hugo static site generator.`
}

// buildPrompt creates a detailed prompt
func (ag *ArticleGenerator) buildPrompt(topic Topic) string {
	// Determine if this article should include code examples
	codeExamplesNeeded := ag.shouldIncludeCodeExamples(topic.Category)

	codeRequirements := ""
	if codeExamplesNeeded {
		codeRequirements = `
   - At least 3-4 practical code examples (in appropriate language for the category)
   - Code examples MUST be:
     * Complete and runnable (not snippets)
     * Well-commented with inline explanations
     * Follow best practices and idiomatic patterns for the language
     * Include proper error handling
     * Show practical, real-world scenarios
     * Between 20-50 lines each`
	} else {
		codeRequirements = `
   - Include practical examples and command-line snippets where relevant
   - Examples should be clear and easy to follow
   - Show real-world scenarios and use cases`
	}

	return fmt.Sprintf(`Write a comprehensive technical article about: "%s"

Topic Details:
- Category: %s
- Difficulty: %s
- Description: %s
- Keywords: %s

Requirements:
1. Write a COMPLETE, production-ready article (1200-2000 words)
2. Include the following sections (all sections are REQUIRED):
   - Introduction (2-3 paragraphs explaining why this topic matters)
   - Prerequisites (what readers need to know before starting)
   - Main content with 4-5 subsections (use ## and ### headings)%s
   - Best practices section (5-7 specific best practices)
   - Common pitfalls section (4-5 pitfalls and how to avoid them)
   - Real-world use cases (2-3 specific examples)
   - Performance considerations (if applicable)
   - Testing approach (how to test this implementation)
   - Conclusion with 3-5 key takeaways
   - Additional resources section (5-7 links)

3. Writing style:
   - Professional but accessible and conversational
   - Use clear, detailed explanations
   - Include specific examples with numbers and metrics
   - Add practical tips marked with 💡 or **Tip:**
   - Make it engaging with real-world context
   - Use subheadings to break up long sections

4. Format requirements:
   - Use proper markdown headings (##, ###)
   - Use bullet points for lists
   - Bold important concepts with **text**
   - Use inline code with backticks for commands/variables
   - Include code blocks with proper language tags (e.g., bash, terraform, go, yaml)
   - NO YAML front matter (I'll add it separately)

5. IMPORTANT: Write the ENTIRE article in one response. Do NOT truncate or summarize. 
   Complete ALL sections fully. The article should be 1500-2500 words.

Start writing the complete article now (without YAML front matter):`,
		topic.Title,
		topic.Category,
		topic.Difficulty,
		topic.Description,
		strings.Join(topic.Keywords, ", "),
		codeRequirements,
	)
}

// shouldIncludeCodeExamples determines if an article category should include code examples
func (ag *ArticleGenerator) shouldIncludeCodeExamples(category string) bool {
	// Categories that should include code examples
	codeCategories := map[string]bool{
		"golang":              true,
		"go programming":      true,
		"programming":         true,
		"python":              true,
		"javascript":          true,
		"rust":                true,
		"backend development": true,
		"web development":     true,
		"full stack":          true,
		"mobile development":  true,
		"systems programming": true,
	}

	lowerCategory := strings.ToLower(category)
	return codeCategories[lowerCategory]
}

// buildPrompt creates a detailed prompt
func (ag *ArticleGenerator) buildPrompt00(topic Topic) string {
	return fmt.Sprintf(`Write a comprehensive technical article about: "%s"

Topic Details:
- Category: %s
- Difficulty: %s
- Description: %s
- Keywords: %s

Requirements:
1. Write a complete, production-ready article (2000-3000 words)
2. Include the following sections:
   - Introduction (explain why this topic matters)
   - Prerequisites (what readers need to know)
   - Main content with multiple subsections (use ## and ### headings)
   - Practical code examples (use Go code in go blocks)
   - Best practices section
   - Common pitfalls and how to avoid them
   - Real-world use cases
   - Performance considerations (if applicable)
   - Testing approach
   - Conclusion with key takeaways
   - Additional resources section

3. Code examples should be:
   - Complete and runnable
   - Well-commented
   - Following Go best practices
   - Include error handling
   - Show practical, real-world scenarios
   - Provided only if the category is "Go Programming" or related

4. Writing style:
   - Professional but accessible
   - Use clear explanations
   - Include specific examples
   - Add practical tips
   - Make it engaging and informative

5. Format:
   - Use proper markdown headings
   - Include code blocks with language tags
   - Use bullet points for lists
   - Bold important concepts
   - NO front matter (I'll add it separately)

Start writing the article content now (without YAML front matter):`,
		topic.Title,
		topic.Category,
		topic.Difficulty,
		topic.Description,
		strings.Join(topic.Keywords, ", "),
	)
}

package generator

import (
	"fmt"
	"log"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxArticleContinuations = 3

// generateArticleWithAI generates an article using the specified AI provider
func (ag *ArticleGenerator) generateArticleWithAI(provider Provider, topic Topic) (*HugoFrontMatter, error) {
	prompt := ag.buildPrompt(topic)

	log.Printf("📝 Generating article with AI...")
	content, err := provider.Call(ag.getSystemPrompt(), prompt)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("AI provider returned empty article content")
	}

	for attempts := 0; ; attempts++ {
		reason := articleIncompleteReason(content)
		if reason == "" {
			log.Printf("✅ Article appears complete")
			break
		}
		if attempts >= maxArticleContinuations {
			return nil, fmt.Errorf("article still incomplete after %d continuation attempts: %s", maxArticleContinuations, reason)
		}

		log.Printf("   ⚠️  Article appears incomplete (%s), requesting continuation...", reason)

		continuation, err := ag.requestContinuation(provider, content)
		if err != nil {
			return nil, fmt.Errorf("request article continuation: %w", err)
		}
		if strings.TrimSpace(continuation) == "" {
			return nil, fmt.Errorf("AI provider returned empty article continuation")
		}
		if strings.Contains(content, strings.TrimSpace(continuation)) {
			return nil, fmt.Errorf("article continuation repeated existing content")
		}

		log.Println("      ✅ Continuation received")
		// Fences and headings must start on a new line, even when the API
		// omits whitespace at the boundary between responses.
		tail := strings.TrimSpace(content[strings.LastIndex(content, "\n")+1:])
		next := strings.TrimSpace(continuation)
		if !strings.HasSuffix(content, "\n") && !strings.HasPrefix(continuation, "\n") &&
			(strings.HasPrefix(tail, "```") || strings.HasPrefix(tail, "~~~") ||
				strings.HasPrefix(next, "#") || strings.HasPrefix(next, "```") || strings.HasPrefix(next, "~~~")) {
			content += "\n"
		}
		content += continuation
	}
	// Add Hugo front matter
	return NewFrontMatter(topic, content, !ag.config.AutoPublish, ag.now()), nil
}

// isArticleIncomplete checks if an article appears to be cut off or incomplete
func (ag *ArticleGenerator) isArticleIncomplete(content string) bool {
	return articleIncompleteReason(content) != ""
}

func articleIncompleteReason(content string) string {
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return "empty article"
	}

	var fence byte
	var fenceLength int
	hasConclusion := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && (line[0] == '`' || line[0] == '~') {
			length := len(line) - len(strings.TrimLeft(line, line[:1]))
			if length >= 3 {
				if fence == 0 {
					// Backticks cannot occur in a backtick fence's info string.
					if line[0] != '`' || !strings.Contains(line[length:], "`") {
						fence, fenceLength = line[0], length
					}
				} else if line[0] == fence && length >= fenceLength && strings.TrimSpace(line[length:]) == "" {
					fence, fenceLength = 0, 0
				}
				continue
			}
		}
		if fence == 0 {
			heading := strings.TrimLeft(line, "#")
			if len(heading) != len(line) && strings.HasPrefix(heading, " ") {
				heading = strings.ToLower(strings.TrimSpace(heading))
				for _, conclusion := range []string{"conclusion", "summary", "wrapping up", "final thoughts", "key takeaways"} {
					if strings.HasPrefix(heading, conclusion) {
						hasConclusion = true
					}
				}
			}
		}
	}
	if fence != 0 {
		return fmt.Sprintf("unclosed %s code fence", strings.Repeat(string(fence), fenceLength))
	}

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

	lastLines := strings.Split(content, "\n")
	lastNonEmptyLine := strings.TrimSpace(lastLines[len(lastLines)-1])
	for _, indicator := range truncationIndicators {
		if strings.Contains(strings.ToLower(lastNonEmptyLine), indicator) {
			return "truncation marker at end"
		}
	}

	// Count words to estimate article completeness
	wordCount := len(strings.Fields(content))

	// If no conclusion section found and word count is low, likely incomplete
	if !hasConclusion && wordCount < 1200 {
		return "fewer than 1200 words and no conclusion heading"
	}

	// Check if article ends with a heading without content
	if strings.HasPrefix(lastNonEmptyLine, "#") {
		return "heading without content at end"
	}

	ending := strings.TrimRight(lastNonEmptyLine, "*_~")
	// Check for incomplete list or bullet points at end
	if lastNonEmptyLine != "" && (strings.HasPrefix(lastNonEmptyLine, "-") ||
		strings.HasPrefix(lastNonEmptyLine, "*")) {
		// Article ends with list item, check if it looks complete
		if strings.HasSuffix(ending, ",") || strings.HasSuffix(ending, ":") {
			return "unfinished list item at end"
		}
	}

	lastChar, _ := utf8.DecodeLastRuneInString(ending)
	validEnd := strings.ContainsRune(".!?)]}`\"'|", lastChar) ||
		unicode.Is(unicode.Pe, lastChar) || unicode.Is(unicode.Pf, lastChar)
	if !validEnd && len(ending) > 20 &&
		!strings.HasPrefix(lastNonEmptyLine, "-") &&
		!strings.HasPrefix(lastNonEmptyLine, "*") &&
		!strings.HasPrefix(lastNonEmptyLine, "```") &&
		!strings.HasPrefix(lastNonEmptyLine, "~~~") &&
		!strings.Contains(lastNonEmptyLine, "http") {
		return "unfinished final sentence"
	}

	return ""
}

// requestContinuation requests the AI to continue an incomplete article
func (ag *ArticleGenerator) requestContinuation(provider Provider, existingContent string) (string, error) {
	continuationPrompt := fmt.Sprintf(`The previous article content appears incomplete: %s.
Please continue from where it stopped and complete the article.

Return ONLY the missing remainder, not a rewritten article or an explanation.
Do not repeat existing sections or add a new Conclusion if one already exists.
If the article ends inside a code fence, continue that code and close the
existing fence before writing prose. Do not open a replacement fence.
Include any remaining required sections, including a Conclusion with key
takeaways if it is missing. Finish the article in this response.

Continue the article now (pick up exactly where the previous content ended):

`, articleIncompleteReason(existingContent))

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
   Complete ALL sections fully within the 1200-2000 word budget.

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

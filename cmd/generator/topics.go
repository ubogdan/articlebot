package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

const topicGenerationSystemPrompt = `You are a technical editor planning articles across the assigned domains.
Treat each topic's assigned domain independently and use tools and languages appropriate to that domain.
Do not make every domain about Go or force programming-language connections onto operational topics.
Return only valid JSON, not a markdown article.`

// Topic represents a technical topic
type Topic struct {
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	Difficulty  string   `json:"difficulty"`
	Description string   `json:"description"`
}

// generateTopicsWithAI uses AI to generate new article topics
func (ag *ArticleGenerator) generateTopicsWithAI(domains []string, topicCount int) error {
	assignedDomains, err := selectTopicDomains(domains, topicCount)
	if err != nil {
		return err
	}

	log.Println("🎨 Generating new article topics with AI...")
	log.Printf("   Domains: %s", strings.Join(domains, ", "))
	log.Println("")

	provider := ag.selectProvider()
	log.Printf("   Using: %s", provider)
	log.Println("")

	// Build the prompt for topic generation
	prompt := ag.buildTopicGenerationPrompt(assignedDomains)

	aiAgent, err := ag.config.Provider(provider)
	if err != nil {
		return err
	}

	response, err := aiAgent.Call(topicGenerationSystemPrompt, prompt)
	if err != nil {
		return err
	}

	// Parse the JSON response
	var generatedTopics struct {
		Topics []Topic `json:"topics"`
	}

	if err := json.Unmarshal([]byte(response), &generatedTopics); err != nil {
		// Try to extract JSON from markdown code blocks
		response = extractJSONFromMarkdown(response)
		if err := json.Unmarshal([]byte(response), &generatedTopics); err != nil {
			return fmt.Errorf("failed to parse topics JSON: %w\nResponse: %s", err, response)
		}
	}

	if err := validateTopicAssignments(generatedTopics.Topics, assignedDomains); err != nil {
		return err
	}

	log.Printf("✅ Generated %d new topics!", len(generatedTopics.Topics))
	log.Println("")

	// Display generated topics
	for i, topic := range generatedTopics.Topics {
		log.Printf("%d. %s", i+1, topic.Title)
		log.Printf("   Category: %s | Difficulty: %s", topic.Category, topic.Difficulty)
		log.Printf("   Keywords: %s", strings.Join(topic.Keywords, ", "))
		log.Println("")
	}

	if shouldSaveTopics() {
		if err := ag.saveGeneratedTopics(generatedTopics.Topics); err != nil {
			return fmt.Errorf("failed to save topics: %w", err)
		}
		log.Println("✅ Topics saved to topics.json")
		log.Println("💡 You can manually merge them into your code or use them as inspiration!")
	} else {
		log.Println("❌ Topics not saved")
	}

	return nil
}

func (ag *ArticleGenerator) loadTopics() ([]Topic, error) {
	// Load existing topics if they exist
	var allTopics struct {
		GeneratedAt time.Time `json:"generated_at"`
		Topics      []Topic   `json:"topics"`
	}

	// Try to load existing file
	data, err := os.ReadFile(TopicsFilename)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &allTopics)
	if err != nil {
		return nil, err
	}

	return allTopics.Topics, nil
}

// saveGeneratedTopics saves generated topics to a JSON file
func (ag *ArticleGenerator) saveGeneratedTopics(topics []Topic) error {
	// Load existing topics if they exist
	var allTopics struct {
		GeneratedAt time.Time `json:"generated_at"`
		Topics      []Topic   `json:"topics"`
	}

	oldTopics, err := ag.loadTopics()
	if err == nil {
		allTopics.Topics = oldTopics
	}

	// Append new topics
	allTopics.GeneratedAt = time.Now()
	allTopics.Topics = append(allTopics.Topics, topics...)

	// Save to file
	data, err := json.MarshalIndent(allTopics, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(TopicsFilename, data, 0644)
}

func selectTopicDomains(domains []string, count int) ([]string, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one topic domain is required")
	}
	if count <= 0 {
		return nil, fmt.Errorf("topic count must be greater than 0")
	}
	for _, domain := range domains {
		if strings.TrimSpace(domain) == "" {
			return nil, fmt.Errorf("topic domains must not be empty")
		}
	}
	assigned := make([]string, count)
	for i := range assigned {
		assigned[i] = domains[rand.Intn(len(domains))]
	}
	return assigned, nil
}

func validateTopicAssignments(topics []Topic, assignedDomains []string) error {
	if len(topics) != len(assignedDomains) {
		return fmt.Errorf("expected %d topics, received %d", len(assignedDomains), len(topics))
	}
	for i, topic := range topics {
		if topic.Category != assignedDomains[i] {
			return fmt.Errorf("topic %d category %q does not match assigned domain %q", i+1, topic.Category, assignedDomains[i])
		}
	}
	return nil
}

// buildTopicGenerationPrompt creates a prompt for preassigned topic domains.
func (ag *ArticleGenerator) buildTopicGenerationPrompt(assignedDomains []string) string {
	var assignments strings.Builder
	for i, domain := range assignedDomains {
		fmt.Fprintf(&assignments, "%d. %s\n", i+1, domain)
	}
	return fmt.Sprintf(`Generate exactly %d innovative and trending article topics for technical blog posts.

Domain assignments (one topic per entry, in this exact order):
%s

Requirements:
1. Topics should be:
   - Current and relevant to %d trends
   - Practical and implementable
   - Appeal to developers and DevOps engineers
   - Cover different difficulty levels (Beginner, Intermediate, Advanced)
   - Centered on their assigned domain, not a shared programming language
   - Go-focused only when assigned Go Development or another explicitly Go-focused domain
   - For DevOps, SRE, and SecOps, focus on domain-native tools, workflows, infrastructure,
     reliability, or security without forcing Go implementations

2. For each topic, provide:
   - Title: Clear, engaging, and SEO-friendly
   - Category: Copy the assigned domain exactly for that topic's position
   - Keywords: 4-6 relevant keywords (lowercase, hyphenated)
   - Difficulty: Beginner, Intermediate, or Advanced
   - Description: 1-2 sentence description of what readers will learn

3. Topics should cover trending areas like:
   - AI/ML operations and deployment
   - Cloud-native development
   - Observability and monitoring
   - Security best practices
   - Performance optimization
   - Modern DevOps tools
   - Kubernetes and containerization
   - Serverless architectures
   - API development patterns

4. Return ONLY valid JSON in this exact format (no markdown, no code blocks):
{
  "topics": [
    {
      "title": "Topic Title Here",
      "category": "Category Name",
      "keywords": ["keyword1", "keyword2", "keyword3"],
      "difficulty": "Intermediate",
      "description": "Brief description of what the article covers."
    }
  ]
}

Generate exactly %d diverse and innovative topics now, preserving the assigned categories and order:`,
		len(assignedDomains),
		assignments.String(),
		time.Now().Year(),
		len(assignedDomains),
	)
}

// extractJSONFromMarkdown extracts JSON from markdown code blocks
func extractJSONFromMarkdown(text string) string {
	// Remove markdown code block markers
	text = strings.TrimSpace(text)

	// Try to find JSON between ```json and ``` or ``` and ```
	if strings.Contains(text, "```json") {
		start := strings.Index(text, "```json") + 7
		end := strings.LastIndex(text, "```")
		if start > 7 && end > start {
			return strings.TrimSpace(text[start:end])
		}
	}

	if strings.Contains(text, "```") {
		start := strings.Index(text, "```") + 3
		end := strings.LastIndex(text, "```")
		if start > 3 && end > start {
			return strings.TrimSpace(text[start:end])
		}
	}

	return text
}

func filterTopicDomains(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--save", "--no-save", "--dry-run":
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if strings.TrimSpace(arg) == "" {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func shouldSaveTopics() bool {
	args := os.Args[2:]
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "--save":
			return true
		case "--no-save", "--dry-run":
			return false
		}
	}

	if fileInfo, err := os.Stdin.Stat(); err == nil && (fileInfo.Mode()&os.ModeCharDevice) == 0 {
		// In non-interactive contexts, default to saving instead of silently discarding
		// generated topics. This keeps automation predictable while still honoring
		// explicit --no-save/--dry-run overrides.
		return true
	}

	log.Println("💾 Save these topics to topics.json? (y/n)")
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return false
	}

	return strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes"
}

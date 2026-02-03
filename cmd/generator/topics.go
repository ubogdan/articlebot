package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

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
	log.Println("🎨 Generating new article topics with AI...")
	log.Printf("   Domains: %s", strings.Join(domains, ", "))
	log.Println("")

	provider := ag.selectProvider()
	log.Printf("   Using: %s", provider)
	log.Println("")

	// Build the prompt for topic generation
	prompt := ag.buildTopicGenerationPrompt(domains, topicCount)

	var response string
	var err error

	aiAgent, err := ag.config.Provider(provider)
	if err != nil {
		return err
	}

	response, err = aiAgent.Call(ag.getSystemPrompt(), prompt)
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

	if len(generatedTopics.Topics) == 0 {
		return fmt.Errorf("no topics generated")
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

	// Ask user if they want to save
	log.Println("💾 Save these topics to topics.json? (y/n)")
	var answer string
	fmt.Scanln(&answer)

	if strings.ToLower(answer) == "y" || strings.ToLower(answer) == "yes" {
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

// buildTopicGenerationPrompt creates a prompt for generating new topics
func (ag *ArticleGenerator) buildTopicGenerationPrompt(domains []string, topics int) string {
	return fmt.Sprintf(`Generate 10 innovative and trending article topics for technical blog posts.

Target Domains: %s

Requirements:
1. Topics should be:
   - Current and relevant to 2025 trends
   - Practical and implementable
   - Appeal to developers and DevOps engineers
   - Cover different difficulty levels (Beginner, Intermediate, Advanced)
   - Focus on Go programming language where applicable

2. For each topic, provide:
   - Title: Clear, engaging, and SEO-friendly
   - Category: One of [%s]
   - Keywords: 4-6 relevant keywords (lowercase, hyphenated)
   - Difficulty: Beginner, Intermediate, or Advanced
   - Description: 1-2 sentence description of what readers will learn

3. Topics should cover trending areas like:
   - AI/ML integration with Go
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

Generate %d diverse and innovative topics now:`,
		strings.Join(domains, ", "),
		strings.Join(domains, ", "),
		topics,
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

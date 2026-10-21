package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ArticleGenerator handles the generation of technical articles
type ArticleGenerator struct {
	config          Config
	topics          []Topic
	usedTopics      map[string]bool
	httpClient      *http.Client
	lastProvider    AIProvider
	providerCounter int
}

func main() {
	log.Println("🤖 Starting Multi-AI Technical Article Generator Bot...")
	log.Println("   Supports: OpenAI (ChatGPT) & Anthropic (Claude)")

	gen := NewArticleGenerator()

	// Load or create config
	err := gen.loadConfig()
	if err != nil {
		log.Printf("⚙️  Creating default config: %v", err)
		gen.createDefaultConfig()
		log.Println("⚠️  Please configure your API keys in config.json")
		return
	}

	err = gen.validateConfig()
	if err != nil {
		log.Printf("❌ Configuration error: %v", err)
		log.Println("💡 Please check config.json and add required API keys")
		return
	}

	log.Printf("✅ Configuration loaded")
	log.Printf("   AI Provider: %s", gen.config.AIProvider)
	log.Printf("   Articles per day: %d", gen.config.ArticlesPerDay)

	// Run in different modes
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "daemon":
			gen.runDaemon()
		case "generate":
			count := 1
			if len(os.Args) > 2 {
				fmt.Sscanf(os.Args[2], "%d", &count)
			}
			gen.generateArticles(count)
		case "topics":
			// Generate new topics with AI
			domains := []string{"Go Development", "DevOps", "SRE", "SecOps"}
			if len(os.Args) > 2 {
				// Allow custom domains from command line
				domains = os.Args[2:]
			}
			if err := gen.generateTopicsWithAI(domains, 10); err != nil {
				log.Printf("❌ Error generating topics: %v", err)
			}
		case "test":
			gen.testConnections()
		case "help":
			printHelp()
		default:
			log.Printf("Unknown command: %s", os.Args[1])
			printHelp()
		}
	} else {
		printHelp()
		//log.Println("")
		//log.Println("🚀 Generating initial batch of articles...")
		//gen.generateArticles(1)

		//fixArticle(gen)
	}
}

func fixArticle(gen *ArticleGenerator) {
	log.Println("🔧 Fixing an existing article...")

	providerName := ProviderClaude
	topicJson := `{
"title": "Zero-Encapsulation Kubernetes Networking: Calico BGP with ToR Switch Integration",
"category": "cluster",
"keywords": ["calico", "bgp-peering", "tor-switch", "network-performance", "kubernetes"],
"difficulty": "Advanced",
"description": "Achieve maximum network performance by configuring Calico to announce node IPs directly to Top-of-Rack switches using BGP, eliminating overlay network overhead entirely."
}`
	var topic Topic
	err := json.Unmarshal([]byte(topicJson), &topic)
	if err != nil {
		log.Printf("Invalid topic JSON: %s\n%v", topicJson, err)
		return
	}

	log.Printf("🎯 Using %s to fix Topic : %s", providerName, topic.Title)

	provider, err := gen.config.Provider(providerName)
	if err != nil {
		log.Printf("❌ Error selecting AI provider: %v", err)
		return
	}

	article, err := gen.generateArticleWithAI(provider, topic)
	if err != nil {
		log.Printf("❌ Error generating article: %v", err)
		return
	}

	filename := article.Save(gen.config.OutputDir)
	if filename != "" {
		log.Printf("✅ Saved: %s", filepath.Base(filename))
	}

}

func printHelp() {
	log.Println("")
	log.Println("💡 Usage:")
	log.Println("   go run main.go test             - Test API connections")
	log.Println("   go run main.go daemon           - Run continuously")
	log.Println("   go run main.go generate [N]     - Generate N articles")
	log.Println("   go run main.go help             - Show this help")
	log.Println("")
	log.Println("📝 AI Provider Options (set in bot-config.json):")
	log.Println("   'openai'    - Use only ChatGPT")
	log.Println("   'claude'    - Use only Claude")
	log.Println("   'random'    - Randomly select provider for each article")
	log.Println("   'alternate' - Alternate between providers")
}

// NewArticleGenerator creates a new article generator
func NewArticleGenerator() *ArticleGenerator {
	rand.Seed(time.Now().UnixNano())

	gen := &ArticleGenerator{
		config:          getDefaultConfig(),
		usedTopics:      make(map[string]bool),
		lastProvider:    ProviderOpenAI,
		providerCounter: 0,
	}

	topics, err := gen.loadTopics()
	if err == nil {
		gen.topics = topics
	}

	return gen
}

// testConnections tests all configured API connections
func (ag *ArticleGenerator) testConnections() {
	log.Println("🔍 Testing API connections...")
	log.Println("")

	providers := 0

	if provider, err := ag.config.Provider(ProviderOpenAI); err == nil {
		providers++
		log.Println("Testing OpenAI (ChatGPT)...")
		if err := provider.Test(); err != nil {
			log.Printf("❌ OpenAI failed: %v", err)
		} else {
			log.Printf("✅ OpenAI connected successfully (Model: %s)", ag.config.OpenAIModel)
		}
		log.Println("")
	}

	if provider, err := ag.config.Provider(ProviderClaude); err == nil {
		providers++
		log.Println("Testing Claude (Anthropic)...")
		if err := provider.Test(); err != nil {
			log.Printf("❌ Claude failed: %v", err)
		} else {
			log.Printf("✅ Claude connected successfully (Model: %s)", ag.config.ClaudeModel)
		}
		log.Println("")
	}

	if providers == 0 {
		log.Println("❌ No API keys configured!")
	}
}

// runDaemon runs the bot continuously
func (ag *ArticleGenerator) runDaemon() {
	log.Println("🔄 Running in daemon mode...")
	log.Printf("⏰ Will generate %d articles every %v", ag.config.ArticlesPerDay, ag.config.GenerationInterval)

	ticker := time.NewTicker(ag.config.GenerationInterval)
	defer ticker.Stop()

	// Generate initial batch
	ag.generateArticles(ag.config.ArticlesPerDay)

	// Continue generating on schedule
	for range ticker.C {
		log.Println("⏰ Scheduled generation triggered...")
		ag.generateArticles(ag.config.ArticlesPerDay)
	}
}

// generateArticles generates multiple articles
func (ag *ArticleGenerator) generateArticles(count int) {
	if len(ag.topics) == 0 {
		log.Printf("❌ No topics found in %s\n", TopicsFilename)
		return
	}

	log.Printf("📚 Generating batch of %d articles...", count)
	for i := 0; i < count; i++ {
		provider := ag.selectProvider()
		topic := ag.selectUnusedTopic()

		log.Printf("🎯 [%d/%d] Topic: %s", i+1, count, topic.Title)
		log.Printf("   Using: %s", provider)

		aiAgent, err := ag.config.Provider(provider)
		if err != nil {
			log.Printf("❌ Error selecting AI provider: %v", err)
			continue
		}

		article, err := ag.generateArticleWithAI(aiAgent, topic)
		if err != nil {
			log.Printf("❌ Error generating article: %v", err)
			continue
		}

		filename := article.Save(ag.config.OutputDir)
		if filename != "" {
			log.Printf("✅ [%d/%d] Saved: %s", i+1, count, filepath.Base(filename))
		}

		// Delay between requests
		if i < count-1 {
			time.Sleep(2 * time.Second)
		}
	}

	log.Println("✨ Batch generation complete!")
}

// selectUnusedTopic selects a topic that hasn't been used recently
func (ag *ArticleGenerator) selectUnusedTopic() Topic {
	if len(ag.usedTopics) >= len(ag.topics) {
		ag.usedTopics = make(map[string]bool)
	}

	var availableTopics []Topic
	for _, topic := range ag.topics {
		if !ag.usedTopics[topic.Title] {
			availableTopics = append(availableTopics, topic)
		}
	}

	if len(availableTopics) == 0 {
		availableTopics = ag.topics
	}

	topic := availableTopics[rand.Intn(len(availableTopics))]
	ag.usedTopics[topic.Title] = true
	return topic
}

// Helper functions
func quoteStrings(strs []string) []string {
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = fmt.Sprintf("\"%s\"", s)
	}
	return quoted
}

func detectSeries(topic Topic) string {
	// Check by category
	switch topic.Category {
	case "SRE":
		return "Site Reliability Engineering"
	case "SecOps":
		return "Security Operations"
	}

	// Check by keywords
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

	// Check by difficulty for Go Development
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

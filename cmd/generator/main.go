package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
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
	if err := newApp().Run(os.Args); err != nil {
		logError("cli", "Command failed: %v", err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	return &cli.App{
		Name:  "articlebot",
		Usage: "Multi-AI technical article generator",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "provider",
				Usage: "Override configured AI provider (openai, claude, ollama, random, alternate)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "daemon",
				Usage: "Run continuously",
				Action: func(c *cli.Context) error {
					gen, err := loadGeneratorForCommand(c)
					if err != nil {
						return err
					}
					if gen == nil {
						return nil
					}
					gen.runDaemon()
					return nil
				},
			},
			{
				Name:      "generate",
				Usage:     "Generate N articles",
				ArgsUsage: "[N]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "title", Usage: "Generate one article with an exact title from topics.json"},
				},
				Action: func(c *cli.Context) error {
					if c.IsSet("title") {
						if strings.TrimSpace(c.String("title")) == "" {
							return fmt.Errorf("--title must not be empty")
						}
						if c.Args().Len() > 0 {
							return fmt.Errorf("--title generates one article and cannot be combined with an article count")
						}
					}
					gen, err := loadGeneratorForCommand(c)
					if err != nil {
						return err
					}
					if gen == nil {
						return nil
					}

					if c.IsSet("title") {
						return gen.generateArticleByTitle(c.String("title"))
					}

					count := 1
					if c.Args().Len() > 0 {
						if n, err := fmt.Sscanf(c.Args().First(), "%d", &count); err != nil || n != 1 {
							return fmt.Errorf("invalid article count %q: %w", c.Args().First(), err)
						}
					}
					gen.generateArticles(count)
					return nil
				},
			},
			{
				Name:      "topics",
				Usage:     "Generate new article topics",
				ArgsUsage: "[domain ...]",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "save", Usage: "Save generated topics to topics.json"},
					&cli.BoolFlag{Name: "no-save", Usage: "Do not save generated topics"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Do not save generated topics"},
				},
				Action: func(c *cli.Context) error {
					gen, err := loadGeneratorForCommand(c)
					if err != nil {
						return err
					}
					if gen == nil {
						return nil
					}

					domains := []string{"Go Development", "DevOps", "SRE", "SecOps"}
					if c.Args().Len() > 0 {
						domains = filterTopicDomains(c.Args().Slice())
						if len(domains) == 0 {
							domains = []string{"Go Development", "DevOps", "SRE", "SecOps"}
						}
					}

					if c.Bool("save") {
						os.Args = append(os.Args, "--save")
					}
					if c.Bool("no-save") {
						os.Args = append(os.Args, "--no-save")
					}
					if c.Bool("dry-run") {
						os.Args = append(os.Args, "--dry-run")
					}

					if err := gen.generateTopicsWithAI(domains, 10); err != nil {
						logError("topics", "Error generating topics: %v", err)
						return err
					}
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "Test API connections",
				Action: func(c *cli.Context) error {
					gen, err := loadGeneratorForCommand(c)
					if err != nil {
						return err
					}
					if gen == nil {
						return nil
					}
					gen.testConnections()
					return nil
				},
			},
			{
				Name:  "help",
				Usage: "Show command help",
				Action: func(c *cli.Context) error {
					printHelp()
					return nil
				},
			},
		},
		Action: func(c *cli.Context) error {
			printHelp()
			return nil
		},
	}

}

func loadGeneratorForCommand(c *cli.Context) (*ArticleGenerator, error) {
	gen := NewArticleGenerator()

	err := gen.loadConfig()
	if err != nil {
		logWarn("config", "Creating default config: %v", err)
		gen.createDefaultConfig()
		logWarn("config", "Please configure your API keys in config.json")
		return nil, nil
	}

	if c.IsSet("provider") {
		gen.config.AIProvider = c.String("provider")
	}

	err = gen.validateConfig()
	if err != nil {
		logError("config", "Configuration error: %v", err)
		logInfo("config", "Please check config.json and add required API keys")
		return nil, err
	}

	logInfo("config", "Configuration loaded")
	logInfo("config", "AI Provider: %s", gen.config.AIProvider)
	logInfo("config", "Articles per day: %d", gen.config.ArticlesPerDay)
	return gen, nil
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
	log.Println("   articlebot test                 - Test API connections")
	log.Println("   articlebot daemon              - Run continuously")
	log.Println("   articlebot generate [N]        - Generate N articles")
	log.Println("   articlebot generate --title \"Exact topic title\" - Generate a matching topic")
	log.Println("   articlebot topics [domain...]  - Generate topic ideas")
	log.Println("   articlebot topics --save       - Save output to topics.json")
	log.Println("   articlebot topics --no-save    - Skip saving")
	log.Println("   articlebot help                - Show this help")
	log.Println("   articlebot --provider openai generate 1 - Override configured provider")
	log.Println("")
	log.Println("📝 AI Provider Options (config.json or --provider before the command):")
	log.Println("   'openai'    - Use only ChatGPT")
	log.Println("   'claude'    - Use only Claude")
	log.Println("   'ollama'    - Use local Ollama")
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
		topic := ag.selectUnusedTopic()

		log.Printf("🎯 [%d/%d] Topic: %s", i+1, count, topic.Title)
		filename, err := ag.generateTopicArticle(topic)
		if err != nil {
			log.Printf("❌ Error generating article: %v", err)
			continue
		}

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

func (ag *ArticleGenerator) generateArticleByTitle(title string) error {
	topics, err := ag.loadTopics()
	if err != nil {
		return fmt.Errorf("load %s: %w", TopicsFilename, err)
	}
	for _, topic := range topics {
		if topic.Title == title {
			log.Printf("Generating article for topic: %s", topic.Title)
			filename, err := ag.generateTopicArticle(topic)
			if err != nil {
				return err
			}
			log.Printf("Saved: %s", filepath.Base(filename))
			return nil
		}
	}
	return fmt.Errorf("topic title %q not found in %s", title, TopicsFilename)
}

func (ag *ArticleGenerator) generateTopicArticle(topic Topic) (string, error) {
	provider := ag.selectProvider()
	log.Printf("   Using: %s", provider)
	aiAgent, err := ag.config.Provider(provider)
	if err != nil {
		return "", fmt.Errorf("select AI provider: %w", err)
	}
	article, err := ag.generateArticleWithAI(aiAgent, topic)
	if err != nil {
		return "", fmt.Errorf("generate article: %w", err)
	}
	filename := article.Save(ag.config.OutputDir)
	if filename == "" {
		return "", fmt.Errorf("failed to save article %q to %s", topic.Title, ag.config.OutputDir)
	}
	return filename, nil
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
		quoted[i] = quoteYAMLString(s)
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

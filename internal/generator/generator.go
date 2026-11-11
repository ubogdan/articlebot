// Package generator implements article and topic generation independently of the CLI.
package generator

import (
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"time"
)

// Paths identifies the files used by a generator. Empty paths use the CLI defaults.
type Paths struct {
	Topics    string
	OutputDir string
}

// Options supplies runtime dependencies. Zero values use production defaults.
type Options struct {
	Paths       Paths
	Now         func() time.Time
	NewProvider func(AIProvider) (Provider, error)
}

// ArticleGenerator coordinates provider selection, generation, and persistence.
type ArticleGenerator struct {
	config          Config
	paths           Paths
	now             func() time.Time
	newProvider     func(AIProvider) (Provider, error)
	topics          []Topic
	usedTopics      map[string]bool
	providerCounter int
}

// New constructs a generator without reading files or calling providers.
// The configuration should be validated before running commands.
func New(config Config, options Options) *ArticleGenerator {
	if options.Paths.Topics == "" {
		options.Paths.Topics = TopicsFilename
	}
	if options.Paths.OutputDir == "" {
		options.Paths.OutputDir = config.OutputDir
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.NewProvider == nil {
		options.NewProvider = config.Provider
	}
	return &ArticleGenerator{
		config:      config,
		paths:       options.Paths,
		now:         options.Now,
		newProvider: options.NewProvider,
		usedTopics:  make(map[string]bool),
	}
}

// TestConnections checks the configured cloud providers.
func (ag *ArticleGenerator) TestConnections() {
	log.Println("🔍 Testing API connections...")
	configured := 0
	for _, entry := range []struct {
		name  AIProvider
		model string
		has   bool
	}{
		{ProviderOpenAI, ag.config.OpenAIModel, ag.config.HasOpenAI()},
		{ProviderClaude, ag.config.ClaudeModel, ag.config.HasClaude()},
	} {
		if !entry.has {
			continue
		}
		configured++
		log.Printf("Testing %s...", entry.name)
		provider, err := ag.newProvider(entry.name)
		if err == nil {
			err = provider.Test()
		}
		if err != nil {
			log.Printf("❌ %s failed: %v", entry.name, err)
		} else {
			log.Printf("✅ %s connected successfully (Model: %s)", entry.name, entry.model)
		}
	}
	if configured == 0 {
		log.Println("❌ No API keys configured!")
	}
}

// RunDaemon generates an initial batch and repeats on the configured interval.
func (ag *ArticleGenerator) RunDaemon() {
	log.Println("🔄 Running in daemon mode...")
	log.Printf("⏰ Will generate %d articles every %v", ag.config.ArticlesPerDay, ag.config.GenerationInterval)
	ticker := time.NewTicker(ag.config.GenerationInterval)
	defer ticker.Stop()
	ag.GenerateArticles(ag.config.ArticlesPerDay)
	for range ticker.C {
		log.Println("⏰ Scheduled generation triggered...")
		ag.GenerateArticles(ag.config.ArticlesPerDay)
	}
}

// GenerateArticles generates a batch, logging individual failures.
func (ag *ArticleGenerator) GenerateArticles(count int) {
	topics, err := ag.loadTopics()
	if err != nil {
		log.Printf("❌ Error loading topics from %s: %v", ag.paths.Topics, err)
		return
	}
	ag.topics = topics
	if len(ag.topics) == 0 {
		log.Printf("❌ No topics found in %s", ag.paths.Topics)
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
		log.Printf("✅ [%d/%d] Saved: %s", i+1, count, filepath.Base(filename))
		if i < count-1 {
			time.Sleep(2 * time.Second)
		}
	}
	log.Println("✨ Batch generation complete!")
}

// GenerateArticleByTitle generates an exact, case-sensitive match from the topics file.
func (ag *ArticleGenerator) GenerateArticleByTitle(title string) error {
	topics, err := ag.loadTopics()
	if err != nil {
		return fmt.Errorf("load %s: %w", ag.paths.Topics, err)
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
	return fmt.Errorf("topic title %q not found in %s", title, ag.paths.Topics)
}

func (ag *ArticleGenerator) generateTopicArticle(topic Topic) (string, error) {
	provider := ag.selectProvider()
	log.Printf("   Using: %s", provider)
	aiAgent, err := ag.newProvider(provider)
	if err != nil {
		return "", fmt.Errorf("select AI provider: %w", err)
	}
	article, err := ag.generateArticleWithAI(aiAgent, topic)
	if err != nil {
		return "", fmt.Errorf("generate article: %w", err)
	}
	filename, err := article.Save(ag.paths.OutputDir)
	if err != nil {
		return "", fmt.Errorf("save article %q: %w", topic.Title, err)
	}
	return filename, nil
}

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

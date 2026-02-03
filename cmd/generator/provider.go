package main

import (
	"math/rand"
)

// AIProvider represents the AI service to use
type AIProvider string

const (
	ProviderOpenAI    AIProvider = "openai"
	ProviderClaude    AIProvider = "claude"
	ProviderRandom    AIProvider = "random"
	ProviderAlternate AIProvider = "alternate"

	ConfigFilename = "config.json"
	TopicsFilename = "topics.json"
)

type Provider interface {
	Call(prompt, message string) (string, error)
	Test() error
}

// selectProvider determines which AI provider to use
func (ag *ArticleGenerator) selectProvider() AIProvider {
	hasOpenAI := ag.config.OpenAIAPIKey != "" && ag.config.OpenAIAPIKey != "your-openai-api-key-here"
	hasClaude := ag.config.ClaudeAPIKey != "" && ag.config.ClaudeAPIKey != "your-claude-api-key-here"

	provider := AIProvider(ag.config.AIProvider)

	switch provider {
	case ProviderOpenAI:
		if hasOpenAI {
			return ProviderOpenAI
		}
		return ProviderClaude

	case ProviderClaude:
		if hasClaude {
			return ProviderClaude
		}
		return ProviderOpenAI

	case ProviderRandom:
		if hasOpenAI && hasClaude {
			if rand.Float32() < 0.5 {
				return ProviderOpenAI
			}
			return ProviderClaude
		} else if hasOpenAI {
			return ProviderOpenAI
		}
		return ProviderClaude

	case ProviderAlternate:
		if hasOpenAI && hasClaude {
			ag.providerCounter++
			if ag.providerCounter%2 == 0 {
				return ProviderOpenAI
			}
			return ProviderClaude
		} else if hasOpenAI {
			return ProviderOpenAI
		}
		return ProviderClaude
	}

	// Default
	if hasOpenAI {
		return ProviderOpenAI
	}
	return ProviderClaude
}

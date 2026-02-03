package providers

type Config struct {
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
}

func WithAPIKey(apiKey string) func(*Config) {
	return func(c *Config) {
		c.apiKey = apiKey
	}
}

func WithModel(model string) func(*Config) {
	return func(c *Config) {
		c.model = model
	}
}

func WithMaxTokens(maxTokens int) func(*Config) {
	return func(c *Config) {
		c.maxTokens = maxTokens
	}
}

func WithTemperature(temperature float64) func(*Config) {
	return func(c *Config) {
		c.temperature = temperature
	}
}

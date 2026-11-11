package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ubogdan/articlebot/internal/generator"
	"github.com/urfave/cli/v2"
)

type appOptions struct {
	configPath string
	generator  generator.Options
	getenv     func(string) string
}

func main() {
	if err := newApp(appOptions{}).Run(os.Args); err != nil {
		logError("cli", "Command failed: %v", err)
		os.Exit(1)
	}
}

func newApp(opts appOptions) *cli.App {
	if opts.configPath == "" {
		opts.configPath = generator.ConfigFilename
	}
	if opts.getenv == nil {
		opts.getenv = os.Getenv
	}
	load := func(c *cli.Context) (*generator.ArticleGenerator, error) {
		return loadGeneratorForCommand(c, opts)
	}
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
					gen, err := load(c)
					if err != nil || gen == nil {
						return err
					}
					gen.RunDaemon()
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
					gen, err := load(c)
					if err != nil || gen == nil {
						return err
					}
					if c.IsSet("title") {
						return gen.GenerateArticleByTitle(c.String("title"))
					}
					count := 1
					if c.Args().Len() > 0 {
						if n, err := fmt.Sscanf(c.Args().First(), "%d", &count); err != nil || n != 1 {
							return fmt.Errorf("invalid article count %q: %w", c.Args().First(), err)
						}
					}
					gen.GenerateArticles(count)
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
					gen, err := load(c)
					if err != nil || gen == nil {
						return err
					}
					domains := filterTopicDomains(c.Args().Slice())
					if len(domains) == 0 {
						domains = []string{"Go Development", "DevOps", "SRE", "SecOps"}
					}
					topics, err := gen.GenerateTopics(domains, 10)
					if err != nil {
						return err
					}
					log.Printf("✅ Generated %d new topics!", len(topics))
					for i, topic := range topics {
						log.Printf("%d. %s", i+1, topic.Title)
						log.Printf("   Category: %s | Difficulty: %s", topic.Category, topic.Difficulty)
						log.Printf("   Keywords: %s", strings.Join(topic.Keywords, ", "))
					}
					save, err := shouldSaveTopics(c)
					if err != nil {
						return err
					}
					if save {
						if err := gen.SaveTopics(topics); err != nil {
							return fmt.Errorf("failed to save topics: %w", err)
						}
						log.Println("✅ Topics saved")
					} else {
						log.Println("❌ Topics not saved")
					}
					return nil
				},
			},
			{
				Name:  "test",
				Usage: "Test API connections",
				Action: func(c *cli.Context) error {
					gen, err := load(c)
					if err != nil || gen == nil {
						return err
					}
					gen.TestConnections()
					return nil
				},
			},
		},
		Action: cli.ShowAppHelp,
	}
}

func loadGeneratorForCommand(c *cli.Context, opts appOptions) (*generator.ArticleGenerator, error) {
	config, err := generator.LoadConfig(opts.configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load config: %w", err)
		}
		logWarn("config", "Creating default config: %v", err)
		if err := generator.WriteDefaultConfig(opts.configPath); err != nil {
			return nil, fmt.Errorf("create default config: %w", err)
		}
		logWarn("config", "Please configure your API keys in %s", opts.configPath)
		return nil, nil
	}
	config.ApplyEnv(opts.getenv)
	if c.IsSet("provider") {
		config.AIProvider = c.String("provider")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	logInfo("config", "Configuration loaded")
	logInfo("config", "AI Provider: %s", config.AIProvider)
	logInfo("config", "Articles per day: %d", config.ArticlesPerDay)
	return generator.New(config, opts.generator), nil
}

func filterTopicDomains(args []string) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || strings.TrimSpace(arg) == "" {
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}

func shouldSaveTopics(c *cli.Context) (bool, error) {
	save, noSave := c.Bool("save"), c.Bool("no-save") || c.Bool("dry-run")
	// urfave/cli leaves flags after the first positional domain in Args.
	for _, arg := range c.Args().Slice() {
		switch strings.ToLower(arg) {
		case "--save":
			save = true
		case "--no-save", "--dry-run":
			noSave = true
		}
	}
	if save && noSave {
		return false, fmt.Errorf("--save cannot be combined with --no-save or --dry-run")
	}
	if noSave {
		return false, nil
	}
	if save {
		return true, nil
	}
	if file, ok := c.App.Reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return false, fmt.Errorf("inspect input: %w", err)
		}
		if info.Mode()&os.ModeCharDevice == 0 {
			return true, nil
		}
	}
	fmt.Fprintln(c.App.Writer, "Save these topics to topics.json? (y/n)")
	var answer string
	if _, err := fmt.Fscanln(c.App.Reader, &answer); err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, fmt.Errorf("read save confirmation: %w", err)
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

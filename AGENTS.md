# AGENTS.md

## Project overview

This repository contains two related Go workflows:

- `cmd/generator/` is a CLI for generating Hugo-ready articles from AI providers.
- `internal/analyzer/` and `internal/parser/` implement markdown linting and content analysis.

The generator is not a generic app template; it is a content-production bot that writes files to `content/posts` and interacts with OpenAI/Anthropic APIs through the provider wrappers in `internal/providers/`.

## Repository layout

- `cmd/generator/`: CLI wiring, environment overrides, and interactive prompts
  - `main.go`: entrypoint and commands (`test`, `daemon`, `generate`, `topics`, `help`)
- `internal/generator/`: generation orchestration and persistence
  - `generator.go`: service construction, injected dependencies, batches, and daemon
  - `config.go`: JSON config loading, env overrides, validation
  - `provider.go`: provider selection (`openai`, `claude`, `random`, `alternate`)
  - `frontmatter.go`: Hugo YAML front matter generation and file-saving
  - `articles.go`: prompt construction, completion checks, continuation requests
- `internal/providers/`: API client implementations for OpenAI and Claude
- `internal/analyzer/`: issue detection and linting rules
- `internal/parser/`: markdown/article parsing utilities
- `content/`: generated or checked content
- `config.json`, `config_example.json`, `topics.json`: runtime state and config

## Commands and workflow

The repo now includes a root `Makefile` and supports both direct Go commands and standard task aliases.

```bash
# common developer tasks
make build
make test
make fmt
make lint

# direct Go commands also work
 go test ./...

# smoke-test AI providers
 go run ./cmd/generator test

# generate one article
 go run ./cmd/generator generate 1

# run the daemon loop
 go run ./cmd/generator daemon

# generate topic suggestions with AI
 go run ./cmd/generator topics

# non-interactive topic generation flags
 go run ./cmd/generator topics --save
 go run ./cmd/generator topics --no-save
 go run ./cmd/generator topics --dry-run
```

When you need a quick validation for generator changes, prefer `go test ./cmd/generator ./internal/generator` before broad repo-level runs.

## Config and environment conventions

- Config is JSON-based and read from the project root as `config.json`.
- Defaults are defined in `internal/generator/config.go` via `DefaultConfig()`.
- API keys can be supplied either in `config.json` or via environment variables:
  - `OPENAI_API_KEY`
  - `ANTHROPIC_API_KEY`
- The CLI calls `Config.ApplyEnv(getenv)` to override in-file keys using an injected environment lookup.
- `Config.Validate()` is the canonical validation path and rejects invalid provider values, empty output dirs, non-positive generation settings, invalid temperature ranges, and placeholder API keys.
- `LoadConfig(path)` only reads JSON and converts the stored interval from hours.

Important examples:

```go
AIProvider: "random",
OpenAIModel: "gpt-4o-mini",
ClaudeModel: "claude-sonnet-4-20250514",
OutputDir: "content/posts",
ArticlesPerDay: 3,
GenerationInterval: 8 * time.Hour,
```

## Provider integration pattern

The generator chooses a provider through `ArticleGenerator.selectProvider()` and then calls the common interface in `internal/generator/provider.go`:

```go
type Provider interface {
    Call(prompt, message string) (string, error)
    Test() error
}
```

Provider wrappers live in `internal/providers/` and use the option pattern:

```go
providers.NewOpenAI(
    key,
    providers.WithModel(model),
    providers.WithTemperature(temp),
    providers.WithMaxTokens(maxTokens),
)
```

`internal/providers/http.go` centralizes shared retry logic and API error handling for OpenAI and Claude requests. Keep any new provider logic aligned with this interface, option style, and retry behavior.

Construct the service with `generator.New(config, Options)`. `Options.NewProvider`
injects provider creation, `Options.Paths` injects topics/output paths, and
`Options.Now` supplies wall-clock time. Defaults preserve production behavior.
Keep flag parsing, environment lookup, and confirmation prompts in the CLI.
Tests should use temporary absolute paths and injected dependencies, not
`os.Chdir`, `os.Args` mutation, or live AI requests.

## Article generation pattern

The generation flow is:

1. Load config and topics
2. Select a provider from `internal/generator/provider.go`
3. Build a prompt in `articles.go`
4. Call `provider.Call(...)`
5. Reject or continue truncated content via `isArticleIncomplete()` and `requestContinuation()`
6. Wrap output in Hugo front matter via `NewFrontMatter(..., now)`
7. Save the file to `content/posts` via `HugoFrontMatter.Save(...)`
8. Do not write transient root-level files like `article.md`; output should be written through the configured output directory and save API.

`internal/generator/frontmatter.go` is the pattern to follow for new generated content. It emits YAML like:

```yaml
---
title: "..."
date: 2026-11-10T00:00:00Z
draft: true
tags: ["..."]
categories: ["..."]
description: "..."
---
```

The file naming convention comes from `slugify()` and `Save()`: `YYYY-MM-DD-<slug>.md`.
`Save()` returns `(string, error)` and derives the filename date from the front
matter timestamp rather than reading the clock again.

## Non-interactive and automation conventions

- Topic generation is safe for automation: `topics` supports `--save`, `--no-save`, and `--dry-run` directly.
- `GenerateTopics()` returns validated topics; `SaveTopics()` persists them. Neither reads process arguments nor prompts.
- Avoid blocking on `fmt.Scanln` in new CLI flows; prefer explicit flags or a non-interactive default path.
- Prefer deterministic, file-based output and do not create ad hoc artifacts in the repo root during generation.

## Analyzer and linting conventions

The analyzer in `internal/analyzer/analyzer.go` is configuration-driven and uses explicit issue types, including:

- `typo`
- `code_syntax`
- `missing_meta`
- `missing_section`
- `broken_link`
- `outdated`
- `style`
- `structure`

Issues are normalized before they are returned: duplicates are removed and results are sorted by severity and location. If you add validation logic, match the existing issue model (`Issue`, `Severity`, `IssueType`) instead of inventing a new reporting channel.

## Codebase-specific guardrails

- Do not add a second top-level app entrypoint; the CLI lives under `cmd/generator`.
- Keep config keys aligned with the JSON tags already in use (`ai_provider`, `generation_interval_hours`, `output_dir`, etc.).
- Preserve the existing Hugo front matter semantics; generated markdown is meant to be consumed by Hugo.
- Prefer file-saving through the existing front matter API instead of ad hoc writing logic.
- If you change behavior around provider models, output paths, or topic generation prompts, update the corresponding tests and examples in `internal/generator` and CLI tests in `cmd/generator`.

## Existing tests to use as examples

- `internal/generator/config_test.go`: verifies config validation and injected env overrides.
- `cmd/generator/topics_test.go`: verifies topic-save flags and non-interactive behavior.
- `internal/generator/generator_test.go`: verifies injected providers, paths, and clock.
- `internal/generator/frontmatter_test.go`: verifies front matter quoting and metadata round-tripping.
- `internal/analyzer/analyzer_dedupe_test.go`: verifies normalization and deduplication of issue results.

Use these as the baseline style for new Go tests in this repo.

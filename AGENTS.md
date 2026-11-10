# AGENTS.md

## Project overview

This repository contains two related Go workflows:

- `cmd/generator/` is a CLI for generating Hugo-ready articles from AI providers.
- `internal/analyzer/` and `internal/parser/` implement markdown linting and content analysis.

The generator is not a generic app template; it is a content-production bot that writes files to `content/posts` and interacts with OpenAI/Anthropic APIs through the provider wrappers in `internal/providers/`.

## Repository layout

- `cmd/generator/`: article-generation CLI and config logic
  - `main.go`: entrypoint and commands (`test`, `daemon`, `generate`, `topics`, `help`)
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

Use the Go toolchain directly from the repo root; there is no Makefile in this checkout.

```bash
# run the test suite
 go test ./...

# smoke-test AI providers
 go run ./cmd/generator test

# generate one article
 go run ./cmd/generator generate 1

# run the daemon loop
 go run ./cmd/generator daemon

# generate topic suggestions with AI
 go run ./cmd/generator topics
```

When you need a quick validation for generator changes, prefer a targeted package test or `go test ./cmd/generator/...` before broad repo-level runs.

## Config and environment conventions

- Config is JSON-based and read from the project root as `config.json`.
- Defaults are defined in `cmd/generator/config.go` via `getDefaultConfig()`.
- API keys can be supplied either in `config.json` or via environment variables:
  - `OPENAI_API_KEY`
  - `ANTHROPIC_API_KEY`
- `Config.applyEnv()` overrides in-file keys when environment variables are present.
- `Config.validateConfig()` accepts only `openai`, `claude`, `random`, and `alternate`.

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

The generator chooses a provider through `ArticleGenerator.selectProvider()` and then calls the common interface in `cmd/generator/provider.go`:

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

Keep any new provider logic aligned with this interface and option style.

## Article generation pattern

The generation flow is:

1. Load config and topics
2. Select a provider from `cmd/generator/provider.go`
3. Build a prompt in `articles.go`
4. Call `provider.Call(...)`
5. Reject or continue truncated content via `isArticleIncomplete()` and `requestContinuation()`
6. Wrap output in Hugo front matter via `NewFrontMatter(...)`
7. Save the file to `content/posts` via `HugoFrontMatter.Save(...)`

`frontmatter.go` is the pattern to follow for new generated content. It emits YAML like:

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

Default analyzer defaults are intentionally opinionated and embedded in `DefaultConfig()`, including common typo maps and outdated pattern rules. If you add validation logic, match the existing issue model (`Issue`, `Severity`, `IssueType`) instead of inventing a new reporting channel.

## Codebase-specific guardrails

- Do not add a second top-level app entrypoint; the CLI lives under `cmd/generator`.
- Keep config keys aligned with the JSON tags already in use (`ai_provider`, `generation_interval_hours`, `output_dir`, etc.).
- Preserve the existing Hugo front matter semantics; generated markdown is meant to be consumed by Hugo.
- Prefer file-saving through the existing front matter API instead of ad hoc writing logic.
- If you change behavior around provider models or output paths, update the corresponding tests and examples, especially in `cmd/generator/*.go`.

## Existing tests to use as examples

- `cmd/generator/config_test.go`: verifies env overrides for API keys.
- `cmd/generator/frontmatter_test.go`: verifies front matter quoting and YAML round-tripping.

Use these as the baseline style for new Go tests in this repo.


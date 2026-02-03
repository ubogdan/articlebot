# Article Bot

A Hugo markdown linter and auto-fixer for technical blogs. Catches typos, code errors, broken links, outdated content, and structural issues.

## Features

- **Typo Detection**: Catches common typos in prose and code blocks
- **Code Syntax Checking**: Language-specific checks for Go, Bash, Terraform
- **Link Validation**: Detects broken, placeholder, and non-HTTPS links
- **Outdated Content Detection**: Flags old library versions and deprecated patterns
- **Metadata Validation**: Ensures required front matter fields are present
- **Structure Analysis**: Checks heading hierarchy and section length
- **Auto-Fix**: Automatically fixes many issues when run with `-fix`

## Installation

```bash
# Clone and build
git clone https://github.com/yourusername/article-bot
cd article-bot
make build

# Or install directly
make install
```

## Usage

### Basic Analysis

```bash
# Analyze all markdown files in a directory
./build/articlebot -dir ./content/posts

# Analyze a single file
./build/articlebot -file ./content/posts/my-article.md

# Verbose output
./build/articlebot -dir ./content -verbose
```

### Auto-Fix Mode

```bash
# Automatically fix issues where possible
./build/articlebot -dir ./content -fix

# Fix a single file
./build/articlebot -file ./content/posts/my-article.md -fix
```

### Report Formats

```bash
# Text output (default)
./build/articlebot -dir ./content

# Markdown report
./build/articlebot -dir ./content -format markdown > report.md

# JSON report (for CI integration)
./build/articlebot -dir ./content -format json > report.json
```

### Custom Configuration

```bash
# Use custom config
./build/articlebot -dir ./content -config ./my-config.json
```

## Configuration

Create a `config.json` file to customize behavior:

```json
{
  "min_word_count": 400,
  "max_word_count": 4000,
  "required_front_matter": ["title", "date", "description", "tags"],
  "required_sections": ["Prerequisites", "Conclusion"],
  "common_typos": {
    "teh": "the",
    "alot": "a lot"
  },
  "code_typos": {
    "vars.": "var.",
    "lsoff": "lsof"
  },
  "outdated_patterns": [
    {
      "pattern": "version\\s*=\\s*\"~>\\s*3\\.0\"",
      "replacement": "version = \"~> 5.0\"",
      "message": "AWS provider 3.x is outdated"
    }
  ]
}
```

## Issue Types

| Type | Description | Auto-Fixable |
|------|-------------|--------------|
| `typo` | Spelling/grammar errors | ✅ |
| `code_syntax` | Code block errors | ✅ (some) |
| `missing_meta` | Missing front matter | ⚠️ (partial) |
| `missing_section` | Missing recommended sections | ❌ |
| `broken_link` | Invalid or problematic links | ✅ (some) |
| `outdated` | Outdated versions/patterns | ✅ |
| `style` | Style guide violations | ❌ |
| `structure` | Document structure issues | ❌ |

## Example Output

```
═══════════════════════════════════════════════════════════════
                    ARTICLE BOT ANALYSIS REPORT                 
═══════════════════════════════════════════════════════════════

Files analyzed: 3
Files with issues: 3
Total issues: 12
Issues fixed: 5

By Severity:
  ❌ error: 3
  ⚠️ warning: 7
  ℹ️ info: 2

═══════════════════════════════════════════════════════════════
                         FILE DETAILS                          
═══════════════════════════════════════════════════════════════

📄 xenserver-autostart.md
   Title: Setting Citrix XenServer to Auto-Start Virtual Machines
   Word count: ~180
   Issues: 4 (2 fixed)

   ❌ Line 45: Possible code typo: 'other-confi:' ✓ FIXED
      💡 Did you mean 'other-config:'?
   ⚠️ Line 12: Possible typo: 'preview'
      💡 Did you mean 'previous'?
```

## CI Integration

### GitHub Actions

```yaml
name: Lint Articles

on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Build Article Bot
        run: |
          git clone https://github.com/yourusername/article-bot
          cd article-bot && make build
      
      - name: Lint Articles
        run: ./article-bot/build/articlebot -dir ./content -format json > report.json
      
      - name: Upload Report
        uses: actions/upload-artifact@v4
        with:
          name: lint-report
          path: report.json
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Run article bot on staged markdown files
staged_md=$(git diff --cached --name-only --diff-filter=ACM | grep '\.md$')

if [ -n "$staged_md" ]; then
    for file in $staged_md; do
        articlebot -file "$file" || exit 1
    done
fi
```

## Development

```bash
# Run tests
make test

# Run with coverage
make coverage

# Format code
make fmt

# Run linter
make lint
```

## Built-in Checks

### Code Block Checks

**Go:**
- Missing package declaration
- Unhandled errors
- Common typos (func, return, string, etc.)

**Bash:**
- Dangerous commands (rm -rf /)
- sudo usage warnings
- Common typos (lsof, grep, echo, etc.)

**Terraform:**
- Hardcoded credentials
- Outdated provider versions
- Variable name typos

### Content Checks

- Repeated words ("the the")
- Common misspellings
- Word count thresholds
- Heading hierarchy violations
- Long sections without subheadings

### Link Checks

- Empty URLs
- Placeholder links (#, TODO)
- Non-HTTPS links
- Spaces in internal links

## License

MIT

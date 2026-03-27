---
name: web-tools
description: Local-first web search and reading CLI for AI agents. Zero cost, no API keys. Use this skill whenever the user asks to search the web, find information online, read an article or webpage, extract content from a URL, or convert files (PDF, DOCX, PPTX, XLSX) to Markdown. Replaces mcp__web_search and mcp__web_reader. Trigger on phrases like "search for", "look up", "find information", "read this article", "what does this page say", "search the web", "google this", or any task that needs web information retrieval.
allowed-tools: Bash(web-tools web-search:*), Bash(web-tools web-reader:*), Bash(docker compose:*)
---

# web-tools — Local-first web search & reading CLI

Local-first web search and reading tools for AI agents. Zero cost, no API keys, no third-party dependencies.

## When to use

- Need to **search the web** for information → `web-search`
- Need to **read/extract content** from a URL or file → `web-reader`
- User asks "look this up", "find information about", "search for", "read this article/page"
- Any task that currently uses `mcp__web_search__web_search_prime` or `mcp__web_reader__webReader` should use these CLIs instead

## Prerequisites

- **web-search requires**: SearXNG Docker container running on `localhost:8888`
  - Start: `docker compose up -d` (from the `docker/` directory in this repo)
  - Verify: `curl -s -o /dev/null -w "%{http_code}" http://localhost:8888`
- **web-reader**: Works standalone. Optional dependencies:
  - `markitdown` (`pip install markitdown`) — for PDF/DOCX/PPTX/XLSX file conversion
  - `agent-browser` (`npm i -g agent-browser`) — for browser fallback on JS-rendered pages

## Building

```bash
# Clone and build
git clone https://github.com/JinFanZheng/web-tools.git
cd web-tools
go build -o web-tools .
# Optionally install to PATH
mv web-tools /usr/local/bin/  # or ~/go/bin/
```

Or download pre-built binaries from [GitHub Releases](https://github.com/JinFanZheng/web-tools/releases).

This produces a single binary `web-tools` with two subcommands.

---

## web-search

Search the web using a local SearXNG instance (aggregates Google, Bing, DuckDuckGo, etc.).

### Usage

```bash
web-tools web-search "<query>" [flags]
```

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | flag | false | JSON structured output |
| `-o, --output` | string | stdout | Write output to file |
| `-n, --limit` | int | 5 | Number of results |
| `--locale` | string | `auto` | Language preference: `zh-CN`, `en-US`, `auto` |
| `--category` | string | `general` | Category: `general` / `images` / `news` / `videos` / `files` |
| `--time-range` | string | `any` | Time filter: `any` / `day` / `week` / `month` / `year` |
| `--engine` | string | `searxng` | Search engine (only searxng available) |

### Common patterns

```bash
# Basic search
web-tools web-search "latest AI news"

# Chinese search, last week, 3 results, JSON output
web-tools web-search "人工智能最新进展" --locale zh-CN --time-range week --limit 3 --json

# News category
web-tools web-search "Tesla" --category news --time-range day

# Write results to file
web-tools web-search "Go readability library" -o /tmp/search-results.md
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (bad params, network timeout) |
| 4 | SearXNG engine unavailable (container not running) |

---

## web-reader

Extract readable content from a URL or local file. Supports web pages, PDFs, Office documents, and text files.

### Usage

```bash
web-tools web-reader <input> [flags]
```

`<input>` can be a URL (`http://`/`https://`) or a local file path. Type is auto-detected.

### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | flag | false | JSON structured output |
| `-o, --output` | string | stdout | Write output to file |
| `--extract` | string | `main` | Extract mode: `main` (body text) / `full` (full page) |
| `--max-words` | int | 0 | Limit output word count (0 = unlimited) |
| `--timeout` | int | 15 | Request timeout in seconds |
| `--no-cache` | flag | false | Ignore cache, force re-fetch |
| `--browser` | flag | false | Force browser rendering via agent-browser |
| `--session` | string | — | agent-browser session name (for login state) |
| `--user-agent` | string | built-in | Custom User-Agent string |
| `--format` | string | `markdown` | Output format: `markdown` / `text` / `html` |

### Input type detection

| Input | Type | Processing |
|-------|------|------------|
| `https://...` | Web page | HTTP fetch → readability extraction → Markdown |
| `*.pdf` | PDF file | markitdown subprocess conversion |
| `*.docx`, `*.doc` | Word file | markitdown subprocess conversion |
| `*.pptx`, `*.ppt` | PowerPoint | markitdown subprocess conversion |
| `*.xlsx`, `*.xls` | Excel file | markitdown subprocess conversion |
| `*.html`, `*.htm` | Local HTML | readability extraction |
| `*.md`, `*.txt`, `*.json`, `*.xml`, `*.csv` | Text file | Direct read, no conversion |

### Cache behavior

- URL requests are cached locally at `~/.cache/web-tools/`
- Cache key: SHA256 of URL, TTL: 300 seconds (5 min)
- Use `--no-cache` to force re-fetch

### Browser fallback

For JS-rendered pages (SPAs) where readability extraction fails:
- Use `--browser` to force browser mode via agent-browser
- Use `--session <name>` to reuse a login session

### Common patterns

```bash
# Read a web article
web-tools web-reader https://example.com/article

# Read with JSON output
web-tools web-reader https://example.com/article --json

# Truncate to 100 words for quick summary
web-tools web-reader https://example.com/article --max-words 100

# Force browser rendering for SPA pages
web-tools web-reader https://some-react-app.com/page --browser

# Read with login session
web-tools web-reader https://internal.company.com/doc --session work-session

# Convert a local PDF
web-tools web-reader ./report.pdf

# Convert office documents to Markdown
web-tools web-reader ./slides.pptx
web-tools web-reader ./data.xlsx
```

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error (bad params, network timeout) |
| 2 | Input unreachable (404, file not found) |
| 3 | Content extraction failed |
| 4 | Engine unavailable (markitdown/agent-browser not found) |

---

## Combined workflow (agent patterns)

### Search then read

```bash
# Step 1: Search
web-tools web-search "Go readability library" --limit 3 --json

# Step 2: Read top results
web-tools web-reader https://github.com/go-shiori/go-readability
web-tools web-reader https://another-result.com/page
```

### Handle JS-heavy sites

```bash
# First try normal extraction
web-tools web-reader https://spa-site.com/page

# If content is sparse (check stderr warnings), retry with browser
web-tools web-reader https://spa-site.com/page --browser
```

### File conversion

```bash
web-tools web-reader ./report.pdf -o /tmp/report.md
web-tools web-reader ./slides.pptx -o /tmp/slides.md
```

## Configuration

Config file (optional): `~/.config/web-tools/config.json` or `./web-tools.json`

```json
{
  "reader": {
    "cache_dir": "~/.cache/web-tools",
    "cache_ttl": 300,
    "default_timeout": 15,
    "browser_fallback": true,
    "markitdown_path": "markitdown",
    "agent_browser_path": "agent-browser"
  },
  "search": {
    "searxng_url": "http://localhost:8888",
    "default_limit": 5,
    "default_locale": "auto"
  }
}
```

Environment variables override config file:
- `SEARXNG_URL` — SearXNG instance address
- `WEB_READER_CACHE_TTL` — Cache TTL in seconds
- `WEB_READER_TIMEOUT` — Default HTTP timeout
- `WEB_READER_NO_BROWSER` — Disable browser fallback
- `MARKITDOWN_PATH` — Path to markitdown binary

---
name: web-tools
description: Local-first web search and reading CLI for AI agents. Zero cost, no API keys. Use this skill whenever the user asks to search the web, find information online, read an article or webpage, extract content from a URL, or convert files (PDF, DOCX, PPTX, XLSX) to Markdown. Trigger on phrases like "search for", "look up", "find information", "read this article", "what does this page say", "search the web", "google this", or any task that needs web information retrieval.
allowed-tools: Bash(web-tools:*), Bash(command -v:*), Bash(go version:*), Bash(go build:*), Bash(git clone:*), Bash(curl:*), Bash(chmod:*), Bash(mkdir:*), Bash(cp:*), Bash(mv:*), Bash(docker compose:*)
---

# web-tools — Local-first web search & reading CLI

Local-first web search and reading tools for AI agents. Zero cost, no API keys, no third-party dependencies.

## When to use

- Need to **search the web** for information → `web-search`
- Need to **read/extract content** from a URL or file → `web-reader`
- Need to check local setup or optional dependencies → `doctor`
- User asks "look this up", "find information about", "search for", "read this article/page"
- Any task that currently uses `mcp__web_search__web_search_prime` or `mcp__web_reader__webReader` should use these CLIs instead

## First use

If `web-tools` is missing, install it before attempting search or read work.
Prefer GitHub Releases for normal use, or build from source when working from a
checkout:

```bash
git clone https://github.com/koda-claw/web-tools.git
cd web-tools
SKILL_DIR="$HOME/.codex/skills" sh scripts/install.sh
```

Then verify the runtime:

```bash
web-tools --version
web-tools doctor --json
```

For agents that only have the repository URL, copy `skills/web-tools` into the
agent's local skills directory, then rely on this skill for the search/read
workflow.

## Prerequisites

- **web-reader**: Works standalone, no external services needed. Optional dependencies:
  - `markitdown` (`pip install markitdown`) — for PDF/DOCX/PPTX/XLSX file conversion
  - `agent-browser` (`npm i -g agent-browser`) — for browser fallback on JS-rendered pages
- **web-search**: Works standalone via DuckDuckGo Lite (zero dependencies). Optional advanced backend:
  - SearXNG (aggregates Google, Bing, DDG): requires Docker → `cd docker && docker compose up -d`
  - Verify SearXNG: `curl -s -o /dev/null -w '%{http_code}' http://localhost:8888`

## Building

```bash
# Clone and build
git clone https://github.com/koda-claw/web-tools.git
cd web-tools
go build -o web-tools .
# Optionally install to PATH
mv web-tools ~/.local/bin/
```

Or download pre-built binaries from [GitHub Releases](https://github.com/koda-claw/web-tools/releases).

This produces a single binary `web-tools` with two subcommands.
It also includes `web-tools doctor` for local diagnostics.

---

## doctor

Check local configuration and optional dependencies.

```bash
web-tools doctor
web-tools doctor --json
```

`doctor` checks config loading, cache directory access, optional MarkItDown, optional agent-browser, and optional SearXNG reachability. Missing optional dependencies produce warnings rather than hard failures.

---

## web-search

Search the web using DuckDuckGo Lite by default (zero dependencies), or a local SearXNG instance for higher throughput and more sources.

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
| `--engine` | string | `auto` | Search engine: `auto` / `duckduckgo` / `searxng` |
| `--include-domain` | string list | — | Only include matching domains; repeat or comma-separate |
| `--exclude-domain` | string list | — | Exclude matching domains; repeat or comma-separate |

Domain filters match exact domains and subdomains. Search results are normalized and deduplicated by URL before output.

### Common patterns

```bash
# Basic search
web-tools web-search "latest AI news"

# Localized search, last week, 3 results, JSON output
web-tools web-search "AI latest developments" --locale en-US --time-range week --limit 3 --json

# News category
web-tools web-search "Tesla" --category news --time-range day

# Write results to file
web-tools web-search "Go readability library" -o /tmp/search-results.md

# Restrict or exclude domains
web-tools web-search "Go readability library" --include-domain github.com
web-tools web-search "AI news" --exclude-domain reddit.com,medium.com
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
| `--extract` | string | `main` | Extract mode: `main` / `full` (all readability fields) |
| `--max-words` | int | 0 | Limit output word count (0 = unlimited) |
| `--timeout` | int | 15 | Request timeout in seconds |
| `--no-cache` | flag | false | Ignore cache, force re-fetch |
| `--browser` | flag | false | Force browser rendering via agent-browser |
| `--session` | string | — | agent-browser session name (for login state) |
| `--user-agent` | string | built-in | Custom User-Agent string |
| `--format` | string | `markdown` | Output format: `markdown` / `text` / `html` |

Format behavior:
- `markdown`: metadata comments plus extracted content (default)
- `text`: extracted body text only, with no metadata comments
- `html`: extracted HTML only when available; plain text and converted local files return a structured input error
- `--json`: stable JSON envelope with all available fields; `--format` only controls non-JSON rendering

Reader quality:
- JSON output includes `quality.score`, `quality.word_count`, `quality.min_words`, `quality.needs_fallback`, and `quality.reasons`
- Sparse extraction warnings are written to stderr, not stdout
- HTTP 4xx/5xx responses do not trigger browser fallback; browser fallback is for network/extraction failures and explicit `--browser`

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

## Agent research workflow

This tool is primarily for Agent research workflows. Prefer composing `web-search` and `web-reader` through this skill, and do not assume a combined `web-research` command exists by default.

### Recommended research loop

1. Check the local setup when the task is broad, blocked, or depends on optional browser/file conversion tools:

```bash
web-tools doctor --json
```

2. Search with structured output:

```bash
web-tools web-search "Go readability library" --limit 5 --json
```

3. Select URLs from the search results. Prefer sources whose title, snippet, domain, and ranking match the user goal. Use domain filters when the task asks for a specific source type or when noisy domains should be skipped:

```bash
web-tools web-search "Go readability library" \
  --include-domain github.com \
  --exclude-domain reddit.com,medium.com \
  --limit 5 \
  --json
```

4. Read selected URLs with JSON output:

```bash
web-tools web-reader "https://github.com/go-shiori/go-readability" --json
```

5. Inspect `quality.score`, `quality.needs_fallback`, `quality.word_count`, and stderr warnings. Retry with browser only when extraction is sparse or the page is likely JS-rendered:

```bash
web-tools web-reader "https://example.com/article" --browser --json
```

6. Synthesize the answer outside web-tools. Preserve source URLs, mention partial failures when they affect confidence, and do not treat read failures as evidence that the source lacks relevant information.

### Agent policy

- Keep `web-search` and `web-reader` as explicit, debuggable steps.
- Do not hide individual search or read errors.
- Do not invent citations; cite URLs actually searched/read.
- Prefer JSON for Agent workflows because stdout remains machine-consumable and warnings stay on stderr.
- Use `--browser` selectively; it is a fallback for sparse extraction or JS-rendered pages, not a default for every URL.
- Use `--include-domain` and `--exclude-domain` to encode source constraints from the user request.
- For high-stakes or current information tasks, read multiple independent sources before answering.

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
    "default_locale": "auto",
    "default_engine": "auto"
  }
}
```

Environment variables override config file:
- `SEARXNG_URL` — SearXNG instance address
- `WEB_READER_CACHE_TTL` — Cache TTL in seconds
- `WEB_READER_TIMEOUT` — Default HTTP timeout
- `WEB_READER_NO_BROWSER` — Disable browser fallback
- `MARKITDOWN_PATH` — Path to markitdown binary

These overrides are applied by both `web-tools web-search` and `web-tools web-reader` at runtime.

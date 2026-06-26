---
name: web-tools
description: Local-first web search and reading CLI for AI agents. Zero cost by default, no API keys required for the built-in path. Use this skill whenever the user asks to search the web, find information online, read an article or webpage, extract content from a URL, or convert files (PDF, DOCX, PPTX, XLSX) to Markdown. Trigger on phrases like "search for", "look up", "find information", "read this article", "what does this page say", "search the web", "google this", or any task that needs web information retrieval.
allowed-tools: Bash(web-tools:*), Bash(command -v:*), Bash(go version:*), Bash(go build:*), Bash(git clone:*), Bash(curl:*), Bash(chmod:*), Bash(mkdir:*), Bash(cp:*), Bash(mv:*), Bash(docker compose:*)
---

# web-tools — Local-first web search & reading CLI

Local-first web search and reading tools for AI agents. Zero cost by default; no API keys are required for the built-in path.

## When to use

- Need to **search the web** for information → `web-search`
- Need to **read/extract content** from a URL or file → `web-reader`
- Need to check local setup or optional dependencies → `doctor`
- Need to inspect local tool health or recent failures → `metrics`
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
web-tools setup --check --json
web-tools metrics --json
```

If `web-tools` is already installed, check whether it can be upgraded before
starting research work:

```bash
web-tools upgrade --check --json
web-tools upgrade
```

`web-tools upgrade` updates the CLI and installs the matching `web-tools` skill
from the same release tag. It verifies release checksums and does not modify
`config.json`, env files, or cache directories. If the binary is installed in a
non-standard location, use:

```bash
web-tools upgrade --bin-dir "$HOME/.local/bin"
```

If the CLI binary is installed but this skill is missing or stale, install it
from the CLI:

```bash
web-tools setup
```

For agents that only have the repository URL, either run `web-tools skill
install` after installing the binary, or copy `skills/web-tools` from the source
checkout into the agent's local skills directory.

`web-tools gui` is for human local setup and diagnostics. Agents should use the
non-interactive CLI commands in this skill instead of depending on the GUI.

## Prerequisites

- **web-reader**: Works standalone, no external services needed. Optional dependencies:
  - `markitdown` (`pip install markitdown`) — for PDF/DOCX/PPTX/XLSX file conversion
  - `agent-browser` (`npm i -g agent-browser`) — for browser fallback on JS-rendered pages
- **web-search**: Works standalone via DuckDuckGo Lite (zero dependencies). Optional advanced backend:
  - SearXNG (aggregates Google, Bing, DDG): requires Docker → `cd docker && docker compose up -d`
  - Explicit built-in providers `bing`, `baidu`, and `sogou`: no key, but captcha/rate-limit prone; use only when selected by the task or user
  - Verify SearXNG: `curl -s -o /dev/null -w '%{http_code}' http://localhost:8888`
  - Optional MCP providers can be configured through `providers.<id>` and selected with `--provider`

## Setup and provider configuration

Use setup check before changing configuration:

```bash
web-tools setup --check --json
```

If BigModel/Zhipu MCP is needed, configure it through the CLI. Do not hand-edit
`config.json` unless a user explicitly asks for it:

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --skip-doctor
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --set-env ZHIPU_APIKEY=<token> --skip-doctor
web-tools setup --check --json
```

The token is stored in `~/.config/web-tools/.env` with `0600` permissions when
`--set-env` is used. `config.json` stores only `auth_env`, never the token
value. The CLI automatically loads `~/.config/web-tools/.env`. It does not
automatically load a project-local `./.env`.

If a user keeps secrets in another env file, run commands with `WEB_TOOLS_ENV`
pointing at that file:

```bash
WEB_TOOLS_ENV=/path/to/web-tools.env web-tools setup --check --json
WEB_TOOLS_ENV=/path/to/web-tools.env web-tools web-search "Go readability library" --provider bigmodel --json
```

Existing shell environment variables take precedence over env file values.
`WEB_TOOLS_ENV` can override values from `~/.config/web-tools/.env`, but not
values already exported in the shell before `web-tools` starts.

Enable provider auto chains only when the user has accepted any remote provider
privacy and cost implications:

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --enable-search-auto --skip-doctor
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --enable-reader-auto --skip-doctor
```

`reader auto` starts as `builtin-reader`. Add remote reader fallback only when
the user explicitly enables it or existing config already includes it; once
configured, `web-reader --provider auto` may use that chain automatically.

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

## metrics

Inspect local, non-sensitive aggregate health metrics.

```bash
web-tools metrics --json
web-tools metrics --range 24h --json
web-tools metrics reset --json
```

Use metrics when a task is blocked, a provider appears unreliable, or the user
asks whether web-tools has been working locally. Metrics are not a factual web
source; they only describe local command execution health.

Safe fields include command status, duration, provider id, result count, reader
quality, fallback recommendation, and error category. Metrics must not contain
queries, URLs, titles, content, local file paths, headers, tokens, env values,
or detailed error strings.

The metrics command does not record itself. To disable metrics collection for a
command or session:

```bash
WEB_TOOLS_NO_METRICS=1 web-tools web-search "Go readability library" --json
```

To use a specific metrics file:

```bash
WEB_TOOLS_METRICS_FILE=/tmp/web-tools-metrics.json web-tools metrics --json
```

If the user asks to clear local statistics, run:

```bash
web-tools metrics reset
```

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
| `--provider` | string | `auto` | Search provider: `auto` / `duckduckgo` / `searxng` / `bing` / `baidu` / `sogou` / configured provider id |
| `--engine` | string | `auto` | Compatibility alias for built-in search engines |
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

# Use an explicit provider
web-tools web-search "Go readability library" --provider duckduckgo --json
web-tools web-search "Go readability library" --provider bing --json
web-tools web-search "人工智能 最新进展" --provider baidu --json
web-tools web-search "人工智能 最新进展" --provider sogou --json
```

If `--provider` and `--engine` are both explicitly passed with different values,
the command fails with a structured input error. Prefer `--provider` in new
workflows.

`bing`, `baidu`, and `sogou` are explicit built-in providers. They are not part of the
default `auto` chain unless the user configures `search.default_provider_chain`
to include them. They pace requests by provider and retry only temporary
502/503/504 failures once. Captcha, 403, and 429 responses are structured
rate-limit errors; do not loop retries or claim the result is unavailable only
because one explicit provider was blocked. Rate-limited providers enter a short
in-process cooldown. Auto/custom chains skip cooling providers temporarily; when
the cooldown expires, the next request probes the provider and clears the state
on success.
Sogou is especially captcha-sensitive on some networks, so treat it as an
optional Chinese-query source, not a stable default fallback.

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
| `--provider` | string | `auto` | Reader provider: `auto` / `builtin-reader` / configured MCP provider id |

Format behavior:
- `markdown`: metadata comments plus extracted content (default)
- `text`: extracted body text only, with no metadata comments
- `html`: extracted HTML only when available; plain text and converted local files return a structured input error
- `--json`: stable JSON envelope with all available fields; `--format` only controls non-JSON rendering

Reader quality:
- JSON output includes a `quality` object. `quality.score` is a string enum (`"high"` / `"medium"` / `"low"` / `"empty"`), NOT a numeric value. Some pages (pure CSR-SPA) may omit the `quality` field entirely; use `word_count` and content structure for heuristic fallback.
- Other quality fields: `quality.word_count`, `quality.min_words`, `quality.reasons` (extraction mode hint)
- Sparse extraction warnings are written to stderr, not stdout
- HTTP 4xx/5xx responses do not trigger browser fallback; browser fallback is for network/extraction failures and explicit `--browser`
- If the user has configured `reader.default_provider_chain` such as `["builtin-reader", "bigmodel"]`, `web-reader --provider auto` will try the next configured reader provider when builtin extraction is empty/low quality or hits a fallback-eligible fetch/extraction error.
- Reader provider fallback is not captcha/login/paywall bypass. Preserve the URL and report those limits honestly.

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

# Use a configured MCP reader provider
web-tools web-reader https://example.com/article --provider bigmodel --json
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

1. 🔴 **Environment readiness check** — Run when the task is broad, blocked, or depends on optional browser/file conversion tools:

```bash
web-tools doctor --json
```

2. 🔴 **Pre-search checks** — For multi-search tasks, ensure: adjacent `web-search` calls spaced ≥ 2s apart; repeated same-query searches spaced ≥ 10s apart (cache first result with `-o`); if previous search returned exit=1 or empty array, wait 10s before next. Then search with structured output:

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

5. 🔴 **Read quality check (required)** — Inspect the `quality` fields from JSON output and stderr warnings. Apply the following decision table:

| Condition | Action |
|-----------|--------|
| `quality.score == "high"`, or (no `quality` field AND `word_count > 200` AND content has structured headings/paragraphs) | ✅ Use directly |
| `quality.score == "medium"` or `"low"` | 🔴 If reader auto chain includes a configured provider, rerun `web-reader URL --provider auto --json --no-cache`; otherwise retry with `--browser` for JS-rendered pages |
| `quality` field missing AND `word_count < 200` | 🔴 Content may be sparse or CSR empty-shell; check stderr warnings then decide `--browser` |
| `quality.word_count < 50` (regardless of score) | 🔴 Content too sparse; this source is likely not an article page — move to next URL in search results |
| stderr has warnings AND `--browser` not yet used | 🔴 Extraction incomplete; retry with `--browser` |
| After `--browser`, score is `"low"` AND word_count < 100 | 🔴 Source truly unusable; report honestly and switch to next source |
| After `--browser`, word_count dropped AND original reader had >500 words of structured content | ⚠️ Page is likely SSR-SPA (server-rendered); keep the original reader result — `--browser` introduces nav/sidebar chrome noise |

> **SSR vs CSR detection**: If the builtin reader returns >500 words with headings/lists/code blocks, the server already rendered complete HTML (SSR/SSG). `--browser` adds chrome noise for SSR pages. Reserve `--browser` for CSR-only pages (HTML is an empty shell, reader extracts <200 words).

```bash
# Browser fallback
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

## Exception handling

This tool chains multiple optional dependencies and network boundaries. When the following conditions arise, follow the table — do **not** silently swallow errors or fabricate results.

### Search exceptions

| Trigger | One-line fix | Still-fails fallback |
|---------|-------------|---------------------|
| `web-search` returns 0 results | Switch `--locale` / shorten keywords / widen `--time-range` | Switch to `--provider duckduckgo` (if using SearXNG); or retry with English keywords |
| `web-search` exit=4 (SearXNG unreachable) | Auto-fallback to DuckDuckGo (`--provider duckduckgo`) | Default `--provider auto` includes fallback logic; usually no manual intervention needed |
| `web-search` exit=1 (network timeout) | Retry once (interval ≥ 2 seconds) | Report "network currently unreachable" honestly; do not guess results |
| DuckDuckGo results are clearly irrelevant | Add `--include-domain` to restrict to trusted domains; or switch `--locale en-US` for English search | Narrow to 2–3 high-trust targeted searches |
| DuckDuckGo suspected rate-limiting (consecutive exit=1 or empty returns) | 🔴 Wait 10 seconds → `web-tools doctor --json` confirm network → retry once | Switch `--locale` to reduce per-request load; use `--provider searxng` for multi-round searches (if deployed) |
| Bing/Baidu/Sogou explicit provider returns captcha/rate-limited | Let the provider cooldown expire; retry once later with a narrower query | Switch provider (`baidu` or `sogou` for Chinese-mainland oriented queries, `bing`/`duckduckgo` for broader queries); do not rapid-loop |
| Frequent multi-round searches trigger rate-limiting | Increase search interval to ≥ 3 seconds; cache results with `-o` to avoid re-searching same terms | Deploy local SearXNG (one-time Docker start, persistent availability, 10x+ throughput) |
| DDG persistently rate-limited AND no SearXNG/Docker | Wait 10-15 seconds, retry once with narrower terms, or use an explicitly configured search provider | Report the search limitation honestly; do not fabricate results |

### Reader exceptions

| Trigger | One-line fix | Still-fails fallback |
|---------|-------------|---------------------|
| `web-reader` exit=2 (404/file not found) | Verify URL spelling; check if login required | Report "source unreachable" honestly; take next URL from search results |
| `web-reader` exit=3 (content extraction failed) | Use configured reader auto provider fallback if available; otherwise `--browser` force browser rendering | Search for alternative sources; cross-validate critical info from at least 2 independent sources |
| `web-reader` exit=4 (engine unavailable) | `pip install 'markitdown[pdf,docx,pptx,xlsx]'` or `npm i -g agent-browser` | Pure-text URLs can still be read directly (no markitdown/agent-browser dependency) |
| `quality.score == "low"` AND still low after `--browser` | Page may be navigation/landing/login-wall/anti-scraping | 🔴 Abandon this source; move to next URL in search results |
| Cache returns stale content | `--no-cache` force refresh | Accept cache within TTL (300s), unless task is time-sensitive |
| `--browser` needed but agent-browser not installed | `npm i -g agent-browser` (one-time install, persistent reuse) | Accept lower-quality plain-text result, but flag reduced confidence to user |

### Dependency exceptions

| Trigger | One-line fix | Still-fails fallback |
|---------|-------------|---------------------|
| `doctor` reports markitdown missing | `pip install 'markitdown[pdf,docx,pptx,xlsx]'` | PDF/DOCX/PPTX/XLSX files cannot be parsed; ask user for plain-text version |
| `doctor` reports SearXNG unreachable | No action needed — `--provider auto` falls back to DuckDuckGo | If high-throughput search is needed, start SearXNG with Docker: see [SearXNG docs](https://docs.searxng.org/) |
| DDG rate-limiting makes search unavailable | Wait 10-15 seconds, retry once, or use configured SearXNG / remote provider if available | Report current search unavailability honestly |

> **Agent rule**: Reader failure ≠ source irrelevance. Failure only means "could not retrieve this time." Report honestly, preserve the URL, and let the user judge.

## Agent Forbidden Actions

> The following behaviors are **strictly prohibited**. Violating any item = output is untrustworthy.

| # | Forbidden | Correct |
|---|----------|---------|
| 1 | **Silently swallow errors or fabricate results** — pretend success on network timeout / rate-limit / 404 | Report error type and exit code honestly; preserve URL for user judgment |
| 2 | **Invent citations** — output URLs or content never actually searched/read | Only cite sources genuinely obtained through `web-search`/`web-reader` |
| 3 | **Treat reader failure as "source has no relevant info"** | Failure only means "could not retrieve this time"; report honestly, preserve URL |
| 4 | **Hide individual search/read errors** — skip silently and continue to next after failure | Report each call's success/failure status independently |
| 5 | **Use `--browser` as the default for every URL** | `--browser` only when readability extraction fails or score=`"low"` |
| 6 | **Hand-edit config.json** or **parse raw MCP responses** | CLI already wraps everything; use `--json` for standardized output |
| 7 | **Print or fabricate API keys** | Keys read from environment variables only; config.json stores only `auth_env` variable name |
| 8 | **Assume a combined `web-research` command exists** | Explicit step-by-step: `web-search` → select URL → `web-reader` → quality check → synthesize |
| 9 | **Immediately re-search same term when rate-limited** | Wait 5–15 seconds; switch `--locale`/`--time-range` or use `--provider searxng` |
| 10 | **Blindly use `--browser` on SSR-SPA pages** | If reader already returned >500 words of structured content, keep original — `--browser` adds chrome noise on SSR pages |
| 11 | **Guess/construct URLs for `web-reader` without searching first** | Must `web-search` first to obtain real URLs; never fabricate URLs from memory |

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
    "agent_browser_path": "agent-browser",
    "default_provider": "auto",
    "default_provider_chain": ["builtin-reader"]
  },
  "search": {
    "searxng_url": "http://localhost:8888",
    "default_limit": 5,
    "default_locale": "auto",
    "default_engine": "auto",
    "default_provider": "auto",
    "default_provider_chain": ["searxng", "duckduckgo"]
  }
}
```

Environment variables override config file:
- `SEARXNG_URL` — SearXNG instance address
- `WEB_READER_CACHE_TTL` — Cache TTL in seconds
- `WEB_READER_TIMEOUT` — Default HTTP timeout
- `WEB_READER_NO_BROWSER` — Disable browser fallback
- `MARKITDOWN_PATH` — Path to markitdown binary
- `WEB_TOOLS_ENV` — Optional env file path to load in addition to `~/.config/web-tools/.env`

These overrides are applied by both `web-tools web-search` and `web-tools web-reader` at runtime.

Env file loading rules:
- Default env file: `~/.config/web-tools/.env`
- Explicit env file: set `WEB_TOOLS_ENV=/path/to/file.env`
- Project-local `./.env` is not loaded automatically
- Shell env values win over both env files

### Optional MCP provider config

The built-in path does not require API keys. Optional remote providers must be
configured explicitly and should read secrets only from environment variables.

Prefer the CLI config command instead of hand-writing JSON:

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
```

If the user provides a token and wants persistent local CLI use, write it to the
user env file through setup:

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --set-env ZHIPU_APIKEY=...
```

This stores the token in `~/.config/web-tools/.env` with `0600` permissions.
The CLI loads that file automatically. Existing shell environment variables win
over env file values, so temporary `export ZHIPU_APIKEY=...` still works for
one-off runs.

For non-default env file locations, set `WEB_TOOLS_ENV` on the command. This is
useful for handoffs where the user provides a repo-specific or agent-specific
secret file without moving it into `~/.config/web-tools/.env`.

Then verify:

```bash
web-tools doctor --json
web-tools web-search "Go readability library" --provider bigmodel --json
web-tools web-reader "https://github.com/go-shiori/go-readability" --provider bigmodel --json
```

If the user wants `--provider auto` to try BigModel for search fallback, add it
to the search chain explicitly:

```bash
web-tools config provider add bigmodel \
  --preset bigmodel \
  --auth-env ZHIPU_APIKEY \
  --enable-search-auto
```

Do not put the API key itself in config. Store only the env var name in
`auth_env`, then rely on `doctor --json` to confirm `auth_configured=true`.
If `auth_configured=false`, ask the user to provide the token or configure the
named env var; do not invent or print secret values.

Do not parse raw MCP responses in the agent. The CLI normalizes MCP responses
into the regular `web-search --json` and `web-reader --json` envelopes.

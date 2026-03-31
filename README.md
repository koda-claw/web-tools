# web-tools

Local-first web search and reading CLI for AI agents.

Zero cost. No API keys. No third-party dependencies.

## What it does

- **web-search**: Search the web via a local SearXNG instance (aggregates Google, Bing, DuckDuckGo)
- **web-reader**: Extract readable content from URLs or convert local files (PDF, DOCX, PPTX, XLSX) to Markdown

## Install

### Download binary

Download from [GitHub Releases](https://github.com/koda-claw/web-tools/releases):

```bash
# macOS ARM64
curl -sL https://github.com/koda-claw/web-tools/releases/latest/download/web-tools-darwin-arm64 -o /usr/local/bin/web-tools && chmod +x /usr/local/bin/web-tools

# macOS x64
curl -sL https://github.com/koda-claw/web-tools/releases/latest/download/web-tools-darwin-amd64 -o /usr/local/bin/web-tools && chmod +x /usr/local/bin/web-tools

# Linux x64
curl -sL https://github.com/koda-claw/web-tools/releases/latest/download/web-tools-linux-amd64 -o /usr/local/bin/web-tools && chmod +x /usr/local/bin/web-tools

# Windows x64
curl -sL https://github.com/koda-claw/web-tools/releases/latest/download/web-tools-windows-amd64.exe -o /usr/local/bin/web-tools.exe
```

### Build from source

Requires Go 1.23+.

```bash
git clone https://github.com/koda-claw/web-tools.git
cd web-tools
go build -o web-tools .
```

## Quick start

### 1. Start SearXNG (required for web-search)

```bash
cd docker && docker compose up -d
```

Verify: `curl -s http://localhost:8888/search?q=test&format=json | head -c 200`

### 2. Install optional dependencies

```bash
# For file conversion (PDF, DOCX, PPTX, XLSX)
pip install markitdown

# For browser fallback (JS-rendered pages)
npm i -g agent-browser
```

### 3. Use

```bash
# Search
web-tools web-search "latest AI news"
web-tools web-search "人工智能" --locale zh-CN --limit 3

# Read a URL
web-tools web-reader https://example.com/article

# Convert a file
web-tools web-reader ./report.pdf
```

## Install as Agent Skill

Compatible with [vercel-labs/skills](https://github.com/vercel-labs/skills) CLI:

```bash
npx skills add koda-claw/web-tools
```

This installs the SKILL.md to your agent's skills directory, enabling AI agents to use web-tools automatically.

## Architecture

```
web-tools
├── cmd/web-reader/     # web-reader CLI entry point
├── cmd/web-search/     # web-search CLI entry point
├── internal/
│   ├── config/         # Configuration loading (file + env + defaults)
│   ├── errors/         # Structured error handling for agent consumption
│   ├── reader/         # HTTP fetch, readability extraction, cache, converter, browser fallback
│   └── search/         # SearXNG client, result parsing, output formatting
├── docker/             # SearXNG docker-compose.yml + settings
└── skills/             # Agent skill documentation (SKILL.md)
```

## License

MIT

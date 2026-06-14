# Research Workflow Design

## Purpose

This document evaluates whether web-tools should add a combined search-then-read workflow, such as a future `web-tools web-research` command.

The goal is to improve agent ergonomics without hiding the composability and debuggability of the existing `web-search` and `web-reader` commands.

## Recommendation

Do not implement a new command yet.

Use the existing commands plus skill documentation as the default research workflow:

1. Run `web-tools web-search <query> --json`.
2. Select URLs from the structured search results.
3. Run `web-tools web-reader <url> --json` for each selected URL.
4. Let the calling agent synthesize, cite, or summarize the collected material.

This keeps the tool small, testable, and transparent. It also lets agents decide which sources to read, how many to read, whether to skip low-quality domains, and when to retry with `--browser`.

## Why Not Implement Now

- The current tools already compose cleanly through JSON.
- A combined command would need source selection policy, retry policy, citation policy, deduplication policy, and partial-failure policy.
- Those policies are agent- and task-dependent. Baking them into the CLI too early would make the tool less predictable.
- `web-reader` browser fallback and quality scoring are still evolving. A combined command should not hide those signals.

## Current Supported Workflow

### Basic Research

```bash
web-tools web-search "golang readability library" --limit 5 --json
web-tools web-reader "https://github.com/go-shiori/go-readability" --json
```

### Filtered Research

```bash
web-tools web-search "golang readability library" \
  --include-domain github.com \
  --exclude-domain reddit.com \
  --limit 5 \
  --json
```

### Reader Retry

```bash
web-tools web-reader "https://example.com/article" --json
web-tools web-reader "https://example.com/article" --browser --json
```

Use the retry only when the first read has low quality metadata or stderr warns that extraction is sparse.

## Proposed Future Command

If approved later, add:

```bash
web-tools web-research <query> [flags]
```

### Proposed Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--limit` | `5` | Number of search results to inspect |
| `--read-limit` | `3` | Number of selected URLs to read |
| `--engine` | config default | Search engine strategy |
| `--locale` | config default | Search locale |
| `--include-domain` | empty | Only include matching domains |
| `--exclude-domain` | empty | Exclude matching domains |
| `--browser` | false | Force browser rendering for reads |
| `--retry-browser` | false | Retry low-quality reads with browser fallback |
| `--json` | false | Structured output |
| `--output` | stdout | Output file |

### Proposed JSON Shape

```json
{
  "ok": true,
  "result": {
    "query": "golang readability library",
    "searched_at": "2026-06-14T00:00:00Z",
    "search": {
      "engine": "duckduckgo",
      "total": 5,
      "results": []
    },
    "reads": [
      {
        "rank": 1,
        "url": "https://example.com/article",
        "status": "ok",
        "result": {
          "title": "Example",
          "content": "...",
          "quality": {
            "score": "high",
            "needs_fallback": false
          }
        }
      }
    ],
    "errors": []
  }
}
```

The command should preserve each underlying search and reader result instead of returning only a synthetic summary. Summarization should stay outside web-tools unless a separate design explicitly approves it.

## Non-Goals For Future Command

- Do not summarize or rank claims.
- Do not invent citations.
- Do not hide individual search/read errors.
- Do not require live SearXNG or browser dependencies.
- Do not replace `web-search` or `web-reader`.

## Approval Gates

Implement `web-research` only if at least one of these becomes true:

- Repeated users or agents need the same search-then-read boilerplate.
- Skill documentation is not enough to produce reliable workflows.
- The desired source-selection policy becomes stable enough to encode.
- There is a clear JSON contract that downstream agents will consume directly.

Before implementation, approve:

- URL selection policy.
- Partial failure behavior.
- Browser retry behavior.
- JSON schema.
- Whether Markdown output should include full read content or only links and metadata.

## Implementation Plan If Approved

### Task A: Add Workflow Types

**Files:** `internal/research/*`

- Define search/read orchestration structs.
- Keep summaries out of scope.
- Preserve embedded `SearchResponse` and `PipelineResult`-compatible data.

**Verify**

- Unit tests for partial successes and failures.

### Task B: Add Command

**Files:** `cmd/web-research/*`, `main.go`

- Add Cobra command and flags.
- Reuse existing config loading.
- Keep stdout machine-consumable.

**Verify**

- Command tests with fixture search and reader functions.

### Task C: Add Documentation

**Files:** `README.md`, `skills/web-tools/SKILL.md`

- Document when to use `web-research` versus explicit command composition.

**Verify**

- Help output and docs agree.

## Test Strategy If Approved

Required tests should stay offline:

- Search fixture returns duplicate and filtered URLs.
- Reader fixture returns high-quality, low-quality, and failed reads.
- Partial failure still returns successful reads and structured errors.
- `--retry-browser` invokes the reader retry only for low-quality reads.
- JSON output remains stable.
- Markdown output does not hide source URLs.

## Current Decision

Task 9 is complete with this design note. The current approved path is documentation and explicit command composition only. Do not implement `web-research` until the approval gates above are met.

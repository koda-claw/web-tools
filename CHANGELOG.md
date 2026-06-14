# Changelog

## Unreleased

### Added

- `web-tools setup` for one-command Agent setup, optional provider configuration, skill install, and doctor checks.
- User env file auto-loading from `~/.config/web-tools/.env`, with `WEB_TOOLS_ENV` override support.
- `web-tools setup --set-env KEY=value` with `--env-file` and `--force-env` for non-interactive env file setup.
- `doctor` env file diagnostics without exposing secret values.

## v1.4.1 - 2026-06-15

Patch release for Agent setup ergonomics and documentation.

### Added

- `web-tools config provider add/list` for CLI-managed provider configuration.
- `web-tools skill install` so a downloaded CLI binary can initialize or update the Agent skill.
- Chinese README at `README.zh-CN.md`.
- README visual assets generated with the image-generator skill.

### Improved

- Agent skill guidance now prefers CLI-managed provider setup over hand-written JSON.

## v1.4.0 - 2026-06-14

Feature release for provider/plugin architecture and MCP-backed search/read providers.

### Added

- Provider/plugin configuration with built-in `searxng`, `duckduckgo`, and `builtin-reader` providers.
- `web-search --provider` with compatibility for existing `--engine` workflows.
- `web-reader --provider` with `builtin-reader` default behavior preserved.
- MCP provider adapter for Streamable HTTP responses, including SSE event parsing and double-encoded `content[0].text` JSON payloads.
- Optional BigModel/Zhipu MCP provider support through `ZHIPU_APIKEY`.
- `doctor --json` provider summaries with auth configured status and no secret value leakage.
- Provider architecture docs, Mermaid flow diagrams, and a provider plugin development guide.

### Verified

- `go test ./...`
- `go vet ./...`
- `./scripts/smoke.sh`
- `git diff --check`
- Live smoke with `ZHIPU_APIKEY`: BigModel Search MCP via `--provider bigmodel`.
- Live smoke with `ZHIPU_APIKEY`: BigModel Reader MCP via `--provider bigmodel`.

## v1.3.2 - 2026-06-14

Patch release from real-world Agent workflow testing.

### Fixed

- `web-search --engine auto` now falls back to the next engine when the current engine has raw results but all of them are removed by domain filters such as `--include-domain` or `--exclude-domain`.
- This fixes a real Agent research case where local SearXNG returned results that did not match `--include-domain github.com`, causing auto mode to stop with zero results instead of trying DuckDuckGo.

### Improved

- Source install now embeds `git describe` into `web-tools --version`, making local checkout installs easier for agents to verify.
- CLI examples were polished to keep README and command help consistent.

### Real Workflow Verification

- Installed CLI and skill with `SKILL_DIR="$HOME/.codex/skills" sh scripts/install.sh`.
- Verified `web-tools doctor --json`.
- Verified `web-tools web-search "Go readability library" --include-domain github.com --limit 3 --json`.
- Verified `web-tools web-reader "https://github.com/go-shiori/go-readability" --json --no-cache`.

## v1.3.1 - 2026-06-14

Patch release focused on release confidence and Agent handoff quality.

### Added

- Offline CLI integration tests that build a temporary `web-tools` binary and run it against local HTTP fixtures.
- Coverage for the Agent search-then-read workflow using `web-search --json` followed by `web-reader --json`.
- Integration coverage for config-driven SearXNG search, domain include/exclude filters, URL dedupe, reader `quality` metadata, and sparse-content warnings.
- README test instructions documenting `go vet ./...`, `go test ./...`, and `./scripts/smoke.sh`.

### Verification

- `go vet ./...`
- `go test ./...`
- `./scripts/smoke.sh`
- GitHub Actions CI passed.

## v1.3.0 - 2026-06-14

Feature release for Agent research workflows.

### Added

- `web-tools doctor` for local diagnostics of config, cache directory, MarkItDown, agent-browser, and optional SearXNG.
- Search domain filters: `--include-domain` and `--exclude-domain`.
- Reader `quality` metadata in JSON output and Markdown comments.
- `skills/web-tools/SKILL.md` Agent research workflow and policy.
- `scripts/install.sh` for installing the CLI and optionally copying the Agent skill via `SKILL_DIR`.
- `docs/research-workflow-design.md` describing the design-first research workflow.

### Changed

- `web-search --engine auto` now tries the next engine when a prior engine returns no results.
- Search results are normalized and deduplicated by URL.
- HTTP 4xx/5xx responses do not trigger browser fallback.
- Sparse reader extraction warnings are written to stderr so stdout remains machine-consumable.

### Not Included

- No `web-research` command yet.
- No CDP or browser backend replacement.
- No CLI-level summarization or citation generation.

# Changelog

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

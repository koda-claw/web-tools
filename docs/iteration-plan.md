# web-tools Iteration Plan

## Purpose

This plan keeps the next iteration focused: first make the existing CLI contract reliable, then add new capabilities. The current baseline is healthy, so the work should improve consistency and operability before expanding scope.

## 文档语言约定

后续新增或更新的迭代计划、方案设计、验收标准、测试用例和完成记录，默认使用中文编写。命令、参数、JSON 字段、文件路径、错误码和代码标识保持原文，避免影响实现和测试对照。

## Current Baseline

- Repository: `github.com/koda-claw/web-tools`
- Current mainline: `v1.2.1`
- Commands:
  - `web-tools web-search`
  - `web-tools web-reader`
- Search already supports `auto`, `duckduckgo`, and `searxng` engines.
- Reader already supports URL extraction, local file reading, cache, MarkItDown conversion, and optional browser fallback.
- Current verification baseline: `go test ./...`

## Success Criteria

- Existing documented flags and config fields behave as documented.
- CLI stdout remains machine-consumable; warnings and diagnostics stay on stderr.
- JSON output remains stable for agent consumption.
- README, skill documentation, and command help describe the same behavior.
- A release/smoke checklist exists so future changes can be verified quickly.
- Each completed task updates this plan with a short completion note or a changed-scope note.

## Non-Goals

- Do not redesign the CLI hierarchy.
- Do not introduce a long-running service or daemon.
- Do not add a new search provider before the current engine/config contract is complete.
- Do not change public JSON fields unless there is a migration note.
- Do not require Docker for the default search path.

## Confirmed Decisions

- `--format=html` is only supported when extracted HTML is available. Plain text, Markdown, JSON, CSV, and converted local files should return a structured input error for HTML output instead of inventing wrapper HTML.
- The first `--extract=full` implementation means "return the full content fields already produced by the readability extractor" rather than "return the original raw page HTML." Raw full-page HTML is out of scope for Phase 1.
- Phase 1 adds and runs `scripts/smoke.sh` locally, but does not require wiring the smoke script into CI. CI integration can happen after the script proves stable.
- `web-tools doctor` stays in Phase 2. It should not be pulled into Phase 1 because it depends on the config, CLI, and documentation contracts being stable first.
- The research workflow stays design-first. Phase 2 may create `docs/research-workflow-design.md`, but no `web-research` command should be implemented until that design is approved.

## Phase 1: Complete the Existing Contract

### Task 1: Fix Config Contract

**Files:** `internal/config/config.go`, `internal/config/loader.go`, `internal/config/loader_test.go`

**Decision**

- Keep the public `Config`, `ReaderConfig`, and `SearchConfig` structs as runtime value structs.
- Add a private config overlay type for JSON loading where optional scalar fields use pointers, for example `*bool` for `reader.browser_fallback` and `*string` for `search.default_engine`.
- Merge only fields present in the overlay. This preserves defaults while allowing config files to explicitly set `browser_fallback` to `false`.

**Implementation**

- Ensure `search.default_engine` from config files is merged into runtime config.
- Implement overlay-based merging for config file loading.
- Ensure `reader.browser_fallback=false` from user config, local config, and `WEB_TOOLS_CONFIG` can override the default.
- Add tests for user config, local config, and `WEB_TOOLS_CONFIG` overrides.

**Verify**

- `go test ./internal/config`
- A config file with `search.default_engine=duckduckgo` is reflected in loaded config.
- A config file can explicitly disable browser fallback.
- Existing config files without `browser_fallback` keep the default `true`.

**Acceptance Criteria**

- Runtime defaults remain unchanged when no config files or env overrides are present.
- User config, local config, and `WEB_TOOLS_CONFIG` merge in the documented priority order.
- `search.default_engine`, `search.default_limit`, and `search.default_locale` are all configurable.
- `reader.browser_fallback=false` is distinguishable from an omitted value.
- Invalid JSON config still reports a clear config-loading error.

**Unit Tests**

- `TestDefaultConfig` keeps current defaults.
- `TestLoadConfigFileOverlay_DefaultEngine` loads `search.default_engine`.
- `TestLoadConfigFileOverlay_BrowserFallbackFalse` preserves explicit false.
- `TestMergePriority_UserLocalEnv` verifies override order.
- `TestWebToolsConfigOverride` verifies `WEB_TOOLS_CONFIG`.

**Integration Tests**

- A CLI-level test or smoke fixture runs with a temp config file and confirms effective search defaults are used by `web-search`.

### Task 2: Align Reader Flags with Behavior

**Files:** `cmd/web-reader/main.go`, `cmd/web-reader/main_test.go`, possibly `internal/reader/*`

**Decision**

- `--extract=main` remains the default readability output.
- `--extract=full` returns all content fields already produced by the readability extractor when available; for local plain-text files it behaves the same as `main`.
- `--format=markdown` renders the existing metadata comments plus content.
- `--format=text` writes only the extracted text/content body to stdout or output file, without metadata comments.
- `--format=html` writes extracted HTML when available. If an input cannot provide extracted HTML, return a structured input error instead of silently producing Markdown or generated wrapper HTML.
- `--json` keeps the existing JSON envelope and includes all available content fields; `--format` controls the non-JSON renderer only.

**Implementation**

- Implement the behavior above for `--extract main/full`.
- Implement the behavior above for `--format markdown/text/html`.
- Reject unsupported flag values with structured input errors.
- Keep default behavior backward-compatible.

**Verify**

- `go test ./cmd/web-reader ./internal/reader`
- `web-tools web-reader --help` matches implemented behavior.
- Markdown, text, and HTML output modes are covered by tests or fixtures.
- Unsupported values such as `--extract raw` and `--format xml` fail before network or file work starts.

**Acceptance Criteria**

- Default `web-reader <input>` output is unchanged.
- `--format=text` produces body text only, with no metadata comments.
- `--format=markdown` produces the existing Markdown renderer output.
- `--format=html` returns extracted HTML when available and structured input errors when unavailable; it does not synthesize HTML wrappers for non-HTML inputs.
- `--json` is not broken by `--format`; JSON remains the stable envelope.
- Invalid `--extract` and `--format` values fail before fetch, conversion, or browser fallback.

**Unit Tests**

- `TestValidateReaderFlags` covers valid and invalid `--extract` / `--format` values.
- `TestRenderPipelineResultMarkdown` covers existing Markdown output.
- `TestRenderPipelineResultText` covers body-only text output.
- `TestRenderPipelineResultHTML` covers HTML output when `HTML` is present.
- `TestRenderPipelineResultHTMLUnavailable` covers structured error when HTML is missing.

**Integration Tests**

- `go run . web-reader <tmpfile> --format text` returns only file content.
- A local `httptest` page exercises URL extraction for Markdown, text, and HTML formats.
- Invalid flag invocations are asserted to exit non-zero without issuing HTTP requests.

### Task 3: Stabilize Search Runtime Defaults

**Files:** `cmd/web-search/main.go`, `cmd/web-search/main_test.go`, `internal/search/search.go`, `internal/search/search_test.go`

**Decision**

- Cobra flag defaults must not hide config defaults.
- In `cmd/web-search`, use `cmd.Flags().Changed("engine")`, `Changed("limit")`, `Changed("locale")`, `Changed("category")`, and `Changed("time-range")` to distinguish omitted flags from explicit user flags.
- When a flag is omitted, pass the zero value through to `search.Do` so `SearchConfig` defaults can apply.
- When a flag is explicitly provided, the flag value overrides config.

**Implementation**

- Update CLI option construction to follow the `Flags().Changed` decision above.
- Ensure `search.default_engine`, `search.default_limit`, and `search.default_locale` apply when corresponding flags are omitted.
- Ensure explicit CLI flags override config defaults consistently.
- Keep unsupported DuckDuckGo options visible as stderr warnings when fallback happens.
- Ensure unknown engine errors are structured enough for agent use.

**Verify**

- `go test ./cmd/web-search ./internal/search`
- Explicit engine selection works for `duckduckgo` and `searxng`.
- Auto mode still falls back from unavailable SearXNG to DuckDuckGo.
- A test command path with config `default_engine=duckduckgo` and no `--engine` uses DuckDuckGo.
- The same config plus explicit `--engine=searxng` uses SearXNG.

**Acceptance Criteria**

- Omitted flags allow `SearchConfig` defaults to apply.
- Explicit flags always override config defaults.
- `--engine`, `--limit`, `--locale`, `--category`, and `--time-range` all follow the same omitted-versus-explicit rule.
- Unknown engines return structured errors.
- Auto fallback keeps warnings on stderr and results on stdout.

**Unit Tests**

- `TestSearchDefaultsFromConfig` covers default engine, limit, and locale.
- `TestSearchExplicitOptionsOverrideConfig` covers CLI-provided options overriding config.
- `TestSearchUnknownEngineError` covers invalid engine names.
- `TestSearchAutoFallbackWarning` covers SearXNG failure followed by DuckDuckGo fallback.

**Integration Tests**

- Command-level tests build a Cobra command with temp config and no flags, then assert config defaults are used.
- Command-level tests repeat with explicit flags and assert flags win.
- No required integration test depends on live DuckDuckGo or SearXNG network access.

### Task 4: Sync Documentation

**Files:** `README.md`, `skills/web-tools/SKILL.md`, `docs/SPEC-ddg-fallback.md`

**Implementation**

- Add `default_engine` to config examples.
- Clarify which dependencies are required versus optional.
- Document DuckDuckGo Lite limitations and SearXNG advanced behavior.
- Make command examples match current help output.

**Verify**

- `go run . --help`
- `go run . web-search --help`
- `go run . web-reader --help`
- Manual review that README and skill docs do not contradict command help.

**Acceptance Criteria**

- README config examples include all supported search defaults.
- Skill documentation matches command help for `--engine`, `--extract`, and `--format`.
- Optional dependencies are labeled optional everywhere.
- DDG Lite limitations and SearXNG advanced behavior are consistent across docs.

**Unit Tests**

- None required for prose-only changes.

**Integration Tests**

- Help-output smoke checks pass.
- Manual doc review confirms examples can be copied and run.

### Task 5: Add Release Smoke Verification

**Files:** `scripts/smoke.sh`, `.github/workflows/ci.yml` if CI integration is chosen

**Decision**

- Add `scripts/smoke.sh` as the single local smoke entrypoint.
- Keep smoke deterministic and offline by default.
- Do not make live DuckDuckGo or SearXNG network calls in the required smoke path.
- Live network checks can be added later behind an explicit environment variable such as `WEB_TOOLS_SMOKE_LIVE=1`.
- Do not wire smoke into CI during Phase 1 unless that is explicitly approved after the local script is stable.

**Implementation**

- Add a lightweight smoke entrypoint for local release checks.
- Cover this required command set:
  - `go test ./...`
  - `go run . --help`
  - `go run . web-search --help`
  - `go run . web-reader --help`
  - `go run . --version`
  - `go run . web-search` with no query, expecting Cobra argument validation to fail before any network call.
  - Create a temporary `.txt` file and read it with `go run . web-reader <tmpfile> --format text`.
- Keep smoke checks fast and deterministic.

**Verify**

- `go test ./...`
- `./scripts/smoke.sh` passes locally.
- CI smoke integration is deferred until after Phase 1 unless explicitly approved.

**Acceptance Criteria**

- Smoke script is deterministic and offline by default.
- Smoke script exits non-zero on any failed command.
- Smoke script creates and cleans up its own temporary files.
- Live network checks, if added later, are opt-in only.

**Unit Tests**

- Covered by the `go test ./...` command inside smoke.

**Integration Tests**

- `./scripts/smoke.sh` itself is the integration test.
- CI integration is deferred in this phase; if later enabled, it must run after unit tests.

## Phase 2: Add Operational Capabilities

Phase 2 starts only after every Phase 1 task is complete and the release gate passes.

### Task 6: Add `web-tools doctor`

**Files:** `main.go`, `cmd/doctor/*`, `internal/config/*`, possibly `internal/reader/*`, `internal/search/*`

**Decision**

- `doctor` is a diagnostics command, not an auto-fix command.
- `doctor` must work without Docker, MarkItDown, or agent-browser installed.
- Missing optional dependencies should produce warning status, not command failure.
- `doctor --json` returns a stable top-level shape:
  - `ok`: overall bool
  - `checks`: list of `{name, status, message, details}`
  - `config`: effective config summary with sensitive values omitted

**Implementation**

- Check config file loading and effective config.
- Check SearXNG reachability.
- Check MarkItDown availability.
- Check agent-browser availability.
- Check cache directory readability/writability.
- Output both human-readable and JSON formats.

**Verify**

- `go test ./cmd/doctor ./internal/config`
- `web-tools doctor --json` returns stable fields.
- Running `web-tools doctor` succeeds even when optional dependencies are absent.

**Acceptance Criteria**

- `doctor` exits zero when only optional dependencies are missing.
- `doctor` exits non-zero only for invalid config or unrecoverable diagnostics failures.
- Human output is readable and JSON output is stable.
- `doctor --json` never includes sensitive values.

**Unit Tests**

- Tests cover each check result: ok, warn, and error.
- Tests cover JSON encoding shape.
- Tests cover optional dependency absence.

**Integration Tests**

- `go run . doctor`
- `go run . doctor --json`
- Temp config fixtures cover valid and invalid config cases.

### Task 7: Improve Search Result Quality

**Files:** `internal/search/*`, `cmd/web-search/*`

**Implementation**

- Add optional domain include/exclude filters.
- Normalize and deduplicate URLs across engines.
- Preserve engine provenance when multiple engines return the same result.
- Keep default output shape compatible.

**Verify**

- `go test ./internal/search ./cmd/web-search`
- Fixture tests cover dedupe and domain filters.

**Acceptance Criteria**

- Domain include/exclude filters are deterministic and documented.
- URL normalization does not remove meaningful query parameters unless explicitly specified.
- Deduped results preserve engine provenance.

**Unit Tests**

- URL normalization table tests.
- Domain include/exclude table tests.
- Deduplication table tests with overlapping engine results.

**Integration Tests**

- Command-level tests use fixture engines, not live network calls.

### Task 8: Improve Reader Quality Scoring

**Files:** `internal/reader/*`, `cmd/web-reader/*`

**Implementation**

- Make sparse-content detection more transparent.
- Include extraction quality metadata in JSON output.
- Improve browser fallback decision rules without forcing browser use for HTTP status errors.

**Verify**

- `go test ./internal/reader ./cmd/web-reader`
- Fixture tests cover sparse, normal, and failed extraction cases.

**Acceptance Criteria**

- Quality metadata is present in JSON output and does not affect Markdown default output.
- Sparse-content warnings remain on stderr.
- HTTP 4xx/5xx still do not trigger browser fallback.

**Unit Tests**

- Quality scoring tests for sparse, normal, and empty content.
- Fallback decision tests for extraction failure versus HTTP status errors.
- JSON output tests for quality metadata.

**Integration Tests**

- Local `httptest` pages cover normal and sparse extraction.
- No required integration test launches a real browser.

### Task 9: Evaluate a Research Workflow Command

**Files:** `docs/research-workflow-design.md` first; implementation files TBD only after design approval

**Implementation**

- Evaluate whether a combined search-then-read workflow should be a new command, a script, or skill documentation only.
- If implemented, keep it composable and avoid hiding individual `web-search` and `web-reader` behavior.

**Verify**

- A short design note exists before implementation.
- Any new command has CLI tests and JSON output tests.

## Suggested Execution Order

1. Task 1: Fix Config Contract
2. Task 2: Align Reader Flags with Behavior
3. Task 3: Stabilize Search Runtime Defaults
4. Task 4: Sync Documentation
5. Task 5: Add Release Smoke Verification
6. Task 6: Add `doctor`
7. Task 7 and Task 8 can proceed independently after Task 6
8. Task 9 only after the core tools are stable

## Verification Bundle

Run this before considering Phase 1 complete:

```bash
go test ./...
go run . --help
go run . web-search --help
go run . web-reader --help
```

If a smoke script is added, include:

```bash
./scripts/smoke.sh
```

## Phase 1 Test Matrix

| Area | Unit tests | Integration tests | Required command |
|------|------------|-------------------|------------------|
| Config contract | Overlay merge, false bool, default engine, priority order | Temp config drives CLI behavior | `go test ./internal/config` |
| Reader flags | Flag validation, Markdown/text/HTML renderers, JSON envelope | Temp file reader, local HTTP fixture, invalid flags fail before network | `go test ./cmd/web-reader ./internal/reader` |
| Search defaults | Config defaults, explicit override, unknown engine, auto fallback | Cobra command tests with temp config and fixture engines | `go test ./cmd/web-search ./internal/search` |
| Documentation | None | Help-output and manual example review | `go run . --help && go run . web-search --help && go run . web-reader --help` |
| Smoke | Package tests | Offline CLI smoke script | `./scripts/smoke.sh` |

## Phase 1 Completion Notes

Update this section as tasks land.

| Task | Status | Completion note |
|------|--------|-----------------|
| Task 1 | Complete | Config loading now uses private JSON overlays so `default_engine` and explicit `browser_fallback=false` merge correctly. Verified with `go test ./internal/config`. |
| Task 2 | Complete | Reader flags now validate `--extract` / `--format`; text and HTML renderers are implemented with JSON envelope preserved. Verified with `go test ./cmd/web-reader ./internal/reader`. |
| Task 3 | Complete | Search CLI now honors config defaults when flags are omitted and explicit flags override config. Verified with `go test ./cmd/web-search ./internal/search`. |
| Task 4 | Complete | README, skill docs, and DDG spec now describe the same config, engine, and reader format behavior. |
| Task 5 | Complete | Added offline `scripts/smoke.sh`; it runs package tests, help/version checks, argument validation, and local text reader verification. |

## Phase 2 Completion Notes

Update this section as tasks land.

| Task | Status | Completion note |
|------|--------|-----------------|
| Task 6 | Complete | Added `web-tools doctor` with human and JSON output for config, cache, optional MarkItDown, optional agent-browser, and optional SearXNG checks. Missing optional dependencies warn without failing. Verified with `go test ./cmd/doctor`. |
| Task 7 | Complete | Added search domain include/exclude filters, URL normalization, result deduplication, and engine provenance helper coverage. Verified with `go test ./cmd/web-search ./internal/search`. |
| Task 8 | Complete | Added reader quality metadata in JSON/Markdown output, clearer sparse-content warnings, and tests for quality scoring plus HTTP status fallback behavior. Verified with `go test ./cmd/web-reader ./internal/reader`. |
| Task 9 | Complete | Added `docs/research-workflow-design.md` and recommended explicit `web-search` + `web-reader` composition for now. A future `web-research` command is gated on approved selection, retry, failure, and JSON policies. |

## Release Gate

Do not tag a new release until:

- Phase 1 tasks are complete.
- The verification bundle passes.
- Documentation and help output agree.
- Any behavior change is mentioned in release notes.
- The Phase 1 Completion Notes table is updated.

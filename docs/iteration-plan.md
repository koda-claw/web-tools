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
- 下一阶段采用 Provider / Plugin 架构，而不是继续把每个外部引擎硬编码到 CLI。Search 和 Reader 都要支持 provider 化，BigModel Search MCP / Reader MCP 作为可选 MCP provider 示例接入，不进入无 key 默认链路。

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
| Post-Task 9 | Complete | Added offline CLI integration tests for Agent search/read workflows, including local SearXNG fixture, reader quality metadata, config defaults, domain filters, and dedupe behavior. Verified with `go test ./...`. |

## Phase 3: Provider / Plugin 架构

Phase 3 的目标是把“新增搜索/读取后端”从 CLI 主流程里解耦出来。后续新增同类服务时，优先通过 `providers.<id>` 配置和通用 adapter 接入；只有出现新协议或新响应结构时才写少量 adapter 代码。

### Phase 3 施工顺序

| Wave | Tasks | 目的 | 默认验证 |
|------|-------|------|----------|
| Wave 1 | Task 10, Task 11 | 固化设计和配置骨架，不改变现有行为 | `go test ./internal/config ./cmd/doctor` |
| Wave 2 | Task 12 | 建立 provider registry 与 capability 解析 | `go test ./internal/provider` |
| Wave 3 | Task 13 | Search CLI 接入 provider，同时保留 `--engine` | `go test ./cmd/web-search ./internal/search ./internal/provider` |
| Wave 4 | Task 14 | Reader CLI 接入 provider，同时保持输出兼容 | `go test ./cmd/web-reader ./internal/reader ./internal/provider` |
| Wave 5 | Task 15 | MCP mock adapter 完成，真实服务仍可选 | `go test ./internal/provider/...` |
| Wave 6 | Task 16 | BigModel 作为可选 provider 接入与文档化 | `go test ./...`，live test 需显式 env |

### Phase 3 总体验收

- 旧命令、旧配置、旧 JSON 输出默认不破。
- `web-search --engine auto`、`web-search --engine duckduckgo`、`web-search --engine searxng` 继续可用。
- 新增 `--provider` 后，文档优先使用 provider，`--engine` 作为兼容别名保留。
- `web-reader` 默认行为仍是 builtin reader，不因未配置远程 provider 变慢或失败。
- `doctor --json` 能报告 provider 配置状态，但不输出 API key、Authorization header、cookie 或 token。
- 默认 CI 全离线；所有外部 provider 使用 mock 覆盖。真实 provider 测试必须显式开启。
- `--provider` 与 `--engine` 同时显式传入且冲突时必须返回 structured input error。
- auto chain 可跳过未满足 `enabled_if_env` 的可选 provider；显式选择该 provider 时缺 env 必须返回可诊断错误。

### Task 10: Provider 架构设计

**文件：** `docs/provider-architecture.md`

**依赖：** 无

**并行：** 可与 Task 11 的代码阅读并行；不可与最终文档审查并行

**目标**

- 定义 search 和 reader 共享的 provider/plugin 架构。
- 明确 `builtin`、`http`、`mcp`、`exec` provider 类型。
- 为 BigModel Search MCP / Reader MCP 预留标准接入方式。
- 明确保留 `--engine` 与旧配置兼容，新增 `--provider` 作为后续主入口。
- 明确 BigModel 是可选 MCP provider，不作为无 key 默认后端。
- 明确新增 provider 的落地路径：配置优先，adapter 次之，协议扩展最后。

**验收标准**

- 文档描述 provider 配置、CLI 行为、doctor 诊断、安全边界和测试策略。
- 文档包含分阶段迁移计划。
- 文档明确第一步只做配置和 registry，不直接接 BigModel。
- 文档列出未确认项，并给出默认建议。

**测试用例**

- 文档评审：确认现有 `searxng`、`duckduckgo`、builtin reader 都有迁移路径。
- 文档评审：确认 BigModel Search MCP 和 Reader MCP 都能通过 `mcp` provider 表达。
- 文档评审：确认无 key 默认链路仍是本地优先。
- 文档评审：确认每个后续 task 都有自动化验证方式。

### Task 11: Provider 配置骨架

**文件：** `internal/config/*`

**依赖：** Task 10

**并行：** 不建议并行修改 `internal/config/*`

**目标**

- 增加顶层 `providers` 配置。
- 增加 `search.default_provider`、`search.default_provider_chain`。
- 增加 `reader.default_provider`、`reader.default_provider_chain`。
- 保持 `search.default_engine` 兼容。
- 增加 provider 的 `type`、`auth_env`、`enabled_if_env`、`timeout`、`search`、`reader`、`command`、`capabilities` 字段。
- `doctor --json` 增加 provider 配置摘要。
- 定义认证配置行为：auto chain 中缺 env 的可选 provider 跳过；显式 provider 缺 env 报错。

**验收标准**

- 旧配置文件无需修改即可继续工作。
- 新 provider 配置可加载、可合并、可被 env override 补充。
- Secret 只通过 env 引用，不直接打印。
- `default_provider` 缺省时继续使用旧 `default_engine`。
- `default_provider_chain` 缺省时使用内置默认链。
- 缺少 `auth_env` 的可选 provider 不影响默认无 key 链路。

**测试用例**

- 单元测试：旧配置加载结果不变。
- 单元测试：provider 配置 overlay 合并正确。
- 单元测试：缺省 provider chain 使用内置默认值。
- 单元测试：`auth_env` 只报告 configured true/false，不输出 env 值。
- 集成测试：临时 `WEB_TOOLS_CONFIG` 配置 provider 后，`doctor --json` 能看到 provider summary。
- 单元测试：auto chain 跳过缺 env 的 `enabled_if_env` provider。
- 单元测试：显式 provider 缺 env 返回 structured config/auth error。

### Task 12: Provider Registry

**文件：** `internal/provider/*`

**依赖：** Task 11

**并行：** 可与 README 草稿并行；不要与 Task 13/14 同时改 registry API

**目标**

- 定义 `Provider`、`SearchProvider`、`ReaderProvider`、`ProviderRegistry`。
- 注册内置 `searxng`、`duckduckgo`、`builtin-reader`。
- 支持按 capability 查找 provider。
- 支持解析 provider chain，并跳过 `enabled_if_env` 未满足的 provider。
- 定义 provider attempt metadata，供 search/reader fallback 复用。

**验收标准**

- 未知 provider 返回结构化错误。
- capability 不匹配返回结构化错误。
- provider chain 可按顺序解析。
- `auto` chain 能从配置或默认值解析。
- registry 不依赖真实网络。

**测试用例**

- 单元测试：registry 注册和查找。
- 单元测试：provider chain fallback。
- 单元测试：capability mismatch。
- 单元测试：`enabled_if_env` 未满足时 provider 不进入可用链。
- 单元测试：重复 provider id 注册返回明确错误。

### Task 13: Search CLI 接入 Provider

**文件：** `cmd/web-search/*`、`internal/search/*`、`internal/provider/*`

**依赖：** Task 12

**并行：** 不与 Task 14 并行修改 provider interface；可在 registry API 稳定后并行

**目标**

- 新增 `--provider`。
- 保留 `--engine` 作为兼容别名。
- `auto` 改为 provider chain。
- 把现有 search engine 包装为 search provider，不重写 SearXNG/DDG 查询逻辑。
- 输出中保留旧 `engine` 字段，并新增兼容 provider metadata。

**验收标准**

- 所有现有 search 测试通过。
- `--engine` 行为不变。
- `--provider` 可以选择 `searxng`、`duckduckgo`、`auto`。
- 同时传入 `--provider` 和 `--engine` 且值不一致时返回 structured input error；值一致时允许执行。
- filtered empty results fallback 行为不回退。

**测试用例**

- 单元测试：`--provider` 优先级。
- 集成测试：mock provider chain。
- 集成测试：filtered empty results fallback 保持可用。
- 单元测试：未知 provider 返回 structured engine/provider error。
- CLI 测试：旧 `--engine duckduckgo` 输出仍可被现有 Agent 消费。
- 单元测试：`--provider bigmodel` 缺少配置的 `auth_env` 时失败且不泄漏 key 名以外的敏感信息。

### Task 14: Reader CLI 接入 Provider

**文件：** `cmd/web-reader/*`、`internal/reader/*`、`internal/provider/*`

**依赖：** Task 12；建议在 Task 13 的 provider metadata 形态稳定后开始

**并行：** 可与 Task 13 在接口冻结后并行；不要同时改 shared output metadata

**目标**

- 新增 `--provider`。
- `builtin-reader` 作为默认 provider。
- 为 reader fallback 到远程 provider 预留 metadata。
- 把当前 URL/file/browser pipeline 包装为 `builtin-reader`，保持缓存和 quality 行为。
- low-quality fallback 先只对 mock provider 开启测试，不默认调用远程服务。
- 远程 reader fallback 必须由 `reader.default_provider_chain` 或后续显式配置开启，默认只执行 `builtin-reader`。

**验收标准**

- 所有现有 reader 测试通过。
- `--provider builtin-reader` 输出和当前默认行为一致。
- reader JSON 输出向后兼容。
- low-quality fallback attempts 可追踪，但不会污染 Markdown/text/html stdout。
- 未配置远程 provider 时默认读取不变。
- 低质量 builtin reader 结果不会在默认配置下自动调用 BigModel 或其他远程 provider。

**测试用例**

- 单元测试：reader provider 选择。
- 集成测试：本地 reader 低质量结果触发 mock provider fallback。
- 集成测试：HTML/text/markdown 输出不破。
- 单元测试：reader provider capability mismatch。
- 集成测试：`--provider builtin-reader --json` 与默认路径关键字段一致。
- 集成测试：默认配置下低质量 reader 不触发远程 fallback。

### Task 15: MCP Provider Adapter

**文件：** `internal/provider/mcp/*`

**依赖：** Task 12；Search/Reader 接入可先用 mock provider，MCP adapter 后接入

**并行：** 可与 BigModel 文档草稿并行；不与 registry API 变更并行

**目标**

- 实现 MCP HTTP client adapter。
- 支持 tool 调用和结果映射。
- 用 mock MCP server 覆盖 search 和 reader。
- 第一阶段优先支持 Streamable HTTP endpoint，并解析 `text/event-stream` response。
- Authorization 从 `auth_env` 读取，只在 request header 中使用。
- 先冻结 MCP mock request/response fixture：`initialize`、`tools/list` 或静态 tool、`tools/call`、SSE event、JSON-RPC error、HTTP error、timeout、malformed result。
- BigModel live probe 已验证 Search tool 为 `web_search_prime`，Reader tool 为 `webReader`，两者 tool result 都是 `content[0].text` 内的 JSON 字符串。

**验收标准**

- 不依赖真实 BigModel API 即可测试。
- Timeout、非 2xx、tool error 都映射为结构化错误。
- Authorization header 不在日志或错误中泄漏。
- Search MCP 和 Reader MCP 都能通过同一个 adapter 类型表达。
- MCP adapter 不包含 BigModel 专用分支，BigModel 只由配置决定 URL/tool。
- MCP adapter 通过 mock server 验证后，才允许接真实 BigModel live test。
- MCP adapter 能解析 SSE `data:` event 内的 JSON-RPC payload。
- MCP adapter 能对 `content[0].text` 做 JSON unquote 和 JSON parse。

**测试用例**

- 单元测试：MCP search response 映射。
- 单元测试：MCP reader response 映射。
- 集成测试：mock MCP search/read provider。
- 单元测试：missing `auth_env` 时返回可诊断错误，不 panic。
- 单元测试：错误消息不包含测试 token。
- 单元测试：malformed MCP search result 返回映射错误。
- 单元测试：reader 文本块 fallback 行为可预测。
- 单元测试：HTTP 400/JSON-RPC error 不泄漏 Authorization header。
- Live smoke：`ZHIPU_APIKEY=...` 时 Search MCP `tools/list` 和 Reader MCP `tools/list` 返回预期 tool name。

### Task 16: BigModel Provider 接入

**文件：** `internal/provider/bigmodel/*`、README、`skills/web-tools/SKILL.md`

**依赖：** Task 15

**并行：** README 与 skill 可以并行；live test 必须在代码完成后顺序执行

**目标**

- 通过 MCP provider 配置接入 BigModel Search MCP 和 Reader MCP。
- 使用 `ZHIPU_APIKEY`，或允许用户通过 `auth_env` 改成其他环境变量名。
- 更新 skill，指导 Agent 如何启用和选择 BigModel provider。
- 文档说明它需要账号、API key 或对应计划额度。
- 无 key 时不进入默认 auto chain。

**验收标准**

- 无 key 场景默认链路不受影响。
- 有 key 时 `--provider bigmodel` search/read 可用。
- `doctor --json` 显示 BigModel provider 配置状态，不显示 key。
- README 和 skill 均说明安装 CLI、安装 skill、配置 provider、运行 doctor、执行 search/read 的路径。

**测试用例**

- 离线 mock MCP 测试进入默认 CI。
- 真实 BigModel 测试通过显式环境变量开启，不进入默认 CI。
- 文档测试：无 key 配置示例不会把真实 token 写入文件。
- Skill 审查：Skill 文档保持英文，不混入中文。

## Phase 3 未确认项

| 项目 | 建议默认决策 | 需要用户确认的时机 |
|------|--------------|--------------------|
| BigModel 是否加入默认 auto chain | 不加入；只有显式配置且 env 存在时才加入 | Task 16 前 |
| MCP 是否支持 SSE | BigModel Streamable HTTP 实测返回 `text/event-stream`，第一阶段需要解析 SSE event；独立 `/sse` endpoint 暂不做 | Task 15 前按此实现 |
| HTTP provider 是否做通用 mapping DSL | 暂不做，等第一个目标服务明确 | 接 Tavily/Exa/Brave 前 |
| Reader low-quality 是否自动远程 fallback | 默认关闭，通过配置启用 | Task 14/16 前 |
| Provider metadata 字段名 | 优先 `provider`、`provider_chain`、`attempts` | Task 13 前冻结 |
| `--provider` 与 `--engine` 冲突 | 返回 structured input error | Task 13 前按此实现 |
| 显式 provider 缺少认证 env | 返回 config/auth error，不 fallback | Task 11/12 前按此实现 |

## Release Gate

Do not tag a new release until:

- The relevant phase tasks are complete.
- Documentation and help output agree.
- Any behavior change is mentioned in release notes.
- The matching Completion Notes table is updated.

For Phase 3 releases, also require:

- `go test ./...` passes.
- Provider tests are offline by default and do not require external API keys.
- `doctor --json` does not leak API keys, Authorization headers, cookies, or tokens.
- Existing `--engine` behavior and existing JSON fields remain compatible.
- New provider docs are reflected in README and, when applicable, `skills/web-tools/SKILL.md`.
- Skill docs remain English.
- Live provider tests, if any, are run only with explicit env vars and recorded as optional verification.

## Phase 3 Completion Notes

Update this section as tasks land.

| Task | Status | Completion note |
|------|--------|-----------------|
| Task 10 | Complete | Provider architecture docs finalized with config, registry, CLI compatibility, MCP, BigModel optional-provider boundary, testing strategy, live probe notes, and open decisions. |
| Task 11 | Complete | Added top-level `providers` config, search/reader default provider fields, provider chain overlay loading, `WEB_TOOLS_CONFIG` merge coverage, and doctor provider summaries without secret leakage. Verified with `go test ./internal/config ./cmd/doctor`. |
| Task 12 | Complete | Added `internal/provider` registry with capability checks, chain resolution, `enabled_if_env` skipping, explicit auth errors, and attempt metadata. Verified with `go test ./internal/provider`. |
| Task 13 | Complete | Added `web-search --provider`, conflict handling with `--engine`, provider metadata in JSON, builtin provider chain support, and MCP search provider support. Verified with `go test ./cmd/web-search ./internal/search`. |
| Task 14 | Complete | Added `web-reader --provider`, kept `builtin-reader` as the default path, and added MCP reader provider support for URL input. Verified with `go test ./cmd/web-reader ./internal/reader`. |
| Task 15 | Complete | Added `internal/provider/mcp` Streamable HTTP adapter with SSE parsing, JSON-RPC handling, double-encoded `content[0].text` parsing, mock MCP tests, and error redaction checks. Verified with `go test ./internal/provider/mcp`. |
| Task 16 | Complete | Added BigModel/Zhipu MCP config docs using `ZHIPU_APIKEY`, README and skill guidance, and live smoke verification for search/read via `--provider bigmodel`. Verified with `go test ./...` and explicit live smoke. |
| Post-Task 16 | Complete | Added `docs/provider-plugin-development-guide.md`, linked it from README, added Mermaid architecture/flow diagrams, and prepared v1.4.0 release notes. Verified with `go test ./...`, `go vet ./...`, `./scripts/smoke.sh`, and `git diff --check`. |

## Phase 4: Agent 易用性迭代

Phase 4 从“能用”转向“好用”，优先补齐 Agent 安装、配置和自检闭环。

### Task 17: CLI 管理 Provider 配置

**目标**

- 新增 `web-tools config provider add bigmodel --preset bigmodel --auth-env ZHIPU_APIKEY`。
- 新增 `web-tools config provider list`。
- 默认写入 `~/.config/web-tools/config.json`，支持 `--config` 指定路径。
- 只写 `auth_env` 名称，不写真实 token。
- 支持 `--enable-search-auto` 显式把 BigModel 加入 search fallback chain。

**验收标准**

- Agent 不需要手写 JSON 即可配置 BigModel provider。
- 配置命令幂等；重复执行不会产生重复 chain。
- `doctor --json` 能看到 provider 状态。
- 缺少 `ZHIPU_APIKEY` 时 provider 为 warn，不破坏默认无 key 路径。

### Task 18: CLI 初始化 Agent Skill

**目标**

- 新增 `web-tools skill install`。
- 默认安装到 `~/.codex/skills/web-tools/SKILL.md`。
- release binary 默认从 GitHub 对应 tag 下载 skill；dev build 默认从 `main` 下载。
- 支持 `--source ./skills/web-tools/SKILL.md` 便于源码 checkout 离线安装。

**验收标准**

- 只有 CLI binary 时也能初始化 skill。
- 已存在 skill 时默认拒绝覆盖，`--force` 可更新。
- Skill 文档指导 Agent 用 CLI 配置 provider，而不是手写 JSON。

## Phase 5: Setup / Env File / Interactive 版本化迭代

Phase 5 继续把 setup 从“命令式可用”推进到“用户和 Agent 都好用”。详细计划见 `docs/setup-env-iteration-plan.md`。

Phase 5 已完成的发布：

| Version | Scope | 目标 |
|---------|-------|------|
| v1.4.2 | Env file 自动加载 + setup 写 env file | 支持 `~/.config/web-tools/.env`，让用户配置一次后 CLI 直接可用。 |

后续 `setup check / repair 建议` 不再单独发布，而是并入 Phase 6 的 `v1.5.0 Local GUI 管理台` 主线。

Phase 5 的安全边界：

- `config.json` 只保存 `auth_env`，不保存真实 API key。
- 默认加载 `~/.config/web-tools/.env`，不默认加载当前目录 `.env`。
- 当前进程环境变量优先级最高。
- Skill 默认指导 Agent 使用非交互命令，不默认使用 `--interactive`。

## Phase 6: Local GUI 可用性迭代

Phase 6 把 CLI/skill/provider/env/doctor 的闭环进一步可视化，提供本地 GUI 管理入口。详细计划见 `docs/gui-iteration-plan.md`。

推荐发布口径：

| Version | Scope | 目标 |
|---------|-------|------|
| v1.5.0 | Local GUI 管理台 | 一次性完成 setup check / repair API、GUI MVP、诊断导出、Agent Guide 和 reader auto 推荐策略。 |

内部里程碑不单独打 tag，全部验收通过后统一发布 `v1.5.0`。

Phase 6 的安全边界：

- GUI 默认只监听 `127.0.0.1`。
- GUI 不展示、不记录、不返回 secret 明文。
- GUI 不替代 CLI；所有配置写入仍走既有 config/env/setup 逻辑。
- Reader/Search 远程 provider auto 开关必须显式确认，不能静默打开。

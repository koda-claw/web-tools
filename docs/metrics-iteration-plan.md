# 本地指标统计迭代计划

## 背景

`web-tools` 已经具备 CLI、skill、provider、GUI、setup check、upgrade 的基础闭环。下一步需要让人类用户和 Agent 能判断工具运行状态：

- 最近哪些命令在使用。
- 搜索和读取是否稳定。
- provider 是否频繁失败或 fallback。
- reader 提取质量是否偏低。
- upgrade/setup 是否健康。

本轮指标只做本地统计和本地展示，不做远程上报。

## 总体目标

新增一个本地优先的指标统计能力：

```bash
web-tools metrics --json
web-tools metrics
web-tools metrics reset
```

默认行为：

1. CLI 命令执行时记录非敏感运行指标。
2. 指标写入本地 state 文件，路径按平台默认规则解析。
3. `metrics --json` 输出稳定 JSON，供 Agent 和 GUI 消费。
4. GUI Dashboard 展示指标摘要。
5. diagnostics 导出包含指标摘要，但不包含 query、URL、正文、token、env value。

## 非目标

- 不做远程 telemetry。
- 不自动上传、同步或分享指标。
- 不记录用户 query、URL、网页正文、文件路径明细或 secret。
- 不做 Prometheus server、OpenTelemetry exporter 或复杂 tracing。
- 不在第一版做无限期原始事件保留。
- 不在第一版做复杂自定义时间选择器；先支持固定区间。

## 设计原则

- **本地优先**：所有指标只写本机。
- **默认低风险**：只记录聚合计数、耗时、状态，不记录原始输入。
- **Agent 可读**：JSON 稳定，字段含义明确。
- **可关闭/可清理**：支持 env/flag 禁用写入，支持 `metrics reset`。
- **轻量可靠**：指标写入失败不影响主命令成功。
- **跨平台**：Linux/macOS/Windows 都有 state path 和原子写策略；文件锁作为后续增强。

## 数据边界

### 可以记录

- command：`web-search`、`web-reader`、`setup`、`doctor`、`upgrade`、`gui-test-search`、`gui-test-reader`。
- provider/engine id：例如 `duckduckgo`、`searxng`、`builtin-reader`、`bigmodel`。
- status：`success`、`error`。
- error category：`input`、`network`、`extract`、`engine`、`internal`。
- duration_ms。
- result_count、word_count bucket、quality score。
- fallback/retry 是否发生或是否建议。
- current_version、target_version、binary_mode 等 upgrade 非敏感字段。
- last_seen_at、first_seen_at、total_count、success_count、error_count。
- time bucket：按小时/天聚合后的非敏感计数。

### 禁止记录

- search query。
- URL 原文。
- 页面 title、正文、metadata 原文。
- 本地文件路径。
- API key、token、env file 内容。
- HTTP headers、request body。
- 完整错误 detail 中可能含 URL/token 的内容。

## 存储设计

默认路径：

- Linux：优先 `XDG_STATE_HOME/web-tools/metrics.json`，否则 `~/.local/state/web-tools/metrics.json`。
- macOS：`~/Library/Application Support/web-tools/metrics.json`。
- Windows：`%LOCALAPPDATA%\\web-tools\\metrics.json`。

可覆盖：

```bash
WEB_TOOLS_METRICS_FILE=/tmp/web-tools-metrics.json web-tools metrics --json
WEB_TOOLS_NO_METRICS=1 web-tools web-search "query"
```

GUI 读取同一个 metrics 文件：

- `internal/gui` 调用 `internal/metrics` 的默认 path resolver。
- GUI 继承 `web-tools gui` 进程环境中的 `WEB_TOOLS_METRICS_FILE`。
- 第一阶段不新增 `web-tools gui --metrics-file`。
- 如果后续需要 GUI 专属路径，再单独扩展 flag。

写入策略：

1. 读取现有 JSON；不存在则初始化。
2. 更新聚合计数。
3. 写入同目录临时文件。
4. rename 到目标路径。
5. 写入失败只输出 debug/静默，不影响主命令。

第一版不强制跨进程文件锁。并发写入采用 best-effort 原子写；如果 JSON 损坏，保留 `.corrupt.<timestamp>` 并重新初始化。

## JSON Schema 建议

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-16T00:00:00Z",
  "period": {
    "first_seen_at": "2026-06-16T00:00:00Z",
    "last_seen_at": "2026-06-16T00:10:00Z"
  },
  "commands": {
    "web-search": {
      "total": 12,
      "success": 10,
      "error": 2,
      "last_status": "success",
      "last_duration_ms": 820,
      "avg_duration_ms": 760
    }
  },
  "providers": {
    "search:duckduckgo": {
      "total": 8,
      "success": 7,
      "error": 1,
      "avg_duration_ms": 900
    }
  },
  "reader_quality": {
    "high": 7,
    "medium": 2,
    "low": 1,
    "fallback_recommended": 1
  },
  "errors": {
    "network": 2,
    "input": 1
  },
  "upgrade": {
    "last_check_at": "2026-06-16T00:10:00Z",
    "last_target_version": "v1.6.0",
    "last_checksum_verified": true
  },
  "time_buckets": {
    "hour": {
      "2026-06-16T00:00:00Z": {
        "commands": {
          "web-search": {
            "total": 3,
            "success": 2,
            "error": 1,
            "avg_duration_ms": 820
          }
        },
        "errors": {
          "network": 1
        },
        "reader_quality": {
          "high": 1,
          "medium": 0,
          "low": 0,
          "fallback_recommended": 0
        }
      }
    },
    "day": {
      "2026-06-16": {
        "commands": {
          "web-reader": {
            "total": 8,
            "success": 7,
            "error": 1,
            "avg_duration_ms": 430
          }
        }
      }
    }
  },
  "recent_events": [
    {
      "at": "2026-06-16T00:10:00Z",
      "command": "web-reader",
      "status": "success",
      "duration_ms": 420,
      "provider": "builtin-reader",
      "quality": "high",
      "error_category": ""
    }
  ]
}
```

`recent_events` 是固定长度 ring buffer，第一版建议保留最近 20 条安全事件。
它只允许枚举、数值、布尔和时间字段，不记录 query、URL、title、content、file path、error detail。
该 buffer 用于 GUI 的最近耗时趋势和最近错误列表。
如果用户希望完全不保留近事件，可后续增加配置，本轮先通过 `WEB_TOOLS_NO_METRICS=1` 完全禁用写入。

`time_buckets` 用于时间区间筛选和图表趋势，不保存原始事件。第一版建议：

- hour bucket 保留最近 30 天。
- day bucket 保留最近 180 天。
- 写入 event 时同时更新总聚合、hour bucket、day bucket 和 recent_events。
- 超出保留期的 bucket 在写入或读取时清理。
- bucket key 使用本地时间还是 UTC 需要统一。建议存储使用 UTC，GUI 展示转换为浏览器本地时间。

## CLI 设计

### `web-tools metrics`

Human 输出：

```text
web-tools metrics
period: 2026-06-16 00:00:00 - 2026-06-16 00:10:00

commands:
  web-search  total=12 success=10 error=2 avg=760ms
  web-reader  total=8  success=7  error=1 avg=420ms

reader quality:
  high=7 medium=2 low=1 fallback_recommended=1
```

JSON 输出：

```bash
web-tools metrics --json
web-tools metrics --range 24h --json
web-tools metrics --range 7d --json
web-tools metrics --range all --json
```

Reset：

```bash
web-tools metrics reset
web-tools metrics reset --json
```

Flags：

| 参数 | 默认值 | 说明 |
|---|---:|---|
| `--json` | `false` | 输出 JSON。 |
| `--file` | env/default | 指定 metrics 文件路径。 |
| `--range` | `all` | 固定时间范围：`1h`、`24h`、`7d`、`30d`、`all`。 |
| `--bucket` | `auto` | 时间桶：`auto`、`hour`、`day`。 |

第一版不支持任意 `--from/--to`。后续如需要自定义时间，可在不改变存储结构的前提下增加：

```bash
web-tools metrics --from 2026-06-16T00:00:00Z --to 2026-06-16T23:59:59Z --json
```

### 记录入口

第一阶段建议记录：

- `web-search`
- `web-reader`
- `setup --check`
- `doctor`
- `upgrade`
- GUI `/api/test/search`
- GUI `/api/test/reader`

是否记录 `metrics` 自身：

- `metrics` 命令永不记录自身，避免查看行为污染统计。其他命令如需禁用写入，使用 `WEB_TOOLS_NO_METRICS=1`。

## GUI 设计

新增 API：

```text
GET  /api/metrics
GET  /api/metrics?range=24h&bucket=auto
POST /api/metrics/reset
```

Dashboard 展示：

- 命令成功率。
- 最近命令耗时。
- provider 成功/失败摘要。
- reader quality 分布。
- upgrade 最近检查结果。
- 时间范围筛选：`1h`、`24h`、`7d`、`30d`、`all`。

### 图表设计

GUI 图表使用 ECharts。
当前实现通过固定版本 CDN 加载 ECharts：

```html
https://cdn.jsdelivr.net/npm/echarts@5.5.1/dist/echarts.min.js
```

如果浏览器无法加载 ECharts，GUI 会回退到内置本地 fallback 图表，保证离线或网络受限时 Dashboard 不空白。
后续如需要完全离线发布包，可把固定版本 ECharts vendored 到 `internal/gui/assets/`，但需要单独评估大文件 diff 和许可证记录。
图表只消费 `/api/metrics` 返回的聚合数据和 `recent_events` 安全字段。

推荐图表：

| 区域 | 图表 | 数据来源 | 说明 |
|---|---|---|---|
| Summary | 数字卡片 | `commands`、`errors` | total、success rate、error count、avg duration。 |
| Commands | ECharts bar | `commands` | 各命令 total/success/error 对比。 |
| Providers | ECharts stacked bar | `providers` | provider 成功/失败分布。 |
| Reader Quality | ECharts donut 或 segmented bar | `reader_quality` | high/medium/low/fallback recommended。 |
| Duration | ECharts line | `recent_events[].duration_ms` | 最近 20 次安全事件耗时趋势。 |
| Time Range Trend | ECharts line/bar | `time_buckets` | 当前时间范围内按小时/天聚合趋势。 |
| Errors | 表格或 bar | `errors`、`recent_events` | error category 汇总和最近错误类别。 |
| Upgrade | 状态卡片 | `upgrade` | 最近 check 时间、target version、checksum verified。 |

空状态：

- 无 metrics 文件：展示空 Dashboard，提示尚无本地运行指标。
- metrics disabled：展示 disabled 状态。
- corrupt recovery：展示 warning，不展示 corrupt 文件内容。

视觉和交互：

- 跟随 GUI 现有 light/dark/system 主题。
- ECharts theme 根据当前主题切换。
- 时间范围选择器使用 segmented control。
- `1h`、`24h` 默认使用 hour bucket；`7d`、`30d` 默认使用 day bucket；`all` 使用总聚合并隐藏趋势图或展示 day bucket 全量。
- 图表容器必须有稳定高度，避免加载后布局跳动。
- 移动端使用单列布局，图表宽度不能溢出。
- Reset 操作需要确认，避免误清空。

Test 页面联动：

- Search Test 成功后展示本次 duration、provider、result_count。
- Reader Test 成功后展示本次 duration、provider、word_count、quality。
- 失败时展示 error category。

不展示：

- query。
- URL。
- 正文。
- secret。

## Agent Skill 更新

`skills/web-tools/SKILL.md` 增加：

- 需要判断工具健康或排查失败时运行 `web-tools metrics --json`。
- 不要把 metrics 当作事实来源；metrics 只描述本地工具运行状态。
- 如果用户要求清理本地统计，运行 `web-tools metrics reset`。

## 实施任务

### Wave 1：设计与存储

#### Task 1：冻结指标语义

**文件：** `docs/metrics-iteration-plan.md`、`docs/metrics-test-acceptance.md`

**实现：**

- 明确记录字段、禁止字段、存储路径、CLI/GUI/Agent 边界。
- 明确第一版不做远程上报。

**验证：**

- 文档覆盖隐私、测试、验收、跨平台。

#### Task 2：新增 `internal/metrics`

**文件：** `internal/metrics/*`

**实现：**

- 定义 `Store`、`Snapshot`、`Event`、`Recorder`。
- 实现 JSON 读写、原子写、corrupt recovery。
- 实现 env/default path 解析。
- 实现禁用写入：`WEB_TOOLS_NO_METRICS=1`。

**验证：**

- 单元测试：空文件初始化。
- 单元测试：聚合计数。
- 单元测试：原子写。
- 单元测试：corrupt 文件重命名并重建。
- 单元测试：禁用写入不创建文件。

### Wave 2：CLI

#### Task 3：新增 `cmd/metrics`

**文件：** `cmd/metrics/main.go`、`main.go`

**实现：**

- 注册 `web-tools metrics`。
- 支持 `--json`、`--file`。
- 支持 `metrics reset`、`metrics reset --json`。
- Human 输出聚合摘要。

**验证：**

- `go test ./cmd/metrics ./internal/metrics`
- `web-tools metrics --help`
- `web-tools metrics --json` 可解析。

#### Task 4：接入命令记录

**文件：** `cmd/web-search/main.go`、`cmd/web-reader/main.go`、`cmd/setup/main.go`、`cmd/doctor/main.go`、`cmd/upgrade/main.go`

**实现：**

- 在命令开始记录 start time。
- 成功/失败时记录聚合 event。
- 错误分类从 `AppError.Category` 提取。
- 不记录 query、URL、正文、文件路径。
- 命令层需要在调用 `HandleError` 或 `os.Exit` 前完成失败 event 记录，避免错误路径漏计。
- 建议提供 `metrics.ObserveCommand(start, command, attrs, err)` 这类 defer 友好 API，让成功/失败路径都走同一处 sanitize。

**验证：**

- 单元/集成测试：运行命令后 metrics 文件计数增加。
- 错误测试：input/network error 只记录 category，不记录 detail。
- `WEB_TOOLS_NO_METRICS=1` 时不写文件。

### Wave 3：Search/Reader 质量指标

#### Task 5：记录 search 结果摘要

**文件：** `cmd/web-search/main.go`、`internal/search/*`

**实现：**

- 记录 provider/engine。
- 记录 result_count。
- 记录 duration_ms、status、error category。

**验证：**

- CLI 集成测试：本地 search fixture 后 metrics 含 `web-search` 和 provider 计数。

#### Task 6：记录 reader 质量摘要

**文件：** `cmd/web-reader/main.go`、`internal/reader/*`

**实现：**

- 记录 provider。
- 记录 word_count bucket。
- 记录 quality score 和 fallback recommendation。
- 不记录 URL/title/content。

**验证：**

- CLI 集成测试：reader fixture 后 metrics 含 quality 分布。
- 稀疏内容测试：`low` 和 `fallback_recommended` 增加。

### Wave 4：GUI 与诊断

#### Task 7：GUI Metrics API

**文件：** `internal/gui/server.go`、`internal/gui/server_test.go`

**实现：**

- 新增 `GET /api/metrics`。
- `GET /api/metrics` 支持 `range` 和 `bucket` query，与 CLI `--range`、`--bucket` 使用同一套过滤逻辑。
- 新增 `POST /api/metrics/reset`。
- diagnostics 中加入 metrics summary。
- Search Test / Reader Test 写入 metrics。
- GUI metrics API 使用 `internal/metrics` 的同一套 Store 和 path resolver，保证 CLI 与 GUI 读取同一份本地文件。

**验证：**

- GUI API 单元测试：`/api/metrics` 返回 JSON。
- Reset API 清空指标。
- diagnostics 不包含 query、URL、token。

#### Task 8：GUI 页面展示

**文件：** `internal/gui/assets/*`

**实现：**

- Dashboard 增加 Metrics 区域。
- 增加时间范围筛选器：`1h`、`24h`、`7d`、`30d`、`all`。
- Test 页面结果展示 duration/provider/count/quality。
- 增加 reset 按钮。
- 中英文文案同步。
- 使用 ECharts 渲染 Commands、Providers、Reader Quality、Duration 图表。
- 图表根据当前 range 读取 filtered snapshot。
- 无数据、disabled、corrupt recovery 都有明确状态。

**验证：**

- `go test ./internal/gui`
- Playwright/CDP 或浏览器手动 smoke：ECharts 渲染非空，页面无溢出，light/dark 正常。
- 移动端视口下图表单列展示且不横向溢出。

### Wave 5：文档与验收

#### Task 9：同步文档与 skill

**文件：** `README.md`、`README.zh-CN.md`、`skills/web-tools/SKILL.md`、`CHANGELOG.md`

**实现：**

- README 增加 metrics 使用说明。
- 中文 README 同步。
- Skill 使用英文说明 metrics 排障流程。
- Changelog 增加版本条目。

**验证：**

- `rg -n "web-tools metrics|metrics reset" README.md README.zh-CN.md skills/web-tools/SKILL.md`
- Skill 不中英混写。

#### Task 10：统一验收

**文件：** `scripts/smoke.sh` 或新增 `scripts/metrics_smoke.sh`

**实现：**

- smoke 覆盖 metrics JSON、reset、禁用写入。
- 覆盖 search/reader 后指标增加。

**验证：**

- `go test ./...`
- `go vet ./...`
- `./scripts/smoke.sh`
- metrics smoke 通过。
- `git diff --check`

## 测试矩阵

### 单元测试

| 模块 | 用例 |
|---|---|
| Path | 默认路径、`WEB_TOOLS_METRICS_FILE`、Windows path |
| Store | 初始化、读写、原子写、reset |
| Aggregation | command/provider/error/quality 计数 |
| Time buckets | hour/day bucket 写入、读取、过期清理 |
| Recent events | ring buffer 只保留最近 20 条安全事件 |
| Privacy | event sanitize 不保留 query、URL、content、token |
| Disable | `WEB_TOOLS_NO_METRICS=1` 不写文件 |
| Corrupt recovery | 损坏 JSON 备份并重建 |

### 集成测试

- CLI search fixture 后 metrics 增加。
- CLI reader fixture 后 quality 增加。
- input error 后只记录 category。
- `metrics reset --json` 后计数清空。
- GUI `/api/metrics` 和 reset API 可用。
- `metrics --range 24h --json` 返回时间筛选后的聚合。
- GUI 时间范围切换后图表数据变化。

### 跨平台测试

- Linux/macOS/Windows CI 运行 metrics 单元测试。
- Windows 路径分隔和 `%LOCALAPPDATA%` 覆盖。
- 原子 rename 行为按平台条件处理。

## 验收标准

本轮完成后必须满足：

- `web-tools metrics --json` 可输出稳定 JSON。
- `web-tools metrics --range 24h --json` 可按固定时间范围输出聚合。
- `web-tools metrics reset --json` 可清空本地指标。
- search/reader/setup/doctor/upgrade 至少记录 command 成功/失败和耗时。
- search/reader 记录 provider 与质量摘要。
- GUI 能展示 metrics summary。
- GUI 使用 ECharts 展示 command/provider/quality/duration 图表。
- GUI 支持 `1h`、`24h`、`7d`、`30d`、`all` 时间范围筛选。
- diagnostics 包含 metrics summary 且不泄漏 secret/query/URL/content。
- `recent_events` 只包含安全字段，最多保留 20 条。
- `time_buckets` 只保存聚合计数，不保存原始事件。
- `WEB_TOOLS_NO_METRICS=1` 可完全禁用写入。
- 指标写入失败不影响主命令。
- README、README.zh-CN、Skill、CHANGELOG 同步。

## 当前实现映射

本轮实现已覆盖：

- `internal/metrics`：本地 JSON store、默认路径、`WEB_TOOLS_METRICS_FILE`、`WEB_TOOLS_NO_METRICS=1`、corrupt recovery、recent events、hour/day buckets、range filter。
- `cmd/metrics`：`metrics`、`metrics --json`、`metrics --range`、`metrics --bucket`、`metrics reset`。
- CLI 采集：`web-search`、`web-reader`、`setup`、`doctor`、`upgrade`。
- GUI API：`GET /api/metrics`、`GET /api/metrics?range=24h&bucket=auto`、`POST /api/metrics/reset`。
- GUI 采集：`/api/test/search`、`/api/test/reader`。
- GUI 图表：Commands、Providers、Reader Quality、Recent Duration。
- GUI 时间范围：`1h`、`24h`、`7d`、`30d`、`all`，刷新后通过 localStorage 恢复。
- 文档：README、README.zh-CN、Skill、CHANGELOG、测试验收文档。
- Smoke：`scripts/metrics_smoke.sh`，并纳入 `make check`。

当前第一版刻意保留的后续增强：

- 不做跨进程文件锁；并发写入仍是 best-effort。
- 不做任意 `--from/--to` 时间范围；先使用固定 range。
- 不把 ECharts vendored 进仓库；当前使用固定 CDN + 本地 fallback。
- 不记录 `metrics` 命令自身，避免查看行为污染统计。

## 风险与处理

| 风险 | 处理 |
|---|---|
| 误记录敏感信息 | Event 类型只接受枚举和数值字段；禁止字符串 detail；测试扫描输出。 |
| 并发写入丢计数 | 第一版 best-effort；写入失败不影响命令；后续可加文件锁。 |
| metrics 文件损坏 | 自动备份 corrupt 文件并重建。 |
| 统计污染用户结果 | `metrics` 命令默认不记录自身。 |
| 用户不想保留统计 | `WEB_TOOLS_NO_METRICS=1` 和 `metrics reset`。 |

## 建议版本

建议作为 `v1.7.0` 发布。理由：

- 新增一等 CLI 命令 `web-tools metrics`。
- 新增本地状态文件。
- GUI、diagnostics、skill 和 smoke 都会联动。

# 本地指标统计测试验收计划

## 目标

本文档用于验收 `web-tools metrics` 迭代，和 `docs/metrics-iteration-plan.md` 配套使用。

验收目标：

- 证明本地指标能记录 CLI/GUI 的非敏感运行摘要。
- 证明 `metrics --json` 和 `metrics reset --json` 对 Agent 稳定可用。
- 证明指标不会记录 query、URL、正文、文件路径、token。
- 证明指标写入失败不会破坏主命令。
- 证明 Linux/macOS/Windows 的路径和写入行为可测试。

## 发布闸门

以下任一项失败，都不能发布：

- `go test ./...` 失败。
- `go vet ./...` 失败。
- `git diff --check` 失败。
- `./scripts/smoke.sh` 失败。
- metrics smoke 失败。
- `metrics --json` 输出无法解析。
- `metrics reset --json` 后计数未清空。
- 指标文件中出现 query、URL、正文、token、env value。
- `WEB_TOOLS_NO_METRICS=1` 后仍写入 metrics 文件。
- metrics 写入失败导致 `web-search` 或 `web-reader` 主命令失败。

以下可降级但必须记录：

- 并发写入下个别计数丢失。第一版只承诺 best-effort。
- metrics 文件损坏时重建指标，但必须保留 corrupt 备份。

## 测试环境

### 本地环境

- 临时 HOME。
- 临时 `WEB_TOOLS_METRICS_FILE`。
- 本地 HTTP fixture，避免真实网络。
- 不依赖真实 provider API key。

### CI 环境

需要覆盖：

- `ubuntu-latest`
- `macos-latest`
- `windows-latest`

Windows 重点验证：

- `%LOCALAPPDATA%` 或显式 `WEB_TOOLS_METRICS_FILE`。
- path 分隔符。
- reset 删除/重建行为。

## 单元测试清单

| ID | 模块 | 场景 | 期望 |
|---|---|---|---|
| UT-001 | Path | 默认路径 | 返回平台默认 metrics 文件 |
| UT-002 | Path | `WEB_TOOLS_METRICS_FILE` | 使用显式文件 |
| UT-003 | Path | `WEB_TOOLS_NO_METRICS=1` | recorder disabled |
| UT-004 | Store | metrics 文件不存在 | 初始化 schema_version=1 |
| UT-005 | Store | 正常写入 event | command total/success 增加 |
| UT-006 | Store | error event | command error 和 error category 增加 |
| UT-007 | Store | provider event | provider 聚合增加 |
| UT-008 | Store | reader quality high/medium/low | quality 分布增加 |
| UT-009 | Store | fallback recommended | fallback_recommended 增加 |
| UT-010 | Store | reset | 计数清空，schema 保留 |
| UT-011 | Store | corrupt JSON | 原文件重命名为 `.corrupt.<timestamp>` 并重建 |
| UT-012 | Store | 目标目录不可写 | 主调用不失败，metrics 写入被忽略或返回可控错误 |
| UT-013 | Aggregation | avg_duration_ms | 按累计 duration 计算均值 |
| UT-014 | Privacy | event 带 query/URL 字段尝试 | 类型层面不接受，输出不包含敏感字符串 |
| UT-015 | Error sanitize | AppError detail 中包含 URL | 只记录 category，不记录 detail |
| UT-016 | Recent events | 连续写入 25 条 event | 只保留最近 20 条 |
| UT-017 | Recent events privacy | event 中不允许 query/URL/content/detail | JSON 中不存在敏感字段 |
| UT-018 | Time buckets | 写入跨小时 event | hour bucket 正确聚合 |
| UT-019 | Time buckets | 写入跨天 event | day bucket 正确聚合 |
| UT-020 | Time buckets retention | 超过保留期 bucket | 读取或写入时清理 |
| UT-021 | Range filter | `1h`、`24h`、`7d`、`30d`、`all` | 返回对应范围内聚合 |
| UT-022 | Range privacy | bucket JSON | 不包含 query、URL、content、token |

## CLI 集成测试清单

| ID | 场景 | 命令形态 | 期望 |
|---|---|---|---|
| IT-001 | 查看空指标 | `web-tools metrics --json` | 输出 schema 和空聚合 |
| IT-002 | reset 空指标 | `web-tools metrics reset --json` | 输出 ok=true |
| IT-003 | search 成功 | `web-tools web-search ... --json` | metrics 中 `web-search.success` 增加 |
| IT-004 | search input error | `web-tools web-search "" --json` | metrics 中 error category=input，不记录 query |
| IT-005 | reader 成功 | `web-tools web-reader <fixture> --json` | metrics 中 `web-reader.success` 和 quality 增加 |
| IT-006 | reader sparse | sparse fixture | `reader_quality.low` 和 fallback 增加 |
| IT-007 | setup check | `web-tools setup --check --json` | metrics 中 `setup.success/error` 增加 |
| IT-008 | doctor | `web-tools doctor --json` | metrics 中 `doctor` 增加 |
| IT-009 | upgrade check | `web-tools upgrade --check --json ...` | metrics 中 `upgrade` 和 checksum status 增加 |
| IT-010 | disable | `WEB_TOOLS_NO_METRICS=1 web-tools web-search ...` | 不创建或不更新 metrics 文件 |
| IT-011 | custom file | `WEB_TOOLS_METRICS_FILE=<tmp> ...` | 指标写入指定文件 |
| IT-012 | reset after use | `web-tools metrics reset --json` | 后续 `metrics --json` 计数为 0 |
| IT-013 | range 24h | `web-tools metrics --range 24h --json` | 只返回 24h 范围内聚合 |
| IT-014 | range all | `web-tools metrics --range all --json` | 返回总聚合 |
| IT-015 | invalid range | `web-tools metrics --range 3h --json` | 返回 input error |

## GUI 集成测试清单

| ID | 场景 | API | 期望 |
|---|---|---|---|
| GUI-001 | 获取指标 | `GET /api/metrics` | 返回 ok=true 和 metrics summary |
| GUI-002 | 重置指标 | `POST /api/metrics/reset` | 清空指标 |
| GUI-003 | search test 成功 | `POST /api/test/search` | metrics 增加 `gui-test-search` |
| GUI-004 | reader test 成功 | `POST /api/test/reader` | metrics 增加 `gui-test-reader` 和 quality |
| GUI-005 | diagnostics | `GET /api/diagnostics` | 包含 metrics summary |
| GUI-006 | privacy | diagnostics / metrics response | 不包含 query、URL、content、token |
| GUI-007 | custom metrics file | GUI 进程设置 `WEB_TOOLS_METRICS_FILE=<tmp>` | `/api/metrics` 读取同一文件 |
| GUI-008 | ECharts render | 打开 Dashboard | Commands、Providers、Reader Quality、Duration 图表非空渲染 |
| GUI-009 | dark/light theme | 切换主题 | ECharts theme 和页面主题一致 |
| GUI-010 | mobile layout | 移动端视口 | 图表单列展示，不横向溢出 |
| GUI-011 | empty state | 无 metrics 文件 | 展示空状态，不报错 |
| GUI-012 | reset confirm | 点击 reset | 需要确认后才清空 |
| GUI-013 | range selector | 点击 `1h`、`24h`、`7d`、`30d`、`all` | 图表使用对应 filtered snapshot |
| GUI-014 | range persistence | 刷新页面 | 保留或恢复默认 range，不出现空白图表 |
| GUI-015 | range API | `GET /api/metrics?range=24h&bucket=auto` | 返回后端过滤后的 snapshot |

## Smoke 测试

新增或扩展：

```bash
./scripts/metrics_smoke.sh
```

覆盖：

1. 临时 HOME 和 `WEB_TOOLS_METRICS_FILE`。
2. `web-tools metrics --json`。
3. 本地 fixture search。
4. 本地 fixture reader。
5. `web-tools metrics --json` 断言计数增加。
6. `web-tools metrics reset --json`。
7. `WEB_TOOLS_NO_METRICS=1` 禁用写入。
8. `recent_events` 最多 20 条，且不包含敏感字符串。
9. `web-tools metrics --range 24h --json` 可解析。
10. `web-tools metrics --range all --json` 可解析。

## GUI 图表验收

使用 Playwright、CDP 或现有浏览器测试工具验证：

```text
http://127.0.0.1:<port>/
```

检查点：

- Dashboard 中 ECharts canvas/svg 节点存在。
- Commands 图表显示命令聚合。
- Providers 图表显示 provider 成功/失败。
- Reader Quality 图表显示 high/medium/low。
- Duration 图表显示最近安全事件耗时。
- 时间范围筛选器包含 `1h`、`24h`、`7d`、`30d`、`all`。
- 切换时间范围后图表更新，不重新记录 metrics。
- 图表在 light/dark/system 主题下可读。
- 375px 宽移动端无横向溢出。
- Reset metrics 有确认动作。
- 无 metrics 文件时展示空状态。

## 隐私验收

构造以下敏感字符串：

```text
secret-token-should-not-appear
https://secret.example.com/private?q=hidden
private search query marker
private page content marker
```

执行 search/reader/GUI/diagnostics 后检查：

```bash
! rg "secret-token-should-not-appear|secret.example.com|private search query marker|private page content marker" "$WEB_TOOLS_METRICS_FILE"
```

同时检查 stdout/stderr 的 metrics JSON 不包含这些字符串。

## 发布前验收

发布前按顺序执行：

```bash
go test ./...
go vet ./...
./scripts/smoke.sh
./scripts/upgrade_smoke.sh
./scripts/metrics_smoke.sh
git diff --check
```

还需要执行：

```bash
tmp_metrics="$(mktemp)"
WEB_TOOLS_METRICS_FILE="$tmp_metrics" web-tools metrics --json
WEB_TOOLS_METRICS_FILE="$tmp_metrics" web-tools metrics reset --json
```

## 发布后验收

tag 发布后，用 release asset 冷启动：

```bash
tmp_dir="$(mktemp -d)"
curl -L https://github.com/koda-claw/web-tools/releases/download/vX.Y.Z/web-tools-darwin-arm64 -o "$tmp_dir/web-tools"
chmod +x "$tmp_dir/web-tools"
WEB_TOOLS_METRICS_FILE="$tmp_dir/metrics.json" "$tmp_dir/web-tools" metrics --json
WEB_TOOLS_METRICS_FILE="$tmp_dir/metrics.json" "$tmp_dir/web-tools" metrics reset --json
```

## 验收记录模板

```markdown
## Metrics 验收记录

- 版本：vX.Y.Z
- 日期：
- commit：
- tag：

### 本地验证

- [ ] go test ./...
- [ ] go vet ./...
- [ ] ./scripts/smoke.sh
- [ ] ./scripts/metrics_smoke.sh
- [ ] git diff --check

### 隐私验证

- [ ] metrics 文件不包含 query
- [ ] metrics 文件不包含 URL
- [ ] metrics 文件不包含 content
- [ ] metrics 文件不包含 token
- [ ] recent_events 最多 20 条
- [ ] recent_events 不包含 query/URL/content/detail
- [ ] time_buckets 不包含 query/URL/content/detail
- [ ] range filter 不泄漏原始输入

### GUI 图表

- [ ] ECharts 图表非空渲染
- [ ] ECharts 加载失败时 fallback 图表非空渲染
- [ ] 时间范围筛选器可用
- [ ] light/dark 主题正常
- [ ] 移动端无横向溢出
- [ ] reset 有确认

### CI

- [ ] ubuntu-latest
- [ ] macos-latest
- [ ] windows-latest

### 发布后

- [ ] release asset 冷启动 metrics --json
- [ ] metrics reset --json
```

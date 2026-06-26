# Bing / Baidu Search Provider 评估与实施计划

## 背景

`web-tools v1.8.0` 已完成 DuckDuckGo Lite 限流识别、短重试、auto/provider fallback 语义，以及进程内 search cache。当前默认无 key 搜索链路仍是：

```text
searxng -> duckduckgo
```

其他 agent 提出的 Python fallback 脚本包含 `Bing`、`Baidu`、`Startpage`，但它没有通过本仓库的 provider 架构、JSON 合约、测试矩阵、国内网络 gate 和 skill 发布边界。因此不能直接写进主 skill 当作 guaranteed fallback。

本计划评估并记录将 Bing / Baidu 做成正式 `web-search` provider 的方案与验收边界。当前第一阶段已实现为显式 provider，默认 chain 仍保持不变。

## 结论先行

- **已实现第一阶段**：Bing / Baidu 已按现有 `internal/search.Engine` 形态实现为 builtin search provider。
- **第一阶段只做显式 provider**：支持 `--provider bing` / `--provider baidu`，不进入默认 chain。
- **Baidu 优先级高于 Bing**：Baidu 对中国大陆中文搜索场景更有价值；Bing 更适合国际/英文补充。
- **默认链路必须后置评估**：只有多网络 smoke、fixture 稳定性、反爬风险和输出质量都达标，才讨论是否加入默认 chain。
- **Startpage 不纳入本计划**：已有 blocked 探针结果，仍保留调研项。

## 成功标准

实现前评估成功：

- 明确 Bing / Baidu 的页面可达性、反爬风险、结果结构、URL 解析规则和地区差异。
- 至少完成当前网络与中国大陆直连网络的 smoke 记录。
- 产出可复核的 live probe 记录，包含原始状态、解析摘要、失败样本和 Go / No-Go 结论。
- 形成是否进入实现阶段的 Go / No-Go 决策。
- 当前 staged / unstaged 的 skill 实验变更已被清理或隔离，未验收 Python fallback 不混入 provider 实现。

实现成功：

- `web-tools web-search "query" --provider baidu --json` 输出标准 `SearchResponse` JSON。
- `web-tools web-search "query" --provider bing --json` 输出标准 `SearchResponse` JSON。
- JSON 仍保持 CLI envelope：顶层 `{ "ok": true, "result": SearchResponse }`。
- stdout 保持机器可读；warnings 仍走 stderr。
- 固定 HTML fixture 覆盖 parser，不依赖真实网络单测。
- 对 Bing/Baidu 显式 provider 做 provider 级请求间隔控制；只对网络错误和临时 502/503/504 做一次保守重试。
- 对 captcha、403、429 不重试，返回 typed `ErrRateLimited`，避免把可用性优化变成激进请求。
- `--provider auto` 默认链路保持 `searxng -> duckduckgo`，除非另开默认链路决策。
- `make check` 通过。

发布成功：

- README / README.zh-CN / CHANGELOG / skill 文档都只描述已验证能力。
- 主 skill 不推荐未验收 fallback。
- 默认 provider chain 是否变化有单独 release gate。

## 非目标

- 不把 Python fallback 脚本作为正式方案。
- 不新增与 `web-search --json` 不兼容的 JSON 输出。
- 不新增与正式 `SearchResult` 不兼容的 result item；结果项必须保留 `rank`、`title`、`url`、`snippet`、`source`、`engines` 等既有字段语义。
- 不在第一阶段把 Bing / Baidu 加入默认 chain。
- 不绕过 provider registry 直接在 skill 中调用外部脚本。
- 不做图片、新闻、视频、文件搜索垂类；第一阶段只做 general web search。
- 不承诺绕过搜索引擎反爬、验证码或地区限制。
- 不把 staged skill 实验变更当作已验收事实；它们必须先通过本计划的合约和测试门槛。

## 关键风险

| 风险 | Bing | Baidu | 对策 |
|------|------|-------|------|
| HTML 结构变化 | 中 | 高 | 固定 fixture + parser 容错 + live smoke |
| 反爬 / captcha | 中 | 中 | typed `ErrRateLimited` + 礼貌限速 + 不进默认 chain |
| 中国大陆可达性 | 待测 | 高概率可达 | 大陆直连 smoke gate |
| 国际网络可达性 | 高概率可达 | 待测 | 当前网络 + CI-like smoke |
| URL 重定向解析 | 中 | 高 | 单独 URL resolver 测试 |
| 广告/低质结果混入 | 中 | 高 | 过滤规则 + source 标记 + 质量评估 |
| 隐私/合规 | 中 | 中 | 默认不启用，文档明确外部请求 |

## 评估产物

新增评估记录，和实现代码分开提交：

```text
docs/search-provider-live-probe-YYYYMMDD.md
```

记录必须包含：

- 执行日期、执行人、git commit、二进制来源。
- 网络环境：当前开发网络、中国大陆直连网络、可选 CI/代理网络。
- 每个 provider/query 的 HTTP status、final URL、响应大小、耗时、是否出现 captcha/blocked/consent。
- parser spike 的结果数、前 5 条 domain、URL 是否可被 `web-reader` 读取。
- 失败样本的保存位置或脱敏摘要。
- 对 Baidu、Bing 分别给出 Go / No-Go，不使用笼统的“整体可用”结论。

评估记录不提交完整搜索结果页面，避免把第三方 HTML 大量固化到仓库；用于 parser 单测的最小 fixture 另放 `internal/search/testdata/`。

## 当前审查结论

截至本计划创建时，工作区曾存在其他 agent 留下的 skill / Python fallback 实验变更：

```text
.gitignore
skills/web-tools/SKILL.md
skills/web-tools/SKILL.md.bak
skills/web-tools/scripts/search-fallback.py
```

这些变更不能直接作为 Bing / Baidu provider 的实现基础，原因：

- `SKILL.md` 即使标注 experimental，仍会把 agent 引向 `scripts/search-fallback.py`，这会绕开正式 `web-search` provider registry。
- `search-fallback.py` 不是 Go provider，没有通过 parser fixture、provider chain、cache key、stderr/stdout 合约、release gate。
- Python 脚本如果声明兼容 `web-search --json`，必须输出顶层 `{ok,result}` envelope，且 `.result.results[]` 保持正式 `SearchResult` 字段语义；不能只返回部分字段。
- `provider_chain` 必须反映实际 provider 解析或 fallback 过程，不能只在成功后写一个最终 provider。
- `.bak` 文件不应进入发布提交。
- 当前 staged 与 unstaged 状态可能不同，必须在提交前同时检查 index 和 worktree。

处理结果与原则：

- Python fallback 脚本已从发布路径移除。
- 主 skill 不再推荐 `scripts/search-fallback.py`。
- Bing/Baidu 通过正式 Go provider 接入；Startpage 仍不纳入本实现。

## 架构方案

### Provider 注册

新增 builtin providers：

```go
"bing": {
    Type: "builtin",
    Capabilities: []string{"search"},
},
"baidu": {
    Type: "builtin",
    Capabilities: []string{"search"},
},
```

### Engine 实现

新增：

```text
internal/search/bing.go
internal/search/baidu.go
internal/search/bing_test.go
internal/search/baidu_test.go
```

实现接口：

```go
type Engine interface {
    Name() string
    HealthCheck() error
    Query(query string, opts SearchOptions) ([]RawResult, error)
}
```

### Chain 策略

第一阶段：

```text
default_provider_chain: ["searxng", "duckduckgo"]
```

用户显式使用：

```bash
web-tools web-search "人工智能 最新进展" --provider baidu --json
web-tools web-search "golang readability" --provider bing --json
```

后续如要加入默认 chain，需要单独评审：

```text
searxng -> duckduckgo -> baidu
searxng -> duckduckgo -> bing
searxng -> duckduckgo -> baidu -> bing
```

默认 chain 的选择不能只看“能返回结果”，还要看地区、语言、反爬、结果质量和延迟。

## 评估计划

### Phase 0：清理当前 skill 实验变更（已完成）

**文件**

- `skills/web-tools/SKILL.md`
- `skills/web-tools/SKILL.md.bak`
- `skills/web-tools/scripts/search-fallback.py`

**执行结果**

- 不合入 `.bak` 文件。
- 删除 `skills/web-tools/scripts/search-fallback.py`。
- 主 `SKILL.md` 不再推荐 Python fallback，也不再扩展 Python allowed-tools。
- 保留 reader quality 的准确修正。
- Bing/Baidu 只通过正式 Go provider 暴露。

**验证**

```bash
git diff --cached --name-status
git diff -- skills/web-tools/SKILL.md skills/web-tools/scripts/search-fallback.py
git status --short
```

验收：

- staged skill 变更不再声称 Bing/Baidu/Startpage 是 guaranteed fallback。
- 主 skill 只描述已发布 CLI 能力。
- Python fallback 脚本不在主 skill 路径。
- `git status --short` 中不能出现 `AD skills/web-tools/SKILL.md.bak` 或类似 staged/unstaged 冲突状态。

### Phase 1：真实网络可达性 smoke

**目标**

确认 Bing / Baidu 在不同网络下是否能稳定返回可解析 HTML，而不是 captcha、blocked、重定向陷阱或空壳页面。

**网络矩阵**

| 网络 | 必需 | 说明 |
|------|------|------|
| 当前开发网络 | 是 | 本机 smoke |
| 中国大陆直连网络 | 是 | 默认推荐国内用户前的硬门槛 |
| GitHub Actions / Ubuntu | 可选 | 用于判断 CI 是否能跑 live smoke |
| 代理/VPN 网络 | 可选 | 只作诊断，不作为默认用户假设 |

**Query 矩阵**

| 类别 | Query | 期望 |
|------|-------|------|
| 英文技术 | `golang readability` | GitHub / pkg docs 等高相关结果 |
| 中文通用 | `人工智能 最新进展` | 中文内容，Baidu 应表现较好 |
| 中文本地 | `北京 天气` | Baidu 可能有强结构化结果，需判断是否适合提取 |
| 域名意图 | `site:github.com go readability` | 检查是否尊重 query intent |
| 低频词 | `xyzzy_no_results_ever_12345` | 应返回空结果或明确失败，不误判为成功 |

**命令样例**

```bash
curl -I --max-time 10 "https://www.bing.com/search?q=golang+readability"
curl -I --max-time 10 "https://www.baidu.com/s?wd=golang+readability"
```

Go spike 后：

```bash
go run . web-search "golang readability" --provider bing --json --no-cache
go run . web-search "人工智能 最新进展" --provider baidu --json --no-cache
```

**记录字段**

- HTTP status
- final URL
- response bytes
- captcha/blocked/consent 信号
- parse result count
- first 5 result domains
- p50/p95 latency
- stderr warnings

**记录模板**

```markdown
| Provider | Network | Query | HTTP | Bytes | Parsed | First domains | Latency | Risk |
|----------|---------|-------|------|-------|--------|---------------|---------|------|
| baidu | cn-direct | 人工智能 最新进展 | 200 | 123456 | 8 | baidu.com, example.cn | 850ms | ok |
```

**Go / No-Go**

- Go：连续多次 smoke 有可解析结果，且没有 captcha/blocked。
- No-Go：频繁 blocked/captcha、结果 URL 无法还原、解析严重不稳定。
- Partial Go：仅允许保留为 explicit provider，不进入默认链路；文档必须写清楚适用网络和语言。

### Phase 2：Baidu provider spike

**文件**

- `internal/search/baidu.go`
- `internal/search/baidu_test.go`
- 可选：`internal/search/baidu_fixtures_test.go`

**实现**

- 请求 `https://www.baidu.com/s?wd=<query>&rn=<limit>`
- 设置桌面 UA、`Accept-Language: zh-CN,zh;q=0.9,en;q=0.8`
- 同一进程内 Baidu 请求间隔默认至少 3 秒。
- 网络错误或 502/503/504 最多重试一次。
- 解析标题、真实 URL、摘要、source。
- 对 captcha/blocked/异常空 body 返回 `ErrRateLimited`。
- 对广告或无真实 URL 的条目谨慎过滤。
- 不支持 `category` / `time_range` 时只在 fallback 场景 stderr warning，不污染 stdout。

**测试**

- fixture：普通中文结果页。
- fixture：英文 query 结果页。
- fixture：captcha/安全验证页。
- fixture：空结果或异常短 body。
- URL resolver：`mu=`、百度跳转、无真实 URL 场景。
- normalization：空标题、相对 URL、重复 URL、广告/推广块过滤。

**验证**

```bash
go test ./internal/search -run 'Baidu|SearchProvider'
go test ./cmd/web-search ./internal/config ./internal/provider ./internal/search
go run . web-search "人工智能 最新进展" --provider baidu --json --no-cache
go run . web-search "golang readability" --provider baidu --json --no-cache
```

### Phase 3：Bing provider spike

**文件**

- `internal/search/bing.go`
- `internal/search/bing_test.go`

**实现**

- 请求 `https://www.bing.com/search?q=<query>&count=<limit>`
- 根据 `opts.Locale` 设置 `cc` / `setlang` / `Accept-Language`。
- 同一进程内 Bing 请求间隔默认至少 2 秒。
- 网络错误或 502/503/504 最多重试一次。
- 解析 `li.b_algo` 结构中的 title、URL、snippet。
- 处理 Bing redirect URL。
- 对 captcha/consent/blocked/异常空 body 返回 typed error。

**测试**

- fixture：普通英文结果页。
- fixture：中文 query 结果页。
- fixture：redirect URL。
- fixture：captcha/consent 页。
- fixture：空结果。
- normalization：空标题、重复 URL、Bing redirect、异常短 snippet。

**验证**

```bash
go test ./internal/search -run 'Bing|SearchProvider'
go test ./cmd/web-search ./internal/config ./internal/provider ./internal/search
go run . web-search "golang readability" --provider bing --json --no-cache
go run . web-search "人工智能 最新进展" --provider bing --json --no-cache
```

### Phase 4：Provider registry 集成

**文件**

- `internal/config/config.go`
- `internal/config/loader_test.go`
- `internal/search/search.go`
- `internal/search/search_test.go`
- `cmd/web-search/main.go`
- `cmd/web-search/main_test.go`
- `cmd/doctor/main.go`，如需展示 provider 状态

**实现**

- 注册 builtin provider：`bing`、`baidu`。
- `NewSearch` / `NewSearchWithConfig` 注入新 engine。
- `enginesForProviders` 能解析 `bing` / `baidu`。
- CLI help 文案加入 provider；文档明确它们是 explicit provider，不进入默认 auto chain。
- 默认 provider chain 暂不改变。

**测试**

- `--provider baidu` 只调用 Baidu engine。
- `--provider bing` 只调用 Bing engine。
- `--provider auto` 默认仍只走 `searxng -> duckduckgo`。
- 自定义 config chain `["baidu", "duckduckgo"]` 能按顺序 fallback。
- 未知 provider、capability mismatch 保持现有 structured error。
- cache key 区分 `--provider baidu`、`--provider bing`、`--provider auto`，避免串结果。
- warnings 仍写 stderr，`--json` stdout 可被 `jq` 解析。

**验证**

```bash
go test ./internal/config ./internal/provider ./internal/search ./cmd/web-search
```

### Phase 5：质量评估

**目标**

判断 Bing/Baidu 结果是否足以让 agent 使用，而不是只看“有结果”。

**评估指标**

| 指标 | 门槛 |
|------|------|
| JSON 合约 | 与 `web-search --json` 完全一致 |
| 结果数 | 常见 query 至少 3 条可用结果 |
| URL 可读性 | `web-reader` 能读取至少 60% 的前 5 条 |
| 相关性 | 前 5 条中至少 3 条与 query 明显相关 |
| 广告污染 | 广告/推广不进入前 3，或可过滤 |
| 中文质量 | Baidu 中文 query 明显优于 DDG 时才推荐 |
| 英文质量 | Bing 英文 query 至少不劣于 DDG 的备用场景 |
| 稳定性 | 连续 10 次 smoke 无 captcha/blocked |

**验证脚本建议**

新增：

```text
scripts/search_provider_smoke.sh
```

覆盖：

```bash
web-tools web-search "golang readability" --provider bing --json --no-cache
web-tools web-search "人工智能 最新进展" --provider baidu --json --no-cache
web-tools web-search "xyzzy_no_results_ever_12345" --provider baidu --json --no-cache
```

脚本只做 smoke，不放入默认 `make check`，避免 CI 因实时网络波动失败。

**机器可读合约检查**

每个 live smoke 都必须通过：

```bash
go run . web-search "人工智能 最新进展" --provider baidu --json --no-cache \
  | jq -e '.ok == true
    and .result.query
    and .result.engine
    and .result.provider
    and (.result.results | type == "array")
    and all(.result.results[]; .rank and .title and .url and .source and (.engines | type == "array"))'

go run . web-search "golang readability" --provider bing --json --no-cache \
  | jq -e '.ok == true
    and .result.query
    and .result.engine
    and .result.provider
    and (.result.results | type == "array")
    and all(.result.results[]; .rank and .title and .url and .source and (.engines | type == "array"))'
```

如命令失败，记录 stderr 和 exit code；不允许把失败包装成空数组后视为成功。

低频词 / 无结果 query 的 contract 检查另做：

```bash
go run . web-search "xyzzy_no_results_ever_12345" --provider baidu --json --no-cache \
  | jq -e '.ok == true and .result.total == 0 and (.result.results | type == "array")'
```

这类 query 允许空结果，但不允许因为 captcha、blocked、解析失败而伪装成正常空结果。

### Phase 6：文档与 skill 更新

**文件**

- `README.md`
- `README.zh-CN.md`
- `CHANGELOG.md`
- `skills/web-tools/SKILL.md`
- `docs/provider-architecture.md`
- 本文档

**更新原则**

- 只写已经实现并验证的 provider。
- 明确 `bing` / `baidu` 是 explicit provider。
- 不写“保证 fallback”。
- 不推荐 Python 脚本替代正式 CLI。
- 国内用户建议优先尝试 `--provider baidu`，但不承诺默认自动选择。

**示例**

```bash
web-tools web-search "人工智能 最新进展" --provider baidu --json
web-tools web-search "golang readability" --provider bing --json
```

### Phase 7：发布 gate

发布前必须通过：

```bash
git status --short
git diff --cached --name-status
make check
go test ./...
go vet ./...
./scripts/smoke.sh
./scripts/upgrade_smoke.sh
./scripts/metrics_smoke.sh
git diff --check
```

如果新增 live smoke 脚本：

```bash
./scripts/search_provider_smoke.sh --provider baidu
./scripts/search_provider_smoke.sh --provider bing
```

发布后必须验证：

```bash
web-tools upgrade --version vX.Y.Z --json
web-tools web-search "人工智能 最新进展" --provider baidu --json --no-cache
web-tools web-search "golang readability" --provider bing --json --no-cache
```

## 默认链路决策 gate

Bing/Baidu 实现后，默认链路仍保持：

```text
searxng -> duckduckgo
```

要改变默认链路，必须另开决策，并满足：

- 中国大陆直连 smoke 通过。
- 国际网络 smoke 通过。
- 连续多日 smoke 无明显 blocked/captcha。
- 平均延迟可接受。
- 结果质量优于或补足 DDG。
- 用户隐私说明已更新。
- skill 不会误导 agent 以为 fallback 一定成功。

推荐默认链路候选要单独评审，不在 provider 首次实现中一起合并。

## 里程碑

| 里程碑 | 产物 | 决策 |
|--------|------|------|
| M1 | 本评估计划 | 是否开始 live probe |
| M2 | Bing/Baidu live probe 记录 | 是否进入 Go spike |
| M3 | Baidu explicit provider | 是否保留为实验/正式 provider |
| M4 | Bing explicit provider | 是否保留为实验/正式 provider |
| M5 | 文档和 skill 更新 | 是否发布 |
| M6 | 多网络 smoke 数据 | 是否讨论默认链路 |

## Go / No-Go 总表

| 决策点 | Go | No-Go |
|--------|----|-------|
| Baidu provider | 大陆中文 smoke 稳定，URL 可还原，fixture 可维护 | captcha/blocked 多、广告污染严重、真实 URL 无法稳定解析 |
| Bing provider | 英文 smoke 稳定，redirect 可还原，fixture 可维护 | consent/blocked 多、HTML 结构不稳定 |
| 加入默认 chain | 多网络多日稳定，质量补足 DDG | 任何地区高概率失败或输出质量不可控 |
| 更新 skill 推荐 | CLI 已发布且通过验收 | 仍是脚本/spike/实验能力 |

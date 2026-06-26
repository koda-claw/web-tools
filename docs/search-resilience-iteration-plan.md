# Search Resilience 迭代方案

## 背景

GitHub 当前有两个待处理 issue：

- `#2 feat: DuckDuckGo rate-limit resilience — backoff retry + multi-engine fallback chain`
- `#3 feat: search result caching + rate-limit awareness in CLI`

这两个问题本质上都指向同一个风险：Agent research loop 会在短时间内重复调用 `web-search`，而默认零依赖链路最终依赖 DuckDuckGo Lite。DuckDuckGo Lite 一旦触发限流、验证码或空响应，当前 CLI 很难区分“真实无结果”和“被搜索后端拦截”，Agent 也缺少可执行的降级信号。

`#2` 在 2026-06-26 的评论中补充了 fallback 候选实测结论：

| Engine | 结论 | 原因 |
|--------|------|------|
| Brave Search | 不采用 | 程序化访问返回 captcha，且页面偏 JS-heavy SPA |
| Mojeek | 不采用 | 多网络下出现 SSL EOF 和 HTTP 502 |
| Startpage | 仅保留调研候选 | 评论中的 `urllib` 原型可解析，但当前本机 `curl -I https://www.startpage.com` 返回 302 到 blocked 页面，不能直接进入默认链路 |

当前主线已有基础能力：

- `web-search --provider auto` / `--engine auto` 已有 provider chain 与 engine fallback。
- 默认 provider chain 是 `searxng -> duckduckgo`。
- metrics 已能记录 `web-search` 的 provider、结果数、错误类别等本地聚合信息。
- `web-reader` 已有磁盘缓存，但 `web-search` 还没有搜索结果缓存。

因此本轮重点不是重做搜索架构，而是在现有 Provider / Plugin 架构内增强稳定性。

## 优化判断

真正值得做的优化有两个：

- DuckDuckGo 限流识别和 typed error。当前代码能识别 captcha，但仍把 HTTP 429、403、异常空响应等情况当作普通 engine error 或正常空结果处理，Agent 无法区分“无结果”和“被限制”。
- 搜索缓存。当前 `Search` 每次 `Do()` 都会真实请求后端；GUI 和库调用场景会复用进程，进程内短 TTL 缓存能减少重复 query。CLI 单次进程收益较小，但实现成本低，且不持久化 query，隐私边界清楚。

不应作为本轮核心优化的点：

- “重做 fallback chain”。当前 `Search.Do()` 已经有 auto/provider chain fallback，issue 正文对此描述过时。本轮只需要补齐 rate-limit 语义和 warning，不需要重写调度。
- 强制 throttle。CLI 每次通常是新进程，进程内 `lastSearchAt` 对普通 CLI 调用帮助有限；可以只做 warning，或者先不做。
- Startpage 默认 fallback。评论里的实测有价值，但本机探针已出现 blocked 页面；即使继续调研，也必须加入中国大陆网络 smoke gate，不能直接放进默认链路。

## 目标

- 降低重复 query 对 DuckDuckGo Lite 的压力。
- 将 DuckDuckGo 限流、验证码、空异常响应从普通 engine error 中区分出来。
- 在 auto/provider chain 模式下，让限流类错误能触发后续 provider fallback。
- 给 CLI 调用者明确的 stderr warning，stdout JSON/Markdown 仍保持机器可读。
- 评估 DuckDuckGo 后是否需要增加零依赖 fallback 候选；Startpage 目前只保留为调研对象，不作为默认方案。
- 保持新 fallback engine 的接入方式符合现有 Provider / Plugin 架构，不重新发明一条调度链路。

## 非目标

- 不改变现有 `SearchResponse` 的核心 JSON 字段语义。
- 不要求用户必须配置付费搜索 provider。
- 不默认调用远程 API key provider。
- 不接入 Brave Search 和 Mojeek 作为本轮 fallback engine；评论中的实测结论已经排除它们。
- 不把搜索缓存做成跨进程数据库或长期索引。
- 不让 warnings 混入 stdout。

## 设计决策

### 1. 先做进程内搜索缓存

`Search` 增加轻量内存缓存，默认 TTL 使用现有 `config.DefaultCacheTTL`，即 300 秒。

缓存 key 应覆盖会影响搜索结果的参数：

- `query`
- `locale`
- `category`
- `time_range`
- `provider/engine` 解析后的模式
- `include_domain`
- `exclude_domain`
- `limit`

说明：issue #3 原文建议排除 `time-range`，但这里建议纳入 key。原因是不同时间窗口语义上应允许返回不同结果，缓存不应把 `day` 和 `year` 混用。

缓存行为：

- 命中且未过期：直接返回缓存响应副本，不发 HTTP 请求。
- 过期或 miss：执行正常搜索，成功后写入缓存。
- 返回错误：不写缓存。
- 空结果：可以缓存，但仅缓存明确成功的空结果；限流、验证码、解析异常不缓存。
- `--no-cache`：绕过读取和写入缓存。

### 2. DuckDuckGo 限流识别使用 typed error

新增稳定 sentinel，例如：

```go
var ErrRateLimited = errors.New("search engine rate limited")
```

DuckDuckGo engine 在以下场景返回可被 `errors.Is(err, ErrRateLimited)` 识别的错误：

- HTTP `429`
- HTTP `403` 且 body 命中验证码/anti-bot 文案
- body 为空或异常短，并且不是可识别的正常无结果页
- HTML 命中 captcha、unusual traffic、please enable javascript 等 anti-bot 模式

错误仍映射到现有 structured error 体系，建议 category 继续使用 `engine`，context 中包含：

- `engine=duckduckgo`
- `reason=rate_limited|captcha|empty_body|blocked`
- `status_code`
- `retry_after`，如果响应头存在

### 3. DuckDuckGo engine 内部做短重试

DuckDuckGo `Query()` 对限流类错误做最多 3 次尝试：

```text
attempt 1 -> wait 2s -> attempt 2 -> wait 4s -> attempt 3
```

约束：

- 只对限流/临时网络错误重试。
- 不对输入错误、HTML 解析结构错误、正常空结果重试。
- 如果响应有 `Retry-After`，优先使用较小的安全等待值，并设置上限，避免 CLI 卡太久。
- 重试 warning 写 stderr，JSON stdout 不受影响。
- 单次请求保留现有 10s timeout。

### 4. Search auto/provider chain 识别限流并 fallback

当前 `Search.Do()` 已能在 auto 模式下遇到 engine query error 后继续尝试后续 candidate。本轮需要补齐语义：

- 如果错误是 `ErrRateLimited`，stderr 明确写：当前 provider 被限流，正在尝试下一个 provider。
- 如果还有后续 provider，继续 fallback。
- 如果所有 provider 都限流或失败，返回最后一个 structured error，并在 attempted/suggestions 中说明 fallback 链路。
- 显式 `--provider duckduckgo` 或 `--engine duckduckgo` 不隐式切其他 provider，只做 DuckDuckGo 内部 retry 后返回错误。

### 5. Startpage 只作为调研项，默认链路不依赖它

根据 `#2` 评论，Brave 和 Mojeek 当前不适合作为 server-side scraping fallback；Startpage 曾被评论验证为可解析。但当前本机探针显示 `https://www.startpage.com` 会 302 到 blocked 页面，因此不能把它当作已成立的 fallback engine。

如果后续继续验证，最多作为候选 fallback chain：

```text
searxng -> duckduckgo -> startpage
```

接入边界：

- 只有 smoke 和 fixture 验证通过后，才实现 `StartpageEngine`。
- 默认只在 `auto` / provider chain 中作为 DuckDuckGo 后备，不改变显式 `--engine duckduckgo` 行为。
- 是否向 `config.DefaultConfig().Providers` 增加 `startpage` builtin provider，由验证结果决定。
- 默认链路是否加入 `startpage`，由实现前真实 smoke 决定。
- Startpage parser 使用 `golang.org/x/net/html`，不新增抓取解析依赖。
- 解析逻辑必须有固定 HTML fixture 测试，不能依赖实时网络作为单元测试。

实现前检查：

- 找到评论中提到的 `search-fallback.py` 原型，提取 URL、headers、CSS class / DOM pattern 线索。
- 用 Go 的 `net/http` + 当前 User-Agent 策略做一次真实 smoke，确认不会立刻触发 captcha。
- 增加中国大陆网络 smoke，至少覆盖常见直连网络；如果大陆直连不可用，不进入默认 chain。
- 记录 Startpage 结果中 URL、title、snippet、source 的映射规则。
- 明确 Startpage 不支持或不稳定支持的 options，例如 `category`、`time_range`、`locale`。

### 6. 搜索间隔 awareness 后置

本轮不实现强制 throttle，也不新增 `--no-throttle`。原因是 CLI 每次执行通常是新进程，进程内 `lastSearchAt` 对普通 CLI 调用帮助有限。后续如果 GUI 或 daemon 场景证明需要，再做 warning：

- `Search` 记录当前进程内 `lastSearchAt`。
- 当同一个 `Search` 实例内两次真实请求间隔 `<2s` 时，stderr 输出 warning。
- 缓存命中不触发 warning，因为没有真实搜索请求。
- 如需关闭 warning，再增加对应开关。

说明：CLI 每次执行通常是新进程，进程内 `lastSearchAt` 对单次 CLI 的价值有限；但对 GUI、测试、未来 daemon 或库调用有价值。跨进程 throttle 需要本地状态文件，暂不做，避免引入锁和隐私边界问题。

## 实施计划

### Phase 1：文档和验收基线

**文件**

- `docs/search-resilience-iteration-plan.md`
- 可选更新：`docs/provider-architecture.md`
- 可选更新：`README.md` / `README.zh-CN.md` 的 Search 限制说明

**验收**

- 方案覆盖 #2/#3。
- 明确哪些需求本轮做，哪些不做。
- 明确测试命令和发布前 smoke。

### Phase 2：搜索缓存

**文件**

- `internal/search/search.go`
- `internal/search/search_test.go`
- `cmd/web-search/main.go`
- `cmd/web-search/main_test.go`

**实现**

- `Search` 增加 cache map、TTL、mutex 或最小同步保护。
- `SearchOptions` 增加 `NoCache bool`。
- `web-search` 增加 `--no-cache`。
- cache hit 返回响应副本，避免调用方修改缓存对象。

**测试**

- 同 query/options 第二次调用不触发 engine。
- TTL 过期后重新触发 engine。
- `--no-cache` 透传到 `SearchOptions`。
- 不同 `time-range`、domain filter、limit 不共用缓存。

### Phase 3：DuckDuckGo 限流 typed error + retry

**文件**

- `internal/search/engine.go`
- `internal/search/duckduckgo.go`
- `internal/search/duckduckgo_test.go`

**实现**

- 增加 `ErrRateLimited` 和 rate-limit wrapper error。
- DuckDuckGo response 检测 HTTP 429、captcha、blocked、异常空 body。
- 对限流类错误做 2s/4s 的短 backoff retry。
- 保持正常空结果不报错。

**测试**

- HTTP 429 最终返回 `errors.Is(err, ErrRateLimited) == true`。
- captcha body 返回 typed rate-limit error。
- 第一次 429、第二次正常时返回结果。
- 正常 no-results fixture 仍返回空结果和 nil error。

### Phase 4：fallback 语义

**文件**

- `internal/search/search.go`
- `internal/search/search_test.go`

**实现**

- auto/provider chain 对 `ErrRateLimited` 输出明确 stderr warning。
- 显式 provider/engine 不跨 provider fallback。

**测试**

- auto 模式下第一个 engine rate-limited，第二个 engine 成功。
- 显式 DuckDuckGo rate-limited 时不尝试 SearXNG。
- warning 只写 stderr，JSON stdout 可解析。

### Phase 5：Startpage / 国内可用 fallback 调研

**文件**

- `internal/search/startpage.go`
- `internal/search/startpage_test.go`
- `internal/search/search.go`
- `internal/search/search_test.go`
- `internal/config/config.go`
- `internal/config/loader_test.go`
- `cmd/doctor/main.go`，如果 doctor 需要展示 builtin provider 状态

**实现**

- 先用小范围 Go spike 验证 Startpage 是否能稳定返回 server-rendered HTML，而不是 blocked/captcha 页面。
- 增加中国大陆网络 smoke；这是默认链路的硬门槛。
- 验证通过后，再考虑新增 `StartpageEngine`，实现 `Name() / HealthCheck() / Query()`。
- 只有验证通过，才将 Startpage 注册为 builtin provider 并支持 provider chain。
- 默认 chain 是否从 `searxng -> duckduckgo` 扩展为 `searxng -> duckduckgo -> startpage`，需要单独确认；当前不建议。
- 对 Startpage 不支持的 `category`、`time_range` 等 options 保持 stderr warning，不污染 stdout。

**测试**

- HTML fixture 能解析 title、URL、snippet、source。
- Startpage redirect 或 tracking URL 能还原真实 URL，若页面存在这类包装。
- auto 模式下 DuckDuckGo rate-limited 后能 fallback 到 Startpage。
- 显式 `--provider startpage` 只调用 Startpage。
- 默认 provider chain 的 config loader 测试与文档一致。

**验收**

- 真实 smoke 至少验证一次：

```bash
go run . web-search "golang readability" --provider startpage --json
go run . web-search "golang readability" --provider auto --json
```

- 如果 Startpage 在当前网络或中国大陆直连下触发 captcha/blocked 或结构变化，本 Phase 不进入代码实现，只保留调研结论或延后。

### Phase 6：docs/skill 同步

**文件**

- `README.md`
- `README.zh-CN.md`
- `skills/web-tools/SKILL.md`，如果本仓库包含该 skill 目录
- `CHANGELOG.md`

**更新内容**

- `web-search --no-cache`
- DuckDuckGo 限流行为和建议间隔。
- provider chain fallback 的实际语义。
- Startpage 或其他零依赖 fallback 的调研状态、国内可用性限制和默认链路决策。
- 缓存隐私边界：进程内缓存，不持久化 query。

## 验证清单

实现完成后至少运行：

```bash
go test ./internal/search ./cmd/web-search
go test ./...
./scripts/smoke.sh
git diff --check
```

如果改动影响 metrics 或 GUI 搜索测试，再追加：

```bash
./scripts/metrics_smoke.sh
go test ./internal/gui ./internal/metrics
```

## 发布门槛

- 所有新增 flag 都有 help、单测和 README 说明。
- `web-search --json` 在 warning 场景下 stdout 仍是合法 JSON。
- metrics 不记录 search query、URL、title、snippet 或详细错误字符串。
- 缓存不持久化 query。
- provider chain 的 fallback 行为在测试中覆盖。
- 任何新增零依赖 fallback 进入默认 chain 前必须有真实 smoke 记录，并包含中国大陆网络 gate；否则只能作为显式 provider、实验项或延后。
- `CHANGELOG.md` 记录用户可见行为变化。

## 风险与后续

- DuckDuckGo HTML 和 anti-bot 页面会变化，检测逻辑要保持宽松但不能把正常无结果误判为限流。
- Startpage 当前已出现 blocked 探针结果，默认链路不应依赖它；如果继续调研，必须用 fixture + 多网络 smoke 双重验证。
- 进程内缓存不能解决独立 CLI 进程之间的重复 query；如果后续 Agent 仍频繁触发限流，再评估短 TTL 磁盘缓存。
- Brave/Mojeek 暂不进入本轮；如果未来重新评估，需要先更新 issue 证据和本方案。
- 远程付费 provider 可以通过现有 provider chain 接入，但默认链路不应偷偷消耗 API key。

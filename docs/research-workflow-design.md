# 研究工作流设计

## 目的

本文评估 web-tools 在 Agent research 场景下应如何组织“先搜索、再读取”的组合工作流，以及是否需要新增一个内置命令，例如未来可能加入的 `web-tools web-research`。

目标是提升 Agent 的研究效率，同时不牺牲现有 `web-search` 与 `web-reader` 命令的可组合性、可调试性和透明度。当前重点不是设计一个 CLI 内置 summarizer，而是定义 Agent 可以通过 skill 稳定执行的 research workflow。

## 建议

暂时不要实现新命令。

当前默认研究工作流应使用现有命令与 skill 文档组合完成：

1. 执行 `web-tools web-search <query> --json`。
2. 从结构化搜索结果中选择要读取的 URL。
3. 对每个选中的 URL 执行 `web-tools web-reader <url> --json`。
4. 根据 `quality` metadata 判断是否需要 `--browser` 重试。
5. 由调用方 Agent 负责综合、引用或总结收集到的材料，并保留来源 URL 与失败项。

这样可以保持工具本身小而稳定、易测试、易排查。也能让 Agent 自行决定读取哪些来源、读取多少来源、是否跳过低质量域名，以及何时使用 `--browser` 重试。

## 为什么现在不实现

- 当前两个命令已经可以通过 JSON 清晰组合，适合作为 Agent research 的稳定原语。
- 组合命令需要内置来源选择策略、重试策略、引用策略、去重策略和部分失败策略。
- 这些策略通常依赖具体 Agent 和具体任务。过早固化到 CLI 中，会降低工具行为的可预测性。
- `web-reader` 的浏览器 fallback 与质量评分仍在演进。组合命令不应隐藏这些信号。

## Agent research 工作流

Skill 应指导 Agent 按以下顺序执行：

1. 必要时运行 `web-tools doctor --json` 检查配置、缓存目录和可选依赖。
2. 使用 `web-tools web-search "<query>" --json` 获取结构化候选来源。
3. 根据标题、摘要、域名、排名和任务需求选择 URL；必要时使用 `--include-domain` 或 `--exclude-domain` 收窄来源。
4. 对选中 URL 使用 `web-tools web-reader "<url>" --json` 读取内容。
5. 检查 `quality.score`、`quality.needs_fallback`、`quality.word_count` 和 stderr 警告；只有低质量或疑似 JS 渲染页面才使用 `--browser` 重试。
6. 综合结果时保留来源 URL，不隐藏失败项，不把读取失败误写成来源没有相关信息。

这个流程把判断权留给 Agent，CLI 只负责提供可验证的搜索、读取、质量和错误信号。

## 可参考的 Agent 实现思路

Agent research 的实现可以参考以下原则：

1. 先判断任务是否必须联网。涉及最新信息、价格、版本、政策、产品规格、新闻、官方文档或高风险事实时，应默认搜索验证。
2. 搜索阶段只产生候选来源，不直接下结论。优先选择官方文档、源码仓库、标准文件、论文、发布公告等一手来源。
3. 来源选择要显式。根据标题、摘要、域名、发布时间和用户目标筛选 URL；必要时用 `--include-domain` 和 `--exclude-domain` 把约束写进命令。
4. 阅读阶段读取正文，不只依赖搜索摘要。对选中的 URL 使用 `web-reader --json`，并检查标题、正文、metadata 和 `quality`。
5. 质量不足时才升级。只有 `quality.needs_fallback=true`、字数过少、正文明显缺失或页面疑似 JS 渲染时，才使用 `--browser` 重试。
6. 综合阶段保留证据链。回答中引用实际读取过的 URL；读取失败、来源冲突或证据不足时要说明，不把失败结果改写成确定结论。
7. 对高风险或强时效任务做交叉验证。至少读取多个独立来源，且优先使用更接近事实源头的材料。

这个思路和 CLI 边界一致：`web-search` 负责候选发现，`web-reader` 负责内容提取，Agent 负责判断、取舍、交叉验证和表达。

## 当前支持的工作流

### 基础研究

```bash
web-tools web-search "golang readability library" --limit 5 --json
web-tools web-reader "https://github.com/go-shiori/go-readability" --json
```

### 带过滤的研究

```bash
web-tools web-search "golang readability library" \
  --include-domain github.com \
  --exclude-domain reddit.com \
  --limit 5 \
  --json
```

### 读取重试

```bash
web-tools web-reader "https://example.com/article" --json
web-tools web-reader "https://example.com/article" --browser --json
```

只有当首次读取返回低质量 metadata，或 stderr 提示内容提取稀疏时，才建议使用浏览器重试。

## 未来命令设想

如果后续确认需要实现，可新增：

```bash
web-tools web-research <query> [flags]
```

### 建议参数

| 参数 | 默认值 | 含义 |
|------|--------|------|
| `--limit` | `5` | 搜索阶段检查的结果数量 |
| `--read-limit` | `3` | 从搜索结果中选中并读取的 URL 数量 |
| `--engine` | 配置默认值 | 搜索引擎策略 |
| `--locale` | 配置默认值 | 搜索 locale |
| `--include-domain` | 空 | 只保留匹配域名 |
| `--exclude-domain` | 空 | 排除匹配域名 |
| `--browser` | `false` | 强制使用浏览器渲染读取 |
| `--retry-browser` | `false` | 对低质量读取结果使用浏览器 fallback 重试 |
| `--json` | `false` | 输出结构化 JSON |
| `--output` | stdout | 输出文件 |

### 建议 JSON 结构

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

命令应保留底层搜索与读取结果，而不是只返回一个合成摘要。除非另有设计明确批准，否则总结能力应留在 web-tools 之外。

## 未来命令的非目标

- 不做内容总结或观点排序。
- 不伪造引用。
- 不隐藏单个搜索或读取错误。
- 不强依赖在线 SearXNG 或浏览器依赖。
- 不替代 `web-search` 或 `web-reader`。

## 批准门槛

只有至少满足以下条件之一，才考虑实现 `web-research`：

- 多个用户或 Agent 反复需要相同的“搜索后读取”样板流程。
- 单靠技能文档无法稳定产生可靠工作流。
- 期望的来源选择策略已经足够稳定，可以编码进工具。
- 下游 Agent 会直接消费一个明确的 JSON 合约。

实现前必须先确认：

- URL 选择策略。
- 部分失败行为。
- 浏览器重试行为。
- JSON schema。
- Markdown 输出应包含完整读取内容，还是只包含链接与 metadata。

## 获批后的实现计划

### 任务 A：新增工作流类型

**文件：** `internal/research/*`

- 定义搜索/读取编排结构。
- 摘要能力保持在范围外。
- 保留可兼容 `SearchResponse` 与 `PipelineResult` 的嵌入数据。

**验证**

- 为部分成功与部分失败增加单元测试。

### 任务 B：新增命令

**文件：** `cmd/web-research/*`、`main.go`

- 新增 Cobra 命令和参数。
- 复用现有配置加载逻辑。
- 保持 stdout 可被机器消费。

**验证**

- 使用 fixture search 和 reader 函数覆盖命令测试。

### 任务 C：补充文档

**文件：** `README.md`、`skills/web-tools/SKILL.md`

- 说明何时使用 `web-research`，何时使用显式命令组合。

**验证**

- help 输出与文档保持一致。

## 获批后的测试策略

必须保持离线可测：

- 搜索 fixture 返回重复 URL 和可过滤 URL。
- 读取 fixture 返回高质量、低质量和失败结果。
- 部分失败时仍返回成功读取结果与结构化错误。
- `--retry-browser` 只对低质量读取结果触发 reader 重试。
- JSON 输出保持稳定。
- Markdown 输出不隐藏来源 URL。

## 当前决策

Task 9 已通过本文完成设计审查。当前批准路径是只提供文档和显式命令组合，不实现 `web-research`。在上面的批准门槛被满足并确认前，不应实现该命令。

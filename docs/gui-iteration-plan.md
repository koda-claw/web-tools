# Web Tools GUI 版本化迭代计划

## 背景

`web-tools` 已经具备 CLI、skill、provider、env file、doctor 的基础闭环，但可用性仍依赖用户或 Agent 理解多条命令：

- `web-tools setup`
- `web-tools config provider add/list`
- `web-tools doctor --json`
- `web-tools web-search`
- `web-tools web-reader`

从可用性角度，GUI 的目标不是替代 CLI，而是提供一个本地可视化控制台，让人类用户可以确认配置状态、管理 provider、写入 env file、执行 smoke 测试，并为 Agent 场景生成明确的安装和排障指引。

## 总体原则

- GUI 只监听 `127.0.0.1`，默认不暴露局域网端口。
- GUI 复用现有 CLI/core 逻辑，不另起一套配置系统。
- GUI 不展示、不记录、不回传 secret 明文。
- `config.json` 仍只保存 `auth_env`，真实 token 只进入当前 shell env 或 `~/.config/web-tools/.env`。
- 默认不自动加载当前目录 `.env`。
- 远程 provider 涉及隐私和成本，search/reader auto 开关必须可见、可控。
- 第一版优先本地单用户场景，不做账号、远程管理、多租户。

## 推荐架构

### 形态

新增命令：

```bash
web-tools gui
web-tools gui --host 127.0.0.1 --port 0
web-tools gui --no-open
```

默认行为：

- 监听 `127.0.0.1`。
- `--port 0` 自动选择空闲端口。
- 默认打开浏览器；Agent 或 CI 可用 `--no-open`。
- 终端输出本地 URL。

### 后端

建议新增：

```text
cmd/gui/main.go
internal/gui/server.go
internal/gui/handlers.go
internal/gui/assets/
```

后端接口建议：

```text
GET  /api/status
POST /api/setup/provider
POST /api/env
POST /api/test/search
POST /api/test/reader
GET  /api/agent-guide
```

所有接口返回稳定 JSON，错误复用现有 structured error 风格。

### 前端

MVP 先使用 Go `embed` 内嵌静态 HTML/CSS/JS，避免引入 Node 构建链。

建议页面：

- Dashboard：版本、doctor、skill、provider、env file 状态。
- Providers：BigModel 配置、auth 状态、search/reader auto 开关。
- Env：写入或更新 `ZHIPU_APIKEY`，默认 mask，不回显明文。
- Interactive Setup：用表单完成 skill、provider、env file 和 reader fallback 配置。
- Test：搜索测试、网页读取测试。
- Agent Guide：安装命令、skill 安装命令、推荐检查命令、当前状态摘要。

## v1.5.0 统一范围

本轮不再拆多个 tag。`setup --check`、repair suggestions、GUI MVP、诊断导出和 Agent Guide 都归入 `v1.5.0`，按内部里程碑顺序推进，全部验收通过后统一发布。人类交互式配置由 GUI 承担，CLI `setup --interactive` 暂不进入本轮。

当前实现状态：

- Milestone 1 已实现：`setup --check`、`--json`、repair suggestions、setupcheck 单元测试和 smoke 覆盖。
- Milestone 2 已实现：`web-tools gui`、本地 server、`/healthz`、status/provider/env/test API、静态 GUI 页面。
- Milestone 3 已实现：诊断导出、Agent Guide、reader auto 显式确认提示和动态建议。
- 发布前仍需执行统一验收：`go test ./...`、`go vet ./...`、`./scripts/smoke.sh`、`git diff --check`，并完成本地 GUI 冒烟。

### Milestone 1: Setup Check / Repair API

目标：先把 GUI 需要的状态诊断抽成可复用能力，CLI 和 GUI 共用。

#### Task 1: `setup --check --json`

**文件**

- `cmd/setup/main.go`
- `cmd/setup/main_test.go`
- 可选：`internal/setupcheck/check.go`

**实现**

- 新增：

```bash
web-tools setup --check
web-tools setup --check --json
```

- 检查：
  - CLI version。
  - skill 是否存在。
  - provider 是否配置。
  - env file 是否存在、是否加载、权限是否安全。
  - BigModel `auth_configured`。
  - search/reader provider chain。

**验收标准**

- 人类输出能看出“缺什么”。
- JSON 输出稳定，GUI 可以直接消费。
- 不输出 secret value。

**测试用例**

- 单元测试：无 skill、无 provider、无 env file 时给出缺失项。
- 单元测试：BigModel 已配置但未认证时给出 repair 建议。
- 单元测试：env file 权限过宽时给 warn。
- 集成测试：临时 HOME + 临时 skill dir 下 `setup --check --json` 可解析。

#### Task 2: Repair Suggestions

**文件**

- `cmd/setup/main.go`
- `cmd/setup/main_test.go`
- 可选：`internal/setupcheck/check.go`

**实现**

输出可复制命令：

```text
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --set-env ZHIPU_APIKEY=<redacted>
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --enable-reader-auto
web-tools skill install --force
```

**验收标准**

- 建议命令不包含真实 token。
- 建议与当前状态匹配，不给无关建议。
- Agent 可以根据 JSON suggestions 继续执行非交互修复。

**测试用例**

- 单元测试：未安装 skill 时建议 `skill install --force`。
- 单元测试：BigModel 未配置时建议 provider setup。
- 单元测试：reader auto 未启用但 BigModel 可用时建议 `--enable-reader-auto`。

### Milestone 2: GUI MVP

目标：提供本地 GUI，可查看状态、配置 BigModel、写 env file、执行基础测试。

#### Task 3: `web-tools gui` 命令与本地 server

**文件**

- `main.go`
- `cmd/gui/main.go`
- `internal/gui/server.go`
- `internal/gui/server_test.go`

**实现**

- 新增 `web-tools gui`。
- 支持 `--host`、`--port`、`--no-open`。
- 默认 host 为 `127.0.0.1`。
- `--port 0` 自动选端口。
- 提供 `/healthz` 和静态首页。

**验收标准**

- 命令启动后输出 URL。
- `/healthz` 返回 `{"ok":true}`。
- 默认不监听 `0.0.0.0`。

**测试用例**

- 单元测试：server 可以绑定 `127.0.0.1:0`。
- 集成测试：HTTP GET `/healthz` 成功。
- 安全测试：默认 host 是 `127.0.0.1`。

#### Task 4: Status API

**文件**

- `internal/gui/handlers.go`
- `internal/gui/handlers_test.go`
- 复用 `doctor` / `setup --check` 的状态结构。

**实现**

```text
GET /api/status
```

返回：

- version。
- doctor report。
- setup check report。
- provider summary。
- env file summary。
- skill summary。

**验收标准**

- JSON 不包含 secret value。
- BigModel `auth_configured`、reader/search chain 可见。
- GUI 可用单个接口渲染 Dashboard。

**测试用例**

- 单元测试：status JSON 不包含 env value。
- 单元测试：缺 provider / 缺 env file 状态可表达。

#### Task 5: Provider / Env 管理 API

**文件**

- `internal/gui/handlers.go`
- `internal/gui/handlers_test.go`

**实现**

```text
POST /api/setup/provider
POST /api/env
```

能力：

- 配置 BigModel provider。
- 切换 `enable_search_auto`。
- 切换 `enable_reader_auto`。
- 写入 env file key。
- 覆盖已有 key 前需要显式 `force=true`。

**验收标准**

- 不返回 token 明文。
- `config.json` 只写 `auth_env`。
- env file 权限为 `0600`。
- 重复写 key 默认失败，`force=true` 才覆盖。

**测试用例**

- 单元测试：provider setup 写 config。
- 单元测试：env write 不泄漏 token。
- 单元测试：force=false 拒绝覆盖。
- 集成测试：写 env 后 status 显示 `auth_configured=true`。

#### Task 6: Test API

**文件**

- `internal/gui/handlers.go`
- `internal/gui/handlers_test.go`

**实现**

```text
POST /api/test/search
POST /api/test/reader
```

请求字段：

- query/url。
- provider。
- limit / timeout。

**验收标准**

- 返回 CLI 同构 JSON。
- 失败时返回 structured error。
- 默认超时可控。
- 不自动把测试内容写入日志。

**测试用例**

- 单元测试：参数校验。
- 集成测试：使用 builtin/mock 路径跑通 search/reader。
- 集成测试：provider auth 缺失时返回可诊断错误。

#### Task 7: GUI 静态页面

**文件**

- `internal/gui/assets/index.html`
- `internal/gui/assets/app.js`
- `internal/gui/assets/styles.css`

**实现**

- Dashboard：
  - Version。
  - Doctor status。
  - Env file status。
  - Provider status。
  - Skill status。
- Provider 管理：
  - BigModel 配置按钮。
  - Reader auto / Search auto 开关。
- Env 管理：
  - token 输入框。
  - 保存按钮。
  - overwrite checkbox。
- Test：
  - Search query。
  - Reader URL。
  - provider selector。
- Agent Guide：
  - 当前推荐命令。

**验收标准**

- 首屏就是可操作控制台，不做 landing page。
- 不显示 secret 明文。
- 所有按钮有 loading/error/success 状态。
- 移动端和桌面端不重叠。

**测试用例**

- Playwright 或浏览器自动化：打开页面，Dashboard 正常渲染。
- Playwright：保存 fake token 后 status 更新。
- Playwright：切换 reader auto 后 provider chain 更新。
- 视觉检查：desktop/mobile 截图无明显重叠。

#### Task 7.1: Test Result Rendering / 快捷 Reader / 刷新恢复

目标：让 GUI 的 Search Test / Reader Test 从 raw JSON 调试输出升级为可操作结果视图，并形成 `search -> read` 的闭环。

**实现**

- Search Test 成功后，在 Search Test 卡片内渲染结果列表：
  - rank。
  - title。
  - URL。
  - snippet。
  - source / engine / provider。
  - 每条结果提供快捷 `Read` 按钮。
- 点击搜索结果的 `Read`：
  - 自动填充 Reader Test 的 URL。
  - 保持当前 Reader provider。
  - 自动触发 Reader Test。
  - Reader 结果在 Reader Test 卡片内渲染。
- Reader Test 成功后，在 Reader Test 卡片内渲染：
  - title。
  - source URL。
  - word count。
  - content type。
  - extract mode。
  - content preview。
  - copy content。
- Output / Diagnostics 继续保留 raw JSON 或错误信息，作为排障入口。
- 暂不引入 Modal。完整内容查看、Raw JSON Modal、历史记录等后续单独迭代。

**刷新恢复策略**

- 使用 `localStorage` 恢复非敏感 UI 状态：
  - language。
  - search query / provider。
  - reader URL / provider。
  - 最近一次 search 结果摘要。
  - 最近一次 reader 结果元数据和正文预览。
- 不恢复、不保存：
  - env token value。
  - provider auth secret。
  - 完整 raw JSON。
  - 完整 reader content。
- 提供 `Clear State` 操作，用于清除 GUI 本地保存的测试状态。

**验收标准**

- Search 成功后卡片内显示结果列表。
- 点击结果 `Read` 后自动触发 Reader Test。
- Reader 成功后显示标题、source、word count、extract mode、正文预览。
- Search / Reader 失败时，在对应卡片内显示错误摘要。
- 刷新页面后恢复测试输入和最近一次非敏感结果预览。
- Env token 输入刷新后必须为空。
- 中英文 UI 都覆盖。
- 桌面/移动端无横向溢出。

### Milestone 3: GUI 可用性增强

目标：让 GUI 更适合日常排障和 Agent handoff。

#### Task 8: 诊断导出

**实现**

- 导出非敏感诊断 JSON。
- 包含 version、doctor、setup check、provider chain、env file metadata。
- 不包含 token。

**验收标准**

- 可直接贴给 Agent 或 issue。
- secret 扫描通过。

#### Task 9: Agent Guide 深化

**实现**

- 显示仓库地址。
- 显示 release 安装命令。
- 显示 skill 安装命令。
- 显示当前推荐 provider 使用方式。
- 根据当前状态动态显示 repair commands。

**验收标准**

- 复制命令可执行。
- 不包含 token。

#### Task 10: Reader Auto 推荐策略

**实现**

- 如果 BigModel 已配置且认证成功，但 reader chain 未包含 BigModel，GUI 提示可开启 reader fallback。
- search fallback 只作为可选，不默认推荐。

**验收标准**

- 提示解释隐私和成本。
- 用户必须显式确认。

## 发布门槛

`v1.5.0` 统一发布前必须完成全部 Milestone，并通过以下验收：

- `go test ./...`
- `go vet ./...`
- `./scripts/smoke.sh`
- `git diff --check`
- GUI server 集成测试。
- 浏览器截图验收：desktop + mobile。
- 临时 HOME 验收：

```bash
HOME=<tmp> web-tools gui --no-open --port 0
curl /api/status
curl /api/setup/provider
curl /api/env
```

- release binary 冷启动验收：
  - `web-tools gui --no-open --port 0`
  - `/api/status` 正常。
  - GUI 能安装/检查 skill。

## 风险与控制

| 风险 | 控制 |
|------|------|
| GUI 泄漏 token | API、日志、页面都只显示 env key 和 configured boolean；测试扫描 fake token。 |
| 本地服务被外部访问 | 默认只监听 `127.0.0.1`；监听非 localhost 需要显式参数和 warning。 |
| GUI 与 CLI 配置逻辑分叉 | GUI handler 只调用现有 config/setup/doctor 逻辑或共享 internal service。 |
| GUI 引入前端构建复杂度 | MVP 用 Go embed + 原生 HTML/JS/CSS。 |
| Reader auto 远程 fallback 带来隐私/成本 | 默认不开启；GUI 提示并要求显式确认。 |
| Playwright 测试增加维护成本 | MVP 只做关键 happy path 和截图检查，不做全面 E2E。 |

## 建议执行顺序

1. Milestone 1：先做 `setup --check --json` 和 repair suggestions。
2. Milestone 2：做 `web-tools gui` server、status API、provider/env API、基础页面。
3. Milestone 3：补诊断导出、Agent Guide 深化、reader auto 推荐策略。
4. 全部验收通过后统一准备 `v1.5.0` release notes、tag 和 release binary 冷启动验收。

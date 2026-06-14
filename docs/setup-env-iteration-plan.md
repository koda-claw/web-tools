# Setup 与 Env File 版本化迭代计划

## 背景

当前 provider 配置已经可以通过 CLI 完成：

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
web-tools config provider add bigmodel --preset bigmodel --auth-env ZHIPU_APIKEY
```

但认证值仍依赖当前进程环境变量。也就是说，如果用户只把 API key 写到：

```text
~/.config/web-tools/.env
```

当前 `web-tools web-search ... --provider bigmodel` 不会自动读取它，`doctor --json` 也会显示 `auth_configured=false`。

这会影响两个场景：

- 人类用户：希望配置一次后，之后直接运行 CLI。
- Agent 场景：skill 知道 provider id 和 `auth_env`，但如果 env file 不自动加载，Agent 仍需要外部 shell 先 export。

因此下一阶段目标是把 setup/config/env/skill 的闭环补齐，同时保持 secret 安全。

## 总体原则

- `config.json` 只保存 `auth_env`，不保存真实 API key。
- API key 可以保存到 env file，但必须以 `.env` 形式隔离，并默认 `0600` 权限。
- 默认自动加载用户级 env file：`~/.config/web-tools/.env`。
- 默认不自动加载当前工作目录 `.env`，避免在任意项目目录误读项目 secret。
- 当前进程环境变量优先级最高，覆盖 env file。
- 日志、错误、doctor 输出永不打印 secret 值。
- Agent 默认使用非交互命令；交互式引导只给人类首次配置使用。

## 目标状态

人类用户首次配置：

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --env-file ~/.config/web-tools/.env
# 按提示或后续命令把 key 写入 env file
web-tools web-search "Go readability library" --provider bigmodel --json
```

Agent 使用：

```bash
web-tools doctor --json
web-tools web-search "Go readability library" --provider bigmodel --json
web-tools web-reader "https://github.com/go-shiori/go-readability" --provider bigmodel --json
```

如果 `doctor --json` 看到 `auth_configured=false`，skill 应指导用户把 key 放入当前 shell 环境或 `~/.config/web-tools/.env`。

## Env 加载策略

加载顺序：

1. 默认配置。
2. 用户配置：`~/.config/web-tools/config.json`。
3. 本地配置：`./web-tools.json`。
4. 用户 env file：`~/.config/web-tools/.env`。
5. 显式 env file：`WEB_TOOLS_ENV=/path/to/.env`。
6. 当前进程环境变量。
7. `WEB_TOOLS_CONFIG` 指向的配置文件覆盖配置项，但不覆盖 env file 中已加载的 secret；显式 shell env 仍最高优先级。

说明：

- env file 中的变量只在当前 `web-tools` 进程内生效。
- 如果当前进程已经存在同名环境变量，env file 不覆盖它。
- env file 支持简单格式：

```env
ZHIPU_APIKEY=...
SEARXNG_URL=http://localhost:8888
```

暂不支持复杂 shell 语法，例如命令替换、变量展开、多行值。

## 版本路线

### v1.4.2: Env File 自动加载与 setup 非交互增强

目标：解决“配置放到 `~/.config/web-tools/.env` 后，CLI 能直接使用”的核心问题。

#### Task 19: 用户级 env file 自动加载

**文件**

- `internal/config/envfile.go`
- `internal/config/loader.go`
- `internal/config/*_test.go`

**实现**

- 新增 env file parser。
- 默认读取 `~/.config/web-tools/.env`。
- 支持 `WEB_TOOLS_ENV=/path/to/.env` 显式指定。
- 当前 shell 环境变量优先于 env file。
- 解析失败返回 structured config warning 或 error 的策略需要固定：
  - 文件不存在：忽略。
  - 行格式错误：返回 config error，避免 silently misconfigured。
  - 权限过宽：先 warn，不阻断；后续可加 `doctor` 检查。

**验收标准**

- 用户把 `ZHIPU_APIKEY=...` 写入 `~/.config/web-tools/.env` 后，`doctor --json` 显示 `auth_configured=true`。
- `web-tools web-search ... --provider bigmodel --json` 能读取 env file 中的 key。
- shell 中已有 `ZHIPU_APIKEY` 时，shell 值优先。
- 不打印 key 值。

**测试用例**

- 单元测试：解析 `KEY=value`、带引号 value、空行、注释。
- 单元测试：env file 不覆盖已有 env。
- 单元测试：格式错误返回明确错误。
- 集成测试：临时 `HOME` 下的 `.config/web-tools/.env` 被加载。
- 集成测试：`WEB_TOOLS_ENV` 指向临时文件时被加载。

#### Task 20: setup 支持 env file 写入

**文件**

- `cmd/setup/main.go`
- `cmd/setup/main_test.go`
- `cmd/config/*` 可选

**实现**

新增参数：

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --env-file ~/.config/web-tools/.env
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --set-env ZHIPU_APIKEY=...
```

建议默认：

- `--env-file` 默认 `~/.config/web-tools/.env`。
- `--set-env` 显式传入时才写 key。
- 不通过普通日志打印 `--set-env` 的值。
- 写入文件权限 `0600`。
- 如果 env file 已有同名 key，默认拒绝覆盖；需要 `--force-env`。

**验收标准**

- 用户可以用非交互命令把 key 写入 env file。
- key 不进入 `config.json`。
- `doctor --json` 能看到 `auth_configured=true`。
- 重复执行不会重复写多行同名 key。

**测试用例**

- 单元测试：新增 key。
- 单元测试：已有 key 且无 `--force-env` 时失败。
- 单元测试：`--force-env` 覆盖。
- 集成测试：setup 写 env file 后 web-search/web-reader 可用 mock provider auth。

#### Task 21: doctor 展示 env file 诊断

**文件**

- `cmd/doctor/main.go`
- `cmd/doctor/main_test.go`

**实现**

`doctor --json` 增加非敏感字段：

```json
{
  "name": "env_file",
  "status": "ok",
  "details": {
    "path": "~/.config/web-tools/.env",
    "loaded": "true"
  }
}
```

provider summary 仍只显示：

- `auth_env`
- `auth_configured`
- `enabled_if_env`
- `enabled`

**验收标准**

- 可以看出 env file 是否存在、是否已加载。
- 不展示任何 env value。
- 权限过宽时给 warn。

#### v1.4.2 发布门槛

- `go test ./...`
- `go vet ./...`
- `./scripts/smoke.sh`
- `git diff --check`
- 临时 HOME 验收：

```bash
HOME=<tmp> web-tools setup --provider bigmodel --set-env ZHIPU_APIKEY=fake --skip-doctor
HOME=<tmp> web-tools doctor --json
```

- 真实 BigModel 可选验收：

```bash
WEB_TOOLS_ENV=~/.config/web-tools/.env web-tools web-search "Go readability library" --provider bigmodel --json
```

### v1.4.3: Setup Check / Repair 建议

目标：让用户和 Agent 更容易知道“缺什么、下一步做什么”。

#### Task 22: setup --check

**实现**

```bash
web-tools setup --check
web-tools setup --check --json
```

检查：

- CLI 是否可运行。
- skill 是否已安装。
- `config.json` 是否存在。
- `~/.config/web-tools/.env` 是否存在。
- provider 是否配置。
- provider 对应 env 是否已配置。
- `doctor` 是否有 hard error。

**验收标准**

- 输出明确下一步建议。
- JSON 输出可被 Agent 消费。
- 不自动修改文件。

#### Task 23: setup repair 建议但不自动修

第一阶段只输出命令建议，例如：

```text
Run: web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
Run: web-tools setup --provider bigmodel --set-env ZHIPU_APIKEY=<redacted>
```

后续如果要自动修复，再加 `--apply`。

### v1.5.0: 人类交互式 Setup

目标：为人类本地首次配置提供安全交互式体验。

#### Task 24: setup --interactive

**交互流程**

1. 是否安装/更新 Agent skill。
2. 选择 skill 目录：
   - `~/.codex/skills`
   - `~/.agents/skills`
   - 自定义
3. 是否配置 provider。
4. 选择 provider：
   - BigModel
   - 暂不配置
5. 选择 env 名，默认 `ZHIPU_APIKEY`。
6. 检查 env 是否存在。
7. 如果不存在，选择：
   - 只输出 export 命令。
   - 写入 `~/.config/web-tools/.env`。
   - 跳过，稍后手动设置。
8. 是否加入 search auto chain。
9. 是否启用 reader auto chain。
   - 默认否，并提示隐私/成本风险。
10. 跑 doctor。
11. 输出测试命令。

**安全约束**

- 读取 API key 时不 echo。
- 输出日志永远 mask。
- 默认不写 shell profile。
- 写入 env file 前明确确认。
- 不把 key 写进 `config.json`。

**Agent 约束**

Skill 不应默认使用 `--interactive`。Agent 默认走非交互命令：

```bash
web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
```

只有用户明确说“帮我交互式配置”时才使用 `--interactive`。

## 文档同步要求

每个版本都要同步：

- `README.md`
- `README.zh-CN.md`
- `skills/web-tools/SKILL.md`
- `CHANGELOG.md`
- `docs/iteration-plan.md`

Skill 文档保持英文；方案和计划文档保持中文。

## 风险与决策

| 风险 | 决策 |
|------|------|
| 自动加载当前目录 `.env` 可能误读项目 secret | 默认不加载当前目录 `.env`。 |
| env file 格式支持过复杂会变成 shell 解析器 | 只支持简单 dotenv 格式。 |
| key 写入日志或错误 | 所有输出只显示 env 名，不显示 value。 |
| Agent 使用 interactive 卡住 | Skill 默认指导非交互命令。 |
| reader 远程 provider 涉及隐私/成本 | reader auto chain 默认不启用，interactive 中额外确认。 |

## 推荐执行顺序

1. v1.4.2：Task 19-21，先解决 env file 自动加载和非交互写入。
2. v1.4.3：Task 22-23，补 check/repair 建议。
3. v1.5.0：Task 24，做人类交互式 setup。

这个顺序优先解决当前真实痛点，同时避免过早把交互式流程做复杂。

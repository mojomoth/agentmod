# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

为编码代理提供按项目隔离和移交。

`agentmod` 将 **Claude Code**、**Codex CLI** 和 **OpenCode** 的配置、技能、插件、会话、缓存和工作上下文保存在您正在处理的项目内部 — 并将该环境打包为一个快照，您可以将其移交给另一台机器。

它有两个作用：

1. **代理主目录路由器。** 在包含 `.agentmod/agentmod.toml` 的目录树内，shell 钩子将每个代理的主目录路由到 `.agentmod/`。在外部，每个变量都完全恢复到原始状态，您的全局设置不受影响。
2. **移交工具。** `agentmod handoff create` 将 `.agentmod/` 打包为可验证的 `.amod` 快照（或者，使用 `--for-git`，在 `.agentmod-handoff/` 下打包为可提交的文件树）。**Git 移动您的源代码；agentmod 移动代理环境。**

## agentmod 不是什么

- **不是 Docker 沙箱。** 它在您自己的 shell 中路由环境变量。没有容器、没有虚拟机、没有系统调用过滤。
- **不是完全的安全隔离。** 忽略路由变量的工具仍然可以访问您的全局主目录。Claude Bash 防护（下文）是深度防御，不是安全边界。
- **不是垫片。** 它从不拦截或包装 `claude`、`codex` 或 `opencode` 命令。您可以继续直接运行它们，无需修改。
- **不是 HOME 更改工具。** `HOME` 从不被重新分配。
- **不是源代码备份工具。** 快照默认不包含您的源代码。请使用 git 进行源代码管理。

## 它是如何工作的

`agentmod hook zsh` / `agentmod hook bash` 打印一个小的自包含 shell 函数（由 `agentmod init` 安装到您的 rc 文件中）。在每个提示符和目录更改时，它向上查找 `.agentmod/agentmod.toml`：

- **进入项目** 时保存当前值并设置：

  | 变量 | 路由到 |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **仅当** `opencode.xdg_full_isolation = true` |

  `PATH` 获得恰好一个条目 `.agentmod/node/bin`（npm 的全局 bin 在路由前缀下）。记账变量（`AGENTMOD_ACTIVE`、`AGENTMOD_PROJECT_ROOT`、`AGENTMOD_ROOT`、`AGENTMOD_VARS`、`AGENTMOD_SAVED_*`）记录要撤销的内容。

- **离开项目** 时恢复每个保存的值并删除 `PATH` 条目 — 完美的逆操作。直接在两个 agentmod 项目之间切换会在一个步骤中重新路由，而不会泄露任何项目的路径。

每个代理的路由可以在 `agentmod.toml` 中关闭（`claude.enabled`、`codex.enabled`、`opencode.enabled`、`node.enabled`）。

## 安装

选择适合您设置的任何一个 — 每个都安装相同的单个二进制文件：

```sh
# npm（为您的平台安装预构建的二进制文件）
npm install -g agentmod

# 安装脚本（下载匹配的发布版本，验证 sha256）
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install（需要 Go 工具链）
go install github.com/mojomoth/agentmod@latest
```

或从源代码构建（Go 1.26+，唯一的模块依赖是 `BurntSushi/toml`）：

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# 将二进制文件放在您的 PATH 上的某个位置
```

## 快速开始

```sh
cd ~/work/myproject
agentmod init          # 创建 .agentmod/，编辑 .gitignore，安装
                       # shell 钩子到您的 rc 文件，提供复制身份验证
# 仅第一次：此 shell 中的钩子还未活跃 —
# 打开一个新终端，或：exec $SHELL

cd ~/work/myproject    # 钩子激活；检查它：
agentmod status        # "AgentMod: active"，列出路由的路径
claude                 # 普通命令 — 现在使用项目本地主目录
agentmod install gstack   # 项目本地技能，全局主目录不受影响

agentmod pack          # 快照到 .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # 随时进行只读诊断
```

在接收机器上：

```sh
cd ~/work/myproject    # 源代码通过 git 到达
agentmod init
agentmod unpack myproject-20260611-123045.amod
# 按照打印的重新登录说明操作；doctor 自动运行
```

## `agentmod init`

幂等 — 重新运行会填入缺少的任何内容，从不覆盖现有的 `agentmod.toml` 或任何用户文件。它：

- 创建 `.agentmod/{claude,codex,opencode,node,snapshots,logs}` 和默认的 `agentmod.toml`；
- 在 `.agentmod/claude/settings.json` 中安装 Claude Bash 防护；
- 将 `.agentmod/` 添加到 `.gitignore`（仅在 git 存储库内创建）；
- 安装 shell 钩子为 `~/.zshrc` 或 `~/.bashrc` 中的一个围栏块（您的 shell 来自 `$SHELL`；块会原地更新，永远不会重复，您自己的 rc 内容从不被触及）；
- 提议 **复制** 现有的 Claude/Codex 身份验证文件到项目本地主目录（见下文"身份验证"）— 复制仅在显式 `y` 时发生。

标志：`--no-shell-hook` 跳过所有 rc 文件编辑；`--yes` / `--non-interactive` 永不提示，因此永不复制身份验证（用于 CI）。

## 使用普通 `claude`、`codex`、`opencode`

没有包装器命令。在活跃项目内，普通命令只需看到路由的主目录：

- **Claude Code** 读取 `CLAUDE_CONFIG_DIR` → 项目本地设置、用户级技能/插件、会话、历史。（项目级 `.claude/` *总是* 本地读取 — 见限制。）
- **Codex CLI** 读取 `CODEX_HOME` → 项目本地 `config.toml`、`auth.json`、会话、历史、日志。
- **OpenCode** 读取 `OPENCODE_CONFIG` → 项目本地配置文件。这是 *部分* 隔离（默认） — 见限制。

### 身份验证

全新的项目本地主目录没有凭证：

- **Claude on macOS**：无需执行任何操作 — 凭证保存在钥匙串中，与每个配置目录共享（这也意味着它们 *不* 按项目隔离）。
- **Claude on Linux/Windows**：在项目内运行 `claude login`，或接受 init 的复制 `~/.claude/.credentials.json` 的提议。
- **Codex**：在项目内运行 `codex login`，或接受 init 的复制 `~/.codex/auth.json` 的提议。

身份验证文件 **永远不会在快照中传送**（无论如何到达那里，都按名称排除）。

## gstack 安装

[gstack](https://github.com/garrytan/gstack) 将其安装程序硬编码到 `~/.claude/skills/gstack` — 这正是 agentmod 存在要防止的全局污染。所以：

```sh
agentmod install gstack            # 克隆到 .agentmod/claude/skills/gstack
agentmod install gstack --force    # 替换现有的项目本地安装
```

安装程序用 git 克隆，从不运行 gstack 自己的设置脚本，并对 `~/.claude/skills` 的列表进行快照（之前和之后） — 任何对全局目录的更改都被报告为违规并导致命令失败。`agentmod doctor` 单独警告何时存在 *全局* gstack 安装（即使是您在采用 agentmod 之前自己安装的），因为全局安装的技能泄漏到每个项目中。

## 移交（`.amod` 快照）

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # 清单 + 编辑报告，无提取
agentmod handoff verify  FILE      # 重新哈希每个成员；不匹配时退出 3
agentmod handoff restore FILE      # 替换 .agentmod/（进行备份）
agentmod pack / agentmod unpack    # create / restore 的别名
```

快照是一个 zip，有六个根成员 — `manifest.json`、`inventory.json`（每个文件的大小/sha256/模式）、`REDACTION.md`（被排除内容及原因，加上秘密扫描发现）、`HANDOFF.md` 和 `RESTORE.md`（接收方的人工说明）、`checksums.txt`（`shasum -a 256 -c` 兼容）— 以及有效负载在 `payload/.agentmod/…` 下。创建是原子的和确定性的；清单记录 git 分支/提交/脏状态，且删除了远程 URL 中的任何凭证。脏工作树拒绝打包，除非 `--allow-dirty`。

`inspect` 和 `verify` 在任何地方都有效 — 接收方可以在拥有任何项目设置之前审计快照。

### 秘密排除策略

两层，都默认启用：

1. **排除规则** 将已知敏感文件从有效负载中删除，并在 `REDACTION.md` 中列出每一个：身份验证文件（按名称）（`.credentials.json`、`auth.json`、`credentials*`）、`*.env` / `.env.*`、SSH 密钥（`id_*`、`*.pem`、`*.pub`）、凭证目录（`.ssh`、`.aws`、`.azure`、`.gcloud`、`.kube`、`.gnupg`、`.docker`）、钥匙串文件、`.git`、`node_modules`、缓存和临时目录。
2. **内容扫描** 对每个 *保留* 文件。私钥物质拒绝创建，除非您传递 `--allow-findings`（并在 `REDACTION.md` 中标记为 HARD）。可能的令牌（AWS 访问密钥 ID、GitHub 令牌、`sk-…` 密钥、`api_key=` 样式的分配）被警告但不阻止。

扫描是启发式的。**在共享快照之前查看 `REDACTION.md`（或 `handoff inspect`）** — 会话和工作上下文按设计传送，可能引用您粘贴到代理对话中的任何内容。快照以 0600 模式写入，原因是这样；像对待私有文件一样对待它们。

## Git 移交

```sh
agentmod pack --for-git    # 在项目根目录写入 .agentmod-handoff/
git add .agentmod-handoff && git commit
```

与 `.amod` 相同的六个成员和有效负载，但作为普通文件的可提交树（`shasum -a 256 -c checksums.txt` 在目录中有效）。除了默认排除之外，它还删除 **会话、记录、历史和日志** 对于所有三个代理 — 这些通常包含粘贴的秘密，不属于存储库。`--include-sessions` 总是拒绝：提交会话需要加密，此版本不实现。安全共享的工作上下文（CLAUDE.md、代理配置、技能、计划）留下。

重新运行替换上一个包；repo 中的任何其他内容都不会被触及。

## 恢复注意事项

`handoff restore` / `unpack` 将每个快照视为不受信任的输入：

- 首先进行完整的校验和验证和库存交叉检查；
- 路径安全计划：zip-slip（`..`）、绝对路径、驱动器字母、非 `.agentmod` 目标、受保护的名称（`.git`、`.ssh`、`.aws`、`.docker`）以及转义或绝对符号链接目标都在写入任何东西之前被拒绝；
- 现有的 `.agentmod/` 在提取之前被重命名为 `.agentmod.backup-<stamp>`；任何失败都会自动回滚到它；
- **快照中的任何内容永远不会被执行**；
- 之后：Claude 防护钩子被重新接线到 *此* 机器的二进制文件，恢复的代理配置中发现的特定于机器的绝对路径被警告（您的文件永远不会被重写），`doctor` 内联运行，并打印所需的重新登录步骤（身份验证从不传送）。

恢复拒绝而不是猜测 — 拒绝的恢复使项目字节相同。

## `agentmod doctor`

只读诊断，随时安全运行（干净时退出 0，有发现时退出 3）：项目/配置/布局状态、shell 钩子安装和活跃性、路由漂移、项目外的悬挂变量、重复的 PATH 条目、HOME/垫片违规、按代理身份验证存在与重新登录说明、OpenCode 泄漏警告、gstack 全局/项目状态、Claude 防护接线、恢复配置中的可移植性风险、现有快照中记录的秘密候选、`.agentmod-handoff/` 内的会话/日志物料，以及存储库的 HEAD 是否仍与最新快照匹配。

## Claude Bash 防护

`agentmod init` 将 `agentmod guard claude-bash` 注册为 Claude Code 项目本地主目录中的 PreToolUse 钩子。它阻止会写入全局代理主目录（`~/.claude`、`~/.codex`、`~/.config/opencode`、`~/.local/share/opencode`）的 Bash 命令、使用 `sudo` 或重新分配 `HOME` — 代理获得原因并可以调整。读取从不被阻止。它是一个 shell 解析启发式深度：有用的护栏，不是沙箱。

## 已知限制

诚实部分。这些是底层工具的属性或有意的 MVP 范围 — `doctor` 和生成的文档也说明了它们。

- **macOS 钥匙串（Claude）。** Claude Code on macOS 在钥匙串中存储 OAuth 凭证，在 *所有* 配置目录中共享。按项目的帐户隔离在 macOS 上是不可能的 — 每个项目都不需要重新登录。Linux/Windows 使用按主目录的 `.credentials.json`，它隔离但需要每个项目登录/复制。
- **OpenCode 默认部分隔离。** OpenCode 没有单个主目录变量；其配置是一个合并链，仍然读取全局 `~/.config/opencode/opencode.json`，会话/存储/身份验证位于全局 XDG 数据目录中。`opencode.xdg_full_isolation = true` 路由 XDG 变量以实现完全隔离 — 但这会影响 *每个* 您在项目内运行的感知 XDG 工具。`doctor` 报告这两种情况。
- **项目 `.claude/` 是本地 Claude 行为。** Claude Code 总是读取 `./.claude/` 无论 `CLAUDE_CONFIG_DIR`。agentmod 为 Claude 增加的价值是隔离 *用户级* 状态（全局技能/插件、会话、历史）；项目 `.claude/` 在 agentmod 之前已经工作。
- **首次会话钩子激活。** 在 `agentmod init` 之后，已在运行的 shell 还没有加载新的 rc 块。打开新终端、`exec $SHELL` 或一次性 `eval "$(agentmod hook zsh)"`（init 打印精确的这个）。同样，bash 钩子通过 `PROMPT_COMMAND` 触发，因此在非交互式 bash 脚本中是惯性的（与 direnv 相同的限制类） — 脚本应该通过 `eval "$(agentmod env --shell bash --activate <root>)"` 显式设置变量，如果它们需要路由。
- **只有 npm 的全局 bin 在 PATH 上。** `.agentmod/node/bin` 是单个受管 PATH 条目。pnpm/bun 全局安装被路由到项目（`PNPM_HOME`、`BUN_INSTALL`）但它们的 bin 目录不被添加到 PATH。
- **树包手动恢复。** `handoff restore` 仅接受 `.amod` 文件；一个提交的 `.agentmod-handoff/` 目录通过按照里面的 `RESTORE.md` 恢复（此版本没有目录读取器）。
- **快照可能需要恢复后修复。** gstack 克隆在没有其 `.git` 的情况下传送（重新运行 `agentmod install gstack --force` 以使其再次可更新），`node/bin` 启动器符号链接悬挂，因为 `node_modules` 被排除（重新运行 `npm install -g …` 在项目内）。
- **Shell 支持是 zsh 和 bash。** 其他 shell 仍然可以手动使用 `agentmod env`。

## 常见问题

**我继续直接使用 `claude` / `codex` / `opencode` 吗？**
是的。这就是重点 — 无包装器、无垫片、无 `agentmod run`。

**为什么 agentmod 不只是改变 `HOME`？**
重新分配 `HOME` 会破坏 SSH、git、钥匙串、dotfiles 和 shell 中的所有其他工具。agentmod 仅路由代理特定的变量。

**为什么我的身份验证在恢复后丢失？**
按设计 — 凭证从不在快照中传送。在新机器上按照打印的重新登录行（或 init 的复制提议）。

**我可以将 `.agentmod/` 提交到 git 吗？**
否 — init gitignores 它（会话、缓存和可能复制的身份验证位于那里）。改为提交安全子集：`agentmod pack --for-git`。

**这与 direnv 有何不同？**
相同的激活模型（目录范围的 env、基于提示符的钩子、完美的退出恢复），但 agentmod 还知道 *什么* 为每个代理路由、创建主目录、防守对全局写入，并执行移交。两者相处良好。

**快照创建失败，显示"秘密候选发现"。**
内容扫描在保留的文件中发现了私钥物质。删除它（或将其移到被排除的位置，例如 `.env`），或用 `--allow-findings` 打包（如果您接受它在快照中）。

**它在 Windows 上工作吗？**
Go 代码构建，路径安全对 Windows 样式的路径强制执行，但 shell 钩子针对 zsh/bash；Windows 在此版本中未测试。

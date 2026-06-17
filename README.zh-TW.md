# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

為編碼代理提供每個項目的隔離和交接。

`agentmod` 將 **Claude Code**、**Codex CLI** 和 **OpenCode** 的配置、技能、插件、會話、緩存和工作上下文保留在您正在處理的項目內部，並將該環境打包成可以交給另一台機器的快照。

它的作用有兩個：

1. **代理家目錄路由器。** 在包含 `.agentmod/agentmod.toml` 的目錄樹內，shell hook 將每個代理的家目錄路由到 `.agentmod/` 中。在此目錄之外，每個變數都完全恢復到原樣，您的全局設置不受影響。
2. **交接工具。** `agentmod handoff create` 將 `.agentmod/` 打包成可驗證的 `.amod` 快照（或者使用 `--for-git`，打包成 `.agentmod-handoff/` 下的可提交文件樹）。**Git 移動您的源代碼；agentmod 移動代理環境。**

## agentmod *不是*什麼

- **不是 Docker 沙箱。** 它在您自己的 shell 中路由環境變數。沒有容器、沒有虛擬機、沒有系統調用過濾。
- **不是完整的安全隔離。** 忽略路由變數的工具仍然可以到達您的全局家目錄。Claude Bash 防護（下面提到）是深度防禦，而不是安全邊界。
- **不是 shim。** 它永遠不會攔截或包裝 `claude`、`codex` 或 `opencode` 命令。您可以直接運行它們，不做任何修改。
- **不是更改 HOME 的工具。** `HOME` 永遠不會被重新分配。
- **不是源代碼備份工具。** 快照默認從不包含您的源代碼。使用 git 進行源代碼管理。

## 工作原理

`agentmod hook zsh` / `agentmod hook bash` 打印一個小的自包含 shell 函數（由 `agentmod init` 安裝到您的 rc 文件中）。在每個提示和目錄更改時，它向上走查尋找 `.agentmod/agentmod.toml`：

- **進入項目**保存當前值並設置：

  | 變數 | 路由到 |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **僅限** `opencode.xdg_full_isolation = true` |

  `PATH` 獲得正好一個條目，`.agentmod/node/bin`（npm 的全局 bin，在路由的前綴下）。記錄變數（`AGENTMOD_ACTIVE`、`AGENTMOD_PROJECT_ROOT`、`AGENTMOD_ROOT`、`AGENTMOD_VARS`、`AGENTMOD_SAVED_*`）記錄要撤銷的內容。

- **離開項目**恢復每個保存的值並移除 `PATH` 條目 — 完美的反向操作。直接在兩個 agentmod 項目之間切換會一步完成重新路由，不會洩露任何一個項目的路徑。

按代理的路由可以在 `agentmod.toml` 中關閉（`claude.enabled`、`codex.enabled`、`opencode.enabled`、`node.enabled`）。

## 安裝

選擇適合您設置的任何一個 — 每個都安裝相同的單個二進制文件：

```sh
# npm（為您的平台安裝預構建的二進制文件）
npm install -g agentmod

# 安裝腳本（下載匹配的版本，驗證 sha256）
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install（需要 Go 工具鏈）
go install github.com/mojomoth/agentmod@latest
```

或從源代碼構建（Go 1.26+，唯一的模塊依賴是 `BurntSushi/toml`）：

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# 將二進制文件放在 PATH 上的某個地方
```

## 快速開始

```sh
cd ~/work/myproject
agentmod init          # 創建 .agentmod/，編輯 .gitignore，將
                       # shell hook 安裝到您的 rc 文件中，提供複製身份驗證
# 僅第一次：此 shell 中的 hook 還沒有活動 —
# 打開新終端，或：exec $SHELL

cd ~/work/myproject    # hook 激活；檢查它：
agentmod status        # "AgentMod: active"，列出路由的路徑
claude                 # 普通命令 — 現在使用項目本地主目錄
agentmod install gstack   # 項目本地技能，全局主目錄不受影響

agentmod pack          # 快照到 .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # 隨時唯讀診斷
```

在接收機器上：

```sh
cd ~/work/myproject    # 源通過 git 到達
agentmod init
agentmod unpack myproject-20260611-123045.amod
# 遵循打印的重新登錄說明；doctor 自動運行
```

## `agentmod init`

冪等 — 重新運行會填入任何缺失的內容，永遠不會覆蓋現有的 `agentmod.toml` 或任何用戶文件。它：

- 創建 `.agentmod/{claude,codex,opencode,node,snapshots,logs}` 和默認的 `agentmod.toml`；
- 將 Claude Bash 防護接入 `.agentmod/claude/settings.json`；
- 將 `.agentmod/` 添加到 `.gitignore`（僅在 git 存儲庫內創建）；
- 在 `~/.zshrc` 或 `~/.bashrc` 中安裝 shell hook 作為圍欄塊（您的 shell 來自 `$SHELL`；該塊就地更新，永遠不會重複，您自己的 rc 內容永遠不會被觸及）；
- 提供**複製**現有 Claude/Codex 身份驗證文件到項目本地主目錄（見下面的「身份驗證」） — 複製僅在明確 `y` 時發生。

標誌：`--no-shell-hook` 跳過所有 rc 文件編輯；`--yes` / `--non-interactive` 永遠不提示，因此永遠不複製身份驗證（對於 CI）。

## 使用普通的 `claude`、`codex`、`opencode`

沒有包裝器命令。在活躍的項目內，普通命令只需看到路由的家目錄：

- **Claude Code** 讀取 `CLAUDE_CONFIG_DIR` → 項目本地設置、用戶級技能/插件、會話、歷史記錄。（項目級 `.claude/` 始終本地讀取 — 見限制。）
- **Codex CLI** 讀取 `CODEX_HOME` → 項目本地 `config.toml`、`auth.json`、會話、歷史記錄、日誌。
- **OpenCode** 讀取 `OPENCODE_CONFIG` → 項目本地配置文件。默認情況下這是**部分**隔離 — 見限制。

### 身份驗證

全新的項目本地主目錄不帶認證開始：

- **Claude on macOS**：無需做任何操作 — 認證存在於鑰匙串中，與每個配置目錄共享（這也意味著它們**不是**按項目隔離的）。
- **Claude on Linux/Windows**：在項目內運行 `claude login`，或接受 init 的提議複製 `~/.claude/.credentials.json`。
- **Codex**：在項目內運行 `codex login`，或接受 init 的提議複製 `~/.codex/auth.json`。

身份驗證文件**永遠不在快照中傳輸**（按名稱排除，無論它們如何到達）。

## gstack 安裝

[gstack](https://github.com/garrytan/gstack) 將其安裝程序硬編碼為 `~/.claude/skills/gstack` — 正是 agentmod 存在的目的要防止的全局污染。所以：

```sh
agentmod install gstack            # 克隆到 .agentmod/claude/skills/gstack
agentmod install gstack --force    # 替換現有的項目本地安裝
```

安裝程序使用 git 克隆，永遠不運行 gstack 自己的設置腳本，並在之前和之後快照 `~/.claude/skills` 的列表 — 對全局目錄的任何更改都報告為違規並使命令失敗。`agentmod doctor` 分別警告何時存在**全局** gstack 安裝（即使是您在採用 agentmod 之前自己安裝的），因為全局安裝的技能會洩露到每個項目中。

## 交接（`.amod` 快照）

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # 清單 + 編輯報告，無提取
agentmod handoff verify  FILE      # 重新哈希每個成員；不匹配時退出 3
agentmod handoff restore FILE      # 替換 .agentmod/（先備份）
agentmod pack / agentmod unpack    # create / restore 的別名
```

快照是一個 zip，有六個根成員 — `manifest.json`、`inventory.json`（每個文件大小/sha256/模式）、`REDACTION.md`（排除了什麼及為什麼，加上秘密掃描發現）、`HANDOFF.md` 和 `RESTORE.md`（接收方的人類說明）、`checksums.txt`（`shasum -a 256 -c` 兼容）— 和 `payload/.agentmod/…` 下的有效載荷。創建是原子和確定性的；清單記錄 git 分支/提交/髒狀態，任何認證已從遠程 URL 中剝離。髒工作樹拒絕打包，除非 `--allow-dirty`。

`inspect` 和 `verify` 可在任何地方工作 — 接收方可以在設置任何項目之前審計快照。

### 秘密排除政策

兩層，默認都開啟：

1. **排除規則**從有效載荷中刪除已知敏感文件，並在 `REDACTION.md` 中列出每個文件：按名稱的身份驗證文件（`.credentials.json`、`auth.json`、`credentials*`）、`*.env` / `.env.*`、SSH 密鑰（`id_*`、`*.pem`、`*.pub`）、認證目錄（`.ssh`、`.aws`、`.azure`、`.gcloud`、`.kube`、`.gnupg`、`.docker`）、鑰匙串文件、`.git`、`node_modules`、緩存和臨時目錄。
2. **內容掃描**遍歷每個**保留的**文件。私鑰物料拒絕創建，除非您傳入 `--allow-findings`（然後標記為 `REDACTION.md` 中的 HARD）。可能的令牌（AWS 訪問密鑰 ID、GitHub 令牌、`sk-…` 密鑰、`api_key=` 風格的分配）被警告但不阻止。

掃描是啟發式的。**在分享快照之前查看 `REDACTION.md`（或 `handoff inspect`）** — 會話和工作上下文按設計傳輸，可能引用您粘貼到代理對話中的任何內容。快照以 0600 模式寫入，原因就是這個；像對待私人文件一樣對待它們。

## Git 交接

```sh
agentmod pack --for-git    # 在項目根目錄下寫入 .agentmod-handoff/
git add .agentmod-handoff && git commit
```

與 `.amod` 相同的六個成員和有效載荷，但作為可提交的普通文件樹（`shasum -a 256 -c checksums.txt` 在目錄中工作）。除默認排除外，它還剝離所有三個代理的**會話、記錄、歷史記錄和日誌** — 這些常規包含粘貼的秘密，不應在存儲庫中。`--include-sessions` 總是拒絕：提交會話將需要加密，此版本未實現。安全共享的工作上下文（CLAUDE.md、代理配置、技能、計劃）保留。

重新運行會替換先前的包；存儲庫中的其他內容都不受影響。

## 恢復警告

`handoff restore` / `unpack` 將每個快照視為不受信任的輸入：

- 完整的校驗和驗證和清單交叉檢查首先進行；
- 路徑安全計劃：zip-slip（`..`）、絕對路徑、驅動器字母、非 `.agentmod` 目標、受保護名稱（`.git`、`.ssh`、`.aws`、`.docker`）和逃逸或絕對符號鏈接目標都在寫任何內容之前被拒絕；
- 現有的 `.agentmod/` 在提取之前重命名為 `.agentmod.backup-<stamp>`；任何故障都自動回滾到它；
- **快照中的任何內容永遠不會被執行**；
- 之後：Claude 防護 hook 被重新接入**此**機器的二進制文件，在恢復的代理配置中發現的機器特定絕對路徑被警告（您的文件永遠不被重寫），`doctor` 內聯運行，並打印所需的重新登錄步驟（身份驗證永遠不傳輸）。

恢復拒絕而不是猜測 — 被拒絕的恢復使項目字節相同。

## `agentmod doctor`

唯讀診斷，安全於任何時間運行（退出 0 清潔，3 有發現）：項目/配置/佈局狀態、shell-hook 安裝和活動狀態、路由漂移、項目外的徘徊變數、重複的 PATH 條目、HOME/shim 違規、按代理的身份驗證存在和重新登錄說明、OpenCode 洩漏警告、gstack 全局/項目狀態、Claude 防護接線、恢復的配置中的可移植性風險、記錄在現有快照中的秘密候選項、`.agentmod-handoff/` 內的會話/日誌物料，以及存儲庫的 HEAD 是否仍與最新快照的匹配。

## Claude Bash 防護

`agentmod init` 在項目本地主目錄中註冊 `agentmod guard claude-bash` 作為 Claude Code PreToolUse hook。它阻止會寫入全局代理家目錄（`~/.claude`、`~/.codex`、`~/.config/opencode`、`~/.local/share/opencode`）、使用 `sudo` 或重新分配 `HOME` 的 Bash 命令 — 代理獲得原因並可以調整。讀取永遠不被阻止。它是一層 shell 解析啟發式深度：有用的護欄，而不是沙箱。

## 已知限制

誠實部分。這些是底層工具的屬性或刻意的 MVP 範圍 — `doctor` 和生成的文檔也陳述它們。

- **macOS 鑰匙串（Claude）。** Claude Code on macOS 在鑰匙串中存儲 OAuth 認證，跨**所有**配置目錄共享。每個項目帳戶隔離在 macOS 上是不可能的 — 每個項目都不需要重新登錄。Linux/Windows 使用按家目錄的 `.credentials.json`，它隔離但需要每個項目的登錄/複製。
- **OpenCode 默認部分隔離。** OpenCode 沒有單一的家變數；其配置是一個仍然讀取全局 `~/.config/opencode/opencode.json` 的合併鏈，會話/存儲/身份驗證存在於全局 XDG 數據目錄中。`opencode.xdg_full_isolation = true` 路由 XDG 變數以完全隔離 — 但這影響您在項目內運行的**每個** XDG 感知工具。`doctor` 報告兩種情況。
- **項目 `.claude/` 是本地 Claude 行為。** Claude Code 總是讀取 `./.claude/`，無論 `CLAUDE_CONFIG_DIR` 如何。agentmod 為 Claude 添加的價值是隔離*用戶級*狀態（全局技能/插件、會話、歷史記錄）；項目 `.claude/` 在 agentmod 之前已經工作。
- **首次會話 hook 激活。** 在 `agentmod init` 之後，已運行的 shell 還沒有加載新的 rc 塊。打開新終端，`exec $SHELL`，或一次性 `eval "$(agentmod hook zsh)"`（init 打印正好這個）。同樣，bash hook 通過 `PROMPT_COMMAND` 觸發，因此在非交互式 bash 腳本中是惯性的（與 direnv 同類限制） — 腳本應通過 `eval "$(agentmod env --shell bash --activate <root>)"` 顯式設置變數，如果它們需要路由。
- **只有 npm 的全局 bin 在 PATH 上。** `.agentmod/node/bin` 是唯一託管的 PATH 條目。pnpm/bun 全局安裝被路由到項目（`PNPM_HOME`、`BUN_INSTALL`）但它們的 bin 目錄不添加到 PATH。
- **樹包手動恢復。** `handoff restore` 只接受 `.amod` 文件；提交的 `.agentmod-handoff/` 目錄通過遵循其內部的 `RESTORE.md` 來恢復（此版本沒有目錄讀取器）。
- **快照可能需要恢復後修復。** gstack 克隆在沒有其 `.git` 的情況下傳輸（重新運行 `agentmod install gstack --force` 以使其再次可更新），`node/bin` 啟動器符號鏈接懸掛，因為 `node_modules` 被排除（在項目內重新運行 `npm install -g …`）。
- **Shell 支持是 zsh 和 bash。** 其他 shell 仍然可以手動使用 `agentmod env`。

## FAQ

**我是否繼續直接使用 `claude` / `codex` / `opencode`？**
是的。那正是重點 — 沒有包裝器、沒有 shim、沒有 `agentmod run`。

**為什麼 agentmod 不只是更改 `HOME`？**
重新分配 `HOME` 會破壞 SSH、git、鑰匙串、點文件和 shell 中的所有其他工具。agentmod 只路由代理特定的變數。

**為什麼恢復後我的身份驗證丟失了？**
按設計 — 認證永遠不在快照中傳輸。在新機器上遵循打印的重新登錄行（或 init 的複製提議）。

**我可以將 `.agentmod/` 提交到 git 嗎？**
否 — init gitignores 它（會話、緩存和可能複製的身份驗證存在於那裡）。改為提交安全子集：`agentmod pack --for-git`。

**這與 direnv 有何不同？**
相同的激活模型（目錄範圍的環境、基於提示 hook、完美退出恢復），但 agentmod 也知道為每個代理路由**什麼**、創建家目錄、防護對抗全局寫入並進行交接。兩者共存很好。

**快照創建失敗，出現"秘密候選發現"。**
內容掃描在保留的文件中找到私鑰物料。刪除它（或將其移動到排除位置如 `.env`），或如果您接受它在快照內，仍然以 `--allow-findings` 打包。

**它在 Windows 上工作嗎？**
Go 代碼構建且路徑安全為 Windows 風格路徑強制執行，但 shell hook 針對 zsh/bash；Windows 在此版本中未測試。

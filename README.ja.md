# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

コーディングエージェント向けのプロジェクト単位の分離とハンドオフ。

`agentmod` は **Claude Code**、**Codex CLI**、**OpenCode** の設定、スキル、プラグイン、セッション、キャッシュ、および作業コンテキストを、現在作業しているプロジェクト内の `.agentmod/` に保持し、その環境をスナップショットにパッケージ化して別のマシンに渡すことができます。

2つの役割を担当します:

1. **エージェント ホーム ルータ**。`.agentmod/agentmod.toml` を含むディレクトリツリー内では、シェルフックが各エージェントのホームを `.agentmod/` に経路立てします。外側では、すべての変数は元の通りに復元され、グローバルセットアップは変更されません。
2. **ハンドオフ ツール**。`agentmod handoff create` は `.agentmod/` を検証可能な `.amod` スナップショットにパッケージ化します（または `--for-git` を使用すると、`.agentmod-handoff/` 下の可視化可能なファイルツリーに変換できます）。**Git はソースコードを移動し、agentmod はエージェント環境を移動します。**

## agentmod が *ない* もの

- **Docker サンドボックスではない**。シェル内の環境変数を経路立てします。コンテナ、VM、システムコール フィルタリングはありません。
- **完全なセキュリティ分離ではない**。経路立てされた変数を無視するツールは、グローバルホームにもアクセスできます。Claude Bash ガード（下記参照）は深層防御であり、セキュリティ境界ではありません。
- **シムではない**。`claude`、`codex`、または `opencode` コマンドをインターセプトまたはラップします。通常のコマンドを直接、未変更で実行し続けます。
- **HOME 変更ツールではない**。`HOME` は再割り当てされることはありません。
- **ソースコード バックアップ ツールではない**。スナップショットにはデフォルトでソースコードが含まれません。ソースにはgitを使用してください。

## 仕組み

`agentmod hook zsh` / `agentmod hook bash` は小さな自己完結型シェル関数を出力し、`agentmod init` によって rc ファイルにインストールされます。毎回プロンプトとディレクトリ変更時に `.agentmod/agentmod.toml` を探して上方向に走査します:

- **プロジェクトに進入する** 現在の値を保存し、以下を設定します:

  | 変数 | 経路立て先 |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **のみ** `opencode.xdg_full_isolation = true` の場合 |

  `PATH` は正確に1つのエントリ `.agentmod/node/bin`（経路立てされたプレフィックス下の npm のグローバル bin）を取得します。ブックキーピング変数（`AGENTMOD_ACTIVE`、`AGENTMOD_PROJECT_ROOT`、`AGENTMOD_ROOT`、`AGENTMOD_VARS`、`AGENTMOD_SAVED_*`）は何を元に戻すかを記録します。

- **プロジェクトを離れる** 保存されたすべての値を復元し、`PATH` エントリを削除します — 完全な逆操作です。2つの agentmod プロジェクト間で直接切り替えると、どちらのプロジェクトのパスもリークせずに1ステップで再経路立てされます。

エージェントごとの経路立ては `agentmod.toml` で無効にできます（`claude.enabled`、`codex.enabled`、`opencode.enabled`、`node.enabled`）。

## インストール

セットアップに合わせて選択してください — 各方法は同じ単一バイナリをインストールします:

```sh
# npm（プラットフォーム用に事前構築されたバイナリをインストール）
npm install -g agentmod

# インストールスクリプト（一致するリリースをダウンロード、sha256 を検証）
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install（Go ツールチェーンが必要）
go install github.com/mojomoth/agentmod@latest
```

またはソースからビルドします（Go 1.26+、唯一のモジュール依存は `BurntSushi/toml`）:

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# PATH 上のどこかにバイナリを配置する
```

## クイックスタート

```sh
cd ~/work/myproject
agentmod init          # creates .agentmod/, edits .gitignore, installs the
                       # shell hook into your rc file, offers to copy auth
# first time only: the hook isn't live in THIS shell yet —
# open a new terminal, or: exec $SHELL

cd ~/work/myproject    # hook activates; check it:
agentmod status        # "AgentMod: active", routed paths listed
claude                 # plain command — now using the project-local home
agentmod install gstack   # project-local skills, global home untouched

agentmod pack          # snapshot to .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # read-only diagnosis any time
```

受け取り側のマシン上:

```sh
cd ~/work/myproject    # source arrived via git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# follow the printed re-login notes; doctor runs automatically
```

## `agentmod init`

べき等（冪等）— 再実行するとすべての不足分が埋まり、既存の `agentmod.toml` またはユーザーファイルは上書きされません。以下を実行します:

- `.agentmod/{claude,codex,opencode,node,snapshots,logs}` とデフォルトの `agentmod.toml` を作成します;
- Claude Bash ガードを `.agentmod/claude/settings.json` に配線します;
- `.agentmod/` を `.gitignore` に追加します（git リポジトリ内でのみ作成）;
- シェルフック を `~/.zshrc` または `~/.bashrc` にインストールします（`$SHELL` からのシェル; ブロックは所定の位置で更新され、重複されることはなく、独自の rc コンテンツは変更されません）;
- 既存の Claude/Codex 認証ファイルをプロジェクトローカルホームにコピー することを提案します（下記の「認証」参照）— コピーは明示的な `y` でのみ発生します。

フラグ: `--no-shell-hook` はすべての rc ファイル編集をスキップします; `--yes` / `--non-interactive` はプロンプトを表示しないため、認証をコピーしません（CI 用）。

## プレーンな `claude`、`codex`、`opencode` を使用する

ラッパーコマンドはありません。アクティブなプロジェクト内では、通常のコマンドが単に経路立てされたホームを見ます:

- **Claude Code** は `CLAUDE_CONFIG_DIR` を読み取ります → プロジェクトローカル設定、ユーザーレベルのスキル/プラグイン、セッション、履歴。（プロジェクトレベルの `.claude/` は *常に* ネイティブに読み取られます — 制限事項を参照）。
- **Codex CLI** は `CODEX_HOME` を読み取ります → プロジェクトローカル `config.toml`、`auth.json`、セッション、履歴、ログ。
- **OpenCode** は `OPENCODE_CONFIG` を読み取ります → プロジェクトローカル設定ファイル。これはデフォルトでは *部分的* 分離です — 制限事項を参照。

### 認証

新しいプロジェクトローカルホームは認証情報なしで開始します:

- **macOS 上の Claude**: 何もすることはありません — 認証情報は Keychain に保存され、すべての設定ディレクトリと共有されます（これは *分離されない* つまりプロジェクトごとに分離されないことも意味します）。
- **Linux/Windows 上の Claude**: プロジェクト内で `claude login` を実行するか、init の提案を受け入れて `~/.claude/.credentials.json` をコピーします。
- **Codex**: プロジェクト内で `codex login` を実行するか、init の提案を受け入れて `~/.codex/auth.json` をコピーします。

認証ファイルはスナップショットで転送 **されません**（名前に関係なく除外）。

## gstack インストール

[gstack](https://github.com/garrytan/gstack) はインストーラを `~/.claude/skills/gstack` にハードコードしています — agentmod が存在する理由はまさに グローバル汚染を防ぐためです。したがって:

```sh
agentmod install gstack            # clone into .agentmod/claude/skills/gstack
agentmod install gstack --force    # replace an existing project-local install
```

インストーラは git でクローンされ、gstack の独自のセットアップスクリプトは実行されず、`~/.claude/skills` のリスティングをスナップショットしてから後で — グローバルディレクトリへの変更は違反として報告され、コマンドに失敗します。`agentmod doctor` は別途、グローバル gstack インストール が存在する場合に警告します（agentmod を採用する前に自分でインストールしたものであっても）。グローバルにインストールされたスキルはすべてのプロジェクトにリークするためです。

## ハンドオフ（`.amod` スナップショット）

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + redaction report, no extraction
agentmod handoff verify  FILE      # re-hash every member; exit 3 on mismatch
agentmod handoff restore FILE      # replace .agentmod/ (backup taken first)
agentmod pack / agentmod unpack    # aliases of create / restore
```

スナップショットは、6つのルートメンバーを持つ zip です — `manifest.json`、`inventory.json`（ファイルごとのサイズ/sha256/モード）、`REDACTION.md`（除外されたもの、理由、秘密スキャン結果）、`HANDOFF.md` と `RESTORE.md`（受信者向けの人間向け指示）、`checksums.txt`（`shasum -a 256 -c` 互換）— およびペイロードは `payload/.agentmod/…` の下にあります。作成はアトミックで確定的です; マニフェストは git ブランチ/コミット/ダーティ状態を記録し、リモート URL から認証情報は削除されます。ダーティワークツリーは `--allow-dirty` がない限りパッキングを拒否します。

`inspect` と `verify` はどこでも動作します — 受信者はプロジェクトをセットアップする前にスナップショットを監査できます。

### 秘密除外ポリシー

2つのレイヤ、両方ともデフォルトで有効:

1. **除外ルール** はペイロードから既知の機密ファイルをドロップし、`REDACTION.md` に各ファイルをリストアップします: 認証ファイル（`.credentials.json`、`auth.json`、`credentials*`）、`*.env` / `.env.*`、SSH キー（`id_*`、`*.pem`、`*.pub`）、認証情報ディレクトリ（`.ssh`、`.aws`、`.azure`、`.gcloud`、`.kube`、`.gnupg`、`.docker`）、キーチェーンファイル、`.git`、`node_modules`、キャッシュ、テンポラリディレクトリ。
2. **すべての保持ファイルに対するコンテンツスキャン**。秘密鍵マテリアルは `--allow-findings` を渡さない限り作成を拒否します（その場合は `REDACTION.md` で HARD としてマークされます）。可能性のあるトークン（AWS アクセスキー ID、GitHub トークン、`sk-…` キー、`api_key=` 形式の割り当て）は警告されますが、ブロックしません。

スキャンはヒューリスティックです。**スナップショットを共有する前に `REDACTION.md`（または `handoff inspect`）を確認してください** — セッションと作業コンテキストは設計によって移動され、エージェント会話に貼り付けたものすべてを引用する可能性があります。スナップショットはこの理由でモード 0600 で書き込まれます; プライベートファイルのように扱います。

## Git ハンドオフ

```sh
agentmod pack --for-git    # writes .agentmod-handoff/ at the project root
git add .agentmod-handoff && git commit
```

`.amod` と同じ 6つのメンバーとペイロード、ただしコミット可能なプレーンファイルのツリー（`shasum -a 256 -c checksums.txt` はディレクトリで動作します）。デフォルト除外に加えて、3つのエージェント全体でセッション、トランスクリプト、履歴、ログを削除します — これらは日常的にペーストされた秘密を含み、リポジトリに属していません。`--include-sessions` は常に拒否します: セッションをコミットする場合、暗号化が必要になり、このバージョンは実装していません。共有しても安全な作業コンテキスト（CLAUDE.md、エージェント設定、スキル、計画）は残ります。

再実行すると前のパッケージが置き換わります; repo 内の他の何も変更されません。

## 復元に関する注意事項

`handoff restore` / `unpack` はすべてのスナップショットを信頼できない入力として扱います:

- 完全なチェックサム検証とインベントリ相互チェックがまず最初;
- パス安全計画: zip-slip（`..`）、絶対パス、ドライブ文字、非 `.agentmod` ターゲット、保護された名前（`.git`、`.ssh`、`.aws`、`.docker`）、エスケープまたは絶対シンボリックリンクターゲットはすべて、何かが書き込まれる前に拒否されます;
- 既存の `.agentmod/` は抽出前に `.agentmod.backup-<stamp>` に名前変更されます; 任意の失敗はそれに自動的にロールバックします;
- **スナップショットから何も実行されることはありません**;
- その後: Claude ガードフックは *このマシンの* バイナリに再配線され、復元されたエージェント設定に見つかったマシン固有の絶対パスは警告されます（ファイルは書き直されません）、`doctor` がインラインで実行され、必要な再ログイン手順が出力されます（認証は転送されません）。

復元は推測ではなく拒否を優先します — 拒否された復元はプロジェクトをバイト単位で同一に残します。

## `agentmod doctor`

読み取り専用診断、いつでも安全に実行できます（exit 0 クリーン、3 つの結果）: プロジェクト/設定/レイアウト状態、シェルフック インストールと生活、経路立てドリフト、プロジェクト外の残存変数、重複した PATH エントリ、HOME/シム違反、エージェントごとの認証存在（再ログイン指示付き）、OpenCode リーク警告、gstack グローバル/プロジェクト状態、Claude ガード配線、復元された設定でのポータビリティリスク、既存スナップショットに記録された秘密候補、`.agentmod-handoff/` 内のセッション/ログマテリアル、およびリポジトリの HEAD が最新スナップショットのものと一致しているかどうか。

## Claude Bash ガード

`agentmod init` は `agentmod guard claude-bash` を Claude Code PreToolUse フックとしてプロジェクトローカルホーム内に登録します。グローバルエージェントホーム（`~/.claude`、`~/.codex`、`~/.config/opencode`、`~/.local/share/opencode`）への書き込み、`sudo` の使用、または `HOME` の再割り当てを行う Bash コマンドをブロックします — エージェントは理由を取得し、調整できます。読み取りはブロックされません。これは 1 つのシェル解析ヒューリスティックの深さです: 有用な保護柵、サンドボックスではありません。

## 既知の制限事項

正直なセクション。これらは基本となるツールの特性または意図的な MVP スコープです — `doctor` と生成ドキュメントもそれらを記載しています。

- **macOS Keychain（Claude）**。macOS 上の Claude Code は OAuth 認証情報を Keychain に保存し、*すべて* の設定ディレクトリ間で共有されます。macOS でのプロジェクト単位のアカウント分離は不可能です — プロジェクトごとの再ログインは不要です。Linux/Windows はプロジェクト単位の `.credentials.json` を使用します。分離されますがプロジェクトごとにログイン/コピーが必要です。
- **OpenCode はデフォルトでは部分的に分離されます**。OpenCode には単一のホーム変数がありません; その設定はマージチェーンで、グローバル `~/.config/opencode/opencode.json` を読み込み続け、セッション/ストレージ/認証はグローバル XDG データディレクトリに保存されます。`opencode.xdg_full_isolation = true` は完全な分離のために XDG 変数を経路立てします — しかしそれはプロジェクト内で実行する *すべて* の XDG 対応ツールに影響します。`doctor` は両方の状況を報告します。
- **プロジェクト `.claude/` はネイティブ Claude 動作です**。Claude Code は `CLAUDE_CONFIG_DIR` に関係なく常に `./.claude/` を読み取ります。agentmod の Claude 向けの追加価値は、*ユーザーレベル* 状態（グローバルスキル/プラグイン、セッション、履歴）を分離することです; プロジェクト `.claude/` は agentmod の前に既に機能していました。
- **最初のセッション フック有効化**。`agentmod init` の直後、既に実行中のシェルは新しい rc ブロックをロードしていません。新しいターミナルを開くか、`exec $SHELL` を実行するか、1 回限り `eval "$(agentmod hook zsh)"` を実行します（init が正確にこれを出力します）。同様に、bash フック は `PROMPT_COMMAND` 経由で発火し、非インタラクティブ bash スクリプトでは無効です（direnv と同じクラスの制限）— スクリプトが経路立てを必要とする場合は、`eval "$(agentmod env --shell bash --activate <root>)"` 経由で変数を明示的に設定する必要があります。
- **PATH にはのみ npm のグローバル bin**。`.agentmod/node/bin` は単一の管理 PATH エントリです。pnpm/bun グローバルインストールはプロジェクトに経路立てされます（`PNPM_HOME`、`BUN_INSTALL`）が、bin ディレクトリ は PATH に追加されません。
- **ツリーパッケージは手動で復元**。`handoff restore` は `.amod` ファイルのみを受け入れます; コミットされた `.agentmod-handoff/` ディレクトリは内部の `RESTORE.md` に従って復元されます（このバージョンはディレクトリリーダーを持たない）。
- **スナップショットは復元後の修復が必要な場合があります**。gstack クローンは `.git` なしで移動します（`agentmod install gstack --force` を再実行して更新可能にする）、`node/bin` ランチャーのシンボリックリンクは `.git` が除外されるため吊り下がります（プロジェクト内で `npm install -g …` を再実行）。
- **シェルサポートは zsh と bash です**。他のシェルは `agentmod env` を手動で使用できます。

## FAQ

**`claude` / `codex` / `opencode` を直接使い続けるか?**
はい。これがポイントです — ラッパーなし、シムなし、`agentmod run` なし。

**agentmod が単に `HOME` を変更しないのはなぜか?**
`HOME` を再割り当てすると、SSH、git、キーチェーン、dotfiles、および シェル内のすべての他のツールが壊れます。agentmod はエージェント固有の変数のみを経路立てします。

**復元後に認証情報が不足しているのはなぜか?**
設計によって — 認証情報はスナップショットで転送されません。新しいマシンで印刷された再ログイン行に従います（または init のコピー提案）。

**`.agentmod/` を git にコミットできますか?**
いいえ — init がそれを無視します（セッション、キャッシュ、およびおそらくコピーされた認証がそこにあります）。代わりに安全なサブセットをコミットします: `agentmod pack --for-git`。

**これは direnv とどう違うか?**
同じアクティベーション モデル（ディレクトリスコープ env、プロンプトフック ベース、終了時の完全復元）、しかし agentmod はまた *何を* 各エージェント向けに経路立てするかを知り、ホームを作成し、グローバル書き込みに対して保護し、ハンドオフを実行します。2つは問題なく共存できます。

**スナップショットの作成が「秘密候補結果」で失敗します。**
コンテンツスキャンは保持ファイル内の秘密鍵マテリアルを発見しました。それを削除するか、`.env` のような除外位置に移動するか、安全に受け入れている場合は `--allow-findings` でとにかくパッキングしてください。

**Windows で動作しますか?**
Go コードはビルドしてパス安全は Windows スタイルのパスに強制されますが、シェルフックは zsh/bash をターゲットにしています; Windows はこのバージョンでテストされていません。

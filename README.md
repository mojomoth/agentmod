# agentmod

Per-project isolation and handoff for coding agents.

`agentmod` keeps the configuration, skills, plugins, sessions, caches, and
working context of **Claude Code**, **Codex CLI**, and **OpenCode** inside the
project you are working on — and packs that environment into a snapshot you
can hand to another machine.

It plays two roles:

1. **Agent Home Router.** Inside a directory tree containing
   `.agentmod/agentmod.toml`, a shell hook routes each agent's home into
   `.agentmod/`. Outside, every variable is restored exactly as it was and
   your global setup is untouched.
2. **Handoff Tool.** `agentmod handoff create` packs `.agentmod/` into a
   verifiable `.amod` snapshot (or, with `--for-git`, a committable file tree
   under `.agentmod-handoff/`). **Git moves your source; agentmod moves the
   agent environment.**

## What agentmod is *not*

- **Not a Docker sandbox.** It routes environment variables in your own
  shell. There is no container, no VM, no syscall filtering.
- **Not full security isolation.** A tool that ignores the routed variables
  can still reach your global homes. The Claude Bash guard (below) is
  defense-in-depth, not a security boundary.
- **Not a shim.** It never intercepts or wraps the `claude`, `codex`, or
  `opencode` commands. You keep running them directly, unmodified.
- **Not a HOME-changing tool.** `HOME` is never reassigned.
- **Not a source-code backup tool.** Snapshots never include your source
  code by default. Use git for source.

## How it works

`agentmod hook zsh` / `agentmod hook bash` print a small self-contained shell
function (installed into your rc file by `agentmod init`). On every prompt
and directory change it walks upward looking for `.agentmod/agentmod.toml`:

- **Entering a project** saves the current values and sets:

  | Variable | Routed to |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **only** with `opencode.xdg_full_isolation = true` |

  `PATH` gains exactly one entry, `.agentmod/node/bin` (npm's global bin
  under the routed prefix). Bookkeeping variables (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) record what to undo.

- **Leaving the project** restores every saved value and strips the `PATH`
  entry — a perfect inverse. Switching directly between two agentmod
  projects re-routes in one step without leaking either project's paths.

Routing per agent can be switched off in `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Install

Build from source (Go 1.26+, the only module dependency is
`BurntSushi/toml`):

```sh
git clone <this repository> && cd agentmod
go build -o agentmod .
# put the binary somewhere on your PATH
```

## Quick start

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

On the receiving machine:

```sh
cd ~/work/myproject    # source arrived via git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# follow the printed re-login notes; doctor runs automatically
```

## `agentmod init`

Idempotent — re-running fills in whatever is missing and never overwrites an
existing `agentmod.toml` or any user file. It:

- creates `.agentmod/{claude,codex,opencode,node,snapshots,logs}` and a
  default `agentmod.toml`;
- wires the Claude Bash guard into `.agentmod/claude/settings.json`;
- adds `.agentmod/` to `.gitignore` (created only inside a git repository);
- installs the shell hook as a fenced block in `~/.zshrc` or `~/.bashrc`
  (your shell from `$SHELL`; the block is updated in place, never
  duplicated, and your own rc content is never touched);
- offers to **copy** existing Claude/Codex auth files into the project-local
  home (see "Auth" below) — copying happens only on an explicit `y`.

Flags: `--no-shell-hook` skips all rc-file edits; `--yes` /
`--non-interactive` never prompts and therefore never copies auth (for CI).

## Using plain `claude`, `codex`, `opencode`

There is no wrapper command. Inside an active project the ordinary commands
simply see the routed homes:

- **Claude Code** reads `CLAUDE_CONFIG_DIR` → project-local settings,
  user-level skills/plugins, sessions, history. (Project-level `.claude/`
  is *always* read natively — see Limitations.)
- **Codex CLI** reads `CODEX_HOME` → project-local `config.toml`,
  `auth.json`, sessions, history, logs.
- **OpenCode** reads `OPENCODE_CONFIG` → the project-local config file.
  This is *partial* isolation by default — see Limitations.

### Auth

Fresh project-local homes start without credentials:

- **Claude on macOS**: nothing to do — credentials live in the Keychain and
  are shared with every config dir (which also means they are *not*
  isolated per project).
- **Claude on Linux/Windows**: run `claude login` inside the project, or
  accept init's offer to copy `~/.claude/.credentials.json`.
- **Codex**: run `codex login` inside the project, or accept init's offer
  to copy `~/.codex/auth.json`.

Auth files **never travel in snapshots** (excluded by name, regardless of
how they got there).

## gstack installation

[gstack](https://github.com/garrytan/gstack) hardcodes its installer to
`~/.claude/skills/gstack` — exactly the global pollution agentmod exists to
prevent. So:

```sh
agentmod install gstack            # clone into .agentmod/claude/skills/gstack
agentmod install gstack --force    # replace an existing project-local install
```

The installer clones with git, never runs gstack's own setup script, and
snapshots the listing of `~/.claude/skills` before and after — any change to
the global directory is reported as a violation and fails the command.
`agentmod doctor` separately warns whenever a *global* gstack install exists
(even one you installed yourself before adopting agentmod), because globally
installed skills leak into every project.

## Handoff (`.amod` snapshots)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + redaction report, no extraction
agentmod handoff verify  FILE      # re-hash every member; exit 3 on mismatch
agentmod handoff restore FILE      # replace .agentmod/ (backup taken first)
agentmod pack / agentmod unpack    # aliases of create / restore
```

A snapshot is a zip with six root members — `manifest.json`,
`inventory.json` (per-file size/sha256/mode), `REDACTION.md` (what was
excluded and why, plus secret-scan findings), `HANDOFF.md` and `RESTORE.md`
(human instructions for the receiver), `checksums.txt`
(`shasum -a 256 -c`-compatible) — and the payload under
`payload/.agentmod/…`. Creation is atomic and deterministic; the manifest
records git branch/commit/dirty state with any credentials stripped from the
remote URL. A dirty worktree refuses to pack unless `--allow-dirty`.

`inspect` and `verify` work anywhere — the receiver can audit a snapshot
before having any project set up.

### Secrets exclusion policy

Two layers, both on by default:

1. **Exclusion rules** drop known-sensitive files from the payload and list
   each one in `REDACTION.md`: auth files by name (`.credentials.json`,
   `auth.json`, `credentials*`), `*.env` / `.env.*`, SSH keys (`id_*`,
   `*.pem`, `*.pub`), credential directories (`.ssh`, `.aws`, `.azure`,
   `.gcloud`, `.kube`, `.gnupg`, `.docker`), keychain files, `.git`,
   `node_modules`, caches and temp dirs.
2. **A content scan** over every *kept* file. Private-key material refuses
   creation outright unless you pass `--allow-findings` (and is then marked
   HARD in `REDACTION.md`). Likely tokens (AWS access key IDs, GitHub
   tokens, `sk-…` keys, `api_key=`-style assignments) are warned about but
   don't block.

The scan is heuristic. **Review `REDACTION.md` (or `handoff inspect`) before
sharing a snapshot** — sessions and working context travel by design and may
quote anything you pasted into an agent conversation. Snapshots are written
mode 0600 for this reason; treat them like private files.

## Git handoff

```sh
agentmod pack --for-git    # writes .agentmod-handoff/ at the project root
git add .agentmod-handoff && git commit
```

Same six members and payload as a `.amod`, but as a committable tree of
plain files (`shasum -a 256 -c checksums.txt` works in the directory). On
top of the default exclusions it strips **sessions, transcripts, history,
and logs** for all three agents — those routinely contain pasted secrets and
don't belong in a repository. `--include-sessions` always refuses:
committing sessions would require encryption, which this version does not
implement. Working context that is safe to share (CLAUDE.md, agent configs,
skills, plans) stays in.

Re-running replaces the previous package; nothing else in the repo is
touched.

## Restore cautions

`handoff restore` / `unpack` treats every snapshot as untrusted input:

- full checksum verification and inventory cross-check first;
- path-safety plan: zip-slip (`..`), absolute paths, drive letters,
  non-`.agentmod` targets, protected names (`.git`, `.ssh`, `.aws`,
  `.docker`), and escaping or absolute symlink targets are all refused
  before anything is written;
- the existing `.agentmod/` is renamed to `.agentmod.backup-<stamp>` before
  extraction; any failure rolls back to it automatically;
- **nothing from a snapshot is ever executed**;
- afterwards: the Claude guard hook is re-wired to *this* machine's binary,
  machine-specific absolute paths found in restored agent configs are
  warned about (your files are never rewritten), `doctor` runs inline, and
  the required re-login steps are printed (auth never travels).

Restores refuse rather than guess — a refused restore leaves the project
byte-identical.

## `agentmod doctor`

Read-only diagnosis, safe to run any time (exit 0 clean, 3 with findings):
project/config/layout state, shell-hook installation and liveness, routing
drift, lingering variables outside projects, duplicate PATH entries,
HOME/shim violations, per-agent auth presence with re-login instructions,
OpenCode leak warnings, gstack global/project state, Claude guard wiring,
and portability risks in restored configs.

## The Claude Bash guard

`agentmod init` registers `agentmod guard claude-bash` as a Claude Code
PreToolUse hook in the project-local home. It blocks Bash commands that
would write to the global agent homes (`~/.claude`, `~/.codex`,
`~/.config/opencode`, `~/.local/share/opencode`), use `sudo`, or reassign
`HOME` — the agent gets the reason back and can adjust. Reads are never
blocked. It is one shell-parse heuristic deep: useful guardrail, not a
sandbox.

## Known limitations

Honesty section. These are properties of the underlying tools or deliberate
MVP scope — `doctor` and the generated docs state them too.

- **macOS Keychain (Claude).** Claude Code on macOS stores OAuth credentials
  in the Keychain, shared across *all* config dirs. Per-project account
  isolation is impossible on macOS — and no re-login is needed per project.
  Linux/Windows use a per-home `.credentials.json`, which isolates but
  requires login/copy per project.
- **OpenCode is partially isolated by default.** OpenCode has no single
  home variable; its config is a merge chain that still reads the global
  `~/.config/opencode/opencode.json`, and sessions/storage/auth live in
  global XDG data dirs. `opencode.xdg_full_isolation = true` routes the
  XDG variables for full isolation — but that affects *every* XDG-aware
  tool you run inside the project. `doctor` reports both situations.
- **Project `.claude/` is native Claude behavior.** Claude Code always
  reads `./.claude/` regardless of `CLAUDE_CONFIG_DIR`. agentmod's added
  value for Claude is isolating *user-level* state (global skills/plugins,
  sessions, history); project `.claude/` already worked before agentmod.
- **First-session hook activation.** Right after `agentmod init`, the
  already-running shell hasn't loaded the new rc block. Open a new
  terminal, `exec $SHELL`, or one-shot `eval "$(agentmod hook zsh)"` (init
  prints exactly this). Likewise, the bash hook fires via `PROMPT_COMMAND`
  and is therefore inert in non-interactive bash scripts (same class of
  limitation as direnv) — scripts should set the variables explicitly via
  `eval "$(agentmod env --shell bash --activate <root>)"` if they need
  routing.
- **Only npm's global bin is on PATH.** `.agentmod/node/bin` is the single
  managed PATH entry. pnpm/bun global installs are routed into the project
  (`PNPM_HOME`, `BUN_INSTALL`) but their bin dirs are not added to PATH.
- **Tree packages restore manually.** `handoff restore` accepts `.amod`
  files only; a committed `.agentmod-handoff/` directory is restored by
  following the `RESTORE.md` inside it (this version has no directory
  reader).
- **Snapshots may need post-restore repair.** The gstack clone travels
  without its `.git` (re-run `agentmod install gstack --force` to make it
  updatable again), and `node/bin` launcher symlinks dangle because
  `node_modules` is excluded (re-run `npm install -g …` inside the
  project).
- **Shell support is zsh and bash.** Other shells can still use
  `agentmod env` manually.

## FAQ

**Do I keep using `claude` / `codex` / `opencode` directly?**
Yes. That is the point — no wrappers, no shims, no `agentmod run`.

**Why doesn't agentmod just change `HOME`?**
Reassigning `HOME` breaks SSH, git, keychains, dotfiles, and every other
tool in the shell. agentmod routes only the agent-specific variables.

**Why is my auth missing after a restore?**
By design — credentials never travel in snapshots. Follow the printed
re-login lines (or init's copy offer) on the new machine.

**Can I commit `.agentmod/` to git?**
No — init gitignores it (sessions, caches, and possibly copied auth live
there). Commit the safe subset instead: `agentmod pack --for-git`.

**How is this different from direnv?**
Same activation model (directory-scoped env, prompt-hook based, perfect
restore on exit), but agentmod also knows *what* to route for each agent,
creates the homes, guards against global writes, and does handoff. The two
coexist fine.

**A snapshot fails to create with "secret-candidate findings".**
The content scan found private-key material in a kept file. Remove it (or
move it to an excluded location like `.env`), or pack anyway with
`--allow-findings` if you accept it being inside the snapshot.

**Does it work on Windows?**
The Go code builds and path safety is enforced for Windows-style paths, but
the shell hooks target zsh/bash; Windows is untested in this version.

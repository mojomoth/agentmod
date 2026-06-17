# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

프로젝트별 격리 및 이관을 위한 코딩 에이전트 도구.

`agentmod`는 **Claude Code**, **Codex CLI**, **OpenCode**의 설정, 스킬, 플러그인, 세션, 캐시 및 작업 컨텍스트를 진행 중인 프로젝트 내에 유지하고, 그 환경을 다른 머신으로 넘길 수 있는 스냅샷으로 패킹합니다.

두 가지 역할을 수행합니다:

1. **에이전트 홈 라우터.** `.agentmod/agentmod.toml`을 포함한 디렉토리 트리 내에서 셸 훅이 각 에이전트의 홈을 `.agentmod/`로 라우팅합니다. 외부에서는 모든 변수가 정확히 원래대로 복원되고 전역 설정은 건드리지 않습니다.
2. **이관 도구.** `agentmod handoff create`는 `.agentmod/`를 검증 가능한 `.amod` 스냅샷으로 패킹합니다 (또는 `--for-git` 옵션으로 `.agentmod-handoff/` 아래의 커밋 가능한 파일 트리로). **Git은 소스 코드를 옮기고, agentmod는 에이전트 환경을 옮깁니다.**

## agentmod가 *아닌* 것

- **Docker 샌드박스가 아닙니다.** 자신의 셸에서 환경 변수를 라우팅할 뿐입니다. 컨테이너, VM, syscall 필터링은 없습니다.
- **완전한 보안 격리가 아닙니다.** 라우팅된 변수를 무시하는 도구는 여전히 전역 홈에 접근할 수 있습니다. Claude Bash 가드(아래)는 심층 방어이지 보안 경계가 아닙니다.
- **Shim이 아닙니다.** `claude`, `codex`, `opencode` 명령을 가로채거나 래핑하지 않습니다. 이들 명령을 직접 수정 없이 실행합니다.
- **HOME 변경 도구가 아닙니다.** `HOME`은 절대 재할당되지 않습니다.
- **소스 코드 백업 도구가 아닙니다.** 스냅샷은 기본적으로 소스 코드를 포함하지 않습니다. 소스는 git을 사용하세요.

## 작동 방식

`agentmod hook zsh` / `agentmod hook bash`는 작은 자체 포함 셸 함수를 출력합니다 (`agentmod init`으로 rc 파일에 설치됨). 모든 프롬프트와 디렉토리 변경 시 `.agentmod/agentmod.toml`을 찾기 위해 위로 탐색합니다:

- **프로젝트 진입** 현재 값을 저장하고 다음을 설정합니다:

  | 변수 | 라우팅 대상 |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **오직** `opencode.xdg_full_isolation = true` 설정일 때만 |

  `PATH`는 정확히 하나의 항목을 얻습니다: `.agentmod/node/bin` (라우팅된 접두사 아래의 npm 전역 bin). 북마크 변수 (`AGENTMOD_ACTIVE`, `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`, `AGENTMOD_SAVED_*`)는 되돌려야 할 사항을 기록합니다.

- **프로젝트 떠나기** 저장된 모든 값을 복원하고 `PATH` 항목을 제거합니다 — 완벽한 역순입니다. 두 agentmod 프로젝트 사이를 직접 전환할 때 어느 프로젝트의 경로도 누수 없이 한 번에 라우팅을 다시 설정합니다.

에이전트별 라우팅은 `agentmod.toml`에서 끌 수 있습니다 (`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## 설치

사용 중인 설정에 맞는 것을 선택하세요 — 모두 동일한 단일 바이너리를 설치합니다:

```sh
# npm (플랫폼용 미리 빌드된 바이너리 설치)
npm install -g agentmod

# 설치 스크립트 (일치하는 릴리스 다운로드, sha256 검증)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (Go 도구체인 필요)
go install github.com/mojomoth/agentmod@latest
```

또는 소스에서 빌드 (Go 1.26+, 유일한 모듈 의존성은 `BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# 바이너리를 PATH의 어딘가에 배치
```

## 빠른 시작

```sh
cd ~/work/myproject
agentmod init          # .agentmod/ 생성, .gitignore 편집, 셸 훅을 rc 파일에 설치,
                       # 인증 복사 제안
# 첫 번째만: 훅이 이 셸에서 아직 활성화되지 않음 —
# 새 터미널을 열거나: exec $SHELL

cd ~/work/myproject    # 훅 활성화; 확인:
agentmod status        # "AgentMod: active", 라우팅된 경로 나열
claude                 # 일반 명령 — 이제 프로젝트 로컬 홈 사용
agentmod install gstack   # 프로젝트 로컬 스킬, 전역 홈 건드리지 않음

agentmod pack          # .agentmod/snapshots/<name>-<stamp>.amod으로 스냅샷
agentmod doctor        # 언제든 읽기 전용 진단
```

수신 머신에서:

```sh
cd ~/work/myproject    # 소스는 git을 통해 도착
agentmod init
agentmod unpack myproject-20260611-123045.amod
# 출력된 재로그인 메모를 따라가세요; doctor가 자동 실행됨
```

## `agentmod init`

멱등성 — 재실행하면 빠진 것을 채우고 기존 `agentmod.toml` 또는 사용자 파일은 절대 덮어쓰지 않습니다. 다음을 수행합니다:

- `.agentmod/{claude,codex,opencode,node,snapshots,logs}` 및 기본 `agentmod.toml` 생성;
- Claude Bash 가드를 `.agentmod/claude/settings.json`에 연결;
- `.agentmod/`를 `.gitignore`에 추가 (git 리포지토리 내에서만 생성);
- 셸 훅을 `~/.zshrc` 또는 `~/.bashrc`의 펜스된 블록으로 설치 (`$SHELL`의 셸; 블록은 제자리에서 업데이트되고 절대 중복되지 않으며 rc 콘텐츠는 건드리지 않음);
- 기존 Claude/Codex 인증 파일을 프로젝트 로컬 홈에 **복사**하도록 제안 (아래 "Auth" 참조) — 복사는 명시적인 `y`에서만 발생합니다.

플래그: `--no-shell-hook`은 모든 rc 파일 편집을 건너뜁니다; `--yes` / `--non-interactive`는 절대 프롬프트하지 않으므로 인증을 복사하지 않습니다 (CI용).

## 일반 `claude`, `codex`, `opencode` 사용

래퍼 명령은 없습니다. 활성 프로젝트 내에서 일반 명령은 단순히 라우팅된 홈을 봅니다:

- **Claude Code**는 `CLAUDE_CONFIG_DIR` → 프로젝트 로컬 설정, 사용자 수준 스킬/플러그인, 세션, 히스토리를 읽습니다. (프로젝트 수준 `.claude/`는 *항상* 기본으로 읽혀집니다 — 제한 사항 참조.)
- **Codex CLI**는 `CODEX_HOME` → 프로젝트 로컬 `config.toml`, `auth.json`, 세션, 히스토리, 로그를 읽습니다.
- **OpenCode**는 `OPENCODE_CONFIG` → 프로젝트 로컬 설정 파일을 읽습니다. 이는 기본적으로 *부분* 격리입니다 — 제한 사항 참조.

### 인증

새로운 프로젝트 로컬 홈은 자격증명 없이 시작됩니다:

- **macOS의 Claude**: 할 일 없음 — 자격증명은 Keychain에 있고 모든 설정 디렉토리와 공유됩니다 (즉, *격리* 상태가 *아님* per 프로젝트).
- **Linux/Windows의 Claude**: 프로젝트 내에서 `claude login`을 실행하거나 init의 `~/.claude/.credentials.json` 복사 제안을 수락합니다.
- **Codex**: 프로젝트 내에서 `codex login`을 실행하거나 init의 `~/.codex/auth.json` 복사 제안을 수락합니다.

인증 파일은 **스냅샷에서 절대 이동하지 않습니다** (어떻게 들어왔든 이름으로 제외됨).

## gstack 설치

[gstack](https://github.com/garrytan/gstack)은 설치 관리자를 `~/.claude/skills/gstack`에 하드코딩합니다 — agentmod가 존재하는 정확한 전역 오염입니다. 따라서:

```sh
agentmod install gstack            # .agentmod/claude/skills/gstack에 복제
agentmod install gstack --force    # 기존 프로젝트 로컬 설치 교체
```

설치 관리자는 git으로 복제하고, gstack의 자체 설정 스크립트를 절대 실행하지 않으며, `~/.claude/skills` 목록을 이전 후로 스냅샷합니다 — 전역 디렉토리의 모든 변경은 위반으로 보고되고 명령이 실패합니다.
`agentmod doctor`는 *전역* gstack 설치가 존재할 때마다 별도로 경고합니다 (agentmod를 채택하기 전에 직접 설치한 것이더라도), 전역으로 설치된 스킬은 모든 프로젝트에 누수되기 때문입니다.

## 이관 (`.amod` 스냅샷)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # 매니페스트 + 편집 보고서, 추출 없음
agentmod handoff verify  FILE      # 모든 멤버 다시 해시; 불일치 시 종료 3
agentmod handoff restore FILE      # .agentmod/ 교체 (백업 먼저 생성)
agentmod pack / agentmod unpack    # create / restore의 별칭
```

스냅샷은 6개의 루트 멤버를 가진 zip — `manifest.json`, `inventory.json` (파일 크기/sha256/모드별), `REDACTION.md` (제외된 것과 이유, 플러스 비밀 스캔 결과), `HANDOFF.md` 및 `RESTORE.md` (수신자를 위한 인간 지침), `checksums.txt` (`shasum -a 256 -c` 호환) — 및 `payload/.agentmod/…` 아래의 페이로드입니다. 생성은 원자적이고 결정론적입니다; 매니페스트는 원격 URL에서 자격증명을 제거한 git 브랜치/커밋/더티 상태를 기록합니다. 더티 워크트리는 `--allow-dirty` 없이 패킹을 거부합니다.

`inspect`와 `verify`는 어디서나 작동합니다 — 수신자는 프로젝트 설정 없이 스냅샷을 감사할 수 있습니다.

### 비밀 제외 정책

두 계층, 모두 기본적으로 켜짐:

1. **제외 규칙**은 알려진 민감한 파일을 페이로드에서 제거하고 각 파일을 `REDACTION.md`에 나열합니다: 인증 파일 이름으로 (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`, SSH 키 (`id_*`, `*.pem`, `*.pub`), 자격증명 디렉토리 (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), 키체인 파일, `.git`, `node_modules`, 캐시 및 임시 디렉토리.
2. **모든 *유지된* 파일에 대한 콘텐츠 스캔.** 개인 키 자료는 `--allow-findings`를 전달하지 않으면 생성을 거부합니다 (그리고 `REDACTION.md`에서 HARD로 표시됨). 가능성 있는 토큰 (AWS 액세스 키 ID, GitHub 토큰, `sk-…` 키, `api_key=` 스타일 할당)은 경고하지만 블로킹하지 않습니다.

스캔은 휴리스틱입니다. **스냅샷을 공유하기 전에 `REDACTION.md` (또는 `handoff inspect`)를 검토하세요** — 세션 및 작업 컨텍스트는 설계상 이동하고 에이전트 대화에 붙여넣은 모든 내용을 인용할 수 있습니다. 이 이유로 스냅샷은 모드 0600으로 작성됩니다; 개인 파일처럼 취급하세요.

## Git 이관

```sh
agentmod pack --for-git    # 프로젝트 루트에서 .agentmod-handoff/ 기록
git add .agentmod-handoff && git commit
```

`.amod`와 동일한 6개 멤버 및 페이로드이지만 일반 파일의 커밋 가능한 트리로 (`shasum -a 256 -c checksums.txt`는 디렉토리에서 작동). 기본 제외 사항 위에 3개 에이전트 모두에 대해 **세션, 트랜스크립트, 히스토리 및 로그**를 제거합니다 — 이들은 일반적으로 붙여넣은 비밀을 포함하고 리포지토리에 속하지 않습니다. `--include-sessions`은 항상 거부합니다: 세션을 커밋하려면 이 버전이 구현하지 않은 암호화가 필요합니다. 공유하기에 안전한 작업 컨텍스트 (CLAUDE.md, 에이전트 설정, 스킬, 계획)는 유지됩니다.

재실행은 이전 패키지를 교체합니다; 리포의 다른 항목은 건드리지 않습니다.

## 복원 주의 사항

`handoff restore` / `unpack`은 모든 스냅샷을 신뢰할 수 없는 입력으로 취급합니다:

- 전체 체크섬 검증 및 인벤토리 교차 확인이 먼저 실행됩니다;
- 경로 안전 계획: zip-slip (`..`), 절대 경로, 드라이브 문자, 비`.agentmod` 대상, 보호된 이름 (`.git`, `.ssh`, `.aws`, `.docker`), 및 이스케이프 또는 절대 심볼릭 링크 대상 — 모두 아무것도 작성되기 전에 거부됩니다;
- 기존 `.agentmod/`는 추출 전에 `.agentmod.backup-<stamp>`으로 이름이 변경됩니다; 모든 실패는 자동으로 롤백합니다;
- **스냅샷의 아무것도 절대 실행되지 않습니다**;
- 그 후: Claude 가드 훅이 *이* 머신의 바이너리로 다시 연결되고, 복원된 에이전트 설정에서 발견된 머신 특정 절대 경로에 대해 경고합니다 (파일은 절대 다시 작성되지 않음), `doctor`가 인라인으로 실행되고, 필요한 재로그인 단계가 출력됩니다 (인증은 절대 이동하지 않음).

복원은 추측하지 않고 거부합니다 — 거부된 복원은 프로젝트를 바이트 동일하게 남깁니다.

## `agentmod doctor`

읽기 전용 진단, 언제든 실행 안전 (종료 0 깨끗함, 3 결과 포함): 프로젝트/설정/레이아웃 상태, 셸 훅 설치 및 생생함, 라우팅 드리프트, 프로젝트 외부의 남은 변수, 중복 PATH 항목, HOME/shim 위반, 재로그인 지침이 있는 에이전트 인증 존재, OpenCode 누수 경고, gstack 전역/프로젝트 상태, Claude 가드 연결, 복원된 설정의 이식성 위험, 기존 스냅샷에 기록된 비밀 후보, `.agentmod-handoff/` 내의 세션/로그 자료, 및 리포지토리의 HEAD가 여전히 최신 스냅샷과 일치하는지 여부.

## Claude Bash 가드

`agentmod init`은 `agentmod guard claude-bash`를 프로젝트 로컬 홈의 Claude Code PreToolUse 훅으로 등록합니다. 전역 에이전트 홈 (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`)에 쓰거나, `sudo`를 사용하거나, `HOME`을 재할당할 수 있는 Bash 명령을 블로킹합니다 — 에이전트는 이유를 받고 조정할 수 있습니다. 읽기는 절대 블로킹되지 않습니다. 이는 한 단계 셸 파싱 휴리스틱입니다: 유용한 안전 기능이지 샌드박스는 아닙니다.

## 알려진 제한 사항

정직 섹션. 이는 기본 도구의 속성이거나 의도적인 MVP 범위입니다 — `doctor`와 생성된 문서도 명시합니다.

- **macOS Keychain (Claude).** macOS의 Claude Code는 OAuth 자격증명을 Keychain에 저장하며, *모든* 설정 디렉토리 간에 공유됩니다. Per 프로젝트 계정 격리는 macOS에서 불가능합니다 — 프로젝트당 재로그인이 필요하지 않습니다. Linux/Windows는 per-홈 `.credentials.json`을 사용하여 격리하지만 프로젝트당 로그인/복사가 필요합니다.
- **OpenCode는 기본적으로 부분적으로 격리됩니다.** OpenCode는 단일 홈 변수가 없으며; 그 설정은 여전히 전역 `~/.config/opencode/opencode.json`을 읽는 머지 체인이고, 세션/스토리지/인증은 전역 XDG 데이터 디렉토리에 있습니다. `opencode.xdg_full_isolation = true`는 완전 격리를 위해 XDG 변수를 라우팅합니다 — 하지만 이는 프로젝트 내에서 실행하는 *모든* XDG 인식 도구에 영향을 줍니다. `doctor`는 두 상황을 보고합니다.
- **프로젝트 `.claude/`는 네이티브 Claude 동작입니다.** Claude Code는 `CLAUDE_CONFIG_DIR`에 관계없이 항상 `./.claude/`를 읽습니다. agentmod의 Claude에 대한 추가 가치는 *사용자 수준* 상태 격리 (전역 스킬/플러그인, 세션, 히스토리); 프로젝트 `.claude/`는 agentmod 이전에 이미 작동했습니다.
- **첫 세션 훅 활성화.** `agentmod init` 직후, 이미 실행 중인 셸이 새 rc 블록을 로드하지 않았습니다. 새 터미널을 열거나 `exec $SHELL` 또는 원샷 `eval "$(agentmod hook zsh)"` (init이 정확히 이를 출력)을 열어보세요. 마찬가지로 bash 훅은 `PROMPT_COMMAND`를 통해 실행되고 따라서 비대화형 bash 스크립트에서 비활성 상태입니다 (direnv와 동일한 제한 클래스) — 라우팅이 필요한 경우 스크립트는 `eval "$(agentmod env --shell bash --activate <root>)"` 명령으로 변수를 명시적으로 설정해야 합니다.
- **PATH에는 npm의 전역 bin만 있습니다.** `.agentmod/node/bin`은 단일 관리 PATH 항목입니다. pnpm/bun 전역 설치는 프로젝트에 라우팅됩니다 (`PNPM_HOME`, `BUN_INSTALL`) 하지만 그들의 bin 디렉토리는 PATH에 추가되지 않습니다.
- **트리 패키지는 수동으로 복원됩니다.** `handoff restore`는 `.amod` 파일만 수락합니다; 커밋된 `.agentmod-handoff/` 디렉토리는 내부 `RESTORE.md`를 따라 복원됩니다 (이 버전은 디렉토리 판독기가 없음).
- **스냅샷은 복원 후 수리가 필요할 수 있습니다.** gstack 클론은 `.git` 없이 이동하고 (`.agentmod/claude/skills/gstack`을 업데이트 가능하게 만들려면 `agentmod install gstack --force`를 다시 실행), `node/bin` 런처 심볼릭 링크는 `node_modules`이 제외되기 때문에 (프로젝트 내에서 `npm install -g …`를 다시 실행).
- **셸 지원은 zsh와 bash입니다.** 다른 셸은 여전히 수동으로 `agentmod env`를 사용할 수 있습니다.

## FAQ

**`claude` / `codex` / `opencode`를 직접 계속 사용합니까?**
네. 그것이 포인트입니다 — 래퍼 없음, shim 없음, `agentmod run` 없음.

**agentmod가 그냥 `HOME`을 변경하지 않는 이유는?**
`HOME` 재할당는 SSH, git, 키체인, dotfile 및 셸의 모든 다른 도구를 깹니다. agentmod는 에이전트 특정 변수만 라우팅합니다.

**복원 후 인증이 누락된 이유는?**
설계상 — 자격증명은 스냅샷에서 절대 이동하지 않습니다. 새 머신에서 출력된 재로그인 라인을 따르세요 (또는 init의 복사 제안).

**`.agentmod/`를 git에 커밋할 수 있습니까?**
아니요 — init이 gitignore합니다 (세션, 캐시 및 가능한 복사 인증이 거기에 있음). 대신 안전한 부분을 커밋하세요: `agentmod pack --for-git`.

**이것이 direnv와 다른 점은?**
동일한 활성화 모델 (디렉토리 범위 env, 프롬프트 훅 기반, 종료 시 완벽한 복원), 하지만 agentmod는 또한 각 에이전트에 대해 무엇을 라우팅할지 알고, 홈을 생성하고, 전역 쓰기를 방지하고, 이관을 수행합니다. 둘은 잘 공존합니다.

**스냅샷이 "secret-candidate findings"로 생성 실패합니다.**
콘텐츠 스캔이 유지된 파일에서 개인 키 자료를 찾았습니다. 제거하세요 (또는 `.env`처럼 제외된 위치로 이동) 또는 스냅샷에 있는 것을 수락하면 `--allow-findings`로 어쨌든 패킹하세요.

**Windows에서 작동합니까?**
Go 코드가 빌드되고 경로 안전이 Windows 스타일 경로에 적용되지만, 셸 훅은 zsh/bash를 대상으로 합니다; Windows는 이 버전에서 미테스트입니다.

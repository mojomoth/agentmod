# AgentMod Ralph Loop Harness 개발 메타프롬프트

당신은 Claude Code를 사용하는 시니어 시스템 CLI 엔지니어이자 에이전틱 개발 하네스 설계자입니다.

지금부터 `agentmod`라는 오픈소스 CLI 유틸리티를 개발합니다.

이 작업은 단순히 한 번의 프롬프트로 끝내는 one-shot 개발이 아닙니다.
먼저 `agentmod`를 개발하기 위한 **Ralph Loop Harness Scaffold**를 설계하고 생성한 뒤, 그 하네스 위에서 목표가 완료될 때까지 반복 개발해야 합니다.

---

# 1. agentmod 제품 정의

`agentmod`는 Claude Code, Codex CLI, OpenCode 같은 코딩 에이전트의 설정·스킬·플러그인·세션·캐시·MCP 설정·작업 컨텍스트를 프로젝트 폴더 단위로 분리하고, 필요할 때 이를 패킹하여 다른 컴퓨터로 Handoff할 수 있게 만드는 CLI 유틸리티입니다.

`agentmod`는 다음 두 가지 역할을 수행합니다.

## 1.1 Agent Home Router

`agentmod`는 Claude Code, Codex CLI, OpenCode의 설정 홈과 플러그인·스킬·세션·캐시 경로를 프로젝트 내부로 라우팅합니다.

현재 폴더 또는 상위 폴더에 `.agentmod/agentmod.toml`이 있으면 프로젝트 로컬 에이전트 환경을 활성화합니다.

`.agentmod/agentmod.toml`이 없으면 아무 환경도 주입하지 않고, 사용자의 기존 전역 Claude / Codex / OpenCode 설정을 그대로 사용합니다.

## 1.2 Agent Environment Handoff Tool

`agentmod`는 프로젝트별 에이전트 환경을 snapshot으로 패킹하고, 다른 컴퓨터에서 복원할 수 있어야 합니다.

소스코드는 Git으로 이동한다고 가정합니다.
`agentmod`는 소스코드 전체를 패킹하는 도구가 아니라, 에이전트 환경과 작업 컨텍스트를 Handoff하는 도구입니다.

---

# 2. 최상위 원칙

다음 원칙은 절대 깨지면 안 됩니다.

1. `agentmod`는 Docker 기반 샌드박스가 아니다.
2. `agentmod`는 shim 방식으로 `claude`, `codex`, `opencode` 명령어를 가로채지 않는다.
3. `agentmod`는 기본 모드에서 `HOME`을 변경하지 않는다.
4. 사용자는 AgentMod 프로젝트 안에서도 기존 명령어인 `claude`, `codex`, `opencode`를 그대로 사용한다.
5. 사용자는 `agentmod run claude`, `agentmod run codex`, `agentmod run opencode` 같은 별도 실행 명령을 사용하지 않는다.
6. `agentmod setup-shell` 같은 별도 명령을 사용자에게 요구하지 않는다.
7. `agentmod init` 하나로 프로젝트 초기화와 shell auto-env hook 설치를 처리한다.
8. `.agentmod/agentmod.toml`이 있는 프로젝트에서만 agentmod가 활성화된다.
9. 프로젝트 밖으로 나가면 agentmod 관련 환경변수는 반드시 해제되어야 한다.
10. 사용자 전역 Claude / Codex / OpenCode 설정을 수정하지 않는다.
11. 프로젝트 간 설정, 스킬, 플러그인, 세션, 캐시가 누수되면 안 된다.
12. `gstack`처럼 `~/.claude/skills`에 직접 설치하는 도구도 프로젝트 내부로 격리해야 한다.
13. Handoff는 기본적으로 소스코드 전체를 포함하지 않는다.
14. Handoff는 기본적으로 secrets, auth, token, credentials를 포함하지 않는다.
15. Git Handoff는 기본적으로 세션과 로그를 포함하지 않는다.
16. Git Handoff에 세션을 포함하려면 암호화가 필수다.
17. Restore는 외부 snapshot을 신뢰하지 않고 안전하게 검증해야 한다.
18. 테스트 통과 전 완료를 선언하지 않는다.

---

# 3. 개발 언어와 아키텍처 방향

`agentmod`는 **Go**로 개발합니다.

Go를 선택하는 이유는 다음과 같습니다.

* 단일 바이너리 배포가 가능하다.
* macOS / Linux / Windows 크로스플랫폼 지원이 쉽다.
* 파일 시스템, 경로 처리, zip, checksum, shell hook 생성에 적합하다.
* CLI 도구 개발과 오픈소스 배포에 적합하다.
* Node / Python 런타임 의존성 없이 설치할 수 있다.

Go가 특정 기능에 부적합하다고 판단되면 반드시 `DECISIONS.md`에 이유를 기록하고 대안을 제시해야 합니다.
임의로 언어를 변경하지 않습니다.

권장 아키텍처 책임은 다음과 같습니다.

* CLI 계층
* Project discovery 계층
* Config / manifest 계층
* Shell hook 계층
* Environment routing 계층
* Doctor / status 진단 계층
* Installer 계층
* Guardrail 계층
* Handoff / snapshot 계층
* Git state 계층
* Redaction / security 계층
* Portability 계층
* Test harness 계층

세부 패키지 구조, 함수명, 내부 구현 방식은 에이전트가 설계합니다.
단, 위 책임들은 분리되어야 하며 테스트 가능해야 합니다.

---

# 4. 먼저 해야 할 일

바로 AgentMod 본 구현을 시작하지 마세요.

먼저 다음을 수행합니다.

1. 저장소 현재 상태를 확인한다.
2. 요구사항을 검토한다.
3. 모순, 위험, 불확실성을 찾는다.
4. `IMPLEMENTATION_PLAN.md`를 작성한다.
5. Ralph Loop Harness scaffold를 생성한다.
6. Harness 문서를 작성한다.
7. Claude Code hook guardrail 설계를 문서화한다.
8. 필요한 skill을 프로젝트 로컬 영역에서만 활성화할 계획을 세운다.
9. 테스트 매트릭스를 만든다.
10. Ralph Loop가 반복 실행 가능한 `PROMPT.md`를 만든다.
11. 그 다음 AgentMod 본 구현을 시작한다.

`IMPLEMENTATION_PLAN.md`에는 반드시 다음이 포함되어야 합니다.

* 요구사항 재정리
* 구현 가능 여부 검증
* 아키텍처 결정
* 위험 요소
* CLI 명령어 설계
* 프로젝트 디렉터리 구조
* shell hook 전략
* Claude / Codex / OpenCode 라우팅 전략
* gstack 격리 전략
* Claude Code guard hook 전략
* Handoff / Snapshot 전략
* Git Handoff 전략
* Windows / macOS / Linux 이식성 전략
* 테스트 전략
* 구현 순서
* 완료 기준

---

# 5. Ralph Loop Harness Scaffold

AgentMod 본 구현 전에 다음 개발 하네스 구조를 생성합니다.

```txt
.harness/
  GOAL.md
  PROMPT.md
  STATE.md
  PLAN.md
  TASKS.md
  DECISIONS.md
  RISKS.md
  CHECKS.md
  TEST_MATRIX.md
  LOOP.md
  DONE.md
  hooks/
  reports/
  skills/
```

각 파일의 책임은 다음과 같습니다.

## GOAL.md

AgentMod의 최종 목표와 완료 조건을 정의합니다.

## PROMPT.md

Ralph Loop에서 반복 입력될 현재 작업 프롬프트입니다.
세션이 끊겨도 다음 반복자가 이어갈 수 있어야 합니다.

## STATE.md

현재 구현 상태, 실패 중인 테스트, 남은 작업, 주의사항을 기록합니다.

## PLAN.md

전체 개발 계획과 단계별 목표를 기록합니다.

## TASKS.md

구현 작업을 작은 단위로 나눈 체크리스트입니다.

## DECISIONS.md

중요한 설계 결정과 변경 이유를 기록합니다.

## RISKS.md

보안, 경로 이식성, 전역 오염, Handoff, secrets, restore 위험을 기록합니다.

## CHECKS.md

매 반복마다 반드시 확인해야 하는 검증 목록입니다.

## TEST_MATRIX.md

기능별 테스트 범위와 완료 기준을 정의합니다.

## LOOP.md

Ralph Loop 운영 규칙을 정의합니다.

## DONE.md

최종 완료 판정과 결과 요약을 기록합니다.

---

# 6. Ralph Loop 개발 방식

이 프로젝트는 다음 개념의 Ralph Loop로 진행합니다.

```txt
while :; do
  cat .harness/PROMPT.md | claude
done
```

단, 실제 실행 방식은 현재 환경에 맞게 검증하고 조정합니다.

중요한 것은 무한 반복 자체가 아니라 다음 원칙입니다.

* 각 반복은 독립 세션처럼 동작할 수 있다.
* 에이전트는 이전 대화 기억에 의존하지 않는다.
* 모든 상태는 파일에 기록한다.
* 각 반복은 가장 작고 중요한 다음 작업 하나를 선택한다.
* 각 반복은 테스트 또는 검증을 실행한다.
* 실패하면 원인을 기록하고 다음 반복으로 넘긴다.
* 완료 조건을 만족하지 않으면 완료 선언을 하지 않는다.
* 완료 조건을 만족하면 `DONE.md`와 최종 리포트를 작성한다.

각 반복의 기본 순서는 다음과 같습니다.

1. `GOAL.md` 읽기
2. `STATE.md` 읽기
3. `CHECKS.md` 읽기
4. `TEST_MATRIX.md` 읽기
5. 현재 실패 상태 확인
6. 가장 중요한 다음 작업 선택
7. 필요한 파일만 수정
8. 테스트 실행
9. 결과 기록
10. `STATE.md` 갱신
11. `PROMPT.md` 갱신
12. 완료 여부 판단

---

# 7. Claude Code Hook Guardrail

하네스는 Claude Code hook을 사용해 guardrail을 구성해야 합니다.

hook은 프로젝트 로컬 설정으로 관리합니다.
전역 Claude 설정을 오염시키지 않습니다.

Guardrail의 목적은 다음입니다.

* 위험한 명령 차단
* 전역 설정 오염 방지
* HOME 변경 방지
* shim 생성 방지
* secrets 유출 방지
* snapshot / restore 보안 위반 방지
* 테스트 실행 누락 방지
* 완료 조건 미충족 상태에서 완료 선언 방지

Guardrail은 최소한 다음 위험을 감지해야 합니다.

* `HOME` 변경 시도
* shim 생성 시도
* 전역 `~/.claude`, `~/.codex`, `~/.config/opencode` 직접 수정 시도
* `sudo` 사용 시도
* `.ssh`, `.aws`, `.docker`, `.git` 직접 수정 시도
* secrets를 snapshot 또는 Git Handoff에 포함하려는 시도
* 소스코드 전체를 Handoff 패키지에 포함하려는 시도
* zip-slip에 취약한 restore 로직
* 테스트 없이 완료 처리하려는 시도

Hook의 세부 구현 방식은 에이전트가 현재 Claude Code 환경에서 검증해 결정합니다.
불확실한 hook input format은 추측하지 말고, 검증 결과를 `DECISIONS.md`에 기록합니다.

---

# 8. 사용할 Skill

다음 skill repository를 프로젝트 로컬 영역에서만 활성화합니다.

* https://github.com/mattpocock/skills
* https://github.com/multica-ai/andrej-karpathy-skills

원칙:

* 전역 설치하지 않는다.
* 프로젝트 로컬 Claude Code 영역에서만 활성화한다.
* 필요한 skill만 선택적으로 활성화한다.
* skill 설치와 활성화는 하네스의 관리 대상이다.
* 설치 결과와 사용 이유를 `.harness/skills/README.md`와 `DECISIONS.md`에 기록한다.
* 추가 skill이 필요하면 먼저 이유를 기록하고 프로젝트 로컬 영역에만 설치한다.

필수 사고 원칙:

* Think Before Coding
* Simplicity First
* Surgical Changes
* Goal-Driven Execution
* 테스트 기반 완료 판정
* 불필요한 추상화 금지
* 구현 전 실패 조건 명확화
* 작은 변경 단위 유지

---

# 9. 제품 구조와 기본 파일

제품명은 `agentmod`입니다.

CLI 명령어는 다음입니다.

```txt
agentmod
```

프로젝트 로컬 디렉터리는 다음입니다.

```txt
.agentmod/
```

설정 파일은 다음입니다.

```txt
.agentmod/agentmod.toml
```

Git 저장용 Handoff 디렉터리는 다음입니다.

```txt
.agentmod-handoff/
```

`agentmod init` 후 `.agentmod/`에는 최소한 다음 책임 영역이 있어야 합니다.

* agentmod 설정
* Claude Code 로컬 홈
* Codex CLI 로컬 홈
* OpenCode 로컬 설정 영역
* npm / pnpm / bun 로컬 cache / prefix
* snapshot 저장 영역
* hooks / sessions / logs / skills / plugins / commands / agents 관련 영역

세부 디렉터리 구조는 에이전트가 설계하되, 다음 요구를 만족해야 합니다.

* Claude 관련 설정·스킬·플러그인·세션은 프로젝트 내부에 있어야 한다.
* Codex 관련 설정·스킬·세션은 프로젝트 내부에 있어야 한다.
* OpenCode 관련 설정·플러그인·명령·모드는 프로젝트 내부에 있어야 한다.
* `.agentmod/`는 기본적으로 Git에 커밋하지 않는다.
* `.agentmod-handoff/`는 Git 저장용 안전 패키지 영역으로 사용할 수 있어야 한다.

---

# 10. 필수 CLI 명령어

## Core

* `agentmod init`
* `agentmod doctor`
* `agentmod status`

## Shell / Guard

* `agentmod hook zsh`
* `agentmod hook bash`
* `agentmod guard claude-bash`

## Installer

* `agentmod install gstack`

## Handoff

* `agentmod handoff create`
* `agentmod handoff restore`
* `agentmod handoff inspect`
* `agentmod handoff verify`
* `agentmod handoff list`

## Alias

* `agentmod pack`
* `agentmod unpack`

선택 명령은 필요하면 에이전트가 제안할 수 있지만, MVP 범위를 흐리지 않아야 합니다.

---

# 11. agentmod init 요구사항

`agentmod init`은 다음을 수행해야 합니다.

* `.agentmod/` 생성
* `.agentmod/agentmod.toml` 생성
* Claude / Codex / OpenCode 프로젝트 로컬 홈 생성
* Node 계열 로컬 prefix/cache 디렉터리 생성
* snapshot 디렉터리 생성
* `.gitignore`에 `.agentmod/` 추가
* shell auto-env hook 설치 여부 확인
* hook이 없으면 shell rc 파일에 agentmod 블록 추가
* 현재 shell에서 hook 활성 여부 진단
* 현재 shell에 즉시 적용되지 않는 경우 정확한 안내 출력
* `agentmod doctor` 수준의 진단 요약 출력

`agentmod init`은 idempotent해야 합니다.

다음 문제가 생기면 안 됩니다.

* shell rc hook 중복 삽입
* `.gitignore` 중복 삽입
* 기존 설정 무분별한 덮어쓰기
* 기존 디렉터리 삭제
* 사용자 전역 설정 수정

일반 CLI 프로세스는 부모 shell의 환경변수를 직접 바꿀 수 없습니다.
따라서 최초 `agentmod init` 직후 현재 터미널 세션에 바로 적용되지 않을 수 있습니다.
이 한계를 숨기지 말고 명확히 안내해야 합니다.

---

# 12. agentmod.toml 요구사항

`.agentmod/agentmod.toml`은 다음 책임을 표현할 수 있어야 합니다.

* schema version
* mode
* isolation policy
* Claude Code routing 설정
* Codex CLI routing 설정
* OpenCode routing 설정
* Node / npm / pnpm / bun local cache/prefix 설정
* gstack installer 설정
* snapshot 기본 정책
* handoff 기본 정책
* Git Handoff 기본 정책
* secrets 포함 금지 정책
* HOME 변경 금지 정책
* global write 차단 정책

필수 기본값:

* `change_home`은 false여야 한다.
* global agent write 방지는 기본 활성화되어야 한다.
* Claude Bash guard는 기본 활성화되어야 한다.
* snapshot은 기본적으로 source code를 포함하지 않아야 한다.
* snapshot은 기본적으로 secrets를 포함하지 않아야 한다.
* Git Handoff는 기본적으로 sessions/logs를 포함하지 않아야 한다.
* Git Handoff에서 sessions를 포함하려면 encryption이 필요해야 한다.

세부 TOML 스키마와 필드명은 에이전트가 설계하되, 위 정책을 표현할 수 있어야 합니다.

---

# 13. Shell Auto-Env 요구사항

agentmod는 shim을 사용하지 않고 shell hook을 사용합니다.

지원 우선순위:

1. zsh
2. bash
3. fish는 선택
4. PowerShell은 향후 확장 또는 restore 호환성 중심으로 고려

shell hook의 책임:

* 현재 디렉터리부터 상위 디렉터리까지 `.agentmod/agentmod.toml` 탐색
* 가장 가까운 agentmod 프로젝트 활성화
* agentmod 프로젝트 안에서는 필요한 환경변수 설정
* agentmod 프로젝트 밖에서는 관련 환경변수 해제
* 프로젝트 간 이동 시 이전 환경 누수 방지
* PATH 중복 추가 방지
* HOME 변경 금지

agentmod 프로젝트 안에서는 최소한 다음 라우팅이 가능해야 합니다.

* Claude Code project-local home
* Codex CLI project-local home
* OpenCode project-local config
* Node / npm / pnpm / bun project-local cache/prefix

환경변수 이름은 명확하고 일관되게 사용합니다.
agentmod 자체 환경변수는 대문자 네이밍을 우선합니다.

예:

* `AGENTMOD_ACTIVE`
* `AGENTMOD_PROJECT_ROOT`
* `AGENTMOD_ROOT`

정확한 환경변수명과 동작은 구현 시 검증하고 문서화합니다.

---

# 14. Claude / Codex / OpenCode 격리 요구사항

## Claude Code

* 프로젝트 안에서는 전역 `~/.claude`가 아니라 `.agentmod/claude`를 사용해야 한다.
* 프로젝트 밖에서는 기존 전역 Claude 설정을 사용해야 한다.
* 프로젝트 안에서 전역 Claude home에 직접 쓰는 명령은 guardrail로 차단해야 한다.
* gstack은 `.agentmod/claude/skills/gstack`에만 설치되어야 한다.
* Claude Code hook input format과 settings 구조는 실제로 검증해야 한다.

## Codex CLI

* 프로젝트 안에서는 `.agentmod/codex`를 Codex home으로 사용해야 한다.
* 프로젝트 밖에서는 기존 전역 Codex 설정을 사용해야 한다.
* 인증과 provider 설정은 안전하게 진단하고, 전역 설정을 무단 복사하지 않는다.
* Codex의 현재 local home 동작은 실제로 검증해야 한다.

## OpenCode

* 프로젝트 안에서는 `.agentmod/opencode`를 프로젝트 로컬 설정 영역으로 사용해야 한다.
* 프로젝트 밖에서는 기존 전역 OpenCode 설정을 사용해야 한다.
* OpenCode가 전역 plugin도 함께 읽는 경우 doctor에서 경고해야 한다.
* OpenCode의 local config 동작은 실제로 검증해야 한다.

---

# 15. gstack 격리 요구사항

gstack은 일반적으로 `~/.claude/skills`에 직접 설치될 수 있으므로 agentmod에서 특별 관리해야 합니다.

요구사항:

* `agentmod install gstack` 명령을 제공한다.
* gstack은 현재 agentmod 프로젝트 내부에만 설치한다.
* 전역 `~/.claude/skills/gstack`에는 설치하지 않는다.
* 설치 전후에 전역 오염 가능성을 검사한다.
* gstack setup 과정이 전역 Claude home에 쓰려고 하면 중단한다.
* Claude Code Bash guard가 전역 Claude home 직접 쓰기를 차단해야 한다.
* gstack 설치 상태는 doctor에서 확인할 수 있어야 한다.
* agentmod 프로젝트가 아닌 곳에서 `agentmod install gstack`을 실행하면 실패해야 한다.
* 이미 설치되어 있으면 안전하게 중단하거나 명시적 force 옵션을 요구해야 한다.
* 네트워크 실패, git 부재, setup 실패를 명확히 안내해야 한다.

세부 clone 방식, setup 검사 방식, 실패 처리 방식은 에이전트가 설계하되, 전역 오염은 허용하지 않습니다.

---

# 16. Claude Code guard hook 요구사항

agentmod 프로젝트 안에서는 Claude Code가 Bash tool로 전역 agent home을 오염시키는 명령을 실행하지 못하게 해야 합니다.

Guard 대상:

* 전역 `~/.claude/skills`
* 전역 `~/.claude/plugins`
* `$HOME/.claude` 하위 쓰기
* OS별 사용자 홈 아래 Claude 전역 경로
* 기타 전역 agent home 오염 위험 경로

차단해야 하는 행위:

* 전역 Claude home으로 clone / copy / move / write
* 전역 Claude home 하위 디렉터리 생성
* 전역 Claude home 하위 파일 삭제
* 전역 plugin / skill 직접 설치

주의:

* 읽기 명령까지 무조건 차단하면 안 된다.
* 쓰기 가능성이 큰 명령만 차단한다.
* hook input format은 실제로 확인한다.
* 포맷을 확신하지 못하면 방어적으로 처리한다.
* guard 자체가 실패하면 안전한 방향으로 동작해야 한다.

---

# 17. Handoff 기능 요구사항

agentmod는 프로젝트별 에이전트 환경을 snapshot으로 패킹하고 복원할 수 있어야 합니다.

## Handoff create

포함 대상:

* agentmod 설정
* Claude 설정, skills, plugins, agents, commands, hooks
* Codex 설정, skills, sessions
* OpenCode 설정, plugins, agents, commands, modes
* MCP 설정
* 작업 context / memory / handoff 문서
* Git 상태 메타데이터
* 일반 Handoff에서는 세션 포함 가능

기본 제외 대상:

* 소스코드 전체
* `.git`
* `node_modules`
* 빌드 산출물
* cache
* tmp
* auth
* credentials
* tokens
* `.env`
* SSH / cloud credentials
* OS credential store

Handoff 생성 시 다음을 만들어야 합니다.

* `.amod` 패키지
* manifest
* inventory
* checksums
* redaction report
* 사람이 읽을 수 있는 HANDOFF 문서

## Handoff restore

복원은 안전해야 합니다.

요구사항:

* snapshot 검증
* checksum 검증
* schema version 확인
* zip-slip 방지
* 절대경로 복원 금지
* 기존 `.agentmod` 백업
* Git remote / branch / HEAD 비교
* secrets 제외 항목 안내
* OS별 경로 이식성 처리
* MCP 절대경로 경고 또는 재작성
* 복원 후 doctor 실행
* 임의 스크립트 자동 실행 금지

## Handoff inspect / verify / list

사용자가 Handoff 패키지 내용을 열어보거나 검증하거나 목록화할 수 있어야 합니다.

---

# 18. Git Handoff 요구사항

agentmod는 Git에 저장 가능한 안전한 Handoff 모드를 제공해야 합니다.

원칙:

* `.agentmod/`는 Git에 올리지 않는다.
* Git 저장용 결과물은 `.agentmod-handoff/`에 둔다.
* Git Handoff는 기본적으로 secrets를 제외한다.
* Git Handoff는 기본적으로 sessions/logs를 제외한다.
* Git Handoff는 소스코드 전체를 포함하지 않는다.
* Git Handoff는 사람이 읽을 수 있는 HANDOFF 문서를 포함한다.
* Git Handoff는 manifest와 inventory를 포함한다.

필수 명령:

* `agentmod handoff create --for-git`
* `agentmod pack --for-git`

세션을 Git Handoff에 포함하려면 암호화가 필수입니다.

MVP에서 암호화를 구현하지 않는 경우:

* `--for-git --include-sessions`는 실패해야 한다.
* 실패 메시지에서 암호화가 필요한 이유를 설명해야 한다.

---

# 19. Git 상태 요구사항

Handoff 생성 시 Git 상태를 확인해야 합니다.

기본 정책:

* Git worktree가 dirty이면 경고한다.
* 소스코드 변경분은 기본 포함하지 않는다.
* 사용자가 명시적으로 허용한 경우에만 dirty 상태로 진행한다.
* patch 포함은 명시적 옵션일 때만 허용한다.
* remote URL에 token이 있으면 redaction한다.

Git metadata에는 최소한 다음을 기록합니다.

* repository 여부
* remote URL sanitized
* branch
* HEAD commit
* dirty 여부
* staged / modified / untracked 요약
* source code included 여부

---

# 20. Snapshot 포맷 요구사항

Snapshot 확장자는 `.amod`로 합니다.

내부 포맷은 zip 계열을 우선합니다.

Snapshot은 다음을 포함해야 합니다.

* manifest
* inventory
* checksums
* redaction report
* HANDOFF 문서
* RESTORE 문서
* payload

중요:

* 외부 snapshot은 신뢰하지 않는다.
* path traversal을 허용하지 않는다.
* 절대경로 복원을 허용하지 않는다.
* `.ssh`, `.aws`, `.docker`, `.git`에는 쓰지 않는다.
* restore는 기본적으로 `.agentmod/` 아래에만 복원한다.

세부 JSON 스키마와 파일 구조는 에이전트가 설계하되, manifest / inventory / checksum / HANDOFF 문서는 반드시 존재해야 합니다.

---

# 21. 경로 이식성 요구사항

AgentMod Handoff는 Windows / macOS / Linux 간 이동을 고려해야 합니다.

요구사항:

* snapshot 내부 경로는 portable하게 정규화한다.
* OS별 path separator 차이를 고려한다.
* 절대경로는 restore 시 재작성하거나 경고한다.
* symlink는 안전하게 처리한다.
* 실행 권한은 가능한 범위에서 복원한다.
* PowerShell 지원은 MVP 이후로 미뤄도 되지만, restore 포맷은 Windows를 깨뜨리지 않도록 설계한다.

---

# 22. agentmod doctor 요구사항

`agentmod doctor`는 현재 상태를 진단해야 합니다.

진단 범위:

* 프로젝트 발견 여부
* agentmod root
* shell 종류
* shell hook 설치 여부
* shell hook 활성 여부
* 현재 환경변수 상태
* HOME 변경 여부
* shim 존재 여부
* Claude / Codex / OpenCode 도구 존재 여부
* Claude project-local home 상태
* Codex project-local home 상태
* OpenCode project-local config 상태
* global Claude write guard 상태
* gstack project-local 설치 여부
* gstack global 설치 위험 여부
* snapshot 디렉터리 상태
* 최근 Handoff 상태
* Git 상태
* portability 위험
* secrets 위험
* MCP 경고

doctor는 다음 상황을 반드시 경고해야 합니다.

* agentmod 프로젝트 안인데 필요한 환경변수가 설정되어 있지 않음
* `.agentmod` 없는 폴더인데 agentmod 환경변수가 남아 있음
* HOME이 변경되어 있음
* shim이 감지됨
* 전역 `~/.claude/skills/gstack`이 존재함
* shell hook이 설치되어 있지만 현재 shell에서 비활성
* PATH에 agentmod 경로가 중복 추가됨
* Git Handoff인데 세션이 암호화 없이 포함됨
* snapshot에 secrets 후보가 포함됨
* restore 대상 Git HEAD가 snapshot과 다름

---

# 23. agentmod status 요구사항

`agentmod status`는 현재 AgentMod 활성 여부를 간단히 보여줘야 합니다.

활성 상태에서는 다음을 보여줍니다.

* project root
* agentmod root
* Claude local home
* Codex local home
* OpenCode local config
* 최근 Handoff 정보

비활성 상태에서는 다음을 보여줍니다.

* AgentMod inactive
* `.agentmod/agentmod.toml`이 현재 디렉터리 또는 상위 디렉터리에 없음
* 기본 전역 agent 설정이 사용될 것임

---

# 24. 보안 요구사항

다음은 반드시 지킵니다.

* 사용자 홈의 기존 설정 파일을 덮어쓰지 않는다.
* shell rc 수정 시 agentmod 블록만 추가/갱신한다.
* 사용자가 작성한 shell 설정을 삭제하지 않는다.
* `.agentmod/`는 기본 gitignore 처리한다.
* HOME 변경 금지
* shim 생성 금지
* sudo 사용 금지
* 전역 npm/brew 설정 변경 금지
* 전역 Claude/Codex/OpenCode 설정 수정 금지
* gstack 설치 시 전역 Claude home 오염 금지
* Snapshot은 secrets/auth 기본 제외
* Git Handoff는 sessions/logs 기본 제외
* Restore 시 zip-slip 방지
* Restore 시 임의 스크립트 자동 실행 금지
* Restore 전 기존 `.agentmod` 백업
* 외부 snapshot 신뢰 금지

---

# 25. 테스트 전략

테스트는 완료 판정의 기준입니다.

하네스는 다음 테스트 카테고리를 정의하고, AgentMod 구현은 이를 통과해야 합니다.

* 프로젝트 탐색 테스트
* 환경변수 생성/해제 테스트
* shell hook 테스트
* init idempotency 테스트
* `.gitignore` 중복 방지 테스트
* HOME 변경 방지 테스트
* shim 생성 방지 테스트
* Claude local home routing 테스트
* Codex local home routing 테스트
* OpenCode local config routing 테스트
* gstack 설치 경로 테스트
* global Claude write guard 테스트
* Handoff create 테스트
* Handoff inspect 테스트
* Handoff verify 테스트
* Handoff restore 테스트
* Git Handoff 테스트
* secrets exclusion 테스트
* source exclusion 테스트
* zip-slip 방지 테스트
* restore backup 테스트
* path portability 테스트
* doctor 진단 테스트

실제 Claude / Codex / OpenCode가 없어도 mock binary나 fixture를 사용해 테스트할 수 있어야 합니다.

---

# 26. 필수 사용자 시나리오

다음 시나리오가 반드시 통과해야 합니다.

## 26.1 proj00: 기본 전역 Claude 사용

`.agentmod`가 없는 폴더에서 `claude`를 실행하면 기본 전역 Claude 설정이 사용되어야 합니다.

이 상태에서 전역 superpowers 플러그인을 설치했다면, `.agentmod`가 없는 일반 폴더에서는 계속 활성화되어야 합니다.

## 26.2 proj01: agentmod 프로젝트 생성

`agentmod init`으로 프로젝트를 생성한 뒤 `claude`를 실행하면 프로젝트 로컬 Claude home이 사용되어야 합니다.

proj00에서 전역 설치한 superpowers는 proj01에서 보이면 안 됩니다.

## 26.3 proj01: gstack 프로젝트 격리 설치

`agentmod install gstack`으로 설치한 gstack은 proj01 내부 `.agentmod/claude/skills/gstack`에만 존재해야 합니다.

전역 `~/.claude/skills/gstack`에는 설치되면 안 됩니다.

## 26.4 proj02: 일반 프로젝트

`.agentmod`가 없는 proj02에서 `claude`를 실행하면 기본 전역 Claude 설정을 사용해야 합니다.

proj01의 gstack은 보이면 안 됩니다.
proj00의 전역 superpowers는 계속 보여야 합니다.

## 26.5 컴퓨터 A → 컴퓨터 B Handoff

컴퓨터 A에서 agentmod 환경을 Handoff 패키지로 생성하고, 컴퓨터 B에서 같은 Git checkout 위에 복원할 수 있어야 합니다.

복원 후 Claude / Codex / OpenCode 설정, gstack, MCP 설정, 컨텍스트가 가능한 범위에서 이어져야 합니다.

secrets와 auth는 기본 제외되며, 필요한 경우 재로그인 안내가 있어야 합니다.

## 26.6 Git Handoff

`agentmod handoff create --for-git` 또는 `agentmod pack --for-git`으로 Git에 저장 가능한 Handoff 패키지를 생성해야 합니다.

생성 결과는 `.agentmod-handoff/`에 위치해야 합니다.

Git Handoff는 기본적으로 소스코드, secrets, auth, sessions, logs를 포함하지 않아야 합니다.

---

# 27. 완료 선언 금지 조건

다음 중 하나라도 해당하면 완료를 선언하지 않습니다.

* 테스트가 없다.
* 테스트를 실행하지 않았다.
* 실패 테스트가 남아 있다.
* `agentmod init`이 idempotent하지 않다.
* shim을 생성한다.
* HOME을 변경한다.
* `.agentmod` 없는 폴더에서 agentmod 환경변수가 남는다.
* `.agentmod` 있는 폴더에서 필요한 환경변수가 설정되지 않는다.
* 프로젝트 간 환경이 누수된다.
* gstack이 전역 `~/.claude`에 설치될 수 있다.
* Handoff가 소스코드를 기본 포함한다.
* Git Handoff가 세션을 암호화 없이 포함한다.
* restore가 zip-slip에 취약하다.
* restore가 기존 `.agentmod`를 백업하지 않는다.
* README에 한계가 명확히 적혀 있지 않다.
* 오픈소스 배포 문서가 없다.
* `.harness/STATE.md`, `.harness/DONE.md`, `.harness/TEST_MATRIX.md`가 갱신되지 않았다.

---

# 28. 최종 완료 조건

다음 조건을 모두 만족해야 합니다.

## Core

* AgentMod는 Go로 구현된다.
* `agentmod init`이 동작한다.
* `agentmod doctor`가 동작한다.
* `agentmod status`가 동작한다.
* `.agentmod/` 구조가 생성된다.
* `.agentmod/agentmod.toml`이 생성된다.
* `.gitignore`가 안전하게 갱신된다.
* init은 idempotent하다.

## Shell Auto-Env

* zsh hook 지원
* bash hook 지원
* `.agentmod` 프로젝트 안에서만 활성화
* 프로젝트 밖에서 환경변수 해제
* PATH 중복 방지
* HOME 변경 없음
* shim 없음

## Agent Routing

* Claude Code project-local home routing
* Codex CLI project-local home routing
* OpenCode project-local config routing
* 전역 설정 오염 방지
* gstack project-local install
* Claude Bash guard

## Handoff

* Handoff create
* Handoff restore
* Handoff inspect
* Handoff verify
* Handoff list
* pack / unpack alias
* `.amod` 패키지 생성
* manifest / inventory / checksums / HANDOFF 문서 생성
* secrets 기본 제외
* source code 기본 제외
* restore 전 백업
* restore 후 doctor 실행
* zip-slip 방지

## Git Handoff

* `.agentmod-handoff/` 생성
* `agentmod handoff create --for-git`
* `agentmod pack --for-git`
* sessions/logs 기본 제외
* secrets/auth 기본 제외
* source code 기본 제외
* session 포함 시 encryption 필수 정책

## Quality

* 테스트 존재
* 핵심 시나리오 테스트 존재
* README 존재
* LICENSE 존재
* SECURITY.md 존재
* CONTRIBUTING.md 존재
* CHANGELOG.md 존재
* IMPLEMENTATION_PLAN.md 존재
* `.harness/` 존재
* 최종 리포트 존재

---

# 29. 오픈소스 배포 문서

다음 파일을 준비합니다.

* README.md
* LICENSE
* CONTRIBUTING.md
* CHANGELOG.md
* SECURITY.md
* CODE_OF_CONDUCT.md

README에는 반드시 다음을 설명합니다.

* agentmod가 무엇인지
* agentmod가 아닌 것

  * Docker sandbox 아님
  * 보안 완전 격리 아님
  * shim 아님
  * HOME 변경 도구 아님
  * 소스코드 백업 도구 아님
* 빠른 시작
* `agentmod init`
* 기본 `claude`, `codex`, `opencode` 명령어 사용 방식
* gstack 설치 방식
* Handoff 사용법
* Git Handoff 사용법
* 보안 주의사항
* secrets 제외 정책
* restore 주의사항
* doctor 사용법
* FAQ

---

# 30. 중요한 구현 태도

모르는 내용을 추측해서 구현하지 않습니다.

특히 다음은 실제 동작을 확인하면서 구현합니다.

* Claude Code hook input format
* Claude Code settings 구조
* Codex의 현재 local home 동작
* OpenCode의 현재 local config 동작
* gstack setup 동작
* 각 도구의 session/history 저장 위치
* MCP 설정 저장 위치

불확실하면 다음 원칙을 따릅니다.

1. 먼저 검증한다.
2. 검증 결과를 `IMPLEMENTATION_PLAN.md` 또는 `DECISIONS.md`에 기록한다.
3. 실패 가능성이 있으면 `agentmod doctor`에서 경고한다.
4. 전역 설정을 오염시킬 가능성이 있으면 실행하지 않는다.
5. secrets 유출 가능성이 있으면 기본 제외한다.
6. 외부 snapshot 복원 시 안전을 우선한다.

---

# 31. 최우선 판단 기준

편의성보다 안전성.
속도보다 검증성.
큰 구현보다 작은 반복.
세션 기억보다 파일 상태.
완료 선언보다 테스트 통과.
전역 오염 없는 프로젝트 단위 격리.
Git 기반 소스 이동 + AgentMod 기반 에이전트 환경 Handoff.

이 기준을 어기면 구현을 중단하고 설계를 수정합니다.


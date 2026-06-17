# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Isolamento por projeto e entrega de contexto para agentes de codificação.

`agentmod` mantém a configuração, skills, plugins, sessões, caches e
contexto de trabalho do **Claude Code**, **Codex CLI**, e **OpenCode** dentro do
projeto em que você está trabalhando — e empacota esse ambiente em um snapshot que
você pode entregar a outra máquina.

Ele desempenha dois papéis:

1. **Agent Home Router.** Dentro de uma árvore de diretórios contendo
   `.agentmod/agentmod.toml`, um gancho de shell roteia o home de cada agente para
   `.agentmod/`. Fora, todas as variáveis são restauradas exatamente como estavam e
   sua configuração global permanece intacta.
2. **Ferramenta de Entrega.** `agentmod handoff create` empacota `.agentmod/` em um
   snapshot `.amod` verificável (ou, com `--for-git`, uma árvore de arquivos commitável
   em `.agentmod-handoff/`). **Git move seu código; agentmod move o
   ambiente do agente.**

## O que agentmod *não* é

- **Não é uma sandbox Docker.** Roteia variáveis de ambiente em seu próprio
  shell. Não há container, VM ou filtragem de syscall.
- **Não é isolamento de segurança completo.** Uma ferramenta que ignora as variáveis
  roteadas ainda pode acessar seus homes globais. O guarda Bash do Claude (abaixo) é
  defesa em profundidade, não um limite de segurança.
- **Não é um shim.** Nunca intercepta ou encapsula os comandos `claude`, `codex` ou
  `opencode`. Você continua executando-os diretamente, sem modificação.
- **Não é uma ferramenta de mudança de HOME.** `HOME` nunca é reatribuído.
- **Não é uma ferramenta de backup de código-fonte.** Os snapshots nunca incluem seu código-fonte
  por padrão. Use git para código-fonte.

## Como funciona

`agentmod hook zsh` / `agentmod hook bash` imprimem uma pequena função de shell
auto-contida (instalada em seu arquivo rc pelo `agentmod init`). A cada prompt
e mudança de diretório, ela caminha para cima procurando por `.agentmod/agentmod.toml`:

- **Entrando em um projeto** salva os valores atuais e define:

  | Variável | Roteado para |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **apenas** com `opencode.xdg_full_isolation = true` |

  `PATH` ganha exatamente uma entrada, `.agentmod/node/bin` (bin global do npm
  sob o prefixo roteado). Variáveis de manutenção (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) registram o que desfazer.

- **Saindo do projeto** restaura cada valor salvo e remove a entrada `PATH` — um
  inverso perfeito. Alternar diretamente entre dois projetos agentmod roteia em uma
  única etapa sem vazar os caminhos de nenhum projeto.

O roteamento por agente pode ser desligado em `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Instalação

Escolha o que se adequa ao seu setup — cada um instala o mesmo binário único:

```sh
# npm (instala o binário pré-compilado para sua plataforma)
npm install -g agentmod

# script de instalação (baixa a versão correspondente, verifica sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (requer o toolchain Go)
go install github.com/mojomoth/agentmod@latest
```

Ou construa a partir do código-fonte (Go 1.26+, a única dependência de módulo é
`BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# coloque o binário em algum lugar no seu PATH
```

## Início rápido

```sh
cd ~/work/myproject
agentmod init          # cria .agentmod/, edita .gitignore, instala o
                       # gancho de shell em seu arquivo rc, oferece copiar auth
# primeira vez apenas: o gancho ainda não está ativo NESTE shell —
# abra um novo terminal, ou: exec $SHELL

cd ~/work/myproject    # gancho ativa; verifique:
agentmod status        # "AgentMod: active", caminhos roteados listados
claude                 # comando simples — agora usando o home local do projeto
agentmod install gstack   # skills locais do projeto, home global intocado

agentmod pack          # snapshot para .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # diagnóstico somente leitura a qualquer momento
```

Na máquina receptora:

```sh
cd ~/work/myproject    # fonte chegou via git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# siga as notas de re-login impressas; doctor é executado automaticamente
```

## `agentmod init`

Idempotente — re-executar preenche o que estiver faltando e nunca sobrescreve um
`agentmod.toml` existente ou qualquer arquivo do usuário. Ele:

- cria `.agentmod/{claude,codex,opencode,node,snapshots,logs}` e um
  `agentmod.toml` padrão;
- conecta o guarda Bash do Claude em `.agentmod/claude/settings.json`;
- adiciona `.agentmod/` ao `.gitignore` (criado apenas dentro de um repositório git);
- instala o gancho de shell como um bloco protegido em `~/.zshrc` ou `~/.bashrc`
  (seu shell de `$SHELL`; o bloco é atualizado no local, nunca
  duplicado, e seu próprio conteúdo rc nunca é tocado);
- oferece **copiar** arquivos de autenticação Claude/Codex existentes para o
  home local do projeto (veja "Auth" abaixo) — cópia acontece apenas em um `y` explícito.

Flags: `--no-shell-hook` pula todas as edições de arquivo rc; `--yes` /
`--non-interactive` nunca solicita e portanto nunca copia auth (para CI).

## Usando `claude`, `codex`, `opencode` simples

Não há comando wrapper. Dentro de um projeto ativo os comandos ordinários
simplesmente veem os homes roteados:

- **Claude Code** lê `CLAUDE_CONFIG_DIR` → configurações locais do projeto,
  skills/plugins de nível de usuário, sessões, histórico. (`.claude/` de nível de projeto
  é *sempre* lido nativamente — veja Limitações.)
- **Codex CLI** lê `CODEX_HOME` → `config.toml` local do projeto,
  `auth.json`, sessões, histórico, logs.
- **OpenCode** lê `OPENCODE_CONFIG` → o arquivo de configuração local do projeto.
  Este é *isolamento parcial* por padrão — veja Limitações.

### Auth

Os homes locais do projeto novinhos começam sem credenciais:

- **Claude no macOS**: nada a fazer — credenciais vivem no Keychain e
  são compartilhadas com cada diretório de configuração (o que também significa que não são
  isoladas por projeto).
- **Claude no Linux/Windows**: execute `claude login` dentro do projeto, ou
  aceite a oferta do init para copiar `~/.claude/.credentials.json`.
- **Codex**: execute `codex login` dentro do projeto, ou aceite a oferta do init
  para copiar `~/.codex/auth.json`.

Arquivos de auth **nunca viajam em snapshots** (excluídos por nome, independentemente de
como chegaram lá).

## Instalação do gstack

[gstack](https://github.com/garrytan/gstack) codifica seu instalador como
`~/.claude/skills/gstack` — exatamente a poluição global que agentmod existe para
prevenir. Então:

```sh
agentmod install gstack            # clone para .agentmod/claude/skills/gstack
agentmod install gstack --force    # substitua uma instalação local de projeto existente
```

O instalador clona com git, nunca executa o próprio script de configuração do gstack, e
snapshots a listagem de `~/.claude/skills` antes e depois — qualquer mudança para
o diretório global é reportada como violação e falha no comando.
`agentmod doctor` separadamente avisa sempre que uma instalação global de gstack existe
(mesmo aquela que você instalou antes de adotar agentmod), porque skills instaladas globalmente
vazam para cada projeto.

## Entrega (snapshots `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + relatório de redação, sem extração
agentmod handoff verify  FILE      # re-hash cada membro; saída 3 em desajuste
agentmod handoff restore FILE      # substitua .agentmod/ (backup feito primeiro)
agentmod pack / agentmod unpack    # aliases de create / restore
```

Um snapshot é um zip com seis membros raiz — `manifest.json`,
`inventory.json` (tamanho/sha256/modo por arquivo), `REDACTION.md` (o que foi
excluído e por quê, mais descobertas de verificação de segredos), `HANDOFF.md` e `RESTORE.md`
(instruções humanas para o receptor), `checksums.txt`
(`shasum -a 256 -c`-compatível) — e o payload sob
`payload/.agentmod/…`. Criação é atômica e determinística; o manifest
registra branch git/commit/estado sujo com quaisquer credenciais removidas da
URL remota. Uma worktree suja recusa empacotar a menos que `--allow-dirty`.

`inspect` e `verify` funcionam em qualquer lugar — o receptor pode auditar um snapshot
antes de ter qualquer projeto configurado.

### Política de exclusão de segredos

Duas camadas, ambas ativadas por padrão:

1. **Regras de exclusão** descartam arquivos conhecidos como sensíveis do payload e listam
   cada um em `REDACTION.md`: arquivos de auth por nome (`.credentials.json`,
   `auth.json`, `credentials*`), `*.env` / `.env.*`, chaves SSH (`id_*`,
   `*.pem`, `*.pub`), diretórios de credenciais (`.ssh`, `.aws`, `.azure`,
   `.gcloud`, `.kube`, `.gnupg`, `.docker`), arquivos de keychain, `.git`,
   `node_modules`, caches e diretórios temp.
2. **Uma varredura de conteúdo** sobre cada arquivo *mantido*. Material de chave privada recusa
   criação a menos que você passe `--allow-findings` (e é então marcado
   HARD em `REDACTION.md`). Prováveis tokens (IDs de chave de acesso AWS, tokens GitHub,
   chaves `sk-…`, atribuições estilo `api_key=`) são avisados mas
   não bloqueiam.

A varredura é heurística. **Revise `REDACTION.md` (ou `handoff inspect`) antes de
compartilhar um snapshot** — sessões e contexto de trabalho viajam por design e podem
citar qualquer coisa que você colou em uma conversa de agente. Snapshots são escritos
modo 0600 por esta razão; trate-os como arquivos privados.

## Entrega Git

```sh
agentmod pack --for-git    # escreve .agentmod-handoff/ na raiz do projeto
git add .agentmod-handoff && git commit
```

Os mesmos seis membros e payload de um `.amod`, mas como uma árvore commitável de
arquivos simples (`shasum -a 256 -c checksums.txt` funciona no diretório). Em
cima das exclusões padrão, remove **sessões, transcrições, histórico,
e logs** para todos os três agentes — aqueles rotineiramente contêm segredos colados e
não pertencem a um repositório. `--include-sessions` sempre recusa:
committing sessões exigiria criptografia, que esta versão não
implementa. Contexto de trabalho que é seguro compartilhar (CLAUDE.md, configs de agente,
skills, planos) permanece.

Re-executar substitui o pacote anterior; nada mais no repo é
tocado.

## Cautelas de restauração

`handoff restore` / `unpack` trata cada snapshot como entrada não confiável:

- verificação completa de checksum e verificação cruzada de inventário primeiro;
- plano de segurança de caminho: zip-slip (`..`), caminhos absolutos, letras de unidade,
  alvos não `.agentmod`, nomes protegidos (`.git`, `.ssh`, `.aws`,
  `.docker`), e escaping ou alvos de symlink absoluto são todos recusados
  antes de qualquer coisa ser escrita;
- o `.agentmod/` existente é renomeado para `.agentmod.backup-<stamp>` antes da
  extração; qualquer falha reverte para isso automaticamente;
- **nada de um snapshot é nunca executado**;
- depois: o gancho de guarda Claude é re-conectado para o binário *desta* máquina,
  caminhos absolutos específicos da máquina encontrados em configs de agente restaurados são
  avisados (seus arquivos nunca são reescritos), `doctor` é executado inline, e
  os passos de re-login necessários são impressos (auth nunca viaja).

Restaurações recusam em vez de adivinhar — uma restauração recusada deixa o projeto
byte-idêntico.

## `agentmod doctor`

Diagnóstico somente leitura, seguro executar a qualquer momento (saída 0 limpa, 3 com descobertas):
estado de projeto/config/layout, instalação e vitalidade do gancho de shell, deriva de roteamento,
variáveis persistentes fora de projetos, entradas PATH duplicadas,
violações HOME/shim, presença de auth por agente com instruções de re-login,
advertências de vazamento OpenCode, estado global/projeto gstack, cabeamento de guarda Claude,
riscos de portabilidade em configs restaurados, candidatos secretos registrados em
snapshots existentes, material de sessão/log dentro de `.agentmod-handoff/`, e
se o HEAD do repositório ainda corresponde ao do snapshot mais novo.

## O guarda Bash do Claude

`agentmod init` registra `agentmod guard claude-bash` como um gancho PreToolUse do Claude Code
no home local do projeto. Bloqueia comandos Bash que
escreveriam para os homes de agente globais (`~/.claude`, `~/.codex`,
`~/.config/opencode`, `~/.local/share/opencode`), usariam `sudo`, ou reatribuiriam
`HOME` — o agente recebe o motivo e pode ajustar. Leituras nunca são
bloqueadas. É um heurístico de parse de shell profundo: guardrail útil, não uma
sandbox.

## Limitações conhecidas

Seção de honestidade. Estas são propriedades das ferramentas subjacentes ou escopo MVP deliberado
— `doctor` e os docs gerados as declaram também.

- **Keychain do macOS (Claude).** Claude Code no macOS armazena credenciais OAuth
  no Keychain, compartilhadas entre *todos* os diretórios de configuração. Isolamento de conta
  por projeto é impossível no macOS — e nenhum re-login é necessário por projeto.
  Linux/Windows usam um `.credentials.json` por home, que isola mas
  requer login/cópia por projeto.
- **OpenCode é parcialmente isolado por padrão.** OpenCode não tem uma única
  variável home; sua configuração é uma cadeia de merge que ainda lê a
  `~/.config/opencode/opencode.json` global, e sessões/storage/auth vivem em
  diretórios XDG globais. `opencode.xdg_full_isolation = true` roteia as
  variáveis XDG para isolamento completo — mas isso afeta *toda* ferramenta
  ciente de XDG que você executa dentro do projeto. `doctor` relata ambas as situações.
- **`.claude/` de projeto é comportamento nativo do Claude.** Claude Code sempre
  lê `./.claude/` independentemente de `CLAUDE_CONFIG_DIR`. O valor adicionado do agentmod para Claude é
  isolar estado *de nível de usuário* (skills/plugins globais, sessões, histórico); `.claude/`
  de projeto já funcionava antes do agentmod.
- **Ativação de gancho de primeira sessão.** Logo após `agentmod init`, o
  shell já em execução não carregou o novo bloco rc. Abra um novo
  terminal, `exec $SHELL`, ou único `eval "$(agentmod hook zsh)"` (init
  imprime exatamente isto). Da mesma forma, o gancho bash dispara via `PROMPT_COMMAND`
  e é portanto inerte em scripts bash não-interativos (mesma classe de
  limitação que direnv) — scripts devem definir as variáveis explicitamente via
  `eval "$(agentmod env --shell bash --activate <root>)"` se eles precisarem
  de roteamento.
- **Apenas bin global do npm está em PATH.** `.agentmod/node/bin` é a única
  entrada PATH gerenciada. Instalações globais de pnpm/bun são roteadas para o projeto
  (`PNPM_HOME`, `BUN_INSTALL`) mas seus diretórios bin não são adicionados ao PATH.
- **Pacotes de árvore restauram manualmente.** `handoff restore` aceita apenas arquivos `.amod`;
  um diretório `.agentmod-handoff/` commitado é restaurado seguindo
  o `RESTORE.md` dentro dele (esta versão não tem leitor de diretório).
- **Snapshots podem precisar de reparo pós-restauração.** O clone gstack viaja
  sem seu `.git` (re-execute `agentmod install gstack --force` para torná-lo
  atualizável novamente), e os symlinks inicializadores de `node/bin` penduram porque
  `node_modules` é excluído (re-execute `npm install -g …` dentro do
  projeto).
- **Suporte de shell é zsh e bash.** Outros shells ainda podem usar
  `agentmod env` manualmente.

## FAQ

**Continuo usando `claude` / `codex` / `opencode` diretamente?**
Sim. Esse é o ponto — sem wrappers, sem shims, sem `agentmod run`.

**Por que agentmod não apenas muda `HOME`?**
Reatribuir `HOME` quebra SSH, git, keychains, dotfiles, e toda ferramenta
no shell. agentmod roteia apenas as variáveis específicas do agente.

**Por que minha auth desaparece após uma restauração?**
Por design — credenciais nunca viajam em snapshots. Siga as linhas de
re-login impressas (ou oferta de cópia do init) na nova máquina.

**Posso commitar `.agentmod/` no git?**
Não — init o gitignore (sessões, caches, e possível auth copiada vivem
lá). Commita o subconjunto seguro em vez disso: `agentmod pack --for-git`.

**Como isto é diferente de direnv?**
Mesmo modelo de ativação (env com escopo de diretório, baseado em prompt-hook, restauração
perfeita ao sair), mas agentmod também sabe *o quê* rotear para cada agente,
cria os homes, guarda contra escritas globais, e faz entrega. Os dois
coexistem bem.

**Um snapshot falha ao criar com "descobertas de candidato secreto".**
A varredura de conteúdo encontrou material de chave privada em um arquivo mantido. Remova-o (ou
mova-o para um local excluído como `.env`), ou empacote mesmo assim com
`--allow-findings` se você aceitar estar dentro do snapshot.

**Funciona no Windows?**
O código Go compila e segurança de caminho é executada para caminhos estilo Windows, mas
os ganchos de shell visam zsh/bash; Windows é não testado nesta versão.

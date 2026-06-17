# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Izolare per-proiect și transmitere pentru agenți de codificare.

`agentmod` ține configurația, abilitățile, plugin-urile, sesiunile, cache-urile și
contextul de lucru al **Claude Code**, **Codex CLI** și **OpenCode** în
proiectul pe care lucrezi — și împachetează acel mediu într-o fotografie
pe care o poți transmite unei alte mașini.

Joacă două roluri:

1. **Agent Home Router.** Într-un arbore de directoare care conține
   `.agentmod/agentmod.toml`, un hook de shell rutează directorul home
   al fiecărui agent în `.agentmod/`. Afară, fiecare variabilă este
   restaurată exact cum era și configurația globală rămâne neatinsă.
2. **Instrument de transmitere.** `agentmod handoff create` împachetează `.agentmod/`
   într-o fotografie verificabilă `.amod` (sau, cu `--for-git`, un arbore
   de fișiere comit-abil sub `.agentmod-handoff/`). **Git mișcă codul sursă;
   agentmod mișcă mediul agentului.**

## Ce NU este agentmod

- **Nu este o sandbox Docker.** Rutează variabilele de mediu în shell-ul tău propriu.
  Nu este niciun container, nicio VM, nicio filtrare syscall.
- **Nu este izolare de securitate completă.** Un instrument care ignoră variabilele
  rutate poate accesa în continuare directoarele tale globale. Garda Bash a lui
  Claude (mai jos) este apărare în profunzime, nu o limită de securitate.
- **Nu este un shim.** Nu interceptează sau avulupeşte niciodată comenzile
  `claude`, `codex` sau `opencode`. Le execuți în continuare direct, nemodificat.
- **Nu este un instrument de schimbare a HOME.** `HOME` nu este niciodată reasignat.
- **Nu este un instrument de backup pentru codul sursă.** Fotografiile nu includ
  niciodată codul sursă în mod implicit. Folosește git pentru sursă.

## Cum funcționează

`agentmod hook zsh` / `agentmod hook bash` tipăresc o funcție shell mică și
independentă (instalată în fișierul tău rc de `agentmod init`). La fiecare
prompt și schimbare de director, se urcă în sus căutând `.agentmod/agentmod.toml`:

- **Intrând într-un proiect** salvează valorile curente și setează:

  | Variabilă | Rutată spre |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **doar** cu `opencode.xdg_full_isolation = true` |

  `PATH` câștigă exact o intrare, `.agentmod/node/bin` (bin-ul global al npm
  sub prefixul rutei). Variabilele de evidență (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) înregistrează ce trebuie să anulezi.

- **Ieșind din proiect** restaurează fiecare valoare salvată și elimină intrarea
  `PATH` — un invers perfect. Schimbarea directă între două proiecte agentmod
  re-rutează într-un singur pas fără a scurge căile niciunui proiect.

Rutarea per agent poate fi oprită în `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Instalare

Alege oricare se potrivește configurației tale — fiecare instalează același
binar unic:

```sh
# npm (instalează binarul precompilat pentru platforma ta)
npm install -g agentmod

# script de instalare (descarcă versiunea corespunzătoare, verifică sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (necesită lanțul de unelte Go)
go install github.com/mojomoth/agentmod@latest
```

Sau construiește din sursă (Go 1.26+, singurul dependency al modulului este
`BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# pune binarul undeva pe PATH
```

## Pornire rapidă

```sh
cd ~/work/myproject
agentmod init          # creează .agentmod/, editează .gitignore, instalează hook-ul
                       # shell în fișierul tău rc, oferă să copieze auth
# doar prima dată: hook-ul nu este activ în ACEST shell încă —
# deschide un terminal nou, sau: exec $SHELL

cd ~/work/myproject    # hook se activează; verifică-l:
agentmod status        # "AgentMod: active", cărări rutate listate
claude                 # comandă simplă — acum folosind directorul home local al proiectului
agentmod install gstack   # abilitățile locale ale proiectului, directorul home global neatins

agentmod pack          # fotografie în .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # diagnosticare numai citire oricând
```

Pe mașina receptoare:

```sh
cd ~/work/myproject    # sursa a sosit prin git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# urmează notele de re-conectare tipărite; doctor se execută automat
```

## `agentmod init`

Idempotent — re-rularea completează orice lipsește și nu suprascrie niciodată
un `agentmod.toml` existent sau niciun fișier al utilizatorului. Aceasta:

- creează `.agentmod/{claude,codex,opencode,node,snapshots,logs}` și un implicit
  `agentmod.toml`;
- conectează garda Bash a lui Claude în `.agentmod/claude/settings.json`;
- adaugă `.agentmod/` la `.gitignore` (creat doar într-un depozit git);
- instalează hook-ul shell ca un bloc delimitat în `~/.zshrc` sau `~/.bashrc`
  (shell-ul tău din `$SHELL`; blocul este actualizat pe loc, niciodată
  duplicat, și propriul tău conținut rc nu este niciodată atins);
- oferă să **copieze** fișierele de autentificare Claude/Codex existente în
  directorul home local al proiectului (vezi "Auth" mai jos) — copierea
  se întâmplă doar la explicit `y`.

Steaguri: `--no-shell-hook` omite toate editările fișierului rc; `--yes` /
`--non-interactive` nu prompt niciodată și prin urmare niciodată nu copiază
auth (pentru CI).

## Folosind `claude`, `codex`, `opencode` obișnuit

Nu există niciun comando wrapper. Într-un proiect activ comenzile obișnuite
pur și simplu văd directoarele home rutate:

- **Claude Code** citește `CLAUDE_CONFIG_DIR` → setări locale ale proiectului,
  abilitățile/plugin-urile de nivel utilizator, sesiuni, istoric. (Proiect
  `.claude/` este *întotdeauna* citit nativ — vezi Limitări.)
- **Codex CLI** citește `CODEX_HOME` → proiect-local `config.toml`,
  `auth.json`, sesiuni, istoric, jurnale.
- **OpenCode** citește `OPENCODE_CONFIG` → fișierul de configurare local al
  proiectului. Aceasta este izolare *parțială* în mod implicit — vezi Limitări.

### Auth

Directoarele home locale ale proiectelor noi încep fără acreditări:

- **Claude pe macOS**: nu este nimic de făcut — acreditările trăiesc în Keychain
  și sunt împărțite cu fiecare director de configurare (ceea ce înseamnă și că
  NU sunt izolate per proiect).
- **Claude pe Linux/Windows**: rulează `claude login` în cadrul proiectului,
  sau acceptă oferta init de a copia `~/.claude/.credentials.json`.
- **Codex**: rulează `codex login` în cadrul proiectului, sau acceptă oferta
  init de a copia `~/.codex/auth.json`.

Fișierele Auth **niciodată nu călătoresc în fotografii** (excluse după nume,
indiferent cum au ajuns acolo).

## Instalare gstack

[gstack](https://github.com/garrytan/gstack) codifica insoleitor programul
de instalare în `~/.claude/skills/gstack` — exact poluarea globală pe care
agentmod o previne. Deci:

```sh
agentmod install gstack            # clonează în .agentmod/claude/skills/gstack
agentmod install gstack --force    # înlocuiește o instalare locală a proiectului existent
```

Programul de instalare clonează cu git, niciodată nu rulează propriul script
de configurare al gstack, și face o fotografie a listării de `~/.claude/skills`
înainte și după — orice schimbare în directorul global este raportată ca
o încălcare și face ca comanda să eșueze. `agentmod doctor` separat avertizează
oricând o instalare globală *gstack* există (chiar una pe care ai instalat-o
tu însuți înainte de adoptarea agentmod), deoarece abilitățile instalate
global scapă în fiecare proiect.

## Transmitere (fotografii `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + raport de redactare, fără extracție
agentmod handoff verify  FILE      # re-hash fiecare membru; ieșire 3 la nonconcordanță
agentmod handoff restore FILE      # înlocuiește .agentmod/ (backup luat mai întâi)
agentmod pack / agentmod unpack    # aliasuri de create / restore
```

O fotografie este o fermoar cu șase membri root — `manifest.json`,
`inventory.json` (per-fișier dimensiune/sha256/mod), `REDACTION.md` (ce a fost
exclus și de ce, plus constatări de scanare secretă), `HANDOFF.md` și `RESTORE.md`
(instrucțiuni umane pentru receptor), `checksums.txt`
(`shasum -a 256 -c`-compatibil) — și sarcina utilă sub
`payload/.agentmod/…`. Crearea este atomică și deterministă; manifestul
înregistrează ramura git/commit/stare murdară cu orice acreditări eliminate
din URL-ul distant. Un arbore murdar refuză să se împacheteze decât `--allow-dirty`.

`inspect` și `verify` funcționează oriunde — receptorul poate audita o fotografie
înainte de a avea vreo configurare a proiectului.

### Politica de excludere a secretelor

Două straturi, ambele implicite:

1. **Reguli de excludere** scapă fișierele cunoscute-sensitive din sarcina utilă
   și listează fiecare în `REDACTION.md`: fișierele auth după nume
   (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`,
   chei SSH (`id_*`, `*.pem`, `*.pub`), directoare cu acreditări (`.ssh`,
   `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), fișiere
   keychain, `.git`, `node_modules`, cache-uri și directoare temp.
2. **O scanare de conținut** peste fiecare *fișier păstrat*. Materialul de
   cheie privată refuză crearea în schimb decât să treci `--allow-findings`
   (și este apoi marcat HARD în `REDACTION.md`). Tokeni probabil (ID-uri
   de acces AWS, tokenuri GitHub, chei `sk-…`, asignări de stil
   `api_key=`) sunt avertizate dar nu blochează.

Scanarea este euristică. **Revizuiți `REDACTION.md` (sau `handoff inspect`)
înainte de a partaja o fotografie** — sesiuni și context de lucru călătoresc
prin design și pot cita orice ați lipit într-o conversație de agent. Fotografiile
sunt scrise mod 0600 din acest motiv; tratează-le ca fișiere private.

## Transmitere Git

```sh
agentmod pack --for-git    # scrie .agentmod-handoff/ la rădăcina proiectului
git add .agentmod-handoff && git commit
```

Aceiași șase membri și sarcină utilă ca o `.amod`, dar ca un arbore comit-abil
al fișierelor obișnuite (`shasum -a 256 -c checksums.txt` funcționează în
director). Pe lângă exclusiunile implicite, scapă **sesiuni, transcrieri,
istoric și jurnale** pentru toți trei agenți — acestea în mod obișnuit conțin
secrete lipite și nu aparțin unui depozit. `--include-sessions` întotdeauna
refuză: comiterea sesiunilor ar necesita criptare, pe care această versiune
nu o implementează. Contextul de lucru care este sigur să fie partajat
(CLAUDE.md, configurări agent, abilitățile, planurile) rămâne.

Re-rularea înlocuiește pachetul anterior; nimic altceva din repo nu este atins.

## Restaurare avertismente

`handoff restore` / `unpack` tratează fiecare fotografie ca intrare neîncredere:

- verificare completă a sumei de control și verificare încrucișată a inventarului
  mai întâi;
- plan de siguranță a cării: zip-slip (`..`), cărări absolute, scrisori de
  unitate, ținte non-`.agentmod`, nume protejate (`.git`, `.ssh`, `.aws`,
  `.docker`), și ținte ale symlink-ului de scăpare sau absolute sunt toate
  refuzate înainte de a fi scris nimic;
- directorul `.agentmod/` existent este redenumit în `.agentmod.backup-<stamp>`
  înainte de extracție; orice eșec se întoarce la el automat;
- **nimic din fotografie nu este niciodată executat**;
- ulterior: hook-ul de garda Claude este re-conectat la binarul *acestei*
  mașini, căile absolute specifice mașinii găsite în configurări agent
  restaurate sunt avertizate (fișierele tale nu sunt niciodată rescrise),
  `doctor` se execută inline, și pașii necesari de re-conectare sunt
  tipăriti (auth niciodată nu călătorește).

Restaurările refuză mai degrabă decât ghicesc — o restaurare refuzată lasă
proiectul exact identic.

## `agentmod doctor`

Diagnosticare numai citire, sigur de a se executa oricând (ieșire 0 curat,
3 cu constatări): stare proiect/configurare/aspect, instalare și vivacitate
hook-ul shell, derivă rutare, variabile rămase în afara proiectelor, intrări
PATH duplicate, HOME/încălcări shim, prezență auth per-agent cu instrucțiuni
de re-conectare, avertismente scurgere OpenCode, stare globală/proiect gstack,
conectare garda Claude, riscuri de portabilitate în configurări restaurate,
candidați secreti înregistrați în fotografii existente, material sesiune/jurnal
în `.agentmod-handoff/`, și dacă HEAD-ul depozitului încă se potrivește cu
fotografiei cel mai nou.

## Garda Bash a lui Claude

`agentmod init` înregistrează `agentmod guard claude-bash` ca un hook
PreToolUse al Claude Code în directorul home local al proiectului. Blochează
comenzile Bash care ar scrie în directoarele home globale ale agentului
(`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`),
folosesc `sudo`, sau reasignează `HOME` — agentul primește motivul înapoi și
poate ajusta. Citirile nu sunt niciodată blocate. Este o euristică de analiză
shell în profunzime: garda utila, nu o sandbox.

## Limitări cunoscute

Secțiune de onestitate. Acestea sunt proprietăți ale instrumentelor de bază
sau domeniu MVP deliberat — `doctor` și documentele generate le menționează
și ele.

- **macOS Keychain (Claude).** Claude Code pe macOS stochează acreditări OAuth
  în Keychain, împărțite pe *toți* directoarele de configurare. Izolare
  cont per-proiect este imposibil pe macOS — și nu este necesară re-conectare
  per proiect. Linux/Windows folosesc un per-home `.credentials.json`, care
  izolează dar necesită conectare/copiere per proiect.
- **OpenCode este parțial izolat în mod implicit.** OpenCode nu are o singură
  variabilă home; configurarea sa este un lanț de fuziune care citește în
  continuare global-ul `~/.config/opencode/opencode.json`, și sesiuni/stocare/auth
  trăiesc în directoare de date XDG globale. `opencode.xdg_full_isolation = true`
  rutează variabilele XDG pentru izolare completă — dar afectează *fiecare*
  instrument care este conștient de XDG pe care îl execuți în cadrul proiectului.
  `doctor` raportează ambele situații.
- **Proiect `.claude/` este comportament nativ Claude.** Claude Code citește
  întotdeauna `./.claude/` indiferent de `CLAUDE_CONFIG_DIR`. Valoarea adăugată
  agentmod pentru Claude izolează starea *la nivel utilizator* (abilitățile
  globale/plugin-uri, sesiuni, istoric); proiect `.claude/` a funcționat deja
  înainte de agentmod.
- **Activare hook sesiune primera.** Imediat după `agentmod init`, shell-ul
  deja-rulare nu a încărcat blocul noului rc. Deschide un terminal nou,
  `exec $SHELL`, sau `eval "$(agentmod hook zsh)"` unic (init tipărește exact
  aceasta). La fel, hook-ul bash se aprinde prin `PROMPT_COMMAND` și prin
  urmare este inert în scripturi bash neinteractive (aceeași clasă de limitare
  ca direnv) — scripturile ar trebui să seteze variabilele în mod explicit
  prin `eval "$(agentmod env --shell bash --activate <root>)"` dacă au
  nevoie de rutare.
- **Doar bin-ul global al npm este pe PATH.** `.agentmod/node/bin` este
  singura intrare PATH gestionată. Instalări globale pnpm/bun sunt rutate
  în proiect (`PNPM_HOME`, `BUN_INSTALL`) dar directoarele lor bin nu sunt
  adăugate la PATH.
- **Pachete de arbore restaură manual.** `handoff restore` acceptă doar
  fișiere `.amod`; un director comis `.agentmod-handoff/` este restaurat
  urmărind `RESTORE.md` din interiorul său (această versiune nu are cititor
  de director).
- **Fotografiile pot avea nevoie de reparare post-restaurare.** Clonul gstack
  călătorește fără `.git` (re-rulează `agentmod install gstack --force`
  pentru a-l face din nou actualizabil), și symlink-urile launcher-ul
  `node/bin` atârnă deoarece `node_modules` este exclus (re-rulează
  `npm install -g …` în cadrul proiectului).
- **Suportul shell este zsh și bash.** Alte shell-uri pot folosi în continuare
  `agentmod env` manual.

## Întrebări frecvente

**Continui să folosesc direct `claude` / `codex` / `opencode`?**
Da. Acesta este punctul — fără wrapper-e, fără shim-uri, fără `agentmod run`.

**De ce nu schimbă agentmod pur și simplu `HOME`?**
Reasignarea `HOME` rupe SSH, git, keychain-uri, dotfișiere și fiecare alt
instrument din shell. agentmod rutează doar variabilele specifice agentului.

**De ce lipsește autentificarea mea după o restaurare?**
Prin design — acreditările niciodată nu călătoresc în fotografii. Urmează
liniile de re-conectare tipărite (sau oferta init de copiere) pe noua mașină.

**Pot comite `.agentmod/` în git?**
Nu — init o gitignore (sesiuni, cache-uri și posibil auth copiat trăiesc
acolo). Comiteți în schimb subsetul sigur: `agentmod pack --for-git`.

**Cum se diferențiază aceasta de direnv?**
Același model de activare (env scoped la director, pe bază de prompt-hook,
restaurare perfectă la ieșire), dar agentmod cunoaște și *ce* să ruteze pentru
fiecare agent, creează directoarele home, apără contra scrisurilor globale și
face transmitere. Cei doi coexistă bine.

**O fotografie eșuează să se creeze cu "constatări de candidat secret".**
Scanarea de conținut a găsit material de cheie privată într-un fișier păstrat.
Elimină-l (sau mută-l într-o locație exclusă cum ar fi `.env`), sau
împachetează oricum cu `--allow-findings` dacă accepți să fie în interiorul
fotografiei.

**Funcționează pe Windows?**
Codul Go construiește și siguranța căii este impusă pentru cărări în stil
Windows, dar hook-urile shell vizează zsh/bash; Windows este netestată în
această versiune.

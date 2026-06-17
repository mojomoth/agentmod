# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Isolamento per progetto e trasferimento per agenti di codifica.

`agentmod` mantiene la configurazione, le competenze, i plugin, le sessioni, i cache e
il contesto di lavoro di **Claude Code**, **Codex CLI** e **OpenCode** all'interno del
progetto su cui stai lavorando — e comprime tale ambiente in uno snapshot che
puoi trasferire a un'altra macchina.

Svolge due ruoli:

1. **Router della Home dell'Agente.** All'interno di un albero di directory contenente
   `.agentmod/agentmod.toml`, un hook di shell instrada la home di ogni agente in
   `.agentmod/`. All'esterno, ogni variabile viene ripristinata esattamente com'era e
   la tua configurazione globale rimane intatta.
2. **Strumento di Trasferimento.** `agentmod handoff create` comprime `.agentmod/` in uno
   snapshot verificabile `.amod` (o, con `--for-git`, un albero di file committabile
   sotto `.agentmod-handoff/`). **Git sposta il tuo codice sorgente; agentmod sposta
   l'ambiente dell'agente.**

## Cosa agentmod NON è

- **Non è una sandbox Docker.** Instrada le variabili d'ambiente nella tua stessa
  shell. Non c'è container, non c'è VM, non c'è filtering di syscall.
- **Non è isolamento completo di sicurezza.** Uno strumento che ignora le variabili
  instradate può comunque accedere alle tue home globali. La protezione Claude Bash
  (di seguito) è difesa in profondità, non un confine di sicurezza.
- **Non è uno shim.** Non intercetta o avvolge mai i comandi `claude`, `codex` o
  `opencode`. Li continui a eseguire direttamente, senza modifiche.
- **Non è uno strumento che cambia HOME.** `HOME` non viene mai riassegnato.
- **Non è uno strumento di backup del codice sorgente.** Gli snapshot non includono
  mai il tuo codice sorgente per impostazione predefinita. Usa git per il codice sorgente.

## Come funziona

`agentmod hook zsh` / `agentmod hook bash` stampano una piccola funzione shell autonoma
(installata nel tuo file rc da `agentmod init`). Ad ogni prompt e cambio di directory
risale cercando `.agentmod/agentmod.toml`:

- **Entrare in un progetto** salva i valori attuali e imposta:

  | Variabile | Instradato a |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **solo** con `opencode.xdg_full_isolation = true` |

  `PATH` guadagna esattamente una voce, `.agentmod/node/bin` (il bin globale di npm
  sotto il prefisso instradato). Le variabili di gestione (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) registrano cosa annullare.

- **Uscire dal progetto** ripristina ogni valore salvato e rimuove la voce `PATH` — un
  inverso perfetto. Passare direttamente tra due progetti agentmod reinstrada in un
  passaggio senza perdite da nessun progetto.

L'instradamento per agente può essere disattivato in `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Installazione

Scegli quello che si adatta al tuo setup — ognuno installa lo stesso singolo binario:

```sh
# npm (installa il binario precompilato per la tua piattaforma)
npm install -g agentmod

# script di installazione (scarica il rilascio corrispondente, verifica sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (richiede la toolchain Go)
go install github.com/mojomoth/agentmod@latest
```

Oppure compila da sorgente (Go 1.26+, la sola dipendenza del modulo è
`BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# metti il binario da qualche parte nel tuo PATH
```

## Avvio rapido

```sh
cd ~/work/myproject
agentmod init          # crea .agentmod/, modifica .gitignore, installa
                       # lo hook di shell nel tuo file rc, offre di copiare l'auth
# solo la prima volta: lo hook non è ancora attivo in QUESTA shell —
# apri un nuovo terminale, oppure: exec $SHELL

cd ~/work/myproject    # lo hook si attiva; controllalo:
agentmod status        # "AgentMod: active", percorsi instradati elencati
claude                 # comando semplice — ora usa la home locale del progetto
agentmod install gstack   # competenze locali del progetto, home globale intatta

agentmod pack          # snapshot a .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # diagnosi di sola lettura in qualsiasi momento
```

Sulla macchina ricevente:

```sh
cd ~/work/myproject    # il sorgente è arrivato tramite git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# segui le note di re-login stampate; doctor viene eseguito automaticamente
```

## `agentmod init`

Idempotente — eseguire di nuovo riempie ciò che manca e non sovrascrive mai un
`agentmod.toml` esistente o alcun file utente. Lo fa:

- crea `.agentmod/{claude,codex,opencode,node,snapshots,logs}` e un
  `agentmod.toml` predefinito;
- collega la protezione Claude Bash in `.agentmod/claude/settings.json`;
- aggiunge `.agentmod/` a `.gitignore` (creato solo all'interno di un repository git);
- installa lo hook di shell come un blocco recintato in `~/.zshrc` o `~/.bashrc`
  (la tua shell da `$SHELL`; il blocco viene aggiornato sul posto, mai
  duplicato, e il tuo contenuto rc non viene toccato);
- offre di **copiare** i file di autenticazione Claude/Codex esistenti nella
  home locale del progetto (vedi "Autenticazione" di seguito) — la copia avviene
  solo su un esplicito `y`.

Flag: `--no-shell-hook` salta tutti gli edit dei file rc; `--yes` /
`--non-interactive` non chiede mai e quindi non copia mai auth (per CI).

## Utilizzo di `claude`, `codex`, `opencode` semplici

Non c'è comando wrapper. All'interno di un progetto attivo i comandi ordinari
semplicemente vedono le home instradate:

- **Claude Code** legge `CLAUDE_CONFIG_DIR` → impostazioni locali del progetto,
  competenze/plugin a livello utente, sessioni, cronologia. (Il `.claude/`
  a livello di progetto è *sempre* letto nativamente — vedi Limitazioni.)
- **Codex CLI** legge `CODEX_HOME` → `config.toml` locale del progetto,
  `auth.json`, sessioni, cronologia, log.
- **OpenCode** legge `OPENCODE_CONFIG` → il file di configurazione locale del progetto.
  Questo è isolamento *parziale* per impostazione predefinita — vedi Limitazioni.

### Autenticazione

Le home locali del progetto appena create iniziano senza credenziali:

- **Claude su macOS**: niente da fare — le credenziali vivono nel Keychain e
  sono condivise con ogni directory di configurazione (il che significa anche che
  sono *non* isolate per progetto).
- **Claude su Linux/Windows**: esegui `claude login` all'interno del progetto, o
  accetta l'offerta di init di copiare `~/.claude/.credentials.json`.
- **Codex**: esegui `codex login` all'interno del progetto, o accetta l'offerta di
  init di copiare `~/.codex/auth.json`.

I file di autenticazione **non viaggiano mai negli snapshot** (esclusi per nome,
indipendentemente da come ci sono arrivati).

## Installazione di gstack

[gstack](https://github.com/garrytan/gstack) codifica il suo installer in
`~/.claude/skills/gstack` — esattamente l'inquinamento globale che agentmod
esiste per prevenire. Quindi:

```sh
agentmod install gstack            # clona in .agentmod/claude/skills/gstack
agentmod install gstack --force    # sostituisce un'installazione locale del progetto esistente
```

L'installer clona con git, non esegue mai lo script setup di gstack, e
scatta l'elenco di `~/.claude/skills` prima e dopo — qualsiasi cambiamento alla
directory globale è segnalato come una violazione e fallisce il comando.
`agentmod doctor` avvisa separatamente ogni volta che esiste un'installazione
gstack *globale* (anche una che hai installato tu stesso prima di adottare agentmod),
perché le competenze installate globalmente trapelano in ogni progetto.

## Trasferimento (snapshot `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + rapporto di redazione, nessuna estrazione
agentmod handoff verify  FILE      # re-hash ogni membro; esce 3 su disallineamento
agentmod handoff restore FILE      # sostituisce .agentmod/ (backup fatto per primo)
agentmod pack / agentmod unpack    # alias di create / restore
```

Uno snapshot è uno zip con sei membri radice — `manifest.json`,
`inventory.json` (size/sha256/mode per file), `REDACTION.md` (cosa è stato
escluso e perché, più i risultati della scansione segreta), `HANDOFF.md` e `RESTORE.md`
(istruzioni umane per il ricevitore), `checksums.txt`
(`shasum -a 256 -c`-compatibile) — e il payload sotto
`payload/.agentmod/…`. La creazione è atomica e deterministica; il manifest
registra il branch/commit/stato sporco di git con le credenziali rimosse dall'URL
remoto. Un worktree sporco rifiuta di comprimere a meno che `--allow-dirty`.

`inspect` e `verify` funzionano ovunque — il ricevitore può controllare uno
snapshot prima di avere un setup di progetto.

### Politica di esclusione dei segreti

Due livelli, entrambi attivi per impostazione predefinita:

1. **Regole di esclusione** lasciano i file sensibili noti dal payload e elencano
   ognuno in `REDACTION.md`: file auth per nome (`.credentials.json`,
   `auth.json`, `credentials*`), `*.env` / `.env.*`, chiavi SSH (`id_*`,
   `*.pem`, `*.pub`), directory di credenziali (`.ssh`, `.aws`, `.azure`,
   `.gcloud`, `.kube`, `.gnupg`, `.docker`), file di keychain, `.git`,
   `node_modules`, cache e directory temp.
2. **Una scansione di contenuto** su ogni file *mantenuto*. Il materiale della chiave
   privata rifiuta la creazione in uscita a meno che tu non passi `--allow-findings`
   (e quindi è marcato HARD in `REDACTION.md`). Probabili token (AWS access key IDs,
   token GitHub, chiavi `sk-…`, assegnazioni di stile `api_key=`) sono avvertiti ma
   non bloccano.

La scansione è euristica. **Rivedi `REDACTION.md` (o `handoff inspect`) prima
di condividere uno snapshot** — le sessioni e il contesto di lavoro viaggiano
di proposito e possono citare qualsiasi cosa tu abbia incollato in una conversazione
con un agente. Gli snapshot sono scritti mode 0600 per questo motivo; trattali
come file privati.

## Trasferimento Git

```sh
agentmod pack --for-git    # scrive .agentmod-handoff/ alla radice del progetto
git add .agentmod-handoff && git commit
```

Lo stesso sei membri e payload di un `.amod`, ma come un albero committabile di
file semplici (`shasum -a 256 -c checksums.txt` funziona nella directory). In
cima alle esclusioni predefinite, rimuove **sessioni, trascrizioni, cronologia,
e log** per tutti e tre gli agenti — quelli contengono routine segreti incollati e
non appartengono a un repository. `--include-sessions` sempre rifiuta:
impegnare sessioni richiederebbe crittografia, che questa versione non
implementa. Il contesto di lavoro che è sicuro da condividere (CLAUDE.md, configurazioni di agenti,
competenze, piani) rimane.

Re-esecuzione sostituisce il pacchetto precedente; nient'altro nel repo è
toccato.

## Avvertenze di ripristino

`handoff restore` / `unpack` tratta ogni snapshot come input non attendibile:

- verifica completa del checksum e cross-check dell'inventario per primo;
- piano di sicurezza del percorso: zip-slip (`..`), percorsi assoluti, lettere di unità,
  target non `.agentmod`, nomi protetti (`.git`, `.ssh`, `.aws`,
  `.docker`), e escaping o target di symlink assoluti sono tutti rifiutati
  prima che qualsiasi cosa sia scritta;
- il `.agentmod/` esistente è rinominato in `.agentmod.backup-<stamp>` prima
  dell'estrazione; qualsiasi guasto fa rollback automaticamente a esso;
- **niente da uno snapshot è mai eseguito**;
- dopo: lo hook di protezione Claude viene ri-cablato al binario *di questa* macchina,
  i percorsi assoluti specifici della macchina trovati nelle configurazioni dell'agente ripristinate
  sono avvertiti (i tuoi file non sono mai riscritti), `doctor` viene eseguito inline, e
  i passaggi di re-login richiesti sono stampati (auth non viaggia mai).

I ripristini rifiutano piuttosto che indovinare — un ripristino rifiutato lascia il
progetto byte-identico.

## `agentmod doctor`

Diagnosi di sola lettura, sicuro da eseguire in qualsiasi momento (esce 0 pulito, 3 con
risultati): stato di progetto/configurazione/layout, installazione e vitalità dello hook di shell,
deriva di instradamento, variabili persistenti fuori dai progetti, voci PATH duplicate,
violazioni di HOME/shim, presenza di auth per agente con istruzioni di re-login,
avvertenze di perdita OpenCode, stato gstack globale/progetto, cablaggio della protezione Claude,
rischi di portabilità nelle configurazioni ripristinate, candidati segretati registrati negli
snapshot esistenti, materiale di sessione/log all'interno di `.agentmod-handoff/`, e
se il HEAD del repository corrisponde ancora allo snapshot più recente.

## La protezione Claude Bash

`agentmod init` registra `agentmod guard claude-bash` come un hook PreToolUse di Claude Code
nella home locale del progetto. Blocca i comandi Bash che scriverebbero nelle home
dell'agente globali (`~/.claude`, `~/.codex`,
`~/.config/opencode`, `~/.local/share/opencode`), usano `sudo`, o riassegnano
`HOME` — l'agente riceve il motivo indietro e può regolarsi. Le letture non sono
mai bloccate. È una euristica di analisi della shell profonda: guida utile, non una
sandbox.

## Limitazioni note

Sezione di onestà. Queste sono proprietà degli strumenti sottostanti o ambito MVP deliberato —
`doctor` e i documenti generati le riportano anche.

- **Keychain macOS (Claude).** Claude Code su macOS memorizza le credenziali OAuth
  nel Keychain, condiviso tra *tutte* le directory di configurazione. L'isolamento
  per account per progetto è impossibile su macOS — e non è necessario un re-login
  per progetto. Linux/Windows usano un `.credentials.json` per home, che isola ma
  richiede login/copia per progetto.
- **OpenCode è parzialmente isolato per impostazione predefinita.** OpenCode non ha una
  singola variabile di home; la sua configurazione è una catena di unione che legge ancora
  il globale `~/.config/opencode/opencode.json`, e sessioni/archiviazione/auth vivono in
  directory dati XDG globali. `opencode.xdg_full_isolation = true` instrada le
  variabili XDG per isolamento completo — ma ciò colpisce *ogni* strumento consapevole
  di XDG che esegui all'interno del progetto. `doctor` riporta entrambe le situazioni.
- **Il `.claude/` del progetto è comportamento nativo di Claude.** Claude Code
  legge sempre `./.claude/` indipendentemente da `CLAUDE_CONFIG_DIR`. Il valore aggiunto
  di agentmod per Claude è isolare lo stato *a livello utente* (competenze/plugin globali,
  sessioni, cronologia); il `.claude/` del progetto ha già funzionato prima di agentmod.
- **Attivazione dello hook della prima sessione.** Subito dopo `agentmod init`, la
  shell già in esecuzione non ha caricato il nuovo blocco rc. Apri un nuovo
  terminale, `exec $SHELL`, o una singola volta `eval "$(agentmod hook zsh)"` (init
  stampa esattamente questo). Allo stesso modo, lo hook bash si attiva tramite `PROMPT_COMMAND`
  e è quindi inerte in script bash non interattivi (stessa classe di
  limitazione di direnv) — gli script devono impostare le variabili esplicitamente tramite
  `eval "$(agentmod env --shell bash --activate <root>)"` se hanno bisogno di
  instradamento.
- **Solo il bin globale di npm è su PATH.** `.agentmod/node/bin` è la singola
  voce PATH gestita. Le installazioni globali di pnpm/bun sono instradate nel progetto
  (`PNPM_HOME`, `BUN_INSTALL`) ma le loro directory bin non sono aggiunte a PATH.
- **I pacchetti dell'albero si ripristinano manualmente.** `handoff restore` accetta file `.amod`
  solo; una directory `.agentmod-handoff/` committata viene ripristinata
  seguendo il `RESTORE.md` al suo interno (questa versione non ha un lettore di directory).
- **Gli snapshot possono aver bisogno di riparazione post-ripristino.** Il clone gstack
  viaggia senza il suo `.git` (esegui di nuovo `agentmod install gstack --force` per
  renderlo aggiornabile di nuovo), e i symlink del launcher `node/bin` penzolano perché
  `node_modules` è escluso (esegui di nuovo `npm install -g …` all'interno del
  progetto).
- **Il supporto della shell è zsh e bash.** Altre shell possono ancora utilizzare
  `agentmod env` manualmente.

## Domande frequenti

**Continuo a usare `claude` / `codex` / `opencode` direttamente?**
Sì. Questo è il punto — nessun wrapper, nessuno shim, nessun `agentmod run`.

**Perché agentmod non cambia semplicemente `HOME`?**
Riassegnare `HOME` rompe SSH, git, keychain, dotfile e ogni altro
strumento nella shell. agentmod instrada solo le variabili specifiche dell'agente.

**Perché la mia auth manca dopo un ripristino?**
Di proposito — le credenziali non viaggiano mai negli snapshot. Segui le righe
di re-login stampate (o l'offerta di copia di init) sulla nuova macchina.

**Posso impegnare `.agentmod/` a git?**
No — init lo gitignora (sessioni, cache e possibilmente auth copiate vivono
lì). Invece impegna il sottoinsieme sicuro: `agentmod pack --for-git`.

**Come è diverso da direnv?**
Lo stesso modello di attivazione (env scoped per directory, basato su hook di prompt, perfetto
ripristino in uscita), ma agentmod sa anche *cosa* instradare per ogni agente,
crea le home, protegge da scritture globali, e fa il trasferimento. I due
coesistono bene.

**Uno snapshot fallisce a crearsi con "risultati di candidati segretati".**
La scansione di contenuto ha trovato materiale di chiave privata in un file mantenuto. Rimuovilo (o
spostalo in una posizione esclusa come `.env`), o comprimi comunque con
`--allow-findings` se lo accetti dentro lo snapshot.

**Funziona su Windows?**
Il codice Go compila e la sicurezza del percorso viene applicata per percorsi di stile Windows, ma
gli hook di shell mirano a zsh/bash; Windows non è testato in questa versione.

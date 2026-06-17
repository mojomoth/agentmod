# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Isolation et transfert par projet pour les agents de codage.

`agentmod` conserve la configuration, les compétences, les plugins, les sessions, les caches et le contexte de travail de **Claude Code**, **Codex CLI** et **OpenCode** à l'intérieur du projet sur lequel vous travaillez, et empaquète cet environnement dans un instantané que vous pouvez transférer à une autre machine.

Il joue deux rôles :

1. **Agent Home Router.** À l'intérieur d'un arborescence de répertoires contenant `.agentmod/agentmod.toml`, un hook shell achemine le home de chaque agent vers `.agentmod/`. À l'extérieur, chaque variable est restaurée exactement comme elle était et votre configuration globale reste intacte.
2. **Outil de transfert.** `agentmod handoff create` empaquète `.agentmod/` dans un instantané `.amod` vérifiable (ou, avec `--for-git`, un arborescence de fichiers engageable sous `.agentmod-handoff/`). **Git déplace votre code source ; agentmod déplace l'environnement de l'agent.**

## Ce que agentmod n'est *pas*

- **Pas un bac à sable Docker.** Il achemine les variables d'environnement dans votre propre shell. Il n'y a pas de conteneur, pas de VM, pas de filtrage d'appels système.
- **Pas une isolation de sécurité complète.** Un outil qui ignore les variables acheminées peut toujours accéder à vos homes globaux. La protection Bash Claude (ci-dessous) est une défense en profondeur, pas une limite de sécurité.
- **Pas un shim.** Il n'intercepte ni n'encapsule jamais les commandes `claude`, `codex` ou `opencode`. Vous continuez à les exécuter directement, sans modification.
- **Pas un outil de changement de HOME.** `HOME` n'est jamais réassigné.
- **Pas un outil de sauvegarde du code source.** Les instantanés n'incluent jamais votre code source par défaut. Utilisez git pour cela.

## Comment cela fonctionne

`agentmod hook zsh` / `agentmod hook bash` impriment une petite fonction shell auto-contenue (installée dans votre fichier rc par `agentmod init`). À chaque invite de commande et changement de répertoire, elle remonte à la recherche de `.agentmod/agentmod.toml` :

- **En entrant dans un projet**, sauvegarde les valeurs actuelles et définit :

  | Variable | Acheminé vers |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **uniquement** avec `opencode.xdg_full_isolation = true` |

  `PATH` gagne exactement une entrée, `.agentmod/node/bin` (le répertoire global bin de npm sous le préfixe acheminé). Les variables de comptabilité (`AGENTMOD_ACTIVE`, `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`, `AGENTMOD_SAVED_*`) enregistrent ce qu'il faut annuler.

- **En quittant le projet**, restaure chaque valeur sauvegardée et supprime l'entrée `PATH` — un inverse parfait. Basculer directement entre deux projets agentmod réachemine en une étape sans fuir les chemins de l'un ou l'autre projet.

L'acheminement par agent peut être désactivé dans `agentmod.toml` (`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Installation

Choisissez celui qui convient à votre configuration — chacun installe le même binaire unique :

```sh
# npm (installe le binaire précompilé pour votre plateforme)
npm install -g agentmod

# script d'installation (télécharge la version correspondante, vérifie sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (nécessite la chaîne d'outils Go)
go install github.com/mojomoth/agentmod@latest
```

Ou compiler à partir de la source (Go 1.26+, la seule dépendance de module est `BurntSushi/toml`) :

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# mettez le binaire quelque part sur votre PATH
```

## Démarrage rapide

```sh
cd ~/work/myproject
agentmod init          # crée .agentmod/, édite .gitignore, installe le
                       # hook shell dans votre fichier rc, propose de copier l'auth
# première fois seulement : le hook n'est pas encore actif dans CETTE shell —
# ouvrez un nouveau terminal, ou : exec $SHELL

cd ~/work/myproject    # le hook s'active ; vérifiez :
agentmod status        # "AgentMod: active", chemins acheminés listés
claude                 # commande simple — utilise maintenant le home local du projet
agentmod install gstack   # compétences locales du projet, home global intact

agentmod pack          # instantané vers .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # diagnostic en lecture seule à tout moment
```

Sur la machine réceptrice :

```sh
cd ~/work/myproject    # la source est arrivée via git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# suivez les notes de reconnexion imprimées ; doctor s'exécute automatiquement
```

## `agentmod init`

Idempotent — réexécuter remplit ce qui manque et ne remplace jamais un `agentmod.toml` existant ou aucun fichier utilisateur. Il :

- crée `.agentmod/{claude,codex,opencode,node,snapshots,logs}` et un `agentmod.toml` par défaut ;
- câble la protection Bash Claude dans `.agentmod/claude/settings.json` ;
- ajoute `.agentmod/` à `.gitignore` (créé uniquement dans un dépôt git) ;
- installe le hook shell comme un bloc délimité dans `~/.zshrc` ou `~/.bashrc` (votre shell depuis `$SHELL` ; le bloc est mis à jour en place, jamais dupliqué, et votre contenu rc personnel n'est jamais touché) ;
- propose de **copier** les fichiers d'authentification Claude/Codex existants dans le home local du projet (voir "Auth" ci-dessous) — la copie se produit uniquement sur un `y` explicite.

Drapeaux : `--no-shell-hook` ignore tous les édits de fichier rc ; `--yes` / `--non-interactive` ne demande jamais et par conséquent ne copie jamais l'auth (pour CI).

## Utilisation de `claude`, `codex`, `opencode` simples

Il n'y a pas de commande wrapper. À l'intérieur d'un projet actif, les commandes ordinaires voient simplement les homes acheminés :

- **Claude Code** lit `CLAUDE_CONFIG_DIR` → paramètres locaux du projet, compétences/plugins au niveau utilisateur, sessions, historique. (Le `.claude/` local du projet est *toujours* lu nativement — voir Limitations.)
- **Codex CLI** lit `CODEX_HOME` → `config.toml`, `auth.json`, sessions, historique, logs locaux au projet.
- **OpenCode** lit `OPENCODE_CONFIG` → le fichier de configuration local au projet. C'est une isolation *partielle* par défaut — voir Limitations.

### Auth

Les homes locaux du projet nouvellement créés commencent sans identifiants :

- **Claude sur macOS** : rien à faire — les identifiants vivent dans le Keychain et sont partagés avec chaque répertoire de configuration (ce qui signifie également qu'ils ne sont *pas* isolés par projet).
- **Claude sur Linux/Windows** : exécutez `claude login` à l'intérieur du projet, ou acceptez l'offre d'init de copier `~/.claude/.credentials.json`.
- **Codex** : exécutez `codex login` à l'intérieur du projet, ou acceptez l'offre d'init de copier `~/.codex/auth.json`.

Les fichiers d'auth ne **voyagent jamais dans les instantanés** (exclus par nom, indépendamment de leur présence).

## Installation de gstack

[gstack](https://github.com/garrytan/gstack) encode en dur son installateur vers `~/.claude/skills/gstack` — exactement la pollution globale que agentmod existe pour prévenir. Donc :

```sh
agentmod install gstack            # clone dans .agentmod/claude/skills/gstack
agentmod install gstack --force    # remplace une installation locale du projet existante
```

L'installateur clone avec git, n'exécute jamais le propre script de configuration de gstack, et crée une capture instantanée de la liste de `~/.claude/skills` avant et après — tout changement au répertoire global est signalé comme une violation et échoue la commande. `agentmod doctor` avertit séparément chaque fois qu'une installation *globale* de gstack existe (même celle que vous avez installée vous-même avant d'adopter agentmod), car les compétences installées globalement fuient dans chaque projet.

## Transfert (instantanés `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifeste + rapport de rédaction, pas d'extraction
agentmod handoff verify  FILE      # re-hash chaque membre ; sortie 3 en cas de non-correspondance
agentmod handoff restore FILE      # remplace .agentmod/ (sauvegarde effectuée en premier)
agentmod pack / agentmod unpack    # alias de create / restore
```

Un instantané est un zip avec six membres racine — `manifest.json`, `inventory.json` (taille/sha256/mode par fichier), `REDACTION.md` (ce qui a été exclu et pourquoi, plus les découvertes de scan secret), `HANDOFF.md` et `RESTORE.md` (instructions humaines pour le récepteur), `checksums.txt` (compatible avec `shasum -a 256 -c`) — et la charge utile sous `payload/.agentmod/…`. La création est atomique et déterministe ; le manifeste enregistre l'état de branche/commit/dirty de git avec tous les identifiants supprimés de l'URL distante. Un arborescence sale refuse de s'empaqueter sauf avec `--allow-dirty`.

`inspect` et `verify` fonctionnent n'importe où — le récepteur peut auditer un instantané avant d'avoir configuré le projet.

### Politique d'exclusion des secrets

Deux couches, toutes les deux activées par défaut :

1. **Règles d'exclusion** qui suppriment les fichiers sensibles connus de la charge utile et listent chacun dans `REDACTION.md` : fichiers d'auth par nom (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`, clés SSH (`id_*`, `*.pem`, `*.pub`), répertoires d'identifiants (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), fichiers de keychain, `.git`, `node_modules`, caches et répertoires temp.
2. **Un scan de contenu** sur chaque fichier *gardé*. Le matériel de clé privée refuse la création à moins de passer `--allow-findings` (et est alors marqué HARD dans `REDACTION.md`). Les jetons probables (AWS access key IDs, GitHub tokens, clés `sk-…`, assignations de style `api_key=`) sont avertis mais ne bloquent pas.

Le scan est heuristique. **Examinez `REDACTION.md` (ou `handoff inspect`) avant de partager un instantané** — les sessions et le contexte de travail voyagent par conception et peuvent citer n'importe quoi que vous avez collé dans une conversation d'agent. Les instantanés sont écrits en mode 0600 pour cette raison ; traitez-les comme des fichiers privés.

## Transfert Git

```sh
agentmod pack --for-git    # écrit .agentmod-handoff/ à la racine du projet
git add .agentmod-handoff && git commit
```

Les mêmes six membres et charge utile qu'un `.amod`, mais comme un arborescence engageable de fichiers simples (`shasum -a 256 -c checksums.txt` fonctionne dans le répertoire). En plus des exclusions par défaut, il supprime **sessions, transcriptions, historique, et logs** pour les trois agents — ceux-ci contiennent routinement des secrets collés et n'appartiennent pas à un référentiel. `--include-sessions` refuse toujours : engager les sessions nécessiterait le chiffrement, que cette version n'implémente pas. Le contexte de travail qui est sûr de partager (CLAUDE.md, configs d'agents, compétences, plans) reste.

La réexécution remplace le package précédent ; rien d'autre dans le référentiel n'est touché.

## Précautions de restauration

`handoff restore` / `unpack` traite chaque instantané comme une entrée non approuvée :

- vérification complète de la somme de contrôle et vérification croisée d'inventaire en premier ;
- plan de sécurité de chemin : zip-slip (`..`), chemins absolus, lettres de lecteur, cibles non-`.agentmod`, noms protégés (`.git`, `.ssh`, `.aws`, `.docker`), et échappement ou cibles de symlink absolus sont tous refusés avant que quoi que ce soit soit écrit ;
- le `.agentmod/` existant est renommé en `.agentmod.backup-<stamp>` avant l'extraction ; tout échec le restaure automatiquement ;
- **rien d'un instantané n'est jamais exécuté** ;
- ensuite : le hook de protection Claude est re-câblé au binaire *de cette* machine, les chemins absolus spécifiques à la machine trouvés dans les configs d'agents restaurés sont avertis (vos fichiers ne sont jamais récrits), `doctor` s'exécute en ligne, et les étapes de reconnexion requises sont imprimées (l'auth ne voyage jamais).

Les restaurations refusent plutôt que de deviner — une restauration refusée laisse le projet octet-identique.

## `agentmod doctor`

Diagnostic en lecture seule, sûr à exécuter à tout moment (sortie 0 clean, 3 avec découvertes) : état du projet/config/layout, installation et liveness du shell-hook, dérive d'acheminement, variables persistantes à l'extérieur des projets, entrées PATH en double, violations HOME/shim, présence d'auth par agent avec instructions de reconnexion, avertissements de fuite OpenCode, état global/projet de gstack, câblage de protection Claude, risques de portabilité dans les configs restaurés, candidats secrets enregistrés dans les instantanés existants, matériel de session/log à l'intérieur de `.agentmod-handoff/`, et si le HEAD du référentiel correspond toujours au plus nouvel instantané.

## La protection Bash Claude

`agentmod init` enregistre `agentmod guard claude-bash` comme un hook Claude Code PreToolUse dans le home local du projet. Il bloque les commandes Bash qui écrivent aux homes d'agents globaux (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`), utilisent `sudo`, ou réassignent `HOME` — l'agent récupère la raison et peut s'ajuster. Les lectures ne sont jamais bloquées. C'est une heuristique d'analyse de shell profonde : une garde utile, pas un bac à sable.

## Limitations connues

Section d'honnêteté. Ce sont des propriétés des outils sous-jacents ou une portée MVP délibérée — `doctor` et les docs générés les énoncent aussi.

- **Keychain macOS (Claude).** Claude Code sur macOS stocke les identifiants OAuth dans le Keychain, partagés sur *tous* les répertoires de configuration. L'isolation de compte par projet est impossible sur macOS — et aucune reconnexion n'est nécessaire par projet. Linux/Windows utilisent un `.credentials.json` par home, qui isole mais nécessite login/copie par projet.
- **OpenCode est partiellement isolé par défaut.** OpenCode n'a pas de variable de home unique ; sa configuration est une chaîne de fusion qui lit toujours le `~/.config/opencode/opencode.json` global, et les sessions/storage/auth vivent dans les répertoires de données XDG globaux. `opencode.xdg_full_isolation = true` achemine les variables XDG pour une isolation complète — mais cela affecte *chaque* outil conscient de XDG que vous exécutez à l'intérieur du projet. `doctor` rapporte les deux situations.
- **Project `.claude/` est comportement Claude natif.** Claude Code lit toujours `./.claude/` indépendamment de `CLAUDE_CONFIG_DIR`. La valeur ajoutée d'agentmod pour Claude isole l'état *au niveau utilisateur* (compétences/plugins globaux, sessions, historique) ; le `.claude/` du projet fonctionnait déjà avant agentmod.
- **Activation du hook de première session.** Immédiatement après `agentmod init`, la shell déjà en cours d'exécution n'a pas chargé le nouveau bloc rc. Ouvrez un nouveau terminal, `exec $SHELL`, ou `eval "$(agentmod hook zsh)"` ponctuel (init imprime exactement ceci). De même, le hook bash se déclenche via `PROMPT_COMMAND` et est par conséquent inerte dans les scripts bash non-interactifs (même classe de limitation que direnv) — les scripts doivent définir les variables explicitement via `eval "$(agentmod env --shell bash --activate <root>)"` s'ils ont besoin d'acheminement.
- **Seulement le global bin de npm est sur PATH.** `.agentmod/node/bin` est l'entrée PATH gérée unique. Les installs globaux pnpm/bun sont acheminés dans le projet (`PNPM_HOME`, `BUN_INSTALL`) mais leurs répertoires bin ne sont pas ajoutés à PATH.
- **Les packages d'arborescence se restaurent manuellement.** `handoff restore` accepte uniquement les fichiers `.amod` ; un répertoire `.agentmod-handoff/` engagé est restauré en suivant le `RESTORE.md` à l'intérieur (cette version n'a pas de lecteur de répertoire).
- **Les instantanés peuvent nécessiter une réparation post-restauration.** Le clone gstack voyage sans son `.git` (réexécutez `agentmod install gstack --force` pour le rendre à nouveau updatable), et les symlinks du lanceur `node/bin` pendent car `node_modules` est exclu (réexécutez `npm install -g …` à l'intérieur du projet).
- **Le support shell est zsh et bash.** D'autres shells peuvent toujours utiliser `agentmod env` manuellement.

## FAQ

**Je continue à utiliser `claude` / `codex` / `opencode` directement ?**
Oui. C'est le point — pas de wrappers, pas de shims, pas de `agentmod run`.

**Pourquoi agentmod ne change-t-il pas simplement `HOME` ?**
Réassigner `HOME` casse SSH, git, keychains, dotfiles, et chaque autre outil dans le shell. agentmod achemine uniquement les variables spécifiques à l'agent.

**Pourquoi mon auth manque-t-il après une restauration ?**
Par conception — les identifiants ne voyagent jamais dans les instantanés. Suivez les lignes de reconnexion imprimées (ou l'offre de copie d'init) sur la nouvelle machine.

**Puis-je engager `.agentmod/` sur git ?**
Non — init le gitignore (sessions, caches, et possiblement auth copié vivent là-bas). Engagez le sous-ensemble sûr à la place : `agentmod pack --for-git`.

**Comment cela diffère-t-il de direnv ?**
Même modèle d'activation (env d'étendue de répertoire, basé sur hook d'invite, restauration parfaite à la sortie), mais agentmod sait aussi *ce qu'il faut* acheminer pour chaque agent, crée les homes, protège contre les écritures globales, et fait le transfert. Les deux coexistent bien.

**Un instantané échoue à créer avec "secret-candidate findings".**
Le scan de contenu a trouvé du matériel de clé privée dans un fichier gardé. Supprimez-le (ou déplacez-le vers un emplacement exclu comme `.env`), ou empaquetez quand même avec `--allow-findings` si vous acceptez qu'il soit à l'intérieur de l'instantané.

**Cela fonctionne-t-il sous Windows ?**
Le code Go se compile et la sécurité de chemin est appliquée pour les chemins de style Windows, mais les hooks shell ciblent zsh/bash ; Windows est non testé dans cette version.

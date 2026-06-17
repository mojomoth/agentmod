# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Aislamiento por proyecto y entrega para agentes de codificación.

`agentmod` mantiene la configuración, habilidades, complementos, sesiones, cachés y
contexto de trabajo de **Claude Code**, **Codex CLI** y **OpenCode** dentro del
proyecto en el que estás trabajando — y empaqueta ese entorno en una instantánea que
puedes entregar a otra máquina.

Desempeña dos funciones:

1. **Enrutador de inicio de agente.** Dentro de un árbol de directorios que contiene
   `.agentmod/agentmod.toml`, un gancho de shell enruta el inicio de cada agente a
   `.agentmod/`. Fuera, cada variable se restaura exactamente como era y
   tu configuración global no se toca.
2. **Herramienta de entrega.** `agentmod handoff create` empaqueta `.agentmod/` en una
   instantánea verificable `.amod` (o, con `--for-git`, un árbol de archivos comprometible
   bajo `.agentmod-handoff/`). **Git mueve tu fuente; agentmod mueve el
   entorno del agente.**

## Lo que agentmod *no es*

- **No es una caja de arena Docker.** Enruta variables de entorno en tu propio
  shell. No hay contenedor, no hay máquina virtual, no hay filtrado de llamadas del sistema.
- **No es aislamiento de seguridad completo.** Una herramienta que ignore las variables
  enrutadas aún puede acceder a tus hogares globales. El guard de Bash de Claude (a continuación) es
  defensa en profundidad, no un límite de seguridad.
- **No es un shim.** Nunca intercepta o envuelve los comandos `claude`, `codex` u
  `opencode`. Los sigues ejecutando directamente, sin modificar.
- **No es una herramienta de cambio de HOME.** `HOME` nunca se reasigna.
- **No es una herramienta de copia de seguridad de código fuente.** Las instantáneas nunca incluyen tu código fuente
  por defecto. Usa git para la fuente.

## Cómo funciona

`agentmod hook zsh` / `agentmod hook bash` imprimen una pequeña función de shell
autónoma (instalada en tu archivo rc por `agentmod init`). En cada indicador de línea
y cambio de directorio, camina hacia arriba buscando `.agentmod/agentmod.toml`:

- **Entrando a un proyecto** guarda los valores actuales y establece:

  | Variable | Enrutado a |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **solo** con `opencode.xdg_full_isolation = true` |

  `PATH` gana exactamente una entrada, `.agentmod/node/bin` (el bin global de npm
  bajo el prefijo enrutado). Las variables de control (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) registran qué deshacer.

- **Saliendo del proyecto** restaura cada valor guardado y elimina la entrada `PATH`
  — un inverso perfecto. Cambiar directamente entre dos proyectos agentmod vuelve a
  enrutar en un paso sin filtrar los caminos de ninguno de los dos proyectos.

El enrutamiento por agente se puede desactivar en `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Instalar

Elige lo que mejor se ajuste a tu configuración — cada uno instala el mismo binario único:

```sh
# npm (instala el binario precompilado para tu plataforma)
npm install -g agentmod

# script de instalación (descarga la versión coincidente, verifica sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (requiere la cadena de herramientas Go)
go install github.com/mojomoth/agentmod@latest
```

O construir desde la fuente (Go 1.26+, la única dependencia de módulo es
`BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# coloca el binario en algún lugar en tu PATH
```

## Inicio rápido

```sh
cd ~/work/myproject
agentmod init          # crea .agentmod/, edita .gitignore, instala el
                       # gancho de shell en tu archivo rc, ofrece copiar auth
# solo la primera vez: el gancho no está activo en ESTE shell aún —
# abre una nueva terminal, o: exec $SHELL

cd ~/work/myproject    # se activa el gancho; compruébalo:
agentmod status        # "AgentMod: active", caminos enrutados listados
claude                 # comando simple — ahora usando el hogar local del proyecto
agentmod install gstack   # habilidades locales del proyecto, hogar global sin tocar

agentmod pack          # instantánea a .agentmod/snapshots/<nombre>-<marca>.amod
agentmod doctor        # diagnóstico de solo lectura en cualquier momento
```

En la máquina receptora:

```sh
cd ~/work/myproject    # la fuente llegó vía git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# sigue las notas de re-inicio impresas; doctor se ejecuta automáticamente
```

## `agentmod init`

Idempotente — re-ejecutar completa lo que falta y nunca sobrescribe un
`agentmod.toml` existente o archivo de usuario alguno. Hace lo siguiente:

- crea `.agentmod/{claude,codex,opencode,node,snapshots,logs}` y un
  `agentmod.toml` predeterminado;
- conecta el guard de Bash de Claude en `.agentmod/claude/settings.json`;
- agrega `.agentmod/` a `.gitignore` (creado solo dentro de un repositorio git);
- instala el gancho de shell como un bloque delimitado en `~/.zshrc` o `~/.bashrc`
  (tu shell de `$SHELL`; el bloque se actualiza en el lugar, nunca
  duplicado, y tu propio contenido rc nunca se toca);
- ofrece **copiar** archivos de autenticación Claude/Codex existentes en el
  hogar local del proyecto (ver "Auth" a continuación) — la copia ocurre solo en un `y` explícito.

Banderas: `--no-shell-hook` salta todas las ediciones de archivo rc; `--yes` /
`--non-interactive` nunca solicita y por lo tanto nunca copia auth (para CI).

## Usando `claude`, `codex`, `opencode` simples

No hay comando envolvente. Dentro de un proyecto activo los comandos ordinarios
simplemente ven los hogares enrutados:

- **Claude Code** lee `CLAUDE_CONFIG_DIR` → configuración local del proyecto,
  habilidades/complementos a nivel de usuario, sesiones, historial. (El `.claude/` a nivel de proyecto
  es *siempre* leído de forma nativa — ver Limitaciones.)
- **Codex CLI** lee `CODEX_HOME` → `config.toml` local del proyecto,
  `auth.json`, sesiones, historial, registros.
- **OpenCode** lee `OPENCODE_CONFIG` → el archivo de configuración local del proyecto.
  Esto es *aislamiento parcial* de forma predeterminada — ver Limitaciones.

### Auth

Los hogares locales del proyecto actuales comienzan sin credenciales:

- **Claude en macOS**: nada que hacer — las credenciales viven en el Llavero y
  se comparten con cada directorio de configuración (lo que también significa que *no* están
  aisladas por proyecto).
- **Claude en Linux/Windows**: ejecuta `claude login` dentro del proyecto, o
  acepta la oferta de init de copiar `~/.claude/.credentials.json`.
- **Codex**: ejecuta `codex login` dentro del proyecto, o acepta la oferta de init
  de copiar `~/.codex/auth.json`.

Los archivos de autenticación **nunca viajan en instantáneas** (excluidos por nombre, independientemente de
cómo llegaron).

## Instalación de gstack

[gstack](https://github.com/garrytan/gstack) codifica su instalador a
`~/.claude/skills/gstack` — exactamente la contaminación global que agentmod existe para
prevenir. Así que:

```sh
agentmod install gstack            # clona en .agentmod/claude/skills/gstack
agentmod install gstack --force    # reemplaza una instalación local del proyecto existente
```

El instalador clona con git, nunca ejecuta el script de configuración propio de gstack, y
toma una instantánea del listado de `~/.claude/skills` antes y después — cualquier cambio en
el directorio global se reporta como una violación y falla el comando.
`agentmod doctor` por separado advierte siempre que una instalación *global* de gstack existe
(incluso una que instalaste tú mismo antes de adoptar agentmod), porque las habilidades instaladas
globalmente filtran en cada proyecto.

## Entrega (instantáneas `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifiesto + informe de redacción, sin extracción
agentmod handoff verify  FILE      # vuelve a hacer hash cada miembro; salida 3 en no coincidencia
agentmod handoff restore FILE      # reemplaza .agentmod/ (copia de seguridad hecha primero)
agentmod pack / agentmod unpack    # alias de crear / restaurar
```

Una instantánea es un zip con seis miembros raíz — `manifest.json`,
`inventory.json` (tamaño/sha256/modo por archivo), `REDACTION.md` (qué fue
excluido y por qué, más hallazgos de escaneo secreto), `HANDOFF.md` y `RESTORE.md`
(instrucciones humanas para el receptor), `checksums.txt`
(`shasum -a 256 -c`-compatible) — y la carga bajo
`payload/.agentmod/…`. La creación es atómica y determinista; el manifiesto
registra rama git/commit/estado sucio con cualquier credencial eliminada de la
URL remota. Un árbol de trabajo sucio se rehúsa a empaquetar a menos que `--allow-dirty`.

`inspect` y `verify` funcionan en cualquier lugar — el receptor puede auditar una instantánea
antes de tener alguna configuración de proyecto.

### Política de exclusión de secretos

Dos capas, ambas activadas de forma predeterminada:

1. **Reglas de exclusión** descartan archivos conocidos como sensibles de la carga y listan
   cada uno en `REDACTION.md`: archivos de autenticación por nombre (`.credentials.json`,
   `auth.json`, `credentials*`), `*.env` / `.env.*`, claves SSH (`id_*`,
   `*.pem`, `*.pub`), directorios de credenciales (`.ssh`, `.aws`, `.azure`,
   `.gcloud`, `.kube`, `.gnupg`, `.docker`), archivos de llavero, `.git`,
   `node_modules`, cachés y directorios temporales.
2. **Un escaneo de contenido** sobre cada archivo *guardado*. El material de clave privada se rehúsa
   a la creación a menos que pases `--allow-findings` (y luego se marca como
   HARD en `REDACTION.md`). Los tokens probables (IDs de clave de acceso de AWS, tokens de GitHub,
   claves `sk-…`, asignaciones de estilo `api_key=`) se advierten pero
   no se bloquean.

El escaneo es heurístico. **Revisa `REDACTION.md` (o `handoff inspect`) antes de
compartir una instantánea** — las sesiones y contexto de trabajo viajan por diseño y pueden
citar cualquier cosa que hayas pegado en una conversación de agente. Las instantáneas se escriben
modo 0600 por esta razón; trátales como archivos privados.

## Entrega de Git

```sh
agentmod pack --for-git    # escribe .agentmod-handoff/ en la raíz del proyecto
git add .agentmod-handoff && git commit
```

Los mismos seis miembros y carga como un `.amod`, pero como un árbol comprometible de
archivos simples (`shasum -a 256 -c checksums.txt` funciona en el directorio). Además de
las exclusiones predeterminadas elimina **sesiones, transcripciones, historial,
y registros** para los tres agentes — aquellos rutinariamente contienen secretos pegados y
no pertenecen en un repositorio. `--include-sessions` siempre se rehúsa:
comprometer sesiones requeriría encriptación, que esta versión no
implementa. El contexto de trabajo que es seguro compartir (CLAUDE.md, configuraciones de agentes,
habilidades, planes) permanece.

Re-ejecutar reemplaza el paquete anterior; nada más en el repositorio es
tocado.

## Advertencias de restauración

`handoff restore` / `unpack` trata cada instantánea como entrada no confiable:

- verificación completa de suma de comprobación e inventario cruzado primero;
- plan de seguridad de ruta: zip-slip (`..`), rutas absolutas, letras de unidad,
  objetivos no `.agentmod`, nombres protegidos (`.git`, `.ssh`, `.aws`,
  `.docker`), y escapar o objetivos de enlaces simbólicos absolutos todos se rechazan
  antes de que se escriba algo;
- el `.agentmod/` existente se renombra a `.agentmod.backup-<marca>` antes de
  la extracción; cualquier fallo se revierte automáticamente a él;
- **nada de una instantánea se ejecuta jamás**;
- después: el gancho guard de Claude se vuelve a conectar al binario *de esta* máquina,
  rutas absolutas específicas de máquina encontradas en configuraciones de agentes restauradas se
  advierten sobre (tus archivos nunca se reescriben), `doctor` se ejecuta en línea, y
  los pasos de re-inicio requeridos se imprimen (la autenticación nunca viaja).

Las restauraciones se rehúsan en lugar de adivinar — una restauración rechazada deja el proyecto
idéntico en bytes.

## `agentmod doctor`

Diagnóstico de solo lectura, seguro de ejecutar en cualquier momento (salida 0 limpio, 3 con hallazgos):
estado del proyecto/configuración/diseño, instalación y vitalidad del gancho de shell, cambio de
enrutamiento, variables persistentes fuera de proyectos, entradas de PATH duplicadas,
violaciones de HOME/shim, presencia de autenticación por agente con instrucciones de re-inicio,
advertencias de filtración de OpenCode, estado global/proyecto de gstack, cableado guard de Claude,
riesgos de portabilidad en configuraciones restauradas, candidatos secretos registrados en
instantáneas existentes, material de sesión/registro dentro de `.agentmod-handoff/`, y
si el HEAD del repositorio aún coincide con el de la instantánea más nueva.

## El guard de Bash de Claude

`agentmod init` registra `agentmod guard claude-bash` como un gancho PreToolUse de Claude Code
en el hogar local del proyecto. Bloquea comandos de Bash que
escribirían en los hogares globales del agente (`~/.claude`, `~/.codex`,
`~/.config/opencode`, `~/.local/share/opencode`), usan `sudo`, o reasignan
`HOME` — el agente recibe la razón y puede ajustar. Las lecturas nunca
se bloquean. Es una heurística de análisis de shell de profundidad: guardia útil, no una
caja de arena.

## Limitaciones conocidas

Sección de honestidad. Estas son propiedades de las herramientas subyacentes o alcance MVP deliberado
— `doctor` y la documentación generada también los establecen.

- **Llavero macOS (Claude).** Claude Code en macOS almacena credenciales OAuth
  en el Llavero, compartidas en *todos* los directorios de configuración. El aislamiento
  de cuentas por proyecto es imposible en macOS — y no se necesita re-inicio por proyecto.
  Linux/Windows usan un `.credentials.json` por hogar, que aísla pero
  requiere inicio de sesión/copia por proyecto.
- **OpenCode está parcialmente aislado de forma predeterminada.** OpenCode no tiene una única
  variable de hogar; su configuración es una cadena de fusión que aún lee la
  `~/.config/opencode/opencode.json` global, y sesiones/almacenamiento/autenticación viven en
  directorios de datos XDG globales. `opencode.xdg_full_isolation = true` enruta las
  variables XDG para aislamiento completo — pero eso afecta *todas* las herramientas
  conscientes de XDG que ejecutas dentro del proyecto. `doctor` reporta ambas situaciones.
- **El `.claude/` del proyecto es comportamiento nativo de Claude.** Claude Code siempre
  lee `./.claude/` independientemente de `CLAUDE_CONFIG_DIR`. El valor agregado de agentmod
  para Claude es aislar *estado a nivel de usuario* (habilidades/complementos globales,
  sesiones, historial); el `.claude/` del proyecto ya funcionaba antes de agentmod.
- **Activación del gancho de primera sesión.** Justo después de `agentmod init`, el shell
  que ya se ejecuta no ha cargado el nuevo bloque rc. Abre una nueva
  terminal, `exec $SHELL`, o `eval "$(agentmod hook zsh)"` de un disparo único (init
  imprime exactamente esto). Del mismo modo, el gancho bash se dispara vía `PROMPT_COMMAND`
  y por lo tanto es inerte en scripts de bash no interactivos (misma clase de
  limitación que direnv) — los scripts deben establecer las variables explícitamente vía
  `eval "$(agentmod env --shell bash --activate <raíz>)"` si necesitan
  enrutamiento.
- **Solo el bin global de npm está en PATH.** `.agentmod/node/bin` es la única entrada
  de PATH administrada. Las instalaciones globales de pnpm/bun se enrutan en el proyecto
  (`PNPM_HOME`, `BUN_INSTALL`) pero sus directorios de bin no se añaden a PATH.
- **Las herramientas de árbol se restauran manualmente.** `handoff restore` acepta archivos `.amod`
  solo; un directorio `.agentmod-handoff/` comprometido se restaura por
  seguir el `RESTORE.md` dentro de él (esta versión no tiene lector de directorio).
- **Las instantáneas pueden necesitar reparación post-restauración.** El clon de gstack viaja
  sin su `.git` (re-ejecuta `agentmod install gstack --force` para hacerlo
  actualizable de nuevo), y los enlaces simbólicos del lanzador `node/bin` cuelgan porque
  `node_modules` se excluye (re-ejecuta `npm install -g …` dentro del
  proyecto).
- **La compatibilidad de shell es zsh y bash.** Otros shells aún pueden usar
  `agentmod env` manualmente.

## Preguntas frecuentes

**¿Sigo usando `claude` / `codex` / `opencode` directamente?**
Sí. Ese es el punto — sin envoltorios, sin shims, sin `agentmod run`.

**¿Por qué agentmod no simplemente cambia `HOME`?**
Reasignar `HOME` rompe SSH, git, llaveros, dotfiles, y cada otra
herramienta en el shell. agentmod enruta solo las variables específicas del agente.

**¿Por qué falta mi autenticación después de una restauración?**
Por diseño — las credenciales nunca viajan en instantáneas. Sigue las líneas de
re-inicio impresas (o la oferta de copia de init) en la máquina nueva.

**¿Puedo comprometer `.agentmod/` en git?**
No — init lo gitignora (sesiones, cachés, y posiblemente auth copiado viven
allí). Compromete el subconjunto seguro en su lugar: `agentmod pack --for-git`.

**¿Cómo es esto diferente de direnv?**
Mismo modelo de activación (env con alcance de directorio, basado en gancho de indicador, restauración perfecta
en salida), pero agentmod también sabe *qué* enrutar para cada agente,
crea los hogares, protege contra escrituras globales, y hace entrega. Los dos
coexisten bien.

**Una instantánea falla al crear con "hallazgos de candidatos secretos".**
El escaneo de contenido encontró material de clave privada en un archivo guardado. Elimínalo (o
muévelo a una ubicación excluida como `.env`), o empaqueta de todos modos con
`--allow-findings` si lo aceptas dentro de la instantánea.

**¿Funciona en Windows?**
El código Go se construye y la seguridad de ruta se aplica para rutas de estilo Windows, pero
los ganchos de shell apuntan a zsh/bash; Windows no se prueba en esta versión.

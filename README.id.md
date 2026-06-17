# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Isolasi per-proyek dan penyerahan untuk agen pengkodean.

`agentmod` menjaga konfigurasi, keterampilan, plugin, sesi, cache, dan
konteks kerja **Claude Code**, **Codex CLI**, dan **OpenCode** di dalam
proyek yang Anda kerjakan — dan mengemas lingkungan tersebut ke dalam
snapshot yang dapat Anda serahkan ke mesin lain.

Ini memainkan dua peran:

1. **Router Rumah Agen.** Di dalam pohon direktori yang berisi
   `.agentmod/agentmod.toml`, hook shell mengarahkan rumah setiap agen ke
   `.agentmod/`. Di luar, setiap variabel dipulihkan persis seperti sebelumnya
   dan pengaturan global Anda tidak tersentuh.
2. **Alat Penyerahan.** `agentmod handoff create` mengemas `.agentmod/` ke
   dalam snapshot `.amod` yang dapat diverifikasi (atau, dengan `--for-git`,
   pohon file yang dapat di-commit di bawah `.agentmod-handoff/`). **Git
   memindahkan sumber Anda; agentmod memindahkan lingkungan agen.**

## Apa yang agentmod *bukan*

- **Bukan sandbox Docker.** Ini mengarahkan variabel lingkungan di shell Anda
  sendiri. Tidak ada kontainer, tidak ada VM, tidak ada pemfilteran syscall.
- **Bukan isolasi keamanan penuh.** Alat yang mengabaikan variabel yang
  diarahkan masih dapat menjangkau rumah global Anda. Penjaga Bash Claude
  (di bawah) adalah pertahanan berlapis, bukan batasan keamanan.
- **Bukan shim.** Itu tidak pernah mengintersepsi atau membungkus perintah
  `claude`, `codex`, atau `opencode`. Anda terus menjalankannya langsung,
  tanpa modifikasi.
- **Bukan alat pengubah HOME.** `HOME` tidak pernah ditugaskan kembali.
- **Bukan alat pencadangan kode sumber.** Snapshot tidak pernah menyertakan
  kode sumber Anda secara default. Gunakan git untuk sumber.

## Cara kerjanya

`agentmod hook zsh` / `agentmod hook bash` mencetak fungsi shell yang kecil
dan berdiri sendiri (diinstal ke dalam file rc Anda oleh `agentmod init`).
Pada setiap prompt dan perubahan direktori, itu berjalan ke atas mencari
`.agentmod/agentmod.toml`:

- **Memasuki proyek** menyimpan nilai saat ini dan menetapkan:

  | Variabel | Diarahkan ke |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **hanya** dengan `opencode.xdg_full_isolation = true` |

  `PATH` mendapatkan tepat satu entri, `.agentmod/node/bin` (bin global npm
  di bawah awalan yang diarahkan). Variabel pembukuan (`AGENTMOD_ACTIVE`,
  `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`,
  `AGENTMOD_SAVED_*`) mencatat apa yang akan dibatalkan.

- **Meninggalkan proyek** memulihkan setiap nilai yang disimpan dan menghapus
  entri `PATH` — kebalikan yang sempurna. Beralih langsung antara dua proyek
  agentmod mengarahkan ulang dalam satu langkah tanpa kebocoran jalur proyek
  mana pun.

Perutean per agen dapat dimatikan dalam `agentmod.toml`
(`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Instalasi

Pilih yang paling sesuai dengan pengaturan Anda — masing-masing menginstal
biner tunggal yang sama:

```sh
# npm (menginstal biner pra-bangun untuk platform Anda)
npm install -g agentmod

# skrip instalasi (mengunduh rilis yang cocok, memverifikasi sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (memerlukan toolchain Go)
go install github.com/mojomoth/agentmod@latest
```

Atau bangun dari sumber (Go 1.26+, satu-satunya dependensi modul adalah
`BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# letakkan biner di suatu tempat di PATH Anda
```

## Memulai dengan cepat

```sh
cd ~/work/myproject
agentmod init          # membuat .agentmod/, mengedit .gitignore, menginstal
                       # hook shell ke file rc Anda, menawarkan untuk menyalin auth
# pertama kali saja: hook tidak aktif di shell INI belum —
# buka terminal baru, atau: exec $SHELL

cd ~/work/myproject    # hook diaktifkan; periksa:
agentmod status        # "AgentMod: active", jalur yang diarahkan terdaftar
claude                 # perintah biasa — sekarang menggunakan rumah lokal proyek
agentmod install gstack   # keterampilan lokal proyek, rumah global tidak tersentuh

agentmod pack          # snapshot ke .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # diagnosa hanya-baca kapan saja
```

Di mesin penerima:

```sh
cd ~/work/myproject    # sumber tiba melalui git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# ikuti catatan re-login yang dicetak; doctor berjalan otomatis
```

## `agentmod init`

Idempoten — menjalankan ulang mengisi apa pun yang hilang dan tidak pernah
menimpa `agentmod.toml` yang ada atau file pengguna apa pun. Ini:

- membuat `.agentmod/{claude,codex,opencode,node,snapshots,logs}` dan
  `agentmod.toml` default;
- memasang penjaga Bash Claude ke dalam `.agentmod/claude/settings.json`;
- menambahkan `.agentmod/` ke `.gitignore` (dibuat hanya di dalam repositori
  git);
- menginstal hook shell sebagai blok yang dibatasi dalam `~/.zshrc` atau
  `~/.bashrc` (shell Anda dari `$SHELL`; blok diperbarui di tempat, tidak
  pernah diduplikasi, dan konten rc Anda sendiri tidak pernah tersentuh);
- menawarkan untuk **menyalin** file auth Claude/Codex yang ada ke rumah
  lokal proyek (lihat "Auth" di bawah) — penyalinan hanya terjadi pada `y`
  yang eksplisit.

Bendera: `--no-shell-hook` melewati semua pengeditan file rc; `--yes` /
`--non-interactive` tidak pernah menampilkan prompt dan oleh karena itu tidak
pernah menyalin auth (untuk CI).

## Menggunakan `claude`, `codex`, `opencode` biasa

Tidak ada perintah wrapper. Di dalam proyek aktif, perintah biasa hanya
melihat rumah yang diarahkan:

- **Claude Code** membaca `CLAUDE_CONFIG_DIR` → pengaturan lokal proyek,
  keterampilan/plugin tingkat pengguna, sesi, riwayat. (Proyek `.claude/`
  *selalu* dibaca secara native — lihat Keterbatasan.)
- **Codex CLI** membaca `CODEX_HOME` → `config.toml` lokal proyek,
  `auth.json`, sesi, riwayat, log.
- **OpenCode** membaca `OPENCODE_CONFIG` → file konfigurasi lokal proyek.
  Ini adalah isolasi *parsial* secara default — lihat Keterbatasan.

### Auth

Rumah lokal proyek segar dimulai tanpa kredensial:

- **Claude di macOS**: tidak ada yang perlu dilakukan — kredensial tinggal di
  Keychain dan dibagikan dengan setiap direktori konfigurasi (yang juga
  berarti mereka *tidak* diisolasi per proyek).
- **Claude di Linux/Windows**: jalankan `claude login` di dalam proyek, atau
  terima penawaran init untuk menyalin `~/.claude/.credentials.json`.
- **Codex**: jalankan `codex login` di dalam proyek, atau terima penawaran
  init untuk menyalin `~/.codex/auth.json`.

File auth **tidak pernah bepergian dalam snapshot** (dikecualikan berdasarkan
nama, terlepas dari cara mereka sampai di sana).

## Instalasi gstack

[gstack](https://github.com/garrytan/gstack) hardcodes installernya ke
`~/.claude/skills/gstack` — tepat polusi global yang agentmod ada untuk
mencegah. Jadi:

```sh
agentmod install gstack            # clone ke .agentmod/claude/skills/gstack
agentmod install gstack --force    # ganti install lokal proyek yang ada
```

Installer melakukan clone dengan git, tidak pernah menjalankan skrip setup
gstack sendiri, dan snapshot daftar `~/.claude/skills` sebelum dan sesudah —
perubahan apa pun ke direktori global dilaporkan sebagai pelanggaran dan
menyebabkan perintah gagal. `agentmod doctor` secara terpisah memperingatkan
kapan pun install gstack *global* ada (bahkan yang Anda instal sendiri
sebelum mengadopsi agentmod), karena keterampilan yang diinstal secara global
bocor ke setiap proyek.

## Penyerahan (snapshot `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + laporan redaksi, tanpa ekstraksi
agentmod handoff verify  FILE      # hash ulang setiap anggota; keluar 3 pada ketidakcocokan
agentmod handoff restore FILE      # ganti .agentmod/ (cadangan dibuat terlebih dahulu)
agentmod pack / agentmod unpack    # alias dari create / restore
```

Snapshot adalah zip dengan enam anggota root — `manifest.json`,
`inventory.json` (ukuran/sha256/mode per file), `REDACTION.md` (apa yang
dikecualikan dan mengapa, ditambah temuan pemindaian rahasia), `HANDOFF.md`
dan `RESTORE.md` (instruksi manusia untuk penerima), `checksums.txt`
(`shasum -a 256 -c`-compatible) — dan muatan di bawah
`payload/.agentmod/…`. Pembuatan adalah atomik dan deterministik; manifest
merekam cabang/komit/status kotor git dengan kredensial apa pun yang
dilepas dari URL jarak jauh. Worktree yang kotor menolak untuk pack kecuali
`--allow-dirty`.

`inspect` dan `verify` bekerja di mana saja — penerima dapat mengaudit
snapshot sebelum memiliki proyek apa pun yang disiapkan.

### Kebijakan pengecualian rahasia

Dua lapisan, keduanya aktif secara default:

1. **Aturan pengecualian** menghapus file sensitif yang diketahui dari muatan
   dan mencantumkan masing-masing dalam `REDACTION.md`: file auth berdasarkan
   nama (`.credentials.json`, `auth.json`, `credentials*`), `*.env` /
   `.env.*`, kunci SSH (`id_*`, `*.pem`, `*.pub`), direktori kredensial
   (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`),
   file keychain, `.git`, `node_modules`, cache dan direktori temp.
2. **Pemindaian konten** di setiap file *yang disimpan*. Material kunci
   pribadi menolak pembuatan dengan jelas kecuali Anda melewatkan
   `--allow-findings` (dan kemudian ditandai HARD dalam `REDACTION.md`).
   Token yang mungkin (AWS access key ID, token GitHub, `sk-…` keys,
   penugasan gaya `api_key=`) dipperingatkan tetapi tidak memblokir.

Pemindaian bersifat heuristik. **Tinjau `REDACTION.md` (atau `handoff
inspect`) sebelum berbagi snapshot** — sesi dan konteks kerja bepergian
dengan desain dan mungkin mengutip apa pun yang Anda tempel ke percakapan
agen. Snapshot ditulis mode 0600 untuk alasan ini; perlakukan seperti file
pribadi.

## Penyerahan Git

```sh
agentmod pack --for-git    # menulis .agentmod-handoff/ di root proyek
git add .agentmod-handoff && git commit
```

Sama enam anggota dan muatan seperti `.amod`, tetapi sebagai pohon file
yang dapat di-commit dari file biasa (`shasum -a 256 -c checksums.txt`
bekerja di direktori). Di atas pengecualian default, ia menghapus **sesi,
transkrip, riwayat, dan log** untuk ketiga agen — yang secara rutin berisi
rahasia yang ditempel dan tidak termasuk dalam repositori. `--include-sessions`
selalu menolak: melakukan commit sesi memerlukan enkripsi, yang versi ini
tidak menerapkan. Konteks kerja yang aman untuk dibagikan (CLAUDE.md, konfigurasi
agen, keterampilan, rencana) tetap ada.

Menjalankan ulang mengganti paket sebelumnya; tidak ada yang lain di repo
yang tersentuh.

## Peringatan pemulihan

`handoff restore` / `unpack` memperlakukan setiap snapshot sebagai input yang
tidak terpercaya:

- verifikasi checksum penuh dan cross-check inventori terlebih dahulu;
- rencana keamanan jalur: zip-slip (`..`), jalur absolut, huruf drive,
  target non-`.agentmod`, nama yang dilindungi (`.git`, `.ssh`, `.aws`,
  `.docker`), dan penghindaran atau target symlink absolut semuanya ditolak
  sebelum apa pun ditulis;
- `.agentmod/` yang ada diganti nama ke `.agentmod.backup-<stamp>` sebelum
  ekstraksi; kegagalan apa pun kembali ke sana secara otomatis;
- **tidak ada yang dari snapshot pernah dieksekusi**;
- sesudahnya: hook penjaga Claude dialihkan kembali ke biner *mesin ini*,
  jalur absolut khusus mesin yang ditemukan dalam konfigurasi agen yang
  dipulihkan diperingatkan tentang (file Anda tidak pernah ditulis ulang),
  `doctor` berjalan inline, dan langkah re-login yang diperlukan dicetak
  (auth tidak pernah bepergian).

Pemulihan menolak daripada menebak — pemulihan yang ditolak meninggalkan
proyek byte-identical.

## `agentmod doctor`

Diagnosa hanya-baca, aman dijalankan kapan saja (keluar 0 bersih, 3 dengan
temuan): proyek/konfigurasi/keadaan tata letak, instalasi dan daya hidup
hook shell, perpindahan perutean, variabel yang tertinggal di luar proyek,
entri PATH duplikat, pelanggaran HOME/shim, kehadiran auth per-agen dengan
instruksi re-login, peringatan kebocoran OpenCode, keadaan gstack
global/proyek, pengawatan penjaga Claude, risiko portabilitas dalam konfigurasi
yang dipulihkan, kandidat rahasia yang dicatat dalam snapshot yang ada,
materi sesi/log di dalam `.agentmod-handoff/`, dan apakah HEAD repositori
masih cocok dengan snapshot terbaru.

## Penjaga Bash Claude

`agentmod init` mendaftarkan `agentmod guard claude-bash` sebagai hook
PreToolUse Claude Code dalam rumah lokal proyek. Ini memblokir perintah Bash
yang akan menulis ke rumah agen global (`~/.claude`, `~/.codex`,
`~/.config/opencode`, `~/.local/share/opencode`), gunakan `sudo`, atau
tugaskan ulang `HOME` — agen mendapatkan alasannya kembali dan dapat
menyesuaikan. Pembacaan tidak pernah diblokir. Itu adalah heuristik parse
shell satu tingkat dalam: guardrail yang berguna, bukan sandbox.

## Keterbatasan yang diketahui

Bagian kejujuran. Ini adalah properti dari alat yang mendasar atau cakupan
MVP yang disengaja — `doctor` dan doc yang dihasilkan juga menyatakannya.

- **Keychain macOS (Claude).** Claude Code di macOS menyimpan kredensial OAuth
  di Keychain, dibagikan di seluruh *semua* direktori konfigurasi. Isolasi
  akun per-proyek tidak mungkin di macOS — dan tidak ada re-login yang
  diperlukan per proyek. Linux/Windows menggunakan `.credentials.json`
  per-rumah, yang mengisolasi tetapi memerlukan login/copy per proyek.
- **OpenCode sebagian diisolasi secara default.** OpenCode tidak memiliki
  variabel rumah tunggal; konfigurasinya adalah rantai penggabungan yang
  masih membaca `~/.config/opencode/opencode.json` global, dan sesi/penyimpanan/auth
  hidup di direktori data XDG global. `opencode.xdg_full_isolation = true`
  mengarahkan variabel XDG untuk isolasi penuh — tetapi itu mempengaruhi
  *setiap* alat yang menyadari XDG yang Anda jalankan di dalam proyek.
  `doctor` melaporkan kedua situasi.
- **Proyek `.claude/` adalah perilaku Claude asli.** Claude Code selalu
  membaca `./.claude/` terlepas dari `CLAUDE_CONFIG_DIR`. nilai tambah
  agentmod untuk Claude mengisolasi keadaan *tingkat pengguna* (keterampilan/plugin
  global, sesi, riwayat); proyek `.claude/` sudah bekerja sebelum agentmod.
- **Aktivasi hook sesi pertama.** Tepat setelah `agentmod init`, shell yang
  sudah berjalan belum memuat blok rc baru. Buka terminal baru, `exec $SHELL`,
  atau one-shot `eval "$(agentmod hook zsh)"` (init mencetak persis ini).
  Demikian pula, hook bash menyala melalui `PROMPT_COMMAND` dan oleh karena
  itu inert dalam skrip bash non-interaktif (kelas keterbatasan yang sama
  seperti direnv) — skrip harus menetapkan variabel secara eksplisit melalui
  `eval "$(agentmod env --shell bash --activate <root>)"` jika mereka
  memerlukan perutean.
- **Hanya bin global npm yang ada di PATH.** `.agentmod/node/bin` adalah
  entri PATH yang dikelola tunggal. Install global pnpm/bun diarahkan ke
  proyek (`PNPM_HOME`, `BUN_INSTALL`) tetapi direktori bin mereka tidak
  ditambahkan ke PATH.
- **Paket pohon dipulihkan secara manual.** `handoff restore` menerima file
  `.amod` hanya; direktori `.agentmod-handoff/` yang di-commit dipulihkan
  dengan mengikuti `RESTORE.md` di dalamnya (versi ini tidak memiliki pembaca
  direktori).
- **Snapshot mungkin memerlukan perbaikan pasca-pemulihan.** Clone gstack
  bepergian tanpa `.git`-nya (jalankan ulang `agentmod install gstack
  --force` untuk membuatnya dapat diperbarui lagi), dan symlink peluncur
  `node/bin` menggantung karena `node_modules` dikecualikan (jalankan ulang
  `npm install -g …` di dalam proyek).
- **Dukungan shell adalah zsh dan bash.** Shell lain masih dapat menggunakan
  `agentmod env` secara manual.

## FAQ

**Apakah saya terus menggunakan `claude` / `codex` / `opencode` secara langsung?**
Ya. Itulah intinya — tidak ada wrapper, tidak ada shim, tidak ada `agentmod run`.

**Mengapa agentmod tidak hanya mengubah `HOME`?**
Menetapkan ulang `HOME` merusak SSH, git, keychain, dotfile, dan setiap alat
lain dalam shell. agentmod hanya mengarahkan variabel khusus agen.

**Mengapa auth saya hilang setelah pemulihan?**
Sesuai desain — kredensial tidak pernah bepergian dalam snapshot. Ikuti
baris re-login yang dicetak (atau penawaran copy init) di mesin baru.

**Bisakah saya melakukan commit `.agentmod/` ke git?**
Tidak — init gitignores itu (sesi, cache, dan auth yang mungkin disalin
tinggal di sana). Commit subset yang aman sebagai gantinya: `agentmod pack
--for-git`.

**Bagaimana ini berbeda dari direnv?**
Model aktivasi yang sama (env dengan cakupan direktori, berbasis prompt-hook,
pemulihan sempurna pada keluar), tetapi agentmod juga tahu *apa* yang harus
diarahkan untuk setiap agen, membuat rumah, menjaga terhadap penulisan global,
dan melakukan penyerahan. Keduanya hidup berdampingan dengan baik.

**Snapshot gagal dibuat dengan temuan "kandidat rahasia".**
Pemindaian konten menemukan material kunci pribadi dalam file yang disimpan.
Hapusnya (atau pindahkan ke lokasi yang dikecualikan seperti `.env`), atau
pack meskipun dengan `--allow-findings` jika Anda menerimanya berada dalam
snapshot.

**Apakah ini berfungsi di Windows?**
Kode Go membangun dan keamanan jalur ditegakkan untuk jalur gaya Windows,
tetapi hook shell menargetkan zsh/bash; Windows tidak teruji dalam versi ini.

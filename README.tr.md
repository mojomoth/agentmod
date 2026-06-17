# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Kodlama aracıları için proje başına yalıtım ve devri.

`agentmod`, **Claude Code**, **Codex CLI** ve **OpenCode**'un yapılandırması, becerilerileri, eklentileri, oturumları, önbellekleri ve çalışma bağlamını çalıştığınız proje içinde tutar — ve başka bir makineye teslim edebileceğiniz bir anlık görüntüye paketler.

İki rol oynar:

1. **Ajan Ev Yönlendiricisi.** `.agentmod/agentmod.toml` içeren bir dizin ağacında, bir kabuk kancası her ajanın evini `.agentmod/` içine yönlendirir. Dışında, her değişken tam olarak önceki haline geri yüklenir ve global kurulumunuz dokunulmaz kalır.
2. **Devri Aracı.** `agentmod handoff create`, `.agentmod/` öğesini doğrulanabilir bir `.amod` anlık görüntüsüne paketler (veya `--for-git` ile, `.agentmod-handoff/` altında işlenebilir bir dosya ağacına). **Git kaynak kodunuzu hareket ettirir; agentmod ajan ortamını hareket ettirir.**

## agentmod *değildir*

- **Docker sandboxı değil.** Ortam değişkenlerini kendi kabuğunuzda yönlendirir. Hiç konteyner, VM veya syscall filtrelemesi yoktur.
- **Tam güvenlik yalıtımı değil.** Yönlendirilmiş değişkenleri yok sayan bir araç yine de global evlere ulaşabilir. Claude Bash koruması (aşağıda) derinlik savunmasıdır, güvenlik sınırı değildir.
- **Shim değil.** `claude`, `codex` veya `opencode` komutlarını asla engellemez veya sarmalamaz. Onları doğrudan ve değiştirilmeden çalıştırmaya devam edersiniz.
- **HOME değiştiren araç değil.** `HOME` hiçbir zaman yeniden atanmaz.
- **Kaynak kodu yedekleme aracı değil.** Anlık görüntüler varsayılan olarak kaynak kodunuzu asla içermez. Kaynak için git kullanın.

## Nasıl çalışır

`agentmod hook zsh` / `agentmod hook bash`, `agentmod init` tarafından rc dosyanıza yüklenen küçük, kendi içinde yeterli bir kabuk işlevi yazdırır. Her isteğe ve dizin değişikliğinde, `.agentmod/agentmod.toml` aramak için yukarı doğru yürür:

- **Bir projeye giriş** mevcut değerleri kaydeder ve ayarlar:

  | Değişken | Yönlendirme Hedefi |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **yalnızca** `opencode.xdg_full_isolation = true` ile |

  `PATH` tam olarak bir giriş kazanır, `.agentmod/node/bin` (yönlendirilmiş önek altındaki npm'nin global bin). Muhasebe değişkenleri (`AGENTMOD_ACTIVE`, `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`, `AGENTMOD_SAVED_*`) geri almak için neyin olduğunu kaydeder.

- **Projeden çıkış** kaydedilen her değeri geri yükler ve `PATH` girişini kaldırır — mükemmel bir ters işlem. İki agentmod projesi arasında doğrudan geçiş, her iki projenin yollarını sızdırmadan tek adımda yönlendirir.

Ajan başına yönlendirme `agentmod.toml` (`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`) içinde kapatılabilir.

## Yükleme

Kurulumunuza uygun olanı seçin — her biri aynı tek ikili dosyayı yükler:

```sh
# npm (platformunuz için önceden oluşturulmuş ikiliyi yükler)
npm install -g agentmod

# kurulum betiği (eşleşen sürümü indirir, sha256 doğrular)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (Go araç zincirini gerektirir)
go install github.com/mojomoth/agentmod@latest
```

Veya kaynaktan derleyin (Go 1.26+, tek modül bağımlılığı `BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# ikiliyi PATH'inizde bir yere koyun
```

## Hızlı başlangıç

```sh
cd ~/work/myproject
agentmod init          # .agentmod/ oluşturur, .gitignore düzenler, kabuğu yükler
                       # rc dosyanıza kancası, auth kopyası sunmayı teklif eder
# yalnızca ilk kez: bu kabukta kanca henüz canlı değil —
# yeni bir terminal açın veya: exec $SHELL

cd ~/work/myproject    # kanca etkinleşir; kontrol edin:
agentmod status        # "AgentMod: active", yönlendirilen yollar listelenir
claude                 # düz komut — şimdi proje-yerel evi kullanıyor
agentmod install gstack   # proje-yerel beceriler, global ev dokunulmaz

agentmod pack          # .agentmod/snapshots/<name>-<stamp>.amod adresine anlık görüntü
agentmod doctor        # herhangi bir zamanda salt okunur teşhis
```

Alan makinesinde:

```sh
cd ~/work/myproject    # kaynak git yoluyla ulaştı
agentmod init
agentmod unpack myproject-20260611-123045.amod
# yazdırılan yeniden giriş notlarını izleyin; doktor otomatik olarak çalışır
```

## `agentmod init`

İdempotent — yeniden çalıştırmak eksik olanı doldurur ve mevcut `agentmod.toml` veya herhangi bir kullanıcı dosyasının üzerine asla yazmaz. Şunları yapar:

- `.agentmod/{claude,codex,opencode,node,snapshots,logs}` ve varsayılan `agentmod.toml` oluşturur;
- Claude Bash korumasını `.agentmod/claude/settings.json` içine bağlar;
- `.agentmod/` öğesini `.gitignore` öğesine ekler (yalnızca bir git deposu içinde oluşturulur);
- kabuk kancasını `~/.zshrc` veya `~/.bashrc` içine bir sınırlandırılmış blok olarak yükler (kabuğunuz `$SHELL` öğesinden; blok yerinde güncellenir, hiçbir zaman çoğaltılmaz ve kendi rc içeriğiniz hiçbir zaman dokunulmaz);
- mevcut Claude/Codex auth dosyalarını proje-yerel eve **kopyalamayı** teklif eder (aşağıya bakın "Auth") — kopyalama yalnızca açık `y` değerinde gerçekleşir.

Bayraklar: `--no-shell-hook` tüm rc-dosya düzenlemelerini atlar; `--yes` / `--non-interactive` hiçbir zaman sorular soramaz ve bu nedenle asla auth kopyalamaz (CI için).

## Düz `claude`, `codex`, `opencode` kullanma

Hiç sarmalayıcı komut yoktur. Etkin bir proje içinde adi komutlar basitçe yönlendirilen evleri görür:

- **Claude Code**, `CLAUDE_CONFIG_DIR` → proje-yerel ayarları, kullanıcı düzeyindeki beceriler/eklentiler, oturumlar, geçmiş okur. (Proje düzeyindeki `.claude/`, *her zaman* doğal olarak okunur — bkz. Sınırlamalar.)
- **Codex CLI**, `CODEX_HOME` → proje-yerel `config.toml`, `auth.json`, oturumlar, geçmiş, günlükleri okur.
- **OpenCode**, `OPENCODE_CONFIG` → proje-yerel yapılandırma dosyasını okur. Bu, varsayılan olarak *kısmi* yalıtımdır — bkz. Sınırlamalar.

### Auth

Taze proje-yerel evler kimlik bilgileri olmadan başlar:

- **Claude on macOS**: yapılacak bir şey yok — kimlik bilgileri Keychain'de yaşar ve her yapılandırma dizini ile paylaşılır (bu aynı zamanda *proje başına yalıtılmadığı* anlamına gelir).
- **Claude on Linux/Windows**: projede `claude login` öğesini çalıştırın veya init'in `~/.claude/.credentials.json` kopyalamayı teklif etmesini kabul edin.
- **Codex**: projede `codex login` öğesini çalıştırın veya init'in `~/.codex/auth.json` kopyalamayı teklif etmesini kabul edin.

Auth dosyaları **hiçbir zaman anlık görüntülerde seyahat etmezler** (nasıl oraya ulaştıklarına bakılmaksızın ad tarafından hariç tutulmuştur).

## gstack kurulumu

[gstack](https://github.com/garrytan/gstack) kurulumunu `~/.claude/skills/gstack` adresine kodlar — agentmod'un var olmasını önlemek için tam olarak global kirlilik. Yani:

```sh
agentmod install gstack            # .agentmod/claude/skills/gstack içine klon
agentmod install gstack --force    # mevcut proje-yerel kurulumu değiştir
```

Kurulum git ile klonlanır, hiçbir zaman gstack'in kendi kurulum betiğini çalıştırmaz ve `~/.claude/skills` taramasını anlık görüntüler — global dizine yapılan herhangi bir değişiklik ihlal olarak bildirilir ve komutu başarısız kılar. `agentmod doctor`, ayrı olarak bir *global* gstack kurulumu var olduğu zaman uyarır (kendinden önce agentmod'u benimsemeden önce yüklediğiniz bir tane olsa bile), çünkü genel yüklenen beceriler her projeye sızar.

## Devri (`.amod` anlık görüntüleri)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + gizlilik raporu, çıkarma yok
agentmod handoff verify  FILE      # her üyeyi yeniden hash; uyuşmazlık durumunda 3'ten çık
agentmod handoff restore FILE      # .agentmod/ öğesini değiştir (yedek alınır)
agentmod pack / agentmod unpack    # create / restore takma adları
```

Anlık görüntü, altı kök üyesi içeren bir zip'tir — `manifest.json`, `inventory.json` (dosya başına boyut/sha256/mod), `REDACTION.md` (hariç tutulmuş ve neden olduğu, artı gizli tarama bulguları), `HANDOFF.md` ve `RESTORE.md` (alıcı için insan talimatları), `checksums.txt` (`shasum -a 256 -c` uyumlu) — ve `payload/.agentmod/…` altındaki yük. Oluşturma atomik ve belirleyicidir; manifest, git dalını/commit/dökülü durumu kaydeder ve herhangi bir kimlik bilgisi uzak URL'den çıkarılır. Kirlenmemiş bir çalışma ağacı `--allow-dirty` olmadan paketlenmesini reddeder.

`inspect` ve `verify` her yerde çalışır — alıcı, herhangi bir proje kurulumu olmadan önce bir anlık görüntüyü denetleyebilir.

### Gizlilik hariç tutma ilkesi

İkisi de varsayılan olarak açıktır:

1. **Hariç tutma kuralları** bilinen duyarlı dosyaları yükten düşürür ve her birini `REDACTION.md` içinde listeler: ad tarafından auth dosyaları (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`, SSH anahtarları (`id_*`, `*.pem`, `*.pub`), kimlik bilgisi dizinleri (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), keychain dosyaları, `.git`, `node_modules`, önbellekler ve geçici dizinler.
2. **Her *tutulmuş* dosya üzerinde bir içerik taraması.** Özel anahtar malzemesi, `--allow-findings` seçeneğini geçmediğiniz sürece oluşturmayı doğrudan reddeder (ve daha sonra REDACTION.md'de HARD olarak işaretlenir). Muhtemel jetonlar (AWS erişim anahtarı kimliği, GitHub jetonları, `sk-…` anahtarları, `api_key=`-tarzı atamalar) uyarılır ancak engellemez.

Tarama buluşsal yöntemdir. **Bir anlık görüntüyü paylaşmadan önce `REDACTION.md` (`handoff inspect`) inceleyin** — oturumlar ve çalışma bağlamı tasarım gereği seyahat eder ve bir ajan konuşmasına yapıştırdığınız herhangi bir şeyi alıntı yapabilir. Anlık görüntüler bu nedenle 0600 modunda yazılır; onları özel dosyalar gibi davranın.

## Git devri

```sh
agentmod pack --for-git    # proje kökünde .agentmod-handoff/ yazar
git add .agentmod-handoff && git commit
```

Bir `.amod` ile aynı altı üye ve yük, ancak işlenebilir bir dosya ağacı (`shasum -a 256 -c checksums.txt` dizinde çalışır). Varsayılan hariç tutmalara ek olarak **oturumları, transkriptleri, geçmişi ve günlükleri** kaldırır — üç ajanın tamamı rutin olarak yapıştırılmış gizlilikleri içerir ve bir depoya ait değildir. `--include-sessions` her zaman reddeder: oturumları işlemek şifreleme gerektirir, bu sürüm uygulamaz. Paylaşılması güvenli olan çalışma bağlamı (CLAUDE.md, ajan yapılandırmaları, beceriler, planlar) kalır.

Önceki paketi yeniden çalıştırmak değiştirir; depodaki başka hiçbir şeye dokunulmaz.

## Geri yükleme ihtiyatları

`handoff restore` / `unpack`, her anlık görüntüyü güvenilmeyen giriş olarak değerlendirir:

- tam sağlama toplamı doğrulaması ve envanter çapraz kontrolü ilk;
- yol-güvenliği planı: zip-slip (`..`), mutlak yollar, sürücü harfleri, `.agentmod` olmayan hedefleri, korunan adlar (`.git`, `.ssh`, `.aws`, `.docker`) ve kaçış veya mutlak sembolik bağlantı hedefleri hiçbir şey yazılmadan önce reddedilir;
- mevcut `.agentmod/`, çıkartma öncesinde `.agentmod.backup-<stamp>` adına yeniden adlandırılır; herhangi bir başarısızlık otomatik olarak geri alır;
- **anlık görüntüden hiçbir şey hiçbir zaman yürütülmez**;
- sonra: Claude koruma kancası bu makinenin ikili dosyasına yeniden bağlanır, geri yüklenen ajan yapılandırmalarında bulunan makineye özgü mutlak yollar uyarılır (dosyalarınız hiçbir zaman yeniden yazılmaz), `doctor` satır içi çalışır ve gerekli yeniden giriş adımları yazdırılır (auth hiçbir zaman seyahat etmez).

Geri yüklemeler tahmin etmek yerine reddeder — reddedilen bir geri yükleme projeyi bayt-özdeş bırakır.

## `agentmod doctor`

Salt okunur teşhis, herhangi bir zaman güvenli bir şekilde çalıştırılır (0 çıkış temiz, 3 bulgular ile): proje/yapılandırma/düzen durumu, kabuk kancası yüklemesi ve canlılığı, yönlendirme sapması, projeler dışında kalan değişkenler, yinelenen PATH girişleri, HOME/shim ihlalleri, ajan başına auth varlığı yeniden giriş talimatları ile, OpenCode sızıntı uyarıları, gstack genel/proje durumu, Claude koruma kablolama, geri yüklenen yapılandırmalardaki taşınabilirlik riskleri, mevcut anlık görüntülerde kaydedilen gizli adayları, `.agentmod-handoff/` içindeki oturum/günlük malzemesi ve depo'nun HEAD yeniden en yeni anlık görüntüsünün eşleşip eşleşmediği.

## Claude Bash koruması

`agentmod init`, `agentmod guard claude-bash` öğesini proje-yerel ev'deki Claude Code PreToolUse kancası olarak kaydeder. Global ajan evlerine (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`) yazacak Bash komutlarını engeller, `sudo` kullanır veya `HOME` — ajan nedeni geri alır ve ayarlanabilir. Okumalar asla engellenmez. Bir kabuk-parse buluşsal yöntemi derinliğindedir: yararlı raylı, sandbox değil.

## Bilinen sınırlamalar

Dürüstlük bölümü. Bunlar, temel araçların özellikleridir veya kasıtlı MVP kapsamıdır — `doctor` ve oluşturulan belgeler onları da belirtir.

- **macOS Keychain (Claude).** macOS'taki Claude Code, OAuth kimlik bilgilerini Keychain'de depolar ve *tüm* yapılandırma dizinleri arasında paylaşılır. macOS'ta proje başına hesap yalıtımı imkansızdır — ve proje başına yeniden giriş gerekli değildir. Linux/Windows, yalıtılan ancak proje başına giriş/kopya gerektiren proje başına `.credentials.json` kullanır.
- **OpenCode varsayılan olarak kısmen yalıtılmıştır.** OpenCode tek bir ev değişkenine sahip değildir; yapılandırması hala genel `~/.config/opencode/opencode.json` okuyan bir birleştirme zinciridir ve oturumlar/depolama/auth genel XDG veri dizinlerinde yaşar. `opencode.xdg_full_isolation = true` tam yalıtım için XDG değişkenlerini yönlendirir — ancak bu, proje içinde çalıştırdığınız *her* XDG-farkındı aracı etkiler. `doctor` her iki durumu da bildirir.
- **Proje `.claude/` doğal Claude davranışıdır.** Claude Code, `CLAUDE_CONFIG_DIR` ne olursa olsun `./.claude/` her zaman okur. agentmod'un Claude için eklenen değeri, *kullanıcı düzeyindeki* durumu (genel beceriler/eklentiler, oturumlar, geçmiş) yalıtmaktır; proje `.claude/` agentmod'dan önce zaten çalışıyordu.
- **İlk oturum kancası aktivasyonu.** `agentmod init` sonra, zaten çalışan kabuk yeni rc bloğunu yüklememiştir. Yeni bir terminal açın, `exec $SHELL` yaptırın veya tek atış `eval "$(agentmod hook zsh)"` (init tam olarak bunu yazdırır). Benzer şekilde, bash kancası `PROMPT_COMMAND` yoluyla ateşlenir ve bu nedenle etkisizdir etkisiz bash betiklerinde (direnv ile aynı sınırlandırma sınıfı) — betikler değişkenleri `eval "$(agentmod env --shell bash --activate <root>)"` aracılığıyla açıkça ayarlamalıdır; yönlendirme gerekiyorsa.
- **Yalnızca npm'nin genel bin PATH'tedir.** `.agentmod/node/bin` yönetilen tek PATH girişidir. pnpm/bun genel kurulumları projeye yönlendirilir (`PNPM_HOME`, `BUN_INSTALL`) ancak bin dizinleri PATH'e eklenmez.
- **Ağaç paketleri el ile geri yüklenir.** `handoff restore`, yalnızca `.amod` dosyalarını kabul eder; işlenmiş `.agentmod-handoff/` dizini içindeki `RESTORE.md` sonra geri yüklenir (bu sürüm dizin okuyucusu yok).
- **Anlık görüntüler geri yükleme onarımı gerekebilir.** gstack klonu `.git` olmadan seyahat eder (güncellenebilir hale getirmek için `agentmod install gstack --force` yeniden çalıştırın) ve `node/bin` başlatıcı sembolik bağlantıları asılı kalır çünkü `node_modules` hariç tutulur (proje içinde `npm install -g …` yeniden çalıştırın).
- **Kabuk desteği zsh ve bash'tir.** Diğer kabuklar yine de `agentmod env` öğesini el ile kullanabilir.

## SSS

**`claude` / `codex` / `opencode` kullanmaya devam mı ederim?**
Evet. Bu mesele — sarmalayıcı yok, shim yok, `agentmod run` yok.

**agentmod neden sadece `HOME` değiştirmiyor?**
`HOME` yeniden atamak SSH, git, keychains, dotfiles ve kabukta diğer her aracı bozar. agentmod yalnızca ajanı özgü değişkenleri yönlendirir.

**Neden kimlik bilgilerim geri yüklendikten sonra kayboldu?**
Tasarım gereği — kimlik bilgileri hiçbir zaman anlık görüntülerde seyahat etmez. Yeni makinede yazdırılan yeniden giriş satırlarını (veya init'in kopya teklifini) izleyin.

**`.agentmod/` öğesini git'e işleyebilir miyim?**
Hayır — init onu gitignores (oturumlar, önbellekler ve muhtemelen kopyalanan auth orada yaşarlar). Bunun yerine güvenli alt kümesini işleyin: `agentmod pack --for-git`.

**Bu direnv'den nasıl farklı?**
Aynı etkinleştirme modeli (dizin-kapsamlı env, isteğe bağlı kancı tabanlı, çıkışta mükemmel geri yükleme), ancak agentmod ayrıca her ajan için *ne* yönlendirileceğini bilir, evleri oluşturur, genel yazmalara karşı koruma sağlar ve devri yapar. İkisi iyi bir şekilde birlikte yaşar.

**Anlık görüntü "gizli-aday bulguları" ile oluşturmayı reddeder.**
İçerik taraması tutulmuş dosyada özel anahtar malzemesi buldu. Kaldırın (veya `.env` gibi hariç tutulan bir konuma taşıyın) veya `--allow-findings` ile paketi yine de paketleyin, eğer anlık görüntünün içinde olması kabul ediyor olabilir.

**Windows'ta çalışıyor mu?**
Go kodu oluşturur ve yol güvenliği Windows tarzı yollar için uygulanır, ancak kabuk kancaları zsh/bash'ı hedefler; Windows bu sürümde test edilmemiştir.

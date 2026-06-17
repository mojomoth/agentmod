# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

कोडिंग एजेंट्स के लिए प्रति-प्रोजेक्ट अलगाव और हैंडऑफ।

`agentmod` **Claude Code**, **Codex CLI**, और **OpenCode** की कॉन्फ़िगरेशन, कौशल, प्लगइन, सेशन, कैश और कार्यशील संदर्भ को आपके काम करने वाली प्रोजेक्ट के अंदर रखता है — और उस पर्यावरण को एक स्नैपशॉट में पैक करता है जो आप दूसरी मशीन को हस्तांतरित कर सकते हैं।

यह दो भूमिकाएं निभाता है:

1. **एजेंट होम राउटर।** `.agentmod/agentmod.toml` युक्त निर्देशिका ट्री के अंदर, एक शेल हुक प्रत्येक एजेंट के होम को `.agentmod/` में रूट करता है। बाहर, हर वेरिएबल बिल्कुल जैसा था वैसा ही पुनर्स्थापित होता है और आपकी वैश्विक सेटअप अछूता रहती है।
2. **हैंडऑफ टूल।** `agentmod handoff create` `.agentmod/` को एक सत्यापन योग्य `.amod` स्नैपशॉट में पैक करता है (या, `--for-git` के साथ, `.agentmod-handoff/` के अंतर्गत एक प्रतिबद्ध फाइल ट्री)। **Git आपके स्रोत को स्थानांतरित करता है; agentmod एजेंट पर्यावरण को स्थानांतरित करता है।**

## agentmod क्या है *नहीं*

- **Docker सैंडबॉक्स नहीं है।** यह अपने शेल में पर्यावरण वेरिएबल को रूट करता है। कोई कंटेनर, कोई VM, कोई syscall फ़िल्टरिंग नहीं है।
- **पूर्ण सुरक्षा अलगाव नहीं है।** एक टूल जो रूट किए गए वेरिएबल को अनदेखा करता है फिर भी आपके वैश्विक होम तक पहुंच सकता है। Claude Bash गार्ड (नीचे) गहराई में बचाव है, सुरक्षा सीमा नहीं है।
- **शिम नहीं है।** यह कभी भी `claude`, `codex`, या `opencode` कमांड को इंटरसेप्ट या लपेटता नहीं है। आप उन्हें सीधे, अपरिवर्तित चलाते रहते हैं।
- **HOME-बदलने वाला टूल नहीं है।** `HOME` को कभी पुनः असाइन नहीं किया जाता है।
- **स्रोत-कोड बैकअप टूल नहीं है।** स्नैपशॉट में डिफ़ॉल्ट रूप से आपका स्रोत कोड कभी शामिल नहीं होता है। स्रोत के लिए git का उपयोग करें।

## यह कैसे काम करता है

`agentmod hook zsh` / `agentmod hook bash` एक छोटा आत्मनिर्भर शेल फ़ंक्शन प्रिंट करता है (`agentmod init` द्वारा आपकी rc फाइल में स्थापित)। हर प्रॉम्प्ट और निर्देशिका परिवर्तन पर यह `.agentmod/agentmod.toml` के लिए ऊपर की ओर चलता है:

- **एक प्रोजेक्ट में प्रवेश करने पर** वर्तमान मानों को सहेजा जाता है और सेट किया जाता है:

  | वेरिएबल | रूट किया गया |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **केवल** `opencode.xdg_full_isolation = true` के साथ |

  `PATH` को बिल्कुल एक प्रविष्टि मिलती है, `.agentmod/node/bin` (रूट किए गए प्रीफिक्स के तहत npm का वैश्विक bin)। बुकीपिंग वेरिएबल (`AGENTMOD_ACTIVE`, `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`, `AGENTMOD_SAVED_*`) रिकॉर्ड करते हैं कि क्या पूर्ववत करना है।

- **प्रोजेक्ट से निकलने पर** हर बचाई गई वेरिएबल को पुनर्स्थापित किया जाता है और `PATH` प्रविष्टि को हटाया जाता है — एक पूर्ण विपरीत। दो agentmod प्रोजेक्ट्स के बीच सीधे स्विच करना बिना किसी प्रोजेक्ट के पथ को लीक किए एक चरण में पुनः रूट करता है।

प्रति-एजेंट रूटिंग को `agentmod.toml` में बंद किया जा सकता है (`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`)।

## इंस्टॉल करें

जो भी आपके सेटअप के लिए उपयुक्त हो वह चुनें — प्रत्येक एक ही एकल बाइनरी स्थापित करता है:

```sh
# npm (आपके प्लेटफॉर्म के लिए पूर्वनिर्मित बाइनरी स्थापित करता है)
npm install -g agentmod

# स्थापन स्क्रिप्ट (मिलान रिलीज को डाउनलोड करता है, sha256 को सत्यापित करता है)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (Go टूलचेन आवश्यक है)
go install github.com/mojomoth/agentmod@latest
```

या स्रोत से बनाएं (Go 1.26+, एकमात्र मॉड्यूल निर्भरता `BurntSushi/toml` है):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# बाइनरी को अपने PATH पर कहीं रखें
```

## त्वरित शुरुआत

```sh
cd ~/work/myproject
agentmod init          # .agentmod/ बनाता है, .gitignore को संपादित करता है, स्थापित करता है
                       # shell hook आपकी rc फाइल में, auth को कॉपी करने की पेशकश करता है
# पहली बार केवल: इस शेल में हुक अभी लाइव नहीं है —
# नया टर्मिनल खोलें, या: exec $SHELL

cd ~/work/myproject    # हुक सक्रिय होता है; जांचिए:
agentmod status        # "AgentMod: सक्रिय", रूट किए गए पथ सूचीबद्ध
claude                 # सादा कमांड — अब प्रोजेक्ट-स्थानीय होम का उपयोग कर रहा है
agentmod install gstack   # प्रोजेक्ट-स्थानीय कौशल, वैश्विक होम अछूता

agentmod pack          # स्नैपशॉट को .agentmod/snapshots/<name>-<stamp>.amod में
agentmod doctor        # कभी भी केवल-पढ़ने योग्य निदान
```

प्राप्त करने वाली मशीन पर:

```sh
cd ~/work/myproject    # स्रोत git के माध्यम से आया
agentmod init
agentmod unpack myproject-20260611-123045.amod
# मुद्रित पुनः-लॉगिन नोट्स का पालन करें; doctor स्वचालित रूप से चलता है
```

## `agentmod init`

Idempotent — पुनः-चलाने से जो भी गायब है भर जाता है और कभी भी किसी मौजूदा `agentmod.toml` या किसी उपयोगकर्ता फाइल को अधिलेखित नहीं करता है। यह:

- `.agentmod/{claude,codex,opencode,node,snapshots,logs}` और एक डिफ़ॉल्ट `agentmod.toml` बनाता है;
- Claude Bash गार्ड को `.agentmod/claude/settings.json` में वायर करता है;
- `.agentmod/` को `.gitignore` में जोड़ता है (केवल एक git रिपॉजिटरी के अंदर बनाया गया);
- शेल हुक को `~/.zshrc` या `~/.bashrc` में एक बद्ध ब्लॉक के रूप में स्थापित करता है (`$SHELL` से आपका शेल; ब्लॉक को जगह पर अपडेट किया जाता है, कभी दोहराया नहीं जाता, और आपकी अपनी rc सामग्री को कभी छुआ नहीं जाता);
- मौजूदा Claude/Codex auth फाइलों को प्रोजेक्ट-स्थानीय होम में **कॉपी** करने की पेशकश करता है (नीचे "Auth" देखें) — कॉपी केवल स्पष्ट `y` पर होता है।

फ्लैग: `--no-shell-hook` सभी rc-फाइल संपादन को छोड़ देता है; `--yes` / `--non-interactive` कभी भी संकेत नहीं देता और इसलिए कभी auth को कॉपी नहीं करता (CI के लिए)।

## सादे `claude`, `codex`, `opencode` का उपयोग करना

कोई रैपर कमांड नहीं है। एक सक्रिय प्रोजेक्ट के अंदर सामान्य कमांड केवल रूट किए गए होम देखते हैं:

- **Claude Code** `CLAUDE_CONFIG_DIR` को पढ़ता है → प्रोजेक्ट-स्थानीय सेटिंग्स, उपयोगकर्ता-स्तरीय कौशल/प्लगइन, सेशन, इतिहास। (प्रोजेक्ट-स्तरीय `.claude/` को *हमेशा* नेटिव रूप से पढ़ा जाता है — सीमाएं देखें।)
- **Codex CLI** `CODEX_HOME` को पढ़ता है → प्रोजेक्ट-स्थानीय `config.toml`, `auth.json`, सेशन, इतिहास, लॉग।
- **OpenCode** `OPENCODE_CONFIG` को पढ़ता है → प्रोजेक्ट-स्थानीय कॉन्फ़िग फाइल। यह डिफ़ॉल्ट रूप से *आंशिक* अलगाव है — सीमाएं देखें।

### Auth

ताज़े प्रोजेक्ट-स्थानीय होम बिना क्रेडेंशियल के शुरू होते हैं:

- **macOS पर Claude**: कुछ नहीं करना है — क्रेडेंशियल Keychain में रहते हैं और हर कॉन्फ़िग dir के साथ साझा किए जाते हैं (जिसका अर्थ है कि वे *अलग नहीं* प्रोजेक्ट के अनुसार अलग नहीं हैं)।
- **Linux/Windows पर Claude**: प्रोजेक्ट के अंदर `claude login` चलाएं, या init की पेशकश को `~/.claude/.credentials.json` को कॉपी करने के लिए स्वीकार करें।
- **Codex**: प्रोजेक्ट के अंदर `codex login` चलाएं, या init की पेशकश को `~/.codex/auth.json` को कॉपी करने के लिए स्वीकार करें।

Auth फाइलें **कभी स्नैपशॉट में यात्रा नहीं करती** (नाम द्वारा बाहर रखी जाती हैं, भले ही वे कहां पहुंचीं)।

## gstack स्थापन

[gstack](https://github.com/garrytan/gstack) अपने installer को `~/.claude/skills/gstack` में हार्डकोड करता है — बिल्कुल वैश्विक प्रदूषण जिससे agentmod मुक्त रहना चाहता है। तो:

```sh
agentmod install gstack            # .agentmod/claude/skills/gstack में क्लोन करें
agentmod install gstack --force    # मौजूदा प्रोजेक्ट-स्थानीय स्थापन को बदलें
```

Installer git के साथ क्लोन करता है, कभी gstack की अपनी setup स्क्रिप्ट को नहीं चलाता, और स्नैपशॉट करता है `~/.claude/skills` की सूची पहले और बाद में — वैश्विक निर्देशिका में कोई भी परिवर्तन एक उल्लंघन के रूप में रिपोर्ट किया जाता है और कमांड विफल हो जाता है। `agentmod doctor` अलग से चेतावनी देता है जब भी *वैश्विक* gstack स्थापन मौजूद होता है (यहां तक कि agentmod को अपनाने से पहले आपने स्वयं स्थापित किया गया), क्योंकि विश्व स्तर पर स्थापित कौशल हर प्रोजेक्ट में रिसता है।

## हैंडऑफ (`.amod` स्नैपशॉट)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + redaction report, कोई निष्कर्षण नहीं
agentmod handoff verify  FILE      # प्रत्येक सदस्य को फिर से हैश करें; mismatch पर exit 3
agentmod handoff restore FILE      # .agentmod/ को बदलें (backup पहले लिया गया)
agentmod pack / agentmod unpack    # create / restore के aliases
```

एक स्नैपशॉट एक zip है जिसमें छह root सदस्य हैं — `manifest.json`, `inventory.json` (प्रति-फाइल size/sha256/mode), `REDACTION.md` (क्या बाहर रखा गया और क्यों, प्लस secret-scan निष्कर्ष), `HANDOFF.md` और `RESTORE.md` (प्राप्तकर्ता के लिए मानव निर्देश), `checksums.txt` (`shasum -a 256 -c`-संगत) — और `payload/.agentmod/…` के अंतर्गत payload। निर्माण परमाणु और नियतिवादी है; manifest git branch/commit/dirty state को किसी भी क्रेडेंशियल को दूरस्थ URL से हटाकर रिकॉर्ड करता है। एक dirty worktree `--allow-dirty` के बिना पैक करने से इनकार करता है।

`inspect` और `verify` कहीं भी काम करते हैं — प्राप्तकर्ता किसी भी प्रोजेक्ट सेटअप के बिना एक स्नैपशॉट को audit कर सकता है।

### Secrets बहिष्करण नीति

दो परतें, दोनों डिफ़ॉल्ट रूप से चालू:

1. **बहिष्करण नियम** payload से ज्ञात-संवेदनशील फाइलों को हटाते हैं और `REDACTION.md` में प्रत्येक को सूचीबद्ध करते हैं: auth फाइलें नाम द्वारा (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`, SSH कुंजी (`id_*`, `*.pem`, `*.pub`), credential निर्देशिकाएं (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), keychain फाइलें, `.git`, `node_modules`, caches और temp dirs।
2. **एक content scan** हर *रखी गई* फाइल पर। Private-key सामग्री जब तक आप `--allow-findings` पास न करें तब तक निर्माण से इनकार करता है (और फिर `REDACTION.md` में HARD के रूप में चिह्नित किया जाता है)। संभावित tokens (AWS access key IDs, GitHub tokens, `sk-…` keys, `api_key=`-शैली assignments) को चेतावनी दी जाती है लेकिन ब्लॉक नहीं करते।

स्कैन heuristic है। **एक स्नैपशॉट साझा करने से पहले `REDACTION.md` (या `handoff inspect`) की समीक्षा करें** — सेशन और कार्यशील संदर्भ डिजाइन द्वारा यात्रा करते हैं और कुछ भी उद्धृत कर सकते हैं जो आपने एजेंट बातचीत में पेस्ट किया हो। स्नैपशॉट इस कारण से mode 0600 में लिखे जाते हैं; उन्हें निजी फाइलों की तरह व्यवहार करें।

## Git हैंडऑफ

```sh
agentmod pack --for-git    # प्रोजेक्ट root पर .agentmod-handoff/ लिखता है
git add .agentmod-handoff && git commit
```

एक `.amod` के समान छह सदस्य और payload, लेकिन सादे फाइलों की एक committable ट्री (`shasum -a 256 -c checksums.txt` निर्देशिका में काम करता है)। डिफ़ॉल्ट बहिष्करण के ऊपर यह तीनों एजेंट के लिए **सेशन, transcripts, इतिहास, और लॉग** को हटाता है — ये नियमित रूप से पेस्ट किए गए secrets और एक repository में नहीं होने चाहिए। कार्यशील संदर्भ जो साझा करना सुरक्षित है (CLAUDE.md, agent configs, कौशल, योजनाएं) रहता है।

पुनः-चलाने से पिछले पैकेज को बदल दिया जाता है; repo में कुछ और नहीं छुआ जाता है।

## Restore सावधानियां

`handoff restore` / `unpack` हर स्नैपशॉट को untrusted input के रूप में मानता है:

- पहले पूर्ण checksum verification और inventory cross-check;
- path-safety योजना: zip-slip (`..`), निरपेक्ष paths, drive letters, गैर-`.agentmod` targets, सुरक्षित नाम (`.git`, `.ssh`, `.aws`, `.docker`), और escaping या निरपेक्ष symlink targets सभी को कुछ भी लिखने से पहले अस्वीकार कर दिया जाता है;
- मौजूदा `.agentmod/` को निष्कर्षण से पहले `.agentmod.backup-<stamp>` को नाम दिया जाता है; कोई भी विफलता इसे स्वचालित रूप से वापस करती है;
- **एक स्नैपशॉट से कुछ भी कभी executed नहीं होता है**;
- उसके बाद: Claude guard हुक इस मशीन की बाइनरी के लिए फिर से wired किया जाता है, restored agent configs में पाए गए मशीन-विशिष्ट निरपेक्ष paths को चेतावनी दी जाती है (आपकी फाइलें कभी फिर से नहीं लिखी जाती हैं), `doctor` inline चलता है, और आवश्यक पुनः-लॉगिन स्टेप्स को मुद्रित किया जाता है (auth कभी यात्रा नहीं करता)।

Restores अनुमान लगाने के बजाय इनकार करते हैं — एक अस्वीकार किया गया restore प्रोजेक्ट को byte-identical छोड़ देता है।

## `agentmod doctor`

केवल-पढ़ने योग्य निदान, कभी भी सुरक्षित रूप से चलाने के लिए (exit 0 clean, 3 निष्कर्षों के साथ): प्रोजेक्ट/कॉन्फ़िग/layout state, shell-hook स्थापन और liveness, routing drift, प्रोजेक्ट के बाहर lingering variables, duplicate PATH प्रविष्टियां, HOME/shim उल्लंघन, प्रति-एजेंट auth उपस्थिति पुनः-लॉगिन निर्देशों के साथ, OpenCode लीक चेतावनियां, gstack वैश्विक/प्रोजेक्ट state, Claude guard wiring, restored configs में portability जोखिम, मौजूदा स्नैपशॉट में रिकॉर्ड किए गए secret candidates, `.agentmod-handoff/` के अंदर सेशन/लॉग सामग्री, और क्या repository का HEAD अभी भी newest snapshot के साथ मेल खाता है।

## Claude Bash गार्ड

`agentmod init` `agentmod guard claude-bash` को Claude Code PreToolUse हुक के रूप में प्रोजेक्ट-स्थानीय होम में रजिस्टर करता है। यह Bash कमांड को ब्लॉक करता है जो वैश्विक एजेंट होम (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`) में लिखते हैं, `sudo` का उपयोग करते हैं, या `HOME` को फिर से असाइन करते हैं — एजेंट को कारण वापस मिलता है और समायोजन कर सकता है। Reads को कभी ब्लॉक नहीं किया जाता है। यह एक शेल-parse heuristic गहरा है: उपयोगी guardrail, सैंडबॉक्स नहीं।

## ज्ञात सीमाएं

ईमानदारी अनुभाग। ये अंतर्निहित उपकरणों के गुण हैं या जानबूझकर MVP scope हैं — `doctor` और generated docs भी इन्हें state करते हैं।

- **macOS Keychain (Claude)।** macOS पर Claude Code OAuth क्रेडेंशियल को Keychain में स्टोर करता है, सभी config dirs के *across* साझा किए गए। Per-project account अलगाव macOS पर असंभव है — और कोई पुनः-लॉगिन प्रोजेक्ट के अनुसार आवश्यक नहीं है। Linux/Windows एक per-home `.credentials.json` का उपयोग करते हैं, जो अलग करते हैं लेकिन प्रोजेक्ट के अनुसार login/copy की आवश्यकता होती है।
- **OpenCode डिफ़ॉल्ट रूप से आंशिक रूप से अलग किया गया है।** OpenCode के पास कोई एकल होम variable नहीं है; इसकी कॉन्फ़िग एक merge chain है जो अभी भी वैश्विक `~/.config/opencode/opencode.json` को पढ़ता है, और sessions/storage/auth वैश्विक XDG data dirs में रहते हैं। `opencode.xdg_full_isolation = true` XDG वेरिएबल को रूट करता है पूर्ण अलगाव के लिए — लेकिन यह प्रोजेक्ट के अंदर चलाने वाले *हर* XDG-aware tool को प्रभावित करता है। `doctor` दोनों स्थितियों को रिपोर्ट करता है।
- **Project `.claude/` native Claude व्यवहार है।** Claude Code हमेशा `./.claude/` को पढ़ता है भले ही `CLAUDE_CONFIG_DIR` की परवाह किए बिना। agentmod का Claude के लिए added value *user-level* state (वैश्विक कौशल/प्लगइन, सेशन, इतिहास) को अलग करना है; प्रोजेक्ट `.claude/` agentmod से पहले पहले से काम करता था।
- **प्रथम-सेशन हुक activation।** `agentmod init` के तुरंत बाद, पहले से चल रहा शेल नए rc block को load नहीं करता है। एक नया टर्मिनल खोलें, `exec $SHELL` करें, या one-shot `eval "$(agentmod hook zsh)"` (init बिल्कुल यह प्रिंट करता है)। इसी तरह, bash हुक `PROMPT_COMMAND` के माध्यम से fires करता है और इसलिए non-interactive bash स्क्रिप्ट में inert है (direnv के समान सीमाओं का एक वर्ग) — scripts को यदि routing की आवश्यकता हो तो `eval "$(agentmod env --shell bash --activate <root>)"` के माध्यम से explicitly वेरिएबल को सेट करना चाहिए।
- **केवल npm का global bin PATH पर है।** `.agentmod/node/bin` एक एकल managed PATH प्रविष्टि है। pnpm/bun global installs को प्रोजेक्ट में रूट किया जाता है (`PNPM_HOME`, `BUN_INSTALL`) लेकिन उनकी bin dirs को PATH में नहीं जोड़ा जाता है।
- **Tree packages manually restore करते हैं।** `handoff restore` केवल `.amod` फाइलों को स्वीकार करता है; एक committed `.agentmod-handoff/` directory को इसके अंदर `RESTORE.md` का पालन करके restore किया जाता है (इस संस्करण के पास कोई directory reader नहीं है)।
- **स्नैपशॉट को post-restore repair की आवश्यकता हो सकती है।** gstack clone इसके `.git` के बिना यात्रा करता है (फिर से चलाएं `agentmod install gstack --force` इसे updatable फिर से बनाने के लिए), और `node/bin` launcher symlinks `node_modules` को बाहर रखा गया है क्योंकि dangle करते हैं (फिर से चलाएं `npm install -g …` प्रोजेक्ट के अंदर)।
- **Shell समर्थन zsh और bash है।** अन्य shells अभी भी `agentmod env` को manually उपयोग कर सकते हैं।

## FAQ

**क्या मैं `claude` / `codex` / `opencode` को सीधे उपयोग करता हूँ?**
हां। यह point है — कोई wrappers, कोई shims, कोई `agentmod run` नहीं।

**agentmod बस `HOME` को क्यों नहीं बदलता?**
`HOME` को पुनः असाइन करना SSH, git, keychains, dotfiles, और shell में हर अन्य tool को तोड़ता है। agentmod केवल agent-विशिष्ट variables को रूट करता है।

**restore के बाद मेरा auth क्यों गायब है?**
डिजाइन द्वारा — credentials कभी स्नैपशॉट में यात्रा नहीं करते हैं। नई मशीन पर मुद्रित पुनः-लॉगिन लाइन (या init की copy पेशकश) का पालन करें।

**क्या मैं `.agentmod/` को git पर कमिट कर सकता हूँ?**
नहीं — init इसे gitignore करता है (सेशन, caches, और संभवतः कॉपी किए गए auth वहां रहते हैं)। इसके बजाय सुरक्षित सबसेट को कमिट करें: `agentmod pack --for-git`।

**यह direnv से कैसे अलग है?**
समान activation मॉडल (directory-scoped env, prompt-hook आधारित, exit पर perfect restore), लेकिन agentmod भी जानता है कि प्रत्येक agent के लिए क्या रूट करना है, होम बनाता है, वैश्विक लिखने के खिलाफ guards करता है, और handoff करता है। दोनों अच्छी तरह coexist करते हैं।

**एक स्नैपशॉट "secret-candidate findings" के साथ बनाने में विफल हो जाता है।**
content scan ने एक रखी गई फाइल में private-key सामग्री पाई। इसे हटाएं (या इसे एक बहिष्कृत स्थान जैसे `.env` में move करें), या अगर आप इसे स्नैपशॉट के अंदर स्वीकार करते हैं तो फिर भी `--allow-findings` के साथ pack करें।

**क्या यह Windows पर काम करता है?**
Go कोड builds करता है और path safety को Windows-style paths के लिए enforce किया जाता है, लेकिन shell hooks zsh/bash को target करते हैं; Windows इस संस्करण में untested है।

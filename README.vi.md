# agentmod

[English](README.md) · [한국어](README.ko.md) · [简体中文](README.zh-CN.md) · [正體中文](README.zh-TW.md) · [Français](README.fr.md) · [日本語](README.ja.md) · [Português](README.pt.md) · [Español](README.es.md) · [Română](README.ro.md) · [Русский](README.ru.md) · [Türkçe](README.tr.md) · [Italiano](README.it.md) · [Tiếng Việt](README.vi.md) · [Українська](README.uk.md) · [Indonesian](README.id.md) · [हिन्दी](README.hi.md) · [فارسی](README.fa.md) · [Беларуская](README.be.md) · [বাংলা](README.bn.md)

Cách ly từng dự án và bàn giao cho các tác nhân mã hóa.

`agentmod` giữ cấu hình, kỹ năng, plugin, phiên làm việc, bộ nhớ đệm và bối cảnh làm việc của **Claude Code**, **Codex CLI**, và **OpenCode** bên trong dự án mà bạn đang làm việc — và đóng gói môi trường đó thành một bản chụp mà bạn có thể giao cho một máy khác.

Nó đóng hai vai trò:

1. **Agent Home Router.** Bên trong một cây thư mục chứa `.agentmod/agentmod.toml`, một hook shell định tuyến trang chủ của mỗi tác nhân vào `.agentmod/`. Bên ngoài, mỗi biến được khôi phục chính xác như trước đây và thiết lập toàn cầu của bạn không bị ảnh hưởng.
2. **Công cụ Bàn giao.** `agentmod handoff create` đóng gói `.agentmod/` thành một bản chụp `.amod` có thể xác minh (hoặc, với `--for-git`, một cây tệp có thể commit dưới `.agentmod-handoff/`). **Git di chuyển mã nguồn của bạn; agentmod di chuyển môi trường tác nhân.**

## Agentmod *không phải* là gì

- **Không phải là hộp cát Docker.** Nó định tuyến các biến môi trường trong shell của riêng bạn. Không có container, không có VM, không có bộ lọc syscall.
- **Không phải là cách ly bảo mật đầy đủ.** Một công cụ bỏ qua các biến được định tuyến vẫn có thể truy cập trang chủ toàn cầu của bạn. Trình bảo vệ Claude Bash (dưới đây) là phòng chống sâu, không phải là ranh giới bảo mật.
- **Không phải là shim.** Nó không bao giờ chặn hoặc bao bọc các lệnh `claude`, `codex`, hoặc `opencode`. Bạn tiếp tục chạy chúng trực tiếp, không được sửa đổi.
- **Không phải là công cụ thay đổi HOME.** `HOME` không bao giờ được gán lại.
- **Không phải là công cụ sao lưu mã nguồn.** Các bản chụp không bao giờ bao gồm mã nguồn của bạn theo mặc định. Sử dụng git cho mã nguồn.

## Cách hoạt động

`agentmod hook zsh` / `agentmod hook bash` in một hàm shell độc lập và nhỏ gọn (được cài đặt vào tệp rc của bạn bởi `agentmod init`). Trên mỗi dấu nhắc và thay đổi thư mục, nó đi lên tìm kiếm `.agentmod/agentmod.toml`:

- **Khi vào một dự án** lưu các giá trị hiện tại và đặt:

  | Biến | Định tuyến đến |
  |---|---|
  | `CLAUDE_CONFIG_DIR` | `.agentmod/claude` |
  | `CODEX_HOME` | `.agentmod/codex` |
  | `OPENCODE_CONFIG` | `.agentmod/opencode/opencode.json` |
  | `NPM_CONFIG_PREFIX` | `.agentmod/node` |
  | `NPM_CONFIG_CACHE` | `.agentmod/node/npm-cache` |
  | `PNPM_HOME` | `.agentmod/node/pnpm` |
  | `BUN_INSTALL` | `.agentmod/node/bun` |
  | `XDG_CONFIG_HOME` / `XDG_DATA_HOME` / `XDG_CACHE_HOME` / `XDG_STATE_HOME` | `.agentmod/opencode/xdg/…` — **chỉ** với `opencode.xdg_full_isolation = true` |

  `PATH` tăng thêm chính xác một mục, `.agentmod/node/bin` (bin toàn cầu của npm dưới tiền tố được định tuyến). Các biến ghi chép (`AGENTMOD_ACTIVE`, `AGENTMOD_PROJECT_ROOT`, `AGENTMOD_ROOT`, `AGENTMOD_VARS`, `AGENTMOD_SAVED_*`) ghi lại những gì cần hoàn tác.

- **Khi rời khỏi dự án** khôi phục mỗi giá trị đã lưu và xóa mục `PATH` — một phép nghịch đảo hoàn hảo. Chuyển đổi trực tiếp giữa hai dự án agentmod định tuyến lại trong một bước mà không rò rỉ đường dẫn của dự án nào.

Định tuyến cho mỗi tác nhân có thể được tắt trong `agentmod.toml` (`claude.enabled`, `codex.enabled`, `opencode.enabled`, `node.enabled`).

## Cài đặt

Chọn cách nào phù hợp với thiết lập của bạn — mỗi cách cài đặt cùng một tệp nhị phân duy nhất:

```sh
# npm (cài đặt tệp nhị phân được biên dịch sẵn cho nền tảng của bạn)
npm install -g agentmod

# script cài đặt (tải phiên bản phù hợp, xác minh sha256)
curl -fsSL https://raw.githubusercontent.com/mojomoth/agentmod/main/install.sh | sh

# go install (yêu cầu bộ công cụ Go)
go install github.com/mojomoth/agentmod@latest
```

Hoặc xây dựng từ nguồn (Go 1.26+, phụ thuộc mô-đun duy nhất là `BurntSushi/toml`):

```sh
git clone https://github.com/mojomoth/agentmod && cd agentmod
go build -o agentmod .
# đặt tệp nhị phân ở đâu đó trên PATH của bạn
```

## Bắt đầu nhanh

```sh
cd ~/work/myproject
agentmod init          # tạo .agentmod/, chỉnh sửa .gitignore, cài đặt
                       # hook shell vào tệp rc của bạn, đề nghị sao chép xác thực
# lần đầu tiên: hook chưa hoạt động trong SHELL này —
# mở terminal mới, hoặc: exec $SHELL

cd ~/work/myproject    # hook kích hoạt; kiểm tra:
agentmod status        # "AgentMod: active", các đường dẫn được định tuyến được liệt kê
claude                 # lệnh đơn giản — bây giờ sử dụng trang chủ cục bộ dự án
agentmod install gstack   # kỹ năng cục bộ dự án, trang chủ toàn cầu không bị ảnh hưởng

agentmod pack          # chụp nhanh vào .agentmod/snapshots/<name>-<stamp>.amod
agentmod doctor        # chẩn đoán chỉ đọc bất cứ lúc nào
```

Trên máy nhận:

```sh
cd ~/work/myproject    # nguồn đã đến qua git
agentmod init
agentmod unpack myproject-20260611-123045.amod
# theo dõi các ghi chú đăng nhập lại được in; doctor chạy tự động
```

## `agentmod init`

Luỹ kỳ — chạy lại điền bất cứ điều gì còn thiếu và không bao giờ ghi đè `agentmod.toml` hoặc bất kỳ tệp người dùng nào. Nó:

- tạo `.agentmod/{claude,codex,opencode,node,snapshots,logs}` và mặc định `agentmod.toml`;
- kết nối trình bảo vệ Claude Bash vào `.agentmod/claude/settings.json`;
- thêm `.agentmod/` vào `.gitignore` (chỉ được tạo bên trong kho git);
- cài đặt hook shell như một khối được rào chắn trong `~/.zshrc` hoặc `~/.bashrc` (shell của bạn từ `$SHELL`; khối được cập nhật tại chỗ, không bao giờ trùng lặp, và nội dung rc riêng của bạn không bị chạm vào);
- đề nghị **sao chép** các tệp xác thực Claude/Codex hiện có vào trang chủ cục bộ dự án (xem "Auth" dưới đây) — sao chép chỉ xảy ra khi `y` rõ ràng.

Cờ: `--no-shell-hook` bỏ qua tất cả chỉnh sửa tệp rc; `--yes` / `--non-interactive` không bao giờ nhắc và do đó không bao giờ sao chép xác thực (cho CI).

## Sử dụng `claude`, `codex`, `opencode` thường

Không có lệnh wrapper. Bên trong một dự án hoạt động, các lệnh thông thường đơn giản là thấy các trang chủ được định tuyến:

- **Claude Code** đọc `CLAUDE_CONFIG_DIR` → cài đặt cục bộ dự án, kỹ năng/plugin cấp người dùng, phiên làm việc, lịch sử. (`.claude/` cấp dự án *luôn* được đọc natively — xem Hạn chế.)
- **Codex CLI** đọc `CODEX_HOME` → `config.toml` cục bộ dự án, `auth.json`, phiên làm việc, lịch sử, nhật ký.
- **OpenCode** đọc `OPENCODE_CONFIG` → tệp cấu hình cục bộ dự án. Đây là cách ly *một phần* theo mặc định — xem Hạn chế.

### Xác thực

Các trang chủ cục bộ dự án mới bắt đầu mà không có thông tin xác thực:

- **Claude trên macOS**: không có gì để làm — thông tin xác thực sống trong Keychain và được chia sẻ với mỗi thư mục cấu hình (điều này cũng có nghĩa là chúng *không* được cách ly cho mỗi dự án).
- **Claude trên Linux/Windows**: chạy `claude login` bên trong dự án, hoặc chấp nhận đề nghị init để sao chép `~/.claude/.credentials.json`.
- **Codex**: chạy `codex login` bên trong dự án, hoặc chấp nhận đề nghị init để sao chép `~/.codex/auth.json`.

Các tệp xác thực **không bao giờ di chuyển trong các bản chụp** (được loại trừ theo tên, không kể chúng đã đến đó như thế nào).

## Cài đặt gstack

[gstack](https://github.com/garrytan/gstack) hardcode trình cài đặt của nó thành `~/.claude/skills/gstack` — chính xác là ô nhiễm toàn cầu mà agentmod tồn tại để ngăn chặn. Vì vậy:

```sh
agentmod install gstack            # clone vào .agentmod/claude/skills/gstack
agentmod install gstack --force    # thay thế cài đặt cục bộ dự án hiện có
```

Trình cài đặt sao chép với git, không bao giờ chạy tập lệnh thiết lập riêng của gstack, và chụp danh sách `~/.claude/skills` trước và sau — bất kỳ thay đổi nào đối với thư mục toàn cầu được báo cáo là vi phạm và không thành công lệnh. `agentmod doctor` riêng biệt cảnh báo bất cứ khi nào tồn tại cài đặt gstack *toàn cầu* (thậm chí là cài đặt bạn tự cài đặt trước khi sử dụng agentmod), vì kỹ năng được cài đặt toàn cầu rò rỉ vào mỗi dự án.

## Bàn giao (bản chụp `.amod`)

```sh
agentmod handoff create [--output PATH] [--allow-findings] [--allow-dirty]
agentmod handoff list
agentmod handoff inspect FILE      # manifest + redaction report, không trích xuất
agentmod handoff verify  FILE      # re-hash từng thành viên; exit 3 khi không khớp
agentmod handoff restore FILE      # thay thế .agentmod/ (sao lưu được thực hiện trước)
agentmod pack / agentmod unpack    # bí danh của create / restore
```

Một bản chụp là zip với sáu thành viên gốc — `manifest.json`, `inventory.json` (kích thước/sha256/mode cho mỗi tệp), `REDACTION.md` (những gì được loại trừ và tại sao, cộng với kết quả quét bí mật), `HANDOFF.md` và `RESTORE.md` (hướng dẫn con người cho người nhận), `checksums.txt` (tương thích với `shasum -a 256 -c`) — và tải trọng dưới `payload/.agentmod/…`. Tạo là nguyên tử và xác định; manifest ghi lại nhánh/commit/trạng thái bẩn của git với bất kỳ thông tin xác thực nào được loại bỏ khỏi URL từ xa. Một worktree bẩn từ chối gói trừ khi `--allow-dirty`.

`inspect` và `verify` hoạt động ở bất cứ đâu — người nhận có thể kiểm toán một bản chụp trước khi có bất kỳ thiết lập dự án nào.

### Chính sách loại trừ bí mật

Hai lớp, cả hai bật theo mặc định:

1. **Quy tắc loại trừ** bỏ các tệp nhạy cảm đã biết khỏi tải trọng và liệt kê từng tệp trong `REDACTION.md`: tệp xác thực theo tên (`.credentials.json`, `auth.json`, `credentials*`), `*.env` / `.env.*`, khóa SSH (`id_*`, `*.pem`, `*.pub`), thư mục thông tin xác thực (`.ssh`, `.aws`, `.azure`, `.gcloud`, `.kube`, `.gnupg`, `.docker`), tệp keychain, `.git`, `node_modules`, bộ nhớ đệm và thư mục tạm thời.
2. **Quét nội dung** trên mỗi tệp *được giữ*. Vật liệu khóa riêng từ chối tạo ngoại trừ nếu bạn vượt qua `--allow-findings` (và sau đó được đánh dấu HARD trong `REDACTION.md`). Các mã thông báo có khả năng (AWS access key IDs, GitHub tokens, `sk-…` keys, `api_key=` style assignments) được cảnh báo nhưng không chặn.

Quét là heuristic. **Xem lại `REDACTION.md` (hoặc `handoff inspect`) trước khi chia sẻ bản chụp** — phiên làm việc và bối cảnh làm việc di chuyển theo thiết kế và có thể trích dẫn bất cứ điều gì bạn dán vào cuộc trò chuyện tác nhân. Bản chụp được viết ở chế độ 0600 vì lý do này; xử lý chúng như các tệp riêng tư.

## Bàn giao Git

```sh
agentmod pack --for-git    # ghi .agentmod-handoff/ tại gốc dự án
git add .agentmod-handoff && git commit
```

Tương tự sáu thành viên và tải trọng như `.amod`, nhưng như một cây tệp bình thường có thể commit (`shasum -a 256 -c checksums.txt` hoạt động trong thư mục). Trên đầu của các loại trừ mặc định, nó loại bỏ **phiên làm việc, bản ghi, lịch sử, và nhật ký** cho cả ba tác nhân — những thứ này thường chứa bí mật dán và không thuộc kho lưu trữ. `--include-sessions` luôn từ chối: commit phiên làm việc sẽ yêu cầu mã hóa, mà phiên bản này không thực hiện. Bối cảnh làm việc an toàn để chia sẻ (CLAUDE.md, config tác nhân, kỹ năng, kế hoạch) vẫn còn.

Chạy lại thay thế gói trước đó; không có gì khác trong repo được chạm vào.

## Cảnh báo khôi phục

`handoff restore` / `unpack` xử lý mỗi bản chụp là đầu vào không đáng tin cậy:

- xác minh tổng kiểm tra đầy đủ và kiểm tra hàng tồn kho chéo trước;
- kế hoạch an toàn đường dẫn: zip-slip (`..`), đường dẫn tuyệt đối, chữ cái ổ đĩa, mục tiêu không phải `.agentmod`, tên được bảo vệ (`.git`, `.ssh`, `.aws`, `.docker`), và vượt qua hoặc đường dẫn symlink tuyệt đối đều bị từ chối trước khi bất cứ điều gì được viết;
- `.agentmod/` hiện có được đổi tên thành `.agentmod.backup-<stamp>` trước khi trích xuất; bất kỳ lỗi nào cũng cuộn lại tự động;
- **không có gì từ bản chụp được thực thi bao giờ**;
- sau đó: hook bảo vệ Claude được lại kết nối với tệp nhị phân của *máy này*, các đường dẫn tuyệt đối dành riêng cho máy được tìm thấy trong config tác nhân được khôi phục bị cảnh báo (tệp của bạn không bao giờ được viết lại), `doctor` chạy nội tuyến, và các bước đăng nhập lại được yêu cầu được in (xác thực không bao giờ di chuyển).

Khôi phục từ chối thay vì đoán — một khôi phục bị từ chối để dự án byte-identical.

## `agentmod doctor`

Chẩn đoán chỉ đọc, an toàn để chạy bất cứ lúc nào (exit 0 sạch, 3 với kết quả): trạng thái config/layout dự án, cài đặt và sáng suốt hook shell, sự trôi dạo định tuyến, biến lingering bên ngoài dự án, mục PATH trùng lặp, vi phạm HOME/shim, sự hiện diện xác thực cho mỗi tác nhân với hướng dẫn đăng nhập lại, cảnh báo OpenCode leak, trạng thái gstack toàn cầu/dự án, kết nối bảo vệ Claude, rủi ro tính di động trong config được khôi phục, ứng cử viên bí mật được ghi lại trong bản chụp hiện có, vật liệu phiên/nhật ký bên trong `.agentmod-handoff/`, và liệu HEAD của kho lưu trữ có còn khớp với bản chụp mới nhất.

## Trình bảo vệ Claude Bash

`agentmod init` đăng ký `agentmod guard claude-bash` như một hook PreToolUse của Claude Code trong trang chủ cục bộ dự án. Nó chặn các lệnh Bash sẽ viết vào các trang chủ tác nhân toàn cầu (`~/.claude`, `~/.codex`, `~/.config/opencode`, `~/.local/share/opencode`), sử dụng `sudo`, hoặc gán lại `HOME` — tác nhân nhận lý do trở lại và có thể điều chỉnh. Lần đọc không bao giờ bị chặn. Nó là một heuristic phân tích shell sâu: rào chắn hữu ích, không phải hộp cát.

## Hạn chế đã biết

Phần Trung thực. Đây là những đặc tính của các công cụ cơ bản hoặc phạm vi MVP cố ý — `doctor` và các tài liệu được tạo ra cũng nêu rõ chúng.

- **macOS Keychain (Claude).** Claude Code trên macOS lưu trữ thông tin xác thực OAuth trong Keychain, được chia sẻ trên *tất cả* thư mục cấu hình. Cách ly tài khoản cho mỗi dự án là không thể trên macOS — và không cần đăng nhập lại cho mỗi dự án. Linux/Windows sử dụng `.credentials.json` cho mỗi trang chủ, cách ly nhưng yêu cầu đăng nhập/sao chép cho mỗi dự án.
- **OpenCode được cách ly một phần theo mặc định.** OpenCode không có biến trang chủ duy nhất; cấu hình của nó là một chuỗi hợp nhất vẫn đọc `~/.config/opencode/opencode.json` toàn cầu, và phiên làm việc/lưu trữ/xác thực sống trong thư mục dữ liệu XDG toàn cầu. `opencode.xdg_full_isolation = true` định tuyến các biến XDG để cách ly đầy đủ — nhưng điều đó ảnh hưởng *mọi* công cụ nhận thức XDG bạn chạy bên trong dự án. `doctor` báo cáo cả hai tình huống.
- **`.claude/` cấp dự án là hành vi Claude natively.** Claude Code luôn đọc `./.claude/` bất kể `CLAUDE_CONFIG_DIR`. giá trị được thêm của agentmod cho Claude là cách ly trạng thái *cấp người dùng* (kỹ năng/plugin toàn cầu, phiên làm việc, lịch sử); `.claude/` cấp dự án đã hoạt động trước agentmod.
- **Kích hoạt hook phiên đầu tiên.** Ngay sau `agentmod init`, shell đang chạy chưa tải khối rc mới. Mở terminal mới, `exec $SHELL`, hoặc một lần `eval "$(agentmod hook zsh)"` (init in chính xác cái này). Tương tự, hook bash bắn qua `PROMPT_COMMAND` và do đó không hoạt động trong tập lệnh bash không tương tác (cùng loại hạn chế như direnv) — tập lệnh nên đặt các biến rõ ràng qua `eval "$(agentmod env --shell bash --activate <root>)"` nếu họ cần định tuyến.
- **Chỉ bin toàn cầu của npm trên PATH.** `.agentmod/node/bin` là mục PATH duy nhất được quản lý. cài đặt toàn cầu pnpm/bun được định tuyến vào dự án (`PNPM_HOME`, `BUN_INSTALL`) nhưng thư mục bin của chúng không được thêm vào PATH.
- **Gói cây khôi phục thủ công.** `handoff restore` chấp nhận các tệp `.amod` chỉ; một thư mục `.agentmod-handoff/` được commit được khôi phục bằng cách theo `RESTORE.md` bên trong nó (phiên bản này không có trình đọc thư mục).
- **Bản chụp có thể cần sửa chữa sau khôi phục.** Clone gstack di chuyển mà không có `.git` của nó (chạy lại `agentmod install gstack --force` để làm cho nó có thể cập nhật được), và các symlink trình khởi chạy `node/bin` sẽ treo vì `node_modules` bị loại trừ (chạy lại `npm install -g …` bên trong dự án).
- **Hỗ trợ shell là zsh và bash.** Các shell khác vẫn có thể sử dụng `agentmod env` thủ công.

## Câu hỏi thường gặp

**Tôi có tiếp tục sử dụng `claude` / `codex` / `opencode` trực tiếp không?**
Có. Đó là điểm — không wrapper, không shim, không `agentmod run`.

**Tại sao agentmod không chỉ thay đổi `HOME`?**
Gán lại `HOME` phá vỡ SSH, git, keychain, dotfile và mọi công cụ khác trong shell. agentmod chỉ định tuyến các biến dành riêng cho tác nhân.

**Tại sao xác thực của tôi bị mất sau khi khôi phục?**
Theo thiết kế — thông tin xác thực không bao giờ di chuyển trong bản chụp. Theo dõi các dòng đăng nhập lại in (hoặc sao chép init) trên máy mới.

**Tôi có thể commit `.agentmod/` to git không?**
Không — init gitignore nó (phiên làm việc, bộ nhớ đệm, và xác thực có thể được sao chép sống ở đó). Commit tập hợp con an toàn thay thế: `agentmod pack --for-git`.

**Điều này khác với direnv như thế nào?**
Cùng mô hình kích hoạt (env có phạm vi thư mục, dựa trên hook dấu nhắc, khôi phục hoàn hảo khi thoát), nhưng agentmod cũng biết *cái gì* để định tuyến cho mỗi tác nhân, tạo trang chủ, bảo vệ chống lại ghi toàn cầu, và làm bàn giao. Hai người cùng tồn tại tốt.

**Bản chụp không tạo được với kết quả "secret-candidate findings".**
Quét nội dung tìm thấy vật liệu khóa riêng trong tệp được giữ. Xóa nó (hoặc di chuyển nó đến một vị trí bị loại trừ như `.env`), hoặc gói ngay cả vậy với `--allow-findings` nếu bạn chấp nhận nó ở trong bản chụp.

**Nó có hoạt động trên Windows không?**
Mã Go xây dựng và an toàn đường dẫn được thực thi cho đường dẫn kiểu Windows, nhưng hook shell nhắm đến zsh/bash; Windows chưa được kiểm tra trong phiên bản này.

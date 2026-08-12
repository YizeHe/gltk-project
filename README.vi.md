# GrokLangToolKit (gltk)

**Ngôn ngữ / Language：** [中文](README.md) · [English](README.en.md) · [Tiếng Việt](README.vi.md)

---

**GrokLang** — bộ công cụ ngôn ngữ phục vụ reverse engineering: **máy ảo bytecode thanh ghi (GLVM)**.  
Luồng chính **không** diễn giải AST và **không** biên dịch sang Python/JS.

| Mục | Giá trị |
|-----|---------|
| Phiên bản | **1.0.0** |
| Ngôn ngữ | GrokLang (`.glk`) |
| Bytecode | `.glkb` (magic `GLKB`) |
| CLI | `gltk` / `gltk.exe` |
| Module path | `groklang/gltk` |
| Runtime | Máy thanh ghi GLVM + native Go |

Kho này là **phần lõi mã nguồn mở + samples + khóa học ngôn ngữ** (`learn/`).  
Kho mẫu RE riêng, file nhị phân lớn và sản phẩm phụ **không** được đưa vào đây.

## Bắt đầu nhanh

```powershell
git clone https://github.com/YizeHe/gltk-project.git
cd gltk-project

go test ./...
go build -o gltk.exe ./cmd/gltk
# tùy chọn: stub runtime cho packer
go build -o glrt.exe ./cmd/glrt

.\gltk.exe version
.\gltk.exe run samples\hello.glk
.\gltk.exe run samples\use_lib.glk
.\gltk.exe compile samples\hello.glk -o hello.glkb
.\gltk.exe disasm hello.glkb

# đóng gói EXE độc lập (cần glrt.exe cạnh gltk hoặc --stub)
.\gltk.exe pack samples\pack_hello.glk -o pack_hello.exe
```

Hoặc:

```powershell
.\build.ps1
```

## Cấu trúc kho

```
cmd/gltk          # CLI
cmd/glrt          # stub runtime packer
internal/         # lexer / parser / compiler / VM / native
stdlib/           # thư viện .glk chuẩn
libs/             # thư viện người dùng mẫu
samples/          # demo chạy được (hello, GUI, PE, net, pack, …)
docs/             # ghi chú ngôn ngữ / VM / stdlib / GUI
learn/            # hướng dẫn theo chương + bài tập (tiếng Trung)
third_party/cui   # GUI Win32 đính kèm (CUI)
```

## Học GrokLang

Bắt đầu từ **[learn/README.md](learn/README.md)** (chương 01–09).

```powershell
.\gltk.exe run learn\examples\ch01_hello.glk
.\gltk.exe run learn\examples\ch02_06_mini_calc.glk
```

## Kiến trúc

```
  Mã nguồn .glk (+ import đường dẫn)
       │
       ▼
  lexer → parser → AST
       │
       ▼
  module.CompileEntry  →  thư viện → Chunk dùng chung
       │
       ▼
  compiler → Chunk (.glkb) → GLVM.Run → natives
```

**Luồng thực thi duy nhất**: `Chunk` → `VM.Run`.  
`gltk run file.glk` = biên dịch nhiều file + chạy VM.

## Import

```glk
import pe, fs, out                 // bare → native
import "libs/helpers.glk"          // path → alias helpers
import "libs/helpers.glk" as h     // alias tường minh
```

Thứ tự tìm thư viện: thư mục file entry → `./lib` `./libs` → `<root>/stdlib` `<root>/libs` → `GLTK_LIB` → cwd.

## Samples

Toàn bộ `samples/` có trong kho, gồm:

| Sample | Chủ đề |
|--------|--------|
| `hello.glk` | Hello World |
| `use_lib.glk` | Import thư viện người dùng |
| `lang_1_0.glk` / `lang_features.glk` | Tính năng ngôn ngữ |
| `pe_info.glk` / `string_scan.glk` | PE / chuỗi |
| `disasm_demo.glk` / `entropy_scan.glk` | Disasm / entropy |
| `gui_hello.glk` / `gui_re_workbench.glk` | GUI |
| `net_demo.glk` / `async_demo.glk` | Mạng / async |
| `pack_hello.glk` | Đóng gói EXE |
| `upx_unpack.glk` | UPX (thuần Go) |

Một số demo cần bạn truyền **đường dẫn binary của riêng bạn** sau `--`.  
Kho **không** đính kèm corpus malware.

## Tài liệu

- [docs/language.md](docs/language.md)
- [docs/vm.md](docs/vm.md) · [docs/bytecode.md](docs/bytecode.md)
- [docs/stdlib.md](docs/stdlib.md)
- [docs/GUI_INTEGRATION.md](docs/GUI_INTEGRATION.md)
- [docs/PACK.md](docs/PACK.md) · [docs/ASYNC.md](docs/ASYNC.md)

## Giấy phép

MIT — xem [LICENSE](LICENSE).

## Không có trong kho này

- Cây `testfile/` / `work/` riêng tư kích thước lớn  
- Khóa học RE / toolchain Android bên thứ ba  
- Sản phẩm phụ proxy / VLESS  
- `gltk.exe` dựng sẵn (hãy build từ mã nguồn)

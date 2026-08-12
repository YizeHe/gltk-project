# GrokLangToolKit (gltk)

**Language：** [中文](README.md) · [English](README.en.md) · [Tiếng Việt](README.vi.md)

---

**GrokLang** reverse-engineering language toolkit: a **register bytecode VM (GLVM)**.  
Main path is **not** AST interpretation and **not** transpile-to-Python/JS.

| Item | Value |
|------|--------|
| Version | **1.0.0** |
| Language | GrokLang (`.glk`) |
| Bytecode | `.glkb` (magic `GLKB`) |
| CLI | `gltk` / `gltk.exe` |
| Module path | `groklang/gltk` |
| Runtime | Register machine GLVM + Go natives |

This repository is the **open-source core + samples + language course** (`learn/`).  
Private reverse-engineering corpora, huge binaries, and side products are **not** included.

## Quick start

```powershell
git clone https://github.com/YizeHe/gltk-project.git
cd gltk-project

go test ./...
go build -o gltk.exe ./cmd/gltk
# optional packer runtime stub
go build -o glrt.exe ./cmd/glrt

.\gltk.exe version
.\gltk.exe run samples\hello.glk
.\gltk.exe run samples\use_lib.glk
.\gltk.exe compile samples\hello.glk -o hello.glkb
.\gltk.exe disasm hello.glkb

# pack standalone EXE (needs glrt.exe next to gltk or --stub)
.\gltk.exe pack samples\pack_hello.glk -o pack_hello.exe
```

Or:

```powershell
.\build.ps1
```

## Repository layout

```
cmd/gltk          # CLI
cmd/glrt          # packer runtime stub
internal/         # lexer / parser / compiler / VM / natives
stdlib/           # standard .glk libraries
libs/             # example user libraries
samples/          # runnable demos (hello, GUI, PE, net, pack, …)
docs/             # language / VM / stdlib / GUI notes
learn/            # chapter tutorials + examples (Chinese)
third_party/cui   # vendored Win32 GUI (CUI)
```

## Learn GrokLang

Start at **[learn/README.md](learn/README.md)** (chapters 01–09).

```powershell
.\gltk.exe run learn\examples\ch01_hello.glk
.\gltk.exe run learn\examples\ch02_06_mini_calc.glk
```

## Architecture

```
  .glk source (+ path imports)
       │
       ▼
  lexer → parser → AST
       │
       ▼
  module.CompileEntry  →  libraries → shared Chunk
       │
       ▼
  compiler → Chunk (.glkb) → GLVM.Run → natives
```

**Only execution path**: `Chunk` → `VM.Run`.  
`gltk run file.glk` = multi-file compile + VM.

## Imports

```glk
import pe, fs, out                 // bare → native
import "libs/helpers.glk"          // path → alias helpers
import "libs/helpers.glk" as h     // explicit alias
```

Library search order: entry dir → `./lib` `./libs` → `<root>/stdlib` `<root>/libs` → `GLTK_LIB` → cwd.

## Samples

All files under `samples/` ship in this repo, including:

| Sample | Topic |
|--------|--------|
| `hello.glk` | Hello World |
| `use_lib.glk` | User library import |
| `lang_1_0.glk` / `lang_features.glk` | Language features |
| `pe_info.glk` / `string_scan.glk` | PE / strings |
| `disasm_demo.glk` / `entropy_scan.glk` | Disasm / entropy |
| `gui_hello.glk` / `gui_re_workbench.glk` | GUI |
| `net_demo.glk` / `async_demo.glk` | Network / async |
| `pack_hello.glk` | Pack to EXE |
| `upx_unpack.glk` | UPX (pure Go path) |

Some demos expect you to pass **your own** binary paths on the command line after `--`.  
No malware corpora are bundled.

## Docs

- [docs/language.md](docs/language.md)
- [docs/vm.md](docs/vm.md) · [docs/bytecode.md](docs/bytecode.md)
- [docs/stdlib.md](docs/stdlib.md)
- [docs/GUI_INTEGRATION.md](docs/GUI_INTEGRATION.md)
- [docs/PACK.md](docs/PACK.md) · [docs/ASYNC.md](docs/ASYNC.md)

## License

MIT — see [LICENSE](LICENSE).

## Not in this repo

- Large private `testfile/` / `work/` trees  
- Bundled third-party RE courses / Android toolchains  
- Proxy / VLESS side products  
- Prebuilt `gltk.exe` (build from source)

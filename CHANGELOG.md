# Changelog

## 1.0.0 — 2026-07-21

### Language / GLVM
- **Top-level `let` is a true global** (`STOREG`/`LOADG`): callbacks and sibling functions share the same binding (fixes GUI handles / config maps).
- Free collection APIs: **`push` / `pop` / `keys` / `has` / `delete` / `clone`**.
- **`array + array`** concatenates into a new array.
- **`try/catch`** covers index OOB, arith type errors, ARRPUSH, bytes OOB, and other common runtime errors (not only CALL/THROW).
- Register overflow diagnostics include **source line** and a “use push / split function” hint.
- Sample: `samples/lang_1_0.glk`.

### GUI (usable, not pretty)
- **CUI vendored** at `third_party/cui` (no Desktop absolute replace).
- Label/checkbox **background fill** (no black garbage).
- **Layout** applied on `AddWidget` / `Show`.
- `gui.is_checked` / `gui.set_checked`; checkbox **on_click**.
- Robust `gui.open_file` (complete OPENFILENAMEW, UTF-16 filters).
- Full **gui_stub** on non-Windows.
- Version string: `cui+gltk-1.0.0`.

### Archive / RE toolkit
- `archive.find_7za` prefers full **7z.exe** (NSIS requires it; standalone 7za lacks NSIS).
- Stdlib: `stdlib/re_triage.glk` broad static triage (PE/NSIS/keywords/report).
- **Native `upx` module (pure Go, no upx.exe)**:
  - `upx.detect` / `upx.info` / `upx.unpack`
  - PE + LZMA (UPX 2-byte props) + PE rebuild; NRV2B/2D/2E paths present
  - Verified: GSP5.exe unpack size matches official `upx -d` (3837656)
  - Sample: `samples/upx_unpack.glk`


### Docs
- `docs/ROADMAP_1.0.md`, this CHANGELOG.
- CLI `gltk version` → **1.0.0**.

### Not in 1.0 (explicit)
- Capstone-level full ISA semantics, Enigma/VMP unpackers, package manager, debugger UI.
- Pretty GUI theming / full dialog keyboard navigation.
- Dynamic debugging / process injection.

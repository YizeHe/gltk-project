# GrokLangToolKit (gltk)

**语言 / Language：** [中文](README.md) · [English](README.en.md) · [Tiếng Việt](README.vi.md)

---

**GrokLang** 逆向工程语言工具包：自研 **寄存器字节码虚拟机 GLVM**。  
主执行路径 **不是** AST 解释，也 **不是** 转译到 Python/JS。

| 项 | 值 |
|---|---|
| 版本 | **1.0.0** |
| 语言 | GrokLang（`.glk`） |
| 字节码 | `.glkb`（magic `GLKB`） |
| CLI | `gltk` / `gltk.exe` |
| 模块路径 | `groklang/gltk` |
| 运行时 | 寄存器机 GLVM + Go native 内建 |

本仓库为 **开源核心 + samples + 语言教程**（`learn/`）。  
私有逆向样本库、大体量二进制与周边产品 **不** 包含在此仓库中。

## 快速开始

```powershell
git clone https://github.com/YizeHe/gltk-project.git
cd gltk-project

go test ./...
go build -o gltk.exe ./cmd/gltk
# 可选：打包用运行时桩
go build -o glrt.exe ./cmd/glrt

.\gltk.exe version
.\gltk.exe run samples\hello.glk
.\gltk.exe run samples\use_lib.glk
.\gltk.exe compile samples\hello.glk -o hello.glkb
.\gltk.exe disasm hello.glkb

# 打独立 EXE（需 glrt.exe 与 gltk 同目录，或指定 --stub）
.\gltk.exe pack samples\pack_hello.glk -o pack_hello.exe
```

或：

```powershell
.\build.ps1
```

## 仓库结构

```
cmd/gltk          # CLI
cmd/glrt          # 打包运行时桩
internal/         # 词法 / 语法 / 编译器 / 虚拟机 / 原生模块
stdlib/           # 标准 .glk 库
libs/             # 示例用户库
samples/          # 可运行示例（hello、GUI、PE、网络、打包等）
docs/             # 语言 / VM / 标准库 / GUI 文档
learn/            # 分章教程 + 练习（中文）
third_party/cui   # 内嵌 Win32 GUI 库（CUI）
```

## 学习 GrokLang

从 **[learn/README.md](learn/README.md)** 开始（第 01–09 章）。

```powershell
.\gltk.exe run learn\examples\ch01_hello.glk
.\gltk.exe run learn\examples\ch02_06_mini_calc.glk
```

## 架构

```
  .glk 源码 (+ path import)
       │
       ▼
  lexer → parser → AST
       │
       ▼
  module.CompileEntry  →  库文件 → 共享 Chunk
       │
       ▼
  compiler → Chunk (.glkb) → GLVM.Run → natives
```

**唯一执行路径**：`Chunk` → `VM.Run`。  
`gltk run file.glk` = 多文件编译 + 虚拟机执行。

## import

```glk
import pe, fs, out                 // bare → 原生模块
import "libs/helpers.glk"          // 路径 → 别名 helpers
import "libs/helpers.glk" as h     // 显式别名
```

库搜索顺序：入口文件目录 → `./lib` `./libs` → `<根>/stdlib` `<根>/libs` → 环境变量 `GLTK_LIB` → 当前工作目录。

## 示例（samples）

仓库内完整提供 `samples/`，包括：

| 示例 | 主题 |
|------|------|
| `hello.glk` | Hello World |
| `use_lib.glk` | 用户库 import |
| `lang_1_0.glk` / `lang_features.glk` | 语言特性 |
| `pe_info.glk` / `string_scan.glk` | PE / 字符串 |
| `disasm_demo.glk` / `entropy_scan.glk` | 反汇编 / 熵 |
| `gui_hello.glk` / `gui_re_workbench.glk` | GUI |
| `net_demo.glk` / `async_demo.glk` | 网络 / 异步 |
| `pack_hello.glk` | 打包 EXE |
| `upx_unpack.glk` | UPX（纯 Go 路径） |

部分示例需要在 `--` 后传入 **你自己的** 二进制路径。  
仓库 **不** 附带恶意样本集。

## 文档

- [docs/language.md](docs/language.md)
- [docs/vm.md](docs/vm.md) · [docs/bytecode.md](docs/bytecode.md)
- [docs/stdlib.md](docs/stdlib.md)
- [docs/GUI_INTEGRATION.md](docs/GUI_INTEGRATION.md)
- [docs/PACK.md](docs/PACK.md) · [docs/ASYNC.md](docs/ASYNC.md)

## 许可证

MIT — 见 [LICENSE](LICENSE)。

## 本仓库不包含

- 大体量私有 `testfile/`、`work/` 目录  
- 第三方逆向课程 / Android 工具链捆绑包  
- 代理 / VLESS 等周边产品  
- 预编译 `gltk.exe`（请从源码构建）

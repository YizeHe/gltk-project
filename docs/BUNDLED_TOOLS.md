# 捆绑逆向工具一览

GrokLangToolKit 将用户放入仓库的第三方工具 / 课程 / 样例统一登记到 **toolkit 注册表**，通过 CLI 与 GrokLang `import tools` 调用。

## 目录布局（保持原仓库结构，不强制搬迁）

| 路径 | 内容 | 类别 |
|------|------|------|
| `droidReverse/` | Android 反编译合集：apktool / dex2jar / jadx / jd-gui / jeb / ClassyShark / smali2java | android |
| `wxapkg/` | 微信小程序 `.wxapkg` Wails GUI 工程；核心算法亦在 GLVM `import wxapkg` | wechat |
| `Reverse-Engineering/` | Windows API 练习用可执行样例与工程 | windows |
| `Z0FCourse_ReverseEngineering/` | Z0F x64 Windows 逆向课程（Markdown + FilesNeeded） | course |
| `resverse-engineering2/reverse-engineering/` | Awesome reverse-engineering 资源清单 | resource |

## CLI

```powershell
cd D:\grokbuild\groklang\gltk
go build -o gltk.exe .\cmd\gltk

.\gltk.exe tools list
.\gltk.exe tools list --category android
.\gltk.exe tools list -q jadx
.\gltk.exe tools info apktool
.\gltk.exe tools path jadx
.\gltk.exe tools run jadx -- --help
.\gltk.exe tools run apktool -- d app.apk -o out_apk

.\gltk.exe course list
.\gltk.exe course path z0f-ch4

.\gltk.exe wxapkg unpack sample.wxapkg .\out_wx [wxid]
```

**说明**

- `tools run` 对 **GUI** 工具会阻塞直到窗口关闭。
- `apktool` / `ClassyShark` 需要系统安装 **Java** 并在 PATH 中。
- `course` / `resource` / `dir` 类型不可 `run`，用 `path` / `info` 打开文档路径。
- Windows lab 可执行文件仅供静态分析练习，请勿在生产环境随意执行未知样本。

## GrokLang

```glk
import tools, out

fn main(args) {
  out.println(tools.root())
  let all = tools.list("android")
  let r = tools.run("jadx", ["--help"])
  out.println(r.exit, r.output)
  return 0
}
```

```powershell
.\gltk.exe run samples\list_tools.glk
.\gltk.exe run samples\list_tools.glk -- android
```

## 自行扩展

1. 把新工具目录放在 `gltk/` 下（或任意固定相对路径）。
2. 在 `internal/toolkit/catalog.go` 的 `Definitions()` 增加一条 `Entry`。
3. 重新 `go build`。  
   （`Reverse-Engineering` 下额外的 `.exe` 会在扫描时自动发现为 `win-lab-*`。）

环境变量：

- `GLTK_LIB`：GrokLang `.glk` 库搜索路径（与工具注册表无关，用于 `import "lib.glk"`）。

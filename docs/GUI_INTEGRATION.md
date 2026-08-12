# CUI → GrokLang 整合计划

## 外部进度监控

路径：`C:\Users\QY\Desktop\项目\CUI`（mimo 负责）

| 目录 | 职责 |
|------|------|
| `win32/` | 原始 Win32 类型与 API 绑定 |
| `core/` | 窗口/消息循环封装 |
| `widget/` | 按钮/输入框等控件 |
| `layout/` | 线性/网格布局 |
| `menu/` | 菜单 |
| `clipboard/` | 剪贴板 |

验收题：`q.md`（20 题 Win32 GUI 综合测试）。

## 当前状态（2026-07-20 整合完成）

- CUI 验收：`go build ./...` 通过；含 `main.go` 20 题入口、`examples/hello`、README
- 包：`win32` / `core` / `widget` / `layout` / `menu` / `clipboard`
- GrokLang：`import gui` → `internal/native/gui.go`（`//go:build windows`）
- `go.mod`：`require cui` + `replace cui => C:/Users/QY/Desktop/项目/CUI`
- 示例：
  - `samples/gui_hello.glk`
  - `samples/gui_re_workbench.glk`（选文件 + pe.summary 日志）

### 已暴露 API

```
gui.available / version / init
gui.window / show / hide / run / quit / set_title / on_close
gui.label / button / lineedit / textedit / checkbox
gui.set_text / get_text / append_text / set_bounds
gui.on_click / msgbox / open_file
gui.vbox / hbox / add
```

## 整合目标

```
import gui, async, pe, out

fn on_analyze() {
  let path = gui.open_file("选择样本")
  let fut = async.spawn(analyze_pe, [path])
  # UI 保持响应
  let report = async.await(fut)
  gui.set_text(log_box, report)
}

fn main(args) {
  let w = gui.window("GrokLang RE", 960, 640)
  ...
  gui.run()  # 消息循环
}
```

## 整合步骤（CUI 合格后）

1. 将 CUI 以 `replace` 或子模块放入 `gltk/third_party/cui` 或 `import cui`
2. `internal/native/gui.go` 包装：
   - `gui.window` / `button` / `label` / `textbox` / `run` / `quit`
   - `gui.open_file` / `save_file` / `msgbox`
   - `gui.post(callback)` 线程切回 UI
3. 标准库 `stdlib/re_workbench.glk` 示例工作台
4. 文档与 `samples/gui_re_workbench.glk`

## 轮询命令

```powershell
Get-ChildItem "C:\Users\QY\Desktop\项目\CUI" -Recurse -File | 
  Sort-Object LastWriteTime -Descending | Select-Object -First 20 FullName, LastWriteTime, Length
```

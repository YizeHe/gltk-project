# CUI - 纯Win32 API自研Go GUI库

一个完全从零开始、不依赖任何第三方GUI库的Go语言Windows原生GUI框架。

## 特性

- **纯Win32 API**: 直接通过 `syscall.NewLazyDLL` 调用 user32.dll、gdi32.dll、kernel32.dll 等
- **自绘控件**: 所有控件（Button、Label、CheckBox、RadioButton等）均自行绘制
- **布局系统**: 支持水平布局(HBox)、垂直布局(VBox)、网格布局(Grid)
- **事件系统**: 完整的鼠标事件、键盘事件、焦点事件支持
- **GDI绘图**: 封装GDI绘图API，支持圆角矩形、渐变、位图等
- **高DPI**: 支持Per-Monitor V2 DPI感知
- **资源管理**: 自动追踪和释放GDI资源，防止泄漏

## 项目结构

```
CUI/
├── main.go                 # 主程序，包含20道测试题入口
├── win32/                  # Win32 API绑定层
│   ├── types.go            # Win32类型定义
│   ├── constants.go        # Win32常量
│   ├── user32.go           # user32.dll绑定
│   ├── gdi32.go            # gdi32.dll绑定
│   ├── kernel32.go         # kernel32.dll绑定
│   ├── gdiplus.go          # GDI+绑定(图片加载)
│   └── clipboard.go        # 剪贴板API
├── core/                   # 框架核心
│   ├── app.go              # 应用程序单例、消息循环
│   ├── window.go           # 窗口封装
│   ├── widget.go           # 控件基类
│   ├── canvas.go           # GDI绘图画布
│   ├── font.go             # 字体管理
│   ├── brush.go            # 画刷管理
│   ├── pen.go              # 画笔管理
│   ├── bitmap.go           # 位图管理
│   ├── color.go            # 颜色类型
│   ├── rect.go             # 矩形类型
│   ├── event.go            # 事件类型
│   ├── dpi.go              # DPI适配
│   ├── dispatcher.go       # 消息路由
│   └── resource_manager.go # 资源管理
├── widget/                 # 控件实现
│   ├── label.go            # 静态文本
│   ├── button.go           # 按钮
│   ├── checkbox.go         # 复选框
│   ├── radiobutton.go      # 单选框
│   ├── lineedit.go         # 单行输入框
│   ├── textedit.go         # 多行文本框
│   ├── imageview.go        # 图片控件
│   ├── panel.go            # 容器面板
│   └── custom_draw.go      # 自绘控件基类
├── layout/                 # 布局系统
│   ├── layout.go           # 布局接口
│   ├── hbox.go             # 水平布局
│   ├── vbox.go             # 垂直布局
│   └── grid.go             # 网格布局
├── menu/                   # 菜单系统
│   └── menu.go             # 菜单和上下文菜单
└── clipboard/              # 剪贴板
    └── clipboard.go        # 剪贴板读写
```

## 快速开始

```go
package main

import (
    "cui/core"
    "cui/layout"
    "cui/widget"
)

func main() {
    app := core.NewApp()

    win := app.NewWindow("Hello CUI", 400, 300)
    win.CenterScreen()

    vbox := layout.NewVBoxLayout()
    vbox.SetPadding(20)
    vbox.SetSpacing(10)
    win.SetLayout(vbox)

    label := widget.NewLabel("Hello, World!")
    label.SetFont(core.NewFont("Microsoft YaHei UI", 16))
    win.AddWidget(label)

    btn := widget.NewButton("Click Me")
    btn.SetOnClick(func() {
        label.SetText("Button Clicked!")
    })
    win.AddWidget(btn)

    win.Show()
    app.Run()
}
```

## 编译运行

```bash
go build -o cui.exe .
./cui.exe
```

## 测试题覆盖

| 编号 | 测试内容 | 状态 |
|------|----------|------|
| 01 | 基础窗口：居中、最小尺寸、关闭 | ✓ |
| 02 | 无边框窗口（简化版） | ✓ |
| 03 | 模态弹窗对话框 | ✓ |
| 04 | 多窗口管理 | ✓ |
| 05 | 静态文本控件 | ✓ |
| 06 | 按钮/复选框/单选框 | ✓ |
| 07 | 单行输入框 | ✓ |
| 08 | 多行文本框 | ✓ |
| 09 | 图片控件 | ✓ |
| 10 | 水平线性布局 | ✓ |
| 11 | 垂直线性布局 | ✓ |
| 12 | 网格布局 | ✓ |
| 13 | 鼠标事件 | ✓ |
| 14 | 键盘事件+快捷键 | ✓ |
| 15 | 右键上下文菜单 | ✓ |
| 16 | 自绘自定义控件 | ✓ |
| 17 | 窗口全局绘图 | ✓ |
| 18 | 剪贴板交互 | ✓ |
| 19 | 压力测试 | ✓ |
| 20 | 边界容错 | ✓ |

## 技术细节

### 控件实现策略
- **自绘控件**: Button、Label、CheckBox、RadioButton、ImageView等通过注册自定义窗口类("CUIWidget")实现完全自绘
- **原生控件**: LineEdit、TextEdit使用Win32原生"Edit"控件，通过SendMessage实现扩展功能

### 消息路由
使用全局HWND→Widget映射表，WndProc根据HWND查找对应控件并分发消息。

### 资源管理
ResourceManager追踪所有GDI对象(HFONT、HBRUSH、HPEN、HBITMAP)，窗口销毁时自动释放。

### 高DPI
启动时调用 `SetProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)`，支持Per-Monitor DPI缩放。

# GLTK 1.0 收口清单

## A. GUI（可用即可）
- [x] 历史 Win32 坑（BindBaseWidget / WS_CHILD host / FillRect / hit-test / Quit / 中文）
- [x] 顶层 let 全局（语言修复，GUI 回调能更新控件）
- [x] CUI 迁入 third_party（可复现构建）
- [x] Label 背景擦除
- [x] open_file 对话框稳健
- [x] checkbox API
- [x] gui_smoke + global-handle 回归

## B. 语言 1.0
- [x] 顶层 let → STOREG
- [x] push/pop/keys/has/delete/clone
- [x] array+array
- [x] try 覆盖更多运行时错误
- [x] 寄存器错误带行号

## C. 逆向库
- [x] stdlib/re_triage.glk 宽谱 triage
- [x] samples/re_batch_triage.glk
- [ ] 用户大量实战样本批处理 → 按失败案例补全库（进行中/待输入）

## D. 发布
- [x] version 1.0.0
- [x] CHANGELOG
- [ ] git push（全部实战样本逆向成功 + 用户确认后）

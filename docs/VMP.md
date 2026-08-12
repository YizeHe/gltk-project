# VMP Assist（VMProtect 脱壳辅助）

> **范围**：检测、节提取、内存 dump 的 PE 头修复。  
> **不做**：完整 VMP handler 去虚拟化 / 通用「一键破解」。

## 背景

VMProtect 3.x 全虚拟化常见特征：

- 节名 `.vmp0` / 变异名（如 `.EY,`、`.;lt`）
- 多个节 **磁盘 raw_size=0**，仅在加载后内存解密
- 入口在 VMP 调度节，导出仅为 JMP/CALL 桩

业务上往往更有效的是绕过（例如 ScriptGuard 层），但静态 triage 与 dump 修头仍需要工具。

## CLI

```text
gltk vmp detect  <file>
gltk vmp info    <file>
gltk vmp extract <file> <outdir>
gltk vmp fixdump <memory_dump.exe> [out.exe]
gltk vmp assist  <file> <outdir>
```

## GrokLang API

```glk
import vmp

vmp.detect(path|bytes) -> bool
vmp.info(path|bytes)   -> map   // alias: analyze
vmp.extract(path|bytes, out_dir) -> map
vmp.fixdump(in_path, out_path?)  -> map
vmp.assist(in_path, out_dir)   -> map
vmp.report(path|bytes)         -> string
```

## 推荐流程

1. **磁盘样本**  
   `gltk vmp assist sample.dll work\vmp_out`  
   → `vmp_report.json` / `vmp_report.txt`、非空节 blob、`raw_full.bin`

2. **若大量 DISK_EMPTY**  
   - 在调试器/沙箱加载模块，等 DllMain/解密完成  
   - dump 整镜像（内存布局）  
   - `gltk vmp fixdump dump.exe dump.fixed.exe`  
   - 再用 `pe` / `disasm` / 字符串扫描

3. **不要期望**  
   - 自动还原全部虚拟化函数为干净 x64  
   - 无条件绕过反调试

## 示例

```powershell
.\gltk.exe vmp info YourCrushLY.dll
.\gltk.exe vmp assist YourCrushLY.dll work\vmp_yc
.\gltk.exe run samples\vmp_assist.glk -- YourCrushLY.dll work\vmp_yc
```

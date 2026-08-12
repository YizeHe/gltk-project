# VMP 脱壳 / 去虚拟化方案（按历史实操步骤）

> 目录：`devmp/`  
> 状态：文档方案（**完整去虚拟化未做成自动化工具**）  
> 对照实现：GLTK `import vmp` / `gltk vmp` 仅覆盖 **检测 · 提取 · dump 修头**（见 [../docs/VMP.md](../docs/VMP.md)）  
> 主要案例：`YourCrushLY.dll`（VMProtect **3.x 全虚拟化**，AMD64）

---

## 0. 先说清目标分层

| 层级 | 含义 | 当时做到哪 | 工具现状 |
|------|------|------------|----------|
| L0 识别 | 是不是 VMP、几节空、入口在哪 | ✅ 完整 | `gltk vmp info/detect` |
| L1 静态抠料 | 磁盘非空节、报告、字符串线索 | ✅ 完整 | `gltk vmp extract/assist` |
| L2 运行时解密后 dump | DllMain 后内存镜像落盘 | ⚠️ 手工/半自动，踩过 anti-dump | `gltk vmp fixdump`（只修头） |
| L3 IAT / 导出重建 | dump 后能被 IDA/Ghidra 当普通 PE 打开 | ⚠️ 部分推断 + 导出 ordinal 表 | 半手工 |
| L4 Handler 语义还原 | 每个 VM opcode → 等价 x64 | ❌ 未自动化 | 本方案描述步骤 |
| L5 业务绕行 | 不脱 VMP 也能拿到结果 | ✅ ScriptGuard / 协议层 | `import ahk` 等 |

**原则（历史复盘）**：对 YourCrush 一类产品，**主战场往往是 ScriptGuard（AHK）而不是 VMP Core**；VMP 只在需要 DLL 内部 HMAC/激活逻辑时才值得继续往 L2–L4 走。硬调 VMP 导出导致崩溃，还曾关联 MACHINE_BANNED 风控，见业务报告。

---

## 1. 样本画像（以 YourCrushLY.dll 为例）

以下数据来自当时 `REPORT_FINAL.md` / `REPORT_GLTK.md` / 综合分析，用于对齐「曾经做过的步骤」：

| 项 | 值 |
|----|-----|
| 大小 | ≈ 16,582,656 bytes |
| 架构 | PE32+ / AMD64 |
| 保护 | VMProtect **3.x 全虚拟化** |
| 磁盘空节 | `.text` `.vmp0` `.rdata` `.data` `.pdata` `.fptable` `.EY,` 等（raw_size=0） |
| 磁盘有料 | 如 `.;lt`（调度/字节码主体，体积约 16MB）、`.reloc` 等 |
| 入口 | 在 **VMP 调度节**（如 `.;lt`），非正常 `.text` OEP |
| 入口 stub（x64） | 典型：`PUSH R13; PUSHFQ; MOVABS R13, imm64; … CALL dispatcher` |
| 版本密钥常量（案例） | `R13 = 0xC62B618AB5A87930`（报告称 VMP version key） |
| Handler 规模（估计） | ~200+ handler；节内大量 `0xCC`（反调试/填充） |
| 导出 | 多为 **ordinal-only**（案例 59 个），无符号名 |
| 导入 | 每 DLL 常只露 1 个 API，其余被 VMP 静态解析/间接调用 |

### 1.1 节语义（工作假设）

```
磁盘布局（示意）
├── 空 raw：.text / .vmp0 / .rdata / .data / .EY, …
│     └── 仅在 DllMain / VMP 自解密后出现在内存
└── 有 raw：.;lt（调度器 + 大量 VM 字节码）、.reloc …

内存解密后（示意）
├── .text          部分桩 / 非虚拟化残留
├── .vmp0          VM 入口 stub 元数据
├── .EY,           虚拟化代码 + 加密块（主体）
├── .;lt           调度器 + handler 表 + opcode 流
└── 真实 IAT       往往藏在加密区，磁盘 IAT 不完整
```

---

## 2. 总流程（按时间顺序做一遍）

```
[A] 文件级 triage
      ↓
[B] 保护确认（VMP 3.x / 是否全虚拟化）
      ↓
[C] 导出 / 导入 / 字符串静态情报
      ↓
[D] 决定策略：绕行 vs 硬脱
      ├─ 绕行：ScriptGuard / 上层协议  → 往往足够
      └─ 硬脱：进入 E
            ↓
[E] 无调试器加载 + 等待自解密
      ↓
[F] 内存 dump（全镜像）
      ↓
[G] fixdump / 对齐 PE 头
      ↓
[H] 重建视图：节、导出、尽量修 IAT
      ↓
[I] Handler 表与调度循环逆向（L4）
      ↓
[J] 按导出 ordinal 逐函数跟踪 / 符号执行 / 记录 trace
      ↓
[K] 产出：伪代码、协议字段、或补丁点
```

下面按阶段写可执行细节。

---

## 3. 阶段 A–C：静态 triage（GLTK 已可自动化）

### 3.1 命令

```powershell
cd D:\path\to\gltk
go build -o gltk.exe ./cmd/gltk

# 识别
.\gltk.exe vmp detect YourCrushLY.dll
.\gltk.exe vmp info   YourCrushLY.dll

# 一键输出目录：报告 + 非空节 + raw_full
.\gltk.exe vmp assist YourCrushLY.dll work\devmp_yc

# 或 GrokLang
.\gltk.exe run samples\vmp_assist.glk -- YourCrushLY.dll work\devmp_yc
```

### 3.2 人工核对清单

1. **空磁盘节数量** ≥ 3 且含 `.vmp0` / 怪异节名 → 高度疑似 VMP 运行时解密。  
2. **入口节** 是否为 `.;lt` / 非 `.text`。  
3. **入口字节** 是否 `49 55`（`push r13`）等 stub。  
4. **字符串** 是否出现 `VMProtect` / `.vmp0`（有时被抹）。  
5. **导出**：`dumpbin /exports` 或自研解析 → ordinal 表写入 `exports.json`。  
6. **导入**：是否「每 DLL 一个 API」→ 说明真实 IAT 在 VM 内解析。

### 3.3 产出物（建议目录）

```
work/devmp_yc/
  vmp_report.json
  vmp_report.txt
  raw_full.bin
  section_*.bin          # 仅磁盘有 raw 的节
  exports.json           # 手补 ordinal 列表
  notes.md               # 本轮观察
```

---

## 4. 阶段 D：策略选择（历史结论）

| 你要的结果 | 建议路径 | 原因 |
|------------|----------|------|
| 宏脚本明文 | **ScriptGuard**（`import ahk`） | 不在 VMP 内 |
| 激活/HMAC/POST 体 | 优先 **抓包 + 合法 DLL 路径**，慎直调 ordinal | 外部乱调易 AV 崩溃 + 服务端风控 |
| DLL 内算法完整还原 | 必须 **L2–L4** | 才值得 dump + handler |

历史踩坑：

- C/Python 反复 `GetProcAddress(ordinal)` 调 `BuildValidatePost` 等 → **ACCESS_VIOLATION**。  
- 无 HMAC 的裸 POST → 可能被标破解行为。  
- **结论**：能绕就绕；必须进 VMP 时，先保证 **干净 dump**，再谈去虚拟化。

---

## 5. 阶段 E–F：运行时解密与 Dump（最关键、最容易失败）

### 5.1 为何磁盘 dump 不够

VMP 把核心节 **raw 置 0**，解密逻辑在加载路径里（DllMain / TLS / stub）。  
**只有进程地址空间里** 才有完整 `.text` / `.EY,` / 解密后的数据。

### 5.2 环境要求（按当时 dumper 观察）

1. **尽量无调试器** 先让 DLL 正常加载完成自解密。  
   - 报告：附加调试器后 anti-dump 触发 → **后续读内存变空/自毁**。  
2. 可选：沙箱 / 干净 VM，关掉常见分析窗口（EnumWindows 类检测）。  
3. 加载方式示例：
   - 宿主 EXE 正常启动并 LoadLibrary；或  
   - 最小 loader：`LoadLibraryW` 后 **Sleep** 足够时间再 dump。  
4. dump 时机：
   - DllMain 返回之后；  
   - 若有延迟解密，在 **第一次调用某导出之前/之后** 各 dump 一份对比。

### 5.3 Dump 内容范围

建议 dump **整个模块映像**：

```
base = 模块基址（PEB / Module32 / EnumProcessModules）
size = SizeOfImage（OptionalHeader）
读 [base, base+size)  → dump.bin
```

可选额外：

- 堆上 `VirtualAlloc` 出的 VMP 解密暂存区（难自动化，靠 VirtualQuery 扫 RX 私有页）。  
- 导出表在内存中的解析结果（便于对照 ordinal）。

### 5.4 注入 / 外置 dumper 注意点

当时 `vmp_dumper_x64` 类思路：

1. 目标进程 **已加载且未处于调试**。  
2. 用 **远程线程** 或外部 `ReadProcessMemory` 拉取映像（后者更不易触发部分反调试，但仍可能完整性校验）。  
3. 失败表现：RPM 全 0、节大小对但内容空、进程直接退出。  
4. 对策：换无调试路径、延迟 dump、dump 后立刻脱离、或内核级读（成本高，一般不做）。

### 5.5 不建议的做法

- 在 x64dbg 里一打开就下断在入口再 dump（经常已触发反调试）。  
- 只 dump `.text` 一小段（丢失 handler 与字节码）。  
- 把磁盘空节文件当「已解密」拿去 IDA。

---

## 6. 阶段 G：fixdump（PE 头修复）

内存布局常见特征：

- `PointerToRawData == 0` 或 `== VirtualAddress`  
- `FileAlignment` 仍是文件对齐，和内存页不对齐  

### 6.1 GLTK

```powershell
.\gltk.exe vmp fixdump memory_dump.dll memory_fixed.dll
```

工具行为（摘要）：

- `FileAlignment := SectionAlignment`  
- 各节 `PointerToRawData := VirtualAddress`  
- `SizeOfRawData` 按 VirtualSize 对齐并夹在镜像内  
- **不** 去虚拟化、**不** 保证 IAT 正确  

### 6.2 手工检查

1. CFF Explorer / PE-bear 打开 `memory_fixed.dll` 是否报致命错。  
2. 节内容是否非空（`.EY,` / `.text` 应有高熵数据）。  
3. 入口 RVA 是否仍指向调度 stub。  
4. 用 GLTK：

```powershell
.\gltk.exe vmp info memory_fixed.dll
.\gltk.exe run samples\string_scan.glk -- memory_fixed.dll
.\gltk.exe run samples\pe_info.glk -- memory_fixed.dll
```

---

## 7. 阶段 H：导出与 IAT 视图重建

### 7.1 导出（ordinal）

1. 从 **未保护宿主** 或导入该 DLL 的 EXE 反汇编 `GetProcAddress(h, ordinal)`。  
2. 或在内存 dump 的导出目录解析（若 VMP 未抹导出）。  
3. 维护表：

```json
{
  "33": "BuildValidatePost",
  "34": "SendRequest",
  "...": "..."
}
```

4. 在 IDA/Ghidra 对导出桩批量命名：`ord_33` → 业务名。

### 7.2 导入（真实 IAT）

磁盘导入往往是「烟幕」。内存中：

1. 找 `LoadLibrary` / `GetProcAddress` 的 VMP 包装 handler。  
2. 或在第一次调用导出后 dump 的 **IAT 槽位** 是否已填真实函数指针。  
3. 用 Scylla / 自研脚本对 **dump 进程** 修 IAT（比纯文件修更准）。  

历史推断用到的 API 家族（非完整）：

- 网络：`WinHttp*` / WinINet  
- 加密：`BCrypt*` / `Crypt*`  
- 反调试：`IsDebuggerPresent`、`NtQueryInformationProcess`、`NtSetInformationThread`  
- 其它：`EnumWindows`、`GetPixel`、`WinVerifyTrust`  

### 7.3 节权限

解密后确认 `.EY,` / handler 区是否 RX；若 dump 丢了 Characteristics，按可执行节修，避免分析工具误判为数据。

---

## 8. 阶段 I：Handler 与调度循环（去虚拟化核心，手工）

> 此阶段 **GLTK 不做自动**；下列为当时静态识别 + 常规 VMP 方法论。

### 8.1 定位调度器

1. 从 **入口 stub** 跟到 `CALL dispatcher`。  
2. 案例描述的模式（逻辑伪代码）：

```c
// 示意，非精确还原
ctx.regs.r13 = 0xC62B618AB5A87930;   // version / key
// 保存通用寄存器与标志
uint8_t *handlers = /* handler 表基址 */;
for (;;) {
    uint8_t opcode = fetch_bytecode(&ctx);
    handlers[opcode](&ctx);            // 间接调用
    // 可能有 opcode 变换 / 下一 PC 计算
}
```

3. 在 dump 中标出：
   - **bytecode 指针 / VIP**（虚拟 IP）  
   - **handler 表**（函数指针数组或 jmp table）  
   - **上下文结构**（寄存器映射、flags、密钥）

### 8.2 Handler 分类（历史 Capstone 分类方向）

| 类别 | 行为倾向 |
|------|----------|
| ALU | add/sub/xor/and/or/not/shift |
| 栈 | push/pop 虚拟栈 |
| 控制流 | jcc / jmp / call / ret（改 VIP） |
| 内存 | load/store 相对模块基址 |
| 系统 | 包装 API 调用（间接） |
| 混淆 | 垃圾运算、不透明谓词、密钥混合 |

步骤：

1. 对 handler 表每一项反汇编，建 **opcode → 语义笔记**。  
2. 找「读 VIP → 取 opcode → 查表 → 执行 → 写回 VIP」循环边界。  
3. 注意 **opcode 本身可能再加密**（按密钥滚动）。

### 8.3 字节码区

- 高熵、几乎填满 → 正常。  
- 记录：`[handler_table)`、`[bytecode_stream)`、`[encrypted_native_islands)`（报告中曾划分区间示意）。  
- 对「保护的原始 x64 岛屿」：有时 VMP 只虚拟化部分函数，岛屿可直接当 native 分析。

---

## 9. 阶段 J：按导出逐函数推进（务实去虚拟化）

完整自动 decompiler 不现实时，用 **目标驱动**：

1. 选一个业务导出（如发送请求 / 校验）。  
2. 在 **无反调试环境** 下用硬件断点 / 日志（尽量非调试器）观察：
   - 入参寄存器 / 栈  
   - 是否跳进 VMP stub  
   - 出参、写全局缓冲  
3. Trace 方案（由易到难）：
   - **黑盒**：只记录输入输出（协议复现够用就停）。  
   - **指令级 trace**（单步极慢且易检测）。  
   - **符号执行**（angr 等）从 stub 跑到第一次 API 调用（成本高）。  
4. 对重复出现的 handler 序列做 **模式折叠** → 伪代码。  
5. 产出最小可用文档：`ord_XX(in) -> out` + 关键常量（AES key 形态、URL、字段顺序）。

### 9.1 与「完整去虚拟化」的差距

| 完整 de-virtualize | 本方案可交付 |
|--------------------|--------------|
| 所有虚拟函数 → 干净 x64 | 单导出 IO 规格 / 伪代码片段 |
| 通用 handler 库 | 单样本 handler 笔记 |
| 一键工具 | 人工 + GLTK L0–L2 辅助 |

---

## 10. 阶段 K：交付物模板

```
devmp_case_<name>/
  00_hashes.txt
  01_vmp_report.json          # gltk vmp assist
  02_exports_ordinals.json
  03_disk_sections/           # extract
  04_memory_dump.bin
  05_memory_fixed.dll         # fixdump
  06_handler_notes.md         # opcode 表
  07_ord_XX_io.md             # 业务导出 IO
  08_iat_reconstructed.txt
  09_decision.md              # 为何停在某层 / 是否改走 ScriptGuard
```

`09_decision.md` 必须写清：**继续 L4 的 ROI** vs **绕行是否已满足目标**。

---

## 11. 反调试与稳定性（历史清单）

| 现象 | 可能原因 | 处理 |
|------|----------|------|
| 一附加调试器内存变空 | anti-dump / 自毁 | 先无调试加载再 RPM dump |
| 调导出立刻 AV | 完整性 / 环境检测失败 | 检查调用约定、父进程、是否缺 TLS |
| 3991 INT3 区 | 填充 / 陷阱 | 不要当有效代码入口乱下断 |
| EnumWindows | 扫分析工具窗口 | 改窗口标题 / 远程 dump |
| GetPixel | 反截屏类检测 | 无头环境或跳过 UI |
| WinVerifyTrust | 校验签名链 | 注意改 dump 后签名失效属正常 |

---

## 12. 与 GLTK 其它模块的配合

| 目标 | 模块 |
|------|------|
| VMP L0–L2 | `vmp` |
| PE 概览 | `pe.summary` / `pe.parse` |
| 字符串 | `str` / `samples/string_scan.glk` |
| 反汇编碎片 | `disasm` |
| AHK 层绕行 | `ahk.decrypt_scriptguard*` |
| UPX 外壳（若有） | `upx.unpack` |
| 熵 | `samples/entropy_scan.glk` |

推荐组合拳（磁盘样本）：

```powershell
.\gltk.exe vmp assist sample.dll work\vmp
.\gltk.exe run samples\entropy_scan.glk -- sample.dll
.\gltk.exe run samples\string_scan.glk -- work\vmp\raw_full.bin
# 若上层是 AHK+ScriptGuard 宿主：
.\gltk.exe run samples\scriptguard_decrypt.glk -- ...
```

---

## 13. 风险与合规

1. 仅用于 **自有样本 / 授权分析**。  
2. 对线上激活接口的重放、爆破不在本方案范围。  
3. 外部模拟 VMP 导出可能导致 **崩溃与账号风控**，历史案例已有 MACHINE_BANNED 关联推断。  
4. 本目录文档 **不提供** 可武器化的完整通用 VMP 自动去虚拟化实现。

---

## 14. 检查清单（做完勾选）

- [ ] `gltk vmp info` 确认 is_vmp / 空节 / 入口节  
- [ ] `assist` 产出 report + 非空节  
- [ ] 明确目标在 L2 还是 L4，还是改走 ScriptGuard  
- [ ] 无调试器成功 LoadLibrary  
- [ ] 取得 SizeOfImage 级 memory dump  
- [ ] `fixdump` 后节内容非空  
- [ ] exports ordinal 表落地  
- [ ] （可选）handler 表与至少 1 个业务导出的 IO 说明  
- [ ] `09_decision.md` 写清停止条件  

---

## 15. 参考路径（仓库内）

| 路径 | 内容 |
|------|------|
| [../docs/VMP.md](../docs/VMP.md) | 工具用法 |
| [../samples/vmp_assist.glk](../samples/vmp_assist.glk) | 脚本示例 |
| [../internal/vmp](../internal/vmp) | L0–L2 实现 |
| 历史报告（私有树，未必开源） | `GTAOL/YourCrushLY/REPORT_*.md`、综合封号分析 |

---

## 16. 一句话总结

**曾经「做过的 VMP」= 完整静态 triage + 对运行时解密与 anti-dump 的认知 + dump 修头 + ordinal/业务绕行；**  
**没有做成通用去虚拟化引擎。**  
本方案把这条路按阶段写死，便于后人 **按同一顺序复现到 L2，并在必要时手工冲 L4**；自动化止步于 GLTK `vmp` 辅助工具。

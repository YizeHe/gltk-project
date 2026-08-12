# GLTK 内存与「垃圾回收」

## 结论（先读这个）

**GLTK 没有自建 GC。**  
运行时 `Value`、数组、映射、闭包、字符串都是普通 Go 堆对象，由 **Go runtime GC** 回收。

这不是“垃圾实现”，而是**宿主语言托管模型**的标准做法（与嵌入 Lua/JS 自建堆不同）：

| 模型 | GLTK |
|------|------|
| 谁分配 | Go 分配器（`make` / 逃逸到堆的 `Value` 字段） |
| 谁回收 | Go concurrent mark-sweep GC |
| 脚本侧 API | 无 `free()`；可选 `async.drop` 释放 **native 句柄表** |
| 真正会“漏”的 | **非内存**资源：文件句柄、全局 async handle 表、VM 帧未清引用 |

## 运行时对象图

```
VM
├── frames[] → Frame{ regs[]Value, upvals[]Value }
├── globals map[string]Value
├── modules
├── files map[id]*os.File     ← 需 Close / VM.Release
└── async.futures             ← 完成后 forget

Value (tagged union, 较大)
├── 标量: bool/int/float/str
├── Bytes []byte              ← 可与父切片共享底层数组
├── Arr *[]Value              ← 引用语义
├── Map *map[string]Value
└── Fn *Closure{ Proto, Upvals }
```

- **数组/映射**：引用语义；多寄存器可指向同一对象。  
- **字符串**：不可变，Go string。  
- **Bytes**：尽量零拷贝子切片；父数组存活则子视图安全（Go 切片语义）。  
- **Upvalue**：按值拷贝（见 `language.md`），不形成外层帧引用环。

## 已修的稳定性问题（0.1.x hygiene）

1. **帧返回不清寄存器** → 大数组可能长时间只被“死帧”挂着才被扫到。  
   - `doReturn` 调用 `releaseFrame`：regs/upvals 置 null 并释放切片。  
2. **async 完成仍占 VM.futures 表** → `SpawnClosure` settle 后 `forget(id)`。  
3. **native 全局 `handles` 表永不删除** → `async.await` / `await_all` 后 `delHandle`；新增 `async.drop`。  
4. **文件句柄** → `VM.Release()` 关闭所有仍打开的 `files`（脚本未 close 时的兜底）。  
5. **寄存器无限膨胀** → `MaxRegisters = 65536`，越界报错而非 OOM。

## 脚本作者注意

```text
// 文件：用完 close，或依赖进程结束 / 引擎 Release
// async：await 后句柄自动 drop；若只 spawn 不 await，请 async.drop(fut)
// 大数组：离开作用域即可，无需 free；全局变量会一直活着
```

## 宿主（Go）嵌入注意

```go
v := vm.New(chunk, nil)
native.InstallGlobals(v)
res, err := v.Run(nil)
v.Release() // 关闭泄漏文件、清空帧；chunk 可共享复用
```

## 为何不做「自建 mark-sweep」

在 Go 里再实现一套堆 + 精确栈根扫描：

- 与 Go GC **双堆** 冲突、调试成本高  
- `Value` 已是 Go 对象，重复 GC 无收益  
- 真正痛点是 **句柄表 / 文件 / 帧引用**，不是缺 mark 位  

若未来要 AOT/无 Go 运行时，再考虑独立堆；当前 VM 路径以 **Go GC + 资源 hygiene** 为准。

## 自检

```powershell
cd D:\grokbuild\groklang\gltk
go test ./internal/vm/ -count=1
go test -vet=off ./internal/native/ -count=1
```

# GLVM — GrokLang 寄存器虚拟机

## 总览

GLVM 是 **寄存器式** 字节码虚拟机：

- 每个调用帧拥有 `regs[]Value`（宽度由 Proto.NumRegs 决定，运行时可扩展）
- 指令循环：取指 → 译码 A/B/C/Bx/sBx → 分发
- **唯一**用户代码执行路径；无 AST 解释器主路径

## Value 类型

| Type | 载荷 |
|------|------|
| Null | — |
| Bool | B |
| Int | I int64 |
| Float | F float64 |
| Str | S string |
| Bytes | Bytes []byte（可零拷贝子切片） |
| Array | *[]Value |
| Map | *map[string]Value |
| Func | *Closure{Proto, Upvals} |
| Native | NativeFunc + 名 |

## 帧与调用

```
Frame {
  proto, protoIx
  regs []Value
  ip int
  upvals []Value
  retReg int   // 返回值写入调用者的寄存器
}
```

- `CALL`：若目标为 `Native`，同步调用 Go 函数；若为 `Func`，压新帧
- `RET`/`RETN`：弹帧，写入调用者 `retReg`；无调用者则结束 `Run`

## 全局与模块

- `globals map[string]Value`
- `IMPORT` / `RegisterModule` 安装 native 模块 map
- 启动时按 Proto 名把用户函数挂到 globals（便于互相调用）
- 内建：`range` `len` `typeof` `str` `int`

## 错误

运行时错误携带近似栈回溯：

```
division by zero
stack traceback:
  main ip=12 line=8
```

## 性能相关

- `VM.Ops`：已执行指令计数（`gltk bench` 使用）
- `MaxOps`：可选看门狗
- `BSLICE` / `bin.slice` / `pe` 资源 `data` 尽量共享底层数组
- 算术热路径为寄存器-寄存器，无栈抖动

## 内存 / GC

**无自建 GC**：对象由 **Go runtime GC** 回收。详见 [`gc.md`](./gc.md)。

要点：

- `RET` 时 `releaseFrame` 清空 regs，避免死帧钉住大数组
- `VM.Release()` 关闭未关文件、清空帧与 async 登记
- 每帧寄存器上限 `MaxRegisters`（65536）
- async 句柄：`await` 后释放；可用 `async.drop`

## 与编译器衔接

```
源码 → parser.Parse → compiler.Compile → *bytecode.Chunk
                                            │
                                            ▼
                              vm.New(chunk) + native.InstallGlobals
                                            │
                                            ▼
                                       vm.Run(args)
```

确认：任何 `gltk run` 均经过 `Chunk` 与 `execute()` 分发循环。

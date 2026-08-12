# GrokLang 语言说明

## 文件

- 源码：`.glk`
- 编译产物：`.glkb`
- 用户库：任意 `.glk`（无需 `main`，导出全部顶层 `fn` 除 `main`）

## 词法

- 行注释 `// ...`，块注释 `/* ... */`
- 标识符：`[A-Za-z_][A-Za-z0-9_]*`
- 整数：十进制或 `0x` 十六进制
- 浮点：`1.23`、`1e-3`
- 字符串：`"..."` / `'...'`，支持 `\n \t \r \\ \" \' \$ \xHH`
- 多行字符串：`"""..."""`（可跨行，同样支持转义与 `${...}` 插值）
- 字符串插值：双引号 / 三引号内 `${expr}`，编译为 `+` 拼接
- 关键字：`fn let if else while for in return import as true false null break continue switch case default try catch throw`

## 语法

### 导入

```glk
// 裸名 → native 模块（运行时 OpIMPORT）
import pe, fs, out, str, bin, crypto, ahk, json, wxapkg

// 路径 → 用户 .glk 库（编译期合并进同一 Chunk）
import "libs/helpers.glk"           // 别名 = 文件名 stem：helpers
import "libs/helpers.glk" as h      // 显式别名
import "./util.glk" as util
```

- **Native**：绑定为全局 map（`pe.parse` 等）。
- **注意**：`import str` 会遮蔽内建全局函数 `str(x)`；需要模块时用 `""+x` 做字符串转换，或避免同时依赖两者。
- **路径库**：递归解析依赖；库内 `fn` 编译为带前缀的 proto（如 `lib#helpers#double`）；在入口 `main` 开头构建 map 并 `STOREG` 到别名。
- 库可再 `import` native 与其它 `.glk`；禁止循环依赖。
- 搜索路径（`module.DefaultSearchPaths`）：
  1. 入口文件所在目录  
  2. 当前目录下 `lib/`、`libs/`  
  3. `<gltk root>/stdlib`、`<gltk root>/libs`  
  4. 环境变量 `GLTK_LIB`（OS 路径列表分隔符）  
  5. 当前工作目录  

### 函数

```
fn name(a, b) {
  return a + b
}

// 匿名函数 / lambda（表达式）
let f = fn(x) { return x + 1 }
```

入口程序需要 `main`（签名习惯 `fn main(args)`，`args` 为字符串数组）。**库文件不需要 `main`**。

嵌套 `fn` 可捕获外层局部变量（**按值拷贝** upvalue：创建时读取，非引用）。

### 语句

- `let x = expr`
- `x = expr` / `a[i] = expr` / `obj.field = expr`
- `if cond { ... } else { ... }` / `else if`
- `while cond { ... }`
- `for i in expr { ... }` — 数组按元素；**map 按键**（等价 `for k in keys(m)`）
- `break` / `continue`（循环内）
- `switch expr { case a, b: ... default: ... }`（无 fall-through；编译为 `==` if 链）
- `try { ... } catch e { ... }` / `throw expr`
- `return` / `return expr`
- 表达式语句（如调用）

### 表达式

- 字面量：`null` `true` `false` 数字 字符串（含插值 / 多行）
- 数组：`[1, 2, 3]`
- 映射：`{a: 1, "b": 2}`
- 运算符：`+ - * / %`、`& | ^ ~ << >>`、`== != < <= > >=`、`&& || !`
- 三元：`cond ? a : b`（优先级低于 `||`）
- 调用：`f(x, y)`、`mod.func(x)`、`helpers.double(21)`
- 下标：`a[i]`
- 字段：`m.key`（等价 `m["key"]`）
- 匿名函数：`fn(params) { ... }`
- 括号分组

### 错误处理

```glk
try {
  throw "bad"
  // 或：native / 调用返回的 error（OpCALL 路径）
} catch e {
  // e 为错误字符串（首行）
}
```

- `OpTRY` / `OpENDTRY` / `OpTHROW`；CALL 错误若栈上有 try 则跳转 catch。
- 其它未捕获运行时错误仍会终止 VM（v1）。

### 内建全局函数（无需 import）

| 名称 | 说明 |
|------|------|
| `range(n)` / `range(a,b)` / `range(a,b,step)` | 整数数组 |
| `len(x)` | 长度 |
| `typeof(x)` | 类型名字符串 |
| `str(x)` / `int(x)` | 转换 |
| `input()` / `input(prompt)` | 读一行 stdin |
| `output(...)` | 同 println |
| `print(...)` / `println(...)` | 打印 |
| `open(path)` / `open(path, mode)` | 打开文件；返回 handle map |
| `close(handle)` | 关闭 |
| `read(handle)` / `write(handle, data)` | 便捷读写 |
| `exists(path)` | 路径是否存在 |
| `exit(code)` | 退出进程 |

**文件 handle map** 字段/方法：

- 字段：`path` `mode` `closed` `_fid`
- 方法：`read()` `read_bytes()` `write(s)` `write_bytes(b)` `readline()` `close()`
- `mode`：`r` `w` `a` `rb` `wb` `ab`（默认 `r`）

### Native 模块摘要

| 模块 | 函数 |
|------|------|
| `out` | `print` `println` |
| `fs` | `read_bytes` `write_bytes` `read_text` `write_text` `mkdir_all` `exists` `open` `remove` `list_dir` `walk` |
| `bin` | `u8` `u16le` `u32le` `u64le` `slice` `rol32` `ror32` `find_bytes` `concat` `from_u32le` |
| `str` | `scan` `contains` `lower` `upper` `from_utf8` `from_utf16le` `len` `split` `replace` `trim` `slice` / `substr` |
| `crypto` | `md5_hex` `sha256_hex` `hmac_sha256_b64` |
| `pe` | `parse(bytes) → {machine,entry,sections,resources}` |
| `ahk` | `extract_u_dwords` `decrypt_scriptguard` `decode_utf16le` |
| `json` | `stringify` `parse` |
| `wxapkg` | `is_encrypted` `decrypt` `list_files` `unpack` `scan` |
| `async` | `spawn` `channel` `mutex` `parallel` 等（见 ASYNC.md） |
| `http` / `net` / `ws` | 网络（见 NETWORK.md） |

### wxapkg 模块

```glk
import wxapkg
// is_encrypted(bytes|path) -> bool
// decrypt(data_bytes, wxid) -> bytes
// list_files(data_bytes) -> [{name, offset, size}, ...]
// unpack(path, out_dir, {wxid, decrypt, beautify_json}) -> {ok, count, error, save_path}
// scan(dir, recursive) -> [paths]
```

算法来自 `wxapkg/wechat/` 核心（AES-CBC+pbkdf2 / 0xBE…0xED 索引），**不依赖 Wails**。  
GUI 工程仍在 `wxapkg/` 目录，与 CLI/VM 分离。

## 语义备注

- 真值：`null` / `false` / `0` / `""` / 空集合为假
- `+` 在任一操作数为字符串时做拼接
- 数组/映射为引用语义（寄存器持有指针式 Value）
- 嵌套函数捕获外层局部为 **upvalue 按值拷贝**（`OpCLOSURE` + `OpGETUPV`）；同文件顶层函数引用经 `MAKEFN`
- `for-in` 对 map：运行时若 `typeof == "map"` 则先 `OpKEYS` 再按数组迭代键
- 执行路径唯一：源码 → 编译 Chunk → GLVM（无 AST 解释）
- REPL：`gltk repl`

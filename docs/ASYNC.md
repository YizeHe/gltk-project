# GrokLang 异步 / 同步系统

模块：`import async`

实现：每个 `async.spawn` 在 **独立 VM 克隆** 上启动 **Go goroutine**，共享只读 chunk/native modules，globals 浅拷贝。适合逆向批处理（并行扫文件、并行 HTTP）。

## API

| 函数 | 说明 |
|------|------|
| `async.spawn(fn, args_array?)` | 异步执行函数，返回 future 句柄 |
| `async.await(future)` | 等待结果 |
| `async.await_all([f1,f2,...])` | 等待多个，返回结果数组 |
| `async.ready(future)` | 是否已完成 |
| `async.sleep(ms)` | 睡眠 |
| `async.parallel(fn, items)` | 对数组每项 spawn `fn(item)` 并 await_all |
| `async.channel(cap)` | 有缓冲 channel |
| `async.send(ch, v, timeout_ms?=-1)` | 发送（-1 阻塞） |
| `async.recv(ch, timeout_ms?=-1)` | 接收 → `{ok,value,closed}` |
| `async.close(ch)` | 关闭 |
| `async.mutex()` / `lock` / `unlock` | 互斥锁 |
| `async.waitgroup()` / `wg_add` / `wg_done` / `wg_wait` | WaitGroup |

## 示例

见 `samples/async_demo.glk`。

## 与 GUI 集成

CUI（Win32 GUI）完成后：主线程跑消息循环，`async.spawn` 跑后台逆向任务，通过 `async.channel` 把结果投递回 UI 线程（由 `gui` native 提供 `gui.post(fn)` 一类接口）。

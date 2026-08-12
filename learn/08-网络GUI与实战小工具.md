# 第 8 章 · 网络、GUI 与实战小工具

> **本章目标**：用 GrokLang 做 **HTTP/DNS/TCP/WebSocket** 客户端与小型服务端自测，并在 Windows 上用 **`import gui`** 搭可交互工具；最后把网络与第 7 章逆向能力拼成「下载 → triage → IOC」小工具。  
> **前置**：第 1–7 章。  
> **示例**：`learn/examples/ch08_01` … `ch08_10`（网络相关已联网验证；GUI smoke 已通过；`ch08_08` 为交互程序）。

---

## 8.0 架构心智模型

```text
GrokLang 脚本
    │
    ├─ import http      ──► 高层 HTTP（TLS、下载、表单）
    ├─ import net       ──► DNS / TCP·UDP dial·listen
    ├─ import ws        ──► WebSocket
    ├─ stdlib/httpx.glk ──► 会话 / JSON 封装
    ├─ stdlib/netutil.glk ► TCP 小工具
    └─ import gui       ──► Win32（CUI）窗口与控件
```

原则（与 `docs/NETWORK.md` 一致）：

- **传输与加密在 Go native**；  
- **业务拼装在 `.glk`**（含 stdlib）。

---

## 8.1 HTTP 客户端

### 8.1.1 基本 GET

```glk
import http, json

let r = http.get(url, {
  timeout: 25,
  user_agent: "my-tool/1.0",
  headers: {"Accept": "application/json"},
  insecure: true
})
// r.ok / r.status / r.body / r.error / r.url / r.headers
```

| opts 字段 | 含义 |
|-----------|------|
| `timeout` | 秒 |
| `headers` | 请求头 map |
| `user_agent` / `ua` | UA |
| `insecure` | 跳过 TLS 校验（RE 抓包环境常用） |
| `follow_redirects` | 是否跟随跳转 |
| `proxy` | 代理 URL |
| `max_body` | 最大 body 字节 |

### 8.1.2 方法全家桶

`get` / `post` / `put` / `patch` / `delete` / `head` / `request` / `download` / `encode_form`

### 8.1.3 示例

```powershell
.\gltk.exe run learn\examples\ch08_01_http_get.glk
.\gltk.exe run learn\examples\ch08_02_http_post_form.glk
```

`ch08_02`：JSON POST、表单编码、小文件 download、HEAD。

### 8.1.4 经验

1. **永远检查 `r.ok` 与 `status`**，网络错误与 HTTP 4xx/5xx 要分开处理。  
2. 大文件用 **`http.download`**，不要 `get` 整包进内存。  
3. 内网自签证书：`insecure: true`（注意安全边界）。  
4. 走本地代理：`proxy: "socks5://127.0.0.1:10808"`（若实现支持 URL 形态）。  
5. 失败时打印 `r.error` 与 `r.url`（最终 URL）。

---

## 8.2 `httpx` 会话层

```glk
import "stdlib/httpx.glk" as httpx

let s = httpx.session_new("https://httpbin.org", {"Accept": "application/json"})
s.timeout = 25
let g = httpx.get_json(s, "/get")
let p = httpx.post_json(s, "/post", {a: 1})
```

适合：固定 base URL、统一头、重试策略（见 `stdlib/httpx.glk` 源码）。

```powershell
.\gltk.exe run learn\examples\ch08_03_httpx_session.glk
```

---

## 8.3 DNS 与原始 TCP

### 8.3.1 DNS

```glk
let dns = net.resolve("example.com")
// dns.ok, dns.addrs[]
```

### 8.3.2 dial / read / write

```glk
let c = net.dial("tcp", "example.com:80", 12)
c.write("HEAD / HTTP/1.0\r\nHost: example.com\r\n\r\n")
let r = c.read(512)
c.close()
```

### 8.3.3 netutil

```glk
import "stdlib/netutil.glk" as netu
let raw = netu.http_get_raw("example.com", 80, "/")
```

```powershell
.\gltk.exe run learn\examples\ch08_04_dns_tcp.glk
```

### 8.3.4 经验

- 协议调试时 **TCP + 手工拼 HTTP** 比直接 `http.get` 更透明。  
- `read` 返回结构含 `ok/n/eof/data`（实现细节以运行时为准）。  
- 超时用 `set_deadline` 或 dial 的 timeout 参数。

---

## 8.4 TCP 服务端骨架

`listen` → `accept` → `read/write` → `close`。

`ch08_05` 在本机 `127.0.0.1:19080` 做 **echo 自测**（`async.spawn` 客户端 + 主线程 accept）。

```powershell
.\gltk.exe run learn\examples\ch08_05_tcp_echo_server.glk
```

**经验**：

- 端口占用就改端口；  
- accept 阻塞时用 **async 另起客户端** 或双终端；  
- spawn 的函数必须是**顶层 fn**。

---

## 8.5 WebSocket

```glk
import ws
let c = ws.connect(url, {timeout: 15})
ws.send(c, "hello", false)
let m = ws.recv(c, 10)
ws.close(c)
```

```powershell
.\gltk.exe run learn\examples\ch08_06_ws_smoke.glk
```

公共 echo 服务不稳定时脚本会 **软失败返回 0**——这是刻意设计，避免 CI 挂死。

**经验**：生产 WS 要自建服务（如第 7 章以外的 CF Worker）；超时与重连自己写。

---

## 8.6 GUI 基础（Windows + CUI）

### 8.6.1 API 地图

```text
gui.available / version / init
gui.window / show / hide / run / quit / set_title / on_close
gui.label / button / lineedit / textedit / checkbox
gui.set_text / get_text / append_text / set_bounds
gui.on_click / msgbox / open_file
gui.vbox / hbox / add
```

### 8.6.2 生命周期（极重要）

```text
gui.init()
创建 → window + 创建控件
      → on_click 绑定
      → show
      → run()     // 阻塞消息循环
      → quit()
```

**禁止**在 `show` 之前做长时间 `http.get`（界面会像「没出来」）。  
**Start 回调里**若同步 HTTP，界面会卡住——教学示例 `ch08_08` 会提示；生产应 `async.spawn`。

### 8.6.3 句柄

控件返回 `{type, id}` map。使用前：

```glk
if typeof(h) == "map" { gui.set_text(h, "...") }
```

### 8.6.4 坑回顾（实战踩过）

| 坑 | 对策 |
|----|------|
| map 字面量全局变量变 null | 配置写死字符串或先 `{}` 再赋值 |
| Start 无响应 | 回调里阻塞网络；先反馈 UI |
| 中文不显示 | CUI DrawText UTF-16 长度（已修） |
| `str(x)` | 用 `to_str` / `""+` |
| 嵌套 `fn name` | 用 lambda |

### 8.6.5 Smoke（不阻塞）

```powershell
.\gltk.exe run learn\examples\ch08_07_gui_smoke.glk
```

### 8.6.6 交互小工具

```powershell
.\gltk.exe run learn\examples\ch08_08_gui_tool.glk
```

URL 输入框 + **Fetch / Clear / Quit** + 日志区。  
点 Fetch 会同步请求 httpbin（演示用）。

更完整 RE 工作台：`samples/gui_re_workbench.glk`。  
代理 GUI：`samples/clash_gui.glk` / `GLTK-Free-Proxy.exe`。

---

## 8.7 实战拼装

### 8.7.1 下载 + 逆向 triage

```powershell
.\gltk.exe run learn\examples\ch08_09_net_re_fetch.glk
```

`http.download` → `fs.read_bytes` → `md5` / `mz` 判断 → 写旁路 txt 报告。

### 8.7.2 「情报风格」：POST + regex IOC

```powershell
.\gltk.exe run learn\examples\ch08_10_threat_intel_style.glk
```

网络拿回显，本地抽邮箱/URL/IP，输出 JSON 计数。  
（真实威胁情报源需合规授权。）

---

## 8.8 设计模式

### 模式 A — CLI 网络工具

```text
parse args → http/net → json 报告 → exit code
```

### 模式 B — GUI 壳 + 网络

```text
show UI → 用户点按钮 →（理想）async.spawn(request)
         → await 后 set_text / msgbox
```

### 模式 C — 网络 + 第 7 章 RE

```text
download sample → triage → pe/strings → report.json
```

### 模式 D — 本地代理客户端

见 Clash-VLESS 专章实践：`samples/clash_gui.glk`（SOCKS 本地 + 远端节点）。

---

## 8.9 安全与合规

1. 只请求你有权访问的 URL。  
2. 扫描器/爆破/未授权渗透 **不要** 用教程示例去攻击他人。  
3. `insecure: true` 仅限实验。  
4. 日志勿打印 Cookie/Token。  
5. GUI 打开本地样本时，**只读解析**（第 7 章原则）。

---

## 8.10 示例总表

| 文件 | 内容 | 阻塞？ |
|------|------|--------|
| `ch08_01_http_get.glk` | GET + JSON | 否 |
| `ch08_02_http_post_form.glk` | POST/form/download/HEAD | 否 |
| `ch08_03_httpx_session.glk` | httpx 会话 | 否 |
| `ch08_04_dns_tcp.glk` | DNS + TCP + netutil | 否 |
| `ch08_05_tcp_echo_server.glk` | 本机 echo 自测 | 否 |
| `ch08_06_ws_smoke.glk` | WebSocket（可软失败） | 否 |
| `ch08_07_gui_smoke.glk` | GUI 创建/show | 否 |
| `ch08_08_gui_tool.glk` | HTTP 小工具 GUI | **交互** |
| `ch08_09_net_re_fetch.glk` | 下载+哈希/MZ | 否 |
| `ch08_10_threat_intel_style.glk` | 网络+IOC | 否 |

```powershell
cd D:\grokbuild\groklang\gltk

.\gltk.exe run learn\examples\ch08_01_http_get.glk
.\gltk.exe run learn\examples\ch08_02_http_post_form.glk
.\gltk.exe run learn\examples\ch08_03_httpx_session.glk
.\gltk.exe run learn\examples\ch08_04_dns_tcp.glk
.\gltk.exe run learn\examples\ch08_05_tcp_echo_server.glk
.\gltk.exe run learn\examples\ch08_06_ws_smoke.glk
.\gltk.exe run learn\examples\ch08_07_gui_smoke.glk
.\gltk.exe run learn\examples\ch08_09_net_re_fetch.glk
.\gltk.exe run learn\examples\ch08_10_threat_intel_style.glk

# 交互
.\gltk.exe run learn\examples\ch08_08_gui_tool.glk
```

---

## 8.11 练习

1. 改 `ch08_01`：对非 200 打印 body 前 200 字符（用 `str.slice`）。  
2. 写脚本：对 `args` 多个 URL 做 HEAD，输出 status 表。  
3. 用 `async.parallel` 并行 GET 三个 URL，汇总状态码。  
4. 扩展 GUI 工具：增加 **Save** 把日志写入 `_tmp_ch08/log.txt`。  
5. 组合第 7 章：下载 → `str.scan` 找 `http` 标记 → JSON 报告。  
6. TCP：写一个只回 `PONG\n` 的服务，客户端发 `PING`。  
7. （加分）阅读 `samples/gui_re_workbench.glk`，画出控件与回调关系图。

---

## 8.12 速查

```glk
import http, net, ws, gui, json, out
import "stdlib/httpx.glk" as httpx

http.get(url, opts) / post / download / encode_form
net.resolve / dial / listen / accept
ws.connect / send / recv / close

gui.init(); let w = gui.window(title,w,h)
gui.button/label/lineedit/textedit
gui.on_click(btn, fn); gui.show(w); gui.run()
```

---

## 8.13 小结

1. **http** 覆盖日常 REST/下载；**net** 覆盖协议级调试；**ws** 覆盖长连接。  
2. **httpx/netutil** 是 glk 层最佳实践封装。  
3. **GUI** 要守消息循环纪律：先 show，后 run；重活异步。  
4. 网络 + RE 可做合法的自动化采集与 triage 工具。  
5. 与 Clash/打包章节结合，可交付完整桌面小工具。

---

## 8.14 下一章预告

→ [第 9 章 · 编译、打包与混淆发布](./09-编译打包与混淆发布.md)

---

*第 8 章完。请先跑通 `ch08_01`～`ch08_07` 与 `ch08_09`～`ch08_10`，再打开 `ch08_08` 点 Fetch 体验 GUI。*

# GLTK 通用网络库

原则：**底层 I/O 用 Go native；上层协议与封装用 GrokLang（stdlib）**。

## 架构

```
stdlib/httpx.glk   session / retry / JSON / form
stdlib/netutil.glk TCP helpers / raw HTTP
        │
   import http | net | ws
        │
internal/native/http.go  HTTP client (连接复用、代理、TLS、下载)
internal/native/net.go   TCP/UDP dial·listen·resolve
internal/native/ws.go    WebSocket (gorilla)
```

## Native API

### `http`

| 函数 | 说明 |
|------|------|
| `get(url, opts?)` | GET |
| `post(url, body?, opts?)` | POST |
| `put` / `patch` / `delete` / `head` | 常用方法 |
| `request(method, url, body?, opts?)` | 任意方法 |
| `download(url, path, opts?)` | 流式写入文件 |
| `encode_form(map)` | `a=1&b=2` 编码 |

**opts map**（或旧式 headers map + timeout int 仍兼容）：

- `timeout` 秒  
- `headers` / 顶层当 headers  
- `user_agent` / `ua`  
- `insecure` TLS 跳过校验（默认 `true`，RE 友好）  
- `follow_redirects`（默认 `true`）  
- `proxy` URL  
- `max_body` 字节  

返回：`{ok, status, body, bytes, headers, url, error}`  

### `net`

| 函数 | 说明 |
|------|------|
| `dial(network, address, timeout?)` | `tcp`/`udp`/… → conn handle |
| `resolve(host)` | DNS → `{ok, addrs}` |
| `listen` / `accept` | 服务端 |
| `read` / `write` / `close` / `set_deadline` | 也可 `conn.read()` 方法 |

### `ws`

| 函数 | 说明 |
|------|------|
| `connect(url, opts?)` | 返回 handle |
| `send(conn, data, binary?)` | 或 `conn.send` |
| `recv(conn, timeout?)` | `{type, text, data}` |
| `close(conn)` | |

## GLTK 上层（stdlib）

```glk
import "stdlib/httpx.glk" as httpx

let s = httpx.session_new("https://api.example.com", {"Authorization": "Bearer x"})
let r = httpx.get_json(s, "/v1/me")
let p = httpx.post_json(s, "/v1/echo", {a: 1})
```

```glk
import "stdlib/netutil.glk" as netu
let c = netu.dial_tcp("example.com:80", 15)
```

## 示例

```powershell
.\gltk.exe run samples\net_demo.glk
```

## 范围说明

已覆盖：**HTTP 客户端全家桶、下载、表单、DNS、TCP 客户端/服务端骨架、WebSocket**。  
未做（可后续加）：HTTP/3、完整 cookie jar 持久化、HTTP/2 细调、mTLS 客户端证书文件加载 UI 等——需要时再加 native 钩子，上层仍可用 glk 扩展。

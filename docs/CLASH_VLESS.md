# GLTK Clash-VLESS Lite

极简代理内核：**本机 SOCKS5 → 单一 VLESS 出站**。不是完整 Clash 兼容实现。

## 能力

| 项目 | 状态 |
|------|------|
| SOCKS5 入站（无认证，CONNECT） | ✅ |
| VLESS over TCP | ✅ |
| TLS + SNI + skip-cert-verify | ✅ |
| 分流 / 规则 / DNS | ❌ |
| UDP / TProxy | ❌ |
| WS / gRPC / HTTP2 / REALITY | ❌ |
| flow (xtls-rprx-vision 等) | ❌ |
| 多节点选择 | 仅用 `proxies[0]` |

## API（`import clash`）

```text
clash.version()           -> string
clash.start(config)       -> {ok, port, listen, proxy, server, tls, error}
clash.stop()              -> {ok, error}
clash.status()            -> {running, port, conns, accepts, errors, bytes_up, bytes_down, ...}
clash.test_parse(uuid)    -> {ok, uuid_hex, header_len, header_hex}
```

`config` 为 **map** 或 **JSON 字符串**。

## 配置示例

见 `samples/clash_vless.json`：

```json
{
  "socks-port": 10808,
  "proxies": [
    {
      "name": "vless-demo",
      "type": "vless",
      "server": "1.2.3.4",
      "port": 443,
      "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "tls": true,
      "servername": "www.example.com",
      "network": "tcp",
      "skip-cert-verify": true
    }
  ]
}
```

也支持扁平字段（无 `proxies` 数组）：`server` / `port` / `uuid` / `tls` 写在根上。

## 运行

```powershell
# 改好 json 后
.\gltk.exe run samples\clash_vless.glk
.\gltk.exe run samples\clash_vless.glk -- samples\clash_vless.json
.\gltk.exe run samples\clash_vless.glk -- --gui

# 自检 UUID → VLESS 头
.\gltk.exe run samples\clash_vless.glk -- --test-parse 12345678-1234-1234-1234-123456789abc
```

代理测通：

```powershell
curl.exe -x socks5h://127.0.0.1:10808 https://httpbin.org/ip
```

## 架构

```
浏览器/curl
    │ SOCKS5
    ▼
clash native (Go)  ──listen 127.0.0.1:socks-port
    │ VLESS header + payload relay
    ▼
远端 VLESS (TCP / TLS)
```

- 转发与协议在 **Go native**（`internal/native/clash_vless.go`）
- 配置 / CLI / GUI 在 **GLTK**（`samples/clash_vless.glk`）

## Cloudflare Worker 服务端

自建极简节点见：`cf-vless-worker/`（VLESS over WebSocket）。

```powershell
cd cf-vless-worker
.\deploy.ps1
# 再改 work/clash_cf_worker.json 里的 workers.dev 域名
.\gltk.exe run samples\clash_vless.glk -- work\clash_cf_worker.json
```

## 说明

1. 进程退出即代理停止（状态不跨进程）。
2. 仅绑定 `127.0.0.1`，不对外暴露。
3. 需要真实 VLESS 服务端才能上网；`--test-parse` 只验证本地打包。
4. 完整 Clash 功能需自行扩展规则与多协议。

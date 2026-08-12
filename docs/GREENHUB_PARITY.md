# GreenHub 逆向能力对齐说明

目标：GrokLangToolKit（GLVM）具备与 `../汇报.md` 中 **GreenHub 2.2.0 研究成果** 同级的**可复现能力**。

## 能力对照

| 原 GreenHub 成果步骤 | gltk 模块 / 脚本 | 状态 |
|---------------------|------------------|------|
| NSIS 安装包解包 | `archive.extract` → 调用 `tools/7z/7za` | ✅ |
| `app.asar` 解包 | `asar.extract` / `asar.list` / `asar.read` | ✅ |
| 扫描 main.js 密钥/CDN | `js.scan_file` | ✅ 检出 `5f5749e77a9b`、CloudFront、`/wes` |
| 拉 config / server_list | `http.get` | ✅ 61 节点扁平化 |
| HMAC 签名算法 | `crypto.sign_query` | ✅ sort + `k=v` + HMAC-SHA256-Base64 |
| 动态 POST 签账号 | `http.post` + `samples/greenhub_re.glk --mint N` | ⚠️ **客户端链路完整**；当前节点返回 **HTTP 404**（服务端已下线该路径，status 仍 200） |
| 结构化产出 | `work/greenhub_gltk/` | ✅ report + cdn json + nodes_flat.tsv |

## 一键流水线

```powershell
cd D:\grokbuild\groklang\gltk
go build -o gltk.exe .\cmd\gltk

# 使用已有 ../extract（最快）
.\gltk.exe run samples\greenhub_re.glk -- --skip-extract

# 需要签号尝试（节点 account 目前 404，属服务端变化）
.\gltk.exe run samples\greenhub_re.glk -- --skip-extract --mint 5

# 从安装包重解（需 7za，较慢）
.\gltk.exe run samples\greenhub_re.glk -- "..\GreenHub Setup 2.2.0.exe"
```

产物目录：`work/greenhub_gltk/`

- `GREENHUB_GLTK_REPORT.md`
- `cdn/config_v2.json`
- `cdn/server_list_v7.json`
- `cdn/nodes_flat.tsv`（61 行域名）
- `mint_log.txt`（若 `--mint`）

## 与历史 53/61 的差异

历史报告中 account 签发成功，是 **2026-07 当时节点 API 仍可用**。  
现测：`GET /d/v1/status` 正常，`POST /d/v1/account` 返回 Flask/Werkzeug 风格 **404 HTML** —— 与签名算法无关（多种 message 格式均 404）。

工具侧已具备同等逆向与请求能力；**活号比例取决于服务端是否恢复 account 接口**。

## 新增 native 一览

| 模块 | API |
|------|-----|
| `archive` | `extract`, `list`, `find_7za`, `nsis_extract` |
| `asar` | `info`, `list`, `read`, `extract` |
| `http` | `get`, `post`, `request`（TLS skip verify，UA GreenHub/2.2.0） |
| `js` | `scan_file`, `find_urls`, `find_hex_keys`, `find_all` |
| `json` | `flatten_servers`（server_list_v7 → 61 节点） |
| `crypto` | `sign_query`, `uuid4`, `now_ms`, `hmac_sha256_*` |
| `str` | `url_encode`, `starts_with`, `ends_with` |

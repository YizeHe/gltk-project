# GLTK: .NET / IL + AHPK2 / AES unpack

Last-mile reverse for samples like **GTAOL Ultra Macro Pro** is first-class in GLTK — no Python, no external ILSpy required for the common path.

## Capability map

| Layer | Module | Role |
|-------|--------|------|
| PE triage | `pe.summary` | image_end, overlay_size, markers |
| .NET host | `dotnet.*` | CLR detect, #US strings, types/methods, **IL disasm** |
| Crypto primitives | `crypto.*` | AES-CBC/ECB, HMAC-SHA256, PBKDF2, base64/hex |
| AHPK2 package | `ahpk2.*` | Trailer parse, HMAC verify, AES decrypt, ZIP extract |

## AHPK2 format

EOF trailer (13 bytes):

```
[ payload | "AHPK2" | int64_le size ]
payload_offset = file_size - 13 - size
```

Pro profile (hardcoded in host; `Main` does not call password `Key()`):

| Material | Base64 |
|----------|--------|
| AES + HMAC key (32) | `QD3D2gPl8Ywm/SwZDMhV8nPzGwFFmRk4WWQdJjKZd44=` |
| AES IV (16) | `IXAwpGDqS//tpxoMHz+gNw==` |
| Expected MAC (32) | `yFsDju+vOJDfLam4sujuAMbGuMbMCs0mizGGzGFVBX4=` |

```
mac = HMAC-SHA256(key, payload)
plain = AES-256-CBC-PKCS7(key, iv, payload)  →  ZIP
```

Override with opts: `key_b64`, `iv_b64`, `mac_b64`, or `profile: "pro"`.

## .NET / IL

- Streams: `#~`, `#Strings`, `#US`, `#Blob`, `#GUID`
- Tables: TypeDef, MethodDef, TypeRef, MemberRef, …
- `dotnet.il(path, type, method)` → ildasm-style text with `ldstr` resolved from #US
- Large PE+overlay: `ParseFile` reads only PE image end (sections), not full overlay

Limitations: no full C# decompiler; EH clauses not pretty-printed; generics as TypeSpec tokens.

## One-shot sample

```powershell
cd D:\grokbuild\groklang\gltk
.\gltk.exe run samples\pro_unpack.glk -- "..\test\GTAOL Ultra Macro Pro.exe" work\pro_gltk
```

Produces `work/pro_gltk/payload.zip`, `work/pro_gltk/unzipped/`, `work/pro_gltk/REPORT_GLTK.md`.

## Manual building blocks

```glk
import crypto, ahpk2, dotnet

// Discover keys from host
let us = dotnet.strings(host_exe)
let il = dotnet.il(host_exe, "P", "Main")

// Or decrypt with explicit materials
let key = crypto.b64_decode("...")
let iv  = crypto.b64_decode("...")
// general AES body (not AHPK2-framed):
// crypto.aes_cbc_decrypt(body, key, iv)

// Package-aware:
ahpk2.unpack(host_exe, out_dir, {profile: "pro"})
```

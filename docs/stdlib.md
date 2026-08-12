# GrokLang Standard Library

The `stdlib/` directory contains reusable `.glk` libraries that ship with GLTK.
They are resolved automatically when you import them by path:

```glk
import "stdlib/re_helpers.glk" as rh
```

Search paths (in order): entry file dir, `./lib`, `./libs`, `<gltk root>/stdlib`, `GLTK_LIB` env, cwd.

---

## Native Modules (import bare name)

These are Go-backed modules registered on the VM at startup.

| Module | Key Functions |
|--------|---------------|
| `re` | `compile`, `match`, `find`, `find_all`, `replace`, `split`, `groups` |
| `str` | `scan`, `contains`, `lower`, `upper`, `split`, `replace`, `trim`, `index_of`, `starts_with`, `ends_with`, `url_encode`, `from_utf8`, `from_utf16le` |
| `fs` | `read_bytes`, `write_bytes`, `read_text`, `write_text`, `mkdir_all`, `exists`, `open`, `remove`, `list_dir`, `walk`, `file_size`, `read_range`, `head`, `tail` |
| `bin` | read: `u8`/`u16le`/`u32le`/`u64le`/`slice`/`find_bytes`/… · patch: `write_at`/`fill`/`nop_fill`/`swap16`/`swap32`/`crc32`/`checksum_sum8` |
| `crypto` | `md5_hex`, `sha256_hex`, `hmac_sha256*`, `b64_encode/decode`, `hex_encode/decode`, `aes_cbc_decrypt/encrypt`, `aes_ecb_decrypt`, `pbkdf2_sha1/sha256`, `sign_query`, `uuid4`, `now_ms` |
| `out` | `print`, `println` |
| `pe` | `parse`, `parse_file`, `summary`, `overlay`, `imports`, `exports` |
| `upx` | `detect`/`is_upx`, `info`, `unpack(path\|bytes, out?)` — pure Go UPX PE unpack (LZMA/NRV) |
| `elf` | `is_elf`, `parse`, `summary` — ELF32/64 sections/segments/symbols |
| `disasm` | `disasm`, `disasm_one` — x86 / x64 / arm64 (pure Go `x/arch`) |
| `strings_re` | `extract`, `extract_utf16`, `entropy`, `entropy_all`, `entropy_map`, `find_high_entropy` |
| `dotnet` | `is_clr`, `info`, `strings`, `types`, `methods`, `il`, `dump`, `summary` — CLI metadata + IL disasm |
| `ahpk2` | `detect`, `info`, `profiles`, `decrypt`, `unpack` — AHPK2 AES-ZIP trailer (Pro profile) |
| `ahk` | `extract_u_dwords`, `decrypt_scriptguard` (SG1), **`decrypt_scriptguard2` / `decrypt_rcdata` / `is_scriptguard2`** (SG2 LCG+xorshift) |
| `json` | `stringify`, `parse`, `flatten_servers` |
| `http` | `get`/`post`/`put`/`patch`/`delete`/`head`/`request`/`download`/`encode_form` — opts: timeout/headers/proxy/ua/insecure |
| `net` | `dial`/`resolve`/`listen`/`accept`/`read`/`write`/`close` — TCP/UDP 原语 |
| `ws` | `connect`/`send`/`recv`/`close` — WebSocket |
| `js` | `scan_file`, `contains` |
| `archive` | `extract` |
| `asar` | `extract` |
| `wxapkg` | `is_encrypted`, `decrypt`, `list_files`, `unpack`, `scan` |
| `async` | `sleep`, `parallel`, `spawn`, `ready`, `await`, `channel`, `send`, `recv`, `close`, `mutex`, `lock`, `unlock` |

### crypto (AES / pack RE)

```glk
import crypto

let key = crypto.b64_decode("QD3D2gPl8Ywm/SwZDMhV8nPzGwFFmRk4WWQdJjKZd44=")
let iv  = crypto.b64_decode("IXAwpGDqS//tpxoMHz+gNw==")
let plain = crypto.aes_cbc_decrypt(cipher, key, iv)          // PKCS7 default
let mac   = crypto.hmac_sha256(key, payload)                 // raw 32 bytes
let dk    = crypto.pbkdf2_sha1(password, salt, 120000, 32)   // .NET Rfc2898DeriveBytes
```

### dotnet (.NET / IL)

```glk
import dotnet

dotnet.is_clr(path)
let s = dotnet.summary(path)           // is_clr, ilonly, interesting_strings
let us = dotnet.strings(path)          // #US heap
let types = dotnet.types(path)
let methods = dotnet.methods(path, "P")
let il = dotnet.il(path, "P", "Main")  // {ok, il, rva}
```

### ahpk2 (GTAOL Pro / general AES trailer)

```glk
import ahpk2

ahpk2.detect(path)                     // magic + payload_size from EOF trailer
ahpk2.unpack(path, out_dir)            // default profile "pro"
ahpk2.unpack(path, out_dir, {profile: "pro", key_b64: "...", iv_b64: "..."})
let d = ahpk2.decrypt(path)            // {ok, plain: bytes, mac_ok, is_zip}
```

End-to-end Pro sample: `samples/pro_unpack.glk`.

### ahk ScriptGuard2 (YourCrushLY / GTAMacro)

```glk
import ahk

ahk.is_scriptguard2(rcdata_path)              // LCG / x64 shellcode markers
let r = ahk.decrypt_rcdata(rcdata_path, "out.ahk")
// r.ok, r.algo="sg2", r.file_iv, r.script, r.plain
// SG1 classic still: ahk.decrypt_scriptguard(ahk_exe_bytes, dwords)
```

End-to-end: `samples/gtamacro_unpack.glk` (Enigma host → PE RCDATA → SG2 → plain AHK, **no Python**).

### Network (native + stdlib)

```glk
import http, net, ws
import "stdlib/httpx.glk" as httpx

http.get(url, {timeout: 20, headers: {"X": "1"}, proxy: "", insecure: true})
http.download(url, "out.bin")
net.resolve("example.com")
let c = net.dial("tcp", "example.com:80", 10)
let s = httpx.session_new("https://api.example.com", {})
httpx.get_json(s, "/v1/x")
```

Docs: `docs/NETWORK.md` · demo: `samples/net_demo.glk`

### disasm / elf / strings_re

```glk
import disasm, elf, strings_re, crypto

let ins = disasm.disasm(crypto.hex_decode("90c34889c0"), "x64", 0x1000, 20)
let e = elf.parse(path)
let hi = strings_re.find_high_entropy(fs.read_bytes(path))
```

### re Module Details

```glk
import re

// Compile and cache a pattern
let r = re.compile("[a-z]+")

// Match: returns bool
re.match("[a-z]+", "hello")        // true

// Find: returns {ok, match, start, end}
let f = re.find("([a-z]+)@([a-z.]+)", "a@b.com")
// f.ok = true, f.match = "a@b.com", f.start = 0

// Find all: returns string array
re.find_all("\d+", "a1 b22 c333")  // ["1", "22", "333"]

// Groups: returns array [full_match, group1, group2, ...]
re.groups("([a-z]+)@([a-z.]+)", "a@b.com")
// ["a@b.com", "a", "b.com"]

// Replace: re.replace(pattern, text, replacement, n?)
re.replace("\d+", "a1b2c3", "X")     // "aXbXcX"
re.replace("\d+", "a1b2c3", "X", 1)  // "aXb2c3"

// Split: re.split(pattern, text)
re.split("\s+", "hello  world\tfoo")  // ["hello", "world", "foo"]
```

---

## Stdlib Libraries (import by path)

### re_helpers — `stdlib/re_helpers.glk`

Regex-based extraction helpers for reverse engineering.

```glk
import "stdlib/re_helpers.glk" as rh

let text = "contact: alice@test.com, https://example.com, 192.168.1.1"

rh.extract_emails(text)      // ["alice@test.com"]
rh.extract_urls(text)        // ["https://example.com"]
rh.extract_ipv4(text)        // ["192.168.1.1"]
rh.extract_domains(text)     // ["test.com", "example.com"]
rh.extract_uuids(text)       // UUIDs from text
rh.extract_hex_strings(text) // hex runs >= 8 chars
rh.extract_pe_rva(text)      // 0x00401000 patterns
rh.extract_import_dlls(text) // "*.dll" names
rh.extract_paths(text)       // "C:\..." paths
rh.extract_cloudfront(text)  // CloudFront URLs
rh.extract_pe_markers(text)  // Nullsoft/Inno/Electron/etc

// Utilities
rh.first_match("\d+", "abc123")  // "123"
rh.has_pattern("\d+", "abc")     // false
rh.count_pattern("\d+", "a1b2")  // 2
rh.replace_all("\d+", "a1b2", "X") // "aXbX"
rh.groups("(\d+)-(\d+)", "1-2") // ["1-2", "1", "2"]
```

### hexutil — `stdlib/hexutil.glk`

Hex conversion and binary dump utilities.

```glk
import "stdlib/hexutil.glk" as hexutil

hexutil.to_hex(255)              // "ff"
hexutil.to_hex_padded(10, 8)    // "0000000a"
hexutil.hex8(0xDEADBEEF)        // "deadbeef"
hexutil.hex16(12345)             // "0000000000003039"

// Hex dump (like hexdump -C)
let data = fs.read_bytes("file.bin")
hexutil.hex_dump(data)           // multi-line offset+hex+ASCII
hexutil.hex_dump_short(data, 32) // compact single-line hex

// Conversion
hexutil.is_hex_string("DEADBEEF") // true
hexutil.hex_to_int("FF")          // 255
hexutil.bytes_to_hex(data)        // continuous hex string

// PE DOS header dump
hexutil.pe_dos_header_dump(data)  // formatted table
```

### pathutil — `stdlib/pathutil.glk`

Path string manipulation (no filesystem calls).

```glk
import "stdlib/pathutil.glk" as pathutil

pathutil.basename("/a/b/c.txt")  // "c.txt"
pathutil.dirname("/a/b/c.txt")   // "/a/b"
pathutil.ext("file.tar.gz")      // ".gz"
pathutil.stem("file.tar.gz")     // "file.tar"
pathutil.join2("/a", "b")        // "/a/b"
pathutil.join3("/a", "b", "c")   // "/a/b/c"
pathutil.is_absolute("/a/b")     // true
pathutil.normalize("a/b/c", true) // "a\b\c"
```

### textutil — `stdlib/textutil.glk`

Text processing helpers for source analysis.

```glk
import "stdlib/textutil.glk" as textutil

let src = fs.read_text("main.js")

textutil.lines(src)              // array of lines
textutil.count_lines(src)        // line count
textutil.strip_line_comments(src) // remove // comments
textutil.strip_all_comments(src)  // remove // and /* */
textutil.indent(src, "  ")       // indent each line
textutil.unindent(src, "  ")     // remove common prefix
textutil.grep("function", src)   // lines matching pattern
textutil.grep_count("import", src) // count of matching lines
textutil.truncate(src, 100)      // first 100 lines
textutil.word_count(src)         // rough word count
```

### io_helpers — `stdlib/io_helpers.glk`

File I/O convenience wrappers.

```glk
import "stdlib/io_helpers.glk" as io_helpers

io_helpers.read_all("file.txt")        // read entire file as string
io_helpers.write_lines("out.txt", lines) // write array of lines
```

### prelude — `stdlib/prelude.glk`

Optional numeric helpers.

```glk
import "stdlib/prelude.glk" as prelude

prelude.min(3, 5)  // 3
prelude.max(3, 5)  // 5
```

---

## Usage Tips

1. **Native modules** use bare names: `import re, str, fs`
2. **Stdlib libraries** use paths: `import "stdlib/hexutil.glk" as hexutil`
3. **Your own libraries**: `import "libs/mylib.glk" as mylib`
4. Libraries export all top-level `fn` except `main`.
5. Libraries can import native modules and other libraries.
6. No circular dependencies allowed.

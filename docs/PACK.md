# GLTK Packer (`gltk pack`)

Build a **standalone Windows EXE** that embeds encrypted GLVM bytecode.

## Components

| Piece | Role |
|-------|------|
| `cmd/glrt` | Runtime stub (GLVM + natives). Packed EXEs = stub + ciphertext + trailer |
| `internal/obfus` | Strong bytecode obfuscation (default on) |
| `internal/packer` | ChaCha20-Poly1305 encrypt + GPK1 trailer |
| `gltk pack` / `import pack` | CLI and GLK API |

## Usage

```powershell
cd D:\grokbuild\groklang\gltk
go build -o gltk.exe ./cmd/gltk
go build -o glrt.exe ./cmd/glrt

# .glk → compile → obfuscate → encrypt → exe
.\gltk.exe pack samples\pack_hello.glk -o work\hello.exe

# already .glkb
.\gltk.exe pack hello.glkb -o work\hello.exe

# no obfuscation (raw bytecode still encrypted at rest)
.\gltk.exe pack samples\pack_hello.glk -o work\hello.exe --no-obfus

# keep intermediate obfuscated glkb
.\gltk.exe pack samples\pack_hello.glk -o work\hello.exe --keep-glkb work\hello.glkb

# GLK front-end
.\gltk.exe run samples\pack_tool.glk -- samples\pack_hello.glk work\hello.exe
```

## Default obfuscation (when not `--no-obfus`)

1. Strip source name / line tables; rename protos  
2. Pad constant pool with bogus strings  
3. Shuffle constant pool (remap LOADK / GETK / …)  
4. Insert random **NOP** sleds  
5. Insert **unreachable junk** blocks (`JMP` over dead ops)  
6. Whole GLKB then **ChaCha20-Poly1305** encrypted; key wrapped with stub PE hash  

On disk the package is **not** a plain `GLKB` file — reverse needs stub understanding + AEAD key recovery.

## Format (EOF trailer `GPK1`)

```
[ glrt PE image ]
[ ciphertext = AEAD(GLKB) ]
[ trailer 88 bytes: magic, ver, flags, payload_off, payload_len, nonce, tag pad, key_blob, reserved ]
```

## Performance

- Pack path is pure Go (no per-pack `go build`); only needs a prebuilt `glrt.exe`  
- Obfuscation is linear in bytecode size; typical samples pack in tens of ms  

## Security notes

- This raises cost vs plain `.glkb` + `gltk run` significantly.  
- A determined reverse engineer with the stub source can still model the loader; do not treat as DRM for high-value secrets alone.  
- For max opacity, keep secrets off-device or use server-side checks.

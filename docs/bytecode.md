# GrokLang 字节码（GLKB）

## 文件格式

```
magic[4]     = "GLKB"
version u16  = 1
source_name  = u32 len + UTF-8
nconst u32
  每项: kind u8 + payload
    0 null
    1 bool  u8
    2 int   i64 LE
    3 float f64 LE bits
    4 str   u32 len + bytes
    5 bytes u32 len + bytes
nproto u32
  每项:
    name (string)
    numregs u8, numparams u8, numupvals u8
    ncode u32, code[] u32 LE
    nlines u32, lines[] u32  (每指令源行，可空)
main_index u32
```

## 指令编码（32-bit LE）

```
bits  0..7   opcode
bits  8..15  A
bits 16..23  B
bits 24..31  C
Bx  = bits 16..31  (unsigned 16)
sBx = int16(Bx)    (signed，用于跳转与 LOADI)
```

## 操作码一览

| Op | 助记符 | 语义 |
|----|--------|------|
| 0 | HALT | 停机 |
| 1 | MOVE | R[A]=R[B] |
| 2 | LOADK | R[A]=K[Bx] |
| 3 | LOADI | R[A]=sBx |
| 4 | LOADN | R[A]=null |
| 5 | LOADB | R[A]=(B!=0) |
| 6 | LOADF | R[A]=K[Bx]（浮点常量） |
| 7-11 | ADD SUB MUL DIV MOD | 算术 |
| 12-18 | AND OR XOR SHL SHR ROL ROR | 位运算 |
| 19-21 | NOT NEG LNOT | 一元 |
| 22-27 | EQ NE LT LE GT GE | 比较 → bool |
| 28 | JMP | ip+=sBx |
| 29 | JT | if R[A] ip+=sBx |
| 30 | JF | if !R[A] ip+=sBx |
| 31 | NEWARR | R[A]=[R[B]..R[B+C-1]] |
| 32 | NEWMAP | R[A]={} |
| 33 | GETI | R[A]=R[B][R[C]] |
| 34 | SETI | R[A][R[B]]=R[C] |
| 35 | GETK | R[A]=R[B][K[C]] |
| 36 | SETK | R[A][K[B]]=R[C] |
| 37 | LEN | R[A]=len(R[B]) |
| 38 | CONCAT | R[A]=R[B]∥R[C] |
| 39-42 | BGET8/16/32/64 | LE 读字节 |
| 43 | BSLICE | R[A]=R[B][R[C]:R[C+1]] |
| 44 | BSET8 | R[A][R[B]]=u8(R[C]) |
| 45 | CALL | R[A]=call R[B] nargs=C args@R[B+1..] |
| 46 | RET | return R[A] |
| 47 | RETN | return null |
| 48 | MAKEFN | R[A]=closure(proto Bx) |
| 49 | CLOSURE | 带 upvalue 的闭包 |
| 50-51 | GETUPV SETUPV | upvalue |
| 52 | NOP | |
| 53-57 | TOSTR TOINT TYPEOF ISNULL IN | 转换/查询 |
| 58-63 | BAND ARRPUSH KEYS ASSERT FORPREP FORLOOP | 辅助 |
| 64-65 | LOADG STOREG | 全局（K[Bx] 为名） |
| 66-67 | DUP SWAP | |
| 68 | IMPORT | R[A]=native_module(K[Bx]) |
| 69 | LIST | 同 NEWARR |

## 调用约定

- 被调函数参数落在子帧 `R[0]..R[NumParams-1]`
- `main` 若 `NumParams>=1`，则 `R[0]=args`（字符串数组）
- `CALL A B C`：函数值在 `R[B]`，参数 `R[B+1]..R[B+C]`，结果写 `R[A]`
- Native 与字节码函数同一调用协议

## 反汇编

```
gltk disasm file.glkb
```

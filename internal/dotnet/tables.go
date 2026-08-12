package dotnet

import (
	"encoding/binary"
	"fmt"
)

// Metadata table IDs (ECMA-335 §II.22).
const (
	TableModule                 = 0x00
	TableTypeRef                = 0x01
	TableTypeDef                = 0x02
	TableFieldPtr               = 0x03
	TableField                  = 0x04
	TableMethodPtr              = 0x05
	TableMethodDef              = 0x06
	TableParamPtr               = 0x07
	TableParam                  = 0x08
	TableInterfaceImpl          = 0x09
	TableMemberRef              = 0x0A
	TableConstant               = 0x0B
	TableCustomAttribute        = 0x0C
	TableFieldMarshal           = 0x0D
	TableDeclSecurity           = 0x0E
	TableClassLayout            = 0x0F
	TableFieldLayout            = 0x10
	TableStandAloneSig          = 0x11
	TableEventMap               = 0x12
	TableEventPtr               = 0x13
	TableEvent                  = 0x14
	TablePropertyMap            = 0x15
	TablePropertyPtr            = 0x16
	TableProperty               = 0x17
	TableMethodSemantics        = 0x18
	TableMethodImpl             = 0x19
	TableModuleRef              = 0x1A
	TableTypeSpec               = 0x1B
	TableImplMap                = 0x1C
	TableFieldRVA               = 0x1D
	TableENCLog                 = 0x1E
	TableENCMap                 = 0x1F
	TableAssembly               = 0x20
	TableAssemblyProcessor      = 0x21
	TableAssemblyOS             = 0x22
	TableAssemblyRef            = 0x23
	TableAssemblyRefProcessor   = 0x24
	TableAssemblyRefOS          = 0x25
	TableFile                   = 0x26
	TableExportedType           = 0x27
	TableManifestResource       = 0x28
	TableNestedClass            = 0x29
	TableGenericParam           = 0x2A
	TableMethodSpec             = 0x2B
	TableGenericParamConstraint = 0x2C
)

// Token type tags in high byte.
const (
	TokenModule      = 0x00
	TokenTypeRef     = 0x01
	TokenTypeDef     = 0x02
	TokenField       = 0x04
	TokenMethodDef   = 0x06
	TokenParam       = 0x08
	TokenMemberRef   = 0x0A
	TokenTypeSpec    = 0x1B
	TokenAssemblyRef = 0x23
	TokenUserString  = 0x70
)

// tablesHeap holds row counts and raw table bytes plus index widths.
type tablesHeap struct {
	HeapSizes byte
	Valid     uint64
	Sorted    uint64
	RowCounts [64]uint32
	// index sizes in bytes (2 or 4)
	StrIdx  int
	GuidIdx int
	BlobIdx int
	// coded / simple table index sizes
	idx [64]int
	// raw contiguous table data
	data []byte
	// offsets into data for each table
	tableOff [64]int
	tableSz  [64]int
	root     *metadataRoot
}

func parseTablesHeap(data []byte, root *metadataRoot) (*tablesHeap, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("dotnet: tables heap too small")
	}
	th := &tablesHeap{root: root, data: data}
	// reserved u32, major u8, minor u8, heapSizes u8, reserved u8, valid u64, sorted u64
	th.HeapSizes = data[6]
	th.Valid = binary.LittleEndian.Uint64(data[8:])
	th.Sorted = binary.LittleEndian.Uint64(data[16:])

	th.StrIdx = 2
	th.GuidIdx = 2
	th.BlobIdx = 2
	if th.HeapSizes&0x01 != 0 {
		th.StrIdx = 4
	}
	if th.HeapSizes&0x02 != 0 {
		th.GuidIdx = 4
	}
	if th.HeapSizes&0x04 != 0 {
		th.BlobIdx = 4
	}

	pos := 24
	for t := 0; t < 64; t++ {
		if th.Valid&(1<<uint(t)) != 0 {
			if pos+4 > len(data) {
				return nil, fmt.Errorf("dotnet: truncated row counts")
			}
			th.RowCounts[t] = binary.LittleEndian.Uint32(data[pos:])
			pos += 4
		}
	}

	// compute simple table index sizes
	for t := 0; t < 64; t++ {
		if th.RowCounts[t] > 0xFFFF {
			th.idx[t] = 4
		} else {
			th.idx[t] = 2
		}
	}

	// compute row sizes and table offsets
	for t := 0; t < 64; t++ {
		if th.Valid&(1<<uint(t)) == 0 {
			continue
		}
		rs := th.rowSize(t)
		if rs <= 0 {
			// unknown table — skip by guessing 0 (can't parse rest); try continue with best-effort
			// For unknown tables we still need size; use conservative estimate fails.
			// Leave size 0 and skip rows only if count is 0.
			if th.RowCounts[t] > 0 {
				return nil, fmt.Errorf("dotnet: unknown/unsupported table 0x%02x with %d rows", t, th.RowCounts[t])
			}
			continue
		}
		th.tableSz[t] = rs
		th.tableOff[t] = pos
		pos += rs * int(th.RowCounts[t])
		if pos > len(data) {
			return nil, fmt.Errorf("dotnet: table 0x%02x exceeds stream (need %d have %d)", t, pos, len(data))
		}
	}
	return th, nil
}

func (th *tablesHeap) coded(tagBits int, tables ...int) int {
	// max rows among tables
	var max uint32
	for _, t := range tables {
		if t >= 0 && th.RowCounts[t] > max {
			max = th.RowCounts[t]
		}
	}
	// if max >= 2^(16-tagBits) need 4 bytes
	limit := uint32(1) << (16 - uint(tagBits))
	if max >= limit {
		return 4
	}
	return 2
}

func (th *tablesHeap) rowSize(table int) int {
	s, g, b := th.StrIdx, th.GuidIdx, th.BlobIdx
	// helpers for coded indexes
	typeDefOrRef := th.coded(2, TableTypeDef, TableTypeRef, TableTypeSpec)
	hasConstant := th.coded(2, TableField, TableParam, TableProperty)
	hasCustomAttribute := th.coded(5,
		TableMethodDef, TableField, TableTypeRef, TableTypeDef, TableParam,
		TableInterfaceImpl, TableMemberRef, TableModule, TableDeclSecurity, TableProperty,
		TableEvent, TableStandAloneSig, TableModuleRef, TableTypeSpec, TableAssembly,
		TableAssemblyRef, TableFile, TableExportedType, TableManifestResource, TableGenericParam,
		TableGenericParamConstraint, TableMethodSpec)
	hasFieldMarshal := th.coded(1, TableField, TableParam)
	hasDeclSecurity := th.coded(2, TableTypeDef, TableMethodDef, TableAssembly)
	memberRefParent := th.coded(3, TableTypeDef, TableTypeRef, TableModuleRef, TableMethodDef, TableTypeSpec)
	hasSemantics := th.coded(1, TableEvent, TableProperty)
	methodDefOrRef := th.coded(1, TableMethodDef, TableMemberRef)
	memberForwarded := th.coded(1, TableField, TableMethodDef)
	implementation := th.coded(2, TableFile, TableAssemblyRef, TableExportedType)
	customAttributeType := th.coded(3, -1, -1, TableMethodDef, TableMemberRef, -1) // tags 2,3 used
	// fix: CustomAttributeType tables: 0 unused,1 unused,2 MethodDef,3 MemberRef,4 unused
	// coded() with -1 is awkward; recompute:
	{
		var max uint32
		if th.RowCounts[TableMethodDef] > max {
			max = th.RowCounts[TableMethodDef]
		}
		if th.RowCounts[TableMemberRef] > max {
			max = th.RowCounts[TableMemberRef]
		}
		if max >= (1 << (16 - 3)) {
			customAttributeType = 4
		} else {
			customAttributeType = 2
		}
	}
	resolutionScope := th.coded(2, TableModule, TableModuleRef, TableAssemblyRef, TableTypeRef)
	typeOrMethodDef := th.coded(1, TableTypeDef, TableMethodDef)

	switch table {
	case TableModule:
		// Generation u16, Name str, Mvid guid, EncId guid, EncBaseId guid
		return 2 + s + g + g + g
	case TableTypeRef:
		// ResolutionScope, TypeName str, TypeNamespace str
		return resolutionScope + s + s
	case TableTypeDef:
		// Flags u32, TypeName str, TypeNamespace str, Extends TypeDefOrRef, FieldList, MethodList
		return 4 + s + s + typeDefOrRef + th.idx[TableField] + th.idx[TableMethodDef]
	case TableFieldPtr:
		return th.idx[TableField]
	case TableField:
		// Flags u16, Name str, Signature blob
		return 2 + s + b
	case TableMethodPtr:
		return th.idx[TableMethodDef]
	case TableMethodDef:
		// RVA u32, ImplFlags u16, Flags u16, Name str, Signature blob, ParamList
		return 4 + 2 + 2 + s + b + th.idx[TableParam]
	case TableParamPtr:
		return th.idx[TableParam]
	case TableParam:
		// Flags u16, Sequence u16, Name str
		return 2 + 2 + s
	case TableInterfaceImpl:
		return th.idx[TableTypeDef] + typeDefOrRef
	case TableMemberRef:
		// Class MemberRefParent, Name str, Signature blob
		return memberRefParent + s + b
	case TableConstant:
		// Type u16, Parent HasConstant, Value blob
		return 2 + hasConstant + b
	case TableCustomAttribute:
		// Parent HasCustomAttribute, Type CustomAttributeType, Value blob
		return hasCustomAttribute + customAttributeType + b
	case TableFieldMarshal:
		return hasFieldMarshal + b
	case TableDeclSecurity:
		// Action u16, Parent HasDeclSecurity, PermissionSet blob
		return 2 + hasDeclSecurity + b
	case TableClassLayout:
		// PackingSize u16, ClassSize u32, Parent TypeDef
		return 2 + 4 + th.idx[TableTypeDef]
	case TableFieldLayout:
		// Offset u32, Field
		return 4 + th.idx[TableField]
	case TableStandAloneSig:
		return b
	case TableEventMap:
		return th.idx[TableTypeDef] + th.idx[TableEvent]
	case TableEventPtr:
		return th.idx[TableEvent]
	case TableEvent:
		// EventFlags u16, Name str, EventType TypeDefOrRef
		return 2 + s + typeDefOrRef
	case TablePropertyMap:
		return th.idx[TableTypeDef] + th.idx[TableProperty]
	case TablePropertyPtr:
		return th.idx[TableProperty]
	case TableProperty:
		// Flags u16, Name str, Type blob
		return 2 + s + b
	case TableMethodSemantics:
		// Semantics u16, Method, Association HasSemantics
		return 2 + th.idx[TableMethodDef] + hasSemantics
	case TableMethodImpl:
		// Class TypeDef, MethodBody MethodDefOrRef, MethodDeclaration MethodDefOrRef
		return th.idx[TableTypeDef] + methodDefOrRef + methodDefOrRef
	case TableModuleRef:
		return s
	case TableTypeSpec:
		return b
	case TableImplMap:
		// MappingFlags u16, MemberForwarded, ImportName str, ImportScope ModuleRef
		return 2 + memberForwarded + s + th.idx[TableModuleRef]
	case TableFieldRVA:
		return 4 + th.idx[TableField]
	case TableENCLog:
		return 4 + 4
	case TableENCMap:
		return 4
	case TableAssembly:
		// HashAlgId u32, Major u16, Minor u16, Build u16, Revision u16, Flags u32, PublicKey blob, Name str, Culture str
		return 4 + 2 + 2 + 2 + 2 + 4 + b + s + s
	case TableAssemblyProcessor:
		return 4
	case TableAssemblyOS:
		return 4 + 4 + 4
	case TableAssemblyRef:
		// Major,Minor,Build,Revision u16*4, Flags u32, PublicKeyOrToken blob, Name str, Culture str, HashValue blob
		return 2 + 2 + 2 + 2 + 4 + b + s + s + b
	case TableAssemblyRefProcessor:
		return 4 + th.idx[TableAssemblyRef]
	case TableAssemblyRefOS:
		return 4 + 4 + 4 + th.idx[TableAssemblyRef]
	case TableFile:
		// Flags u32, Name str, HashValue blob
		return 4 + s + b
	case TableExportedType:
		// Flags u32, TypeDefId u32, TypeName str, TypeNamespace str, Implementation
		return 4 + 4 + s + s + implementation
	case TableManifestResource:
		// Offset u32, Flags u32, Name str, Implementation
		return 4 + 4 + s + implementation
	case TableNestedClass:
		return th.idx[TableTypeDef] + th.idx[TableTypeDef]
	case TableGenericParam:
		// Number u16, Flags u16, Owner TypeOrMethodDef, Name str
		return 2 + 2 + typeOrMethodDef + s
	case TableMethodSpec:
		// Method MethodDefOrRef, Instantiation blob
		return methodDefOrRef + b
	case TableGenericParamConstraint:
		// Owner GenericParam, Constraint TypeDefOrRef
		return th.idx[TableGenericParam] + typeDefOrRef
	default:
		return 0
	}
}

func (th *tablesHeap) row(table int, rid uint32) []byte {
	if rid == 0 || rid > th.RowCounts[table] {
		return nil
	}
	rs := th.tableSz[table]
	off := th.tableOff[table] + int(rid-1)*rs
	if off+rs > len(th.data) {
		return nil
	}
	return th.data[off : off+rs]
}

func (th *tablesHeap) readIndex(row []byte, off int, size int) (uint32, int) {
	if size == 4 {
		return binary.LittleEndian.Uint32(row[off:]), off + 4
	}
	return uint32(binary.LittleEndian.Uint16(row[off:])), off + 2
}

// --- typed row accessors ---

// TypeDefRow is a TypeDef table row (1-based RID).
type TypeDefRow struct {
	RID       uint32
	Flags     uint32
	Name      string
	Namespace string
	Extends   uint32 // coded TypeDefOrRef
	FieldList uint32
	MethodList uint32
}

// MethodDefRow is a MethodDef table row.
type MethodDefRow struct {
	RID       uint32
	RVA       uint32
	ImplFlags uint16
	Flags     uint16
	Name      string
	Signature []byte
	ParamList uint32
}

// TypeRefRow is a TypeRef table row.
type TypeRefRow struct {
	RID            uint32
	ResolutionScope uint32
	Name           string
	Namespace      string
}

// MemberRefRow is a MemberRef table row.
type MemberRefRow struct {
	RID       uint32
	Class     uint32 // coded MemberRefParent
	Name      string
	Signature []byte
}

// FieldRow is a Field table row.
type FieldRow struct {
	RID       uint32
	Flags     uint16
	Name      string
	Signature []byte
}

// ModuleRow is Module table row 1.
type ModuleRow struct {
	Name string
}

// AssemblyRow is Assembly table row 1.
type AssemblyRow struct {
	Name           string
	Culture        string
	Major, Minor   uint16
	Build, Revision uint16
	Flags          uint32
}

func (th *tablesHeap) GetModule() ModuleRow {
	row := th.row(TableModule, 1)
	if row == nil {
		return ModuleRow{}
	}
	// skip Generation u16
	off := 2
	idx, _ := th.readIndex(row, off, th.StrIdx)
	return ModuleRow{Name: th.root.String(idx)}
}

func (th *tablesHeap) GetAssembly() (AssemblyRow, bool) {
	row := th.row(TableAssembly, 1)
	if row == nil {
		return AssemblyRow{}, false
	}
	// HashAlgId u32, Maj,Min,Build,Rev u16*4, Flags u32, PublicKey blob, Name, Culture
	off := 4
	a := AssemblyRow{}
	a.Major = binary.LittleEndian.Uint16(row[off:])
	a.Minor = binary.LittleEndian.Uint16(row[off+2:])
	a.Build = binary.LittleEndian.Uint16(row[off+4:])
	a.Revision = binary.LittleEndian.Uint16(row[off+6:])
	off += 8
	a.Flags = binary.LittleEndian.Uint32(row[off:])
	off += 4
	_, off = th.readIndex(row, off, th.BlobIdx)
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	cIdx, _ := th.readIndex(row, off, th.StrIdx)
	a.Name = th.root.String(nIdx)
	a.Culture = th.root.String(cIdx)
	return a, true
}

func (th *tablesHeap) TypeDefCount() int { return int(th.RowCounts[TableTypeDef]) }
func (th *tablesHeap) MethodCount() int  { return int(th.RowCounts[TableMethodDef]) }
func (th *tablesHeap) TypeRefCount() int { return int(th.RowCounts[TableTypeRef]) }
func (th *tablesHeap) MemberRefCount() int {
	return int(th.RowCounts[TableMemberRef])
}
func (th *tablesHeap) FieldCount() int { return int(th.RowCounts[TableField]) }

func (th *tablesHeap) GetTypeDef(rid uint32) (TypeDefRow, bool) {
	row := th.row(TableTypeDef, rid)
	if row == nil {
		return TypeDefRow{}, false
	}
	td := TypeDefRow{RID: rid}
	td.Flags = binary.LittleEndian.Uint32(row[0:])
	off := 4
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	nsIdx, off := th.readIndex(row, off, th.StrIdx)
	td.Name = th.root.String(nIdx)
	td.Namespace = th.root.String(nsIdx)
	typeDefOrRef := th.coded(2, TableTypeDef, TableTypeRef, TableTypeSpec)
	td.Extends, off = th.readIndex(row, off, typeDefOrRef)
	td.FieldList, off = th.readIndex(row, off, th.idx[TableField])
	td.MethodList, _ = th.readIndex(row, off, th.idx[TableMethodDef])
	return td, true
}

func (th *tablesHeap) GetMethodDef(rid uint32) (MethodDefRow, bool) {
	row := th.row(TableMethodDef, rid)
	if row == nil {
		return MethodDefRow{}, false
	}
	md := MethodDefRow{RID: rid}
	md.RVA = binary.LittleEndian.Uint32(row[0:])
	md.ImplFlags = binary.LittleEndian.Uint16(row[4:])
	md.Flags = binary.LittleEndian.Uint16(row[6:])
	off := 8
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	sigIdx, off := th.readIndex(row, off, th.BlobIdx)
	md.Name = th.root.String(nIdx)
	md.Signature = th.root.Blob(sigIdx)
	md.ParamList, _ = th.readIndex(row, off, th.idx[TableParam])
	return md, true
}

func (th *tablesHeap) GetTypeRef(rid uint32) (TypeRefRow, bool) {
	row := th.row(TableTypeRef, rid)
	if row == nil {
		return TypeRefRow{}, false
	}
	tr := TypeRefRow{RID: rid}
	resolutionScope := th.coded(2, TableModule, TableModuleRef, TableAssemblyRef, TableTypeRef)
	off := 0
	tr.ResolutionScope, off = th.readIndex(row, off, resolutionScope)
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	nsIdx, _ := th.readIndex(row, off, th.StrIdx)
	tr.Name = th.root.String(nIdx)
	tr.Namespace = th.root.String(nsIdx)
	return tr, true
}

func (th *tablesHeap) GetMemberRef(rid uint32) (MemberRefRow, bool) {
	row := th.row(TableMemberRef, rid)
	if row == nil {
		return MemberRefRow{}, false
	}
	mr := MemberRefRow{RID: rid}
	memberRefParent := th.coded(3, TableTypeDef, TableTypeRef, TableModuleRef, TableMethodDef, TableTypeSpec)
	off := 0
	mr.Class, off = th.readIndex(row, off, memberRefParent)
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	sigIdx, _ := th.readIndex(row, off, th.BlobIdx)
	mr.Name = th.root.String(nIdx)
	mr.Signature = th.root.Blob(sigIdx)
	return mr, true
}

func (th *tablesHeap) GetField(rid uint32) (FieldRow, bool) {
	row := th.row(TableField, rid)
	if row == nil {
		return FieldRow{}, false
	}
	f := FieldRow{RID: rid}
	f.Flags = binary.LittleEndian.Uint16(row[0:])
	off := 2
	nIdx, off := th.readIndex(row, off, th.StrIdx)
	sigIdx, _ := th.readIndex(row, off, th.BlobIdx)
	f.Name = th.root.String(nIdx)
	f.Signature = th.root.Blob(sigIdx)
	return f, true
}

// MethodRange returns inclusive method RID range for a TypeDef.
func (th *tablesHeap) MethodRange(typeRID uint32) (start, end uint32) {
	td, ok := th.GetTypeDef(typeRID)
	if !ok {
		return 0, 0
	}
	start = td.MethodList
	if typeRID < th.RowCounts[TableTypeDef] {
		next, _ := th.GetTypeDef(typeRID + 1)
		end = next.MethodList - 1
	} else {
		end = th.RowCounts[TableMethodDef]
	}
	if start == 0 {
		return 0, 0
	}
	if end < start {
		end = start - 1 // empty
	}
	return start, end
}

// Decode coded index helpers for token resolution.
func decodeTypeDefOrRef(coded uint32) (table int, rid uint32) {
	tag := coded & 0x3
	rid = coded >> 2
	switch tag {
	case 0:
		return TableTypeDef, rid
	case 1:
		return TableTypeRef, rid
	case 2:
		return TableTypeSpec, rid
	}
	return -1, 0
}

func decodeMemberRefParent(coded uint32) (table int, rid uint32) {
	tag := coded & 0x7
	rid = coded >> 3
	switch tag {
	case 0:
		return TableTypeDef, rid
	case 1:
		return TableTypeRef, rid
	case 2:
		return TableModuleRef, rid
	case 3:
		return TableMethodDef, rid
	case 4:
		return TableTypeSpec, rid
	}
	return -1, 0
}

func decodeMethodDefOrRef(coded uint32) (table int, rid uint32) {
	tag := coded & 0x1
	rid = coded >> 1
	if tag == 0 {
		return TableMethodDef, rid
	}
	return TableMemberRef, rid
}

func makeToken(table int, rid uint32) uint32 {
	return (uint32(table) << 24) | (rid & 0x00FFFFFF)
}

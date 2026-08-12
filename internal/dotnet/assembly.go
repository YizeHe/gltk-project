package dotnet

import (
	"fmt"
	"strings"
)

// Assembly is a parsed .NET managed PE image.
type Assembly struct {
	pe   *peInfo
	meta *metadataRoot

	// Info fields
	RuntimeVersion  string
	Flags           uint32
	EntryPointToken uint32
	IsILOnly        bool
	Is32BitRequired bool
	ModuleName      string
	AssemblyName    string
	AssemblyVersion string
}

// TypeDef is a high-level type description.
type TypeDef struct {
	Token       uint32
	Name        string
	Namespace   string
	Flags       uint32
	MethodStart uint32 // MethodDef RID inclusive
	MethodEnd   uint32 // MethodDef RID inclusive
	FullName    string
}

// Method is a high-level method description.
type Method struct {
	Token     uint32
	Name      string
	RVA       uint32
	ImplFlags uint16
	Flags     uint16
	Signature []byte
	TypeName  string // declaring type full name
	TypeRID   uint32
	RID       uint32
}

func parseAssembly(pe *peInfo) (*Assembly, error) {
	meta, err := parseMetadata(pe)
	if err != nil {
		return nil, err
	}
	a := &Assembly{
		pe:              pe,
		meta:            meta,
		RuntimeVersion:  meta.Version,
		Flags:           pe.CLI.Flags,
		EntryPointToken: pe.CLI.EntryPointToken,
		IsILOnly:        pe.CLI.Flags&FlagILOnly != 0,
		Is32BitRequired: pe.CLI.Flags&Flag32BitRequired != 0,
	}
	mod := meta.Tables.GetModule()
	a.ModuleName = mod.Name
	if ar, ok := meta.Tables.GetAssembly(); ok {
		a.AssemblyName = ar.Name
		a.AssemblyVersion = fmt.Sprintf("%d.%d.%d.%d", ar.Major, ar.Minor, ar.Build, ar.Revision)
	}
	return a, nil
}

// Info returns map-like summary fields.
func (a *Assembly) Info() map[string]interface{} {
	th := a.meta.Tables
	m := map[string]interface{}{
		"ok":               true,
		"version":          a.RuntimeVersion,
		"flags":            a.Flags,
		"entry_point":      fmt.Sprintf("0x%08X", a.EntryPointToken),
		"entry_point_tok":  a.EntryPointToken,
		"is_ilonly":        a.IsILOnly,
		"is_32bit":         a.Is32BitRequired,
		"module":           a.ModuleName,
		"assembly":         a.AssemblyName,
		"assembly_version": a.AssemblyVersion,
		"type_count":       th.TypeDefCount(),
		"method_count":     th.MethodCount(),
		"typeref_count":    th.TypeRefCount(),
		"memberref_count":  th.MemberRefCount(),
		"field_count":      th.FieldCount(),
		"metadata_rva":     a.pe.CLI.MetaDataRVA,
		"metadata_size":    a.pe.CLI.MetaDataSize,
	}
	return m
}

// UserStrings returns all #US heap strings.
func (a *Assembly) UserStrings() []string {
	return a.meta.AllUserStrings()
}

// Types returns all TypeDef entries (including <Module>).
func (a *Assembly) Types() []TypeDef {
	th := a.meta.Tables
	n := th.TypeDefCount()
	out := make([]TypeDef, 0, n)
	for i := uint32(1); i <= uint32(n); i++ {
		td, ok := th.GetTypeDef(i)
		if !ok {
			continue
		}
		start, end := th.MethodRange(i)
		full := td.Name
		if td.Namespace != "" {
			full = td.Namespace + "." + td.Name
		}
		out = append(out, TypeDef{
			Token:       makeToken(TableTypeDef, i),
			Name:        td.Name,
			Namespace:   td.Namespace,
			Flags:       td.Flags,
			MethodStart: start,
			MethodEnd:   end,
			FullName:    full,
		})
	}
	return out
}

// Methods returns all MethodDef entries with declaring type names.
// If typeFilter is non-empty, only methods of matching types (full or short name, case-insensitive).
func (a *Assembly) Methods(typeFilter string) []Method {
	th := a.meta.Tables
	types := a.Types()
	filter := strings.ToLower(strings.TrimSpace(typeFilter))
	var out []Method
	for _, t := range types {
		if filter != "" {
			if strings.ToLower(t.FullName) != filter && strings.ToLower(t.Name) != filter {
				continue
			}
		}
		if t.MethodStart == 0 {
			continue
		}
		for rid := t.MethodStart; rid <= t.MethodEnd; rid++ {
			md, ok := th.GetMethodDef(rid)
			if !ok {
				continue
			}
			out = append(out, Method{
				Token:     makeToken(TableMethodDef, rid),
				Name:      md.Name,
				RVA:       md.RVA,
				ImplFlags: md.ImplFlags,
				Flags:     md.Flags,
				Signature: md.Signature,
				TypeName:  t.FullName,
				TypeRID:   t.Token & 0xFFFFFF,
				RID:       rid,
			})
		}
	}
	return out
}

// FindMethod locates a method by type and method name.
// typeName may be "Namespace.Name" or short name; empty matches any type.
func (a *Assembly) FindMethod(typeName, methodName string) (Method, bool) {
	methods := a.Methods(typeName)
	want := strings.ToLower(methodName)
	for _, m := range methods {
		if strings.ToLower(m.Name) == want {
			return m, true
		}
	}
	// if typeName was empty or short, Methods already filtered; try without type filter
	if typeName != "" {
		methods = a.Methods("")
		tn := strings.ToLower(typeName)
		for _, m := range methods {
			if strings.ToLower(m.Name) == want {
				if strings.ToLower(m.TypeName) == tn || strings.HasSuffix(strings.ToLower(m.TypeName), "."+tn) {
					return m, true
				}
			}
		}
	}
	return Method{}, false
}

// DumpIL disassembles the method body at the method's RVA.
func (a *Assembly) DumpIL(m Method) (string, error) {
	if m.RVA == 0 {
		return "", fmt.Errorf("method has no RVA (abstract/extern/runtime)")
	}
	return a.disassemble(m)
}

// ResolveToken returns a human-readable name for a metadata token.
func (a *Assembly) ResolveToken(token uint32) string {
	table := int(token >> 24)
	rid := token & 0x00FFFFFF
	th := a.meta.Tables
	switch table {
	case TokenUserString:
		return fmt.Sprintf("%q", a.meta.UserString(rid))
	case TableMethodDef:
		md, ok := th.GetMethodDef(rid)
		if !ok {
			return fmt.Sprintf("MethodDef[%d]", rid)
		}
		// find type
		tname := a.typeNameForMethod(rid)
		if tname != "" {
			return tname + "::" + md.Name
		}
		return md.Name
	case TableMemberRef:
		mr, ok := th.GetMemberRef(rid)
		if !ok {
			return fmt.Sprintf("MemberRef[%d]", rid)
		}
		parent := a.resolveMemberRefParent(mr.Class)
		if parent != "" {
			return parent + "::" + mr.Name
		}
		return mr.Name
	case TableTypeDef:
		td, ok := th.GetTypeDef(rid)
		if !ok {
			return fmt.Sprintf("TypeDef[%d]", rid)
		}
		if td.Namespace != "" {
			return td.Namespace + "." + td.Name
		}
		return td.Name
	case TableTypeRef:
		tr, ok := th.GetTypeRef(rid)
		if !ok {
			return fmt.Sprintf("TypeRef[%d]", rid)
		}
		if tr.Namespace != "" {
			return tr.Namespace + "." + tr.Name
		}
		return tr.Name
	case TableField:
		f, ok := th.GetField(rid)
		if !ok {
			return fmt.Sprintf("Field[%d]", rid)
		}
		return f.Name
	case TableTypeSpec:
		return fmt.Sprintf("TypeSpec[%d]", rid)
	case TableStandAloneSig:
		return fmt.Sprintf("StandAloneSig[%d]", rid)
	default:
		return fmt.Sprintf("token_0x%08X", token)
	}
}

func (a *Assembly) typeNameForMethod(methodRID uint32) string {
	for _, t := range a.Types() {
		if methodRID >= t.MethodStart && methodRID <= t.MethodEnd && t.MethodStart != 0 {
			return t.FullName
		}
	}
	return ""
}

func (a *Assembly) resolveMemberRefParent(coded uint32) string {
	table, rid := decodeMemberRefParent(coded)
	switch table {
	case TableTypeDef:
		td, ok := a.meta.Tables.GetTypeDef(rid)
		if !ok {
			return ""
		}
		if td.Namespace != "" {
			return td.Namespace + "." + td.Name
		}
		return td.Name
	case TableTypeRef:
		tr, ok := a.meta.Tables.GetTypeRef(rid)
		if !ok {
			return ""
		}
		if tr.Namespace != "" {
			return tr.Namespace + "." + tr.Name
		}
		return tr.Name
	case TableTypeSpec:
		return fmt.Sprintf("TypeSpec[%d]", rid)
	case TableModuleRef:
		return fmt.Sprintf("ModuleRef[%d]", rid)
	case TableMethodDef:
		md, ok := a.meta.Tables.GetMethodDef(rid)
		if ok {
			return md.Name
		}
	}
	return ""
}

// Dump produces a multi-type textual dump of types/methods and optional IL.
func (a *Assembly) Dump(opts DumpOptions) string {
	var b strings.Builder
	fmt.Fprintf(&b, "// Runtime: %s\n", a.RuntimeVersion)
	fmt.Fprintf(&b, "// Module: %s\n", a.ModuleName)
	if a.AssemblyName != "" {
		fmt.Fprintf(&b, "// Assembly: %s %s\n", a.AssemblyName, a.AssemblyVersion)
	}
	fmt.Fprintf(&b, "// Flags: 0x%X ILONLY=%v\n", a.Flags, a.IsILOnly)
	fmt.Fprintf(&b, "// EntryPoint: 0x%08X\n", a.EntryPointToken)
	fmt.Fprintf(&b, "// Types: %d  Methods: %d\n\n", a.meta.Tables.TypeDefCount(), a.meta.Tables.MethodCount())

	if opts.Strings || opts.All {
		fmt.Fprintf(&b, "// === User Strings (#US) ===\n")
		for i, s := range a.UserStrings() {
			fmt.Fprintf(&b, "// US[%d]: %q\n", i, s)
		}
		b.WriteByte('\n')
	}

	types := a.Types()
	filter := strings.ToLower(opts.TypeFilter)
	for _, t := range types {
		if filter != "" {
			if strings.ToLower(t.FullName) != filter && strings.ToLower(t.Name) != filter {
				continue
			}
		}
		// skip empty module type unless requested
		if t.Name == "<Module>" && filter == "" && !opts.IncludeModule {
			continue
		}
		fmt.Fprintf(&b, ".class %s  // token 0x%08X flags=0x%X methods=%d\n",
			t.FullName, t.Token, t.Flags, max(0, int(t.MethodEnd)-int(t.MethodStart)+1))
		if t.MethodStart == 0 {
			continue
		}
		for rid := t.MethodStart; rid <= t.MethodEnd; rid++ {
			md, ok := a.meta.Tables.GetMethodDef(rid)
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "  .method %s rva=0x%X token=0x%08X flags=0x%X impl=0x%X\n",
				md.Name, md.RVA, makeToken(TableMethodDef, rid), md.Flags, md.ImplFlags)
			if opts.IL || opts.All {
				if md.RVA != 0 {
					il, err := a.DumpIL(Method{
						Token: makeToken(TableMethodDef, rid),
						Name:  md.Name,
						RVA:   md.RVA,
						RID:   rid,
					})
					if err != nil {
						fmt.Fprintf(&b, "    // IL error: %v\n", err)
					} else {
						for _, line := range strings.Split(il, "\n") {
							if line != "" {
								fmt.Fprintf(&b, "    %s\n", line)
							}
						}
					}
				}
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// DumpOptions controls Assembly.Dump.
type DumpOptions struct {
	IL            bool
	Strings       bool
	All           bool
	TypeFilter    string
	IncludeModule bool
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// InterestingStrings filters user strings for RE triage (keys, passwords, crypto hints).
func (a *Assembly) InterestingStrings() []string {
	var out []string
	for _, s := range a.UserStrings() {
		if isInterestingString(s) {
			out = append(out, s)
		}
	}
	return out
}

func isInterestingString(s string) bool {
	if len(s) < 3 {
		return false
	}
	lower := strings.ToLower(s)
	keywords := []string{
		"ahpk", "password", "passwd", "aes", "hmac", "sha256", "sha1", "md5",
		"encrypt", "decrypt", "cipher", "secret", "token", "apikey", "api_key",
		"private", "license", "activation", "base64", "key", "salt", "iv",
		"auth", "login", "credential", "protected",
	}
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	// long base64-ish
	if len(s) >= 16 && isBase64ish(s) {
		return true
	}
	// hex key material
	if len(s) >= 16 && isHexish(s) {
		return true
	}
	return false
}

func isBase64ish(s string) bool {
	// allow standard base64 charset and optional padding
	pad := 0
	for i := len(s) - 1; i >= 0 && s[i] == '='; i-- {
		pad++
		if pad > 2 {
			return false
		}
	}
	body := s[:len(s)-pad]
	if len(body) < 12 {
		return false
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func isHexish(s string) bool {
	if len(s)%2 != 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}

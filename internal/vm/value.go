package vm

import (
	"fmt"
	"strconv"
	"strings"

	"groklang/gltk/internal/bytecode"
)

// Type tags a runtime Value.
type Type uint8

const (
	TypeNull Type = iota
	TypeBool
	TypeInt
	TypeFloat
	TypeStr
	TypeBytes
	TypeArray
	TypeMap
	TypeFunc
	TypeNative
)

// NativeFunc is a Go builtin callable from GLVM.
type NativeFunc func(vm *VM, args []Value) (Value, error)

// Closure is a callable function with optional upvalues.
type Closure struct {
	Proto   *bytecode.Proto
	ProtoIx int
	Upvals  []Value
}

// Value is a tagged runtime value (register contents).
type Value struct {
	Typ  Type
	B    bool
	I    int64
	F    float64
	S    string
	Bytes []byte // zero-copy view when possible
	Arr  *[]Value
	Map  *map[string]Value
	Fn   *Closure
	Nat  NativeFunc
	NatName string
}

func Null() Value             { return Value{Typ: TypeNull} }
func Bool(b bool) Value       { return Value{Typ: TypeBool, B: b} }
func Int(i int64) Value       { return Value{Typ: TypeInt, I: i} }
func Float(f float64) Value   { return Value{Typ: TypeFloat, F: f} }
func Str(s string) Value      { return Value{Typ: TypeStr, S: s} }
func Bytes(b []byte) Value    { return Value{Typ: TypeBytes, Bytes: b} }
func Array(a []Value) Value   { return Value{Typ: TypeArray, Arr: &a} }
func MapVal(m map[string]Value) Value {
	return Value{Typ: TypeMap, Map: &m}
}
func Func(c *Closure) Value { return Value{Typ: TypeFunc, Fn: c} }
func Native(name string, f NativeFunc) Value {
	return Value{Typ: TypeNative, Nat: f, NatName: name}
}

// Truthy follows JS-ish rules: null/false/0/"" empty are false.
func (v Value) Truthy() bool {
	switch v.Typ {
	case TypeNull:
		return false
	case TypeBool:
		return v.B
	case TypeInt:
		return v.I != 0
	case TypeFloat:
		return v.F != 0
	case TypeStr:
		return v.S != ""
	case TypeBytes:
		return len(v.Bytes) != 0
	case TypeArray:
		return v.Arr != nil && len(*v.Arr) != 0
	case TypeMap:
		return v.Map != nil && len(*v.Map) != 0
	default:
		return true
	}
}

// TypeName returns a short type name.
func (v Value) TypeName() string {
	switch v.Typ {
	case TypeNull:
		return "null"
	case TypeBool:
		return "bool"
	case TypeInt:
		return "int"
	case TypeFloat:
		return "float"
	case TypeStr:
		return "str"
	case TypeBytes:
		return "bytes"
	case TypeArray:
		return "array"
	case TypeMap:
		return "map"
	case TypeFunc:
		return "func"
	case TypeNative:
		return "native"
	default:
		return "?"
	}
}

// String for debugging / print.
func (v Value) String() string {
	switch v.Typ {
	case TypeNull:
		return "null"
	case TypeBool:
		if v.B {
			return "true"
		}
		return "false"
	case TypeInt:
		return strconv.FormatInt(v.I, 10)
	case TypeFloat:
		return strconv.FormatFloat(v.F, 'g', -1, 64)
	case TypeStr:
		return v.S
	case TypeBytes:
		return fmt.Sprintf("bytes[%d]", len(v.Bytes))
	case TypeArray:
		if v.Arr == nil {
			return "[]"
		}
		parts := make([]string, len(*v.Arr))
		for i, e := range *v.Arr {
			parts[i] = e.String()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case TypeMap:
		if v.Map == nil {
			return "{}"
		}
		parts := make([]string, 0, len(*v.Map))
		for k, e := range *v.Map {
			parts = append(parts, k+": "+e.String())
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case TypeFunc:
		if v.Fn != nil && v.Fn.Proto != nil {
			return "fn:" + v.Fn.Proto.Name
		}
		return "fn"
	case TypeNative:
		return "native:" + v.NatName
	default:
		return "?"
	}
}

// AsInt coerces to int64.
func (v Value) AsInt() (int64, error) {
	switch v.Typ {
	case TypeInt:
		return v.I, nil
	case TypeFloat:
		return int64(v.F), nil
	case TypeBool:
		if v.B {
			return 1, nil
		}
		return 0, nil
	case TypeStr:
		i, err := strconv.ParseInt(v.S, 10, 64)
		return i, err
	default:
		return 0, fmt.Errorf("cannot convert %s to int", v.TypeName())
	}
}

// AsFloat coerces to float64.
func (v Value) AsFloat() (float64, error) {
	switch v.Typ {
	case TypeFloat:
		return v.F, nil
	case TypeInt:
		return float64(v.I), nil
	case TypeStr:
		return strconv.ParseFloat(v.S, 64)
	default:
		return 0, fmt.Errorf("cannot convert %s to float", v.TypeName())
	}
}

// AsStr coerces to string.
func (v Value) AsStr() string {
	return v.String()
}

// AsBytes returns byte slice.
func (v Value) AsBytes() ([]byte, error) {
	switch v.Typ {
	case TypeBytes:
		return v.Bytes, nil
	case TypeStr:
		return []byte(v.S), nil
	default:
		return nil, fmt.Errorf("cannot convert %s to bytes", v.TypeName())
	}
}

// Len of value.
func (v Value) Len() (int64, error) {
	switch v.Typ {
	case TypeStr:
		return int64(len(v.S)), nil
	case TypeBytes:
		return int64(len(v.Bytes)), nil
	case TypeArray:
		if v.Arr == nil {
			return 0, nil
		}
		return int64(len(*v.Arr)), nil
	case TypeMap:
		if v.Map == nil {
			return 0, nil
		}
		return int64(len(*v.Map)), nil
	default:
		return 0, fmt.Errorf("len not defined for %s", v.TypeName())
	}
}

// Equal deep-ish equality.
func (v Value) Equal(o Value) bool {
	if v.Typ != o.Typ {
		// int/float loose?
		if (v.Typ == TypeInt && o.Typ == TypeFloat) || (v.Typ == TypeFloat && o.Typ == TypeInt) {
			vf, _ := v.AsFloat()
			of, _ := o.AsFloat()
			return vf == of
		}
		return false
	}
	switch v.Typ {
	case TypeNull:
		return true
	case TypeBool:
		return v.B == o.B
	case TypeInt:
		return v.I == o.I
	case TypeFloat:
		return v.F == o.F
	case TypeStr:
		return v.S == o.S
	case TypeBytes:
		if len(v.Bytes) != len(o.Bytes) {
			return false
		}
		for i := range v.Bytes {
			if v.Bytes[i] != o.Bytes[i] {
				return false
			}
		}
		return true
	case TypeArray:
		if v.Arr == nil || o.Arr == nil {
			return v.Arr == o.Arr
		}
		if len(*v.Arr) != len(*o.Arr) {
			return false
		}
		for i := range *v.Arr {
			if !(*v.Arr)[i].Equal((*o.Arr)[i]) {
				return false
			}
		}
		return true
	case TypeMap, TypeFunc, TypeNative:
		return false // identity not implemented deeply
	default:
		return false
	}
}

// Compare for ordering: returns -1,0,1
func (v Value) Compare(o Value) (int, error) {
	if v.Typ == TypeStr && o.Typ == TypeStr {
		if v.S < o.S {
			return -1, nil
		}
		if v.S > o.S {
			return 1, nil
		}
		return 0, nil
	}
	vf, err1 := v.AsFloat()
	of, err2 := o.AsFloat()
	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("cannot compare %s and %s", v.TypeName(), o.TypeName())
	}
	if vf < of {
		return -1, nil
	}
	if vf > of {
		return 1, nil
	}
	return 0, nil
}

// ConstToValue converts bytecode constant to Value.
func ConstToValue(k bytecode.Constant) Value {
	switch k.Kind {
	case bytecode.ConstNull:
		return Null()
	case bytecode.ConstBool:
		return Bool(k.Bool)
	case bytecode.ConstInt:
		return Int(k.Int)
	case bytecode.ConstFloat:
		return Float(k.Float)
	case bytecode.ConstStr:
		return Str(k.Str)
	case bytecode.ConstBytes:
		return Bytes(k.Bytes)
	default:
		return Null()
	}
}

// GetIndex a[i]
func (v Value) GetIndex(idx Value) (Value, error) {
	switch v.Typ {
	case TypeArray:
		i, err := idx.AsInt()
		if err != nil {
			return Null(), err
		}
		if v.Arr == nil || i < 0 || int(i) >= len(*v.Arr) {
			return Null(), fmt.Errorf("array index out of range: %d", i)
		}
		return (*v.Arr)[i], nil
	case TypeMap:
		key := idx.AsStr()
		if v.Map == nil {
			return Null(), nil
		}
		val, ok := (*v.Map)[key]
		if !ok {
			return Null(), nil
		}
		return val, nil
	case TypeStr:
		i, err := idx.AsInt()
		if err != nil {
			return Null(), err
		}
		if i < 0 || int(i) >= len(v.S) {
			return Null(), fmt.Errorf("string index out of range")
		}
		return Str(string(v.S[i])), nil
	case TypeBytes:
		i, err := idx.AsInt()
		if err != nil {
			return Null(), err
		}
		if i < 0 || int(i) >= len(v.Bytes) {
			return Null(), fmt.Errorf("bytes index out of range")
		}
		return Int(int64(v.Bytes[i])), nil
	default:
		return Null(), fmt.Errorf("cannot index %s", v.TypeName())
	}
}

// SetIndex a[i] = val
func (v Value) SetIndex(idx, val Value) error {
	switch v.Typ {
	case TypeArray:
		i, err := idx.AsInt()
		if err != nil {
			return err
		}
		if v.Arr == nil || i < 0 || int(i) >= len(*v.Arr) {
			return fmt.Errorf("array index out of range: %d", i)
		}
		(*v.Arr)[i] = val
		return nil
	case TypeMap:
		if v.Map == nil {
			m := map[string]Value{}
			*v.Map = m
		}
		(*v.Map)[idx.AsStr()] = val
		return nil
	default:
		return fmt.Errorf("cannot set index on %s", v.TypeName())
	}
}

package native

import (
	"encoding/json"

	"groklang/gltk/internal/vm"
)

func moduleJSON() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"stringify":       jsonStringify,
		"parse":           jsonParse,
		"flatten_servers": jsonFlattenServers, // GreenHub server_list_v7 → flat nodes
	})
}

// json.flatten_servers(body_or_value) → array of maps {domain,id,country,jd_type,label_zh,...}
// Handles GreenHub shape: { data: [ { servers: [ {domain,...} ] } ] }
func jsonFlattenServers(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Array(nil), nil
	}
	var raw interface{}
	if args[0].Typ == vm.TypeStr || args[0].Typ == vm.TypeBytes {
		s := args[0].AsStr()
		if b, err := args[0].AsBytes(); err == nil {
			s = string(b)
		}
		if err := json.Unmarshal([]byte(s), &raw); err != nil {
			return vm.Null(), err
		}
	} else {
		var err error
		raw, err = valueToJSON(args[0])
		if err != nil {
			return vm.Null(), err
		}
	}
	var out []vm.Value
	walkServers(raw, &out)
	return vm.Array(out), nil
}

func walkServers(raw interface{}, out *[]vm.Value) {
	switch t := raw.(type) {
	case map[string]interface{}:
		// leaf node
		if dom, ok := t["domain"].(string); ok && dom != "" {
			*out = append(*out, jsonToValue(t))
			return
		}
		if servers, ok := t["servers"].([]interface{}); ok {
			for _, s := range servers {
				walkServers(s, out)
			}
			return
		}
		if data, ok := t["data"].([]interface{}); ok {
			for _, s := range data {
				walkServers(s, out)
			}
			return
		}
	case []interface{}:
		for _, s := range t {
			walkServers(s, out)
		}
	}
}

func jsonStringify(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Str("null"), nil
	}
	v, err := valueToJSON(args[0])
	if err != nil {
		return vm.Null(), err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Str(string(b)), nil
}

func jsonParse(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), nil
	}
	var raw interface{}
	if err := json.Unmarshal([]byte(args[0].AsStr()), &raw); err != nil {
		return vm.Null(), err
	}
	return jsonToValue(raw), nil
}

func valueToJSON(v vm.Value) (interface{}, error) {
	switch v.Typ {
	case vm.TypeNull:
		return nil, nil
	case vm.TypeBool:
		return v.B, nil
	case vm.TypeInt:
		return v.I, nil
	case vm.TypeFloat:
		return v.F, nil
	case vm.TypeStr:
		return v.S, nil
	case vm.TypeBytes:
		return v.Bytes, nil
	case vm.TypeArray:
		if v.Arr == nil {
			return []interface{}{}, nil
		}
		arr := make([]interface{}, len(*v.Arr))
		for i, e := range *v.Arr {
			x, err := valueToJSON(e)
			if err != nil {
				return nil, err
			}
			arr[i] = x
		}
		return arr, nil
	case vm.TypeMap:
		if v.Map == nil {
			return map[string]interface{}{}, nil
		}
		m := make(map[string]interface{}, len(*v.Map))
		for k, e := range *v.Map {
			x, err := valueToJSON(e)
			if err != nil {
				return nil, err
			}
			m[k] = x
		}
		return m, nil
	default:
		return v.AsStr(), nil
	}
}

func jsonToValue(raw interface{}) vm.Value {
	switch t := raw.(type) {
	case nil:
		return vm.Null()
	case bool:
		return vm.Bool(t)
	case float64:
		if t == float64(int64(t)) {
			return vm.Int(int64(t))
		}
		return vm.Float(t)
	case string:
		return vm.Str(t)
	case []interface{}:
		arr := make([]vm.Value, len(t))
		for i, e := range t {
			arr[i] = jsonToValue(e)
		}
		return vm.Array(arr)
	case map[string]interface{}:
		m := make(map[string]vm.Value, len(t))
		for k, e := range t {
			m[k] = jsonToValue(e)
		}
		return vm.MapVal(m)
	default:
		return vm.Str("")
	}
}

package native

import (
	"regexp"
	"sync"

	"groklang/gltk/internal/vm"
)

func moduleRe() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"compile":  reCompile,
		"match":    reMatch,
		"find":     reFind,
		"find_all": reFindAll,
		"replace":  reReplace,
		"split":    reSplit,
		"groups":   reGroups,
	})
}

var (
	reCacheMu sync.RWMutex
	reCache   = map[string]*regexp.Regexp{}
)

func reGet(pattern string) (*regexp.Regexp, error) {
	reCacheMu.RLock()
	if r, ok := reCache[pattern]; ok {
		reCacheMu.RUnlock()
		return r, nil
	}
	reCacheMu.RUnlock()
	r, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	reCacheMu.Lock()
	reCache[pattern] = r
	reCacheMu.Unlock()
	return r, nil
}

func rePattern(args []vm.Value) (string, error) {
	if len(args) < 1 {
		return "", errf("re: missing pattern")
	}
	// handle map from re.compile: {pattern: "..."}
	if args[0].Typ == vm.TypeMap && args[0].Map != nil {
		if p, ok := (*args[0].Map)["pattern"]; ok {
			return p.AsStr(), nil
		}
	}
	return args[0].AsStr(), nil
}

func reCompile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("re.compile(pattern)")
	}
	pat := args[0].AsStr()
	if _, err := reGet(pat); err != nil {
		return vm.Null(), err
	}
	return vm.MapVal(map[string]vm.Value{
		"type":    vm.Str("regexp"),
		"pattern": vm.Str(pat),
	}), nil
}

func reMatch(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Bool(false), nil
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Bool(r.MatchString(args[1].AsStr())), nil
}

func reFind(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), nil
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	loc := r.FindStringIndex(args[1].AsStr())
	if loc == nil {
		return vm.MapVal(map[string]vm.Value{"ok": vm.Bool(false)}), nil
	}
	s := args[1].AsStr()
	return vm.MapVal(map[string]vm.Value{
		"ok":    vm.Bool(true),
		"match": vm.Str(s[loc[0]:loc[1]]),
		"start": vm.Int(int64(loc[0])),
		"end":   vm.Int(int64(loc[1])),
	}), nil
}

func reFindAll(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Array(nil), nil
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	max := -1
	if len(args) >= 3 {
		if n, e := args[2].AsInt(); e == nil {
			max = int(n)
		}
	}
	ms := r.FindAllString(args[1].AsStr(), max)
	arr := make([]vm.Value, len(ms))
	for i, m := range ms {
		arr[i] = vm.Str(m)
	}
	return vm.Array(arr), nil
}

func reReplace(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("re.replace(pat, text, repl, n?)")
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	text := args[1].AsStr()
	repl := args[2].AsStr()
	n := -1
	if len(args) >= 4 {
		if i, e := args[3].AsInt(); e == nil {
			n = int(i)
		}
	}
	if n < 0 {
		return vm.Str(r.ReplaceAllString(text, repl)), nil
	}
	count := 0
	result := r.ReplaceAllStringFunc(text, func(m string) string {
		if count >= n {
			return m
		}
		count++
		// Expand $1 etc. against this match by ReplaceAllString on the match alone
		return r.ReplaceAllString(m, repl)
	})
	return vm.Str(result), nil
}

func reSplit(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Array(nil), nil
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	max := -1
	if len(args) >= 3 {
		if n, e := args[2].AsInt(); e == nil {
			max = int(n)
		}
	}
	parts := r.Split(args[1].AsStr(), max)
	arr := make([]vm.Value, len(parts))
	for i, p := range parts {
		arr[i] = vm.Str(p)
	}
	return vm.Array(arr), nil
}

func reGroups(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Array(nil), nil
	}
	pat, err := rePattern(args)
	if err != nil {
		return vm.Null(), err
	}
	r, err := reGet(pat)
	if err != nil {
		return vm.Null(), err
	}
	m := r.FindStringSubmatch(args[1].AsStr())
	if m == nil {
		return vm.Array(nil), nil
	}
	arr := make([]vm.Value, len(m))
	for i, g := range m {
		arr[i] = vm.Str(g)
	}
	return vm.Array(arr), nil
}

package native

import (
	"math/rand"
	"sort"
	"time"

	"groklang/gltk/internal/vm"
)

func moduleTime() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"now":       timeNowUnix,
		"now_ms":    timeNowMS,
		"now_iso":   timeNowISO,
		"format":    timeFormat,
		"sleep_ms":  timeSleepMS,
		"since_ms":  timeSinceMS,
	})
}

func moduleRand() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"int":   randInt,
		"float": randFloat,
		"seed":  randSeed,
		"pick":  randPick,
		"shuffle": randShuffle,
	})
}

func moduleSort() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"strings": sortStrings,
		"ints":    sortInts,
		"sort":    sortGeneric,
	})
}

func timeNowUnix(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Int(time.Now().Unix()), nil
}

func timeNowMS(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Int(time.Now().UnixMilli()), nil
}

func timeNowISO(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Str(time.Now().Format(time.RFC3339)), nil
}

// time.format(unix_sec, layout?) layout default RFC3339
func timeFormat(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("time.format(unix_sec, layout?)")
	}
	sec, err := args[0].AsInt()
	if err != nil {
		return vm.Null(), err
	}
	layout := time.RFC3339
	if len(args) >= 2 && args[1].AsStr() != "" {
		layout = args[1].AsStr()
		// common shortcuts
		switch layout {
		case "date":
			layout = "2006-01-02"
		case "datetime":
			layout = "2006-01-02 15:04:05"
		case "time":
			layout = "15:04:05"
		}
	}
	return vm.Str(time.Unix(sec, 0).Format(layout)), nil
}

func timeSleepMS(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	ms := int64(0)
	if len(args) >= 1 {
		if x, err := args[0].AsInt(); err == nil {
			ms = x
		}
	}
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	return vm.Null(), nil
}

func timeSinceMS(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(0), nil
	}
	start, err := args[0].AsInt()
	if err != nil {
		return vm.Int(0), nil
	}
	return vm.Int(time.Now().UnixMilli() - start), nil
}

var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randSeed(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) >= 1 {
		if s, err := args[0].AsInt(); err == nil {
			globalRand = rand.New(rand.NewSource(s))
		}
	}
	return vm.Null(), nil
}

// rand.int(max) or rand.int(min, max) half-open [min,max)
func randInt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Int(int64(globalRand.Int())), nil
	}
	if len(args) == 1 {
		max, err := args[0].AsInt()
		if err != nil || max <= 0 {
			return vm.Int(0), nil
		}
		return vm.Int(int64(globalRand.Int63n(max))), nil
	}
	min, err1 := args[0].AsInt()
	max, err2 := args[1].AsInt()
	if err1 != nil || err2 != nil || max <= min {
		return vm.Int(min), nil
	}
	return vm.Int(min + globalRand.Int63n(max-min)), nil
}

func randFloat(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Float(globalRand.Float64()), nil
}

func randPick(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil || len(*args[0].Arr) == 0 {
		return vm.Null(), errf("rand.pick(array)")
	}
	arr := *args[0].Arr
	return arr[globalRand.Intn(len(arr))], nil
}

func randShuffle(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("rand.shuffle(array)")
	}
	src := *args[0].Arr
	out := make([]vm.Value, len(src))
	copy(out, src)
	globalRand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return vm.Array(out), nil
}

func sortStrings(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Array(nil), nil
	}
	src := *args[0].Arr
	ss := make([]string, len(src))
	for i, v := range src {
		ss[i] = v.AsStr()
	}
	sort.Strings(ss)
	out := make([]vm.Value, len(ss))
	for i, s := range ss {
		out[i] = vm.Str(s)
	}
	return vm.Array(out), nil
}

func sortInts(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Array(nil), nil
	}
	src := *args[0].Arr
	is := make([]int64, 0, len(src))
	for _, v := range src {
		if n, err := v.AsInt(); err == nil {
			is = append(is, n)
		}
	}
	sort.Slice(is, func(i, j int) bool { return is[i] < is[j] })
	out := make([]vm.Value, len(is))
	for i, n := range is {
		out[i] = vm.Int(n)
	}
	return vm.Array(out), nil
}

// sort.sort(array) — ints and strings mixed by string key; pure ints numeric
func sortGeneric(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Array(nil), nil
	}
	src := *args[0].Arr
	allInt := true
	for _, v := range src {
		if v.Typ != vm.TypeInt {
			allInt = false
			break
		}
	}
	if allInt {
		return sortInts(nil, args)
	}
	return sortStrings(nil, args)
}

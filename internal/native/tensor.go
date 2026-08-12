package native

import (
	"encoding/binary"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"unsafe"

	"groklang/gltk/internal/vm"
)

// ============================================================
//  GrokTorch — Tensor module with automatic differentiation
//  Tensors are GLVM maps: {data: bytes, shape: [int...], grad: bytes?}
//  data/grad are raw float32 little-endian byte sequences.
// ============================================================

// ---- global autograd state ----
var (
	tensorGradEnabled bool
	tensorTape        []tapeEntry
	tensorTapeMu      sync.Mutex
	// Adam optimizer states: map[tensor_id]*adamState
	tensorAdamStates   = map[uintptr]*adamState{}
	tensorAdamStatesMu sync.Mutex
)

type tapeEntry struct {
	backward func()
}

type adamState struct {
	m []float32 // first moment
	v []float32 // second moment
}

// ---- module registration ----

func moduleTensor() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		// creation
		"zeros":  tZeros,
		"ones":   tOnes,
		"randn":  tRandn,
		"rand":   tRand,
		"fill":   tFill,
		"copy":   tCopy,
		// element-wise arithmetic
		"add": tAdd,
		"sub": tSub,
		"mul": tMul,
		"div": tDiv,
		"neg": tNeg,
		// matrix
		"matmul":    tMatmul,
		"transpose": tTranspose,
		"reshape":   tReshape,
		"slice_2d":  tSlice2D,
		// element-wise math
		"exp":  tExp,
		"log":  tLog,
		"sqrt": tSqrt,
		"pow":  tPow,
		// activation
		"relu":        tReLU,
		"gelu":        tGELU,
		"softmax":     tSoftmax,
		"log_softmax": tLogSoftmax,
		// reduction
		"sum":    tSum,
		"mean":   tMean,
		"max":    tMax,
		"argmax": tArgmax,
		"sample": tSample,
		// loss
		"cross_entropy": tCrossEntropy,
		// autograd
		"enable_grad":  tEnableGrad,
		"disable_grad": tDisableGrad,
		"zero_grad":    tZeroGrad,
		"backward":    tBackward,
		"get_grad":    tGetGrad,
		"step_adam":   tStepAdam,
		// utility
		"shape":         tShape,
		"numel":         tNumElements,
		"print_stats":   tPrintStats,
		"to_value":      tToValue,
		"force_gc":      tForceGC,
		// data manipulation
		"embed_lookup": tEmbedLookup,
		"cat":          tCat,
		// attention
		"scaled_dot_product_attention": tSDPA,
		// BPE tokenizer
		"bpe_train":  tBPETrain,
		"bpe_encode": tBPEEncode,
		"bpe_decode": tBPEDecode,
	})
}

// ============================================================
//  Internal helpers
// ============================================================

// tensorData extracts (float32 slice, shape) from a tensor map.
func tensorData(v vm.Value) ([]float32, []int64, error) {
	if v.Typ != vm.TypeMap || v.Map == nil {
		return nil, nil, errf("expected tensor map")
	}
	m := *v.Map
	dataV, ok := m["data"]
	if !ok || dataV.Typ != vm.TypeBytes {
		return nil, nil, errf("tensor missing data")
	}
	shapeV, ok := m["shape"]
	if !ok || shapeV.Typ != vm.TypeArray || shapeV.Arr == nil {
		return nil, nil, errf("tensor missing shape")
	}
	// decode shape
	shape := make([]int64, len(*shapeV.Arr))
	for i, sv := range *shapeV.Arr {
		n, errr := sv.AsInt()
		if errr != nil {
			return nil, nil, errf("invalid shape element")
		}
		shape[i] = n
	}
	// decode float32 data
	raw := dataV.Bytes
	n := len(raw) / 4
	floats := make([]float32, n)
	for i := 0; i < n; i++ {
		floats[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return floats, shape, nil
}

// makeTensor creates a tensor map from float32 data and shape.
func makeTensor(data []float32, shape []int64) vm.Value {
	raw := make([]byte, len(data)*4)
	for i, f := range data {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(f))
	}
	shapeVals := make([]vm.Value, len(shape))
	for i, s := range shape {
		shapeVals[i] = vm.Int(s)
	}
	return vm.MapVal(map[string]vm.Value{
		"data":  vm.Bytes(raw),
		"shape": vm.Array(shapeVals),
	})
}

// totalElements returns product of shape dimensions.
// Scalar (empty shape) has 1 element.
func totalElements(shape []int64) int64 {
	if len(shape) == 0 {
		return 1
	}
	n := int64(1)
	for _, s := range shape {
		n *= s
	}
	return n
}

// argAsTensor converts a vm.Value (tensor map, float, or int) to (data, shape).
func argAsTensor(v vm.Value) ([]float32, []int64, error) {
	d, s, err := tensorData(v)
	if err == nil {
		return d, s, nil
	}
	// treat as 0-d scalar
	f, errf := v.AsFloat()
	if errf != nil {
		return []float32{0}, []int64{}, nil
	}
	return []float32{float32(f)}, []int64{}, nil
}

// broadcastShapes returns the output shape after broadcasting two shapes.
func broadcastShapes(a, b []int64) ([]int64, error) {
	la, lb := len(a), len(b)
	n := la
	if lb > n {
		n = lb
	}
	out := make([]int64, n)
	for i := 1; i <= n; i++ {
		da := int64(1)
		if i <= la {
			da = a[la-i]
		}
		db := int64(1)
		if i <= lb {
			db = b[lb-i]
		}
		if da != db && da != 1 && db != 1 {
			return nil, errf("incompatible shapes for broadcast")
		}
		if da > db {
			out[n-i] = da
		} else {
			out[n-i] = db
		}
	}
	return out, nil
}

func shapeEqual(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// gradData gets/creates the gradient field of a tensor.
func gradData(t vm.Value) []float32 {
	if t.Typ != vm.TypeMap || t.Map == nil {
		return nil
	}
	m := *t.Map
	gv, ok := m["grad"]
	if ok && gv.Typ == vm.TypeBytes {
		raw := gv.Bytes
		n := len(raw) / 4
		f := make([]float32, n)
		for i := 0; i < n; i++ {
			f[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		}
		return f
	}
	return nil
}

// ensureGrad ensures a tensor has a grad field of zeros with matching shape.
func ensureGrad(t vm.Value) []float32 {
	if t.Typ != vm.TypeMap || t.Map == nil {
		return nil
	}
	m := *t.Map
	gv, ok := m["grad"]
	if ok && gv.Typ == vm.TypeBytes {
		raw := gv.Bytes
		n := len(raw) / 4
		f := make([]float32, n)
		for i := 0; i < n; i++ {
			f[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		}
		return f
	}
	// create zero gradient
	_, shape, err := tensorData(t)
	if err != nil {
		return nil
	}
	nel := totalElements(shape)
	f := make([]float32, nel)
	raw := make([]byte, nel*4) // all zeros
	for i := int64(0); i < nel; i++ {
		binary.LittleEndian.PutUint32(raw[i*4:], 0)
	}
	m["grad"] = vm.Bytes(raw)
	return f
}

// setGradData writes gradient data back to tensor map.
func setGradData(t vm.Value, g []float32) {
	if t.Typ != vm.TypeMap || t.Map == nil {
		return
	}
	raw := make([]byte, len(g)*4)
	for i, v := range g {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(v))
	}
	(*t.Map)["grad"] = vm.Bytes(raw)
}

func mustOneArg(args []vm.Value, name string) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tensor.%s expects 1 arg", name)
	}
	return args[0], nil
}

func mustTwoArgs(args []vm.Value, name string) (vm.Value, vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), vm.Null(), errf("tensor.%s expects 2 args", name)
	}
	return args[0], args[1], nil
}

// addTape records a backward op if grad is enabled.
func addTape(entry tapeEntry) {
	if !tensorGradEnabled {
		return
	}
	tensorTapeMu.Lock()
	tensorTape = append(tensorTape, entry)
	tensorTapeMu.Unlock()
}

// ============================================================
//  Creation ops
// ============================================================

func tZeros(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("tensor.zeros(shape)")
	}
	shape := make([]int64, len(*args[0].Arr))
	for i, v := range *args[0].Arr {
		n, err := v.AsInt()
		if err != nil || n < 0 {
			return vm.Null(), errf("invalid shape")
		}
		shape[i] = n
	}
	nel := totalElements(shape)
	return makeTensor(make([]float32, nel), shape), nil
}

func tOnes(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("tensor.ones(shape)")
	}
	shape := make([]int64, len(*args[0].Arr))
	for i, v := range *args[0].Arr {
		n, err := v.AsInt()
		if err != nil || n < 0 {
			return vm.Null(), errf("invalid shape")
		}
		shape[i] = n
	}
	nel := totalElements(shape)
	data := make([]float32, nel)
	for i := range data {
		data[i] = 1.0
	}
	return makeTensor(data, shape), nil
}

func tFill(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.fill(shape, value)")
	}
	if args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("tensor.fill: shape must be array")
	}
	val, err := args[1].AsFloat()
	if err != nil {
		val = 0
	}
	shape := make([]int64, len(*args[0].Arr))
	for i, v := range *args[0].Arr {
		n, e := v.AsInt()
		if e != nil || n < 0 {
			return vm.Null(), errf("invalid shape")
		}
		shape[i] = n
	}
	nel := totalElements(shape)
	data := make([]float32, nel)
	for i := range data {
		data[i] = float32(val)
	}
	return makeTensor(data, shape), nil
}

func tRandn(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("tensor.randn(shape)")
	}
	shape := make([]int64, len(*args[0].Arr))
	for i, v := range *args[0].Arr {
		n, err := v.AsInt()
		if err != nil || n < 0 {
			return vm.Null(), errf("invalid shape")
		}
		shape[i] = n
	}
	nel := totalElements(shape)
	data := make([]float32, nel)
	for i := range data {
		data[i] = float32(rand.NormFloat64())
	}
	return makeTensor(data, shape), nil
}

func tRand(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), errf("tensor.rand(shape)")
	}
	shape := make([]int64, len(*args[0].Arr))
	for i, v := range *args[0].Arr {
		n, err := v.AsInt()
		if err != nil || n < 0 {
			return vm.Null(), errf("invalid shape")
		}
		shape[i] = n
	}
	nel := totalElements(shape)
	data := make([]float32, nel)
	for i := range data {
		data[i] = float32(rand.Float64())
	}
	return makeTensor(data, shape), nil
}

func tCopy(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "copy")
	if err != nil {
		return vm.Null(), err
	}
	d, s, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	out := make([]float32, len(d))
	copy(out, d)
	return makeTensor(out, s), nil
}

// ============================================================
//  Element-wise ops (with autograd)
// ============================================================

func elementWiseBroadcast(aData, bData []float32, aShape, bShape []int64, op fnFloatOp) ([]float32, []int64, error) {
	if len(aShape) == 0 {
		// scalar a, tensor b
		nel := totalElements(bShape)
		out := make([]float32, nel)
		av := aData[0]
		for i := range out {
			out[i] = op(av, bData[i])
		}
		return out, bShape, nil
	}
	if len(bShape) == 0 {
		nel := totalElements(aShape)
		out := make([]float32, nel)
		bv := bData[0]
		for i := range out {
			out[i] = op(aData[i], bv)
		}
		return out, aShape, nil
	}
	if shapeEqual(aShape, bShape) {
		nel := totalElements(aShape)
		out := make([]float32, nel)
		for i := int64(0); i < nel; i++ {
			out[i] = op(aData[i], bData[i])
		}
		return out, aShape, nil
	}
	outShape, err := broadcastShapes(aShape, bShape)
	if err != nil {
		return nil, nil, err
	}
	// full broadcast iteration
	nel := totalElements(outShape)
	out := make([]float32, nel)
	for i := int64(0); i < nel; i++ {
		ai := broadcastIndex(i, outShape, aShape)
		bi := broadcastIndex(i, outShape, bShape)
		out[i] = op(aData[ai], bData[bi])
	}
	return out, outShape, nil
}

func broadcastIndex(flatIdx int64, outShape, idxShape []int64) int64 {
	if len(idxShape) == 0 {
		return 0
	}
	// compute multi-dim index for outShape, then map to idxShape
	rem := flatIdx
	idx := int64(0)
	stride := int64(1)
	for dim := len(outShape) - 1; dim >= 0; dim-- {
		coord := rem % outShape[dim]
		rem /= outShape[dim]
		idxDim := dim - (len(outShape) - len(idxShape))
		if idxDim >= 0 {
			if idxShape[idxDim] == 1 {
				// broadcast: coordinate stays 0
			} else {
				idx += coord * stride
			}
			// advance stride for idxShape
			if idxDim > 0 {
				stride *= idxShape[idxDim]
			} else {
				// first dim stride stays 1
			}
		}
	}
	// simpler: recalculate stride from idxShape
	idx = int64(0)
	stride = int64(1)
	rem2 := flatIdx
	for dim := len(outShape) - 1; dim >= 0; dim-- {
		coord := rem2 % outShape[dim]
		rem2 /= outShape[dim]
		idxDim := dim - (len(outShape) - len(idxShape))
		if idxDim >= 0 && idxDim < len(idxShape) {
			effCoord := coord
			if idxShape[idxDim] == 1 {
				effCoord = 0
			}
			idx += effCoord * stride
		}
		if idxDim > 0 && idxDim-1 < len(idxShape) {
			stride *= idxShape[idxDim-1]
		}
	}
	// better: just compute from end
	idx = 0
	ostride := int64(1)
	for d := 0; d < len(idxShape); d++ {
		od := len(outShape) - len(idxShape) + d
		coord := (flatIdx / outStride(outShape, od+1)) % outShape[od]
		if idxShape[d] == 1 {
			coord = 0
		}
		idx += coord * idxStride(idxShape, d+1)
		_ = ostride
	}
	return idx
}

func outStride(shape []int64, fromDim int) int64 {
	s := int64(1)
	for i := fromDim; i < len(shape); i++ {
		s *= shape[i]
	}
	return s
}

func idxStride(shape []int64, fromDim int) int64 {
	s := int64(1)
	for i := fromDim; i < len(shape); i++ {
		s *= shape[i]
	}
	return s
}

// numpy-style broadcasting
func simpleBroadcast(aData, bData []float32, aShape, bShape []int64) ([]float32, []int64, error) {
	// Same shape — fast path
	if shapeEqual(aShape, bShape) {
		return aData, aShape, nil
	}
	// Compute broadcast shape
	ndim := len(aShape)
	if len(bShape) > ndim {
		ndim = len(bShape)
	}
	bcShape := make([]int64, ndim)
	for i := 0; i < ndim; i++ {
		ai := len(aShape) - 1 - i
		bi := len(bShape) - 1 - i
		di := ndim - 1 - i
		var aDim, bDim int64 = 1, 1
		if ai >= 0 {
			aDim = aShape[ai]
		}
		if bi >= 0 {
			bDim = bShape[bi]
		}
		if aDim == bDim {
			bcShape[di] = aDim
		} else if aDim == 1 {
			bcShape[di] = bDim
		} else if bDim == 1 {
			bcShape[di] = aDim
		} else {
			return nil, nil, errf("broadcast not supported for " + fmtShape(aShape) + " vs " + fmtShape(bShape) + ". Use tensor.reshape first")
		}
	}
	// If broadcast shape equals aShape, no expansion needed for A
	if shapeEqual(bcShape, aShape) {
		return aData, bcShape, nil
	}
	// Actually broadcast: fill output using strides
	nel := totalElements(bcShape)
	out := make([]float32, nel)
	for i := int64(0); i < nel; i++ {
		aIdx := int64(0)
		astride := int64(1)
		rem := i
		for d := ndim - 1; d >= 0; d-- {
			coord := rem % bcShape[d]
			rem /= bcShape[d]
			ad := len(aShape) - 1 - (ndim - 1 - d)
			if ad >= 0 {
				if aShape[ad] == bcShape[d] {
					aIdx += coord * astride
				}
				astride *= aShape[ad]
			}
		}
		out[i] = aData[aIdx]
	}
	return out, bcShape, nil
}

func fmtShape(s []int64) string {
	if len(s) == 0 {
		return "[]"
	}
	out := "["
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += fmtInt(v)
	}
	return out + "]"
}

func fmtInt(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

type fnFloatOp func(a, b float32) float32
type fnFloatSingle func(a float32) float32

func addOp(a, b float32) float32 { return a + b }
func subOp(a, b float32) float32 { return a - b }
func mulOp(a, b float32) float32 { return a * b }
func divOp(a, b float32) float32 { return a / b }

func execElementWise(a vm.Value, b vm.Value, op fnFloatOp, gradA func(gradOut, a, b float32) (float32, float32), name string) (vm.Value, error) {
	aD, aS, err := argAsTensor(a)
	if err != nil {
		return vm.Null(), err
	}
	bD, bS, err := argAsTensor(b)
	if err != nil {
		return vm.Null(), err
	}
	// For now only same-shape or scalar-tensor
	bcastAD, bcastAS, err := simpleBroadcast(aD, bD, aS, bS)
	if err != nil {
		return vm.Null(), err
	}
	bcastBD, bcastBS, err := simpleBroadcast(bD, aD, bS, aS)
	if err != nil {
		return vm.Null(), err
	}
	outShape := bcastAS
	if len(bcastAS) == 0 {
		outShape = bcastBS
	}
	nel := totalElements(outShape)
	if nel == 0 {
		nel = 1
	}
	out := make([]float32, nel)
	for i := int64(0); i < nel; i++ {
		av := bcastAD[i%int64(len(bcastAD))]
		bv := bcastBD[i%int64(len(bcastBD))]
		out[i] = op(av, bv)
	}

	ret := makeTensor(out, outShape)
	if tensorGradEnabled && gradA != nil {
		// capture references for backward
		aRef := a
		bRef := b
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			bGoD := ensureGrad(bRef)
			if aGoD == nil && bGoD == nil {
				return
			}
			nelOut := int64(len(goD))
			for i := int64(0); i < nelOut; i++ {
				goV := goD[i]
				aOrig := bcastAD[i%int64(len(bcastAD))]
				bOrig := bcastBD[i%int64(len(bcastBD))]
				ga, gb := gradA(goV, aOrig, bOrig)
				if aGoD != nil && len(aGoD) > 0 {
					ai := i % int64(len(aGoD))
					aGoD[ai] += ga
				}
				if bGoD != nil && len(bGoD) > 0 {
					bi := i % int64(len(bGoD))
					bGoD[bi] += gb
				}
			}
			if aGoD != nil && len(aGoD) > 0 {
				setGradData(aRef, aGoD)
			}
			if bGoD != nil && len(bGoD) > 0 {
				setGradData(bRef, bGoD)
			}
		}})
	}
	return ret, nil
}

func tAdd(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, b, err := mustTwoArgs(args, "add")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWise(a, b, addOp,
		func(gOut, aV, bV float32) (float32, float32) { return gOut, gOut }, "add")
}

func tSub(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, b, err := mustTwoArgs(args, "sub")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWise(a, b, subOp,
		func(gOut, aV, bV float32) (float32, float32) { return gOut, -gOut }, "sub")
}

func tMul(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, b, err := mustTwoArgs(args, "mul")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWise(a, b, mulOp,
		func(gOut, aV, bV float32) (float32, float32) { return gOut * bV, gOut * aV }, "mul")
}

func tDiv(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, b, err := mustTwoArgs(args, "div")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWise(a, b, divOp,
		func(gOut, aV, bV float32) (float32, float32) { return gOut / bV, -gOut * aV / (bV * bV) }, "div")
}

func tNeg(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "neg")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	out := make([]float32, len(aD))
	for i, v := range aD {
		out[i] = -v
	}
	ret := makeTensor(out, aS)
	if tensorGradEnabled {
		aRef := a
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := int64(0); i < int64(len(goD)) && i < int64(len(aGoD)); i++ {
				aGoD[i] += -goD[i]
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

// ============================================================
//  Matmul (2D matrix multiply with autograd)
// ============================================================

func tMatmul(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, b, err := mustTwoArgs(args, "matmul")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	bD, bS, err := tensorData(b)
	if err != nil {
		return vm.Null(), err
	}
	if len(aS) != 2 || len(bS) != 2 {
		return vm.Null(), errf("matmul requires 2D tensors, got shapes " + fmtShape(aS) + " and " + fmtShape(bS))
	}
	m, k := aS[0], aS[1]
	k2, n := bS[0], bS[1]
	if k != k2 {
		return vm.Null(), errf("matmul shape mismatch: [M,K]@[K,N], got K=" + fmtInt(k) + " vs " + fmtInt(k2))
	}
	out := make([]float32, m*n)
	for i := int64(0); i < m; i++ {
		for j := int64(0); j < n; j++ {
			sum := float32(0)
			for kk := int64(0); kk < k; kk++ {
				sum += aD[i*k+kk] * bD[kk*n+j]
			}
			out[i*n+j] = sum
		}
	}
	ret := makeTensor(out, []int64{m, n})
	if tensorGradEnabled {
		aRef := a
		bRef := b
		outRef := ret
		mCp, kCp, nCp := m, k, n
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			// grad_a = grad_out @ b^T  [M,N] @ [N,K] -> [M,K]
			aGoD := ensureGrad(aRef)
			if aGoD != nil {
				for i := int64(0); i < mCp; i++ {
					for j := int64(0); j < kCp; j++ {
						sum := float32(0)
						for nn := int64(0); nn < nCp; nn++ {
							sum += goD[i*nCp+nn] * bD[j*nCp+nn]
						}
						aGoD[i*kCp+j] += sum
					}
				}
				setGradData(aRef, aGoD)
			}
			// grad_b = a^T @ grad_out  [K,M] @ [M,N] -> [K,N]
			bGoD := ensureGrad(bRef)
			if bGoD != nil {
				for i := int64(0); i < kCp; i++ {
					for j := int64(0); j < nCp; j++ {
						sum := float32(0)
						for mm := int64(0); mm < mCp; mm++ {
							sum += aD[mm*kCp+i] * goD[mm*nCp+j]
						}
						bGoD[i*nCp+j] += sum
					}
				}
				setGradData(bRef, bGoD)
			}
		}})
	}
	return ret, nil
}

// ============================================================
//  Transpose & Reshape & Slice
// ============================================================

func tTranspose(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "transpose")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aS) != 2 {
		return vm.Null(), errf("transpose requires 2D tensor")
	}
	m, n := aS[0], aS[1]
	out := make([]float32, m*n)
	for i := int64(0); i < m; i++ {
		for j := int64(0); j < n; j++ {
			out[j*m+i] = aD[i*n+j]
		}
	}
	ret := makeTensor(out, []int64{n, m})
	if tensorGradEnabled {
		aRef, mC, nC := a, m, n
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := int64(0); i < mC; i++ {
				for j := int64(0); j < nC; j++ {
					aGoD[i*nC+j] += goD[j*mC+i]
				}
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func tReshape(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.reshape(tensor, shape)")
	}
	aD, aS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if args[1].Typ != vm.TypeArray || args[1].Arr == nil {
		return vm.Null(), errf("tensor.reshape: shape must be array")
	}
	newShape := make([]int64, len(*args[1].Arr))
	for i, v := range *args[1].Arr {
		n, e := v.AsInt()
		if e != nil {
			return vm.Null(), errf("invalid shape")
		}
		newShape[i] = n
	}
	if totalElements(aS) != totalElements(newShape) {
		return vm.Null(), errf("reshape: total elements must match")
	}
	out := make([]float32, len(aD))
	copy(out, aD)
	ret := makeTensor(out, newShape)
	if tensorGradEnabled {
		aRef := args[0]
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := 0; i < len(goD) && i < len(aGoD); i++ {
				aGoD[i] += goD[i]
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func tSlice2D(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 5 {
		return vm.Null(), errf("tensor.slice_2d(tensor, rStart, rEnd, cStart, cEnd)")
	}
	aD, aS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if len(aS) != 2 {
		return vm.Null(), errf("slice_2d requires 2D tensor")
	}
	r1, _ := args[1].AsInt()
	r2, _ := args[2].AsInt()
	c1, _ := args[3].AsInt()
	c2, _ := args[4].AsInt()
	if r1 < 0 {
		r1 = aS[0] + r1
	}
	if r2 < 0 {
		r2 = aS[0] + r2
	}
	if c1 < 0 {
		c1 = aS[1] + c1
	}
	if c2 < 0 {
		c2 = aS[1] + c2
	}
	if r1 < 0 {
		r1 = 0
	}
	if r2 > aS[0] {
		r2 = aS[0]
	}
	if c1 < 0 {
		c1 = 0
	}
	if c2 > aS[1] {
		c2 = aS[1]
	}
	nR, nC := r2-r1, c2-c1
	out := make([]float32, nR*nC)
	for i := int64(0); i < nR; i++ {
		for j := int64(0); j < nC; j++ {
			out[i*nC+j] = aD[(r1+i)*aS[1]+(c1+j)]
		}
	}
	ret := makeTensor(out, []int64{nR, nC})
	if tensorGradEnabled {
		aRef := args[0]
		outRef := ret
		r1C, r2C, c1C, c2C, colsC := r1, r2, c1, c2, aS[1]
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			nr := r2C - r1C
			nc := c2C - c1C
			for i := int64(0); i < nr; i++ {
				for j := int64(0); j < nc; j++ {
					aGoD[(r1C+i)*colsC+(c1C+j)] += goD[i*nc+j]
				}
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

// ============================================================
//  Element-wise math functions (with autograd)
// ============================================================

func execElementWiseSingle(a vm.Value, op fnFloatSingle, gradOp fnFloatSingle, name string) (vm.Value, error) {
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	out := make([]float32, len(aD))
	for i, v := range aD {
		out[i] = op(v)
	}
	ret := makeTensor(out, aS)
	if tensorGradEnabled && gradOp != nil {
		aRef := a
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := 0; i < len(goD) && i < len(aGoD); i++ {
				aGoD[i] += goD[i] * gradOp(aD[i])
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func tExp(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "exp")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWiseSingle(a,
		func(x float32) float32 { return float32(math.Exp(float64(x))) },
		func(x float32) float32 { return float32(math.Exp(float64(x))) },
		"exp")
}

func tLog(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "log")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWiseSingle(a,
		func(x float32) float32 { return float32(math.Log(float64(x))) },
		func(x float32) float32 { return float32(1.0 / float64(x)) },
		"log")
}

func tSqrt(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "sqrt")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWiseSingle(a,
		func(x float32) float32 { return float32(math.Sqrt(float64(x))) },
		func(x float32) float32 { return float32(0.5 / math.Sqrt(float64(x))) },
		"sqrt")
}

func tPow(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.pow(tensor, exponent)")
	}
	exp, err := args[1].AsFloat()
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	out := make([]float32, len(aD))
	for i, v := range aD {
		out[i] = float32(math.Pow(float64(v), exp))
	}
	ret := makeTensor(out, aS)
	if tensorGradEnabled {
		aRef := args[0]
		outRef := ret
		e := exp
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := 0; i < len(goD) && i < len(aGoD); i++ {
				aGoD[i] += goD[i] * float32(float64(e)*math.Pow(float64(aD[i]), float64(e)-1))
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

// ============================================================
//  Activations
// ============================================================

func tReLU(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "relu")
	if err != nil {
		return vm.Null(), err
	}
	return execElementWiseSingle(a, func(x float32) float32 {
		if x > 0 {
			return x
		}
		return 0
	}, func(x float32) float32 {
		if x > 0 {
			return 1
		}
		return 0
	}, "relu")
}

func tGELU(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "gelu")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	out := make([]float32, len(aD))
	for i, x := range aD {
		out[i] = geluFloat(x)
	}
	ret := makeTensor(out, aS)
	if tensorGradEnabled {
		aRef := a
		outRef := ret
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := 0; i < len(goD) && i < len(aGoD); i++ {
				aGoD[i] += goD[i] * geluGradFloat(aD[i])
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func geluFloat(x float32) float32 {
	xf := float64(x)
	return float32(0.5 * xf * (1 + math.Tanh(math.Sqrt(2/math.Pi)*(xf+0.044715*xf*xf*xf))))
}

func geluGradFloat(x float32) float32 {
	// approximate GELU gradient
	xf := float64(x)
	c := math.Sqrt(2.0 / math.Pi)
	x3 := xf * xf * xf
	inner := c * (xf + 0.044715*x3)
	tanhInner := math.Tanh(inner)
	sech2 := 1 - tanhInner*tanhInner
	return float32(0.5*(1+tanhInner) + 0.5*xf*sech2*c*(1+3*0.044715*xf*xf))
}

// ============================================================
//  Softmax (along last dim)
// ============================================================

func tSoftmax(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "softmax")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aS) != 2 {
		return vm.Null(), errf("softmax currently supports 2D tensors")
	}
	rows, cols := aS[0], aS[1]
	out := make([]float32, rows*cols)
	for i := int64(0); i < rows; i++ {
		// find max for numerical stability
		maxVal := aD[i*cols]
		for j := int64(1); j < cols; j++ {
			if aD[i*cols+j] > maxVal {
				maxVal = aD[i*cols+j]
			}
		}
		sum := float32(0)
		for j := int64(0); j < cols; j++ {
			v := float32(math.Exp(float64(aD[i*cols+j] - maxVal)))
			out[i*cols+j] = v
			sum += v
		}
		for j := int64(0); j < cols; j++ {
			out[i*cols+j] /= sum
		}
	}
	ret := makeTensor(out, aS)
	if tensorGradEnabled {
		aRef := a
		outRef := ret
		r, c := rows, cols
		addTape(tapeEntry{backward: func() {
			goD := gradData(outRef)
			if goD == nil {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			for i := int64(0); i < r; i++ {
				// grad_input = softmax_out * (grad_output - sum(grad_output * softmax_out))
				dot := float32(0)
				for j := int64(0); j < c; j++ {
					dot += goD[i*c+j] * out[i*c+j]
				}
				for j := int64(0); j < c; j++ {
					aGoD[i*c+j] += out[i*c+j] * (goD[i*c+j] - dot)
				}
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func tLogSoftmax(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "log_softmax")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aS) != 2 {
		return vm.Null(), errf("log_softmax currently supports 2D tensors")
	}
	rows, cols := aS[0], aS[1]
	out := make([]float32, rows*cols)
	for i := int64(0); i < rows; i++ {
		maxVal := aD[i*cols]
		for j := int64(1); j < cols; j++ {
			if aD[i*cols+j] > maxVal {
				maxVal = aD[i*cols+j]
			}
		}
		sum := float32(0)
		for j := int64(0); j < cols; j++ {
			sum += float32(math.Exp(float64(aD[i*cols+j] - maxVal)))
		}
		logSum := maxVal + float32(math.Log(float64(sum)))
		for j := int64(0); j < cols; j++ {
			out[i*cols+j] = aD[i*cols+j] - logSum
		}
	}
	return makeTensor(out, aS), nil
}

// ============================================================
//  Reduction ops
// ============================================================

func tSum(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "sum")
	if err != nil {
		return vm.Null(), err
	}
	aD, aS, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	sum := float32(0)
	for _, v := range aD {
		sum += v
	}
	ret := makeTensor([]float32{sum}, []int64{})
	if tensorGradEnabled {
		aRef := a
		nel := totalElements(aS)
		addTape(tapeEntry{backward: func() {
			goD := gradData(ret)
			if goD == nil || len(goD) == 0 {
				return
			}
			aGoD := ensureGrad(aRef)
			if aGoD == nil {
				return
			}
			g := goD[0]
			for i := int64(0); i < nel && i < int64(len(aGoD)); i++ {
				aGoD[i] += g
			}
			setGradData(aRef, aGoD)
		}})
	}
	return ret, nil
}

func tMean(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "mean")
	if err != nil {
		return vm.Null(), err
	}
	aD, _, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aD) == 0 {
		return vm.Float(0), nil
	}
	sum := float32(0)
	for _, v := range aD {
		sum += v
	}
	return vm.Float(float64(sum) / float64(len(aD))), nil
}

func tMax(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "max")
	if err != nil {
		return vm.Null(), err
	}
	aD, _, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aD) == 0 {
		return vm.Float(0), nil
	}
	m := aD[0]
	for _, v := range aD[1:] {
		if v > m {
			m = v
		}
	}
	return vm.Float(float64(m)), nil
}

func tArgmax(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "argmax")
	if err != nil {
		return vm.Null(), err
	}
	aD, _, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(aD) == 0 {
		return vm.Int(-1), nil
	}
	best, bestI := aD[0], int64(0)
	for i, v := range aD[1:] {
		if v > best {
			best = v
			bestI = int64(i + 1)
		}
	}
	return vm.Int(bestI), nil
}

func tSample(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	// tensor.sample(logits, temperature) -> sampled index
	if len(args) < 2 {
		return vm.Null(), errf("tensor.sample(logits, temperature)")
	}
	aD, _, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	temp, _ := args[1].AsFloat()
	if temp <= 0 {
		temp = 1.0
	}
	// Softmax with temperature
	n := len(aD)
	if n == 0 {
		return vm.Int(0), nil
	}
	// Find max for numerical stability
	maxV := aD[0]
	for _, v := range aD[1:] {
		if v > maxV {
			maxV = v
		}
	}
	sum := float32(0.0)
	probs := make([]float32, n)
	for i, v := range aD {
		p := float32(math.Exp(float64(v-maxV) / temp))
		probs[i] = p
		sum += p
	}
	// Sample
	r := float32(rand.Float64()) * sum
	cum := float32(0.0)
	for i, p := range probs {
		cum += p
		if r <= cum {
			return vm.Int(int64(i)), nil
		}
	}
	return vm.Int(int64(n - 1)), nil
}

// ============================================================
//  Cross-entropy loss (logits [B,V], targets [B] as int tensor)
// ============================================================

func tCrossEntropy(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.cross_entropy(logits, targets)")
	}
	logD, logS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if len(logS) != 2 {
		return vm.Null(), errf("cross_entropy: logits must be 2D [batch, vocab]")
	}
	batch, vocab := logS[0], logS[1]

	// targets as flat array of indices (can be tensor or array of ints)
	var targets []int64
	if args[1].Typ == vm.TypeMap {
		tD, tS, err2 := tensorData(args[1])
		if err2 == nil && len(tS) == 1 && tS[0] == batch {
			targets = make([]int64, len(tD))
			for i, v := range tD {
				targets[i] = int64(v)
			}
		}
	}
	if targets == nil && args[1].Typ == vm.TypeArray && args[1].Arr != nil {
		targets = make([]int64, len(*args[1].Arr))
		for i, v := range *args[1].Arr {
			n, e := v.AsInt()
			if e != nil {
				return vm.Null(), errf("cross_entropy: invalid target")
			}
			targets[i] = n
		}
	}
	if targets == nil {
		// try scalar target for single example
		n, err2 := args[1].AsInt()
		if err2 != nil {
			return vm.Null(), errf("cross_entropy: targets must be array or tensor")
		}
		targets = []int64{n}
	}

	// compute softmax for each row and NLL
	totalLoss := float32(0)
	softmaxOut := make([]float32, batch*vocab)
	for i := int64(0); i < batch; i++ {
		// find max
		maxV := logD[i*vocab]
		for j := int64(1); j < vocab; j++ {
			if logD[i*vocab+j] > maxV {
				maxV = logD[i*vocab+j]
			}
		}
		sum := float32(0)
		for j := int64(0); j < vocab; j++ {
			v := float32(math.Exp(float64(logD[i*vocab+j] - maxV)))
			softmaxOut[i*vocab+j] = v
			sum += v
		}
		for j := int64(0); j < vocab; j++ {
			softmaxOut[i*vocab+j] /= sum
		}
		// NLL
		ti := targets[i%int64(len(targets))]
		if ti >= 0 && ti < vocab {
			totalLoss += float32(-math.Log(float64(softmaxOut[i*vocab+ti]) + 1e-12))
		}
	}
	avgLoss := totalLoss / float32(batch)
	ret := makeTensor([]float32{avgLoss}, []int64{})
	if tensorGradEnabled {
		logRef := args[0]
		retRef := ret
		bC, vC := batch, vocab
		tC := targets
		sO := softmaxOut
		addTape(tapeEntry{backward: func() {
			goD := gradData(retRef)
			if goD == nil || len(goD) == 0 {
				return
			}
			scale := goD[0] / float32(bC)
			logGoD := ensureGrad(logRef)
			if logGoD == nil {
				return
			}
			for i := int64(0); i < bC; i++ {
				ti := tC[i%int64(len(tC))]
				for j := int64(0); j < vC; j++ {
					grad := sO[i*vC+j]
					if j == ti {
						grad -= 1
					}
					logGoD[i*vC+j] += scale * grad
				}
			}
			setGradData(logRef, logGoD)
		}})
	}
	return ret, nil
}

// ============================================================
//  Autograd engine
// ============================================================

func tEnableGrad(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	tensorGradEnabled = true
	tensorTapeMu.Lock()
	tensorTape = nil
	tensorTapeMu.Unlock()
	return vm.Null(), nil
}

func tDisableGrad(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	tensorGradEnabled = false
	tensorTapeMu.Lock()
	tensorTape = nil
	tensorTapeMu.Unlock()
	return vm.Null(), nil
}

func tZeroGrad(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 || args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), nil
	}
	for _, p := range *args[0].Arr {
		if p.Typ == vm.TypeMap && p.Map != nil {
			delete(*p.Map, "grad")
		}
	}
	return vm.Null(), nil
}

func tBackward(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tensor.backward(loss_tensor)")
	}
	loss := args[0]
	// Set loss gradient to 1
	g := ensureGrad(loss)
	if g != nil && len(g) > 0 {
		g[0] = 1.0
		setGradData(loss, g)
	}
	// Execute tape in reverse
	tensorTapeMu.Lock()
	tape := make([]tapeEntry, len(tensorTape))
	copy(tape, tensorTape)
	tensorTape = nil
	tensorTapeMu.Unlock()
	for i := len(tape) - 1; i >= 0; i-- {
		tape[i].backward()
	}
	// hint GC to reclaim tape closures & intermediate tensor grads
	runtime.GC()
	return vm.Null(), nil
}

func tGetGrad(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "get_grad")
	if err != nil {
		return vm.Null(), err
	}
	g := gradData(a)
	if g == nil {
		return vm.Null(), nil
	}
	_, s, _ := tensorData(a)
	return makeTensor(g, s), nil
}

// ============================================================
//  Adam optimizer step (in Go for speed)
// ============================================================

func tStepAdam(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	// tensor.step_adam(params_array, lr, beta1, beta2, eps, t)
	if len(args) < 6 {
		return vm.Null(), errf("tensor.step_adam(params, lr, beta1, beta2, eps, t)")
	}
	if args[0].Typ != vm.TypeArray || args[0].Arr == nil {
		return vm.Null(), nil
	}
	lr, _ := args[1].AsFloat()
	beta1, _ := args[2].AsFloat()
	beta2, _ := args[3].AsFloat()
	eps, _ := args[4].AsFloat()
	t, _ := args[5].AsFloat()

	biasCorr1 := 1.0 - math.Pow(beta1, t)
	biasCorr2 := 1.0 - math.Pow(beta2, t)

	for _, p := range *args[0].Arr {
		pD, pS, err := tensorData(p)
		if err != nil {
			continue
		}
		gD := gradData(p)
		if gD == nil {
			continue
		}
		nel := totalElements(pS)
		// get or create adam state
		ptr := uintptr(0)
		if p.Map != nil {
			ptr = uintptr(unsafePointer(p))
		}
		tensorAdamStatesMu.Lock()
		st, ok := tensorAdamStates[ptr]
		if !ok {
			st = &adamState{m: make([]float32, nel), v: make([]float32, nel)}
			tensorAdamStates[ptr] = st
		}
		tensorAdamStatesMu.Unlock()

		for i := int64(0); i < nel && int(i) < len(st.m); i++ {
			g := gD[i]
			st.m[i] = float32(beta1)*st.m[i] + float32(1-beta1)*g
			st.v[i] = float32(beta2)*st.v[i] + float32(1-beta2)*g*g
			mHat := st.m[i] / float32(biasCorr1)
			vHat := st.v[i] / float32(biasCorr2)
			pD[i] -= float32(lr) * mHat / float32(math.Sqrt(float64(vHat))+eps)
		}
		// write back updated params
		raw := make([]byte, nel*4)
		for i := int64(0); i < nel; i++ {
			binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(pD[i]))
		}
		if p.Map != nil {
			(*p.Map)["data"] = vm.Bytes(raw)
		}
	}
	return vm.Null(), nil
}

func unsafePointer(v vm.Value) uintptr {
	// Use map pointer as key for Adam state
	if v.Typ == vm.TypeMap && v.Map != nil {
		return uintptr(unsafe.Pointer(v.Map))
	}
	return 0
}

// ============================================================
//  Utility
// ============================================================

func tShape(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "shape")
	if err != nil {
		return vm.Null(), err
	}
	_, s, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	out := make([]vm.Value, len(s))
	for i, v := range s {
		out[i] = vm.Int(v)
	}
	return vm.Array(out), nil
}

func tNumElements(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "numel")
	if err != nil {
		return vm.Null(), err
	}
	_, s, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	return vm.Int(totalElements(s)), nil
}

func tPrintStats(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "print_stats")
	if err != nil {
		return vm.Null(), err
	}
	name := "<tensor>"
	if len(args) >= 2 {
		name = args[1].AsStr()
	}
	d, s, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(d) == 0 {
		return vm.Null(), nil
	}
	minV, maxV := d[0], d[0]
	sum := float32(0)
	for _, v := range d {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
		sum += v
	}
	meanV := sum / float32(len(d))
	// variance
	varSum := float32(0)
	for _, v := range d {
		dv := v - meanV
		varSum += dv * dv
	}
	stdV := float32(math.Sqrt(float64(varSum) / float64(len(d))))

	// Return stats as string
	info := name + " " + fmtShape(s) + " min=" + fmtFloat(float64(minV)) + " max=" + fmtFloat(float64(maxV)) + " mean=" + fmtFloat(float64(meanV)) + " std=" + fmtFloat(float64(stdV))
	return vm.Str(info), nil
}

func tToValue(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	a, err := mustOneArg(args, "to_value")
	if err != nil {
		return vm.Null(), err
	}
	// Handle non-tensor values directly (e.g., float from tMean)
	if a.Typ == vm.TypeFloat {
		return a, nil
	}
	if a.Typ == vm.TypeInt {
		return vm.Float(float64(a.I)), nil
	}
	d, _, err := tensorData(a)
	if err != nil {
		return vm.Null(), err
	}
	if len(d) == 0 {
		return vm.Float(0), nil
	}
	return vm.Float(float64(d[0])), nil
}

func tForceGC(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	runtime.GC()
	return vm.Null(), nil
}

// ============================================================
//  Data manipulation: embed_lookup and cat
// ============================================================

// tEmbedLookup: gathers rows from weight matrix given indices array.
// tensor.embed_lookup(weight [V,D], indices [N] as array of ints) -> [N,D]
func tEmbedLookup(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.embed_lookup(weight, indices)")
	}
	wD, wS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	if len(wS) != 2 {
		return vm.Null(), errf("embed_lookup: weight must be 2D [vocab, dim]")
	}
	vocab, dim := wS[0], wS[1]

	// Parse indices (array of ints)
	var indices []int64
	if args[1].Typ == vm.TypeArray && args[1].Arr != nil {
		for _, v := range *args[1].Arr {
			n, e := v.AsInt()
			if e != nil {
				return vm.Null(), errf("embed_lookup: invalid index")
			}
			indices = append(indices, n)
		}
	}
	if indices == nil {
		// try single int
		n, e := args[1].AsInt()
		if e != nil {
			return vm.Null(), errf("embed_lookup: indices must be array or int")
		}
		indices = []int64{n}
	}

	n := int64(len(indices))
	out := make([]float32, n*dim)
	for i := int64(0); i < n; i++ {
		idx := indices[i]
		if idx < 0 {
			idx = vocab + idx
		}
		if idx < 0 || idx >= vocab {
			idx = 0 // clamp for safety
		}
		copy(out[i*dim:(i+1)*dim], wD[idx*dim:(idx+1)*dim])
	}
	ret := makeTensor(out, []int64{n, dim})

	if tensorGradEnabled {
		weightRef := args[0]
		indicesC := make([]int64, len(indices))
		copy(indicesC, indices)
		nC, dimC, vocabC := n, dim, vocab
		addTape(tapeEntry{backward: func() {
			goD := gradData(ret)
			if goD == nil {
				return
			}
			wGoD := ensureGrad(weightRef)
			if wGoD == nil {
				return
			}
			for i := int64(0); i < nC; i++ {
				idx := indicesC[i]
				if idx < 0 || idx >= vocabC {
					continue
				}
				for j := int64(0); j < dimC; j++ {
					wGoD[idx*dimC+j] += goD[i*dimC+j]
				}
			}
			setGradData(weightRef, wGoD)
		}})
	}
	return ret, nil
}

// tCat: concatenates tensors along dim 0.
// tensor.cat(tensors_array) -> concatenated tensor
func tCat(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 1 {
		return vm.Null(), errf("tensor.cat(tensors_array)")
	}
	if args[0].Typ != vm.TypeArray || args[0].Arr == nil || len(*args[0].Arr) == 0 {
		return vm.Null(), errf("tensor.cat: expects non-empty array of tensors")
	}
	tensors := *args[0].Arr

	// Validate all tensors have same shape except dim 0
	var totalRows int64
	var nCols int64
	var allData [][]float32
	var allShapes [][]int64

	for i, t := range tensors {
		d, s, err := tensorData(t)
		if err != nil {
			return vm.Null(), errf("tensor.cat: element " + fmtInt(int64(i)) + " is not a tensor")
		}
		if len(s) == 0 {
			return vm.Null(), errf("tensor.cat: scalar tensors not supported")
		}
		if i == 0 {
			nCols = s[len(s)-1]
		}
		// Check trailing dims match
		prodOther := int64(1)
		for _, sd := range s[:len(s)-1] {
			prodOther *= sd
		}
		if s[len(s)-1] != nCols {
			return vm.Null(), errf("tensor.cat: last dimension mismatch")
		}
		totalRows += prodOther
		allData = append(allData, d)
		allShapes = append(allShapes, s)
	}

	// Flatten all and concatenate
	out := make([]float32, totalRows*nCols)
	offset := int64(0)
	for _, d := range allData {
		copy(out[offset:offset+int64(len(d))], d)
		offset += int64(len(d))
	}

	// Output shape: sum of dim0, rest same
	outShape := make([]int64, len(allShapes[0]))
	outShape[0] = totalRows
	for d := 1; d < len(allShapes[0]); d++ {
		outShape[d] = allShapes[0][d]
	}

	ret := makeTensor(out, outShape)

	if tensorGradEnabled {
		// Save refs for backward
		tensorRefs := make([]vm.Value, len(tensors))
		copy(tensorRefs, tensors)
		offsets := make([]int64, len(tensors))
		off := int64(0)
		for i, d := range allData {
			offsets[i] = off
			off += int64(len(d))
		}
		addTape(tapeEntry{backward: func() {
			goD := gradData(ret)
			if goD == nil {
				return
			}
			for i, ref := range tensorRefs {
				g := ensureGrad(ref)
				if g == nil || len(allData[i]) == 0 {
					continue
				}
				for j := 0; j < len(allData[i]) && int(offsets[i])+j < len(goD); j++ {
					g[j] += goD[int(offsets[i])+j]
				}
				setGradData(ref, g)
			}
		}})
	}
	return ret, nil
}

func fmtFloat(f float64) string {
	// Simple float formatting
	n := int64(f * 1000000)
	neg := n < 0
	if neg {
		n = -n
	}
	intPart := n / 1000000
	fracPart := n % 1000000
	s := fmtInt(intPart)
	if fracPart > 0 {
		s += "."
		fs := fmtInt(fracPart)
		// pad with leading zeros
		for len(fs) < 6 {
			fs = "0" + fs
		}
		// trim trailing zeros
		for len(fs) > 0 && fs[len(fs)-1] == '0' {
			fs = fs[:len(fs)-1]
		}
		if len(fs) > 0 {
			s += fs
		}
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ============================================================
//  Scaled Dot-Product Attention with causal mask
// ============================================================

// tSDPA: tensor.scaled_dot_product_attention(q, k, v)
// All are [seq, d_k] where q.shape == k.shape == v.shape.
// Returns [seq, d_k] with causal masking (position i attends to positions ≤ i).
func tSDPA(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 3 {
		return vm.Null(), errf("tensor.scaled_dot_product_attention(q, k, v)")
	}
	qD, qS, err := tensorData(args[0])
	if err != nil {
		return vm.Null(), err
	}
	kD, kS, err := tensorData(args[1])
	if err != nil {
		return vm.Null(), err
	}
	vD, vS, err := tensorData(args[2])
	if err != nil {
		return vm.Null(), err
	}
	if len(qS) != 2 || len(kS) != 2 || len(vS) != 2 {
		return vm.Null(), errf("sdpa: all inputs must be 2D")
	}
	seq, dk := qS[0], qS[1]
	if kS[0] != seq || kS[1] != dk {
		return vm.Null(), errf("sdpa: q and k must have same shape")
	}
	if vS[0] != seq || vS[1] != dk {
		return vm.Null(), errf("sdpa: v must have same shape as q, k")
	}

	scale := float32(1.0) / float32(math.Sqrt(float64(dk)))

	// Compute scores = Q @ K^T / sqrt(dk), then apply causal mask, then softmax
	scores := make([]float32, seq*seq)
	for i := int64(0); i < seq; i++ {
		for j := int64(0); j < seq; j++ {
			if j > i {
				scores[i*seq+j] = float32(math.Inf(-1)) // causal mask
			} else {
				dot := float32(0)
				for d := int64(0); d < dk; d++ {
					dot += qD[i*dk+d] * kD[j*dk+d]
				}
				scores[i*seq+j] = dot * scale
			}
		}
	}

	// Softmax per row
	for i := int64(0); i < seq; i++ {
		// Find max for numerical stability
		maxV := scores[i*seq]
		for j := int64(1); j < seq; j++ {
			if scores[i*seq+j] > maxV {
				maxV = scores[i*seq+j]
			}
		}
		sum := float32(0)
		for j := int64(0); j < seq; j++ {
			scores[i*seq+j] = float32(math.Exp(float64(scores[i*seq+j] - maxV)))
			sum += scores[i*seq+j]
		}
		for j := int64(0); j < seq; j++ {
			scores[i*seq+j] /= sum
		}
	}

	// Output = attn @ V
	out := make([]float32, seq*dk)
	for i := int64(0); i < seq; i++ {
		for d := int64(0); d < dk; d++ {
			sum := float32(0)
			for j := int64(0); j < seq; j++ {
				sum += scores[i*seq+j] * vD[j*dk+d]
			}
			out[i*dk+d] = sum
		}
	}

	ret := makeTensor(out, []int64{seq, dk})

	if tensorGradEnabled {
		qRef, kRef, vRef := args[0], args[1], args[2]
		attnWeights := make([]float32, seq*seq)
		copy(attnWeights, scores)
		seqC, dkC := seq, dk
		addTape(tapeEntry{backward: func() {
			goD := gradData(ret)
			if goD == nil {
				return
			}
			// grad_V = attn^T @ grad_out  [seq, seq] @ [seq, dk] -> [seq, dk]
			vGoD := ensureGrad(vRef)
			if vGoD != nil {
				for i := int64(0); i < seqC; i++ {
					for d := int64(0); d < dkC; d++ {
						sum := float32(0)
						for j := int64(0); j < seqC; j++ {
							sum += attnWeights[j*seqC+i] * goD[j*dkC+d]
						}
						vGoD[i*dkC+d] += sum
					}
				}
				setGradData(vRef, vGoD)
			}
			// grad_attn = grad_out @ V^T  [seq, dk] @ [dk, seq] -> [seq, seq]
			gradAttn := make([]float32, seqC*seqC)
			for i := int64(0); i < seqC; i++ {
				for j := int64(0); j < seqC; j++ {
					sum := float32(0)
					for d := int64(0); d < dkC; d++ {
						sum += goD[i*dkC+d] * vD[j*dkC+d]
					}
					gradAttn[i*seqC+j] = sum
				}
			}
			// Apply causal mask to grad_attn
			for i := int64(0); i < seqC; i++ {
				for j := int64(0); j < seqC; j++ {
					if j > i {
						gradAttn[i*seqC+j] = 0
					}
				}
			}
			// grad for scores through softmax: s * (grad_attn - sum(grad_attn * s))
			// Actually for full softmax Jacobian: d_s = s * (g - dot(g, s))
			for i := int64(0); i < seqC; i++ {
				dot := float32(0)
				for j := int64(0); j < seqC; j++ {
					dot += gradAttn[i*seqC+j] * attnWeights[i*seqC+j]
				}
				for j := int64(0); j < seqC; j++ {
					gradAttn[i*seqC+j] = attnWeights[i*seqC+j] * (gradAttn[i*seqC+j] - dot)
				}
			}
			// grad_Q = grad_scores @ K / scale  [seq, seq] @ [seq, dk] -> [seq, dk]
			qGoD := ensureGrad(qRef)
			if qGoD != nil {
				for i := int64(0); i < seqC; i++ {
					for d := int64(0); d < dkC; d++ {
						sum := float32(0)
						for j := int64(0); j < seqC; j++ {
							sum += gradAttn[i*seqC+j] * kD[j*dkC+d]
						}
						qGoD[i*dkC+d] += sum * scale
					}
				}
				setGradData(qRef, qGoD)
			}
			// grad_K = grad_scores^T @ Q / scale  [seq, seq] @ [seq, dk] -> [seq, dk]
			kGoD := ensureGrad(kRef)
			if kGoD != nil {
				for i := int64(0); i < seqC; i++ {
					for d := int64(0); d < dkC; d++ {
						sum := float32(0)
						for j := int64(0); j < seqC; j++ {
							sum += gradAttn[j*seqC+i] * qD[j*dkC+d]
						}
						kGoD[i*dkC+d] += sum * scale
					}
				}
				setGradData(kRef, kGoD)
			}
		}})
	}
	return ret, nil
}

// ============================================================
//  BPE Tokenizer
// ============================================================

// tBPETrain: tensor.bpe_train(text, vocab_size, max_chars?) -> [[a,b],[c,d],...]
// Trains byte-pair encoding on first max_chars of text (default: all).
// merge_rules[i] = [token_a, token_b] means merging them creates token ID (256 + i).
func tBPETrain(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.bpe_train(text, vocab_size [, max_chars])")
	}
	text := []byte(args[0].AsStr())
	vs, err := args[1].AsInt()
	if err != nil {
		return vm.Null(), errf("vocab_size: %v", err)
	}
	vocabSize := int(vs)
	if vocabSize <= 256 || vocabSize > 32768 {
		return vm.Null(), errf("vocab_size must be 257..32768")
	}

	// Optional max_chars
	maxChars := len(text)
	if len(args) >= 3 {
		mc, err := args[2].AsInt()
		if err == nil && int(mc) < maxChars {
			maxChars = int(mc)
		}
	}
	if maxChars > len(text) {
		maxChars = len(text)
	}
	text = text[:maxChars]

	numMerges := vocabSize - 256
	if numMerges <= 0 {
		return vm.Array(nil), nil
	}

	// Convert text to int tokens
	tokens := make([]int, len(text))
	for i, b := range text {
		tokens[i] = int(b)
	}

	merges := make([]vm.Value, 0, numMerges)

	for merge := 0; merge < numMerges; merge++ {
		// Count adjacent pairs
		pairCounts := make(map[[2]int]int, len(tokens)/2)
		for i := 0; i < len(tokens)-1; i++ {
			pair := [2]int{tokens[i], tokens[i+1]}
			pairCounts[pair]++
		}

		// Find most frequent pair
		var bestPair [2]int
		bestCount := 0
		for pair, count := range pairCounts {
			if count > bestCount || (count == bestCount && (pair[0] < bestPair[0] || (pair[0] == bestPair[0] && pair[1] < bestPair[1]))) {
				bestCount = count
				bestPair = pair
			}
		}

		if bestCount < 2 {
			break
		}

		newID := 256 + merge

		// Apply merge: replace all occurrences of bestPair with newID
		newTokens := make([]int, 0, len(tokens))
		i := 0
		for i < len(tokens) {
			if i < len(tokens)-1 && tokens[i] == bestPair[0] && tokens[i+1] == bestPair[1] {
				newTokens = append(newTokens, newID)
				i += 2
			} else {
				newTokens = append(newTokens, tokens[i])
				i++
			}
		}
		tokens = newTokens

		merges = append(merges, vm.Array([]vm.Value{vm.Int(int64(bestPair[0])), vm.Int(int64(bestPair[1]))}))
	}

	return vm.Array(merges), nil
}

// tBPEEncode: tensor.bpe_encode(text, merge_rules, max_chars?) -> [token_id, ...]
// Encodes text into BPE token IDs using pre-trained merge rules.
func tBPEEncode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.bpe_encode(text, merge_rules [, max_chars])")
	}
	text := []byte(args[0].AsStr())

	// Optional max_chars
	maxChars := len(text)
	if len(args) >= 3 {
		mc, err := args[2].AsInt()
		if err == nil && int(mc) < maxChars {
			maxChars = int(mc)
		}
	}
	if maxChars > len(text) {
		maxChars = len(text)
	}
	text = text[:maxChars]

	// Parse merge rules
	rulesV := args[1]
	if rulesV.Typ != vm.TypeArray || rulesV.Arr == nil {
		return vm.Null(), errf("merge_rules must be an array")
	}
	rules := *rulesV.Arr
	numRules := len(rules)

	// Start with byte tokens
	tokens := make([]int, len(text))
	for i, b := range text {
		tokens[i] = int(b)
	}

	// Greedy merge: for each rule in order, merge all occurrences
	for ri := 0; ri < numRules; ri++ {
		rule := rules[ri]
		if rule.Typ != vm.TypeArray || rule.Arr == nil || len(*rule.Arr) < 2 {
			continue
		}
		a64, _ := (*rule.Arr)[0].AsInt()
		b64, _ := (*rule.Arr)[1].AsInt()
		a := int(a64)
		b := int(b64)
		newID := 256 + ri

		merged := make([]int, 0, len(tokens))
		i := 0
		for i < len(tokens) {
			if i < len(tokens)-1 && tokens[i] == a && tokens[i+1] == b {
				merged = append(merged, newID)
				i += 2
			} else {
				merged = append(merged, tokens[i])
				i++
			}
		}
		tokens = merged
	}

	result := make([]vm.Value, len(tokens))
	for i, t := range tokens {
		result[i] = vm.Int(int64(t))
	}
	return vm.Array(result), nil
}

// tBPEDecode: tensor.bpe_decode(tokens, merge_rules) -> text
// Decodes BPE token IDs back into text.
func tBPEDecode(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("tensor.bpe_decode(tokens, merge_rules)")
	}

	// Parse tokens
	tokensV := args[0]
	if tokensV.Typ != vm.TypeArray || tokensV.Arr == nil {
		return vm.Null(), errf("tokens must be an array")
	}
	tokens := *tokensV.Arr

	// Parse merge rules
	rulesV := args[1]
	if rulesV.Typ != vm.TypeArray || rulesV.Arr == nil {
		return vm.Null(), errf("merge_rules must be an array")
	}
	rules := *rulesV.Arr

	// Build reverse lookup: token_id -> byte sequence
	type entry struct {
		a, b int
	}
	mergeMap := make(map[int]entry, len(rules))
	for ri, rv := range rules {
		if rv.Typ == vm.TypeArray && rv.Arr != nil && len(*rv.Arr) >= 2 {
			a64, _ := (*rv.Arr)[0].AsInt()
			b64, _ := (*rv.Arr)[1].AsInt()
			mergeMap[256+ri] = entry{int(a64), int(b64)}
		}
	}

	// Recursively expand a token ID to bytes
	var expand func(id int) []byte
	expand = func(id int) []byte {
		if id < 256 {
			return []byte{byte(id)}
		}
		if e, ok := mergeMap[id]; ok {
			return append(expand(e.a), expand(e.b)...)
		}
		return []byte{'?'}
	}

	var result []byte
	for _, tv := range tokens {
		tid, _ := tv.AsInt()
		result = append(result, expand(int(tid))...)
	}
	return vm.Str(string(result)), nil
}

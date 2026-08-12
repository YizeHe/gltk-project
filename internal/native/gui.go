//go:build windows

package native

import (
	"sync"

	"cui/core"
	"cui/layout"
	"cui/widget"

	"groklang/gltk/internal/vm"
)

func moduleGUI() vm.Value {
	return moduleMap(map[string]vm.NativeFunc{
		"available":   guiAvailable,
		"init":        guiInit,
		"window":      guiWindow,
		"show":        guiShow,
		"hide":        guiHide,
		"run":         guiRun,
		"quit":        guiQuit,
		"set_title":   guiSetTitle,
		"label":       guiLabel,
		"button":      guiButton,
		"lineedit":    guiLineEdit,
		"textedit":    guiTextEdit,
		"checkbox":    guiCheckbox,
		"set_text":    guiSetText,
		"get_text":    guiGetText,
		"append_text": guiAppendText,
		"set_bounds":  guiSetBounds,
		"on_click":    guiOnClick,
		"on_close":    guiOnClose,
		"msgbox":      guiMsgBox,
		"vbox":        guiVBox,
		"hbox":        guiHBox,
		"add":         guiAdd,
		"open_file":   guiOpenFile,
		"is_checked":  guiIsChecked,
		"set_checked": guiSetChecked,
		"version":     guiVersion,
	})
}

type guiKind int

const (
	gkWindow guiKind = iota + 1
	gkLabel
	gkButton
	gkLineEdit
	gkTextEdit
	gkCheckbox
	gkLayout
)

type guiObj struct {
	kind guiKind
	win  *core.Window
	lab  *widget.Label
	btn  *widget.Button
	edit *widget.LineEdit
	text *widget.TextEdit
	chk  *widget.CheckBox
	// layout adapters
	vbox *layout.VBoxLayout
	hbox *layout.HBoxLayout
}

var (
	guiOnce sync.Once
	guiApp  *core.App
	guiMu   sync.Mutex
	guiSeq  int64
	guiObjs = map[int64]*guiObj{}
)

func guiEnsure() *core.App {
	guiOnce.Do(func() {
		guiApp = core.NewApp()
	})
	return guiApp
}

func guiPut(o *guiObj) int64 {
	guiMu.Lock()
	defer guiMu.Unlock()
	guiSeq++
	id := guiSeq
	guiObjs[id] = o
	return id
}

func guiGet(id int64) *guiObj {
	guiMu.Lock()
	defer guiMu.Unlock()
	return guiObjs[id]
}

func guiHandle(v vm.Value) (int64, error) {
	if v.Typ != vm.TypeMap || v.Map == nil {
		return 0, errf("gui: expected handle map {type,id}")
	}
	idv, ok := (*v.Map)["id"]
	if !ok {
		return 0, errf("gui: handle missing id")
	}
	return idv.AsInt()
}

func guiMap(kind string, id int64) vm.Value {
	return vm.MapVal(map[string]vm.Value{
		"type": vm.Str(kind),
		"id":   vm.Int(id),
	})
}

func guiAvailable(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Bool(true), nil
}

func guiVersion(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	return vm.Str("cui+gltk-1.0.0"), nil
}

func guiInit(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	guiEnsure()
	return vm.Bool(true), nil
}

func guiWindow(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	app := guiEnsure()
	title := "GrokLang"
	var w, h int64 = 800, 600
	if len(args) >= 1 {
		title = args[0].AsStr()
	}
	if len(args) >= 2 {
		w, _ = args[1].AsInt()
	}
	if len(args) >= 3 {
		h, _ = args[2].AsInt()
	}
	win := app.NewWindow(title, int32(w), int32(h))
	if win == nil {
		return vm.Null(), errf("gui.window: create failed")
	}
	win.CenterScreen()
	id := guiPut(&guiObj{kind: gkWindow, win: win})
	return guiMap("window", id), nil
}

func guiShow(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	o.win.Show()
	return vm.Bool(true), nil
}

func guiHide(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	o.win.Hide()
	return vm.Bool(true), nil
}

func guiRun(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	guiEnsure().Run()
	return vm.Bool(true), nil
}

func guiQuit(_ *vm.VM, _ []vm.Value) (vm.Value, error) {
	if guiApp != nil {
		guiApp.Quit()
	}
	return vm.Bool(true), nil
}

func guiSetTitle(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.set_title(win, title)")
	}
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	o.win.SetTitle(args[1].AsStr())
	return vm.Bool(true), nil
}

func mustWindow(args []vm.Value, i int) (*guiObj, error) {
	if len(args) <= i {
		return nil, errf("gui: missing window handle")
	}
	id, err := guiHandle(args[i])
	if err != nil {
		return nil, err
	}
	o := guiGet(id)
	if o == nil || o.win == nil {
		return nil, errf("gui: not a window handle")
	}
	return o, nil
}

func mustObj(args []vm.Value, i int) (*guiObj, error) {
	if len(args) <= i {
		return nil, errf("gui: missing handle")
	}
	id, err := guiHandle(args[i])
	if err != nil {
		return nil, err
	}
	o := guiGet(id)
	if o == nil {
		return nil, errf("gui: invalid handle")
	}
	return o, nil
}

func asWidget(o *guiObj) core.Widget {
	switch o.kind {
	case gkLabel:
		return o.lab
	case gkButton:
		return o.btn
	case gkLineEdit:
		return o.edit
	case gkTextEdit:
		return o.text
	case gkCheckbox:
		return o.chk
	case gkWindow:
		return o.win
	default:
		return nil
	}
}

func attachKnown(win *core.Window, o *guiObj) {
	w := asWidget(o)
	if w == nil || o.kind == gkWindow {
		return
	}
	// AddWidget reparents, shows, reapplies bounds, invalidates.
	// Do not SetParent alone — host→window reparent must go through AddWidget.
	win.AddWidget(w)
}

func parseBounds(args []vm.Value, start int, dx, dy, dw, dh int32) (int32, int32, int32, int32) {
	x, y, w, h := dx, dy, dw, dh
	if len(args) >= start+4 {
		xi, _ := args[start].AsInt()
		yi, _ := args[start+1].AsInt()
		wi, _ := args[start+2].AsInt()
		hi, _ := args[start+3].AsInt()
		x, y, w, h = int32(xi), int32(yi), int32(wi), int32(hi)
	}
	return x, y, w, h
}

// gui.label(parent, text, x?, y?, w?, h?)
func guiLabel(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	po, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	text := ""
	if len(args) >= 2 {
		text = args[1].AsStr()
	}
	lab := widget.NewLabel(text)
	if lab == nil {
		return vm.Null(), errf("gui.label: create failed")
	}
	x, y, w, h := parseBounds(args, 2, 10, 10, 200, 24)
	lab.SetBounds(core.Rect{X: x, Y: y, Width: w, Height: h})
	o := &guiObj{kind: gkLabel, lab: lab}
	attachKnown(po.win, o)
	return guiMap("label", guiPut(o)), nil
}

func guiButton(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	po, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	text := "Button"
	if len(args) >= 2 {
		text = args[1].AsStr()
	}
	btn := widget.NewButton(text)
	if btn == nil {
		return vm.Null(), errf("gui.button: create failed")
	}
	x, y, w, h := parseBounds(args, 2, 10, 40, 100, 32)
	btn.SetBounds(core.Rect{X: x, Y: y, Width: w, Height: h})
	o := &guiObj{kind: gkButton, btn: btn}
	attachKnown(po.win, o)
	return guiMap("button", guiPut(o)), nil
}

func guiLineEdit(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	po, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	ed := widget.NewLineEdit()
	if ed == nil {
		return vm.Null(), errf("gui.lineedit failed")
	}
	x, y, w, h := parseBounds(args, 1, 10, 80, 300, 28)
	ed.SetBounds(core.Rect{X: x, Y: y, Width: w, Height: h})
	o := &guiObj{kind: gkLineEdit, edit: ed}
	attachKnown(po.win, o)
	return guiMap("lineedit", guiPut(o)), nil
}

func guiTextEdit(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	po, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	te := widget.NewTextEdit()
	if te == nil {
		return vm.Null(), errf("gui.textedit failed")
	}
	x, y, w, h := parseBounds(args, 1, 10, 120, 400, 200)
	te.SetBounds(core.Rect{X: x, Y: y, Width: w, Height: h})
	o := &guiObj{kind: gkTextEdit, text: te}
	attachKnown(po.win, o)
	return guiMap("textedit", guiPut(o)), nil
}

func guiCheckbox(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	po, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	text := "Check"
	if len(args) >= 2 {
		text = args[1].AsStr()
	}
	cb := widget.NewCheckBox(text)
	if cb == nil {
		return vm.Null(), errf("gui.checkbox failed")
	}
	x, y, w, h := parseBounds(args, 2, 10, 40, 160, 28)
	cb.SetBounds(core.Rect{X: x, Y: y, Width: w, Height: h})
	o := &guiObj{kind: gkCheckbox, chk: cb}
	attachKnown(po.win, o)
	return guiMap("checkbox", guiPut(o)), nil
}

func guiSetText(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.set_text(ctrl, text)")
	}
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	t := args[1].AsStr()
	switch o.kind {
	case gkLabel:
		o.lab.SetText(t)
	case gkButton:
		o.btn.SetText(t)
	case gkLineEdit:
		o.edit.SetText(t)
	case gkTextEdit:
		o.text.SetText(t)
	case gkCheckbox:
		o.chk.SetText(t)
	case gkWindow:
		o.win.SetTitle(t)
	}
	return vm.Bool(true), nil
}

func guiGetText(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Str(""), err
	}
	switch o.kind {
	case gkLabel:
		return vm.Str(o.lab.Text()), nil
	case gkLineEdit:
		return vm.Str(o.edit.Text()), nil
	case gkTextEdit:
		return vm.Str(o.text.Text()), nil
	case gkButton:
		return vm.Str(o.btn.Text()), nil
	case gkCheckbox:
		return vm.Str(o.chk.Text()), nil
	}
	return vm.Str(""), nil
}

func guiAppendText(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.append_text(textedit, text)")
	}
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	if o.kind != gkTextEdit || o.text == nil {
		return vm.Null(), errf("gui.append_text: need textedit")
	}
	o.text.AppendText(args[1].AsStr())
	return vm.Bool(true), nil
}

func guiSetBounds(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 5 {
		return vm.Null(), errf("gui.set_bounds(ctrl, x, y, w, h)")
	}
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	x, _ := args[1].AsInt()
	y, _ := args[2].AsInt()
	w, _ := args[3].AsInt()
	h, _ := args[4].AsInt()
	r := core.Rect{X: int32(x), Y: int32(y), Width: int32(w), Height: int32(h)}
	switch o.kind {
	case gkLabel:
		o.lab.SetBounds(r)
	case gkButton:
		o.btn.SetBounds(r)
	case gkLineEdit:
		o.edit.SetBounds(r)
	case gkTextEdit:
		o.text.SetBounds(r)
	case gkCheckbox:
		o.chk.SetBounds(r)
	case gkWindow:
		o.win.SetBounds(r)
	}
	return vm.Bool(true), nil
}

func guiOnClick(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.on_click(ctrl, fn)")
	}
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	if args[1].Typ != vm.TypeFunc || args[1].Fn == nil {
		return vm.Null(), errf("gui.on_click: function required")
	}
	cl := args[1].Fn
	vmRef := v
	handler := func() {
		if _, err := vmRef.CallClosure(cl, nil); err != nil {
			// surface callback errors instead of silent freeze
			guiMessageBox("GLTK GUI error", err.Error())
		}
	}
	switch {
	case o.kind == gkButton && o.btn != nil:
		o.btn.SetOnClick(handler)
	case o.kind == gkCheckbox && o.chk != nil:
		o.chk.SetOnClick(handler)
	default:
		return vm.Null(), errf("gui.on_click: button or checkbox required")
	}
	return vm.Bool(true), nil
}

// gui.is_checked(checkbox) -> bool
func guiIsChecked(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Bool(false), err
	}
	if o.kind != gkCheckbox || o.chk == nil {
		return vm.Bool(false), errf("gui.is_checked: need checkbox")
	}
	return vm.Bool(o.chk.Checked()), nil
}

// gui.set_checked(checkbox, bool)
func guiSetChecked(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.set_checked(checkbox, bool)")
	}
	o, err := mustObj(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	if o.kind != gkCheckbox || o.chk == nil {
		return vm.Null(), errf("gui.set_checked: need checkbox")
	}
	o.chk.SetChecked(args[1].Truthy())
	return vm.Bool(true), nil
}

func guiOnClose(v *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.on_close(win, fn)")
	}
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	if args[1].Typ != vm.TypeFunc || args[1].Fn == nil {
		return vm.Null(), errf("gui.on_close: need function")
	}
	cl := args[1].Fn
	vmRef := v
	o.win.SetOnClose(func() bool {
		res, err := vmRef.CallClosure(cl, nil)
		if err != nil {
			return true
		}
		// if fn returns false, cancel close
		if res.Typ == vm.TypeBool {
			return res.B
		}
		return true
	})
	return vm.Bool(true), nil
}

func guiMsgBox(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	msg, title := "", "GrokLang"
	if len(args) >= 1 {
		msg = args[0].AsStr()
	}
	if len(args) >= 2 {
		title = args[1].AsStr()
	}
	guiMessageBox(title, msg)
	return vm.Bool(true), nil
}

// gui.vbox(win, padding?, spacing?) — set vertical layout on window
func guiVBox(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	pad, sp := int32(12), int32(8)
	if len(args) >= 2 {
		if n, e := args[1].AsInt(); e == nil {
			pad = int32(n)
		}
	}
	if len(args) >= 3 {
		if n, e := args[2].AsInt(); e == nil {
			sp = int32(n)
		}
	}
	vb := layout.NewVBox(pad, sp)
	o.win.SetLayout(vb)
	id := guiPut(&guiObj{kind: gkLayout, vbox: vb, win: o.win})
	return guiMap("vbox", id), nil
}

func guiHBox(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	o, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	pad, sp := int32(12), int32(8)
	if len(args) >= 2 {
		if n, e := args[1].AsInt(); e == nil {
			pad = int32(n)
		}
	}
	if len(args) >= 3 {
		if n, e := args[2].AsInt(); e == nil {
			sp = int32(n)
		}
	}
	hb := layout.NewHBox(pad, sp)
	o.win.SetLayout(hb)
	id := guiPut(&guiObj{kind: gkLayout, hbox: hb, win: o.win})
	return guiMap("hbox", id), nil
}

// gui.add(win, ctrl) — ensure widget is on window (already attached at create)
func guiAdd(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	if len(args) < 2 {
		return vm.Null(), errf("gui.add(win, ctrl)")
	}
	wo, err := mustWindow(args, 0)
	if err != nil {
		return vm.Null(), err
	}
	co, err := mustObj(args, 1)
	if err != nil {
		return vm.Null(), err
	}
	attachKnown(wo.win, co)
	return vm.Bool(true), nil
}

// gui.open_file(filter_desc?, owner_window?) -> path string (empty if cancel)
func guiOpenFile(_ *vm.VM, args []vm.Value) (vm.Value, error) {
	filter := "All Files\x00*.*\x00"
	if len(args) >= 1 && args[0].AsStr() != "" {
		// simple: "Exe\x00*.exe\x00All\x00*.*\x00"
		filter = args[0].AsStr()
		if filter[len(filter)-1] != 0 {
			filter += "\x00"
		}
	}
	var owner uintptr
	// Optional owner window handle for modal dialog
	if len(args) >= 2 {
		if id, err := guiHandle(args[1]); err == nil {
			if o := guiGet(id); o != nil && o.win != nil {
				owner = uintptr(o.win.Handle())
			}
		}
	} else {
		// Prefer first tracked window as owner when available
		guiMu.Lock()
		for _, o := range guiObjs {
			if o != nil && o.win != nil {
				owner = uintptr(o.win.Handle())
				break
			}
		}
		guiMu.Unlock()
	}
	path := guiOpenFileDialog(filter, owner)
	return vm.Str(path), nil
}

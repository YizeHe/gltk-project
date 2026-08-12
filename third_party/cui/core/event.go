package core

// MouseButton represents a mouse button
type MouseButton int

const (
	MouseButtonLeft   MouseButton = 0
	MouseButtonRight  MouseButton = 1
	MouseButtonMiddle MouseButton = 2
)

// KeyModifiers represents keyboard modifiers
type KeyModifiers int

const (
	KeyModNone    KeyModifiers = 0
	KeyModShift   KeyModifiers = 1
	KeyModControl KeyModifiers = 2
	KeyModAlt     KeyModifiers = 4
)

// MouseEvent represents a mouse event
type MouseEvent struct {
	X, Y   int32
	Button MouseButton
	Wheel  int32
	Modifiers KeyModifiers
}

// KeyEvent represents a keyboard event
type KeyEvent struct {
	Key       uint32
	Char      rune
	Modifiers KeyModifiers
	IsRepeat  bool
}

// PaintEvent represents a paint event
type PaintEvent struct {
	Rect Rect
}

// ResizeEvent represents a resize event
type ResizeEvent struct {
	Width, Height int32
}

// FocusEvent represents a focus event
type FocusEvent struct {
	Gained bool
}

// ScrollEvent represents a scroll event
type ScrollEvent struct {
	DeltaX, DeltaY int32
}

// EventCallback types
type MouseCallback func(event MouseEvent)
type KeyCallback func(event KeyEvent)
type PaintCallback func(canvas *Canvas)
type ResizeCallback func(event ResizeEvent)
type FocusCallback func(event FocusEvent)
type ClickCallback func()
type ScrollCallback func(event ScrollEvent)
type CloseCallback func() bool // return false to prevent close

// EventType for generic event routing
type EventType int

const (
	EventClick EventType = iota
	EventDoubleClick
	EventMouseDown
	EventMouseUp
	EventMouseMove
	EventMouseEnter
	EventMouseLeave
	EventMouseWheel
	EventKeyDown
	EventKeyUp
	EventChar
	EventPaint
	EventResize
	EventFocusIn
	EventFocusOut
	EventClose
	EventScroll
	EventValueChanged
)

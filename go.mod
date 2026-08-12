module groklang/gltk

go 1.25.0

require (
	cui v0.0.0
	github.com/gorilla/websocket v1.5.3
	golang.org/x/arch v0.29.0
	golang.org/x/crypto v0.54.0
)

require (
	github.com/ulikunitz/xz v0.5.12 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// Vendored Win32 GUI library (CUI)
replace cui => ./third_party/cui

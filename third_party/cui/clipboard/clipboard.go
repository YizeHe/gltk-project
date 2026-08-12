package clipboard

import "cui/win32"

// GetText retrieves Unicode text from the clipboard
func GetText(hwndOwner win32.HWND) (string, bool) {
	return win32.ClipboardGetText(hwndOwner)
}

// SetText places Unicode text on the clipboard
func SetText(hwndOwner win32.HWND, text string) bool {
	return win32.ClipboardSetText(hwndOwner, text)
}

// HasText checks if text is available in the clipboard
func HasText() bool {
	return win32.IsClipboardFormatAvailable(win32.CF_UNICODETEXT) != 0
}

// Clear empties the clipboard
func Clear(hwndOwner win32.HWND) bool {
	if win32.OpenClipboard(hwndOwner) == 0 {
		return false
	}
	defer win32.CloseClipboard()
	return win32.EmptyClipboard() != 0
}

//go:build !((linux || freebsd || openbsd || netbsd) && !wayland)

package desktop

import "fyne.io/fyne/v2"

// raiseOver brings w to the front; see raise_x11.go. Elsewhere Fyne's own
// focus request is all there is.
func raiseOver(w, _ fyne.Window) { w.RequestFocus() }

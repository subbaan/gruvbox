//go:build (linux || freebsd || openbsd || netbsd) && !wayland

package desktop

import (
	"encoding/binary"
	"io"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// raiseOver makes w a dialog of parent and brings it to the front.
//
// With focus-stealing prevention on (xfwm4 has it on by default), a window
// manager may map a new window behind the active one. It also refuses
// Fyne's RequestFocus, which asks as the application. So on X11 the window
// is marked transient for the main window (WM_TRANSIENT_FOR, which keeps
// it above its parent), and once it is mapped the window manager is asked
// to activate it as a user request (_NET_ACTIVE_WINDOW, source 2). The user
// did ask for it: they opened it.
//
// It runs off the UI thread on its own X connection. X errors come back as
// values, so the worst a failure does is leave the window where it was.
func raiseOver(w, parent fyne.Window) {
	child, owner := x11Window(w), x11Window(parent)
	if child == 0 {
		return
	}
	go func() {
		c, err := xgb.NewConn()
		if err != nil {
			return
		}
		defer c.Close()
		if owner != 0 && owner != child {
			var data [4]byte
			binary.LittleEndian.PutUint32(data[:], uint32(owner))
			if xproto.ChangePropertyChecked(c, xproto.PropModeReplace, child, xproto.AtomWmTransientFor, xproto.AtomWindow, 32, 1, data[:]).Check() != nil {
				return // the window has gone
			}
		}
		// Activating before the window manager has mapped it does nothing.
		for i := 0; i < 100; i++ {
			a, err := xproto.GetWindowAttributes(c, child).Reply()
			if err != nil {
				return
			}
			if a.MapState == xproto.MapStateViewable {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		name := "_NET_ACTIVE_WINDOW"
		atom, err := xproto.InternAtom(c, true, uint16(len(name)), name).Reply()
		if err != nil || atom.Atom == xproto.AtomNone {
			return
		}
		event := xproto.ClientMessageEvent{
			Format: 32,
			Window: child,
			Type:   atom.Atom,
			Data:   xproto.ClientMessageDataUnionData32New([]uint32{2, xproto.TimeCurrentTime, uint32(owner), 0, 0}),
		}
		root := xproto.Setup(c).DefaultScreen(c).Root
		xproto.SendEventChecked(c, false, root, xproto.EventMaskSubstructureRedirect|xproto.EventMaskSubstructureNotify, string(event.Bytes())).Check()
	}()
}

// xgb logs connection trouble to stderr; a failed raise should be silent.
func init() { xgb.Logger = log.New(io.Discard, "", 0) }

// x11Window is a Fyne window's X11 id, or 0 (not shown yet, or no X11).
func x11Window(w fyne.Window) xproto.Window {
	native, ok := w.(driver.NativeWindow)
	if !ok {
		return 0
	}
	var id xproto.Window
	native.RunNative(func(context any) {
		if x, ok := context.(driver.X11WindowContext); ok {
			id = xproto.Window(x.WindowHandle)
		}
	})
	return id
}

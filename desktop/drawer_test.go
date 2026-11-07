package desktop

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestDrawerReplacesInspectorOneAtATime(t *testing.T) {
	u, _ := redesignUI(t)
	offset := u.workspace.Offset
	u.euclideanGenerator()
	if u.drawerTitled("Euclidean rhythm") == nil || u.sidePane.Visible() || !u.drawerHost.Visible() {
		t.Fatal("generator did not open in a drawer over the inspector")
	}
	if u.workspace.Offset > drawerOffset {
		t.Fatal("drawer did not widen the side pane")
	}
	u.lockGenerator()
	if u.drawerTitled("Euclidean rhythm") != nil || u.drawerTitled("Parameter-lock generator") == nil {
		t.Fatal("opening a second drawer did not replace the first")
	}
	if _, ok := u.generatorRefresh["Euclidean rhythm"]; ok {
		t.Fatal("replaced drawer did not run its close hook")
	}
	u.key(fyne.KeyEscape)
	if u.activeDrawer != nil || !u.sidePane.Visible() || u.drawerHost.Visible() || len(u.generatorRefresh) != 0 {
		t.Fatal("Escape did not close the drawer and restore the inspector")
	}
	if u.workspace.Offset != offset {
		t.Fatalf("split offset not restored: %v != %v", u.workspace.Offset, offset)
	}
	u.setLockMode(true)
	u.soundPresets()
	u.key(fyne.KeyEscape)
	if u.activeDrawer != nil || !u.lockMode {
		t.Fatal("Escape should close the drawer before leaving lock mode")
	}
	u.setLockMode(false)
}

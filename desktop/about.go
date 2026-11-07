package desktop

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/subbaan/gruvbox/config"
)

// KofiURL is where people can support Gruvbox's development.
const KofiURL = "https://ko-fi.com/subbaan"

const aboutTitle = "About Gruvbox"

// openURL hands a link to the desktop's browser; tests replace it.
var openURL = func(link *url.URL) error { return fyne.CurrentApp().OpenURL(link) }

// showAbout opens the About card: version, a short pitch and the Ko-fi link.
// Enter and Escape both just close it; only the button or link opens a browser.
func (u *UI) showAbout() {
	name := widget.NewLabelWithStyle("Gruvbox v"+config.Version, fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	pitch := widget.NewLabel("A desktop groovebox: TB-303-style acid voices, a Dual synth, analog-modelled drums, a sampler and per-track effects.\n\nIf Gruvbox makes you a groove worth keeping, you can buy me a coffee on Ko-fi. It keeps the updates coming. Thank you!")
	pitch.Wrapping = fyne.TextWrapWord
	link, _ := url.Parse(KofiURL)
	hyperlink := widget.NewHyperlink("ko-fi.com/subbaan", link)
	support := widget.NewButton("♥ Support on Ko-fi", func() { u.openKofi() })
	support.Importance = widget.HighImportance
	content := container.NewVBox(name, pitch, container.NewHBox(support, hyperlink))
	u.confirm(aboutTitle, "Close", "", content, func(bool) {})
	// A wrapping label reports its height for the width it was last laid out at.
	if m := u.modals[aboutTitle]; m != nil {
		pitch.Resize(fyne.NewSize(modalWidth-40, pitch.MinSize().Height))
		m.pop.Resize(fyne.NewSize(modalWidth, m.pop.Content.MinSize().Height))
	}
}

func (u *UI) openKofi() {
	link, err := url.Parse(KofiURL)
	if err == nil {
		err = openURL(link)
	}
	if err != nil {
		u.showError(err)
		return
	}
	u.status.SetText("Opened " + KofiURL + " in your browser")
}

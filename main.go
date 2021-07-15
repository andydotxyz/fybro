//go:generate fyne bundle -o bundled.go Icon.png

package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
)

const (
	prefServerCountKey = "server.count"
	prefServerPrefix   = "server.%d."
	prefServerTypeKey  = "type"

	winTitle = "Fibro"
)

func main() {
	a := app.NewWithID("xyz.andy.fibro")
	a.SetIcon(resourceIconPng)
	w := a.NewWindow(winTitle)

	u := &ui{win: w}
	w.SetContent(u.makeUI())
	w.Resize(fyne.NewSize(480, 320))
	go u.runLogins(w, a)
	w.ShowAndRun()

	// after app quits
	disconnectAll()
}

func (u *ui) runLogins(w fyne.Window, a fyne.App) {
	count := a.Preferences().Int(prefServerCountKey)
	if count == 0 {
		prefix := fmt.Sprintf(prefServerPrefix, 0)
		disc, err := connect("discord", a)
		if err != nil {
			dialog.ShowError(err, fyne.CurrentApp().Driver().AllWindows()[0])
			return
		}
		disc.login(w, prefix, u)
		a.Preferences().SetInt(prefServerCountKey, 1) // TODO handle actual server add
		a.Preferences().SetString(prefix+prefServerTypeKey, "discord")
	}
	for i := 0; i < count; i++ {
		prefPrefix := fmt.Sprintf(prefServerPrefix, i)
		typeKey := prefPrefix + prefServerTypeKey

		srv, err := connect(a.Preferences().String(typeKey), a)
		if err != nil {
			dialog.ShowError(err, w)
			continue
		}
		srv.login(w, prefPrefix, u)
	}
}

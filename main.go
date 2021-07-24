//go:generate fyne bundle -o bundled.go Icon.png
//go:generate fyne bundle -o bundled.go -append img/whatsapp.png
//go:generate fyne bundle -o bundled.go -append img/telegram.png

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
	w.SetContent(u.makeUI(w, a))
	w.Resize(fyne.NewSize(480, 320))
	go u.runLogins(w, a)
	w.ShowAndRun()

	// after app quits
	disconnectAll()
}

func (u *ui) runLogins(w fyne.Window, a fyne.App) {
	count := a.Preferences().Int(prefServerCountKey)
	if count == 0 {
		u.addLogin(w, a)
	}
	for i := 0; i < count; i++ {
		prefPrefix := fmt.Sprintf(prefServerPrefix, i)
		typeKey := prefPrefix + prefServerTypeKey

		srv, err := connect(a.Preferences().String(typeKey), a)
		if err != nil {
			dialog.ShowError(err, w)
			continue
		}
		srv.login(prefPrefix, u)
	}
}

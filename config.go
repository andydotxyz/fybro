package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var (
	selectedID     string
	selectedOption service
	selectedLogin  func(string, fyne.App)
)

func (u *ui) addLogin(w fyne.Window, a fyne.App) {
	content := u.loginContent(a)
	dialog.ShowCustomConfirm("Choose server to add", "Log In", "Cancel",
		content, func(ok bool) {
			if !ok {
				return
			}

			count := a.Preferences().Int(prefServerCountKey)
			prefix := fmt.Sprintf(prefServerPrefix, count)
			a.Preferences().SetInt(prefServerCountKey, count+1)
			a.Preferences().SetString(prefix+prefServerTypeKey, selectedID)
			go selectedLogin(prefix, a)
		}, w)
}

func (u *ui) loginContent(a fyne.App) fyne.CanvasObject {
	var opts []struct {
		id      string
		srv     service
		content fyne.CanvasObject
		login   func(string, fyne.App)
	}

	for id, data := range services {
		srv := data(a)
		content, save := srv.configure(u)
		opts = append(opts, struct {
			id      string
			srv     service
			content fyne.CanvasObject
			login   func(string, fyne.App)
		}{id, srv, content, save})
	}

	details := container.NewMax(widget.NewLabel("and add your login details"))
	title := widget.NewLabelWithStyle("Choose a server", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	list := widget.NewList(
		func() int {
			return len(opts)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("service...")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(opts[id].id)
		})
	list.OnSelected = func(id widget.ListItemID) {
		opt := opts[id]
		title.SetText(fmt.Sprintf("Add a %s server", strings.Title(opt.id)))
		details.Objects = []fyne.CanvasObject{
			opts[id].content,
		}
		details.Refresh()

		selectedID = opt.id
		selectedOption = opt.srv
		selectedLogin = opt.login
	}

	return container.NewBorder(nil, nil, list, nil,
		container.NewBorder(title, nil, nil, nil, details))
}

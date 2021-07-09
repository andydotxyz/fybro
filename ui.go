package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/diamondburned/arikawa/session"
)

type ui struct {
	servers, channels, messages *widget.List

	data           *appData
	currentServer  *server
	currentChannel *channel
	conn           *session.Session
}

func (u *ui) makeUI() fyne.CanvasObject {
	u.servers = widget.NewList(
		func() int {
			if u.data == nil {
				return 0
			}
			return len(u.data.servers)
		},
		func() fyne.CanvasObject {
			img := &canvas.Image{}
			img.SetMinSize(fyne.NewSize(theme.IconInlineSize()*2, theme.IconInlineSize()*2))
			return img
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*canvas.Image).Resource = u.data.servers[id].icon()
			o.Refresh()
		})
	u.servers.OnSelected = func(id widget.ListItemID) {
		u.currentServer = u.data.servers[id]
		u.channels.Select(0)
		u.refresh()
	}

	u.channels = widget.NewList(
		func() int {
			if u.currentServer == nil {
				return 0
			}
			return len(u.currentServer.channels)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText("# " + u.currentServer.channels[id].name)
		})
	u.channels.OnSelected = func(id widget.ListItemID) {
		u.currentChannel = u.currentServer.channels[id]
		u.refresh()
	}

	u.messages = widget.NewList(
		func() int {
			if u.currentChannel == nil {
				return 0
			}
			return len(u.currentChannel.messages)
		},
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel(""))
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			objs := o.(*fyne.Container).Objects
			objs[0].(*widget.Label).SetText(u.currentChannel.messages[id].author)
			objs[1].(*widget.Label).SetText(u.currentChannel.messages[id].content)
		})

	content := container.NewHSplit(u.channels, u.messages)
	content.Offset = 0.3
	return container.NewBorder(nil, nil, u.servers, nil, content)
}

func (u *ui) refresh() {
	u.servers.Refresh()
	u.channels.Refresh()
	u.messages.Refresh()
}

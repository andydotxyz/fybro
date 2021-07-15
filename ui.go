package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ui struct {
	servers, channels *widget.List
	messages          *fyne.Container
	messageScroll     *container.Scroll
	create            *widget.Entry

	data           *appData
	currentServer  *server
	currentChannel *channel
}

func (u *ui) appendMessages(list []*message) {
	items := u.messages.Objects
	for _, m := range list {
		items = append(items, newMessageCell(m))
	}
	u.messages.Objects = items
	u.messages.Refresh()
	u.messageScroll.ScrollToBottom()
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
		u.channels.Unselect(0)
		u.channels.Select(0)
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
		u.messages.Objects = nil
		u.appendMessages(u.currentChannel.messages)
	}

	u.messages = container.NewVBox()
	u.messageScroll = container.NewScroll(u.messages)

	u.create = widget.NewEntry()
	u.create.OnSubmitted = u.send
	messagePane := container.NewBorder(nil,
		container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("",
			theme.MailSendIcon(), func() {
				u.send(u.create.Text)
			}), u.create), nil, nil, u.messageScroll)
	content := container.NewHSplit(u.channels, messagePane)
	content.Offset = 0.3
	return container.NewBorder(nil, nil, u.servers, nil, content)
}

func (u *ui) send(data string) {
	srv := u.currentServer.service
	srv.send(u.currentChannel, data)
	u.create.SetText("")
}

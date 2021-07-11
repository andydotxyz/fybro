package main

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/ningen/v2"
)

type ui struct {
	servers, channels *widget.List
	messages          *fyne.Container
	messageScroll     *container.Scroll
	create            *widget.Entry

	data *appData

	currentMu      sync.Mutex
	currentServer  *server
	currentChannel *channel
	conn           *ningen.State
}

func (u *ui) appendMessages(list []*message) {
	items := u.messages.Objects
	for _, m := range list {
		cell := newMessageCell(m)

		if len(items) > 0 {
			// If the last author is the same as this new message, then we can
			// render a small version of it.
			last, ok := items[len(items)-1].(*messageCell)
			if ok && last.msg.author == cell.msg.author {
				cell.small = true
			}
		}

		items = append(items, cell)
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
			u.data.servers[id].setIconInto(o.(*canvas.Image))
		})
	u.servers.OnSelected = func(id widget.ListItemID) {
		u.currentServer = u.data.servers[id]
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
		u.currentMu.Lock()
		defer u.currentMu.Unlock()

		u.currentChannel = u.currentServer.channels[id]
		u.messages.Objects = nil

		msgs := u.loadRecentMessages(discord.ChannelID(u.currentChannel.id))
		u.appendMessages(msgs)
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
	u.conn.SendText(discord.ChannelID(u.currentChannel.id), data)
	u.create.SetText("")
}

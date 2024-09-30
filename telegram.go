package main

import (
	"context"
	"log"
	"path/filepath"
	"strconv"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
	msg2 "github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

const prefTelegramTelKey = "auth.tel"

type telegram struct {
	app     fyne.App
	ip      string
	proto   *gotgproto.Client
	context *ext.Context

	server *server
	ui     *ui
}

func initTelegram(a fyne.App) service {
	return &telegram{app: a, ip: telegramDefaultIP}
}

func (t *telegram) configure(u *ui) (fyne.CanvasObject, func(prefix string, a fyne.App)) {
	t.ui = u
	tel := widget.NewEntry()
	return widget.NewForm(
			&widget.FormItem{Text: "Telephone", Widget: tel}),
		func(prefix string, a fyne.App) {
			a.Preferences().SetString(prefix+prefTelegramTelKey, tel.Text)

			t.login(prefix, u)
		}
}

func (t *telegram) disconnect() {
	t.proto.Stop()
}

func (t *telegram) getUser(id int64) *user {
	uid := strconv.Itoa(int(id))
	if usr, found := t.server.users[uid]; found {
		return usr
	}

	data, err := t.context.Raw.UsersGetUsers(t.context, []tg.InputUserClass{&tg.InputUser{UserID: id}})
	if err != nil || len(data) == 0 {
		fyne.LogError("Failed to download user info", err)
		return nil
	}

	u, _ := data[0].AsNotEmpty()
	user := &user{username: u.Username, name: userDisplayName(u)}
	//if u.Photo.(mtproto.TL_userProfilePhoto).Photo_small != nil {
	//	time.Sleep(time.Second*10)
	//	p := u.Photo.(mtproto.TL_userProfilePhoto).Photo_small.(mtproto.TL_fileLocation)
	//	l := &mtproto.TL_inputFileLocation{
	//		Secret: p.Secret,
	//		Local_id: p.Local_id,
	//		Volume_id: p.Volume_id,
	//	}
	//	f, err := t.proto.UploadGetFile(l, 0)
	//	log.Println("F", f, err)
	//}

	t.server.users[uid] = user
	return user
}

func (t *telegram) login(prefix string, u *ui) {
	t.ui = u
	p := t.app.Preferences()
	num := p.String(prefix + prefTelegramTelKey)

	path := filepath.Join(fyne.CurrentApp().Storage().RootURI().Path(), "fybro-telegram.sqlite")
	client, err := gotgproto.NewClient(
		telegramAppID,
		telegramAppHash,
		gotgproto.ClientTypePhone(num),
		&gotgproto.ClientOpts{
			AuthConversator: &inputGetter{num: num},
			Session:         sessionMaker.SqlSession(sqlite.Open(path)),
			InMemory:        false,
		},
	)

	if err != nil {
		fyne.LogError("Connect failed", err)
		return
	}

	t.proto = client
	t.context = client.CreateContext()

	client.Dispatcher.AddHandler(&updateHandler{t: t, u: u})
	go func() {
		client.Idle()
	}()

	t.loadServers(t.context, prefix, u)
}

func (t *telegram) loadServers(s *ext.Context, prefix string, u *ui) {
	srv := &server{service: t, name: "Telegram", iconResource: resourceTelegramPng}
	srv.users = make(map[string]*user)
	t.server = srv

	if u.data == nil {
		u.data = &appData{}
	}
	u.data.servers = append(u.data.servers, srv)
	if len(u.data.servers) > 0 {
		u.currentServer = u.data.servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	// try group chats
	ret, err := s.Raw.MessagesGetDialogs(s, &tg.MessagesGetDialogsRequest{OffsetPeer: &tg.InputPeerEmpty{}})
	if err != nil {
		fyne.LogError("Unknown protocol error", err)
	}
	for _, c := range ret.(*tg.MessagesDialogsSlice).Chats {
		chat := c.(*tg.Chat)
		chn := &channel{name: chat.Title, id: strconv.Itoa(int(chat.ID)), direct: false, server: srv}

		if len(srv.channels) == 0 {
			id, _ := strconv.Atoi(chn.id)
			chn.messages = t.loadMessages(s, int64(id), false)
			if srv == u.currentServer {
				u.setChannel(chn)
			}
		}
		srv.channels = append(srv.channels, chn)
	}
	u.channels.Refresh()

	// direct messages
	contacts, err := s.Raw.ContactsGetTopPeers(s, &tg.ContactsGetTopPeersRequest{Correspondents: true})
	if contacts != nil {
		for _, c := range contacts.(*tg.ContactsTopPeers).Users {
			chat, _ := c.AsNotEmpty()
			chn := &channel{name: userDisplayName(chat), id: strconv.Itoa(int(chat.ID)), direct: true, server: srv}

			if len(srv.channels) == 0 {
				cid, _ := strconv.Atoi(chn.id)
				chn.messages = t.loadMessages(s, int64(cid), true)
				if srv == u.currentServer {
					u.setChannel(chn)
				}
			}
			srv.channels = append(srv.channels, chn)
		}
	}
	u.channels.Refresh()

	for i, c := range srv.channels {
		if i == 0 {
			continue // we did this one above
		}
		id, _ := strconv.Atoi(c.id)
		c.messages = t.loadMessages(s, int64(id), c.direct)
	}
}

func (t *telegram) loadMessages(s *ext.Context, id int64, direct bool) []*message {
	var nid tg.InputPeerClass
	if direct {
		nid = &tg.InputPeerUser{UserID: id}
	} else {
		nid = &tg.InputPeerChat{ChatID: id}
	}
	ret, err := s.Raw.MessagesGetHistory(s, &tg.MessagesGetHistoryRequest{Peer: nid})
	//	ret, err := s.MessagesGetHistory(nid, 0, 0, 0, 15, 6500000, 0)
	if err != nil {
		fyne.LogError("Unknown message download error", err)
		return nil
	}

	var list []*message
	ms := ret.(*tg.MessagesMessagesSlice).Messages
	for i := len(ms) - 1; i >= 0; i-- { // newest message is first in response
		data, ok := ms[i].AsNotEmpty()
		if !ok {
			log.Println("Could not parse message")
			continue
		}

		m := data.(*tg.Message)
		from := id
		if m.FromID != nil {
			from = m.FromID.(*tg.PeerUser).UserID
		}
		msg := &message{content: m.Message, user: t.getUser(from)}
		list = append(list, msg)
	}

	return list
}

func (t *telegram) send(ch *channel, text string) {
	id, _ := strconv.Atoi(ch.id)
	send := msg2.NewSender(t.proto.API())
	var builder *msg2.RequestBuilder

	if ch.direct {
		builder = send.To(&tg.InputPeerUser{UserID: int64(id)})
	} else {
		builder = send.To(&tg.InputPeerChat{ChatID: int64(id)})
	}

	_, err := builder.Text(context.Background(), text)
	if err != nil {
		fyne.LogError("Failed to send message", err)
		return
	}

	msg := &message{content: text, user: t.getUser(t.context.Self.ID)}
	ch.messages = append(ch.messages, msg)
	t.ui.messages.Objects = nil
	t.ui.appendMessages(ch.messages)
}

func userDisplayName(u *tg.User) string {
	if u.FirstName != "" || u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Phone
}

type inputGetter struct {
	num string
}

func (i *inputGetter) AskPhoneNumber() (string, error) {
	return i.num, nil
}

func (i *inputGetter) AskCode() (string, error) {
	w := fyne.CurrentApp().Driver().AllWindows()[0]
	wg := sync.WaitGroup{}

	conf := widget.NewEntry()
	dialog.ShowForm("Telegram code for "+i.num, "Log in", "cancel",
		[]*widget.FormItem{
			{Text: "Auth Code", Widget: conf},
		}, func(ok bool) {
			wg.Done()
		}, w)
	wg.Add(1)

	wg.Wait()
	return conf.Text, nil
}

func (i *inputGetter) AskPassword() (string, error) {
	w := fyne.CurrentApp().Driver().AllWindows()[0]
	wg := sync.WaitGroup{}

	conf := widget.NewPasswordEntry()
	dialog.ShowForm("Telegram password for "+i.num, "Log in", "cancel",
		[]*widget.FormItem{
			{Text: "Password", Widget: conf},
		}, func(ok bool) {
			wg.Done()
		}, w)
	wg.Add(1)

	wg.Wait()
	return conf.Text, nil
}

func (i *inputGetter) AuthStatus(gotgproto.AuthStatus) {
}

type updateHandler struct {
	t *telegram
	u *ui
}

func (u *updateHandler) CheckUpdate(_ *ext.Context, up *ext.Update) error {
	switch t := up.UpdateClass.(type) {
	case *tg.UpdateNewMessage:
		m := up.EffectiveMessage
		from := int64(0)
		if m.FromID != nil {
			from = m.FromID.(*tg.PeerUser).UserID
		} else {
			log.Println("unknown from")
		}
		msg := &message{content: m.Message.Message, user: u.t.getUser(from)}

		cid := int64(0)
		if u, ok := m.PeerID.(*tg.PeerUser); ok {
			cid = u.UserID
		} else if c, ok := m.PeerID.(*tg.PeerChat); ok {
			cid = c.ChatID
		} else {
			log.Println("Unknown type", m.PeerID)
		}

		ch := findServerChan(u.t.server, strconv.Itoa(int(cid)))
		ch.messages = append(ch.messages, msg)

		if ch == u.u.currentChannel {
			u.u.messages.Objects = nil
			u.u.appendMessages(u.u.currentChannel.messages)
		}
	case *tg.UpdateEditMessage:
		log.Println("TODO handle edited message")
	case *tg.UpdateUserStatus, *tg.UpdateUserTyping, *tg.UpdateReadHistoryInbox, *tg.UpdateReadHistoryOutbox:
		log.Println("ignoring typing/read status")
	default:
		log.Println("Unknown update", t)
	}

	return nil
}

package main

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"github.com/bobrovde/mtproto"
)

const prefTelegramTelKey = "auth.tel"

type telegram struct {
	app    fyne.App
	ip     string
	proto  *mtproto.MTProto
	server *server
}

func initTelegram(a fyne.App) service {
	return &telegram{app: a, ip: telegramDefaultIP}
}

func (t *telegram) configure(u *ui) (fyne.CanvasObject, func(prefix string, a fyne.App)) {
	tel := widget.NewEntry()
	return widget.NewForm(
			&widget.FormItem{Text: "Telephone", Widget: tel}),
		func(prefix string, a fyne.App) {
			a.Preferences().SetString(prefix+prefTelegramTelKey, tel.Text)

			t.login(prefix, u)
		}
}

func (t *telegram) disconnect() {
	_ = t.proto.Disconnect()
}

func (t *telegram) getUser(id int32) *user {
	uid := strconv.Itoa(int(id))
	if usr, found := t.server.users[uid]; found {
		return usr
	}

	data, err := t.proto.UsersGetFullUsers(mtproto.TL_inputUser{User_id: id})
	if err != nil {
		fyne.LogError("Failed to download user info", err)
		return nil
	}

	u := data.User.(mtproto.TL_user)
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
	authFile, _ := storage.Child(t.app.Storage().RootURI(), prefix+"auth.token")
	exists, _ := storage.Exists(authFile)
	m, err := mtproto.NewMTProto(telegramAppID, telegramAppHash,
		mtproto.WithServer(t.ip, false),
		mtproto.WithAuthFile(authFile.Path(), !exists))
	if err != nil {
		fyne.LogError("Connect failed", err)
		return
	}
	err = m.Connect()
	t.proto = m
	if err != nil {
		fyne.LogError("Connect failed", err)
		return
	}

	ret, err := m.UpdatesGetState()
	if err != nil {
		fyne.LogError("Failed to init updates", err)
	} else {
		go func() {
			pts := (*ret).(mtproto.TL_updates_state).Pts
			qts := (*ret).(mtproto.TL_updates_state).Qts
			date := int32(time.Now().Unix())
			for {
				time.Sleep(time.Second * 10)

				ret, err := m.UpdatesGetState()
				if err != nil {
					fyne.LogError("update request failed", err)
					continue
				}

				data := (*ret).(mtproto.TL_updates_state)
				if pts == data.Pts {
					continue
				}

				items, err := m.UpdatesGetDifference(pts, 0, date, qts)
				if err != nil {
					fyne.LogError("difference request failed", err)
					continue
				}
				for _, item := range (*items).(mtproto.TL_updates_difference).New_messages {
					m := item.(mtproto.TL_message)

					cid := int32(0)
					if tl, ok := m.To_id.(mtproto.TL_peerChat); ok {
						cid = tl.Chat_id
					} else {
						cid = m.To_id.(mtproto.TL_peerUser).User_id
					}
					msg := &message{content: m.Message, user: t.getUser(m.From_id)}
					ch := findServerChan(t.server, strconv.Itoa(int(cid)))
					ch.messages = append(ch.messages, msg)

					if ch == u.currentChannel {
						u.messages.Objects = nil
						u.appendMessages(u.currentChannel.messages)
					}
				}
				pts = data.Pts
				qts = data.Qts
				date = int32(time.Now().Unix())
			}
		}()
	}
	if exists {
		t.loadServers(m, prefix, u)
		return
	}

	p := t.app.Preferences()
	num := p.String(prefix + prefTelegramTelKey)
	c, err := m.AuthSendCode(num)
	if err != nil {
		if status, ok := err.(mtproto.TL_rpc_error); ok {
			if status.Error_message == "AUTH_RESTART" {
				t.login(prefix, u)
			} else if strings.ContainsAny(status.Error_message, "PHONE_MIGRATE_") {
				id, _ := strconv.Atoi(status.Error_message[14:])
				storage.Delete(authFile)

				t.ip, _ = m.GetDCIP(int32(id))
				t.login(prefix, u)
			} else {
				fyne.LogError("Unknown protocol error", err)
			}
		} else {
			fyne.LogError("Unknown error", err)
		}
		return
	}
	hash := c.Phone_code_hash
	conf := widget.NewEntry()
	dialog.ShowForm("Telegram code for "+num, "Log in", "cancel",
		[]*widget.FormItem{
			{Text: "Auth Code", Widget: conf},
		}, func(ok bool) {
			code := conf.Text
			_, err := m.AuthSignIn(num, code, hash)
			if err != nil {
				fyne.LogError("Failed to log in to telegram", err)
				return
			}

			t.loadServers(m, prefix, u)
		}, u.win)
}

func (t *telegram) loadServers(s *mtproto.MTProto, prefix string, u *ui) {
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
	ret, err := s.ChatsGetAllChats([]int32{})
	if err != nil {
		if status, ok := err.(mtproto.TL_rpc_error); ok {
			strings.ContainsAny(status.Error_message, "AUTH")
			authFile, _ := storage.Child(t.app.Storage().RootURI(), prefix+"auth.token")
			_ = storage.Delete(authFile)
			t.login(prefix, u)
			return
		} else {
			fyne.LogError("Unknown protocol error", err)
		}
	}
	for _, c := range (*ret).(mtproto.TL_messages_chats).Chats {
		chat := c.(mtproto.TL_chat)
		chn := &channel{name: chat.Title, id: strconv.Itoa(int(chat.Id)), direct: false, server: srv}

		if len(srv.channels) == 0 {
			id, _ := strconv.Atoi(chn.id)
			chn.messages = t.loadMessages(s, id, false)
			if srv == u.currentServer {
				u.setChannel(chn)
			}
		}
		srv.channels = append(srv.channels, chn)
	}
	u.channels.Refresh()

	// direct messages
	ret, err = s.ContactsGetTopPeers(true, false, false, false, false, 0, 0, 0)
	if ret != nil {
		for _, c := range (*ret).(mtproto.TL_contacts_topPeers).Users {
			chat := c.(mtproto.TL_user)
			chn := &channel{name: userDisplayName(chat), id: strconv.Itoa(int(chat.Id)), direct: true, server: srv}

			if len(srv.channels) == 0 {
				cid, _ := strconv.Atoi(chn.id)
				chn.messages = t.loadMessages(s, cid, true)
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
		c.messages = t.loadMessages(s, id, c.direct)
	}
}

func (t *telegram) loadMessages(s *mtproto.MTProto, id int, direct bool) []*message {
	var nid mtproto.TL
	if direct {
		nid = mtproto.TL_inputPeerUser{User_id: int32(id)}
	} else {
		nid = mtproto.TL_inputPeerChat{Chat_id: int32(id)}
	}
	ret, err := s.MessagesGetHistory(nid, 0, 0, 0, 15, 6500000, 0)
	if err != nil {
		fyne.LogError("Unknown message download error", err)
		return nil
	}

	var list []*message
	ms := (*ret).(mtproto.TL_messages_messagesSlice).Messages
	for i := len(ms) - 1; i >= 0; i-- { // newest message is first in response
		m := ms[i].(mtproto.TL_message)
		msg := &message{content: m.Message, user: t.getUser(m.From_id)}
		list = append(list, msg)
	}

	return list
}

func (t *telegram) send(ch *channel, text string) {
	id, _ := strconv.Atoi(ch.id)
	var nid mtproto.TL
	if ch.direct {
		nid = mtproto.TL_inputPeerUser{User_id: int32(id)}
	} else {
		nid = mtproto.TL_inputPeerChat{Chat_id: int32(id)}
	}

	t.proto.MessagesSendMessage(true, false, false, true,
		nid, 0, text, rand.Int63(), mtproto.TL_null{}, nil)
}

func userDisplayName(u mtproto.TL_user) string {
	if u.First_name != "" || u.Last_name != "" {
		return u.First_name + " " + u.Last_name
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Phone
}

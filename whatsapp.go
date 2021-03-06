package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Rhymen/go-whatsapp"
	"github.com/skip2/go-qrcode"
)

const (
	prefWhatsEncKeyKey      = "sess.enc"
	prefWhatsMacKeyKey      = "sess.mac"
	prefWhatsClientIDKey    = "sess.client"
	prefWhatsClientTokenKey = "sess.token"
	prefWhatsServerTokenKey = "sess.server"
)

type whatsApp struct {
	app    fyne.App
	conn   *whatsapp.Conn
	server *server
	ui     *ui
}

func initWhatsApp(a fyne.App) service {
	return &whatsApp{app: a}
}

func (w *whatsApp) configure(u *ui) (fyne.CanvasObject, func(prefix string, a fyne.App)) {
	w.conn = w.setupClient(60)

	return widget.NewLabel("Open WhatsApp on your phone and\nprepare to scan QR code"), func(prefix string, a fyne.App) {
		qrChan := make(chan string)
		var qrScreen dialog.Dialog
		go func() {
			png, _ := qrcode.Encode(<-qrChan, qrcode.Medium, int(200*u.win.Canvas().Scale()))
			img := canvas.NewImageFromReader(bytes.NewReader(png), "qr.png")
			img.SetMinSize(fyne.NewSize(200, 200))
			qrScreen = dialog.NewCustom("WhatsApp QR scan", "Cancel", img, u.win)
			qrScreen.Show()
		}()

		sess, err := w.conn.Login(qrChan)
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}

		encStr := base64.StdEncoding.EncodeToString(sess.EncKey)
		macStr := base64.StdEncoding.EncodeToString(sess.MacKey)
		qrScreen.Hide()
		a.Preferences().SetString(prefix+prefWhatsEncKeyKey, encStr)
		a.Preferences().SetString(prefix+prefWhatsMacKeyKey, macStr)
		a.Preferences().SetString(prefix+prefWhatsClientIDKey, sess.ClientId)
		a.Preferences().SetString(prefix+prefWhatsClientTokenKey, sess.ClientToken)
		a.Preferences().SetString(prefix+prefWhatsServerTokenKey, sess.ServerToken)
		w.login(prefix, u)
	}
}

func (w *whatsApp) disconnect() {
	_, _ = w.conn.Disconnect()
}

func (w *whatsApp) login(prefix string, u *ui) {
	w.ui = u
	if w.conn == nil {
		w.conn = w.setupClient(5)

		p := w.app.Preferences()
		encBytes, _ := base64.StdEncoding.DecodeString(p.String(prefix + prefWhatsEncKeyKey))
		macBytes, _ := base64.StdEncoding.DecodeString(p.String(prefix + prefWhatsMacKeyKey))
		load := whatsapp.Session{
			EncKey:      encBytes,
			MacKey:      macBytes,
			ClientId:    p.String(prefix + prefWhatsClientIDKey),
			ClientToken: p.String(prefix + prefWhatsClientTokenKey),
			ServerToken: p.String(prefix + prefWhatsServerTokenKey)}
		_, err := w.conn.RestoreWithSession(load)
		if err != nil {
			log.Println("Failed to recover WhatsApp session", err)
			return
		}
	}

	srv := &server{service: w, name: "WhatsApp", iconResource: resourceWhatsappPng}
	srv.users = make(map[string]*user)
	w.server = srv

	if u.data == nil {
		u.data = &appData{}
	}
	u.data.servers = append(u.data.servers, srv)
	if len(u.data.servers) > 0 {
		u.currentServer = u.data.servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	w.conn.AddHandler(w)
}

func (w *whatsApp) send(ch *channel, text string) {
	_, err := w.conn.Send(whatsapp.TextMessage{Text: text, Info: whatsapp.MessageInfo{
		RemoteJid: ch.id}})
	if err != nil {
		log.Println("Error sending", err)
		return
	}

	msg := &message{content: text, user: w.getUser(w.conn.Info.Wid)}
	ch.messages = append(ch.messages, msg)
	w.ui.appendMessages([]*message{msg})
}

func (w *whatsApp) setupClient(secs int) *whatsapp.Conn {
	wac, _ := whatsapp.NewConn(time.Duration(secs) * time.Second)
	wac.SetClientVersion(2, 2121, 6)
	_ = wac.SetClientName("Fybro Cross-service chat", "Fybro", "0.1")

	return wac
}

func (w *whatsApp) HandleError(err error) {
	log.Println("WhatsApp error", err)
}

func (w *whatsApp) HandleTextMessage(m whatsapp.TextMessage) {
	from := m.Info.RemoteJid
	if m.Info.FromMe {
		from = w.conn.Info.Wid
	} else if m.Info.Source.Participant != nil {
		from = *m.Info.Source.Participant
	}
	msg := &message{content: m.Text, user: w.getUser(from)}
	var ch *channel
	for _, c := range w.server.channels {
		if c.id == m.Info.RemoteJid {
			ch = c
			break
		}
	}
	if ch == nil {
		ch = &channel{id: m.Info.RemoteJid, server: w.server}
		w.server.channels = append(w.server.channels, ch)

		data, err := w.conn.GetGroupMetaData(m.Info.RemoteJid)
		if err == nil {
			vals := make(map[string]interface{})
			d := json.NewDecoder(strings.NewReader(<-data))
			_ = d.Decode(&vals)
			if name, ok := vals["subject"].(string); ok {
				ch.name = name
			} else {
				ch.name = w.getUser(m.Info.RemoteJid).name
			}
		} else {
			log.Println("get channel title error", err)
		}
	}
	ch.messages = append(ch.messages, msg)

	if ch == w.ui.currentChannel {
		w.ui.appendMessages([]*message{msg})
	}
}

var userLock sync.RWMutex

func (w *whatsApp) getUser(id string) *user {
	userLock.RLock()
	usr, found := w.server.users[id]
	userLock.RUnlock()
	if found {
		return usr
	}

	user := &user{name: "someone",
		username: id}

	if contact, ok := w.conn.Store.Contacts[id]; ok {
		user.name = contact.Name
		user.username = contact.Short
	}

	data, err := w.conn.GetProfilePicThumb(id)
	if err == nil {
		vals := make(map[string]interface{})
		d := json.NewDecoder(strings.NewReader(<-data))
		_ = d.Decode(&vals)
		if url, ok := vals["eurl"].(string); ok {
			user.avatarURL = url
		}
	}
	userLock.Lock()
	w.server.users[id] = user
	userLock.Unlock()
	return user
}

package main

import (
	"bytes"
	"encoding/base64"
	"log"
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
	wac, _ := whatsapp.NewConn(30 * time.Second)
	wac.SetClientVersion(2, 2121, 6)
	wac.SetClientName("Fibro Cross-service chat", "Fibro", "0.1")
	w.conn = wac

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

		sess, err := wac.Login(qrChan)
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
		wac, _ := whatsapp.NewConn(10 * time.Second)
		wac.SetClientVersion(2, 2121, 6)
		wac.SetClientName("Fibro Cross-service chat", "Fibro", "0.1")
		w.conn = wac

		p := w.app.Preferences()
		encBytes, _ := base64.StdEncoding.DecodeString(p.String(prefix + prefWhatsEncKeyKey))
		macBytes, _ := base64.StdEncoding.DecodeString(p.String(prefix + prefWhatsMacKeyKey))
		load := whatsapp.Session{
			EncKey:      encBytes,
			MacKey:      macBytes,
			ClientId:    p.String(prefix + prefWhatsClientIDKey),
			ClientToken: p.String(prefix + prefWhatsClientTokenKey),
			ServerToken: p.String(prefix + prefWhatsServerTokenKey)}
		_, err := wac.RestoreWithSession(load)
		if err != nil {
			log.Println("Failed tor recover WhatsApp session", err)
		}
	}

	srv := &server{service: w, name: "WhatsApp", iconURL: "https://www.freepngimg.com/thumb/whatsapp/77102-whatsapp-computer-call-telephone-icons-png-image-high-quality.png"}
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

	msg := &message{author: w.conn.Info.Wid, content: text}
	ch.messages = append(ch.messages, msg)
	w.ui.appendMessages([]*message{msg})
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
	msg := &message{content: m.Text, author: from}
	var ch *channel
	for _, c := range w.server.channels {
		if c.id == m.Info.RemoteJid {
			ch = c
			break
		}
	}
	if ch == nil {
		ch = &channel{id: m.Info.RemoteJid, name: m.Info.RemoteJid, server: w.server}
		w.server.channels = append(w.server.channels, ch)
	}
	ch.messages = append(ch.messages, msg)

	if ch == w.ui.currentChannel {
		w.ui.appendMessages([]*message{msg})
	}
}

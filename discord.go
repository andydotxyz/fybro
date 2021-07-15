package main

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/diamondburned/arikawa/gateway"

	discapi "github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/session"
)

const prefDiscordTokenKey = "auth.token"

type discord struct {
	app  fyne.App
	conn *session.Session
}

func initDiscord(a fyne.App) service {
	return &discord{app: a}
}

func (d *discord) disconnect() {
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

func (d *discord) loadChannels(u *ui) {
	for _, s := range u.data.servers {
		cs, _ := d.conn.Client.Channels(discapi.GuildID(s.id))
		for _, c := range cs {
			if c.Type == discapi.GuildCategory || c.Type == discapi.GuildVoice {
				continue // ignore voice and groupings for now
			}

			chn := &channel{id: int(c.ID), name: c.Name}
			if len(s.channels) == 0 {
				chn.messages = d.loadRecentMessages(c.ID)
				if s == u.currentServer {
					u.currentChannel = chn
					u.messages.Objects = nil
					u.appendMessages(u.currentChannel.messages)
				}
			}
			s.channels = append(s.channels, chn)
		}
	}
	u.channels.Refresh()

	for _, s := range u.data.servers {
		for i, c := range s.channels {
			if i == 0 {
				continue // we did this one above
			}
			c.messages = d.loadRecentMessages(discapi.ChannelID(c.id))
		}
	}
}

func (d *discord) loadRecentMessages(id discapi.ChannelID) []*message {
	ms, err := d.conn.Client.Messages(id, 15)
	if err != nil {
		return nil
	}

	var list []*message
	for i := len(ms) - 1; i >= 0; i-- { // newest message is first in response
		m := ms[i]
		msg := &message{author: m.Author.Username, content: m.Content,
			avatar: m.Author.AvatarURL()}
		list = append(list, msg)
	}

	return list
}

func (d *discord) loadServers(s *session.Session, u *ui) {
	d.conn = s

	var servers []*server
	gs, err := s.Client.Guilds(0)
	if err != nil {
		log.Println("Error getting guilds")
		return
	}
	for _, g := range gs {
		servers = append(servers, &server{service: d, name: g.Name, id: int(g.ID), iconURL: g.IconURL()})
	}

	if u.data == nil {
		u.data = &appData{}
	}
	u.data.servers = append(u.data.servers, servers...)
	u.currentServer = nil
	u.currentChannel = nil
	if len(u.data.servers) > 0 {
		u.currentServer = u.data.servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	err = s.Open()
	if err != nil {
		log.Println("Error opening session", err)
		d.conn = nil
		return
	}
	s.AddHandler(func(ev *gateway.MessageCreateEvent) {
		ch := findChan(u.data, int(ev.GuildID), int(ev.ChannelID))
		if ch == nil {
			log.Println("Could not find channel for incoming message")
			return
		}

		msg := &message{author: ev.Author.Username, content: ev.Content,
			avatar: ev.Author.AvatarURL()}
		ch.messages = append(ch.messages, msg)
		if ch == u.currentChannel {
			u.appendMessages([]*message{msg})
		}
	})

	d.loadChannels(u)
}

func (d *discord) login(w fyne.Window, prefix string, u *ui) {
	tok := d.app.Preferences().String(prefix + prefDiscordTokenKey)
	if tok != "" {
		sess, err := session.New(tok)
		if err == nil {
			d.loadServers(sess, u)
			return
		} else {
			log.Println("Error connecting with token", err)
		}
	}

	email := widget.NewEntry()
	pass := widget.NewPasswordEntry()
	dialog.ShowForm("Log in to Discord", "Log in", "cancel",
		[]*widget.FormItem{
			{Text: "Email", Widget: email},
			{Text: "Password", Widget: pass},
		}, func(ok bool) {
			if ok {
				d.doLogin(email.Text, pass.Text, w, d.app.Preferences(), prefix, u)
			}
		}, w)
}

func (d *discord) send(ch *channel, text string) {
	d.conn.SendText(discapi.ChannelID(ch.id), text)
}

func (d *discord) doLogin(email, pass string, w fyne.Window, p fyne.Preferences, prefix string, u *ui) {
	sess, err := session.Login(email, pass, "")
	if err == nil {
		p.SetString(prefix+prefDiscordTokenKey, sess.Token)
		d.loadServers(sess, u)
		return
	}

	if err != session.ErrMFA {
		log.Println("Login Err", err)
		return
	}

	mfa := widget.NewEntry()
	dialog.ShowForm("Multi-Factor required", "Confirm", "Cancel",
		[]*widget.FormItem{
			{Text: "Please enter your MFA token", Widget: mfa},
		},
		func(ok bool) {
			if !ok {
				return
			}
			sess, err := session.Login(email, pass, mfa.Text)
			if err != nil {
				log.Println("Failure in MFA verification")
				return
			}

			p.SetString(prefix+prefDiscordTokenKey, sess.Token)
			d.loadServers(sess, u)
		}, w)
}

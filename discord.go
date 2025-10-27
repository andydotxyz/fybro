package main

import (
	"context"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	discapi "github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
)

const prefDiscordTokenKey = "auth.token"

type discord struct {
	app  fyne.App
	conn *session.Session
	ctx  context.Context
}

func initDiscord(a fyne.App) service {
	return &discord{app: a, ctx: context.Background()}
}

func (d *discord) configure(u *ui) (fyne.CanvasObject, func(prefix string, a fyne.App)) {
	email := widget.NewEntry()
	pass := widget.NewPasswordEntry()
	return widget.NewForm(
			&widget.FormItem{Text: "Email", Widget: email},
			&widget.FormItem{Text: "Password", Widget: pass}),
		func(prefix string, a fyne.App) {
			d.doLogin(email.Text, pass.Text, d.app.Preferences(), prefix, u)
		}
}

func (d *discord) disconnect() {
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

func (d *discord) loadChannels(u *ui) {
	for _, s := range u.data.servers {
		id, _ := strconv.Atoi(s.id)
		cs, _ := d.conn.Client.Channels(discapi.GuildID(id))
		for _, c := range cs {
			if c.Type == discapi.GuildCategory || c.Type == discapi.GuildVoice {
				continue // ignore voice and groupings for now
			}

			chn := &channel{id: strconv.Itoa(int(c.ID)), name: "#" + c.Name, server: s}
			if len(s.channels) == 0 {
				chn.messages = d.loadRecentMessages(c.ID)
				if s == u.currentServer {
					u.setChannel(chn)
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

			id, _ := strconv.Atoi(c.id)
			c.messages = d.loadRecentMessages(discapi.ChannelID(id))
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
		msg := &message{content: m.Content, user: &user{
			name:      m.Author.Username,
			avatarURL: m.Author.AvatarURL()},
		}
		list = append(list, msg)
	}

	return list
}

func (d *discord) loadServers(s *session.Session, u *ui) {
	d.conn = s

	var servers []*server
	gs, err := s.Client.Guilds(0)
	if err != nil {
		fyne.LogError("Error getting guilds", err)
		return
	}
	for _, g := range gs {
		servers = append(servers, &server{service: d, name: g.Name, id: strconv.Itoa(int(g.ID)), iconURL: g.IconURL()})
	}

	if u.data == nil {
		u.data = &appData{}
	}
	u.data.servers = append(u.data.servers, servers...)
	if len(u.data.servers) > 0 {
		u.currentServer = u.data.servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	err = s.Open(d.ctx)
	if err != nil {
		fyne.LogError("Error opening session", err)
		d.conn = nil
		return
	}
	s.AddHandler(func(ev *gateway.MessageCreateEvent) {
		ch := findChan(u.data, strconv.Itoa(int(ev.GuildID)), strconv.Itoa(int(ev.ChannelID)))
		if ch == nil {
			fyne.LogError("Could not find channel for incoming message", nil)
			return
		}

		msg := &message{content: ev.Content, user: &user{
			name:      ev.Author.Username,
			avatarURL: ev.Author.AvatarURL()},
		}
		ch.messages = append(ch.messages, msg)
		if ch == u.currentChannel {
			u.appendMessages([]*message{msg})
		}
	})

	d.loadChannels(u)
}

func (d *discord) login(prefix string, u *ui) {
	tok := d.app.Preferences().String(prefix + prefDiscordTokenKey)
	if tok != "" {
		d.loadServers(session.New(tok), u)
	}
}

func (d *discord) send(ch *channel, text string) {
	id, _ := strconv.Atoi(ch.id)
	d.conn.SendMessage(discapi.ChannelID(id), text)
}

func (d *discord) doLogin(email, pass string, p fyne.Preferences, prefix string, u *ui) {
	sess, err := session.Login(d.ctx, email, pass, "")
	if err == nil {
		p.SetString(prefix+prefDiscordTokenKey, sess.Token)
		d.loadServers(sess, u)
		return
	}

	if err != session.ErrMFA {
		fyne.LogError("Login Err", err)
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
			sess, err := session.Login(d.ctx, email, pass, mfa.Text)
			if err != nil {
				fyne.LogError("Failure in MFA verification", err)
				return
			}

			p.SetString(prefix+prefDiscordTokenKey, sess.Token)
			d.loadServers(sess, u)
		}, u.win)
}

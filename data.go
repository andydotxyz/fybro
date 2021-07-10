package main

import (
	"log"

	"fyne.io/fyne/v2"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/diamondburned/arikawa/session"
)

type appData struct {
	servers []*server
}

type server struct {
	id            int
	name, iconURL string
	channels      []*channel
}

func (s *server) icon() fyne.Resource {
	// TODO cache this resource
	icon, err := fyne.LoadResourceFromURLString(s.iconURL)
	if err != nil {
		fyne.LogError("Failed to read icon "+s.iconURL, err)
		return nil
	}

	return icon
}

type channel struct {
	id       int
	name     string
	messages []*message
}

type message struct {
	content, author string
}

func (u *ui) loadChannels() {
	for _, s := range u.data.servers {
		cs, _ := u.conn.Client.Channels(discord.GuildID(s.id))
		for _, c := range cs {
			if c.Type == discord.GuildCategory || c.Type == discord.GuildVoice {
				continue // ignore voice and groupings for now
			}

			chn := &channel{id: int(c.ID), name: c.Name}
			if len(s.channels) == 0 {
				chn.messages = u.loadRecentMessages(c.ID)
				if s == u.currentServer {
					u.currentChannel = chn
					u.messages.Refresh()
					u.messages.ScrollToBottom()
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
			c.messages = u.loadRecentMessages(discord.ChannelID(c.id))
		}
	}
}

func (u *ui) loadRecentMessages(id discord.ChannelID) []*message {
	ms, err := u.conn.Client.Messages(id, 15)
	if err != nil {
		return nil
	}

	var list []*message
	for i := len(ms) - 1; i >= 0; i-- { // newest message is first in response
		m := ms[i]
		msg := &message{author: m.Author.Username, content: m.Content}
		list = append(list, msg)
	}

	return list
}

func loadServers(s *session.Session, u *ui) {
	var servers []*server
	gs, err := s.Client.Guilds(0)
	if err != nil {
		log.Println("Error getting guilds")
		return
	}
	for _, g := range gs {
		servers = append(servers, &server{name: g.Name, id: int(g.ID), iconURL: g.IconURL()})
	}

	u.data = &appData{servers: servers}
	u.currentServer = nil
	u.currentChannel = nil
	if len(servers) > 0 {
		u.currentServer = servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	u.conn = s
	err = s.Open()
	if err != nil {
		log.Println("Error opening session", err)
		u.conn = nil
		return
	}
	s.AddHandler(func(ev *gateway.MessageCreateEvent) {
		ch := findChan(u.data, int(ev.GuildID), int(ev.ChannelID))
		if ch == nil {
			log.Println("Could not find channel for incoming message")
			return
		}

		ch.messages = append(ch.messages, &message{author: ev.Author.Username, content: ev.Content})
		if ch == u.currentChannel {
			u.messages.Refresh()
			u.messages.ScrollToBottom()
		}
	})

	u.loadChannels()
}

func findChan(d *appData, sID, cID int) *channel {
	for _, s := range d.servers {
		if s.id == sID {
			for _, c := range s.channels {
				if c.id == cID {
					return c
				}
			}
		}
	}
	return nil
}

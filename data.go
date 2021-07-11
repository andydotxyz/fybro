package main

import (
	"log"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/diamondburned/arikawa/v2/discord"
	"github.com/diamondburned/arikawa/v2/gateway"
	"github.com/diamondburned/arikawa/v2/session"
	"github.com/diamondburned/arikawa/v2/state"
	"github.com/diamondburned/arikawa/v2/state/store/defaultstore"
	"github.com/diamondburned/ningen/v2"
	"github.com/diamondburned/ningen/v2/md"
)

type appData struct {
	servers []*server
}

type server struct {
	id            uint64
	name, iconURL string
	channels      []*channel

	avatar     fyne.Resource
	avatarMu   sync.Mutex
	avatarOnce sync.Once
}

func (s *server) setIconInto(img *canvas.Image) {
	s.avatarMu.Lock()
	avatar := s.avatar
	s.avatarMu.Unlock()

	if avatar != nil {
		img.Resource = avatar
	}

	s.avatarOnce.Do(func() {
		go func() {
			// TODO cache this resource
			icon, err := fyne.LoadResourceFromURLString(s.iconURL)
			if err != nil {
				fyne.LogError("Failed to read icon "+s.iconURL, err)
				return
			}

			s.avatarMu.Lock()
			s.avatar = icon
			s.avatarMu.Unlock()

			img.Refresh()
		}()
	})
}

type channel struct {
	id   uint64
	name string
}

type message struct {
	content string
	author  string
	avatar  string
}

func (u *ui) loadChannels() {
	for _, s := range u.data.servers {
		cs, _ := u.conn.Channels(discord.GuildID(s.id))
		for _, c := range cs {
			if c.Type != discord.GuildText {
				continue // ignore voice and groupings for now
			}

			chn := &channel{id: uint64(c.ID), name: c.Name}
			s.channels = append(s.channels, chn)
		}
	}
	u.channels.Refresh()
}

func (u *ui) renderMessage(m *discord.Message) *message {
	node := md.ParseWithMessage([]byte(m.Content), u.conn.Cabinet, m, true)
	var builder strings.Builder
	md.DefaultRenderer.Render(&builder, []byte(m.Content), node)

	return &message{author: m.Author.Username,
		content: strings.TrimSuffix(builder.String(), "\n"),
		avatar:  m.Author.AvatarURLWithType(discord.PNGImage) + "?size=64"}
}

func (u *ui) loadRecentMessages(id discord.ChannelID) []*message {
	ms, err := u.conn.Messages(id)
	if err != nil {
		return nil
	}

	var list []*message
	var subscribed bool

	for i := len(ms) - 1; i >= 0; i-- { // newest message is first in response
		m := ms[i]
		list = append(list, u.renderMessage(&m))

		if !subscribed && m.GuildID.IsValid() {
			u.conn.MemberState.Subscribe(m.GuildID)
			subscribed = true
		}
	}

	return list
}

func loadServers(session *session.Session, u *ui) {
	cabinet := defaultstore.New()
	cabinet.MessageStore = defaultstore.NewMessage(25) // 25 messages max

	state := state.NewFromSession(session, cabinet)

	gs, err := state.Guilds()
	if err != nil {
		log.Println("Error getting guilds")
		return
	}

	var servers []*server
	for _, g := range gs {
		servers = append(servers, &server{name: g.Name, id: uint64(g.ID),
			iconURL: g.IconURLWithType(discord.PNGImage) + "?size=64"})
	}

	u.data = &appData{servers: servers}
	u.currentServer = nil
	u.currentChannel = nil
	if len(servers) > 0 {
		u.currentServer = servers[0]
		u.servers.Select(0)
	}
	u.servers.Refresh()

	u.conn, err = ningen.FromState(state)
	if err != nil {
		log.Println("Error wrapping Discord session", err)
		return
	}

	u.conn.AddHandler(func(msg *gateway.MessageCreateEvent) {
		u.currentMu.Lock()
		defer u.currentMu.Unlock()

		if u.currentChannel == nil ||
			uint64(msg.ChannelID) != u.currentChannel.id {
			return
		}

		u.appendMessages([]*message{
			u.renderMessage(&msg.Message),
		})
	})

	if err = u.conn.Open(); err != nil {
		log.Println("Error opening session", err)
		u.conn = nil
		return
	}

	u.loadChannels()
}

func findChan(d *appData, sID, cID uint64) *channel {
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

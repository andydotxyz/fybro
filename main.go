//go:generate fyne bundle -o bundled.go Icon.png

package main

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/diamondburned/arikawa/session"
)

const (
	prefServerCountKey  = "server.count"
	prefServerPrefix    = "server.%d."
	prefServerTypeKey   = "type"
	prefDiscordTokenKey = "auth.token"
)

func main() {
	a := app.NewWithID("xyz.andy.fibro")
	a.SetIcon(resourceIconPng)
	w := a.NewWindow("Fibro: Discord")

	u := &ui{}
	w.SetContent(u.makeUI())
	w.Resize(fyne.NewSize(480, 320))
	go u.runLogins(w, a)
	w.ShowAndRun()

	// after app quits
	if u.conn != nil {
		_ = u.conn.Close()
	}
}

func (u *ui) runLogins(w fyne.Window, a fyne.App) {
	count := a.Preferences().Int(prefServerCountKey)
	if count == 0 {
		prefix := fmt.Sprintf(prefServerPrefix, 0)
		loginDiscord(w, a.Preferences(), prefix, u)
		a.Preferences().SetInt(prefServerCountKey, 1) // TODO handle actual server add
		a.Preferences().SetString(prefix+prefServerTypeKey, "discord")
	}
	for i := 0; i < count; i++ {
		prefPrefix := fmt.Sprintf(prefServerPrefix, i)
		typeKey := prefPrefix + prefServerTypeKey

		switch a.Preferences().String(typeKey) {
		case "discord":
			loginDiscord(w, a.Preferences(), prefPrefix, u)
		default:
			fyne.LogError("Unknown server type", nil)
		}
	}
}

func loginDiscord(w fyne.Window, p fyne.Preferences, prefix string, u *ui) {
	tok := p.String(prefix + prefDiscordTokenKey)
	if tok != "" {
		sess, err := session.New(tok)
		if err == nil {
			loadServers(sess, u)
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
				doLogin(email.Text, pass.Text, w, p, prefix, u)
			}
		}, w)
}

func doLogin(email, pass string, w fyne.Window, p fyne.Preferences, prefix string, u *ui) {
	sess, err := session.Login(email, pass, "")
	if err == nil {
		p.SetString(prefix+prefDiscordTokenKey, sess.Token)
		loadServers(sess, u)
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
			loadServers(sess, u)
		}, w)
}

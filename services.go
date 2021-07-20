package main

import (
	"errors"

	"fyne.io/fyne/v2"
)

type service interface {
	configure(*ui) (fyne.CanvasObject, func(prefix string, a fyne.App))
	disconnect()
	login(prefix string, u *ui)
	send(*channel, string)
}

var (
	connected []service
	services  = map[string]func(fyne.App) service{
		"discord":  initDiscord,
		"telegram": initTelegram,
		"whatsapp": initWhatsApp,
	}
)

func connect(id string, a fyne.App) (service, error) {
	srv, ok := services[id]
	if !ok {
		return nil, errors.New("unknown server id " + id)
	}

	ret := srv(a)
	connected = append(connected, ret)
	return ret, nil
}

func disconnectAll() {
	live := connected
	connected = nil
	for _, srv := range live {
		srv.disconnect()
	}
}

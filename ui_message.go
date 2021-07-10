package main

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	resCache = map[string]fyne.Resource{}
	resCacheLock sync.RWMutex
)

type messageCell struct {
	widget.BaseWidget
	msg *message
}

func newMessageCell(m *message) *messageCell {
	ret := &messageCell{msg: m}
	ret.ExtendBaseWidget(ret)
	return ret
}

func (m *messageCell) avatarResource() fyne.Resource {
	if m.msg.avatar == "" {
		return nil
	}

	resCacheLock.RLock()
	ret, ok := resCache[m.msg.avatar]
	resCacheLock.RUnlock()
	if ok {
		return ret
	}
	url, err := storage.ParseURI(m.msg.avatar)
	if err != nil || url == nil {
		return nil
	}
	ret, _ = storage.LoadResourceFromURI(url)
	resCacheLock.Lock()
	resCache[m.msg.avatar] = ret
	resCacheLock.Unlock()
	return ret
}

func (m *messageCell) setMessage(new *message) {
	m.msg = new
	m.Refresh()
}

func (m *messageCell) CreateRenderer() fyne.WidgetRenderer {
	return &messageRenderer{m: m,
		top: widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		main: widget.NewLabel(""), pic: widget.NewIcon(nil)}
}

type messageRenderer struct {
	m *messageCell
	top, main *widget.Label
	pic       *widget.Icon
}

func (m *messageRenderer) Destroy() {
}

func (m *messageRenderer) Layout(s fyne.Size) {
	iconSize := float32(32)
	remainWidth := s.Width - iconSize - theme.Padding() * 2
	remainStart := iconSize+theme.Padding()*2
	m.pic.Resize(fyne.NewSize(iconSize, iconSize))
	m.pic.Move(fyne.NewPos(theme.Padding(), theme.Padding()))
	m.top.Move(fyne.NewPos(remainStart, -theme.Padding()))
	m.top.Resize(fyne.NewSize(remainWidth, m.top.MinSize().Height))
	m.main.Move(fyne.NewPos(remainStart, m.top.MinSize().Height-theme.Padding()*4))
	m.main.Resize(fyne.NewSize(remainWidth, m.main.MinSize().Height))
}

func (m *messageRenderer) MinSize() fyne.Size {
	s := m.top.MinSize()
	return fyne.NewSize(s.Width, s.Height*2 - theme.Padding()*4)
}

func (m *messageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{m.top, m.main, m.pic}
}

func (m *messageRenderer) Refresh() {
	m.top.SetText(m.m.msg.author)
	m.main.SetText(m.m.msg.content)
	go m.pic.SetResource(m.m.avatarResource())
}



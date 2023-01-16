package main

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const iconSize = float32(32)

var (
	resCache     = map[string]fyne.Resource{}
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
	if m.msg.user.avatarURL == "" {
		return nil
	}

	resCacheLock.RLock()
	ret, ok := resCache[m.msg.user.avatarURL]
	resCacheLock.RUnlock()
	if ok {
		return ret
	}
	url, err := storage.ParseURI(m.msg.user.avatarURL)
	if err != nil || url == nil {
		return nil
	}
	ret, _ = storage.LoadResourceFromURI(url)
	resCacheLock.Lock()
	resCache[m.msg.user.avatarURL] = ret
	resCacheLock.Unlock()
	return ret
}

func (m *messageCell) setMessage(new *message) {
	m.msg = new
	m.Refresh()
}

func (m *messageCell) CreateRenderer() fyne.WidgetRenderer {
	name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Wrapping = fyne.TextTruncate
	body := widget.NewRichText()
	body.Wrapping = fyne.TextWrapWord
	return &messageRenderer{m: m,
		top:  name,
		main: body, pic: widget.NewIcon(nil), sep: widget.NewSeparator()}
}

type messageRenderer struct {
	m    *messageCell
	top  *widget.Label
	main *widget.RichText
	pic  *widget.Icon
	sep  *widget.Separator
}

func (m *messageRenderer) Destroy() {
}

func (m *messageRenderer) Layout(s fyne.Size) {
	remainWidth := s.Width - iconSize - theme.Padding()*2
	remainStart := iconSize + theme.Padding()*2
	m.pic.Resize(fyne.NewSize(iconSize, iconSize))
	m.pic.Move(fyne.NewPos(theme.Padding(), theme.Padding()))
	m.top.Move(fyne.NewPos(remainStart, -theme.Padding()))
	m.top.Resize(fyne.NewSize(remainWidth, m.top.MinSize().Height))
	m.main.Move(fyne.NewPos(remainStart, m.top.MinSize().Height-theme.Padding()*4))
	m.main.Resize(fyne.NewSize(remainWidth, m.main.MinSize().Height))
	m.sep.Move(fyne.NewPos(0, s.Height-theme.SeparatorThicknessSize()))
	m.sep.Resize(fyne.NewSize(s.Width, theme.SeparatorThicknessSize()))
}

func (m *messageRenderer) MinSize() fyne.Size {
	s1 := m.top.MinSize()
	s2 := m.main.MinSize()
	w := fyne.Max(s1.Width, s2.Width)
	return fyne.NewSize(w+iconSize+theme.Padding()*2,
		s1.Height+s2.Height-theme.Padding()*4)
}

func (m *messageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{m.top, m.main, m.pic, m.sep}
}

func (m *messageRenderer) Refresh() {
	if m.m.msg.user.name != "" {
		m.top.SetText(m.m.msg.user.name)
	} else {
		m.top.SetText(m.m.msg.user.username)
	}
	m.main.ParseMarkdown(m.m.msg.content)
	go m.pic.SetResource(m.m.avatarResource())
}

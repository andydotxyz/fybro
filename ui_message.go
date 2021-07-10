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
	body := widget.NewLabel("")
	body.Wrapping = fyne.TextWrapWord
	return &messageRenderer{m: m,
		top: widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		main: body, pic: widget.NewIcon(nil), sep: widget.NewSeparator()}
}

type messageRenderer struct {
	m *messageCell
	top, main *widget.Label
	pic       *widget.Icon
	sep       *widget.Separator
}

func (m *messageRenderer) Destroy() {
}

func (m *messageRenderer) Layout(s fyne.Size) {
	remainWidth := s.Width - iconSize - theme.Padding() * 2
	remainStart := iconSize+theme.Padding()*2
	m.pic.Resize(fyne.NewSize(iconSize, iconSize))
	m.pic.Move(fyne.NewPos(theme.Padding(), theme.Padding()))
	m.top.Move(fyne.NewPos(remainStart, -theme.Padding()))
	m.top.Resize(fyne.NewSize(remainWidth, m.top.MinSize().Height))
	m.main.Move(fyne.NewPos(remainStart, m.top.MinSize().Height-theme.Padding()*4))
	m.main.Resize(fyne.NewSize(remainWidth, m.main.MinSize().Height))
	m.sep.Move(fyne.NewPos(0, s.Height - theme.SeparatorThicknessSize()))
	m.sep.Resize(fyne.NewSize(s.Width, theme.SeparatorThicknessSize()))
}

func (m *messageRenderer) MinSize() fyne.Size {
	s1 := m.top.MinSize()
	s2 := m.main.MinSize()
	w := fyne.Max(s1.Width, s2.Width)
	return fyne.NewSize(w + iconSize + theme.Padding()*2,
		s1.Height + s2.Height - theme.Padding()*4)
}

func (m *messageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{m.top, m.main, m.pic, m.sep}
}

func (m *messageRenderer) Refresh() {
	m.top.SetText(m.m.msg.author)
	m.main.SetText(m.m.msg.content)
	go m.pic.SetResource(m.m.avatarResource())
}



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
	msg   *message
	small bool
}

func newMessageCell(m *message) *messageCell {
	ret := &messageCell{msg: m}
	ret.ExtendBaseWidget(ret)
	return ret
}

func (m *messageCell) avatarResource() fyne.Resource {
	if m.msg.avatar == "" || m.small {
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

	r := &messageRenderer{m: m, main: body}
	if !m.small {
		r.top = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		r.sep = widget.NewSeparator()
		r.pic = widget.NewIcon(nil)
	}
	return r
}

type messageRenderer struct {
	m         *messageCell
	top, main *widget.Label
	pic       *widget.Icon
	sep       *widget.Separator
}

func (m *messageRenderer) Destroy() {
}

func (m *messageRenderer) Layout(s fyne.Size) {
	remainWidth := s.Width - iconSize - theme.Padding()*2
	remainStart := iconSize + theme.Padding()*2

	if m.m.small {
		m.main.Move(fyne.NewPos(remainStart, -theme.Padding()*4))
		m.main.Resize(fyne.NewSize(remainWidth, m.main.MinSize().Height))
		return
	}

	m.sep.Move(fyne.NewPos(0, -theme.SeparatorThicknessSize()))
	m.sep.Resize(fyne.NewSize(s.Width, theme.SeparatorThicknessSize()))
	m.pic.Resize(fyne.NewSize(iconSize, iconSize))
	m.pic.Move(fyne.NewPos(theme.Padding(), theme.Padding()))
	m.top.Move(fyne.NewPos(remainStart, -theme.Padding()))
	m.top.Resize(fyne.NewSize(remainWidth, m.top.MinSize().Height))
	m.main.Move(fyne.NewPos(remainStart, m.top.MinSize().Height-theme.Padding()*4))
	m.main.Resize(fyne.NewSize(remainWidth, m.main.MinSize().Height))
}

func (m *messageRenderer) MinSize() fyne.Size {
	size := m.main.MinSize()
	w := size.Width
	h := size.Height

	if !m.m.small {
		size := m.top.MinSize()
		w = fyne.Max(w, size.Width)
		h += size.Height
	}

	return fyne.NewSize(w+iconSize+theme.Padding()*2, h-theme.Padding()*4)
}

func (m *messageRenderer) Objects() []fyne.CanvasObject {
	if m.m.small {
		return []fyne.CanvasObject{m.main}
	}
	return []fyne.CanvasObject{m.top, m.main, m.pic, m.sep}
}

func (m *messageRenderer) Refresh() {
	m.main.SetText(m.m.msg.content)

	if !m.m.small {
		m.top.SetText(m.m.msg.author)
		go m.pic.SetResource(m.m.avatarResource())
	}
}

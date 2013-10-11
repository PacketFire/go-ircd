package main

import (
	"bytes"
	"sync"
)

// A mode set.
type Modeset struct {
	modes map[rune]string
	mm    sync.RWMutex
}

func NewModeset() Modeset {
	return Modeset{modes: make(map[rune]string)}
}

func (m *Modeset) Get(r rune) (val string, ok bool) {
	m.mm.RLock()
	defer m.mm.RUnlock()

	val, ok = m.modes[r]
	return
}

func (m *Modeset) Set(r rune, val string) (old string) {
	m.mm.Lock()
	defer m.mm.Unlock()

	old = m.modes[r]
	m.modes[r] = val
	return
}

func (m *Modeset) Clear(r rune) (old string) {
	m.mm.Lock()
	defer m.mm.Unlock()

	old = m.modes[r]
	delete(m.modes, r)
	return
}

// Get a string like +lL 10,#overflow for a channel
// or like +iwx for a user
// todo: add params
func (m *Modeset) GetString() (modes, params string) {
	m.mm.RLock()
	defer m.mm.RUnlock()

	modebuf := bytes.NewBufferString("+")
	parambuf := bytes.NewBufferString("")

	for v, _ := range m.modes {
		modebuf.WriteRune(v)
	}

	return modebuf.String(), parambuf.String()
}

package ircd

import (
	"bytes"
	"sync"
)

// A mode set.
type Modeset struct {
	modes map[rune]string
	mu    sync.Mutex
}

func NewModeset() *Modeset {
	return &Modeset{
		modes: make(map[rune]string),
	}
}

func (m *Modeset) Get(r rune) (val string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	val, ok = m.modes[r]
	return
}

func (m *Modeset) Set(r rune, val string) (old string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	old = m.modes[r]
	m.modes[r] = val
	return
}

func (m *Modeset) Clear(r rune) (old string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	old = m.modes[r]
	delete(m.modes, r)
	return
}

// Get a string like +lL 10,#overflow for a channel
// or like +iwx for a user
// todo: add params
func (m *Modeset) GetString() (modes, params string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	modebuf := bytes.NewBufferString("+")
	parambuf := bytes.NewBufferString("")

	for v, _ := range m.modes {
		modebuf.WriteRune(v)
	}

	return modebuf.String(), parambuf.String()
}

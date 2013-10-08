package main

// A mode set.
type Modeset struct {
	modes map[rune]string
}

func NewModeset() Modeset {
	return Modeset{make(map[rune]string)}
}

func (m *Modeset) Get(r rune) (val string, ok bool) {
	val, ok = m.modes[r]
	return
}

func (m *Modeset) Set(r rune, val string) (old string) {
	old = m.modes[r]
	m.modes[r] = val
	return
}

func (m *Modeset) Clear(r rune) (old string) {
	old = m.modes[r]
	delete(m.modes, r)
	return
}

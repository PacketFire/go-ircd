package main

import (
	"strings"
	"testing"
)

func TestModeset(t *testing.T) {
	m := NewModeset()

	modes := []rune{'i', 'w', 'x'}

	for _, r := range modes {
		m.Set(r, "")

		if _, ok := m.Get(r); !ok {
			t.Errorf("mode %c wasn't set", r)
		}
	}

	modestr, _ := m.GetString()

	for _, r := range modes {
		if strings.IndexRune(modestr, r) == -1 {
			t.Errorf("mode %c not found in modestr %q", r, modestr)
		}
	}
}

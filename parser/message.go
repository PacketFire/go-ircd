package parser

import (
	"bytes"
	"fmt"
)

type Message struct {
	Prefix  string
	Command string
	Args    []string
}

func (m Message) String() string {
	return fmt.Sprintf("prefix: %q command: %q args %#v", m.Prefix, m.Command, m.Args)
}

func (m *Message) UnmarshalText(b []byte) error {

	var s [][]byte

	if b[0] == ':' {
		s = bytes.SplitN(b[1:], []byte{' '}, 2)
		m.Prefix = string(s[0])
		b = s[1]
	}

	s = bytes.SplitN(b, []byte{' '}, 2)
	m.Command = string(s[0])
	b = s[1]

	for {
		if b[0] == ':' {
			m.Args = append(m.Args, string(b))
			break
		} else {
			s = bytes.SplitN(b, []byte{' '}, 2)

			if len(s) == 0 {
				break
			}

			m.Args = append(m.Args, string(s[0]))

			if len(s) < 2 {
				break
			}

			b = s[1]
		}
	}

	return nil
}

func (m *Message) MarshalText() (text []byte, err error) {
	var b bytes.Buffer

	if len(m.Prefix) > 0 {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
		b.WriteByte(' ')
	}

	b.WriteString(m.Command)

	for _, arg := range m.Args {
		b.WriteByte(' ')
		b.WriteString(arg)
	}

	return b.Bytes(), nil
}

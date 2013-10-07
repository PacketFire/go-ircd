package parser

import (
	"bytes"
	"fmt"
	"strings"
)

type Message struct {
	Prefix  string
	Command string

	Args []string
}

func (m Message) String() string {
	return fmt.Sprintf("prefix:%q command:%q args:%#v", m.Prefix, m.Command, m.Args)
}

func (m *Message) UnmarshalText(b []byte) error {
	b = bytes.TrimSpace(b)

	if len(b) <= 0 {
		return fmt.Errorf("short line")
	}

	if b[0] == ':' {
		split := bytes.SplitN(b, []byte{' '}, 2)
		if len(split) <= 1 {
			return fmt.Errorf("short line")
		}
		m.Prefix = string(split[0][1:])
		b = split[1]
	}
	split := bytes.SplitN(b, []byte{':'}, 2)
	args := bytes.Split(bytes.TrimSpace(split[0]), []byte{' '})
	m.Command = string(bytes.ToUpper(args[0]))
	m.Args = make([]string, 0, len(args))
	for _, arg := range args[1:] {
		m.Args = append(m.Args, string(arg))
	}
	if len(split) > 1 {
		m.Args = append(m.Args, string(split[1]))
	}

	return nil
}

func (m *Message) MarshalText() (text []byte, err error) {
	var buf bytes.Buffer
	if len(m.Prefix) > 0 {
		buf.WriteByte(':')
		buf.WriteString(m.Prefix)
		buf.WriteByte(' ')
	}
	buf.WriteString(m.Command)
	for i, arg := range m.Args {
		buf.WriteByte(' ')
		if i == len(m.Args)-1 {
			if strings.IndexAny(arg, " :") >= 0 {
				buf.WriteByte(':')
			}
		}
		buf.WriteString(m.Args[i])
	}
	return buf.Bytes(), nil
}

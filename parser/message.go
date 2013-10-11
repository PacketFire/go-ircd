// IRC Protocol message marshaller and unmarshaller.
// refer to http://tools.ietf.org/html/rfc1459#section-2.3.1 for
// information on the format of protocol messages
// TODO(mischief): add IRCv3.2 tags as per http://ircv3.org/specification/message-tags-3.2
package parser

import (
	"bytes"
	"fmt"
	"strings"
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
	if len(b) < 1 {
		/*  Empty  messages  are  silently  ignored, which
				    permits use of the sequence CR-LF between messages
		        without extra problems. */
		return nil
	}

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
	var b bytes.Buffer

	if len(m.Prefix) > 0 {
		b.WriteByte(':')
		b.WriteString(m.Prefix)
	}

	if len(m.Prefix) > 0 && len(m.Command) > 0 {
		b.WriteByte(' ')
	}

	b.WriteString(m.Command)

	for i, arg := range m.Args {
		b.WriteByte(' ')
		if strings.Index(arg, " ") >= 0 || strings.Index(arg, ":") == 0 {
			if i == len(m.Args)-1 {
				b.WriteByte(':')
			} else {
				// a parameter has a space or : and it's not the last arg - wrong
				return nil, fmt.Errorf("space or : in arg %d %q", i, arg)
			}
		}
		b.WriteString(arg)
	}

	return b.Bytes(), nil
}

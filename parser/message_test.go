package parser

import (
	"bytes"
	"testing"
)

type mtest struct {
	line []byte
	msg  Message
}

var (
	mtests = []mtest{
		mtest{
			[]byte(":the.server.name NOTICE * :*** Looking up your hostname..."),
			Message{"the.server.name", "NOTICE", []string{"*", "*** Looking up your hostname..."}},
		},
	}
)

func TestUnmarshalMessage(t *testing.T) {
	for _, ut := range mtests {
		var m Message
		if err := m.UnmarshalText(ut.line); err != nil {
			t.Fatalf("unmarshal failed: %s", err)
		}

		t.Logf("unmarshal %q got %#v", ut.line, m)

		if m.Prefix != ut.msg.Prefix {
			t.Error("bad prefix")
		}

		if m.Command != ut.msg.Command {
			t.Error("bad command")
		}

		if len(m.Args) != len(ut.msg.Args) {
			t.Error("bad arg count")
		}
	}
}

func TestMarshalMessage(t *testing.T) {
	for _, mt := range mtests {
		line, err := mt.msg.MarshalText()

		if err != nil {
			t.Fatalf("marshal failed: %s", err)
		}

		if bytes.Compare(line, mt.line) != 0 {
			t.Error("different lines")
		}
	}
}

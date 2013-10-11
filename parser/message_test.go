// Mesage marshall/unmarshall tests
//
// TODO(mischief): add more tests from https://raw.github.com/grawity/code/master/lib/test-irc-join.txt
package parser

import (
	"bytes"
	"strings"
	"testing"
)

type mtest struct {
	line        []byte
	msg         Message
	shouldFail  bool
	skipMarshal bool
}

var (
	mtests = []mtest{

		mtest{
			[]byte(":nick!user@host.com PRIVMSG #42o3L1t3 :l0l sw4g 0m9 s0 c00l 42o"),
			Message{"nick!user@host.com", "PRIVMSG", []string{"#42o3L1t3", "l0l sw4g 0m9 s0 c00l 42o"}},
			false,
			false,
		},

		mtest{
			[]byte("COMMAND :no arguments here!"),
			Message{"", "COMMAND", []string{"no arguments here!"}},
			false,
			false,
		},

		mtest{
			[]byte("COMMAND nothing but arguments!"),
			Message{"", "COMMAND", []string{"nothing", "but", "arguments!"}},
			false,
			false,
		},

		// empty - valid per rfc
		mtest{
			[]byte(""),
			Message{"", "", nil},
			false,
			false,
		},

		// empty prefix - short message
		mtest{
			[]byte(":"),
			Message{":", "", nil},
			true,
			true,
		},

		// prefix, no command - short message
		mtest{
			[]byte(":foo"),
			Message{"foo", "", nil},
			true,
			false,
		},

		/* no tags for now */
		/*
		   mtest{
		     []byte("@foo"),
		     Message{"@foo", "", []string{""}},
		     false,
		   },

		   mtest{
		     []byte("@foo :bar"),
		     Message{"", "", []string{""}},
		     false,
		   },
		*/

		mtest{
			[]byte("foo bar baz asdf"),
			Message{"", "foo", []string{"bar", "baz", "asdf"}},
			false,
			false,
		},

		mtest{
			[]byte("foo bar baz :asdf quux"),
			Message{"", "foo", []string{"bar", "baz", "asdf quux"}},
			false,
			false,
		},

		mtest{
			[]byte("foo bar baz"),
			Message{"", "foo", []string{"bar", "baz"}},
			false,
			false,
		},

		mtest{
			[]byte("foo bar baz ::asdf"),
			Message{"", "foo", []string{"bar", "baz", ":asdf"}},
			false,
			false,
		},

		mtest{
			[]byte(":test foo bar baz asdf"),
			Message{"test", "foo", []string{"bar", "baz", "asdf"}},
			false,
			false,
		},

		mtest{
			[]byte(":test foo bar baz :asdf quux"),
			Message{"test", "foo", []string{"bar", "baz", "asdf quux"}},
			false,
			false,
		},

		mtest{
			[]byte(":test foo bar baz"),
			Message{"test", "foo", []string{"bar", "baz"}},
			false,
			false,
		},

		mtest{
			[]byte(":test foo bar baz ::asdf"),
			Message{"test", "foo", []string{"bar", "baz", ":asdf"}},
			false,
			false,
		},

		mtest{
			[]byte(":foo bar"),
			Message{"foo", "bar", nil},
			false,
			false,
		},

		mtest{
			[]byte(":foo :bar baz"),
			Message{"foo", "", []string{"bar baz"}},
			false,
			false,
		},

		mtest{
			[]byte(":foo :bar baz"),
			Message{"foo", "", []string{"bar baz"}},
			false,
			false,
		},

		mtest{
			[]byte(":foo :bar baz"),
			Message{"foo", "", []string{"bar baz"}},
			false,
			false,
		},
	}
)

func TestUnmarshalText(t *testing.T) {
	for _, ut := range mtests {
		var m Message

		if err := m.UnmarshalText(ut.line); err != nil {
			if ut.shouldFail {
				t.Logf("expected failure: on %q: %s", ut.line, err)
				continue
			}
			t.Fatalf("unmarshal failed on %q: %s", ut.line, err)
		}

		t.Logf("unmarshal %q got %#v", ut.line, m)

		if m.Prefix != ut.msg.Prefix {
			t.Fatalf("Bad prefix")
		}

		if m.Command != strings.ToUpper(ut.msg.Command) {
			t.Fatalf("Bad command")
		}

		if len(m.Args) != len(ut.msg.Args) {
			t.Fatalf("Bad Arguments count")
		}

		for i, arg := range m.Args {

			if arg != ut.msg.Args[i] {
				t.Fatalf("Bad argument %v", i)
			}
		}
	}
}

func TestMarshalMessage(t *testing.T) {
	for _, mt := range mtests {
		if mt.skipMarshal {
			continue
		}

		line, err := mt.msg.MarshalText()

		if err != nil && mt.shouldFail {
			t.Logf("expected failure on %s: %s", mt.msg, err)
			continue
		}

		if err != nil {
			t.Errorf("marshal %s failed: %s", mt.msg, err)
			continue
		}

		if bytes.Compare(line, mt.line) != 0 {
			t.Errorf("different lines: expected %q got %q", mt.line, line)
		}
	}
}

var unmarshaltext_message_bench = []byte(":server.kevlar.net NOTICE user :*** This is a test")

func BenchmarkUnmarshalText(b *testing.B) {
	var m Message
	for i := 0; i < b.N; i++ {
		m.UnmarshalText(unmarshaltext_message_bench)
	}

}

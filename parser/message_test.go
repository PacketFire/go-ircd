package parser
import (
	"testing"
)

type mtest struct {
	line []byte
	msg Message
}

var (
	
	mtests = []mtest{

		mtest{
			[]byte(":nick!user@host.com PRIVMSG #42o3L1t3 :l0l sw4g 0m9 s0 c00l 42o"),
			Message2{"nick!user@host.com", "PRIVMSG", []string{"#42o3L1t3", ":l0l sw4g 0m9 s0 c00l 42o"}},
		},
		
		mtest{
			[]byte("COMMAND :no arguments here!"),
			Message2{"", "COMMAND", []string{":no arguments here!"}},
		},

		mtest{
			[]byte("COMMAND nothing but arguments!"),
			Message2{"", "COMMAND", []string{"nothing", "but", "arguments!"}},
		},

	}

)

func TestUnmarshalText(t *testing.T) {

	for _, ut := range mtests {
		var m Message

		if err := m.UnmarshalText(ut.line); err != nil {
			t.Fatalf("unmarshal failed: %s", err)
		}

		t.Logf("unmarshal %q got %#v", ut.line, m)

		if m.Prefix != ut.msg.Prefix {
			t.Fatalf("Bad prefix")
		}

		if m.Command != ut.msg.Command {
			t.Fatalf("Bad command")
		}

		if len(m.Args) != len(ut.msg.Args) {
			t.Fatalf("Bad Arguments count")
		}

		for i, arg := range m.Args {

			if (arg != ut.msg.Args[i]) {
				t.Fatalf("Bad argument %v", i)
			}
		}
	}
}

var unmarshaltext_message_bench = []byte(":server.kevlar.net NOTICE user :*** This is a test")

func BenchmarkUnmarshalText (b *testing.B) {
	var m Message
	for i := 0; i < b.N; i++ {
		m.UnmarshalText(unmarshaltext_message_bench)
	}

}

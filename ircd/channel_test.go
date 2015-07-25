package ircd

import (
	"sync"
	"testing"

	"github.com/PacketFire/go-ircd/parser"
)

type fakeClient struct {
	sync.Mutex

	nick string
	host string

	msgs []string
}

func (f *fakeClient) Nick() string {
	f.Lock()
	defer f.Unlock()
	return f.nick
}

func (f *fakeClient) SetNick(nick string) string {
	f.Lock()
	defer f.Unlock()
	old := f.nick
	f.nick = nick
	return old
}

func (f *fakeClient) User() string {
	return f.Nick()
}

func (f *fakeClient) SetUser(user string) string {
	return f.SetNick(user)
}

func (f *fakeClient) Real() string {
	return f.Nick()
}

// oops
func (f *fakeClient) SetReal(realname string) string {
	return f.SetNick(realname)
}

func (f *fakeClient) Host() string {
	f.Lock()
	defer f.Unlock()
	return f.host
}

func (f *fakeClient) Prefix() string {
	f.Lock()
	defer f.Unlock()
	return f.nick + "!" + f.nick + "@" + f.host
}

func (f *fakeClient) Send(prefix, command string, args ...string) error {
	f.Lock()
	defer f.Unlock()

	m := parser.Message{Prefix: prefix, Command: command, Args: args}
	b, err := m.MarshalText()
	if err != nil {
		return err
	}

	f.msgs = append(f.msgs, string(b))
	return nil
}

// testing recieving messages on a client
func (f *fakeClient) popmsg() string {
	if len(f.msgs) == 0 {
		return ""
	}

	m := f.msgs[0]
	f.msgs = f.msgs[1:]
	return m
}

func (f *fakeClient) Join(c Channel) {
}

func (f *fakeClient) Part(c Channel) {
}

func (f *fakeClient) Channels() []Channel {
	return nil
}

func TestChannelTopic(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	ch := NewChannel(srv, "testserver", "#test")
	if ch == nil {
		t.Errorf("error creating channel")
		return
	}

	ch.SetTopic("hello world!")
	if ch.Topic() != "hello world!" {
		t.Errorf("setting topic failed")
		return
	}
}

func TestChannelJoinPart(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	ch := NewChannel(srv, "testserver", "#test")
	if ch == nil {
		t.Errorf("error creating channel")
		return
	}

	u := &fakeClient{nick: "test", host: "127.0.0.1"}

	ch.Join(u)
	if list := ch.Users(); len(list) != 1 {
		t.Errorf("bad number of users in channel %v %v", len(list), list)
		return
	}

	ch.Part(u)
	if list := ch.Users(); len(list) != 0 {
		t.Errorf("bad number of users in channel %v %v", len(list), list)
		return
	}

	err := ch.Close()
	if err != nil {
		t.Errorf("error stopping channel: %v", err)
		return
	}
}

func TestChannelDoubleJoin(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	ch := NewChannel(srv, "testserver", "#test")
	if ch == nil {
		t.Errorf("error creating channel")
		return
	}

	u := &fakeClient{nick: "test", host: "127.0.0.1"}
	if err := ch.Join(u); err != nil {
		t.Errorf("error joining channel: %v", err)
		return
	}

	if err := ch.Join(u); err == nil {
		t.Errorf("joining channel twice should error!")
		return
	}
}

func TestChannelPrivmsg(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	ch := NewChannel(srv, "testserver", "#test")
	if ch == nil {
		t.Errorf("error creating channel")
		return
	}

	u1 := &fakeClient{nick: "test1", host: "127.0.0.1"}
	u2 := &fakeClient{nick: "test2", host: "127.0.0.1"}

	ch.Join(u1)
	ch.Join(u2)

	if list := ch.Users(); len(list) != 2 {
		t.Errorf("bad number of users in channel %v %v", len(list), list)
		return
	}

	ch.Send(u1.Prefix(), "PRIVMSG", ch.Name(), "hello")

	gothello := false
	for msg := u2.popmsg(); msg != ""; msg = u2.popmsg() {
		if msg == ":test1!test1@127.0.0.1 PRIVMSG #test hello" {
			gothello = true
		}
	}

	if !gothello {
		t.Errorf("did not recieve privmsg")
		return
	}

	ch.Part(u1)
	ch.Part(u2)

	err := ch.Close()
	if err != nil {
		t.Errorf("error stopping channel: %v", err)
		return
	}
}

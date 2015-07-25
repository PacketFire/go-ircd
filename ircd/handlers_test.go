package ircd

import (
	"testing"

	"github.com/PacketFire/go-ircd/parser"
)

type mockServer struct {
	DoName         func() string
	DoAddClient    func(c Client) error
	DoRemoveClient func(c Client) error
	DoHandle       func(c Client, m parser.Message) error
	DoChannels     func() map[string]Channel
	DoFindChannel  func(ch string) Channel
	DoAddChannel   func(ch Channel)
	DoFindByNick   func(nick string) Client
	DoPrivmsg      func(from Client, to, msg string) error
}

func (m *mockServer) Name() string                              { return m.DoName() }
func (m *mockServer) AddClient(c Client) error                  { return m.DoAddClient(c) }
func (m *mockServer) RemoveClient(c Client) error               { return m.DoRemoveClient(c) }
func (m *mockServer) Handle(c Client, msg parser.Message) error { return m.DoHandle(c, msg) }
func (m *mockServer) Channels() map[string]Channel              { return m.DoChannels() }
func (m *mockServer) FindChannel(ch string) Channel             { return m.DoFindChannel(ch) }
func (m *mockServer) AddChannel(ch Channel)                     { m.DoAddChannel(ch) }
func (m *mockServer) FindByNick(nick string) Client             { return m.DoFindByNick(nick) }
func (m *mockServer) Privmsg(from Client, to, msg string) error { return m.DoPrivmsg(from, to, msg) }

type mockClient struct {
	DoNick     func() string // e.g. "foo"
	DoSetNick  func(nick string) string
	DoUser     func() string // e.g. "foo"
	DoSetUser  func(user string) string
	DoReal     func() string // e.g. "Billy Joe Foo"
	DoSetReal  func(realname string) string
	DoHost     func() string                                      // e.g. "127.0.0.1"
	DoPrefix   func() string                                      // e.g., "foo!foo@127.0.0.1"
	DoSend     func(prefix, command string, args ...string) error // send a message to the client
	DoJoin     func(c Channel)                                    // join user to channel
	DoPart     func(c Channel)                                    // part user from channel
	DoChannels func() []Channel                                   // list of channels user is in
}

func TestHandleNick(t *testing.T) {
}

func TestHandleUser(t *testing.T) {
}

func TestHandleQuit(t *testing.T) {
}

func TestHandlePing(t *testing.T) {
}

func TestHandlePrivmsg(t *testing.T) {
}

func TestHandleMode(t *testing.T) {
}

func TestHandleWho(t *testing.T) {
}

func TestHandleJoin(t *testing.T) {
}

func TestHandlePart(t *testing.T) {
}

func TestHandleList(t *testing.T) {
}

func TestHandleTopic(t *testing.T) {
}

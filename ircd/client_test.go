package ircd

import (
	"fmt"
	"net"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/PacketFire/go-ircd/parser"
)

type fakeServer struct {
	nclients int32
}

func (f *fakeServer) Name() string {
	return "testserver"
}

func (f *fakeServer) AddClient(c Client) error {
	atomic.AddInt32(&f.nclients, 1)
	return nil
}

func (f *fakeServer) RemoveClient(c Client) error {
	nv := atomic.AddInt32(&f.nclients, -1)
	if nv < 0 {
		panic("negative client count")
	}

	return nil
}

func (f *fakeServer) Handle(c Client, m parser.Message) error {
	h, ok := msgtab[m.Command]
	if !ok {
		return fmt.Errorf("command %q not implemented", m.Command)
	}

	return h(f, c, m)
}

func (f *fakeServer) Channels() map[string]Channel {
	return nil
}

func (f *fakeServer) FindChannel(ch string) Channel {
	return nil
}

func (f *fakeServer) AddChannel(ch Channel) {
}

func (f *fakeServer) FindByNick(nick string) Client {
	return nil
}

func (f *fakeServer) Privmsg(from Client, to, msg string) error {
	return nil
}

func TestClientEOF(t *testing.T) {
	srv := &fakeServer{}

	// the irc client
	srvcon, clntcon := net.Pipe()

	c := NewClient(srv, srvcon)

	srv.AddClient(c)

	clntcon.Close()

	for srv.nclients > 0 {
		runtime.Gosched()
	}
}

// should be part of ircd tests.. oh well.
func TestClientHandlers(t *testing.T) {
	srv := &fakeServer{}

	// the irc client
	srvcon, clntcon := net.Pipe()

	c := NewClient(srv, srvcon)

	srv.AddClient(c)

	clntcon.Close()

	for srv.nclients > 0 {
		runtime.Gosched()
	}
}

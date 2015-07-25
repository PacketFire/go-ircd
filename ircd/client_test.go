package ircd

import (
	"net"
	"runtime"
	"testing"
)

func TestClientEOF(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

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
	srv, donef := mkServer()
	defer donef()

	// the irc client
	srvcon, clntcon := net.Pipe()

	c := NewClient(srv, srvcon)

	srv.AddClient(c)

	clntcon.Close()

	for srv.nclients > 0 {
		runtime.Gosched()
	}
}

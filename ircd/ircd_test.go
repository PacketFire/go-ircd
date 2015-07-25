package ircd

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/kballard/goirc/irc"
)

func tlisten() (net.Listener, net.Addr) {
	taddr := net.TCPAddr{IP: net.ParseIP("127.0.0.1")}
	l, err := net.ListenTCP("tcp", &taddr)
	if err != nil {
		panic(err)
	}

	return l, l.Addr()
}

func mkServer() (*Ircd, func()) {
	l, _ := tlisten()
	defer l.Close()

	conf := Config{"test", l}

	d := New(conf)

	go d.Serve()

	return d, func() {
		err := d.Stop()
		if err != nil {
			panic(err)
		}
	}
}

func TestServerAddRemove(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	tests := []struct {
		c      Client
		adderr bool
		remerr bool
	}{
		{&fakeClient{nick: "", host: "127.0.0.1"}, true, true},
		{&fakeClient{nick: "test", host: "127.0.0.1"}, false, false},
		{&fakeClient{nick: "test", host: "127.0.0.1"}, true, true},
	}

	for _, tt := range tests {
		err := srv.AddClient(tt.c)
		if tt.adderr != (err != nil) {
			t.Errorf("have %v want %v", tt.adderr, err)
		}

	}

	for _, tt := range tests {
		err := srv.RemoveClient(tt.c)
		if tt.remerr != (err != nil) {
			t.Errorf("have %v want %v", tt.remerr, err)
		}
	}
}

func TestServerChannels(t *testing.T) {
	srv, donef := mkServer()
	defer donef()

	chans := []string{"&foo", "#bar"}

	for _, chname := range chans {
		ch := NewChannel(srv, srv.Name(), chname)
		srv.AddChannel(ch)
		och := srv.FindChannel(chname)
		if och == nil {
			t.Errorf("channel not found: %v", chname)
		}
	}

	ochans := srv.Channels()

	for _, chname := range chans {
		if _, ok := ochans[chname]; !ok {
			t.Errorf("channel not found: %v", chname)
		}
	}
}

func BenchmarkJoinPart(b *testing.B) {
	l, err := net.Listen("tcp", "0.0.0.0:6667")
	if err != nil {
		b.Fatalf("main: can't listen: %v", err)
	}
	//defer l.Close()

	conf := Config{
		Hostname: "testing",
		Listener: l,
	}

	d := New(conf)
	go d.Serve()
	//defer d.Stop()

	partc := make(chan irc.Line)
	eomotdc := make(chan irc.Line)
	eonamesc := make(chan irc.Line)

	part := func(con *irc.Conn, line irc.Line) {
		partc <- line
	}

	eomotd := func(con *irc.Conn, line irc.Line) {
		eomotdc <- line
	}

	eonames := func(con *irc.Conn, line irc.Line) {
		eonamesc <- line
	}

	config := irc.Config{
		Host: "127.0.0.1",
		Init: func(hr irc.HandlerRegistry) {
			hr.AddHandler(irc.DISCONNECTED, func(*irc.Conn, irc.Line) {
				close(partc)
				close(eomotdc)
				close(eonamesc)
			})
			hr.AddHandler("PART", part)
			hr.AddHandler("376", eomotd)
			hr.AddHandler("366", eonames)
		},

		Nick:     fmt.Sprintf("%d", rand.Int31()),
		User:     "goirc",
		RealName: "goirc",
	}

	c, err := irc.Connect(config)
	if err != nil {
		panic(err)
	}

	<-eomotdc

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c.Join([]string{"#testing"}, nil)
		<-eonamesc
		//c.Raw("PART #testing")
		c.Part([]string{"#testing"}, "bye")
		_, ok := <-partc
		if !ok {
			break
		}
	}

	c.Quit("")

	if err := l.Close(); err != nil {
		b.Fatal(err)
	}

	time.Sleep(time.Millisecond)
}

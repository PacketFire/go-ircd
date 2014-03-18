package ircd

import (
	"fmt"
	"github.com/kballard/goirc/irc"
	"math/rand"
	"net"
	"testing"
	"time"
)

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

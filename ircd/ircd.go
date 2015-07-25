package ircd

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PacketFire/go-ircd/parser"
)

type Server interface {
	Name() string
	AddClient(c Client) error
	RemoveClient(c Client) error
	Handle(c Client, m parser.Message) error

	Channels() map[string]Channel
	FindChannel(ch string) Channel
	AddChannel(ch Channel)

	FindByNick(nick string) Client
	Privmsg(from Client, to, msg string) error
}

type Config struct {
	Hostname string
	Listener net.Listener
}

type Ircd struct {
	conf     Config
	boottime time.Time

	// number of currently connected clients
	nclients int32

	// Client set, protected by RWMutex
	clients map[string]Client
	cm      sync.RWMutex

	channels map[string]Channel
	chm      sync.RWMutex

	stop chan chan error
}

func New(conf Config) *Ircd {
	return &Ircd{
		conf:     conf,
		boottime: time.Now(),
		clients:  make(map[string]Client),
		channels: make(map[string]Channel),
		stop:     make(chan chan error, 1),
	}
}

func (i *Ircd) Name() string {
	return i.conf.Hostname
}

func (i *Ircd) AddClient(c Client) error {
	nick := c.Nick()
	if nick == "" {
		return fmt.Errorf("bad nick")
	}

	if oldc := i.FindByNick(nick); oldc != nil {
		return fmt.Errorf("nick exists")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	i.clients[nick] = c
	atomic.AddInt32(&i.nclients, 1)

	i.DoMotd(c)

	return nil
}

func (i *Ircd) RemoveClient(c Client) error {
	nick := c.Nick()
	if nick == "" {
		return fmt.Errorf("bad nick")
	}

	oldc := i.FindByNick(nick)
	if oldc == nil {
		return fmt.Errorf("no such nick %s", nick)
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	delete(i.clients, nick)
	atomic.AddInt32(&i.nclients, -1)

	return nil
}

func (i *Ircd) FindByNick(nick string) Client {
	i.cm.RLock()
	defer i.cm.RUnlock()

	for onick, cl := range i.clients {
		if nick == onick {
			return cl
		}
	}

	return nil
}

func (i *Ircd) Channels() map[string]Channel {
	i.chm.RLock()
	defer i.chm.RUnlock()

	chans := make(map[string]Channel, len(i.channels))

	for name, ch := range i.channels {
		chans[name] = ch
	}

	return chans
}

func (i *Ircd) FindChannel(name string) Channel {
	i.chm.RLock()
	defer i.chm.RUnlock()

	return i.channels[name]
}

func (i *Ircd) AddChannel(ch Channel) {
	i.chm.RLock()
	defer i.chm.RUnlock()

	name := ch.Name()
	if _, ok := i.channels[name]; ok {
		panic("channel already exists")
	}

	i.channels[name] = ch
}

// Stop the irc server.
func (i *Ircd) Stop() error {
	stop := make(chan error)
	i.stop <- stop
	return <-stop
}

// Accept connections and serve clients until an error or a call to Stop occurs.
func (i *Ircd) Serve() (err error) {
	accept := make(chan net.Conn)

	go func() {
		for {
			rw, e := i.conf.Listener.Accept()
			if e != nil {
				break
			}
			accept <- rw
		}
	}()

forever:
	for {
		select {
		case stop := <-i.stop:
			i.conf.Listener.Close()
			stop <- err
			break forever
		case rw := <-accept:
			NewClient(i, rw)
		}
	}

	return
}

func (i *Ircd) Handle(c Client, m parser.Message) error {
	h, ok := msgtab[m.Command]
	if !ok {
		return fmt.Errorf("command %q not implemented", m.Command)
	}

	go func() {
		if err := h(i, c, m); err != nil {
			Errorf(c, "%s: %s", m.Command, err)
		}
	}()
	return nil
}

type MessageHandler func(s Server, c Client, m parser.Message) error

var (
	msgtab = map[string]MessageHandler{
		"NICK":    HandleNick,
		"USER":    HandleUser,
		"QUIT":    HandleQuit,
		"PING":    HandlePing,
		"PRIVMSG": HandlePrivmsg,
		"MODE":    HandleMode,
		"WHO":     HandleWho,
		"JOIN":    HandleJoin,
		"PART":    HandlePart,
		"LIST":    HandleList,
		"TOPIC":   HandleTopic,
	}
)

// Send a PRIVMSG from Client to channel or another client by name
func (i *Ircd) Privmsg(from Client, to, msg string) error {
	if tocl := i.FindByNick(to); tocl != nil {
		return Privmsg(tocl, from, msg)
	} else if toch := i.FindChannel(to); toch != nil {
		return toch.Send(from.Prefix(), "PRIVMSG", toch.Name(), msg)
	} else {
		return from.Send(i.Name(), "401", from.Nick(), to, "No such user/nick")
	}
}

func (i *Ircd) DoMotd(c Client) {
	Numeric(i.Name(), c, "001", fmt.Sprintf("Welcome %s", c.Prefix()))
	Numeric(i.Name(), c, "002", fmt.Sprintf("We are %s running go-ircd", i.Name()))
	Numeric(i.Name(), c, "003", fmt.Sprintf("Booted %s", i.boottime))
	Numeric(i.Name(), c, "004", i.Name(), "go-ircd", "v", "m")

	nc := atomic.LoadInt32(&i.nclients)

	Numeric(i.Name(), c, "251", fmt.Sprintf("There are %d users and %d services on %d servers", nc, 0, 1))
	Numeric(i.Name(), c, "252", "0", "operator(s) online")
	Numeric(i.Name(), c, "253", "0", "unknown connection(s)")
	Numeric(i.Name(), c, "254", "0", "channel(s) formed")
	Numeric(i.Name(), c, "255", fmt.Sprintf("I have %d clients and %d servers", nc, 1))

	Numeric(i.Name(), c, "375", "- Message of the Day -")
	Numeric(i.Name(), c, "372", "- It works!")
	Numeric(i.Name(), c, "376", "End of MOTD")
}

func SplitPrefix(prefix string) (nick, user, host string) {
	bang := strings.LastIndex(prefix, "!")
	nick = prefix[:bang]

	prefix = prefix[bang+1:]
	at := strings.LastIndex(prefix, "@")
	user = prefix[:at]

	host = prefix[at+1:]
	return
}

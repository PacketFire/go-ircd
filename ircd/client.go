package ircd

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/PacketFire/go-ircd/parser"
)

type Client interface {
	Nick() string // e.g. "foo"
	SetNick(nick string) string
	User() string // e.g. "foo"
	SetUser(user string) string
	Real() string // e.g. "Billy Joe Foo"
	SetReal(realname string) string
	Host() string                                      // e.g. "127.0.0.1"
	Prefix() string                                    // e.g., "foo!foo@127.0.0.1"
	Send(prefix, command string, args ...string) error // send a message to the client
	Join(c Channel)                                    // join user to channel
	Part(c Channel)                                    // part user from channel
	Channels() []Channel                               // list of channels user is in
}

// Irc client.
type ircClient struct {
	// server reference
	srv Server

	in   chan parser.Message
	out  chan parser.Message
	quit chan string

	// connection
	con net.Conn

	// scanner of incoming irc messages
	inlines *bufio.Scanner

	// used to prevent multiple clients appearing on NICK
	welcome sync.Once

	// only QUIT once
	quitonce sync.Once

	// RWMutex for the below data. we can't have multiple people reading/writing..
	mu sync.RWMutex

	// various names
	nick, user, realname string
	host                 string

	// user modes
	modes *Modeset

	// Channels we are on
	channels map[string]Channel
}

// Allocate a new Client
func NewClient(srv Server, c net.Conn) *ircClient {
	cl := &ircClient{
		srv:      srv,
		in:       make(chan parser.Message, 10),
		out:      make(chan parser.Message, 10),
		quit:     make(chan string),
		con:      c,
		inlines:  bufio.NewScanner(c),
		modes:    NewModeset(),
		channels: make(map[string]Channel),
	}

	// grab just the ip of the remote user. pretty sure it's a TCPConn...
	if c != nil {
		if tcpa, ok := c.RemoteAddr().(*net.TCPAddr); ok {
			cl.host = tcpa.IP.String()
		}
	}

	if cl.host == "" {
		cl.host = "UNKNOWN"
	}

	go cl.readin()
	go cl.loop()

	return cl
}

func (c *ircClient) readin() {
	defer c.con.Close()

	for c.inlines.Scan() {
		var m parser.Message
		if err := m.UnmarshalText(c.inlines.Bytes()); err != nil {
			Errorf(c, "malformed message")
		}

		log.Printf("readin: %s -> %s", c, m)

		c.in <- m
	}

	if err := c.inlines.Err(); err != nil {
		c.quitonce.Do(func() {
			c.quit <- fmt.Sprintf("read error: %s", err)
		})
		log.Printf("readin error: %s", err)
	} else {
		c.quitonce.Do(func() {
			c.quit <- "EOF"
		})
	}

	log.Printf("%s readin is done", c)
}

func (c *ircClient) loop() {
	for {
		select {
		case m := <-c.in:
			if err := c.srv.Handle(c, m); err != nil {
				Errorf(c, "%s", err)
			}
		case m := <-c.out:
			b, err := m.MarshalText()
			if err != nil {
				log.Printf("MarshalText: %s failed: %s", m, err)
				break
			}

			// too noisy for now.
			//log.Printf("Send: %s <- %s", c, m)
			fmt.Fprintf(c.con, "%s\r\n", b)
		case why := <-c.quit:
			log.Printf("%s QUIT: %s", c.Prefix(), why)

			for _, ch := range c.channels {
				ch.Send(c.Prefix(), "QUIT", why)
				ch.Part(c)
			}

			c.con.Close()
			c.srv.RemoveClient(c)
			return
		}
	}
}

// delete me
const (
	CNick = iota
	CUser
	CReal
	CHost
)

func (c *ircClient) Str(op int, nw string) (old string) {
	c.mu.RLock()

	switch op {
	case CNick:
		old = c.nick
	case CUser:
		old = c.user
	case CReal:
		old = c.realname
	case CHost:
		old = c.host
	}

	if nw == "" {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	switch op {
	case CNick:
		c.nick = nw
	case CUser:
		c.user = nw
	case CReal:
		c.realname = nw
	case CHost:
		c.host = nw
	}

	return
}

// Get nick
func (c *ircClient) Nick() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nick
}

// Set nick, return old. "" for get only.
func (c *ircClient) SetNick(n string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n == "" {
		return c.nick
	}

	old := c.nick
	c.nick = n
	return old
}

// Get user
func (c *ircClient) User() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.user
}

// Set user, return old. "" for get only.
func (c *ircClient) SetUser(u string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if u == "" {
		return c.user
	}

	old := c.user
	c.user = u
	return old
}

// Get real
func (c *ircClient) Real() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.realname
}

// Set realname, return old. "" for get only.
func (c *ircClient) SetReal(r string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if r == "" {
		return c.realname
	}

	old := c.realname
	c.realname = r
	return old
}

// Get host, read only.
func (c *ircClient) Host() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.host
}

// make a prefix from this client
func (c *ircClient) Prefix() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return fmt.Sprintf("%s!%s@%s", c.nick, c.user, c.host)
}

func (c *ircClient) String() string {
	return c.Prefix()
}

func (c *ircClient) Send(prefix, command string, args ...string) error {
	m := parser.Message{
		Prefix:  prefix,
		Command: command,
		Args:    args,
	}

	c.out <- m

	return nil
}

func (c *ircClient) Join(ch Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.channels[ch.Name()] = ch
}

func (c *ircClient) Part(ch Channel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.channels, ch.Name())
}

func (c *ircClient) Channels() []Channel {
	c.mu.Lock()
	defer c.mu.Unlock()

	chans := make([]Channel, len(c.channels))

	for _, ch := range c.channels {
		chans = append(chans, ch)
	}

	return chans
}

func (c *ircClient) Greet() {
	c.welcome.Do(func() {
		c.srv.AddClient(c)
	})
}

func (c *ircClient) Quit(why string) {
	c.quitonce.Do(func() {
		c.quit <- fmt.Sprintf("connection closed: %s", why)
	})
}

func Privmsg(to, from Client, msg string) error {
	return to.Send(from.Prefix(), "PRIVMSG", to.Nick(), msg)
}

func Numeric(server string, c Client, code string, msg ...string) {
	c.Send(server, code, append([]string{c.Nick()}, msg...)...)
}

func Errorf(c Client, format string, args ...interface{}) {
	c.Send("", "ERROR", fmt.Sprintf(format, args...))
}

// ErrorParams sends an error to a client indicating more parameters are required for a command.
func ErrorParams(server string, c Client, cmd string) {
	c.Send(server, "461", cmd, "not enough parameters")
}

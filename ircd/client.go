package ircd

import (
	"bufio"
	"fmt"
	"github.com/PacketFire/go-ircd/parser"
	"log"
	"net"
	"sync"
)

// Irc client.
type Client struct {
	// server reference
	serv *Ircd

	In   chan parser.Message
	Out  chan parser.Message
	Quit chan string
	Join chan *Channel
	Part chan *Channel

	// connection
	con net.Conn

	// scanner of incoming irc messages
	inlines *bufio.Scanner

	// used to prevent multiple clients appearing on NICK
	welcome sync.Once

	// only QUIT once
	quit sync.Once

	// RWMutex for the below data. we can't have multiple people reading/writing..
	mu sync.RWMutex

	// various names
	nick, user, realname string
	host                 string

	// user modes
	modes *Modeset

	// Channels we are on
	channels map[string]*Channel
}

// Allocate a new Client
func NewClient(ircd *Ircd, c net.Conn) *Client {
	cl := &Client{
		serv:     ircd,
		In:       make(chan parser.Message, 10),
		Out:      make(chan parser.Message, 10),
		Quit:     make(chan string),
		Join:     make(chan *Channel),
		Part:     make(chan *Channel),
		con:      c,
		inlines:  bufio.NewScanner(c),
		modes:    NewModeset(),
		channels: make(map[string]*Channel),
	}

	// grab just the ip of the remote user. pretty sure it's a TCPConn...
	tcpa := c.RemoteAddr().(*net.TCPAddr)
	cl.host = tcpa.IP.String()

	go cl.readin()
	go cl.loop()

	return cl
}

func (c *Client) readin() {
	defer c.con.Close()

	for c.inlines.Scan() {
		var m parser.Message
		if err := m.UnmarshalText(c.inlines.Bytes()); err != nil {
			c.Error("malformed message")
		}

		log.Printf("readin: %s -> %s", c, m)

		c.In <- m
	}

	c.serv.RemoveClient(c)

	if err := c.inlines.Err(); err != nil {
		c.quit.Do(func() {
			c.Quit <- fmt.Sprintf("ERROR: %s", err)
		})
		log.Printf("readin error: %s", err)
	} else {
		c.quit.Do(func() {
			c.Quit <- "QUIT: EOF"
		})
	}

	log.Printf("%s readin is done", c)
}

func (c *Client) loop() {

forever:
	for {
		select {
		case m := <-c.In:
			if h, ok := msgtab[m.Command]; ok {
				go func() {
					if err := h(c.serv, c, m); err != nil {
						c.Errorf("%s: %s", m.Command, err)
					}
				}()
			} else {
				c.Errorf("not implemented: %s", m.Command)
			}
		case m := <-c.Out:
			b, err := m.MarshalText()
			if err != nil {
				log.Printf("MarshalText: %s failed: %s", m, err)
			}

			log.Printf("Send: %s <- %s", c, m)
			fmt.Fprintf(c.con, "%s\r\n", b)
		case why := <-c.Quit:
			log.Printf("%s QUIT: %s", c.Prefix(), why)

			for _, ch := range c.channels {
				//ch.Part <- c
				ch.Send(c.Prefix(), "QUIT", why)
			}

			break forever
		case ch := <-c.Join:
			c.channels[ch.name] = ch
		case ch := <-c.Part:
			delete(c.channels, ch.name)
		}
	}

	c.con.Close()
}

const (
	CNick = iota
	CUser
	CReal
	CHost
)

func (c *Client) Str(op int, nw string) (old string) {
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

// Set nick, return old. "" for get only.
func (c *Client) Nick(n string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n == "" {
		return c.nick
	}

	old := c.nick
	c.nick = n
	return old
}

// Set user, return old. "" for get only.
func (c *Client) User(u string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if u == "" {
		return c.user
	}

	old := c.user
	c.user = u
	return old
}

// Set realname, return old. "" for get only.
func (c *Client) Real(r string) string {
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
func (c *Client) Host() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.host
}

// make a prefix from this client
func (c *Client) Prefix() string {
	return fmt.Sprintf("%s!%s@%s", c.nick, c.user, c.host)
}

func (c *Client) String() string {
	return c.Prefix()
}

func (c *Client) Send(prefix, command string, args ...string) {
	m := parser.Message{
		Prefix:  prefix,
		Command: command,
		Args:    args,
	}

	c.Out <- m
}

func (c *Client) Privmsg(from *Client, msg string) {
	c.Send(from.Prefix(), "PRIVMSG", c.nick, msg)
}

func (c *Client) Error(what string) {
	c.Send("", "ERROR", what)
}

func (c *Client) Errorf(format string, args ...interface{}) {
	c.Error(fmt.Sprintf(format, args...))
}

func (c *Client) EParams(cmd string) {
	c.Send(c.serv.Name(), "461", cmd, "not enough parameters")
}

func (c *Client) Numeric(code string, msg ...string) {
	c.Send(c.serv.Name(), code, append([]string{c.nick}, msg...)...)
}

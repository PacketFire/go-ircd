package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/PacketFire/go-ircd/parser"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	flag.Parse()

	defer func() {
		if r := recover(); r != nil {
			log.Printf("main: recovered from %v", r)
		}
	}()

	l, err := net.Listen("tcp", ":6667")
	if err != nil {
		log.Fatalf("main: can't listen: %v", err)
	}

	host, err := os.Hostname()
	ircd := NewIrcd(host)

	if err := ircd.Serve(l); err != nil {
		log.Printf("Serve: %s", err)
	}
}

type Ircd struct {
	hostname string
	boottime time.Time

	// number of currently connected clients
	nclients int

	// nick->client, protected by RWMutex
	clients map[string]*Client
	cm      sync.RWMutex

	channels map[string]*Channel
	chm      sync.RWMutex
}

func NewIrcd(host string) Ircd {
	return Ircd{
		hostname: host,
		boottime: time.Now(),
		clients:  make(map[string]*Client),
		channels: make(map[string]*Channel),
	}
}

// Allocate a new Channel
func (i *Ircd) NewChannel(name string) *Channel {
	ch := &Channel{
		serv:   i,
		name:   name,
		users:  make(map[*Client]struct{}),
		modes:  NewModeset(),
		umodes: make(map[*Client]Modeset),
	}

	return ch
}

// Allocate a new Client
func (i *Ircd) NewClient(c net.Conn) *Client {
	cl := Client{
		serv:    i,
		con:     c,
		inlines: bufio.NewScanner(c),
		modes:   NewModeset(),
	}

	// grab just the ip of the remote user. pretty sure it's a TCPConn...
	tcpa := c.RemoteAddr().(*net.TCPAddr)

	cl.host = tcpa.IP.String()

	return &cl
}

func (i *Ircd) AddClient(c *Client) error {
	if c.nick == "" {
		return fmt.Errorf("bad nick")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	if _, ok := i.clients[c.nick]; ok {
		return fmt.Errorf("nick exists")
	}

	i.clients[c.nick] = c
	i.nclients++

	return nil
}

func (i *Ircd) RemoveClient(c *Client) error {
	if c.nick == "" {
		return fmt.Errorf("bad nick")
	}

	i.cm.Lock()
	defer i.cm.Unlock()

	if _, ok := i.clients[c.nick]; ok {
		delete(i.clients, c.nick)
		i.nclients--
	} else {
		return fmt.Errorf("no such nick %s", c.nick)
	}

	return nil
}

func (i *Ircd) FindByNick(nick string) *Client {
	i.cm.RLock()
	defer i.cm.RUnlock()

	return i.clients[nick]
}

func (i *Ircd) FindChannel(name string) *Channel {
	i.chm.RLock()
	defer i.chm.RUnlock()

	return i.channels[name]
}

func (i *Ircd) ChangeNick(old, nw string) error {
	i.cm.Lock()
	defer i.cm.Unlock()
	if c, ok := i.clients[old]; ok {
		i.clients[nw] = c
		delete(i.clients, old)
	} else {
		return fmt.Errorf("no such nick %s", old)
	}

	return nil
}

func (i *Ircd) Serve(l net.Listener) error {
	for {
		rw, err := l.Accept()
		if err != nil {
			return err
		}

		c := i.NewClient(rw)

		go i.serveClient(c)
	}
}

type MessageHandler func(i *Ircd, c *Client, m parser.Message) error

var (
	msgtab = map[string]MessageHandler{
		"NICK":    (*Ircd).HandleNick,
		"USER":    (*Ircd).HandleUser,
		"QUIT":    (*Ircd).HandleQuit,
		"PING":    (*Ircd).HandlePing,
		"PRIVMSG": (*Ircd).HandlePrivmsg,
		"MODE":    (*Ircd).HandleMode,
		"WHO":     (*Ircd).HandleWho,
		"JOIN":    (*Ircd).HandleJoin,
		"PART":    (*Ircd).HandlePart,
	}
)

func (i *Ircd) serveClient(c *Client) {
	defer c.con.Close()

	for c.inlines.Scan() {
		var m parser.Message
		if err := m.UnmarshalText(c.inlines.Bytes()); err != nil {
			c.Error("malformed message")
		}

		log.Printf("Client.Serve: %s -> %s", c, m)

		if h, ok := msgtab[m.Command]; ok {
			if err := h(i, c, m); err != nil {
				c.Errorf("%s: %s", m.Command, err)
			}
		} else {
			c.Errorf("not implemented: %s", m.Command)
		}
	}

	i.RemoveClient(c)

	if err := c.inlines.Err(); err != nil {
		log.Printf("serveClient: %s", err)
	}

	log.Printf("serveClient: %s is done", c)
}

func (i *Ircd) HandleNick(c *Client, m parser.Message) error {
	if len(m.Args) != 1 {
		c.EParams(m.Command)
		return nil
	} else if i.FindByNick(m.Args[0]) != nil {
		// check if nick is in use
		c.Send(i.hostname, "433", "*", m.Args[0], "Nickname already in use")
		return nil
	}

	// write lock
	c.Lock()

	oldnick := c.nick
	// check if we're actually updating an existing client's nick.
	if oldc := i.FindByNick(c.nick); oldc != nil {
		i.ChangeNick(c.nick, m.Args[0])
	}

	c.nick = m.Args[0]
	c.Unlock()

	c.RLock()
	defer c.RUnlock()
	if c.nick != "" && c.user != "" {
		// ack the nick change only if we have an established user/nick
		c.Send(fmt.Sprintf("%s!%s@%s", oldnick, c.user, c.host), "NICK", c.nick)

		// send motd when everything is ready, just once
		c.welcome.Do(func() {
			i.AddClient(c)
			i.DoMotd(c)
		})
	}

	return nil
}

func (i *Ircd) HandleUser(c *Client, m parser.Message) error {
	if len(m.Args) != 4 {
		c.EParams(m.Command)
		return nil
	}

	// write lock
	c.Lock()
	c.user = m.Args[0]
	c.realname = m.Args[3]
	c.Unlock()

	c.RLock()
	defer c.RUnlock()
	if c.nick != "" && c.user != "" {
		// send motd when everything is ready, just once
		c.welcome.Do(func() {
			i.AddClient(c)
			i.DoMotd(c)
		})
	}

	return nil
}

func (i *Ircd) HandleQuit(c *Client, m parser.Message) error {
	c.Error("goodbye")
	c.con.Close()

	return nil
}

func (i *Ircd) HandlePing(c *Client, m parser.Message) error {
	if len(m.Args) != 1 {
		c.EParams(m.Command)
		return nil
	}

	c.Send(i.hostname, "PONG", i.hostname, m.Args[0])

	return nil
}

func (i *Ircd) HandlePrivmsg(c *Client, m parser.Message) error {
	if len(m.Args) != 2 {
		c.EParams(m.Command)
		return nil
	}

	i.Privmsg(c, m.Args[0], m.Args[1])

	return nil
}

const (
	ModeQuery = iota
	ModeAdd
	ModeDel
)

// Change modes.
//
// TODO(mischief): check for user/chan modes when channels are implemented
func (i *Ircd) HandleMode(c *Client, m parser.Message) error {
	c.Lock()
	defer c.Unlock()

	//dir := ModeAdd

	switch n := len(m.Args); n {
	case 1, 2:
		// query
		if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
			// channel
			if ch := i.FindChannel(m.Args[0]); ch != nil {
				if n == 1 {
					// get
					ch.um.RLock()
					defer ch.um.RUnlock()
					return nil
				} else {
					// set
					ch.um.Lock()
					defer ch.um.Unlock()
					modes, _ := ch.modes.GetString()
					c.Send("324", m.Args[0], modes)
					return nil
				}
			} else {
				// not found
				c.Numeric("401", m.Args[0], "No such nick/channel")
				return nil
			}
		} else {
			// user
			if m.Args[0] != c.nick {
				// do nothing if this query is not for the sending user
				return nil
			}

			if n == 1 {
				// get
				modes, _ := c.modes.GetString()
				c.Numeric("221", modes)
			} else {
				// set
				// TODO implement mode set for user
				c.EParams(m.Command)
				return nil
			}
		}
	default:
		c.EParams(m.Command)
		return nil
	}

	return nil
}

// WHO
//
// reply format:
// "<channel> <user> <host> <server> <nick> ( "H" / "G" > ["*"] [ ( "@" / "+" ) ] :<hopcount> <real name>" */
func (i *Ircd) HandleWho(c *Client, m parser.Message) error {
	//	var operOnly bool

	// TODO: zero args should show all non-invisible users
	if len(m.Args) < 1 {
		goto done
	}

	if strings.Index(m.Args[0], "#") == 0 || strings.Index(m.Args[0], "&") == 0 {
		// WHO for channel
		u := make(map[string]*Client)

		i.chm.RLock()
		if ch, ok := i.channels[m.Args[0]]; ok {
			ch.um.RLock()
			for cl, _ := range ch.users {
				cl.RLock()
				u[cl.nick] = cl
				cl.RUnlock()
			}
			ch.um.RUnlock()
		}
		i.chm.RUnlock()

		for _, cl := range u {
			// read lock cl
			cl.RLock()
			c.Send(c.serv.hostname, "352", c.nick, m.Args[0], cl.user, cl.host, i.hostname, cl.nick, "H", fmt.Sprintf("0 %s", cl.realname))
			cl.RUnlock()
		}

	} else {
		// WHO for nick
		if cl := i.FindByNick(m.Args[0]); cl != nil {
			cl.RLock()
			c.Send(c.serv.hostname, "352", c.nick, "0", cl.user, cl.host, i.hostname, cl.nick, "H", fmt.Sprintf("0 %s", cl.realname))
			cl.RUnlock()
		}
	}

	/*
		if len(m.Args) == 2 && m.Args[1] == "o" {
			operOnly = true
		} else {
			operOnly = false
		}
	*/

done:
	c.Send(i.hostname, "315", "end of WHO")

	return nil
}

// JOIN
func (i *Ircd) HandleJoin(c *Client, m parser.Message) error {
	var thech *Channel
	if len(m.Args) < 1 {
		c.Numeric("461", m.Command, "need more parameters")
		return nil
	}

	log.Printf("%s attempts to join %q", c, m.Args[0])

	i.chm.Lock()
	defer i.chm.Unlock()

	chans := strings.Split(m.Args[0], ",")

	for _, chname := range chans {
		if ch, ok := i.channels[chname]; ok {
			// channel exists - easy
			ch.AddClient(c)

			thech = ch
		} else {
			// channel doesn't exist

			// sanity checks on channel.
			if chname == "" {
				c.Numeric("403", "*", "No such channel")
				return nil
			}

			if strings.Index(chname, "#") != 0 && strings.Index(chname, "&") != 0 {
				c.Numeric("403", chname, "No such channel")
				return nil
			}

			if len(chname) < 2 {
				c.Numeric("403", chname, "No such channel")
				return nil
			}

			// checks passed. make a channel.

			newch := i.NewChannel(chname)

			i.channels[chname] = newch

			newch.AddClient(c)

			thech = newch

			log.Printf("JOIN: New Channel %s", newch.name)
		}

		// send messages about join
		thech.Join(c)

	}

	return nil
}

func (i *Ircd) HandlePart(c *Client, m parser.Message) error {
	if len(m.Args) < 1 {
		c.Numeric("461", m.Command, "need more parameters")
		return nil
	}

	i.chm.Lock()
	defer i.chm.Unlock()

	chans := strings.Split(m.Args[0], ",")

	for _, chname := range chans {
		if ch, ok := i.channels[chname]; ok {
			if err := ch.RemoveUser(c); err != nil {
				c.Numeric("442", chname, "not on channel")
			}
		}
	}

	return nil
}

func (i *Ircd) Privmsg(from *Client, to, msg string) {
	from.RLock()
	defer from.RUnlock()

	if tocl := i.FindByNick(to); tocl != nil {
		tocl.RLock()
		defer tocl.RUnlock()
		tocl.Privmsg(from, msg)
	} else if toch := i.FindChannel(to); toch != nil {
		toch.Privmsg(from, msg)
	} else {
		from.Send(i.hostname, "401", from.nick, to, "No such user/nick")
	}
}

func (i *Ircd) DoMotd(c *Client) {
	c.Numeric("001", fmt.Sprintf("Welcome %s", c.Prefix()))
	c.Numeric("002", fmt.Sprintf("We are %s running go-ircd", i.hostname))
	c.Numeric("003", fmt.Sprintf("Booted %s", i.boottime))
	c.Numeric("004", i.hostname, "go-ircd", "v", "m")

	c.Numeric("251", fmt.Sprintf("There are %d users and %d services on %d servers", i.nclients, 0, 1))
	c.Numeric("252", "0", "operator(s) online")
	c.Numeric("253", "0", "unknown connection(s)")
	c.Numeric("254", "0", "channel(s) formed")
	c.Numeric("255", fmt.Sprintf("I have %d clients and %d servers", c.serv.nclients, 1))

	c.Numeric("375", "- Message of the Day -")
	c.Numeric("372", "- It works!")
	c.Numeric("376", "End of MOTD")

}

type Client struct {
	// server reference
	serv *Ircd

	// connection
	con net.Conn

	// scanner of incoming irc messages
	inlines *bufio.Scanner

	// used to prevent multiple clients appearing on NICK
	welcome sync.Once

	// RWMutex for the below data. we can't have multiple people reading/writing..
	sync.RWMutex

	// various names
	nick, user, realname string
	host                 string

	// user modes
	modes Modeset

	// Channels we are on
	channels map[string]*Channel
}

// make a prefix from this client
func (c *Client) Prefix() string {
	return fmt.Sprintf("%s!%s@%s", c.nick, c.user, c.host)
}

func (c Client) String() string {
	return c.Prefix()
}

func (c *Client) Send(prefix, command string, args ...string) error {
	m := parser.Message{
		Prefix:  prefix,
		Command: command,
		Args:    args,
	}

	b, err := m.MarshalText()
	if err != nil {
		return fmt.Errorf("marshalling %s failed: %s", m, err)
	}

	log.Printf("Send: %s <- %s", c, m)
	fmt.Fprintf(c.con, "%s\r\n", b)
	return err
}

func (c *Client) Privmsg(from *Client, msg string) {
	c.RLock()
	c.Send(from.Prefix(), "PRIVMSG", c.nick, msg)
	c.RUnlock()
}

func (c *Client) Error(content string) error {
	return c.Send(c.serv.hostname, "NOTICE", []string{"*", content}...)
}

func (c *Client) Errorf(format string, args ...interface{}) error {
	return c.Error(fmt.Sprintf(format, args...))
}

func (c *Client) EParams(cmd string) error {
	return c.Send(c.serv.hostname, "461", c.nick, cmd, "not enough parameters")
}

func (c *Client) Numeric(code string, msg ...string) error {
	out := append([]string{c.nick}, msg...)
	return c.Send(c.serv.hostname, code, out...)
}
